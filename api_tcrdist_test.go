package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"clonoloom/internal/tcrdist"
)

const tcrdistTSV = "sequence_id\tv_call\tj_call\tjunction_aa\tproductive\tumi_count\trepertoire_id\n" +
	"a\tTRBV5-1*01\tTRBJ2-7*01\tCASSLAGGPEAFF\tT\t3\tR1\n" +
	"b\tTRBV5-1*01\tTRBJ2-7*01\tCASSLAGGPEQYF\tT\t2\tR1\n" +
	"c\tTRBV6-2*01\tTRBJ1-1*01\tCASSPGQGAYEQYF\tT\t5\tR2\n" +
	"d\tTRBV19*01\tTRBJ2-1*01\tCASRDRWNSYNEQFF\tT\t1\tR2\n" +
	"e\tIGHV3-23*01\tIGHJ4*02\tCAKDRGYW\tT\t4\tR2\n"

// TCRdist is an analysis like the others: without the V genes' loops it says so (the page offers
// them); comparing the CDR3s alone, each T-cell clonotype's neighbors and nearest are found and
// kept, a clonotype's nearest are answered on request, again and again, and the result exports.
func TestTCRdistNeighborsAndNearest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trb.tsv")
	os.WriteFile(path, []byte(tcrdistTSV), 0o644)
	a, handler := testApp(t)
	open := postJSON(t, handler, "/api/datasets/open", `{"paths": [`+jsonString(path)+`]}`)
	var sum datasetSummary
	json.Unmarshal(open.Body.Bytes(), &sum)
	base := `"dataset": "` + sum.ID + `", "definition": {"name": "AA", "junction": "aa", "v": "allele", "j": "gene"}`

	if rec := postJSON(t, handler, "/api/tcrdist", `{`+base+`}`); rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"no-loops"`) {
		t.Fatalf("without the loops: %d %s", rec.Code, rec.Body)
	}
	if rec := postJSON(t, handler, "/api/tcrdist", `{"dataset": "`+sum.ID+`", "definition": {"junction": "nt"}, "tcrdist": {"cdr3Only": true}}`); rec.Code != http.StatusBadRequest {
		t.Errorf("nucleotide junctions: %d %s", rec.Code, rec.Body)
	}
	body := base + `, "tcrdist": {"cdr3Only": true, "radius": 30}`
	started := postJSON(t, handler, "/api/tcrdist", `{`+body+`}`)
	var job map[string]any
	json.Unmarshal(started.Body.Bytes(), &job)
	if started.Code != http.StatusOK || job["job"] == nil || !strings.HasPrefix(job["artifact"].(string), "tcd-") {
		t.Fatalf("started: %d %s", started.Code, started.Body)
	}
	id := job["artifact"].(string)
	waitForJob(t, a, job["job"].(string), 10*time.Second)
	var read struct {
		Result tcrdistDocument `json:"result"`
	}
	if err := json.Unmarshal(request2(t, handler, "/api/tcrdist/"+id), &read); err != nil {
		t.Fatal(err)
	}
	doc := read.Result
	if doc.Compared != 4 || doc.Radius != 30 || !doc.CDR3Only || doc.Loops != nil || doc.Excluded[tcrdist.ReasonNotTCell] != 1 || len(doc.Samples) != 2 {
		t.Fatalf("%+v", doc)
	}
	byJunction := map[string]tcrdistRow{}
	for _, r := range doc.Clonotype {
		byJunction[r.Chains[0].Junction] = r
	}
	x, y := byJunction["CASSLAGGPEAFF"], byJunction["CASSLAGGPEQYF"]
	want := tcrdist.CDR3Distance(tcrdist.NewChain("CASSLAGGPEAFF", nil), tcrdist.NewChain("CASSLAGGPEQYF", nil))
	if x.Nearest != y.ID || y.Nearest != x.ID || x.Distance != want || want > 30 || x.Neighbors < 1 {
		t.Errorf("the two alike: %+v %+v (distance %d)", x, y, want)
	}
	// Asked twice (the second from the receptors kept): the same nearest, nearest first.
	for range 2 {
		rec := postJSON(t, handler, "/api/tcrdist/nearest", `{`+body+`, "clonotype": "`+x.ID+`", "n": 3}`)
		var near struct {
			Nearest []struct {
				ID       string   `json:"id"`
				Distance int      `json:"distance"`
				Regions  [][4]int `json:"regions"`
			} `json:"nearest"`
		}
		json.Unmarshal(rec.Body.Bytes(), &near)
		if rec.Code != http.StatusOK || len(near.Nearest) != 3 || near.Nearest[0].ID != y.ID || near.Nearest[0].Distance != want || near.Nearest[0].Regions[0][3] != want || near.Nearest[1].Distance < want {
			t.Fatalf("nearest: %d %s", rec.Code, rec.Body)
		}
	}
	if rec := postJSON(t, handler, "/api/tcrdist/nearest", `{`+body+`, "clonotype": "nothing"}`); rec.Code != http.StatusNotFound {
		t.Errorf("an unknown clonotype: %d", rec.Code)
	}
	if rec := request(t, handler, http.MethodGet, "/api/export?what=result&artifact="+id, nil, nil); rec.Code != http.StatusOK {
		t.Errorf("export: %d %s", rec.Code, rec.Body)
	}

	// With the loops (the database fetched for the validation, when it is), their checksum in the
	// result and the organism guessed.
	loops := filepath.Join("validation", "cache", "tcrdist3-loops", "tcrdist", "db", "combo_xcr_2024-03-05.tsv")
	raw, err := os.ReadFile(loops)
	if err != nil {
		t.Skip("tcrdist3-loops not fetched (node validation/fetch.mjs tcrdist3-loops)")
	}
	target := filepath.Join(a.references.Dir, "tcrdist3-loops", "tcrdist", "db", "combo_xcr_2024-03-05.tsv")
	os.MkdirAll(filepath.Dir(target), 0o755)
	os.WriteFile(target, raw, 0o644)
	started = postJSON(t, handler, "/api/tcrdist", `{`+base+`}`)
	json.Unmarshal(started.Body.Bytes(), &job)
	if started.Code != http.StatusOK {
		t.Fatalf("with loops: %d %s", started.Code, started.Body)
	}
	waitForJob(t, a, job["job"].(string), 10*time.Second)
	json.Unmarshal(request2(t, handler, "/api/tcrdist/"+job["artifact"].(string)), &read)
	doc = read.Result
	if doc.CDR3Only || doc.Loops == nil || doc.Loops.SHA256[:12] != "1d7fcac37ad6" || doc.Organism != "human" || doc.Radius != 24 || doc.Compared != 4 {
		t.Fatalf("with loops: %+v", doc)
	}
}
