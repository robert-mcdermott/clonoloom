package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"clonoloom/internal/clonotype"
	"clonoloom/internal/phenotype"
	"clonoloom/internal/pseudobulk"
	"clonoloom/internal/simulate"
	"clonoloom/internal/study"
)

func pseudobulkFixture(t *testing.T) (*app, http.Handler, pseudobulkRequest, string) {
	t.Helper()
	a, h := testApp(t)
	dir := filepath.Join(t.TempDir(), "study")
	sum, e := simulate.Run(context.Background(), simulate.Presets["pseudobulk-study"], dir, "validation", nil)
	if e != nil {
		t.Fatal(e)
	}
	paths := []string{}
	for _, f := range sum.Files {
		if !strings.Contains(f, "/") && (strings.HasSuffix(f, ".tsv") || strings.HasSuffix(f, ".airr.json")) {
			paths = append(paths, filepath.Join(dir, f))
		}
	}
	rec := postJSON(t, h, "/api/datasets/open", string(mustJSON(map[string]any{"paths": paths})))
	var ds datasetSummary
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &ds) != nil {
		t.Fatal(rec.Code, rec.Body)
	}
	dataset, _ := a.datasets.get(ds.ID)
	dataset.Credits = []credit{{Title: "Pseudobulk controls", License: "Fixture terms", LicenseClass: "terms", Citation: "Artificial count controls"}}
	req := pseudobulkRequest{Options: pseudobulk.Options{Annotation: "state", MinCells: 10}}
	for i, sample := range ds.Samples {
		path := filepath.Join(dir, "expression", sample.Name)
		rec = postJSON(t, h, "/api/expression/open", string(mustJSON(map[string]any{"path": path, "cellsPath": filepath.Join(path, "cells.tsv")})))
		var in expressionSummary
		if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &in) != nil {
			t.Fatal(rec.Code, rec.Body)
		}
		p := phenotypeRequest{ID: in.ID, Dataset: ds.ID, Sample: &i, Definition: clonotype.Definition{Chains: []string{"TRA", "TRB"}}, Annotation: study.Sample{Name: sample.Name, Subject: sample.Name[:3], Time: "visit " + sample.Name[4:]}, Options: phenotype.Options{Annotation: "state", Breakpoints: []int{1, 5, 10}}}
		rec = postJSON(t, h, "/api/phenotypes", string(mustJSON(p)))
		var started struct{ Artifact, Job string }
		if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &started) != nil {
			t.Fatal(rec.Code, rec.Body)
		}
		waitForJob(t, a, started.Job, 15*time.Second)
		req.Phenotypes = append(req.Phenotypes, started.Artifact)
	}
	return a, h, req, dir
}
func TestPseudobulkTruthReplayExports(t *testing.T) {
	a, h, req, dir := pseudobulkFixture(t)
	s, e := a.capturePseudobulk(context.Background(), req)
	if e != nil {
		t.Fatal(e)
	}
	r, e := pseudobulk.Compute(context.Background(), s.Input)
	if e != nil {
		t.Fatal(e)
	}
	if len(s.Input.Samples) != 8 || r.Subjects != 4 || len(r.Groups) != 96 {
		t.Fatal("accounting", r.Subjects, len(r.Groups))
	}
	// Independent literal truth and Matrix Market reader: never use the expression reader,
	// phenotype sizes or production aggregation to calculate expected counts.
	truth, e := os.ReadFile(filepath.Join(dir, "truth/observations.tsv"))
	if e != nil {
		t.Fatal(e)
	}
	rd := csv.NewReader(bytes.NewReader(truth))
	rd.Comma = '\t'
	rows, e := rd.ReadAll()
	if e != nil {
		t.Fatal(e)
	}
	sizes := map[[2]string]int{}
	clones := map[[2]string]string{}
	for _, v := range rows[1:] {
		if v[5] == "true" {
			sizes[[2]string{v[0], v[2]}]++
			clones[[2]string{v[0], v[1]}] = v[2]
		}
	}
	for si, sample := range s.Input.Samples {
		name := sample.Annotation.Name
		path := filepath.Join(dir, "expression", name)
		b, _ := os.ReadFile(filepath.Join(path, "barcodes.tsv"))
		barcodes := strings.Fields(string(b))
		ann, _ := os.ReadFile(filepath.Join(path, "cells.tsv"))
		ar := csv.NewReader(bytes.NewReader(ann))
		ar.Comma = '\t'
		labels, _ := ar.ReadAll()
		types := map[string]string{}
		for _, v := range labels[1:] {
			types[v[0]] = v[1]
		}
		counts := map[[2]string][]uint64{}
		cells := map[[2]string]int{}
		membership := map[int][2]string{}
		for ci, barcode := range barcodes {
			typ := types[barcode]
			clone := clones[[2]string{name, barcode}]
			if typ == "" || clone == "" {
				continue
			}
			n := sizes[[2]string{name, clone}]
			bin := ">10"
			switch {
			case n == 1:
				bin = "1"
			case n <= 5:
				bin = "2–5"
			case n <= 10:
				bin = "6–10"
			}
			key := [2]string{typ, bin}
			cells[key]++
			membership[ci] = key
			if counts[key] == nil {
				counts[key] = make([]uint64, 24)
			}
		}
		raw, _ := os.ReadFile(filepath.Join(path, "matrix.mtx"))
		lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
		for _, line := range lines[2:] {
			v := strings.Fields(line)
			fi, _ := strconv.Atoi(v[0])
			ci, _ := strconv.Atoi(v[1])
			count, _ := strconv.ParseUint(v[2], 10, 64)
			if key, ok := membership[ci-1]; ok {
				counts[key][fi-1] += count
			}
		}
		for _, g := range r.Groups {
			if g.Sample != si {
				continue
			}
			key := [2]string{g.Type, g.Bin}
			if g.Cells != cells[key] || string(mustJSON(g.Counts)) != string(mustJSON(counts[key])) {
				t.Fatalf("truth differs %s %v: cells %d/%d counts %v/%v", name, key, g.Cells, cells[key], g.Counts, counts[key])
			}
		}
		if r.Coverage[si].Included != 417 || r.Coverage[si].Excluded["Missing cell-type annotation"] != 2 || r.Coverage[si].Excluded["Unavailable unique clonotype size"] != 1 {
			t.Fatal("coverage", r.Coverage[si])
		}
	}
	reversed := req
	reversed.Phenotypes = append([]string{}, req.Phenotypes...)
	for i, j := 0, len(reversed.Phenotypes)-1; i < j; i, j = i+1, j-1 {
		reversed.Phenotypes[i], reversed.Phenotypes[j] = reversed.Phenotypes[j], reversed.Phenotypes[i]
	}
	s2, e := a.capturePseudobulk(context.Background(), reversed)
	if e != nil || s2.artifactID() != s.artifactID() {
		t.Fatal("order identity", e)
	}
	// Storage location is not scientific identity; captured source bytes are.
	var relocated pseudobulkSpec
	json.Unmarshal(mustJSON(s), &relocated)
	relocated.Parents[0].Input.Path = "/relocated"
	relocated.Parents[0].Input.Sources[0].Path = "/relocated/matrix"
	if relocated.artifactID() != s.artifactID() {
		t.Fatal("source path changed identity")
	}
	source := filepath.Join(dir, "expression", s.Input.Samples[0].Annotation.Name, "matrix.mtx")
	original, _ := os.ReadFile(source)
	if e = os.WriteFile(source, append(append([]byte{}, original...), '\n'), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = a.capturePseudobulk(context.Background(), req); e == nil {
		t.Fatal("changed source accepted")
	}
	if e = os.WriteFile(source, original, 0600); e != nil {
		t.Fatal(e)
	}
	response := postJSON(t, h, "/api/pseudobulk", string(mustJSON(req)))
	if response.Code != 200 {
		t.Fatal(response.Body)
	}
	var started struct {
		Artifact, Job string
		Ready         bool
	}
	json.Unmarshal(response.Body.Bytes(), &started)
	waitForJob(t, a, started.Job, 15*time.Second)
	response = postJSON(t, h, "/api/pseudobulk", string(mustJSON(req)))
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"ready":true`) {
		t.Fatal("reuse", response.Body)
	}
	var doc pseudobulkDocument
	m, _, e := a.readCheckedResult(started.Artifact, pseudobulkKind, &doc)
	if e != nil {
		t.Fatal(e)
	}
	before := mustJSON(doc)
	if string(mustJSON(doc.Analysis)) != string(mustJSON(r)) {
		t.Fatal("saved result differs")
	}
	for _, bad := range []string{`{}`, `{"extra":true}`, string(mustJSON(req)) + ` {}`, fmt.Sprintf(`{"phenotypes":[%q,%q],"options":{"annotation":"state"}}`, req.Phenotypes[0], req.Phenotypes[0])} {
		if response := postJSON(t, h, "/api/pseudobulk", bad); response.Code == 200 {
			t.Fatal("accepted", bad)
		}
	}
	if e = os.RemoveAll(dir); e != nil {
		t.Fatal(e)
	}
	// Replay uses captured raw counts, independent of source files and parent artifacts.
	for _, p := range s.Parents {
		path, _ := a.artifacts.Path(p.ID, "phenotypes.json")
		if e = os.Remove(path); e != nil {
			t.Fatal(e)
		}
	}
	resultPath, _ := a.artifacts.Path(m.ID, "pseudobulk.json")
	if e = os.RemoveAll(filepath.Dir(resultPath)); e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(m.Lineage.Job.Spec, &s); e != nil {
		t.Fatal(e)
	}
	work, e := a.jobs.Remake(pseudobulkKind, m.Lineage.Job.Spec)
	if e != nil {
		t.Fatal(e)
	}
	replay, _, e := a.jobs.Submit(pseudobulkKind, "Replay", started.Artifact+"-replay", s, work)
	if e != nil {
		t.Fatal(e)
	}
	waitForJob(t, a, replay.ID, 15*time.Second)
	var back pseudobulkDocument
	_, _, e = a.readCheckedResult(started.Artifact, pseudobulkKind, &back)
	if e != nil || !bytes.Equal(before, mustJSON(back)) {
		t.Fatal("source-free replay differs", e)
	}
	download := func(suffix string) *httptest.ResponseRecorder {
		return request(t, h, http.MethodGet, "/api/pseudobulk/"+started.Artifact+"/download"+suffix, nil, nil)
	}
	if v := download(""); v.Code != 409 {
		t.Fatal("terms", v.Code)
	}
	v := download("?accept=1")
	if v.Code != 200 {
		t.Fatal(v.Code, v.Body)
	}
	z, e := zip.NewReader(bytes.NewReader(v.Body.Bytes()), int64(v.Body.Len()))
	if e != nil {
		t.Fatal(e)
	}
	found := map[string]bool{}
	for _, f := range z.File {
		found[f.Name] = true
		stream, _ := f.Open()
		raw, _ := io.ReadAll(stream)
		stream.Close()
		if f.Name == "observations.csv" {
			rows, e := csv.NewReader(bytes.NewReader(raw)).ReadAll()
			if e != nil || len(rows) != 2305 {
				t.Fatal("complete CSV", len(rows), e)
			}
		}
	}
	for _, name := range []string{"pseudobulk.json", "manifest.json", "cells.json", "observations.csv", "NOTICE.txt"} {
		if !found[name] {
			t.Fatal("missing export", name)
		}
	}
	path, _ := a.artifacts.Path(m.ID, "pseudobulk.json")
	os.WriteFile(path, []byte("changed"), 0600)
	if v := download("?accept=1"); v.Code != 409 {
		t.Fatal("checksum", v.Code)
	}
}
