package expansion

import (
	"errors"
	"math"
	"testing"

	"clonoloom/internal/clonotype"
	"clonoloom/internal/rep"
)

// table makes a clonotype table of junctions and their counts in each sample.
func table(junctions []string, counts [][]float64) *clonotype.Table {
	t := &clonotype.Table{Unit: rep.Templates, Start: []int32{0}, SampleTotals: make([]float64, len(counts[0]))}
	for i, j := range junctions {
		t.Clonotypes = append(t.Clonotypes, clonotype.Clonotype{ID: "c-" + j, Chains: []clonotype.Chain{{Locus: "TRB", Junction: j}}})
		for s, n := range counts[i] {
			if n > 0 {
				t.SampleOf = append(t.SampleOf, uint32(s))
				t.Abundance = append(t.Abundance, n)
				t.SampleTotals[s] += n
			}
		}
		t.Start = append(t.Start, int32(len(t.SampleOf)))
	}
	return t
}

func TestCompareCallsWhatChanged(t *testing.T) {
	// Samples 0 and 1 together against sample 2: A grows, B shrinks, C and D stay, E too few. (The
	// changes are small, as the tests assume: a clonotype that grew to a large share would shrink
	// every other one's.)
	ct := table([]string{"A", "B", "C", "D", "E"}, [][]float64{
		{10, 10, 60},
		{100, 100, 20},
		{500, 500, 500},
		{5000, 5000, 5000},
		{1, 0, 2},
	})
	res, err := Compare(ct, []uint32{0, 1}, []uint32{2}, Options{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.TotalA != 11221 || res.TotalB != 5582 || res.Tested != 4 || res.LeftOut != 1 || res.Increased != 1 || res.Decreased != 1 || !math.IsNaN(float64(res.Pi0)) {
		t.Fatalf("%+v", res)
	}
	// Those called first, by adjusted p-value.
	if res.Clonotypes[0].Call == "" || res.Clonotypes[1].Call == "" || res.Clonotypes[2].Call != "" {
		t.Fatalf("%+v", res.Clonotypes)
	}
	for _, c := range res.Clonotypes {
		if c.ID == "c-A" && (c.Call != Increase || c.A != 20 || c.B != 60 || math.Abs(float64(c.Log2Fold)-math.Log2((60.0/5582)/(20.0/11221))) > 1e-12) {
			t.Errorf("A: %+v", c)
		}
		if c.ID == "c-B" && c.Call != Decrease {
			t.Errorf("B: %+v", c)
		}
	}
	// One-sided: only the increase is called; Storey's π₀ estimated.
	res, _ = Compare(ct, []uint32{0, 1}, []uint32{2}, Options{Alternative: Increase, Correction: Storey, Test: Fisher}, nil)
	if res.Increased != 1 || res.Decreased != 0 || math.IsNaN(float64(res.Pi0)) {
		t.Errorf("one-sided: %+v", res)
	}
}

func TestCompareNeedsCounts(t *testing.T) {
	ct := table([]string{"A"}, [][]float64{{2.5, 3}})
	if _, err := Compare(ct, []uint32{0}, []uint32{1}, Options{}, nil); !errors.Is(err, ErrNotCounts) {
		t.Errorf("%v", err)
	}
	for _, o := range []Options{{Test: "t"}, {Alternative: "up"}, {Correction: "holm"}, {Alpha: 2}, {MinTotal: -1}} {
		if _, err := o.Normalize(); err == nil {
			t.Errorf("%+v: no error", o)
		}
	}
}

func TestExactPOnItsEdges(t *testing.T) {
	// A clonotype of one sample only, and a share that makes one outcome certain.
	if p := BinomialP(0, 10, 0.5, TwoSided); math.Abs(p-2.0/1024) > 1e-15 {
		t.Errorf("0 of 10: %v", p)
	}
	if BinomialP(3, 3, 1, TwoSided) != 1 || BinomialP(2, 3, 1, TwoSided) != 0 || BinomialP(0, 0, 0.5, TwoSided) != 1 {
		t.Error("degenerate shares")
	}
	if p := FisherP(5, 5, 100, 100, TwoSided); p != 1 {
		t.Errorf("equal: %v", p)
	}
}
