package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeReportCapturedEvidenceAndPages(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sim.tsv")
	raw, err := os.ReadFile("testdata/native-reports/simulated-trust4.tsv")
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(path, raw, 0600)
	a, h := testApp(t)
	opened := postJSON(t, h, "/api/datasets/open", `{"paths":[`+jsonString(path)+`]}`)
	if opened.Code != 200 {
		t.Fatal(opened.Body)
	}
	var summary datasetSummary
	json.Unmarshal(opened.Body.Bytes(), &summary)
	if summary.Unit != "" || !summary.Sources[0].NativeEvidence || summary.SingleCell {
		t.Fatal("invented semantics", summary)
	}
	os.Remove(path)
	endpoint := "/api/datasets/" + summary.ID + "/native-report?source=0"
	page := request(t, h, "GET", endpoint+"&offset=1&limit=1", nil, nil)
	if page.Code != 200 || !strings.Contains(page.Body.String(), `"total":7`) || !strings.Contains(page.Body.String(), `"line":3`) {
		t.Fatal(page.Body)
	}
	row := request(t, h, "GET", endpoint+"&record=0", nil, nil)
	var detail struct {
		HeaderText, RowText string
		Values              []string
	}
	json.Unmarshal(row.Body.Bytes(), &detail)
	lines := strings.Split(string(raw), "\n")
	if row.Code != 200 || detail.HeaderText != lines[0] || detail.RowText != lines[1] || detail.Values[1] != "0.624512345678901234" {
		t.Fatal(row.Body)
	}
	for _, query := range []string{"&record=-1", "&record=7", "&record=0&record=1", "&limit=51", "&offset=8", "&source=0"} {
		if r := request(t, h, "GET", endpoint+query, nil, nil); r.Code != 400 {
			t.Fatal(query, r.Code)
		}
	}
	ds, _ := a.datasets.get(summary.ID)
	ds.Result.Sources[0].Native.Rows[0].Text = strings.Repeat("x", 4<<20)
	if r := request(t, h, "GET", endpoint+"&record=0", nil, nil); r.Code != 422 {
		t.Fatal("preview bound", r.Code)
	}
}

func TestCellRangerJSONInspection(t *testing.T) {
	data, err := os.ReadFile("testdata/cellranger-json/simulated_all_contig_annotations.json")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "renamed.json")
	os.WriteFile(path, data, 0600)
	_, h := testApp(t)
	opened := postJSON(t, h, "/api/datasets/open", `{"paths":[`+jsonString(path)+`]}`)
	if opened.Code != 200 {
		t.Fatal(opened.Body)
	}
	var summary datasetSummary
	json.Unmarshal(opened.Body.Bytes(), &summary)
	if summary.Unit != "cells" || !summary.SingleCell || !summary.Sources[0].NativeEvidence {
		t.Fatal("single-cell semantics", summary)
	}
	if strings.Contains(opened.Body.String(), "9007199254740993") {
		t.Fatal("raw evidence leaked into summary")
	}
	os.Remove(path)
	endpoint := "/api/datasets/" + summary.ID + "/native-report?source=0"
	page := request(t, h, "GET", endpoint+"&offset=4&limit=2", nil, nil)
	if page.Code != 200 || !strings.Contains(page.Body.String(), `"layout":"cellranger-nested-v1"`) || !strings.Contains(page.Body.String(), `"total":8`) {
		t.Fatal(page.Body)
	}
	row := request(t, h, "GET", endpoint+"&record=0", nil, nil)
	var detail struct {
		RawJSON string
		Fields  []struct{ Name, Value string }
	}
	json.Unmarshal(row.Body.Bytes(), &detail)
	var raw []json.RawMessage
	json.Unmarshal(data, &raw)
	if row.Code != 200 || detail.RawJSON != string(raw[0]) || !strings.Contains(detail.RawJSON, "0.1234567890123456789012345") {
		t.Fatal("changed evidence", row.Body)
	}
	for _, f := range detail.Fields {
		if f.Name == "read_count" && f.Value != "2418" {
			t.Fatal("count text", f)
		}
	}
	row = request(t, h, "GET", endpoint+"&record=6", nil, nil)
	if row.Code != 200 || !strings.Contains(row.Body.String(), `"value":"null"`) || !strings.Contains(row.Body.String(), `"value":"0"`) || !strings.Contains(row.Body.String(), `"value":"false"`) {
		t.Fatal(row.Body)
	}
}
