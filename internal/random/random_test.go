package random

import (
	"math"
	"testing"
)

// The generator's draws are fixed by its seed (a change of Go's helpers cannot change a study).
func TestRandomDraws(t *testing.T) {
	r := New(42, 0)
	first := []float64{r.Float64(), r.Float64()}
	r2 := New(42, 0)
	if r2.Float64() != first[0] || r2.Float64() != first[1] {
		t.Fatal("not reproducible")
	}
	for _, c := range []struct {
		name       string
		draw       func() float64
		mean, vari float64
	}{
		{"Poisson(3)", func() float64 { return float64(r.Poisson(3)) }, 3, 3},
		{"Poisson(40)", func() float64 { return float64(r.Poisson(40)) }, 40, 40},
		{"NegBinomial(8, 2)", func() float64 { return float64(r.NegBinomial(8, 2)) }, 8, 8 + 64.0/2},
		{"Gamma(0.5)", func() float64 { return r.Gamma(0.5) }, 0.5, 0.5},
		{"IntN(10)", func() float64 { return float64(r.IntN(10)) }, 4.5, 8.25},
	} {
		var s, s2 float64
		const n = 200000
		for range n {
			x := c.draw()
			s, s2 = s+x, s2+x*x
		}
		mean := s / n
		vari := s2/n - mean*mean
		if math.Abs(mean-c.mean) > 4*math.Sqrt(c.vari/n) || math.Abs(vari-c.vari)/c.vari > 0.05 {
			t.Errorf("%s: mean %.3f variance %.3f", c.name, mean, vari)
		}
	}
}

// Binomial and multinomial draws: their means and variances, and a multinomial's total.
func TestBinomialAndMultinomial(t *testing.T) {
	r := New(7, 0)
	for _, c := range []struct {
		n int
		p float64
	}{{20, 0.1}, {1000, 0.003}, {500, 0.4}, {100000, 0.02}, {50, 0.9}, {7, 1}, {0, 0.5}} {
		var s, s2 float64
		const reps = 100000
		for range reps {
			x := float64(r.Binomial(c.n, c.p))
			s, s2 = s+x, s2+x*x
		}
		mean, vari := s/reps, s2/reps-(s/reps)*(s/reps)
		want := float64(c.n) * c.p
		wantVar := want * (1 - c.p)
		if math.Abs(mean-want) > 5*math.Sqrt(wantVar/reps)+1e-9 || (wantVar > 0 && math.Abs(vari-wantVar)/wantVar > 0.05) || (wantVar == 0 && vari != 0) {
			t.Errorf("Binomial(%d, %v): mean %.4f variance %.4f, want %.4f and %.4f", c.n, c.p, mean, vari, want, wantVar)
		}
	}
	chances := []float64{0.5, 0.3, 0.15, 0.05}
	sums := make([]float64, len(chances))
	var out []int
	for range 20000 {
		out = r.Multinomial(1000, chances, out)
		total := 0
		for i, k := range out {
			total += k
			sums[i] += float64(k)
		}
		if total != 1000 {
			t.Fatalf("a multinomial of %d", total)
		}
	}
	for i, p := range chances {
		if got := sums[i] / 20000 / 1000; math.Abs(got-p) > 0.002 {
			t.Errorf("category %d: %.4f, want %.4f", i, got, p)
		}
	}
}
