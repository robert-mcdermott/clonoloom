package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestADCPublicOverlapEvidence(t *testing.T) {
	a, h := testApp(t)
	a.adc.Offline = true // Captured comparisons must work without network permission.
	a.adc.Transport = adcTransport(func(*http.Request) (*http.Response, error) {
		t.Fatal("comparison contacted repository")
		return nil, nil
	})
	for _, name := range []string{"a", "b", "c", "d"} {
		name = "adc-download-sim-" + name
		if err := os.CopyFS(filepath.Join(a.store.dir, name), os.DirFS(filepath.Join("testdata/public-overlap", name))); err != nil {
			t.Fatal(err)
		}
	}
	opened := postJSON(t, h, "/api/datasets/open", `{"paths":["testdata/public-overlap/query.tsv"]}`)
	if opened.Code != 200 {
		t.Fatal(opened.Code, opened.Body)
	}
	var ds datasetSummary
	if err := json.Unmarshal(opened.Body.Bytes(), &ds); err != nil {
		t.Fatal(err)
	}
	catalog := request(t, h, "GET", "/api/adc/overlap/catalog?dataset="+ds.ID, nil, nil)
	if catalog.Code != 200 || !strings.Contains(catalog.Body.String(), "adc-download-sim-d") {
		t.Fatal(catalog.Code, catalog.Body)
	}
	body := `{"dataset":` + jsonString(ds.ID) + `,"sample":0,"locus":"TRB","captures":["adc-download-sim-a","adc-download-sim-b","adc-download-sim-c","adc-download-sim-d"],"accept":true}`
	for _, bad := range []string{strings.Replace(body, `"accept":true`, `"accept":false`, 1), strings.Replace(body, `"sample":0`, `"sample":-1`, 1), strings.Replace(body, `"TRB"`, `"mixed"`, 1), strings.ReplaceAll(body, "adc-download-sim-a", "../adc-download-sim-a"), strings.ReplaceAll(body, "adc-download-sim-b", "adc-download-sim-a"), strings.Replace(body, `"sample":0`, `"sample":null`, 1), strings.Replace(body, `"sample":0,`, "", 1)} {
		if w := postJSON(t, h, "/api/adc/overlap", bad); w.Code == 200 {
			t.Fatalf("accepted %s", bad)
		}
	}
	r := httptest.NewRequest("POST", "/api/adc/overlap", strings.NewReader(body))
	r.Host = "127.0.0.1:8860"
	r.RemoteAddr = "203.0.113.1:1234"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal(w.Code)
	}
	good := postJSON(t, h, "/api/adc/overlap", body)
	if good.Code != 200 {
		t.Fatal(good.Code, good.Body)
	}
	var result struct {
		Document json.RawMessage `json:"document"`
		Path     string          `json:"path"`
	}
	if err := json.Unmarshal(good.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(result.Path)
	if err != nil {
		t.Fatal(err)
	}
	var native, response any
	json.Unmarshal(saved, &native)
	json.Unmarshal(result.Document, &response)
	if !reflect.DeepEqual(native, response) || !strings.Contains(string(saved), `"queryKeysShared": 24`) {
		t.Fatal(string(saved))
	}
	if err := os.WriteFile(filepath.Join(a.store.dir, "adc-download-sim-a/page-001.json"), []byte(`{"Rearrangement":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if w := postJSON(t, h, "/api/adc/overlap", body); w.Code == 200 {
		t.Fatal("accepted changed capture")
	}
	files, _ := filepath.Glob(filepath.Join(a.store.dir, "adc-overlap-*.json"))
	if len(files) != 1 {
		t.Fatal("failed request installed result", files)
	}
}
