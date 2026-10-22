// Package expansion tests each clonotype between two samples (clonoloom-spec/requirements.md, T2;
// roadmap.md, wave 3, slice 6), as DeWitt et al. 2015 did: with k₁ and k₂ the clonotype's cells or
// molecules in samples of N₁ and N₂, whether its frequency differs. Two tests, each conditional on
// the clonotype's total k = k₁ + k₂:
//   - the binomial test of k₁ ~ Binomial(k, θ), θ = N₁/(N₁ + N₂), the share of the depth
//     (Rytlewski et al. 2019's reference code; the test the beta-binomial of slice 7 extends);
//   - Fisher's exact test of the 2×2 table [[k₁, k₂], [N₁ − k₁, N₂ − k₂]] (DeWitt et al. 2015),
//     k₁ hypergeometric given the margins.
//
// Two-sided p-values sum the probabilities of every outcome no more likely than the one seen (with
// R's relative tolerance of 10⁻⁷); one-sided ones a tail. Clonotypes with fewer than minTotal in
// the two samples together are not tested, nor counted in the false discovery rate, which is
// controlled by Benjamini and Hochberg's procedure or Storey's q-values. Every sum is of terms
// rounded on their own, so the results are the same on every processor.
package expansion

import (
	"errors"
	"fmt"
	"math"
	"sort"

	"clonoloom/internal/clonotype"
	"clonoloom/internal/diversity"
	"clonoloom/internal/dmath"
)

// The tests.
const (
	Binomial = "binomial"
	Fisher   = "fisher"
)

// The alternatives: a change either way, more in the second sample, or less.
const (
	TwoSided = "two-sided"
	Increase = "increase"
	Decrease = "decrease"
)

// The corrections for testing many clonotypes.
const (
	BH     = "bh"
	Storey = "storey"
)

// StoreyLambda is the λ at which Storey's π₀ is estimated: the share of p-values of at least λ,
// over 1 − λ (Storey 2002).
const StoreyLambda = 0.5

// Options are how the clonotypes are tested: the test, the alternative, the correction and the
// false discovery rate at which a clonotype is called, and the least total of the two samples a
// clonotype needs to be tested.
type Options struct {
	Test        string  `json:"test"`
	Alternative string  `json:"alternative"`
	Correction  string  `json:"correction"`
	Alpha       float64 `json:"alpha"`
	MinTotal    int     `json:"minTotal"`
}

// Normalize completes options with the defaults (Rytlewski et al. 2019's: the binomial test,
// two-sided, Benjamini–Hochberg at 0.01, at least 5 in the two samples together) and checks them.
func (o Options) Normalize() (Options, error) {
	if o.Test == "" {
		o.Test = Binomial
	}
	if o.Alternative == "" {
		o.Alternative = TwoSided
	}
	if o.Correction == "" {
		o.Correction = BH
	}
	if o.Alpha == 0 {
		o.Alpha = 0.01
	}
	if o.MinTotal == 0 {
		o.MinTotal = 5
	}
	switch {
	case o.Test != Binomial && o.Test != Fisher:
		return o, fmt.Errorf("test %q: binomial or fisher", o.Test)
	case o.Alternative != TwoSided && o.Alternative != Increase && o.Alternative != Decrease:
		return o, fmt.Errorf("alternative %q: two-sided, increase or decrease", o.Alternative)
	case o.Correction != BH && o.Correction != Storey:
		return o, fmt.Errorf("correction %q: bh or storey", o.Correction)
	case !(o.Alpha > 0 && o.Alpha < 1):
		return o, errors.New("alpha: between 0 and 1")
	case o.MinTotal < 1:
		return o, errors.New("minTotal: at least 1")
	}
	return o, nil
}

// Clonotype is a clonotype tested: its counts and frequencies in the two samples, the log₂ of
// their ratio (missing where either is 0), its p-value and adjusted p-value (BH's, or Storey's
// q-value), and its call: increase, decrease, or none.
type Clonotype struct {
	ID       string            `json:"id"`
	Chains   []clonotype.Chain `json:"chains"`
	A        float64           `json:"a"`
	B        float64           `json:"b"`
	FreqA    float64           `json:"freqA"`
	FreqB    float64           `json:"freqB"`
	Log2Fold diversity.Number  `json:"log2Fold"`
	P        float64           `json:"p"`
	Q        float64           `json:"q"`
	Call     string            `json:"call,omitempty"`
}

