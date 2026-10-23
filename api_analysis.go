package main

// Analyses of a dataset's samples (diversity, rarefaction, gene usage, overlap, tracking) over
// HTTP: a job computes one over every sample under the dataset's clonotype definition and filter,
// and keeps the result as an artifact (internal/artifacts) whose id is made from what it measured:
// the dataset (its files' fingerprints), the definition's meaning, the filter, the options (and,
// for tracking, the samples' annotation and the clonotypes chosen) and the method's version, so
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
	"clonoloom/internal/bcell"
	"clonoloom/internal/clonotype"
	"clonoloom/internal/diversity"
	"clonoloom/internal/jobs"
	"clonoloom/internal/qc"
	"clonoloom/internal/study"
	"clonoloom/internal/track"
)

// analysis is a kind of analysis: its jobs' and artifacts' kind (and its routes, /api/{kind}, and
// its result's file, {kind}.json), its artifact ids' prefix, its method's version (which changes
// when what it answers changes), its name in words, whether it uses the samples' groups and
// subjects (the spec's Groups and Subjects; ignored otherwise), what of a study over time it uses
// (over: "track", the spec's Study and Track; "compare", its Study and Compare; "class", its Study
// and Class; "attribute", its Study), or what else it takes ("tcrdist", the spec's TCRdist; "pgen",
// its PGen; "", nothing), its work (given its own kind), and what completes its spec.
type analysis struct {
	kind     string
	prefix   string
	version  int
	title    string
	grouped  bool
	subjects bool
	over     string
	work     func(a *app, k analysis, spec analysisSpec) jobs.Func
	// prepare completes a normalized spec with what the app knows (the reference data it uses), or
	// refuses it; nil when nothing is needed.
	prepare func(a *app, spec *analysisSpec) error
}

// analyses are every kind, registered and resumed alike.
var analyses = []analysis{diversityAnalysis, rarefactionAnalysis, usageAnalysis, overlapAnalysis, trackingAnalysis, expansionAnalysis, classesAnalysis, attributionAnalysis, tcrdistAnalysis, pgenAnalysis, neighborhoodAnalysis, networkAnalysis, specificityAnalysis, familyAnalysis, germlineAnalysis, shmAnalysis}

// analysisSpec is what an analysis measures, and how. Groups and Subjects name each sample's group
// (condition) and subject (by sample name), for the analyses that use them; Study is the samples'
// annotation a study over time is read from (internal/study), Track the clonotypes it follows,
// Compare the two time points it tests the clonotypes between, and Class how it classes them
// across time points.
type analysisSpec struct {
	Dataset    string               `json:"dataset"`
	Definition clonotype.Definition `json:"definition"`
	Filter     qc.Filter            `json:"filter"`
	Options    diversity.Options    `json:"options"`
	Groups     map[string]string    `json:"groups,omitempty"`
	Subjects   map[string]string    `json:"subjects,omitempty"`
	Study      []study.Sample       `json:"study,omitempty"`
	Track      *track.Choice        `json:"track,omitempty"`
	Compare    *track.Comparison    `json:"compare,omitempty"`
	Class      *track.Classing      `json:"class,omitempty"`
	TCRdist    *tcrdistOptions      `json:"tcrdist,omitempty"`
	PGen       *pgenOptions         `json:"pgen,omitempty"`
	// Specificity evidence and sequence neighborhoods (api_specificity.go, api_networks.go, api_neighborhoods.go).
	Specificity   *specificityOptions  `json:"specificity,omitempty"`
	Networks      *networkOptions      `json:"networks,omitempty"`
	Neighborhoods *neighborhoodOptions `json:"neighborhoods,omitempty"`
	Families      *bcell.FamilyOptions `json:"families,omitempty"`
	Germlines     *germlineOptions     `json:"germlines,omitempty"`
	SHM           *shmOptions          `json:"shm,omitempty"`
	Method        int                  `json:"method"`
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
	if s.Track != nil {
		c, err := s.Track.Normalize()
		if err != nil {
			return s, err
		}
		s.Track = &c
	}
	if s.Compare != nil {
		c, err := s.Compare.Normalize()
		if err != nil {
			return s, err
		}
		s.Compare = &c
	}
	if s.Class != nil {
		c, err := s.Class.Normalize()
		if err != nil {
			return s, err
		}
		s.Class = &c
	}
	s.Method = version
	return s, nil
}

