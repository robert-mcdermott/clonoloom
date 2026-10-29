package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"clonoloom/internal/airr"
	"clonoloom/internal/rep"
)

func TestMetadataMatchingNeverRepairsExplicitIDs(t *testing.T) {
	r := &airr.Repertoire{RepertoireID: "name"}
	records := map[string]*airr.Repertoire{"name": r}
	for _, tc := range []struct {
		sample rep.Sample
		want   *airr.Repertoire
		basis  string
	}{
		{rep.Sample{Name: "name"}, r, "sample name (legacy)"},
		{rep.Sample{Name: "name", RepertoireID: "other"}, nil, "repertoire_id"},
		{rep.Sample{Name: "name", RepertoireID: "NAME"}, nil, "repertoire_id"},
		{rep.Sample{Name: "other", RepertoireID: "name"}, r, "repertoire_id"},
	} {
		got, basis := repertoireMatch(tc.sample, records)
		if got != tc.want || basis != tc.basis {
			t.Fatalf("%+v: %v %s", tc.sample, got, basis)
		}
	}
}

func TestMetadataInspectionExactAndPaged(t *testing.T) {
	dir := t.TempDir()
	table, meta := filepath.Join(dir, "study.tsv"), filepath.Join(dir, "study.json")
	os.WriteFile(table, []byte(datasetTSV), 0600)
	raw := `{ "repertoire_id":"R1", "extra":{"integer":9007199254740993,"decimal":1.234567890123456789,"large":1e400,"null":null,"false":false,"zero":-0} }`
	os.WriteFile(meta, []byte(`{"Info":{"version":"2.0"},"Repertoire":[`+raw+`,{"repertoire_id":"unused"}]}`), 0600)
	app, handler := testApp(t)
	opened := postJSON(t, handler, "/api/datasets/open", `{"paths":[`+jsonString(table)+`,`+jsonString(meta)+`]}`)
	if opened.Code != 200 {
		t.Fatal(opened.Body)
	}
	var summary datasetSummary
	json.Unmarshal(opened.Body.Bytes(), &summary)
	endpoint := "/api/datasets/" + summary.ID + "/metadata"
	response := request(t, handler, "GET", endpoint+"?limit=1", nil, nil)
	var page metadataPage
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if response.Code != 200 || page.Total != 2 || page.MatchedRecords != 1 || page.UnmatchedRecords != 1 || page.SamplesWithMetadata != 1 || page.SamplesWithoutMetadata != 1 || len(page.Records) != 1 || page.Records[0].IDMatches != 1 || page.Files[0].SchemaVersion != "2.0" {
		t.Fatalf("wrong coverage: %s", response.Body)
	}
	second := request(t, handler, "GET", endpoint+"?offset=1&limit=1", nil, nil)
	json.Unmarshal(second.Body.Bytes(), &page)
	if len(page.Records) != 1 || page.Records[0].RepertoireID != "unused" || page.Records[0].MatchedSamples != 0 {
		t.Fatal(second.Body)
	}
	os.Remove(meta) // Inspection is the captured evidence, not a fresh filesystem read.
	detail := request(t, handler, "GET", endpoint+"?record=0", nil, nil)
	var record struct {
		RawJSON string `json:"rawJSON"`
	}
	json.Unmarshal(detail.Body.Bytes(), &record)
	if detail.Code != 200 || record.RawJSON != raw {
		t.Fatalf("numeric/source evidence changed: %s", detail.Body)
	}
	// Optional independent oracle compares native output without binary64 decoding.
	if output := os.Getenv("CLONOLOOM_METADATA_EVIDENCE"); output != "" {
		if err := os.WriteFile(output, detail.Body.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, query := range []string{"limit=0", "limit=51", "offset=-1", "offset=3", "offset=no", "offset=0&offset=1", "record=2", "record=-1", "record=0&record=1", "record=no"} {
		if got := request(t, handler, "GET", endpoint+"?"+query, nil, nil); got.Code != 400 {
			t.Fatalf("%s: %d", query, got.Code)
		}
	}
	ds, _ := app.datasets.get(summary.ID)
	ds.Result.Repertoires[0].Raw = []byte(strings.Repeat(" ", 4<<20+1))
	if got := request(t, handler, "GET", endpoint+"?record=0", nil, nil); got.Code != 422 {
		t.Fatal("oversize preview not refused")
	}
	if got := request(t, handler, "GET", "/api/datasets/missing/metadata", nil, nil); got.Code != 404 {
		t.Fatal("missing dataset")
	}
}

func TestYAMLMetadataInspectionDistinguishesSourceAndProjection(t *testing.T) {
	dir := t.TempDir()
	table, meta := filepath.Join(dir, "study.tsv"), filepath.Join(dir, "study.yaml")
	source := "# source comment\r\nrepertoire_id: R1\r\nvalue: 9007199254740993\r\nflag: false\r\n"
	os.WriteFile(table, []byte(datasetTSV), 0600)
	os.WriteFile(meta, []byte(source), 0600)
	app, handler := testApp(t)
	opened := postJSON(t, handler, "/api/datasets/open", `{"paths":[`+jsonString(table)+`,`+jsonString(meta)+`]}`)
	if opened.Code != 200 {
		t.Fatal(opened.Body)
	}
	var summary datasetSummary
	json.Unmarshal(opened.Body.Bytes(), &summary)
	endpoint := "/api/datasets/" + summary.ID + "/metadata?record=0"
	os.Remove(meta)
	response := request(t, handler, "GET", endpoint, nil, nil)
	var detail map[string]any
	json.Unmarshal(response.Body.Bytes(), &detail)
	if response.Code != 200 || detail["format"] != "yaml" || detail["sourceYAML"] != source || !strings.Contains(detail["rawJSON"].(string), "9007199254740993") {
		t.Fatal(response.Body)
	}
	ds, _ := app.datasets.get(summary.ID)
	ds.Result.Metadata[0].SourceText = strings.Repeat("#", 4<<20+1)
	response = request(t, handler, "GET", endpoint, nil, nil)
	detail = map[string]any{}
	json.Unmarshal(response.Body.Bytes(), &detail)
	if response.Code != 200 || detail["sourceYAML"] != nil || detail["sourceNotice"] == nil || detail["rawJSON"] == nil {
		t.Fatal("oversized source preview must not truncate or hide its record projection")
	}
}
