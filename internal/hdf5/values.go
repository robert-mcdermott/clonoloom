package hdf5

// Scalar/vector/matrix values and inline attributes follow HDF5 format 3.0, sections III.E,
// IV.A.2.a, IV.A.2.d and IV.A.2.m. No native library or object deserialization is involved.
import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"unicode/utf8"
)

func dataspace(b []byte, limit uint64) ([]uint64, uint64, error) {
	c := cursor{b: b}
	v, rank, flags := c.u8(), c.u8(), c.u8()
	if (v != 1 && v != 2) || rank > 2 || flags&^byte(1) != 0 {
		return nil, 0, fmt.Errorf("unsupported HDF5 dataspace (at most two dimensions)")
	}
	if v == 1 {
		c.take(5)
	} else {
		kind := c.u8()
		if kind > 1 || (kind == 0 && rank != 0) {
			return nil, 0, fmt.Errorf("unsupported HDF5 null dataspace")
		}
	}
	shape := make([]uint64, int(rank))
	n := uint64(1)
	for i := range shape {
		shape[i] = c.u64()
		if shape[i] > limit || (shape[i] > 0 && n > limit/shape[i]) {
			return nil, 0, fmt.Errorf("HDF5 dataspace exceeds element limit %d", limit)
		}
		n *= shape[i]
	}
	if flags&1 != 0 {
		c.take(uint64(rank) * 8)
	}
	if n > limit {
		return nil, 0, fmt.Errorf("HDF5 dataspace exceeds element limit")
	}
	return shape, n, c.err
}

// Value retains exact decimal text for integer annotations as well as numeric values for
// bounded count/coordinate consumers. Numbers is nil for strings; missingness is a schema rule.
type Value struct {
	Shape   []uint64
	Text    []string
	Numbers []float64
	Kind    string
	DType   string
}

func (f *Reader) Value(path string, limit uint64) (Value, error) {
	t, shape, b, e := f.array(path, limit)
	if e != nil {
		return Value{}, fmt.Errorf("%s: %w", path, e)
	}
	return f.value(t, shape, b)
}
func (f *Reader) value(t datatype, shape []uint64, b []byte) (Value, error) {
	n := uint64(1)
	for _, d := range shape {
		n *= d
	}
	v := Value{Shape: shape}
	if n > MaxArray/24 {
		return v, fmt.Errorf("HDF5 decoded value limit")
	}
	f.decoded += n * 24
	if f.decoded > 512<<20 {
		return v, fmt.Errorf("HDF5 decoded input exceeds 512 MiB")
	}
	if t.class == 3 || t.class == 9 {
		v.Kind = "string"
		var e error
		v.Text, e = f.strings(t, n, b)
		return v, e
	}
	v.Kind = "integer"
	v.DType = fmt.Sprintf("uint%d", t.width*8)
	if t.signed {
		v.DType = fmt.Sprintf("int%d", t.width*8)
	}
	if t.class == 1 {
		v.Kind = "float"
		v.DType = fmt.Sprintf("float%d", t.width*8)
	}
	if t.class == 8 {
		v.Kind = "boolean"
		v.DType = "bool"
	}
	v.Text = make([]string, int(n))
	v.Numbers = make([]float64, int(n))
	var order binary.ByteOrder = le
	if t.big {
		order = binary.BigEndian
	}
	for i := range v.Text {
		if i%4096 == 0 {
			if e := f.ctx.Err(); e != nil {
				return Value{}, e
			}
		}
		p := b[i*int(t.width) : (i+1)*int(t.width)]
		var u uint64
		switch t.width {
		case 1:
			u = uint64(p[0])
		case 2:
			u = uint64(order.Uint16(p))
		case 4:
			u = uint64(order.Uint32(p))
		case 8:
			u = order.Uint64(p)
		default:
			return v, fmt.Errorf("unsupported numeric width")
		}
		switch t.class {
		case 0:
			if t.signed {
				x := int64(u<<(64-t.width*8)) >> (64 - t.width*8)
				v.Numbers[i] = float64(x)
				v.Text[i] = strconv.FormatInt(x, 10)
			} else {
				v.Numbers[i] = float64(u)
				v.Text[i] = strconv.FormatUint(u, 10)
			}
		case 1:
			x := math.Float64frombits(u)
			if t.width == 4 {
				x = float64(math.Float32frombits(uint32(u)))
			}
			v.Numbers[i] = x
			v.Text[i] = strconv.FormatFloat(x, 'g', -1, int(t.width*8))
		case 8:
			if u > 1 {
				return v, fmt.Errorf("invalid HDF5 boolean value")
			}
			v.Numbers[i] = float64(u)
			v.Text[i] = strconv.FormatBool(u == 1)
		default:
			return v, fmt.Errorf("unsupported numeric datatype")
		}
	}
	return v, nil
}

