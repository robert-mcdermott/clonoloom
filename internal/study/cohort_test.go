package study

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func cohortFixture() CohortInput {
	return CohortInput{Dataset: "source-identity", Spec: CohortSpec{Unit: "subject", Layout: "independent", GroupField: "condition", Levels: []string{"A", "B"}}, Samples: []CohortSample{
		{Name: "a", Subject: "S1", Condition: "A", Batch: "run"},
		{Name: "b", Subject: "S2", Condition: "A", Batch: "run"},
		{Name: "c", Subject: "S3", Condition: "B", Batch: "run"},
		{Name: "d", Subject: "S4", Condition: "B", Batch: "run"},
	}}
}
func hasCohortFinding(d CohortDesign, code string) bool {
	for _, f := range d.Findings {
		if f.Code == code {
			return true
		}
	}
	return false
}
func TestCohortDesignIdentityAndUnits(t *testing.T) {
	in := cohortFixture()
	before, _ := json.Marshal(in)
	d, err := CheckCohort(in)
	if err != nil || !d.OK || d.Included != 4 || d.Subjects != 4 || d.Groups[0].Subjects != 2 || d.Groups[1].Samples != 2 {
		t.Fatalf("%+v %v", d, err)
	}
	after, _ := json.Marshal(in)
	if string(before) != string(after) {
		t.Fatal("input mutated")
	}
	in.Samples[0], in.Samples[3] = in.Samples[3], in.Samples[0]
	reordered, _ := CheckCohort(in)
	if !reflect.DeepEqual(d, reordered) {
		t.Fatal("source order changed design")
	}
	in.Samples[0].Tissue = "blood"
	changed, _ := CheckCohort(in)
	if changed.ID == d.ID {
		t.Fatal("annotation absent from identity")
	}
	in.Dataset = "other-sources"
	sources, _ := CheckCohort(in)
	if sources.ID == changed.ID {
		t.Fatal("sources absent from identity")
	}
	in.Spec.Levels = []string{"B", "A"}
	reversed, _ := CheckCohort(in)
	if reversed.ID == sources.ID {
		t.Fatal("contrast direction absent from identity")
	}
}
func TestCohortRefusalsAndMissingness(t *testing.T) {
	for _, tc := range []struct {
		name, code string
		change     func(*CohortInput)
	}{
		{"subject missing", "cohort-no-subject", func(i *CohortInput) { i.Samples[0].Subject = "" }},
		{"group missing", "cohort-no-group", func(i *CohortInput) { i.Samples[0].Condition = "" }},
		{"repeat", "cohort-repeated-samples", func(i *CohortInput) { i.Samples[1].Subject = "S1" }},
		{"shared", "cohort-shared-subject", func(i *CohortInput) { i.Samples[2].Subject = "S1" }},
		{"incomplete", "cohort-incomplete-pairs", func(i *CohortInput) { i.Spec.Layout = "paired" }},
		{"batch confounded", "cohort-batch-confounded", func(i *CohortInput) {
			for n := range i.Samples {
				i.Samples[n].Batch = i.Samples[n].Condition
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := cohortFixture()
			tc.change(&in)
			d, err := CheckCohort(in)
			if err != nil || d.OK || !hasCohortFinding(d, tc.code) {
				t.Fatalf("%+v %v", d, err)
			}
		})
	}
	in := cohortFixture()
	in.Spec.Layout = "paired"
	in.Samples[2].Subject = "S1"
	in.Samples[3].Subject = "S2"
	d, err := CheckCohort(in)
	if err != nil || !d.OK || d.Subjects != 2 || d.CompletePairs != 2 {
		t.Fatalf("%+v %v", d, err)
	}
	in.Samples = append(in.Samples, CohortSample{Name: "extra", Condition: ""})
	d, _ = CheckCohort(in)
	if d.OK {
		t.Fatal("missing annotation silently dropped")
	}
	in.Spec.Exclude = map[string]string{"extra": "collection failed"}
	d, _ = CheckCohort(in)
	if !d.OK || d.Excluded != 1 || d.Rows[4].Reason != "Excluded: collection failed" {
		t.Fatalf("%+v", d)
	}
	in.Samples[0].Batch = ""
	d, _ = CheckCohort(in)
	if !d.OK || !hasCohortFinding(d, "cohort-missing-batch") {
		t.Fatal("missing batch not warned")
	}
	in.Spec.Exclude = nil
	in.Samples[4].Condition = "C"
	d, _ = CheckCohort(in)
	if !d.OK || d.Excluded != 1 || d.Rows[4].Reason != "Outside the selected contrast" {
		t.Fatalf("%+v", d)
	}
}
func TestCohortRejectsInvalidSpecifications(t *testing.T) {
	for _, change := range []func(*CohortInput){
		func(i *CohortInput) { i.Samples = make([]CohortSample, MaxCohortSamples+1) },
		func(i *CohortInput) { i.Samples[0].Subject = strings.Repeat("x", 4097) },
		func(i *CohortInput) { i.Spec.Levels[0] = strings.Repeat("x", 4097) },
		func(i *CohortInput) { i.Spec.Unit = "cell" }, func(i *CohortInput) { i.Spec.Layout = "automatic" },
		func(i *CohortInput) { i.Spec.GroupField = "hla" }, func(i *CohortInput) { i.Spec.Levels = []string{"A", " A "} },
		func(i *CohortInput) { i.Spec.Exclude = map[string]string{"absent": "bad"} }, func(i *CohortInput) { i.Spec.Exclude = map[string]string{"a": " "} },
		func(i *CohortInput) { i.Samples[0].Name = "b" }, func(i *CohortInput) { i.Samples = nil },
	} {
		in := cohortFixture()
		change(&in)
		if _, err := CheckCohort(in); err == nil {
			t.Fatalf("accepted %+v", in)
		}
	}
}
func TestCohortGroupsHaveLiteralLabels(t *testing.T) {
	in := cohortFixture()
	in.Spec.GroupField = "timepoint"
	in.Spec.Levels = []string{"day 28", "week 4"}
	for n := range in.Samples {
		if n < 2 {
			in.Samples[n].Timepoint = "day 28"
		} else {
			in.Samples[n].Timepoint = "week 4"
		}
	}
	d, err := CheckCohort(in)
	if err != nil || !d.OK || d.Groups[0].Samples != 2 || d.Groups[1].Samples != 2 {
		t.Fatalf("%+v %v", d, err)
	}
}
