package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"clonoloom/internal/artifacts"
	"clonoloom/internal/clonotype"
	"clonoloom/internal/export"
	"clonoloom/internal/hdf5"
	"clonoloom/internal/phenotype"
	"clonoloom/internal/study"
)

func annDataParentFixture(t *testing.T, a *app, h http.Handler) (string, string) {
	t.Helper()
	dir := t.TempDir()
	if err := expressionInputExample().Make(context.Background(), dir, nil); err != nil {
		t.Fatal(err)
	}
	cells := filepath.Join(dir, "cells.tsv")
	if err := os.WriteFile(cells, []byte("barcode\tstate\tmap_x\tmap_y\nAA-1\tA\t0\t1\nBB-1\t\t2\t3\nCC-1\tMissing annotation\t\t\nDD-1\tB\t4\t5\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var ds datasetSummary
	var in expressionSummary
	r := postJSON(t, h, "/api/datasets/open", string(mustJSON(map[string]any{"paths": []string{filepath.Join(dir, "receptors.tsv")}})))
	if r.Code != 200 {
		t.Fatal(r.Body)
	}
	json.Unmarshal(r.Body.Bytes(), &ds)
	r = postJSON(t, h, "/api/expression/open", string(mustJSON(map[string]any{"path": dir, "cellsPath": cells})))
	if r.Code != 200 {
		t.Fatal(r.Body)
	}
	json.Unmarshal(r.Body.Bytes(), &in)
	sample := 0
	req := phenotypeRequest{ID: in.ID, Dataset: ds.ID, Sample: &sample, Definition: clonotype.Definition{Chains: []string{"TRA", "TRB"}}, Annotation: study.Sample{Name: ds.Samples[0].Name, Subject: "P1", Time: "day 0"}, Options: phenotype.Options{Annotation: "state"}}
	r = postJSON(t, h, "/api/phenotypes", string(mustJSON(req)))
	if r.Code != 200 {
		t.Fatal(r.Body)
	}
	var s struct{ Artifact, Job string }
	json.Unmarshal(r.Body.Bytes(), &s)
	waitForJob(t, a, s.Job, 10*time.Second)
	return s.Artifact, dir
}
func TestAnnDataArtifactWorkflow(t *testing.T) {
	a, h := testApp(t)
	parent, dir := annDataParentFixture(t, a, h)
	for _, bad := range []string{`{}`, `{"phenotypes":"x","receptors":[]}`, `{"phenotypes":"x"} {}`} {
		if r := postJSON(t, h, "/api/anndata-export", bad); r.Code != 400 {
			t.Fatal(r.Code, r.Body)
		}
	}
	start := func(start bool) struct {
		Artifact, Job string
		Ready         bool
	} {
		t.Helper()
		r := postJSON(t, h, "/api/anndata-export", string(mustJSON(map[string]any{"phenotypes": parent, "start": start})))
		if r.Code != 200 {
			t.Fatal(r.Code, r.Body)
		}
		var s struct {
			Artifact, Job string
			Ready         bool
		}
		json.Unmarshal(r.Body.Bytes(), &s)
		return s
	}
	if s := start(false); s.Ready || s.Job != "" {
		t.Fatal("lookup submitted work")
	}
	s := start(true)
	waitForJob(t, a, s.Job, 10*time.Second)
	var answer struct {
		Artifact artifacts.Manifest
		Result   annDataDocument
	}
	if err := json.Unmarshal(request2(t, h, "/api/anndata-export/"+s.Artifact), &answer); err != nil {
		t.Fatal(err)
	}
	if answer.Result.Cells != 4 || answer.Result.Features != 3 || len(answer.Result.Embeddings) != 1 || answer.Result.Embeddings[0] != "X_supplied" || len(answer.Artifact.Lineage.Parents) != 1 {
		t.Fatalf("%+v", answer.Result)
	}
	var spec annDataSpec
	json.Unmarshal(answer.Artifact.Lineage.Job.Spec, &spec)
	if len(spec.Receptors) == 0 || spec.PhenotypesSHA256 == "" {
		t.Fatal("missing receptor snapshot or parent pin")
	}
	path, _ := a.artifacts.Path(s.Artifact, "cells.h5ad")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	f, err := hdf5.Open(context.Background(), bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	value, err := f.Value("/uns/clonoloom/phenotypes_json", 1)
	if err != nil || !strings.Contains(value.Text[0], `"subject":"P1"`) || !strings.Contains(value.Text[0], `"timepoint":"day 0"`) {
		t.Fatal("missing sample provenance", err)
	}
	// Saved exports are portable: no original expression or receptor source needed to download.
	os.RemoveAll(dir)
	raw := request2(t, h, "/api/anndata-export/"+s.Artifact+"/download")
	z, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, f := range z.File {
		r, e := f.Open()
		if e != nil {
			t.Fatal(e)
		}
		files[f.Name], e = io.ReadAll(r)
		r.Close()
		if e != nil {
			t.Fatal(e)
		}
	}
	if len(files["cells.h5ad"]) == 0 || !bytes.Contains(files["NOTICE.txt"], []byte("raw")) || len(files["manifest.json"]) == 0 {
		t.Fatal("incomplete download")
	}
	if !start(true).Ready {
		t.Fatal("cached export needs inputs")
	}
	// The saved file, not a regenerated result, is checked on every download.
	if err = os.WriteFile(path, []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	r := request(t, h, "GET", "/api/anndata-export/"+s.Artifact+"/download", nil, nil)
	if r.Code != 409 {
		t.Fatal("served corrupted export", r.Code)
	}
	spec.Method++
	if _, err := a.jobs.Remake(annDataKind, mustJSON(spec)); err == nil {
		t.Fatal("resumed old method")
	}
}
func TestAnnDataChangedSourcesAndParents(t *testing.T) {
	for _, which := range []string{"counts", "parent"} {
		t.Run(which, func(t *testing.T) {
			a, h := testApp(t)
			parent, dir := annDataParentFixture(t, a, h)
			if which == "counts" {
				f, err := os.OpenFile(filepath.Join(dir, "matrix.mtx"), os.O_APPEND|os.O_WRONLY, 0600)
				if err != nil {
					t.Fatal(err)
				}
				f.WriteString("\n")
				f.Close()
			} else {
				path, _ := a.artifacts.Path(parent, phenotypeKind+".json")
				os.WriteFile(path, []byte("{}"), 0600)
			}
			r := postJSON(t, h, "/api/anndata-export", string(mustJSON(map[string]any{"phenotypes": parent})))
			if which == "parent" {
				if r.Code != 409 {
					t.Fatal(r.Code, r.Body)
				}
				return
			}
			if r.Code != 200 {
				t.Fatal(r.Code, r.Body)
			}
			var s struct{ Artifact, Job string }
			json.Unmarshal(r.Body.Bytes(), &s)
			deadline := time.Now().Add(10 * time.Second)
			for {
				j, _ := a.jobs.Get(s.Job)
				if j.State.Finished() {
					if !strings.Contains(j.Error, "expression sources changed") {
						t.Fatal(j.Error)
					}
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("job did not finish")
				}
				time.Sleep(10 * time.Millisecond)
			}
			if _, err := a.artifacts.Get(s.Artifact); err == nil {
				t.Fatal("published changed input")
			}
		})
	}
}
func TestAnnDataRestrictedDownload(t *testing.T) {
	a, h := testApp(t)
	parent, _ := annDataParentFixture(t, a, h)
	var p phenotypeDocument
	a.readCheckedResult(parent, phenotypeKind, &p)
	ds, _ := a.datasets.get(p.Dataset)
	// Use a saved notice fixture to exercise the same terms gate as other exports.
	r := postJSON(t, h, "/api/anndata-export", string(mustJSON(map[string]any{"phenotypes": parent})))
	var s struct{ Artifact, Job string }
	json.Unmarshal(r.Body.Bytes(), &s)
	waitForJob(t, a, s.Job, 10*time.Second)
	var doc annDataDocument
	m, _, err := a.readCheckedResult(s.Artifact, annDataKind, &doc)
	if err != nil {
		t.Fatal(err)
	}
	doc.Notice.Credits = []export.Credit{{Title: ds.Name, LicenseClass: "terms", License: "Personal study only"}}
	id := "h5ad-restricted-fixture"
	dir, err := a.artifacts.Stage(id)
	if err != nil {
		t.Fatal(err)
	}
	source, _ := a.artifacts.Path(s.Artifact, "cells.h5ad")
	raw, _ := os.ReadFile(source)
	os.WriteFile(filepath.Join(dir, "cells.h5ad"), raw, 0600)
	os.WriteFile(filepath.Join(dir, annDataKind+".json"), mustJSON(doc), 0600)
	m.ID = id
	m.Files = []artifacts.File{{Path: "cells.h5ad"}, {Path: annDataKind + ".json"}}
	if _, err = a.artifacts.Promote(m, nil); err != nil {
		t.Fatal(err)
	}
	if r := request(t, h, "GET", "/api/anndata-export/"+id+"/download", nil, nil); r.Code != 409 {
		t.Fatal(r.Code)
	}
	request2(t, h, "/api/anndata-export/"+id+"/download?accept=1")
}
