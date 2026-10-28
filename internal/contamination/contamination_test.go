package contamination

import (
	"clonoloom/internal/diversity"
	"clonoloom/internal/seq"
	"context"
	"encoding/json"
	"math"
	"testing"
)

func fixture() Input {
	nt := "TGTGCTAGCAGCCAGGGTCATACCGCTAACGAGCAGTACTTT"
	r := Receptor{V: "TRBV5-1", J: "TRBJ2-7", NT: nt, AA: seq.Translate(nt), Counts: []float64{100, 2, 100, 0}, Pgen: diversity.Number(math.NaN())}
	r.ID = ID(r.V, r.J, r.NT)
	return Input{Samples: []Depth{{Sample: Sample{Name: "a", Subject: "one"}, Total: 200, Eligible: 100}, {Sample: Sample{Name: "b", Subject: "two"}, Total: 1000, Eligible: 2}, {Sample: Sample{Name: "repeat", Subject: "one"}, Total: 200, Eligible: 100}, {Sample: Sample{Name: "empty", Subject: "three"}}}, Receptors: []Receptor{r}}
}
func TestContaminationUnitsAndNulls(t *testing.T) {
	in := fixture()
	r, e := Compute(context.Background(), in)
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Pairs) != 5 || len(r.Matches) != 2 || r.SameSubjectPairs != 1 {
		t.Fatal(r)
	}
	m := r.Matches[0]
	if m.FractionA != .5 || m.FractionB != .002 || m.Ratio != 250 || !m.Review || m.Pattern != "Asymmetric observed fractions" || m.PgenClass != "Unknown Pgen" {
		t.Fatal(m)
	}
	if r.Pairs[1].Reason == "" || !math.IsNaN(float64(r.Pairs[1].FractionA)) {
		t.Fatal("zero depth treated as no sharing")
	}
	in.Receptors[0].Pgen = 1e-4
	high, e := Compute(context.Background(), in)
	if e != nil || high.Pairs[0].HighPgen != 1 || !high.Matches[0].Review {
		t.Fatal("publicness suppressed evidence", e)
	}
	in.Options.HighFraction = .9
	low, e := Compute(context.Background(), in)
	if e != nil || len(low.Matches) != len(high.Matches) || low.Matches[0].Review {
		t.Fatal("threshold removed matches", e)
	}
	in.Receptors[0].Pgen = 0
	zero, _ := Compute(context.Background(), in)
	if zero.Pairs[0].UnknownPgen != 0 || zero.Pairs[0].HighPgen != 0 {
		t.Fatal("model zero missing")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = Compute(ctx, in); e != context.Canceled {
		t.Fatal(e)
	}
	for _, mutate := range []func(*Input){func(x *Input) { x.Receptors[0].Counts[0] = math.NaN() }, func(x *Input) { x.Samples[0].Eligible = 99 }, func(x *Input) { x.Samples[1].Sample.Name = "a" }, func(x *Input) { x.Samples[1].Sample.Subject = "" }, func(x *Input) { x.Receptors[0].NT = "NNN" }, func(x *Input) { x.Options.Ratio = .5 }, func(x *Input) { x.Receptors = append(x.Receptors, x.Receptors[0]) }} {
		v := fixture()
		mutate(&v)
		if _, e = Compute(context.Background(), v); e == nil {
			t.Fatal("invalid evidence accepted")
		}
	}
	if _, e = json.Marshal(r); e != nil {
		t.Fatal(e)
	}
}
