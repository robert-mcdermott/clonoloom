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

	"clonoloom/internal/study"
)

func cohortFeatureFixture(t *testing.T) (*app, http.Handler, cohortFeaturesRequest, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "subject-measurements.tsv")
	data := "sequence_id\tv_call\tj_call\tjunction_aa\tproductive\tumi_count\trepertoire_id\n" +
		"a\tTRBV1*01\tTRBJ1-1*01\tCASSF\tF\t8\ta\n" +
		"b\tTRBV1*01\tTRBJ1-1*01\tCASSF\tT\t9\tb\n" +
		"c\tTRBV1*01\tTRBJ1-1*01\tCASSF\tT\t2\tc\n" +
		"d\tTRBV1*01\tTRBJ1-1*01\tCASRF\tT\t6\tc\n" +
		"e\tTRBV1*01\tTRBJ1-1*01\tCASSF\tT\t1\td\n" +
		"f\tTRBV1*01\tTRBJ1-1*01\tCASRF\tT\t2\td\n"
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	a, h := testApp(t)
	opened := postJSON(t, h, "/api/datasets/open", `{"paths":[`+jsonString(path)+`]}`)
	var summary datasetSummary
	if err := json.Unmarshal(opened.Body.Bytes(), &summary); err != nil || opened.Code != 200 {
		t.Fatal(opened.Code, opened.Body, err)
	}
	input := study.CohortInput{Dataset: summary.ID, Spec: study.CohortSpec{Unit: "subject", Layout: "independent", GroupField: "condition", Levels: []string{"A", "B"}}}
	for i, name := range []string{"a", "b", "c", "d"} {
		group := "A"
		if i > 1 {
			group = "B"
		}
		input.Samples = append(input.Samples, study.CohortSample{Name: name, Subject: name, Condition: group, Batch: "run"})
	}
	design, err := study.CheckCohort(input)
	if err != nil || !design.OK {
		t.Fatal(design, err)
	}
	r := postJSON(t, h, "/api/diversity", `{"dataset":`+jsonString(summary.ID)+`,"definition":{"junction":"aa","v":"gene","j":"gene"},"filter":{"productiveOnly":true},"options":{"replicates":2}}`)
	var start struct{ Artifact, Job string }
	if err = json.Unmarshal(r.Body.Bytes(), &start); err != nil || r.Code != 200 {
		t.Fatal(r.Code, r.Body, err)
	}
	waitForJob(t, a, start.Job, 10*time.Second)
	return a, h, cohortFeaturesRequest{Design: cohortFeatureDesign{Record: design.ID + "-1", Name: "First observations", Revision: 1, ID: design.ID, Input: design.Input}, Diversity: start.Artifact}, path
}
func TestCohortFeaturesArtifactWorkflow(t *testing.T) {
	a, h, body, source := cohortFeatureFixture(t)
	start := func(payload any) struct {
		Artifact, Job string
		Ready         bool
	} {
		t.Helper()
		r := postJSON(t, h, "/api/cohort-features", string(mustJSON(payload)))
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
	probe := start(struct {
		cohortFeaturesRequest
		Start bool `json:"start"`
	}{body, false})
	if probe.Ready || probe.Job != "" {
		t.Fatal(probe)
	}
	s := start(body)
	if s.Artifact != probe.Artifact || s.Ready {
		t.Fatal(s)
	}
	waitForJob(t, a, s.Job, 5*time.Second)
	if again := start(body); !again.Ready || again.Artifact != s.Artifact {
		t.Fatal(again)
	}
	raw := request2(t, h, "/api/cohort-features/"+s.Artifact)
	var response struct{ Result cohortFeaturesDocument }
	if err := json.Unmarshal(raw, &response); err != nil {
		t.Fatal(err)
	}
	doc := response.Result
	if len(doc.Table.Rows) != 4 || doc.Table.Rows[0].Values[0].Value != 0 || doc.Table.Rows[0].Values[2].Missing == "" || doc.Table.Rows[1].Values[5].Missing == "" || doc.Parent.Unit != "umis" || !doc.Parent.Filter.ProductiveOnly || len(doc.Notice.Files) != 1 || doc.Notice.Files[0].SHA256 == "" {
		t.Fatalf("%+v", doc)
	}
	m, err := a.artifacts.Get(s.Artifact)
	if err != nil {
		t.Fatal(err)
	}
	var spec cohortFeaturesSpec
	if err = json.Unmarshal(m.Lineage.Job.Spec, &spec); err != nil {
		t.Fatal(err)
	}
	if _, err = a.jobs.Remake(cohortFeaturesKind, m.Lineage.Job.Spec); err != nil {
		t.Fatal(err)
	}
	if _, _, err = a.cohortFeatureParents(spec); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*cohortFeaturesSpec){func(s *cohortFeaturesSpec) { s.SHA256 = "changed" }, func(s *cohortFeaturesSpec) { s.Design.Revision++ }, func(s *cohortFeaturesSpec) { s.Design.ID = "changed" }, func(s *cohortFeaturesSpec) { s.Method++ }} {
		other := spec
		change(&other)
		if other.artifactID() == s.Artifact {
			t.Fatal("identity omits evidence")
		}
	}
	bad := spec
	bad.Method++
	if _, err = a.jobs.Remake(cohortFeaturesKind, mustJSON(bad)); err == nil {
		t.Fatal("changed method resumed")
	}
	// Recreate with the durable job spec: no dataset or source reads are needed by the worker.
	os.Remove(source)
	replay := spec
	replay.Design.Revision++
	work, err := a.jobs.Remake(cohortFeaturesKind, mustJSON(replay))
	if err != nil {
		t.Fatal(err)
	}
	job, _, err := a.jobs.Submit(cohortFeaturesKind, "replayed features", replay.artifactID(), replay, work)
	if err != nil {
		t.Fatal(err)
	}
	waitForJob(t, a, job.ID, 5*time.Second)
	var replayed cohortFeaturesDocument
	if _, _, err = a.readCheckedResult(replay.artifactID(), cohortFeaturesKind, &replayed); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(mustJSON(replayed.Table), mustJSON(doc.Table)) {
		t.Fatal("replay changed values")
	}
	parentPath, _ := a.artifacts.Path(body.Diversity, "diversity.json")
	os.WriteFile(parentPath, []byte("{}"), 0600)
	if _, _, err = a.cohortFeatureParents(spec); err == nil {
		t.Fatal("accepted changed parent")
	}
	if r := postJSON(t, h, "/api/cohort-features", string(mustJSON(body))); r.Code != 409 {
		t.Fatal(r.Code, r.Body)
	}
	// Kept evidence, complete export and notices survive missing source and parent files.
	os.Remove(parentPath)
	if !bytes.Equal(raw, request2(t, h, "/api/cohort-features/"+s.Artifact)) {
		t.Fatal("read depended on source or parent")
	}
	r := request(t, h, http.MethodGet, "/api/cohort-features/"+s.Artifact+"/download", nil, nil)
	if r.Code != 200 {
		t.Fatal(r.Code, r.Body)
	}
	archive, err := zip.NewReader(bytes.NewReader(r.Body.Bytes()), int64(r.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, f := range archive.File {
		reader, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			t.Fatal(err)
		}
		files[f.Name] = data
	}
	for _, name := range []string{"subject-features.csv", "catalog.json", "cohort-features.json", "manifest.json", "NOTICE.txt"} {
		if len(files[name]) == 0 {
			t.Fatal("missing export", name, files)
		}
	}
	rows, err := csv.NewReader(bytes.NewReader(files["subject-features.csv"])).ReadAll()
	if err != nil || len(rows) != 5 || rows[1][8] != "0" || rows[1][12] != "" {
		t.Fatal(rows, err)
	}
	if !bytes.Equal(files["cohort-features.json"], mustJSON(doc)) {
		t.Fatal("export changed snapshot")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err = a.cohortFeaturesWork(spec)(ctx, nil); err != context.Canceled {
		t.Fatal(err)
	}
	resultPath, _ := a.artifacts.Path(s.Artifact, cohortFeaturesKind+".json")
	os.WriteFile(resultPath, []byte("{}"), 0600)
	if r = request(t, h, http.MethodGet, "/api/cohort-features/"+s.Artifact+"/download", nil, nil); r.Code != 409 {
		t.Fatal("exported changed artifact", r.Code)
	}
}
func TestCohortFeaturesRejectInvalidRequests(t *testing.T) {
	a, h, body, _ := cohortFeatureFixture(t)
	for _, raw := range []string{string(mustJSON(body)) + ` {}`, strings.Replace(string(mustJSON(body)), `"diversity":`, `"unexpected":true,"diversity":`, 1)} {
		if r := postJSON(t, h, "/api/cohort-features", raw); r.Code != 400 {
			t.Fatal(r.Code, r.Body)
		}
	}
	for _, change := range []func(*cohortFeaturesRequest){func(s *cohortFeaturesRequest) { s.Design.ID = "forged" }, func(s *cohortFeaturesRequest) { s.Design.Revision = 0 }, func(s *cohortFeaturesRequest) { s.Diversity = "absent" }, func(s *cohortFeaturesRequest) { s.Design.Input.Dataset = "other" }, func(s *cohortFeaturesRequest) { s.Design.Input.Samples = s.Design.Input.Samples[:3] }} {
		b := body
		change(&b)
		if r := postJSON(t, h, "/api/cohort-features", string(mustJSON(b))); r.Code != 409 {
			t.Fatal(r.Code, r.Body)
		}
	}
	_, hash, err := a.cohortFeatureParents(cohortFeaturesSpec{cohortFeaturesRequest: body})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = a.cohortFeatureParents(cohortFeaturesSpec{cohortFeaturesRequest: body, SHA256: hash + "changed"}); err == nil {
		t.Fatal("accepted replacement with different hash")
	}
}

func TestCohortFeaturesRetainProviderTerms(t *testing.T) {
	a, h, body, _ := cohortFeatureFixture(t)
	ds, _ := a.datasets.get(body.Design.Input.Dataset)
	ds.Credits = []credit{{Title: "Artificial provider terms control", License: "Fixture terms", LicenseClass: "terms", Citation: "Fixture citation"}}
	r := postJSON(t, h, "/api/cohort-features", string(mustJSON(body)))
	var s struct{ Artifact, Job string }
	if err := json.Unmarshal(r.Body.Bytes(), &s); err != nil || r.Code != 200 {
		t.Fatal(r.Code, r.Body, err)
	}
	waitForJob(t, a, s.Job, 5*time.Second)
	path := "/api/cohort-features/" + s.Artifact + "/download"
	if r = request(t, h, http.MethodGet, path, nil, nil); r.Code != 409 || !strings.Contains(r.Body.String(), "restricted") {
		t.Fatal(r.Code, r.Body)
	}
	if r = request(t, h, http.MethodGet, path+"?accept=1", nil, nil); r.Code != 200 {
		t.Fatal(r.Code, r.Body)
	}
	z, err := zip.NewReader(bytes.NewReader(r.Body.Bytes()), int64(r.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range z.File {
		if f.Name == "NOTICE.txt" {
			reader, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			raw, err := io.ReadAll(reader)
			reader.Close()
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(raw), "Fixture terms") || !strings.Contains(string(raw), "These results were computed") || !strings.Contains(string(raw), "Fixture citation") {
				t.Fatal(string(raw))
			}
			return
		}
	}
	t.Fatal("provider notice absent")
}

func TestCohortFeaturesChangedParentFailsWorker(t *testing.T) {
	a, _, body, _ := cohortFeatureFixture(t)
	spec := cohortFeaturesSpec{cohortFeaturesRequest: body, Method: study.FeatureVersion, SHA256: strings.Repeat("0", 64)}
	work, err := a.jobs.Remake(cohortFeaturesKind, mustJSON(spec))
	if err != nil {
		t.Fatal(err)
	}
	job, _, err := a.jobs.Submit(cohortFeaturesKind, "Changed-parent control", spec.artifactID(), spec, work)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		j, _ := a.jobs.Get(job.ID)
		if j.State.Finished() {
			if j.State != "failed" || !strings.Contains(j.Error, "parent changed") {
				t.Fatal(j)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("worker timeout")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if _, err = a.artifacts.Get(spec.artifactID()); err == nil {
		t.Fatal("promoted changed parent")
	}
}
