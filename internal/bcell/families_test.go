package bcell

import (
	"clonoloom/internal/rep"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
)

func familyRow(id, sample, cell, locus, junction string) rep.Row {
	r := rep.NewRow()
	r.SequenceID = id
	r.Sample = sample
	r.Cell = cell
	r.Locus = locus
	r.Junction = junction
	r.V = locus + "V1-1*01"
	r.J = locus + "J1*01"
	r.Productive = rep.True
	r.Count = 2
	return r
}
func familyTable(t *testing.T, rows []rep.Row) *rep.Table {
	t.Helper()
	b := rep.NewBuilder(rep.Reads)
	for i, r := range rows {
		if r.Line == 0 {
			r.Line = uint32(i + 2)
		}
		if err := b.Append(&r); err != nil {
			t.Fatal(err)
		}
	}
	return b.Table()
}
func runFamilies(t *testing.T, rows []rep.Row, o FamilyOptions) FamilyResult {
	t.Helper()
	r, e := Families(context.Background(), familyTable(t, rows), nil, []FamilySource{{Name: "test.tsv", SHA256: "sha"}}, map[string]string{"a": "one", "b": "one", "c": "two"}, o, nil)
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func threshold(v float64) *float64 { return &v }
func TestFamiliesSingleLinkageAndScopes(t *testing.T) {
	rows := []rep.Row{familyRow("a", "a", "", "IGH", "AAAAAAAAAAAA"), familyRow("b", "a", "", "IGH", "CAAAAAAAAAAA"), familyRow("c", "b", "", "IGH", "CCAAAAAAAAAA"), familyRow("dup", "b", "", "IGH", "AAAAAAAAAAAA"), familyRow("other", "c", "", "IGH", "AAAAAAAAAAAA"), familyRow("missing", "missing", "", "IGH", "AAAAAAAAAAAA")}
	r := runFamilies(t, rows, FamilyOptions{Threshold: threshold(1.0 / 12)})
	if len(r.Families) != 2 || len(r.Representatives) != 4 || r.Eligible != 5 || r.WithoutNeighbor != 1 || r.Excluded["sample has no subject"] != 1 {
		t.Fatalf("%+v", r)
	}
	for _, f := range r.Families {
		if f.Scope == "subject:one" && (f.Representatives != 3 || f.Observations != 4 || len(f.Samples) != 2) {
			t.Fatal(f)
		}
	}
	if r.Comparisons != 3 {
		t.Fatal(r.Comparisons)
	}
	for _, r := range r.Representatives {
		if r.Scope == "subject:one" && (r.Nearest == nil || *r.Nearest != 1.0/12) {
			t.Fatal(r)
		}
	}
	if len(runFamilies(t, rows, FamilyOptions{Threshold: threshold(0)}).Families) != 4 {
		t.Fatal("zero cutoff is exact identity")
	}
	r = runFamilies(t, rows, FamilyOptions{})
	if len(r.Families) != 0 || r.Representatives[0].Family != "" {
		t.Fatal("unset threshold assigned families")
	}
	r = runFamilies(t, rows, FamilyOptions{Scope: "sample", Threshold: threshold(1.0 / 12)})
	if r.Eligible != 6 || len(r.Families) != 5 {
		t.Fatal("sample scope must isolate samples", r.Families)
	}
}
func TestFamiliesPairedAndAmbiguous(t *testing.T) {
	rows := []rep.Row{}
	for i, l := range []string{"AAAAAAAAAAAA", "CAAAAAAAAAAA", "CCCCAAAAAAAA"} {
		c := fmt.Sprint(i)
		rows = append(rows, familyRow(c+"h", "a", c, "IGH", "AAAAAAAAAAAA"), familyRow(c+"l", "a", c, "IGK", l))
	}
	rows = append(rows, rows[0]) // repeated identical chain is not a second receptor
	r := runFamilies(t, rows, FamilyOptions{Threshold: threshold(1.0 / 12)})
	if r.Options.Mode != "paired" || r.Unit != "cells" || r.Eligible != 3 || len(r.Families) != 2 {
		t.Fatal(r)
	}
	for _, o := range r.Observations {
		if *o.Abundance != 1 {
			t.Fatal(o)
		}
	}
	r = runFamilies(t, rows, FamilyOptions{Mode: "heavy", Threshold: threshold(0)})
	if r.Eligible != 3 || len(r.Families) != 1 || len(r.Representatives) != 1 || r.WithoutNeighbor != 1 {
		t.Fatal(r)
	}
	rows = append(rows, familyRow("dual", "a", "0", "IGL", "AAAAAAAAAAAA"), familyRow("missing-light", "b", "1", "IGH", "AAAAAAAAAAAA"))
	r = runFamilies(t, rows, FamilyOptions{Threshold: threshold(1)})
	if r.Eligible != 2 || r.Excluded["requires one distinct productive light chain"] != 2 {
		t.Fatal(r)
	}
	// Same barcode in another sample is a separate observation, not a chimeric cell.
	rows = append(rows, familyRow("other-light", "b", "1", "IGK", "AAAAAAAAAAAA"))
	r = runFamilies(t, rows, FamilyOptions{Threshold: threshold(1)})
	if r.Eligible != 3 {
		t.Fatal(r)
	}
}
func TestFamiliesEligibilityMissingAndStability(t *testing.T) {
	base := familyRow("good", "a", "", "IGH", "AAAAAAAAAAAA")
	rows := []rep.Row{base}
	for i := 0; i < 7; i++ {
		r := base
		r.SequenceID = fmt.Sprint(i)
		switch i {
		case 0:
			r.V = "IGHV1-1*01,IGHV1-1*02"
			r.Junction = "aaaaaaaaaaaa"
			r.Count = 0
		case 1:
			r.V = "IGHV1-1*01,IGHV2-1*01"
		case 2:
			r.Junction = "AAAAANAAAAAA"
		case 3:
			r.Productive = rep.Missing
		case 4:
			r.J = "IGKJ1*01"
		case 5:
			r.Junction = "AAAA"
		case 6:
			r.Count = math.NaN()
		}
		rows = append(rows, r)
	}
	for i := range rows {
		rows[i].Line = uint32(i + 2)
	}
	r := runFamilies(t, rows, FamilyOptions{Threshold: threshold(0)})
	if r.Eligible != 3 || len(r.Families) != 1 || r.Families[0].Samples[0].Abundance != nil {
		t.Fatal(r)
	}
	first, _ := json.Marshal(r)
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	second, _ := json.Marshal(runFamilies(t, rows, FamilyOptions{Threshold: threshold(0)}))
	if string(first) != string(second) {
		t.Fatal("row order changed result")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := Families(ctx, familyTable(t, rows), nil, nil, nil, FamilyOptions{}, nil); e != context.Canceled {
		t.Fatal(e)
	}
	for _, o := range []FamilyOptions{{Mode: "bad"}, {Mode: "paired"}, {Scope: "bad"}, {Threshold: threshold(-1)}, {Threshold: threshold(math.NaN())}, {Threshold: threshold(1.1)}} {
		if _, e := o.Normalize(false); e == nil {
			t.Fatal(o)
		}
	}
}
func TestFamilyWorkLimitAndFilter(t *testing.T) {
	rows := make([]rep.Row, 4000)
	for i := range rows {
		b := []byte(strings.Repeat("A", 30))
		n := i
		for j := 0; j < 6; j++ {
			b[j] = "ACGT"[n%4]
			n /= 4
		}
		rows[i] = familyRow(fmt.Sprint(i), "a", "", "IGH", string(b))
	}
	table := familyTable(t, rows)
	sources := []FamilySource{{Name: "test"}}
	if _, e := Families(context.Background(), table, nil, sources, nil, FamilyOptions{Scope: "sample"}, nil); e == nil || !strings.Contains(e.Error(), "work limit") {
		t.Fatal(e)
	}
	keep := make([]bool, len(rows))
	keep[0] = true
	r, e := Families(context.Background(), table, keep, sources, nil, FamilyOptions{Scope: "sample"}, nil)
	if e != nil || r.Eligible != 1 || r.FilteredRows != 3999 {
		t.Fatal(r, e)
	}
}

func TestFamilyPartitionsAndHistogram(t *testing.T) {
	rows := []rep.Row{familyRow("a", "a", "", "IGH", "AAAAAAAAAAAA"), familyRow("b", "a", "", "IGH", "TTTTTTTTTTTT"), familyRow("c", "a", "", "IGH", "AAAAAAAAA"), familyRow("d", "a", "", "IGH", "AAAAAAAAAAAA"), familyRow("e", "a", "", "IGH", "AAAAAAAAAAAA")}
	rows[3].V = "IGHV2-1*01"
	rows[4].J = "IGHJ2*01"
	r := runFamilies(t, rows, FamilyOptions{Threshold: threshold(1)})
	if len(r.Families) != 4 || r.WithoutNeighbor != 3 || r.Histogram[19].Count != 2 || r.Comparisons != 1 {
		t.Fatal(r)
	}
}

func FuzzFamilyDistance(f *testing.F) {
	f.Add([]byte{0, 1, 2}, []byte{1, 2, 3})
	f.Fuzz(func(t *testing.T, a, b []byte) {
		n := min(len(a), len(b), 999)
		if n == 0 {
			return
		}
		x, y := make([]byte, n), make([]byte, n)
		mismatches := 0
		for i := range x {
			x[i] = "ACGT"[a[i]%4]
			y[i] = "ACGT"[b[i]%4]
			if x[i] != y[i] {
				mismatches++
			}
		}
		d := normalizedHamming(string(x), string(y))
		if d < 0 || d > 1 || d != normalizedHamming(string(y), string(x)) || d != float64(mismatches)/float64(n) {
			t.Fatal(d)
		}
	})
}
