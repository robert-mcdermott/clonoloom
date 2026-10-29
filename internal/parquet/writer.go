package parquet

// Adapted from SituCraft v0.9.0 internal/parquet/writer.go (Apache-2.0).
// ClonoLoom adds optional columns, context/bounds, sticky errors and sorted metadata.
// Apache Parquet format and Thrift Compact Protocol:
// https://parquet.apache.org/docs/file-format/ and apache/parquet-format parquet.thrift.
// A writer of flat typed scientific tables with required/optional columns. Values
// are PLAIN-encoded in version 1 data pages of about a megabyte, each compressed with GZIP, which
// every Parquet reader decodes; the footer's metadata is written in Thrift's compact protocol.
// Written from the Apache Parquet format specification (parquet-format: the file layout,
// parquet.thrift and Encodings.md). The same values always give the same bytes.

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"unicode/utf8"
)

// Field is one column a Writer writes.
type Field struct {
	Name string
	Type Type
	// Annotation says what the physical type holds: "string" (UTF-8 text in a BYTE_ARRAY),
	// "uint" (an unsigned INT32 or INT64), "uint8" (an unsigned byte in an INT32), or "" (the type
	// itself).
	Annotation string
	Optional   bool
}

// Writer writes a Parquet file: row groups, then the footer when closed.
type Writer struct {
	w         io.Writer
	pos       int64
	fields    []Field
	groups    []writtenGroup
	rows      int64
	keyValue  [][2]string
	createdBy string
	// PageSize bounds a data page's PLAIN-encoded size (about a megabyte by default).
	PageSize      int
	closed        bool
	ctx           context.Context
	err           error
	metadataBytes int
	chunks        int
}

type writtenChunk struct {
	offset            int64
	values            int64
	compressed, plain int64
}

type writtenGroup struct {
	rows   int64
	chunks []writtenChunk
}

// Converted and logical annotations (parquet.thrift).
const (
	convertedUTF8   = 0
	convertedUint8  = 11
	convertedUint32 = 13
	convertedUint64 = 14
	logicalString   = 1
	logicalInteger  = 10
)

// NewWriter starts a Parquet file on w, its columns those named; createdBy names the writer.
func NewWriter(ctx context.Context, w io.Writer, fields []Field, createdBy string) (*Writer, error) {
	if len(fields) == 0 || len(fields) > 1100 || len(createdBy) > 4096 {
		return nil, errors.New("parquet: a file needs a column")
	}
	seen := map[string]bool{}
	for _, f := range fields {
		if f.Name == "" || len(f.Name) > 256 || !utf8.ValidString(f.Name) || seen[f.Name] {
			return nil, fmt.Errorf("parquet: column name %q is empty or repeated", f.Name)
		}
		seen[f.Name] = true
		switch f.Type {
		case Boolean, Int32, Int64, Float, Double, ByteArray:
		default:
			return nil, fmt.Errorf("parquet: column %s: %s is not written", f.Name, f.Type)
		}
		if f.Annotation != "" && f.Annotation != "string" && f.Annotation != "uint" && f.Annotation != "uint8" {
			return nil, errors.New("parquet: unsupported annotation")
		}
		if f.Annotation == "string" && f.Type != ByteArray || f.Annotation == "uint" && f.Type != Int32 && f.Type != Int64 || f.Annotation == "uint8" && f.Type != Int32 {
			return nil, fmt.Errorf("parquet: column %s: %s cannot hold a %s", f.Name, f.Type, f.Annotation)
		}
	}
	out := &Writer{ctx: ctx, w: w, fields: append([]Field(nil), fields...), createdBy: createdBy, PageSize: 1 << 20}
	if err := out.write([]byte("PAR1")); err != nil {
		return nil, err
	}
	return out, nil
}

// SetKeyValue adds a key-value pair to the file's metadata (GeoParquet's "geo", pandas' "pandas").
func (w *Writer) SetKeyValue(key, value string) error {
	if w.closed || w.err != nil {
		return errors.New("parquet: writer is closed or failed")
	}
	if len(key) == 0 || len(key) > 256 || !utf8.ValidString(key) || !utf8.ValidString(value) {
		return errors.New("parquet: invalid metadata text")
	}
	if len(value)+len(key)+w.metadataBytes > 1<<20 {
		return errors.New("parquet: metadata exceeds 1 MiB")
	}
	w.metadataBytes += len(key) + len(value)
	for i, kv := range w.keyValue {
		if kv[0] == key {
			w.keyValue[i][1] = value
			return nil
		}
	}
	w.keyValue = append(w.keyValue, [2]string{key, value})
	return nil
}

