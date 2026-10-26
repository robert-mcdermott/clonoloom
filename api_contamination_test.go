package main

import (
	"archive/zip"
	"bytes"
	"clonoloom/internal/contamination"
	"clonoloom/internal/simulate"
	"context"
	"encoding/csv"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func contaminationFixture(t *testing.T) (*app, http.Handler, contaminationRequest, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "study")
	sum, e := simulate.Run(context.Background(), simulate.Presets["contamination-study"], dir, "validation", nil)
	if e != nil {
		t.Fatal(e)
	}
	paths := []string{}
	for _, f := range sum.Files {
		if !strings.Contains(f, "/") && (strings.HasSuffix(f, ".tsv") || strings.HasSuffix(f, ".airr.json")) {
			paths = append(paths, filepath.Join(dir, f))
		}
	}
	a, h := testApp(t)
	resp := postJSON(t, h, "/api/datasets/open", string(mustJSON(map[string]any{"paths": paths})))
	var ds datasetSummary
	if resp.Code != 200 || json.Unmarshal(resp.Body.Bytes(), &ds) != nil {
		t.Fatal(resp.Code, resp.Body)
	}
	dataset, _ := a.datasets.get(ds.ID)
	dataset.Credits = []credit{{Title: "Contamination fixture", License: "Fixture terms", LicenseClass: "terms", Citation: "Fixture source"}}
	var samples []contamination.Sample
	raw, _ := os.ReadFile(filepath.Join(dir, "truth/samples.json"))
	if e = json.Unmarshal(raw, &samples); e != nil {
		t.Fatal(e)
	}
	pgen := postJSON(t, h, "/api/pgen", string(mustJSON(map[string]any{"dataset": ds.ID, "definition": contaminationDefinition(), "pgen": map[string]any{"models": "clonoloom", "organism": "human", "genes": false}})))
	var start struct{ Artifact, Job string }
	if pgen.Code != 200 || json.Unmarshal(pgen.Body.Bytes(), &start) != nil {
		t.Fatal(pgen.Code, pgen.Body)
	}
	waitForJob(t, a, start.Job, 120*time.Second)
	req := contaminationRequest{Dataset: ds.ID, Species: "human", Unrelated: true, Samples: samples, Pgen: start.Artifact}
	return a, h, req, dir
}
func TestContaminationTruthReplayAndExports(t *testing.T) {
	a, h, req, dir := contaminationFixture(t)
	s, err := a.captureContamination(req)
	if err != nil {
		t.Fatal(err)
	}
	r, err := contamination.Compute(context.Background(), s.Input)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Evidence.Samples) != 9 || len(r.Pairs) != 35 || r.SameSubjectPairs != 1 {
		t.Fatal("subject pairs", r)
	}

	// Independently join every literal truth record by V/J gene and nucleotide
	// sequence, before comparing every captured pair and abundance denominator.
	truthBytes, err := os.ReadFile(filepath.Join(dir, "truth/observations.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	rd := csv.NewReader(bytes.NewReader(truthBytes))
	rd.Comma = '\t'
	truthRows, err := rd.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]map[string]float64{}
	totals := map[string]float64{}
	for _, row := range truthRows[1:] {
		sample := row[0]
		if counts[sample] == nil {
			counts[sample] = map[string]float64{}
		}
		n, e := strconv.ParseFloat(row[6], 64)
		if e != nil {
			t.Fatal(e)
		}
		key := strings.Split(row[3], "*")[0] + "/" + strings.Split(row[4], "*")[0] + "/" + row[5]
		counts[sample][key] += n
		totals[sample] += n
	}
	for _, p := range r.Pairs {
		an, bn := r.Evidence.Samples[p.A].Sample.Name, r.Evidence.Samples[p.B].Sample.Name
		shared := 0
		aa, bb := 0.0, 0.0
		for key, amount := range counts[an] {
			if other := counts[bn][key]; other > 0 {
				shared++
				aa += amount
				bb += other
			}
		}
		if p.Shared != shared || p.SharedA != aa || p.SharedB != bb || r.Evidence.Samples[p.A].Total != totals[an] || r.Evidence.Samples[p.B].Total != totals[bn] {
			t.Fatal("literal truth pair", p)
		}
	}
	// Literal truth controls: D01 and D02 share three transfers and three controls.
	pair := -1
	for i, p := range r.Pairs {
		if r.Evidence.Samples[p.A].Sample.Name == "D01-1" && r.Evidence.Samples[p.B].Sample.Name == "D02-1" {
			pair = i
		}
	}
	if pair < 0 || r.Pairs[pair].Shared != 6 {
		t.Fatal("planted shared identities", pair)
	}
	positive := 0
	for _, m := range r.Matches {
		if m.A == r.Pairs[pair].A && m.B == r.Pairs[pair].B && m.Pattern == "Asymmetric observed fractions" {
			positive++
		}
	}
	if positive != 3 {
		t.Fatal("planted asymmetric controls", positive)
	}
	resp := postJSON(t, h, "/api/contamination", string(mustJSON(req)))
	var start struct {
		Artifact, Job string
		Ready         bool
	}
	if resp.Code != 200 || json.Unmarshal(resp.Body.Bytes(), &start) != nil {
		t.Fatal(resp.Code, resp.Body)
	}
	waitForJob(t, a, start.Job, 60*time.Second)
	var doc contaminationDocument
	m, _, err := a.readCheckedResult(start.Artifact, contaminationKind, &doc)
	if err != nil {
		t.Fatal(err)
	}
	resp = postJSON(t, h, "/api/contamination", string(mustJSON(req)))
	if resp.Code != 200 || !strings.Contains(resp.Body.String(), `"ready":true`) {
		t.Fatal("ready reuse", resp.Body)
	}
	// Parent tampering is refused on new capture; replay uses its recorded snapshot.
	parentPath, _ := a.artifacts.Path(req.Pgen, "pgen.json")
	parentBytes, err := os.ReadFile(parentPath)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(parentPath, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = a.captureContamination(req); err == nil {
		t.Fatal("tampered parent accepted")
	}
	if err = os.WriteFile(parentPath, parentBytes, 0600); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*contaminationRequest){func(r *contaminationRequest) { r.Unrelated = false }, func(r *contaminationRequest) { r.Samples[0].Name = "unknown" }, func(r *contaminationRequest) { r.Species = "mouse" }} {
		var changed contaminationRequest
		json.Unmarshal(mustJSON(req), &changed)
		mutate(&changed)
		if _, err = a.captureContamination(changed); err == nil {
			t.Fatal("invalid request accepted")
		}
	}

	if rr := postJSON(t, h, "/api/contamination", `{"unknown":true}`); rr.Code != 400 {
		t.Fatal(rr.Code)
	}
	if err = os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	// Recreate through the registered durable handler with both parent and sources absent.
	if err = os.RemoveAll(filepath.Dir(parentPath)); err != nil {
		t.Fatal(err)
	}
	resultPath, _ := a.artifacts.Path(m.ID, "contamination.json")
	if err = os.RemoveAll(filepath.Dir(resultPath)); err != nil {
		t.Fatal(err)
	}
	work, err := a.jobs.Remake(contaminationKind, m.Lineage.Job.Spec)
	if err != nil {
		t.Fatal(err)
	}
	var recorded contaminationSpec
	if err = json.Unmarshal(m.Lineage.Job.Spec, &recorded); err != nil {
		t.Fatal(err)
	}
	j, _, err := a.jobs.Submit(contaminationKind, "Replay contamination", m.ID, recorded, work)
	if err != nil {
		t.Fatal(err)
	}
	waitForJob(t, a, j.ID, 60*time.Second)
	var back contaminationDocument
	if _, _, err = a.readCheckedResult(m.ID, contaminationKind, &back); err != nil || !bytes.Equal(mustJSON(doc), mustJSON(back)) {
		t.Fatal("replay drift", err)
	}
	response := request(t, h, http.MethodGet, "/api/contamination/"+m.ID+"/download", nil, nil)
	if response.Code != 409 {
		t.Fatal("provider terms", response.Code)
	}
	response = request(t, h, http.MethodGet, "/api/contamination/"+m.ID+"/download?accept=1", nil, nil)
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body)
	}
	z, err := zip.NewReader(bytes.NewReader(response.Body.Bytes()), int64(response.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, f := range z.File {
		rd, e := f.Open()
		if e != nil {
			t.Fatal(e)
		}
		files[f.Name], e = io.ReadAll(rd)
		rd.Close()
		if e != nil {
			t.Fatal(e)
		}
	}
	for _, name := range []string{"contamination.json", "manifest.json", "samples.json", "matches.csv", "pairs.csv", "NOTICE.txt"} {
		if len(files[name]) == 0 {
			t.Fatal(name)
		}
	}
	table, err := csv.NewReader(bytes.NewReader(files["pairs.csv"])).ReadAll()
	if err != nil || len(table) != 36 {
		t.Fatal(err, len(table))
	}
	if !strings.Contains(string(files["NOTICE.txt"]), "Fixture source") {
		t.Fatal("terms lost")
	}
	path, _ := a.artifacts.Path(m.ID, "contamination.json")
	os.WriteFile(path, []byte("{}"), 0600)
	if rr := request(t, h, http.MethodGet, "/api/contamination/"+m.ID, nil, nil); rr.Code != 409 {
		t.Fatal("checksum accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if e := a.contaminationWork(s)(ctx, nil); e != context.Canceled {
		t.Fatal(e)
	}
	bad := s
	bad.PgenSHA256 = "changed"
	if bad.check() == nil {
		t.Fatal("changed capture provenance")
	}
}

func TestContaminationCaptureRefusals(t *testing.T) {
	a, _, req, _ := contaminationFixture(t)
	changed := req
	changed.Filter.MinCount = 1
	if _, e := a.captureContamination(changed); e == nil {
		t.Fatal("Pgen QC mismatch accepted")
	}
	changed = req
	changed.Pgen = ""
	s, e := a.captureContamination(changed)
	if e != nil {
		t.Fatal(e)
	}
	for _, r := range s.Input.Receptors {
		if !math.IsNaN(float64(r.Pgen)) {
			t.Fatal("absent Pgen treated as low")
		}
	}
	ds, _ := a.datasets.get(req.Dataset)
	table := ds.Result.Table
	original := table.Count[0]
	table.Count[0] = math.NaN()
	if _, e = a.captureContamination(changed); e == nil {
		t.Fatal("missing counts accepted")
	}
	table.Count[0] = original
	originalNT := table.Junction[0]
	table.Junction[0] = 0
	incomplete, e := a.captureContamination(changed)
	table.Junction[0] = originalNT
	if e != nil {
		t.Fatal(e)
	}
	total, eligible := 0.0, 0.0
	for _, d := range incomplete.Input.Samples {
		total += d.Total
		eligible += d.Eligible
	}
	if total-eligible != original {
		t.Fatal("unevaluable sequence removed from denominator", total, eligible)
	}
	originalCells := table.Cell
	table.Cell = []uint32{}
	if _, e = a.captureContamination(changed); e == nil {
		t.Fatal("single-cell source accepted")
	}
	table.Cell = originalCells
	s.Input.Receptors[0].Pgen = 1e-5
	if s.check() == nil {
		t.Fatal("Pgen without captured parent")
	}
}
