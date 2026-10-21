package simulate

import (
	"math"
	"math/rand/v2"
)

// Random draws the simulator's numbers from a PCG generator (whose output its algorithm fixes),
// each draw made here rather than by the standard library's helpers, whose methods may change
// between Go versions: the same seed gives the same study, byte for byte.
type Random struct {
	src *rand.PCG
}

// NewRandom seeds a generator; stream separates independent draws from one seed (the study's
// subjects, its samples).
func NewRandom(seed, stream uint64) *Random {
	return &Random{src: rand.NewPCG(seed, stream^0x9e3779b97f4a7c15)}
}

// Float64 is uniform on [0, 1), from the top 53 bits.
func (r *Random) Float64() float64 {
	return float64(r.src.Uint64()>>11) / (1 << 53)
}

// IntN is uniform on [0, n), without modulo bias (Lemire's method).
func (r *Random) IntN(n int) int {
	if n <= 0 {
		panic("IntN of a non-positive n")
	}
	bound := uint64(n)
	for {
		hi, lo := mul64(r.src.Uint64(), bound)
		if lo >= -bound%bound {
			return int(hi)
		}
	}
}

func mul64(a, b uint64) (hi, lo uint64) {
	const mask32 = 1<<32 - 1
	a0, a1 := a&mask32, a>>32
	b0, b1 := b&mask32, b>>32
	w0 := a0 * b0
	t := a1*b0 + w0>>32
	w1, w2 := t&mask32, t>>32
	w1 += a0 * b1
	hi = a1*b1 + w2 + w1>>32
	lo = a * b
	return
}

// Normal is a standard normal draw (Marsaglia's polar method, one of the pair).
func (r *Random) Normal() float64 {
	for {
		u, v := 2*r.Float64()-1, 2*r.Float64()-1
		s := u*u + v*v
		if s > 0 && s < 1 {
			return u * math.Sqrt(-2*math.Log(s)/s)
		}
	}
}

// Gamma draws from a gamma distribution of this shape and scale 1 (Marsaglia and Tsang 2000).
func (r *Random) Gamma(shape float64) float64 {
	if shape < 1 {
		// Boost: Gamma(a) = Gamma(a+1)·U^(1/a).
		return r.Gamma(shape+1) * math.Pow(r.Float64(), 1/shape)
	}
	d := shape - 1.0/3
	c := 1 / math.Sqrt(9*d)
	for {
		x := r.Normal()
		v := 1 + c*x
		if v <= 0 {
			continue
		}
		v = v * v * v
		u := r.Float64()
		if u < 1-0.0331*x*x*x*x || math.Log(u) < 0.5*x*x+d*(1-v+math.Log(v)) {
			return d * v
		}
	}
}

// Poisson draws a Poisson count of this mean: by inversion for a small mean, else by the PTRS
// transformed rejection (Hörmann 1993).
func (r *Random) Poisson(mean float64) int {
	if mean <= 0 {
		return 0
	}
	if mean < 10 {
		limit, product, k := math.Exp(-mean), r.Float64(), 0
		for product > limit {
			k++
			product *= r.Float64()
		}
		return k
	}
	logMean := math.Log(mean)
	b := 0.931 + 2.53*math.Sqrt(mean)
	a := -0.059 + 0.02483*b
	invAlpha := 1.1239 + 1.1328/(b-3.4)
	vr := 0.9277 - 3.6224/(b-2)
	for {
		u := r.Float64() - 0.5
		v := r.Float64()
		us := 0.5 - math.Abs(u)
		k := math.Floor((2*a/us+b)*u + mean + 0.43)
		if us >= 0.07 && v <= vr {
			return int(k)
		}
		if k < 0 || (us < 0.013 && v > us) {
			continue
		}
		lg, _ := math.Lgamma(k + 1)
		if math.Log(v)+math.Log(invAlpha)-math.Log(a/(us*us)+b) <= -mean+k*logMean-lg {
			return int(k)
		}
	}
}

// NegBinomial draws a negative binomial count of this mean and size (dispersion: the variance is
// mean + mean²/size), as a gamma–Poisson mixture.
func (r *Random) NegBinomial(mean, size float64) int {
	if mean <= 0 {
		return 0
	}
	return r.Poisson(r.Gamma(size) * mean / size)
}

// Discrete draws an index with probability proportional to its cumulative weight's step:
// cumulative is increasing, its last value the total.
func (r *Random) Discrete(cumulative []float64) int {
	x := r.Float64() * cumulative[len(cumulative)-1]
	lo, hi := 0, len(cumulative)-1
	for lo < hi {
		mid := (lo + hi) / 2
		if cumulative[mid] > x {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	return lo
}

// cumulate answers the running sums of weights.
func cumulate(weights []float64) []float64 {
	out := make([]float64, len(weights))
	sum := 0.0
	for i, w := range weights {
		sum += w
		out[i] = sum
	}
	return out
}
