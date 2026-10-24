package main

import (
	"clonoloom/internal/expression"
	"clonoloom/internal/simulate"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestExpressionEvidenceAndReopening(t *testing.T) {
	app, h := testApp(t)
	dir := t.TempDir()
	example := expressionInputExample()
	if err := example.Make(context.Background(), dir, nil); err != nil {
		t.Fatal(err)
	}
	var ds datasetSummary
	rec := postJSON(t, h, "/api/datasets/open", `{"paths":[`+jsonString(filepath.Join(dir, "receptors.tsv"))+`]}`)
	if rec.Code != 200 {
		t.Fatal(rec.Body)
	}
	json.Unmarshal(rec.Body.Bytes(), &ds)
	open := func(expected string) expressionSummary {
		t.Helper()
		rec := postJSON(t, h, "/api/expression/open", `{"path":`+jsonString(dir)+`,"expectedId":`+jsonString(expected)+`}`)
		if rec.Code != 200 {
			t.Fatal(rec.Code, rec.Body)
		}
		var summary expressionSummary
		if err := json.Unmarshal(rec.Body.Bytes(), &summary); err != nil {
			t.Fatal(err)
		}
		return summary
	}
	summary := open("")
	if summary.Cells != 4 || summary.Features != 3 || summary.Entries != 6 || summary.Types["Antibody Capture"] != 1 || summary.Unit != "UMIs" {
		t.Fatal(summary)
	}
	page := func(extra string) expressionPage {
		t.Helper()
		rec := postJSON(t, h, "/api/expression/page", `{"id":`+jsonString(summary.ID)+extra+`}`)
		if rec.Code != 200 {
			t.Fatal(rec.Code, rec.Body)
		}
		var page expressionPage
		json.Unmarshal(rec.Body.Bytes(), &page)
		return page
	}
	unbound := page(`,"limit":1`)
	if unbound.Links != nil || len(unbound.Cells) != 1 || unbound.Cells[0].ChainRows != nil {
		t.Fatal(unbound)
	}
	binding := `,"dataset":` + jsonString(ds.ID) + `,"sample":0`
	linked := page(binding)
	if linked.Links.Matched != 2 || linked.Links.ReceptorOnly != 1 || linked.Links.ExpressionOnly != 2 || linked.Links.MissingBarcodeRows != 1 || *linked.Cells[0].ChainRows != 2 || *linked.Cells[0].GeneUMIs != 5 || *linked.Cells[1].ChainRows != 1 {
		t.Fatal(linked)
	}
	last := page(binding + `,"offset":3,"limit":1`)
	if len(last.Cells) != 1 || last.Cells[0].Barcode != "DD-1" || *last.Cells[0].GeneUMIs != 0 {
		t.Fatal(last)
	}
	beyond := page(`,"offset":9223372036854775807`)
	if len(beyond.Cells) != 0 {
		t.Fatal(beyond)
	}
	features := page(`,"view":"features","offset":1,"limit":2`)
	if len(features.Features) != 2 || features.Features[1].Type != "Antibody Capture" {
		t.Fatal(features)
	}
	for _, extra := range []string{`,"sample":0`, `,"dataset":"x"`, binding + `,"limit":201`, binding + `,"offset":-1`, `,"view":"unknown"`, `,"dataset":` + jsonString(ds.ID) + `,"sample":42`} {
		rec := postJSON(t, h, "/api/expression/page", `{"id":`+jsonString(summary.ID)+extra+`}`)
		if rec.Code != 400 {
			t.Fatal(rec.Code, rec.Body)
		}
	}
	// Eviction/restart does not alter identity; every companion is reread.
	app.expression.mu.Lock()
	app.expression.byID = nil
	app.expression.mu.Unlock()
	rec = postJSON(t, h, "/api/expression/page", `{"id":`+jsonString(summary.ID)+`}`)
	if rec.Code != 404 {
		t.Fatal(rec.Code)
	}
	if open(summary.ID).ID != summary.ID {
		t.Fatal("identity changed")
	}
	matrix := filepath.Join(dir, "matrix.mtx")
	data, err := os.ReadFile(matrix)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(matrix, []byte(strings.Replace(string(data), "1 1 2", "1 1 9", 1)), 0600); err != nil {
		t.Fatal(err)
	}
	rec = postJSON(t, h, "/api/expression/open", `{"path":`+jsonString(dir)+`,"expectedId":`+jsonString(summary.ID)+`}`)
	if rec.Code != 409 {
		t.Fatal(rec.Code, rec.Body)
	}
	if open("").ID == summary.ID {
		t.Fatal("drift not detected")
	}
	// The endpoint reads local paths only, behind the same-origin host's usual protection.
	req := httptest.NewRequest(http.MethodPost, "/api/expression/open", strings.NewReader(`{"path":`+jsonString(dir)+`}`))
	req.RemoteAddr = "192.0.2.1:1234"
	out := httptest.NewRecorder()
	app.handleExpressionOpen(out, req)
	if out.Code != 403 {
		t.Fatal(out.Code)
	}
}

func TestExpressionAnnotationsAndImportedMap(t *testing.T) {
	app, h := testApp(t)
	dir := filepath.Join(t.TempDir(), "study")
	if _, err := simulate.Run(context.Background(), simulate.Presets["cell-states"], dir, "validation", nil); err != nil {
		t.Fatal(err)
	}
	items, problem := walkFolder(dir)
	if problem != "" || len(items) != 5 {
		t.Fatal("expression or truth leaked into receptor inputs", items, problem)
	}
	path := filepath.Join(dir, "expression", "S01-1")
	cells := filepath.Join(path, "cells.tsv")
	open := func(expected string) *httptest.ResponseRecorder {
		return postJSON(t, h, "/api/expression/open", `{"path":`+jsonString(path)+`,"cellsPath":`+jsonString(cells)+`,"expectedId":`+jsonString(expected)+`}`)
	}
	rec := open("")
	if rec.Code != 200 {
		t.Fatal(rec.Body)
	}
	var s expressionSummary
	json.Unmarshal(rec.Body.Bytes(), &s)
	if s.Annotations.Mapped != 118 || s.Annotations.WithoutCoordinates != 2 || s.Annotations.WithoutAnnotation != 1 || s.Annotations.AnnotationOnly != 1 || len(s.Sources) != 4 {
		t.Fatal(s)
	}
	page := func(view string, offset int) expressionPage {
		t.Helper()
		rec := postJSON(t, h, "/api/expression/page", `{"id":`+jsonString(s.ID)+`,"view":`+jsonString(view)+`,"offset":`+strconv.Itoa(offset)+`}`)
		if rec.Code != 200 {
			t.Fatal(rec.Body)
		}
		var p expressionPage
		json.Unmarshal(rec.Body.Bytes(), &p)
		return p
	}
	p := page("map", 100)
	if len(p.Map) != 118 || p.Count != 120 || len(p.Annotations) != 20 || p.Annotations[19].Expression || p.Annotations[19].Barcode != "ANNOTATION-ONLY" {
		t.Fatal(p)
	}
	p = page("cells", 0)
	if p.Cells[1].Annotation != nil || p.Cells[2].Annotation.Position != nil || p.Cells[3].Annotation.Values[0] != "" {
		t.Fatal(p.Cells[:4])
	}
	p = page("annotations", 999)
	if p.Offset != 120 || len(p.Annotations) != 0 {
		t.Fatal(p)
	}
	if open(s.ID).Code != 200 {
		t.Fatal("unchanged files rejected")
	}
	raw, _ := os.ReadFile(cells)
	os.WriteFile(cells, []byte(strings.Replace(string(raw), "state A", "state Z", 1)), 0600)
	if rec := open(s.ID); rec.Code != 409 {
		t.Fatal("changed annotations accepted", rec.Body)
	}
	// The rejected replacement retains the previous cached evidence. Map limits never silently
	// truncate the population, and its full annotation pages remain available.
	app.expression.mu.Lock()
	app.expression.byID[s.ID].Cells.Coverage.Mapped = expression.MaxMapPoints + 1
	app.expression.mu.Unlock()
	rec = postJSON(t, h, "/api/expression/page", `{"id":`+jsonString(s.ID)+`,"view":"map"}`)
	if rec.Code != 400 {
		t.Fatal(rec.Body)
	}
	page("annotations", 0)
	for _, body := range []string{`{"example":true,"studyExample":true}`, `{"studyExample":true,"cellsPath":"x"}`} {
		if rec := postJSON(t, h, "/api/expression/open", body); rec.Code != 400 {
			t.Fatal(rec.Body)
		}
	}
}

func TestExpressionWithoutGeneExpressionIsNotMeasured(t *testing.T) {
	_, h := testApp(t)
	dir := t.TempDir()
	for name, data := range map[string]string{"matrix.mtx": "%%MatrixMarket matrix coordinate integer general\n1 1 1\n1 1 2\n", "features.tsv": "ab1\tAntibody\tAntibody Capture\n", "barcodes.tsv": "one\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	rec := postJSON(t, h, "/api/expression/open", `{"path":`+jsonString(dir)+`}`)
	if rec.Code != 200 {
		t.Fatal(rec.Body)
	}
	var summary expressionSummary
	json.Unmarshal(rec.Body.Bytes(), &summary)
	rec = postJSON(t, h, "/api/expression/page", `{"id":`+jsonString(summary.ID)+`}`)
	if rec.Code != 200 {
		t.Fatal(rec.Body)
	}
	var page expressionPage
	json.Unmarshal(rec.Body.Bytes(), &page)
	if len(page.Cells) != 1 || page.Cells[0].GeneUMIs != nil || page.Cells[0].Genes != nil {
		t.Fatal(page)
	}
}
