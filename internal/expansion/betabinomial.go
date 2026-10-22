package expansion

// The beta-binomial test (Rytlewski et al. 2019; clonoloom-spec/roadmap.md, wave 3, slice 7): a
// clonotype's count in the first sample, given its total n in the two, is beta-binomial with mean
// θ = N₁/(N₁ + N₂) (the share of the depth) and a scale v(n) = 2 + (b ln n + a)², so that two
// samples of one repertoire may differ by more than sampling, as two blood samples do. The 2, which
// keeps the beta unimodal, is in Rytlewski et al.'s reference code, not in their paper's equation.
// a and b are fitted by maximum likelihood on clonotypes of samples that should not differ:
// replicate samples, or two time points the user names as stable. Every product added into a sum
// is rounded on its own, so the fit and the p-values are the same on every processor.

import (
	"errors"
	"math"
	"sort"

	"clonoloom/internal/dmath"
)

// BetaBinomial is the beta-binomial test; Auto the beta-binomial test where a variance model can
// be fitted, else the binomial test.
const (
	BetaBinomial = "betabinomial"
	Auto         = "auto"
)

// MaxV bounds the scale: beyond it, the beta-binomial is the binomial for any count a repertoire
// holds, and the log-likelihood loses its precision.
const MaxV = 1e7

// Model is Rytlewski et al.'s variance model: v(n) = 2 + (b ln n + a)², at most MaxV. (a, b) and
// (−a, −b) give the same model; a fitted model has b ≥ 0.
type Model struct {
	A float64 `json:"a"`
	B float64 `json:"b"`
}

// V is the model's scale for a clonotype of n in the two samples.
func (m Model) V(n float64) float64 {
	s := float64(m.B*dmath.Log(n)) + m.A
	return math.Min(2+float64(s*s), MaxV)
}

// BetaBinomialP is the p-value of k of n under the beta-binomial of mean θ and scale v: two-sided,
// summing the outcomes no more likely than k (with R's relative tolerance), or one-sided, the lower
// tail (increase: fewer in the first sample than θ expects) or the upper (decrease).
func BetaBinomialP(k, n int64, theta, v float64, alt string) float64 {
	if n <= 0 {
		return 1
	}
	if theta <= 0 || theta >= 1 {
		if (theta <= 0 && k == 0) || (theta >= 1 && k == n) {
			return 1
		}
		return 0
	}
	alpha, beta := float64(theta*v), float64((1-theta)*v)
	return exactP(0, n, k, func(x int64) float64 {
		// p(x + 1) / p(x) = (n − x) / (x + 1) · (x + α) / (n − x − 1 + β)
		return dmath.Log(float64(n-x)) - dmath.Log(float64(x+1)) + dmath.Log(float64(x)+alpha) - dmath.Log(float64(n-x-1)+beta)
	}, alt)
}

// lnBeta is log B(x, y).
func lnBeta(x, y float64) float64 {
	return dmath.Lgamma(x) + dmath.Lgamma(y) - dmath.Lgamma(x+y)
}

// LogPMF is the log-probability of k of n under the beta-binomial of mean θ and scale v.
func LogPMF(k, n int64, theta, v float64) float64 {
	alpha, beta := float64(theta*v), float64((1-theta)*v)
	lnC := dmath.Lgamma(float64(n+1)) - dmath.Lgamma(float64(k+1)) - dmath.Lgamma(float64(n-k+1))
	return lnC + lnBeta(float64(k)+alpha, float64(n-k)+beta) - lnBeta(alpha, beta)
}

// Observation is one or more clonotypes of two samples that should not differ: k in the first of
// n in the two, the share θ the first is expected to hold, and how many clonotypes are so.
type Observation struct {
	K     int64   `json:"k"`
	N     int64   `json:"n"`
	Theta float64 `json:"theta"`
	Count int     `json:"count"`
}

// LogLikelihood is the observations' log-likelihood under the model.
func LogLikelihood(m Model, obs []Observation) float64 {
	total := 0.0
	for _, o := range obs {
		total += float64(float64(o.Count) * LogPMF(o.K, o.N, o.Theta, m.V(float64(o.N))))
	}
	return total
}

// Fitted is a variance model fitted on observations: the model, the log-likelihood it reaches, the
// observations' clonotypes, and the Nelder–Mead iterations it took.
type Fitted struct {
	Model      Model   `json:"model"`
	LogLik     float64 `json:"logLik"`
	Clonotypes int     `json:"clonotypes"`
	Iterations int     `json:"iterations"`
}

