package simulate

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"clonoloom/internal/airr"
	"clonoloom/internal/expression"
	"clonoloom/internal/reads"
)

func TestCellStatesReproducibilityAndCoverage(t *testing.T) {
	for _, column := range airr.Required {
		if !slices.Contains(cellStateColumns, column) {
			t.Fatal("missing AIRR column", column)
		}
	}
	s := Presets["cell-states"]
	a, b := filepath.Join(t.TempDir(), "a"), filepath.Join(t.TempDir(), "b")
	first, err := Run(context.Background(), s, a, "validation", nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Run(context.Background(), s, b, "validation", nil)
	if err != nil {
		t.Fatal(err)
	}
	if first.Subjects != 2 || first.Samples != 4 || first.Rows != 960 || !reflect.DeepEqual(first.Files, second.Files) {
		t.Fatal(first, second)
	}
	for _, name := range first.Files {
		raw, _ := os.ReadFile(filepath.Join(a, name))
		other, _ := os.ReadFile(filepath.Join(b, name))
		if string(raw) != string(other) {
			t.Fatal("nondeterministic", name)
		}
	}
	var truth cellStateTruth
	raw, _ := os.ReadFile(filepath.Join(a, "truth", "truth.json"))
	if err = json.Unmarshal(raw, &truth); err != nil {
		t.Fatal(err)
	}
	for _, file := range truth.Files {
		raw, err := os.ReadFile(filepath.Join(a, file.Path))
		if err != nil || fmt.Sprintf("%x", sha256.Sum256(raw)) != file.SHA256 {
			t.Fatal(file, err)
		}
	}
	paths, _ := filepath.Glob(filepath.Join(a, "*.tsv"))
	receptors, err := reads.Open(context.Background(), paths)
	if err != nil {
		t.Fatal(err)
	}
	if len(receptors.Table.Samples) != 4 {
		t.Fatal("sample boundaries lost")
	}
	for sample, info := range receptors.Table.Samples {
		dir := filepath.Join(a, "expression", info.Name)
		in, err := expression.OpenWithCells(context.Background(), dir, filepath.Join(dir, "cells.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		link, err := expression.Link(context.Background(), in.Matrix, receptors.Table, sample)
		if err != nil || link.Matched != 119 || link.ExpressionOnly != 1 || link.ReceptorOnly != 1 || link.ReceptorCells != 120 {
			t.Fatal(link, err)
		}
		if in.Matrix.GeneUMIs[119] != 0 || in.Cells.Coverage.Mapped != 118 || in.Cells.Coverage.AnnotationOnly != 1 || in.Cells.Rows[2].Values[0] != "" {
			t.Fatal(in.Cells.Coverage)
		}
	}
	s.Seed++
	c := filepath.Join(t.TempDir(), "changed")
	if _, err := Run(context.Background(), s, c, "validation", nil); err != nil {
		t.Fatal(err)
	}
	x, _ := os.ReadFile(filepath.Join(a, "S01-1.tsv"))
	y, _ := os.ReadFile(filepath.Join(c, "S01-1.tsv"))
	if string(x) == string(y) {
		t.Fatal("seed ignored")
	}
	if _, err := Run(context.Background(), s, a, "validation", nil); err == nil {
		t.Fatal("overwrote study")
	}
}

func TestCellStateCloudLayoutPreservesBiologicalEvidence(t *testing.T) {
	s := Presets["cell-states-clouds"]
	dir := filepath.Join(t.TempDir(), "clouds")
	first, err := Run(context.Background(), s, dir, "validation", nil)
	if err != nil {
		t.Fatal(err)
	}
	copyDir := filepath.Join(t.TempDir(), "same")
	if _, err := Run(context.Background(), s, copyDir, "validation", nil); err != nil {
		t.Fatal(err)
	}
	for _, name := range first.Files {
		a, _ := os.ReadFile(filepath.Join(dir, name))
		b, _ := os.ReadFile(filepath.Join(copyDir, name))
		if string(a) != string(b) {
			t.Fatal("clouds not reproducible", name)
		}
	}
	grid := *s.CellStates
	grid.Layout = ""
	s.CellStates = &grid
	gridDir := filepath.Join(t.TempDir(), "grid")
	if _, err := Run(context.Background(), s, gridDir, "validation", nil); err != nil {
		t.Fatal(err)
	}
	for _, sample := range []string{"S01-1", "S01-2", "S02-1", "S02-2"} {
		for _, name := range []string{sample + ".tsv", "expression/" + sample + "/matrix.mtx", "expression/" + sample + "/features.tsv", "expression/" + sample + "/barcodes.tsv"} {
			a, _ := os.ReadFile(filepath.Join(dir, name))
			b, _ := os.ReadFile(filepath.Join(gridDir, name))
			if string(a) != string(b) {
				t.Fatal("layout changed biological evidence", name)
			}
		}
		path := filepath.Join(dir, "expression", sample)
		in, err := expression.OpenWithCells(context.Background(), path, filepath.Join(path, "cells.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		if in.Cells.Coverage.Mapped != 358 {
			t.Fatal(in.Cells.Coverage)
		}
		seen := map[[2]float64]bool{}
		for _, row := range in.Cells.Rows {
			if row.Expression && row.Position != nil {
				p := *row.Position
				if math.IsNaN(p[0]) || math.IsNaN(p[1]) || p[0] == math.Trunc(p[0]) || seen[p] {
					t.Fatal("invalid or quantized cloud", p)
				}
				seen[p] = true
			}
		}
	}
}

func TestCellStatesParametersAndCancellation(t *testing.T) {
	s := Presets["cell-states"]
	for _, c := range []CellStateStudy{{0, 120, 12, ""}, {21, 120, 12, ""}, {2, 11, 12, ""}, {2, 10001, 12, ""}, {2, 120, 2, ""}, {2, 120, 101, ""}, {20, 10000, 12, ""}, {2, 120, 12, "unknown"}} {
		s.CellStates = &c
		if s.Check() == nil {
			t.Fatal(c)
		}
	}
	s = Presets["cell-states"]
	s.BCells = Presets["bcell-immunization"].BCells
	if s.Check() == nil {
		t.Fatal("mixed models accepted")
	}
	s = Presets["cell-states"]
	dir := filepath.Join(t.TempDir(), "cancelled")
	ctx, cancel := context.WithCancel(context.Background())
	if _, err := Run(ctx, s, dir, "validation", func(float64, string) { cancel() }); err != context.Canceled {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("partial output installed", err)
	}
}

func TestPseudobulkStudyIdentity(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "pseudobulk")
	sum, e := Run(context.Background(), Presets["pseudobulk-study"], dir, "validation", nil)
	if e != nil {
		t.Fatal(e)
	}
	if sum.Subjects != 4 || sum.Samples != 8 || sum.Rows != 6720 {
		t.Fatal(sum)
	}
	h := sha256.New()
	for _, name := range sum.Files {
		raw, e := os.ReadFile(filepath.Join(dir, name))
		if e != nil {
			t.Fatal(e)
		}
		fmt.Fprintln(h, name)
		h.Write(raw)
	}
	digest := fmt.Sprintf("%x", h.Sum(nil))
	if digest != "6ac6339c8dcd9b468a34dd746de193410625379170adb819d98c45bbd6173875" {
		t.Fatal("pseudobulk simulation digest", digest)
	}
	var truth cellStateTruth
	raw, _ := os.ReadFile(filepath.Join(dir, "truth/truth.json"))
	if e = json.Unmarshal(raw, &truth); e != nil {
		t.Fatal(e)
	}
	if truth.Model != "clonoloom-artificial-pseudobulk-v1" || truth.CloneSampling == "" {
		t.Fatal("model not explicit", truth)
	}
	for _, f := range truth.Files {
		raw, e := os.ReadFile(filepath.Join(dir, f.Path))
		if e != nil || fmt.Sprintf("%x", sha256.Sum256(raw)) != f.SHA256 {
			t.Fatal("source hash", f, e)
		}
	}
	if _, e = Run(context.Background(), Presets["pseudobulk-study"], dir, "validation", nil); e == nil {
		t.Fatal("overwrote existing study")
	}
}
