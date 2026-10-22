// Classes across time points and attribution (internal/track) against the truth of simulated
// studies over time. Classes: on the CAR-T example (its variance between blood samples, the
// beta-binomial test fitted on the samples taken twice before the infusion), each clonotype classed
// emerging, contracting or transient is right where the truth's expected frequency moved the way
// its calls say. Attribution: each blood sample's share in the clonotypes of its reference sample,
// against the truth's share in the reference population's clonotypes, which it underestimates by
// the clonotypes the reference sample missed; and before the infusion, the share that owes nothing
// to it.
package validation

import (
	"math"
	"path/filepath"
	"strings"
	"testing"

	"clonoloom/internal/expansion"
	"clonoloom/internal/track"
)

func TestClassesOfACARTSeries(t *testing.T) {
	table, names, design, truth, dir := openSeries(t, "car-t", 0.1)
	res, err := track.Classify(table, names, design, track.Classing{Base: "day -7"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Test != expansion.BetaBinomial || len(res.Subjects) != 3 {
		t.Fatalf("%s, %d subjects", res.Test, len(res.Subjects))
	}
	// The truth's expected frequency at the base and each later time point.
	times := map[float64]int{}
	for i, tp := range truth.Study.Series.Times {
		times[tp.Day] = i
	}
	counts := map[string]int{}
	classed, wrong, expanding, expandingFound := 0, 0, 0, 0
	for _, s := range res.Subjects {
		var file string
		for _, st := range truth.Series {
			if st.Subject == s.Subject {
				file = st.File
			}
		}
		clonotypes := readSeriesFile(t, filepath.Join(dir, file))
		base := times[*s.Base.Days]
		for _, c := range s.Clonotypes {
			counts[c.Class]++
			ch := c.Chains[0]
			row := clonotypes[ch.V+"|"+ch.J+"|"+ch.Junction]
			if row.trajectory == "product expanding" {
				expanding++
				if c.Class == track.Emerging || c.Class == track.Transient {
					expandingFound++
				}
			}
			if c.Class == track.Persistent {
				continue
			}
			classed++
			right := true
			for k, call := range c.Calls {
				f0, f1 := row.frequency[base], row.frequency[times[*s.Later[k].Days]]
				if (call == expansion.Increase && !(f1 > f0)) || (call == expansion.Decrease && !(f1 < f0)) {
					right = false
				}
			}
			if !right {
				wrong++
			}
		}
	}
	t.Logf("%d clonotypes classed by their calls (%d emerging, %d transient, %d contracting), %d against the truth; %d persistent listed; %d of the expanding clones listed classed emerging or transient",
		classed, counts[track.Emerging], counts[track.Transient], counts[track.Contracting], wrong, counts[track.Persistent], expandingFound)
	if classed < 10 || float64(wrong) > 0.05*float64(classed) {
		t.Errorf("%d classed, %d against the truth", classed, wrong)
	}
}

// Attribution on the CAR-T and transplant examples (with their variance between blood samples).
// The share in the reference's clonotypes, against the share the truth expects in the clonotypes
// the reference sample holds, and against the truth's share in all the reference population's
// clonotypes (which it underestimates: the reference sample misses some). The share derived from
// the reference, against the reference's true share of the blood: told for the transplant's donors,
// withheld for the CAR-T products, made from each patient's blood.
func TestAttributionAgainstTheTruth(t *testing.T) {
	for _, preset := range []string{"car-t", "transplant"} {
		table, names, design, truth, dir := openSeries(t, preset, 0.1)
		res, err := track.Attribute(table, names, design, 0.95)
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Subjects) != 3 || len(res.Without) != 0 {
			t.Fatalf("%s: %+v", preset, res)
		}
		population, infused := map[string]float64{}, map[string]float64{}
		day := map[string]float64{}
		var references []string
		for _, s := range truth.Samples {
			if s.Reference {
				references = append(references, s.ID)
			}
			if s.Attribution != nil {
				population[s.ID], infused[s.ID], day[s.ID] = *s.Attribution, *s.ReferenceShare, *s.Day
			}
		}
		times := map[float64]int{}
		for i, tp := range truth.Study.Series.Times {
			times[tp.Day] = i
		}
		var diff, worst, gap, derivedWorst float64
		n, held, derived := 0, 0, 0
		for _, s := range res.Subjects {
			if (preset == "transplant") != s.Derived || s.Overlap == nil {
				t.Errorf("%s %s: derived %v, overlap %v (%s)", preset, s.Subject, s.Derived, s.Overlap, s.Why)
			}
			// What the reference sample holds, and the truth's expected frequencies.
			var file string
			for _, st := range truth.Series {
				if st.Subject == s.Subject {
					file = st.File
				}
			}
			clonotypes := readSeriesFile(t, filepath.Join(dir, file))
			var refFile string
			for _, r := range references {
				if strings.HasPrefix(r, s.Subject+"-") {
					refFile = r
				}
			}
			heldByRef, _ := sampleClonotypes(t, filepath.Join(dir, refFile+".tsv"))
			for _, x := range s.Samples {
				want := 0.0
				for key, c := range clonotypes {
					if heldByRef[key] > 0 {
						want += c.frequency[times[day[x.Name]]]
					}
				}
				d := float64(x.Value.Frequency) - want
				diff += math.Abs(d)
				worst = math.Max(worst, math.Abs(d))
				gap += population[x.Name] - float64(x.Value.Frequency)
				n++
				if float64(x.Value.Lower) <= want && want <= float64(x.Value.Upper) {
					held++
				}
				if x.From != nil {
					derived++
					derivedWorst = math.Max(derivedWorst, math.Abs(float64(x.From.Frequency)-infused[x.Name]))
				}
			}
		}
		t.Logf("%s: %d samples; the share in the reference sample's clonotypes within %.4f of the truth's (%.4f on average), its interval holding it in %d; %.3f below the share in the whole reference population's clonotypes on average; the share derived from the reference told for %d samples, at most %.3f from the truth",
			preset, n, worst, diff/float64(n), held, gap/float64(n), derived, derivedWorst)
		if worst > 0.03 || (preset == "transplant" && (derived != 12 || derivedWorst > 0.07)) || (preset == "car-t" && derived != 0) {
			t.Errorf("%s: within %.4f, %d derived at most %.3f from the truth", preset, worst, derived, derivedWorst)
		}
	}
}
