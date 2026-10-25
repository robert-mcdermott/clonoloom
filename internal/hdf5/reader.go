// Package hdf5 reads a bounded subset of HDF5 without native libraries. Written from The HDF
// Group's File Format Specifications 1.1 and 3.0, not another reader's implementation:
// https://support.hdfgroup.org/documentation/hdf5/latest/_f_m_t3.html
// See docs/FORMATS.md for supported encodings and resource limits. Unsupported storage never
// becomes an empty dataset. This is a single-file, read-only reader; links never open files.
package hdf5

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

const (
	MaxFile     = 1 << 30
	MaxArray    = 128 << 20
	maxMetadata = 64 << 20
	undefined   = ^uint64(0)
)

var le = binary.LittleEndian

// Reader owns no file descriptor. The caller keeps r open and unchanged for its lifetime.
// It is deliberately not concurrent: traversal and decoded-byte budgets apply to one input.
type Reader struct {
	ctx                          context.Context
	r                            io.ReaderAt
	size, base, root             uint64
	readBytes, decoded, metadata uint64
	nodes                        int
	objects                      map[uint64][]message
	groups                       map[uint64]map[string]uint64
	heaps                        map[uint64]map[uint32][]byte
}

type message struct {
	kind uint16
	data []byte
}

// cursor is a checked decoder; malformed sizes set an error before any slice or allocation.
type cursor struct {
	b   []byte
	err error
}

func (c *cursor) take(n uint64) []byte {
	if c.err != nil {
		return nil
	}
	if n > uint64(len(c.b)) {
		c.err = fmt.Errorf("truncated HDF5 field")
		return nil
	}
	b := c.b[:int(n)]
	c.b = c.b[int(n):]
	return b
}
func (c *cursor) num(n uint64) uint64 {
	b := c.take(n)
	var v uint64
	for i := len(b) - 1; i >= 0; i-- {
		v = v<<8 | uint64(b[i])
	}
	return v
}
func (c *cursor) u8() byte    { return byte(c.num(1)) }
func (c *cursor) u16() uint16 { return uint16(c.num(2)) }
func (c *cursor) u32() uint32 { return uint32(c.num(4)) }
func (c *cursor) u64() uint64 { return c.num(8) }

func (f *Reader) read(at, n uint64) ([]byte, error) {
	if err := f.ctx.Err(); err != nil {
		return nil, err
	}
	if n > MaxArray || at > f.size || n > f.size-at {
		return nil, fmt.Errorf("HDF5 address or byte count outside file/128 MiB read limit")
	}
	f.readBytes += n
	f.nodes++
	if f.readBytes > 2<<30 || f.nodes > 1000000 {
		return nil, fmt.Errorf("HDF5 read budget exceeded")
	}
	b := make([]byte, int(n))
	_, err := f.r.ReadAt(b, int64(at))
	return b, err
}
func (f *Reader) at(at, n uint64) ([]byte, error) {
	if at == undefined || at > f.size-f.base {
		return nil, fmt.Errorf("HDF5 undefined or invalid address")
	}
	return f.read(f.base+at, n)
}
func (f *Reader) meta(at, n uint64) ([]byte, error) {
	if n > 4<<20 || f.metadata+n > maxMetadata {
		return nil, fmt.Errorf("HDF5 metadata limit exceeded")
	}
	f.metadata += n
	return f.at(at, n)
}
func checkSum(b []byte) error {
	if len(b) < 4 || lookup3(b[:len(b)-4], 0) != le.Uint32(b[len(b)-4:]) {
		return fmt.Errorf("HDF5 metadata checksum mismatch")
	}
	return nil
}

