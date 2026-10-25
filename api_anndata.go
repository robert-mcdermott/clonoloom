package main

// AnnData is a durable export of verified evidence and saved analyses. A download never reruns
// a fit, pools samples or substitutes changed source files. Provider notices travel with it.
import (
	"archive/zip"
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
	"slices"

	"clonoloom/internal/anndata"
	"clonoloom/internal/artifacts"
	"clonoloom/internal/clonotype"
	"clonoloom/internal/export"
	"clonoloom/internal/expression"
	"clonoloom/internal/jobs"
	"clonoloom/internal/phenotype"
)

const annDataKind = "anndata-export"
const annDataCitation = "AnnData on-disk format specification: https://anndata.readthedocs.io/en/stable/fileformat-prose.html (anndata 0.1.0, dataframe/array/string/categorical 0.2.0, CSR 0.1.0); HDF Group, HDF5 File Format Specification 3.0. ClonoLoom's receptor-link and provenance schema version 1."
const annDataDescription = "Raw sparse integer counts for all input cells and feature modalities, with exact barcode/sample receptor links, assignment and chain status, clone sizes, supplied annotations, and saved embeddings. Missing annotations use categorical code -1; unavailable PCA/UMAP rows are NaN, never zero coordinates. Source and computed coordinates have distinct names. No expression refitting, new biological inference or scirpy raw-chain encoding. Original annotation labels and the complete saved phenotype/map documents, parent manifests, definitions, QC, sample metadata and source fingerprints are in uns/clonoloom. Imported annotations retain their reader's text/missing-value semantics, not original pandas dtypes."

type annDataSpec struct {
	Phenotypes       string                `json:"phenotypes"`
	PhenotypesSHA256 string                `json:"phenotypesSha256"`
	Map              string                `json:"map"`
	MapSHA256        string                `json:"mapSha256"`
	Input            expressionMapSpec     `json:"input"`
	Receptors        []clonotype.Clonotype `json:"receptors"`
	Notice           export.Notice         `json:"notice"`
	Method           int                   `json:"method"`
}

func (s annDataSpec) artifactID() string {
	sum := sha256.Sum256(mustJSON(struct {
		Phenotypes, Hash, Map, MapHash string
		Method                         int
	}{s.Phenotypes, s.PhenotypesSHA256, s.Map, s.MapSHA256, s.Method}))
	return "h5ad-" + hex.EncodeToString(sum[:])[:24]
}

type annDataDocument struct {
	Format      string        `json:"format"`
	Version     int           `json:"version"`
	Description string        `json:"description"`
	Phenotypes  string        `json:"phenotypes"`
	Dataset     string        `json:"dataset"`
	Expression  string        `json:"expression"`
	Cells       int           `json:"cells"`
	Features    int           `json:"features"`
	Entries     int           `json:"entries"`
	Embeddings  []string      `json:"embeddings"`
	Notice      export.Notice `json:"notice"`
}

