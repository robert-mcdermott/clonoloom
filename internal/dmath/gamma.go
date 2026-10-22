package dmath

import "math"

// The log-gamma function and log1p, the same bits on every processor (the package's comment says
// why), for rarefaction's subsample sums and the random draws: Go's math.Lgamma and math.Log1p may
// fuse a multiply and an add on some processors and not others, and lean on math.Log, which has
// assembly versions on some.

// Lgamma is log Γ(x) for x > 0 (NaN otherwise): fdlibm 5.3's __ieee754_lgamma_r (Sun
// Microsystems, 1993; "Permission to use, copy, modify, and distribute this software is freely
// granted, provided that this notice is preserved"), its positive branch, every product rounded on
// its own and its logarithms this package's.
func Lgamma(x float64) float64 {
	const (
		ymin  = 1.461632144968362245
		tiny  = 1.0 / (1 << 70)
		two58 = 1 << 58
		tc    = 1.46163214496836224576e+00
		tf    = -1.21486290535849611461e-01
		tt    = -3.63867699703950536541e-18 // −(tf's tail)
	)
	switch {
	case math.IsNaN(x) || x <= 0:
		return math.NaN()
	case math.IsInf(x, 1):
		return x
	case x < tiny:
		return -Log(x)
	case x == 1 || x == 2:
		return 0
	}
	switch {
	case x < 2:
		// lgamma(x) = lgamma(x + 1) − log(x), about the minimum or its neighbors.
		out, y, i := 0.0, 0.0, 0
		if x <= 0.9 {
			out = -Log(x)
			switch {
			case x >= ymin-1+0.27:
				y, i = 1-x, 0
			case x >= ymin-1-0.27:
				y, i = x-(tc-1), 1
			default:
				y, i = x, 2
			}
		} else {
			switch {
			case x >= ymin+0.27:
				y, i = 2-x, 0
			case x >= ymin-0.27:
				y, i = x-tc, 1
			default:
				y, i = x-1, 2
			}
		}
		switch i {
		case 0:
			z := mul(y, y)
			p1 := horner(z, lgamAEven[:])
			p2 := mul(z, horner(z, lgamAOdd[:]))
			p := mul(y, p1) + p2
			out += p - mul(0.5, y)
		case 1:
			z := mul(y, y)
			w := mul(z, y)
			p1 := horner(w, lgamT0[:])
			p2 := horner(w, lgamT1[:])
			p3 := horner(w, lgamT2[:])
			p := mul(z, p1) - (tt - mul(w, p2+mul(y, p3)))
			out += tf + p
		default:
			p1 := mul(y, horner(y, lgamU[:]))
			p2 := 1 + mul(y, horner(y, lgamV[:]))
			out += p1/p2 - mul(0.5, y)
		}
		return out
	case x < 8:
		i := int(x)
		y := x - float64(i)
		p := mul(y, horner(y, lgamS[:]))
		q := 1 + mul(y, horner(y, lgamR[:]))
		out := mul(0.5, y) + p/q
		// lgamma(1 + s) = log(s) + lgamma(s).
		z := 1.0
		for k := i - 1; k >= 2; k-- {
			z = mul(z, y+float64(k))
		}
		if i >= 3 {
			out += Log(z)
		}
		return out
	case x < two58:
		t := Log(x)
		z := 1 / x
		y := mul(z, z)
		w := lgamW0 + mul(z, horner(y, lgamW[:]))
		return mul(x-0.5, t-1) + w
	}
	return mul(x, Log(x)-1)
}

// fdlibm's coefficients (e_lgamma_r.c's a0–a11, t0–t14, u0–u5, v1–v5, s0–s6, r1–r6, w0–w6),
// arranged for horner.
var (
	lgamAEven = [...]float64{7.72156649015328655494e-02, 6.73523010531292681824e-02, 7.38555086081402883957e-03, 1.19270763183362067845e-03, 2.20862790713908385557e-04, 2.52144565451257326939e-05}
	lgamAOdd  = [...]float64{3.22467033424113591611e-01, 2.05808084325167332806e-02, 2.89051383673415629091e-03, 5.10069792153511336608e-04, 1.08011567247583939954e-04, 4.48640949618915160150e-05}
	lgamT0    = [...]float64{4.83836122723810047042e-01, -3.27885410759859649565e-02, 6.10053870246291332635e-03, -1.40346469989232843813e-03, 3.15632070903625950361e-04}
	lgamT1    = [...]float64{-1.47587722994593911752e-01, 1.79706750811820387126e-02, -3.68452016781138256760e-03, 8.81081882437654011382e-04, -3.12754168375120860518e-04}
	lgamT2    = [...]float64{6.46249402391333854778e-02, -1.03142241298341437450e-02, 2.25964780900612472250e-03, -5.38595305356740546715e-04, 3.35529192635519073543e-04}
	lgamU     = [...]float64{-7.72156649015328655494e-02, 6.32827064025093366517e-01, 1.45492250137234768737e+00, 9.77717527963372745603e-01, 2.28963728064692451092e-01, 1.33810918536787660377e-02}
	lgamV     = [...]float64{2.45597793713041134822e+00, 2.12848976379893395361e+00, 7.69285150456672783825e-01, 1.04222645593369134254e-01, 3.21709242282423911810e-03}
	lgamS     = [...]float64{-7.72156649015328655494e-02, 2.14982415960608852501e-01, 3.25778796408930981787e-01, 1.46350472652464452805e-01, 2.66422703033638609560e-02, 1.84028451407337715652e-03, 3.19475326584100867617e-05}
	lgamR     = [...]float64{1.39200533467621045958e+00, 7.21935547567138069525e-01, 1.71933865632803078993e-01, 1.86459191715652901344e-02, 7.77942496381893596434e-04, 7.32668430744625636189e-06}
	lgamW     = [...]float64{8.33333333333329678849e-02, -2.77777777728775536470e-03, 7.93650558643019558500e-04, -5.95187557450339963135e-04, 8.36339918996282139126e-04, -1.63092934096575273989e-03}
)

const lgamW0 = 4.18938533204672725052e-01

// Log1p is log(1 + x), accurate for small x: log(u) · x / (u − 1), u = 1 + x rounded, which
// cancels u's rounding error (Goldberg D. What every computer scientist should know about
// floating-point arithmetic. ACM Comput Surv 1991;23:5–48, theorem 4).
func Log1p(x float64) float64 {
	switch {
	case math.IsNaN(x) || x < -1:
		return math.NaN()
	case x == -1:
		return math.Inf(-1)
	case math.IsInf(x, 1):
		return x
	}
	u := 1 + x
	if u == 1 {
		return x
	}
	return mul(Log(u), x/(u-1))
}
