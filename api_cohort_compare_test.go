package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"clonoloom/internal/cohort"
)

func cohortComparisonFixture(t *testing.T) (*app, http.Handler, string, string) {
	t.Helper()
	a, h, body, source := cohortFeatureFixture(t)
	r := postJSON(t, h, "/api/cohort-features", string(mustJSON(body)))
	var s struct{ Artifact, Job string }
	if err := json.Unmarshal(r.Body.Bytes(), &s); err != nil || r.Code != 200 {
		t.Fatal(r.Code, r.Body, err)
	}
	waitForJob(t, a, s.Job, 5*time.Second)
	return a, h, s.Artifact, source
}
func TestCohortComparisonArtifactWorkflow(t *testing.T) {
	a, h, parent, source := cohortComparisonFixture(t)
	body := cohortCompareRequest{Features: parent, Options: cohort.Options{Features: []string{"abundance", "shannon"}, Permutations: 999}}
	start := func(payload any) struct {
		Artifact, Job string
		Ready         bool
	} {
		t.Helper()
		r := postJSON(t, h, "/api/cohort-compare", string(mustJSON(payload)))
		if r.Code != 200 {
			t.Fatal(r.Code, r.Body)
		}
		var s struct {
			Artifact, Job string
			Ready         bool
		}
		if err := json.Unmarshal(r.Body.Bytes(), &s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	// Comparisons need neither raw sources nor the original diversity parent.
	os.Remove(source)
	var feature cohortFeaturesDocument
	fm, _, err := a.readCheckedResult(parent, cohortFeaturesKind, &feature)
	if err != nil {
		t.Fatal(err)
	}
	path, _ := a.artifacts.Path(feature.ParentManifest.ID, "diversity.json")
	os.Remove(path)
	probe := start(struct {
		cohortCompareRequest
		Start bool `json:"start"`
	}{body, false})
	if probe.Ready || probe.Job != "" {
		t.Fatal(probe)
	}
	started := start(body)
	if started.Artifact != probe.Artifact {
		t.Fatal(started, probe)
	}
	waitForJob(t, a, started.Job, 5*time.Second)
	if ready := start(body); !ready.Ready || ready.Artifact != started.Artifact {
		t.Fatal(ready)
	}
	var doc cohortCompareDocument
	m, _, err := a.readCheckedResult(started.Artifact, cohortCompareKind, &doc)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Result.Family != 4 || doc.Result.Tested != 2 || doc.Result.Comparisons[1].Missing != 1 || doc.ParentManifest.ID != fm.ID || !strings.Contains(doc.Description, "BH family") || strings.Contains(doc.Description, "No pooling, averaging, depth normalization, imputation or group testing") {
		t.Fatal(doc.Description, doc.Result)
	}
	var spec cohortCompareSpec
	if err = json.Unmarshal(m.Lineage.Job.Spec, &spec); err != nil {
		t.Fatal(err)
	}
	work, err := a.jobs.Remake(cohortCompareKind, m.Lineage.Job.Spec)
	if err != nil || work == nil {
		t.Fatal(err)
	}
	for _, change := range []func(*cohortCompareSpec){func(s *cohortCompareSpec) { s.SHA256 = "changed" }, func(s *cohortCompareSpec) { s.Method++ }, func(s *cohortCompareSpec) { s.Options.Confidence = .9 }, func(s *cohortCompareSpec) { s.Options.Seed++ }, func(s *cohortCompareSpec) { s.Options.Permutations++ }, func(s *cohortCompareSpec) { s.Options.Features = []string{"abundance"} }} {
		other := spec
		change(&other)
		if other.artifactID() == started.Artifact {
			t.Fatal("identity omits options")
		}
	}
	// Replay from the recorded job after removing only its derived artifact.
	resultPath, _ := a.artifacts.Path(started.Artifact, cohortCompareKind+".json")
	if err = os.RemoveAll(filepath.Dir(resultPath)); err != nil {
		t.Fatal(err)
	}
	replay, _, err := a.jobs.Submit(cohortCompareKind, "Replay comparison", started.Artifact, spec, work)
	if err != nil {
		t.Fatal(err)
	}
	waitForJob(t, a, replay.ID, 5*time.Second)
	var back cohortCompareDocument
	if _, _, err = a.readCheckedResult(started.Artifact, cohortCompareKind, &back); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(mustJSON(doc), mustJSON(back)) {
		t.Fatal("replay changed document")
	}
	// Altered feature parent fails both POST and a resumed worker, without promotion.
	parentPath, _ := a.artifacts.Path(parent, cohortFeaturesKind+".json")
	os.WriteFile(parentPath, []byte("{}"), 0600)
	if r := postJSON(t, h, "/api/cohort-compare", string(mustJSON(body))); r.Code != 409 {
		t.Fatal(r.Code, r.Body)
	}
	bad := spec
	bad.Options.Seed++
	job, _, err := a.jobs.Submit(cohortCompareKind, "Changed parent", bad.artifactID(), bad, a.cohortCompareWork(bad))
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		j, _ := a.jobs.Get(job.ID)
		if j.State.Finished() {
			if j.State != "failed" {
				t.Fatal(j)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("job timeout")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if _, err = a.artifacts.Get(bad.artifactID()); err == nil {
		t.Fatal("promoted changed parent")
	}
	// Saved JSON, CSV and all source notices remain available independently.
	os.Remove(parentPath)
	raw := request2(t, h, "/api/cohort-compare/"+started.Artifact)
	if !bytes.Contains(raw, []byte(`"family":4`)) {
		t.Fatal(string(raw))
	}
	r := request(t, h, http.MethodGet, "/api/cohort-compare/"+started.Artifact+"/download", nil, nil)
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
	for _, name := range []string{"comparisons.csv", "cohort-compare.json", "subject-features.csv", "manifest.json", "NOTICE.txt"} {
		if len(files[name]) == 0 {
			t.Fatal("missing export", name)
		}
	}
	records, err := csv.NewReader(bytes.NewReader(files["comparisons.csv"])).ReadAll()
	if err != nil || len(records) != 5 || records[3][12] != "" {
		t.Fatal(records, err)
	}
	if !bytes.Equal(files["cohort-compare.json"], mustJSON(doc)) {
		t.Fatal("export changed evidence")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err = a.cohortCompareWork(spec)(ctx, nil); err != context.Canceled {
		t.Fatal(err)
	}
	os.WriteFile(resultPath, []byte("{}"), 0600)
	if r = request(t, h, http.MethodGet, "/api/cohort-compare/"+started.Artifact+"/download", nil, nil); r.Code != 409 {
		t.Fatal("exported changed result")
	}
}
func TestCohortComparisonRefusals(t *testing.T) {
	a, h, parent, _ := cohortComparisonFixture(t)
	body := `{"features":` + jsonString(parent) + `,"options":{"features":["abundance"]}}`
	for _, raw := range []string{body + " {}", strings.Replace(body, `"features":`, `"unknown":1,"features":`, 1), strings.Replace(body, `["abundance"]`, `[]`, 1), strings.Replace(body, `["abundance"]`, `["no-such-feature"]`, 1)} {
		r := postJSON(t, h, "/api/cohort-compare", raw)
		if r.Code != 400 {
			t.Fatal(r.Code, r.Body)
		}
	}
	if _, err := a.jobs.Remake(cohortCompareKind, mustJSON(cohortCompareSpec{Method: 99, SHA256: "x"})); err == nil {
		t.Fatal("changed method accepted")
	}
	s := cohortCompareSpec{cohortCompareRequest: cohortCompareRequest{Features: parent}, SHA256: "wrong"}
	if _, _, _, err := a.cohortCompareParent(s); err == nil {
		t.Fatal("changed checksum accepted")
	}
}

func TestCohortComparisonRetainsProviderTerms(t *testing.T) {
	a, h, body, _ := cohortFeatureFixture(t)
	ds, _ := a.datasets.get(body.Design.Input.Dataset)
	ds.Credits = []credit{{Title: "Artificial provider terms control", License: "Fixture terms", LicenseClass: "terms", Citation: "Fixture citation"}}
	var s struct{ Artifact, Job string }
	for _, step := range []struct{ path, body string }{
		{"/api/cohort-features", string(mustJSON(body))},
		{"/api/cohort-compare", ""},
	} {
		payload := step.body
		if payload == "" {
			payload = string(mustJSON(cohortCompareRequest{Features: s.Artifact, Options: cohort.Options{Features: []string{"abundance"}}}))
		}
		r := postJSON(t, h, step.path, payload)
		if err := json.Unmarshal(r.Body.Bytes(), &s); err != nil || r.Code != 200 {
			t.Fatal(r.Code, r.Body, err)
		}
		waitForJob(t, a, s.Job, 5*time.Second)
	}
	path := "/api/cohort-compare/" + s.Artifact + "/download"
	r := request(t, h, http.MethodGet, path, nil, nil)
	if r.Code != 409 || !strings.Contains(r.Body.String(), "restricted") {
		t.Fatal(r.Code, r.Body)
	}
	r = request(t, h, http.MethodGet, path+"?accept=1", nil, nil)
	if r.Code != 200 {
		t.Fatal(r.Code, r.Body)
	}
	z, err := zip.NewReader(bytes.NewReader(r.Body.Bytes()), int64(r.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range z.File {
		if f.Name != "NOTICE.txt" {
			continue
		}
		reader, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		raw, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			t.Fatal(err)
		}
		for _, text := range []string{"Fixture terms", "Fixture citation", "These results were computed", "BH family", "10.1093/biomet/34.1-2.28"} {
			if !strings.Contains(string(raw), text) {
				t.Fatal(text, string(raw))
			}
		}
		return
	}
	t.Fatal("provider notice absent")
}
