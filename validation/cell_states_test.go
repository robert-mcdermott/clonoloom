package validation

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"clonoloom/internal/expression"
	"clonoloom/internal/reads"
	"clonoloom/internal/simulate"
)

func TestCellStatesAgainstIndependentTruth(t *testing.T) {
	checkCellStatesTruth(t, "cell-states", "cell_states.json")
}

func TestCellStateCloudsAgainstIndependentTruth(t *testing.T) {
	checkCellStatesTruth(t, "cell-states-clouds", "cell_states_clouds.json")
}

func checkCellStatesTruth(t *testing.T, preset, file string) {
	t.Helper()
	var ref struct {
		Sources map[string]string
		Samples []struct {
			Name, DenseSHA256                     string
			GeneUMIs                              []uint64
			Genes                                 []int
			Matched, ExpressionOnly, ReceptorOnly int
			Annotations                           expression.CellCoverage
		}
	}
	raw, err := os.ReadFile(filepath.Join("reference", file))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &ref); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "study")
	if _, err = simulate.Run(context.Background(), simulate.Presets[preset], dir, "validation", nil); err != nil {
		t.Fatal(err)
	}
	for name, digest := range ref.Sources {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || fmt.Sprintf("%x", sha256.Sum256(raw)) != digest {
			t.Fatal("source differs", name, err)
		}
	}
	paths, _ := filepath.Glob(filepath.Join(dir, "*.tsv"))
	data, err := reads.Open(context.Background(), paths)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range ref.Samples {
		folder := filepath.Join(dir, "expression", want.Name)
		in, err := expression.OpenWithCells(context.Background(), folder, filepath.Join(folder, "cells.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		m := in.Matrix
		if !reflect.DeepEqual(m.GeneUMIs, want.GeneUMIs) || !reflect.DeepEqual(m.Genes, want.Genes) || !reflect.DeepEqual(in.Cells.Coverage, want.Annotations) {
			t.Fatal("independent measurements differ", want.Name)
		}
		dense := make([]uint64, len(m.Features)*len(m.Barcodes))
		for c := range m.Barcodes {
			for at := m.Indptr[c]; at < m.Indptr[c+1]; at++ {
				dense[int(m.Indices[at])*len(m.Barcodes)+c] = m.Counts[at]
			}
		}
		h := sha256.New()
		for _, v := range dense {
			var raw [8]byte
			binary.LittleEndian.PutUint64(raw[:], v)
			h.Write(raw[:])
		}
		if fmt.Sprintf("%x", h.Sum(nil)) != want.DenseSHA256 {
			t.Fatal("SciPy values differ", want.Name)
		}
		sample := -1
		for i, s := range data.Table.Samples {
			if s.Name == want.Name {
				sample = i
			}
		}
		links, err := expression.Link(context.Background(), m, data.Table, sample)
		if err != nil || links.Matched != want.Matched || links.ExpressionOnly != want.ExpressionOnly || links.ReceptorOnly != want.ReceptorOnly {
			t.Fatal(links, err)
		}
	}
	t.Log(preset, "all four matrices, gene totals and exact source coverage agree")
}
