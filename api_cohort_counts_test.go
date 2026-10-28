package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"clonoloom/internal/countmodel"
	"clonoloom/internal/random"
	"clonoloom/internal/study"
)

func countFeatureFixture(t *testing.T, terms bool) (*app, http.Handler, string, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "replicated-counts.tsv")
	var data strings.Builder
	data.WriteString("sequence_id\tv_call\tj_call\tjunction_aa\tproductive\tumi_count\trepertoire_id\n")
	input := study.CohortInput{Spec: study.CohortSpec{Unit: "subject", Layout: "independent", GroupField: "condition", Levels: []string{"A", "B"}}}
	rng := random.New(728, 1)
	for i := 0; i < 32; i++ {
		name := fmt.Sprintf("S%02d", i)
		group := "A"
		if i >= 16 {
			group = "B"
		}
		input.Samples = append(input.Samples, study.CohortSample{Name: name, Subject: name, Condition: group, Batch: fmt.Sprintf("B%d", i%2)})
		for j, aa := range []string{"CASSAF", "CASSCF", "CASSDF", "CASSEF", "CASSFF", "CASSGF", "CASSHF", "CASSIF", "CASSKF"} {
			mean := float64(50 + j*20)
			if j == 0 && i >= 16 {
				mean *= 3
			}
			count := rng.Poisson(rng.Gamma(5) * mean / 5)
			if j == 8 && i >= 16 {
				count = 0
			}
			fmt.Fprintf(&data, "%s-%d\tTRBV1*01\tTRBJ1*01\t%s\tT\t%d\t%s\n", name, j, aa, count, name)
		}
		fmt.Fprintf(&data, "%s-excluded\tTRBV1*01\tTRBJ1*01\tCASSPF\tF\t99999\t%s\n", name, name)
	}
	if err := os.WriteFile(path, []byte(data.String()), 0600); err != nil {
		t.Fatal(err)
	}
	a, h := testApp(t)
	opened := postJSON(t, h, "/api/datasets/open", `{"paths":[`+jsonString(path)+`]}`)
	var ds datasetSummary
	if err := json.Unmarshal(opened.Body.Bytes(), &ds); err != nil || opened.Code != 200 {
		t.Fatal(opened.Code, opened.Body, err)
	}
	input.Dataset = ds.ID
	if terms {
		source, _ := a.datasets.get(ds.ID)
		source.Credits = []credit{{Title: "Count fixture terms", License: "Fixture terms", LicenseClass: "terms", Citation: "Fixture citation"}}
	}
	design, err := study.CheckCohort(input)
	if err != nil || !design.OK {
		t.Fatal(design, err)
	}
	var s struct{ Artifact, Job string }
	for _, step := range []struct {
		path string
		body any
	}{
		{"/api/diversity", map[string]any{"dataset": ds.ID, "definition": map[string]any{"junction": "aa", "v": "gene", "j": "gene"}, "filter": map[string]any{"productiveOnly": true}, "options": map[string]any{"replicates": 2}}},
		{"/api/cohort-features", nil},
	} {
		body := step.body
		if body == nil {
			body = cohortFeaturesRequest{Design: cohortFeatureDesign{Record: design.ID + "-1", Name: "Replicated study", Revision: 1, ID: design.ID, Input: design.Input}, Diversity: s.Artifact}
		}
		r := postJSON(t, h, step.path, string(mustJSON(body)))
		if err = json.Unmarshal(r.Body.Bytes(), &s); err != nil || r.Code != 200 {
			t.Fatal(r.Code, r.Body, err)
		}
		waitForJob(t, a, s.Job, 10*time.Second)
	}
	return a, h, s.Artifact, path
}

