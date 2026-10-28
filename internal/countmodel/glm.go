// Package countmodel fits replicated NB2 counts with a log link and explicit offsets.
// McCarthy, Chen & Smyth, Nucleic Acids Res (2012) 40:4288–4297,
// doi:10.1093/nar/gks042: Fisher scoring, Cox–Reid adjusted profile likelihood,
// and nested-model likelihood-ratio tests. This is an unmoderated per-clonotype
// implementation, not the full edgeR pipeline. See research.md §5.33.
package countmodel

import (
	"context"
	"errors"
	"math"

	"clonoloom/internal/dmath"
)

func mul(a, b float64) float64 { return float64(a * b) }
func finite(x float64) bool    { return !math.IsNaN(x) && !math.IsInf(x, 0) }

type Fit struct {
	Beta          []float64   `json:"beta"`
	Mu            []float64   `json:"mu"`
	Covariance    [][]float64 `json:"covariance"`
	LogLikelihood float64     `json:"logLikelihood"`
	LogDet        float64     `json:"logDet"`
	Iterations    int         `json:"iterations"`
}

// factor returns the lower Cholesky factor of a symmetric positive definite matrix.
// The relative pivot guard refuses singular/ill-conditioned information, never adds a ridge.
func factor(a [][]float64) ([][]float64, float64, error) {
	p := len(a)
	l := make([][]float64, p)
	scale := 0.0
	for i := range a {
		l[i] = make([]float64, p)
		scale = math.Max(scale, a[i][i])
	}
	logdet := 0.0
	for i := 0; i < p; i++ {
		for j := 0; j <= i; j++ {
			v := a[i][j]
			for k := 0; k < j; k++ {
				v -= mul(l[i][k], l[j][k])
			}
			if i == j {
				if !finite(v) || v <= scale*1e-11 {
					return nil, 0, errors.New("rank-deficient or ill-conditioned design/information")
				}
				l[i][j] = math.Sqrt(v)
				logdet += 2 * dmath.Log(l[i][j])
			} else {
				l[i][j] = v / l[j][j]
			}
		}
	}
	return l, logdet, nil
}
func solve(l [][]float64, b []float64) []float64 {
	x := append([]float64(nil), b...)
	for i := range x {
		for j := 0; j < i; j++ {
			x[i] -= mul(l[i][j], x[j])
		}
		x[i] /= l[i][i]
	}
	for i := len(x) - 1; i >= 0; i-- {
		for j := i + 1; j < len(x); j++ {
			x[i] -= mul(l[j][i], x[j])
		}
		x[i] /= l[i][i]
	}
	return x
}
func information(x [][]float64, mu []float64, alpha float64) [][]float64 {
	p := len(x[0])
	a := make([][]float64, p)
	for i := range a {
		a[i] = make([]float64, p)
	}
	for i, row := range x {
		w := mu[i] / (1 + mul(alpha, mu[i]))
		for j := 0; j < p; j++ {
			for k := 0; k <= j; k++ {
				a[j][k] += mul(mul(w, row[j]), row[k])
			}
		}
	}
	for j := range a {
		for k := 0; k < j; k++ {
			a[k][j] = a[j][k]
		}
	}
	return a
}
func likelihood(x [][]float64, y, offset, beta []float64, alpha float64) (float64, []float64) {
	r := 1 / alpha
	lgR := dmath.Lgamma(r)
	mu := make([]float64, len(y))
	ll := 0.0
	for i, row := range x {
		eta := offset[i]
		for j, v := range row {
			eta += mul(v, beta[j])
		}
		if eta < -30 || eta > 30 {
			return math.Inf(-1), nil
		}
		m := dmath.Exp(eta)
		mu[i] = m
		logsum := dmath.Log(r + m)
		ll += dmath.Lgamma(y[i]+r) - lgR - dmath.Lgamma(y[i]+1) + mul(r, dmath.Log(r)-logsum) + mul(y[i], eta-logsum)
	}
	return ll, mu
}

