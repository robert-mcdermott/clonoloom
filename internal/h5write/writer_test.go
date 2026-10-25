package h5write

import (
	"bytes"
	"clonoloom/internal/hdf5"
	"context"
	"math"
	"strings"
	"testing"
)

func TestWriterHeapBoundariesAndTypes(t *testing.T) {
	f := NewFile()
	values := make([]string, 8200)
	for i := range values {
		values[i] = "μ cell 🧬"
	}
	values[4095] = ""
	values[4096] = strings.Repeat("長", 30000)
	if err := f.Root.Strings("labels", values); err != nil {
		t.Fatal(err)
	}
	f.Root.AttrBool("ordered", false)
	f.Root.Float64s("coordinates", []uint64{2, 2}, []float64{-1.5, 2, math.NaN(), math.NaN()})
	f.Root.Bools("mask", []bool{false, true})
	f.Root.Int64s("counts", []int64{1<<53 - 1})
	f.Root.Strings("empty", []string{})
	raw := f.Bytes()
	if !bytes.Equal(raw, f.Bytes()) {
		t.Fatal("writer reused mutated heap addresses")
	}
	r, err := hdf5.Open(context.Background(), bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.Value("/labels", 10000)
	if err != nil {
		t.Fatal(err)
	}
	for i, v := range got.Text {
		if v != values[i] {
			t.Fatal("heap boundary", i)
		}
	}
	got, err = r.Value("/coordinates", 4)
	if err != nil || got.Numbers[0] != -1.5 || !math.IsNaN(got.Numbers[3]) {
		t.Fatal(got, err)
	}
	got, err = r.Value("/mask", 2)
	if err != nil || got.Kind != "boolean" || got.Numbers[1] != 1 {
		t.Fatal(got, err)
	}
	got, err = r.Value("/empty", 1)
	if err != nil || len(got.Text) != 0 {
		t.Fatal(got, err)
	}
	got, err = r.Value("/counts", 1)
	if err != nil || got.Text[0] != "9007199254740991" {
		t.Fatal(got, err)
	}
}
func TestWriterRejectsInvalidNamesAndText(t *testing.T) {
	for _, name := range []string{"", "..", ".", "x/y", "x\x00y", strings.Repeat("x", 256)} {
		if _, err := NewFile().Root.Group(name); err == nil {
			t.Fatal(name)
		}
	}
	for _, v := range []string{"\xff", "a\x00b"} {
		if err := NewFile().Root.Strings("x", []string{v}); err == nil {
			t.Fatal("invalid string")
		}
	}
	f := NewFile()
	f.Root.Int32s("x", nil)
	if err := f.Root.Strings("x", nil); err == nil {
		t.Fatal("duplicate dataset")
	}
	if err := f.Root.Float64s("y", []uint64{2, 3}, []float64{1}); err == nil {
		t.Fatal("wrong dimensions")
	}
}
