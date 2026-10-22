package qc

// Sample coverage: the share of a repertoire's abundance that belongs to clonotypes the sample
// observed, estimated from its singletons and doubletons by Chao and Jost's improvement of Good's
// estimator (Chao A, Jost L. Coverage-based rarefaction and extrapolation: standardizing samples by
// completeness rather than size. Ecology 2012;93:2533–2547, doi:10.1890/11-1952.1; Chao et al.
// 2014, Ecol Monogr 84:45, Eq. 12 and Appendix G for the case without doubletons):
//
//	Ĉ = 1 − (f1/n) · (n−1)f1 / ((n−1)f1 + 2f2)          f2 > 0
//	Ĉ = 1 − (f1/n) · (n−1)(f1−1) / ((n−1)(f1−1) + 2)    f2 = 0
//
// where n is the sample's total count and f1, f2 the numbers of clonotypes counted once and twice.
// It is iNEXT's sample coverage. Counts must be whole numbers (reads, UMIs, templates).
func Coverage(n, f1, f2 float64) float64 {
	if n <= 0 {
		return 0
	}
	if f1 == 0 {
		return 1
	}
	var factor float64
	if f2 > 0 {
		a := float64((n - 1) * f1)
		factor = a / (a + float64(2*f2))
	} else {
		a := float64((n - 1) * (f1 - 1))
		factor = a / (a + 2)
	}
	return 1 - float64(float64(f1/n)*factor)
}
