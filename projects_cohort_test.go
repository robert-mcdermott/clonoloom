package main

// Recompute the release's cohort chain through real workers. Verify every fixture byte
// before restoring parent manifests: job/time provenance is part of each child's record.
import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"clonoloom/internal/artifacts"
	"clonoloom/internal/jobs"
	"clonoloom/internal/study"
)

func remakeCohortFixture(t *testing.T, a *app, dir, id, kind string, each func(*testing.T, string, []byte, []byte)) bool {
	if filepath.Base(dir) != "0.7.0" {
		return false
	}
	raw := gunzipFile(t, filepath.Join(dir, "artifacts", id, "manifest.json.gz"))
	var err error
	var m artifacts.Manifest
	if err = json.Unmarshal(raw, &m); err != nil || m.Lineage.Job == nil {
		t.Fatal("missing replay provenance", err)
	}
	decode := func(out any) {
		t.Helper()
		if err := json.Unmarshal(m.Lineage.Job.Spec, out); err != nil {
			t.Fatal(err)
		}
	}
	var work jobs.Func
	var gotID string
	switch kind {
	case "diversity":
		var s analysisSpec
		decode(&s)
		gotID = s.artifactID("div-")
		work = a.diversityWork(diversityAnalysis, s)
	case cohortFeaturesKind:
		var s cohortFeaturesSpec
		decode(&s)
		gotID = s.artifactID()
		work = a.cohortFeaturesWork(s)
	case cohortCompareKind:
		var s cohortCompareSpec
		decode(&s)
		gotID = s.artifactID()
		work = a.cohortCompareWork(s)
	case cohortCountsKind:
		var s cohortCountsSpec
		decode(&s)
		gotID = s.artifactID()
		work = a.cohortCountsWork(s)
	default:
		t.Fatal("unexpected release result", kind)
	}
	if gotID != id {
		t.Fatalf("identity changed: %s != %s", gotID, id)
	}
	oldVersion, oldCommit := version, commit
	version, commit = m.Lineage.Software[0].Version, m.Lineage.Software[0].Commit
	defer func() { version, commit = oldVersion, oldCommit }()
	j, _, err := a.jobs.Submit(kind, "Cohort release replay", id, m.Lineage.Job.Spec, work)
	if err != nil {
		t.Fatal(err)
	}
	waitForJob(t, a, j.ID, 2*time.Minute)
	for _, f := range m.Files {
		want := gunzipFile(t, filepath.Join(dir, "artifacts", id, f.Path+".gz"))
		sum := sha256.Sum256(want)
		if hex.EncodeToString(sum[:]) != f.SHA256 {
			t.Fatal("fixture checksum changed", f.Path)
		}
		path, err := a.artifacts.Path(id, f.Path)
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		each(t, f.Path, want, got)
		if t.Failed() {
			t.FailNow()
		}
	}
	if err = os.WriteFile(filepath.Join(a.artifacts.Dir(), id, "manifest.json"), raw, 0644); err != nil {
		t.Fatal(err)
	}
	if findings, err := a.artifacts.Verify(id, nil); err != nil || len(findings) > 0 {
		t.Fatal(findings, err)
	}
	return true
}

func TestCohortReleaseDesignChecks(t *testing.T) {
	raw, err := os.ReadFile("validation/projects/0.7.0/project.json")
	if err != nil {
		t.Fatal(err)
	}
	var p struct {
		CohortDesigns []struct {
			Name   string
			Result study.CohortDesign
		}
	}
	if err = json.Unmarshal(raw, &p); err != nil {
		t.Fatal(err)
	}
	if len(p.CohortDesigns) != 5 {
		t.Fatal("missing guided studies")
	}
	for i, d := range p.CohortDesigns {
		got, err := study.CheckCohort(d.Result.Input)
		if err != nil {
			t.Fatal(err)
		}
		if string(mustJSON(got)) != string(mustJSON(d.Result)) {
			t.Fatal("saved design changed", i)
		}
		if got.OK != (i != 4) {
			t.Fatal("confounded design disposition changed")
		}
		if i == 2 && (got.Groups[0].Subjects != 12 || got.Groups[1].Subjects != 20) {
			t.Fatal("unbalanced cohort changed")
		}
		if i == 3 && got.CompletePairs != 28 {
			t.Fatal("paired cohort changed")
		}
		// The guide's assignments must agree with every literal generating truth row,
		// not just produce a structurally valid design with the expected group sizes.
		preset := strings.TrimPrefix(d.Name, "Guided ")
		raw, err := os.ReadFile(filepath.Join("validation/projects/0.7.0/data", "simulated-"+preset, "truth/observations.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		reader := csv.NewReader(strings.NewReader(string(raw)))
		reader.Comma = '\t'
		rows, err := reader.ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		assignments := map[string]study.CohortSample{}
		for _, sample := range got.Input.Samples {
			assignments[sample.Name] = sample
		}
		for _, row := range rows[1:] {
			sample, ok := assignments[row[0]]
			if !ok || sample.Subject != row[1] || sample.Batch != row[3] {
				t.Fatal("guide differs from generating subject/batch", row[0])
			}
			level := sample.Condition
			want := "control"
			if row[2] == "1" {
				want = "stimulated"
			}
			if got.Input.Spec.Layout == "paired" {
				level = sample.Timepoint
				want = "day 0"
				if row[2] == "1" {
					want = "day 14"
				}
			}
			if level != want {
				t.Fatal("guide differs from generating group", row[0], level, want)
			}
		}

	}
	// Check every source and all truth files, including files not consumed by analysis.
	dirs, _ := filepath.Glob("validation/projects/0.7.0/data/*/truth/truth.json")
	if len(dirs) != 5 {
		t.Fatal("missing truth")
	}
	for _, path := range dirs {
		var truth struct {
			Files []struct{ Path, SHA256 string }
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(raw, &truth); err != nil {
			t.Fatal(err)
		}
		for _, f := range truth.Files {
			if fileSHA256(filepath.Join(filepath.Dir(filepath.Dir(path)), f.Path)) != f.SHA256 {
				t.Fatal("source changed", f.Path)
			}
		}
	}
}
