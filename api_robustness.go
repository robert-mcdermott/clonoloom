package main

// Definition sensitivity starts from a pinned subject comparison. Capture each
// definition from the same opened source; replay only the saved abundance evidence.
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

	"clonoloom/internal/artifacts"
	"clonoloom/internal/clonotype"
	"clonoloom/internal/cohort"
	"clonoloom/internal/diversity"
	"clonoloom/internal/jobs"
	"clonoloom/internal/robustness"
	"clonoloom/internal/study"
)

const robustnessKind = "definition-robustness"

type robustnessRequest struct {
	Comparison  string                 `json:"comparison"`
	Definitions []clonotype.Definition `json:"definitions"`
}
type robustnessSpec struct {
	robustnessRequest
	SHA256 string             `json:"sha256"`
	Inputs []robustness.Input `json:"inputs"`
	Method int                `json:"method"`
}

func (s robustnessSpec) artifactID() string {
	hash := sha256.Sum256(mustJSON(s))
	return "rb-" + hex.EncodeToString(hash[:])[:24]
}

type robustnessDocument struct {
	Format         string                `json:"format"`
	Version        int                   `json:"version"`
	Description    string                `json:"description"`
	Parent         cohortCompareDocument `json:"parent"`
	ParentManifest artifacts.Manifest    `json:"parentManifest"`
	Result         *robustness.Result    `json:"analysis"`
}

