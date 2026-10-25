package main

// Release replay uses the real durable workers. Relocated source paths and filesystem mtimes
// are restored to recorded provenance only AFTER verifying their bytes/identities. No scientific
// value is replaced. A verified original parent manifest (including its job ID/time/software)
// is installed for each child, since AnnData intentionally carries that provenance verbatim.
import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"clonoloom/internal/artifacts"
	"clonoloom/internal/jobs"
)

func remakeCellFixture(t *testing.T, a *app, dir, id, kind string, each func(*testing.T, string, []byte, []byte)) bool {
	switch kind {
	case expressionMapKind, phenotypeKind, modularityKind, congaKind, annDataKind:
	default:
		return false
	}
	read := func(path string, out any) []byte {
		t.Helper()
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if out != nil {
			if err := json.Unmarshal(raw, out); err != nil {
				t.Fatal(err)
			}
		}
		return raw
	}
	var loc struct{ Original, Fixture string }
	read(filepath.Join(dir, "locations.json"), &loc)
	base, err := filepath.Abs(filepath.Join(dir, loc.Fixture))
	if err != nil {
		t.Fatal(err)
	}
	relocate := func(path string) string {
		t.Helper()
		if path == "" {
			return ""
		}
		if !strings.HasPrefix(path, loc.Original+"/") {
			t.Fatalf("unrecorded source location %s", path)
		}
		return filepath.Join(base, filepath.FromSlash(strings.TrimPrefix(path, loc.Original+"/")))
	}
	move := func(s *expressionMapSpec) { s.Path = relocate(s.Path); s.CellsPath = relocate(s.CellsPath) }
	var m artifacts.Manifest
	manifest := read(filepath.Join(dir, "artifacts", id, "manifest.json"), &m)
	if m.Lineage.Job == nil {
		t.Fatal("missing durable job spec")
	}
	var work jobs.Func
	var gotID string
	decode := func(out any) {
		t.Helper()
		if err := json.Unmarshal(m.Lineage.Job.Spec, out); err != nil {
			t.Fatal(err)
		}
	}
	switch kind {
	case expressionMapKind:
		var s expressionMapSpec
		decode(&s)
		gotID = s.artifactID()
		move(&s)
		work = a.expressionMapWork(s)
	case phenotypeKind:
		var s phenotypeSpec
		decode(&s)
		gotID = s.artifactID()
		move(&s.Input)
		work = a.phenotypeWork(s)
	case modularityKind:
		var s modularitySpec
		decode(&s)
		gotID = s.artifactID()
		work = a.modularityWork(s)
	case congaKind:
		var s congaSpec
		decode(&s)
		gotID = s.artifactID()
		work = a.congaWork(s)
	case annDataKind:
		var s annDataSpec
		decode(&s)
		gotID = s.artifactID()
		move(&s.Input)
		work = a.annDataWork(s)
	}
	if gotID != id {
		t.Fatalf("method identity changed: %s, want %s", gotID, id)
	}
	// AnnData embeds the generating program's version/commit. Replaying that release preserves
	// its provenance, while the current worker must reproduce every data byte.
	oldVersion, oldCommit := version, commit
	version, commit = m.Lineage.Software[0].Version, m.Lineage.Software[0].Commit
	defer func() { version, commit = oldVersion, oldCommit }()
	j, _, err := a.jobs.Submit(kind, "Release fixture", id, m.Lineage.Job.Spec, work)
	if err != nil {
		t.Fatal(err)
	}
	waitForJob(t, a, j.ID, 2*time.Minute)
	for _, f := range m.Files {
		want := gunzipFile(t, filepath.Join(dir, "artifacts", id, f.Path+".gz"))
		path, err := a.artifacts.Path(id, f.Path)
		if err != nil {
			t.Fatal(err)
		}
		got := read(path, nil)
		// Validate every original file against the release manifest before using it as a parent.
		sum := sha256.Sum256(want)
		if hex.EncodeToString(sum[:]) != f.SHA256 {
			t.Fatal("fixture checksum changed: " + f.Path)
		}
		switch kind {
		case expressionMapKind:
			var w, g expressionMapDocument
			if err := json.Unmarshal(want, &w); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(got, &g); err != nil {
				t.Fatal(err)
			}
			restoreFixtureLocations(t, &g.Input, w.Input, relocate)
			got = mustJSON(g)
		case phenotypeKind:
			var w, g phenotypeDocument
			if err := json.Unmarshal(want, &w); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(got, &g); err != nil {
				t.Fatal(err)
			}
			restoreFixtureLocations(t, &g.Input, w.Input, relocate)
			if len(g.ReceptorSources) != len(w.ReceptorSources) {
				t.Fatal("receptor source count changed")
			}
			for i := range g.ReceptorSources {
				actual, recorded := &g.ReceptorSources[i], w.ReceptorSources[i]
				if actual.Path != relocate(recorded.Path) || fileSHA256(actual.Path) != recorded.SHA256 {
					t.Fatal("receptor source changed")
				}
				actual.Path, actual.Modified = recorded.Path, recorded.Modified
			}
			got = mustJSON(g)
		}
		each(t, f.Path, want, got)
		if t.Failed() {
			t.FailNow()
		}
		if err := os.WriteFile(path, want, 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(a.artifacts.Dir(), id, "manifest.json"), manifest, 0644); err != nil {
		t.Fatal(err)
	}
	findings, err := a.artifacts.Verify(id, nil)
	if err != nil || len(findings) > 0 {
		t.Fatalf("restored parent failed verification: %v %v", findings, err)
	}
	return true
}

func restoreFixtureLocations(t *testing.T, got *expressionSummary, want expressionSummary, relocate func(string) string) {
	t.Helper()
	if got.Path != relocate(want.Path) || got.ID != want.ID || len(got.Sources) != len(want.Sources) {
		t.Fatal("expression source identity changed")
	}
	got.Path = want.Path
	for i := range got.Sources {
		g, w := &got.Sources[i], want.Sources[i]
		if g.Path != relocate(w.Path) || fileSHA256(g.Path) != w.SHA256 {
			t.Fatal("expression source changed")
		}
		g.Path = w.Path
	}
}

// Retain the complete study, including the matrices not used by S01-1's saved analyses and
// the separate truth. This detects accidental fixture truncation, not just a changed result.
func TestSingleCellReleaseStudySources(t *testing.T) {
	dir := filepath.Join("validation", "projects", "0.6.0", "data")
	raw, err := os.ReadFile(filepath.Join(dir, "truth", "truth.json"))
	if err != nil {
		t.Fatal(err)
	}
	var truth struct {
		Model string
		Files []struct{ Path, SHA256 string }
	}
	if err := json.Unmarshal(raw, &truth); err != nil {
		t.Fatal(err)
	}
	if truth.Model != "clonoloom-artificial-cell-states-clouds-v1" || len(truth.Files) != 23 {
		t.Fatalf("unexpected study: %s, %d files", truth.Model, len(truth.Files))
	}
	for _, file := range truth.Files {
		if got := fileSHA256(filepath.Join(dir, filepath.FromSlash(file.Path))); got != file.SHA256 {
			t.Errorf("changed %s: %s, want %s", file.Path, got, file.SHA256)
		}
	}
}
