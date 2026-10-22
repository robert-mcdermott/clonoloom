// ClonoLoom's readers and QC on a simulated study, against its truth (internal/simulate): every
// sample read with its metadata, no finding, its clonotypes the truth's count, and the sample
// coverage QC estimates (Chao and Jost 2012) near the coverage a sample of its depth has on
// average.
package validation

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"clonoloom/internal/qc"
	"clonoloom/internal/reads"
	"clonoloom/internal/simulate"
	"clonoloom/internal/validate"
)

func TestQCOfASimulatedStudyAgainstItsTruth(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "standard")
	if _, err := simulate.Run(context.Background(), simulate.Presets["standard"], dir, "validation", nil); err != nil {
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
	if report := validate.Check(res); len(report.Findings) != 0 || len(res.Repertoires) != 12 {
		t.Fatalf("%d findings, %d repertoires", len(report.Findings), len(res.Repertoires))
	}
	var truth struct {
		Samples []struct {
			ID               string  `json:"id"`
			Rows             int     `json:"rows"`
			Clonotypes       int     `json:"clonotypes"`
			ExpectedCoverage float64 `json:"expectedCoverage"`
		} `json:"samples"`
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "truth", "truth.json"))
	json.Unmarshal(raw, &truth)
	report := qc.Compute(res.Table, qc.Filter{})
	if len(report.Samples) != len(truth.Samples) {
		t.Fatalf("%d samples, the truth %d", len(report.Samples), len(truth.Samples))
	}
	largest := 0.0
	for _, want := range truth.Samples {
		var got *qc.Sample
		for i := range report.Samples {
			if report.Samples[i].Name == want.ID {
				got = &report.Samples[i]
			}
		}
		if got == nil || got.Rows != want.Rows || got.Clonotypes != want.Clonotypes {
			t.Errorf("%s: %+v; the truth %+v", want.ID, got, want)
			continue
		}
		d := math.Abs(float64(got.Coverage) - want.ExpectedCoverage)
		largest = math.Max(largest, d)
		if d > 0.01 {
			t.Errorf("%s: coverage %.4f, expected %.4f", want.ID, float64(got.Coverage), want.ExpectedCoverage)
		}
	}
	t.Logf("largest difference between the estimated and the expected coverage: %.4f", largest)
}
