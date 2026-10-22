package diversity

// Rarefaction and extrapolation (clonoloom-spec/requirements.md, R2 and Q3): the Hill numbers of
// order 0, 1 and 2 a sample would show at another size, smaller (interpolated, the expectation of
// a subsample drawn without replacement) or larger (extrapolated, toward the estimated
// repertoire), and at another sample coverage, so that samples of different depths are compared
// at equal size or, better, at equal completeness (Chao and Jost 2012; Chao et al. 2014). Every
// value has a bootstrap interval, from the same replicates Measure draws.
//
// The interpolated Shannon diversity needs the expected Y ln Y of each clonotype in a subsample
// (Y hypergeometric). iNEXT sums every subsample count from 1 to m for each, which takes too long
// for repertoires of a million cells; each sum here starts at the distribution's mode and stops
// where its terms no longer count, the same expectation with a few terms. The formulas, and where
// iNEXT's results differ, are in clonoloom-spec/research.md §5.4.

import (
	"math"
	"sort"
)

// Orders are the Hill numbers the curves give: 0 (clonotypes), 1 (Shannon), 2 (Simpson).
var Orders = []float64{0, 1, 2}

// Point is one point of a curve: a sample size, how its values were found (interpolated,
// observed, extrapolated; unreachable, its size missing, for a coverage the sample cannot reach),
// the sample coverage there and the Hill numbers of each order.
type Point struct {
	Size     Number  `json:"size"`
	Method   string  `json:"method"`
	Coverage Value   `json:"coverage"`
	Hill     []Value `json:"hill"`
}

// Curves are a sample's rarefaction and extrapolation curves. BySize are the values at sizes from
// 1 to the endpoint (CurveSizes); ByCoverage the values where the sample reaches each coverage
// BySize does (the
// size found for each, the interval from each replicate's own size, iNEXT's "unconditional"
// variance). AtSize and AtCoverage are the same at the levels asked, to compare samples.
type Curves struct {
	Abundance  float64 `json:"abundance"`
	Clonotypes int     `json:"clonotypes"`
	Whole      bool    `json:"whole"`
	BySize     []Point `json:"bySize"`
	ByCoverage []Point `json:"byCoverage"`
	AtSize     []Point `json:"atSize"`
	AtCoverage []Point `json:"atCoverage"`
	Replicates int     `json:"replicates"`
}

// Levels are where the curves are computed: Knots points from size 1 to Endpoint (0: twice the
// sample, as Chao et al. 2014 advise), and the sizes and coverages the samples are compared at.
type Levels struct {
	Knots     int       `json:"knots"`
	Endpoint  float64   `json:"endpoint"`
	Sizes     []float64 `json:"sizes"`
	Coverages []float64 `json:"coverages"`
}

// DefaultKnots is iNEXT's: about half the points interpolated, half extrapolated.
const DefaultKnots = 40

// Grid is the sizes along a curve, as iNEXT spaces them: Knots/2 − 1 evenly from 1 to n − 1, n
// itself and Knots/2 evenly from n + 1 to the endpoint (or Knots evenly to an endpoint no larger
// than n), each rounded down, the first replaced by 1.
func Grid(n float64, knots int, endpoint float64) []float64 {
	if knots < 4 {
		knots = DefaultKnots
	}
	if endpoint <= 0 {
		endpoint = 2 * n
	}
	var m []float64
	if endpoint <= n {
		m = seq(1, endpoint, knots)
	} else {
		m = append(seq(1, n-1, knots/2-1), n)
		m = append(m, seq(n+1, endpoint, knots/2)...)
	}
	for i := range m {
		m[i] = math.Floor(m[i])
	}
	m[0] = 1
	out := m[:0]
	for i, v := range m {
		if i == 0 || v != out[len(out)-1] {
			out = append(out, v)
		}
	}
	sort.Float64s(out)
	return out
}

