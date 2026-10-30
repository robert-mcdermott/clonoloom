package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAIRRObjectsCapturedAndPaged(t *testing.T) {
	dir := t.TempDir()
	meta := filepath.Join(dir, "objects.json")
	raw, _ := os.ReadFile("testdata/airr-objects/study.json")
	os.WriteFile(meta, raw, 0600)
	_, h := testApp(t)
	opened := postJSON(t, h, "/api/datasets/open", `{"paths":["testdata/airr-objects/study.tsv",`+jsonString(meta)+`]}`)
	if opened.Code != 200 {
		t.Fatal(opened.Body)
	}
	var s datasetSummary
	json.Unmarshal(opened.Body.Bytes(), &s)
	endpoint := "/api/datasets/" + s.ID + "/airr-objects"
	os.Remove(meta)
	page := request(t, h, "GET", endpoint+"?limit=1&offset=2", nil, nil)
	if page.Code != 200 || !strings.Contains(page.Body.String(), `"total":7`) || !strings.Contains(page.Body.String(), `source-family-1`) {
		t.Fatal(page.Body)
	}
	detail := request(t, h, "GET", endpoint+"?record=2", nil, nil)
	var d struct {
		RawJSON    string `json:"rawJSON"`
		SourceText string `json:"sourceText"`
	}
	json.Unmarshal(detail.Body.Bytes(), &d)
	if detail.Code != 200 || !strings.Contains(d.RawJSON, "9007199254740993") || !strings.Contains(d.RawJSON, "1.234567890123456789") || d.SourceText != string(raw) {
		t.Fatal(detail.Body)
	}
	for _, q := range []string{"record=-1", "record=7", "record=0&record=1", "offset=8", "limit=0", "limit=51", "limit=x"} {
		if r := request(t, h, "GET", endpoint+"?"+q, nil, nil); r.Code != 400 {
			t.Fatalf("%s: %d", q, r.Code)
		}
	}
}
