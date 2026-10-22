// Package diversity measures a sample's diversity from its clonotypes' abundances
// (clonoloom-spec/requirements.md, R1 and R5): the observed measures (Hill numbers and their
// profile, Shannon's entropy, Simpson's index, evenness and clonality, Gini's coefficient, the
// largest clonotypes' shares, clonal space homeostasis) and the estimated ones (Chao1, ACE, and
// the asymptotic Hill numbers of order 1 and 2), each with an interval: Chao1's from its
// variance, the others' from a bootstrap of the sample (Chao et al. 2014, Appendix G).
//
// Every measure is computed from the sample's frequency counts (how many clonotypes were seen
// once, twice, …), a few hundred numbers however many clonotypes there are, so that a bootstrap
// of two hundred replicates is quick. The formulas, their sources and where reference
// implementations differ are in clonoloom-spec/research.md §5.3.
package diversity

import (
	"encoding/json"
	"math"
	"sort"

	"clonoloom/internal/dmath"
)

// Number is a value that may be missing (NaN), written as null.
type Number float64

func (n Number) MarshalJSON() ([]byte, error) {
	f := float64(n)
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return []byte("null"), nil
	}
	return json.Marshal(f)
}

func (n *Number) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*n = Number(math.NaN())
		return nil
	}
	var f float64
	if err := json.Unmarshal(data, &f); err != nil {
		return err
	}
	*n = Number(f)
	return nil
}

// Freq is a sample's frequency counts: Count clonotypes seen Abundance times each, by abundance
// ascending.
type Freq []struct {
	Abundance float64
	Count     int
}

// FreqOf counts a sample's clonotypes by abundance (those with none are left out).
func FreqOf(abundances []float64) Freq {
	byValue := map[float64]int{}
	for _, a := range abundances {
		if a > 0 {
			byValue[a]++
		}
	}
	out := make(Freq, 0, len(byValue))
	for a, c := range byValue {
		out = append(out, struct {
			Abundance float64
			Count     int
		}{a, c})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Abundance < out[j].Abundance })
	return out
}

// stats are a sample's totals: its abundance n, its clonotypes S, f1 and f2, and whether every
// abundance is a whole number (the estimators need counts).
type stats struct {
	n         float64
	s         int
	f1, f2    float64
	whole     bool
	maxCount  float64
	sumXX1    float64 // Σ X(X−1)
	abundance float64
}

func statsOf(f Freq) stats {
	st := stats{whole: true}
	for _, g := range f {
		c := float64(g.Count)
		st.n += float64(g.Abundance * c)
		st.s += g.Count
		st.sumXX1 += float64(c * g.Abundance * (g.Abundance - 1))
		if g.Abundance != math.Trunc(g.Abundance) {
			st.whole = false
		}
		switch g.Abundance {
		case 1:
			st.f1 = c
		case 2:
			st.f2 = c
		}
		st.maxCount = max(st.maxCount, g.Abundance)
	}
	st.abundance = st.n
	return st
}

// Profile is the orders of the Hill numbers' profile.
var Profile = []float64{0, 0.25, 0.5, 0.75, 1, 1.25, 1.5, 1.75, 2, 2.5, 3, 3.5, 4}

// hill is the observed Hill number of order q (Hill 1973): (Σ p^q)^(1/(1−q)), its limit exp(H)
// at q = 1.
func hill(f Freq, n float64, q float64) float64 {
	if q == 1 {
		return dmath.Exp(shannon(f, n))
	}
	if q == 0 {
		s := 0
		for _, g := range f {
			s += g.Count
		}
		return float64(s)
	}
	sum := 0.0
	for _, g := range f {
		sum += float64(float64(g.Count) * dmath.Pow(g.Abundance/n, q))
	}
	return dmath.Pow(sum, 1/(1-q))
}

