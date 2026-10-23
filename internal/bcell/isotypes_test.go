package bcell

import (
	"context"
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"

	"clonoloom/internal/rep"
	"clonoloom/internal/study"
)

func TestObservedIsotypeResolution(t *testing.T) {
	for _, test := range []struct {
		calls              []string
		class, gene, issue string
	}{
		{[]string{"IGHG1*01,IGHG1*02"}, "IgG", "IGHG1", ""},
		{[]string{"IGHM*01(1250)"}, "IgM", "IGHM", ""},
		{[]string{"IGHM*01/IGHD*01"}, "Unresolved", "Unresolved", "unsupported"},
		{[]string{"IGHM*bad"}, "Unresolved", "Unresolved", "unsupported"},
		{[]string{"IGHM*"}, "Unresolved", "Unresolved", "unsupported"},
		{[]string{"IGHM*01(IGHD)"}, "Unresolved", "Unresolved", "unsupported"},
		{[]string{"IGHM*01(Inf)"}, "Unresolved", "Unresolved", "unsupported"},
		{[]string{"IGHM*01(12)extra"}, "Unresolved", "Unresolved", "unsupported"},
		{[]string{"Ighg2b*01"}, "IgG", "IGHG2B", ""},
		{[]string{"IGHG1,IGHG2"}, "IgG", "Unresolved", "subclasses"},
		{[]string{"IGHG"}, "IgG", "Unresolved", "class-only"},
		{[]string{"IGHA"}, "IgA", "Unresolved", "class-only"},
		{[]string{"IGHM", "IGHD"}, "Unresolved", "Unresolved", "classes"},
		{[]string{"IGHM", "IGHM*01"}, "IgM", "IGHM", ""},
		{[]string{"IGHM", ""}, "Unresolved", "Unresolved", "missing"},
		{[]string{"IGHM,garbage"}, "Unresolved", "Unresolved", "unsupported"},
		{[]string{"IGHEP1"}, "Unresolved", "Unresolved", "unsupported"},
		{[]string{"IGKC"}, "Unresolved", "Unresolved", "unsupported"},
		{[]string{"IGHV1-1"}, "Unresolved", "Unresolved", "unsupported"},
		{[]string{strings.Repeat("X", 4097)}, "Unresolved", "Unresolved", "unsupported"},
		{nil, "Unresolved", "Unresolved", "missing"},
	} {
		c, g, why := observedIsotype(test.calls)
		if c != test.class || g != test.gene || !strings.Contains(why, test.issue) || (test.issue == "" && why != "") {
			t.Fatal(test, c, g, why)
		}
	}
}
func TestIsotypeCountingAndEvidence(t *testing.T) {
	var rows []rep.Row
	for i, c := range []string{"IGHM", "IGHD", "IGHG1", "IGHG1,IGHG2", "", "IGHEP1"} {
		r := familyRow(string(rune('a'+i)), "a", "", "IGH", "AAAAAAAAAAAA")
		r.C = c
		r.Count = float64(i)
		rows = append(rows, r)
	}
	rows[2].Count = math.NaN()
	excluded := familyRow("excluded", "a", "", "IGH", "AAAAAAAAAAAA")
	excluded.Productive = rep.False
	excluded.C = "IGHA1"
	rows = append(rows, excluded)
	tab := familyTable(t, rows)
	sources := []FamilySource{{Name: "test.tsv", SHA256: "hash"}}
	parent, err := Families(context.Background(), tab, nil, sources, map[string]string{"a": "one"}, FamilyOptions{Threshold: threshold(0)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ann := []study.Sample{{Name: "a", Time: "day 14", Tissue: "blood"}}
	result, err := Isotypes(context.Background(), tab, sources, parent, ann)
	if err != nil || result.Unit != "heavy-chain rows" || result.AbundanceUnit != parent.Unit || result.Assigned != 6 || result.ResolvedClass != 4 || result.ResolvedGene != 3 || len(result.Observations) != 7 || len(result.Families) != 1 || len(result.Pairs) != 3 {
		t.Fatal(result, err)
	}
	for _, p := range result.Pairs {
		if p.Families != 1 || p.TotalFamilies != 1 || p.EvaluableFamilies != 1 {
			t.Fatal(p)
		}
	}
	totals := map[string]int{}
	for _, u := range result.Usage {
		if u.Total != 6 || u.Share != float64(u.Observations)/6 || u.TotalAbundance != nil || u.AbundanceShare != nil || u.TotalMissingAbundance != 1 || u.Timepoint != "day 14" {
			t.Fatal(u)
		}
		totals[u.Family+u.Level] += u.Observations
	}
	for _, n := range totals {
		if n != 6 {
			t.Fatal(totals)
		}
	}
	// Permuting parent arrays must not change floating accumulation, tables or source order.
	before, _ := json.Marshal(result)
	for i, j := 0, len(parent.Observations)-1; i < j; i, j = i+1, j-1 {
		parent.Observations[i], parent.Observations[j] = parent.Observations[j], parent.Observations[i]
	}
	after, err := Isotypes(context.Background(), tab, sources, parent, ann)
	raw, _ := json.Marshal(after)
	if err != nil || string(before) != string(raw) {
		t.Fatal("order changed result", err)
	}
	parent.Observations[0].Rows[0].Source.SHA256 = "changed"
	if _, err := Isotypes(context.Background(), tab, sources, parent, ann); err == nil {
		t.Fatal("source mismatch accepted")
	}
}
func TestIsotypeCellsAndMissingVersusZero(t *testing.T) {
	// Repeated identical heavy contigs and one light chain still count as one cell.
	rows := []rep.Row{}
	for i, c := range []string{"IGHM", "IGHM*01"} {
		r := familyRow(string(rune('a'+i)), "a", "cell1", "IGH", "AAAAAAAAAAAA")
		r.C = c
		r.Count = 100
		rows = append(rows, r)
	}
	light := familyRow("light", "a", "cell1", "IGK", "CCCCCCCCCCCC")
	light.C = "IGKC"
	rows = append(rows, light)
	second := familyRow("second", "a", "cell2", "IGH", "AAAAAAAAAAAA")
	second.C = "IGHD"
	rows = append(rows, second)
	light.SequenceID = "light2"
	light.Cell = "cell2"
	rows = append(rows, light)
	tab := familyTable(t, rows)
	sources := []FamilySource{{Name: "test.tsv", SHA256: "hash"}}
	parent, err := Families(context.Background(), tab, nil, sources, map[string]string{"a": "one"}, FamilyOptions{Threshold: threshold(0)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Isotypes(context.Background(), tab, sources, parent, nil)
	if err != nil || result.Assigned != 2 || result.Unit != "cells" || result.Families[0].Evidence != "IgM/IgD only; not switching evidence" || len(result.Pairs) != 1 {
		t.Fatal(result, err)
	}
	for _, u := range result.Usage {
		if u.Total != 2 || u.TotalAbundance == nil || *u.TotalAbundance != 2 || u.AbundanceShare == nil || *u.AbundanceShare != .5 {
			t.Fatal(u)
		}
	}
	// Invalid cell identity cannot redirect a source row to another observation.
	copyParent := parent
	copyParent.Observations = append([]FamilyObservation{}, parent.Observations...)
	copyParent.Observations[0].Cell = "other"
	if _, err := Isotypes(context.Background(), tab, sources, copyParent, nil); err == nil {
		t.Fatal("cell mismatch")
	}
	// A known zero denominator keeps abundance 0 and share missing.
	row := familyRow("zero", "a", "", "IGH", "AAAAAAAAAAAA")
	row.C = "IGHG1"
	row.Count = 0
	zero := familyTable(t, []rep.Row{row})
	p, err := Families(context.Background(), zero, nil, sources, map[string]string{"a": "one"}, FamilyOptions{Threshold: threshold(0)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	r, err := Isotypes(context.Background(), zero, sources, p, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range r.Usage {
		if u.Abundance == nil || *u.Abundance != 0 || u.TotalAbundance == nil || *u.TotalAbundance != 0 || u.AbundanceShare != nil || u.Share != 1 {
			t.Fatal(u)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Isotypes(ctx, zero, sources, p, nil); err != context.Canceled {
		t.Fatal(err)
	}
	p.Options.Threshold = nil
	if _, err := Isotypes(context.Background(), zero, sources, p, nil); err == nil {
		t.Fatal("unthresholded parent")
	}
}
func TestIsotypeAbundanceBounds(t *testing.T) {
	for _, v := range []float64{-1, math.NaN(), math.Inf(1)} {
		a := isotypeAmount{}
		if a.add(&v) == nil {
			t.Fatal(v)
		}
	}
	a := isotypeAmount{}
	v := math.MaxFloat64
	if a.add(&v) != nil || a.add(&v) == nil {
		t.Fatal(a)
	}
}
func FuzzIsotypeCalls(f *testing.F) {
	f.Add("IGHG1*01,IGHG2*01", "IGHM")
	f.Add("IGHM*01/IGHD*01", "IGHM*01(1250)")
	f.Fuzz(func(t *testing.T, a, b string) {
		if len(a) > 10000 || len(b) > 10000 {
			return
		}
		c, g, why := observedIsotype([]string{a, b})
		d, h, other := observedIsotype([]string{b, a})
		if !reflect.DeepEqual([]string{c, g, why}, []string{d, h, other}) {
			t.Fatal("order dependent")
		}
		if g != unresolvedIsotype && isotypeClasses[g] != c {
			t.Fatal(c, g, why)
		}
	})
}

func TestIsotypeSubjectBoundariesAndEmptyEvidence(t *testing.T) {
	var rows []rep.Row
	for i, gene := range []string{"IGHM", "IGHG1", "IGHA1"} {
		sample := string(rune('a' + i))
		row := familyRow(sample, sample, "same-barcode", "IGH", "AAAAAAAAAAAA")
		row.C = gene
		rows = append(rows, row)
	}
	tab := familyTable(t, rows)
	sources := []FamilySource{{Name: "test.tsv", SHA256: "hash"}}
	for _, scope := range []string{"subject", "sample"} {
		parent, err := Families(context.Background(), tab, nil, sources, map[string]string{"a": "one", "b": "one", "c": "two"}, FamilyOptions{Mode: "heavy", Scope: scope, Threshold: threshold(0)}, nil)
		if err != nil {
			t.Fatal(err)
		}
		r, err := Isotypes(context.Background(), tab, sources, parent, nil)
		if err != nil || r.Assigned != 3 || len(r.Observations) != 3 {
			t.Fatal(err, r)
		}
		if scope == "subject" && (len(r.Families) != 2 || len(r.Pairs) != 1 || r.Pairs[0].Left != "IGHG1" || r.Pairs[0].Right != "IGHM" || r.Pairs[0].Families != 1 || r.Pairs[0].TotalFamilies != 2) {
			t.Fatal(r)
		}
		if scope == "sample" && (len(r.Families) != 3 || len(r.Pairs) != 0) {
			t.Fatal(r)
		}
		for _, u := range r.Usage {
			if u.Total != 1 || u.Share != 1 {
				t.Fatal("samples pooled", u)
			}
		}
	}
	// A family with entirely missing C calls remains visible with unresolved usage.
	rows = rows[:1]
	rows[0].C = ""
	tab = familyTable(t, rows)
	p, err := Families(context.Background(), tab, nil, sources, nil, FamilyOptions{Mode: "heavy", Scope: "sample", Threshold: threshold(0)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	r, err := Isotypes(context.Background(), tab, sources, p, nil)
	if err != nil || r.Assigned != 1 || r.ResolvedClass != 0 || len(r.Pairs) != 0 || r.Families[0].Evidence != "no resolved constant gene" {
		t.Fatal(err, r)
	}
	for _, u := range r.Usage {
		if u.State != "Unresolved" || u.Share != 1 {
			t.Fatal(u)
		}
	}
	rows[0].Productive = rep.False
	tab = familyTable(t, rows)
	p, err = Families(context.Background(), tab, nil, sources, nil, FamilyOptions{Mode: "heavy", Scope: "sample", Threshold: threshold(0)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	r, err = Isotypes(context.Background(), tab, sources, p, nil)
	if err != nil || r.Assigned != 0 || len(r.Usage) != 0 || len(r.Families) != 0 || len(r.Observations) != 1 || r.Observations[0].Excluded == "" {
		t.Fatal(err, r)
	}
}
