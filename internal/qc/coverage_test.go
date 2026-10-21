package qc

import (
	"math"
	"os"
	"testing"
)

// iNEXT's spider data (Sackett et al. 2011, as iNEXT distributes them): its sample coverage
// (DataInfo's SC) is 0.9289 for Girdled and 0.9446 for Logged.
func TestCoverageAgreesWithINEXT(t *testing.T) {
	counts := func(xs ...int) (n, f1, f2 float64) {
		for _, x := range xs {
			n += float64(x)
			if x == 1 {
				f1++
			}
			if x == 2 {
				f2++
			}
		}
		return
	}
	rep := func(v, k int) []int {
		out := make([]int, k)
		for i := range out {
			out[i] = v
		}
		return out
	}
	girdled := append([]int{46, 22, 17, 15, 15, 9, 8, 6, 6, 4}, append(rep(2, 4), rep(1, 12)...)...)
	logged := append([]int{88, 22, 16, 15, 13, 10, 8, 8, 7, 7, 7, 5, 4, 4, 4, 3, 3, 3, 3}, append(rep(2, 4), rep(1, 14)...)...)
	for _, c := range []struct {
		name   string
		counts []int
		want   float64
	}{{"Girdled", girdled, 0.9289}, {"Logged", logged, 0.9446}} {
		got := Coverage(counts(c.counts...))
		if math.Abs(got-c.want) > 5e-5 {
			t.Errorf("%s: %.6f, iNEXT %.4f", c.name, got, c.want)
		}
	}
	if Coverage(10, 0, 3) != 1 || Coverage(0, 0, 0) != 0 {
		t.Error("edge cases")
	}
	// Without doubletons: the f2 = 0 form.
	if got, want := Coverage(20, 4, 0), 1-(4.0/20)*(19.0*3)/(19.0*3+2); math.Abs(got-want) > 1e-15 {
		t.Errorf("f2 = 0: %v, want %v", got, want)
	}
}

func writeFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}
