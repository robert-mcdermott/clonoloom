package diversity

import (
	"encoding/json"
	"math"
	"math/big"
	"runtime"
	"strings"
	"testing"

	"clonoloom/internal/random"
)

// bruteHill is the expected Hill numbers of a subsample of size m by the definition (Chao et al.
// 2014, Eq. 4): E f_k(m) = Σ_i C(X_i, k)·C(n−X_i, m−k)/C(n, m) for every k from 1 to m, as iNEXT
// sums them; and the coverage, 1 − Σ_i (X_i/n)·C(n−X_i, m)/C(n−1, m).
func bruteHill(x []float64, m float64) (h [3]float64, cov float64) {
	n := 0.0
	for _, v := range x {
		n += v
	}
	lchoose := func(a, b float64) float64 {
		if b < 0 || b > a {
			return math.Inf(-1)
		}
		x, _ := math.Lgamma(a + 1)
		y, _ := math.Lgamma(b + 1)
		z, _ := math.Lgamma(a - b + 1)
		return x - y - z
	}
	var s0, s1, s2 float64
	for k := 1.0; k <= m; k++ {
		fk := 0.0
		for _, xi := range x {
			fk += math.Exp(lchoose(xi, k) + lchoose(n-xi, m-k) - lchoose(n, m))
		}
		s0 += fk
		s1 -= fk * k / m * math.Log(k/m)
		s2 += fk * (k / m) * (k / m)
	}
	missed := 0.0
	for _, xi := range x {
		missed += xi / n * math.Exp(lchoose(n-xi, m)-lchoose(n-1, m))
	}
	return [3]float64{s0, math.Exp(s1), 1 / s2}, 1 - missed
}

func curveOf(x []float64) *curve {
	f := FreqOf(x)
	return newCurve(f, statsOf(f))
}

func TestInterpolationAgainstTheDefinition(t *testing.T) {
	for _, x := range [][]float64{girdled, logged, {5, 3, 1, 1}, {40, 1, 1, 1, 2, 2, 7}} {
		c := curveOf(x)
		for _, m := range []float64{1, 2, 3, 7, math.Floor(c.n / 3), math.Floor(c.n / 2), c.n - 2, c.n - 1} {
			if m < 1 || m >= c.n {
				continue
			}
			want, wantCov := bruteHill(x, m)
			got := c.hill(m)
			for q := range 3 {
				if math.Abs(got[q]-want[q]) > 1e-10*want[q] {
					t.Errorf("n %v, m %v, q %d: %.15g, by definition %.15g", c.n, m, q, got[q], want[q])
				}
			}
			if gc := c.coverage(m); math.Abs(gc-wantCov) > 1e-12 {
				t.Errorf("n %v, m %v: coverage %.15g, by definition %.15g", c.n, m, gc, wantCov)
			}
		}
		// At n, the values seen; at n − 1 for richness, S − f1/n.
		if h := c.hill(c.n); h != c.obs {
			t.Errorf("at n: %v, seen %v", h, c.obs)
		}
		if got := c.interpolated(c.n - 1)[0]; math.Abs(got-(float64(c.st.s)-c.st.f1/c.n)) > 1e-10 {
			t.Errorf("richness at n − 1: %v", got)
		}
	}
}

// Large samples: the sums from the mode, stopped where the terms no longer count, against every
// term of the distribution in 256-bit arithmetic (the probabilities relative to the lowest count's,
// each from the one before).
func TestMeanYLogYAgainstEveryTerm(t *testing.T) {
	for _, c := range []struct{ n, x, m float64 }{{1e6, 5000, 4e5}, {1e6, 1, 5e5}, {1e6, 2, 999999}, {200000, 150000, 100000}, {50, 50, 20}, {1e7, 30, 9e6}, {1000, 10, 995}} {
		lo, hi := max(0, c.m+c.x-c.n), min(c.x, c.m)
		bf := func(v float64) *big.Float { return new(big.Float).SetPrec(256).SetFloat64(v) }
		p, sum, weighted := bf(1), bf(0), bf(0)
		for k := lo; k <= hi; k++ {
			if k > lo {
				// P(k)/P(k−1) = (x−k+1)(m−k+1) / (k (n−x−m+k)).
				p.Mul(p, bf((c.x-k+1)*(c.m-k+1)))
				p.Quo(p, bf(k*(c.n-c.x-c.m+k)))
			}
			sum.Add(sum, p)
			weighted.Add(weighted, new(big.Float).SetPrec(256).Mul(p, bf(ylny(k))))
		}
		want, _ := new(big.Float).SetPrec(256).Quo(weighted, sum).Float64()
		if got := meanYLogY(c.n, c.x, c.m); math.Abs(got-want) > 1e-13*max(want, 1) {
			t.Errorf("%+v: %.15g, every term %.15g", c, got, want)
		}
	}
	// logNone by its product, small and large.
	for _, c := range []struct{ n, x, m float64 }{{1e6, 1, 3}, {1e6, 40, 50000}, {1e6, 5000, 400000}, {100, 60, 30}} {
		want := 0.0
		for j := 0.0; j < c.x; j++ {
			want += math.Log((c.n - c.m - j) / (c.n - j))
		}
		if got := logNone(c.n, c.x, c.m); math.Abs(got-want) > 1e-12*max(1, -want) {
			t.Errorf("logNone %+v: %.15g, product %.15g", c, got, want)
		}
	}
}

