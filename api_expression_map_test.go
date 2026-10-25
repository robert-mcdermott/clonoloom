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
)

func TestExpressionMapJobIdentityAndReopening(t *testing.T) {
	a, h := testApp(t)
	dir := t.TempDir()
	for _, name := range []string{"matrix.mtx", "features.tsv", "barcodes.tsv", "cells.tsv"} {
		raw, err := os.ReadFile(filepath.Join("testdata/expression-map", name))
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	var input expressionSummary
	opened := postJSON(t, h, "/api/expression/open", string(mustJSON(map[string]any{"path": dir, "cellsPath": filepath.Join(dir, "cells.tsv")})))
	if opened.Code != 200 {
		t.Fatal(opened.Body)
	}
	json.Unmarshal(opened.Body.Bytes(), &input)
	options := expression.AnalysisOptions{MinGenes: 2, MinCells: 2, Features: 6, Components: 4, Neighbors: 7, Epochs: 10}
	body := map[string]any{"id": input.ID, "options": options, "start": false}
	var lookup struct {
		Artifact, Job string
		Ready         bool
	}
	rec := postJSON(t, h, "/api/expression-map", string(mustJSON(body)))
	json.Unmarshal(rec.Body.Bytes(), &lookup)
	if rec.Code != 200 || lookup.Artifact == "" || lookup.Job != "" || lookup.Ready {
		t.Fatal(rec.Code, rec.Body)
	}
	body["start"] = true
	rec = postJSON(t, h, "/api/expression-map", string(mustJSON(body)))
	var started struct{ Artifact, Job string }
	json.Unmarshal(rec.Body.Bytes(), &started)
	if started.Job == "" {
		t.Fatal(rec.Body)
	}
	waitForJob(t, a, started.Job, 10*time.Second)
	var answer struct {
		Artifact artifacts.Manifest
		Result   expressionMapDocument
	}
	raw := request2(t, h, "/api/expression-map/"+started.Artifact)
	if err := json.Unmarshal(raw, &answer); err != nil {
		t.Fatal(err)
	}
	if answer.Artifact.State != artifacts.Complete || answer.Artifact.Lineage.Parents[0].ID != input.ID || len(answer.Result.Result.Cells) != 96 || answer.Result.Version != expression.AnalysisVersion || !strings.Contains(answer.Result.Description, "seed 20261024") {
		t.Fatal("missing result/provenance")
	}
	if findings, err := a.artifacts.Verify(started.Artifact, nil); err != nil || len(findings) > 0 {
		t.Fatal(err, findings)
	}
	a.expression.mu.Lock()
	a.expression.byID = nil
	a.expression.mu.Unlock()
	rec = postJSON(t, h, "/api/expression-map", string(mustJSON(body)))
	json.Unmarshal(rec.Body.Bytes(), &lookup)
	if !lookup.Ready || lookup.Artifact != started.Artifact {
		t.Fatal(rec.Body)
	}
	if err := os.Remove(filepath.Join(dir, "matrix.mtx")); err != nil {
		t.Fatal(err)
	}
	// Reading a result and its complete provenance no longer needs any source or receptor dataset.
	if got := request2(t, h, "/api/expression-map/"+started.Artifact); string(got) != string(raw) {
		t.Fatal("result changed after eviction/source removal")
	}
	for _, bad := range []string{`{"id":"missing"}`, `{"id":"missing","options":{"features":1001}}`, `{"id":"missing","options":{"unknown":1}}`} {
		if rec := postJSON(t, h, "/api/expression-map", bad); rec.Code != 400 && rec.Code != 404 {
			t.Fatal(rec.Code, rec.Body)
		}
	}
	effective, _ := options.Normalize()
	spec := expressionMapSpec{ID: input.ID, Options: effective, Method: expression.AnalysisVersion}
	if spec.artifactID() != started.Artifact {
		t.Fatal("identity mismatch")
	}
	spec.Path = "elsewhere"
	if spec.artifactID() != started.Artifact {
		t.Fatal("path entered identity")
	}
	spec.Options.Seed++
	if spec.artifactID() == started.Artifact {
		t.Fatal("seed absent from identity")
	}
	spec.Options.Seed--
	spec.Method++
	if spec.artifactID() == started.Artifact {
		t.Fatal("version absent from identity")
	}
	// Tampered results cannot be read as valid evidence.
	file, _ := a.artifacts.Path(started.Artifact, "expression-map.json")
	if err := os.WriteFile(file, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if rec := request(t, h, http.MethodGet, "/api/expression-map/"+started.Artifact, nil, nil); rec.Code != 409 {
		t.Fatal(rec.Code, rec.Body)
	}
}
func TestExpressionMapJobRejectsChangedSources(t *testing.T) {
	a, _ := testApp(t)
	in, err := expression.OpenWithCells(context.Background(), "testdata/expression-map", "testdata/expression-map/cells.tsv")
	if err != nil {
		t.Fatal(err)
	}
	options, _ := (expression.AnalysisOptions{Epochs: 10}).Normalize()
	spec := expressionMapSpec{ID: "changed", Path: in.Path, CellsPath: "testdata/expression-map/cells.tsv", Options: options, Method: expression.AnalysisVersion}
	job, _, err := a.jobs.Submit(expressionMapKind, "changed source test", spec.artifactID(), spec, a.expressionMapWork(spec))
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		j, _ := a.jobs.Get(job.ID)
		if j.State.Finished() {
			if !strings.Contains(j.Error, "sources changed") {
				t.Fatal(j)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("job timed out")
		}
		time.Sleep(time.Millisecond * 10)
	}
	if _, err := a.artifacts.Get(spec.artifactID()); err == nil {
		t.Fatal("failed job promoted an artifact")
	}
}
