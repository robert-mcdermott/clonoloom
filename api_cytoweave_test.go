package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func cytoFixture(t *testing.T) (*app, http.Handler, cytoRequest) {
	t.Helper()
	a, h := testApp(t)
	opened := postJSON(t, h, "/api/datasets/open", `{"paths":["testdata/cytoweave/receptors.tsv"]}`)
	if opened.Code != 200 {
		t.Fatal(opened.Body)
	}
	var ds datasetSummary
	json.Unmarshal(opened.Body.Bytes(), &ds)
	sample := 0
	req := cytoRequest{Dataset: ds.ID, Sample: &sample}
	for file, dst := range map[string]*string{"wells.csv": &req.Wells, "mapping.csv": &req.Mapping, "provenance.json": &req.Provenance} {
		b, e := os.ReadFile("testdata/cytoweave/" + file)
		if e != nil {
			t.Fatal(e)
		}
		*dst = string(b)
	}
	return a, h, req
}
func TestCytoWeaveExactScope(t *testing.T) {
	a, h, req := cytoFixture(t)
	review := postJSON(t, h, "/api/cytoweave/review", string(mustJSON(req)))
	if review.Code != 200 {
		t.Fatal(review.Code, review.Body)
	}
	var got struct {
		Fingerprint string       `json:"fingerprint"`
		Evidence    cytoEvidence `json:"evidence"`
	}
	json.Unmarshal(review.Body.Bytes(), &got)
	d := got.Evidence
	r := d.Result
	if r.Matched != 80 || r.AbsentTargets != 10 || r.Unmapped != 6 || r.ReceptorCells != 84 || len(r.ReceptorOnly) != 4 || r.MissingBarcodeRows != 1 {
		t.Fatal(r)
	}
	if len(r.Observations[0].ReceptorRows) != 2 || r.Observations[3].Values[4] != "" || r.Observations[3].Values[7] != "" || r.Observations[6].Values[8] != "NaN" {
		t.Fatal("lost source evidence")
	}
	for _, v := range []cytoText{d.Wells, d.Mapping, d.Provenance} {
		if v.SHA256 != replayDigest([]byte(v.Text)) {
			t.Fatal("hash")
		}
	}
	req.Fingerprint = got.Fingerprint
	exported := postJSON(t, h, "/api/cytoweave/export", string(mustJSON(req)))
	if exported.Code != 200 || replayDigest(exported.Body.Bytes()) != got.Fingerprint {
		t.Fatal(exported.Body)
	}
	if dir := os.Getenv("CLONOLOOM_CYTOWEAVE_OUT"); dir != "" {
		os.MkdirAll(dir, 0700)
		if e := os.WriteFile(filepath.Join(dir, "links.json"), exported.Body.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
	}
	// Same barcodes elsewhere must never fill sample A's missing targets.
	other := 1
	req.Sample = &other
	changed := postJSON(t, h, "/api/cytoweave/export", string(mustJSON(req)))
	if changed.Code != 409 {
		t.Fatal(changed.Code, changed.Body)
	}
	changed = postJSON(t, h, "/api/cytoweave/review", string(mustJSON(req)))
	json.Unmarshal(changed.Body.Bytes(), &got)
	if changed.Code != 200 || got.Evidence.Result.Matched != 3 || got.Evidence.Result.AbsentTargets != 87 {
		t.Fatal(changed.Body)
	}
	req.Sample = derefInt(0)
	req.Fingerprint = got.Fingerprint
	ds, _ := a.datasets.get(req.Dataset)
	ds.Credits = []credit{{Title: "Artificial terms control", License: "Permission required", LicenseClass: "terms"}}
	review = postJSON(t, h, "/api/cytoweave/review", string(mustJSON(req)))
	json.Unmarshal(review.Body.Bytes(), &got)
	req.Fingerprint = got.Fingerprint
	denied := postJSON(t, h, "/api/cytoweave/export", string(mustJSON(req)))
	if denied.Code != 403 {
		t.Fatal(denied.Code, denied.Body)
	}
	req.AcceptTerms = true
	if w := postJSON(t, h, "/api/cytoweave/export", string(mustJSON(req))); w.Code != 200 {
		t.Fatal(w.Body)
	}
}
func derefInt(n int) *int { return &n }
func TestCytoWeaveRefusals(t *testing.T) {
	_, h, base := cytoFixture(t)
	cases := map[string]func(*cytoRequest){
		"missing sample": func(r *cytoRequest) { r.Sample = nil }, "negative sample": func(r *cytoRequest) { r.Sample = derefInt(-1) }, "unknown dataset": func(r *cytoRequest) { r.Dataset = "missing" },
		"duplicate well":      func(r *cytoRequest) { r.Wells += strings.Split(r.Wells, "\n")[1] + "\n" },
		"duplicate event":     func(r *cytoRequest) { r.Wells = strings.Replace(r.Wells, "A2,1,2,2,", "A2,1,2,1,", 1) },
		"coordinate conflict": func(r *cytoRequest) { r.Wells = strings.Replace(r.Wells, "A1,1,1,1,", "A1,1,2,1,", 1) },
		"duplicate target":    func(r *cytoRequest) { r.Mapping = strings.Replace(r.Mapping, "BC00002-1", "BC00001-1", 1) },
		"duplicate key":       func(r *cytoRequest) { r.Mapping += "A1,other\n" },
		"unknown well":        func(r *cytoRequest) { r.Mapping += "Z1,other\n" },
		"duplicate header":    func(r *cytoRequest) { r.Wells = strings.Replace(r.Wells, "CD4 (FITC-A)", "CD3 (APC-A)", 1) },
		"oversize":            func(r *cytoRequest) { r.Wells = strings.Repeat("x", (4<<20)+1) },
		"missing provenance":  func(r *cytoRequest) { r.Provenance = "{}" },
		"duplicate provenance": func(r *cytoRequest) {
			r.Provenance = strings.Replace(r.Provenance, `"sample":`, `"sample":"other","sample":`, 1)
		},
		"transformed": func(r *cytoRequest) { r.Provenance = strings.Replace(r.Provenance, `"linear"`, `"arcsinh"`, 1) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			req := base
			mutate(&req)
			w := postJSON(t, h, "/api/cytoweave/review", string(mustJSON(req)))
			if w.Code < 400 {
				t.Fatal(w.Body)
			}
		})
	}
	// Case/suffix/whitespace are evidence, never normalization instructions.
	for _, target := range []string{"bc00001-1", "BC00001", " BC00001-1"} {
		req := base
		req.Mapping = strings.Replace(req.Mapping, "BC00001-1", target, 1)
		w := postJSON(t, h, "/api/cytoweave/review", string(mustJSON(req)))
		if w.Code != 200 || !bytes.Contains(w.Body.Bytes(), []byte(`"matched":79`)) {
			t.Fatal(w.Body)
		}
	}
}

