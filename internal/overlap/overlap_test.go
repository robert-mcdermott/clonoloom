package overlap

import (
	"encoding/json"
	"math"
	"runtime"
	"testing"

	"clonoloom/internal/clonotype"
	"clonoloom/internal/diversity"
	"clonoloom/internal/random"
	"clonoloom/internal/rep"
)

// Three samples by hand. A: x 3, y 1, z 2, u 1; B: y 2, z 2, w 5; C: v 4, u 1. A and B are one
// subject's (P1), C another's (P2); A and C are of condition g1, B of g2.
func handTable(t *testing.T) (*clonotype.Table, []string) {
	t.Helper()
	b := rep.NewBuilder(rep.Templates)
	add := func(sample, aa string, count float64) {
		r := rep.NewRow()
		r.Sample, r.Locus, r.V, r.J, r.JunctionAA, r.Count, r.Productive = sample, "TRB", "TRBV5-1", "TRBJ2-7", aa, count, rep.True
		if err := b.Append(&r); err != nil {
			t.Fatal(err)
		}
	}
	junction := map[string]string{"x": "CASSXF", "y": "CASSYF", "z": "CASSZF", "u": "CASSUF", "w": "CASSWF", "v": "CASSVF"}
	for _, r := range []struct {
		sample, clonotype string
		count             float64
	}{{"A", "x", 3}, {"A", "y", 1}, {"A", "z", 2}, {"A", "u", 1}, {"B", "y", 2}, {"B", "z", 2}, {"B", "w", 5}, {"C", "v", 4}, {"C", "u", 1}} {
		add(r.sample, junction[r.clonotype], r.count)
	}
	rt := b.Table()
	ct, err := clonotype.Compute(rt, nil, clonotype.Definition{Junction: "aa", V: "none", J: "none", ProductiveOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, s := range rt.Samples {
		names = append(names, s.Name)
	}
	return ct, names
}

func near(v Number, want float64) bool { return math.Abs(float64(v)-want) < 1e-12 }

func TestPairsByHand(t *testing.T) {
	ct, names := handTable(t)
	subjects := map[string]string{"A": "P1", "B": "P1", "C": "P2"}
	groups := map[string]string{"A": "g1", "B": "g2", "C": "g1"}
	res := Measure(ct, names, subjects, groups, diversity.Options{})
	if len(res.Pairs) != 3 || res.Depth != 5 || res.Samples[0].Clonotypes != 4 || res.Samples[1].Abundance != 9 {
		t.Fatalf("%+v", res)
	}
	ab := res.Pairs[0].Measures
	for id, want := range map[string]float64{
		Shared: 2, Jaccard: 2.0 / 5, Sorensen: 4.0 / 7, Coefficient: 2.0 / 3,
		// Σab = 1·2 + 2·2 = 6; A = 7, B = 9; Σa² = 15, Σb² = 33; Σa(a−1) = 8, Σb(b−1) = 24.
		MorisitaHorn: 12 / ((15.0/49 + 33.0/81) * 63), Morisita: 4.0 / 11,
		AbundanceOfA: 3.0 / 7, AbundanceOfB: 4.0 / 9,
	} {
		if !near(ab[id].Estimate, want) {
			t.Errorf("A and B %s: %v, want %v", id, float64(ab[id].Estimate), want)
		}
	}
	if ac := res.Pairs[1].Measures; !near(ac[Shared].Estimate, 1) || !near(ac[Jaccard].Estimate, 1.0/5) {
		t.Errorf("A and C %+v", ac)
	}
	// Rarefied to 5 cells: C is not subsampled; A and B are.
	seen := func(n, x float64) float64 { return diversity.SeenChance(n, x, 5) }
	eA := seen(7, 3) + seen(7, 1) + seen(7, 2) + seen(7, 1)
	eB := 2*seen(9, 2) + seen(9, 5)
	eShared := seen(7, 1)*seen(9, 2) + seen(7, 2)*seen(9, 2)
	if !near(ab[Shared+Rarefied].Estimate, eShared) || !near(ab[Jaccard+Rarefied].Estimate, eShared/(eA+eB-eShared)) {
		t.Errorf("rarefied %+v %+v", ab[Shared+Rarefied], ab[Jaccard+Rarefied])
	}
	if bc := res.Pairs[2].Measures; !near(bc[Shared+Rarefied].Estimate, 0) {
		t.Errorf("B and C rarefied %+v", bc[Shared+Rarefied])
	}
	// The summaries: A–B the same subject, A–C and B–C different ones; A–C the same condition.
	kinds := map[string]int{}
	for _, s := range res.Summaries {
		kinds[s.Kind] = s.Pairs
	}
	if kinds["same subject"] != 1 || kinds["different subjects"] != 2 || kinds["same condition"] != 1 || kinds["different conditions"] != 2 {
		t.Errorf("summaries %v", kinds)
	}
	// Public by subject: u alone is in both subjects.
	p := res.Public
	if p.By != "subject" || p.Units != 2 || p.Incidence[0].Clonotypes != 5 || p.Incidence[1].Clonotypes != 1 || len(p.Top) != 1 || p.Top[0].Chains[0].Junction != "CASSUF" || p.Top[0].Abundance != 2 {
		t.Errorf("public %+v", p)
	}
	// Without subjects, by sample: y, z and u are each in two samples, z first (more abundant).
	q := Measure(ct, names, nil, nil, diversity.Options{}).Public
	if q.By != "sample" || q.Units != 3 || q.Incidence[1].Clonotypes != 3 || q.Top[0].Chains[0].Junction != "CASSZF" {
		t.Errorf("public by sample %+v", q)
	}
}

// The rarefied expectation against subsamples drawn at random.
func TestRarefiedAgainstSubsamples(t *testing.T) {
	ct, names := handTable(t)
	want := float64(Measure(ct, names, nil, nil, diversity.Options{}).Pairs[0].Measures[Shared+Rarefied].Estimate)
	cells := func(counts map[string]int) []string {
		var out []string
		for k, c := range counts {
			for range c {
				out = append(out, k)
			}
		}
		return out
	}
	a := cells(map[string]int{"x": 3, "y": 1, "z": 2, "u": 1})
	b := cells(map[string]int{"y": 2, "z": 2, "w": 5})
	r := random.New(1, 1)
	subsample := func(pool []string) map[string]bool {
		p := append([]string(nil), pool...)
		for i := range 5 {
			j := i + r.IntN(len(p)-i)
			p[i], p[j] = p[j], p[i]
		}
		out := map[string]bool{}
		for _, c := range p[:5] {
			out[c] = true
		}
		return out
	}
	const draws = 200000
	sum, sq := 0.0, 0.0
	for range draws {
		sa, sb := subsample(a), subsample(b)
		n := 0.0
		for k := range sa {
			if sb[k] {
				n++
			}
		}
		sum += n
		sq += n * n
	}
	mean := sum / draws
	se := math.Sqrt((sq/draws - mean*mean) / draws)
	if math.Abs(mean-want) > 4*se {
		t.Errorf("shared at 5 cells: %v exactly, %v ± %v by subsampling", want, mean, se)
	}
}

func TestTheBootstrap(t *testing.T) {
	ct, names := handTable(t)
	opt := diversity.DefaultOptions()
	a := Measure(ct, names, nil, nil, opt)
	before := runtime.GOMAXPROCS(1)
	b := Measure(ct, names, nil, nil, opt)
	runtime.GOMAXPROCS(before)
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	if string(ja) != string(jb) {
		t.Fatal("different results on one processor")
	}
	for _, p := range a.Pairs {
		for id, v := range p.Measures {
			if math.IsNaN(float64(v.Estimate)) {
				continue
			}
			if math.IsNaN(float64(v.SE)) || v.Lower > v.Estimate+1e-12 || v.Upper < v.Estimate-1e-12 || v.Lower < 0 {
				t.Errorf("%d–%d %s: %+v", p.A, p.B, id, v)
			}
		}
	}
	if a.Replicates != 200 {
		t.Errorf("replicates %d", a.Replicates)
	}
}
