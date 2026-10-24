package main

// Raw expression evidence (wave 6, slice 1), separate from rearrangement datasets and analyses.
// A project pins the expression ID and explicit receptor dataset/sample; a fresh open verifies
// all three source fingerprints. Pages and coverage numbers are computed here, never in JS.
import (
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"sync"

	"clonoloom/internal/expression"
)

type expressionInputs struct {
	mu      sync.Mutex
	opening sync.Mutex
	byID    map[string]*expression.Input
}

type expressionSummary struct {
	ID            string              `json:"id"`
	Path          string              `json:"path"`
	Sources       []expression.Source `json:"sources"`
	ReaderVersion int                 `json:"readerVersion"`
	Cells         int                 `json:"cells"`
	Features      int                 `json:"features"`
	Entries       int                 `json:"entries"`
	Types         map[string]int      `json:"types"`
	Unit          string              `json:"unit"`
}

func summarizeExpression(in *expression.Input) expressionSummary {
	s := expressionSummary{in.ID, in.Path, in.Sources, expression.ReaderVersion, len(in.Matrix.Barcodes), len(in.Matrix.Features), in.Matrix.Entries, map[string]int{}, "UMIs"}
	for _, f := range in.Matrix.Features {
		s.Types[f.Type]++
	}
	return s
}
func (a *app) registerExpression(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/expression/open", a.handleExpressionOpen)
	mux.HandleFunc("POST /api/expression/page", a.handleExpressionPage)
}
func (a *app) handleExpressionOpen(w http.ResponseWriter, r *http.Request) {
	if !isLoopbackRequest(r) {
		writeError(w, 403, "Only this computer may open files.")
		return
	}
	var body struct {
		Path       string `json:"path"`
		Example    bool   `json:"example"`
		ExpectedID string `json:"expectedId"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		writeError(w, 400, "Send a MEX directory path.")
		return
	}
	if body.Example {
		if body.Path != "" {
			writeError(w, 400, "Choose a path or the artificial example, not both.")
			return
		}
		body.Path = filepath.Join(a.examples.Dir, "synthetic-expression")
	}
	path, err := absPath(body.Path)
	if err != nil || body.Path == "" {
		writeError(w, 400, "Choose a MEX directory.")
		return
	}
	a.expression.opening.Lock()
	defer a.expression.opening.Unlock()
	in, err := expression.Open(r.Context(), path)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	if body.ExpectedID != "" && body.ExpectedID != in.ID {
		writeError(w, 409, "The expression sources changed. Restore the recorded files or open the changed matrix as a new input.")
		return
	}
	a.expression.mu.Lock()
	// Bounded retention: one matrix at a time; saved inputs reopen from their fingerprints.
	a.expression.byID = map[string]*expression.Input{in.ID: in}
	a.expression.mu.Unlock()
	writeJSON(w, summarizeExpression(in))
}

type expressionCell struct {
	Barcode   string  `json:"barcode"`
	GeneUMIs  *uint64 `json:"geneUMIs"`
	Genes     *int    `json:"genes"`
	ChainRows *int    `json:"chainRows,omitempty"`
}
type expressionPage struct {
	Summary  expressionSummary    `json:"summary"`
	Links    *expression.Links    `json:"links,omitempty"`
	Offset   int                  `json:"offset"`
	Count    int                  `json:"count"`
	Cells    []expressionCell     `json:"cells"`
	Features []expression.Feature `json:"features"`
}

func (a *app) handleExpressionPage(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID      string `json:"id"`
		Dataset string `json:"dataset"`
		Sample  *int   `json:"sample"`
		View    string `json:"view"`
		Offset  int    `json:"offset"`
		Limit   int    `json:"limit"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		writeError(w, 400, "Send an expression ID and optional dataset/sample binding.")
		return
	}
	if body.View == "" {
		body.View = "cells"
	}
	if body.View != "cells" && body.View != "features" {
		writeError(w, 400, "View must be cells or features.")
		return
	}
	if body.Offset < 0 || body.Limit < 0 || body.Limit > 200 {
		writeError(w, 400, "Offset must be nonnegative; limit must be between 1 and 200.")
		return
	}
	if body.Limit == 0 {
		body.Limit = 50
	}
	if (body.Dataset == "") != (body.Sample == nil) {
		writeError(w, 400, "Give both the receptor dataset and its sample index, or neither.")
		return
	}
	a.expression.mu.Lock()
	in := a.expression.byID[body.ID]
	a.expression.mu.Unlock()
	if in == nil {
		writeError(w, 404, "Reopen the expression input from its recorded sources.")
		return
	}
	page := expressionPage{Summary: summarizeExpression(in), Offset: body.Offset, Cells: []expressionCell{}, Features: []expression.Feature{}}
	if body.Dataset != "" {
		ds, ok := a.datasets.get(body.Dataset)
		if !ok {
			writeError(w, 404, "Open the receptor dataset first.")
			return
		}
		links, err := expression.Link(r.Context(), in.Matrix, ds.Result.Table, *body.Sample)
		if err != nil {
			writeError(w, 400, err.Error())
			return
		}
		page.Links = &links
	}
	m := in.Matrix
	page.Count = len(m.Barcodes)
	if body.View == "features" {
		page.Count = len(m.Features)
	}
	start := min(body.Offset, page.Count)
	end := start + min(body.Limit, page.Count-start)
	for i := start; i < end; i++ {
		if body.View == "features" {
			page.Features = append(page.Features, m.Features[i])
			continue
		}
		row := expressionCell{Barcode: m.Barcodes[i]}
		if page.Summary.Types["Gene Expression"] > 0 {
			row.GeneUMIs = &m.GeneUMIs[i]
			row.Genes = &m.Genes[i]
		}
		if page.Links != nil {
			n := page.Links.ChainRows[i]
			row.ChainRows = &n
		}
		page.Cells = append(page.Cells, row)
	}
	writeJSON(w, page)
}
