package hdf5

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
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
	case 1:
		if (t.width != 4 && t.width != 8) || flags&^uint64(0xff21) != 0 {
			return t, fmt.Errorf("unsupported HDF5 float layout")
		}
		offset, precision := c.u16(), c.u16()
		exponent, exponentBits, mantissa, mantissaBits, bias := c.u8(), c.u8(), c.u8(), c.u8(), c.u32()
		sign := byte(flags >> 8)
		if offset != 0 || uint64(precision) != t.width*8 || flags&0x30 != 0x20 || mantissa != 0 || (t.width == 4 && (sign != 31 || exponent != 23 || exponentBits != 8 || mantissaBits != 23 || bias != 127)) || (t.width == 8 && (sign != 63 || exponent != 52 || exponentBits != 11 || mantissaBits != 52 || bias != 1023)) {
			return t, fmt.Errorf("unsupported HDF5 non-IEEE float")
		}
		t.big = flags&1 != 0
	case 8:
		// h5py booleans are the two-member FALSE/TRUE enumeration, not arbitrary enum labels.
		if flags != 2 || t.width != 1 {
			return t, fmt.Errorf("unsupported HDF5 enumeration")
		}
		parent := c.take(12)
		base, err := typeInfo(parent)
		if err != nil || base.class != 0 || base.width != 1 {
			return t, fmt.Errorf("unsupported HDF5 enum base")
		}
		for _, name := range []string{"FALSE", "TRUE"} {
			start := len(c.b)
			end := bytes.IndexByte(c.b, 0)
			if end < 0 || string(c.take(uint64(end))) != name || c.u8() != 0 {
				return t, fmt.Errorf("unsupported HDF5 boolean labels")
			}
			if v>>4 < 3 {
				c.take(uint64((8 - (start-len(c.b))%8) % 8))
			}
		}
		if c.u8() != 0 || c.u8() != 1 {
			return t, fmt.Errorf("unsupported HDF5 boolean values")
		}
	case 9:
		if t.width != 16 || flags&15 != 1 || flags>>12 != 0 || (flags>>4)&15 > 2 || (flags>>8)&15 > 1 {
			return t, fmt.Errorf("unsupported HDF5 variable-length datatype")
		}
		parent, e := typeInfo(c.take(12))
		if e != nil || parent.class != 0 || parent.width != 1 || parent.signed {
			return t, fmt.Errorf("unsupported HDF5 string parent")
		}
		t.charset = byte(flags >> 8)
		t.padding = byte((flags >> 4) & 15)
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

