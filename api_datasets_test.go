package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func postJSON(t *testing.T, handler http.Handler, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
	req.Host = "127.0.0.1:8860"
	req.RemoteAddr = "127.0.0.1:5555"
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

const datasetTSV = "sequence_id\tv_call\tj_call\tjunction_aa\tproductive\tumi_count\trepertoire_id\n" +
	"a\tTRBV5-1*01\tTRBJ2-7*01\tCASSLAGGPEAFF\tT\t3\tR1\n" +
	"b\tTRBV6-2*01\tTRBJ1-1*01\tCASSPGQGAYEQYF\tF\t2\tR1\n" +
	"c\tTRBV5-1*01\tTRBJ2-7*01\tCASSLAGGPEAFF\tT\t5\tR2\n"

func TestOpenADatasetAndFindItAgain(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "study.tsv")
	meta := filepath.Join(dir, "study.airr.json")
	os.WriteFile(path, []byte(datasetTSV), 0o644)
	os.WriteFile(meta, []byte(`[{"repertoire_id": "R1", "subject": {"subject_id": "S01"}, "sample": [{"tissue": {"label": "blood"}}]}]`), 0o644)
	_, handler := testApp(t)
	rec := postJSON(t, handler, "/api/datasets/open", `{"paths": [`+jsonString(path)+`, `+jsonString(meta)+`]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	var s datasetSummary
	json.Unmarshal(rec.Body.Bytes(), &s)
	if s.Rows != 3 || len(s.Samples) != 2 || s.Unit != "umis" || s.Samples[0].Abundance != 5 || s.Samples[0].Productive != 1 || s.Samples[0].Loci["TRB"] != 2 {
		t.Fatalf("%+v", s)
	}
	if s.Samples[0].Repertoire == nil || s.Samples[0].Repertoire.SubjectID != "S01" || s.Samples[1].Repertoire != nil {
		t.Fatalf("metadata %+v", s.Samples)
	}
	if len(s.Sources) != 1 || len(s.Sources[0].SHA256) != 64 || s.Sources[0].Size != int64(len(datasetTSV)) || !strings.HasPrefix(s.ID, "d-") {
		t.Fatalf("%+v", s.Sources)
	}
	// The same files open as the same dataset; the dataset is described by its ID.
	again := postJSON(t, handler, "/api/datasets/open", `{"paths": [`+jsonString(path)+`], "name": "Renamed"}`)
	var s2 datasetSummary
	json.Unmarshal(again.Body.Bytes(), &s2)
	if s2.ID != s.ID || s2.Name != "Renamed" {
		t.Fatalf("%s %s %s", s2.ID, s.ID, s2.Name)
	}
	if got := request(t, handler, http.MethodGet, "/api/datasets/"+s.ID, nil, nil); got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"Renamed"`) {
		t.Fatalf("%d %s", got.Code, got.Body)
	}
	// A file that moved is found again by its fingerprint, under another name.
	moved := filepath.Join(dir, "elsewhere", "deeper", "renamed.tsv")
	os.MkdirAll(filepath.Dir(moved), 0o755)
	os.Rename(path, moved)
	body, _ := json.Marshal(map[string]any{"name": "study.tsv", "size": s.Sources[0].Size, "sha256": s.Sources[0].SHA256, "near": []string{dir}})
	found := postJSON(t, handler, "/api/datasets/locate", string(body))
	if found.Code != http.StatusOK || !strings.Contains(found.Body.String(), "renamed.tsv") {
		t.Fatalf("%d %s", found.Code, found.Body)
	}
	missing := postJSON(t, handler, "/api/datasets/open", `{"paths": [`+jsonString(path)+`]}`)
	if missing.Code != http.StatusNotFound || !strings.Contains(missing.Body.String(), `"missing"`) {
		t.Fatalf("%d %s", missing.Code, missing.Body)
	}
}

func TestOpenRefusalsSayWhy(t *testing.T) {
	dir := t.TempDir()
	clono := filepath.Join(dir, "clonotypes.csv")
	os.WriteFile(clono, []byte("clonotype_id,frequency,proportion,cdr3s_aa,cdr3s_nt\n"), 0o644)
	other := filepath.Join(dir, "other.tsv")
	os.WriteFile(other, []byte("a\tb\n1\t2\n"), 0o644)
	_, handler := testApp(t)
	for path, want := range map[string]string{clono: "clonotype summary", other: "not a format"} {
		rec := postJSON(t, handler, "/api/datasets/open", `{"paths": [`+jsonString(path)+`]}`)
		if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), want) {
			t.Errorf("%s: %d %s", filepath.Base(path), rec.Code, rec.Body)
		}
	}
	// A folder of tables opens as one dataset.
	os.Remove(clono)
	os.Remove(other)
	for _, name := range []string{"S1_day0.tsv", "S1_day28.tsv"} {
		os.WriteFile(filepath.Join(dir, name), []byte(datasetTSV), 0o644)
	}
	rec := postJSON(t, handler, "/api/datasets/open", `{"paths": [`+jsonString(dir)+`]}`)
	var s datasetSummary
	json.Unmarshal(rec.Body.Bytes(), &s)
	if rec.Code != http.StatusOK || len(s.Sources) != 2 || s.Name != filepath.Base(dir) {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	// Another computer may not name paths.
	req := httptest.NewRequest(http.MethodPost, "/api/datasets/open", strings.NewReader(`{"paths": ["/etc"]}`))
	req.Host, req.RemoteAddr = "127.0.0.1:8860", "10.1.2.3:5555"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("another computer: %d", w.Code)
	}
}

func TestQCOfADatasetUnderAFilter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "study.tsv")
	os.WriteFile(path, []byte(datasetTSV), 0o644)
	_, handler := testApp(t)
	var s datasetSummary
	json.Unmarshal(postJSON(t, handler, "/api/datasets/open", `{"paths": [`+jsonString(path)+`]}`).Body.Bytes(), &s)
	rec := postJSON(t, handler, "/api/qc", `{"dataset": "`+s.ID+`", "filter": {"productiveOnly": true}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	var answer qcAnswer
	if err := json.Unmarshal(rec.Body.Bytes(), &answer); err != nil {
		t.Fatal(err)
	}
	r1 := answer.Report.Samples[0]
	if r1.Name != "R1" || r1.Kept != 1 || r1.Removed["not productive"] != 1 || r1.Productive != 0.6 || !answer.Report.Filter.ProductiveOnly {
		t.Fatalf("%+v", r1)
	}
	if rec := postJSON(t, handler, "/api/qc", `{"dataset": "d-nothing"}`); rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "not-open") {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}