// Result is the comparison of two samples (or two groups of samples, pooled): their totals, the
// clonotypes tested (those called first, by adjusted p-value; then the rest by p-value), how many
// were not tested for having fewer than minTotal, how many were called each way, and Storey's π₀
// where it was used.
type Result struct {
	TotalA     float64          `json:"totalA"`
	TotalB     float64          `json:"totalB"`
	Tested     int              `json:"tested"`
	LeftOut    int              `json:"leftOut"`
	Increased  int              `json:"increased"`
	Decreased  int              `json:"decreased"`
	Pi0        diversity.Number `json:"pi0"`
	Clonotypes []Clonotype      `json:"clonotypes"`
}

// ErrNotCounts: the tests need whole counts of cells or molecules.
var ErrNotCounts = errors.New("the tests need whole counts of cells or molecules: these abundances are not whole numbers")

// Compare tests every clonotype of t found in either group of samples (a, b; each group's samples
// pooled) as the options say.
func Compare(t *clonotype.Table, a, b []uint32, opt Options) (Result, error) {
	opt, err := opt.Normalize()
	if err != nil {
		return Result{}, err
	}
	inA, inB := map[uint32]bool{}, map[uint32]bool{}
	var res Result
	for _, s := range a {
		inA[s] = true
		res.TotalA += t.SampleTotals[s]
	}
	for _, s := range b {
		inB[s] = true
		res.TotalB += t.SampleTotals[s]
	}
	res.Pi0 = diversity.Number(math.NaN())
	res.Clonotypes = []Clonotype{}
	if !whole(res.TotalA) || !whole(res.TotalB) {
		return res, ErrNotCounts
	}
	if res.TotalA <= 0 || res.TotalB <= 0 {
		return res, errors.New("each group of samples needs clonotypes to compare")
	}
	var ps []float64
	for i := range t.Clonotypes {
		samples, values := t.Abundances(i)
		var ka, kb float64
		for k, s := range samples {
			if inA[s] {
				ka += values[k]
			}
			if inB[s] {
				kb += values[k]
			}
		}
		if ka+kb == 0 {
			continue
		}
		if !whole(ka) || !whole(kb) {
			return res, ErrNotCounts
		}
		if ka+kb < float64(opt.MinTotal) {
			res.LeftOut++
			continue
		}
		c := t.Clonotypes[i]
		x := Clonotype{ID: c.ID, Chains: c.Chains, A: ka, B: kb, FreqA: ka / res.TotalA, FreqB: kb / res.TotalB, Log2Fold: diversity.Number(math.NaN())}
		if ka > 0 && kb > 0 {
			x.Log2Fold = diversity.Number(dmath.Log(x.FreqB/x.FreqA) / math.Ln2)
		}
		if opt.Test == Fisher {
			x.P = FisherP(int64(ka), int64(kb), int64(res.TotalA), int64(res.TotalB), opt.Alternative)
		} else {
			x.P = BinomialP(int64(ka), int64(ka+kb), res.TotalA/(res.TotalA+res.TotalB), opt.Alternative)
		}
		res.Clonotypes = append(res.Clonotypes, x)
		ps = append(ps, x.P)
	}
	res.Tested = len(res.Clonotypes)
	q := AdjustBH(ps)
	if opt.Correction == Storey && len(ps) > 0 {
		pi0 := StoreyPi0(ps, StoreyLambda)
		res.Pi0 = diversity.Number(pi0)
		for i := range q {
			q[i] = float64(pi0 * q[i])
		}
	}
	for i := range res.Clonotypes {
		x := &res.Clonotypes[i]
		x.Q = q[i]
		if x.Q <= opt.Alpha {
			switch {
			case x.FreqB > x.FreqA && opt.Alternative != Decrease:
				x.Call = Increase
				res.Increased++
			case x.FreqB < x.FreqA && opt.Alternative != Increase:
				x.Call = Decrease
				res.Decreased++
			}
		}
	}
	// Those called first, by adjusted p-value; then by p-value; ties by the table's order.
	sort.SliceStable(res.Clonotypes, func(i, j int) bool {
		a, b := res.Clonotypes[i], res.Clonotypes[j]
		if (a.Call != "") != (b.Call != "") {
			return a.Call != ""
		}
		if a.Q != b.Q {
			return a.Q < b.Q
		}
		return a.P < b.P
	})
	return res, nil
}

