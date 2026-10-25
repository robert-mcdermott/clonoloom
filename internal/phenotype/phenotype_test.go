package phenotype

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"clonoloom/internal/clonotype"
	"clonoloom/internal/qc"
	"clonoloom/internal/rep"
)

func fixture(t *testing.T) (*rep.Table, []Cell) {
	t.Helper()
	b := rep.NewBuilder(rep.Cells)
	serial := 0
	add := func(sample, cell, locus, aa, clone string, productive rep.Flag) {
		r := rep.NewRow()
		r.Sample = sample
		r.Cell = cell
		r.Locus = locus
		r.JunctionAA = aa
		r.Clone = clone
		r.Productive = productive
		r.Count = 1
		r.UMIs = 1
		r.SequenceID = fmt.Sprint(serial)
		serial++
		if err := b.Append(&r); err != nil {
			t.Fatal(err)
		}
	}
	// Three distinct S1 cells in one clone, including a receptor-only cell. Duplicate contigs
	// must not inflate size. S2 repeats the same barcode and junction but is a separate population.
	for _, c := range []string{"a", "b", "receptor-only"} {
		add("S1", c, "TRA", "CAAF", "v1", rep.True)
		add("S1", c, "TRB", "CBBF", "v1", rep.True)
	}
	add("S1", "a", "TRB", "CBBF", "v1", rep.True)
	add("S2", "a", "TRA", "CAAF", "v1", rep.True)
	add("S2", "a", "TRB", "CBBF", "v1", rep.True)
	add("S1", "excluded", "TRB", "CCCF", "v2", rep.False)
	add("S1", "unassigned", "TRB", "", "", rep.True)
	add("S1", "single", "TRB", "CDDF", "v3", rep.True)
	add("S1", "conflict", "TRB", "CEEF", "v4", rep.True)
	add("S1", "conflict", "TRA", "CFFF", "v5", rep.True)
	for i := 0; i < 3; i++ {
		add("S1", "multi", "TRB", "CGGF", "v6", rep.True)
	}
	add("S1", "mixed", "TRB", "CHHF", "v7", rep.True)
	add("S1", "mixed", "IGH", "CJJF", "v7", rep.True)
	add("S1", "unknown", "", "CKKF", "v8", rep.True)
	add("S1", "", "TRB", "CLLF", "v9", rep.True)
	var cells []Cell
	for i, c := range []string{"a", "b", "excluded", "unassigned", "single", "conflict", "multi", "mixed", "unknown", "none"} {
		v := "State A"
		if i%2 == 1 {
			v = ""
		}
		var p *[2]float64
		if c != "b" && c != "single" {
			p = &[2]float64{float64(i), 1}
		}
		cells = append(cells, Cell{Barcode: c, Values: []string{v}, Position: p})
	}
	return b.Table(), cells
}
func compute(t *testing.T, tab *rep.Table, cells []Cell, d clonotype.Definition, f qc.Filter, o Options) *Result {
	t.Helper()
	keep, _ := qc.Apply(tab, f)
	ct, err := clonotype.Compute(tab, keep, d)
	if err != nil {
		t.Fatal(err)
	}
	r, err := Compute(context.Background(), tab, ct, f, 0, cells, []string{"state"}, o)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func TestDistinctCellsPopulationsAndMissingness(t *testing.T) {
	tab, cells := fixture(t)
	d := clonotype.Definition{Chains: []string{"TRA", "TRB"}}
	f := qc.Filter{ProductiveOnly: true}
	r := compute(t, tab, cells, d, f, Options{Annotation: "state"})
	c := r.Cells
	if *c[0].Size != 3 || *c[1].Size != 3 || c[0].Clone != c[1].Clone || c[0].Bin != "3–5" {
		t.Fatal(c[:2])
	}
	if c[2].Status != "QC excluded" || c[3].Status != "Unassigned clonotype" || c[9].Status != "No receptor evidence" {
		t.Fatal(c)
	}
	if c[0].Chains != "Paired" || c[4].Chains != "Single arm" || c[6].Chains != "Multi-chain (>2 per locus)" || c[7].Chains != "Mixed receptors" || c[8].Chains != "Unresolved locus" {
		t.Fatal(c)
	}
	if r.Coverage.Receptor != 10 || r.Coverage.Matched != 9 || r.Coverage.ExpressionOnly != 1 || r.Coverage.ReceptorOnly != 1 || r.Coverage.MissingBarcodeRows != 1 || r.Coverage.Mapped != 8 {
		t.Fatal(r.Coverage)
	}
	mapped := compute(t, tab, cells, d, f, Options{Population: "mapped", Annotation: "state"})
	if *mapped.Cells[0].Size != 1 || *mapped.Cells[1].Size != 1 || mapped.Cells[4].Size != nil || mapped.Cells[4].Status != "Outside size population" {
		t.Fatal(mapped.Cells)
	}
	for _, a := range r.Associations {
		if a.Clone == c[0].Clone {
			if a.Cells != 1 || a.CloneExpression != 2 || a.GroupExpression != 5 || a.CloneShare != .5 || a.GroupShare != .2 {
				t.Fatal(a)
			}
		}
	}
	total := 0
	for _, g := range r.Groups {
		total += g.Cells
		if g.Cells != 5 {
			t.Fatal(g)
		}
	}
	if total != 10 {
		t.Fatal(r.Groups)
	}
	// Missing labels are distinct from a literal label saying Missing.
	cells[0].Values = []string{"Missing"}
	r = compute(t, tab, cells, d, f, Options{Annotation: "state"})
	if len(r.Groups) != 3 {
		t.Fatal(r.Groups)
	}
}
func TestAmbiguousDefinitionsAndQC(t *testing.T) {
	tab, cells := fixture(t)
	all := compute(t, tab, cells, clonotype.Definition{}, qc.Filter{}, Options{})
	if all.Cells[0].Status != "Ambiguous clonotype" || all.Cells[0].Size != nil || len(all.Cells[0].Candidates) != 2 {
		t.Fatal(all.Cells[0])
	}
	vendor := compute(t, tab, cells, clonotype.Definition{Vendor: true}, qc.Filter{}, Options{})
	if vendor.Cells[5].Status != "Ambiguous clonotype" || vendor.Cells[5].Size != nil {
		t.Fatal(vendor.Cells[5])
	}
	paired := compute(t, tab, cells, clonotype.Definition{Chains: []string{"TRA", "TRB"}, RequireAll: true}, qc.Filter{MaxChains: 2}, Options{})
	if paired.Cells[4].Status != "Unassigned clonotype" || paired.Cells[6].Status != "QC excluded" {
		t.Fatal(paired.Cells)
	}
	// No eligible or sized cells gives a missing fraction, never a zero expansion rate.
	excluded := compute(t, tab, cells, clonotype.Definition{}, qc.Filter{ExcludeSamples: []string{"S1"}}, Options{})
	if excluded.Groups[0].ExpandedShare != nil {
		t.Fatal(excluded.Groups)
	}
}
func TestBoundsCancellationAndDeterminism(t *testing.T) {
	for _, o := range []Options{{Population: "all"}, {Breakpoints: []int{2, 1}}, {Breakpoints: []int{0}}, {Breakpoints: []int{1, 1}}, {Breakpoints: []int{1, 2, 3, 4, 5, 6}}} {
		if _, err := o.Normalize(); err == nil {
			t.Fatal(o)
		}
	}
	o, _ := (Options{}).Normalize()
	for n, want := range map[int]string{1: "1", 2: "2", 3: "3–5", 5: "3–5", 6: ">5"} {
		if o.Bin(n) != want {
			t.Fatal(n)
		}
	}
	tab, cells := fixture(t)
	d := clonotype.Definition{Chains: []string{"TRA", "TRB"}}
	ct, _ := clonotype.Compute(tab, nil, d)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Compute(ctx, tab, ct, qc.Filter{}, 0, cells, []string{"state"}, Options{}); err == nil {
		t.Fatal("cancellation")
	}
	for _, bad := range [][]Cell{append(append([]Cell{}, cells...), cells[0]), {{Barcode: "a"}}, make([]Cell, MaxCells+1)} {
		if _, err := Compute(context.Background(), tab, ct, qc.Filter{}, 0, bad, []string{"state"}, Options{}); err == nil {
			t.Fatal("bad cells accepted")
		}
	}
	if _, err := Compute(context.Background(), tab, ct, qc.Filter{}, 1, cells, []string{"state"}, Options{Annotation: "absent"}); err == nil {
		t.Fatal("unknown annotation")
	}
	r := compute(t, tab, cells, d, qc.Filter{}, Options{Annotation: "state"})
	raw, _ := json.Marshal(r)
	for i := 0; i < 5; i++ {
		again := compute(t, tab, cells, d, qc.Filter{}, Options{Annotation: "state"})
		if !reflect.DeepEqual(r, again) {
			t.Fatal("nondeterministic ordering")
		}
	}
	if digest := fmt.Sprintf("%x", sha256.Sum256(raw)); digest != "73194b2b80429f342a99b3955dc7e949cf95cb9b1ad97e70f449e0a6c673abf9" {
		t.Fatalf("phenotype digest %s", digest)
	}
}

func TestPhenotypeMapBoundAndEmptyPopulations(t *testing.T) {
	tab, cells := fixture(t)
	ct, err := clonotype.Compute(tab, nil, clonotype.Definition{Chains: []string{"TRB"}})
	if err != nil {
		t.Fatal(err)
	}
	many := make([]Cell, MaxMapPoints+1)
	for i := range many {
		many[i] = Cell{Barcode: fmt.Sprintf("cell-%d", i), Values: []string{""}, Position: &[2]float64{0, 0}}
	}
	if _, err := Compute(context.Background(), tab, ct, qc.Filter{}, 0, many, []string{"state"}, Options{}); err == nil {
		t.Fatal("oversized map was silently sampled")
	}
	empty, err := Compute(context.Background(), tab, ct, qc.Filter{}, 0, nil, nil, Options{Population: "mapped"})
	if err != nil || len(empty.Cells) != 0 || len(empty.Groups) != 0 || empty.Coverage.Sized != 0 {
		t.Fatal(empty, err)
	}
	for _, c := range empty.Clones {
		if c.Size != 0 || c.Mapped != 0 {
			t.Fatal(c)
		}
	}
	if _, err := Compute(context.Background(), tab, ct, qc.Filter{}, len(tab.Samples), cells, []string{"state"}, Options{}); err == nil {
		t.Fatal("invalid sample")
	}
}
