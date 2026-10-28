package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func checkpointFixture(t *testing.T) (*app, []byte, string) {
	t.Helper()
	a, _ := testApp(t)
	t.Cleanup(a.jobs.Close)
	id := certificateFixture(t, a, cohortCompareKind)
	project := []byte(`{"format":"clonoloom-project","version":1,"id":"p-check","name":"Artificial cohort checkpoint","history":[],"datasets":[{"id":"d-1","sources":[{"path":"/absent/source.tsv","sha256":"recorded-only"}],"definition":{"junction":"aa"},"filter":"f-1"}],"samples":[{"sample":"S01","subject":"P01","condition":"A"}],"filters":[{"id":"f-1","rules":{"minCount":2}}],"cohortDesigns":[],"artifacts":[{"id":"` + id + `","kind":"cohort-compare"}]}`)
	return a, project, id
}
func TestCheckpointRecoveryExactAndReplay(t *testing.T) {
	a, project, id := checkpointFixture(t)
	m, n, err := a.certificateNotice(id)
	if err != nil {
		t.Fatal(err)
	}
	original, record, err := a.makeCertificate(context.Background(), m, n)
	if err != nil {
		t.Fatal(err)
	}
	original.Close()
	defer os.Remove(original.Name())
	saved, err := a.createCheckpoint(context.Background(), project, "Before QC revision")
	if err != nil {
		t.Fatal(err)
	}
	d, err := a.loadCheckpoint(context.Background(), saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(d.Files["project.json"], project) {
		t.Fatal("project bytes changed")
	}
	var revised map[string]json.RawMessage
	json.Unmarshal(project, &revised)
	revised["filters"] = json.RawMessage(`[{"id":"f-2","rules":{"minCount":3}}]`)
	revised["samples"] = json.RawMessage(`[{"sample":"S01","subject":"P01","condition":"B"}]`)
	revised["history"] = json.RawMessage(`[{"action":"Revision"}]`)
	after, err := a.collectCheckpoint(context.Background(), mustJSON(revised), a.artifacts.Dir())
	if err != nil {
		t.Fatal(err)
	}
	diff, err := compareCheckpoints(d, after)
	if err != nil {
		t.Fatal(err)
	}
	if len(diff.Changes) != 3 {
		t.Fatal(diff)
	}
	second, err := a.createCheckpoint(context.Background(), mustJSON(revised), "After QC revision")
	if err != nil {
		t.Fatal(err)
	}
	newer, err := a.loadCheckpoint(context.Background(), second.ID)
	if err != nil {
		t.Fatal(err)
	}
	compared, err := compareCheckpoints(d, newer)
	if err != nil || !bytes.Equal(mustJSON(compared.Changes), mustJSON(diff.Changes)) {
		t.Fatal(compared, err)
	}
	// Remove the only attached result; the checkpoint must recover its original bytes,
	// without original receptor files or a separate parent artifact.
	if err = os.RemoveAll(filepath.Join(a.artifacts.Dir(), id)); err != nil {
		t.Fatal(err)
	}
	if err = a.recoverCheckpoint(context.Background(), d); err != nil {
		t.Fatal(err)
	}
	if err = a.recoverCheckpoint(context.Background(), d); err != nil {
		t.Fatal("retry", err)
	}
	got, err := a.collectCheckpoint(context.Background(), project, a.artifacts.Dir())
	if err != nil {
		t.Fatal(err)
	}
	same, err := compareCheckpoints(d, got)
	if err != nil || len(same.Changes) != 0 {
		t.Fatal(same, err)
	}
	for path, want := range d.Files {
		if path == "project.json" {
			continue
		}
		raw, e := os.ReadFile(filepath.Join(a.artifacts.Dir(), strings.TrimPrefix(path, "artifacts/")))
		if e != nil || !bytes.Equal(want, raw) {
			t.Fatal(path, e)
		}
	}
	m, n, err = a.certificateNotice(id)
	if err != nil {
		t.Fatal(err)
	}
	cert, restored, err := a.makeCertificate(context.Background(), m, n)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { cert.Close(); os.Remove(cert.Name()) }()
	if record.Fingerprint != restored.Fingerprint {
		t.Fatal("recovery changed certificate identity")
	}
	st, _ := cert.Stat()
	replayed := replayCertificate(context.Background(), cert, st.Size(), record.Fingerprint)
	if replayed.Verdict != "replay-match" || !replayed.Recomputed {
		t.Fatal(replayed)
	}
	if dir := os.Getenv("CLONOLOOM_CHECKPOINT_ORACLE_DIR"); dir != "" {
		path, _ := a.checkpointPath(saved.ID)
		if err = os.WriteFile(filepath.Join(dir, "checkpoint-path.txt"), []byte(path), 0600); err != nil {
			t.Fatal(err)
		} // persistent evidence for the independent oracle
		if err = checkpointWrite(context.Background(), filepath.Join(dir, "evidence"), d.Files); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, "evidence", "checkpoint.json"), mustJSON(saved), 0600); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, "replay.json"), mustJSON(replayed), 0600); err != nil {
			t.Fatal(err)
		}
	}
}
func TestCheckpointRefusalsAndNoOverwrite(t *testing.T) {
	a, project, id := checkpointFixture(t)
	saved, err := a.createCheckpoint(context.Background(), project, "Before revision")
	if err != nil {
		t.Fatal(err)
	}
	d, err := a.loadCheckpoint(context.Background(), saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(a.artifacts.Dir(), id, cohortCompareKind+".json")
	original, _ := os.ReadFile(target)
	bad := append([]byte{}, original...)
	bad[len(bad)/2] ^= 1
	os.WriteFile(target, bad, 0600)
	if err = a.recoverCheckpoint(context.Background(), d); err == nil {
		t.Fatal("overwrote conflicting result")
	}
	after, _ := os.ReadFile(target)
	if !bytes.Equal(after, bad) {
		t.Fatal("conflict changed")
	}
	if _, err = a.createCheckpoint(context.Background(), project, "Changed result"); err == nil {
		t.Fatal("checkpointed changed bytes")
	}
	os.WriteFile(target, original, 0600)
	path, _ := a.checkpointPath(saved.ID)
	p := filepath.Join(path, "project.json")
	os.WriteFile(p, append(project, ' '), 0600)
	if _, err = a.loadCheckpoint(context.Background(), saved.ID); err == nil {
		t.Fatal("accepted changed project bytes")
	}
	os.WriteFile(p, project, 0600)
	recordPath := filepath.Join(path, "checkpoint.json")
	raw, _ := os.ReadFile(recordPath)
	var record checkpointRecord
	json.Unmarshal(raw, &record)
	record.Version++
	os.WriteFile(recordPath, mustJSON(record), 0600)
	if _, err = a.loadCheckpoint(context.Background(), saved.ID); err == nil {
		t.Fatal("accepted future checkpoint")
	}
	os.WriteFile(recordPath, raw, 0600)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = a.createCheckpoint(ctx, project, "Cancelled"); err == nil {
		t.Fatal("ignored cancellation")
	}
	if _, err = a.loadCheckpoint(ctx, saved.ID); err == nil {
		t.Fatal("ignored cancellation during load")
	}
	if err = a.recoverCheckpoint(ctx, d); err == nil {
		t.Fatal("ignored cancellation during recovery")
	}
	entries, _ := os.ReadDir(filepath.Join(a.store.dir, "checkpoints"))
	if len(entries) != 1 {
		t.Fatal("left staging files", entries)
	}
	for _, input := range []string{`null`, `{"format":"clonoloom-project","version":2,"id":"p-x"}`, `{"format":"clonoloom-project","version":1,"id":"p-x","artifacts":null}`, `{"format":"clonoloom-project","version":1,"version":1,"id":"p-x"}`} {
		if _, _, _, err := checkpointProject([]byte(input)); err == nil {
			t.Fatal("accepted", input)
		}
	}
	if _, err = a.loadCheckpoint(context.Background(), "../escape"); err == nil {
		t.Fatal("traversal")
	}
}
func TestCheckpointDiffExactTokensAndCategories(t *testing.T) {
	before := json.RawMessage(`{"datasets":[{"sources":[{"sha256":"old"}]}],"definitions":[{"junction":"aa"}],"filters":[{"minCount":9007199254740992}],"samples":[{"condition":null}],"cohortDesigns":[{"members":["S1","S2"]}],"artifacts":[{"id":"a"}],"empty":null}`)
	after := json.RawMessage(`{"datasets":[{"sources":[{"sha256":"new"}]}],"definitions":[{"junction":"nt"}],"filters":[{"minCount":9007199254740993}],"samples":[{"condition":"A"}],"cohortDesigns":[{"members":["S2","S1"]}],"artifacts":[{"id":"b"}]}`)
	changes := []checkpointChange{}
	if err := checkpointDiff(before, after, "", &changes); err != nil {
		t.Fatal(err)
	}
	if len(changes) != 8 {
		t.Fatal(changes)
	}
	found := false
	for _, c := range changes {
		if c.Path == "/filters/0/minCount" {
			found = true
			if c.Before != "9007199254740992" || c.After != "9007199254740993" {
				t.Fatal("rounded", c)
			}
		}
		if c.Path == "/empty" && (c.Before != "null" || c.After != "(absent)") {
			t.Fatal(c)
		}
	}
	if !found {
		t.Fatal(changes)
	}
	changes = nil
	if err := checkpointDiff(json.RawMessage(`{"a":1,"b":2}`), json.RawMessage(`{"b":2,"a":1}`), "", &changes); err != nil || len(changes) != 0 {
		t.Fatal(changes, err)
	}
}
func TestCheckpointAPIContracts(t *testing.T) {
	a, project, id := checkpointFixture(t)
	h, err := a.handler()
	if err != nil {
		t.Fatal(err)
	}
	r := request(t, h, "POST", "/api/checkpoints", bytes.NewReader(mustJSON(map[string]any{"project": json.RawMessage(project), "name": "Checkpoint"})), nil)
	var record checkpointRecord
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &record) != nil {
		t.Fatal(r.Code, r.Body)
	}
	for _, op := range []string{"compare", "recover"} {
		result := request(t, h, "POST", "/api/checkpoints/"+record.ID+"/"+op, bytes.NewReader(mustJSON(map[string]any{"project": json.RawMessage(project)})), nil)
		if result.Code != 200 {
			t.Fatal(op, result.Code, result.Body)
		}
	}
	wrong := bytes.Replace(project, []byte(`"p-check"`), []byte(`"p-other"`), 1)
	result := request(t, h, "POST", "/api/checkpoints/"+record.ID+"/recover", bytes.NewReader(mustJSON(map[string]any{"project": json.RawMessage(wrong)})), nil)
	if result.Code != 409 {
		t.Fatal("cross-project recovery", result.Code)
	}
	// Unsupported/incomplete saved evidence cannot silently disappear from a checkpoint.
	m, err := a.artifacts.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	m.Files[0].Location = "source"
	os.WriteFile(filepath.Join(a.artifacts.Dir(), id, "manifest.json"), mustJSON(m), 0600)
	if _, err = a.createCheckpoint(context.Background(), project, "External output"); err == nil {
		t.Fatal("captured source path")
	}
}