// CurveSizes are the sizes a curve is computed at: iNEXT's (Grid), and Knots/2 − 2 more spaced
// evenly on a logarithmic scale from 1 to the endpoint, so that the curve is drawn as well on a
// logarithmic axis (repertoires' sizes span orders of magnitude, and iNEXT's first step is a
// twentieth of the sample).
func CurveSizes(n float64, knots int, endpoint float64) []float64 {
	g := Grid(n, knots, endpoint)
	end := g[len(g)-1]
	k := max(knots, 4) / 2
	if knots < 4 {
		k = DefaultKnots / 2
	}
	for i := 1; i < k-1; i++ {
		g = append(g, math.Round(math.Exp(math.Log(end)*float64(i)/float64(k-1))))
	}
	sort.Float64s(g)
	out := g[:0]
	for i, v := range g {
		if i == 0 || v != out[len(out)-1] {
			out = append(out, v)
		}
	}
	return out
}

// seq is R's seq(from, to, length.out = k), computed as R computes it.
func seq(from, to float64, k int) []float64 {
	switch {
	case k <= 0:
		return nil
	case k == 1:
		return []float64{from}
	case k == 2:
		return []float64{from, to}
	}
	if from == to {
		out := make([]float64, k)
		for i := range out {
			out[i] = from
		}
		return out
	}
	n1 := float64(k - 1)
	out := []float64{from}
	for i := 1; i < k-1; i++ {
		out = append(out, from+float64(i)*((to-from)/n1))
	}
	return append(out, to)
}

// curve holds what a sample's curves are computed from.
type curve struct {
	f   Freq
	st  stats
	n   float64
	obs [3]float64 // the Hill numbers seen
	asy [3]float64 // the whole repertoire's: Chao1, the asymptotic Shannon and Simpson diversity
	// beta: how fast the extrapolated values approach the asymptote, from the values at n − 1.
	beta [3]float64
	p2   float64 // ΣX(X−1)/(n(n−1)): the chance two cells drawn without replacement match
	a    float64 // the coverage deficit's ratio from one size to the next (Chao and Jost 2012)
}

func newCurve(f Freq, st stats) *curve {
	c := &curve{f: f, st: st, n: st.n}
	n := st.n
	c.obs = [3]float64{float64(st.s), hill(f, n, 1), hill(f, n, 2)}
	c.p2 = st.sumXX1 / (n * (n - 1))
	c.asy = [3]float64{float64(st.s) + f0(st), math.Exp(shannonAsymptotic(f, st)), 1 / c.p2}
	if st.f1 > 0 {
		f0 := f0(st)
		c.a = n * f0 / (n*f0 + st.f1)
	} else {
		c.a = 1
	}
	before := c.interpolated(n - 1)
	for q := range 2 {
		if c.asy[q] != before[q] {
			c.beta[q] = (c.obs[q] - before[q]) / (c.asy[q] - before[q])
		}
	}
	return c
}

// hill is the Hill numbers at size m: interpolated below n (between the whole sizes around a
// size that is not whole, as iNEXT does), extrapolated above.
func (c *curve) hill(m float64) [3]float64 {
	switch {
	case m == c.n:
		return c.obs
	case m > c.n:
		var out [3]float64
		for q := range 2 {
			out[q] = c.obs[q] + (c.asy[q]-c.obs[q])*(1-math.Pow(1-c.beta[q], m-c.n))
		}
		out[2] = 1 / (1/m + (1-1/m)*c.p2)
		return out
	case m == math.Floor(m):
		return c.interpolated(m)
	}
	lo, hi := c.hill(math.Floor(m)), c.hill(math.Ceil(m))
	w := m - math.Floor(m)
	return [3]float64{lo[0]*(1-w) + hi[0]*w, lo[1]*(1-w) + hi[1]*w, lo[2]*(1-w) + hi[2]*w}
}

// interpolated is the expected Hill numbers of a subsample of whole size m < n drawn without
// replacement (Chao et al. 2014, Eqs. 4–6): the clonotypes, Σ (1 − P(none of X)); the Shannon
// diversity, exp(ln m − Σ E[Y ln Y]/m); the Simpson diversity, 1/(1/m + (1 − 1/m)·p2).
func (c *curve) interpolated(m float64) [3]float64 {
	var s, ylogy float64
	for _, g := range c.f {
		k := float64(g.Count)
		s += k * -math.Expm1(logNone(c.n, g.Abundance, m))
		ylogy += k * meanYLogY(c.n, g.Abundance, m)
	}
	return [3]float64{s, math.Exp(math.Log(m) - ylogy/m), 1 / (1/m + (1-1/m)*c.p2)}
}