func Open(ctx context.Context, r io.ReaderAt, size int64) (*Reader, error) {
	if size < 0 || size > MaxFile {
		return nil, fmt.Errorf("HDF5 source exceeds 1 GiB limit")
	}
	f := &Reader{ctx: ctx, r: r, size: uint64(size), objects: map[uint64][]message{}, groups: map[uint64]map[string]uint64{}}
	var pos uint64
	for {
		b, err := f.read(pos, 8)
		if err != nil {
			return nil, fmt.Errorf("HDF5 signature not found: %w", err)
		}
		if bytes.Equal(b, []byte("\x89HDF\r\n\x1a\n")) {
			break
		}
		if pos == 0 {
			pos = 512
		} else {
			pos *= 2
		}
		if pos > f.size {
			return nil, fmt.Errorf("HDF5 signature not found")
		}
	}
	b, err := f.read(pos, 12)
	if err != nil {
		return nil, err
	}
	switch b[8] {
	case 0, 1:
		n := uint64(96)
		if b[8] == 1 {
			n += 4
		}
		b, err = f.read(pos, n)
		if err != nil {
			return nil, err
		}
		if b[9] != 0 || b[10] != 0 || b[12] != 0 || b[13] != 8 || b[14] != 8 {
			return nil, fmt.Errorf("unsupported HDF5 superblock encoding (requires 8-byte offsets and lengths)")
		}
		if le.Uint32(b[20:]) != 0 {
			return nil, fmt.Errorf("HDF5 file is not closed cleanly")
		}
		o := 24
		if b[8] == 1 {
			o += 4
		}
		f.base = le.Uint64(b[o:])
		eof := le.Uint64(b[o+16:])
		driver := le.Uint64(b[o+24:])
		if f.base != pos || eof != f.size || driver != undefined {
			return nil, fmt.Errorf("unsupported HDF5 base/EOF or external file driver")
		}
		f.root = le.Uint64(b[o+40:])
	case 2, 3:
		if b[9] != 8 || b[10] != 8 || b[11] != 0 {
			return nil, fmt.Errorf("unsupported HDF5 superblock widths or open-file flags")
		}
		b, err = f.read(pos, 48)
		if err != nil {
			return nil, err
		}
		if err = checkSum(b); err != nil {
			return nil, err
		}
		f.base = le.Uint64(b[12:])
		ext := le.Uint64(b[20:])
		eof := le.Uint64(b[28:])
		f.root = le.Uint64(b[36:])
		if f.base != pos || eof != f.size || ext != undefined {
			return nil, fmt.Errorf("unsupported HDF5 superblock extension or base/EOF")
		}
	default:
		return nil, fmt.Errorf("unsupported HDF5 superblock version %d", b[8])
	}
	return f, nil
}

func (f *Reader) object(at uint64) ([]message, error) {
	if v, ok := f.objects[at]; ok {
		return v, nil
	}
	if len(f.objects) >= 4096 {
		return nil, fmt.Errorf("HDF5 object limit exceeded")
	}
	h, err := f.meta(at, 16)
	if err != nil {
		return nil, err
	}
	modern := string(h[:4]) == "OHDR"
	var block []byte
	var expected int
	var creation bool
	if modern {
		if h[4] != 2 || h[5]&0xc0 != 0 {
			return nil, fmt.Errorf("unsupported HDF5 object header")
		}
		flags := h[5]
		creation = flags&4 != 0
		p := uint64(6)
		if flags&32 != 0 {
			p += 16
		}
		if flags&16 != 0 {
			p += 4
		}
		width := uint64(1) << (flags & 3)
		h, err = f.meta(at, p+width)
		if err != nil {
			return nil, err
		}
		c := cursor{b: h[p:]}
		n := c.num(width)
		if n > 4<<20 {
			return nil, fmt.Errorf("HDF5 object header exceeds metadata limit")
		}
		h, err = f.meta(at, p+width+n+4)
		if err != nil {
			return nil, err
		}
		if err = checkSum(h); err != nil {
			return nil, err
		}
		block = h[p+width : len(h)-4]
	} else {
		if h[0] != 1 {
			return nil, fmt.Errorf("unsupported HDF5 object header version")
		}
		expected = int(le.Uint16(h[2:]))
		block, err = f.meta(at+16, uint64(le.Uint32(h[8:])))
		if err != nil {
			return nil, err
		}
	}
	blocks := [][]byte{block}
	visited := map[uint64]bool{at: true}
	var out []message
	total := 0
	for i := 0; i < len(blocks); i++ {
		c := cursor{b: blocks[i]}
		prefix := 8
		if modern {
			prefix = 4
			if creation {
				prefix += 2
			}
		}
		for len(c.b) >= prefix && c.err == nil {
			var kind uint16
			if modern {
				kind = uint16(c.u8())
			} else {
				kind = c.u16()
			}
			n := c.u16()
			flags := c.u8()
			if modern {
				if creation {
					c.take(2)
				}
			} else {
				c.take(3)
			}
			data := c.take(uint64(n))
			if c.err != nil {
				break
			}
			total++
			if total > 65536 {
				return nil, fmt.Errorf("HDF5 message limit exceeded")
			}
			if flags&2 != 0 {
				return nil, fmt.Errorf("unsupported HDF5 shared object message")
			}
			if flags&128 != 0 && kind > 22 {
				return nil, fmt.Errorf("unsupported mandatory HDF5 message %d", kind)
			}
			if kind == 16 {
				d := cursor{b: data}
				address, size := d.u64(), d.u64()
				if d.err != nil {
					return nil, d.err
				}
				if visited[address] || len(blocks) >= 64 {
					return nil, fmt.Errorf("HDF5 continuation cycle/limit")
				}
				visited[address] = true
				b, e := f.meta(address, size)
				if e != nil {
					return nil, e
				}
				if modern {
					if len(b) < 8 || string(b[:4]) != "OCHK" {
						return nil, fmt.Errorf("invalid HDF5 continuation")
					}
					if e = checkSum(b); e != nil {
						return nil, e
					}
					b = b[4 : len(b)-4]
				}
				blocks = append(blocks, b)
			} else if kind != 0 {
				out = append(out, message{kind, data})
			}
		}
		if c.err != nil {
			return nil, c.err
		}
		if !modern && len(c.b) != 0 {
			return nil, fmt.Errorf("invalid HDF5 message alignment")
		}
	}
	if !modern && total != expected {
		return nil, fmt.Errorf("HDF5 object message count mismatch")
	}
	f.objects[at] = out
	return out, nil
}

