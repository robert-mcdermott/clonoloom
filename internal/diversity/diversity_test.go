package diversity

import (
	"encoding/json"
	"math"
	"runtime"
	"testing"
)

// iNEXT's spider data (Girdled and Logged), whose estimates the iNEXT vignette and Chao et al.
// 2014 print.
var girdled = []float64{46, 22, 17, 15, 15, 9, 8, 6, 6, 4, 2, 2, 2, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
var logged = []float64{88, 22, 16, 15, 13, 10, 8, 8, 7, 7, 7, 5, 4, 4, 4, 3, 3, 3, 3, 2, 2, 2, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}

func TestThePrintedSpiderEstimates(t *testing.T) {
	for _, c := range []struct {
		name                                    string
		data                                    []float64
		chao1, hill1, hill2, hill2Obs, coverage float64
	}{
		{"Girdled", girdled, 43.893, 13.826, 8.175, 7.840, 0.9289},
		{"Logged", logged, 61.403, 16.337, 6.920, 6.761, 0.9446},
	} {
		r := Measure(c.data, DefaultOptions())
		m := func(id string) float64 { return float64(r.Measures[id].Estimate) }
		for _, check := range []struct {
			what      string
			got, want float64
			places    float64
		}{{"Chao1", m("chao1"), c.chao1, 1e-3}, {"asymptotic Hill 1", m("hill1Estimated"), c.hill1, 1e-3}, {"asymptotic Hill 2", m("hill2Estimated"), c.hill2, 1e-3},
			{"Hill 2", m("hill2"), c.hill2Obs, 1e-3}, {"coverage", m("coverage"), c.coverage, 1e-4}} {
			if math.Abs(check.got-check.want) > check.places/2 {
				t.Errorf("%s %s: %.5f, printed %.5f", c.name, check.what, check.got, check.want)
			}
		}
		if r.Clonotypes != len(c.data) || r.Replicates != 200 {
			t.Errorf("%s: %+v", c.name, r)
		}
	}
}

// Closed forms on a small sample, by hand.
func TestTheObservedMeasures(t *testing.T) {
	r := Measure([]float64{5, 3, 1, 1, 0}, Options{})
	m := func(id string) float64 { return float64(r.Measures[id].Estimate) }
	p := []float64{0.5, 0.3, 0.1, 0.1}
	h := 0.0
	for _, x := range p {
		h -= x * math.Log(x)
	}
	for id, want := range map[string]float64{
		"richness": 4, "shannon": h, "hill1": math.Exp(h), "simpson": 0.36, "hill2": 1 / 0.36, "giniSimpson": 0.64,
		"evenness": h / math.Log(4), "clonality": 1 - h/math.Log(4), "simpsonClonality": 0.6, "maxFrequency": 0.5,
		"top10": 1, "hill@0": 4, "hill@2": 1 / 0.36, "hill@0.5": math.Pow(math.Sqrt(0.5)+math.Sqrt(0.3)+2*math.Sqrt(0.1), 2),
		// Gini of 1, 1, 3, 5: Σ (2i − 5) x_(i) / (4 · 10) = (−3 − 1 + 3 + 15) / 40.
		"gini": 14.0 / 40, "homeostasis.hyperexpanded": 1, "homeostasis.rare": 0,
	} {
		if got := m(id); math.Abs(got-want) > 1e-12 {
			t.Errorf("%s: %.15g, want %.15g", id, got, want)
		}
	}
	if r.Clonotypes != 4 || r.Singletons != 2 || r.Doubletons != 0 || r.Abundance != 10 {
		t.Errorf("%+v", r)
	}
	// Without a bootstrap, no interval but Chao1's.
	if !math.IsNaN(float64(r.Measures["hill1"].Lower)) || math.IsNaN(float64(r.Measures["chao1"].Upper)) {
		t.Errorf("intervals %+v %+v", r.Measures["hill1"], r.Measures["chao1"])
	}
}

// A replicate's draws are its own: the same intervals with one processor or many, and again.
func TestTheBootstrapIsReproducible(t *testing.T) {
	opt := DefaultOptions()
	opt.Stream = 3
	a := Measure(logged, opt)
	before := runtime.GOMAXPROCS(1)
	b := Measure(logged, opt)
	runtime.GOMAXPROCS(before)
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	if string(ja) != string(jb) {
		t.Fatal("different results on one processor")
	}
	opt.Stream = 4
	c := Measure(logged, opt)
	if c.Measures["hill1Estimated"].SE == a.Measures["hill1Estimated"].SE {
		t.Error("another stream drew the same replicates")
	}
	// Intervals hold their estimate, within their bounds.
	for id, v := range a.Measures {
		lo, hi := float64(v.Lower), float64(v.Upper)
		if math.IsNaN(lo) {
			continue
		}
		if id != "chao1" && (lo > float64(v.Estimate)+1e-9 || hi < float64(v.Estimate)-1e-9) {
			t.Errorf("%s: %v outside %v–%v", id, float64(v.Estimate), lo, hi)
		}
		if bl, bh := bounds(id, float64(a.Clonotypes)); lo < bl-1e-12 || hi > bh+1e-12 {
			t.Errorf("%s: interval %v–%v outside %v–%v", id, lo, hi, bl, bh)
		}
	}
}

// Edge cases: one clonotype, none, counts that are not whole (only what does not need counts), no
// clonotype seen once (iNEXT's other Chao1 interval), every rare clonotype a singleton (no ACE).
func TestEdgeCases(t *testing.T) {
	if r := Measure(nil, DefaultOptions()); r.Clonotypes != 0 || len(r.Measures) != 0 {
		t.Errorf("none: %+v", r)
	}
	one := Measure([]float64{7}, DefaultOptions())
	if float64(one.Measures["richness"].Estimate) != 1 || !math.IsNaN(float64(one.Measures["evenness"].Estimate)) || float64(one.Measures["gini"].Estimate) != 0 {
		t.Errorf("one: %+v", one.Measures)
	}
	frac := Measure([]float64{0.5, 1.5, 2}, DefaultOptions())
	if frac.Whole || frac.Replicates != 0 {
		t.Errorf("fractions bootstrapped: %+v", frac)
	}
	if _, ok := frac.Measures["chao1"]; ok {
		t.Error("Chao1 of counts that are not whole")
	}
	if float64(frac.Measures["hill2"].Estimate) != 1/(0.125*0.125+0.375*0.375+0.25) {
		t.Errorf("hill2 %v", float64(frac.Measures["hill2"].Estimate))
	}
	// No singletons: Chao1 is the clonotypes seen, with iNEXT's interval above them.
	x2 := []float64{10, 40}
	for range 50 {
		x2 = append(x2, 2)
	}
	for range 20 {
		x2 = append(x2, 3)
	}
	c := Measure(x2, Options{}).Measures["chao1"]
	if float64(c.Estimate) != 72 || math.Abs(float64(c.SE)-2.345) > 5e-4 || math.Abs(float64(c.Lower)-75.55) > 5e-3 || math.Abs(float64(c.Upper)-85.851) > 5e-4 {
		t.Errorf("Chao1 without singletons: %+v (iNEXT: 72, 2.345, 75.55–85.851)", c)
	}
	if ace := float64(Measure([]float64{1, 1, 1, 50}, Options{}).Measures["ace"].Estimate); !math.IsNaN(ace) {
		t.Errorf("ACE with every rare clonotype a singleton: %v", ace)
	}
}

func TestNumericalPieces(t *testing.T) {
	// ψ(1) = −γ, ψ(n) = H_{n−1} − γ.
	if math.Abs(digamma(1)+0.5772156649015329) > 1e-13 || math.Abs(digamma(10)-(1+0.5+1.0/3+0.25+0.2+1.0/6+1.0/7+0.125+1.0/9-0.5772156649015329)) > 1e-13 {
		t.Errorf("digamma %v %v", digamma(1), digamma(10))
	}
	// The tail sum by Euler–Maclaurin against direct summation.
	for _, c := range []struct{ x, n float64 }{{0.999, 80}, {0.9999, 1000}, {0.6, 60}, {0.99999, 5000}} {
		direct, term := 0.0, 1.0
		for j := 0.0; j < 5e7; j++ {
			direct += term / (c.n + j)
			term *= c.x
			if term < 1e-18 {
				break
			}
		}
		if got := tail(c.x, c.n); math.Abs(got-direct) > 1e-10*direct {
			t.Errorf("tail(%v, %v): %.15g, direct %.15g", c.x, c.n, got, direct)
		}
	}
	for _, p := range []float64{0.975, 0.5, 0.995, 0.025} {
		z := normalQuantile(p)
		if got := 0.5 * math.Erfc(-z/math.Sqrt2); math.Abs(got-p) > 1e-14 {
			t.Errorf("quantile %v: %v", p, z)
		}
	}
}

// The clonotypes seen hold the sample coverage of the repertoire, a clonotype seen once tuned down
// most.
func TestDetectedFrequencies(t *testing.T) {
	p := Detected(logged)
	sum := 0.0
	for _, v := range p {
		sum += v
	}
	n := 0.0
	for _, v := range logged {
		n += v
	}
	st := statsOf(FreqOf(logged))
	if math.Abs(sum-coverage(st)) > 1e-12 {
		t.Errorf("detected frequencies sum to %v, coverage %v", sum, coverage(st))
	}
	if p[0] > 88/n || p[0] < 87/n || p[len(p)-1] >= 1/n {
		t.Errorf("88 cells: %v (of %v); one: %v (of %v)", p[0]*n, n, p[len(p)-1]*n, n)
	}
}
