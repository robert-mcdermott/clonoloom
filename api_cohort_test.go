package main

import (
	"clonoloom/internal/study"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCohortEndpointChecksDatasetMembership(t *testing.T) {
	path := filepath.Join(t.TempDir(), "study.tsv")
	if err := os.WriteFile(path, []byte(datasetTSV), 0600); err != nil {
		t.Fatal(err)
	}
	_, handler := testApp(t)
	opened := postJSON(t, handler, "/api/datasets/open", `{"paths":[`+jsonString(path)+`]}`)
	var summary datasetSummary
	if err := json.Unmarshal(opened.Body.Bytes(), &summary); err != nil || opened.Code != http.StatusOK {
		t.Fatalf("%s %v", opened.Body, err)
	}
	input := study.CohortInput{Dataset: summary.ID, Spec: study.CohortSpec{Unit: "subject", Layout: "paired", GroupField: "condition", Levels: []string{"pre", "post"}}, Samples: []study.CohortSample{{Name: "R1", Subject: "S1", Condition: "pre"}, {Name: "R2", Subject: "S1", Condition: "post"}}}
	data, _ := json.Marshal(input)
	rec := postJSON(t, handler, "/api/cohort/design", string(data))
	var d study.CohortDesign
	json.Unmarshal(rec.Body.Bytes(), &d)
	if rec.Code != http.StatusOK || d.OK || d.Subjects != 1 || d.CompletePairs != 1 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	for _, body := range []string{
		strings.Replace(string(data), `"R2"`, `"unknown"`, 1),
		strings.Replace(string(data), `"R2"`, `"R1"`, 1),
		string(data) + ` {}`,
		strings.Replace(string(data), `"unit":"subject"`, `"unit":"subject","unexpected":true`, 1),
	} {
		if r := postJSON(t, handler, "/api/cohort/design", body); r.Code != http.StatusBadRequest {
			t.Fatalf("accepted %s: %d %s", body, r.Code, r.Body)
		}
	}
	input.Samples = input.Samples[:1]
	data, _ = json.Marshal(input)
	if r := postJSON(t, handler, "/api/cohort/design", string(data)); r.Code != http.StatusBadRequest {
		t.Fatalf("accepted subset: %s", r.Body)
	}
	input.Dataset = "absent"
	data, _ = json.Marshal(input)
	if r := postJSON(t, handler, "/api/cohort/design", string(data)); r.Code != http.StatusNotFound {
		t.Fatalf("accepted absent dataset: %s", r.Body)
	}
}
