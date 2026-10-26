package main

import (
	"archive/zip"
	"bytes"
	"clonoloom/internal/clonotype"
	"clonoloom/internal/simulate"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func motifFixture(t *testing.T) (*app, http.Handler, motifsRequest, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "motif-study")
	sum, err := simulate.Run(context.Background(), simulate.Presets["motif-study"], dir, "validation", nil)
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{}
	for _, f := range sum.Files {
		if !strings.Contains(f, "/") && (strings.HasSuffix(f, ".tsv") || strings.HasSuffix(f, ".airr.json")) {
			paths = append(paths, filepath.Join(dir, f))
		}
	}
	a, h := testApp(t)
	r := postJSON(t, h, "/api/datasets/open", string(mustJSON(map[string]any{"paths": paths})))
	var ds datasetSummary
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &ds) != nil {
		t.Fatal(r.Code, r.Body)
	}
	body := motifsRequest{Dataset: ds.ID, Definition: clonotype.Definition{Junction: "aa", V: "gene", J: "gene"}, Subjects: map[string]string{}}
	for i := 1; i <= 12; i++ {
		s := fmt.Sprintf("S%02d", i)
		body.Subjects[s] = s
		if i <= 4 {
			body.Query = append(body.Query, s)
		} else {
			body.Background = append(body.Background, s)
		}
	}
	return a, h, body, dir
}
func TestMotifArtifactTruthAndReplay(t *testing.T) {
	a, h, body, dir := motifFixture(t)
	ds, _ := a.datasets.get(body.Dataset)
	ds.Credits = []credit{{Title: "Motif fixture", License: "Fixture terms", LicenseClass: "terms", Citation: "Fixture citation"}}
	spec, err := a.captureMotifs(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(spec.QueryRecords) != 108 || len(spec.BackgroundRecords) != 1000 {
		t.Fatal(len(spec.QueryRecords), len(spec.BackgroundRecords))
	}
	// Independently reconcile every captured unique unit and abundance with literal truth.
	raw, err := os.ReadFile(filepath.Join(dir, "truth/receptors.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	rd := csv.NewReader(bytes.NewReader(raw))
	rd.Comma = '\t'
	rows, err := rd.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	truth := map[string]float64{}
	for _, row := range rows[1:] {
		n, e := strconv.ParseFloat(row[5], 64)
		if e != nil {
			t.Fatal(e)
		}
		truth[row[2]+"/"+strings.Split(row[3], "*")[0]+"/"+row[4]] += n
	}
	for _, pair := range []struct {
		arm     string
		records bool
	}{{"query", true}, {"background", false}} {
		records := spec.BackgroundRecords
		if pair.records {
			records = spec.QueryRecords
		}
		for _, rec := range records {
			k := pair.arm + "/" + rec.V + "/" + rec.Junction
			if truth[k] != rec.Abundance {
				t.Fatal("truth mismatch", rec)
			}
			delete(truth, k)
		}
	}
	if len(truth) != 0 {
		t.Fatal("uncaptured truth", len(truth))
	}
	var start struct {
		Artifact, Job string
		Ready         bool
	}
	r := postJSON(t, h, "/api/motifs", string(mustJSON(body)))
	if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &start) != nil {
		t.Fatal(r.Code, r.Body)
	}
	waitForJob(t, a, start.Job, 30*time.Second)
	var doc motifsDocument
	m, _, err := a.readCheckedResult(start.Artifact, motifsKind, &doc)
	if err != nil {
		t.Fatal(err)
	}
	local, global := 0, 0
	for _, g := range doc.Result.Groups {
		if g.Reported {
			if g.Kind == "local" {
				local++
			} else {
				global++
			}
		}
	}
	if local == 0 || global == 0 {
		t.Fatal("positive control lost", local, global)
	}
	t.Logf("family %d, tested %d, reported local %d global %d", doc.Result.Family, doc.Result.Tested, local, global)
	r = postJSON(t, h, "/api/motifs", string(mustJSON(body)))
	if r.Code != 200 || !bytes.Contains(r.Body.Bytes(), []byte(`"ready":true`)) {
		t.Fatal(r.Code, r.Body)
	}
	var recorded motifsSpec
	if err = json.Unmarshal(m.Lineage.Job.Spec, &recorded); err != nil {
		t.Fatal(err)
	}
	work, err := a.jobs.Remake(motifsKind, m.Lineage.Job.Spec)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	path, _ := a.artifacts.Path(m.ID, "motifs.json")
	if err = os.RemoveAll(filepath.Dir(path)); err != nil {
		t.Fatal(err)
	}
	j, _, err := a.jobs.Submit(motifsKind, "Replay motifs", m.ID, recorded, work)
	if err != nil {
		t.Fatal(err)
	}
	waitForJob(t, a, j.ID, 30*time.Second)
	var back motifsDocument
	if _, _, err = a.readCheckedResult(m.ID, motifsKind, &back); err != nil || !bytes.Equal(mustJSON(doc), mustJSON(back)) {
		t.Fatal("replay changed", err)
	}
	response := request(t, h, http.MethodGet, "/api/motifs/"+m.ID+"/download", nil, nil)
	if response.Code != 409 {
		t.Fatal(response.Code)
	}
	response = request(t, h, http.MethodGet, "/api/motifs/"+m.ID+"/download?accept=1", nil, nil)
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body)
	}
	z, err := zip.NewReader(bytes.NewReader(response.Body.Bytes()), int64(response.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, f := range z.File {
		reader, e := f.Open()
		if e != nil {
			t.Fatal(e)
		}
		files[f.Name], e = io.ReadAll(reader)
		reader.Close()
		if e != nil {
			t.Fatal(e)
		}
	}
	for _, f := range []string{"motifs.json", "manifest.json", "signatures.csv", "receptors.csv", "NOTICE.txt"} {
		if len(files[f]) == 0 {
			t.Fatal("missing", f)
		}
	}
	if !strings.Contains(string(files["NOTICE.txt"]), "Fixture citation") {
		t.Fatal("terms lost")
	}
	g := doc.Result.Groups[0]
	response = request(t, h, http.MethodGet, "/api/motifs/"+m.ID+"/group/"+g.ID, nil, nil)
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body)
	}
	if err = os.WriteFile(path, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if response = request(t, h, http.MethodGet, "/api/motifs/"+m.ID, nil, nil); response.Code != 409 {
		t.Fatal("tampered artifact accepted")
	}
}
func TestMotifRequestRefusals(t *testing.T) {
	a, h, body, _ := motifFixture(t)
	s, err := a.captureMotifs(body)
	if err != nil || s.check() != nil {
		t.Fatal("captured evidence", err)
	}
	for _, mutate := range []func(*motifsSpec){
		func(s *motifsSpec) { s.Method++ },
		func(s *motifsSpec) { s.Options.MinK = 0 },
		func(s *motifsSpec) { s.QueryRecords[0].Samples = []string{s.Background[0]} },
		func(s *motifsSpec) { s.QueryRecords[0].Subjects = []string{"not-a-member"} },
		func(s *motifsSpec) { s.Background = append(s.Background, s.Query[0]) },
		func(s *motifsSpec) { s.QueryRecords[0].Clonotypes = nil },
	} {
		var changed motifsSpec
		if err := json.Unmarshal(mustJSON(s), &changed); err != nil {
			t.Fatal(err)
		}
		mutate(&changed)
		if err := changed.check(); err == nil {
			t.Fatal("inconsistent captured evidence accepted")
		}
	}
	for _, mutate := range []func(*motifsRequest){func(b *motifsRequest) { b.Background = b.Query }, func(b *motifsRequest) { b.Query = []string{"missing"} }, func(b *motifsRequest) { b.Definition.Junction = "nt" }, func(b *motifsRequest) { b.Definition.V = "none" }, func(b *motifsRequest) { b.Subjects = map[string]string{"missing": "subject"} }} {
		b := body
		mutate(&b)
		if _, err := a.captureMotifs(b); err == nil {
			t.Fatal("invalid request accepted", b)
		}
	}
	if r := postJSON(t, h, "/api/motifs", `{"unknown":1}`); r.Code != 400 {
		t.Fatal(r.Code, r.Body)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := a.motifsWork(motifsSpec{})(ctx, nil); err != context.Canceled {
		t.Fatal(err)
	}
}
