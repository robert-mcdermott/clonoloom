package track

import (
	"math"
	"testing"

	"clonoloom/internal/clonotype"
	"clonoloom/internal/rep"
	"clonoloom/internal/study"
)

// Wilson's interval against R's prop.test(k, n, correct = FALSE), which is Wilson's.
func TestWilsonAgainstR(t *testing.T) {
	z := 1.959963984540054
	for _, c := range []struct{ k, n, lo, hi float64 }{
		{0, 10, 0, 0.27753279986288903},
		{5, 10, 0.23659309051256408, 0.76340690948743595},
		{1, 1000, 0.00017654637062607817, 0.00564255859795793208},
		{37, 412, 0.065855918723968121, 0.121334312479646517},
		{10, 10, 0.72246720013711085, 1},
	} {
		lo, hi := Wilson(c.k, c.n, z)
		if math.Abs(lo-c.lo) > 1e-14 || math.Abs(hi-c.hi) > 1e-14 {
			t.Errorf("%v of %v: %.17g–%.17g, R %.17g–%.17g", c.k, c.n, lo, hi, c.lo, c.hi)
		}
	}
	if lo, hi := Wilson(0, 0, z); !math.IsNaN(lo) || !math.IsNaN(hi) {
		t.Errorf("0 of 0: %v–%v", lo, hi)
	}
}

