package main

// The record-and-reach release keeps saved bytes, source reopening and recomputation distinct.
import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"clonoloom/internal/artifacts"
)

func remakeRecordFixture(t *testing.T, a *app, dir, id, kind string, each func(*testing.T, string, []byte, []byte)) bool {
	if filepath.Base(dir) != "0.8.0" {
		return false
	}
	raw := gunzipFile(t, filepath.Join(dir, "artifacts", id, "manifest.json.gz"))
	var m artifacts.Manifest
	if err := json.Unmarshal(raw, &m); err != nil || m.Lineage.Job == nil || kind != batchqcKind {
		t.Fatal("missing captured worker", err)
	}
	oldVersion, oldCommit := version, commit
	version, commit = m.Lineage.Software[0].Version, m.Lineage.Software[0].Commit
	defer func() { version, commit = oldVersion, oldCommit }()
	work, err := a.jobs.Remake(kind, m.Lineage.Job.Spec)
	if err != nil {
		t.Fatal(err)
	}
	j, _, err := a.jobs.Submit(kind, "Record release replay", id, m.Lineage.Job.Spec, work)
	if err != nil {
		t.Fatal(err)
	}
	waitForJob(t, a, j.ID, time.Minute)
	for _, f := range m.Files {
		want := gunzipFile(t, filepath.Join(dir, "artifacts", id, f.Path+".gz"))
		if replayDigest(want) != f.SHA256 {
			t.Fatal("fixture checksum", f.Path)
		}
		got, err := os.ReadFile(filepath.Join(a.artifacts.Dir(), id, f.Path))
		if err != nil {
			t.Fatal(err)
		}
		each(t, f.Path, want, got)
	}
	return true
}

func TestRecordReleaseEvidenceAndRecovery(t *testing.T) {
	const dir = "validation/projects/0.8.0"
	read := func(path string) []byte {
		t.Helper()
		b, e := os.ReadFile(filepath.Join(dir, path))
		if e != nil {
			t.Fatal(e)
		}
		return b
	}
	var study struct{ Artifact, Checkpoint, Certificate, Archive string }
	if err := json.Unmarshal(read("study.json"), &study); err != nil {
		t.Fatal(err)
	}
	cert := read("evidence/result.certificate.clz")
	report := replayCertificate(context.Background(), bytes.NewReader(cert), int64(len(cert)), study.Certificate)
	if report.Verdict != "replay-match" || !report.Recomputed || report.Integrity.Verdict != "integrity-match" {
		t.Fatal(report)
	}
	archive := read("evidence/study.clz")
	d, err := readProjectArchive(context.Background(), bytes.NewReader(archive), int64(len(archive)), study.Archive)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(d.Files["project.json"], read("original-project.json")) {
		t.Fatal("archive project changed")
	}
	a, _ := testApp(t)
	defer a.jobs.Close()
	if _, err = a.importArchive(context.Background(), d, true); err != nil {
		t.Fatal(err)
	}
	cp, err := a.loadCheckpoint(context.Background(), study.Checkpoint)
	if err != nil {
		t.Fatal(err)
	}
	// Remove only this isolated test library's result; recovery restores exactly the recorded bytes.
	if err = os.RemoveAll(filepath.Join(a.artifacts.Dir(), study.Artifact)); err != nil {
		t.Fatal(err)
	}
	if err = a.recoverCheckpoint(context.Background(), cp); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(a.artifacts.Dir(), study.Artifact, "batch-qc.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, gunzipFile(t, filepath.Join(dir, "artifacts", study.Artifact, "batch-qc.json.gz"))) {
		t.Fatal("recovery changed science")
	}
	var truth struct {
		Files []struct{ Path, SHA256 string }
	}
	if err = json.Unmarshal(read("data/truth/truth.json"), &truth); err != nil {
		t.Fatal(err)
	}
	if len(truth.Files) == 0 {
		t.Fatal("missing source truth")
	}
	for _, f := range truth.Files {
		if replayDigest(read("data/"+f.Path)) != f.SHA256 {
			t.Fatal("source/truth changed", f.Path)
		}
	}
}

func TestReleaseVersionsAgree(t *testing.T) {
	for file, token := range map[string]string{
		"web/app.js":                               "const VERSION = '" + version + "'",
		"web/index.html":                           "app.js?v=" + version,
		"CITATION.cff":                             "version: " + version,
		"clients/python/pyproject.toml":            "version = \"" + version + "\"",
		"clients/python/src/clonoloom/_version.py": "__version__ = \"" + version + "\"",
		"clients/r/DESCRIPTION":                    "Version: " + version,
	} {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(raw), token) {
			t.Errorf("%s version differs from %s", file, version)
		}
	}
}
