// Adapted from SituCraft internal/hdf5/writer.go (Apache-2.0).
// Extended for AnnData: UTF-8 variable-length strings, float64, booleans and dataset attributes.
// Package h5write writes bounded HDF5 files for AnnData: compact groups, contiguous
// uncompressed datasets, little-endian integers and IEEE floats, booleans and UTF-8 strings.
// HDF Group File Format Specification 3.0: superblock 2, object headers 2, global heaps.
// Identical contents and insertion order produce identical bytes; no timestamps are stored.
package h5write

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"unicode/utf8"
)

// Group is a group of datasets and groups, in the order they are added.
type Group struct {
	name     string
	children []any // *Group or *dataset
	attrs    []*dataset
	names    map[string]bool
	address  uint64
}

// File is an HDF5 file being built; its root group is the file's.
type File struct {
	Root *Group
}

// NewFile starts a file.
func NewFile() *File { return &File{Root: newGroup("")} }

func newGroup(name string) *Group { return &Group{name: name, names: map[string]bool{}} }

// datatype is a supported HDF5 atomic datatype.
type datatype struct {
	class  byte // 0 integer, 1 float, 3 fixed string, 8 boolean enum, 9 variable string
	size   uint32
	signed bool
}

// dataset is a dataset, or an attribute's value.
type dataset struct {
	name    string
	typ     datatype
	dims    []uint64 // nil: a scalar
	data    []byte   // little-endian, row by row
	address uint64   // of its object header (datasets) once written
	attrs   []*dataset
	text    []string
	dataAt  uint64
}

func (g *Group) claim(name string) error {
	if name == "" || name == "." || name == ".." || len(name) > 255 || bytes.ContainsAny([]byte(name), "/\x00") {
		return fmt.Errorf("hdf5: %q is not a name", name)
	}
	if g.names[name] {
		return fmt.Errorf("hdf5: %q is in the group twice", name)
	}
	g.names[name] = true
	return nil
}

// Group adds a group.
func (g *Group) Group(name string) (*Group, error) {
	if err := g.claim(name); err != nil {
		return nil, err
	}
	child := newGroup(name)
	g.children = append(g.children, child)
	return child, nil
}

func ints32(v []int32) []byte {
	out := make([]byte, 0, 4*len(v))
	for _, x := range v {
		out = binary.LittleEndian.AppendUint32(out, uint32(x))
	}
	return out
}

func ints64(v []int64) []byte {
	out := make([]byte, 0, 8*len(v))
	for _, x := range v {
		out = binary.LittleEndian.AppendUint64(out, uint64(x))
	}
	return out
}

// strings are fixed-length strings as long as the longest (at least 1), null-padded.
func strs(v []string) (datatype, []byte) {
	width := 1
	for _, s := range v {
		width = max(width, len(s))
	}
	out := make([]byte, width*len(v))
	for i, s := range v {
		copy(out[i*width:], s)
	}
	return datatype{class: 3, size: uint32(width)}, out
}

func (g *Group) add(name string, typ datatype, dims []uint64, data []byte) error {
	if err := g.claim(name); err != nil {
		return err
	}
	g.children = append(g.children, &dataset{name: name, typ: typ, dims: dims, data: data})
	return nil
}

// Int32s adds a one-dimensional dataset of signed 32-bit integers.
func (g *Group) Int32s(name string, v []int32) error {
	return g.add(name, datatype{0, 4, true}, []uint64{uint64(len(v))}, ints32(v))
}

// Int64s adds a one-dimensional dataset of signed 64-bit integers.
func (g *Group) Int64s(name string, v []int64) error {
	return g.add(name, datatype{0, 8, true}, []uint64{uint64(len(v))}, ints64(v))
}

// Strings adds a UTF-8 variable-length string vector, as required by AnnData.
func (g *Group) Strings(name string, v []string) error {
	for _, s := range v {
		if !utf8.ValidString(s) || bytes.IndexByte([]byte(s), 0) >= 0 {
			return fmt.Errorf("hdf5: invalid UTF-8 string")
		}
	}
	if err := g.add(name, datatype{9, 16, false}, []uint64{uint64(len(v))}, nil); err != nil {
		return err
	}
	d := g.children[len(g.children)-1].(*dataset)
	d.text = append([]string{}, v...)
	return nil
}

// ScalarString adds a UTF-8 scalar.
func (g *Group) ScalarString(name, v string) error {
	if err := g.Strings(name, []string{v}); err != nil {
		return err
	}
	g.children[len(g.children)-1].(*dataset).dims = nil
	return nil
}

