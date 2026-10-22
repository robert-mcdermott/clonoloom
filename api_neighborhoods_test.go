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

// Neighborhoods are an analysis like the others: the dataset's commonest locus against a
// background drawn from a recombination model, each centroid with its radius, its neighborhood and
// its enrichment; the loci answered on request; a shuffled background instead; and exported.
func TestNeighborhoodsOfClonotypes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trb.tsv")
	os.WriteFile(path, []byte(tcrdistTSV), 0o644)
	a, handler := testApp(t)
	open := postJSON(t, handler, "/api/datasets/open", `{"paths": [`+jsonString(path)+`]}`)
	var sum datasetSummary
	json.Unmarshal(open.Body.Bytes(), &sum)
	base := `"dataset": "` + sum.ID + `", "definition": {"name": "AA", "junction": "aa", "v": "gene", "j": "gene"}`

	// The loci, before any result.
	loci := postJSON(t, handler, "/api/neighborhoods/loci", `{`+base+`}`)
	var answer struct {
		Locus string         `json:"locus"`
		Loci  map[string]int `json:"loci"`
	}
	json.Unmarshal(loci.Body.Bytes(), &answer)
	if loci.Code != http.StatusOK || answer.Locus != "TRB" || answer.Loci["TRB"] == 0 {
		t.Fatalf("loci: %d %s", loci.Code, loci.Body)
	}

	// Without the V genes' loops, the CDR3s alone are compared; a definition by V gene needs them.
	if rec := postJSON(t, handler, "/api/neighborhoods", `{`+base+`}`); rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"no-loops"`) {
		t.Fatalf("the loops not downloaded: %d %s", rec.Code, rec.Body)
	}
	byFamily := `"dataset": "` + sum.ID + `", "definition": {"name": "Family", "junction": "aa", "v": "family", "j": "gene"}`
	started := postJSON(t, handler, "/api/neighborhoods", `{`+byFamily+`, "neighborhoods": {"size": 2000}}`)
	var job map[string]any
	json.Unmarshal(started.Body.Bytes(), &job)
	if started.Code != http.StatusOK || job["job"] == nil || !strings.HasPrefix(job["artifact"].(string), "nbh-") {
		t.Fatalf("started: %d %s", started.Code, started.Body)
	}
	id := job["artifact"].(string)
	waitForJob(t, a, job["job"].(string), 60*time.Second)
	var read struct {
		Result neighborhoodDocument `json:"result"`
	}
	if err := json.Unmarshal(request2(t, handler, "/api/neighborhoods/"+id), &read); err != nil {
		t.Fatal(err)
	}
	doc := read.Result
	if doc.Locus != "TRB" || doc.Source.Kind != "model" || !doc.Source.Weighted || doc.Source.Receptors < 1000 {
		t.Fatalf("%+v", doc)
	}
	if !doc.Options.CDR3Only || doc.Loops != nil || doc.Organism != "" {
		t.Errorf("a definition by V family compares the CDR3s alone: %+v", doc.Options)
	}
	if doc.Result.Receptors == 0 || doc.Result.Centers == 0 || len(doc.Rows) == 0 {
		t.Fatalf("%+v", doc.Result)
	}
	for _, r := range doc.Rows {
		if r.ID == "" || r.Chain.Locus != "TRB" || r.Neighbors < doc.Options.MinNeighbors || len(r.Members) == 0 {
			t.Fatalf("%+v", r)
		}
		if r.Members[0].Junction != r.Chain.Junction || r.Members[0].Distance != 0 {
			t.Errorf("a neighborhood does not hold its centroid first: %+v", r.Members[0])
		}
		if r.Radius < 0 || r.Radius > doc.Options.MaxRadius || !(r.RelativeRate > 0) {
			t.Errorf("%+v", r)
		}
	}
	// The centroids are the most enriched first.
	for i := 1; i < len(doc.Rows); i++ {
		if doc.Rows[i-1].ChiSquare < doc.Rows[i].ChiSquare {
			t.Fatalf("centroid %d is less enriched than the one after it", i-1)
		}
	}
	if rec := request(t, handler, http.MethodGet, "/api/export?what=result&artifact="+id, nil, nil); rec.Code != http.StatusOK {
		t.Errorf("export: %d %s", rec.Code, rec.Body)
	}

	// A shuffled background needs no model, and weighs each receptor 1.
	started = postJSON(t, handler, "/api/neighborhoods", `{`+byFamily+`, "neighborhoods": {"background": "shuffled"}}`)
	json.Unmarshal(started.Body.Bytes(), &job)
	if started.Code != http.StatusOK {
		t.Fatalf("shuffled: %d %s", started.Code, started.Body)
	}
	waitForJob(t, a, job["job"].(string), 60*time.Second)
	var again struct {
		Result neighborhoodDocument `json:"result"`
	}
	json.Unmarshal(request2(t, handler, "/api/neighborhoods/"+job["artifact"].(string)), &again)
	if again.Result.Source.Kind != "shuffled" || again.Result.Source.Weighted || again.Result.Source.Receptors != again.Result.Result.Receptors {
		t.Fatalf("shuffled: %+v", again.Result.Source)
	}

	// Options that make no sense are refused.
	for _, body := range []string{
		`{"background": "nothing"}`,
		`{"size": 10}`,
		`{"control": 2}`,
		`{"maxRadius": -1}`,
		`{"minNeighbors": -1}`,
		`{"locus": "TRX"}`,
	} {
		if rec := postJSON(t, handler, "/api/neighborhoods", `{`+byFamily+`, "neighborhoods": `+body+`}`); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: %d %s", body, rec.Code, rec.Body)
		}
	}
}
