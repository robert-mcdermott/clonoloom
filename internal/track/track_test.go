package track

import (
	"errors"
	"math"
	"strings"
	"testing"

	"clonoloom/internal/clonotype"
	"clonoloom/internal/expansion"
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

// Two time points compared in each subject that has both, the samples at one time pooled; and the
// clonotypes called followed.
func TestCompareTwoTimePoints(t *testing.T) {
	ct := table(rep.Templates, junctions, counts)
	design := study.Describe(samples)
	if s := design.Subjects[0].Samples; s[0].Key != "reference" || s[1].Key != "day -7" || s[4].Key != "day 28" {
		t.Fatalf("keys %+v", s)
	}
	c := Comparison{From: "day -7", To: "day 7", Options: expansion.Options{Alpha: 0.05}}
	res, err := Test(ct, names, design, c)
	if err != nil {
		t.Fatal(err)
	}
	// The beta-binomial test, its variance fitted on the two samples taken a week before: CASSC and
	// CASSD have 5 or more in the two.
	if len(res.Subjects) != 1 || len(res.Without) != 1 || res.Without[0] != "P2" || res.Caveat == "" || res.Comparison.Test != expansion.Auto || res.Test != expansion.BetaBinomial {
		t.Fatalf("%+v", res)
	}
	if f := res.Fit; f == nil || f.On != "replicates" || len(f.Pairs) != 1 || f.Pairs[0].Clonotypes != 2 || f.Clonotypes != 2 || len(f.Points) != 2 || math.Abs(f.Pairs[0].Theta-99.0/197) > 1e-15 {
		t.Fatalf("fit %+v", res.Fit)
	}
	s := res.Subjects[0]
	// Before: 200 templates in two samples; day 7: 100. CASSB, 2 in all, is not tested.
	if s.From.Label != "day -7" || len(s.From.Samples) != 2 || s.TotalA != 200 || s.TotalB != 100 || s.Tested != 3 || s.LeftOut != 1 || s.Increased != 1 || s.Decreased != 1 {
		t.Fatalf("%+v", s)
	}
	if s.Clonotypes[0].ID != "c-CASSA" || s.Clonotypes[0].Call != expansion.Increase || s.Clonotypes[1].ID != "c-CASSD" || s.Clonotypes[1].Call != expansion.Decrease {
		t.Errorf("%+v", s.Clonotypes)
	}
	// Followed: the calls, the most significant first.
	f, err := Follow(ct, names, design, Choice{By: ByTest, Test: &c}, 0.95)
	if err != nil {
		t.Fatal(err)
	}
	if f.Choice.N != 20 || len(f.Subjects) != 1 || len(f.Without) != 1 {
		t.Fatalf("%+v", f)
	}
	got := f.Subjects[0].Clonotypes
	if len(got) != 2 || got[0].Why[0] != (Reason{Called: expansion.Increase, Rank: 1}) && got[1].Why[0] != (Reason{Called: expansion.Increase, Rank: 1}) {
		t.Errorf("%+v", got)
	}
	// Without replicates, automatically the binomial test, and said why; the beta-binomial test
	// asked for, refused; fitted on two time points named as stable instead.
	var plain []study.Sample
	for _, x := range samples {
		if x.Name != "P1-pre-b" {
			plain = append(plain, x)
		}
	}
	res, _ = Test(ct, names, study.Describe(plain), c)
	if res.Test != expansion.Binomial || res.Fit != nil || !strings.HasPrefix(res.Caveat, "No replicate samples") {
		t.Errorf("without replicates: %+v", res)
	}
	if _, err := Test(ct, names, study.Describe(plain), Comparison{From: "day -7", To: "day 7", Options: expansion.Options{Test: expansion.BetaBinomial}}); !errors.Is(err, expansion.ErrNoObservations) {
		t.Errorf("beta-binomial without replicates: %v", err)
	}
	res, err = Test(ct, names, study.Describe(plain), Comparison{From: "day -7", To: "day 7", Stable: &Stable{From: "day 7", To: "day 28"}})
	if err != nil || res.Test != expansion.BetaBinomial || res.Fit.On != "stable" || res.Fit.Pairs[0].A[0] != "P1-d7" {
		t.Errorf("stable: %v %+v", err, res.Fit)
	}
	// Against the infusion product; in reads, refused; a comparison of one time with itself.
	if res, _ := Test(ct, names, design, Comparison{From: "reference", To: "day 28"}); len(res.Subjects) != 1 || res.Subjects[0].From.Label != "infusion product" {
		t.Errorf("%+v", res)
	}
	if _, err := Test(table(rep.Reads, junctions, counts), names, design, c); !errors.Is(err, ErrReads) {
		t.Errorf("reads: %v", err)
	}
	if _, err := (Choice{By: ByTest, Test: &Comparison{From: "day 7", To: "day 7"}}).Normalize(); err == nil {
		t.Error("one time compared with itself")
	}
}

// Classes from the base (a week before) to day 7 and day 28: CASSA up at both (emerging), CASSD
// down at day 7 only (transient), CASSC never called and found throughout (persistent), CASSB never
// called and missing at day 7 (not classed).
func TestClassify(t *testing.T) {
	ct := table(rep.Templates, junctions, counts)
	res, err := Classify(ct, names, study.Describe(samples), Classing{Base: "day -7", Options: expansion.Options{Test: expansion.Binomial, Alpha: 0.05}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Test != expansion.Binomial || len(res.Subjects) != 1 || len(res.Without) != 1 || res.Fit != nil {
		t.Fatalf("%+v", res)
	}
	s := res.Subjects[0]
	if s.Base.Label != "day -7" || len(s.Later) != 2 || s.Later[1].Label != "week 4" || s.Counts != (ClassCounts{Emerging: 1, Transient: 1, Persistent: 1, Unclassed: 1}) {
		t.Fatalf("%+v", s)
	}
	got := map[string]ClassedClonotype{}
	for _, c := range s.Clonotypes {
		got[c.ID] = c
	}
	if a := got["c-CASSA"]; a.Class != Emerging || a.Calls[0] != expansion.Increase || a.Calls[1] != expansion.Increase || a.Frequencies[1] != 0.3 {
		t.Errorf("CASSA %+v", a)
	}
	if d := got["c-CASSD"]; d.Class != Transient || d.Calls[0] != expansion.Decrease || d.Calls[1] != "" {
		t.Errorf("CASSD %+v", d)
	}
	if c := got["c-CASSC"]; c.Class != Persistent || c.Q != 1 {
		t.Errorf("CASSC %+v", c)
	}
	if s.Clonotypes[0].ID != "c-CASSA" || s.Clonotypes[2].ID != "c-CASSC" {
		t.Errorf("order %+v", s.Clonotypes)
	}
	// Automatically, the beta-binomial test, fitted on the two samples taken a week before.
	if res, err := Classify(ct, names, study.Describe(samples), Classing{Base: "day -7"}); err != nil || res.Test != expansion.BetaBinomial || res.Fit == nil {
		t.Errorf("auto: %v %+v", err, res)
	}
	if _, err := (Classing{}).Normalize(); err == nil {
		t.Error("no base")
	}
}

// Each sample's share in the clonotypes of the infusion product (CASSA, CASSB, CASSC), and before
// the infusion.
func TestAttribute(t *testing.T) {
	ct := table(rep.Templates, junctions, counts)
	res, err := Attribute(ct, names, study.Describe(samples), 0.95)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Subjects) != 1 || len(res.Without) != 1 || res.Without[0] != "P2" || !res.Intervals {
		t.Fatalf("%+v", res)
	}
	a := res.Subjects[0]
	if a.Label != "infusion product" || a.Clonotypes != 3 || len(a.Samples) != 4 || len(a.Points) != 3 || a.Before == nil {
		t.Fatalf("%+v", a)
	}
	want := []float64{0.055, 0.34, 0.14}
	for k, p := range a.Points {
		if math.Abs(float64(p.Value.Frequency)-want[k]) > 1e-15 || !(float64(p.Value.Lower) < want[k] && want[k] < float64(p.Value.Upper)) {
			t.Errorf("%s: %+v", p.Point.Label, p.Value)
		}
	}
	if a.Before.Count != 11 || a.Before.Total != 200 || a.Samples[1].Name != "P1-pre-b" || a.Samples[1].Value.Count != 6 {
		t.Errorf("before %+v, samples %+v", a.Before, a.Samples)
	}
	// The product's clonotypes are all found before the infusion: its cells cannot be told apart.
	if a.Overlap == nil || *a.Overlap != 1 || a.Derived || a.Why == "" || a.Points[1].From != nil {
		t.Errorf("overlap %v, derived %v (%s)", a.Overlap, a.Derived, a.Why)
	}
	// A donor sharing no clonotype with the recipient before the transplant: of a later sample's
	// cells in clonotypes of one side only (6 of the donor's, 3 of the recipient's), 6 of 9.
	ct = table(rep.Templates, []string{"CASSX", "CASSY", "CASSZ"}, [][]float64{{10, 0, 6}, {0, 20, 3}, {0, 0, 1}})
	res, err = Attribute(ct, []string{"D", "pre", "d30"}, study.Describe([]study.Sample{
		{Name: "D", Subject: "R1", Tissue: "donor"}, {Name: "pre", Subject: "R1", Time: "baseline"}, {Name: "d30", Subject: "R1", Time: "day 30"}}), 0.95)
	if err != nil {
		t.Fatal(err)
	}
	b := res.Subjects[0]
	if !b.Derived || *b.Overlap != 0 || b.Points[0].From != nil || b.Points[1].From == nil || b.Points[1].From.Count != 6 || b.Points[1].From.Total != 9 || float64(b.Points[1].Value.Frequency) != 0.6 {
		t.Errorf("%+v", b)
	}
}

// Following a class: the emerging clonotypes, CASSA.
func TestFollowAClass(t *testing.T) {
	ct := table(rep.Templates, junctions, counts)
	res, err := Follow(ct, names, study.Describe(samples), Choice{By: ByClass, Class: Emerging, Classing: &Classing{Base: "day -7", Options: expansion.Options{Test: expansion.Binomial, Alpha: 0.05}}}, 0.95)
	if err != nil {
		t.Fatal(err)
	}
	if res.Choice.N != 20 || len(res.Subjects) != 1 || len(res.Subjects[0].Clonotypes) != 1 || res.Subjects[0].Clonotypes[0].Why[0] != (Reason{Class: Emerging, Rank: 1}) {
		t.Fatalf("%+v", res)
	}
	if _, err := (Choice{By: ByClass, Class: "rising", Classing: &Classing{Base: "day 0"}}).Normalize(); err == nil {
		t.Error("an unknown class")
	}
}
