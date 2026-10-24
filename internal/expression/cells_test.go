package expression

import (
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const cellText = "barcode\tstate\tmap_x\tmap_y\nAA-1\tA\t0\t-2\nBB-1\t\t\t\nother\tB\t1e2\t3\n"

func TestCellAnnotations(t *testing.T) {
	m := &Matrix{Barcodes: []string{"AA-1", "BB-1", "CC-1"}}
	c, err := ReadCells(context.Background(), strings.NewReader(cellText), m)
	if err != nil {
		t.Fatal(err)
	}
	want := CellCoverage{Columns: []string{"state"}, Rows: 3, Matched: 2, AnnotationOnly: 1, WithoutAnnotation: 1, Mapped: 1, WithoutCoordinates: 2}
	if !reflect.DeepEqual(c.Coverage, want) || c.Rows[0].Position == nil || *c.Rows[0].Position != [2]float64{0, -2} || c.Rows[1].Position != nil || c.Rows[1].Values[0] != "" || c.Rows[2].Expression {
		t.Fatal(c)
	}
	var compressed bytes.Buffer
	z := gzip.NewWriter(&compressed)
	z.Write([]byte(strings.ReplaceAll(cellText, "\n", "\r\n")))
	z.Close()
	other, err := ReadCells(context.Background(), bytes.NewReader(compressed.Bytes()), m)
	if err != nil || !reflect.DeepEqual(c, other) {
		t.Fatal(other, err)
	}
	broken := append([]byte(nil), compressed.Bytes()...)
	broken[len(broken)-5] ^= 255
	if _, err := ReadCells(context.Background(), bytes.NewReader(broken), m); err == nil {
		t.Fatal("corrupt gzip accepted")
	}
	for _, text := range []string{"", "barcode\n", "cell\tstate\n", "barcode\tx\tx\n", "barcode\tmap_x\n", "barcode\tstate\na\tA\na\tB\n", "barcode\tstate\na\n", "barcode\tstate\n a\tA\n", "barcode\tstate\na\t A\n", "barcode\tstate\na\t\x00\n", "barcode\tstate\na\t\xff\n", "barcode\tmap_x\tmap_y\na\t1\t\n", "barcode\tmap_x\tmap_y\na\tNaN\t2\n", "barcode\tmap_x\tmap_y\na\tInf\t2\n", "barcode\tmap_x\tmap_y\na\t1e13\t2\n", "barcode\tstate\n" + strings.Repeat("x", maxLineBytes)} {
		if _, err := ReadCells(context.Background(), strings.NewReader(text), m); err == nil {
			t.Fatalf("accepted %q", text[:min(100, len(text))])
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ReadCells(ctx, strings.NewReader(cellText), m); err != context.Canceled {
		t.Fatal(err)
	}
	coordinates, err := ReadCells(context.Background(), strings.NewReader("barcode\tmap_x\tmap_y\nAA-1\t0\t0\n"), m)
	if err != nil || coordinates.Coverage.Columns == nil || coordinates.Coverage.Mapped != 1 {
		t.Fatal(coordinates, err)
	}
}

func TestCellSourceIdentity(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"matrix.mtx", "features.tsv", "barcodes.tsv"} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "expression-mex", name))
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()
	before, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "cells.tsv")
	os.WriteFile(path, []byte(cellText), 0600)
	plain, err := OpenWithCells(ctx, dir, "")
	if err != nil || plain.ID != before.ID {
		t.Fatal("implicit attachment", err)
	}
	attached, err := OpenWithCells(ctx, dir, path)
	if err != nil || attached.ID == plain.ID || len(attached.Sources) != 4 {
		t.Fatal(attached, err)
	}
	moved := filepath.Join(dir, "moved.tsv")
	os.WriteFile(moved, []byte(cellText), 0600)
	same, err := OpenWithCells(ctx, dir, moved)
	if err != nil || same.ID != attached.ID {
		t.Fatal("relocation changed identity", err)
	}
	os.WriteFile(path, []byte(strings.Replace(cellText, "\tA\t", "\tZ\t", 1)), 0600)
	changed, err := OpenWithCells(ctx, dir, path)
	if err != nil || changed.ID == attached.ID {
		t.Fatal("drift accepted", err)
	}
	if _, err = OpenWithCells(ctx, dir, dir); err == nil {
		t.Fatal("directory accepted")
	}
}

func FuzzCellAnnotations(f *testing.F) {
	f.Add([]byte(cellText))
	f.Add([]byte("barcode\tmap_x\tmap_y\n"))
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 1<<20 {
			t.Skip()
		}
		c, err := ReadCells(context.Background(), bytes.NewReader(raw), &Matrix{Barcodes: []string{"AA-1"}})
		if err == nil && c.Coverage.Matched+c.Coverage.AnnotationOnly != len(c.Rows) {
			t.Fatal("lost rows")
		}
	})
}
