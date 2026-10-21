package main

// Diversity (internal/diversity; clonoloom-spec/requirements.md, R1 and R5) over HTTP: a job
// measures every sample of a dataset under its clonotype definition and filter, and keeps the
// result as an artifact (internal/artifacts) whose id is made from what it measured: the dataset
// (its files' fingerprints), the definition's meaning, the filter and the bootstrap's options, so
// asking again answers the result kept. Results are read back by their id.

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
	"clonoloom/internal/diversity"
	"clonoloom/internal/jobs"
	"clonoloom/internal/qc"
)

// diversityMethodVersion changes when the measures or the bootstrap change what they answer.
const diversityMethodVersion = 1

type diversitySpec struct {
	Dataset    string               `json:"dataset"`
	Definition clonotype.Definition `json:"definition"`
	Filter     qc.Filter            `json:"filter"`
	Options    diversity.Options    `json:"options"`
	Method     int                  `json:"method"`
}

// diversityDocument is a diversity artifact's diversity.json.
type diversityDocument struct {
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
	Samples      []diversitySample    `json:"samples"`
}

type diversitySample struct {
	Name   string           `json:"name"`
	Result diversity.Result `json:"result"`
}

// spec completes a request: the definition normalized (its meaning, not its name, makes the id),
// the options' defaults.
func (s diversitySpec) normalized() (diversitySpec, error) {
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
	s.Method = diversityMethodVersion
	return s, nil
}

func (s diversitySpec) artifactID() string {
	key := struct {
		Dataset    string            `json:"dataset"`
		Definition string            `json:"definition"`
		Filter     qc.Filter         `json:"filter"`
		Options    diversity.Options `json:"options"`
		Method     int               `json:"method"`
	}{s.Dataset, s.Definition.Canonical(), s.Filter, s.Options, s.Method}
	raw, _ := json.Marshal(key)
	sum := sha256.Sum256(raw)
	return "div-" + hex.EncodeToString(sum[:])[:16]
}

func (a *app) registerDiversity(mux *http.ServeMux) {
	// POST /api/diversity {dataset, definition, filter, options, start}: the result kept for these,
	// or a job to make it (start: false only looks).
	mux.HandleFunc("POST /api/diversity", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			diversitySpec
			Start *bool `json:"start"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "Send {\"dataset\", \"definition\", \"filter\", \"options\"}.")
			return
		}
		spec, err := body.diversitySpec.normalized()
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		id := spec.artifactID()
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
		job, _, err := a.jobs.Submit("diversity", "Diversity of "+ds.Name, id, spec, a.diversityWork(spec))
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, err.Error())
			return
		}
		writeJSON(w, map[string]any{"artifact": id, "ready": false, "job": job.ID})
	})
	mux.HandleFunc("GET /api/diversity/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		m, err := a.artifacts.Get(id)
		if err != nil || m.Kind != "diversity" {
			writeError(w, http.StatusNotFound, "No such diversity result.")
			return
		}
		path, err := a.artifacts.Path(id, "diversity.json")
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

// diversityWork measures each sample of the dataset and keeps the result as an artifact.
func (a *app) diversityWork(spec diversitySpec) jobs.Func {
	return func(ctx context.Context, run *jobs.Run) error {
		ds, ok := a.datasets.get(spec.Dataset)
		if !ok {
			return errors.New("the dataset is not open: open it again from its files, then measure again")
		}
		run.Progress(0, "Grouping clonotypes")
		_, entry, _, err := a.clonotypeTable(clonotypeRequest{Dataset: spec.Dataset, Definition: spec.Definition, Filter: spec.Filter})
		if err != nil {
			return err
		}
		t := entry.table
		abundances := make([][]float64, len(ds.Result.Table.Samples))
		for i := range t.Clonotypes {
			samples, values := t.Abundances(i)
			for k, s := range samples {
				abundances[s] = append(abundances[s], values[k])
			}
		}
		doc := diversityDocument{Format: "clonoloom-diversity", Version: 1, Dataset: ds.ID, DatasetName: ds.Name, Definition: spec.Definition, DefinitionID: t.DefinitionID,
			Describe: spec.Definition.Describe(), Filter: spec.Filter, Unit: string(t.Unit), Options: spec.Options}
		for i, s := range ds.Result.Table.Samples {
			if err := ctx.Err(); err != nil {
				return err
			}
			run.Progress(float64(i)/float64(len(abundances)), "Measuring "+s.Name)
			opt := spec.Options
			opt.Stream = uint64(i)
			doc.Samples = append(doc.Samples, diversitySample{Name: s.Name, Result: diversity.Measure(abundances[i], opt)})
		}
		run.Validating()
		id := spec.artifactID()
		dir, err := a.artifacts.Stage(id)
		if err != nil {
			return err
		}
		raw, err := json.MarshalIndent(doc, "", " ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "diversity.json"), raw, 0o644); err != nil {
			return err
		}
		specJSON, _ := json.Marshal(spec)
		m := artifacts.Manifest{ID: id, Kind: "diversity", Name: "Diversity · " + ds.Name, Dataset: ds.ID,
			Lineage: artifacts.Lineage{
				Parents: []artifacts.Parent{{Kind: "dataset", ID: ds.ID, Name: ds.Name}},
				Job:     &artifacts.Job{ID: run.ID(), Kind: "diversity", Spec: specJSON},
				Method: artifacts.Method{Name: "Hill numbers, Chao1, ACE and the asymptotic Shannon and Simpson diversity, with bootstrap intervals", Version: fmt.Sprint(diversityMethodVersion),
					Citation: "Chao A, Gotelli NJ, Hsieh TC, et al. Rarefaction and extrapolation with Hill numbers. Ecol Monogr. 2014;84:45–67. doi:10.1890/13-0133.1"},
				Parameters: specJSON,
				Software:   []artifacts.Software{{Name: "clonoloom", Version: version, Commit: buildCommit()}},
			},
			Files: []artifacts.File{{Path: "diversity.json", Role: "diversity"}},
		}
		samples := len(doc.Samples)
		_, err = a.artifacts.Promote(m, func(dir string, m *artifacts.Manifest) []artifacts.Finding {
			raw, err := os.ReadFile(filepath.Join(dir, "diversity.json"))
			var back diversityDocument
			if err == nil {
				err = json.Unmarshal(raw, &back)
			}
			if err != nil || len(back.Samples) != samples {
				return []artifacts.Finding{{Severity: "fatal", Code: "diversity.unreadable", Message: "The result cannot be read back."}}
			}
			m.Summary, _ = json.Marshal(map[string]any{"samples": samples, "unit": doc.Unit, "definition": doc.Definition.Name})
			return nil
		})
		if err != nil {
			return err
		}
		run.Log("info", "Measured %d samples; the result is %s.", samples, id)
		return nil
	}
}