// tree walks old-style B-trees with cycle/depth bounds. The callback sees only leaf records.
func (f *Reader) tree(at uint64, kind byte, keySize int, leaf func([]byte, uint64) error) error {
	seen := map[uint64]bool{}
	var walk func(uint64, int, int) error
	walk = func(at uint64, depth, level int) error {
		if depth > 32 || seen[at] || len(seen) >= 65536 {
			return fmt.Errorf("HDF5 B-tree cycle/limit")
		}
		seen[at] = true
		h, e := f.meta(at, 24)
		if e != nil {
			return e
		}
		if string(h[:4]) != "TREE" || h[4] != kind || (level >= 0 && int(h[5]) != level) {
			return fmt.Errorf("invalid HDF5 B-tree header")
		}
		n := int(le.Uint16(h[6:]))
		if n > 4096 {
			return fmt.Errorf("HDF5 B-tree fanout limit")
		}
		b, e := f.meta(at+24, uint64(n*(keySize+8)+keySize))
		if e != nil {
			return e
		}
		for i := 0; i < n; i++ {
			p := i * (keySize + 8)
			key := b[p : p+keySize]
			child := le.Uint64(b[p+keySize:])
			if h[5] > 0 {
				e = walk(child, depth+1, int(h[5])-1)
			} else {
				e = leaf(key, child)
			}
			if e != nil {
				return e
			}
		}
		return nil
	}
	return walk(at, 0, -1)
}