func (w *Writer) write(b []byte) error {
	if w.err != nil {
		return w.err
	}
	if err := w.ctx.Err(); err != nil {
		w.err = err
		return err
	}
	if int64(len(b)) > 256<<20-w.pos {
		w.err = errors.New("parquet: output exceeds 256 MiB")
		return w.err
	}
	n, err := w.w.Write(b)
	w.pos += int64(n)
	if err == nil && n != len(b) {
		err = io.ErrShortWrite
	}
	w.err = err
	return err
}

// length is a column's number of values, for its type.
func length(f Field, v *Values) int {
	switch f.Type {
	case Boolean:
		return len(v.Bool)
	case Int32:
		return len(v.Int32)
	case Int64:
		return len(v.Int64)
	case Float:
		return len(v.Float32)
	case Double:
		return len(v.Float64)
	default:
		return len(v.Bytes)
	}
}

// WriteRowGroup writes one row group: a Values for each column, in order, of equal length, with
// the slice for the column's type set (Bool, Int32, Int64, Float32, Float64 or Bytes).
func (w *Writer) WriteRowGroup(columns []*Values) error {
	if w.closed || w.err != nil {
		return errors.New("parquet: the file is closed or failed")
	}
	if err := w.ctx.Err(); err != nil {
		return err
	}
	if w.PageSize < 1 || w.PageSize > 8<<20 || len(w.groups) >= 4096 || w.chunks+len(w.fields) > 16384 {
		return errors.New("parquet: page/group/chunk bound exceeded")
	}
	if len(columns) != len(w.fields) {
		return fmt.Errorf("parquet: %d columns given for %d", len(columns), len(w.fields))
	}
	rows := -1
	for i, c := range columns {
		if c == nil {
			return errors.New("parquet: nil column")
		}
		n := length(w.fields[i], c)
		if n > 8192 || c.Valid != nil && len(c.Valid) != n {
			return errors.New("parquet: invalid row group or validity length")
		}
		for row := 0; row < n; row++ {
			valid := c.Valid == nil || c.Valid[row]
			if !valid && !w.fields[i].Optional {
				return errors.New("parquet: null in required column")
			}
			if w.fields[i].Type == ByteArray && valid && (len(c.Bytes[row]) > 1<<20 || w.fields[i].Annotation == "string" && !utf8.Valid(c.Bytes[row])) {
				return errors.New("parquet: oversized or invalid text value")
			}
		}
		if rows >= 0 && n != rows {
			return fmt.Errorf("parquet: column %s has %d values, not %d", w.fields[i].Name, n, rows)
		}
		rows = n
	}
	groupBytes := 0
	for i, c := range columns {
		for row := 0; row < rows; row++ {
			groupBytes += size(w.fields[i], c, row) + 2
			if groupBytes > 32<<20 {
				return errors.New("parquet: row group exceeds 32 MiB")
			}
		}
	}
	g := writtenGroup{rows: int64(rows)}
	for i, c := range columns {
		chunk, err := w.writeChunk(w.fields[i], c, rows)
		if err != nil {
			w.err = err
			return err
		}
		g.chunks = append(g.chunks, chunk)
	}
	w.chunks += len(w.fields)
	w.groups = append(w.groups, g)
	w.rows += int64(rows)
	return nil
}

// encodePlain encodes values [from, to) of a column.
func encodePlain(f Field, v *Values, from, to int, out *bytes.Buffer) {
	var tmp [8]byte
	switch f.Type {
	case Boolean:
		// Bit-packed, least significant bit first.
		for i := from; i < to; i += 8 {
			var b byte
			for k := 0; k < 8 && i+k < to; k++ {
				if v.Bool[i+k] {
					b |= 1 << k
				}
			}
			out.WriteByte(b)
		}
	case Int32:
		for _, x := range v.Int32[from:to] {
			binary.LittleEndian.PutUint32(tmp[:], uint32(x))
			out.Write(tmp[:4])
		}
	case Int64:
		for _, x := range v.Int64[from:to] {
			binary.LittleEndian.PutUint64(tmp[:], uint64(x))
			out.Write(tmp[:8])
		}
	case Float:
		for _, x := range v.Float32[from:to] {
			binary.LittleEndian.PutUint32(tmp[:], math.Float32bits(x))
			out.Write(tmp[:4])
		}
	case Double:
		for _, x := range v.Float64[from:to] {
			binary.LittleEndian.PutUint64(tmp[:], math.Float64bits(x))
			out.Write(tmp[:8])
		}
	default:
		for _, b := range v.Bytes[from:to] {
			binary.LittleEndian.PutUint32(tmp[:], uint32(len(b)))
			out.Write(tmp[:4])
			out.Write(b)
		}
	}
}

