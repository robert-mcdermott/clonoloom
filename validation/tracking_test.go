// Tracking (internal/track) against the truth of simulated studies over time: each clonotype's
// frequency in each blood sample with its 95% interval (Wilson's), against the frequency the
// truth expects there. Where samples differ only by sampling, the intervals hold the truth about
// 95% of the time; where blood samples also differ biologically (as the CAR-T example's do, and
// as real ones do), they hold it far less often: Wilson's interval is for sampling alone, which is
// why the expansion tests (slices 6 and 7) model the biological variance from replicates.
package validation

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"clonoloom/internal/clonotype"
	"clonoloom/internal/reads"
	"clonoloom/internal/simulate"
	"clonoloom/internal/study"
	"clonoloom/internal/track"
)

func TestTrackingIntervalsAgainstTheTruth(t *testing.T) {
	if raceDetector {
		t.Skip("measured without the race detector (CI's data job), which slows its simulations tenfold")
	}
	for _, c := range []struct {
		name     string
		variance float64
		check    func(coverage float64) bool
		want     string
	}{
		{"sampling only", 0, func(c float64) bool { return c > 0.935 && c < 0.965 }, "within 0.935 and 0.965"},
		{"with biological variance", 0.1, func(c float64) bool { return c < 0.8 }, "below 0.8, the intervals being for sampling alone"},
	} {
		t.Run(c.name, func(t *testing.T) {
			covered, checked := trackingCoverage(t, c.variance)
			coverage := float64(covered) / float64(checked)
			t.Logf("variance %.1f between samples: the 95%% intervals hold the truth in %d of %d (%.3f)", c.variance, covered, checked, coverage)
			if !c.check(coverage) {
				t.Errorf("coverage %.3f, expected %s", coverage, c.want)
			}
		})
	}
}

// trackingCoverage makes the CAR-T example with the variance between samples given, follows each
// subject's 150 clonotypes the truth expects most of before the infusion, and counts the blood
// samples' intervals that hold the frequency the truth expects at their time point.
func trackingCoverage(t *testing.T, variance float64) (covered, checked int) {
	preset := simulate.Presets["car-t"]
	series := *preset.Series
	series.Variance = variance
	preset.Series = &series
	dir := filepath.Join(t.TempDir(), "car-t")
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
	// Clonotypes as the truth keys them: V and J as called, the junction's amino acids.
	table, err := clonotype.Compute(res.Table, nil, clonotype.Definition{Junction: "aa", V: "allele", J: "allele", ProductiveOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	idOf := map[string]string{}
	for _, c := range table.Clonotypes {
		ch := c.Chains[0]
		idOf[ch.V+"|"+ch.J+"|"+ch.Junction] = c.ID
	}
	var names []string
	for _, s := range res.Table.Samples {
		names = append(names, s.Name)
	}
	times := map[float64]int{}
	for i, tp := range series.Times {
		times[tp.Day] = i
	}
	for _, st := range truth.Series {
		clonotypes := readSeriesFile(t, filepath.Join(dir, st.File))
		keys := make([]string, 0, len(clonotypes))
		for key := range clonotypes {
			if idOf[key] != "" {
				keys = append(keys, key)
			}
		}
		sort.Slice(keys, func(a, b int) bool {
			fa, fb := clonotypes[keys[a]].frequency[0], clonotypes[keys[b]].frequency[0]
			if fa != fb {
				return fa > fb
			}
			return keys[a] < keys[b]
		})
		keys = keys[:min(150, len(keys))]
		keyOf := map[string]string{}
		var list []string
		for _, key := range keys {
			list = append(list, idOf[key])
			keyOf[idOf[key]] = key
		}
		var samples []study.Sample
		for _, r := range res.Repertoires {
			if r.SubjectID == st.Subject {
				samples = append(samples, study.Sample{Name: r.RepertoireID, Subject: r.SubjectID, Time: r.TimePoint, Tissue: r.Tissue})
			}
		}
		got, err := track.Follow(table, names, study.Describe(samples), track.Choice{By: track.ByList, List: list}, 0.95)
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Subjects) != 1 || len(got.Subjects[0].Clonotypes) != len(list) || !got.Intervals {
			t.Fatalf("%s: %+v", st.Subject, got)
		}
		s := got.Subjects[0]
		for _, c := range s.Clonotypes {
			for k, v := range c.Samples {
				p := s.Points[s.Samples[k].Point]
				if p.Days == nil || p.Reference != "" {
					continue // the infusion product: its truth is not a blood sample's
				}
				f := clonotypes[keyOf[c.ID]].frequency[times[*p.Days]]
				checked++
				if float64(v.Lower) <= f && f <= float64(v.Upper) {
					covered++
				}
			}
		}
	}
	return covered, checked
}