// shannon is Shannon's entropy, natural log.
func shannon(f Freq, n float64) float64 {
	h := 0.0
	for _, g := range f {
		p := g.Abundance / n
		h -= float64(float64(g.Count) * p * dmath.Log(p))
	}
	return h
}

// Homeostasis bins: the share of the sample's abundance in clonotypes of each frequency, as
// immunarch's clonal space homeostasis bins it (rare ≤ 10⁻⁵ < small ≤ 10⁻⁴ < medium ≤ 10⁻³ <
// large ≤ 10⁻² < hyperexpanded).
var Homeostasis = []struct {
	ID    string
	Upper float64
}{{"rare", 1e-5}, {"small", 1e-4}, {"medium", 1e-3}, {"large", 1e-2}, {"hyperexpanded", 1}}

// observed computes the observed measures of frequency counts, into out by measure id.
func observed(f Freq, st stats, out map[string]float64) {
	n := st.n
	s := float64(st.s)
	out["richness"] = s
	h := shannon(f, n)
	out["shannon"] = h
	out["hill1"] = dmath.Exp(h)
	lambda := 0.0
	for _, g := range f {
		p := g.Abundance / n
		lambda += float64(float64(g.Count) * p * p)
	}
	out["simpson"] = lambda
	out["giniSimpson"] = 1 - lambda
	out["hill2"] = 1 / lambda
	out["simpsonClonality"] = math.Sqrt(lambda)
	if st.s > 1 {
		out["evenness"] = h / dmath.Log(s)
		out["clonality"] = 1 - out["evenness"]
	} else {
		out["evenness"], out["clonality"] = math.NaN(), math.NaN()
	}
	// Gini's coefficient of the clonotypes' abundances, ascending: Σ (2i − S − 1) x_(i) / (S Σx);
	// a group of f equal abundances at ranks a+1…a+f adds x·f·(2a + f − S).
	g, a := 0.0, 0.0
	for _, grp := range f {
		c := float64(grp.Count)
		g += float64(grp.Abundance * c * (float64(2*a) + c - s))
		a += c
	}
	if st.s > 1 {
		out["gini"] = g / (s * n)
	} else {
		out["gini"] = 0
	}
	out["maxFrequency"] = st.maxCount / n
	// The largest clonotypes' shares.
	for _, top := range []struct {
		id string
		k  int
	}{{"top10", 10}, {"top100", 100}, {"top1000", 1000}} {
		left, share := top.k, 0.0
		for i := len(f) - 1; i >= 0 && left > 0; i-- {
			take := min(left, f[i].Count)
			share += float64(float64(take) * f[i].Abundance / n)
			left -= take
		}
		out[top.id] = share
	}
	for _, b := range Homeostasis {
		out["homeostasis."+b.ID] = 0
	}
	for _, grp := range f {
		p := grp.Abundance / n
		for k, b := range Homeostasis {
			if p <= b.Upper || k == len(Homeostasis)-1 {
				out["homeostasis."+b.ID] += float64(float64(grp.Count) * p)
				break
			}
		}
	}
	for _, q := range Profile {
		out[profileID(q)] = hill(f, n, q)
	}
}

func profileID(q float64) string {
	b, _ := json.Marshal(q)
	return "hill@" + string(b)
}

// coverage is the sample coverage estimate of Chao and Jost 2012 (iNEXT's), with its f2 = 0
// form.
func coverage(st stats) float64 {
	n, f1, f2 := st.n, st.f1, st.f2
	if n <= 0 {
		return math.NaN()
	}
	if f2 > 0 {
		return 1 - float64(f1/n*(float64((n-1)*f1)/(float64((n-1)*f1)+float64(2*f2))))
	}
	return 1 - float64(f1/n*(float64((n-1)*(f1-1))/(float64((n-1)*(f1-1))+2)))
}