// table makes a clonotype table of junctions (one TRB chain each) and their counts in each sample.
func table(unit rep.Unit, junctions []string, counts [][]float64) *clonotype.Table {
	t := &clonotype.Table{Unit: unit, Start: []int32{0}, SampleTotals: make([]float64, len(counts[0]))}
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

// A subject with an infusion product, two samples before it and two after.
var (
	names   = []string{"P1-product", "P1-pre-a", "P1-pre-b", "P1-d7", "P1-d28", "P2-d0"}
	samples = []study.Sample{
		{Name: "P1-product", Subject: "P1", Tissue: "infusion product"},
		{Name: "P1-pre-a", Subject: "P1", Time: "day -7"},
		{Name: "P1-pre-b", Subject: "P1", Time: "day -7"},
		{Name: "P1-d7", Subject: "P1", Time: "day 7"},
		{Name: "P1-d28", Subject: "P1", Time: "week 4"},
		{Name: "P2-d0", Subject: "P2", Time: "day 0"},
	}
	junctions = []string{"CASSA", "CASSB", "CASSC", "CASSD"}
	counts    = [][]float64{
		// product, pre-a, pre-b, d7, d28, P2
		{50, 1, 0, 30, 10, 0},
		{30, 0, 2, 0, 0, 5},
		{20, 4, 4, 4, 4, 0},
		{0, 95, 94, 66, 86, 95},
	}
)

func TestFollowByReference(t *testing.T) {
	ct := table(rep.Templates, junctions, counts)
	res, err := Follow(ct, names, study.Describe(samples), Choice{N: 2}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if res.Choice.By != ByReference || !res.Intervals || res.Caveat != "" || len(res.Subjects) != 1 || len(res.Without) != 1 || res.Without[0] != "P2" {
		t.Fatalf("%+v", res)
	}
	s := res.Subjects[0]
	// The product, the two samples before (pooled), day 7 and week 4.
	if len(s.Points) != 4 || s.Points[0].Kind != "reference" || s.Points[0].Label != "infusion product" || len(s.Points[1].Samples) != 2 || s.Points[3].Label != "week 4" || *s.Points[3].Days != 28 {
		t.Fatalf("points %+v", s.Points)
	}
	if len(s.Samples) != 5 || s.Samples[2].Point != 1 || s.Points[1].Total != 200 {
		t.Fatalf("samples %+v, total %v", s.Samples, s.Points[1].Total)
	}
	// The product's two largest, highest at their peak first.
	if len(s.Clonotypes) != 2 || s.Clonotypes[0].ID != "c-CASSA" || s.Clonotypes[1].ID != "c-CASSB" || s.Clonotypes[1].Why[0] != (Reason{Sample: "P1-product", Rank: 2}) {
		t.Fatalf("clonotypes %+v", s.Clonotypes)
	}
	b := s.Clonotypes[1]
	// Pooled before the infusion: 2 of 200.
	if v := b.Points[1]; v.Count != 2 || v.Total != 200 || float64(v.Frequency) != 0.01 {
		t.Errorf("pooled %+v", v)
	}
	// Absent at day 7: 0 of 100, from 0 to what 100 could miss.
	if v := b.Points[2]; v.Count != 0 || float64(v.Frequency) != 0 || float64(v.Lower) != 0 || math.Abs(float64(v.Upper)-0.036995) > 1e-5 {
		t.Errorf("absent %+v", v)
	}
	if b.Peak != 0 || s.Clonotypes[0].Peak != 0 {
		t.Errorf("peaks %d, %d", s.Clonotypes[0].Peak, b.Peak)
	}
	// The share of each point in the clonotypes followed: day 7, 30 of 100.
	if math.Abs(float64(s.Points[2].Followed)-0.30) > 1e-15 {
		t.Errorf("followed %v", s.Points[2].Followed)
	}
}

func TestFollowEachSamplesLargest(t *testing.T) {
	ct := table(rep.Templates, junctions, counts)
	// No reference sample: each sample's largest, the subject saying so.
	var plain []study.Sample
	for _, s := range samples[1:] {
		plain = append(plain, s)
	}
	res, err := Follow(ct, names, study.Describe(plain), Choice{By: ByReference, N: 1}, 0.95)
	if err != nil {
		t.Fatal(err)
	}
	s := res.Subjects[0]
	if s.By != ByLargest || len(s.Points) != 3 || len(s.Clonotypes) != 1 || s.Clonotypes[0].ID != "c-CASSD" || len(s.Clonotypes[0].Why) != 4 {
		t.Fatalf("%+v", s)
	}
	res, _ = Follow(ct, names, study.Describe(samples), Choice{By: ByLargest, N: 1}, 0.95)
	if got := res.Subjects[0].Clonotypes; len(got) != 2 || got[0].ID != "c-CASSD" || got[1].ID != "c-CASSA" {
		t.Fatalf("%+v", got)
	}
}

func TestFollowAList(t *testing.T) {
	ct := table(rep.Reads, junctions, counts)
	res, err := Follow(ct, names, study.Describe(samples), Choice{By: ByList, List: []string{"cassc", " c-CASSB ", "CASSZ", "cassc"}}, 0.95)
	if err != nil {
		t.Fatal(err)
	}
	if res.Intervals || res.Caveat == "" || len(res.Unmatched) != 1 || res.Unmatched[0] != "CASSZ" || len(res.Choice.List) != 3 {
		t.Fatalf("%+v", res)
	}
	got := res.Subjects[0].Clonotypes
	if len(got) != 2 || got[0].ID != "c-CASSB" || got[1].Why[0].Listed != "cassc" {
		t.Fatalf("%+v", got)
	}
	if v := got[0].Points[0]; !math.IsNaN(float64(v.Lower)) || float64(v.Frequency) != 0.3 {
		t.Errorf("in reads: %+v", v)
	}
}

func TestChoiceNormalize(t *testing.T) {
	for _, c := range []Choice{{By: "test"}, {By: ByList}, {By: ByList, List: []string{" "}}, {N: -1}, {N: MaxN + 1}} {
		if _, err := c.Normalize(); err == nil {
			t.Errorf("%+v: no error", c)
		}
	}
	if c, _ := (Choice{By: ByLargest}).Normalize(); c.N != 5 {
		t.Errorf("%+v", c)
	}
	if c, _ := (Choice{}).Normalize(); c.By != ByReference || c.N != 10 {
		t.Errorf("%+v", c)
	}
}
