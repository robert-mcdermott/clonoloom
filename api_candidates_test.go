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

	"clonoloom/internal/bcell"
	"clonoloom/internal/examples"
)

func candidateAPIFixture(t *testing.T) (*app, http.Handler, candidateRequest) {
	t.Helper()
	a, h := testApp(t)
	dir := t.TempDir()
	refdir := filepath.Join(dir, simulatedGermlines, "reference")
	os.MkdirAll(refdir, 0700)
	raw, e := os.ReadFile("validation/projects/0.5.0/references/" + simulatedGermlines + "/reference/germlines.fasta")
	if e != nil {
		t.Fatal(e)
	}
	os.WriteFile(filepath.Join(refdir, "germlines.fasta"), raw, 0600)
	a.examples = &examples.Library{Dir: dir, Generated: []examples.Generated{{Source: examples.Source{Name: simulatedGermlines, License: "Artificial"}, Marker: "reference/germlines.fasta", Files: func(string) [][]string { return nil }}}}
	paths := []string{}
	for _, sample := range []string{"S01-d14", "S01-d28", "S01-pre", "S02-d14", "S02-d28", "S02-pre"} {
		name := sample + ".tsv"
		raw, e := os.ReadFile(filepath.Join("testdata/candidates", name))
		if e != nil {
			t.Fatal(e)
		}
		path := filepath.Join(dir, name)
		os.WriteFile(path, raw, 0600)
		paths = append(paths, path)
	}
	ds, e := a.openDataset(context.Background(), paths, nil, "Artificial candidate review")
	if e != nil {
		t.Fatal(e)
	}
	ds.Credits = []credit{{Title: "Artificial restricted controls", LicenseClass: "terms", License: "Explicit permission required"}}
	start := func(kind string, body any) string {
		t.Helper()
		w := postJSON(t, h, "/api/"+kind, string(mustJSON(body)))
		var job struct{ Job, Artifact string }
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &job) != nil || job.Job == "" {
			t.Fatal(kind, w.Code, w.Body)
		}
		waitForJob(t, a, job.Job, 30*time.Second)
		return job.Artifact
	}
	subjects := map[string]string{}
	for _, s := range ds.Result.Table.Samples {
		subjects[s.Name] = strings.Split(s.Name, "-")[0]
	}
	fam := start("families", map[string]any{"dataset": ds.ID, "subjects": subjects, "families": map[string]any{"mode": "paired", "scope": "subject", "threshold": 0.1}})
	germ := start("germlines", map[string]any{"dataset": ds.ID, "germlines": map[string]any{"families": fam, "reference": simulatedGermlines}})
	shm := start("shm", map[string]any{"dataset": ds.ID, "shm": map[string]any{"germlines": germ}})
	iso := start("isotypes", map[string]any{"dataset": ds.ID, "isotypes": map[string]any{"families": fam}})
	return a, h, candidateRequest{Families: fam, SHM: shm, Isotypes: iso, Options: bcell.CandidateOptions{Sample: "S01-d28", Baseline: "S01-pre", PreferredClass: "IgG", Criteria: []bcell.CandidateCriterion{{Metric: "change", Direction: "descending"}, {Metric: "mutation", Direction: "descending"}}}}
}
func TestCandidatesNativeEvidenceAndPermissions(t *testing.T) {
	a, h, q := candidateAPIFixture(t)
	w := postJSON(t, h, "/api/bcell-candidates/review", string(mustJSON(q)))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	var response struct {
		Review   string
		Evidence candidateDocument
	}
	if e := json.Unmarshal(w.Body.Bytes(), &response); e != nil {
		t.Fatal(e)
	}
	q.Review = response.Review
	// The historical subset has 24/4 heavy rows (cells) and two S01 families.
	if len(response.Evidence.Parents) != 4 || len(response.Evidence.Result.Rows) != 2 || response.Evidence.Result.CurrentTotal != 24 || response.Evidence.Result.BaselineTotal != 4 {
		t.Fatalf("unexpected candidate denominators: %d/%d, %d families", response.Evidence.Result.CurrentTotal, response.Evidence.Result.BaselineTotal, len(response.Evidence.Result.Rows))
	}
	if w = postJSON(t, h, "/api/bcell-candidates/export", string(mustJSON(q))); w.Code != 403 {
		t.Fatal(w.Code, w.Body)
	}
	q.AcceptTerms = true
	w = postJSON(t, h, "/api/bcell-candidates/export", string(mustJSON(q)))
	if w.Code != 200 || replayDigest(w.Body.Bytes()) != q.Review {
		t.Fatal(w.Code, w.Body)
	}
	reviewBytes := append([]byte(nil), w.Body.Bytes()...)
	obs := response.Evidence.Result.Rows[0].Observations[0]
	q.Observation = obs.ID
	q.Row = obs.Rows[0].ID
	w = postJSON(t, h, "/api/bcell-candidates/selection", string(mustJSON(q)))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	var selected struct {
		Selection string
		Evidence  candidateSequence
	}
	if e := json.Unmarshal(w.Body.Bytes(), &selected); e != nil {
		t.Fatal(e)
	}
	q.Selection = selected.Selection
	if !selected.Evidence.Eligible || selected.Evidence.Fields["sequence_aa"] != selected.Evidence.Sequence || replayDigest([]byte(selected.Evidence.Sequence)) != selected.Evidence.SHA256 {
		t.Fatal(selected.Evidence.Eligible, selected.Evidence.Reason, selected.Evidence.Row.SequenceID)
	}
	w = postJSON(t, h, "/api/bcell-candidates/selection-export", string(mustJSON(q)))
	if w.Code != 200 || replayDigest(w.Body.Bytes()) != q.Selection {
		t.Fatal(w.Code, w.Body)
	}
	selectionBytes := append([]byte(nil), w.Body.Bytes()...)
	if w = postJSON(t, h, "/api/bcell-candidates/fasta", string(mustJSON(q))); w.Code != 403 {
		t.Fatal(w.Code, w.Body)
	}
	q.AcceptSequence = true
	w = postJSON(t, h, "/api/bcell-candidates/fasta", string(mustJSON(q)))
	if w.Code != 200 || w.Body.String() != selected.Evidence.FASTA {
		t.Fatal(w.Code, w.Body)
	}
	if dir := os.Getenv("CLONOLOOM_CANDIDATES_OUT"); dir != "" {
		os.MkdirAll(dir, 0700)
		os.WriteFile(filepath.Join(dir, "api-review.json"), reviewBytes, 0600)
		os.WriteFile(filepath.Join(dir, "api-selection.json"), selectionBytes, 0600)
		os.WriteFile(filepath.Join(dir, "api.fasta"), w.Body.Bytes(), 0600)
	}

	// An original nucleotide sequence must not fill a missing sequence_aa.
	missingChecked := false
	for _, family := range response.Evidence.Result.Rows {
		for _, observation := range family.Observations {
			for _, row := range observation.Rows {
				if row.Line != 2 || row.Source.Name != "S01-d28.tsv" {
					continue
				}
				missing := q
				missingChecked = true
				missing.Observation = observation.ID
				missing.Row = row.ID
				w = postJSON(t, h, "/api/bcell-candidates/selection", string(mustJSON(missing)))
				if w.Code != 200 {
					t.Fatal(w.Code, w.Body)
				}
				var absent struct {
					Selection string
					Evidence  candidateSequence
				}
				if e := json.Unmarshal(w.Body.Bytes(), &absent); e != nil {
					t.Fatal(e)
				}
				if absent.Evidence.Eligible || absent.Evidence.Sequence != "" || absent.Evidence.Fields["sequence"] == "" {
					t.Fatal("missing amino acids repaired")
				}
				missing.Selection = absent.Selection
				if w = postJSON(t, h, "/api/bcell-candidates/fasta", string(mustJSON(missing))); w.Code != 403 {
					t.Fatal(w.Code, w.Body)
				}
			}
		}
	}
	if !missingChecked {
		t.Fatal("missing-amino-acid control was not exercised")
	}
	bad := q
	bad.Observation = "not-selected"
	if w = postJSON(t, h, "/api/bcell-candidates/selection", string(mustJSON(bad))); w.Code != 400 {
		t.Fatal(w.Code, w.Body)
	}
	bad = q
	bad.Options.Baseline = "S02-pre"
	if w = postJSON(t, h, "/api/bcell-candidates/review", string(mustJSON(bad))); w.Code != 400 {
		t.Fatal(w.Code, w.Body)
	}
	bad = q
	bad.Selection = "changed"
	if w = postJSON(t, h, "/api/bcell-candidates/fasta", string(mustJSON(bad))); w.Code != 409 {
		t.Fatal(w.Code, w.Body)
	}
	ds, _ := a.datasets.get(response.Evidence.Dataset)
	source := ds.Result.Sources[ds.Result.Table.Source[selected.Evidence.RowIndex]]
	os.WriteFile(source.Path, []byte("changed"), 0600)
	if w = postJSON(t, h, "/api/bcell-candidates/fasta", string(mustJSON(q))); w.Code != 400 || !strings.Contains(w.Body.String(), "Source bytes differ") {
		t.Fatal(w.Code, w.Body)
	}
	ds.Credits = nil
	if w = postJSON(t, h, "/api/bcell-candidates/export", string(mustJSON(q))); w.Code != 409 {
		t.Fatal(w.Code, w.Body)
	}
	path, _ := a.artifacts.Path(q.Families, "families.json")
	os.WriteFile(path, []byte("{}"), 0600)
	if w = postJSON(t, h, "/api/bcell-candidates/review", string(mustJSON(q))); w.Code != 400 || !strings.Contains(w.Body.String(), "checksum") {
		t.Fatal(w.Code, w.Body)
	}
}