func (a *app) annDataParents(s annDataSpec) (phenotypeDocument, expressionMapDocument, artifacts.Manifest, artifacts.Manifest, error) {
	var p phenotypeDocument
	var m expressionMapDocument
	var mm artifacts.Manifest
	pm, ph, err := a.readCheckedResult(s.Phenotypes, phenotypeKind, &p)
	if err == nil && (p.Result == nil || p.Format != "clonoloom-phenotypes" || p.Version != phenotype.Version || s.PhenotypesSHA256 != "" && ph != s.PhenotypesSHA256) {
		err = errors.New("unsupported or changed phenotype parent")
	}
	if err != nil {
		return p, m, pm, mm, err
	}
	if p.Map != "" {
		p, m, err = a.modularityParents(modularitySpec{modularityRequest: modularityRequest{Phenotypes: s.Phenotypes}, PhenotypesSHA256: s.PhenotypesSHA256, Map: s.Map, MapSHA256: s.MapSHA256})
		if err == nil {
			mm, err = a.artifacts.Get(p.Map)
		}
	} else if !p.Supplied || s.Map != "" {
		err = errors.New("phenotype map provenance is inconsistent")
	}
	return p, m, pm, mm, err
}
func (a *app) registerAnnData(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/anndata-export", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Phenotypes string `json:"phenotypes"`
			Start      *bool  `json:"start"`
		}
		d := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
		d.DisallowUnknownFields()
		if err := d.Decode(&body); err != nil || body.Phenotypes == "" {
			writeError(w, 400, "Choose a saved phenotype result; only phenotypes and start are accepted.")
			return
		}
		if d.Decode(&struct{}{}) != io.EOF {
			writeError(w, 400, "Send one JSON object.")
			return
		}
		s := annDataSpec{Phenotypes: body.Phenotypes, Method: anndata.Version}
		p, _, pm, mm, err := a.annDataParents(s)
		if err != nil {
			writeError(w, 409, err.Error())
			return
		}
		for _, f := range pm.Files {
			if f.Path == phenotypeKind+".json" {
				s.PhenotypesSHA256 = f.SHA256
			}
		}
		s.Map = p.Map
		for _, f := range mm.Files {
			if f.Path == expressionMapKind+".json" {
				s.MapSHA256 = f.SHA256
			}
		}
		id := s.artifactID()
		if saved, err := a.artifacts.Get(id); err == nil && saved.Kind == annDataKind && saved.State == artifacts.Complete {
			writeJSON(w, map[string]any{"artifact": id, "ready": true})
			return
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
		if pm.Lineage.Job == nil {
			writeError(w, 409, "The phenotype parent has no source-selection job record.")
			return
		}
		var parent phenotypeSpec
		if json.Unmarshal(pm.Lineage.Job.Spec, &parent) != nil || parent.ID != p.Input.ID || parent.Input.Path == "" {
			writeError(w, 409, "The phenotype source selection is unavailable.")
			return
		}
		s.Input = parent.Input
		ds, entry, code, err := a.clonotypeTable(clonotypeRequest{Dataset: p.Dataset, Definition: p.Result.Definition, Filter: p.Result.Filter})
		if err != nil {
			writeError(w, code, err.Error())
			return
		}
		if string(mustJSON(ds.Result.Sources)) != string(mustJSON(p.ReceptorSources)) {
			writeError(w, 409, "Receptor source records changed; recompute phenotypes first.")
			return
		}
		s.Receptors = []clonotype.Clonotype{}
		wanted := map[string]bool{}
		for _, c := range p.Result.Cells {
			if c.Clone != "" {
				wanted[c.Clone] = true
			}
			for _, id := range c.Candidates {
				wanted[id] = true
			}
		}
		for _, c := range entry.table.Clonotypes {
			if wanted[c.ID] {
				s.Receptors = append(s.Receptors, clonotype.Clonotype{ID: c.ID, Chains: c.Chains, Vendor: c.Vendor})
				delete(wanted, c.ID)
			}
		}
		if len(wanted) > 0 {
			writeError(w, 409, "Saved receptor assignments are absent from this definition.")
			return
		}
		s.Notice = a.notice(ds, "cells", p.Result.Filter, p.Result.Definition.Describe(), p.Result.DefinitionID)
		s.Notice.Samples = 1
		for _, source := range p.Input.Sources {
			s.Notice.Files = append(s.Notice.Files, export.File{Name: "expression " + source.Role + ": " + filepath.Base(source.Path), SHA256: source.SHA256})
		}
		s.Notice.What = annDataDescription
		s.Notice.Method = annDataCitation
		j, _, err := a.jobs.Submit(annDataKind, "AnnData export · "+p.Result.Sample, id, s, a.annDataWork(s))
		if err != nil {
			writeError(w, 503, err.Error())
			return
		}
		writeJSON(w, map[string]any{"artifact": id, "ready": false, "job": j.ID})
	})
	mux.HandleFunc("GET /api/anndata-export/{id}", func(w http.ResponseWriter, r *http.Request) {
		var doc annDataDocument
		m, _, err := a.readCheckedResult(r.PathValue("id"), annDataKind, &doc)
		if err != nil {
			writeError(w, 409, err.Error())
			return
		}
		writeJSON(w, map[string]any{"artifact": m, "result": doc})
	})
	mux.HandleFunc("GET /api/anndata-export/{id}/download", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var doc annDataDocument
		m, _, err := a.readCheckedResult(id, annDataKind, &doc)
		if err != nil {
			writeError(w, 409, err.Error())
			return
		}
		if doc.Notice.Restricted() && r.URL.Query().Get("accept") != "1" {
			restricted(w, doc.Notice)
			return
		}
		path, err := a.artifacts.Path(id, "cells.h5ad")
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		sum := sha256.Sum256(raw)
		verified := false
		for _, f := range m.Files {
			if f.Path == "cells.h5ad" && f.SHA256 == hex.EncodeToString(sum[:]) {
				verified = true
			}
		}
		if !verified {
			writeError(w, 409, "AnnData bytes differ from their saved checksum.")
			return
		}
		a.writeZip(w, id, func(z *zip.Writer) error {
			f, err := z.Create("cells.h5ad")
			if err != nil {
				return err
			}
			if _, err = f.Write(raw); err != nil {
				return err
			}
			f, err = z.Create("manifest.json")
			if err != nil {
				return err
			}
			if _, err = f.Write(mustJSON(m)); err != nil {
				return err
			}
			f, err = z.Create(annDataKind + ".json")
			if err != nil {
				return err
			}
			if _, err = f.Write(mustJSON(doc)); err != nil {
				return err
			}
			return writeNotice(z, doc.Notice)
		})
	})
}
func (a *app) annDataWork(s annDataSpec) jobs.Func {
	return func(ctx context.Context, run *jobs.Run) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if s.Method != anndata.Version {
			return errors.New("AnnData export version changed; prepare a new export")
		}
		run.Progress(0, "Verifying parents and raw expression sources")
		p, m, pm, mm, err := a.annDataParents(s)
		if err != nil {
			return err
		}
		in, err := expression.OpenSelected(ctx, s.Input.Path, s.Input.CellsPath, s.Input.AnnData)
		if err != nil {
			return err
		}
		if in.ID != p.Input.ID || in.ID != s.Input.ID {
			return errors.New("expression sources changed; reopen them and recompute phenotypes")
		}
		var analysis *expression.AnalysisResult
		if p.Map != "" {
			analysis = m.Result
		}
		meta := map[string]string{"phenotypes_json": string(mustJSON(p)), "phenotype_manifest_json": string(mustJSON(pm)), "receptors_json": string(mustJSON(s.Receptors)), "notice": s.Notice.Text(), "description": annDataDescription, "citation": annDataCitation, "software_json": string(mustJSON(artifacts.Software{Name: "clonoloom", Version: version, Commit: buildCommit()}))}
		if analysis != nil {
			meta["expression_map_json"] = string(mustJSON(m))
			meta["expression_map_manifest_json"] = string(mustJSON(mm))
		}
		run.Progress(.3, "Writing sparse counts, linked annotations and embeddings")
		raw, err := anndata.Export(ctx, in, p.Result, analysis, meta)
		if err != nil {
			return err
		}
		id := s.artifactID()
		dir, err := a.artifacts.Stage(id)
		if err != nil {
			return err
		}
		defer a.artifacts.Discard(id)
		if err = os.WriteFile(filepath.Join(dir, "cells.h5ad"), raw, 0644); err != nil {
			return err
		}
		embeddings := []string{}
		if in.Cells != nil && in.Cells.Coverage.Mapped > 0 {
			embeddings = append(embeddings, "X_supplied")
		}
		if analysis != nil {
			embeddings = append(embeddings, "X_clonoloom_pca", "X_clonoloom_umap")
		}
		doc := annDataDocument{Format: "clonoloom-anndata-export", Version: anndata.Version, Description: annDataDescription, Phenotypes: s.Phenotypes, Dataset: p.Dataset, Expression: in.ID, Cells: len(in.Matrix.Barcodes), Features: len(in.Matrix.Features), Entries: len(in.Matrix.Counts), Embeddings: embeddings, Notice: s.Notice}
		if err = os.WriteFile(filepath.Join(dir, annDataKind+".json"), mustJSON(doc), 0644); err != nil {
			return err
		}
		if err = ctx.Err(); err != nil {
			return err
		}
		run.Validating()
		parents := []artifacts.Parent{{Kind: phenotypeKind, ID: s.Phenotypes, SHA256: s.PhenotypesSHA256}}
		if s.Map != "" {
			parents = append(parents, artifacts.Parent{Kind: expressionMapKind, ID: s.Map, SHA256: s.MapSHA256})
		}
		manifest := artifacts.Manifest{ID: id, Kind: annDataKind, Name: "AnnData export · " + p.Result.Sample, Dataset: p.Dataset, Lineage: artifacts.Lineage{Parents: parents, Job: &artifacts.Job{ID: run.ID(), Kind: annDataKind, Spec: mustJSON(s)}, Parameters: mustJSON(map[string]any{"phenotypes": s.Phenotypes, "matrix": "raw counts", "observations": "all expression cells"}), Method: artifacts.Method{Name: "AnnData export with sample-scoped receptor links", Version: fmt.Sprint(anndata.Version), Citation: annDataCitation}, Software: []artifacts.Software{{Name: "clonoloom", Version: version, Commit: buildCommit()}}}, Files: []artifacts.File{{Path: "cells.h5ad", Role: "anndata"}, {Path: annDataKind + ".json", Role: annDataKind}}, Summary: mustJSON(doc)}
		saved, err := a.artifacts.Promote(manifest, func(dir string, _ *artifacts.Manifest) []artifacts.Finding {
			back, e := expression.OpenSelected(ctx, filepath.Join(dir, "cells.h5ad"), "", &expression.AnnDataSelection{Layer: "X", FeatureName: "feature_name", FeatureType: "feature_type"})
			if e == nil && (string(mustJSON(back.Matrix.Features)) != string(mustJSON(in.Matrix.Features)) || !slices.Equal(back.Matrix.Barcodes, in.Matrix.Barcodes) || !slices.Equal(back.Matrix.Indptr, in.Matrix.Indptr) || !slices.Equal(back.Matrix.Indices, in.Matrix.Indices) || !slices.Equal(back.Matrix.Counts, in.Matrix.Counts)) {
				e = errors.New("raw matrix changed during export")
			}
			if e != nil {
				return []artifacts.Finding{{Severity: "fatal", Code: "anndata.roundtrip", Message: e.Error()}}
			}
			return nil
		})
		if err != nil {
			return err
		}
		if saved.State != artifacts.Complete {
			return errors.New("AnnData export failed validation")
		}
		run.Log("info", "Saved %d cells and %d features in %s.", doc.Cells, doc.Features, id)
		return nil
	}
}
func (a *app) handleAnnDataJobs() {
	a.jobs.Handle(annDataKind, func(raw json.RawMessage) (jobs.Func, error) {
		var s annDataSpec
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		if s.Method != anndata.Version || s.Phenotypes == "" || s.PhenotypesSHA256 == "" || s.Input.ID == "" || s.Input.Path == "" || s.Map != "" && s.MapSHA256 == "" {
			return nil, errors.New("AnnData export parent, source selection or version is unavailable")
		}
		return a.annDataWork(s), nil
	}, nil)
}