// coverage is the sample coverage at size m: below n, the expected share of the repertoire's
// cells whose clonotype a subsample of size m holds, 1 − Σ (X/n)·C(n−X, m)/C(n−1, m); at n and
// above, 1 − (f1/n)·A^(m−n+1) (Chao and Jost 2012). Between whole sizes below n, linearly, as
// iNEXT does.
func (c *curve) coverage(m float64) float64 {
	n := c.n
	switch {
	case m >= n:
		return 1 - c.st.f1/n*math.Pow(c.a, m-n+1)
	case m <= 0:
		return 0
	case m != math.Floor(m):
		w := m - math.Floor(m)
		return c.coverage(math.Floor(m))*(1-w) + c.coverage(math.Ceil(m))*w
	}
	missed := 0.0
	for _, g := range c.f {
		if g.Abundance <= n-m {
			missed += float64(g.Count) * g.Abundance / n * math.Exp(logNone(n-1, g.Abundance-1, m))
		}
	}
	return 1 - missed
}

// sizeAt is the size at which the sample reaches coverage target: below the sample's own coverage,
// by bisection over the whole sizes, then linearly between them; above it, from the extrapolated
// coverage's formula. No smaller than 1, as iNEXT has it; +Inf where the target is out of reach.
func (c *curve) sizeAt(target float64) float64 {
	n := c.n
	own := c.coverage(n)
	switch {
	case target == own:
		return n
	case target > own:
		if target >= 1 || c.a <= 0 || c.a >= 1 || c.st.f1 == 0 {
			return math.Inf(1)
		}
		return n - 1 + (math.Log(n/c.st.f1)+math.Log(1-target))/math.Log(c.a)
	}
	lo, hi := 0.0, n // coverage(lo) ≤ target < coverage(hi)
	for hi-lo > 1 {
		mid := math.Floor((lo + hi) / 2)
		if c.coverage(mid) <= target {
			lo = mid
		} else {
			hi = mid
		}
	}
	cl, ch := c.coverage(lo), c.coverage(hi)
	m := lo
	if ch > cl {
		m += (target - cl) / (ch - cl)
	}
	return max(m, 1)
}

// method says how a size's values are found.
func (c *curve) method(m float64) string {
	switch {
	case m < c.n:
		return "interpolated"
	case m > c.n:
		return "extrapolated"
	}
	return "observed"
}

// SeenChance is the chance that a subsample of m cells, drawn without replacement from a sample of
// n, holds a clonotype of x cells: 1 − C(n−x, m)/C(n, m).
func SeenChance(n, x, m float64) float64 {
	if x <= 0 || m <= 0 {
		return 0
	}
	if m >= n {
		return 1
	}
	return -math.Expm1(logNone(n, x, m))
}

// logNone is ln P(none of a clonotype's x cells among m drawn without replacement from n),
// ln[C(n−x, m)/C(n, m)]: −Inf where x + m > n. Symmetric in x and m; a product of the smaller's
// terms where it is small, else by lgammaRatio, without subtracting large logarithms.
func logNone(n, x, m float64) float64 {
	if x <= 0 || m <= 0 {
		return 0
	}
	if x+m > n {
		return math.Inf(-1)
	}
	a, b := min(x, m), max(x, m)
	if a <= 32 {
		s := 0.0
		for j := 0.0; j < a; j++ {
			s += math.Log1p(-b / (n - j))
		}
		return s
	}
	return lgammaRatio(n-a-b+1, a) - lgammaRatio(n-a+1, a)
}

// lgammaRatio is ln Γ(z+a) − ln Γ(z): by Stirling's series where z is large, so that the two
// large logarithms never meet.
func lgammaRatio(z, a float64) float64 {
	if z < 30 {
		hi, _ := math.Lgamma(z + a)
		lo, _ := math.Lgamma(z)
		return hi - lo
	}
	w := z + a
	return (w-0.5)*math.Log1p(a/z) + a*math.Log(z) - a + stirling(w) - stirling(z)
}

