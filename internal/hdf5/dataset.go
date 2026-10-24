package hdf5

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"unicode/utf8"
)

type contextStream struct {
	f *Reader
	r io.Reader
}

func (r contextStream) Read(p []byte) (int, error) {
	if err := r.f.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

type datatype struct {
	class            byte
	width            uint64
	big, signed      bool
	padding, charset byte
}
type filter struct {
	id   uint16
	args []uint32
}

func typeInfo(b []byte) (datatype, error) {
	c := cursor{b: b}
	v := c.u8()
	flags := c.num(3)
	t := datatype{class: v & 15, width: uint64(c.u32())}
	if v>>4 < 1 || v>>4 > 3 || t.width == 0 || t.width > 1<<20 {
		return t, fmt.Errorf("unsupported HDF5 datatype version/width")
	}
	switch t.class {
	case 0:
		if t.width != 1 && t.width != 2 && t.width != 4 && t.width != 8 {
			return t, fmt.Errorf("unsupported HDF5 integer width")
		}
		if flags&^uint64(9) != 0 || c.u16() != 0 || uint64(c.u16()) != 8*t.width {
			return t, fmt.Errorf("unsupported HDF5 integer bit layout")
		}
		t.big = flags&1 != 0
		t.signed = flags&8 != 0
	case 3:
		t.padding = byte(flags & 15)
		t.charset = byte((flags >> 4) & 15)
		if flags>>8 != 0 || t.padding > 2 || t.charset > 1 {
			return t, fmt.Errorf("unsupported HDF5 fixed string encoding")
		}
	default:
		return t, fmt.Errorf("unsupported HDF5 datatype class %d (requires integers or fixed strings)", t.class)
	}
	return t, c.err
}

func filters(b []byte) ([]filter, error) {
	c := cursor{b: b}
	version, n := c.u8(), c.u8()
	if (version != 1 && version != 2) || n > 32 {
		return nil, fmt.Errorf("unsupported HDF5 filter pipeline")
	}
	if version == 1 {
		c.take(6)
	}
	var out []filter
	for i := byte(0); i < n && c.err == nil; i++ {
		id := c.u16()
		var name uint16
		if version == 1 || id >= 256 {
			name = c.u16()
		}
		flags := c.u16()
		count := c.u16()
		if flags&^uint16(1) != 0 || count > 32 {
			return nil, fmt.Errorf("unsupported HDF5 filter flags/parameters")
		}
		c.take(uint64(name))
		f := filter{id: id}
		for j := uint16(0); j < count; j++ {
			f.args = append(f.args, c.u32())
		}
		if version == 1 && count%2 != 0 {
			c.take(4)
		}
		switch id {
		case 1:
			if count != 1 || f.args[0] > 9 {
				return nil, fmt.Errorf("invalid HDF5 deflate parameters")
			}
		case 2:
			if count != 1 {
				return nil, fmt.Errorf("invalid HDF5 shuffle parameters")
			}
		case 3:
			if count != 0 {
				return nil, fmt.Errorf("invalid HDF5 Fletcher32 parameters")
			}
		default:
			return nil, fmt.Errorf("unsupported HDF5 filter %d", id)
		}
		out = append(out, f)
	}
	return out, c.err
}

// HDF5 Fletcher32 checksums are two sums of big-endian 16-bit words, stored little-endian.
// This follows the HDF5 filter format; odd input is padded with a zero low byte for the sum.
func fletcher(b []byte) uint32 {
	var a, bSum uint32
	for i := 0; i < len(b); i += 2 {
		v := uint32(b[i]) << 8
		if i+1 < len(b) {
			v |= uint32(b[i+1])
		}
		a += v
		a = (a & 0xffff) + (a >> 16)
		bSum += a
		bSum = (bSum & 0xffff) + (bSum >> 16)
	}
	return bSum<<16 | a
}

func (f *Reader) unfilter(data []byte, pipeline []filter, mask uint32, expected, width uint64) ([]byte, error) {
	if len(pipeline) < 32 && mask>>len(pipeline) != 0 {
		return nil, fmt.Errorf("invalid HDF5 chunk filter mask")
	}
	for i := len(pipeline) - 1; i >= 0; i-- {
		if err := f.ctx.Err(); err != nil {
			return nil, err
		}
		if mask&(1<<i) != 0 {
			continue
		}
		switch pipeline[i].id {
		case 1:
			input := bytes.NewReader(data)
			r, err := zlib.NewReader(input)
			if err != nil {
				return nil, fmt.Errorf("HDF5 deflate: %w", err)
			}
			data, err = io.ReadAll(io.LimitReader(contextStream{f, r}, int64(expected)+129))
			closeErr := r.Close()
			if err != nil {
				return nil, fmt.Errorf("HDF5 deflate: %w", err)
			}
			if closeErr != nil {
				return nil, closeErr
			}
			if uint64(len(data)) > expected+128 || input.Len() != 0 {
				return nil, fmt.Errorf("HDF5 compressed chunk exceeds size or has trailing bytes")
			}
		case 2:
			w := uint64(pipeline[i].args[0])
			if w != width || w == 0 || len(data)%int(w) != 0 {
				return nil, fmt.Errorf("invalid HDF5 shuffle element width")
			}
			n := len(data) / int(w)
			out := make([]byte, len(data))
			for j := 0; j < int(w); j++ {
				for k := 0; k < n; k++ {
					out[k*int(w)+j] = data[j*n+k]
				}
			}
			data = out
		case 3:
			if len(data) < 4 || fletcher(data[:len(data)-4]) != le.Uint32(data[len(data)-4:]) {
				return nil, fmt.Errorf("HDF5 Fletcher32 checksum mismatch")
			}
			data = data[:len(data)-4]
		}
	}
	if uint64(len(data)) != expected {
		return nil, fmt.Errorf("HDF5 chunk decoded size mismatch")
	}
	return data, nil
}

func (f *Reader) vector(path string, limit uint64) (datatype, uint64, []byte, error) {
	var t datatype
	at, err := f.resolve(path)
	if err != nil {
		return t, 0, nil, err
	}
	messages, err := f.object(at)
	if err != nil {
		return t, 0, nil, err
	}
	var space, typ, layout, pipe []byte
	seen := map[uint16]bool{}
	for _, m := range messages {
		if seen[m.kind] && (m.kind == 1 || m.kind == 3 || m.kind == 8 || m.kind == 11) {
			return t, 0, nil, fmt.Errorf("duplicate HDF5 dataset message")
		}
		seen[m.kind] = true
		switch m.kind {
		case 1:
			space = m.data
		case 3:
			typ = m.data
		case 8:
			layout = m.data
		case 11:
			pipe = m.data
		case 4, 5, 12, 13, 14, 18, 21, 22: // Fill declarations, attributes and timestamps do not change fully allocated values.
		default:
			return t, 0, nil, fmt.Errorf("unsupported HDF5 dataset message %d (external/virtual storage is not read)", m.kind)
		}
	}
	c := cursor{b: space}
	v, rank, flags := c.u8(), c.u8(), c.u8()
	if rank != 1 || flags&^byte(1) != 0 || (v != 1 && v != 2) {
		return t, 0, nil, fmt.Errorf("HDF5 %s requires a one-dimensional simple dataspace", path)
	}
	if v == 1 {
		c.take(5)
	} else {
		if c.u8() != 1 {
			return t, 0, nil, fmt.Errorf("HDF5 null/scalar dataspace is unsupported")
		}
	}
	n := c.u64()
	if flags&1 != 0 {
		c.take(8)
	}
	if c.err != nil {
		return t, 0, nil, c.err
	}
	if n > limit {
		return t, 0, nil, fmt.Errorf("HDF5 %s exceeds element limit %d", path, limit)
	}
	t, err = typeInfo(typ)
	if err != nil {
		return t, 0, nil, err
	}
	if n > MaxArray/t.width {
		return t, 0, nil, fmt.Errorf("HDF5 dataset exceeds 128 MiB decoded limit")
	}
	size := n * t.width
	f.decoded += size
	if f.decoded > 512<<20 {
		return t, 0, nil, fmt.Errorf("HDF5 decoded input exceeds 512 MiB")
	}
	var pipeline []filter
	if pipe != nil {
		pipeline, err = filters(pipe)
		if err != nil {
			return t, 0, nil, err
		}
	}
	c = cursor{b: layout}
	v, kind := c.u8(), c.u8()
	if v != 3 && v != 4 {
		return t, 0, nil, fmt.Errorf("unsupported HDF5 layout version %d", v)
	}
	var data []byte
	switch kind {
	case 0:
		if len(pipeline) > 0 {
			return t, 0, nil, fmt.Errorf("HDF5 filters require chunked storage")
		}
		bytes := uint64(c.u16())
		if bytes != size {
			return t, 0, nil, fmt.Errorf("HDF5 compact size mismatch")
		}
		data = c.take(bytes)
	case 1:
		if len(pipeline) > 0 {
			return t, 0, nil, fmt.Errorf("HDF5 filters require chunked storage")
		}
		addr, bytes := c.u64(), c.u64()
		if bytes != size {
			return t, 0, nil, fmt.Errorf("HDF5 contiguous size mismatch")
		}
		if size > 0 {
			data, err = f.at(addr, size)
		}
	case 2:
		if v != 3 {
			return t, 0, nil, fmt.Errorf("unsupported HDF5 v4 chunk index; use classic v1 B-tree chunks or MEX")
		}
		dims := c.u8()
		tree := c.u64()
		chunk, elem := uint64(c.u32()), uint64(c.u32())
		if c.err != nil {
			return t, 0, nil, c.err
		}
		if dims != 2 || chunk == 0 || elem != t.width || chunk > MaxArray/elem {
			return t, 0, nil, fmt.Errorf("invalid HDF5 vector chunk dimensions")
		}
		chunkBytes := chunk * elem
		data = make([]byte, int(size))
		seen := map[uint64]bool{}
		chunks := (n + chunk - 1) / chunk
		if chunks > 262144 {
			return t, 0, nil, fmt.Errorf("HDF5 chunk count exceeds 262144")
		}
		if tree == undefined {
			if n != 0 {
				return t, 0, nil, fmt.Errorf("HDF5 unallocated dataset is not count evidence")
			}
		} else {
			err = f.tree(tree, 1, 24, func(key []byte, addr uint64) error {
				stored := uint64(le.Uint32(key))
				mask := le.Uint32(key[4:])
				start := le.Uint64(key[8:])
				last := le.Uint64(key[16:])
				if start >= n || start%chunk != 0 || last != 0 || seen[start] || uint64(len(seen)) >= chunks {
					return fmt.Errorf("invalid/duplicate HDF5 chunk coordinate")
				}
				seen[start] = true
				if stored > chunkBytes+65536 || stored > MaxArray {
					return fmt.Errorf("HDF5 stored chunk exceeds size bound")
				}
				raw, e := f.at(addr, stored)
				if e != nil {
					return e
				}
				raw, e = f.unfilter(raw, pipeline, mask, chunkBytes, elem)
				if e != nil {
					return e
				}
				copy(data[int(start*elem):], raw[:min(chunkBytes, (n-start)*elem)])
				return nil
			})
			if err == nil && uint64(len(seen)) != chunks {
				err = fmt.Errorf("HDF5 missing chunks are not silently filled")
			}
		}
	default:
		return t, 0, nil, fmt.Errorf("unsupported HDF5 storage layout %d", kind)
	}
	if c.err != nil {
		return t, 0, nil, c.err
	}
	return t, n, data, err
}

// Integers returns a vector of nonnegative, full-width integers in source order. Floats and
// negative signed values are rejected, never rounded, truncated or reinterpreted as counts.
func (f *Reader) Integers(path string, limit uint64) ([]uint64, error) {
	t, n, b, err := f.vector(path, limit)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if t.class != 0 {
		return nil, fmt.Errorf("%s: expected HDF5 integer datatype", path)
	}
	if n > MaxArray/8 {
		return nil, fmt.Errorf("HDF5 decoded integer vector exceeds 128 MiB")
	}
	out := make([]uint64, int(n))
	var order binary.ByteOrder = le
	if t.big {
		order = binary.BigEndian
	}
	for i := range out {
		if i%65536 == 0 {
			if err = f.ctx.Err(); err != nil {
				return nil, err
			}
		}
		v := b[i*int(t.width) : (i+1)*int(t.width)]
		var x uint64
		switch t.width {
		case 1:
			x = uint64(v[0])
		case 2:
			x = uint64(order.Uint16(v))
		case 4:
			x = uint64(order.Uint32(v))
		case 8:
			x = order.Uint64(v)
		}
		if t.signed && x&(uint64(1)<<(t.width*8-1)) != 0 {
			return nil, fmt.Errorf("%s: negative HDF5 integer", path)
		}
		out[i] = x
	}
	return out, nil
}

// Strings reads fixed-length ASCII/UTF-8 strings, honoring the datatype's padding rule.
func (f *Reader) Strings(path string, limit uint64) ([]string, error) {
	t, n, b, err := f.vector(path, limit)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if t.class != 3 {
		return nil, fmt.Errorf("%s: expected HDF5 fixed string datatype", path)
	}
	if n > MaxArray/16 {
		return nil, fmt.Errorf("HDF5 decoded string vector exceeds 128 MiB")
	}
	out := make([]string, int(n))
	var total int
	for i := range out {
		if i%4096 == 0 {
			if err = f.ctx.Err(); err != nil {
				return nil, err
			}
		}
		s := b[i*int(t.width) : (i+1)*int(t.width)]
		if t.padding == 2 {
			s = bytes.TrimRight(s, " ")
		} else {
			if end := bytes.IndexByte(s, 0); end >= 0 {
				if len(bytes.Trim(s[end:], "\x00")) != 0 {
					return nil, fmt.Errorf("HDF5 nonzero string padding")
				}
				s = s[:end]
			}
		}
		if !utf8.Valid(s) {
			return nil, fmt.Errorf("HDF5 invalid UTF-8 string")
		}
		if t.charset == 0 {
			for _, v := range s {
				if v >= 128 {
					return nil, fmt.Errorf("HDF5 non-ASCII byte in ASCII string")
				}
			}
		}
		total += len(s)
		if total > 64<<20 {
			return nil, fmt.Errorf("HDF5 string values exceed 64 MiB")
		}
		out[i] = string(s)
	}
	return out, nil
}