// ErrNoObservations: a model needs clonotypes to fit on.
var ErrNoObservations = errors.New("no clonotypes to fit the variance on: the samples that should not differ hold none with enough cells or molecules")

// Fit fits the model by maximum likelihood: Nelder–Mead from (a, b) = (1, 1), as Rytlewski et
// al.'s code starts, restarted from its best point until it no longer improves.
func Fit(obs []Observation) (Fitted, error) {
	n := 0
	for _, o := range obs {
		n += o.Count
	}
	if n == 0 {
		return Fitted{}, ErrNoObservations
	}
	negLL := func(x [2]float64) float64 { return -LogLikelihood(Model{A: x[0], B: x[1]}, obs) }
	x, f, iterations := [2]float64{1, 1}, 0.0, 0
	for restart := 0; restart < 8; restart++ {
		nx, nf, it := nelderMead(negLL, x)
		iterations += it
		improved := restart == 0 || nf < f-float64(1e-10*math.Max(1, math.Abs(f)))
		if nf < f || restart == 0 {
			x, f = nx, nf
		}
		if !improved {
			break
		}
	}
	m := Model{A: x[0], B: x[1]}
	if m.B < 0 {
		m = Model{A: -m.A, B: -m.B}
	}
	return Fitted{Model: m, LogLik: -f, Clonotypes: n, Iterations: iterations}, nil
}

// nelderMead minimizes f in two dimensions from x0 (Nelder and Mead 1965, with the coefficients
// and initial simplex scipy uses: reflection 1, expansion 2, contraction ½, shrinkage ½, each
// coordinate moved by 5%), until the simplex's values and points agree to 10⁻¹⁰ and 10⁻⁸, or for
// 2,000 iterations. Answers the best point, its value and the iterations.
func nelderMead(f func([2]float64) float64, x0 [2]float64) ([2]float64, float64, int) {
	type vertex struct {
		x [2]float64
		f float64
	}
	s := [3]vertex{{x: x0}}
	for i := 0; i < 2; i++ {
		x := x0
		if x[i] != 0 {
			x[i] = float64(1.05 * x[i])
		} else {
			x[i] = 0.00025
		}
		s[i+1].x = x
	}
	for i := range s {
		s[i].f = f(s[i].x)
	}
	// along answers c + t(p − c), each product rounded on its own.
	along := func(c, p [2]float64, t float64) [2]float64 {
		return [2]float64{c[0] + float64(t*(p[0]-c[0])), c[1] + float64(t*(p[1]-c[1]))}
	}
	it := 0
	for ; it < 2000; it++ {
		sort.SliceStable(s[:], func(i, j int) bool { return s[i].f < s[j].f })
		spread, reach := 0.0, 0.0
		for _, v := range s[1:] {
			spread = math.Max(spread, math.Abs(v.f-s[0].f))
			reach = math.Max(reach, math.Max(math.Abs(v.x[0]-s[0].x[0]), math.Abs(v.x[1]-s[0].x[1])))
		}
		if spread <= 1e-10*math.Max(1, math.Abs(s[0].f)) && reach <= 1e-8 {
			break
		}
		c := [2]float64{(s[0].x[0] + s[1].x[0]) / 2, (s[0].x[1] + s[1].x[1]) / 2}
		worst := s[2]
		r := vertex{x: along(c, worst.x, -1)}
		r.f = f(r.x)
		switch {
		case r.f < s[0].f:
			e := vertex{x: along(c, worst.x, -2)}
			e.f = f(e.x)
			if e.f < r.f {
				s[2] = e
			} else {
				s[2] = r
			}
			continue
		case r.f < s[1].f:
			s[2] = r
			continue
		case r.f < worst.f:
			// Outside the simplex: halfway to the reflection.
			oc := vertex{x: along(c, worst.x, -0.5)}
			oc.f = f(oc.x)
			if oc.f <= r.f {
				s[2] = oc
				continue
			}
		default:
			// Inside: halfway to the worst.
			ic := vertex{x: along(c, worst.x, 0.5)}
			ic.f = f(ic.x)
			if ic.f < worst.f {
				s[2] = ic
				continue
			}
		}
		for i := 1; i < 3; i++ {
			s[i].x = along(s[0].x, s[i].x, 0.5)
			s[i].f = f(s[i].x)
		}
	}
	sort.SliceStable(s[:], func(i, j int) bool { return s[i].f < s[j].f })
	return s[0].x, s[0].f, it
}
