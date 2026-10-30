package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"clonoloom/internal/artifacts"
)

func likelihoodFixture(t *testing.T) (*app, http.Handler, likelihoodRequest) {
	t.Helper()
	a, h := testApp(t)
	base := "validation/projects/0.5.0"
	id := "lin-8f4ffe74bbf3069b"
	raw, readErr := os.ReadFile(filepath.Join(base, "artifacts", id, "manifest.json"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	var m artifacts.Manifest
	json.Unmarshal(raw, &m)
	parent := gunzipFile(t, filepath.Join(base, "artifacts", id, "lineages.json.gz"))
	var doc lineageDocument
	json.Unmarshal(parent, &doc)
	var p struct {
		Datasets []struct{ Sources []struct{ Name string } }
	}
	b, e := os.ReadFile(filepath.Join(base, "project.json"))
	if e != nil {
		t.Fatal(e)
	}
	json.Unmarshal(b, &p)
	paths := []string{}
	for _, s := range p.Datasets[0].Sources {
		paths = append(paths, filepath.Join(base, "data", s.Name))
	}
	ds, e := a.openDataset(context.Background(), paths, nil, "Artificial historical lineage control")
	if e != nil {
		t.Fatal(e)
	}
	// This fixture rebinds the same supplied source bytes to the current dataset ID;
	// the historical files themselves are never rewritten.
	m.Dataset = ds.ID
	doc.Dataset = ds.ID
	parent = mustJSON(doc)
	dir, e := a.artifacts.Stage(id)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(dir, "lineages.json"), parent, 0600); e != nil {
		t.Fatal(e)
	}
	m.Files = []artifacts.File{{Path: "lineages.json", Role: "lineages"}}
	if _, e = a.artifacts.Promote(m, nil); e != nil {
		t.Fatal(e)
	}
	for _, tree := range doc.Trees {
		if tree.Excluded == "" && tree.Taxa > 2 && tree.Taxa <= 7 {
			return a, h, likelihoodRequest{Artifact: id, Tree: tree.ID}
		}
	}
	t.Fatal("no bounded historical panel")
	return nil, nil, likelihoodRequest{}
}
func TestLineageLikelihoodReview(t *testing.T) {
	a, h, req := likelihoodFixture(t)
	w := postJSON(t, h, "/api/lineage-likelihood/review", string(mustJSON(req)))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	var got struct {
		Review   string             `json:"review"`
		Evidence likelihoodDocument `json:"evidence"`
	}
	json.Unmarshal(w.Body.Bytes(), &got)
	req.Review = got.Review
	if got.Evidence.Result.Model != "JC69" || got.Evidence.Result.Topologies != 945 || got.Evidence.ParentSHA256 != replayDigest([]byte(got.Evidence.ParentText)) {
		t.Fatal(got)
	}
	saved := postJSON(t, h, "/api/lineage-likelihood/export", string(mustJSON(req)))
	if saved.Code != 200 || replayDigest(saved.Body.Bytes()) != req.Review {
		t.Fatal(saved.Code, saved.Body)
	}
	if dir := os.Getenv("CLONOLOOM_ML_OUT"); dir != "" {
		os.MkdirAll(dir, 0700)
		if e := os.WriteFile(filepath.Join(dir, "api.json"), saved.Body.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
	}
	m, _ := a.artifacts.Get(req.Artifact)
	ds, _ := a.datasets.get(m.Dataset)
	ds.Credits = []credit{{Title: "Artificial restricted evidence", LicenseClass: "terms", License: "Review required"}}
	stale := postJSON(t, h, "/api/lineage-likelihood/export", string(mustJSON(req)))
	if stale.Code != 409 {
		t.Fatal("changed notice accepted", stale.Code)
	}
	w = postJSON(t, h, "/api/lineage-likelihood/review", string(mustJSON(req)))
	if w.Code != 200 {
		t.Fatal(w.Body)
	}
	json.Unmarshal(w.Body.Bytes(), &got)
	req.Review = got.Review
	denied := postJSON(t, h, "/api/lineage-likelihood/export", string(mustJSON(req)))
	if denied.Code != 403 {
		t.Fatal(denied.Body)
	}
	req.AcceptTerms = true
	if w = postJSON(t, h, "/api/lineage-likelihood/export", string(mustJSON(req))); w.Code != 200 {
		t.Fatal(w.Body)
	}
	path, _ := a.artifacts.Path(req.Artifact, "lineages.json")
	os.WriteFile(path, []byte("{}"), 0600)
	if w = postJSON(t, h, "/api/lineage-likelihood/export", string(mustJSON(req))); w.Code != 400 || !strings.Contains(w.Body.String(), "checksum") {
		t.Fatal(w.Body)
	}
}
func TestLineageLikelihoodRefusals(t *testing.T) {
	_, h, req := likelihoodFixture(t)
	for _, r := range []likelihoodRequest{{Artifact: req.Artifact, Tree: "missing"}, {Artifact: "missing", Tree: req.Tree}, {Artifact: req.Artifact, Tree: req.Tree, Review: "missing"}} {
		op := "review"
		if r.Review != "" {
			op = "export"
		}
		w := postJSON(t, h, "/api/lineage-likelihood/"+op, string(mustJSON(r)))
		if w.Code < 400 {
			t.Fatal(w.Body)
		}
	}
}