func (f *Reader) array(path string, limit uint64) (datatype, []uint64, []byte, error) {
	var t datatype
	at, err := f.resolve(path)
	if err != nil {
		return t, nil, nil, err
	}
	messages, err := f.object(at)
	if err != nil {
		return t, nil, nil, err
	}
	var space, typ, layout, pipe []byte
	seen := map[uint16]bool{}
	for _, m := range messages {
		if seen[m.kind] && (m.kind == 1 || m.kind == 3 || m.kind == 8 || m.kind == 11) {
			return t, nil, nil, fmt.Errorf("duplicate HDF5 dataset message")
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
			return t, nil, nil, fmt.Errorf("unsupported HDF5 dataset message %d (external/virtual storage is not read)", m.kind)
		}
	}
	shape, n, err := dataspace(space, limit)
	if err != nil {
		return t, nil, nil, err
	}
	t, err = typeInfo(typ)
	if err != nil {
		return t, nil, nil, err
	}
	if n > MaxArray/t.width {
		return t, nil, nil, fmt.Errorf("HDF5 dataset exceeds 128 MiB decoded limit")
	}
	size := n * t.width
	f.decoded += size
	if f.decoded > 512<<20 {
		return t, nil, nil, fmt.Errorf("HDF5 decoded input exceeds 512 MiB")
	}
	var pipeline []filter
	if pipe != nil {
		pipeline, err = filters(pipe)
		if err != nil {
			return t, nil, nil, err
		}
	}
	c := cursor{b: layout}
	v, kind := c.u8(), c.u8()
	if v != 3 && v != 4 {
		return t, nil, nil, fmt.Errorf("unsupported HDF5 layout version %d", v)
	}
	var data []byte
	switch kind {
	case 0:
		if len(pipeline) > 0 {
			return t, nil, nil, fmt.Errorf("HDF5 filters require chunked storage")
		}
		bytes := uint64(c.u16())
		if bytes != size {
			return t, nil, nil, fmt.Errorf("HDF5 compact size mismatch")
		}
		data = c.take(bytes)
	case 1:
		if len(pipeline) > 0 {
			return t, nil, nil, fmt.Errorf("HDF5 filters require chunked storage")
		}
		addr, bytes := c.u64(), c.u64()
		if bytes != size {
			return t, nil, nil, fmt.Errorf("HDF5 contiguous size mismatch")
		}
		if size > 0 {
			data, err = f.at(addr, size)
		}
	case 2:
		if v != 3 {
			return t, nil, nil, fmt.Errorf("unsupported HDF5 v4 chunk index; use classic v1 B-tree chunks or MEX")
		}
		dims := c.u8()
		tree := c.u64()
		if int(dims) != len(shape)+1 || len(shape) == 0 {
			return t, nil, nil, fmt.Errorf("invalid HDF5 chunk rank")
		}
		chunks := make([]uint64, len(shape))
		chunkElements := uint64(1)
		expectedChunks := uint64(1)
		for i := range chunks {
			chunks[i] = uint64(c.u32())
			if chunks[i] == 0 || chunks[i] > MaxArray/chunkElements {
				return t, nil, nil, fmt.Errorf("invalid HDF5 chunk dimensions")
			}
			chunkElements *= chunks[i]
			parts := (shape[i] + chunks[i] - 1) / chunks[i]
			if parts > 262144 || (parts > 0 && expectedChunks > 262144/parts) {
				return t, nil, nil, fmt.Errorf("HDF5 chunk count exceeds 262144")
			}
			expectedChunks *= parts
		}
		elem := uint64(c.u32())
		if c.err != nil || elem != t.width || chunkElements > MaxArray/elem {
			return t, nil, nil, fmt.Errorf("invalid HDF5 chunk element width")
		}
		chunkBytes := chunkElements * elem
		data = make([]byte, int(size))
		seen := map[uint64]bool{}
		if tree == undefined {
			if n != 0 {
				return t, nil, nil, fmt.Errorf("HDF5 unallocated dataset is not count evidence")
			}
		} else {
			err = f.tree(tree, 1, 8+8*int(dims), func(key []byte, addr uint64) error {
				stored, mask := uint64(le.Uint32(key)), le.Uint32(key[4:])
				starts := make([]uint64, len(shape))
				flat := uint64(0)
				for i := range starts {
					starts[i] = le.Uint64(key[8+i*8:])
					if starts[i] >= shape[i] || starts[i]%chunks[i] != 0 {
						return fmt.Errorf("invalid HDF5 chunk coordinate")
					}
					flat = flat*shape[i] + starts[i]
				}
				if le.Uint64(key[8+len(shape)*8:]) != 0 || seen[flat] || uint64(len(seen)) >= expectedChunks {
					return fmt.Errorf("invalid/duplicate HDF5 chunk coordinate")
				}
				seen[flat] = true
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
				for j := uint64(0); j < chunkElements; j++ {
					if j%65536 == 0 {
						if e = f.ctx.Err(); e != nil {
							return e
						}
					}
					at, remain, inside := uint64(0), j, true
					coords := make([]uint64, len(shape))
					for k := len(shape) - 1; k >= 0; k-- {
						coords[k] = starts[k] + remain%chunks[k]
						remain /= chunks[k]
					}
					for k, x := range coords {
						if x >= shape[k] {
							inside = false
							break
						}
						at = at*shape[k] + x
					}
					if inside {
						copy(data[at*elem:(at+1)*elem], raw[j*elem:(j+1)*elem])
					}
				}
				return nil
			})
			if err == nil && uint64(len(seen)) != expectedChunks {
				err = fmt.Errorf("HDF5 missing chunks are not silently filled")
			}
		}

	default:
		return t, nil, nil, fmt.Errorf("unsupported HDF5 storage layout %d", kind)
	}
	if c.err != nil {
		return t, nil, nil, c.err
	}
	return t, shape, data, err
}

func (f *Reader) vector(path string, limit uint64) (datatype, uint64, []byte, error) {
	t, shape, b, err := f.array(path, limit)
	if err != nil {
		return t, 0, nil, err
	}
	if len(shape) != 1 {
		return t, 0, nil, fmt.Errorf("HDF5 %s requires a one-dimensional simple dataspace", path)
	}
	return t, shape[0], b, nil
}

// Integers returns a vector of nonnegative, full-width integers in source order. Floats and
// negative signed values are rejected, never rounded, truncated or reinterpreted as counts.
func (f *Reader) Integers(path string, limit uint64) ([]uint64, error) {
	t, n, b, err := f.vector(path, limit)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if t.class != 0 {
		return nil, fmt.Errorf("%s: expected HDF5 integer datatype class 0", path)
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
		return nil, err
	}
	return f.strings(t, n, b)
}
