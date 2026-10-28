// Package stats supplies distribution functions for subject-level tests.
// Adapted from SituCraft 0.9.0 internal/stats/stats.go and intervals.go,
// Copyright 2026 Robert McDermott, Apache-2.0. Beta continued fraction:
// Abramowitz & Stegun (1964), 26.5.8, evaluated by Lentz's method.
package stats

import (
	"clonoloom/internal/dmath"
	"math"
)

func mul(a, b float64) float64 { return float64(a * b) }

// BetaCDF is the beta distribution function of shapes a and b at x: the regularized incomplete
// beta function I_x(a, b).
func BetaCDF(x, a, b float64) float64 {
	switch {
	case x <= 0:
		return 0
	case x >= 1:
		return 1
	}
	la, lb, lab := dmath.Lgamma(a), dmath.Lgamma(b), dmath.Lgamma(a+b)
	front := dmath.Exp(mul(a, dmath.Log(x)) + mul(b, dmath.Log1p(-x)) - (la + lb - lab))
	// The fraction converges fast below the mean; above it, by symmetry.
	if x < (a+1)/(a+b+2) {
		return front * betaFraction(x, a, b) / a
	}
	return 1 - front*betaFraction(1-x, b, a)/b
}

// betaFraction is A&S 26.5.8's continued fraction, by Lentz's method.
func betaFraction(x, a, b float64) float64 {
	const tiny = 1e-300
	c, d := 1.0, 1-mul(a+b, x)/(a+1)
	if math.Abs(d) < tiny {
		d = tiny
	}
	d = 1 / d
	h := d
	for m := 1; m <= 10000; m++ {
		fm := float64(m)
		// The even step, then the odd.
		for _, num := range []float64{
			mul(mul(fm, b-fm), x) / mul(a+mul(2, fm)-1, a+mul(2, fm)),
			mul(mul(-(a+fm), a+b+fm), x) / mul(a+mul(2, fm), a+mul(2, fm)+1),
		} {
			d = 1 + mul(num, d)
			if math.Abs(d) < tiny {
				d = tiny
			}
			c = 1 + num/c
			if math.Abs(c) < tiny {
				c = tiny
			}
			d = 1 / d
			h *= mul(d, c)
		}
		if math.Abs(mul(d, c)-1) < 1e-15 {
			break
		}
	}
	return h
}

// solve finds where an increasing function f crosses target on [lo, hi], by bisection.
func solve(f func(float64) float64, target, lo, hi float64) float64 {
	for i := 0; i < 200; i++ {
		mid := lo + float64((hi-lo)/2)
		if mid <= lo || mid >= hi {
			break
		}
		if f(mid) < target {
			lo = mid
		} else {
			hi = mid
		}
	}
	return lo + float64((hi-lo)/2)
}

// BetaQuantile is the p-quantile of the beta distribution of shapes a and b.
func BetaQuantile(p, a, b float64) float64 {
	switch {
	case !(a > 0) || !(b > 0) || !(p >= 0 && p <= 1):
		return math.NaN()
	case p == 0:
		return 0
	case p == 1:
		return 1
	}
	return solve(func(x float64) float64 { return BetaCDF(x, a, b) }, p, 0, 1)
}

// StudentTTwoSided is the two-sided p-value of a t statistic with df degrees of freedom: the
// chance of a |T| at least |t|, I_{df/(df+t²)}(df/2, 1/2).
func StudentTTwoSided(t, df float64) float64 {
	switch {
	case math.IsNaN(t) || !(df > 0):
		return math.NaN()
	case math.IsInf(t, 0):
		return 0
	}
	return BetaCDF(df/(df+mul(t, t)), df/2, 0.5)
}

// StudentTQuantile is the p-quantile of Student's t distribution with df degrees of freedom.
func StudentTQuantile(p, df float64) float64 {
	switch {
	case !(df > 0) || !(p >= 0 && p <= 1):
		return math.NaN()
	case p == 0.5:
		return 0
	case p < 0.5:
		return -StudentTQuantile(1-p, df)
	case p == 1:
		return math.Inf(1)
	}
	// |T| beyond t has chance 2(1 − p): x = df/(df + t²) is that beta quantile.
	x := BetaQuantile(2*(1-p), df/2, 0.5)
	return math.Sqrt(df * (1 - x) / x)
}