func (f *Reader) group(at uint64) (map[string]uint64, error) {
	if g, ok := f.groups[at]; ok {
		return g, nil
	}
	messages, err := f.object(at)
	if err != nil {
		return nil, err
	}
	g := map[string]uint64{}
	isGroup := false
	add := func(name string, addr uint64) error {
		if name == "" || name == "." || strings.ContainsAny(name, "/\x00") || !utf8.ValidString(name) || len(name) > 65535 {
			return fmt.Errorf("invalid HDF5 link name")
		}
		if _, ok := g[name]; ok {
			return fmt.Errorf("duplicate HDF5 link %q", name)
		}
		if len(g) >= 4096 {
			return fmt.Errorf("HDF5 group link limit")
		}
		// Local heap names can overlap; bound copied keys as well as source metadata.
		f.metadata += uint64(len(name))
		if f.metadata > maxMetadata {
			return fmt.Errorf("HDF5 link metadata limit exceeded")
		}
		g[name] = addr
		return nil
	}
	for _, m := range messages {
		switch m.kind {
		case 17:
			isGroup = true
			c := cursor{b: m.data}
			tree, heap := c.u64(), c.u64()
			if c.err != nil {
				return nil, c.err
			}
			h, e := f.meta(heap, 32)
			if e != nil {
				return nil, e
			}
			if string(h[:4]) != "HEAP" || h[4] != 0 {
				return nil, fmt.Errorf("unsupported HDF5 local heap")
			}
			names, e := f.meta(le.Uint64(h[24:]), le.Uint64(h[8:]))
			if e != nil {
				return nil, e
			}
			e = f.tree(tree, 0, 8, func(_ []byte, node uint64) error {
				h, e := f.meta(node, 8)
				if e != nil {
					return e
				}
				if string(h[:4]) != "SNOD" || h[4] != 1 {
					return fmt.Errorf("invalid HDF5 symbol node")
				}
				n := int(le.Uint16(h[6:]))
				if n > 4096 {
					return fmt.Errorf("HDF5 symbol limit")
				}
				b, e := f.meta(node+8, uint64(n*40))
				if e != nil {
					return e
				}
				for i := 0; i < n; i++ {
					p := b[i*40:]
					off := le.Uint64(p)
					if off >= uint64(len(names)) {
						return fmt.Errorf("invalid HDF5 name offset")
					}
					end := bytes.IndexByte(names[off:], 0)
					if end < 0 {
						return fmt.Errorf("unterminated HDF5 link")
					}
					addr := le.Uint64(p[8:])
					cache := le.Uint32(p[16:])
					if cache > 2 {
						return fmt.Errorf("unsupported HDF5 symbol cache")
					}
					if cache == 2 {
						addr = undefined
					}
					if e = add(string(names[off:off+uint64(end)]), addr); e != nil {
						return e
					}
				}
				return nil
			})
			if e != nil {
				return nil, e
			}
		case 2:
			isGroup = true
			c := cursor{b: m.data}
			if c.u8() != 0 {
				return nil, fmt.Errorf("unsupported HDF5 link info")
			}
			flags := c.u8()
			if flags&^byte(3) != 0 {
				return nil, fmt.Errorf("unsupported HDF5 link flags")
			}
			if flags&1 != 0 {
				c.take(8)
			}
			heap, index := c.u64(), c.u64()
			if c.err != nil {
				return nil, c.err
			}
			if heap != undefined || index != undefined {
				return nil, fmt.Errorf("unsupported HDF5 dense group; use a classic/compact group file or MEX")
			}
		case 6:
			isGroup = true
			c := cursor{b: m.data}
			if c.u8() != 1 {
				return nil, fmt.Errorf("unsupported HDF5 link version")
			}
			flags := c.u8()
			if flags&0xe0 != 0 {
				return nil, fmt.Errorf("unsupported HDF5 link flags")
			}
			var typ byte
			if flags&8 != 0 {
				typ = c.u8()
			}
			if flags&4 != 0 {
				c.take(8)
			}
			if flags&16 != 0 {
				if c.u8() > 1 {
					return nil, fmt.Errorf("unsupported HDF5 link charset")
				}
			}
			name := string(c.take(c.num(uint64(1) << (flags & 3))))
			addr := uint64(undefined)
			if typ == 0 {
				addr = c.u64()
			}
			if c.err != nil {
				return nil, c.err
			}
			if err = add(name, addr); err != nil {
				return nil, err
			}
		case 1, 3, 8:
			return nil, fmt.Errorf("expected HDF5 group, found dataset")
		case 10:
			isGroup = true
		}
	}
	if !isGroup {
		return nil, fmt.Errorf("missing HDF5 group structure")
	}
	f.groups[at] = g
	return g, nil
}

func (f *Reader) resolve(path string) (uint64, error) {
	if path == "" || path == "/" {
		return f.root, nil
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) > 32 {
		return 0, fmt.Errorf("HDF5 path depth limit")
	}
	at := f.root
	seen := map[uint64]bool{}
	for _, part := range parts {
		if seen[at] {
			return 0, fmt.Errorf("HDF5 group cycle")
		}
		seen[at] = true
		g, err := f.group(at)
		if err != nil {
			return 0, err
		}
		next, ok := g[part]
		if !ok {
			return 0, fmt.Errorf("HDF5 required path /%s is missing", strings.Join(parts, "/"))
		}
		if next == undefined {
			return 0, fmt.Errorf("unsupported HDF5 soft/external link at %s", part)
		}
		at = next
	}
	return at, nil
}
