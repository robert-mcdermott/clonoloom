package study

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"

	"clonoloom/internal/diversity"
)

func TestSubjectFeaturesSelectionMissingnessAndCSV(t *testing.T) {
	input := cohortFixture()
	samples := map[string]diversity.Result{}
	counts := [][]float64{{}, {9}, {.5, 1.5}, {1, 2, 3}}
	for i, s := range input.Samples {
		r := diversity.Measure(counts[i], diversity.Options{})
		samples[s.Name] = r
	}
	before, _ := json.Marshal(samples)
	table, err := SubjectFeatures(input, samples, "umis")
	if err != nil {
		t.Fatal(err)
	}
	if len(table.Rows) != 4 || len(table.Catalog) != 13 || table.Catalog[0].Unit != "umis" || table.Catalog[2].Unit != "nats" || table.Catalog[3].Unit != "effective clonotypes" {
		t.Fatal(table)
	}
	empty := table.Rows[0].Values
	if empty[0].Value != 0 || empty[1].Value != 0 || empty[0].Missing != "" {
		t.Fatal(empty)
	}
	for _, v := range empty[2:] {
		if !math.IsNaN(float64(v.Value)) || v.Missing == "" {
			t.Fatal(v)
		}
	}
	if table.Rows[1].Values[5].Missing == "" || table.Rows[1].Values[6].Missing == "" || table.Rows[2].Values[12].Missing == "" {
		t.Fatal(table)
	}
	after, _ := json.Marshal(samples)
	if !bytes.Equal(before, after) {
		t.Fatal("mutated parent")
	}
	var b bytes.Buffer
	if err = FeatureCSV(&b, table); err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(&b).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 5 || rows[1][8] != "0" || rows[1][10] != "0" || rows[1][12] != "" || rows[1][13] == "" {
		t.Fatal(rows)
	}
	raw, _ := json.Marshal(table)
	var back FeatureTable
	if err = json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	round, _ := json.Marshal(back)
	if !bytes.Equal(raw, round) || !strings.Contains(string(raw), `"value":null`) {
		t.Fatal("null round trip")
	}
	// Selection excludes a repeated sample explicitly, with no pooling or averaging.
	input.Samples = append(input.Samples, CohortSample{Name: "repeat", Subject: "S1", Condition: "A", Batch: "run"})
	samples["repeat"] = samples["d"]
	if _, err = SubjectFeatures(input, samples, "umis"); err == nil {
		t.Fatal("accepted repeated subject")
	}
	input.Spec.Exclude = map[string]string{"repeat": "second library not selected"}
	selected, err := SubjectFeatures(input, samples, "umis")
	if err != nil || !reflect.DeepEqual(mustFeatureJSON(selected), mustFeatureJSON(table)) {
		t.Fatal("selection changed measurements", err)
	}
	delete(samples, "repeat")
	if _, err = SubjectFeatures(input, samples, "umis"); err == nil {
		t.Fatal("accepted incomplete parent")
	}
}
func mustFeatureJSON(v any) string { b, _ := json.Marshal(v); return string(b) }
func TestSubjectFeaturesPairedAndRefusals(t *testing.T) {
	input := cohortFixture()
	input.Spec.Layout = "paired"
	input.Samples[2].Subject = "S1"
	input.Samples[3].Subject = "S2"
	samples := map[string]diversity.Result{}
	for _, s := range input.Samples {
		samples[s.Name] = diversity.Result{}
	}
	got, err := SubjectFeatures(input, samples, "cells")
	if err != nil || len(got.Rows) != 4 || got.Rows[0].Subject != got.Rows[2].Subject {
		t.Fatal(got, err)
	}
	if _, err = SubjectFeatures(input, samples, "unknown"); err == nil {
		t.Fatal("unknown unit")
	}
	samples["a"] = diversity.Result{Abundance: 2}
	if _, err = SubjectFeatures(input, samples, "cells"); err == nil {
		t.Fatal("inconsistent richness")
	}
	samples["a"] = diversity.Result{Abundance: math.Inf(1), Clonotypes: 1}
	if _, err = SubjectFeatures(input, samples, "cells"); err == nil {
		t.Fatal("infinite abundance")
	}
}
