package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

var reportEmbedded = regexp.MustCompile(`<script type="application/octet-stream" id="(report-record|report-file-[0-9]+)"(?: data-sha256="([a-f0-9]+)")?>([A-Za-z0-9+/=]+)</script>`)

func unpackHTMLReport(t *testing.T, page []byte) (reportRecord, map[string][]byte, string) {
	t.Helper()
	members := reportEmbedded.FindAllSubmatch(page, -1)
	if len(members) < 2 {
		t.Fatal("missing report payload")
	}
	raw, err := base64.StdEncoding.DecodeString(string(members[0][3]))
	if err != nil {
		t.Fatal(err)
	}
	var r reportRecord
	if err = json.Unmarshal(raw, &r); err != nil {
		t.Fatal(err)
	}
	if string(members[0][2]) != replayDigest(raw) || len(r.Files) != len(members)-1 {
		t.Fatal("report inventory")
	}
	files := map[string][]byte{}
	for i, f := range r.Files {
		data, err := base64.StdEncoding.DecodeString(string(members[i+1][3]))
		if err != nil || int64(len(data)) != f.Size || replayDigest(data) != f.SHA256 {
			t.Fatal(f.Path, err)
		}
		files[f.Path] = data
	}
	return r, files, replayDigest(raw)
}
func reportFixture(t *testing.T, kind string) (*app, string) {
	t.Helper()
	if kind != batchqcKind && kind != motifsKind {
		a, _ := testApp(t)
		return a, certificateFixture(t, a, kind)
	}
	a, _, _, spec := sourceRunFixture(t, kind)
	var id string
	if kind == batchqcKind {
		var s batchqcSpec
		json.Unmarshal(spec, &s)
		id = s.artifactID()
	} else {
		var s motifsSpec
		json.Unmarshal(spec, &s)
		// A bounded captured-evidence fixture; the full motif example exceeds this
		// report's table budget and remains available through native certificates.
		s.QueryRecords = s.QueryRecords[:12]
		s.BackgroundRecords = s.BackgroundRecords[:40]
		spec = mustJSON(s)
		id = s.artifactID()
	}
	work, err := a.jobs.Remake(kind, spec)
	if err != nil {
		t.Fatal(err)
	}
	j, _, err := a.jobs.Submit(kind, "Report fixture", id, spec, work)
	if err != nil {
		t.Fatal(err)
	}
	waitForJob(t, a, j.ID, time.Minute)
	return a, id
}
func TestHTMLReportsExactEvidenceAndReplay(t *testing.T) {
	for _, kind := range []string{cohortFeaturesKind, cohortCompareKind, cohortCountsKind, batchqcKind, motifsKind} {
		t.Run(kind, func(t *testing.T) {
			a, id := reportFixture(t, kind)
			m, n, err := a.certificateNotice(id)
			if err != nil {
				t.Fatal(err)
			}
			page, fingerprint, err := a.makeHTMLReport(context.Background(), []reportSelection{{m, n}}, "Artificial study <script>alert('x')</script>")
			if err != nil {
				t.Fatal(err)
			}
			record, files, got := unpackHTMLReport(t, page)
			if got != fingerprint || len(record.Results) != 1 {
				t.Fatal("identity")
			}
			original, err := os.ReadFile(filepath.Join(a.artifacts.Dir(), id, kind+".json"))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(original, files[id+".result.json"]) {
				t.Fatal("result JSON re-encoded")
			}
			body := string(files["report.html"])
			if strings.Contains(body, "<script>") || !strings.Contains(body, "&lt;script&gt;") || !strings.Contains(body, "Scientific") && !strings.Contains(body, "scientific") {
				t.Fatal("escaping/limits")
			}
			archive := files[id+".certificate.clz"]
			z, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
			if err != nil {
				t.Fatal(err)
			}
			for _, f := range z.File {
				if f.Name == "artifact/"+kind+".json" {
					in, _ := f.Open()
					data, _ := io.ReadAll(in)
					in.Close()
					if !bytes.Equal(data, original) {
						t.Fatal("certificate result differs")
					}
				}
			}
			// The target artifact is gone; replay uses only the embedded evidence.
			os.RemoveAll(filepath.Join(a.artifacts.Dir(), id))
			replayed := replayCertificate(context.Background(), bytes.NewReader(archive), int64(len(archive)), record.Results[0].Fingerprint)
			if replayed.Verdict != "replay-match" || !replayed.Recomputed {
				t.Fatalf("report certificate replay: %+v", replayed)
			}
			if out := os.Getenv("CLONOLOOM_REPORT_ORACLE_DIR"); out != "" {
				os.MkdirAll(out, 0700)
				if err = os.WriteFile(filepath.Join(out, kind+".html"), page, 0600); err != nil {
					t.Fatal(err)
				}
				os.WriteFile(filepath.Join(out, kind+".replay.json"), mustJSON(replayed), 0600)
			}
		})
	}
}
func TestHTMLReportTablesAndRefusals(t *testing.T) {
	cells := 0
	table, err := reportTable(json.RawMessage(`[{"n":9007199254740993,"null":null,"name":"<img src=x onerror=alert(1)>"},{"n":1.0}]`), "Exact <table>", &cells)
	if err != nil || !strings.Contains(table, "9007199254740993") || !strings.Contains(table, "1.0") || !strings.Contains(table, "(absent)") || !strings.Contains(table, ">null<") || strings.Contains(table, "<img") {
		t.Fatal(table, err)
	}
	if _, err = reportTable(json.RawMessage(`{"n":1,"n":2}`), "", &cells); err == nil {
		t.Fatal("duplicate key")
	}
	cells = 350000
	if _, err = reportTable(json.RawMessage(`[{"x":1}]`), "", &cells); err == nil {
		t.Fatal("truncated table")
	}
	a, h := testApp(t)
	id := certificateFixture(t, a, cohortCompareKind)
	for _, body := range []string{`{"title":"x","artifacts":[]}`, `{"title":"x","artifacts":["` + id + `","` + id + `"]}`, `{"title":"x","artifacts":["../escape"]}`, `{"title":"x","artifacts":["` + id + `"],"extra":true}`} {
		r := postJSON(t, h, "/api/reports/html", body)
		if r.Code == 200 {
			t.Fatal("accepted", body)
		}
	}
	r := postJSON(t, h, "/api/reports/html", string(mustJSON(map[string]any{"title": "Study", "artifacts": []string{id}})))
	if r.Code != 200 || r.Header().Get("X-ClonoLoom-Report") == "" {
		t.Fatal(r.Code, r.Body)
	}
	m, n, err := a.certificateNotice(id)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err = a.makeHTMLReport(ctx, []reportSelection{{m, n}}, "Cancelled"); err == nil {
		t.Fatal("cancelled export")
	}
	for _, selected := range [][]reportSelection{nil, {{m, n}, {m, n}}} {
		if _, _, err = a.makeHTMLReport(context.Background(), selected, "Refused"); err == nil {
			t.Fatal("selection bound")
		}
	}
	if _, _, err = a.makeHTMLReport(context.Background(), []reportSelection{{m, n}}, strings.Repeat("x", 201)); err == nil {
		t.Fatal("title bound")
	}
	os.WriteFile(filepath.Join(a.artifacts.Dir(), id, m.Kind+".json"), []byte(`{}`), 0600)
	if _, _, err = a.makeHTMLReport(context.Background(), []reportSelection{{m, n}}, "Changed"); err == nil {
		t.Fatal("changed result")
	}
}
func TestHTMLReportRestrictedTerms(t *testing.T) {
	a, id := reportFixture(t, batchqcKind)
	h, err := a.handler()
	if err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"title": "Private study", "artifacts": []string{id}}
	r := postJSON(t, h, "/api/reports/html", string(mustJSON(body)))
	if r.Code != 409 || !strings.Contains(r.Body.String(), "restricted") {
		t.Fatal(r.Code, r.Body)
	}
	body["accept"] = true
	r = postJSON(t, h, "/api/reports/html", string(mustJSON(body)))
	if r.Code != 200 {
		t.Fatal(r.Code, r.Body)
	}
}

