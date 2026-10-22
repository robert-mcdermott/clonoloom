// The beta-binomial test and its variance model (internal/expansion) against an independent R
// implementation (validation/reference/betabinomial.R): p-values on 150 cases, two-sided and
// one-sided; the log-likelihood of two sets of clonotypes drawn from the model; and the model
// fitted on them by maximum likelihood, which must reach R's likelihood and give R's scale.
package validation

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"clonoloom/internal/expansion"
)

type betaBinomialReference struct {
	Made  string `json:"made"`
	Cases []struct {
		K     int64      `json:"k"`
		N     int64      `json:"n"`
		Theta float64    `json:"theta"`
		V     float64    `json:"v"`
		P     [3]float64 `json:"p"`
	} `json:"cases"`
	Sets []struct {
		Theta        float64         `json:"theta"`
		Truth        expansion.Model `json:"truth"`
		Observations []struct {
			K     int64 `json:"k"`
			N     int64 `json:"n"`
			Count int   `json:"count"`
		} `json:"observations"`
		LogLikAtOne   float64 `json:"logLikAtOne"`
		LogLikAtTruth float64 `json:"logLikAtTruth"`
		Fitted        struct {
			A      float64 `json:"a"`
			B      float64 `json:"b"`
			LogLik float64 `json:"logLik"`
		} `json:"fitted"`
	} `json:"sets"`
}

func TestBetaBinomialAgainstR(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("reference", "betabinomial.json"))
	if err != nil {
		t.Fatal(err)
	}
	var ref betaBinomialReference
	if err := json.Unmarshal(raw, &ref); err != nil {
		t.Fatal(err)
	}
	worst := 0.0
	for _, c := range ref.Cases {
		for k, alt := range []string{expansion.TwoSided, expansion.Increase, expansion.Decrease} {
			got := expansion.BetaBinomialP(c.K, c.N, c.Theta, c.V, alt)
			if !closeP(got, c.P[k]) {
				t.Errorf("%s: %d of %d at θ %g, v %g: %.17g, R %.17g", alt, c.K, c.N, c.Theta, c.V, got, c.P[k])
			}
			if w := math.Min(c.P[k], 1); w > 1e-300 {
				worst = math.Max(worst, math.Abs(got-w)/w)
			}
		}
	}
	t.Logf("%d cases against %s: the largest relative difference %.2g", len(ref.Cases), ref.Made, worst)
	for i, set := range ref.Sets {
		var obs []expansion.Observation
		for _, o := range set.Observations {
			obs = append(obs, expansion.Observation{K: o.K, N: o.N, Theta: set.Theta, Count: o.Count})
		}
		for _, at := range []struct {
			m    expansion.Model
			want float64
		}{{expansion.Model{A: 1, B: 1}, set.LogLikAtOne}, {set.Truth, set.LogLikAtTruth}} {
			if got := expansion.LogLikelihood(at.m, obs); math.Abs(got-at.want) > 1e-9*math.Abs(at.want) {
				t.Errorf("set %d at %+v: log-likelihood %.12f, R %.12f", i, at.m, got, at.want)
			}
		}
		fit, err := expansion.Fit(obs)
		if err != nil {
			t.Fatal(err)
		}
		r := expansion.Model{A: set.Fitted.A, B: set.Fitted.B}
		t.Logf("set %d, truth %+v: fitted (%.5f, %.5f) in %d iterations, log-likelihood %.6f; R (%.5f, %.5f), %.6f", i, set.Truth, fit.Model.A, fit.Model.B, fit.Iterations, fit.LogLik, r.A, r.B, set.Fitted.LogLik)
		if fit.LogLik < set.Fitted.LogLik-1e-6 {
			t.Errorf("set %d: the fit reaches %.9f, R %.9f", i, fit.LogLik, set.Fitted.LogLik)
		}
		for _, n := range []float64{5, 50, 500, 5000} {
			if got, want := fit.Model.V(n), r.V(n); math.Abs(got-want) > 1e-3*want {
				t.Errorf("set %d: v(%g) %.6f, R's %.6f", i, n, got, want)
			}
		}
	}
}