// Float64s adds a row-major IEEE-754 array. NaN represents unavailable coordinates.
func (g *Group) Float64s(name string, dims []uint64, v []float64) error {
	n := uint64(1)
	for _, d := range dims {
		if d > 1<<30 || n > 1<<30 {
			return fmt.Errorf("hdf5: shape too large")
		}
		n *= d
	}
	if n != uint64(len(v)) {
		return fmt.Errorf("hdf5: shape differs from data")
	}
	raw := make([]byte, 8*len(v))
	for i, x := range v {
		binary.LittleEndian.PutUint64(raw[i*8:], math.Float64bits(x))
	}
	return g.add(name, datatype{1, 8, false}, dims, raw)
}

// Bools adds an HDF5 FALSE/TRUE enum, recognized by h5py as bool.
func (g *Group) Bools(name string, v []bool) error {
	raw := make([]byte, len(v))
	for i, x := range v {
		if x {
			raw[i] = 1
		}
	}
	return g.add(name, datatype{8, 1, false}, []uint64{uint64(len(v))}, raw)
}
func (g *Group) AttrBool(name string, v bool) {
	x := byte(0)
	if v {
		x = 1
	}
	g.attr(name, datatype{8, 1, false}, nil, []byte{x})
}

// DatasetEncoding adds AnnData encoding attributes to a previously added dataset.
func (g *Group) DatasetEncoding(name, kind, version string) error {
	for _, c := range g.children {
		if d, ok := c.(*dataset); ok && d.name == name {
			for _, kv := range [][2]string{{"encoding-type", kind}, {"encoding-version", version}} {
				t, b := strs([]string{kv[1]})
				d.attrs = append(d.attrs, &dataset{name: kv[0], typ: t, data: b})
			}
			return nil
		}
	}
	return fmt.Errorf("hdf5: no dataset %s", name)
}

func (g *Group) attr(name string, typ datatype, dims []uint64, data []byte) {
	g.attrs = append(g.attrs, &dataset{name: name, typ: typ, dims: dims, data: data})
}

// AttrString sets a scalar fixed-length string attribute.
func (g *Group) AttrString(name, v string) {
	typ, data := strs([]string{v})
	g.attr(name, typ, nil, data)
}

// AttrStrings sets an attribute of fixed-length strings.
func (g *Group) AttrStrings(name string, v []string) {
	typ, data := strs(v)
	g.attr(name, typ, []uint64{uint64(len(v))}, data)
}

// AttrInt64 sets a scalar 64-bit integer attribute.
func (g *Group) AttrInt64(name string, v int64) {
	g.attr(name, datatype{0, 8, true}, nil, ints64([]int64{v}))
}

// AttrInt64s sets an attribute of 64-bit integers.
func (g *Group) AttrInt64s(name string, v []int64) {
	g.attr(name, datatype{0, 8, true}, []uint64{uint64(len(v))}, ints64(v))
}

// undefined is the undefined address.
const undefined = ^uint64(0)

// Message types (the specification's IV.A.2).
const (
	msgDataspace = 0x01
	msgLinkInfo  = 0x02
	msgDatatype  = 0x03
	msgFillValue = 0x05
	msgLink      = 0x06
	msgLayout    = 0x08
	msgGroupInfo = 0x0a
	msgAttribute = 0x0c
)

type message struct {
	typ   byte
	flags byte
	body  []byte
}

func le16(b []byte, v int) []byte    { return binary.LittleEndian.AppendUint16(b, uint16(v)) }
func le32(b []byte, v uint32) []byte { return binary.LittleEndian.AppendUint32(b, v) }
func le64(b []byte, v uint64) []byte { return binary.LittleEndian.AppendUint64(b, v) }

// dataspaceBody is a version 2 dataspace: scalar, or simple of the dims.
func dataspaceBody(dims []uint64) []byte {
	if dims == nil {
		return []byte{2, 0, 0, 0}
	}
	b := []byte{2, byte(len(dims)), 0, 1}
	for _, d := range dims {
		b = le64(b, d)
	}
	return b
}

