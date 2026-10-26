package main

// Captured sample QC and within-batch descriptive scores; research §5.39.
import (
	"archive/zip"
	"clonoloom/internal/artifacts"
	"clonoloom/internal/clonotype"
	"clonoloom/internal/export"
	"clonoloom/internal/expression"
	"clonoloom/internal/jobs"
	"clonoloom/internal/phenotype"
	"clonoloom/internal/pseudobulk"
	"clonoloom/internal/qc"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const pseudobulkKind = "pseudobulk"

type pseudobulkRequest struct {
	Phenotypes []string           `json:"phenotypes"`
	Options    pseudobulk.Options `json:"options"`
}
type pseudobulkParent struct {
	ID     string            `json:"id"`
	SHA256 string            `json:"sha256"`
	Input  expressionSummary `json:"input"`
}
type pseudobulkSpec struct {
	pseudobulkRequest
	Dataset     string               `json:"dataset"`
	Definition  clonotype.Definition `json:"definition"`
	Filter      qc.Filter            `json:"filter"`
	SizeOptions phenotype.Options    `json:"sizeOptions"`
	Parents     []pseudobulkParent   `json:"parents"`
	Input       pseudobulk.Input     `json:"input"`
	Notice      export.Notice        `json:"notice"`
	Method      int                  `json:"method"`
}

func (s pseudobulkSpec) artifactID() string {
	s.Notice.Made = time.Time{}
	s.Parents = slices.Clone(s.Parents)
	for i := range s.Parents {
		s.Parents[i].Input.Path = ""
		s.Parents[i].Input.Sources = slices.Clone(s.Parents[i].Input.Sources)
		for j := range s.Parents[i].Input.Sources {
			s.Parents[i].Input.Sources[j].Path = ""
		}
	}
	return "pb-" + fmt.Sprintf("%x", sha256.Sum256(mustJSON(s)))[:24]
}

type pseudobulkDocument struct {
	Format      string             `json:"format"`
	Version     int                `json:"version"`
	Description string             `json:"description"`
	Evidence    pseudobulkSpec     `json:"evidence"`
	Analysis    *pseudobulk.Result `json:"analysis"`
}

func (a *app) capturePseudobulk(ctx context.Context, req pseudobulkRequest) (pseudobulkSpec, error) {
	s := pseudobulkSpec{pseudobulkRequest: req, Method: pseudobulk.Version}
	var err error
	s.Options, err = s.Options.Normalize()
	if err != nil {
		return s, err
	}
	if len(s.Phenotypes) < 1 || len(s.Phenotypes) > 32 {
		return s, errors.New("select 1–32 saved phenotype results")
	}
	s.Phenotypes = slices.Clone(s.Phenotypes)
	slices.Sort(s.Phenotypes)
	s.Input = pseudobulk.Input{Options: s.Options, Samples: []pseudobulk.Sample{}}
	entries := 0
	for i, id := range s.Phenotypes {
		if i > 0 && id == s.Phenotypes[i-1] {
			return s, errors.New("duplicate phenotype selection")
		}
		var p phenotypeDocument
		m, hash, e := a.readCheckedResult(id, phenotypeKind, &p)
		if e != nil {
			return s, e
		}
		if p.Result == nil || p.Version != phenotype.Version || p.Format != "clonoloom-phenotypes" || m.Lineage.Job == nil {
			return s, errors.New("unsupported phenotype parent")
		}
		var source phenotypeSpec
		if e = json.Unmarshal(m.Lineage.Job.Spec, &source); e != nil || source.ID != p.Input.ID || source.Dataset != p.Dataset {
			return s, errors.New("phenotype source selection is unavailable")
		}
		if i == 0 {
			s.Dataset = p.Dataset
			s.Definition = p.Result.Definition
			s.Filter = p.Result.Filter
			s.SizeOptions = p.Result.Options
			ds, ok := a.datasets.get(p.Dataset)
			if !ok {
				return s, errors.New("open the receptor dataset before capturing a new pseudobulk result")
			}
			if string(mustJSON(ds.Result.Sources)) != string(mustJSON(p.ReceptorSources)) {
				return s, errors.New("receptor source records changed")
			}
			s.Notice = a.notice(ds, "raw Gene Expression counts", s.Filter, s.Definition.Describe(), p.Result.DefinitionID)
			s.Notice.What = pseudobulk.Description
			s.Notice.Method = pseudobulk.Citation
			s.Notice.Derived = true
			s.Notice.Samples = len(s.Phenotypes)
		} else if p.Dataset != s.Dataset || p.Result.Definition.ID() != s.Definition.ID() || string(mustJSON(p.Result.Filter)) != string(mustJSON(s.Filter)) || p.Result.Options.Population != s.SizeOptions.Population || !slices.Equal(p.Result.Options.Breakpoints, s.SizeOptions.Breakpoints) {
			return s, errors.New("parents must share receptor dataset, definition, QC and receptor-size population/breakpoints")
		}
		in, e := expression.OpenSelected(ctx, source.Input.Path, source.Input.CellsPath, source.Input.AnnData)
		if e != nil {
			return s, e
		}
		if in.ID != p.Input.ID {
			return s, errors.New("expression source fingerprint changed")
		}
		features, cells, bins, e := pseudobulk.Capture(in.Matrix, p.Result, s.Options)
		if e != nil {
			return s, e
		}
		if i == 0 {
			s.Input.Features = features
			s.Input.Bins = bins
		} else if !slices.Equal(features, s.Input.Features) || !slices.Equal(bins, s.Input.Bins) {
			return s, errors.New("feature IDs, names, modalities and order must match exactly; absent features are not zeros")
		}
		for _, c := range cells {
			entries += len(c.Counts)
		}
		if entries > pseudobulk.MaxEntries {
			return s, errors.New("captured sparse counts exceed 2000000 entries")
		}
		s.Input.Samples = append(s.Input.Samples, pseudobulk.Sample{Parent: id, Expression: in.ID, Annotation: p.Annotation, Cells: cells})
		s.Parents = append(s.Parents, pseudobulkParent{ID: id, SHA256: hash, Input: summarizeExpression(in)})
		for _, f := range in.Sources {
			s.Notice.Files = append(s.Notice.Files, export.File{Name: p.Annotation.Name + " expression " + f.Role, SHA256: f.SHA256})
		}
	}
	// Canonical biological observation order, independent of request or hash ordering.
	order := make([]int, len(s.Input.Samples))
	for i := range order {
		order[i] = i
	}
	slices.SortFunc(order, func(i, j int) int {
		a, b := s.Input.Samples[i].Annotation, s.Input.Samples[j].Annotation
		for _, pair := range [][2]string{{a.Subject, b.Subject}, {a.Time, b.Time}, {a.Name, b.Name}} {
			if c := strings.Compare(pair[0], pair[1]); c != 0 {
				return c
			}
		}
		return 0
	})
	samples := slices.Clone(s.Input.Samples)
	parents := slices.Clone(s.Parents)
	for i, j := range order {
		s.Input.Samples[i] = samples[j]
		s.Parents[i] = parents[j]
		s.Phenotypes[i] = parents[j].ID
	}
	if _, err = pseudobulk.Compute(ctx, s.Input); err != nil {
		return s, err
	}
	return s, nil
}
func (a *app) registerPseudobulk(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/pseudobulk", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			pseudobulkRequest
			Start *bool `json:"start"`
		}
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&body); err != nil {
			writeError(w, 400, err.Error())
			return
		}
		if dec.Decode(&struct{}{}) != io.EOF {
			writeError(w, 400, "Send one JSON object.")
			return
		}
		s, err := a.capturePseudobulk(r.Context(), body.pseudobulkRequest)
		if err != nil {
			writeError(w, 422, err.Error())
			return
		}
		id := s.artifactID()
		if m, e := a.artifacts.Get(id); e == nil && m.Kind == pseudobulkKind && m.State == artifacts.Complete {
			var doc pseudobulkDocument
			if _, _, e = a.readCheckedResult(id, pseudobulkKind, &doc); e != nil {
				writeError(w, 409, e.Error())
				return
			}
			writeJSON(w, map[string]any{"artifact": id, "ready": true})
			return
		}
		if body.Start != nil && !*body.Start {
			writeJSON(w, map[string]any{"artifact": id, "ready": false})
			return
		}
		j, _, err := a.jobs.Submit(pseudobulkKind, "Pseudobulk counts · "+s.Notice.Dataset, id, s, a.pseudobulkWork(s))
		if err != nil {
			writeError(w, 503, err.Error())
			return
		}
		writeJSON(w, map[string]any{"artifact": id, "job": j.ID, "ready": false})
	})
	mux.HandleFunc("GET /api/pseudobulk/{id}", func(w http.ResponseWriter, r *http.Request) {
		var doc pseudobulkDocument
		m, _, err := a.readCheckedResult(r.PathValue("id"), pseudobulkKind, &doc)
		if err != nil {
			writeError(w, 409, err.Error())
			return
		}
		writeJSON(w, map[string]any{"artifact": m, "result": doc})
	})
	mux.HandleFunc("GET /api/pseudobulk/{id}/download", func(w http.ResponseWriter, r *http.Request) {
		var doc pseudobulkDocument
		m, _, err := a.readCheckedResult(r.PathValue("id"), pseudobulkKind, &doc)
		if err != nil {
			writeError(w, 409, err.Error())
			return
		}
		n := doc.Evidence.Notice
		n.Made = m.Created
		if n.Restricted() && r.URL.Query().Get("accept") != "1" {
			restricted(w, n)
			return
		}
		a.writeZip(w, m.ID, func(z *zip.Writer) error {
			for _, f := range []struct {
				name  string
				value any
			}{{"pseudobulk.json", doc}, {"manifest.json", m}, {"cells.json", doc.Evidence.Input.Samples}} {
				out, e := z.Create(f.name)
				if e != nil {
					return e
				}
				if _, e = out.Write(mustJSON(f.value)); e != nil {
					return e
				}
			}
			out, e := z.Create("observations.csv")
			if e != nil {
				return e
			}
			if e = pseudobulk.CSV(out, doc.Evidence.Input, doc.Analysis); e != nil {
				return e
			}
			return writeNotice(z, n)
		})
	})
}
func (s pseudobulkSpec) check() error {
	if s.Method != pseudobulk.Version || s.Dataset == "" || string(mustJSON(s.Options)) != string(mustJSON(s.Input.Options)) || len(s.Phenotypes) != len(s.Input.Samples) || len(s.Parents) != len(s.Phenotypes) {
		return errors.New("inconsistent captured pseudobulk provenance")
	}
	for i, id := range s.Phenotypes {
		if s.Input.Samples[i].Parent != id || s.Parents[i].ID != id || s.Parents[i].Input.ID != s.Input.Samples[i].Expression {
			return errors.New("captured parent binding differs")
		}
	}
	return nil
}
func (a *app) pseudobulkWork(s pseudobulkSpec) jobs.Func {
	return func(ctx context.Context, run *jobs.Run) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := s.check(); err != nil {
			return err
		}
		result, err := pseudobulk.Compute(ctx, s.Input)
		if err != nil {
			return err
		}
		run.Progress(.8, "Saving exact pseudobulk counts and cell membership")
		desc := fmt.Sprintf("%s Dataset %s (%s); %d samples, %d unique subjects; annotation %q, minimum cells %d. Definition: %s. QC: %s. Size population %s; inclusive breakpoints %v. %d eligible groups of %d. All categories and absent groups remain in the export. No log transform or pseudocount.", pseudobulk.Description, s.Notice.Dataset, s.Dataset, len(s.Input.Samples), result.Subjects, s.Options.Annotation, s.Options.MinCells, s.Definition.Describe(), s.Filter.Describe(), s.SizeOptions.Population, s.SizeOptions.Breakpoints, result.Eligible, len(result.Groups))
		doc := pseudobulkDocument{Format: "clonoloom-pseudobulk", Version: pseudobulk.Version, Description: desc, Evidence: s, Analysis: result}
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
		if err = os.WriteFile(filepath.Join(dir, "pseudobulk.json"), raw, 0644); err != nil {
			return err
		}
		if err = ctx.Err(); err != nil {
			return err
		}
		run.Validating()
		parents := []artifacts.Parent{}
		for _, p := range s.Parents {
			parents = append(parents, artifacts.Parent{ID: p.ID, Kind: phenotypeKind, SHA256: p.SHA256})
		}
		m := artifacts.Manifest{ID: id, Kind: pseudobulkKind, Name: "Pseudobulk counts · " + s.Notice.Dataset, Dataset: s.Dataset, Lineage: artifacts.Lineage{Parents: parents, Job: &artifacts.Job{ID: run.ID(), Kind: pseudobulkKind, Spec: mustJSON(s)}, Parameters: mustJSON(s.pseudobulkRequest), Method: artifacts.Method{Name: "Exact raw pseudobulk sums", Version: "1", Citation: pseudobulk.Citation}, Software: []artifacts.Software{{Name: "clonoloom", Version: version, Commit: buildCommit()}}}, Files: []artifacts.File{{Path: "pseudobulk.json", Role: pseudobulkKind}}, Summary: mustJSON(map[string]any{"samples": len(s.Input.Samples), "subjects": result.Subjects, "groups": len(result.Groups), "eligible": result.Eligible})}
		saved, err := a.artifacts.Promote(m, func(dir string, _ *artifacts.Manifest) []artifacts.Finding {
			b, e := os.ReadFile(filepath.Join(dir, "pseudobulk.json"))
			var back pseudobulkDocument
			if e == nil {
				e = json.Unmarshal(b, &back)
			}
			if e != nil || back.Analysis == nil || string(mustJSON(back)) != string(raw) {
				return []artifacts.Finding{{Severity: "fatal", Code: "pseudobulk.unreadable", Message: "Pseudobulk result failed read-back validation."}}
			}
			return nil
		})
		if err != nil {
			return err
		}
		if saved.State != artifacts.Complete {
			return errors.New("pseudobulk promotion failed")
		}
		return nil
	}
}
func (a *app) handlePseudobulkJobs() {
	a.jobs.Handle(pseudobulkKind, func(raw json.RawMessage) (jobs.Func, error) {
		var s pseudobulkSpec
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		if err := s.check(); err != nil {
			return nil, err
		}
		return a.pseudobulkWork(s), nil
	}, nil)
}