// stirling is ln Γ(z) − [(z − ½) ln z − z + ½ ln 2π], z ≥ 30, to double precision.
func stirling(z float64) float64 {
	r := 1 / z
	r2 := r * r
	return r * (1.0/12 - r2*(1.0/360-r2*(1.0/1260-r2*(1.0/1680-r2*(1.0/1188)))))
}

// meanYLogY is E[Y ln Y] for Y the cells of a clonotype of x among m drawn without replacement
// from n (hypergeometric): its probabilities, relative to the mode's, from the ratio of each to
// the next, summed outward from the mode until they no longer count.
func meanYLogY(n, x, m float64) float64 {
	lo, hi := max(0, m+x-n), min(x, m)
	if hi <= 1 {
		return 0 // Y is 0 or 1, and 1·ln 1 = 0
	}
	mode := min(max(math.Floor((m+1)*(x+1)/(n+2)), lo), hi)
	sum, weighted := 1.0, ylny(mode)
	w := 1.0
	for k := mode; k < hi; k++ {
		w *= (x - k) * (m - k) / ((k + 1) * (n - x - m + k + 1))
		if w < 1e-18*sum {
			break
		}
		sum += w
		weighted += w * ylny(k+1)
	}
	w = 1
	for k := mode; k > lo; k-- {
		w *= k * (n - x - m + k) / ((x - k + 1) * (m - k + 1))
		if w < 1e-18*sum {
			break
		}
		sum += w
		weighted += w * ylny(k-1)
	}
	return weighted / sum
}

func ylny(y float64) float64 {
	if y <= 1 {
		return 0
	}
	return y * math.Log(y)
}

// Rarefy computes a sample's curves at the levels asked, each value with its bootstrap interval
// (opt as Measure's: with the same seed and stream, the same replicates). Abundances must be whole
// numbers; otherwise, and for samples of fewer than two cells, only the totals are given.
func Rarefy(abundances []float64, lv Levels, opt Options) Curves {
	f := FreqOf(abundances)
	st := statsOf(f)
	res := Curves{Abundance: st.n, Clonotypes: st.s, Whole: st.whole}
	if !st.whole || st.n < 2 {
		return res
	}
	if opt.Confidence <= 0 || opt.Confidence >= 1 {
		opt.Confidence = 0.95
	}
	z := normalQuantile(0.5 + opt.Confidence/2)
	sizes := CurveSizes(st.n, lv.Knots, lv.Endpoint)
	c := newCurve(f, st)
	// The coverages the sample reaches along its curve, each once.
	var coverages []float64
	for _, m := range sizes {
		if v := c.coverage(m); len(coverages) == 0 || v != coverages[len(coverages)-1] {
			coverages = append(coverages, v)
		}
	}
	// Every value as one row: by size (coverage and the three orders), by coverage (the three
	// orders), at the sizes asked, at the coverages asked.
	row := func(c *curve) []float64 {
		var out []float64
		for _, sz := range [][]float64{sizes, lv.Sizes} {
			for _, m := range sz {
				h := c.hill(m)
				out = append(out, c.coverage(m), h[0], h[1], h[2])
			}
		}
		for _, cv := range [][]float64{coverages, lv.Coverages} {
			for _, target := range cv {
				h := [3]float64{math.NaN(), math.NaN(), math.NaN()}
				if m := c.sizeAt(target); !math.IsInf(m, 0) {
					h = c.hill(m)
				}
				out = append(out, h[0], h[1], h[2])
			}
		}
		return out
	}
	point := row(c)
	var replicates [][]float64
	if opt.Replicates > 1 {
		replicates = bootstrap(f, st, opt, func(rf Freq) []float64 {
			rst := statsOf(rf)
			if rst.n < 2 {
				return nil
			}
			return row(newCurve(rf, rst))
		})
		kept := replicates[:0]
		for _, r := range replicates {
			if r != nil {
				kept = append(kept, r)
			}
		}
		replicates = kept
		res.Replicates = opt.Replicates
	}
	k := 0
	value := func(lo, hi float64) Value {
		v := Value{Estimate: Number(point[k]), SE: Number(math.NaN()), Lower: Number(math.NaN()), Upper: Number(math.NaN())}
		if replicates != nil && !math.IsNaN(point[k]) {
			if se := spread(replicates, k); !math.IsNaN(se) {
				v.SE = Number(se)
				v.Lower = Number(min(max(point[k]-z*se, lo), hi))
				v.Upper = Number(max(min(point[k]+z*se, hi), lo))
			}
		}
		k++
		return v
	}
	hills := func() []Value {
		return []Value{value(1, math.Inf(1)), value(1, math.Inf(1)), value(1, math.Inf(1))}
	}
	for _, sz := range []*[]Point{&res.BySize, &res.AtSize} {
		list := sizes
		if sz == &res.AtSize {
			list = lv.Sizes
		}
		for _, m := range list {
			p := Point{Size: Number(m), Method: c.method(m), Coverage: value(0, 1)}
			p.Hill = hills()
			*sz = append(*sz, p)
		}
	}
	for _, cv := range []*[]Point{&res.ByCoverage, &res.AtCoverage} {
		list := coverages
		if cv == &res.AtCoverage {
			list = lv.Coverages
		}
		for _, target := range list {
			m := c.sizeAt(target)
			p := Point{Size: Number(m), Method: c.method(m), Coverage: Value{Estimate: Number(target), SE: Number(math.NaN()), Lower: Number(math.NaN()), Upper: Number(math.NaN())}}
			if math.IsInf(m, 0) {
				p.Size, p.Method = Number(math.NaN()), "unreachable"
			}
			p.Hill = hills()
			*cv = append(*cv, p)
		}
	}
	return res
}

