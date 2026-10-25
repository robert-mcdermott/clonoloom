package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"clonoloom/internal/artifacts"
	"clonoloom/internal/clonotype"
	"clonoloom/internal/expression"
	"clonoloom/internal/phenotype"
	"clonoloom/internal/study"
)

func TestPhenotypeArtifactsAndBindings(t *testing.T) {
	a, h := testApp(t)
	dir := t.TempDir()
	if err := expressionInputExample().Make(context.Background(), dir, nil); err != nil {
		t.Fatal(err)
	}
	cells := filepath.Join(dir, "cells.tsv")
	if err := os.WriteFile(cells, []byte("barcode\tstate\tmap_x\tmap_y\nAA-1\tA\t0\t1\nBB-1\t\t2\t3\nCC-1\tMissing\t\t\nDD-1\tB\t4\t5\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var ds datasetSummary
	rec := postJSON(t, h, "/api/datasets/open", string(mustJSON(map[string]any{"paths": []string{filepath.Join(dir, "receptors.tsv")}})))
	if rec.Code != 200 {
		t.Fatal(rec.Body)
	}
	json.Unmarshal(rec.Body.Bytes(), &ds)
	var in expressionSummary
	rec = postJSON(t, h, "/api/expression/open", string(mustJSON(map[string]any{"path": dir, "cellsPath": cells})))
	if rec.Code != 200 {
		t.Fatal(rec.Body)
	}
	json.Unmarshal(rec.Body.Bytes(), &in)
	sample := 0
	req := phenotypeRequest{ID: in.ID, Dataset: ds.ID, Sample: &sample, Definition: clonotype.Definition{Chains: []string{"TRA", "TRB"}}, Annotation: study.Sample{Name: ds.Samples[0].Name, Subject: "P1", Time: "day 0"}, Options: phenotype.Options{Annotation: "state"}}
	start := func(req phenotypeRequest) struct {
		Artifact, Job string
		Ready         bool
	} {
		t.Helper()
		rec := postJSON(t, h, "/api/phenotypes", string(mustJSON(req)))
		if rec.Code != 200 {
			t.Fatal(rec.Code, rec.Body)
		}
		var s struct {
			Artifact, Job string
			Ready         bool
		}
		json.Unmarshal(rec.Body.Bytes(), &s)
		return s
	}
	s := start(req)
	waitForJob(t, a, s.Job, 10*time.Second)
	raw := request2(t, h, "/api/phenotypes/"+s.Artifact)
	var answer struct {
		Artifact artifacts.Manifest
		Result   phenotypeDocument
	}
	if err := json.Unmarshal(raw, &answer); err != nil {
		t.Fatal(err)
	}
	r := answer.Result.Result
	if r == nil || r.Coverage.Matched != 2 || r.Coverage.Mapped != 3 || r.Cells[0].Size == nil || *r.Cells[0].Size != 1 || r.Cells[1].Status != "Unassigned clonotype" || len(r.Groups) != 4 {
		t.Fatalf("%+v", r)
	}
	if answer.Artifact.Lineage.Parents[0].ID != ds.ID || answer.Result.Annotation.Subject != "P1" || r.DefinitionID == "" {
		t.Fatal("lost provenance")
	}
	if !start(req).Ready {
		t.Fatal("cache")
	}
	spec := phenotypeSpec{phenotypeRequest: req, Method: phenotype.Version}
	spec.Definition, _ = spec.Definition.Normalize()
	spec.Options, _ = spec.Options.Normalize()
	id := spec.artifactID()
	if id != s.Artifact {
		t.Fatal("identity")
	}
	spec.Input.Path = "elsewhere"
	spec.Definition.Name = "renamed"
	if spec.artifactID() != id {
		t.Fatal("paths/name changed identity")
	}
	changed := []phenotypeSpec{spec, spec, spec, spec, spec}
	changed[0].Annotation.Subject = "P2"
	changed[1].Options.Population = "mapped"
	changed[2].Filter.ProductiveOnly = true
	changed[3].MapSHA256 = "other"
	changed[4].Method++
	for _, c := range changed {
		if c.artifactID() == id {
			t.Fatal("semantic identity absent")
		}
	}
	for _, bad := range []string{`{}`, `{"id":"x","dataset":"x","sample":0,"annotation":{"name":"s"},"options":{"population":"pooled"}}`, `{"id":"x","dataset":"x","sample":0,"unknown":1}`} {
		if rec := postJSON(t, h, "/api/phenotypes", bad); rec.Code != 400 {
			t.Fatal(rec.Code, rec.Body)
		}
	}
	// A computed map must name this exact expression input and pass checksum verification.
	mapSpec := expressionMapSpec{ID: "another-input", Options: expression.AnalysisOptions{}, Method: expression.AnalysisVersion}
	mapID := mapSpec.artifactID()
	stage, err := a.artifacts.Stage(mapID)
	if err != nil {
		t.Fatal(err)
	}
	parent := expressionMapDocument{Input: expressionSummary{ID: in.ID}, Result: &expression.AnalysisResult{Axes: [2]string{"UMAP 1", "UMAP 2"}, Cells: []expression.AnalysisCell{{Barcode: "AA-1", Position: &[2]float64{8, 9}}}}}
	file := expressionMapKind + ".json"
	if err := os.WriteFile(filepath.Join(stage, file), mustJSON(parent), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := a.artifacts.Promote(artifacts.Manifest{ID: mapID, Kind: expressionMapKind, Name: "Fixture map", Files: []artifacts.File{{Path: file}}}, nil); err != nil {
		t.Fatal(err)
	}
	req.Map = mapID
	s2 := start(req)
	waitForJob(t, a, s2.Job, 10*time.Second)
	var computed struct{ Result phenotypeDocument }
	json.Unmarshal(request2(t, h, "/api/phenotypes/"+s2.Artifact), &computed)
	if computed.Result.Supplied || computed.Result.Result.Coverage.Mapped != 1 || *computed.Result.Result.Cells[0].Position != [2]float64{8, 9} || computed.Result.Result.Cells[1].Position != nil {
		t.Fatal("map parent positions")
	}
	req.ID = "mismatched"
	if rec := postJSON(t, h, "/api/phenotypes", string(mustJSON(req))); rec.Code != 400 {
		t.Fatal(rec.Code, rec.Body)
	}
	req.ID = in.ID
	path, _ := a.artifacts.Path(mapID, file)
	os.WriteFile(path, []byte("{}"), 0600)
	if rec := postJSON(t, h, "/api/phenotypes", string(mustJSON(req))); rec.Code != 409 {
		t.Fatal(rec.Code, rec.Body)
	}
	// Saved phenotype files stand alone, even when parent/source files disappear.
	a.expression.mu.Lock()
	a.expression.byID = nil
	a.expression.mu.Unlock()
	os.Remove(filepath.Join(dir, "matrix.mtx"))
	if string(request2(t, h, "/api/phenotypes/"+s.Artifact)) != string(raw) {
		t.Fatal("saved result depends on inputs")
	}
	req.Map = ""
	if !start(req).Ready {
		t.Fatal("cached result depends on inputs")
	}
	path, _ = a.artifacts.Path(s.Artifact, "phenotypes.json")
	os.WriteFile(path, []byte("{}"), 0600)
	if rec := request(t, h, http.MethodGet, "/api/phenotypes/"+s.Artifact, nil, nil); rec.Code != 409 {
		t.Fatal(rec.Code, rec.Body)
	}
}
func TestPhenotypeJobRejectsChangedSourceAndCancellation(t *testing.T) {
	a, _ := testApp(t)
	in, err := expression.Open(context.Background(), "testdata/expression-mex")
	if err != nil {
		t.Fatal(err)
	}
	sample := 0
	spec := phenotypeSpec{phenotypeRequest: phenotypeRequest{ID: "changed", Dataset: "not-open", Sample: &sample}, Input: expressionMapSpec{Path: in.Path}, Method: phenotype.Version}
	job, _, err := a.jobs.Submit(phenotypeKind, "changed source", spec.artifactID(), spec, a.phenotypeWork(spec))
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		j, _ := a.jobs.Get(job.ID)
		if j.State.Finished() {
			if j.State != "failed" || j.Error == "" {
				t.Fatal(j)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("job timeout")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if _, err := a.artifacts.Get(spec.artifactID()); err == nil {
		t.Fatal("failed job promoted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := a.phenotypeWork(spec)(ctx, nil); err == nil {
		t.Fatal("cancelled job completed")
	}
}
