package main

// Computed expression maps are immutable artifacts, independent of receptor clonotyping.
// Jobs reopen fingerprinted sources; saved results remain readable without those sources.
import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"clonoloom/internal/artifacts"
	"clonoloom/internal/expression"
	"clonoloom/internal/jobs"
)

const expressionMapKind = "expression-map"

type expressionMapSpec struct {
	ID        string                       `json:"id"`
	Path      string                       `json:"path"`
	CellsPath string                       `json:"cellsPath,omitempty"`
	AnnData   *expression.AnnDataSelection `json:"anndata,omitempty"`
	Options   expression.AnalysisOptions   `json:"options"`
	Method    int                          `json:"method"`
}

func (s expressionMapSpec) artifactID() string {
	raw, _ := json.Marshal(struct {
		ID      string
		Options expression.AnalysisOptions
		Method  int
	}{s.ID, s.Options, s.Method})
	sum := sha256.Sum256(raw)
	return "emap-" + hex.EncodeToString(sum[:])[:24]
}

type expressionMapDocument struct {
	Format      string                     `json:"format"`
	Version     int                        `json:"version"`
	Input       expressionSummary          `json:"input"`
	Description string                     `json:"description"`
	Result      *expression.AnalysisResult `json:"analysis"`
}

func expressionMapDescription(r *expression.AnalysisResult) string {
	o := r.Options
	return fmt.Sprintf("Gene Expression counts from %d barcodes were filtered to %d cells with at least %d detected genes and positive gene totals; %d cells were excluded. Counts were normalized to %g per cell using all Gene Expression features, then transformed by ln(1+x). Among genes detected in at least %d retained cells, the %d selected features were ranked by sample variance (ties by feature ID), centered without variance scaling or clipping, and reduced to %d nonzero principal components. Exact Euclidean neighborhoods contained %d other cells, with ties by source barcode order. UMAP used their fuzzy union, PCA initialization, minimum distance %g, spread 1, %d epochs, five negative samples, learning rate 1, repulsion 1 and seed %d. The graph has %d connected components. All effective numerical rules and source fingerprints are recorded. Supplied coordinates and receptor annotations did not enter the fit. This exploratory map does not establish cell types, clonotypes, antigen specificity or subject-level effects.", r.InputCells, len(r.Cells), o.MinGenes, len(r.Excluded), o.Target, o.MinCells, len(r.Features), r.Components, r.Neighbors.K, *o.MinDist, o.Epochs, o.Seed, r.GraphComponents)
}

const expressionMapCitation = "Wolf FA, Angerer P, Theis FJ. SCANPY. Genome Biol. 2018;19:15. doi:10.1186/s13059-017-1382-0; Jolliffe IT, Cadima J. Principal component analysis: a review and recent developments. Phil Trans R Soc A. 2016;374:20150202. doi:10.1098/rsta.2015.0202; McInnes L, Healy J, Melville J. UMAP. 2018. doi:10.48550/arXiv.1802.03426. Variance ranking and PCA initialization are ClonoLoom choices."

