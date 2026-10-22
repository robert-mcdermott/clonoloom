package dmath

import "math"

// Expm1 is e^x − 1, accurate for small x, where e^x − 1 would lose its digits: fdlibm 5.3's
// s_expm1.c (the package's notice), every product rounded on its own. x is reduced to
// r = x − k ln 2 with |r| ≤ ln 2 / 2, where a rational approximation of r/(e^r − 1) is used, and
// 2^k put back exactly into the exponent.
func Expm1(x float64) float64 {
	const (
		ln2X56    = 3.88162421113569373274e+01 // 56 ln 2
		ln2HalfX3 = 1.03972077083991796413e+00 // 1.5 ln 2
		ln2Half   = 3.46573590279972654709e-01 // ln 2 / 2
		tiny      = 1.0 / (1 << 54)
		q1        = -3.33333333333331316428e-02
		q2        = 1.58730158725481460165e-03
		q3        = -7.93650757867487942473e-05
		q4        = 4.00821782732936239552e-06
		q5        = -2.01099218183624371326e-07
	)
	switch {
	case math.IsInf(x, 1) || math.IsNaN(x):
		return x
	case math.IsInf(x, -1):
		return -1
	}
	sign := x < 0
	absx := math.Abs(x)
	if absx >= ln2X56 {
		if sign {
			return -1 // e^x below 2^-56: −1, rounded
		}
		if absx >= oThreshold {
			return math.Inf(1)
		}
	}
	// Argument reduction.
	var c float64
	k := 0
	switch {
	case absx > ln2Half:
		var hi, lo float64
		if absx < ln2HalfX3 {
			if !sign {
				hi, lo, k = x-ln2Hi, ln2Lo, 1
			} else {
				hi, lo, k = x+ln2Hi, -ln2Lo, -1
			}
		} else {
			if !sign {
				k = int(mul(invLn2, x) + 0.5)
			} else {
				k = int(mul(invLn2, x) - 0.5)
			}
			t := float64(k)
			hi = x - mul(t, ln2Hi) // exact
			lo = mul(t, ln2Lo)
		}
		x = hi - lo
		c = (hi - x) - lo
	case absx < tiny:
		return x
	}
	// x is now in the primary range.
	hfx := mul(0.5, x)
	hxs := mul(x, hfx)
	r1 := 1 + mul(hxs, q1+mul(hxs, q2+mul(hxs, q3+mul(hxs, q4+mul(hxs, q5)))))
	t := 3 - mul(r1, hfx)
	e := mul(hxs, (r1-t)/(6.0-mul(x, t)))
	if k == 0 {
		return x - (mul(x, e) - hxs) // c is 0
	}
	e = mul(x, e-c) - c
	e -= hxs
	switch {
	case k == -1:
		return mul(0.5, x-e) - 0.5
	case k == 1:
		if x < -0.25 {
			return mul(-2, e-(x+0.5))
		}
		return 1 + mul(2, x-e)
	case k <= -2 || k > 56: // e^x − 1 is then e^x − 1 rounded
		y := 1 - (e - x)
		y = math.Float64frombits(math.Float64bits(y) + uint64(k)<<52) // 2^k into the exponent
		return y - 1
	}
	if k < 20 {
		t := math.Float64frombits(0x3ff0000000000000 - (0x20000000000000 >> uint(k))) // 1 − 2^-k
		y := t - (e - x)
		return math.Float64frombits(math.Float64bits(y) + uint64(k)<<52)
	}
	t = math.Float64frombits(uint64(0x3ff-k) << 52) // 2^-k
	y := x - (e + t)
	y++
	return math.Float64frombits(math.Float64bits(y) + uint64(k)<<52)
}