func TestExtrapolationAndCoverage(t *testing.T) {
	c := curveOf(logged)
	n := c.n
	// Far beyond the sample, the extrapolated values approach the whole repertoire's.
	far := c.hill(1e7 * n)
	for q, want := range []float64{c.asy[0], c.asy[1], c.asy[2]} {
		if math.Abs(far[q]-want) > 1e-6*want {
			t.Errorf("q %d far out: %v, asymptote %v", q, far[q], want)
		}
	}
	// Richness's extrapolation is Chao et al. 2014's Eq. 9: S + f0[1 − (1 − f1/(n f0 + f1))^m*].
	f0 := f0(c.st)
	for _, ms := range []float64{1, 10, n} {
		want := float64(c.st.s) + f0*(1-math.Pow(1-c.st.f1/(n*f0+c.st.f1), ms))
		if got := c.hill(n + ms)[0]; math.Abs(got-want) > 1e-10*want {
			t.Errorf("richness at n + %v: %v, Eq. 9 %v", ms, got, want)
		}
	}
	// Coverage at n is Chao and Jost's estimate; size and coverage invert each other.
	if got := c.coverage(n); math.Abs(got-coverage(c.st)) > 1e-15 {
		t.Errorf("coverage at n %v, estimate %v", got, coverage(c.st))
	}
	for _, m := range []float64{1, 5, 17.5, n / 2, n - 1, n, n + 3, 1.7 * n} {
		if back := c.sizeAt(c.coverage(m)); math.Abs(back-m) > 1e-6*m {
			t.Errorf("size %v → coverage %v → size %v", m, c.coverage(m), back)
		}
	}
	if !math.IsInf(c.sizeAt(1), 1) {
		t.Error("full coverage reached")
	}
	// Coverage rises with size.
	prev := 0.0
	for m := 1.0; m <= 2*n; m++ {
		v := c.coverage(m)
		if v < prev-1e-15 {
			t.Fatalf("coverage falls at %v: %v < %v", m, v, prev)
		}
		prev = v
	}
}

func TestGridIsINEXTs(t *testing.T) {
	// iNEXT 3.0.2's default sizes (knots 40, endpoint 2n) for the spider data and for a sample of 6.
	for _, c := range []struct {
		n    float64
		want []float64
	}{
		{168, []float64{1, 10, 19, 28, 37, 47, 56, 65, 74, 84, 93, 102, 111, 120, 130, 139, 148, 157, 167, 168, 169, 177, 186, 195, 204, 212, 221, 230, 239, 248, 256, 265, 274, 283, 292, 300, 309, 318, 327, 336}},
		{252, []float64{1, 14, 28, 42, 56, 70, 84, 98, 112, 126, 139, 153, 167, 181, 195, 209, 223, 237, 251, 252, 253, 266, 279, 292, 305, 319, 332, 345, 358, 371, 385, 398, 411, 424, 437, 451, 464, 477, 490, 504}},
		{6, []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}},
	} {
		got, _ := json.Marshal(Grid(c.n, 40, 0))
		want, _ := json.Marshal(c.want)
		if string(got) != string(want) {
			t.Errorf("n %v: %s, iNEXT %s", c.n, got, want)
		}
	}
	if l := Grid(252, 40, 126); len(l) != 40 || l[39] != 126 {
		t.Errorf("to n/2: %v", l)
	}
	// The curves' sizes: iNEXT's, and more spaced evenly on a logarithmic scale.
	c := CurveSizes(1e6, 40, 0)
	at := map[float64]bool{}
	for _, v := range c {
		at[v] = true
	}
	for _, v := range Grid(1e6, 40, 0) {
		if !at[v] {
			t.Errorf("iNEXT's size %v missing", v)
		}
	}
	if len(c) != 58 || !at[2] || !at[10] || !at[98] {
		t.Errorf("%d sizes: %v", len(c), c[:12])
	}
	if small := CurveSizes(1e6, 4, 0); len(small) != 4 {
		t.Errorf("four knots: %v", small)
	}
}