// SuggestedLevels are where a cohort's samples are compared. Size and Coverage need rarefaction
// only: the smallest sample's size, and the lowest coverage any sample has. ExtrapolatedSize and
// ExtrapolatedCoverage are iNEXT's defaults (after Chao et al. 2014 and Chao and Jost 2012): the
// smallest size, and the lowest coverage, any sample reaches at twice its size. In ClonoLoom's
// simulations of repertoires with many rare clonotypes, values extrapolated that far were off by
// 1–3% while their intervals were narrower still (research.md §5.4), so the rarefied levels come
// first. Samples that cannot be rarefied (not whole, fewer than two cells) are left out; with none
// left, nothing is suggested. SizeOf and CoverageOf are the samples that set Size and Coverage.
type SuggestedLevels struct {
	Size                 float64 `json:"size"`
	Coverage             float64 `json:"coverage"`
	ExtrapolatedSize     float64 `json:"extrapolatedSize"`
	ExtrapolatedCoverage float64 `json:"extrapolatedCoverage"`
	SizeOf               int     `json:"sizeOf"`
	CoverageOf           int     `json:"coverageOf"`
	Samples              int     `json:"samples"`
}

func Suggest(samples [][]float64) (SuggestedLevels, bool) {
	inf := math.Inf(1)
	out := SuggestedLevels{Size: inf, Coverage: inf, ExtrapolatedSize: inf, ExtrapolatedCoverage: inf, SizeOf: -1, CoverageOf: -1}
	for i, x := range samples {
		f := FreqOf(x)
		st := statsOf(f)
		if !st.whole || st.n < 2 {
			continue
		}
		out.Samples++
		c := &curve{f: f, st: st, n: st.n, a: 1}
		if st.f1 > 0 {
			f0 := f0(st)
			c.a = st.n * f0 / (st.n*f0 + st.f1)
		}
		if st.n < out.Size {
			out.Size, out.SizeOf = st.n, i
		}
		if v := c.coverage(st.n); v < out.Coverage {
			out.Coverage, out.CoverageOf = v, i
		}
		out.ExtrapolatedSize = min(out.ExtrapolatedSize, 2*st.n)
		out.ExtrapolatedCoverage = min(out.ExtrapolatedCoverage, c.coverage(2*st.n))
	}
	return out, out.Samples > 0
}
