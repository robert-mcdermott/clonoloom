package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"clonoloom/internal/cohort"
	"clonoloom/internal/prism"
	"clonoloom/internal/study"
)

func prismFixture(t *testing.T, paired bool) (*app, string, string) {
	t.Helper()
	a, h, body, _ := cohortFeatureFixture(t)
	if paired {
		body.Design.Input.Spec.Layout = "paired"
		for i := range body.Design.Input.Samples {
			body.Design.Input.Samples[i].Subject = []string{"00123", "=P<&02", "00123", "=P<&02"}[i]
		}
		d, e := study.CheckCohort(body.Design.Input)
		if e != nil || !d.OK {
			t.Fatal(d, e)
		}
		body.Design.ID = d.ID
		body.Design.Input = d.Input
	}
	w := postJSON(t, h, "/api/cohort-features", string(mustJSON(body)))
	var s struct{ Artifact, Job string }
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &s) != nil {
		t.Fatal(w.Code, w.Body)
	}
	waitForJob(t, a, s.Job, time.Minute)
	features := s.Artifact
	w = postJSON(t, h, "/api/cohort-compare", string(mustJSON(cohortCompareRequest{Features: features, Options: cohort.Options{Features: []string{"abundance", "shannon"}, Permutations: 999}})))
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &s) != nil {
		t.Fatal(w.Code, w.Body)
	}
	waitForJob(t, a, s.Job, time.Minute)
	return a, features, s.Artifact
}
func prismFiles(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	z, e := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if e != nil {
		t.Fatal(e)
	}
	out := map[string][]byte{}
	for _, f := range z.File {
		r, e := f.Open()
		if e != nil {
			t.Fatal(e)
		}
		out[f.Name], e = io.ReadAll(r)
		r.Close()
		if e != nil {
			t.Fatal(e)
		}
	}
	var record prismRecord
	if e = json.Unmarshal(out["prism.json"], &record); e != nil {
		t.Fatal(e)
	}
	for _, f := range record.Files {
		if int64(len(out[f.Path])) != f.Size || replayDigest(out[f.Path]) != f.SHA256 {
			t.Fatal("inventory", f.Path)
		}
	}
	return out
}
func TestPrismNativeExports(t *testing.T) {
	for _, paired := range []bool{false, true} {
		a, features, comparison := prismFixture(t, paired)
		h, e := a.handler()
		if e != nil {
			t.Fatal(e)
		}
		for _, id := range []string{features, comparison} {
			m, n, e := a.certificateNotice(id)
			if e != nil {
				t.Fatal(e)
			}
			data, _, e := a.makePrism(context.Background(), reportSelection{m, n}, "Artificial subject study")
			if e != nil {
				t.Fatal(e)
			}
			files := prismFiles(t, data)
			original, e := os.ReadFile(filepath.Join(a.artifacts.Dir(), id, m.Kind+".json"))
			if e != nil || !bytes.Equal(files["result.json"], original) {
				t.Fatal("changed original", e)
			}
			var mapping prism.Mapping
			json.Unmarshal(files["mapping.json"], &mapping)
			expected := 4
			if paired {
				expected = 2
			}
			if len(mapping.Tables[0].Rows) != expected {
				t.Fatal("subject count")
			}
			for _, table := range mapping.Tables {
				for _, r := range table.Rows {
					structural := 0
					for _, c := range r.Cells {
						if c.State == "structural-empty" {
							structural++
						}
					}
					if paired && structural != 0 || !paired && structural != 1 {
						t.Fatal("false pairing")
					}
				}
			}
			again, _, e := a.makePrism(context.Background(), reportSelection{m, n}, "Artificial subject study")
			if e != nil || !bytes.Equal(again, data) {
				t.Fatal("nondeterministic", e)
			}
			w := postJSON(t, h, "/api/reports/prism", string(mustJSON(map[string]any{"artifact": id, "title": "Artificial subject study", "accept": true})))
			if w.Code != 200 || w.Header().Get("X-ClonoLoom-Prism") != replayDigest(w.Body.Bytes()) {
				t.Fatal(w.Code, w.Body)
			}
			if dir := os.Getenv("CLONOLOOM_PRISM_OUT"); dir != "" {
				os.MkdirAll(dir, 0700)
				layout := "independent"
				if paired {
					layout = "paired"
				}
				if e := os.WriteFile(filepath.Join(dir, layout+"-"+m.Kind+".zip"), data, 0600); e != nil {
					t.Fatal(e)
				}
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if _, _, e := a.makePrism(ctx, reportSelection{m, n}, "Cancelled"); e == nil {
				t.Fatal("cancel")
			}
		}
	}
}
func TestPrismRefusals(t *testing.T) {
	a, features, _ := prismFixture(t, false)
	h, _ := a.handler()
	for _, body := range []string{`{}`, `{"artifact":"../escape","title":"x"}`, `{"artifact":"` + features + `","title":"x","extra":1}`, `{"artifact":"` + features + `","title":""}`} {
		if w := postJSON(t, h, "/api/reports/prism", body); w.Code == 200 {
			t.Fatal("accepted invalid request")
		}
	}
	m, n, e := a.certificateNotice(features)
	if e != nil {
		t.Fatal(e)
	}
	m.Kind = cohortCountsKind
	if _, _, e := a.makePrism(context.Background(), reportSelection{m, n}, "Unsupported"); e == nil {
		t.Fatal("non-subject result")
	}
	path := filepath.Join(a.artifacts.Dir(), features, "cohort-features.json")
	os.WriteFile(path, []byte(`{}`), 0600)
	if w := postJSON(t, h, "/api/reports/prism", `{"artifact":"`+features+`","title":"x"}`); w.Code == 200 {
		t.Fatal("changed evidence")
	}
}
func TestPrismMappingControls(t *testing.T) {
	a, _, id := prismFixture(t, true)
	raw, e := os.ReadFile(filepath.Join(a.artifacts.Dir(), id, "cohort-compare.json"))
	if e != nil {
		t.Fatal(e)
	}
	// Exact lexical numbers and XML-sensitive labels never pass through browser floats.
	changed := bytes.ReplaceAll(raw, []byte(`"00123"`), []byte(`"changed subject"`))
	if _, e := prism.Map(context.Background(), changed); e == nil {
		t.Fatal("changed design identity accepted")
	}
	for name, change := range map[string]func(map[string]any){
		"layout": func(d map[string]any) { d["analysis"].(map[string]any)["layout"] = "independent" },
		"point identity": func(d map[string]any) {
			points := d["analysis"].(map[string]any)["comparisons"].([]any)[0].(map[string]any)["points"].([]any)
			points[0].(map[string]any)["subject"] = "other"
		},
		"duplicate point": func(d map[string]any) {
			c := d["analysis"].(map[string]any)["comparisons"].([]any)[0].(map[string]any)
			p := c["points"].([]any)
			p[1] = p[0]
		},
		"missing denominator": func(d map[string]any) {
			d["analysis"].(map[string]any)["comparisons"].([]any)[1].(map[string]any)["missing"] = 0
		},
		"missing value": func(d map[string]any) {
			c := d["analysis"].(map[string]any)["comparisons"].([]any)[0].(map[string]any)
			delete(c["points"].([]any)[0].(map[string]any), "value")
		},
	} {
		var doc map[string]any
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.UseNumber()
		dec.Decode(&doc)
		change(doc)
		if _, e := prism.Map(context.Background(), mustJSON(doc)); e == nil {
			t.Fatal(name)
		}
	}
	// A fully reordered point list must preserve identity-based pairing.
	var doc map[string]any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	dec.Decode(&doc)
	for _, c := range doc["analysis"].(map[string]any)["comparisons"].([]any) {
		p := c.(map[string]any)["points"].([]any)
		for i, j := 0, len(p)-1; i < j; i, j = i+1, j-1 {
			p[i], p[j] = p[j], p[i]
		}
	}
	reordered, e := prism.Map(context.Background(), mustJSON(doc))
	if e != nil || reordered.Tables[0].Rows[0].Cells[0].Sample != "a" || reordered.Tables[0].Rows[0].Cells[1].Sample != "c" {
		t.Fatal("pairing by order", e)
	}
	for _, token := range []string{"NaN", "null", "1e999999999", "-0", "9007199254740993", "1e-301", "1e301"} {
		if prism.Number(token) == nil {
			t.Fatal("unsafe number", token)
		}
	}
	if e := prism.Number("0.1234567890123456789"); e != nil {
		t.Fatal(e)
	}
	if _, e := prism.Map(context.Background(), []byte(`{"format":1,"format":2}`)); e == nil {
		t.Fatal("duplicate key")
	}
	if _, e := prism.Map(context.Background(), []byte(strings.Repeat(" ", (32<<20)+1))); e == nil {
		t.Fatal("bytes")
	}
}

func TestPrismExcludedSubjects(t *testing.T) {
	a, h, id, _ := countFeatureFixture(t, false)
	var doc cohortFeaturesDocument
	if _, _, e := a.readCheckedResult(id, cohortFeaturesKind, &doc); e != nil {
		t.Fatal(e)
	}
	input := doc.Selection.Input
	excluded := input.Samples[0].Name
	input.Spec.Exclude = map[string]string{excluded: "Declared artificial QC exclusion"}
	design, e := study.CheckCohort(input)
	if e != nil || !design.OK {
		t.Fatal(design, e)
	}
	body := cohortFeaturesRequest{Design: cohortFeatureDesign{Record: design.ID + "-1", Name: "Declared exclusion control", Revision: 1, ID: design.ID, Input: design.Input}, Diversity: doc.ParentManifest.ID}
	w := postJSON(t, h, "/api/cohort-features", string(mustJSON(body)))
	var start struct{ Artifact, Job string }
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &start) != nil {
		t.Fatal(w.Code, w.Body)
	}
	waitForJob(t, a, start.Job, time.Minute)
	m, n, e := a.certificateNotice(start.Artifact)
	if e != nil {
		t.Fatal(e)
	}
	raw, _, e := a.makePrism(context.Background(), reportSelection{m, n}, "Artificial exclusion control")
	if e != nil {
		t.Fatal(e)
	}
	files := prismFiles(t, raw)
	var mapping prism.Mapping
	json.Unmarshal(files["mapping.json"], &mapping)
	if len(mapping.Excluded) != 1 || mapping.Excluded[0].Name != excluded || mapping.Excluded[0].Reason != "Excluded: Declared artificial QC exclusion" {
		t.Fatal("excluded evidence missing")
	}
	if dir := os.Getenv("CLONOLOOM_PRISM_OUT"); dir != "" {
		os.MkdirAll(dir, 0700)
		if e := os.WriteFile(filepath.Join(dir, "independent-excluded.zip"), raw, 0600); e != nil {
			t.Fatal(e)
		}
	}
}

