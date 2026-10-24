package main

// Immutable, sample-scoped receptor overlays. A map is a parent, never refitted by selection.
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
	"clonoloom/internal/clonotype"
	"clonoloom/internal/expression"
	"clonoloom/internal/jobs"
	"clonoloom/internal/phenotype"
	"clonoloom/internal/qc"
	"clonoloom/internal/reads"
	"clonoloom/internal/study"
)

const phenotypeKind = "phenotypes"
const phenotypeCitation = "Sturm G et al. Scirpy. Bioinformatics. 2020;36:4817–4818. doi:10.1093/bioinformatics/btaa611; scirpy.tl.clonal_expansion API (sample groups and inclusive breakpoints); 10x Genomics, Integrated V(D)J and Gene Expression Analysis in Loupe Browser. Distinct-cell counting, ambiguity and denominator rules are ClonoLoom policies; no inferential test."

type phenotypeFilter struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version int    `json:"version"`
	Reason  string `json:"reason"`
}
type phenotypeRequest struct {
	FilterRecord *phenotypeFilter     `json:"filterRecord,omitempty"`
	ID           string               `json:"id"`
	Map          string               `json:"map"`
	Dataset      string               `json:"dataset"`
	Sample       *int                 `json:"sample"`
	Definition   clonotype.Definition `json:"definition"`
	Filter       qc.Filter            `json:"filter"`
	Annotation   study.Sample         `json:"annotation"`
	Options      phenotype.Options    `json:"options"`
}
type phenotypeSpec struct {
	phenotypeRequest
	Input     expressionMapSpec `json:"input"` // source paths for durable jobs, excluded from identity
	MapSHA256 string            `json:"mapSha256"`
	Method    int               `json:"method"`
}

func (s phenotypeSpec) artifactID() string {
	request := s.phenotypeRequest
	request.Definition.Name = ""
	raw := mustJSON(struct {
		Request   phenotypeRequest
		MapSHA256 string
		Method    int
	}{request, s.MapSHA256, s.Method})
	sum := sha256.Sum256(raw)
	return "pheno-" + hex.EncodeToString(sum[:])[:24]
}

type phenotypeDocument struct {
	ReceptorSources []reads.Source    `json:"receptorSources"`
	Format          string            `json:"format"`
	Version         int               `json:"version"`
	Dataset         string            `json:"dataset"`
	DatasetName     string            `json:"datasetName"`
	Input           expressionSummary `json:"input"`
	Map             string            `json:"map"`
	Axes            [2]string         `json:"axes"`
	Supplied        bool              `json:"supplied"`
	Annotation      study.Sample      `json:"annotation"`
	Description     string            `json:"description"`
	Result          *phenotype.Result `json:"analysis"`
}