func TestCohortCountsArtifactWorkflow(t *testing.T) {
	a, h, parent, source := countFeatureFixture(t, true)
	rawSource, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	body := cohortCountsRequest{Features: parent, Options: countmodel.Options{AdjustBatch: true}}
	start := func(payload any) struct {
		Artifact, Job string
		Ready         bool
	} {
		t.Helper()
		r := postJSON(t, h, "/api/cohort-counts", string(mustJSON(payload)))
		var s struct {
			Artifact, Job string
			Ready         bool
		}
		if err := json.Unmarshal(r.Body.Bytes(), &s); err != nil || r.Code != 200 {
			t.Fatal(r.Code, r.Body, err)
		}
		return s
	}
	probe := start(struct {
		cohortCountsRequest
		Start bool `json:"start"`
	}{body, false})
	if probe.Ready || probe.Job != "" {
		t.Fatal(probe)
	}
	s := start(body)
	waitForJob(t, a, s.Job, 10*time.Second)
	if s.Artifact != probe.Artifact || !start(body).Ready {
		t.Fatal(s, probe)
	}
	var doc cohortCountsDocument
	m, _, err := a.readCheckedResult(s.Artifact, cohortCountsKind, &doc)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Result.Family != 9 || doc.Result.Tested != 8 || len(doc.Result.Rows) != 9 || doc.Result.Design.ResidualDF != 29 || doc.Features.Parent.Unit != "umis" {
		t.Fatal(doc.Result)
	}
	after, err := os.ReadFile(source)
	if err != nil || !bytes.Equal(rawSource, after) {
		t.Fatal("source changed", err)
	}
	var spec cohortCountsSpec
	if err = json.Unmarshal(m.Lineage.Job.Spec, &spec); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*cohortCountsSpec){func(s *cohortCountsSpec) { s.SHA256 = "different" }, func(s *cohortCountsSpec) { s.Method++ }, func(s *cohortCountsSpec) { s.Options.Confidence = .9 }, func(s *cohortCountsSpec) { s.Options.MinCount++ }, func(s *cohortCountsSpec) { s.Options.MinSamples++ }, func(s *cohortCountsSpec) { s.Options.AdjustBatch = false }, func(s *cohortCountsSpec) { s.Input.Clonotypes[0].Values[0]++ }} {
		var other cohortCountsSpec
		json.Unmarshal(mustJSON(spec), &other)
		change(&other)
		if other.artifactID() == s.Artifact {
			t.Fatal("identity omits evidence")
		}
	}
	// Replay uses the captured matrix and verified parent, with no source reads.
	if err = os.Remove(source); err != nil {
		t.Fatal(err)
	}
	resultPath, _ := a.artifacts.Path(s.Artifact, cohortCountsKind+".json")
	if err = os.RemoveAll(filepath.Dir(resultPath)); err != nil {
		t.Fatal(err)
	}
	work, err := a.jobs.Remake(cohortCountsKind, m.Lineage.Job.Spec)
	if err != nil {
		t.Fatal(err)
	}
	replay, _, err := a.jobs.Submit(cohortCountsKind, "Replay counts", s.Artifact, spec, work)
	if err != nil {
		t.Fatal(err)
	}
	waitForJob(t, a, replay.ID, 10*time.Second)
	var back cohortCountsDocument
	if _, _, err = a.readCheckedResult(s.Artifact, cohortCountsKind, &back); err != nil || !bytes.Equal(mustJSON(doc), mustJSON(back)) {
		t.Fatal("replay differs", err)
	}
	parentPath, _ := a.artifacts.Path(parent, cohortFeaturesKind+".json")
	os.WriteFile(parentPath, []byte("{}"), 0600)
	if r := postJSON(t, h, "/api/cohort-counts", string(mustJSON(body))); r.Code != 409 {
		t.Fatal(r.Code, r.Body)
	}
	if err = a.cohortCountsWork(spec)(context.Background(), nil); err == nil {
		t.Fatal("changed parent accepted")
	}
	os.Remove(parentPath)
	if r := request(t, h, http.MethodGet, "/api/cohort-counts/"+s.Artifact, nil, nil); r.Code != 200 {
		t.Fatal(r.Code, r.Body)
	}
	url := "/api/cohort-counts/" + s.Artifact + "/download"
	if r := request(t, h, http.MethodGet, url, nil, nil); r.Code != 409 {
		t.Fatal("lost provider terms", r.Code)
	}
	r := request(t, h, http.MethodGet, url+"?accept=1", nil, nil)
	if r.Code != 200 {
		t.Fatal(r.Code, r.Body)
	}
	z, err := zip.NewReader(bytes.NewReader(r.Body.Bytes()), int64(r.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, f := range z.File {
		rd, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		files[f.Name], err = io.ReadAll(rd)
		rd.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"manifest.json", "cohort-counts.json", "comparisons.csv", "design.csv", "counts.csv", "NOTICE.txt"} {
		if len(files[name]) == 0 {
			t.Fatal("missing export", name)
		}
	}
	if !bytes.Equal(files["cohort-counts.json"], mustJSON(doc)) || !bytes.Contains(files["NOTICE.txt"], []byte("Fixture terms")) {
		t.Fatal("export lost evidence or terms")
	}
	for name, want := range map[string]int{"comparisons.csv": 10, "counts.csv": 10, "design.csv": 33} {
		rows, e := csv.NewReader(bytes.NewReader(files[name])).ReadAll()
		if e != nil || len(rows) != want {
			t.Fatal(name, len(rows), e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err = a.cohortCountsWork(spec)(ctx, nil); err != context.Canceled {
		t.Fatal(err)
	}
	os.WriteFile(resultPath, []byte("{}"), 0600)
	if r = request(t, h, http.MethodGet, url+"?accept=1", nil, nil); r.Code != 409 {
		t.Fatal("exported altered result")
	}
}
func TestCohortCountsRefusals(t *testing.T) {
	a, h, parent, _ := countFeatureFixture(t, false)
	for _, raw := range []string{`{"features":` + jsonString(parent) + `,"unknown":1}`, `{"features":` + jsonString(parent) + `} {}`, `{"features":` + jsonString(parent) + `,"options":{"minSamples":1}}`, `{"features":` + jsonString(parent) + `,"options":{"confidence":1}}`} {
		r := postJSON(t, h, "/api/cohort-counts", raw)
		if r.Code != 400 {
			t.Fatal(r.Code, r.Body)
		}
	}
	if _, err := a.jobs.Remake(cohortCountsKind, mustJSON(cohortCountsSpec{Method: 99, SHA256: "x"})); err == nil {
		t.Fatal("changed method accepted")
	}
	s := cohortCountsSpec{cohortCountsRequest: cohortCountsRequest{Features: parent}, SHA256: "wrong"}
	if _, _, _, err := a.cohortCountsParent(s); err == nil {
		t.Fatal("changed parent checksum accepted")
	}
}