func (s analysisSpec) artifactID(prefix string) string {
	key := struct {
		Dataset    string            `json:"dataset"`
		Definition string            `json:"definition"`
		Filter     qc.Filter         `json:"filter"`
		Options    diversity.Options `json:"options"`
		Groups     map[string]string `json:"groups,omitempty"`
		Subjects   map[string]string `json:"subjects,omitempty"`
		Study      []study.Sample    `json:"study,omitempty"`
		Track      *track.Choice     `json:"track,omitempty"`
		Compare    *track.Comparison `json:"compare,omitempty"`
		Class      *track.Classing   `json:"class,omitempty"`
		TCRdist    *tcrdistOptions   `json:"tcrdist,omitempty"`
		PGen       *pgenOptions      `json:"pgen,omitempty"`

		Specificity   *specificityOptions  `json:"specificity,omitempty"`
		Networks      *networkOptions      `json:"networks,omitempty"`
		Neighborhoods *neighborhoodOptions `json:"neighborhoods,omitempty"`
		Families      *bcell.FamilyOptions `json:"families,omitempty"`
		Germlines     *germlineOptions     `json:"germlines,omitempty"`
		SHM           *shmOptions          `json:"shm,omitempty"`
		Method        int                  `json:"method"`
	}{s.Dataset, s.Definition.Canonical(), s.Filter, s.Options, s.Groups, s.Subjects, s.Study, s.Track, s.Compare, s.Class, s.TCRdist, s.PGen, s.Specificity, s.Networks, s.Neighborhoods, s.Families, s.Germlines, s.SHM, s.Method}
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
		if !k.grouped {
			body.Groups = nil
		}
		if !k.subjects {
			body.Subjects = nil
		}
		switch k.over {
		case "track":
			body.Compare, body.Class = nil, nil
			if body.Track == nil {
				body.Track = &track.Choice{}
			}
		case "compare":
			body.Track, body.Class = nil, nil
			if body.Compare == nil {
				writeError(w, http.StatusBadRequest, "Send the comparison: {\"compare\": {\"from\", \"to\"}}, two time points' keys.")
				return
			}
		case "class":
			body.Track, body.Compare = nil, nil
			if body.Class == nil {
				writeError(w, http.StatusBadRequest, "Send the classing: {\"class\": {\"base\"}}, the base time point's key.")
				return
			}
		case "attribute":
			body.Track, body.Compare, body.Class = nil, nil, nil
		case "tcrdist":
			body.Study, body.Track, body.Compare, body.Class = nil, nil, nil, nil
			if body.TCRdist == nil {
				body.TCRdist = &tcrdistOptions{}
			}
		case "pgen":
			body.Study, body.Track, body.Compare, body.Class = nil, nil, nil, nil
			if body.PGen == nil {
				body.PGen = &pgenOptions{}
			}
		case "specificity":
			body.Study, body.Track, body.Compare, body.Class = nil, nil, nil, nil
			if body.Specificity == nil {
				body.Specificity = &specificityOptions{}
			}
		case "networks":
			body.Study, body.Track, body.Compare, body.Class = nil, nil, nil, nil
			if body.Networks == nil {
				body.Networks = &networkOptions{}
			}
		case "shm":
			body.Study, body.Track, body.Compare, body.Class = nil, nil, nil, nil
			if body.SHM == nil {
				body.SHM = &shmOptions{}
			}
			body.Definition = clonotype.Definition{}
			body.Options = diversity.Options{}
		case "germlines":
			body.Study, body.Track, body.Compare, body.Class = nil, nil, nil, nil
			if body.Germlines == nil {
				body.Germlines = &germlineOptions{}
			}
			body.Definition = clonotype.Definition{}
			body.Options = diversity.Options{}
		case "families":
			body.Study, body.Track, body.Compare, body.Class = nil, nil, nil, nil
			if body.Families == nil {
				body.Families = &bcell.FamilyOptions{}
			}
			// Families operate on rearrangements, independently of the active clonotype definition.
			body.Definition = clonotype.Definition{}
			body.Options = diversity.Options{}
		case "neighborhoods":
			body.Study, body.Track, body.Compare, body.Class = nil, nil, nil, nil
			if body.Neighborhoods == nil {
				body.Neighborhoods = &neighborhoodOptions{}
			}
		default:
			body.Study, body.Track, body.Compare, body.Class = nil, nil, nil, nil
		}
		if k.over != "shm" {
			body.SHM = nil
		}
		if k.over != "germlines" {
			body.Germlines = nil
		}
		if k.over != "families" {
			body.Families = nil
		}
		if k.over != "specificity" {
			body.Specificity = nil
		}
		if k.over != "networks" {
			body.Networks = nil
		}
		if k.over != "tcrdist" {
			body.TCRdist = nil
		}
		if k.over != "pgen" {
			body.PGen = nil
		}
		if k.over != "neighborhoods" {
			body.Neighborhoods = nil
		}
		spec, err := body.analysisSpec.normalized(k.version)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if k.prepare != nil {
			if err := k.prepare(a, &spec); err != nil {
				writeErrorCode(w, http.StatusBadRequest, prepareCode(err), err.Error())
				return
			}
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
// document's head, the clonotype table and each sample's clonotype abundances, by sample.
func (a *app) samplesOf(k analysis, spec analysisSpec, run *jobs.Run) (*dataset, analysisHead, *clonotype.Table, [][]float64, error) {
	ds, ok := a.datasets.get(spec.Dataset)
	if !ok {
		return nil, analysisHead{}, nil, nil, errors.New("the dataset is not open: open it again from its files, then measure again")
	}
	run.Progress(0, "Grouping clonotypes")
	_, entry, _, err := a.clonotypeTable(clonotypeRequest{Dataset: spec.Dataset, Definition: spec.Definition, Filter: spec.Filter})
	if err != nil {
		return nil, analysisHead{}, nil, nil, err
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
	return ds, head, t, abundances, nil
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
	raw, err := json.Marshal(doc)
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
	if spec.SHM != nil {
		m.Lineage.Parents = append(m.Lineage.Parents, artifacts.Parent{Kind: "germlines", ID: spec.SHM.Germlines, SHA256: spec.SHM.GermlineSHA256})
	}
	if spec.Germlines != nil {
		m.Lineage.Parents = append(m.Lineage.Parents, artifacts.Parent{Kind: "families", ID: spec.Germlines.Families, SHA256: spec.Germlines.FamilySHA256})
		for _, f := range spec.Germlines.Files {
			m.Lineage.Parents = append(m.Lineage.Parents, artifacts.Parent{Kind: "reference", ID: f.Reference, Name: f.File, SHA256: f.SHA256})
		}
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