// size is the PLAIN size of value i.
func size(f Field, v *Values, i int) int {
	switch f.Type {
	case Boolean:
		return 1 // an upper bound (a bit)
	case Int32, Float:
		return 4
	case Int64, Double:
		return 8
	default:
		return 4 + len(v.Bytes[i])
	}
}

func (w *Writer) writeChunk(f Field, v *Values, rows int) (writtenChunk, error) {
	c := writtenChunk{offset: w.pos, values: int64(rows)}
	var page, packed bytes.Buffer
	for from := 0; from < rows || from == 0 && rows == 0; {
		// A page holds whole values up to the page size (at least one).
		to, bytesIn := from, 0
		for to < rows && (to == from || bytesIn+size(f, v, to) <= w.PageSize) {
			bytesIn += size(f, v, to)
			to++
		}
		page.Reset()
		if f.Optional {
			var levels []byte
			for i := from; i < to; {
				present := v.Valid == nil || v.Valid[i]
				end := i + 1
				for end < to && (v.Valid == nil || v.Valid[end]) == present {
					end++
				}
				levels = binary.AppendUvarint(levels, uint64(end-i)<<1)
				if present {
					levels = append(levels, 1)
				} else {
					levels = append(levels, 0)
				}
				i = end
			}
			var prefix [4]byte
			binary.LittleEndian.PutUint32(prefix[:], uint32(len(levels)))
			page.Write(prefix[:])
			page.Write(levels)
		}
		encodePresent(f, v, from, to, &page)
		packed.Reset()
		gz, err := gzip.NewWriterLevel(&packed, gzip.DefaultCompression)
		if err != nil {
			return c, err
		}
		gz.Write(page.Bytes())
		if err := gz.Close(); err != nil {
			return c, err
		}
		var t thriftWriter
		t.begin()
		t.i32(1, pageData)
		t.i32(2, int32(page.Len()))
		t.i32(3, int32(packed.Len()))
		t.fieldStruct(5)
		t.i32(1, int32(to-from))
		t.i32(2, encPlain)
		t.i32(3, encRLE)
		t.i32(4, encRLE)
		t.end()
		t.end()
		if err := w.write(t.buf); err != nil {
			return c, err
		}
		if err := w.write(packed.Bytes()); err != nil {
			return c, err
		}
		c.compressed += int64(len(t.buf) + packed.Len())
		c.plain += int64(len(t.buf) + page.Len())
		if to == from {
			break
		}
		from = to
	}
	return c, nil
}

// Close writes the footer. It does not close the underlying writer.
func (w *Writer) Close() error {
	if w.err != nil {
		return w.err
	}
	if w.closed {
		return nil
	}
	w.closed = true
	var t thriftWriter
	t.begin()
	t.i32(1, 1)
	t.fieldList(2, tStruct, len(w.fields)+1)
	t.begin()
	t.str(4, "schema")
	t.i32(5, int32(len(w.fields)))
	t.end()
	for _, f := range w.fields {
		t.begin()
		t.i32(1, int32(f.Type))
		if f.Optional {
			t.i32(3, 1)
		} else {
			t.i32(3, 0)
		}
		t.str(4, f.Name)
		switch f.Annotation {
		case "string":
			t.i32(6, convertedUTF8)
			t.fieldStruct(10)
			t.fieldStruct(logicalString)
			t.end()
			t.end()
		case "uint", "uint8":
			bits := 32
			if f.Type == Int64 {
				bits = 64
			}
			if f.Annotation == "uint8" {
				bits = 8
			}
			t.i32(6, map[int]int32{8: convertedUint8, 32: convertedUint32, 64: convertedUint64}[bits])
			t.fieldStruct(10)
			t.fieldStruct(logicalInteger)
			t.byteField(1, int8(bits))
			t.boolField(2, false)
			t.end()
			t.end()
		}
		t.end()
	}
	t.i64(3, w.rows)
	t.fieldList(4, tStruct, len(w.groups))
	for _, g := range w.groups {
		t.begin()
		t.fieldList(1, tStruct, len(g.chunks))
		var total, compressed int64
		for i, c := range g.chunks {
			f := w.fields[i]
			t.begin()
			t.i64(2, c.offset)
			t.fieldStruct(3)
			t.i32(1, int32(f.Type))
			t.fieldList(2, tI32, 2)
			t.listI32(encPlain)
			t.listI32(encRLE)
			t.fieldList(3, tBinary, 1)
			t.listStr(f.Name)
			t.i32(4, int32(Gzip))
			t.i64(5, c.values)
			t.i64(6, c.plain)
			t.i64(7, c.compressed)
			t.i64(9, c.offset)
			t.end()
			t.end()
			total += c.plain
			compressed += c.compressed
		}
		t.i64(2, total)
		t.i64(3, g.rows)
		if len(g.chunks) > 0 {
			t.i64(5, g.chunks[0].offset)
		}
		t.i64(6, compressed)
		t.end()
	}
	sort.Slice(w.keyValue, func(i, j int) bool { return w.keyValue[i][0] < w.keyValue[j][0] })
	if len(w.keyValue) > 0 {
		t.fieldList(5, tStruct, len(w.keyValue))
		for _, kv := range w.keyValue {
			t.begin()
			t.str(1, kv[0])
			t.str(2, kv[1])
			t.end()
		}
	}
	if w.createdBy != "" {
		t.str(6, w.createdBy)
	}
	t.end()
	if len(t.buf) > 8<<20 {
		w.err = errors.New("parquet: footer exceeds 8 MiB")
		return w.err
	}
	if err := w.write(t.buf); err != nil {
		return err
	}
	var tail [8]byte
	binary.LittleEndian.PutUint32(tail[:4], uint32(len(t.buf)))
	copy(tail[4:], "PAR1")
	return w.write(tail[:])
}