// datatypeBody is a version 1 datatype: little-endian two's-complement integers, or null-padded
// ASCII strings.
func datatypeBody(t datatype) []byte {
	b := []byte{t.class | 1<<4, 0, 0, 0}
	switch t.class {
	case 0:
		if t.signed {
			b[1] = 1 << 3
		}
		b = le32(b, t.size)
		b = le16(b, 0)             // bit offset
		b = le16(b, int(8*t.size)) // precision
	case 1:
		b[1] = 0x20
		b[2] = 63
		b = le32(b, 8)
		b = le16(b, 0)
		b = le16(b, 64)
		b = append(b, 52, 11, 0, 52)
		b = le32(b, 1023)
	case 8:
		b[1] = 2
		b = le32(b, 1)
		b = append(b, datatypeBody(datatype{0, 1, false})...)
		for _, s := range []string{"FALSE", "TRUE"} {
			p := append([]byte(s), 0)
			for len(p)%8 != 0 {
				p = append(p, 0)
			}
			b = append(b, p...)
		}
		b = append(b, 0, 1)
	case 9:
		b[1] = 1
		b[2] = 1
		b = le32(b, 16)
		b = append(b, datatypeBody(datatype{0, 1, false})...)
	case 3:
		b[1] = 1 // null-padded, ASCII
		b = le32(b, t.size)
	}
	return b
}

// header is a version 2 object header holding the messages, with its checksum.
func header(msgs []message) []byte {
	var chunk []byte
	for _, m := range msgs {
		chunk = append(chunk, m.typ)
		chunk = le16(chunk, len(m.body))
		chunk = append(chunk, m.flags)
		chunk = append(chunk, m.body...)
	}
	// Flags 2: the chunk's size in 4 bytes; no times, no attribute phase change values.
	b := append([]byte("OHDR"), 2, 2)
	b = le32(b, uint32(len(chunk)))
	b = append(b, chunk...)
	return le32(b, lookup3(b, 0))
}

func attributeMessage(a *dataset) message {
	dt, ds := datatypeBody(a.typ), dataspaceBody(a.dims)
	b := []byte{3, 0}
	b = le16(b, len(a.name)+1)
	b = le16(b, len(dt))
	b = le16(b, len(ds))
	b = append(b, 0) // ASCII name
	b = append(b, a.name...)
	b = append(b, 0)
	b = append(b, dt...)
	b = append(b, ds...)
	b = append(b, a.data...)
	return message{typ: msgAttribute, body: b}
}

// writer lays the file out: the superblock, every dataset's data, then the object headers,
// children before their group, so that each header knows its links' addresses.
type writer struct {
	buf bytes.Buffer
}

func (w *writer) at() uint64 { return uint64(w.buf.Len()) }

// strings writes global-heap collections of at most 4096 objects, and returns
// the 16-byte length/address/index descriptors. Empty strings use null descriptors.
func (w *writer) strings(v []string) []byte {
	out := make([]byte, 16*len(v))
	for start := 0; start < len(v); start += 4096 {
		end := min(start+4096, len(v))
		var heap []byte
		heap = append(heap, []byte("GCOL")...)
		heap = append(heap, 1, 0, 0, 0)
		heap = le64(heap, 0)
		address := w.at()
		id := 0
		for i := start; i < end; i++ {
			s := v[i]
			if len(s) == 0 {
				continue
			}
			id++
			heap = le16(heap, id)
			heap = le16(heap, 1)
			heap = le32(heap, 0)
			heap = le64(heap, uint64(len(s)))
			heap = append(heap, []byte(s)...)
			for len(heap)%8 != 0 {
				heap = append(heap, 0)
			}
			p := out[16*i:]
			binary.LittleEndian.PutUint32(p, uint32(len(s)))
			binary.LittleEndian.PutUint64(p[4:], address)
			binary.LittleEndian.PutUint32(p[12:], uint32(id))
		}
		// A collection is at least 4096 bytes; free-space object includes its header.
		free := max(16, 4096-len(heap))
		heap = le16(heap, 0)
		heap = le16(heap, 0)
		heap = le32(heap, 0)
		heap = le64(heap, uint64(free))
		heap = append(heap, make([]byte, free-16)...)
		binary.LittleEndian.PutUint64(heap[8:], uint64(len(heap)))
		w.buf.Write(heap)
	}
	return out
}

func (w *writer) data(g *Group) {
	for _, c := range g.children {
		switch c := c.(type) {
		case *Group:
			w.data(c)
		case *dataset:
			if c.text != nil {
				c.data = w.strings(c.text)
			}
			c.dataAt = undefined
			if len(c.data) > 0 {
				c.dataAt = w.at()
				w.buf.Write(c.data)
			}
		}
	}
}