// FitNB fits coefficients at a strictly positive, fixed NB2 dispersion alpha.
// Var(Y)=mu+alpha*mu². No pseudocount is added to the likelihood or reported counts.
func FitNB(ctx context.Context, x [][]float64, y, offset []float64, alpha float64) (*Fit, error) {
	if len(x) != len(y) || len(y) != len(offset) || len(y) == 0 || len(x[0]) == 0 || !(alpha >= 1e-5 && alpha <= 100) {
		return nil, errors.New("invalid NB model dimensions or dispersion")
	}
	p := len(x[0])
	beta := make([]float64, p)
	total, exposure := 0.0, 0.0
	for i, row := range x {
		if len(row) != p || row[0] != 1 || !finite(y[i]) || y[i] < 0 || y[i] != math.Trunc(y[i]) || !finite(offset[i]) {
			return nil, errors.New("NB model needs rectangular design with first-column intercept, whole nonnegative counts and finite offsets")
		}
		for _, v := range row {
			if !finite(v) {
				return nil, errors.New("non-finite design")
			}
		}
		total += y[i]
		exposure += dmath.Exp(offset[i])
	}
	if !(total > 0) || (!finite(exposure) || exposure <= 0) {
		return nil, errors.New("all-zero or invalid exposure")
	}
	beta[0] = dmath.Log(total / exposure)
	ll, mu := likelihood(x, y, offset, beta, alpha)
	if !finite(ll) {
		return nil, errors.New("initial count means outside numerical bounds")
	}
	for iter := 1; iter <= 100; iter++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		l, logdet, err := factor(information(x, mu, alpha))
		if err != nil {
			return nil, err
		}
		score := make([]float64, p)
		for i, row := range x {
			v := (y[i] - mu[i]) / (1 + mul(alpha, mu[i]))
			for j := range score {
				score[j] += mul(row[j], v)
			}
		}
		delta := solve(l, score)
		size := 0.0
		for _, v := range delta {
			size = math.Max(size, math.Abs(v))
		}
		if size < 1e-7 {
			cov := make([][]float64, p)
			for j := 0; j < p; j++ {
				unit := make([]float64, p)
				unit[j] = 1
				cov[j] = solve(l, unit)
			}
			return &Fit{Beta: beta, Mu: mu, Covariance: cov, LogLikelihood: ll, LogDet: logdet, Iterations: iter}, nil
		}
		step := math.Min(1, 5/size)
		accepted := false
		for half := 0; half < 30; half++ {
			next := make([]float64, p)
			for j := range next {
				next[j] = beta[j] + mul(step, delta[j])
			}
			nl, nm := likelihood(x, y, offset, next, alpha)
			if finite(nl) && nl >= ll-1e-9 {
				beta, ll, mu = next, nl, nm
				accepted = true
				break
			}
			step /= 2
		}
		if !accepted {
			return nil, errors.New("NB scoring line search did not converge")
		}
		for _, b := range beta {
			if math.Abs(b) > 30 {
				return nil, errors.New("unbounded coefficient or separated counts")
			}
		}
	}
	return nil, errors.New("NB scoring exceeded 100 iterations")
}

// FitDispersion maximizes l(beta_hat,alpha) - 0.5 log|X'WX| per feature.
// A fixed log grid brackets the maximum, then golden-section refinement. Boundary
// optima are explicit failures, not a Poisson fallback or an arbitrary ceiling estimate.
func FitDispersion(ctx context.Context, x [][]float64, y, offset []float64) (float64, *Fit, error) {
	const grid = 28
	low, high := dmath.Log(1e-5), dmath.Log(100)
	score := func(z float64) (float64, *Fit, error) {
		f, e := FitNB(ctx, x, y, offset, math.Max(1e-5, math.Min(100, dmath.Exp(z))))
		if e != nil {
			return 0, nil, e
		}
		return f.LogLikelihood - float64(f.LogDet/2), f, nil
	}
	best, bestScore := 0, math.Inf(-1)
	for i := 0; i <= grid; i++ {
		z := low + mul(high-low, float64(i)/grid)
		v, _, e := score(z)
		if e != nil {
			return 0, nil, e
		}
		if v > bestScore {
			best, bestScore = i, v
		}
	}
	if best == 0 || best == grid {
		return 0, nil, errors.New("dispersion optimum at search boundary (1e-5–100); inference unavailable")
	}
	lo := low + mul(high-low, float64(best-1)/grid)
	hi := low + mul(high-low, float64(best+1)/grid)
	const ratio = .6180339887498948482
	a := hi - mul(ratio, hi-lo)
	b := lo + mul(ratio, hi-lo)
	fa, _, err := score(a)
	if err != nil {
		return 0, nil, err
	}
	fb, _, err := score(b)
	if err != nil {
		return 0, nil, err
	}
	for i := 0; i < 34; i++ {
		if fa > fb {
			hi, b, fb = b, a, fa
			a = hi - mul(ratio, hi-lo)
			fa, _, err = score(a)
		} else {
			lo, a, fa = a, b, fb
			b = lo + mul(ratio, hi-lo)
			fb, _, err = score(b)
		}
		if err != nil {
			return 0, nil, err
		}
	}
	z := a
	if fb > fa {
		z = b
	}
	_, fit, err := score(z)
	return dmath.Exp(z), fit, err
}
