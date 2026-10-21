package diversity

import (
	"math"
	"runtime"
	"sort"
	"sync"

	"clonoloom/internal/random"
)

// Options are a measurement's: the bootstrap's replicates (Chao et al. 2014 use 200), its seed
// and stream (a sample's own, so that each sample's replicates are its own and the same in every
// run), and the intervals' confidence.
type Options struct {
	Replicates int     `json:"replicates"`
	Seed       uint64  `json:"seed"`
	Stream     uint64  `json:"stream"`
	Confidence float64 `json:"confidence"`
}

// DefaultOptions: 200 replicates, 95% intervals.
func DefaultOptions() Options {
	return Options{Replicates: 200, Seed: 20140101, Confidence: 0.95}
}

// Value is a measure's estimate, its standard error and its interval.
type Value struct {
	Estimate Number `json:"estimate"`
	SE       Number `json:"se"`
	Lower    Number `json:"lower"`
	Upper    Number `json:"upper"`
}

// Result is a sample's diversity.
type Result struct {
	// Abundance: the sample's, in its unit; Clonotypes: those seen, with Singletons and
	// Doubletons among them.
	Abundance  float64 `json:"abundance"`
	Clonotypes int     `json:"clonotypes"`
	Singletons int     `json:"singletons"`
	Doubletons int     `json:"doubletons"`
	// Whole: every abundance a whole number. The estimated measures and the bootstrap need
	// counts; without them only the observed measures are given.
	Whole      bool             `json:"whole"`
	Measures   map[string]Value `json:"measures"`
	Replicates int              `json:"replicates"`
}

// bounds: where each measure's interval is clamped (a share between 0 and 1, a Hill number at
// least 1, …).
func bounds(id string, clonotypes float64) (lo, hi float64) {
	switch id {
	case "simpson", "giniSimpson", "evenness", "clonality", "simpsonClonality", "gini", "top10", "top100", "top1000", "maxFrequency", "coverage":
		return 0, 1
	case "shannon":
		return 0, math.Inf(1)
	case "chao1", "ace":
		return clonotypes, math.Inf(1)
	case "richness":
		return 0, math.Inf(1)
	}
	if len(id) > 12 && id[:12] == "homeostasis." {
		return 0, 1
	}
	return 1, math.Inf(1)
}