func (w *writer) headers(g *Group) {
	var links []message
	for _, c := range g.children {
		var name string
		var addr uint64
		switch c := c.(type) {
		case *Group:
			w.headers(c)
			name, addr = c.name, c.address
		case *dataset:
			c.address = w.at()
			layout := []byte{3, 1}
			layout = le64(layout, c.dataAt)
			layout = le64(layout, uint64(len(c.data)))
			msgs := []message{
				{typ: msgDataspace, body: dataspaceBody(c.dims)},
				{typ: msgDatatype, flags: 1, body: datatypeBody(c.typ)},
				// Fill value, version 3: allocated late, written if set, none set.
				{typ: msgFillValue, flags: 1, body: []byte{3, 2 | 2<<2}},
				{typ: msgLayout, body: layout},
			}
			for _, a := range c.attrs {
				msgs = append(msgs, attributeMessage(a))
			}
			w.buf.Write(header(msgs))
			name, addr = c.name, c.address
		}
		link := []byte{1, 0, byte(len(name))}
		link = append(link, name...)
		links = append(links, message{typ: msgLink, body: le64(link, addr)})
	}
	info := []byte{0, 0}
	info = le64(info, undefined) // no fractal heap: the links are in the header
	info = le64(info, undefined)
	msgs := []message{{typ: msgLinkInfo, body: info}, {typ: msgGroupInfo, body: []byte{0, 0}}}
	msgs = append(msgs, links...)
	for _, a := range g.attrs {
		msgs = append(msgs, attributeMessage(a))
	}
	g.address = w.at()
	w.buf.Write(header(msgs))
}

// Bytes is the file.
func (f *File) Bytes() []byte {
	w := &writer{}
	w.buf.Write(make([]byte, 48)) // the superblock, written last
	w.data(f.Root)
	w.headers(f.Root)
	out := w.buf.Bytes()
	sb := append([]byte{0x89, 'H', 'D', 'F', '\r', '\n', 0x1a, '\n'}, 2, 8, 8, 0)
	sb = le64(sb, 0)         // base address
	sb = le64(sb, undefined) // no superblock extension
	sb = le64(sb, uint64(len(out)))
	sb = le64(sb, f.Root.address)
	sb = le32(sb, lookup3(sb, 0))
	copy(out, sb)
	return out
}

// WriteTo writes the file to w.
func (f *File) WriteTo(w io.Writer) (int64, error) {
	raw := f.Bytes()
	n, err := w.Write(raw)
	if err == nil && n != len(raw) {
		err = io.ErrShortWrite
	}
	return int64(n), err
}

// Write writes the file to path.
func (f *File) Write(path string) error {
	if f.Root == nil {
		return errors.New("hdf5: no root group")
	}
	return os.WriteFile(path, f.Bytes(), 0o644)
}

// lookup3 is Bob Jenkins' hashlittle (lookup3.c, 2006, public domain), the checksum of HDF5's
// metadata.
func lookup3(k []byte, initval uint32) uint32 {
	rot := func(x uint32, n uint) uint32 { return x<<n | x>>(32-n) }
	a := 0xdeadbeef + uint32(len(k)) + initval
	b, c := a, a
	for len(k) > 12 {
		a += binary.LittleEndian.Uint32(k)
		b += binary.LittleEndian.Uint32(k[4:])
		c += binary.LittleEndian.Uint32(k[8:])
		a -= c
		a ^= rot(c, 4)
		c += b
		b -= a
		b ^= rot(a, 6)
		a += c
		c -= b
		c ^= rot(b, 8)
		b += a
		a -= c
		a ^= rot(c, 16)
		c += b
		b -= a
		b ^= rot(a, 19)
		a += c
		c -= b
		c ^= rot(b, 4)
		b += a
		k = k[12:]
	}
	if len(k) == 0 {
		return c
	}
	var tail [12]byte
	copy(tail[:], k)
	a += binary.LittleEndian.Uint32(tail[:])
	b += binary.LittleEndian.Uint32(tail[4:])
	c += binary.LittleEndian.Uint32(tail[8:])
	c ^= b
	c -= rot(b, 14)
	a ^= c
	a -= rot(c, 11)
	b ^= a
	b -= rot(a, 25)
	c ^= b
	c -= rot(b, 16)
	a ^= c
	a -= rot(c, 4)
	b ^= a
	b -= rot(a, 14)
	c ^= b
	c -= rot(b, 24)
	return c
}