func TestHTMLReportExplicitSelectionAndRepeatability(t *testing.T) {
	a, _ := testApp(t)
	first := certificateFixture(t, a, cohortFeaturesKind)
	second := certificateFixture(t, a, cohortCompareKind)
	selected := []reportSelection{}
	for _, id := range []string{second, first} {
		m, n, err := a.certificateNotice(id)
		if err != nil {
			t.Fatal(err)
		}
		selected = append(selected, reportSelection{m, n})
	}
	page, fingerprint, err := a.makeHTMLReport(context.Background(), selected, "Two declared results")
	if err != nil {
		t.Fatal(err)
	}
	again, fp, err := a.makeHTMLReport(context.Background(), selected, "Two declared results")
	if err != nil || fp != fingerprint || !bytes.Equal(page, again) {
		t.Fatal("repeat export changed bytes", err)
	}
	r, files, _ := unpackHTMLReport(t, page)
	if len(files) != 7 || r.Results[0].Artifact != second || r.Results[1].Artifact != first {
		t.Fatal("selection/order not explicit")
	}
}

func FuzzHTMLReportTables(f *testing.F) {
	for _, seed := range []string{`{"n":9007199254740993,"empty":null}`, `[{"label":"</script><img src=x>","n":1.0},{}]`, `[]`} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 65536 {
			t.Skip()
		}
		cells := 0
		table, err := reportTable(raw, "Recorded values", &cells)
		if err == nil && strings.Contains(table, "<script>") {
			t.Fatal("unescaped metadata")
		}
	})
}
