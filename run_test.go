package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"clonoloom/internal/artifacts"
	"clonoloom/internal/certificate"
	"clonoloom/internal/export"
)

func sourceRunFixture(t *testing.T, kind string) (*app, runRecipe, inputBindings, json.RawMessage) {
	t.Helper()
	var a *app
	var datasetID string
	var params json.RawMessage
	var spec json.RawMessage
	clock := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)
	switch kind {
	case batchqcKind:
		host, _, req, _ := batchqcFixture(t)
		a = host
		datasetID = req.Dataset
		s, e := a.captureBatchQC(req)
		if e != nil {
			t.Fatal(e)
		}
		s.Notice.Made = clock
		spec = mustJSON(s)
		req.Dataset = ""
		params = mustJSON(req)
	case motifsKind:
		host, _, req, _ := motifFixture(t)
		a = host
		datasetID = req.Dataset
		s, e := a.captureMotifs(req)
		if e != nil {
			t.Fatal(e)
		}
		s.Notice.Made = clock
		spec = mustJSON(s)
		req.Dataset = ""
		params = mustJSON(req)
	default:
		t.Fatal(kind)
	}
	t.Cleanup(a.jobs.Close)
	ds, _ := a.datasets.get(datasetID)
	task := recipeTask{ID: "study", Operation: "compute", Kind: kind, Study: ds.Name, Parameters: params}
	bindings := inputBindings{}
	for i, in := range ds.Inputs {
		raw, e := os.ReadFile(in.Path)
		if e != nil {
			t.Fatal(e)
		}
		name := fmt.Sprintf("source%d", i)
		task.Inputs = append(task.Inputs, recipeInput{name, in.Name, int64(len(raw)), replayDigest(raw)})
		bindings[name] = in.Path
	}
	for _, c := range ds.Credits {
		task.Credits = append(task.Credits, export.Credit{Title: c.Title, Source: c.Source, License: c.License, LicenseURL: c.LicenseURL, LicenseClass: c.LicenseClass, Citation: c.Citation})
	}
	return a, runRecipe{Format: "clonoloom-run-recipe", Version: 1, Time: clock.Format(time.RFC3339), Tasks: []recipeTask{task}}, bindings, spec
}
func invokeRun(t *testing.T, recipe runRecipe, bindings inputBindings, out string, extra ...string) (int, string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "recipe.json")
	if e := os.WriteFile(p, mustJSON(recipe), 0600); e != nil {
		t.Fatal(e)
	}
	args := []string{"--recipe", p, "--out", out}
	for _, in := range recipe.Tasks[0].Inputs {
		if path, ok := bindings[in.Name]; ok {
			args = append(args, "--input", in.Name+"="+path)
		}
	}
	args = append(args, extra...)
	var stdout, stderr bytes.Buffer
	code := runHeadless(args, &stdout, &stderr)
	return code, stdout.String() + stderr.String()
}
func TestHeadlessSourceWorkersExactAndPortable(t *testing.T) {
	// These captured source-run checksums include software provenance. Keep the
	// original fixture version, just as the release-project replay tests do.
	oldVersion, oldCommit := version, commit
	version, commit = "0.7.0", ""
	defer func() { version, commit = oldVersion, oldCommit }()
	for _, kind := range []string{batchqcKind, motifsKind} {
		t.Run(kind, func(t *testing.T) {
			a, recipe, bindings, expectedSpec := sourceRunFixture(t, kind)
			out := filepath.Join(t.TempDir(), "run")
			if code, text := invokeRun(t, recipe, bindings, out, "--accept-terms"); code != 0 {
				t.Fatal(code, text)
			}
			var m artifacts.Manifest
			raw, e := os.ReadFile(filepath.Join(out, "manifest.json"))
			if e != nil {
				t.Fatal(e)
			}
			json.Unmarshal(raw, &m)
			if !bytes.Equal(mustJSON(json.RawMessage(expectedSpec)), mustJSON(m.Lineage.Job.Spec)) {
				t.Fatalf("headless capture differs from app capture\n%s\n%s", expectedSpec, m.Lineage.Job.Spec)
			}
			work, e := a.jobs.Remake(kind, expectedSpec)
			if e != nil {
				t.Fatal(e)
			}
			j, _, e := a.jobs.Submit(kind, "UI-equivalent worker", m.ID, expectedSpec, work)
			if e != nil {
				t.Fatal(e)
			}
			waitForJob(t, a, j.ID, 60*time.Second)
			original, e := os.ReadFile(filepath.Join(a.artifacts.Dir(), m.ID, kind+".json"))
			if e != nil {
				t.Fatal(e)
			}
			headless, _ := os.ReadFile(filepath.Join(out, kind+".json"))
			t.Logf("%s output SHA-256: %s", kind, replayDigest(headless))
			pinned := map[string]string{
				batchqcKind: "731242703fcd74a2a8f7b834c46442f482fa1d02ad7a7c46aad8074d5eb53ea2",
				motifsKind:  "79a97542b9bfccb465a6539bfe7c979d0cae6f327dfdb904c4f93823070089e7",
			}
			if replayDigest(headless) != pinned[kind] {
				t.Fatal("pinned captured result changed", kind)
			}
			if !bytes.Equal(original, headless) {
				t.Fatal("worker output differs")
			}
			h, e := a.handler()
			if e != nil {
				t.Fatal(e)
			}
			_, notice, e := a.certificateNotice(m.ID)
			if e != nil {
				t.Fatal(e)
			}
			meta := request(t, h, "GET", "/api/certificates/"+m.ID, nil, nil)
			if notice.Restricted() && meta.Code != 409 {
				t.Fatal("provider gate", meta.Code)
			}
			download := request(t, h, "GET", "/api/certificates/"+m.ID+"/download?accept=1", nil, nil)
			if download.Code != 200 {
				t.Fatal(download.Code, download.Body)
			}
			api := request(t, h, "POST", "/api/certificates/verify?mode=replay", bytes.NewReader(download.Body.Bytes()), nil)
			var apiReport replayReport
			if api.Code != 200 || json.Unmarshal(api.Body.Bytes(), &apiReport) != nil || apiReport.Verdict != "replay-match" {
				t.Fatal(api.Code, api.Body)
			}
			cert, _ := os.ReadFile(filepath.Join(out, "result.certificate.clz"))
			archive, e := zip.NewReader(bytes.NewReader(cert), int64(len(cert)))
			if e != nil {
				t.Fatal(e)
			}
			for _, name := range []string{"manifest.json", "NOTICE.txt"} {
				file, e := archive.Open(name)
				if e != nil {
					t.Fatal(e)
				}
				embedded, e := io.ReadAll(file)
				file.Close()
				standalone, readErr := os.ReadFile(filepath.Join(out, name))
				if e != nil || readErr != nil || !bytes.Equal(embedded, standalone) {
					t.Fatal("changed archived bytes", name, e, readErr)
				}
			}
			r := replayCertificate(context.Background(), bytes.NewReader(cert), int64(len(cert)), "")
			if r.Verdict != "replay-match" || !r.Recomputed || len(r.Steps) != 1 {
				t.Fatalf("%+v", r)
			}
			// Relocating named inputs and replaying the effective recipe preserves every output.
			effectiveRaw, _ := os.ReadFile(filepath.Join(out, "effective-recipe.json"))
			var effective runRecipe
			if e = json.Unmarshal(effectiveRaw, &effective); e != nil {
				t.Fatal(e)
			}
			moved := inputBindings{}
			for name, path := range bindings {
				raw, e := os.ReadFile(path)
				if e != nil {
					t.Fatal(e)
				}
				target := filepath.Join(t.TempDir(), "renamed.data")
				os.WriteFile(target, raw, 0600)
				moved[name] = target
			}
			again := filepath.Join(t.TempDir(), "again")
			if code, text := invokeRun(t, effective, moved, again, "--accept-terms"); code != 0 {
				t.Fatal(code, text)
			}
			var record runRecord
			b, _ := os.ReadFile(filepath.Join(out, "run.json"))
			json.Unmarshal(b, &record)
			if record.Status != "complete" || record.Artifact != m.ID {
				t.Fatal(record)
			}
			for _, file := range record.Outputs {
				one, _ := os.ReadFile(filepath.Join(out, file.Path))
				two, _ := os.ReadFile(filepath.Join(again, file.Path))
				if !bytes.Equal(one, two) || int64(len(one)) != file.Size || replayDigest(one) != file.SHA256 {
					t.Fatal("output drift", file.Path)
				}
			}
			if code, _ := invokeRun(t, recipe, bindings, out, "--accept-terms"); code != 2 {
				t.Fatal("overwrote existing run")
			}
			after, _ := os.ReadFile(filepath.Join(out, "run.json"))
			if !bytes.Equal(b, after) {
				t.Fatal("existing run changed")
			}
			// Changed scientific evidence with freshly valid checksums must still differ.
			var changed map[string]json.RawMessage
			json.Unmarshal(headless, &changed)
			changed["description"] = mustJSON("changed scientific description")
			altered := mustJSON(changed)
			m.Files[0].Size = int64(len(altered))
			m.Files[0].SHA256 = replayDigest(altered)
			var zip bytes.Buffer
			_, e = certificate.Build(context.Background(), &zip, mustJSON(m), []byte("Test notice"), artifacts.Software{Name: "clonoloom", Version: version}, func(string) (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(altered)), nil })
			if e != nil {
				t.Fatal(e)
			}
			r = replayCertificate(context.Background(), bytes.NewReader(zip.Bytes()), int64(zip.Len()), "")
			if r.Verdict != "differs" || r.Integrity.Verdict != "integrity-match" {
				t.Fatal(r)
			}
			// An otherwise coherent archive with a future worker version is unavailable.
			if kind == batchqcKind {
				var s batchqcSpec
				json.Unmarshal(m.Lineage.Job.Spec, &s)
				s.Method++
				m.Lineage.Job.Spec = mustJSON(s)
			} else {
				var s motifsSpec
				json.Unmarshal(m.Lineage.Job.Spec, &s)
				s.Method++
				m.Lineage.Job.Spec = mustJSON(s)
			}
			zip.Reset()
			_, e = certificate.Build(context.Background(), &zip, mustJSON(m), []byte("Test notice"), artifacts.Software{Name: "clonoloom", Version: version}, func(string) (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(altered)), nil })
			if e != nil {
				t.Fatal(e)
			}
			r = replayCertificate(context.Background(), bytes.NewReader(zip.Bytes()), int64(zip.Len()), "")
			if r.Verdict != "unavailable" || r.Recomputed || r.Integrity.Verdict != "integrity-match" {
				t.Fatal(r)
			}
		})
	}
}
func TestHeadlessRefusalsAndTaskSelection(t *testing.T) {
	_, recipe, bindings, _ := sourceRunFixture(t, batchqcKind)
	out := filepath.Join(t.TempDir(), "restricted")
	if code, text := invokeRun(t, recipe, bindings, out); code != 2 || !strings.Contains(text, "accept-terms") {
		t.Fatal(code, text)
	}
	if _, e := os.Stat(filepath.Join(out, "batch-qc.json")); !os.IsNotExist(e) {
		t.Fatal("exported restricted evidence")
	}
	original := mustJSON(recipe)
	cases := []struct {
		name   string
		change func(*runRecipe)
		code   int
	}{
		{"hash", func(r *runRecipe) { r.Tasks[0].Inputs[0].SHA256 = strings.Repeat("0", 64) }, 2},
		{"missing", func(r *runRecipe) { r.Tasks[0].Inputs[0].Name = "missing" }, 2},
		{"unsupported", func(r *runRecipe) { r.Tasks[0].Kind = "diversity" }, 2},
		{"unknown-parameters", func(r *runRecipe) { r.Tasks[0].Parameters = json.RawMessage(`{"extra":true}`) }, 2},
		{"duplicate-key", func(r *runRecipe) { r.Tasks[0].Parameters = json.RawMessage(`{"samples":[],"samples":[]}`) }, 2},
		{"bad-science", func(r *runRecipe) { r.Tasks[0].Parameters = json.RawMessage(`{"samples":[]}`) }, 1},
		{"path", func(r *runRecipe) { r.Tasks[0].Inputs[0].File = "../escape.tsv" }, 2},
		{"oversize", func(r *runRecipe) { r.Tasks[0].Inputs[0].Size = certificate.MaxFileBytes + 1 }, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var r runRecipe
			json.Unmarshal(original, &r)
			tc.change(&r)
			out := filepath.Join(t.TempDir(), "run")
			if code, text := invokeRun(t, r, bindings, out, "--accept-terms"); code != tc.code {
				t.Fatal(code, text)
			}
			if raw, e := os.ReadFile(filepath.Join(out, "run.json")); e == nil {
				var record runRecord
				json.Unmarshal(raw, &record)
				if record.Status == "complete" {
					t.Fatal("failed task marked complete")
				}
			}
		})
	}
	second := recipe.Tasks[0]
	second.ID = "other"
	recipe.Tasks = append(recipe.Tasks, second)
	if code, _ := invokeRun(t, recipe, bindings, filepath.Join(t.TempDir(), "ambiguous"), "--accept-terms"); code != 2 {
		t.Fatal("implicit batch")
	}
	if code, text := invokeRun(t, recipe, bindings, filepath.Join(t.TempDir(), "selected"), "--task", "other", "--accept-terms"); code != 0 {
		t.Fatal(code, text)
	}
	if code, text := invokeRun(t, recipe, bindings, filepath.Join(t.TempDir(), "cancelled"), "--task", "study", "--timeout", "1ns", "--accept-terms"); code != 2 || !strings.Contains(text, "cancelled") {
		t.Fatal(code, text)
	}
}
func TestHeadlessCertificateTask(t *testing.T) {
	raw := replayFixture(t, cohortFeaturesKind, nil)
	path := filepath.Join(t.TempDir(), "evidence.clz")
	os.WriteFile(path, raw, 0600)
	recipe := runRecipe{Format: "clonoloom-run-recipe", Version: 1, Time: "2026-10-27T00:00:00Z", Tasks: []recipeTask{{ID: "verify", Operation: "replay", Kind: cohortFeaturesKind, Inputs: []recipeInput{{"evidence", "evidence.clz", int64(len(raw)), replayDigest(raw)}}}}}
	out := filepath.Join(t.TempDir(), "out")
	if code, text := invokeRun(t, recipe, inputBindings{"evidence": path}, out); code != 0 {
		t.Fatal(code, text)
	}
	var r replayReport
	b, _ := os.ReadFile(filepath.Join(out, "replay.json"))
	json.Unmarshal(b, &r)
	if r.Verdict != "replay-match" {
		t.Fatal(r)
	}
	recipe.Tasks[0].Fingerprint = strings.Repeat("0", 64)
	if code, _ := invokeRun(t, recipe, inputBindings{"evidence": path}, filepath.Join(t.TempDir(), "differs")); code != 1 {
		t.Fatal("wrong pin", code)
	}
}
func TestCertificateSupportMatrix(t *testing.T) {
	a, h := testApp(t)
	defer a.jobs.Close()
	r := request(t, h, "GET", "/api/certificates/support", nil, nil)
	var rows []workflowSupport
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &rows) != nil {
		t.Fatal(r.Code, r.Body)
	}
	if len(rows) != len(a.jobs.Kinds()) {
		t.Fatal("omitted registered kind")
	}
	known := map[string]bool{}
	count := 0
	js, e := os.ReadFile("web/lib/certificates.js")
	if e != nil {
		t.Fatal(e)
	}
	docs, e := os.ReadFile("docs/HEADLESS.md")
	if e != nil {
		t.Fatal(e)
	}
	for _, row := range rows {
		if !strings.Contains(string(docs), "`"+row.Kind+"`") {
			t.Fatal("undocumented kind", row.Kind)
		}
		if known[row.Kind] {
			t.Fatal("duplicate kind")
		}
		known[row.Kind] = true
		if row.Replay {
			count++
			if !row.Certificate || !strings.Contains(string(js), "'"+row.Kind+"'") {
				t.Fatal(row)
			}
		} else {
			if row.Certificate || row.SourceRun || row.Scope != "" {
				t.Fatal(row)
			}
		}
	}
	if count != 5 {
		t.Fatal("support matrix drift", count)
	}
}

