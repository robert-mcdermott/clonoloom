package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"clonoloom/internal/artifacts"
	"clonoloom/internal/certificate"
	"clonoloom/internal/diversity"
	"clonoloom/internal/jobs"
)

func replayFixture(t *testing.T, kind string, edit func(*artifacts.Manifest, []byte) []byte) []byte {
	t.Helper()
	a, _ := testApp(t)
	defer a.jobs.Close()
	id := certificateFixture(t, a, kind)
	m, n, e := a.certificateNotice(id)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := os.ReadFile(filepath.Join(a.artifacts.Dir(), id, kind+".json"))
	if e != nil {
		t.Fatal(e)
	}
	if edit != nil {
		raw = edit(&m, raw)
	}
	m.Files[0].Size = int64(len(raw))
	m.Files[0].SHA256 = replayDigest(raw)
	var out bytes.Buffer
	_, e = certificate.Build(context.Background(), &out, mustJSON(m), []byte(n.Text()), artifacts.Software{Name: "clonoloom", Version: "0.7.0"}, func(string) (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(raw)), nil })
	if e != nil {
		t.Fatal(e)
	}
	return out.Bytes()
}
func TestCertificateReplayExactWorkers(t *testing.T) {
	a, h := testApp(t)
	defer a.jobs.Close()
	for _, kind := range []string{cohortFeaturesKind, cohortCompareKind, cohortCountsKind} {
		t.Run(kind, func(t *testing.T) {
			raw := replayFixture(t, kind, nil)
			got := replayCertificate(context.Background(), bytes.NewReader(raw), int64(len(raw)), "")
			if got.Verdict != "replay-match" || !got.Recomputed || got.Integrity.Verdict != "integrity-match" {
				t.Fatalf("%+v", got)
			}
			count := 2
			if kind == cohortFeaturesKind {
				count = 1
			}
			if len(got.Steps) != count {
				t.Fatal(got.Steps)
			}
			for _, s := range got.Steps {
				if s.Expected != s.Actual || !s.MetadataMatched || s.Verdict != "replay-match" {
					t.Fatal(s)
				}
			}
			response := request(t, h, http.MethodPost, "/api/certificates/verify?mode=replay", bytes.NewReader(raw), nil)
			if response.Code != 200 {
				t.Fatal(response.Code, response.Body)
			}
			var api replayReport
			if e := json.Unmarshal(response.Body.Bytes(), &api); e != nil {
				t.Fatal(e)
			}
			if !bytes.Equal(mustJSON(got), mustJSON(api)) {
				t.Fatal("API replay differs")
			}
			path := filepath.Join(t.TempDir(), "evidence.clz")
			os.WriteFile(path, raw, 0600)
			reportPath := filepath.Join(t.TempDir(), "report.json")
			var stdout, stderr bytes.Buffer
			if code := runVerify([]string{"--replay", "--report", reportPath, path}, &stdout, &stderr); code != 0 {
				t.Fatal(code, stdout.String(), stderr.String())
			}
			var cli replayReport
			b, _ := os.ReadFile(reportPath)
			json.Unmarshal(b, &cli)
			if !bytes.Equal(mustJSON(cli), mustJSON(got)) {
				t.Fatal("CLI replay differs")
			}
			if code := runVerify([]string{"--replay", "--report", reportPath, path}, &stdout, &stderr); code != 2 {
				t.Fatal("replay report overwritten", code)
			}
			after, _ := os.ReadFile(reportPath)
			if !bytes.Equal(b, after) {
				t.Fatal("saved replay report changed")
			}
			// No normal store is consulted or populated by any replay.
			entries, _ := os.ReadDir(a.artifacts.Dir())
			for _, e := range entries {
				if e.Name() != "staging" {
					t.Fatal("replay polluted library", e.Name())
				}
			}
		})
	}
	if response := request(t, h, http.MethodPost, "/api/certificates/verify?mode=unknown", nil, nil); response.Code != 400 {
		t.Fatal("unknown mode accepted", response.Code)
	}
}
func TestCertificateReplayDiffersAndUnavailable(t *testing.T) {
	cases := []struct {
		name, kind, verdict string
		edit                func(*artifacts.Manifest, []byte) []byte
	}{
		{"one-bit-effect", cohortCompareKind, "differs", func(m *artifacts.Manifest, b []byte) []byte {
			var d cohortCompareDocument
			json.Unmarshal(b, &d)
			v := d.Result.Comparisons[0].Mean.Estimate
			d.Result.Comparisons[0].Mean.Estimate = diversity.Number(math.Nextafter(float64(v), math.Inf(1)))
			return mustJSON(d)
		}},
		{"metadata", cohortCountsKind, "differs", func(m *artifacts.Manifest, b []byte) []byte { m.Lineage.Method.Citation = "changed"; return b }},
		{"missing-parent", cohortFeaturesKind, "unavailable", func(m *artifacts.Manifest, b []byte) []byte {
			var d map[string]json.RawMessage
			json.Unmarshal(b, &d)
			delete(d, "parent")
			return mustJSON(d)
		}},
		{"changed-parent", cohortFeaturesKind, "differs", func(m *artifacts.Manifest, b []byte) []byte {
			var d cohortFeaturesDocument
			json.Unmarshal(b, &d)
			d.Parent.DatasetName += " changed"
			return mustJSON(d)
		}},
		{"parent-manifest-checksum", cohortFeaturesKind, "differs", func(m *artifacts.Manifest, b []byte) []byte {
			var d cohortFeaturesDocument
			json.Unmarshal(b, &d)
			d.ParentManifest.Files[0].SHA256 = strings.Repeat("0", 64)
			return mustJSON(d)
		}},
		{"unknown-method", cohortCompareKind, "unavailable", func(m *artifacts.Manifest, b []byte) []byte {
			var s cohortCompareSpec
			json.Unmarshal(m.Lineage.Job.Spec, &s)
			s.Method = 99
			m.Lineage.Job.Spec = mustJSON(s)
			return b
		}},
		{"missing-counts", cohortCountsKind, "unavailable", func(m *artifacts.Manifest, b []byte) []byte {
			var s cohortCountsSpec
			json.Unmarshal(m.Lineage.Job.Spec, &s)
			s.Input.Samples = nil
			m.Lineage.Job.Spec = mustJSON(s)
			return b
		}},
		{"parent-binding", cohortCountsKind, "differs", func(m *artifacts.Manifest, b []byte) []byte {
			m.Lineage.Parents[0].SHA256 = strings.Repeat("0", 64)
			return b
		}},
		{"unsupported-kind", cohortFeaturesKind, "unavailable", func(m *artifacts.Manifest, b []byte) []byte {
			m.Kind = "future"
			m.Files[0].Path = "future.json"
			return b
		}},
		{"ambiguous-recipe", cohortCompareKind, "unavailable", func(m *artifacts.Manifest, b []byte) []byte {
			// Fixtures may be indented. Prepend a duplicate key rather than assume
			// a particular colon/whitespace encoding; the original key remains last.
			m.Lineage.Job.Spec = append([]byte(`{"method":999,`), m.Lineage.Job.Spec[1:]...)
			return b
		}},
		{"count-total-worker-refusal", cohortCountsKind, "unavailable", func(m *artifacts.Manifest, b []byte) []byte {
			var s cohortCountsSpec
			json.Unmarshal(m.Lineage.Job.Spec, &s)
			s.Input.Totals[0]++
			m.ID = s.artifactID()
			m.Lineage.Job.Spec = mustJSON(s)
			return b
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := replayFixture(t, tc.kind, tc.edit)
			got := replayCertificate(context.Background(), bytes.NewReader(raw), int64(len(raw)), "")
			if got.Integrity.Verdict != "integrity-match" || got.Verdict != tc.verdict || len(got.Findings) == 0 {
				t.Fatalf("%+v", got)
			}
			path := filepath.Join(t.TempDir(), "case.clz")
			os.WriteFile(path, raw, 0600)
			var out, err bytes.Buffer
			want := 1
			if tc.verdict == "unavailable" {
				want = 2
			}
			if code := runVerify([]string{"--replay", path}, &out, &err); code != want {
				t.Fatal(code, want, out.String(), err.String())
			}
		})
	}
	raw := replayFixture(t, cohortCountsKind, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := replayCertificate(ctx, bytes.NewReader(raw), int64(len(raw)), ""); got.Verdict != "unavailable" || got.Recomputed {
		t.Fatal(got)
	}
	if got := replayCertificate(context.Background(), bytes.NewReader(raw), int64(len(raw)), strings.Repeat("0", 64)); got.Verdict != "differs" || got.Recomputed {
		t.Fatal(got)
	}
}

func TestCertificateReplayReleaseCohortControls(t *testing.T) {
	a, _ := testApp(t)
	defer a.jobs.Close()
	b, e := os.ReadFile("validation/projects/0.7.0/project.json")
	if e != nil {
		t.Fatal(e)
	}
	var p struct {
		Artifacts []struct{ ID, Kind, Name string }
	}
	if e = json.Unmarshal(b, &p); e != nil {
		t.Fatal(e)
	}
	count := 0
	for _, entry := range p.Artifacts {
		if entry.Kind == "diversity" {
			continue
		}
		count++
		t.Run(entry.Name, func(t *testing.T) {
			id := certificateFixture(t, a, entry.Kind, entry.ID)
			m, n, e := a.certificateNotice(id)
			if e != nil {
				t.Fatal(e)
			}
			f, record, e := a.makeCertificate(context.Background(), m, n)
			if e != nil {
				t.Fatal(e)
			}
			defer func() { f.Close(); os.Remove(f.Name()) }()
			// Nothing in the replay store is hydrated from the application's library.
			if e = os.RemoveAll(filepath.Join(a.artifacts.Dir(), id)); e != nil {
				t.Fatal(e)
			}
			stat, e := f.Stat()
			if e != nil {
				t.Fatal(e)
			}
			r := replayCertificate(context.Background(), f, stat.Size(), record.Fingerprint)
			if r.Verdict != "replay-match" || !r.Recomputed {
				t.Fatalf("%+v", r)
			}
			if !bytes.Equal(mustJSON(m), mustJSON(r.Original)) {
				t.Fatal("original provenance changed")
			}
			if p := os.Getenv("CLONOLOOM_REPLAY_ORACLE_DIR"); p != "" {
				raw, e := io.ReadAll(f)
				if e != nil {
					t.Fatal(e)
				}
				if e = os.WriteFile(filepath.Join(p, id+".clz"), raw, 0600); e != nil {
					t.Fatal(e)
				}
				if e = os.WriteFile(filepath.Join(p, id+".report.json"), mustJSON(r), 0600); e != nil {
					t.Fatal(e)
				}
			}
		})
	}
	if count != 12 {
		t.Fatal("release control coverage changed", count)
	}
}

func TestCertificateReplayCancellationAndCleanup(t *testing.T) {
	raw := replayFixture(t, cohortFeaturesKind, nil)
	temp := t.TempDir()
	t.Setenv("TMPDIR", temp)
	t.Setenv("TMP", temp)
	t.Setenv("TEMP", temp)
	r := replayCertificate(context.Background(), bytes.NewReader(raw), int64(len(raw)), "")
	if r.Verdict != "replay-match" {
		t.Fatal(r)
	}
	entries, e := os.ReadDir(temp)
	if e != nil || len(entries) != 0 {
		t.Fatal("temporary replay store retained", entries, e)
	}
	// Cancellation of a running worker must reach its context, not just the waiter.
	a, _ := testApp(t)
	defer a.jobs.Close()
	id := certificateFixture(t, a, cohortFeaturesKind)
	m, _ := a.artifacts.Get(id)
	output, _ := os.ReadFile(filepath.Join(a.artifacts.Dir(), id, cohortFeaturesKind+".json"))
	os.RemoveAll(filepath.Join(a.artifacts.Dir(), id))
	started := make(chan struct{})
	stopped := make(chan struct{})
	a.jobs.Handle(cohortFeaturesKind, func(json.RawMessage) (jobs.Func, error) {
		return func(ctx context.Context, _ *jobs.Run) error {
			close(started)
			<-ctx.Done()
			close(stopped)
			return ctx.Err()
		}, nil
	}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { <-started; cancel() }()
	r.Original = &m
	if err := replayCohort(ctx, a, m, output, &r); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop")
	}
}

func TestVerifyExistingReportRefusedBeforeReplay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.json")
	if err := os.WriteFile(path, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	if code := runVerify([]string{"--replay", "--report", path, "missing-input.clz"}, &out, &stderr); code != 2 || !strings.Contains(stderr.String(), "report path already exists") || out.Len() != 0 {
		t.Fatal(code, out.String(), stderr.String())
	}
	b, err := os.ReadFile(path)
	if err != nil || string(b) != "keep" {
		t.Fatal(string(b), err)
	}
}
