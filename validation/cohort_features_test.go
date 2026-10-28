package validation

import (
	"encoding/json"
	"math"
	"os"
	"testing"

	"clonoloom/internal/diversity"
	"clonoloom/internal/study"
)

func TestCohortFeaturesAgainstRawAbundanceReference(t *testing.T) {
	raw, err := os.ReadFile("reference/cohort_features.json")
	if err != nil {
		t.Fatal(err)
	}
	var ref struct {
		Cases []struct {
			Name     string
			Counts   []float64
			Expected map[string]*float64
		}
	}
	if err = json.Unmarshal(raw, &ref); err != nil {
		t.Fatal(err)
	}
	input := study.CohortInput{Dataset: "raw-abundance-reference", Spec: study.CohortSpec{Unit: "subject", Layout: "independent", GroupField: "condition", Levels: []string{"A", "B"}}}
	samples := map[string]diversity.Result{}
	expected := map[string]map[string]*float64{}
	for i, c := range ref.Cases {
		group := "A"
		if i >= len(ref.Cases)/2 {
			group = "B"
		}
		input.Samples = append(input.Samples, study.CohortSample{Name: c.Name, Subject: c.Name, Condition: group, Batch: "shared"})
		r := diversity.Measure(c.Counts, diversity.Options{})
		samples[c.Name] = r
		expected[c.Name] = c.Expected
	}
	table, err := study.SubjectFeatures(input, samples, "umis")
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range table.Rows {
		for i, f := range table.Catalog {
			want := expected[row.Name][f.ID]
			got := row.Values[i]
			if want == nil {
				if !math.IsNaN(float64(got.Value)) || got.Missing == "" {
					t.Fatalf("%s %s: expected missing: %+v", row.Name, f.ID, got)
				}
			} else if got.Missing != "" || math.IsNaN(float64(got.Value)) || math.Abs(float64(got.Value)-*want) > 1e-12*math.Max(1, math.Abs(*want)) {
				t.Fatalf("%s %s: %v want %v", row.Name, f.ID, got, *want)
			}
		}
	}
}
