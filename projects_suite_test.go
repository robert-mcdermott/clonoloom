package main

// Release checks distinguish original-byte integrity, source recomputation and
// conditional reviews. They do not add a scientific certificate replay kind.
import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"clonoloom/internal/artifacts"
	"clonoloom/internal/bcell"
)

func TestSuiteReleaseEvidence(t *testing.T) {
	const dir = "validation/projects/0.9.0"
	read := func(path string) []byte {
		t.Helper()
		raw, e := os.ReadFile(filepath.Join(dir, path))
		if e != nil {
			t.Fatal(e)
		}
		return raw
	}
	var study struct {
		Format                               string
		Version                              int
		Software, BcellDataset, QueryDataset string
		Files                                []struct{ Path, SHA256 string }
		Inspectors                           []struct{ Dataset, Endpoint, File string }
	}
	if e := json.Unmarshal(read("study.json"), &study); e != nil {
		t.Fatal(e)
	}
	if study.Format != "clonoloom-suite-study" || study.Version != 1 || study.Software != "0.9.0" || len(study.Files) != 73 || len(study.Inspectors) != 6 {
		t.Fatal("incomplete suite study")
	}
	seen := map[string]bool{}
	for _, f := range study.Files {
		if !filepath.IsLocal(f.Path) || seen[f.Path] || replayDigest(read(f.Path)) != f.SHA256 {
			t.Fatal("changed inventory", f.Path)
		}
		seen[f.Path] = true
	}
	if e := filepath.WalkDir(dir, func(path string, entry os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if !entry.IsDir() {
			rel, _ := filepath.Rel(dir, path)
			if rel != "study.json" && !seen[filepath.ToSlash(rel)] {
				t.Errorf("unrecorded fixture file %s", rel)
			}
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	var project fixtureProject
	if e := json.Unmarshal(read("project.json"), &project); e != nil {
		t.Fatal(e)
	}
	if len(project.Datasets) != 8 || len(project.Artifacts) != 5 {
		t.Fatal("study scopes changed")
	}
	a, h := testApp(t)
	var e error
	for _, d := range project.Datasets {
		paths := []string{}
		for _, f := range d.Sources {
			paths = append(paths, filepath.Join(dir, f.Path))
		}
		for _, f := range d.Metadata {
			paths = append(paths, filepath.Join(dir, f.Path))
		}
		ds, e := a.openDataset(context.Background(), paths, nil, d.Name)
		if e != nil || ds.ID != d.ID {
			t.Fatal("source identity", e, d.ID)
		}
	}
	// Preserve the release's original manifests only after checking every artifact file.
	for _, kept := range project.Artifacts {
		raw := read("artifacts/" + kept.ID + "/manifest.json")
		var m artifacts.Manifest
		if e = json.Unmarshal(raw, &m); e != nil {
			t.Fatal(e)
		}
		dst := filepath.Join(a.artifacts.Dir(), kept.ID)
		if e = os.MkdirAll(dst, 0700); e != nil {
			t.Fatal(e)
		}
		for _, f := range m.Files {
			b := gunzipFile(t, filepath.Join(dir, "artifacts", kept.ID, f.Path+".gz"))
			if replayDigest(b) != f.SHA256 {
				t.Fatal("parent bytes", f.Path)
			}
			if e = os.WriteFile(filepath.Join(dst, f.Path), b, 0600); e != nil {
				t.Fatal(e)
			}
		}
		if e = os.WriteFile(filepath.Join(dst, "manifest.json"), raw, 0600); e != nil {
			t.Fatal(e)
		}
		if findings, e := a.artifacts.Verify(kept.ID, nil); e != nil || len(findings) > 0 {
			t.Fatal(e, findings)
		}
	}
	// Only opening-path and filesystem mtime metadata may differ on another machine.
	normalizeInspection := func(raw []byte) []byte {
		var x map[string]json.RawMessage
		if e := json.Unmarshal(raw, &x); e != nil {
			t.Fatal(e)
		}
		if b, ok := x["source"]; ok && len(b) > 0 && b[0] == '{' {
			var source map[string]json.RawMessage
			if e := json.Unmarshal(b, &source); e != nil {
				t.Fatal(e)
			}
			delete(source, "path")
			delete(source, "modified")
			x["source"] = mustJSON(source)
		}
		return mustJSON(x)
	}
	for _, i := range study.Inspectors {
		r := request(t, h, "GET", "/api/datasets/"+i.Dataset+"/"+i.Endpoint, nil, nil)
		if r.Code != 200 || !bytes.Equal(normalizeInspection(r.Body.Bytes()), normalizeInspection(read("evidence/"+i.File))) {
			t.Fatal("source inspection differs", i.Endpoint, r.Code)
		}
	}
	var saved candidateDocument
	if e = json.Unmarshal(read("evidence/candidates.json"), &saved); e != nil {
		t.Fatal(e)
	}
	oldVersion, oldCommit := version, commit
	parts := strings.SplitN(strings.TrimPrefix(saved.Software, "ClonoLoom "), "; ", 2)
	if len(parts) != 2 {
		t.Fatal("software provenance")
	}
	version, commit = parts[0], parts[1]
	defer func() { version, commit = oldVersion, oldCommit }()
	q := candidateRequest{Options: saved.Result.Options, Review: replayDigest(read("evidence/candidates.json"))}
	for _, p := range saved.Parents {
		switch p.Manifest.Kind {
		case "families":
			q.Families = p.Manifest.ID
		case "shm":
			q.SHM = p.Manifest.ID
		case "isotypes":
			q.Isotypes = p.Manifest.ID
		}
	}
	got, e := a.candidateReview(context.Background(), q)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := json.MarshalIndent(got, "", "  ")
	if e != nil || !bytes.Equal(raw, read("evidence/candidates.json")) {
		t.Fatal("native candidate review differs", e)
	}
	var selected candidateSequence
	if e = json.Unmarshal(read("evidence/candidate-sequence.json"), &selected); e != nil {
		t.Fatal(e)
	}
	q.Observation, q.Row = selected.Observation.ID, selected.Row.ID
	chain, e := a.candidateSequence(context.Background(), got, q)
	if e != nil {
		t.Fatal(e)
	}
	raw, e = json.MarshalIndent(chain, "", "  ")
	if e != nil || !bytes.Equal(raw, read("evidence/candidate-sequence.json")) || chain.FASTA != string(read("evidence/candidate.fasta")) {
		t.Fatal("native supplied chain differs", e)
	}
	var likelihood likelihoodDocument
	if e = json.Unmarshal(read("evidence/likelihood.json"), &likelihood); e != nil {
		t.Fatal(e)
	}
	_, parent, panel, e := a.likelihoodParent(likelihood.Parent.ID, likelihood.Panel.ID)
	if e != nil || replayDigest(parent) != likelihood.ParentSHA256 || !bytes.Equal(mustJSON(panel), mustJSON(likelihood.Panel)) {
		t.Fatal("likelihood parent differs", e)
	}
	fit, e := bcell.Likelihood(context.Background(), panel)
	if e != nil || !bytes.Equal(mustJSON(fit), mustJSON(likelihood.Result)) {
		t.Fatal("conditional likelihood differs", e)
	}
	// Captured public evidence stays separate from the B-cell study and needs no network.
	a.adc.Offline = true
	names := []string{}
	for _, suffix := range []string{"a", "b", "c", "d"} {
		name := "adc-download-sim-" + suffix
		names = append(names, name)
		if e = os.CopyFS(filepath.Join(a.store.dir, name), os.DirFS(filepath.Join(dir, "data/public-overlap", name))); e != nil {
			t.Fatal(e)
		}
	}
	result := postJSON(t, h, "/api/adc/overlap", string(mustJSON(map[string]any{"dataset": study.QueryDataset, "sample": 0, "locus": "TRB", "captures": names, "accept": true})))
	if result.Code != 200 {
		t.Fatal(result.Code, result.Body)
	}
	var actual struct{ Document json.RawMessage }
	if e = json.Unmarshal(result.Body.Bytes(), &actual); e != nil {
		t.Fatal(e)
	}
	var before, after map[string]json.RawMessage
	if e = json.Unmarshal(read("evidence/public-overlap.json"), &before); e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(actual.Document, &after); e != nil {
		t.Fatal(e)
	}
	for _, key := range []string{"queryDataset", "querySample", "querySources", "querySourceUnit", "result"} {
		var b, c bytes.Buffer
		if e = json.Compact(&b, before[key]); e != nil {
			t.Fatal(e)
		}
		if e = json.Compact(&c, after[key]); e != nil {
			t.Fatal(e)
		}
		if !bytes.Equal(b.Bytes(), c.Bytes()) {
			t.Fatal("public evidence differs", key)
		}
	}
}
