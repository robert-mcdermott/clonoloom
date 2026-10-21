package main

import (
	"crypto/sha256"
	"encoding/hex"
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
	// The metadata are part of the dataset, by their fingerprint: the tables alone are another.
	if len(s.Metadata) != 1 || s.Metadata[0].Repertoires != 1 || len(s.Metadata[0].SHA256) != 64 {
		t.Fatalf("metadata %+v", s.Metadata)
	}
	alone := postJSON(t, handler, "/api/datasets/open", `{"paths": [`+jsonString(path)+`]}`)
	var s0 datasetSummary
	json.Unmarshal(alone.Body.Bytes(), &s0)
	if s0.ID == s.ID || len(s0.Metadata) != 0 || s0.Samples[0].Repertoire != nil {
		t.Fatalf("the tables alone: %s %+v", s0.ID, s0.Metadata)
	}
	// The same files open as the same dataset; the dataset is described by its ID.
	again := postJSON(t, handler, "/api/datasets/open", `{"paths": [`+jsonString(path)+`, `+jsonString(meta)+`], "name": "Renamed"}`)
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

func TestClonotypePagesAndOneClonotype(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "study.tsv")
	os.WriteFile(path, []byte(datasetTSV), 0o644)
	_, handler := testApp(t)
	var s datasetSummary
	json.Unmarshal(postJSON(t, handler, "/api/datasets/open", `{"paths": [`+jsonString(path)+`]}`).Body.Bytes(), &s)
	rec := postJSON(t, handler, "/api/clonotypes", `{"dataset": "`+s.ID+`", "definition": {"junction": "aa", "v": "gene", "j": "gene"}, "sort": "total"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	var page clonotypePage
	json.Unmarshal(rec.Body.Bytes(), &page)
	// CASSLAGGPEAFF in R1 (3) and R2 (5); CASSPGQGAYEQYF is not productive.
	if page.Count != 1 || page.Matched != 1 || page.Rows[0].Total != 8 || page.Rows[0].In["0"] != 3 || page.Rows[0].In["1"] != 5 || page.Excluded["not productive"] != 1 || page.Unit != "umis" {
		t.Fatalf("%+v", page)
	}
	if page.Rows[0].Frequency != 1 || !strings.Contains(page.Describe, "amino-acid junction") || len(page.Samples) != 2 {
		t.Fatalf("%+v", page)
	}
	none := postJSON(t, handler, "/api/clonotypes", `{"dataset": "`+s.ID+`", "definition": {}, "query": {"text": "CASSQ"}}`)
	var empty clonotypePage
	json.Unmarshal(none.Body.Bytes(), &empty)
	if empty.Matched != 0 || empty.Count != 1 {
		t.Fatalf("%+v", empty)
	}
	one := postJSON(t, handler, "/api/clonotypes/one", `{"dataset": "`+s.ID+`", "definition": {}, "id": "`+page.Rows[0].ID+`"}`)
	if one.Code != http.StatusOK || !strings.Contains(one.Body.String(), `"memberCount":2`) || !strings.Contains(one.Body.String(), `"sequenceId":"a"`) {
		t.Fatalf("%d %s", one.Code, one.Body)
	}
	if bad := postJSON(t, handler, "/api/clonotypes", `{"dataset": "`+s.ID+`", "definition": {"chains": ["TRA", "TRB"]}}`); bad.Code != http.StatusUnprocessableEntity || !strings.Contains(bad.Body.String(), "single-cell") {
		t.Fatalf("%d %s", bad.Code, bad.Body)
	}
	if bad := postJSON(t, handler, "/api/clonotypes", `{"dataset": "`+s.ID+`", "definition": {}, "sort": "colour"}`); bad.Code != http.StatusBadRequest {
		t.Fatalf("%d", bad.Code)
	}
}

// Files the page added to the library (dropped on the window) are known by their names: the
// metadata among them are read as metadata, not as a table.
func TestLibraryFilesOpenByTheirNames(t *testing.T) {
	_, handler := testApp(t)
	put := func(content string) string {
		sum := sha256.Sum256([]byte(content))
		sha := hex.EncodeToString(sum[:])
		if rec := request(t, handler, http.MethodPut, "/api/library/files/"+sha, strings.NewReader(content), nil); rec.Code != http.StatusOK {
			t.Fatalf("%d %s", rec.Code, rec.Body)
		}
		return sha
	}
	table := put(datasetTSV)
	meta := put(`[{"repertoire_id": "R2", "subject": {"subject_id": "S02"}}]`)
	rec := postJSON(t, handler, "/api/datasets/open", `{"library": [{"sha256": "`+table+`", "name": "study.tsv"}, {"sha256": "`+meta+`", "name": "study.airr.json"}]}`)
	var s datasetSummary
	json.Unmarshal(rec.Body.Bytes(), &s)
	if rec.Code != http.StatusOK || len(s.Metadata) != 1 || s.Metadata[0].Name != "study.airr.json" || s.Samples[1].Repertoire == nil || s.Samples[1].Repertoire.SubjectID != "S02" {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}

// A dataset opened again is read from the table cache; the cache is listed and cleared.
func TestDatasetsReopenFromTheCache(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "study.tsv")
	os.WriteFile(path, []byte(datasetTSV), 0o644)
	a, handler := testApp(t)
	first := postJSON(t, handler, "/api/datasets/open", `{"paths": [`+jsonString(path)+`]}`)
	var s1 datasetSummary
	json.Unmarshal(first.Body.Bytes(), &s1)
	usage := func() map[string]any {
		var u map[string]any
		json.Unmarshal(request(t, handler, http.MethodGet, "/api/cache", nil, nil).Body.Bytes(), &u)
		return u
	}
	if u := usage(); u["enabled"] != true || u["tables"] != 1.0 {
		t.Fatalf("after opening: %v", u)
	}
	// Closed and opened again (as after a restart): the same dataset, from the cache.
	a.datasets.remove(s1.ID)
	second := postJSON(t, handler, "/api/datasets/open", `{"paths": [`+jsonString(path)+`]}`)
	var s2 datasetSummary
	json.Unmarshal(second.Body.Bytes(), &s2)
	if s2.ID != s1.ID || s2.Rows != s1.Rows || s2.Sources[0].Path != path || usage()["tables"] != 1.0 {
		t.Fatalf("reopened: %s %d %v", s2.ID, s2.Rows, usage())
	}
	del := httptest.NewRequest(http.MethodDelete, "/api/cache", nil)
	del.Host, del.RemoteAddr = "127.0.0.1:8860", "127.0.0.1:5555"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, del)
	if rec.Code != http.StatusOK || usage()["tables"] != 0.0 {
		t.Fatalf("clear: %d %v", rec.Code, usage())
	}
}

// Jobs over HTTP: listed, read, and refused when there is no such job.
func TestTheJobsAPI(t *testing.T) {
	_, handler := testApp(t)
	if rec := request(t, handler, http.MethodGet, "/api/jobs", nil, nil); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"jobs":[]`) {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	for _, path := range []string{"/api/jobs/nothing"} {
		if rec := request(t, handler, http.MethodGet, path, nil, nil); rec.Code != http.StatusNotFound {
			t.Errorf("%s: %d", path, rec.Code)
		}
	}
	if rec := postJSON(t, handler, "/api/jobs/nothing/cancel", `{}`); rec.Code != http.StatusNotFound {
		t.Errorf("cancel: %d", rec.Code)
	}
	if rec := request(t, handler, http.MethodGet, "/api/artifacts", nil, nil); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"artifacts":[]`) {
		t.Errorf("artifacts: %d %s", rec.Code, rec.Body)
	}
}