func TestCheckpointBoundsCompatibilityAndPreflight(t *testing.T) {
	versions, err := os.ReadDir("validation/projects")
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range versions {
		if !v.IsDir() || !strings.HasPrefix(v.Name(), "0.") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join("validation/projects", v.Name(), "project.json"))
		if err != nil {
			t.Fatal(err)
		}
		if _, _, _, err = checkpointProject(raw); err != nil {
			t.Fatal(v.Name(), err)
		}
	}
	if _, _, _, err := checkpointProject(bytes.Repeat([]byte(" "), checkpointProjectLimit+1)); err == nil {
		t.Fatal("project bound")
	}
	refs := []map[string]string{}
	for i := 0; i < 129; i++ {
		refs = append(refs, map[string]string{"id": fmt.Sprintf("result-%d", i)})
	}
	if _, _, _, err := checkpointProject(mustJSON(map[string]any{"format": "clonoloom-project", "version": 1, "id": "p-bound", "artifacts": refs})); err == nil {
		t.Fatal("attached result bound")
	}
	before, after := make([]int, 4097), make([]int, 4097)
	for i := 0; i < 4096; i++ {
		after[i] = 1
	}
	changes := []checkpointChange{}
	if err := checkpointDiff(mustJSON(before), mustJSON(after), "", &changes); err != nil || len(changes) != 4096 {
		t.Fatal("exact difference bound", len(changes), err)
	}
	after[4096] = 1
	changes = nil
	if err := checkpointDiff(mustJSON(before), mustJSON(after), "", &changes); err == nil {
		t.Fatal("truncated differences")
	}
	a, project, first := checkpointFixture(t)
	second := certificateFixture(t, a, cohortFeaturesKind)
	var doc map[string]json.RawMessage
	json.Unmarshal(project, &doc)
	doc["artifacts"] = mustJSON([]map[string]string{{"id": first}, {"id": second}})
	record, err := a.createCheckpoint(context.Background(), mustJSON(doc), "Preflight all results")
	if err != nil {
		t.Fatal(err)
	}
	d, err := a.loadCheckpoint(context.Background(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{first, second}
	sort.Strings(ids)
	os.RemoveAll(filepath.Join(a.artifacts.Dir(), ids[0]))
	conflict := filepath.Join(a.artifacts.Dir(), ids[1], "manifest.json")
	os.WriteFile(conflict, []byte(`{"changed":true}`), 0600)
	if err = a.recoverCheckpoint(context.Background(), d); err == nil {
		t.Fatal("conflict not detected")
	}
	if _, err = os.Stat(filepath.Join(a.artifacts.Dir(), ids[0])); !os.IsNotExist(err) {
		t.Fatal("installed first result before checking later conflict")
	}
}
func FuzzCheckpointProject(f *testing.F) {
	f.Add([]byte(`{"format":"clonoloom-project","version":1,"id":"p-test","artifacts":[]}`))
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > checkpointProjectLimit {
			t.Skip()
		}
		_, _, _, _ = checkpointProject(raw)
	})
}
