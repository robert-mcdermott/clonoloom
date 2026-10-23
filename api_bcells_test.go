package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBCellInputs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "one.tsv")
	other := filepath.Join(dir, "two.tsv")
	for _, p := range []string{path, other} {
		if err := os.WriteFile(p, bcellExampleData, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	_, h := testApp(t)
	var ds datasetSummary
	rec := postJSON(t, h, "/api/datasets/open", `{"paths":[`+jsonString(path)+`,`+jsonString(other)+`]}`)
	if rec.Code != 200 {
		t.Fatal(rec.Body)
	}
	json.Unmarshal(rec.Body.Bytes(), &ds)
	fetch := func(extra string) bcellPage {
		t.Helper()
		rec := postJSON(t, h, "/api/bcells/inputs", `{"dataset":"`+ds.ID+`"`+extra+`}`)
		if rec.Code != 200 {
			t.Fatal(rec.Code, rec.Body)
		}
		var page bcellPage
		if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		return page
	}
	page := fetch(`,"limit":2`)
	if page.Count != 14 || len(page.Rows) != 2 || page.Rows[0].Fields != nil || page.Rows[0].Evidence.Valid != true || page.Rows[1].Evidence.Paired {
		t.Fatalf("%+v", page)
	}
	all := fetch(`,"limit":200`)
	ids := map[string]bool{}
	for _, r := range all.Rows {
		if ids[r.ID] {
			t.Fatal("duplicate identity", r.ID)
		}
		ids[r.ID] = true
	}
	// Duplicate supplied IDs across sources retain separate source identities.
	if all.Rows[0].SequenceID != all.Rows[7].SequenceID || all.Rows[0].ID == all.Rows[7].ID {
		t.Fatal(all.Rows)
	}
	detail := fetch(`,"id":"` + page.Rows[0].ID + `"`)
	if detail.Rows[0].Fields["np1_length"] != "0" || detail.Rows[0].Fields["sequence"] != "ACGTACGTACGT" {
		t.Fatal(detail)
	}
	next := fetch(`,"offset":2,"limit":2`)
	if next.Rows[0].Locus != "IGK" || next.Rows[1].Evidence.Valid {
		t.Fatal(next)
	}
	filtered := fetch(`,"filter":{"loci":["IGK"]}`)
	if filtered.Count != 2 || len(filtered.Samples) != 1 {
		t.Fatal(filtered)
	}
	empty := fetch(`,"filter":{"loci":["TRB"]}`)
	if empty.Count != 0 || len(empty.Rows) != 0 {
		t.Fatal(empty)
	}
	for _, extra := range []string{`,"offset":-1`, `,"limit":201`, `,"definition":{"junction":"bad"}`} {
		r := postJSON(t, h, "/api/bcells/inputs", `{"dataset":"`+ds.ID+`"`+extra+`}`)
		if r.Code != 400 {
			t.Fatal(r.Code, r.Body)
		}
	}
	missing := postJSON(t, h, "/api/bcells/inputs", `{"dataset":"`+ds.ID+`","id":"absent"}`)
	if missing.Code != 404 {
		t.Fatal(missing.Code)
	}
	removed := postJSON(t, h, "/api/bcells/inputs", `{"dataset":"`+ds.ID+`","filter":{"loci":["TRB"]},"id":"`+page.Rows[0].ID+`"}`)
	if removed.Code != 404 {
		t.Fatal(removed.Code)
	}
	if !strings.Contains(rec.Body.String(), "evidence-alignment") {
		t.Fatal("findings missing", rec.Body)
	}
}

func TestBCellInputsMixedAndJunctionOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mixed.tsv")
	data := "sequence_id\tlocus\tv_call\tj_call\tjunction_aa\tproductive\n" +
		"same\tTRB\tTRBV5-1\tTRBJ1-1\tCASSF\tT\n" +
		"same\tIGH\tIGHV1-2\tIGHJ4\tCARF\tT\n"
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	_, h := testApp(t)
	var ds datasetSummary
	rec := postJSON(t, h, "/api/datasets/open", `{"paths":[`+jsonString(path)+`]}`)
	json.Unmarshal(rec.Body.Bytes(), &ds)
	rec = postJSON(t, h, "/api/bcells/inputs", `{"dataset":"`+ds.ID+`"}`)
	var page bcellPage
	json.Unmarshal(rec.Body.Bytes(), &page)
	if rec.Code != 200 || page.Count != 1 || page.Rows[0].Locus != "IGH" || page.Rows[0].Evidence.Sequence || page.Rows[0].Evidence.Valid || len(page.Rows[0].Evidence.Issues) != 0 {
		t.Fatal(rec.Code, rec.Body)
	}
}
