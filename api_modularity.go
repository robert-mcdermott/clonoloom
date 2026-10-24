package main

// Modularity consumes immutable parents, so source files need not be reopened. Neither receptor
// labels nor map coordinates are allowed to refit the expression-neighbor graph.
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
	"clonoloom/internal/modularity"
	"clonoloom/internal/phenotype"
)

const modularityKind = "modularity"
const modularityCitation = "scirpy.tl.clonotype_modularity documented connectivity score; Sturm et al. 2020, doi:10.1093/bioinformatics/btaa611. Phipson & Smyth 2010, doi:10.2202/1544-6115.1585; Hemerik & Goeman 2018, doi:10.1007/s11749-017-0571-1 (random permutation tests). Benjamini & Yekutieli 2001, doi:10.1214/aos/1013699998 (FDR under dependence). Population, binary graph and analytic expectation are ClonoLoom policies."

type modularityRequest struct {
	Phenotypes string             `json:"phenotypes"`
	Options    modularity.Options `json:"options"`
}
type modularitySpec struct {
	modularityRequest
	PhenotypesSHA256 string `json:"phenotypesSha256"`
	Map              string `json:"map"`
	MapSHA256        string `json:"mapSha256"`
	Method           int    `json:"method"`
}

func (s modularitySpec) artifactID() string {
	sum := sha256.Sum256(mustJSON(s))
	return "mod-" + hex.EncodeToString(sum[:])[:24]
}

type modularityDocument struct {
	Format      string `json:"format"`
	Version     int    `json:"version"`
	Description string `json:"description"`
	// Full parent assignment/provenance survives standalone export, including cells absent from
	// the graph. The graph itself remains in the separately fingerprinted expression-map parent.
	Phenotypes         phenotypeDocument  `json:"phenotypes"`
	Map                string             `json:"map"`
	ExpressionExcluded int                `json:"expressionExcluded"`
	Result             *modularity.Result `json:"analysis"`
}

