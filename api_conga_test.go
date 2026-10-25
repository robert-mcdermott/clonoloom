package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"clonoloom/internal/artifacts"
	"clonoloom/internal/clonotype"
	"clonoloom/internal/conga"
	"clonoloom/internal/expression"
	"clonoloom/internal/phenotype"
	"clonoloom/internal/study"
)

func congaParentsFixture(t *testing.T, a *app, h http.Handler) (string, string, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "receptors.tsv")
	var b strings.Builder
	b.WriteString("sequence_id\tcell_id\tproductive\tlocus\tv_call\tj_call\tjunction_aa\n")
	for i, m := range []string{"AAAA", "AAAS", "AASS", "CCCC", "CCCT", "CCTT"} {
		fmt.Fprintf(&b, "a%d\tb%d\tT\tTRA\tTRAV1\tTRAJ1\tCAV%sAF\nb%d\tb%d\tT\tTRB\tTRBV1\tTRBJ1\tCAS%sTF\n", i, i, m, i, i, m)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0600); err != nil {
		t.Fatal(err)
	}
	rec := postJSON(t, h, "/api/datasets/open", string(mustJSON(map[string]any{"paths": []string{path}})))
	if rec.Code != 200 {
		t.Fatal(rec.Body)
	}
	var ds datasetSummary
	json.Unmarshal(rec.Body.Bytes(), &ds)
	d, _ := clonotype.Presets[3].Normalize()
	dataset, entry, _, err := a.clonotypeTable(clonotypeRequest{Dataset: ds.ID, Definition: d})
	if err != nil {
		t.Fatal(err)
	}
	pc := []phenotype.Cell{}
	ec := []expression.AnalysisCell{}
	for i, c := range entry.table.Clonotypes {
		barcode := fmt.Sprint("b", i)
		pc = append(pc, phenotype.Cell{Barcode: barcode, Clone: c.ID, Status: "Assigned", Chains: "Paired"})
		ec = append(ec, expression.AnalysisCell{Barcode: barcode, Scores: []float64{float64(i), 0}})
	}
	save := func(id, kind string, doc any, parents []artifacts.Parent) string {
		dir, err := a.artifacts.Stage(id)
		if err != nil {
			t.Fatal(err)
		}
		file := kind + ".json"
		if err = os.WriteFile(filepath.Join(dir, file), mustJSON(doc), 0600); err != nil {
			t.Fatal(err)
		}
		m, err := a.artifacts.Promote(artifacts.Manifest{ID: id, Kind: kind, Name: "Comparison fixture", Lineage: artifacts.Lineage{Parents: parents}, Files: []artifacts.File{{Path: file}}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		return m.Files[0].SHA256
	}
	m := expressionMapDocument{Format: "clonoloom-expression-map", Version: expression.AnalysisVersion, Input: expressionSummary{ID: "expr-comparison"}, Result: &expression.AnalysisResult{Components: 2, Cells: ec}}
	mapID := "emap-comparison-fixture"
	hash := save(mapID, expressionMapKind, m, nil)
	p := phenotypeDocument{Format: "clonoloom-phenotypes", Version: phenotype.Version, Dataset: ds.ID, ReceptorSources: dataset.Result.Sources, Input: m.Input, Map: mapID, Annotation: study.Sample{Name: ds.Samples[0].Name, Subject: "P1"}, Result: &phenotype.Result{Definition: d, DefinitionID: d.ID(), Sample: ds.Samples[0].Name, Cells: pc}}
	parent := "pheno-comparison-fixture"
	save(parent, phenotypeKind, p, []artifacts.Parent{{Kind: expressionMapKind, ID: mapID, SHA256: hash}})
	return parent, mapID, ds.ID
}
func TestCongaArtifactWorkflow(t *testing.T) {
	a, h := testApp(t)
	parent, mapID, dataset := congaParentsFixture(t, a, h)
	body := congaRequest{Phenotypes: parent, Options: conga.Options{CDR3Only: true, Neighbors: []int{1, 2}}}
	full := body
	full.Options = conga.Options{Organism: "human", Neighbors: []int{1, 2}}
	if rec := postJSON(t, h, "/api/conga", string(mustJSON(full))); rec.Code != 409 || !strings.Contains(rec.Body.String(), "no-loops") {
		t.Fatal("missing full-distance reference must not fall back to CDR3", rec.Code, rec.Body)
	}
	start := func(body any) struct {
		Artifact, Job string
		Ready         bool
	} {
		t.Helper()
		rec := postJSON(t, h, "/api/conga", string(mustJSON(body)))
		if rec.Code != 200 {
			t.Fatal(rec.Code, rec.Body)
		}
		var s struct {
			Artifact, Job string
			Ready         bool
		}
		json.Unmarshal(rec.Body.Bytes(), &s)
		return s
	}
	s := start(body)
	waitForJob(t, a, s.Job, 10*time.Second)
	raw := request2(t, h, "/api/conga/"+s.Artifact)
	var v struct {
		Artifact artifacts.Manifest
		Result   congaDocument
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	if v.Result.Result == nil || v.Result.Result.Tested != 12 || v.Result.Result.Eligible != 6 || v.Result.Phenotypes.Annotation.Subject != "P1" || len(v.Artifact.Lineage.Parents) != 2 || !strings.Contains(v.Result.Description, "CoNGA-style") {
		t.Fatal(v)
	}
	if !start(body).Ready {
		t.Fatal("cache miss")
	}
	var spec congaSpec
	json.Unmarshal(v.Artifact.Lineage.Job.Spec, &spec)
	if spec.artifactID() != s.Artifact || len(spec.Receptors) != 6 {
		t.Fatal(spec)
	}
	if _, err := a.jobs.Remake(congaKind, mustJSON(spec)); err != nil {
		t.Fatal(err)
	}
	bad := spec
	bad.Method++
	if _, err := a.jobs.Remake(congaKind, mustJSON(bad)); err == nil {
		t.Fatal("accepted old method")
	}
	for _, bad := range []string{`{}`, `{"phenotypes":"x","options":{"cdr3Only":true,"neighbors":[1,1]}}`, `{"phenotypes":"x","options":{"cdr3Only":true},"receptors":[]}`} {
		if r := postJSON(t, h, "/api/conga", bad); r.Code != 400 {
			t.Fatal(r.Code, r.Body)
		}
	}
	lookup := start(map[string]any{"phenotypes": parent, "options": conga.Options{CDR3Only: true, Neighbors: []int{3}}, "start": false})
	if lookup.Ready || lookup.Job != "" || lookup.Artifact == s.Artifact {
		t.Fatal(lookup)
	}
	// Fresh requests must agree with the source snapshot pinned by the phenotype parent.
	ds, _ := a.datasets.get(dataset)
	savedSources := ds.Result.Sources
	ds.Result.Sources = nil
	if rec := postJSON(t, h, "/api/conga", string(mustJSON(body))); rec.Code != 409 {
		t.Fatal(rec.Code, rec.Body)
	}
	ds.Result.Sources = savedSources
	// Submitted jobs no longer depend on the mutable in-memory receptor table or source files.
	for _, source := range ds.Result.Sources {
		os.Remove(source.Path)
	}
	spec.Options.Neighbors = []int{1}
	j, _, err := a.jobs.Submit(congaKind, "pinned snapshot", spec.artifactID(), spec, a.congaWork(spec))
	if err != nil {
		t.Fatal(err)
	}
	waitForJob(t, a, j.ID, 10*time.Second)
	// Changed graph bytes refuse promotion; completed results remain standalone.
	path, _ := a.artifacts.Path(mapID, expressionMapKind+".json")
	os.WriteFile(path, []byte("{}"), 0600)
	spec.Options.Neighbors = []int{4}
	j, _, err = a.jobs.Submit(congaKind, "changed parent", spec.artifactID(), spec, a.congaWork(spec))
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		job, _ := a.jobs.Get(j.ID)
		if job.State.Finished() {
			if job.State != "failed" {
				t.Fatal(job)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("job timeout")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if _, err = a.artifacts.Get(spec.artifactID()); err == nil {
		t.Fatal("promoted changed graph")
	}
	path, _ = a.artifacts.Path(parent, phenotypeKind+".json")
	os.Remove(path)
	if string(request2(t, h, "/api/conga/"+s.Artifact)) != string(raw) {
		t.Fatal("saved read depends on parents")
	}
	path, _ = a.artifacts.Path(s.Artifact, congaKind+".json")
	os.WriteFile(path, []byte("{}"), 0600)
	if rec := request(t, h, http.MethodGet, "/api/conga/"+s.Artifact, nil, nil); rec.Code != 409 {
		t.Fatal(rec.Code, rec.Body)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err = a.congaWork(spec)(ctx, nil); err != context.Canceled {
		t.Fatal(err)
	}
}
func TestCongaRefusesImportedOrUnpinnedParents(t *testing.T) {
	for _, change := range []func(*phenotypeDocument, *string){func(p *phenotypeDocument, _ *string) { p.Supplied = true }, func(_ *phenotypeDocument, h *string) { *h = "changed" }, func(p *phenotypeDocument, _ *string) { p.Version++ }} {
		a, h := testApp(t)
		parent, _ := modularityParentsFixture(t, a, change)
		rec := postJSON(t, h, "/api/conga", string(mustJSON(congaRequest{Phenotypes: parent, Options: conga.Options{CDR3Only: true}})))
		if rec.Code != 409 {
			t.Fatal(rec.Code, rec.Body)
		}
	}
}