// f0 is Chao1's estimate of the clonotypes not seen (Chao 1984; the bias-corrected form of Chao et
// al. 2014, Eq. B.5, with its f2 = 0 branch).
func f0(st stats) float64 {
	c := (st.n - 1) / st.n
	if st.f2 > 0 {
		return c * st.f1 * st.f1 / (2 * st.f2)
	}
	return c * st.f1 * (st.f1 - 1) / 2
}

// chao1 answers Chao1 with its interval: the variance as iNEXT and immunarch give it, the
// interval log-transformed on the clonotypes not seen, so it never falls below those seen. Where
// no clonotype is estimated unseen (none seen once; or one, and none twice), iNEXT's interval
// instead: the variance Σ f_k(e^−k − e^−2k) − (Σ k e^−k f_k)²/n, the interval
// S/(1−P) ± z·SE/(1−P) with P = Σ f_k e^−k / S, no lower than S (it may not hold S itself).
func chao1(st stats, z float64, f Freq) (est, se, lower, upper float64) {
	s := float64(st.s)
	f1, f2, n := st.f1, st.f2, st.n
	if f0(st) == 0 {
		var v, a, p float64
		for _, g := range f {
			c, k := float64(g.Count), g.Abundance
			v += float64(c * (dmath.Exp(-k) - dmath.Exp(-2*k)))
			a += float64(k * dmath.Exp(-k) * c)
			p += float64(c * dmath.Exp(-k))
		}
		v -= float64(a * a / n)
		p /= s
		se = math.Sqrt(max(v, 0))
		return s, se, max(s, s/(1-p)-float64(z*se)/(1-p)), s/(1-p) + float64(z*se)/(1-p)
	}
	c := (n - 1) / n
	est = s + f0(st)
	var v float64
	if f2 > 0 {
		r := f1 / f2
		v = f2 * (float64(c*r*r/2) + float64(c*c*r*r*r) + float64(c*c*r*r*r*r/4))
	} else {
		v = float64(c*f1*(f1-1)/2) + float64(c*c*f1*(float64(2*f1)-1)*(float64(2*f1)-1)/4) - float64(c*c*f1*f1*f1*f1/4/est)
	}
	t := est - s
	if t <= 0 || v <= 0 {
		return est, math.Sqrt(max(v, 0)), est, est
	}
	k := dmath.Exp(z * math.Sqrt(dmath.Log(1+v/(t*t))))
	return est, math.Sqrt(v), s + t/k, s + float64(t*k)
}

// ace is the abundance-based coverage estimator (Chao and Lee 1992) with the rare clonotypes
// those seen at most 10 times, as vegan's estimateR computes it. Where the rare clonotypes'
// coverage is zero (every rare clonotype a singleton) it is not estimable (NaN), as vegan says;
// with no rare clonotype, it is those seen.
func ace(f Freq, st stats) float64 {
	var sRare, sAbund, nRare, sumI float64
	for _, g := range f {
		c := float64(g.Count)
		if g.Abundance <= 10 {
			sRare += c
			nRare += float64(c * g.Abundance)
			sumI += float64(c * g.Abundance * (g.Abundance - 1))
		} else {
			sAbund += c
		}
	}
	s := float64(st.s)
	if sRare == 0 {
		return s
	}
	cAce := 1 - st.f1/nRare
	if cAce <= 0 {
		return math.NaN()
	}
	gamma := 0.0
	if nRare > 1 {
		gamma = max(float64(sRare/cAce*sumI/(nRare*(nRare-1)))-1, 0)
	}
	return max(s, sAbund+sRare/cAce+float64(st.f1/cAce*gamma))
}