func (a *app) registerExpressionMap(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/expression-map", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ID      string                     `json:"id"`
			Options expression.AnalysisOptions `json:"options"`
			Start   *bool                      `json:"start"`
		}
		decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&body); err != nil {
			writeError(w, 400, "Send an open expression id and expression-map options: "+err.Error())
			return
		}
		options, err := body.Options.Normalize()
		if err != nil {
			writeError(w, 400, err.Error())
			return
		}
		spec := expressionMapSpec{ID: body.ID, Options: options, Method: expression.AnalysisVersion}
		id := spec.artifactID()
		if m, err := a.artifacts.Get(id); err == nil && m.Kind == expressionMapKind && m.State == artifacts.Complete {
			writeJSON(w, map[string]any{"artifact": id, "ready": true})
			return
		}
		a.expression.mu.Lock()
		in := a.expression.byID[body.ID]
		a.expression.mu.Unlock()
		if in == nil {
			writeError(w, 404, "Reopen the expression input from its recorded sources first.")
			return
		}
		spec.Path = in.Path
		for _, s := range in.Sources {
			if s.Role == "cells" {
				spec.CellsPath = s.Path
			}
		}
		if in.AnnData != nil {
			selection := in.AnnData.Selection
			spec.AnnData = &selection
		}
		if body.Start != nil && !*body.Start {
			out := map[string]any{"artifact": id, "ready": false}
			for _, j := range a.jobs.List(0) {
				if j.Key == id && !j.State.Finished() {
					out["job"] = j.ID
				}
			}
			writeJSON(w, out)
			return
		}
		job, _, err := a.jobs.Submit(expressionMapKind, "Expression map · "+filepath.Base(in.Path), id, spec, a.expressionMapWork(spec))
		if err != nil {
			writeError(w, 503, err.Error())
			return
		}
		writeJSON(w, map[string]any{"artifact": id, "ready": false, "job": job.ID})
	})
	mux.HandleFunc("GET /api/expression-map/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		m, err := a.artifacts.Get(id)
		if err != nil || m.Kind != expressionMapKind || m.State != artifacts.Complete {
			writeError(w, 404, "No complete expression map with that ID.")
			return
		}
		findings, err := a.artifacts.Verify(id, nil)
		if err != nil || len(findings) > 0 {
			writeError(w, 409, "The saved expression map failed checksum verification.")
			return
		}
		path, err := a.artifacts.Path(id, expressionMapKind+".json")
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"artifact":%s,"result":%s}`, mustJSON(m), raw)
	})
}
func (a *app) expressionMapWork(spec expressionMapSpec) jobs.Func {
	return func(ctx context.Context, run *jobs.Run) error {
		if spec.Method != expression.AnalysisVersion {
			return errors.New("expression-map method version changed; start a new analysis")
		}
		run.Progress(0, "Verifying expression sources")
		in, err := expression.OpenSelected(ctx, spec.Path, spec.CellsPath, spec.AnnData)
		if err != nil {
			return err
		}
		if in.ID != spec.ID {
			return errors.New("expression sources changed since this map was requested; reopen them as a new input")
		}
		result, err := expression.Analyze(ctx, in, spec.Options, 0, func(f float64, s string) { run.Progress(.05+float64(.9*f), s) })
		if err != nil {
			return err
		}
		doc := expressionMapDocument{Format: "clonoloom-expression-map", Version: expression.AnalysisVersion, Input: summarizeExpression(in), Description: expressionMapDescription(result), Result: result}
		raw, err := json.Marshal(doc)
		if err != nil {
			return err
		}
		id := spec.artifactID()
		dir, err := a.artifacts.Stage(id)
		if err != nil {
			return err
		}
		defer a.artifacts.Discard(id)
		file := expressionMapKind + ".json"
		if err := os.WriteFile(filepath.Join(dir, file), raw, 0o644); err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		run.Validating()
		parameters := mustJSON(struct {
			Options expression.AnalysisOptions `json:"options"`
			Rules   map[string]any             `json:"rules"`
		}{result.Options, result.Rules})
		manifest := artifacts.Manifest{
			ID: id, Kind: expressionMapKind, Name: "Expression map · " + filepath.Base(in.Path),
			Lineage: artifacts.Lineage{
				Parents:    []artifacts.Parent{{Kind: "expression", ID: in.ID}},
				Job:        &artifacts.Job{ID: run.ID(), Kind: expressionMapKind, Spec: mustJSON(spec)},
				Parameters: parameters,
				Method:     artifacts.Method{Name: "Expression normalization, PCA, exact neighbors and UMAP", Version: fmt.Sprint(expression.AnalysisVersion), Citation: expressionMapCitation},
				Software:   []artifacts.Software{{Name: "clonoloom", Version: version, Commit: buildCommit()}},
			},
			Files:   []artifacts.File{{Path: file, Role: expressionMapKind}},
			Summary: mustJSON(map[string]any{"cells": len(result.Cells), "excluded": len(result.Excluded), "features": len(result.Features), "components": result.Components, "edges": result.Graph.Edges()}),
		}
		m, err := a.artifacts.Promote(manifest, func(dir string, m *artifacts.Manifest) []artifacts.Finding {
			raw, err := os.ReadFile(filepath.Join(dir, file))
			var back expressionMapDocument
			if err == nil {
				err = json.Unmarshal(raw, &back)
			}
			if err != nil || back.Result == nil || len(back.Result.Cells) != len(result.Cells) || back.Input.ID != spec.ID {
				return []artifacts.Finding{{Severity: "fatal", Code: "expression-map.unreadable", Message: "The computed expression result could not be read back."}}
			}
			return nil
		})
		if err != nil {
			return err
		}
		if m.State != artifacts.Complete {
			return errors.New("the expression map failed validation")
		}
		run.Log("info", "Saved %d expression cells and %d graph edges as %s.", len(result.Cells), result.Graph.Edges(), id)
		return nil
	}
}
func (a *app) handleExpressionMapJobs() {
	a.jobs.Handle(expressionMapKind, func(raw json.RawMessage) (jobs.Func, error) {
		var spec expressionMapSpec
		if err := json.Unmarshal(raw, &spec); err != nil || spec.ID == "" || spec.Path == "" {
			return nil, errors.New("the expression map job has no input")
		}
		if spec.Method != expression.AnalysisVersion {
			return nil, errors.New("expression-map method version changed; start a new analysis")
		}
		if _, err := spec.Options.Normalize(); err != nil {
			return nil, err
		}
		return a.expressionMapWork(spec), nil
	}, nil)
}