func TestRarefyIsReproducibleAndBounded(t *testing.T) {
	opt := DefaultOptions()
	opt.Replicates, opt.Stream = 50, 7
	lv := Levels{Sizes: []float64{100, 400}, Coverages: []float64{0.8, 0.95, 0.999}}
	a := Rarefy(logged, lv, opt)
	before := runtime.GOMAXPROCS(1)
	b := Rarefy(logged, lv, opt)
	runtime.GOMAXPROCS(before)
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	if string(ja) != string(jb) {
		t.Fatal("different curves on one processor")
	}
	if len(a.BySize) <= 40 || len(a.AtSize) != 2 || len(a.AtCoverage) != 3 || len(a.ByCoverage) == 0 {
		t.Fatalf("points: %d %d %d %d", len(a.BySize), len(a.AtSize), len(a.ByCoverage), len(a.AtCoverage))
	}
	for _, p := range append(a.BySize, a.AtSize...) {
		for q, v := range p.Hill {
			if p.Size > 1 && (math.IsNaN(float64(v.SE)) || v.Lower > v.Estimate || v.Upper < v.Estimate || v.Lower < 1) {
				t.Errorf("size %v q %d: %+v", p.Size, q, v)
			}
		}
		if p.Coverage.Lower < 0 || p.Coverage.Upper > 1 {
			t.Errorf("size %v coverage %+v", p.Size, p.Coverage)
		}
	}
	// At the sample's own coverage, the values seen.
	var own Point
	for _, p := range a.BySize {
		if p.Method == "observed" {
			own = p
		}
	}
	cOwn := Rarefy(logged, Levels{Coverages: []float64{float64(own.Coverage.Estimate)}}, Options{})
	if p := cOwn.AtCoverage[0]; float64(p.Size) != a.Abundance || float64(own.Size) != a.Abundance || p.Method != "observed" || p.Hill[1].Estimate != own.Hill[1].Estimate {
		t.Errorf("at its own coverage: %+v", p)
	}
	// Full coverage is out of reach: said so, and written with its size missing.
	out := Rarefy(logged, Levels{Knots: 4, Coverages: []float64{1}}, opt)
	raw, err := json.Marshal(out)
	if p := out.AtCoverage[0]; err != nil || p.Method != "unreachable" || !math.IsNaN(float64(p.Size)) || !math.IsNaN(float64(p.Hill[0].Estimate)) {
		t.Errorf("unreachable: %v %+v", err, p)
	}
	if !strings.Contains(string(raw), `"size":null,"method":"unreachable"`) {
		t.Errorf("written as %s", raw)
	}
	// A sample of fractions, or of one cell, is not rarefied.
	if r := Rarefy([]float64{0.5, 2}, lv, opt); r.Whole || r.BySize != nil {
		t.Errorf("fractions: %+v", r)
	}
}

func TestSuggestedLevels(t *testing.T) {
	s, ok := Suggest([][]float64{{0.5}, girdled, logged, nil})
	if !ok || s.Samples != 2 || s.Size != 168 || s.SizeOf != 1 || s.ExtrapolatedSize != 2*168 {
		t.Fatalf("%+v", s)
	}
	cg, cl := curveOf(girdled), curveOf(logged)
	// Girdled's coverage is 0.929 and Logged's 0.945 (iNEXT prints them).
	if s.Coverage != cg.coverage(cg.n) || s.CoverageOf != 1 {
		t.Errorf("coverage %v of %d, want Girdled's %v", s.Coverage, s.CoverageOf, cg.coverage(cg.n))
	}
	if want := min(cg.coverage(2*cg.n), cl.coverage(2*cl.n)); s.ExtrapolatedCoverage != want {
		t.Errorf("extrapolated coverage %v, want %v", s.ExtrapolatedCoverage, want)
	}
	if _, ok := Suggest([][]float64{{0.5}}); ok {
		t.Error("suggested for fractions")
	}
}

// A repertoire of a million cells: the curves within seconds.
func BenchmarkRarefyAMillionCells(b *testing.B) {
	r := random.New(5, 0)
	var x []float64
	total := 0.0
	for total < 1e6 {
		v := math.Floor(math.Pow(1-r.Float64(), -1/1.2))
		x = append(x, v)
		total += v
	}
	opt := DefaultOptions()
	opt.Replicates = 50
	b.ResetTimer()
	for range b.N {
		Rarefy(x, Levels{}, opt)
	}
}