// readCheckedResult verifies both the manifest and the exact bytes subsequently parsed. This
// also catches a file replaced between Verify and ReadFile (not just before Verify).
func (a *app) readCheckedResult(id, kind string, out any) (artifacts.Manifest, string, error) {
	m, err := a.artifacts.Get(id)
	if err != nil || m.Kind != kind || m.State != artifacts.Complete {
		return m, "", fmt.Errorf("choose a complete %s result", kind)
	}
	findings, err := a.artifacts.Verify(id, nil)
	if err != nil || len(findings) > 0 {
		return m, "", fmt.Errorf("%s failed checksum verification", kind)
	}
	file := kind + ".json"
	path, err := a.artifacts.Path(id, file)
	if err != nil {
		return m, "", err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return m, "", err
	}
	sum := sha256.Sum256(raw)
	hash := hex.EncodeToString(sum[:])
	found := false
	for _, f := range m.Files {
		if f.Path == file && f.SHA256 == hash {
			found = true
		}
	}
	if !found {
		return m, "", errors.New("result bytes differ from saved manifest")
	}
	if err = json.Unmarshal(raw, out); err != nil {
		return m, "", err
	}
	return m, hash, nil
}
func (a *app) modularityParents(s modularitySpec) (phenotypeDocument, expressionMapDocument, error) {
	var p phenotypeDocument
	var m expressionMapDocument
	_, ph, err := a.readCheckedResult(s.Phenotypes, phenotypeKind, &p)
	if err != nil {
		return p, m, err
	}
	if p.Result == nil || p.Supplied || p.Map == "" {
		return p, m, errors.New("choose phenotypes made with a computed expression map; imported coordinates have no expression graph")
	}
	if p.Version != phenotype.Version || p.Format != "clonoloom-phenotypes" {
		return p, m, errors.New("unsupported phenotype parent version")
	}
	if s.PhenotypesSHA256 != "" && ph != s.PhenotypesSHA256 {
		return p, m, errors.New("phenotype parent changed")
	}
	_, mh, err := a.readCheckedResult(p.Map, expressionMapKind, &m)
	if err != nil {
		return p, m, err
	}
	if m.Version != expression.AnalysisVersion || m.Format != "clonoloom-expression-map" {
		return p, m, errors.New("unsupported expression-map parent version")
	}
	if m.Result == nil || m.Input.ID != p.Input.ID {
		return p, m, errors.New("phenotype and graph expression inputs differ")
	}
	if s.Map != "" && (s.Map != p.Map || s.MapSHA256 != mh) {
		return p, m, errors.New("expression-map parent changed")
	}
	// The overlay must itself pin these exact graph bytes; don't accept a replacement with the
	// same artifact ID and newly written checksum.
	pm, err := a.artifacts.Get(s.Phenotypes)
	if err != nil {
		return p, m, err
	}
	match := false
	for _, parent := range pm.Lineage.Parents {
		if parent.Kind == expressionMapKind && parent.ID == p.Map && parent.SHA256 == mh {
			match = true
		}
	}
	if !match {
		return p, m, errors.New("phenotype parent does not pin this expression graph")
	}
	return p, m, nil
}
func (a *app) registerModularity(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/modularity", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			modularityRequest
			Start *bool `json:"start"`
		}
		d := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
		d.DisallowUnknownFields()
		if err := d.Decode(&body); err != nil {
			writeError(w, 400, err.Error())
			return
		}
		s := modularitySpec{modularityRequest: body.modularityRequest, Method: modularity.Version}
		var err error
		if s.Options, err = s.Options.Normalize(); err != nil {
			writeError(w, 400, err.Error())
			return
		}
		if s.Phenotypes == "" {
			writeError(w, 400, "Choose a saved phenotype result with a computed expression map.")
			return
		}
		p, m, err := a.modularityParents(s)
		if err != nil {
			writeError(w, 409, err.Error())
			return
		}
		// Pin the bytes verified in this request; the job verifies them again before computing.
		var ignored phenotypeDocument
		_, s.PhenotypesSHA256, err = a.readCheckedResult(s.Phenotypes, phenotypeKind, &ignored)
		if err != nil {
			writeError(w, 409, err.Error())
			return
		}
		s.Map = p.Map
		_, s.MapSHA256, err = a.readCheckedResult(s.Map, expressionMapKind, &m)
		if err != nil {
			writeError(w, 409, err.Error())
			return
		}
		id := s.artifactID()
		if saved, err := a.artifacts.Get(id); err == nil && saved.Kind == modularityKind && saved.State == artifacts.Complete {
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
		j, _, err := a.jobs.Submit(modularityKind, "Clonotype modularity · "+p.Result.Sample, id, s, a.modularityWork(s))
		if err != nil {
			writeError(w, 503, err.Error())
			return
		}
		writeJSON(w, map[string]any{"artifact": id, "ready": false, "job": j.ID})
	})
	mux.HandleFunc("GET /api/modularity/{id}", func(w http.ResponseWriter, r *http.Request) {
		if m, err := a.artifacts.Get(r.PathValue("id")); err != nil || m.Kind != modularityKind || m.State != artifacts.Complete {
			writeError(w, 404, "No complete modularity result with that ID.")
			return
		}
		var doc modularityDocument
		m, _, err := a.readCheckedResult(r.PathValue("id"), modularityKind, &doc)
		if err != nil {
			writeError(w, 409, err.Error())
			return
		}
		writeJSON(w, map[string]any{"artifact": m, "result": doc})
	})
}
func (a *app) modularityWork(s modularitySpec) jobs.Func {
	return func(ctx context.Context, run *jobs.Run) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if s.Method != modularity.Version {
			return errors.New("modularity method version changed; start a new result")
		}
		run.Progress(0, "Verifying phenotype and expression-graph parents")
		p, m, err := a.modularityParents(s)
		if err != nil {
			return err
		}
		lookup := map[string]modularity.Cell{}
		for _, c := range p.Result.Cells {
			if _, ok := lookup[c.Barcode]; ok {
				return errors.New("duplicate phenotype barcode")
			}
			clone := c.Clone
			if c.Status != "Assigned" && c.Status != "Outside size population" {
				clone = ""
			}
			lookup[c.Barcode] = modularity.Cell{Barcode: c.Barcode, Clone: clone, Reason: c.Status}
		}
		cells := make([]modularity.Cell, len(m.Result.Cells))
		for i, c := range m.Result.Cells {
			v, ok := lookup[c.Barcode]
			if !ok {
				return errors.New("graph barcode absent from phenotype input")
			}
			cells[i] = v
		}
		result, err := modularity.Compute(ctx, m.Result.Graph, cells, s.Options, func(p float64) { run.Progress(.1+float64(.8*p), "Sampling size-matched expression subgraphs") })
		if err != nil {
			return err
		}
		description := fmt.Sprintf("Clonotype modularity for sample %s: %d uniquely assigned cells out of %d retained expression-graph cells; %d induced undirected edges. Counts discard positive fuzzy-weight magnitudes and preserve isolated cells. Score = log2((observed edges + 1)/(expected edges + 1)); expectation is analytic for uniform size-matched subsets. Upper-tail tests include ties: exhaustive for at most %d subsets, otherwise %d independent Monte Carlo draws with (tail + 1)/(draws + 1), seed %d. Benjamini–Yekutieli adjusts all %d testable clones within this result; singleton and whole-population clones are not tested. Graph population sizes differ from receptor-sample sizes. The fixed graph comes from PCA neighborhoods, never distances on UMAP. Null assumes clone membership exchangeable among eligible cells in this sample; no conditioning on cell type, batch, receptor family or vertex degree. Results can reflect those factors and do not establish antigen specificity, causality, or subject-level effects. Cells are not independent biological replicates. Corrections do not cover repeated choices of graph, QC, definition or sample.", p.Result.Sample, result.Eligible, result.GraphCells, result.Edges, modularity.ExactLimit, result.Options.Permutations, result.Options.Seed, result.Tested)
		doc := modularityDocument{Format: "clonoloom-modularity", Version: modularity.Version, Description: description, Phenotypes: p, Map: s.Map, ExpressionExcluded: len(p.Result.Cells) - len(cells), Result: result}
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
		file := modularityKind + ".json"
		if err = os.WriteFile(filepath.Join(dir, file), raw, 0644); err != nil {
			return err
		}
		if err = ctx.Err(); err != nil {
			return err
		}
		run.Validating()
		manifest := artifacts.Manifest{ID: id, Kind: modularityKind, Name: "Clonotype modularity · " + p.Result.Sample, Dataset: p.Dataset, Lineage: artifacts.Lineage{Parents: []artifacts.Parent{{Kind: phenotypeKind, ID: s.Phenotypes, SHA256: s.PhenotypesSHA256}, {Kind: expressionMapKind, ID: s.Map, SHA256: s.MapSHA256}}, Job: &artifacts.Job{ID: run.ID(), Kind: modularityKind, Spec: mustJSON(s)}, Parameters: mustJSON(s.modularityRequest), Method: artifacts.Method{Name: "Size-conditioned expression-graph connectivity", Version: fmt.Sprint(modularity.Version), Citation: modularityCitation}, Software: []artifacts.Software{{Name: "clonoloom", Version: version, Commit: buildCommit()}}}, Files: []artifacts.File{{Path: file, Role: modularityKind}}, Summary: mustJSON(map[string]any{"eligible": result.Eligible, "tested": result.Tested, "edges": result.Edges})}
		saved, err := a.artifacts.Promote(manifest, func(dir string, _ *artifacts.Manifest) []artifacts.Finding {
			raw, err := os.ReadFile(filepath.Join(dir, file))
			var back modularityDocument
			if err == nil {
				err = json.Unmarshal(raw, &back)
			}
			if err != nil || back.Result == nil || back.Result.Tested != result.Tested || back.Map != s.Map {
				return []artifacts.Finding{{Severity: "fatal", Code: "modularity.unreadable", Message: "The modularity result could not be read back."}}
			}
			return nil
		})
		if err != nil {
			return err
		}
		if saved.State != artifacts.Complete {
			return errors.New("modularity result failed validation")
		}
		run.Log("info", "Saved %d tested clones in %s.", result.Tested, id)
		return nil
	}
}
func (a *app) handleModularityJobs() {
	a.jobs.Handle(modularityKind, func(raw json.RawMessage) (jobs.Func, error) {
		var s modularitySpec
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		if s.Phenotypes == "" || s.PhenotypesSHA256 == "" || s.Map == "" || s.MapSHA256 == "" || s.Method != modularity.Version {
			return nil, errors.New("modularity parent or method is unavailable")
		}
		if _, err := s.Options.Normalize(); err != nil {
			return nil, err
		}
		return a.modularityWork(s), nil
	}, nil)
}