func TestPrismProviderAcknowledgment(t *testing.T) {
	a, h, body, _ := cohortFeatureFixture(t)
	ds, _ := a.datasets.get(body.Design.Input.Dataset)
	ds.Credits = []credit{{Title: "Artificial Prism terms control", License: "Private study only", LicenseClass: "terms", Citation: "Fixture citation"}}
	w := postJSON(t, h, "/api/cohort-features", string(mustJSON(body)))
	var started struct{ Artifact, Job string }
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &started) != nil {
		t.Fatal(w.Code, w.Body)
	}
	waitForJob(t, a, started.Job, time.Minute)
	request := map[string]any{"artifact": started.Artifact, "title": "Private study control"}
	w = postJSON(t, h, "/api/reports/prism", string(mustJSON(request)))
	if w.Code != 409 || !strings.Contains(w.Body.String(), "restricted") {
		t.Fatal("provider gate", w.Code, w.Body)
	}
	request["accept"] = true
	w = postJSON(t, h, "/api/reports/prism", string(mustJSON(request)))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	files := prismFiles(t, w.Body.Bytes())
	if !bytes.Contains(files["NOTICE.txt"], []byte("Private study only")) {
		t.Fatal("provider notice missing")
	}
}

func TestPrismAllMissingFeatures(t *testing.T) {
	a, h, body, _ := cohortFeatureFixture(t)
	w := postJSON(t, h, "/api/diversity", string(mustJSON(map[string]any{"dataset": body.Design.Input.Dataset, "definition": map[string]any{"junction": "aa", "v": "gene", "j": "gene"}, "filter": map[string]any{"productiveOnly": true, "minCount": 1000}, "options": map[string]any{"replicates": 2}})))
	var start struct{ Artifact, Job string }
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &start) != nil {
		t.Fatal(w.Code, w.Body)
	}
	waitForJob(t, a, start.Job, time.Minute)
	body.Diversity = start.Artifact
	w = postJSON(t, h, "/api/cohort-features", string(mustJSON(body)))
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &start) != nil {
		t.Fatal(w.Code, w.Body)
	}
	waitForJob(t, a, start.Job, time.Minute)
	m, n, e := a.certificateNotice(start.Artifact)
	if e != nil {
		t.Fatal(e)
	}
	data, _, e := a.makePrism(context.Background(), reportSelection{m, n}, "Artificial no-retained-abundance control")
	if e != nil {
		t.Fatal(e)
	}
	files := prismFiles(t, data)
	var mapping prism.Mapping
	json.Unmarshal(files["mapping.json"], &mapping)
	for _, r := range mapping.Tables[2].Rows {
		for _, c := range r.Cells {
			if c.State == "observed" {
				t.Fatal("expected all-missing Shannon table")
			}
		}
	}
	if dir := os.Getenv("CLONOLOOM_PRISM_OUT"); dir != "" {
		os.MkdirAll(dir, 0700)
		if e := os.WriteFile(filepath.Join(dir, "independent-all-missing.zip"), data, 0600); e != nil {
			t.Fatal(e)
		}
	}
}
