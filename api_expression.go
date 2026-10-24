package main

// Raw expression evidence (wave 6, slice 1), separate from rearrangement datasets and analyses.
// A project pins the expression ID and explicit receptor dataset/sample; a fresh open verifies
// every source fingerprint. Pages and coverage numbers are computed here, never in JS.
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
	ID            string                      `json:"id"`
	Path          string                      `json:"path"`
	Sources       []expression.Source         `json:"sources"`
	ReaderVersion int                         `json:"readerVersion"`
	Cells         int                         `json:"cells"`
	Features      int                         `json:"features"`
	Entries       int                         `json:"entries"`
	Types         map[string]int              `json:"types"`
	Unit          string                      `json:"unit"`
	Annotations   *expression.CellCoverage    `json:"annotations,omitempty"`
	AnnData       *expression.AnnDataEvidence `json:"anndata,omitempty"`
}

func summarizeExpression(in *expression.Input) expressionSummary {
	s := expressionSummary{ID: in.ID, Path: in.Path, Sources: in.Sources, ReaderVersion: expression.ReaderVersion, Cells: len(in.Matrix.Barcodes), Features: len(in.Matrix.Features), Entries: in.Matrix.Entries, Types: map[string]int{}, Unit: "UMIs"}
	if in.AnnData != nil {
		s.AnnData = in.AnnData
		s.Unit = "counts"
	}
	if in.Cells != nil {
		s.Annotations = &in.Cells.Coverage
	}
	for _, f := range in.Matrix.Features {
		s.Types[f.Type]++
	}
	return s
}
func (a *app) registerExpression(mux *http.ServeMux) {
	a.registerExpressionMap(mux)
	mux.HandleFunc("POST /api/expression/open", a.handleExpressionOpen)
	mux.HandleFunc("POST /api/expression/page", a.handleExpressionPage)
}
func (a *app) handleExpressionOpen(w http.ResponseWriter, r *http.Request) {
	if !isLoopbackRequest(r) {
		writeError(w, 403, "Only this computer may open files.")
		return
	}
	var body struct {
		Path           string                       `json:"path"`
		AnnData        *expression.AnnDataSelection `json:"anndata"`
		AnnDataExample bool                         `json:"anndataExample"`
		Example        bool                         `json:"example"`
		HDF5Example    bool                         `json:"hdf5Example"`
		ExpectedID     string                       `json:"expectedId"`
		CellsPath      string                       `json:"cellsPath"`
		StudyExample   bool                         `json:"studyExample"`
		StudyPreset    string                       `json:"studyPreset"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		writeError(w, 400, "Send a MEX directory or 10x HDF5 file path.")
		return
	}
	if body.StudyPreset != "" && (!body.StudyExample || (body.StudyPreset != "cell-states" && body.StudyPreset != "cell-states-clouds")) {
		writeError(w, 400, "studyPreset requires studyExample and must be cell-states or cell-states-clouds.")
		return
	}
	if body.Example || body.StudyExample || body.HDF5Example || body.AnnDataExample {
		if body.Path != "" || body.CellsPath != "" || body.AnnData != nil || body.AnnDataExample && (body.Example || body.StudyExample || body.HDF5Example) || body.Example && body.StudyExample || body.HDF5Example && (body.Example || body.StudyExample) {
			writeError(w, 400, "Choose a path or the artificial example, not both.")
			return
		}
		body.Path = filepath.Join(a.examples.Dir, "synthetic-expression")
		if body.AnnDataExample {
			body.Path = filepath.Join(a.examples.Dir, "synthetic-anndata", "cells.h5ad")
			body.AnnData = &expression.AnnDataSelection{Layer: "layers/counts", Embedding: "X_umap", AIRRKey: "airr", FeatureName: "gene_name", FeatureType: "feature_type"}
		}
		if body.HDF5Example {
			body.Path = filepath.Join(a.examples.Dir, "synthetic-expression-hdf5", "matrix.h5")
		}
		if body.StudyExample {
			if body.StudyPreset == "" {
				body.StudyPreset = "cell-states"
			}
			body.Path = filepath.Join(a.examples.Dir, "simulated-"+body.StudyPreset, "expression", "S01-1")
			body.CellsPath = filepath.Join(body.Path, "cells.tsv")
		}
	}
	path, err := absPath(body.Path)
	if err != nil || body.Path == "" {
		writeError(w, 400, "Choose a MEX directory or 10x HDF5 file.")
		return
	}
	a.expression.opening.Lock()
	defer a.expression.opening.Unlock()
	if body.CellsPath != "" {
		body.CellsPath, err = absPath(body.CellsPath)
		if err != nil {
			writeError(w, 400, err.Error())
			return
		}
	}
	in, err := expression.OpenSelected(r.Context(), path, body.CellsPath, body.AnnData)
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
	Barcode    string                     `json:"barcode"`
	GeneUMIs   *uint64                    `json:"geneUMIs"`
	Genes      *int                       `json:"genes"`
	ChainRows  *int                       `json:"chainRows,omitempty"`
	Annotation *expression.CellAnnotation `json:"annotation,omitempty"`
}
type expressionPage struct {
	Summary        expressionSummary           `json:"summary"`
	Links          *expression.Links           `json:"links,omitempty"`
	Offset         int                         `json:"offset"`
	Count          int                         `json:"count"`
	Cells          []expressionCell            `json:"cells"`
	Features       []expression.Feature        `json:"features"`
	Annotations    []expression.CellAnnotation `json:"annotations,omitempty"`
	VariableValues [][]string                  `json:"variableValues,omitempty"`
	AIRR           []expression.AIRRRow        `json:"airr,omitempty"`
	AIRRCells      []expression.AIRRCell       `json:"airrCells,omitempty"`
	Map            []expression.CellAnnotation `json:"map,omitempty"`
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
	if body.View != "cells" && body.View != "features" && body.View != "annotations" && body.View != "map" && body.View != "airr" && body.View != "airr-cells" {
		writeError(w, 400, "View must be cells, features, annotations, map, airr or airr-cells.")
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
	if body.View == "airr" || body.View == "airr-cells" {
		if in.AnnData == nil || in.AnnData.AIRR == nil {
			writeError(w, 400, "Select an AIRR obsm key first.")
			return
		}
		if body.View == "airr-cells" {
			rows := in.AnnData.AIRR.CellRows
			page.Count = len(rows)
			page.Offset = min(body.Offset, len(rows))
			end := page.Offset + min(body.Limit, len(rows)-page.Offset)
			page.AIRRCells = rows[page.Offset:end]
			writeJSON(w, page)
			return
		}
		rows := in.AnnData.AIRR.Rows
		page.Count = len(rows)
		page.Offset = min(body.Offset, len(rows))
		end := page.Offset + min(body.Limit, len(rows)-page.Offset)
		page.AIRR = rows[page.Offset:end]
		writeJSON(w, page)
		return
	}
	if body.View == "annotations" || body.View == "map" {
		if in.Cells == nil {
			writeError(w, 400, "Select a cell annotation table first.")
			return
		}
		if body.View == "map" {
			if in.Cells.Coverage.Mapped > expression.MaxMapPoints {
				writeError(w, 400, "Imported maps are limited to 20000 expression barcodes with coordinates; inspect the complete annotation table instead.")
				return
			}
			for i, row := range in.Cells.Rows {
				if i%4096 == 0 {
					if err := r.Context().Err(); err != nil {
						writeError(w, 400, err.Error())
						return
					}
				}
				if row.Expression && row.Position != nil {
					page.Map = append(page.Map, row)
				}
			}
		}
		page.Count = len(in.Cells.Rows)
		start := min(body.Offset, page.Count)
		page.Offset = start
		end := start + min(body.Limit, page.Count-start)
		page.Annotations = in.Cells.Rows[start:end]
		writeJSON(w, page)
		return
	}
	page.Count = len(m.Barcodes)
	if body.View == "features" {
		page.Count = len(m.Features)
	}
	start := min(body.Offset, page.Count)
	page.Offset = start
	end := start + min(body.Limit, page.Count-start)
	for i := start; i < end; i++ {
		if body.View == "features" {
			page.Features = append(page.Features, m.Features[i])
			if in.AnnData != nil {
				page.VariableValues = append(page.VariableValues, in.AnnData.VariableValues[i])
			}
			continue
		}
		row := expressionCell{Barcode: m.Barcodes[i]}
		if in.Cells != nil {
			if index, ok := in.Cells.ByBarcode[row.Barcode]; ok {
				row.Annotation = &in.Cells.Rows[index]
			}
		}
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
