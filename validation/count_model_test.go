package validation

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"testing"

	"clonoloom/internal/countmodel"
	"clonoloom/internal/dmath"
)

func TestCountModelAgainstStatsmodels(t *testing.T) {
	raw, e := os.ReadFile("reference/count_model.json")
	if e != nil {
		t.Fatal(e)
	}
	var ref struct {
		Cases []struct {
			Name                                           string
			X                                              [][]float64
			Y, Offset, Beta, Mu                            []float64
			Covariance                                     [][]float64
			Alpha, LogLikelihood, NullLogLikelihood, LR, P float64
			Fixed                                          struct {
				Alpha, LogLikelihood float64
				Beta                 []float64
			}
		}
	}
	if e = json.Unmarshal(raw, &ref); e != nil {
		t.Fatal(e)
	}
	if len(ref.Cases) != 6 {
		t.Fatal("missing references")
	}
	for _, c := range ref.Cases {
		t.Run(c.Name, func(t *testing.T) {
			close := func(label string, got, want, tol float64) {
				t.Helper()
				if math.IsNaN(got) || math.Abs(got-want) > tol*math.Max(1, math.Abs(want)) {
					t.Errorf("%s %.12g want %.12g", label, got, want)
				}
			}
			fixed, e := countmodel.FitNB(context.Background(), c.X, c.Y, c.Offset, c.Fixed.Alpha)
			if e != nil {
				t.Fatal(e)
			}
			close("fixed loglik", fixed.LogLikelihood, c.Fixed.LogLikelihood, 1e-9)
			for i, b := range fixed.Beta {
				close("fixed beta", b, c.Fixed.Beta[i], 1e-7)
			}
			alpha, f, e := countmodel.FitDispersion(context.Background(), c.X, c.Y, c.Offset)
			if e != nil {
				t.Fatal(e)
			}
			close("dispersion", alpha, c.Alpha, 2e-5)
			close("loglik", f.LogLikelihood, c.LogLikelihood, 2e-6)
			for i, b := range f.Beta {
				close("beta", b, c.Beta[i], 2e-5)
				for j, v := range f.Covariance[i] {
					close("covariance", v, c.Covariance[i][j], 2e-5)
				}
			}
			for i, v := range f.Mu {
				close("fitted", v, c.Mu[i], 2e-5)
			}
			nullX := make([][]float64, len(c.X))
			for i, x := range c.X {
				nullX[i] = append([]float64{x[0]}, x[2:]...)
			}
			null, e := countmodel.FitNB(context.Background(), nullX, c.Y, c.Offset, alpha)
			if e != nil {
				t.Fatal(e)
			}
			close("null loglik", null.LogLikelihood, c.NullLogLikelihood, 2e-6)
			lr := 2 * (f.LogLikelihood - null.LogLikelihood)
			close("LR", lr, c.LR, 2e-5)
			close("p", dmath.Erfc(math.Sqrt(lr/2)), c.P, 2e-5)
		})
	}
}
