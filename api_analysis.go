package main

// Analyses of a dataset's samples (diversity, rarefaction) over HTTP: a job computes one over every
// sample under the dataset's clonotype definition and filter, and keeps the result as an artifact
// (internal/artifacts) whose id is made from what it measured: the dataset (its files'
// fingerprints), the definition's meaning, the filter, the options and the method's version, so
// asking again answers the result kept. Results are read back by their id.

import (
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
	"clonoloom/internal/diversity"
	"clonoloom/internal/jobs"
	"clonoloom/internal/qc"
)

// analysis is a kind of analysis: its jobs' and artifacts' kind (and its routes, /api/{kind}, and
// its result's file, {kind}.json), its artifact ids' prefix, its method's version (which changes
// when what it answers changes), its name in words, and its work (given its own kind).
type analysis struct {
	kind    string
	prefix  string
	version int
	title   string
	work    func(a *app, k analysis, spec analysisSpec) jobs.Func
}

// analyses are every kind, registered and resumed alike.
var analyses = []analysis{diversityAnalysis, rarefactionAnalysis}

// analysisSpec is what an analysis measures, and how.
type analysisSpec struct {
	Dataset    string               `json:"dataset"`
	Definition clonotype.Definition `json:"definition"`
	Filter     qc.Filter            `json:"filter"`
	Options    diversity.Options    `json:"options"`
	Method     int                  `json:"method"`
}

// normalized completes a request: the definition normalized (its meaning, not its name, makes the
// id), the options' defaults, the method's version.
func (s analysisSpec) normalized(version int) (analysisSpec, error) {
	def, err := s.Definition.Normalize()
	if err != nil {
		return s, err
	}
	s.Definition = def
	d := diversity.DefaultOptions()
	if s.Options.Replicates == 0 {
		s.Options.Replicates = d.Replicates
	}
	if s.Options.Replicates < 0 || s.Options.Replicates > 2000 {
		return s, errors.New("replicates: from 0 to 2000")
	}
	if s.Options.Seed == 0 {
		s.Options.Seed = d.Seed
	}
	if s.Options.Confidence == 0 {
		s.Options.Confidence = d.Confidence
	}
	if s.Options.Confidence <= 0 || s.Options.Confidence >= 1 {
		return s, errors.New("confidence: between 0 and 1")
	}
	s.Options.Stream = 0
	s.Method = version
	return s, nil
}

func (s analysisSpec) artifactID(prefix string) string {
	key := struct {
		Dataset    string            `json:"dataset"`
		Definition string            `json:"definition"`
		Filter     qc.Filter         `json:"filter"`
		Options    diversity.Options `json:"options"`
		Method     int               `json:"method"`
	}{s.Dataset, s.Definition.Canonical(), s.Filter, s.Options, s.Method}
	raw, _ := json.Marshal(key)
	sum := sha256.Sum256(raw)
	return prefix + hex.EncodeToString(sum[:])[:16]
}

func (a *app) registerAnalyses(mux *http.ServeMux) {
	for _, k := range analyses {
		a.registerAnalysis(mux, k)
	}
}

func (a *app) registerAnalysis(mux *http.ServeMux, k analysis) {
	// POST /api/{kind} {dataset, definition, filter, options, start}: the result kept for these, or
	// a job to make it (start: false only looks).
	mux.HandleFunc("POST /api/"+k.kind, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			analysisSpec
			Start *bool `json:"start"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "Send {\"dataset\", \"definition\", \"filter\", \"options\"}.")
			return
		}
		spec, err := body.analysisSpec.normalized(k.version)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		id := spec.artifactID(k.prefix)
		if m, err := a.artifacts.Get(id); err == nil && m.State == artifacts.Complete {
			writeJSON(w, map[string]any{"artifact": id, "ready": true})
			return
		}
		ds, ok := a.datasets.get(spec.Dataset)
		if !ok {
			writeErrorCode(w, http.StatusNotFound, "not-open", "The dataset is not open: open it again from its files.")
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
		job, _, err := a.jobs.Submit(k.kind, k.title+" of "+ds.Name, id, spec, k.work(a, k, spec))
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, err.Error())
			return
		}
		writeJSON(w, map[string]any{"artifact": id, "ready": false, "job": job.ID})
	})
	mux.HandleFunc("GET /api/"+k.kind+"/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		m, err := a.artifacts.Get(id)
		if err != nil || m.Kind != k.kind {
			writeError(w, http.StatusNotFound, "No such "+k.kind+" result.")
			return
		}
		path, err := a.artifacts.Path(id, k.kind+".json")
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"artifact": %s, "result": %s}`, mustJSON(m), raw)
	})
}

