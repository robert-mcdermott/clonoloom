package main

import (
	"archive/zip"
	"bytes"
	"clonoloom/internal/countmodel"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"clonoloom/internal/certificate"
)

func unpackWorkbook(t *testing.T, data []byte) (workbookRecord, map[string][]byte) {
	t.Helper()
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, f := range z.File {
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		raw, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
		files[f.Name] = raw
	}
	var record workbookRecord
	if err := json.Unmarshal(files["workbook.json"], &record); err != nil {
		t.Fatal(err)
	}
	for _, f := range record.Files {
		raw, ok := files[f.Path]
		if !ok || int64(len(raw)) != f.Size || replayDigest(raw) != f.SHA256 {
			t.Fatal("inventory differs", f.Path)
		}
	}
	return record, files
}
func TestWorkbookSavedEvidence(t *testing.T) {
	for _, kind := range []string{cohortFeaturesKind, cohortCompareKind, cohortCountsKind, batchqcKind, motifsKind} {
		t.Run(kind, func(t *testing.T) {
			var a *app
			var id string
			if kind == cohortCountsKind {
				var parent string
				var h http.Handler
				a, h, parent, _ = countFeatureFixture(t, false)
				w := postJSON(t, h, "/api/cohort-counts", string(mustJSON(cohortCountsRequest{Features: parent, Options: countmodel.Options{AdjustBatch: true}})))
				var started struct{ Artifact, Job string }
				if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &started) != nil {
					t.Fatal(w.Code, w.Body)
				}
				waitForJob(t, a, started.Job, time.Minute)
				id = started.Artifact
			} else if kind == motifsKind {
				var spec json.RawMessage
				a, _, _, spec = sourceRunFixture(t, kind)
				var s motifsSpec
				json.Unmarshal(spec, &s)
				s.QueryRecords = s.QueryRecords[:3]
				s.BackgroundRecords = s.BackgroundRecords[:8]
				spec, id = mustJSON(s), s.artifactID()
				work, err := a.jobs.Remake(kind, spec)
				if err != nil {
					t.Fatal(err)
				}
				j, _, err := a.jobs.Submit(kind, "Artificial workbook motif control", id, spec, work)
				if err != nil {
					t.Fatal(err)
				}
				waitForJob(t, a, j.ID, time.Minute)
			} else {
				a, id = reportFixture(t, kind)
			}
			m, n, err := a.certificateNotice(id)
			if err != nil {
				t.Fatal(err)
			}
			data, record, err := a.makeWorkbook(context.Background(), []reportSelection{{m, n}}, "Artificial study: exact recorded evidence")
			if err != nil {
				t.Fatal(err)
			}
			check, files := unpackWorkbook(t, data)
			if check.Results[0].Artifact != id || record.Results[0].Values.Nodes < 1 || record.Results[0].Provenance.Nodes < 1 {
				t.Fatal("missing evidence")
			}
			original, err := os.ReadFile(filepath.Join(a.artifacts.Dir(), id, kind+".json"))
			if err != nil || !bytes.Equal(original, files["R1.result.json"]) {
				t.Fatal("native bytes changed", err)
			}
			packed := files["R1.certificate.clz"]
			report := certificate.Verify(context.Background(), bytes.NewReader(packed), int64(len(packed)), record.Results[0].CertificateFingerprint)
			if report.Verdict != "integrity-match" || report.Recomputed {
				t.Fatal(report)
			}
			again, _, err := a.makeWorkbook(context.Background(), []reportSelection{{m, n}}, "Artificial study: exact recorded evidence")
			if err != nil || !bytes.Equal(again, data) {
				t.Fatal("nondeterministic export", err)
			}
			if dir := os.Getenv("CLONOLOOM_WORKBOOK_OUT"); dir != "" {
				if err := os.MkdirAll(dir, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, kind+".zip"), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
func TestWorkbookAPIAndRefusals(t *testing.T) {
	a, h := testApp(t)
	id := certificateFixture(t, a, cohortFeaturesKind)
	for _, body := range []string{`{"title":"x","artifacts":[]}`, `{"title":"x","artifacts":["` + id + `","` + id + `"]}`, `{"title":"x","artifacts":["../escape"]}`, `{"title":"x","artifacts":["` + id + `"],"extra":true}`, `{"title":"","artifacts":["` + id + `"]}`} {
		if w := postJSON(t, h, "/api/reports/workbook", body); w.Code == 200 {
			t.Fatal("accepted", body)
		}
	}
	body := string(mustJSON(map[string]any{"title": "Artificial features", "artifacts": []string{id}}))
	w := postJSON(t, h, "/api/reports/workbook", body)
	if w.Code != 200 || w.Header().Get("X-ClonoLoom-Workbook") != replayDigest(w.Body.Bytes()) {
		t.Fatal(w.Code, w.Body)
	}
	unpackWorkbook(t, w.Body.Bytes())
	second := certificateFixture(t, a, cohortCompareKind)
	multiple := postJSON(t, h, "/api/reports/workbook", string(mustJSON(map[string]any{"title": "Artificial combined evidence", "artifacts": []string{id, second}})))
	if multiple.Code != 200 {
		t.Fatal(multiple.Code, multiple.Body)
	}
	record, files := unpackWorkbook(t, multiple.Body.Bytes())
	if len(record.Results) != 2 || record.Results[1].ResultSheet != "R2 values" || len(files["R2.manifest.json"]) == 0 {
		t.Fatal("multi-result provenance mapping")
	}
	m, n, err := a.certificateNotice(id)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err = a.makeWorkbook(ctx, []reportSelection{{m, n}}, "Cancelled"); err == nil {
		t.Fatal("cancel")
	}
	if _, _, err = a.makeWorkbook(context.Background(), []reportSelection{{m, n}, {m, n}}, "Duplicate"); err == nil {
		t.Fatal("duplicate")
	}
	if err = os.WriteFile(filepath.Join(a.artifacts.Dir(), id, m.Kind+".json"), []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	if w := postJSON(t, h, "/api/reports/workbook", body); w.Code == 200 {
		t.Fatal("changed source")
	}
}
func TestWorkbookProviderTerms(t *testing.T) {
	a, id := reportFixture(t, batchqcKind)
	h, err := a.handler()
	if err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"title": "Private study", "artifacts": []string{id}}
	w := postJSON(t, h, "/api/reports/workbook", string(mustJSON(body)))
	if w.Code != 409 || !strings.Contains(w.Body.String(), "restricted") {
		t.Fatal(w.Code, w.Body)
	}
	body["accept"] = true
	w = postJSON(t, h, "/api/reports/workbook", string(mustJSON(body)))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
}

func TestWorkbookOversizedEvidence(t *testing.T) {
	// Historical evidence stays immutable; larger results remain native exports.
	a, id := reportFixture(t, cohortCountsKind)
	m, n, err := a.certificateNotice(id)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = a.makeWorkbook(context.Background(), []reportSelection{{m, n}}, "Full historical count evidence"); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatal("oversized evidence was not refused by a size bound", err)
	}
}