func whole(x float64) bool { return x == math.Trunc(x) && x >= 0 }

// relErr is R's tolerance in deciding which outcomes are no more likely than the one seen.
const relErr = 1 + 1e-7

// BinomialP is the p-value of k successes of n at probability θ: two-sided (R's binom.test), or
// one-sided, the lower tail (increase: fewer in the first sample than θ expects) or the upper
// (decrease).
func BinomialP(k, n int64, theta float64, alt string) float64 {
	if n <= 0 {
		return 1
	}
	if theta <= 0 || theta >= 1 {
		// A degenerate share: every outcome but one has no chance.
		if (theta <= 0 && k == 0) || (theta >= 1 && k == n) {
			return 1
		}
		return 0
	}
	odds := dmath.Log(theta) - dmath.Log(1-theta)
	return exactP(0, n, k, func(x int64) float64 {
		// p(x + 1) / p(x) = (n − x) / (x + 1) · θ / (1 − θ)
		return dmath.Log(float64(n-x)) - dmath.Log(float64(x+1)) + odds
	}, alt)
}

// FisherP is Fisher's exact test's p-value for a clonotype of k₁ of N₁ in the first sample and k₂
// of N₂ in the second: two-sided (R's fisher.test), or one-sided, the lower tail (increase) or the
// upper (decrease).
func FisherP(k1, k2, n1, n2 int64, alt string) float64 {
	k := k1 + k2
	lo, hi := max(0, k-n2), min(k, n1)
	return exactP(lo, hi, k1, func(x int64) float64 {
		// p(x + 1) / p(x) = (N₁ − x)(k − x) / ((x + 1)(N₂ − k + x + 1))
		return dmath.Log(float64(n1-x)) + dmath.Log(float64(k-x)) - dmath.Log(float64(x+1)) - dmath.Log(float64(n2-k+x+1))
	}, alt)
}

// exactP is the p-value of x in a distribution on lo..hi given by the log of each probability's
// ratio to the one before: the weights relative to the largest, summed (as R's fisher.test sums its
// hypergeometric densities).
func exactP(lo, hi, x int64, logRatio func(x int64) float64, alt string) float64 {
	if x < lo || x > hi {
		return 0
	}
	w := make([]float64, hi-lo+1)
	top := 0.0
	for i := lo; i < hi; i++ {
		w[i-lo+1] = w[i-lo] + logRatio(i)
		top = math.Max(top, w[i-lo+1])
	}
	total, tail := 0.0, 0.0
	seen := dmath.Exp(w[x-lo] - top)
	bound := float64(seen * relErr)
	for i := lo; i <= hi; i++ {
		d := dmath.Exp(w[i-lo] - top)
		total += d
		switch alt {
		case Increase:
			if i <= x {
				tail += d
			}
		case Decrease:
			if i >= x {
				tail += d
			}
		default:
			if d <= bound {
				tail += d
			}
		}
	}
	return math.Min(1, tail/total)
}

// AdjustBH is Benjamini and Hochberg's adjusted p-values (R's p.adjust, "BH"): the least, over the
// p-values at least as large, of m/rank times the p-value, at most 1.
func AdjustBH(p []float64) []float64 {
	m := len(p)
	order := make([]int, m)
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return p[order[a]] > p[order[b]] })
	out := make([]float64, m)
	least := 1.0
	for r, i := range order {
		rank := m - r
		least = math.Min(least, float64(float64(m)/float64(rank)*p[i]))
		out[i] = least
	}
	return out
}

// StoreyPi0 is Storey's estimate of the share of true nulls: the share of p-values of at least λ,
// over 1 − λ, at most 1 (the qvalue package's, at one λ).
func StoreyPi0(p []float64, lambda float64) float64 {
	n := 0
	for _, x := range p {
		if x >= lambda {
			n++
		}
	}
	return math.Min(1, float64(n)/float64(len(p))/(1-lambda))
}
