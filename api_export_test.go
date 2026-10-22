package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// unzip answers a zip's files by name.
func unzip(t *testing.T, raw []byte) map[string]string {
	t.Helper()
	z, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatalf("not a zip: %v (%q)", err, raw[:min(len(raw), 80)])
	}
	out := map[string]string{}
	for _, f := range z.File {
		r, _ := f.Open()
		b, _ := io.ReadAll(r)
		out[f.Name] = string(b)
	}
	return out
}

// A dataset's clonotypes and rearrangements, and a kept result, download as zips with a NOTICE;
// rows of data under a provider's terms only once the request says it has read them.
func TestExports(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "study.tsv")
	os.WriteFile(path, []byte(datasetTSV), 0o644)
	a, handler := testApp(t)
	open := postJSON(t, handler, "/api/datasets/open", `{"paths": [`+jsonString(path)+`]}`)
	var s datasetSummary
	json.Unmarshal(open.Body.Bytes(), &s)
	query := "dataset=" + s.ID + "&definition=" + url.QueryEscape(`{"name": "AA", "junction": "aa", "v": "gene", "j": "gene"}`) + "&filter=" + url.QueryEscape(`{"productiveOnly": true}`)
	rec := request(t, handler, http.MethodGet, "/api/export?what=clonotypes&"+query, nil, nil)
	if rec.Code != http.StatusOK || !strings.HasSuffix(rec.Header().Get("Content-Disposition"), `-clonotypes.zip"`) {
		t.Fatalf("%d %s %s", rec.Code, rec.Header(), rec.Body)
	}
	files := unzip(t, rec.Body.Bytes())
	if !strings.HasPrefix(files["clonotypes.csv"], "clonotype_id,chain1_locus") || !strings.Contains(files["NOTICE.txt"], "Filter: productive only.") || !strings.Contains(files["NOTICE.txt"], "the data are yours") {
		t.Errorf("%v", files)
	}
	rec = request(t, handler, http.MethodGet, "/api/export?what=rearrangements&"+query, nil, nil)
	files = unzip(t, rec.Body.Bytes())
	// The filter keeps the productive rows: two of three.
	if n := strings.Count(files["rearrangements.tsv"], "\n"); n != 3 || !strings.Contains(files["rearrangements.tsv"], "\tclone_id\t") {
		t.Errorf("%d lines: %s", n, files["rearrangements.tsv"])
	}
	// A kept result.
	got := runAnalysisOn(t, a, handler, s.ID, "diversity")
	rec = request(t, handler, http.MethodGet, "/api/export?what=result&artifact="+got, nil, nil)
	files = unzip(t, rec.Body.Bytes())
	if !strings.HasPrefix(files["diversity.csv"], "sample,abundance,clonotypes,measure,estimate,se,lower,upper") || !strings.Contains(files["NOTICE.txt"], "kept as "+got) {
		t.Errorf("%v", files)
	}
	// Data under terms: refused until accepted, then said not to be shared.
	ds, _ := a.datasets.get(s.ID)
	ds.Credits = []credit{{Title: "Emerson et al. 2017", Source: "immuneACCESS", License: "immuneACCESS Terms of Use", LicenseClass: "terms", Citation: "Emerson RO, et al."}}
	rec = request(t, handler, http.MethodGet, "/api/export?what=clonotypes&"+query, nil, nil)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), `"code":"restricted"`) || !strings.Contains(rec.Body.String(), "immuneACCESS Terms of Use") {
		t.Errorf("not accepted: %d %s", rec.Code, rec.Body)
	}
	rec = request(t, handler, http.MethodGet, "/api/export?what=clonotypes&accept=1&"+query, nil, nil)
	if files := unzip(t, rec.Body.Bytes()); !strings.Contains(files["NOTICE.txt"], "THESE DATA MAY NOT BE SHARED") {
		t.Errorf("%s", files["NOTICE.txt"])
	}
	rec = request(t, handler, http.MethodGet, "/api/export?what=result&accept=1&artifact="+got, nil, nil)
	if files := unzip(t, rec.Body.Bytes()); !strings.Contains(files["NOTICE.txt"], "The data themselves may not be shared") {
		t.Errorf("%s", files["NOTICE.txt"])
	}
	for _, bad := range []string{"/api/export?what=nothing", "/api/export?what=result&artifact=div-none", "/api/export?what=clonotypes&dataset=d-none&definition={}"} {
		if rec := request(t, handler, http.MethodGet, bad, nil, nil); rec.Code == http.StatusOK {
			t.Errorf("%s: %d", bad, rec.Code)
		}
	}
}
