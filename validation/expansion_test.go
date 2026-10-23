// The two-sample tests (internal/expansion) against R (validation/reference/expansion.R): Fisher's
// exact test and the binomial test, two-sided and one-sided, on 215 cases from a clonotype absent
// from one sample to thousands in samples of a million; Benjamini and Hochberg's adjusted
// p-values; and Storey's q-values at λ = 0.5.
package validation

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"clonoloom/internal/clonotype"
	"clonoloom/internal/expansion"
	"clonoloom/internal/reads"
	"clonoloom/internal/simulate"
	"clonoloom/internal/study"
	"clonoloom/internal/track"
)

type expansionReference struct {
	Made  string `json:"made"`
	Cases []struct {
		K1       int64      `json:"k1"`
		K2       int64      `json:"k2"`
		N1       int64      `json:"n1"`
		N2       int64      `json:"n2"`
		Fisher   [3]float64 `json:"fisher"`
		Binomial [3]float64 `json:"binomial"`
	} `json:"cases"`
	BH     []float64 `json:"bh"`
	Storey struct {
		Pi0 float64   `json:"pi0"`
		Q   []float64 `json:"q"`
	} `json:"storey"`
}

// close says whether two p-values agree: relatively to 10⁻⁹, or both below 10⁻³⁰⁰; R's are not
// held to at most 1 (its fisher.test can answer 1 + 2⁻⁵²).
func closeP(got, want float64) bool {
	want = math.Min(want, 1)
	if want < 1e-300 && got < 1e-300 {
		return true
	}
	return math.Abs(got-want) <= 1e-9*want
}

func TestExpansionTestsAgainstR(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("reference", "expansion.json"))
	if err != nil {
		t.Fatal(err)
	}
	var ref expansionReference
	if err := json.Unmarshal(raw, &ref); err != nil {
		t.Fatal(err)
	}
	alts := []string{expansion.TwoSided, expansion.Increase, expansion.Decrease}
	var ps []float64
	worst := 0.0
	for _, c := range ref.Cases {
		for k, alt := range alts {
			f := expansion.FisherP(c.K1, c.K2, c.N1, c.N2, alt)
			b := expansion.BinomialP(c.K1, c.K1+c.K2, float64(c.N1)/float64(c.N1+c.N2), alt)
			if !closeP(f, c.Fisher[k]) {
				t.Errorf("Fisher, %s: %d of %d, %d of %d: %.17g, R %.17g", alt, c.K1, c.N1, c.K2, c.N2, f, c.Fisher[k])
			}
			if !closeP(b, c.Binomial[k]) {
				t.Errorf("binomial, %s: %d of %d, %d of %d: %.17g, R %.17g", alt, c.K1, c.N1, c.K2, c.N2, b, c.Binomial[k])
			}
			for _, pair := range [][2]float64{{f, c.Fisher[k]}, {b, c.Binomial[k]}} {
				if w := math.Min(pair[1], 1); w > 1e-300 {
					worst = math.Max(worst, math.Abs(pair[0]-w)/w)
				}
			}
		}
		ps = append(ps, expansion.BinomialP(c.K1, c.K1+c.K2, float64(c.N1)/float64(c.N1+c.N2), expansion.TwoSided))
	}
	t.Logf("%d cases against %s: the largest relative difference %.2g", len(ref.Cases), ref.Made, worst)
	bh := expansion.AdjustBH(ps)
	pi0 := expansion.StoreyPi0(ps, expansion.StoreyLambda)
	if math.Abs(pi0-ref.Storey.Pi0) > 1e-15 {
		t.Errorf("π₀ %.17g, R %.17g", pi0, ref.Storey.Pi0)
	}
	for i := range bh {
		if !closeP(bh[i], ref.BH[i]) || !closeP(pi0*bh[i], ref.Storey.Q[i]) {
			t.Errorf("case %d: BH %.17g (R %.17g), Storey %.17g (R %.17g)", i, bh[i], ref.BH[i], pi0*bh[i], ref.Storey.Q[i])
		}
	}
}