func TestCytoWeaveExactTokensAndUnicode(t *testing.T) {
	_, h, req := cytoFixture(t)
	req.Wells = "\ufeffwell,row,column,event,population,exact,missing\r\nA1,1,1,9007199254740993,\"quoted\r\nlabel\",0.123456789012345678901,\r\n"
	req.Mapping = "well,cell_id\r\nA1,BC00001-1\r\n"
	w := postJSON(t, h, "/api/cytoweave/review", string(mustJSON(req)))
	if w.Code != 200 {
		t.Fatal(w.Body)
	}
	var got struct {
		Fingerprint string       `json:"fingerprint"`
		Evidence    cytoEvidence `json:"evidence"`
	}
	json.Unmarshal(w.Body.Bytes(), &got)
	if got.Evidence.Wells.Text != req.Wells || got.Evidence.Result.Observations[0].Values[3] != "9007199254740993" || got.Evidence.Result.Observations[0].Values[5] != "0.123456789012345678901" {
		t.Fatal("precision or original bytes changed")
	}
	req.Fingerprint = got.Fingerprint
	if w = postJSON(t, h, "/api/cytoweave/export", string(mustJSON(req))); w.Code != 200 {
		t.Fatal(w.Body)
	}
	req.Mapping = strings.Replace(req.Mapping, "BC00001-1", "BC00002-1", 1)
	if w = postJSON(t, h, "/api/cytoweave/export", string(mustJSON(req))); w.Code != 409 {
		t.Fatal("changed evidence exported", w.Code)
	}
	for _, raw := range []string{`{"dataset":"\ud800"}`, `{"dataset":"\udc00"}`, `{"dataset":"a","dataset":"b"}`, "{\"dataset\":\"\xff\"}"} {
		w = postJSON(t, h, "/api/cytoweave/review", raw)
		if w.Code != 400 {
			t.Fatal("malformed identity accepted", w.Code, w.Body)
		}
	}
	// Unknown source fields and unpaired escaped provenance text cannot silently collapse.
	req.Provenance = strings.Replace(req.Provenance, `"sample": "Simulated plate A"`, `"sample": "\ud800"`, 1)
	if w = postJSON(t, h, "/api/cytoweave/review", string(mustJSON(req))); w.Code != 400 {
		t.Fatal(w.Body)
	}
}

func TestCytoWeaveSourceIdentity(t *testing.T) {
	_, _, base := cytoFixture(t)
	original, e := os.ReadFile("testdata/cytoweave/receptors.tsv")
	if e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"changed", "trimmed cell", "trimmed repertoire", "gzip"} {
		t.Run(name, func(t *testing.T) {
			_, h := testApp(t)
			raw := original
			if name == "trimmed cell" {
				raw = bytes.Replace(raw, []byte("\tBC00001-1\t"), []byte("\t BC00001-1 \t"), 1)
			}
			if name == "trimmed repertoire" {
				raw = bytes.ReplaceAll(raw, []byte("Artificial donor A"), []byte(" Artificial donor A "))
			}
			path := filepath.Join(t.TempDir(), "source.tsv")
			if name == "gzip" {
				path += ".gz"
				var b bytes.Buffer
				g := gzip.NewWriter(&b)
				g.Write(raw)
				g.Close()
				raw = b.Bytes()
			}
			if e := os.WriteFile(path, raw, 0600); e != nil {
				t.Fatal(e)
			}
			opened := postJSON(t, h, "/api/datasets/open", string(mustJSON(map[string]any{"paths": []string{path}})))
			if opened.Code != 200 {
				t.Fatal(opened.Body)
			}
			var ds datasetSummary
			json.Unmarshal(opened.Body.Bytes(), &ds)
			if name == "changed" {
				if e := os.WriteFile(path, append(raw, '\n'), 0600); e != nil {
					t.Fatal(e)
				}
			}
			req := base
			req.Dataset = ds.ID
			w := postJSON(t, h, "/api/cytoweave/review", string(mustJSON(req)))
			if name == "gzip" {
				if w.Code != 200 {
					t.Fatal(w.Body)
				}
			} else if w.Code != 400 {
				t.Fatal("normalized/changed source accepted", w.Code, w.Body)
			}
		})
	}
}
