package expression

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestHDF5MatchesMEXAndPreservesIdentity(t *testing.T) {
	ctx := context.Background()
	base := filepath.Join("..", "..", "testdata")
	mex, err := Open(ctx, filepath.Join(base, "expression-mex"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(base, "expression-hdf5", "checksum.h5")
	in, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in.Matrix, mex.Matrix) {
		t.Fatal("HDF5 differs from MEX")
	}
	if in.ID == mex.ID || len(in.Sources) != 1 || in.Sources[0].Role != "matrix-hdf5" {
		t.Fatal("source identities conflated", in)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(t.TempDir(), "moved.h5")
	if err = os.WriteFile(moved, raw, 0600); err != nil {
		t.Fatal(err)
	}
	again, err := Open(ctx, moved)
	if err != nil || again.ID != in.ID {
		t.Fatal("relocation changed identity", err)
	}
	cells := filepath.Join(t.TempDir(), "cells.tsv")
	if err = os.WriteFile(cells, []byte("barcode\tstate\tmap_x\tmap_y\nAA-1\tKnown\t1\t2\nOTHER\tOther\t3\t4\n"), 0600); err != nil {
		t.Fatal(err)
	}
	annotated, err := OpenWithCells(ctx, path, cells)
	if err != nil {
		t.Fatal(err)
	}
	if annotated.ID == in.ID || len(annotated.Sources) != 2 || annotated.Cells.Coverage.Mapped != 1 || annotated.Cells.Coverage.AnnotationOnly != 1 {
		t.Fatal(annotated)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = Read10xHDF5(cancelled, bytes.NewReader(raw), int64(len(raw))); err == nil {
		t.Fatal("ignored cancellation")
	}
}

func FuzzHDF5Expression(f *testing.F) {
	for _, name := range []string{"contiguous", "checksum", "v108", "userblock"} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "expression-hdf5", name+".h5"))
		if err != nil {
			f.Fatal(err)
		}
		f.Add(raw)
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			t.Skip()
		}
		m, err := Read10xHDF5(context.Background(), bytes.NewReader(b), int64(len(b)))
		if err == nil {
			if len(m.Indptr) != len(m.Barcodes)+1 || m.Indptr[len(m.Barcodes)] != len(m.Counts) {
				t.Fatal("accepted malformed matrix")
			}
		}
	})
}