// Measure measures a sample's diversity from its clonotypes' abundances.
func Measure(abundances []float64, opt Options) Result {
	f := FreqOf(abundances)
	st := statsOf(f)
	res := Result{Abundance: st.n, Clonotypes: st.s, Singletons: int(st.f1), Doubletons: int(st.f2), Whole: st.whole, Measures: map[string]Value{}}
	if st.s == 0 {
		return res
	}
	point := map[string]float64{}
	observed(f, st, point)
	if st.whole {
		estimated(f, st, point)
	}
	ids := make([]string, 0, len(point))
	for id := range point {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if opt.Confidence <= 0 || opt.Confidence >= 1 {
		opt.Confidence = 0.95
	}
	z := normalQuantile(0.5 + opt.Confidence/2)
	replicates := [][]float64(nil)
	if st.whole && opt.Replicates > 1 && st.n > 0 {
		replicates = bootstrap(f, st, ids, opt)
		res.Replicates = opt.Replicates
	}
	for k, id := range ids {
		v := Value{Estimate: Number(point[id]), SE: Number(math.NaN()), Lower: Number(math.NaN()), Upper: Number(math.NaN())}
		if replicates != nil {
			var sum, n float64
			for _, r := range replicates {
				if x := r[k]; !math.IsNaN(x) && !math.IsInf(x, 0) {
					sum += x
					n++
				}
			}
			if n > 1 {
				mean := sum / n
				var ss float64
				for _, r := range replicates {
					if x := r[k]; !math.IsNaN(x) && !math.IsInf(x, 0) {
						ss += (x - mean) * (x - mean)
					}
				}
				se := math.Sqrt(ss / (n - 1))
				lo, hi := bounds(id, float64(st.s))
				// The asymptotic Shannon diversity is at least that seen in the sample, which is
				// biased low (iNEXT's floor). iNEXT floors the Simpson diversity too, but the one seen
				// is biased high, and deep samples see it just below the estimate: the floor cut the
				// interval's lower half away, and in simulations its intervals held the truth in under
				// half the samples (validation/coverage_test.go). ClonoLoom does not floor it.
				if id == "hill1Estimated" {
					lo = max(lo, point["hill1"])
				}
				v.SE = Number(se)
				v.Lower = Number(min(max(point[id]-z*se, lo), hi))
				v.Upper = Number(max(min(point[id]+z*se, hi), lo))
			}
		}
		if math.IsNaN(point[id]) {
			v.Lower, v.Upper = Number(math.NaN()), Number(math.NaN())
		}
		res.Measures[id] = v
	}
	// Chao1's interval is its own: from its variance, log-transformed (iNEXT's and immunarch's).
	if st.whole {
		est, se, lo, hi := chao1(st, z, f)
		res.Measures["chao1"] = Value{Estimate: Number(est), SE: Number(se), Lower: Number(lo), Upper: Number(hi)}
	}
	return res
}

// bootstrap draws replicates of the sample from its estimated repertoire (Chao et al. 2014,
// Appendix G): the clonotypes seen, their frequencies tuned down for having been seen
// (p̂ = (X/n)(1 − λ(1 − X/n)^n), λ = (1 − Ĉ)/Σ (X/n)(1 − X/n)^n), and ⌈f̂0⌉ clonotypes not seen,
// each (1 − Ĉ)/⌈f̂0⌉; each replicate a multinomial sample of the same size, every measure
// computed again from it. Each replicate has its own random stream and its own slot, so the
// result is the same however many processors draw them.
func bootstrap(f Freq, st stats, ids []string, opt Options) [][]float64 {
	n := st.n
	cHat := coverage(st)
	if math.IsNaN(cHat) || cHat > 1 {
		cHat = 1
	}
	unseen := 0
	if cHat < 1 {
		unseen = int(math.Ceil(f0(st)))
	}
	denominator := 0.0
	for _, g := range f {
		x := g.Abundance / n
		denominator += float64(g.Count) * x * math.Exp(n*math.Log1p(-x))
	}
	lambda := 0.0
	if denominator > 0 && cHat < 1 {
		lambda = (1 - cHat) / denominator
	}
	chances := make([]float64, 0, st.s+unseen)
	for _, g := range f {
		x := g.Abundance / n
		p := x * (1 - lambda*math.Exp(n*math.Log1p(-x)))
		for range g.Count {
			chances = append(chances, max(p, 0))
		}
	}
	for range unseen {
		chances = append(chances, (1-cHat)/float64(unseen))
	}
	draws := int(math.Round(n))
	out := make([][]float64, opt.Replicates)
	work := make(chan int)
	var wg sync.WaitGroup
	for range max(1, runtime.GOMAXPROCS(0)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var counts []int
			var hist []int
			var seen []int
			values := map[string]float64{}
			for b := range work {
				r := random.New(opt.Seed, opt.Stream<<24|uint64(b))
				counts = r.Multinomial(draws, chances, counts)
				// The replicate's frequency counts, through a histogram reused between replicates.
				seen = seen[:0]
				for _, c := range counts {
					if c == 0 {
						continue
					}
					if c >= len(hist) {
						hist = append(hist, make([]int, c-len(hist)+1)...)
					}
					if hist[c] == 0 {
						seen = append(seen, c)
					}
					hist[c]++
				}
				sort.Ints(seen)
				rf := make(Freq, len(seen))
				for i, c := range seen {
					rf[i].Abundance, rf[i].Count = float64(c), hist[c]
					hist[c] = 0
				}
				rst := statsOf(rf)
				clear(values)
				observed(rf, rst, values)
				estimated(rf, rst, values)
				row := make([]float64, len(ids))
				for k, id := range ids {
					v, ok := values[id]
					if !ok {
						v = math.NaN()
					}
					row[k] = v
				}
				out[b] = row
			}
		}()
	}
	for b := range opt.Replicates {
		work <- b
	}
	close(work)
	wg.Wait()
	return out
}

// normalQuantile is the standard normal distribution's quantile function (Acklam's rational
// approximation, refined by one step of Halley's method: accurate to about 1e-15).
func normalQuantile(p float64) float64 {
	if p <= 0 || p >= 1 {
		return math.NaN()
	}
	a := []float64{-3.969683028665376e+01, 2.209460984245205e+02, -2.759285104469687e+02, 1.383577518672690e+02, -3.066479806614716e+01, 2.506628277459239e+00}
	b := []float64{-5.447609879822406e+01, 1.615858368580409e+02, -1.556989798598866e+02, 6.680131188771972e+01, -1.328068155288572e+01}
	c := []float64{-7.784894002430293e-03, -3.223964580411365e-01, -2.400758277161838e+00, -2.549732539343734e+00, 4.374664141464968e+00, 2.938163982698783e+00}
	d := []float64{7.784695709041462e-03, 3.224671290700398e-01, 2.445134137142996e+00, 3.754408661907416e+00}
	const low = 0.02425
	var x float64
	switch {
	case p < low:
		q := math.Sqrt(-2 * math.Log(p))
		x = (((((c[0]*q+c[1])*q+c[2])*q+c[3])*q+c[4])*q + c[5]) / ((((d[0]*q+d[1])*q+d[2])*q+d[3])*q + 1)
	case p > 1-low:
		q := math.Sqrt(-2 * math.Log(1-p))
		x = -(((((c[0]*q+c[1])*q+c[2])*q+c[3])*q+c[4])*q + c[5]) / ((((d[0]*q+d[1])*q+d[2])*q+d[3])*q + 1)
	default:
		q := p - 0.5
		r := q * q
		x = (((((a[0]*r+a[1])*r+a[2])*r+a[3])*r+a[4])*r + a[5]) * q / (((((b[0]*r+b[1])*r+b[2])*r+b[3])*r+b[4])*r + 1)
	}
	// One Halley step against the normal CDF (from math.Erfc).
	e := 0.5*math.Erfc(-x/math.Sqrt2) - p
	u := e * math.Sqrt(2*math.Pi) * math.Exp(x*x/2)
	return x - u/(1+x*u/2)
}
