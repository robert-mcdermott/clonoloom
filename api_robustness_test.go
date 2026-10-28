package main

import (
	"archive/zip"
	"bytes"
	"clonoloom/internal/clonotype"
	"clonoloom/internal/cohort"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefinitionAuditArtifactWorkflow(t *testing.T) {
	a, h, features, source := countFeatureFixture(t, true)
	var started struct {
		Artifact, Job string
		Ready         bool
	}
	start := func(path string, body any) {
		t.Helper()
		r := postJSON(t, h, path, string(mustJSON(body)))
		started = struct {
			Artifact, Job string
			Ready         bool
		}{}
		if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &started) != nil {
			t.Fatal(r.Code, r.Body)
		}
	}
	start("/api/cohort-compare", cohortCompareRequest{Features: features, Options: cohort.Options{Features: []string{"shannon", "richness"}, Permutations: 999}})
	waitForJob(t, a, started.Job, 10*time.Second)
	parent := started.Artifact
	var cc cohortCompareDocument
	if _, _, err := a.readCheckedResult(parent, cohortCompareKind, &cc); err != nil {
		t.Fatal(err)
	}
	alt := cc.Features.Parent.Definition
	alt.Junction = "nt"
	req := robustnessRequest{Comparison: parent, Definitions: []clonotype.Definition{alt}}
	start("/api/definition-robustness", struct {
		robustnessRequest
		Start bool `json:"start"`
	}{req, false})
	probe := started.Artifact
	if started.Job != "" || started.Ready {
		t.Fatal(started)
	}
	start("/api/definition-robustness", req)
	if started.Artifact != probe {
		t.Fatal("probe changed identity")
	}
	waitForJob(t, a, started.Job, 10*time.Second)
	id := started.Artifact
	var doc robustnessDocument
	m, _, err := a.readCheckedResult(id, robustnessKind, &doc)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Result.Family != 8 || doc.Result.Variants[1].Table.Rows[0].Values[0].Value != 0 || doc.Result.Summaries[1].Available != 1 {
		t.Fatal(doc.Result)
	}
	if !bytes.Equal(mustJSON(doc.Result.Variants[0].Analysis), mustJSON(cc.Result)) {
		t.Fatal("baseline changed")
	}
	start("/api/definition-robustness", req)
	if !started.Ready || started.Artifact != id {
		t.Fatal(started)
	}
	if r := postJSON(t, h, "/api/definition-robustness", `{"comparison":"x","unknown":1}`); r.Code != 400 {
		t.Fatal(r.Code, r.Body)
	}
	badreq := req
	badreq.Definitions = []clonotype.Definition{cc.Features.Parent.Definition}
	if r := postJSON(t, h, "/api/definition-robustness", string(mustJSON(badreq))); r.Code != 400 {
		t.Fatal(r.Code, r.Body)
	}
	var spec robustnessSpec
	if err = json.Unmarshal(m.Lineage.Job.Spec, &spec); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*robustnessSpec){func(s *robustnessSpec) { s.Method++ }, func(s *robustnessSpec) { s.SHA256 = "changed" }, func(s *robustnessSpec) { s.Comparison = "other" }} {
		other := spec
		change(&other)
		if other.artifactID() == id {
			t.Fatal("identity omits provenance")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err = a.robustnessWork(spec)(ctx, nil); err != context.Canceled {
		t.Fatal(err)
	}
	// Content tampering fails checksum validation before any resumed inference.
	parentFile, err := a.artifacts.Path(parent, cohortCompareKind+".json")
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(parentFile)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(parentFile, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err = a.robustnessParent(context.Background(), spec); err == nil {
		t.Fatal("tampered comparison accepted")
	}
	if err = a.robustnessWork(spec)(context.Background(), nil); err == nil {
		t.Fatal("resumed worker accepted tampered parent")
	}
	if err = os.WriteFile(parentFile, original, 0600); err != nil {
		t.Fatal(err)
	}
	// Changing a job's pinned parent checksum is independently refused.
	changed := spec
	changed.SHA256 = "changed"
	if _, _, _, err = a.robustnessParent(context.Background(), changed); err == nil {
		t.Fatal("changed parent hash accepted")
	}
	// Replay uses captured abundance vectors and the complete comparison parent,
	// without raw files or its diversity/feature grandparents.
	if err = os.Remove(source); err != nil {
		t.Fatal(err)
	}
	for _, v := range []struct{ id, file string }{{features, cohortFeaturesKind + ".json"}, {cc.Features.ParentManifest.ID, "diversity.json"}} {
		path, e := a.artifacts.Path(v.id, v.file)
		if e != nil {
			t.Fatal(e)
		}
		if e = os.Remove(path); e != nil {
			t.Fatal(e)
		}
	}
	work, err := a.jobs.Remake(robustnessKind, m.Lineage.Job.Spec)
	if err != nil {
		t.Fatal(err)
	}
	path, err := a.artifacts.Path(id, robustnessKind+".json")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.RemoveAll(filepath.Dir(path)); err != nil {
		t.Fatal(err)
	}
	replay, _, err := a.jobs.Submit(robustnessKind, "Replay definition audit", id, spec, work)
	if err != nil {
		t.Fatal(err)
	}
	waitForJob(t, a, replay.ID, 10*time.Second)
	var back robustnessDocument
	if _, _, err = a.readCheckedResult(id, robustnessKind, &back); err != nil || !bytes.Equal(mustJSON(doc), mustJSON(back)) {
		t.Fatal("replay changed result", err)
	}
	// Terms and complete exports remain available even without the comparison source.
	parentPath, _ := a.artifacts.Path(parent, cohortCompareKind+".json")
	if err = os.Remove(parentPath); err != nil {
		t.Fatal(err)
	}
	if r := request(t, h, http.MethodGet, "/api/definition-robustness/"+id+"/download", nil, nil); r.Code != 409 {
		t.Fatal(r.Code, r.Body)
	}
	r := request(t, h, http.MethodGet, "/api/definition-robustness/"+id+"/download?accept=1", nil, nil)
	if r.Code != 200 {
		t.Fatal(r.Code, r.Body)
	}
	z, err := zip.NewReader(bytes.NewReader(r.Body.Bytes()), int64(r.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, f := range z.File {
		rd, e := f.Open()
		if e != nil {
			t.Fatal(e)
		}
		files[f.Name], e = io.ReadAll(rd)
		rd.Close()
		if e != nil {
			t.Fatal(e)
		}
	}
	for _, name := range []string{"definition-robustness.json", "manifest.json", "sensitivity.csv", "definition-1-subject-features.csv", "definition-2-subject-features.csv", "NOTICE.txt"} {
		if len(files[name]) == 0 {
			t.Fatal("missing export", name)
		}
	}
	if !strings.Contains(string(files["NOTICE.txt"]), "Fixture citation") || !bytes.Equal(files["definition-robustness.json"], mustJSON(doc)) {
		t.Fatal("provenance lost")
	}
	if _, _, _, err = a.robustnessParent(context.Background(), spec); err == nil {
		t.Fatal("missing parent accepted")
	}
	if r := request(t, h, http.MethodGet, "/api/definition-robustness/"+id, nil, nil); r.Code != 200 {
		t.Fatal(r.Code, r.Body)
	}
}