// encodePresent writes only defined values, densely packing nullable booleans.
func encodePresent(f Field, v *Values, from, to int, out *bytes.Buffer) {
	if v.Valid == nil {
		encodePlain(f, v, from, to, out)
		return
	}
	if f.Type == Boolean {
		var b byte
		bit := uint(0)
		for i := from; i < to; i++ {
			if !v.Valid[i] {
				continue
			}
			if v.Bool[i] {
				b |= 1 << bit
			}
			bit++
			if bit == 8 {
				out.WriteByte(b)
				b = 0
				bit = 0
			}
		}
		if bit > 0 {
			out.WriteByte(b)
		}
		return
	}
	for i := from; i < to; i++ {
		if v.Valid[i] {
			encodePlain(f, v, i, i+1, out)
		}
	}
}

// thriftWriter emits the subset of Thrift's compact protocol needed by Parquet metadata.
type thriftWriter struct {
	buf  []byte
	last []int16
}

func (t *thriftWriter) uvarint(v uint64) { t.buf = binary.AppendUvarint(t.buf, v) }
func (t *thriftWriter) zigzag(v int64)   { t.uvarint(uint64(v<<1) ^ uint64(v>>63)) }

// begin opens a struct (a list's element, or the outermost); end writes its stop.
func (t *thriftWriter) begin() { t.last = append(t.last, 0) }
func (t *thriftWriter) end() {
	t.buf = append(t.buf, tStop)
	t.last = t.last[:len(t.last)-1]
}

func (t *thriftWriter) header(id int16, typ byte) {
	last := &t.last[len(t.last)-1]
	if d := id - *last; d > 0 && d <= 15 {
		t.buf = append(t.buf, byte(d)<<4|typ)
	} else {
		t.buf = append(t.buf, typ)
		t.zigzag(int64(id))
	}
	*last = id
}

func (t *thriftWriter) i32(id int16, v int32) {
	t.header(id, tI32)
	t.zigzag(int64(v))
}

func (t *thriftWriter) i64(id int16, v int64) {
	t.header(id, tI64)
	t.zigzag(v)
}

func (t *thriftWriter) byteField(id int16, v int8) {
	t.header(id, tByte)
	t.buf = append(t.buf, byte(v))
}

func (t *thriftWriter) boolField(id int16, v bool) {
	if v {
		t.header(id, tTrue)
	} else {
		t.header(id, tFalse)
	}
}

func (t *thriftWriter) str(id int16, s string) {
	t.header(id, tBinary)
	t.uvarint(uint64(len(s)))
	t.buf = append(t.buf, s...)
}

// fieldStruct opens a struct-valued field; end closes it.
func (t *thriftWriter) fieldStruct(id int16) {
	t.header(id, tStruct)
	t.begin()
}

// fieldList writes a list field's header; its n elements follow.
func (t *thriftWriter) fieldList(id int16, elem byte, n int) {
	t.header(id, tList)
	if n < 15 {
		t.buf = append(t.buf, byte(n)<<4|elem)
	} else {
		t.buf = append(t.buf, 0xf0|elem)
		t.uvarint(uint64(n))
	}
}

func (t *thriftWriter) listI32(v int32) { t.zigzag(int64(v)) }
func (t *thriftWriter) listStr(s string) {
	t.uvarint(uint64(len(s)))
	t.buf = append(t.buf, s...)
}