// openSeries makes a preset's series with the variance between samples given, and answers its
// clonotypes (amino-acid junction, V and J as called, as the truth keys them), the samples' names,
// the study's design and the truth.
func openSeries(t *testing.T, name string, variance float64) (*clonotype.Table, []string, study.Design, seriesTruth, string) {
	preset := simulate.Presets[name]
	series := *preset.Series
	series.Variance = variance
	preset.Series = &series
	dir := filepath.Join(t.TempDir(), name)
	if _, err := simulate.Run(context.Background(), preset, dir, "validation", nil); err != nil {
		t.Fatal(err)
	}
	var truth seriesTruth
	raw, _ := os.ReadFile(filepath.Join(dir, "truth", "truth.json"))
	if err := json.Unmarshal(raw, &truth); err != nil {
		t.Fatal(err)
	}
	var paths []string
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tsv") || strings.HasSuffix(e.Name(), ".json") {
			paths = append(paths, filepath.Join(dir, e.Name()))
		}
	}
	res, err := reads.Open(context.Background(), paths)
	if err != nil {
		t.Fatal(err)
	}
	table, err := clonotype.Compute(res.Table, nil, clonotype.Definition{Junction: "aa", V: "allele", J: "allele", ProductiveOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, s := range res.Table.Samples {
		names = append(names, s.Name)
	}
	var samples []study.Sample
	for _, r := range res.Repertoires {
		samples = append(samples, study.Sample{Name: r.RepertoireID, Subject: r.SubjectID, Time: r.TimePoint, Tissue: r.Tissue})
	}
	return table, names, study.Describe(samples), truth, dir
}

// Where nothing changes (the null series, day 0 against day 14, two samples each), every call is a
// false discovery. Without variance between samples beyond sampling, no test calls anything at a
// false discovery rate of 0.01. With the series' variance, the binomial and Fisher's tests call some
// (about 0.2% of those tested, as Rytlewski et al. found between blood samples two weeks apart);
// the beta-binomial test, its variance fitted on the replicate samples, (almost) none.
func TestExpansionTestsOnANullSeries(t *testing.T) {
	if raceDetector {
		t.Skip("measured without the race detector (CI's data job), which slows its simulations tenfold")
	}
	for _, c := range []struct {
		variance float64
		test     string
		check    func(calls int) bool
		want     string
	}{
		{0, expansion.Binomial, func(n int) bool { return n <= 2 }, "at most 2"},
		{0, expansion.Fisher, func(n int) bool { return n <= 2 }, "at most 2"},
		{0, expansion.Auto, func(n int) bool { return n <= 2 }, "at most 2"},
		{0.1, expansion.Binomial, func(n int) bool { return n >= 5 }, "some, the test accounting for sampling alone"},
		{0.1, expansion.Fisher, func(n int) bool { return n >= 5 }, "some, the test accounting for sampling alone"},
		{0.1, expansion.Auto, func(n int) bool { return n <= 2 }, "at most 2, the beta-binomial test allowing for the variance"},
	} {
		table, names, design, _, _ := openSeries(t, "null-series", c.variance)
		res, err := track.Test(table, names, design, track.Comparison{From: "day 0", To: "day 14", Options: expansion.Options{Test: c.test}})
		if err != nil {
			t.Fatal(err)
		}
		calls, tested := 0, 0
		for _, s := range res.Subjects {
			calls += s.Increased + s.Decreased
			tested += s.Tested
		}
		t.Logf("variance %.1f, %s (%s): %d of %d clonotypes in %d subjects called at a false discovery rate of 0.01 (%.4f of those tested)", c.variance, c.test, res.Test, calls, tested, len(res.Subjects), float64(calls)/float64(tested))
		if res.Fit != nil {
			t.Logf("  fitted on %d pairs, %d clonotypes: %+v; between them, a test at 0.05 calls %.3f by sampling alone, %.3f by the model", len(res.Fit.Pairs), res.Fit.Clonotypes, res.Fit.Model, res.Fit.CalledBySampling, res.Fit.CalledByModel)
		}
		if len(res.Subjects) != 4 || !c.check(calls) || (c.test == expansion.Auto) != (res.Test == expansion.BetaBinomial) {
			t.Errorf("variance %.1f, %s: %d subjects, %d calls, expected %s", c.variance, c.test, len(res.Subjects), calls, c.want)
		}
		// Between the replicate samples, the model calls about the 5% a test at 0.05 should.
		if res.Fit != nil && (res.Fit.CalledByModel < 0.02 || res.Fit.CalledByModel > 0.08 || len(res.Fit.Pairs) != 12) {
			t.Errorf("variance %.1f: %d pairs; the model calls %.3f between replicates", c.variance, len(res.Fit.Pairs), res.Fit.CalledByModel)
		}
	}
}

// The CAR-T series between the week before the infusion and two weeks after, when the infusion
// product is a large share of the blood and a few of its clones have grown 20 times: a call is
// right where the truth's expected frequency moved the way it was called. Without variance between
// samples beyond sampling, nearly every call is right. With the series' variance, the binomial
// test's calls are wrong more often than the false discovery rate asked; the beta-binomial test's,
// its variance fitted on the samples taken twice before the infusion, not, at the cost of calling
// fewer of the clones that did change. How many of the expanding clones tested are called is said
// too: most are small.
func TestExpansionTestsOnACARTSeries(t *testing.T) {
	if raceDetector {
		t.Skip("measured without the race detector (CI's data job), which slows its simulations tenfold")
	}
	for _, c := range []struct {
		variance float64
		test     string
		check    func(wrong float64) bool
		want     string
	}{
		{0, expansion.Binomial, func(w float64) bool { return w <= 0.02 }, "at most 0.02 of the calls against the truth"},
		{0.1, expansion.Binomial, func(w float64) bool { return w > 0.02 }, "more than 0.02 against the truth, the test accounting for sampling alone"},
		{0.1, expansion.Auto, func(w float64) bool { return w <= 0.02 }, "at most 0.02 against the truth, the beta-binomial test allowing for the variance"},
	} {
		table, names, design, truth, dir := openSeries(t, "car-t", c.variance)
		res, err := track.Test(table, names, design, track.Comparison{From: "day -7", To: "day 14", Options: expansion.Options{Test: c.test}})
		if err != nil {
			t.Fatal(err)
		}
		calls, wrong, expanding, found := 0, 0, 0, 0
		for _, s := range res.Subjects {
			var file string
			for _, st := range truth.Series {
				if st.Subject == s.Subject {
					file = st.File
				}
			}
			clonotypes := readSeriesFile(t, filepath.Join(dir, file))
			for _, x := range s.Clonotypes {
				ch := x.Chains[0]
				row := clonotypes[ch.V+"|"+ch.J+"|"+ch.Junction]
				before, after := row.frequency[0], row.frequency[2]
				if x.Call != "" {
					calls++
					if (x.Call == expansion.Increase) != (after > before) || after == before {
						wrong++
					}
				}
				if row.trajectory == "product expanding" {
					expanding++
					if x.Call == expansion.Increase {
						found++
					}
				}
			}
		}
		share := float64(wrong) / float64(calls)
		t.Logf("variance %.1f, %s (%s): %d calls, %d against the truth (%.3f); %d of %d expanding clones tested called increased", c.variance, c.test, res.Test, calls, wrong, share, found, expanding)
		if calls < 10 || !c.check(share) || found == 0 {
			t.Errorf("variance %.1f, %s: %d calls, %d against the truth, %d of %d expanding clones; expected %s", c.variance, c.test, calls, wrong, found, expanding, c.want)
		}
	}
}