func (f *Reader) strings(t datatype, n uint64, b []byte) ([]string, error) {
	if t.class != 3 && t.class != 9 {
		return nil, fmt.Errorf("expected HDF5 string datatype")
	}
	if n > MaxArray/16 {
		return nil, fmt.Errorf("HDF5 decoded string vector exceeds 128 MiB")
	}
	out := make([]string, int(n))
	total := 0
	for i := range out {
		if i%4096 == 0 {
			if e := f.ctx.Err(); e != nil {
				return nil, e
			}
		}
		s := b[i*int(t.width) : (i+1)*int(t.width)]
		if t.class == 9 {
			size, addr, id := uint64(le.Uint32(s)), le.Uint64(s[4:]), le.Uint32(s[12:])
			if size == 0 && id == 0 {
				s = nil
			} else {
				var e error
				s, e = f.heapObject(addr, id)
				if e != nil {
					return nil, e
				}
				if uint64(len(s)) != size {
					return nil, fmt.Errorf("HDF5 variable string length mismatch")
				}
			}
		} else if t.padding == 2 {
			s = bytes.TrimRight(s, " ")
		} else if end := bytes.IndexByte(s, 0); end >= 0 {
			if len(bytes.Trim(s[end:], "\x00")) != 0 {
				return nil, fmt.Errorf("HDF5 nonzero string padding")
			}
			s = s[:end]
		}
		if !utf8.Valid(s) {
			return nil, fmt.Errorf("HDF5 invalid UTF-8 string")
		}
		if t.charset == 0 {
			for _, x := range s {
				if x >= 128 {
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
	f.decoded += uint64(total)
	if f.decoded > 512<<20 {
		return nil, fmt.Errorf("HDF5 decoded input exceeds 512 MiB")
	}
	return out, nil
}
func (f *Reader) heapObject(addr uint64, id uint32) ([]byte, error) {
	if id == 0 || id > 65535 {
		return nil, fmt.Errorf("invalid HDF5 global heap ID")
	}
	if f.heaps == nil {
		f.heaps = map[uint64]map[uint32][]byte{}
	}
	objects, ok := f.heaps[addr]
	if !ok {
		header, e := f.meta(addr, 16)
		if e != nil {
			return nil, e
		}
		size := le.Uint64(header[8:])
		if string(header[:4]) != "GCOL" || header[4] != 1 || size < 16 {
			return nil, fmt.Errorf("invalid HDF5 global heap")
		}
		data, e := f.meta(addr, size)
		if e != nil {
			return nil, e
		}
		objects = map[uint32][]byte{}
		c := cursor{b: data[16:]}
		for len(c.b) >= 16 && c.err == nil {
			index := uint32(c.u16())
			c.take(6)
			n := c.u64()
			if index == 0 {
				if n > uint64(len(c.b))+16 {
					return nil, fmt.Errorf("invalid HDF5 heap free space")
				}
				break
			}
			if _, exists := objects[index]; exists || n > MaxArray {
				return nil, fmt.Errorf("invalid HDF5 heap object")
			}
			objects[index] = c.take(n)
			c.take((8 - n%8) % 8)
		}
		if c.err != nil {
			return nil, c.err
		}
		f.heaps[addr] = objects
	}
	b, ok := objects[id]
	if !ok {
		return nil, fmt.Errorf("missing HDF5 global heap object")
	}
	return b, nil
}

func (f *Reader) Attribute(path, name string) (Value, error) {
	at, e := f.resolve(path)
	if e != nil {
		return Value{}, e
	}
	ms, e := f.object(at)
	if e != nil {
		return Value{}, e
	}
	var found *Value
	for _, m := range ms {
		if m.kind == 21 { // Dense attributes are not inline; refusing avoids treating unavailable metadata as absent.
			c := cursor{b: m.data}
			v, flags := c.u8(), c.u8()
			if flags&1 != 0 {
				c.take(2)
			}
			heap := c.u64()
			if v != 0 || flags&^byte(3) != 0 || c.err != nil || heap != undefined {
				return Value{}, fmt.Errorf("unsupported HDF5 dense attributes")
			}
		}
		if m.kind != 12 {
			continue
		}
		c := cursor{b: m.data}
		version, flags := c.u8(), c.u8()
		ns, ts, ss := uint64(c.u16()), uint64(c.u16()), uint64(c.u16())
		if version < 1 || version > 3 || flags != 0 {
			return Value{}, fmt.Errorf("unsupported HDF5 attribute encoding")
		}
		if version == 3 && c.u8() > 1 {
			return Value{}, fmt.Errorf("invalid HDF5 attribute charset")
		}
		field := func(n uint64) []byte {
			v := c.take(n)
			if version == 1 {
				c.take((8 - n%8) % 8)
			}
			return v
		}
		nb := field(ns)
		typ := field(ts)
		space := field(ss)
		if c.err != nil {
			return Value{}, c.err
		}
		if len(nb) == 0 || nb[len(nb)-1] != 0 {
			return Value{}, fmt.Errorf("invalid HDF5 attribute name")
		}
		if string(nb[:len(nb)-1]) != name {
			continue
		}
		if found != nil {
			return Value{}, fmt.Errorf("duplicate HDF5 attribute")
		}
		t, e := typeInfo(typ)
		if e != nil {
			return Value{}, e
		}
		shape, n, e := dataspace(space, 65536)
		if e != nil {
			return Value{}, e
		}
		if n > MaxArray/t.width {
			return Value{}, fmt.Errorf("HDF5 attribute size limit")
		}
		b := c.take(n * t.width)
		if c.err != nil {
			return Value{}, c.err
		}
		v, e := f.value(t, shape, b)
		if e != nil {
			return Value{}, e
		}
		found = &v
	}
	if found == nil {
		return Value{}, fmt.Errorf("%s: missing HDF5 attribute %q", path, name)
	}
	return *found, nil
}
