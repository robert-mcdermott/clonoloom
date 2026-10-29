package parquet

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// A deterministic writer control with exact int64 extremes, nullable flags/text, two groups
// and split pages. PyArrow and DuckDB independently read the optional fixture in CI.
func TestWriterIndependentFixture(t *testing.T) {
	write := func() []byte {
		t.Helper()
		var out bytes.Buffer
		w, e := NewWriter(context.Background(), &out, []Field{{Name: "count", Type: Int64, Optional: true}, {Name: "flag", Type: Boolean, Optional: true}, {Name: "text", Type: ByteArray, Annotation: "string", Optional: true}, {Name: "value", Type: Double, Optional: true}}, "ClonoLoom writer control")
		if e != nil {
			t.Fatal(e)
		}
		w.PageSize = 16
		if e = w.SetKeyValue("z", "last"); e != nil {
			t.Fatal(e)
		}
		if e = w.SetKeyValue("a", "first"); e != nil {
			t.Fatal(e)
		}
		for repeat := 0; repeat < 2; repeat++ {
			if e = w.WriteRowGroup([]*Values{
				{Int64: []int64{0, 9007199254740993, math.MaxInt64, math.MinInt64, 0}, Valid: []bool{true, true, true, true, false}},
				{Bool: []bool{false, true, false, true, false}, Valid: []bool{true, true, false, true, true}},
				{Bytes: [][]byte{[]byte(""), []byte("αβ\t\n\"quoted\""), nil, []byte("HLA-A*02:01"), []byte("NaN")}, Valid: []bool{true, true, false, true, true}},
				{Float64: []float64{0, math.Copysign(0, -1), 0.125, 1, 0}, Valid: []bool{true, true, true, true, false}},
			}); e != nil {
				t.Fatal(e)
			}
		}
		if e = w.Close(); e != nil {
			t.Fatal(e)
		}
		if e = w.Close(); e != nil {
			t.Fatal(e)
		}
		return out.Bytes()
	}
	raw := write()
	if !bytes.Equal(raw, write()) {
		t.Fatal("nondeterministic bytes")
	}
	if string(raw[:4]) != "PAR1" || string(raw[len(raw)-4:]) != "PAR1" {
		t.Fatal("bad magic")
	}
	if dir := os.Getenv("CLONOLOOM_PARQUET_FIXTURES"); dir != "" {
		if e := os.MkdirAll(dir, 0700); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(filepath.Join(dir, "writer-control.parquet"), raw, 0600); e != nil {
			t.Fatal(e)
		}
		var empty bytes.Buffer
		w, e := NewWriter(context.Background(), &empty, []Field{{Name: "count", Type: Int64, Optional: true}}, "empty control")
		if e != nil {
			t.Fatal(e)
		}
		if e = w.Close(); e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(dir, "empty-control.parquet"), empty.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
	}
}

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }

type brokenWriter struct{ calls int }

func (w *brokenWriter) Write(p []byte) (int, error) {
	w.calls++
	if w.calls > 1 {
		return 0, errors.New("broken output")
	}
	return len(p), nil
}
func TestWriterBoundsAndFailures(t *testing.T) {
	for _, fields := range [][]Field{nil, {{Name: "x", Type: Int64}, {Name: "x", Type: Int64}}, {{Name: "x", Type: ByteArray, Annotation: "unknown"}}, {{Name: "x", Type: Int64, Annotation: "string"}}} {
		if _, e := NewWriter(context.Background(), io.Discard, fields, ""); e == nil {
			t.Fatal("invalid schema accepted")
		}
	}
	if _, e := NewWriter(context.Background(), shortWriter{}, []Field{{Name: "x", Type: Int64}}, ""); e != io.ErrShortWrite {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := NewWriter(ctx, io.Discard, []Field{{Name: "x", Type: Int64}}, ""); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	w, _ := NewWriter(context.Background(), io.Discard, []Field{{Name: "x", Type: Int64}}, "")
	for _, values := range [][]*Values{nil, {nil}, {{Int64: []int64{1}, Valid: []bool{false}}}, {{Int64: []int64{1}, Valid: []bool{true, false}}}} {
		if e := w.WriteRowGroup(values); e == nil {
			t.Fatal("invalid values")
		}
	}
	w.PageSize = 0
	if e := w.WriteRowGroup([]*Values{{Int64: []int64{1}}}); e == nil {
		t.Fatal("invalid page bound")
	}
	if e := w.SetKeyValue("k", string(bytes.Repeat([]byte{'x'}, 1<<20))); e == nil {
		t.Fatal("unbounded metadata")
	}
	broken := &brokenWriter{}
	w, _ = NewWriter(context.Background(), broken, []Field{{Name: "x", Type: Int64}}, "")
	if e := w.WriteRowGroup([]*Values{{Int64: []int64{1}}}); e == nil {
		t.Fatal("output failure lost")
	}
	if e := w.Close(); e == nil {
		t.Fatal("failed writer closed successfully")
	}
}

func TestWriterResourceBounds(t *testing.T) {
	newWriter := func(fields []Field) *Writer {
		t.Helper()
		w, e := NewWriter(context.Background(), io.Discard, fields, "bounded control")
		if e != nil {
			t.Fatal(e)
		}
		return w
	}
	field := []Field{{Name: "count", Type: Int64, Optional: true}}
	for _, bound := range []string{"rows", "groups", "chunks", "page", "output"} {
		t.Run(bound, func(t *testing.T) {
			w := newWriter(field)
			values := []*Values{{Int64: []int64{1}}}
			switch bound {
			case "rows":
				values[0].Int64 = make([]int64, 8193)
			case "groups":
				w.groups = make([]writtenGroup, 4096)
			case "chunks":
				w.chunks = 16384
			case "page":
				w.PageSize = (8 << 20) + 1
			case "output":
				w.pos = 256 << 20
			}
			if e := w.WriteRowGroup(values); e == nil {
				t.Fatal("resource bound accepted")
			}
		})
	}
	for _, value := range [][]byte{bytes.Repeat([]byte{'x'}, (1<<20)+1), {0xff}} {
		w := newWriter([]Field{{Name: "annotation", Type: ByteArray, Annotation: "string"}})
		if e := w.WriteRowGroup([]*Values{{Bytes: [][]byte{value}}}); e == nil {
			t.Fatal("unbounded or invalid UTF-8 cell accepted")
		}
	}
	// Individually valid cells still cannot form an unbounded row group.
	w := newWriter([]Field{{Name: "annotation", Type: ByteArray, Annotation: "string"}})
	values := make([][]byte, 33)
	for i := range values {
		values[i] = bytes.Repeat([]byte{'x'}, 1<<20)
	}
	if e := w.WriteRowGroup([]*Values{{Bytes: values}}); e == nil {
		t.Fatal("oversized row group accepted")
	}
}