// shannonAsymptotic is Chao, Wang and Jost's 2013 estimator of the entropy of the whole
// repertoire: Σ_{1≤X≤n−1} (X/n)(ψ(n) − ψ(X)) + (f1/n)(1−A)^(1−n)[−ln A − Σ_{r=1}^{n−1} (1−A)^r/r].
func shannonAsymptotic(f Freq, st stats) float64 {
	n := st.n
	psiN := digamma(n)
	h := 0.0
	for _, g := range f {
		if g.Abundance <= n-1 {
			h += float64(float64(g.Count) * g.Abundance / n * (psiN - digamma(g.Abundance)))
		}
	}
	var a float64
	switch {
	case st.f2 > 0:
		a = 2 * st.f2 / (float64((n-1)*st.f1) + float64(2*st.f2))
	case st.f1 > 0:
		a = 2 / (float64((n-1)*(st.f1-1)) + 2)
	default:
		a = 1
	}
	if st.f1 > 0 && a < 1 {
		// (1−A)^(1−n)[−ln A − Σ_{r<n} (1−A)^r/r] = (1−A)·Σ_{j≥0} (1−A)^j/(n+j).
		x := 1 - a
		h += float64(st.f1 / n * x * tail(x, n))
	}
	return h
}

// tail is Σ_{j≥0} x^j/(n+j), 0 ≤ x < 1: summed directly where its terms fall fast, else by the
// Euler–Maclaurin formula, e^{ns}E1(ns) + 1/(2n) + (s/n + 1/n²)/12 − (s³/n + 3s²/n² + 6s/n³ +
// 6/n⁴)/720 with s = −ln x.
func tail(x, n float64) float64 {
	s := -dmath.Log(x)
	if n < 50 || s > 0.5 {
		sum, term := 0.0, 1.0
		for j := 0.0; j < 1e8; j++ {
			add := term / (n + j)
			sum += add
			if add < sum*1e-17 {
				break
			}
			term *= x
		}
		return sum
	}
	return scaledE1(n*s) + 1/float64(2*n) + (s/n+1/float64(n*n))/12 - (float64(s*s*s/n)+float64(3*s*s/(n*n))+float64(6*s/(n*n*n))+6/float64(n*n*n*n))/720
}

// scaledE1 is e^z·E1(z), z > 0: by its series below 1, else by its continued fraction (Lentz).
func scaledE1(z float64) float64 {
	if z < 1 {
		const euler = 0.5772156649015329
		sum, term := 0.0, 1.0
		for k := 1.0; k < 200; k++ {
			term *= -z / k
			add := -term / k
			sum += add
			if math.Abs(add) < 1e-17*math.Abs(sum) {
				break
			}
		}
		return dmath.Exp(z) * (-euler - dmath.Log(z) + sum)
	}
	// e^z E1(z) = 1/(z+1− 1/(z+3− 4/(z+5− …))).
	b := z + 1
	c := 1 / 1e-300
	d := 1 / b
	h := d
	for i := 1.0; i < 1000; i++ {
		an := -i * i
		b += 2
		d = 1 / (float64(an*d) + b)
		c = b + an/c
		del := float64(c * d)
		h *= del
		if math.Abs(del-1) < 1e-16 {
			break
		}
	}
	return h
}

// digamma is ψ(x), x > 0: by recurrence to x ≥ 10, then its asymptotic series (whose next term
// is then below 10⁻¹³).
func digamma(x float64) float64 {
	result := 0.0
	for x < 10 {
		result -= 1 / x
		x++
	}
	inv := 1 / x
	inv2 := inv * inv
	return result + dmath.Log(x) - float64(0.5*inv) - float64(inv2*(1.0/12-float64(inv2*(1.0/120-float64(inv2*(1.0/252-float64(inv2*(1.0/240-float64(inv2*(1.0/132))))))))))
}

// estimated computes the estimated measures, into out by measure id.
func estimated(f Freq, st stats, out map[string]float64) {
	est, _, _, _ := chao1(st, 1.96, f)
	out["chao1"] = est
	out["ace"] = ace(f, st)
	out["hill1Estimated"] = dmath.Exp(shannonAsymptotic(f, st))
	if st.sumXX1 > 0 {
		out["hill2Estimated"] = st.n * (st.n - 1) / st.sumXX1
	} else {
		out["hill2Estimated"] = math.NaN()
	}
	out["coverage"] = coverage(st)
}