func TestHeadlessInputAndSchemaBounds(t *testing.T) {
	for _, raw := range []string{`null`, `[]`, `{"version":1,"version":2}`, `{"unknown":1}`, `{} {}`} {
		path := filepath.Join(t.TempDir(), "recipe.json")
		os.WriteFile(path, []byte(raw), 0600)
		if _, _, _, err := readRunRecipe(path, ""); err == nil {
			t.Fatal("accepted", raw)
		}
	}
	var bindings inputBindings
	if bindings.Set("input=/path/with=equals") != nil || bindings["input"] != "/path/with=equals" {
		t.Fatal(bindings)
	}
	if bindings.Set("input=another") == nil || bindings.Set("../escape=path") == nil {
		t.Fatal("bad binding accepted")
	}
	root := t.TempDir()
	path := filepath.Join(root, "source.tsv")
	raw := []byte("original input bytes")
	os.WriteFile(path, raw, 0600)
	in := recipeInput{"source", "source.tsv", int64(len(raw)), replayDigest(raw)}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if snapshotRunInput(ctx, in, path, filepath.Join(root, "cancelled")) == nil {
		t.Fatal("ignored cancellation")
	}
	if snapshotRunInput(context.Background(), in, root, filepath.Join(root, "directory")) == nil {
		t.Fatal("accepted directory")
	}
	link := filepath.Join(root, "link.tsv")
	if err := os.Symlink(path, link); err == nil {
		if snapshotRunInput(context.Background(), in, link, filepath.Join(root, "linked")) == nil {
			t.Fatal("accepted link")
		}
		if _, _, _, err := readRunRecipe(link, ""); err == nil {
			t.Fatal("linked recipe")
		}
	}
	copy := filepath.Join(root, "copy.tsv")
	if err := snapshotRunInput(context.Background(), in, path, copy); err != nil {
		t.Fatal(err)
	}
	if snapshotRunInput(context.Background(), in, path, copy) == nil {
		t.Fatal("overwrote snapshot")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(raw, after) {
		t.Fatal("modified source")
	}
}
func FuzzRunRecipeJSON(f *testing.F) {
	f.Add([]byte(`{"format":"clonoloom-run-recipe","version":1,"tasks":[]}`))
	f.Add([]byte(`{"version":1,"version":2}`))
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > recipeLimit {
			t.Skip()
		}
		var r runRecipe
		_ = strictRunJSON(raw, &r)
	})
}
