package main

import (
	"archive/zip"
	"bytes"
	"clonoloom/internal/batchqc"
	"clonoloom/internal/rep"
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

func batchqcFixture(t *testing.T) (*app, http.Handler, batchqcRequest, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "study")
	sum, e := simulate.Run(context.Background(), simulate.Presets["batch-qc-study"], dir, "validation", nil)
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
	dataset.Credits = []credit{{Title: "Batch QC fixture", License: "Fixture terms", LicenseClass: "terms", Citation: "Fixture source"}}
	var samples []batchqc.Assignment
	raw, _ := os.ReadFile(filepath.Join(dir, "truth/samples.json"))
	if e = json.Unmarshal(raw, &samples); e != nil {
		t.Fatal(e)
	}
	req := batchqcRequest{Dataset: ds.ID, Samples: samples}
	return a, h, req, dir
}
func TestBatchQCTruthReplayExports(t *testing.T) {
	a, h, req, dir := batchqcFixture(t)
	s, e := a.captureBatchQC(req)
	if e != nil {
		t.Fatal(e)
	}
	r, e := batchqc.Compute(context.Background(), s.Input)
	if e != nil {
		t.Fatal(e)
	}
	if r.Included != 24 || r.Excluded != 2 || r.Subjects != 24 || len(r.Groups) != 24 {
		t.Fatal("accounting", r)
	}
	reordered := req
	reordered.Samples = append([]batchqc.Assignment{}, req.Samples...)
	for i, j := 0, len(reordered.Samples)-1; i < j; i, j = i+1, j-1 {
		reordered.Samples[i], reordered.Samples[j] = reordered.Samples[j], reordered.Samples[i]
	}
	canonical, e := a.captureBatchQC(reordered)
	if e != nil || canonical.artifactID() != s.artifactID() {
		t.Fatal("assignment order changed identity", e)
	}
	// Independently sum every literal generating row; no production QC helpers used.
	raw, _ := os.ReadFile(filepath.Join(dir, "truth/observations.tsv"))
	rd := csv.NewReader(bytes.NewReader(raw))
	rd.Comma = '\t'
	rows, e := rd.ReadAll()
	if e != nil {
		t.Fatal(e)
	}
	type tally struct {
		n                          int
		total, prod, unknown, v, j float64
		bad, badProd               bool
	}
	truth := map[string]*tally{}
	for _, row := range rows[1:] {
		p := truth[row[0]]
		if p == nil {
			p = &tally{}
			truth[row[0]] = p
		}
		p.n++
		n, e := strconv.ParseFloat(row[7], 64)
		if e != nil {
			p.bad = true
			if row[4] == "T" {
				p.badProd = true
			}
			continue
		}
		p.total += n
		if row[4] == "" {
			p.unknown += n
		}
		if row[4] == "T" {
			p.prod += n
			if row[5] != "" {
				p.v += n
			}
			if row[6] != "" {
				p.j += n
			}
		}
	}
	for _, sample := range s.Input.Samples {
		p := truth[sample.Assignment.Name]
		if sample.Rows != p.n || sample.Kept != p.n {
			t.Fatal("rows", sample)
		}
		values := []float64{p.total, float64(p.n), p.prod / p.total, p.unknown / p.total, p.v / p.prod, p.j / p.prod}
		for i, want := range values {
			got := float64(sample.Values[i].Value)
			missing := (p.bad && (i == 0 || i == 2 || i == 3)) || (p.badProd && (i == 4 || i == 5))
			if missing {
				if !math.IsNaN(got) || sample.Values[i].Reason == "" {
					t.Fatal("missing hidden", sample.Assignment.Name, i)
				}
			} else if math.Abs(got-want) > 1e-14 {
				t.Fatal("truth metric", sample.Assignment.Name, i, got, want)
			}
		}
	}
	resp := postJSON(t, h, "/api/batch-qc", string(mustJSON(req)))
	var start struct{ Artifact, Job string }
	if resp.Code != 200 || json.Unmarshal(resp.Body.Bytes(), &start) != nil {
		t.Fatal(resp.Code, resp.Body)
	}
	waitForJob(t, a, start.Job, 60*time.Second)
	var doc batchqcDocument
	m, _, e := a.readCheckedResult(start.Artifact, batchqcKind, &doc)
	if e != nil {
		t.Fatal(e)
	}
	if rr := postJSON(t, h, "/api/batch-qc", string(mustJSON(req))); rr.Code != 200 || !strings.Contains(rr.Body.String(), `"ready":true`) {
		t.Fatal("ready reuse", rr.Body)
	}
	for _, body := range []string{`{"unknown":true}`, `{} {}`} {
		if rr := postJSON(t, h, "/api/batch-qc", body); rr.Code != 400 {
			t.Fatal(rr.Code)
		}
	}
	for _, mutate := range []func(*batchqcRequest){func(q *batchqcRequest) { q.Samples = q.Samples[1:] }, func(q *batchqcRequest) { q.Samples[0].Name = "absent" }, func(q *batchqcRequest) { q.Samples[1].Subject = q.Samples[0].Subject }, func(q *batchqcRequest) { q.Samples[0].Batch = "" }, func(q *batchqcRequest) { q.Options.MinSamples = 2 }} {
		var v batchqcRequest
		json.Unmarshal(mustJSON(req), &v)
		mutate(&v)
		if _, e = a.captureBatchQC(v); e == nil {
			t.Fatal("invalid capture accepted")
		}
	}
	os.RemoveAll(dir)
	resultPath, _ := a.artifacts.Path(m.ID, "batch-qc.json")
	os.RemoveAll(filepath.Dir(resultPath))
	work, e := a.jobs.Remake(batchqcKind, m.Lineage.Job.Spec)
	if e != nil {
		t.Fatal(e)
	}
	var recorded batchqcSpec
	json.Unmarshal(m.Lineage.Job.Spec, &recorded)
	job, _, e := a.jobs.Submit(batchqcKind, "Replay batch QC", m.ID, recorded, work)
	if e != nil {
		t.Fatal(e)
	}
	waitForJob(t, a, job.ID, 60*time.Second)
	var back batchqcDocument
	if _, _, e = a.readCheckedResult(m.ID, batchqcKind, &back); e != nil || !bytes.Equal(mustJSON(doc), mustJSON(back)) {
		t.Fatal("replay drift", e)
	}
	response := request(t, h, http.MethodGet, "/api/batch-qc/"+m.ID+"/download", nil, nil)
	if response.Code != 409 {
		t.Fatal("provider terms", response.Code)
	}
	response = request(t, h, http.MethodGet, "/api/batch-qc/"+m.ID+"/download?accept=1", nil, nil)
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body)
	}
	z, e := zip.NewReader(bytes.NewReader(response.Body.Bytes()), int64(response.Body.Len()))
	if e != nil {
		t.Fatal(e)
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
	for _, name := range []string{"batch-qc.json", "manifest.json", "samples.json", "observations.csv", "NOTICE.txt"} {
		if len(files[name]) == 0 {
			t.Fatal(name)
		}
	}
	table, e := csv.NewReader(bytes.NewReader(files["observations.csv"])).ReadAll()
	if e != nil || len(table) != 145 {
		t.Fatal("complete observations", e, len(table))
	}
	if !strings.Contains(string(files["NOTICE.txt"]), "Fixture source") {
		t.Fatal("credits lost")
	}
	path, _ := a.artifacts.Path(m.ID, "batch-qc.json")
	os.WriteFile(path, []byte("{}"), 0600)
	if rr := request(t, h, http.MethodGet, "/api/batch-qc/"+m.ID, nil, nil); rr.Code != 409 {
		t.Fatal("checksum accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if e := a.batchqcWork(s)(ctx, nil); e != context.Canceled {
		t.Fatal(e)
	}
}
func TestBatchQCCaptureUniverses(t *testing.T) {
	a, _, req, _ := batchqcFixture(t)
	original, e := a.captureBatchQC(req)
	if e != nil {
		t.Fatal(e)
	}
	req.Filter.ProductiveOnly = true
	filtered, e := a.captureBatchQC(req)
	if e != nil {
		t.Fatal(e)
	}
	for i, s := range filtered.Input.Samples {
		if !bytes.Equal(mustJSON(s.Values[2:4]), mustJSON(original.Input.Samples[i].Values[2:4])) {
			t.Fatal("productive rule changed status universe")
		}
		if s.Kept >= s.Rows {
			t.Fatal("filter not applied")
		}
	}
	ds, _ := a.datasets.get(req.Dataset)
	req.Filter.ProductiveOnly = false
	table := ds.Result.Table
	unit := table.Unit
	table.Unit = rep.UnitUnknown
	if _, e = a.captureBatchQC(req); e == nil {
		t.Fatal("unknown unit accepted")
	}
	table.Unit = unit
	originalCount := table.Count[0]
	table.Count[0] = 1e16
	if _, e = a.captureBatchQC(req); e == nil {
		t.Fatal("inexact integer abundance accepted")
	}
	table.Count[0] = originalCount
	table.Cell = make([]uint32, table.Len())
	for i := range table.Cell {
		table.Cell[i] = uint32(i + 1)
	}
	cells, e := a.captureBatchQC(req)
	if e != nil {
		t.Fatal(e)
	}
	if !cells.Input.SingleCell || cells.Input.Unit != "cells" {
		t.Fatal("cell units")
	}
	for _, s := range cells.Input.Samples {
		if float64(s.Values[0].Value) != float64(s.Kept) {
			t.Fatal("cell denominator", s)
		}
	}
	// Repeated contigs at the same barcode cannot inflate retained cell abundance.
	table.Cell[1] = table.Cell[0]
	cells, e = a.captureBatchQC(req)
	if e != nil {
		t.Fatal(e)
	}
	if float64(cells.Input.Samples[0].Values[0].Value) != float64(cells.Input.Samples[0].Kept-1) {
		t.Fatal("barcodes counted as contigs")
	}
	table.Cell[0] = 0
	cells, e = a.captureBatchQC(req)
	if e != nil {
		t.Fatal(e)
	}
	if !math.IsNaN(float64(cells.Input.Samples[0].Values[0].Value)) {
		t.Fatal("missing barcode hidden")
	}
}