func (a *app) robustnessParent(ctx context.Context, s robustnessSpec) (cohortCompareDocument, artifacts.Manifest, string, error) {
	var p cohortCompareDocument
	m, hash, err := a.readCheckedResult(s.Comparison, cohortCompareKind, &p)
	if err != nil {
		return p, m, hash, err
	}
	fail := func(text string) (cohortCompareDocument, artifacts.Manifest, string, error) {
		return p, m, hash, errors.New(text)
	}
	if s.SHA256 != "" && s.SHA256 != hash {
		return fail("comparison parent changed")
	}
	if p.Format != "clonoloom-cohort-compare" || p.Version != cohort.Version || p.Result == nil || p.Features.Format != "clonoloom-cohort-features" || p.Features.Version != study.FeatureVersion {
		return fail("unsupported comparison parent format")
	}
	f := p.Features
	d, err := study.CheckCohort(f.Selection.Input)
	if err != nil || !d.OK || string(mustJSON(d)) != string(mustJSON(f.Design)) || d.ID != f.Selection.ID || f.Parent.Dataset != m.Dataset || d.Input.Dataset != m.Dataset || f.Parent.DefinitionID != f.Parent.Definition.ID() {
		return fail("comparison design, dataset or definition inconsistent")
	}
	samples := map[string]diversity.Result{}
	for _, v := range f.Parent.Samples {
		if _, ok := samples[v.Name]; ok {
			return fail("duplicate parent sample")
		}
		samples[v.Name] = v.Result
	}
	table, err := study.SubjectFeatures(d.Input, samples, f.Parent.Unit)
	if err != nil || string(mustJSON(table)) != string(mustJSON(f.Table)) {
		return fail("comparison features differ from embedded measurements")
	}
	var spec cohortCompareSpec
	if m.Lineage.Job == nil || json.Unmarshal(m.Lineage.Job.Spec, &spec) != nil || spec.Method != cohort.Version || spec.artifactID() != m.ID || spec.Features != p.ParentManifest.ID {
		return fail("comparison job provenance inconsistent")
	}
	embedded := sha256.Sum256(mustJSON(f))
	if spec.SHA256 != hex.EncodeToString(embedded[:]) {
		return fail("embedded feature checksum differs")
	}
	recomputed, err := cohort.Compute(ctx, d.Input, f.Table, spec.Options)
	if err != nil {
		return p, m, hash, err
	}
	if string(mustJSON(recomputed)) != string(mustJSON(p.Result)) {
		return fail("comparison differs from saved measurements and test settings")
	}
	return p, m, hash, nil
}
func (a *app) registerRobustness(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/definition-robustness", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			robustnessRequest
			Start *bool `json:"start"`
		}
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&body); err != nil {
			writeError(w, 400, err.Error())
			return
		}
		if dec.Decode(&struct{}{}) != io.EOF {
			writeError(w, 400, "Send one JSON object.")
			return
		}
		s := robustnessSpec{robustnessRequest: body.robustnessRequest, Method: robustness.Version}
		p, _, hash, err := a.robustnessParent(r.Context(), s)
		if err != nil {
			writeError(w, 409, err.Error())
			return
		}
		s.SHA256 = hash
		defs, err := robustness.Definitions(p.Features.Parent.Definition, s.Definitions)
		if err != nil {
			writeError(w, 400, err.Error())
			return
		}
		s.Definitions = defs[1:]
		if p.Features.Design.Included > 200 {
			writeError(w, 422, "At most 200 selected subject/group observations are supported.")
			return
		}
		for i, def := range defs {
			if err = r.Context().Err(); err != nil {
				return
			}
			ds, entry, _, e := a.clonotypeTable(clonotypeRequest{Dataset: p.Features.Parent.Dataset, Definition: def, Filter: p.Features.Parent.Filter})
			if e != nil {
				writeError(w, 409, "Open the original receptor dataset to capture definition evidence: "+e.Error())
				return
			}
			in := robustness.Input{Definition: def}
			if string(entry.table.Unit) != p.Features.Parent.Unit {
				e = errors.New("abundance unit differs from baseline")
			} else {
				in, e = robustness.Capture(entry.table, sampleNames(ds))
				in.Definition = def
			}
			if e != nil {
				if i == 0 {
					writeError(w, 422, e.Error())
					return
				}
				in = robustness.Input{Definition: def, Failure: e.Error(), Excluded: entry.table.Excluded}
			}
			s.Inputs = append(s.Inputs, in)
		}
		// Fail stale baselines before submitting work; changed source counts must not
		// become a claimed definition effect.
		table, err := robustness.Features(r.Context(), p.Features.Design.Input, s.Inputs[0], p.Features.Parent.Unit)
		if err != nil || string(mustJSON(table)) != string(mustJSON(p.Features.Table)) {
			writeError(w, 409, "Baseline source measurements differ from the saved comparison.")
			return
		}
		id := s.artifactID()
		if m, e := a.artifacts.Get(id); e == nil && m.Kind == robustnessKind && m.State == artifacts.Complete {
			var back robustnessDocument
			if _, _, e = a.readCheckedResult(id, robustnessKind, &back); e != nil {
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
		j, _, err := a.jobs.Submit(robustnessKind, "Definition audit · "+p.Features.Selection.Name, id, s, a.robustnessWork(s))
		if err != nil {
			writeError(w, 503, err.Error())
			return
		}
		writeJSON(w, map[string]any{"artifact": id, "job": j.ID, "ready": false})
	})
	mux.HandleFunc("GET /api/definition-robustness/{id}", func(w http.ResponseWriter, r *http.Request) {
		var doc robustnessDocument
		m, _, err := a.readCheckedResult(r.PathValue("id"), robustnessKind, &doc)
		if err != nil {
			writeError(w, 409, err.Error())
			return
		}
		writeJSON(w, map[string]any{"artifact": m, "result": doc})
	})
	mux.HandleFunc("GET /api/definition-robustness/{id}/download", func(w http.ResponseWriter, r *http.Request) {
		var doc robustnessDocument
		m, _, err := a.readCheckedResult(r.PathValue("id"), robustnessKind, &doc)
		if err != nil {
			writeError(w, 409, err.Error())
			return
		}
		notice := doc.Parent.Features.Notice
		notice.What = doc.Description
		notice.Method += " " + robustness.Citation
		notice.Derived = true
		notice.Made = m.Created
		for _, sw := range m.Lineage.Software {
			if sw.Name == "clonoloom" {
				notice.Version, notice.Commit = sw.Version, sw.Commit
			}
		}
		if notice.Restricted() && r.URL.Query().Get("accept") != "1" {
			restricted(w, notice)
			return
		}
		a.writeZip(w, m.ID, func(z *zip.Writer) error {
			for _, f := range []struct {
				name  string
				value any
			}{{"manifest.json", m}, {"definition-robustness.json", doc}} {
				w, e := z.Create(f.name)
				if e != nil {
					return e
				}
				if _, e = w.Write(mustJSON(f.value)); e != nil {
					return e
				}
			}
			w, e := z.Create("sensitivity.csv")
			if e != nil {
				return e
			}
			if e = robustness.CSV(w, doc.Result); e != nil {
				return e
			}
			for i, v := range doc.Result.Variants {
				if v.Table == nil {
					continue
				}
				w, e = z.Create(fmt.Sprintf("definition-%d-subject-features.csv", i+1))
				if e != nil {
					return e
				}
				if e = study.FeatureCSV(w, *v.Table); e != nil {
					return e
				}
			}
			return writeNotice(z, notice)
		})
	})
}
func (a *app) robustnessWork(s robustnessSpec) jobs.Func {
	return func(ctx context.Context, run *jobs.Run) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if s.Method != robustness.Version || s.SHA256 == "" {
			return errors.New("definition audit method or parent unavailable")
		}
		p, pm, _, err := a.robustnessParent(ctx, s)
		if err != nil {
			return err
		}
		defs, err := robustness.Definitions(p.Features.Parent.Definition, s.Definitions)
		if err != nil {
			return err
		}
		if len(defs) != len(s.Inputs) {
			return errors.New("captured definitions differ from request")
		}
		for i, d := range defs {
			if string(mustJSON(d)) != string(mustJSON(s.Inputs[i].Definition)) {
				return errors.New("captured definition changed")
			}
		}
		result, err := robustness.Compute(ctx, p.Features.Design.Input, p.Features.Parent.Unit, s.Inputs, p.Result.Options, p.Features.Table, func(v float64) { run.Progress(.1+float64(.8*v), "Recomputing definition sensitivity") })
		if err != nil {
			return err
		}
		if string(mustJSON(result.Variants[0].Analysis)) != string(mustJSON(p.Result)) {
			return errors.New("baseline inference differs from pinned comparison")
		}
		description := fmt.Sprintf("%s Comparison parent %s SHA-256 %s. Dataset %s (%s); saved design %s version %d, %s: %q minus %q, %d subjects and %d observations. QC: %s; abundance unit %s. %d declared definitions, %d features, %d BY slots. All intervals are marginal %.1f%%; the estimate range is descriptive, not a confidence interval.", robustness.Description, s.Comparison, s.SHA256, p.Features.Parent.DatasetName, p.Features.Parent.Dataset, p.Features.Selection.Name, p.Features.Selection.Revision, p.Features.Design.Input.Spec.Layout, p.Result.Levels[1], p.Result.Levels[0], p.Features.Design.Subjects, p.Features.Design.Included, p.Features.Parent.Filter.Describe(), p.Features.Parent.Unit, len(result.Variants), len(result.Summaries), result.Family, 100*p.Result.Options.Confidence)
		for _, f := range p.Features.Design.Findings {
			description += " Design finding: " + f.Title + ". " + f.Detail
		}
		doc := robustnessDocument{Format: "clonoloom-definition-robustness", Version: robustness.Version, Description: description, Parent: p, ParentManifest: pm, Result: result}
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
		file := robustnessKind + ".json"
		if err = os.WriteFile(filepath.Join(dir, file), raw, 0644); err != nil {
			return err
		}
		if err = ctx.Err(); err != nil {
			return err
		}
		run.Validating()
		m := artifacts.Manifest{ID: id, Kind: robustnessKind, Name: "Definition audit · " + p.Features.Selection.Name, Dataset: p.Features.Parent.Dataset, Lineage: artifacts.Lineage{Parents: []artifacts.Parent{{Kind: cohortCompareKind, ID: s.Comparison, SHA256: s.SHA256}}, Job: &artifacts.Job{ID: run.ID(), Kind: robustnessKind, Spec: mustJSON(s)}, Parameters: mustJSON(s.robustnessRequest), Method: artifacts.Method{Name: "Descriptive definition sensitivity", Version: fmt.Sprint(robustness.Version), Citation: robustness.Citation}, Software: []artifacts.Software{{Name: "clonoloom", Version: version, Commit: buildCommit()}}}, Files: []artifacts.File{{Path: file, Role: robustnessKind}}, Summary: mustJSON(map[string]any{"definitions": len(result.Variants), "features": len(result.Summaries), "family": result.Family})}
		saved, err := a.artifacts.Promote(m, func(dir string, _ *artifacts.Manifest) []artifacts.Finding {
			b, e := os.ReadFile(filepath.Join(dir, file))
			var back robustnessDocument
			if e == nil {
				e = json.Unmarshal(b, &back)
			}
			if e != nil || back.Result == nil || string(mustJSON(back)) != string(raw) {
				return []artifacts.Finding{{Severity: "fatal", Code: "robustness.unreadable", Message: "Definition audit failed read-back validation."}}
			}
			return nil
		})
		if err != nil {
			return err
		}
		if saved.State != artifacts.Complete {
			return errors.New("definition audit failed promotion")
		}
		return nil
	}
}
func (a *app) handleRobustnessJobs() {
	a.jobs.Handle(robustnessKind, func(raw json.RawMessage) (jobs.Func, error) {
		var s robustnessSpec
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		if s.Method != robustness.Version || s.SHA256 == "" {
			return nil, errors.New("definition audit method or parent hash unavailable")
		}
		return a.robustnessWork(s), nil
	}, nil)
}
