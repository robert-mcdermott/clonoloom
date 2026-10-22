package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// runAnalysis opens a dataset of two samples, looks for the kind's result (there is none), starts
// its job and waits for it, asks again (the kept result answers at once) and reads the result back.
func runAnalysis(t *testing.T, kind string) []byte {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "study.tsv")
	os.WriteFile(path, []byte(datasetTSV), 0o644)
	a, handler := testApp(t)
	open := postJSON(t, handler, "/api/datasets/open", `{"paths": [`+jsonString(path)+`]}`)
	var s datasetSummary
	json.Unmarshal(open.Body.Bytes(), &s)
	request := `{"dataset": "` + s.ID + `", "definition": {"name": "AA", "junction": "aa", "v": "gene", "j": "gene"}, "options": {"replicates": 50}}`
	look := postJSON(t, handler, "/api/"+kind, strings.Replace(request, "{\"dataset\"", "{\"start\": false, \"dataset\"", 1))
	var first map[string]any
	json.Unmarshal(look.Body.Bytes(), &first)
	if look.Code != http.StatusOK || first["ready"] != false || first["job"] != nil {
		t.Fatalf("looked: %d %s", look.Code, look.Body)
	}
	started := postJSON(t, handler, "/api/"+kind, request)
	var job map[string]any
	json.Unmarshal(started.Body.Bytes(), &job)
	if started.Code != http.StatusOK || job["job"] == nil || job["artifact"] != first["artifact"] {
		t.Fatalf("started: %d %s", started.Code, started.Body)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		j, _ := a.jobs.Get(job["job"].(string))
		if j.State.Finished() {
			if j.Error != "" {
				t.Fatal(j.Error)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the job did not finish")
		}
		time.Sleep(20 * time.Millisecond)
	}
	again := postJSON(t, handler, "/api/"+kind, request)
	if !strings.Contains(again.Body.String(), `"ready":true`) {
		t.Fatalf("asked again: %s", again.Body)
	}
	if rec := postJSON(t, handler, "/api/"+kind, `{"dataset": "d-nothing", "definition": {"junction": "aa"}}`); rec.Code != http.StatusNotFound {
		t.Errorf("a dataset not open: %d", rec.Code)
	}
	return request2(t, handler, "/api/"+kind+"/"+job["artifact"].(string))
}

// Diversity is measured by a job and kept as an artifact named for what it measured: asked again,
// the kept result answers at once; read back, it holds every sample.
func TestDiversityIsMeasuredAndKept(t *testing.T) {
	got := runAnalysis(t, "diversity")
	var read struct {
		Artifact struct {
			ID    string `json:"id"`
			Kind  string `json:"kind"`
			State string `json:"state"`
		} `json:"artifact"`
		Result diversityDocument `json:"result"`
	}
	if err := json.Unmarshal(got, &read); err != nil {
		t.Fatalf("%v: %s", err, got)
	}
	if read.Artifact.Kind != "diversity" || read.Artifact.State != "complete" || !strings.HasPrefix(read.Artifact.ID, "div-") || len(read.Result.Samples) != 2 || read.Result.Unit != "umis" || read.Result.Options.Replicates != 50 {
		t.Fatalf("%+v", read)
	}
	r1 := read.Result.Samples[0].Result
	if r1.Clonotypes != 1 || float64(r1.Measures["richness"].Estimate) != 1 {
		t.Errorf("R1: %+v", r1)
	}
}

// Rarefaction alike: every sample's curves, the levels suggested and the values there.
func TestRarefactionIsMeasuredAndKept(t *testing.T) {
	got := runAnalysis(t, "rarefaction")
	var read struct {
		Artifact struct {
			ID   string `json:"id"`
			Kind string `json:"kind"`
		} `json:"artifact"`
		Result rarefactionDocument `json:"result"`
	}
	if err := json.Unmarshal(got, &read); err != nil {
		t.Fatalf("%v: %s", err, got)
	}
	doc := read.Result
	if read.Artifact.Kind != "rarefaction" || !strings.HasPrefix(read.Artifact.ID, "rar-") || len(doc.Samples) != 2 || doc.Knots != 40 || doc.Levels == nil {
		t.Fatalf("%+v", read)
	}
	// R1 holds 3 UMIs, R2 5, each of one clonotype: compared at 3 UMIs (or 6, extrapolated) and at
	// coverage 1.
	if l := doc.Levels; l.Size != 3 || l.ExtrapolatedSize != 6 || l.Coverage != 1 || l.Samples != 2 {
		t.Errorf("levels %+v", l)
	}
	r2 := doc.Samples[1].Curves
	if len(r2.BySize) == 0 || len(r2.AtSize) != 2 || len(r2.AtCoverage) != 2 || float64(r2.AtSize[0].Size) != 3 || float64(r2.AtSize[0].Hill[0].Estimate) != 1 {
		t.Errorf("R2: %+v", r2)
	}
}

func request2(t *testing.T, handler http.Handler, path string) []byte {
	t.Helper()
	rec := request(t, handler, http.MethodGet, path, nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("%s: %d %s", path, rec.Code, rec.Body)
	}
	return rec.Body.Bytes()
}
