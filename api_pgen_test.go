package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"clonoloom/internal/pgen"
)

// Generation probabilities are an analysis like the others: with ClonoLoom's own model when
// OLGA's are not downloaded (and a request for OLGA's refused so that the page offers them); each
// TRB clonotype's probability, none for a locus without a model; a clonotype's chains answered on
// request; exported; and with OLGA's models when they are fetched.
func TestPgenOfClonotypes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trb.tsv")
	os.WriteFile(path, []byte(tcrdistTSV), 0o644)
	a, handler := testApp(t)
	open := postJSON(t, handler, "/api/datasets/open", `{"paths": [`+jsonString(path)+`]}`)
	var sum datasetSummary
	json.Unmarshal(open.Body.Bytes(), &sum)
	base := `"dataset": "` + sum.ID + `", "definition": {"name": "AA", "junction": "aa", "v": "gene", "j": "gene"}`

	if rec := postJSON(t, handler, "/api/pgen", `{`+base+`, "pgen": {"models": "olga"}}`); rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"no-models"`) {
		t.Fatalf("OLGA's models not downloaded: %d %s", rec.Code, rec.Body)
	}
	started := postJSON(t, handler, "/api/pgen", `{`+base+`}`)
	var job map[string]any
	json.Unmarshal(started.Body.Bytes(), &job)
	if started.Code != http.StatusOK || job["job"] == nil || !strings.HasPrefix(job["artifact"].(string), "pgn-") {
		t.Fatalf("started: %d %s", started.Code, started.Body)
	}
	id := job["artifact"].(string)
	waitForJob(t, a, job["job"].(string), 30*time.Second)
	var read struct {
		Result pgenDocument `json:"result"`
	}
	if err := json.Unmarshal(request2(t, handler, "/api/pgen/"+id), &read); err != nil {
		t.Fatal(err)
	}
	doc := read.Result
	if doc.Models != "clonoloom" || doc.Set.Checksum != clonoloomModel || doc.Organism != "human" || len(doc.Samples) != 2 || doc.Without["no model of human IGH"] != 1 {
		t.Fatalf("%+v", doc)
	}
	own := pgen.Simulator()
	for _, r := range doc.Rows {
		if r.Chains[0].Locus != "TRB" {
			if r.Pgen != -1 {
				t.Errorf("IGH has a probability: %+v", r)
			}
			continue
		}
		want, _ := own.AA(r.Chains[0].Junction, nil, nil)
		if r.Pgen != want || r.Pgen <= 0 || r.Chain[0] != want {
			t.Errorf("%s: %g, the model %g", r.Chains[0].Junction, r.Pgen, want)
		}
	}
	if doc.Samples[0].With != 2 || doc.Samples[0].Quartiles[1] >= 0 {
		t.Errorf("samples %+v", doc.Samples)
	}
	if rec := request(t, handler, http.MethodGet, "/api/export?what=result&artifact="+id, nil, nil); rec.Code != http.StatusOK {
		t.Errorf("export: %d %s", rec.Code, rec.Body)
	}
	// A clonotype's chains, on request, with its genes.
	rec := postJSON(t, handler, "/api/pgen/chains", `{"chains": [{"locus": "TRB", "junction": "CASSLAGGPEAFF", "v": "TRBV5-1", "j": "TRBJ1-1"}], "junction": "aa", "pgen": {"genes": true}}`)
	var answer struct {
		Chains []struct {
			Pgen  *float64 `json:"pgen"`
			Model string   `json:"model"`
		} `json:"chains"`
		Pgen *float64 `json:"pgen"`
	}
	json.Unmarshal(rec.Body.Bytes(), &answer)
	alone, _ := own.AA("CASSLAGGPEAFF", nil, nil)
	if rec.Code != http.StatusOK || answer.Pgen == nil || *answer.Pgen <= 0 || *answer.Pgen >= alone || answer.Chains[0].Model != own.Name {
		t.Fatalf("chains: %d %s (alone %g)", rec.Code, rec.Body, alone)
	}

	// With OLGA's models (fetched for the validation, when they are).
	src := filepath.Join("validation", "cache", "olga-models")
	if _, err := os.Stat(filepath.Join(src, "human_T_beta", "model_params.txt")); err != nil {
		t.Skip("olga-models not fetched (node validation/fetch.mjs olga-models)")
	}
	copyTree(t, src, filepath.Join(a.references.Dir, "olga-models"))
	started = postJSON(t, handler, "/api/pgen", `{`+base+`}`)
	json.Unmarshal(started.Body.Bytes(), &job)
	waitForJob(t, a, job["job"].(string), 30*time.Second)
	var again struct {
		Result pgenDocument `json:"result"`
	}
	json.Unmarshal(request2(t, handler, "/api/pgen/"+job["artifact"].(string)), &again)
	doc = again.Result
	if doc.Models != "olga" || doc.Loci["TRB"] != "human_T_beta" || doc.Loci["IGH"] != "human_B_heavy" || len(doc.Without) != 0 {
		t.Fatalf("with OLGA's: %+v", doc)
	}
}

func copyTree(t *testing.T, from, to string) {
	t.Helper()
	filepath.Walk(from, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(from, path)
		os.MkdirAll(filepath.Join(to, filepath.Dir(rel)), 0o755)
		in, _ := os.Open(path)
		defer in.Close()
		out, _ := os.Create(filepath.Join(to, rel))
		defer out.Close()
		io.Copy(out, in)
		return nil
	})
}
