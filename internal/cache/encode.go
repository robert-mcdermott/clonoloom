package cache

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"math"
	"os"

	"clonoloom/internal/reads"
	"clonoloom/internal/rep"
)

// A cached table's file (.clt): the magic bytes, the length of its header, its header (JSON: the
// sources, problems, samples and unit, and where each column lies in the body), and its body: each
// column's values little-endian, one after another. formatVersion changes when the layout does
// (an entry of another version is a different key, never read).
var magic = []byte("CLOLTBL\x00")

const formatVersion = 2

type header struct {
	Format    string                     `json:"format"`
	Version   int                        `json:"version"`
	Key       string                     `json:"key"`
	Readers   int                        `json:"readers"`
	Rows      int                        `json:"rows"`
	Unit      rep.Unit                   `json:"unit"`
	TableUnit rep.Unit                   `json:"tableUnit"`
	Sources   []reads.Source             `json:"sources"`
	Native    map[int]*reads.NativeTable `json:"native,omitempty"`
	Problems  *reads.Problems            `json:"problems"`
	Samples   []rep.Sample               `json:"samples"`
	Columns   []column                   `json:"columns"`
	// CRC: the body's CRC-32 (Castagnoli).
	CRC uint32 `json:"crc"`
}

type column struct {
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Offset int    `json:"offset"`
	Length int    `json:"length"`
}

// slot is one column of the table, by the kind of its values.
type slot struct {
	name string
	u8   *[]uint8
	i8   *[]rep.Flag
	u16  *[]uint16
	u32  *[]uint32
	f64  *[]float64
}

func slots(t *rep.Table) []slot {
	out := []slot{
		{name: "sample", u32: &t.Sample}, {name: "locus", u8: &t.Locus},
		{name: "v", u32: &t.V}, {name: "d", u32: &t.D}, {name: "j", u32: &t.J}, {name: "c", u32: &t.C},
		{name: "junction", u32: &t.Junction}, {name: "junctionAA", u32: &t.JunctionAA},
		{name: "productive", i8: &t.Productive}, {name: "vjInFrame", i8: &t.VJInFrame}, {name: "stopCodon", i8: &t.StopCodon},
		{name: "count", f64: &t.Count}, {name: "reads", f64: &t.Reads}, {name: "umis", f64: &t.UMIs},
		{name: "cell", u32: &t.Cell}, {name: "clone", u32: &t.Clone}, {name: "sequenceID", u32: &t.SequenceID},
		{name: "isCell", i8: &t.IsCell}, {name: "highConfidence", i8: &t.HighConfidence}, {name: "fullLength", i8: &t.FullLength},
		{name: "source", u16: &t.Source}, {name: "line", u32: &t.Line},
	}
	for i, name := range rep.EvidenceFields {
		out = append(out, slot{name: "evidence." + name, u32: &t.Evidence[i]})
	}
	return out
}

// stores are the table's string stores, by name.
func stores(t *rep.Table) []struct {
	name string
	s    **rep.Strings
} {
	return []struct {
		name string
		s    **rep.Strings
	}{{"dict", &t.Dict}, {"junctions", &t.Junctions}, {"junctionsAA", &t.JunctionsAA}, {"ids", &t.IDs}, {"evidence", &t.EvidenceText}}
}

func encode(k string, res *reads.Result) ([]byte, error) {
	t := res.Table
	h := header{Format: "clonoloom-table-cache", Version: formatVersion, Key: k, Readers: reads.ReaderVersion, Rows: t.Len(), Unit: res.Unit, TableUnit: t.Unit,
		Sources: res.Sources, Problems: res.Problems, Samples: t.Samples}
	for i, source := range res.Sources {
		if source.Native != nil {
			if h.Native == nil {
				h.Native = map[int]*reads.NativeTable{}
			}
			h.Native[i] = source.Native
		}
	}
	var body []byte
	add := func(name, kind string, data []byte) {
		h.Columns = append(h.Columns, column{Name: name, Kind: kind, Offset: len(body), Length: len(data)})
		body = append(body, data...)
	}
	for _, s := range slots(t) {
		switch {
		case s.u8 != nil && *s.u8 != nil:
			add(s.name, "u8", *s.u8)
		case s.i8 != nil && *s.i8 != nil:
			b := make([]byte, len(*s.i8))
			for i, v := range *s.i8 {
				b[i] = byte(v)
			}
			add(s.name, "i8", b)
		case s.u16 != nil && *s.u16 != nil:
			b := make([]byte, 0, 2*len(*s.u16))
			for _, v := range *s.u16 {
				b = binary.LittleEndian.AppendUint16(b, v)
			}
			add(s.name, "u16", b)
		case s.u32 != nil && *s.u32 != nil:
			add(s.name, "u32", appendU32(nil, *s.u32))
		case s.f64 != nil && *s.f64 != nil:
			b := make([]byte, 0, 8*len(*s.f64))
			for _, v := range *s.f64 {
				b = binary.LittleEndian.AppendUint64(b, math.Float64bits(v))
			}
			add(s.name, "f64", b)
		}
	}
	for _, st := range stores(t) {
		if *st.s == nil {
			continue
		}
		data, ends := (*st.s).Raw()
		add(st.name+".text", "bytes", data)
		add(st.name+".ends", "u32", appendU32(nil, ends))
	}
	h.CRC = crc32.Checksum(body, crc32.MakeTable(crc32.Castagnoli))
	head, err := json.Marshal(h)
	if err != nil {
		return nil, err
	}
	out := make([]byte, 0, len(magic)+4+len(head)+len(body))
	out = append(out, magic...)
	out = binary.LittleEndian.AppendUint32(out, uint32(len(head)))
	out = append(out, head...)
	return append(out, body...), nil
}

