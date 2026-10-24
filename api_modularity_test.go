package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"clonoloom/internal/artifacts"
	"clonoloom/internal/expression"
	"clonoloom/internal/graph"
	"clonoloom/internal/modularity"
	"clonoloom/internal/phenotype"
	"clonoloom/internal/study"
)

func modularityParentsFixture(t *testing.T, a *app, edit ...func(*phenotypeDocument, *string)) (string, string) {
	t.Helper()
	save := func(id, kind string, doc any, parents []artifacts.Parent) string {
		t.Helper()
		dir, err := a.artifacts.Stage(id)
		if err != nil {
			t.Fatal(err)
		}
		file := kind + ".json"
		if err = os.WriteFile(filepath.Join(dir, file), mustJSON(doc), 0600); err != nil {
			t.Fatal(err)
		}
		m, err := a.artifacts.Promote(artifacts.Manifest{ID: id, Kind: kind, Name: "Fixture " + kind, Lineage: artifacts.Lineage{Parents: parents}, Files: []artifacts.File{{Path: file}}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		return m.Files[0].SHA256
	}
	cells := []expression.AnalysisCell{{Barcode: "a"}, {Barcode: "b"}, {Barcode: "c"}, {Barcode: "d"}, {Barcode: "e"}}
	g := &graph.CSR{N: 5, Offsets: []int{0, 2, 4, 6, 7, 8}, Targets: []int32{1, 2, 0, 2, 0, 1, 4, 3}, Weights: []float64{1, 1, 1, 1, 1, 1, 1, 1}}
	m := expressionMapDocument{Format: "clonoloom-expression-map", Version: expression.AnalysisVersion, Input: expressionSummary{ID: "expr-fixture"}, Result: &expression.AnalysisResult{Cells: cells, Graph: g}}
	hash := save("emap-modularity-fixture", expressionMapKind, m, nil)
	p := phenotypeDocument{Format: "clonoloom-phenotypes", Version: phenotype.Version, Dataset: "receptor-fixture", Input: m.Input, Map: "emap-modularity-fixture", Annotation: study.Sample{Name: "S1", Subject: "P1"}, Result: &phenotype.Result{Sample: "S1", Cells: []phenotype.Cell{{Barcode: "a", Clone: "A", Status: "Assigned"}, {Barcode: "b", Clone: "A", Status: "Assigned"}, {Barcode: "c", Clone: "A", Status: "Assigned"}, {Barcode: "d", Clone: "B", Status: "Assigned"}, {Barcode: "e", Clone: "B", Status: "Assigned"}, {Barcode: "outside", Clone: "A", Status: "Assigned"}}}}
	for _, change := range edit {
		change(&p, &hash)
	}
	save("pheno-modularity-fixture", phenotypeKind, p, []artifacts.Parent{{Kind: expressionMapKind, ID: p.Map, SHA256: hash}})
	return "pheno-modularity-fixture", p.Map
}

func TestModularityRefusesUnusablePhenotypeParents(t *testing.T) {
	for name, change := range map[string]func(*phenotypeDocument, *string){
		"imported":         func(p *phenotypeDocument, _ *string) { p.Supplied = true; p.Map = "" },
		"other input":      func(p *phenotypeDocument, _ *string) { p.Input.ID = "other" },
		"version":          func(p *phenotypeDocument, _ *string) { p.Version++ },
		"unrecorded graph": func(_ *phenotypeDocument, hash *string) { *hash = "other" },
	} {
		t.Run(name, func(t *testing.T) {
			a, h := testApp(t)
			parent, _ := modularityParentsFixture(t, a, change)
			if rec := postJSON(t, h, "/api/modularity", string(mustJSON(modularityRequest{Phenotypes: parent}))); rec.Code != 409 {
				t.Fatal(rec.Code, rec.Body)
			}
		})
	}
}
func TestModularityArtifactWorkflow(t *testing.T) {
	a, h := testApp(t)
	parent, mapID := modularityParentsFixture(t, a)
	start := func(body string) struct {
		Artifact, Job string
		Ready         bool
	} {
		t.Helper()
		rec := postJSON(t, h, "/api/modularity", body)
		if rec.Code != 200 {
			t.Fatal(rec.Code, rec.Body)
		}
		var s struct {
			Artifact, Job string
			Ready         bool
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	body := string(mustJSON(modularityRequest{Phenotypes: parent}))
	s := start(body)
	waitForJob(t, a, s.Job, 10*time.Second)
	raw := request2(t, h, "/api/modularity/"+s.Artifact)
	var answer struct {
		Artifact artifacts.Manifest
		Result   modularityDocument
	}
	if err := json.Unmarshal(raw, &answer); err != nil {
		t.Fatal(err)
	}
	if answer.Result.Result == nil || answer.Result.Result.Tested != 2 || answer.Result.ExpressionExcluded != 1 || answer.Result.Phenotypes.Annotation.Subject != "P1" || answer.Result.Result.Eligible != 5 {
		t.Fatal(answer.Result)
	}
	if len(answer.Artifact.Lineage.Parents) != 2 || answer.Artifact.Lineage.Parents[0].SHA256 == "" || answer.Artifact.Lineage.Parents[1].SHA256 == "" || !strings.Contains(answer.Result.Description, "exchangeable") {
		t.Fatal("missing provenance")
	}
	if !start(body).Ready {
		t.Fatal("cache miss")
	}
	var spec modularitySpec
	json.Unmarshal(answer.Artifact.Lineage.Job.Spec, &spec)
	if spec.artifactID() != s.Artifact {
		t.Fatal("identity")
	}
	remade, err := a.jobs.Remake(modularityKind, mustJSON(spec))
	if err != nil || remade == nil {
		t.Fatal("resume handler", err)
	}
	badVersion := spec
	badVersion.Method++
	if _, err := a.jobs.Remake(modularityKind, mustJSON(badVersion)); err == nil {
		t.Fatal("resumed changed method")
	}
	for _, change := range []func(*modularitySpec){func(s *modularitySpec) { s.Options.Seed++ }, func(s *modularitySpec) { s.Options.Permutations++ }, func(s *modularitySpec) { s.PhenotypesSHA256 = "other" }, func(s *modularitySpec) { s.MapSHA256 = "other" }, func(s *modularitySpec) { s.Method++ }} {
		next := spec
		change(&next)
		if next.artifactID() == s.Artifact {
			t.Fatal("identity omits settings/parents")
		}
	}
	for _, bad := range []string{`{}`, `{"phenotypes":"x","options":{"permutations":1}}`, `{"phenotypes":"x","unknown":1}`} {
		if rec := postJSON(t, h, "/api/modularity", bad); rec.Code != 400 {
			t.Fatal(rec.Code, rec.Body)
		}
	}
	lookup := start(string(mustJSON(map[string]any{"phenotypes": parent, "options": modularity.Options{Seed: 17}, "start": false})))
	if lookup.Job != "" || lookup.Ready || lookup.Artifact == s.Artifact {
		t.Fatal(lookup)
	}
	// A changed parent must fail a new job without promoting anything.
	path, _ := a.artifacts.Path(mapID, expressionMapKind+".json")
	os.WriteFile(path, []byte("{}"), 0600)
	spec.Options.Seed = 123
	j, _, err := a.jobs.Submit(modularityKind, "changed parent", spec.artifactID(), spec, a.modularityWork(spec))
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		v, _ := a.jobs.Get(j.ID)
		if v.State.Finished() {
			if v.State != "failed" {
				t.Fatal(v)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("job timeout")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if _, err := a.artifacts.Get(spec.artifactID()); err == nil {
		t.Fatal("promoted changed parent")
	}
	if rec := postJSON(t, h, "/api/modularity", body); rec.Code != 409 {
		t.Fatal(rec.Code, rec.Body)
	}
	// Standalone result reads don't require either parent, nor any expression/receptor source.
	path, _ = a.artifacts.Path(parent, phenotypeKind+".json")
	os.Remove(path)
	if string(request2(t, h, "/api/modularity/"+s.Artifact)) != string(raw) {
		t.Fatal("saved result needs parent")
	}
	path, _ = a.artifacts.Path(s.Artifact, modularityKind+".json")
	os.WriteFile(path, []byte("{}"), 0600)
	if rec := request(t, h, http.MethodGet, "/api/modularity/"+s.Artifact, nil, nil); rec.Code != 409 {
		t.Fatal(rec.Code, rec.Body)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := a.modularityWork(spec)(ctx, nil); err != context.Canceled {
		t.Fatal(err)
	}
}
