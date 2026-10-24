package simulate

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
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

func TestCellStatesParametersAndCancellation(t *testing.T) {
	s := Presets["cell-states"]
	for _, c := range []CellStateStudy{{0, 120, 12}, {21, 120, 12}, {2, 11, 12}, {2, 10001, 12}, {2, 120, 2}, {2, 120, 101}, {20, 10000, 12}} {
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