func appendU32(b []byte, v []uint32) []byte {
	if b == nil {
		b = make([]byte, 0, 4*len(v))
	}
	for _, x := range v {
		b = binary.LittleEndian.AppendUint32(b, x)
	}
	return b
}

var errCorrupt = errors.New("a cached table that is not whole")

// load reads a cached table.
func load(path string) (*reads.Result, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) < len(magic)+4 || !bytes.Equal(data[:len(magic)], magic) {
		return nil, errCorrupt
	}
	n := int(binary.LittleEndian.Uint32(data[len(magic):]))
	start := len(magic) + 4
	if start+n > len(data) {
		return nil, errCorrupt
	}
	var h header
	if err := json.Unmarshal(data[start:start+n], &h); err != nil {
		return nil, err
	}
	body := data[start+n:]
	if h.Format != "clonoloom-table-cache" || h.Version != formatVersion || h.Readers != reads.ReaderVersion {
		return nil, errCorrupt
	}
	nativeBytes := 0
	for i, native := range h.Native {
		if i < 0 || i >= len(h.Sources) || native == nil || len(native.Rows) != h.Sources[i].Rows || len(native.Rows) > 1000000 {
			return nil, errCorrupt
		}
		nativeBytes += native.Size()
		if nativeBytes > 64<<20 {
			return nil, errCorrupt
		}
		h.Sources[i].Native = native
	}
	for _, source := range h.Sources {
		if (source.Format == reads.FormatTRUST4 || source.Format == reads.FormatVDJtools) && source.Native == nil {
			return nil, errCorrupt
		}
	}
	if crc32.Checksum(body, crc32.MakeTable(crc32.Castagnoli)) != h.CRC {
		return nil, errCorrupt
	}
	cols := map[string][]byte{}
	for _, c := range h.Columns {
		if c.Offset < 0 || c.Length < 0 || c.Offset > len(body) || c.Length > len(body)-c.Offset {
			return nil, errCorrupt
		}
		cols[c.Name] = body[c.Offset : c.Offset+c.Length]
	}
	t := &rep.Table{Samples: h.Samples, Unit: h.TableUnit}
	for _, s := range slots(t) {
		b, ok := cols[s.name]
		if !ok {
			continue
		}
		switch {
		case s.u8 != nil:
			*s.u8 = append([]uint8(nil), b...)
		case s.i8 != nil:
			v := make([]rep.Flag, len(b))
			for i, x := range b {
				v[i] = rep.Flag(int8(x))
			}
			*s.i8 = v
		case s.u16 != nil:
			if len(b)%2 != 0 {
				return nil, errCorrupt
			}
			v := make([]uint16, len(b)/2)
			for i := range v {
				v[i] = binary.LittleEndian.Uint16(b[2*i:])
			}
			*s.u16 = v
		case s.u32 != nil:
			v, err := readU32(b)
			if err != nil {
				return nil, err
			}
			*s.u32 = v
		case s.f64 != nil:
			if len(b)%8 != 0 {
				return nil, errCorrupt
			}
			v := make([]float64, len(b)/8)
			for i := range v {
				v[i] = math.Float64frombits(binary.LittleEndian.Uint64(b[8*i:]))
			}
			*s.f64 = v
		}
	}
	for _, st := range stores(t) {
		text, ok := cols[st.name+".text"]
		if !ok {
			continue
		}
		ends, err := readU32(cols[st.name+".ends"])
		if err != nil {
			return nil, err
		}
		s, err := rep.StringsFrom(append([]byte(nil), text...), ends)
		if err != nil {
			return nil, err
		}
		*st.s = s
	}
	if t.Len() != h.Rows {
		return nil, fmt.Errorf("%w: %d rows, %d recorded", errCorrupt, t.Len(), h.Rows)
	}
	// Every column that has rows has them all.
	for _, s := range slots(t) {
		l := -1
		switch {
		case s.u8 != nil && *s.u8 != nil:
			l = len(*s.u8)
		case s.i8 != nil && *s.i8 != nil:
			l = len(*s.i8)
		case s.u16 != nil && *s.u16 != nil:
			l = len(*s.u16)
		case s.u32 != nil && *s.u32 != nil:
			l = len(*s.u32)
		case s.f64 != nil && *s.f64 != nil:
			l = len(*s.f64)
		}
		if l >= 0 && l != h.Rows {
			return nil, fmt.Errorf("%w: column %s", errCorrupt, s.name)
		}
	}
	for _, column := range t.Evidence {
		for _, code := range column {
			if code != 0 && (t.EvidenceText == nil || uint64(code) > uint64(t.EvidenceText.Len())) {
				return nil, errCorrupt
			}
		}
	}
	problems := h.Problems
	if problems == nil {
		problems = &reads.Problems{}
		json.Unmarshal([]byte("{}"), problems)
	}
	return &reads.Result{Table: t, Sources: h.Sources, Problems: problems, Unit: h.Unit}, nil
}

func readU32(b []byte) ([]uint32, error) {
	if len(b)%4 != 0 {
		return nil, errCorrupt
	}
	v := make([]uint32, len(b)/4)
	for i := range v {
		v[i] = binary.LittleEndian.Uint32(b[4*i:])
	}
	return v, nil
}