func (a *app) checkedExpressionMap(id string) (*expressionMapDocument, string, error) {
	m, err := a.artifacts.Get(id)
	if err != nil || m.Kind != expressionMapKind || m.State != artifacts.Complete {
		return nil, "", errors.New("choose a complete expression map")
	}
	findings, err := a.artifacts.Verify(id, nil)
	if err != nil || len(findings) > 0 {
		return nil, "", errors.New("expression map failed checksum verification")
	}
	path, err := a.artifacts.Path(id, expressionMapKind+".json")
	if err != nil {
		return nil, "", err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	var doc expressionMapDocument
	if err = json.Unmarshal(raw, &doc); err != nil {
		return nil, "", err
	}
	if doc.Result == nil {
		return nil, "", errors.New("expression map has no analysis")
	}
	sum := sha256.Sum256(raw)
	return &doc, hex.EncodeToString(sum[:]), nil
}
func (a *app) registerPhenotypes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/phenotypes", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			phenotypeRequest
			Start *bool `json:"start"`
		}
		decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&body); err != nil {
			writeError(w, 400, err.Error())
			return
		}
		s := phenotypeSpec{phenotypeRequest: body.phenotypeRequest, Method: phenotype.Version}
		if s.ID == "" || s.Dataset == "" || s.Sample == nil || *s.Sample < 0 {
			writeError(w, 400, "Choose an expression input and an explicit receptor dataset/sample; unlinked inputs have no clonotype size.")
			return
		}
		var err error
		s.Definition, err = s.Definition.Normalize()
		if err != nil {
			writeError(w, 400, err.Error())
			return
		}
		s.Options, err = s.Options.Normalize()
		if err != nil {
			writeError(w, 400, err.Error())
			return
		}
		if s.Filter.MinCount < 0 || s.Filter.MaxChains < 0 {
			writeError(w, 400, "QC thresholds must be nonnegative.")
			return
		}
		if s.Map != "" {
			parent, hash, err := a.checkedExpressionMap(s.Map)
			if err != nil {
				writeError(w, 409, err.Error())
				return
			}
			if parent.Input.ID != s.ID {
				writeError(w, 400, "The computed map belongs to a different expression input.")
				return
			}
			s.MapSHA256 = hash
		}
		// Sample name is part of the supplied metadata: callers must pin it for cached requests.
		if s.Annotation.Name == "" {
			writeError(w, 400, "Pin the selected sample name with its subject/timepoint annotation.")
			return
		}
		id := s.artifactID()
		if m, err := a.artifacts.Get(id); err == nil && m.Kind == phenotypeKind && m.State == artifacts.Complete {
			writeJSON(w, map[string]any{"artifact": id, "ready": true})
			return
		}
		ds, ok := a.datasets.get(s.Dataset)
		if !ok {
			writeError(w, 404, "Reopen the receptor dataset from its files first.")
			return
		}
		if ds.Result.Table.Cell == nil || *s.Sample >= len(ds.Result.Table.Samples) || s.Annotation.Name != ds.Result.Table.Samples[*s.Sample].Name {
			writeError(w, 400, "Choose a single-cell dataset and its exact sample name/index.")
			return
		}
		a.expression.mu.Lock()
		in := a.expression.byID[s.ID]
		a.expression.mu.Unlock()
		if in == nil {
			writeError(w, 404, "Reopen the expression input from its recorded sources first.")
			return
		}
		s.Input = expressionMapSpec{ID: in.ID, Path: in.Path}
		for _, f := range in.Sources {
			if f.Role == "cells" {
				s.Input.CellsPath = f.Path
			}
		}
		if in.AnnData != nil {
			v := in.AnnData.Selection
			s.Input.AnnData = &v
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
		job, _, err := a.jobs.Submit(phenotypeKind, "Clonotype phenotypes · "+s.Annotation.Name, id, s, a.phenotypeWork(s))
		if err != nil {
			writeError(w, 503, err.Error())
			return
		}
		writeJSON(w, map[string]any{"artifact": id, "ready": false, "job": job.ID})
	})
	mux.HandleFunc("GET /api/phenotypes/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		m, err := a.artifacts.Get(id)
		if err != nil || m.Kind != phenotypeKind || m.State != artifacts.Complete {
			writeError(w, 404, "No complete phenotype result with that ID.")
			return
		}
		findings, err := a.artifacts.Verify(id, nil)
		if err != nil || len(findings) > 0 {
			writeError(w, 409, "The saved phenotype result failed checksum verification.")
			return
		}
		path, err := a.artifacts.Path(id, phenotypeKind+".json")
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
func (a *app) phenotypeWork(s phenotypeSpec) jobs.Func {
	return func(ctx context.Context, run *jobs.Run) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if s.Method != phenotype.Version {
			return errors.New("phenotype method version changed; start a new result")
		}
		run.Progress(0, "Verifying expression and receptor inputs")
		in, err := expression.OpenSelected(ctx, s.Input.Path, s.Input.CellsPath, s.Input.AnnData)
		if err != nil {
			return err
		}
		if in.ID != s.ID {
			return errors.New("expression sources changed; reopen them as a new input")
		}
		ds, entry, _, err := a.clonotypeTable(clonotypeRequest{Dataset: s.Dataset, Definition: s.Definition, Filter: s.Filter})
		if err != nil {
			return err
		}
		if s.Sample == nil || *s.Sample < 0 || *s.Sample >= len(ds.Result.Table.Samples) || ds.Result.Table.Samples[*s.Sample].Name != s.Annotation.Name {
			return errors.New("the receptor sample binding changed")
		}
		if len(in.Matrix.Barcodes) > phenotype.MaxCells {
			return fmt.Errorf("overlays accept at most %d expression cells", phenotype.MaxCells)
		}
		columns := []string{}
		if in.Cells != nil {
			columns = in.Cells.Coverage.Columns
		}
		axes := [2]string{"map_x", "map_y"}
		if in.AnnData != nil && in.AnnData.Embedding != nil {
			axes = in.AnnData.Embedding.Axes
		}
		positions := map[string]*[2]float64{}
		if s.Map != "" {
			parent, hash, err := a.checkedExpressionMap(s.Map)
			if err != nil {
				return err
			}
			if hash != s.MapSHA256 || parent.Input.ID != s.ID {
				return errors.New("expression-map parent changed; request a new result")
			}
			axes = parent.Result.Axes
			for _, c := range parent.Result.Cells {
				positions[c.Barcode] = c.Position
			}
		}
		cells := make([]phenotype.Cell, 0, len(in.Matrix.Barcodes))
		for i, barcode := range in.Matrix.Barcodes {
			if i%4096 == 0 {
				if err := ctx.Err(); err != nil {
					return err
				}
			}
			c := phenotype.Cell{Barcode: barcode, Values: make([]string, len(columns))}
			if in.Cells != nil {
				if index, ok := in.Cells.ByBarcode[barcode]; ok {
					ann := in.Cells.Rows[index]
					c.Values = ann.Values
					if s.Map == "" {
						c.Position = ann.Position
					}
				}
			}
			if s.Map != "" {
				c.Position = positions[barcode]
			}
			cells = append(cells, c)
		}
		run.Progress(.4, "Counting distinct cells and supplied phenotypes")
		result, err := phenotype.Compute(ctx, ds.Result.Table, entry.table, s.Filter, *s.Sample, cells, columns, s.Options)
		if err != nil {
			return err
		}
		description := fmt.Sprintf("Exact barcode matching linked expression input %s to receptor dataset %s, sample %s only. Clonotypes: %s. QC: %s. Size population: %s; inclusive breakpoints %v. Sizes count uniquely assigned distinct cells, never chain rows. Multiple memberships and conflicting vendor IDs are ambiguous. Phenotype tables include all %d expression cells (%d mapped), including missing labels. Expanded means size > 1; expansion share is expanded / sized cells. Association fractions use all expression cells of that clone, or all expression cells of that annotation group, including missing annotations. Unassigned and unsized cells have no numeric clone size. Subject %q; timepoint %q (empty means missing). Samples are not pooled; cells are not biological replicates. No enrichment test, inferred phenotype or antigen-specificity claim.", s.ID, s.Dataset, result.Sample, result.Definition.Describe(), s.Filter.Describe(), result.Options.Population, result.Options.Breakpoints, len(result.Cells), result.Coverage.Mapped, s.Annotation.Subject, s.Annotation.Time)
		doc := phenotypeDocument{ReceptorSources: ds.Result.Sources, Format: "clonoloom-phenotypes", Version: phenotype.Version, Dataset: s.Dataset, DatasetName: ds.Name, Input: summarizeExpression(in), Map: s.Map, Axes: axes, Supplied: s.Map == "", Annotation: s.Annotation, Description: description, Result: result}
		raw, err := json.Marshal(doc)
		if err != nil {
			return err
		}
		id := s.artifactID()
		dir, err := a.artifacts.Stage(id)
		if err != nil {
			return err
		}
		defer a.artifacts.Discard(id)
		file := phenotypeKind + ".json"
		if err := os.WriteFile(filepath.Join(dir, file), raw, 0644); err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		run.Validating()
		parents := []artifacts.Parent{{Kind: "dataset", ID: ds.ID, Name: ds.Name}, {Kind: "expression", ID: in.ID}}
		if s.Map != "" {
			parents = append(parents, artifacts.Parent{Kind: expressionMapKind, ID: s.Map, SHA256: s.MapSHA256})
		}
		manifest := artifacts.Manifest{ID: id, Kind: phenotypeKind, Name: "Clonotype phenotypes · " + result.Sample, Dataset: ds.ID, Lineage: artifacts.Lineage{Parents: parents, Job: &artifacts.Job{ID: run.ID(), Kind: phenotypeKind, Spec: mustJSON(s)}, Parameters: mustJSON(s.phenotypeRequest), Method: artifacts.Method{Name: "Sample-scoped clonotype sizes and descriptive phenotype counts", Version: fmt.Sprint(phenotype.Version), Citation: phenotypeCitation}, Software: []artifacts.Software{{Name: "clonoloom", Version: version, Commit: buildCommit()}}}, Files: []artifacts.File{{Path: file, Role: phenotypeKind}}, Summary: mustJSON(result.Coverage)}
		m, err := a.artifacts.Promote(manifest, func(dir string, m *artifacts.Manifest) []artifacts.Finding {
			raw, err := os.ReadFile(filepath.Join(dir, file))
			var back phenotypeDocument
			if err == nil {
				err = json.Unmarshal(raw, &back)
			}
			if err != nil || back.Result == nil || len(back.Result.Cells) != len(result.Cells) || back.Input.ID != s.ID {
				return []artifacts.Finding{{Severity: "fatal", Code: "phenotypes.unreadable", Message: "The phenotype result could not be read back."}}
			}
			return nil
		})
		if err != nil {
			return err
		}
		if m.State != artifacts.Complete {
			return errors.New("phenotype result failed validation")
		}
		run.Log("info", "Saved %d expression cells and %d sample-scoped clonotypes as %s.", len(result.Cells), len(result.Clones), id)
		return nil
	}
}
func (a *app) handlePhenotypeJobs() {
	a.jobs.Handle(phenotypeKind, func(raw json.RawMessage) (jobs.Func, error) {
		var s phenotypeSpec
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		if s.ID == "" || s.Input.Path == "" || s.Dataset == "" || s.Sample == nil || s.Method != phenotype.Version {
			return nil, errors.New("phenotype job input or method version is unavailable")
		}
		if _, err := s.Options.Normalize(); err != nil {
			return nil, err
		}
		return a.phenotypeWork(s), nil
	}, nil)
}