func mustJSON(v any) []byte {
	raw, _ := json.Marshal(v)
	return raw
}

// analysisHead is what every analysis's document begins with: the data and how they were grouped.
type analysisHead struct {
	Format       string               `json:"format"`
	Version      int                  `json:"version"`
	Dataset      string               `json:"dataset"`
	DatasetName  string               `json:"datasetName"`
	Definition   clonotype.Definition `json:"definition"`
	DefinitionID string               `json:"definitionId"`
	Describe     string               `json:"describe"`
	Filter       qc.Filter            `json:"filter"`
	Unit         string               `json:"unit"`
	Options      diversity.Options    `json:"options"`
}

// samplesOf groups the dataset's clonotypes as the spec says, answering the dataset, the
// document's head and each sample's clonotype abundances, by sample.
func (a *app) samplesOf(k analysis, spec analysisSpec, run *jobs.Run) (*dataset, analysisHead, [][]float64, error) {
	ds, ok := a.datasets.get(spec.Dataset)
	if !ok {
		return nil, analysisHead{}, nil, errors.New("the dataset is not open: open it again from its files, then measure again")
	}
	run.Progress(0, "Grouping clonotypes")
	_, entry, _, err := a.clonotypeTable(clonotypeRequest{Dataset: spec.Dataset, Definition: spec.Definition, Filter: spec.Filter})
	if err != nil {
		return nil, analysisHead{}, nil, err
	}
	t := entry.table
	abundances := make([][]float64, len(ds.Result.Table.Samples))
	for i := range t.Clonotypes {
		samples, values := t.Abundances(i)
		for j, s := range samples {
			abundances[s] = append(abundances[s], values[j])
		}
	}
	head := analysisHead{Format: "clonoloom-" + k.kind, Version: 1, Dataset: ds.ID, DatasetName: ds.Name, Definition: spec.Definition, DefinitionID: t.DefinitionID,
		Describe: spec.Definition.Describe(), Filter: spec.Filter, Unit: string(t.Unit), Options: spec.Options}
	return ds, head, abundances, nil
}

// keepAnalysis writes the document as the artifact's {kind}.json and promotes the artifact, once
// it reads back with every sample.
func (a *app) keepAnalysis(k analysis, spec analysisSpec, ds *dataset, run *jobs.Run, doc any, samples int, method artifacts.Method, summary map[string]any) error {
	run.Validating()
	id := spec.artifactID(k.prefix)
	file := k.kind + ".json"
	dir, err := a.artifacts.Stage(id)
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(doc, "", " ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, file), raw, 0o644); err != nil {
		return err
	}
	specJSON, _ := json.Marshal(spec)
	method.Version = fmt.Sprint(k.version)
	m := artifacts.Manifest{ID: id, Kind: k.kind, Name: k.title + " · " + ds.Name, Dataset: ds.ID,
		Lineage: artifacts.Lineage{
			Parents:    []artifacts.Parent{{Kind: "dataset", ID: ds.ID, Name: ds.Name}},
			Job:        &artifacts.Job{ID: run.ID(), Kind: k.kind, Spec: specJSON},
			Method:     method,
			Parameters: specJSON,
			Software:   []artifacts.Software{{Name: "clonoloom", Version: version, Commit: buildCommit()}},
		},
		Files: []artifacts.File{{Path: file, Role: k.kind}},
	}
	_, err = a.artifacts.Promote(m, func(dir string, m *artifacts.Manifest) []artifacts.Finding {
		raw, err := os.ReadFile(filepath.Join(dir, file))
		var back struct {
			Samples []json.RawMessage `json:"samples"`
		}
		if err == nil {
			err = json.Unmarshal(raw, &back)
		}
		if err != nil || len(back.Samples) != samples {
			return []artifacts.Finding{{Severity: "fatal", Code: k.kind + ".unreadable", Message: "The result cannot be read back."}}
		}
		m.Summary, _ = json.Marshal(summary)
		return nil
	})
	if err != nil {
		return err
	}
	run.Log("info", "Measured %d samples; the result is %s.", samples, id)
	return nil
}
