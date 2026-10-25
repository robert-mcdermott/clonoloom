package main

// Cohort comparisons consume a verified feature artifact. The saved design and
// measurements are rechecked without reopening source files or refitting diversity.
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
	"clonoloom/internal/cohort"
	"clonoloom/internal/diversity"
	"clonoloom/internal/jobs"
	"clonoloom/internal/study"
)

const cohortCompareKind = "cohort-compare"

type cohortCompareRequest struct {
	Features string         `json:"features"`
	Options  cohort.Options `json:"options"`
}
type cohortCompareSpec struct {
	cohortCompareRequest
	SHA256 string `json:"sha256"`
	Method int    `json:"method"`
}

func (s cohortCompareSpec) artifactID() string {
	sum := sha256.Sum256(mustJSON(s))
	return "cc-" + hex.EncodeToString(sum[:])[:24]
}

type cohortCompareDocument struct {
	Format         string                 `json:"format"`
	Version        int                    `json:"version"`
	Description    string                 `json:"description"`
	Features       cohortFeaturesDocument `json:"features"`
	ParentManifest artifacts.Manifest     `json:"parentManifest"`
	Result         *cohort.Result         `json:"analysis"`
}

func (a *app) cohortCompareParent(s cohortCompareSpec) (cohortFeaturesDocument, artifacts.Manifest, string, error) {
	var p cohortFeaturesDocument
	m, hash, err := a.readCheckedResult(s.Features, cohortFeaturesKind, &p)
	if err != nil {
		return p, m, hash, err
	}
	fail := func(message string) (cohortFeaturesDocument, artifacts.Manifest, string, error) {
		return p, m, hash, errors.New(message)
	}
	if s.SHA256 != "" && s.SHA256 != hash {
		return fail("feature parent changed after selection")
	}
	if p.Format != "clonoloom-cohort-features" || p.Version != study.FeatureVersion || p.Parent.Dataset != m.Dataset || p.Parent.Format != "clonoloom-diversity" || p.Parent.Version != 1 || p.Parent.DefinitionID != p.Parent.Definition.ID() {
		return fail("feature parent format or definition differs")
	}
	d, err := study.CheckCohort(p.Selection.Input)
	if err != nil {
		return p, m, hash, err
	}
	if !d.OK || d.ID != p.Selection.ID || p.Parent.Dataset != d.Input.Dataset || string(mustJSON(d)) != string(mustJSON(p.Design)) {
		return fail("saved feature design is inconsistent")
	}
	samples := map[string]diversity.Result{}
	for _, sample := range p.Parent.Samples {
		if _, exists := samples[sample.Name]; exists {
			return fail("duplicate feature-parent sample")
		}
		samples[sample.Name] = sample.Result
	}
	table, err := study.SubjectFeatures(d.Input, samples, p.Parent.Unit)
	if err != nil {
		return p, m, hash, err
	}
	if string(mustJSON(table)) != string(mustJSON(p.Table)) {
		return fail("saved features differ from their embedded measurements")
	}
	return p, m, hash, nil
}
func (a *app) registerCohortCompare(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/cohort-compare", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			cohortCompareRequest
			Start *bool `json:"start"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&body); err != nil {
			writeError(w, 400, err.Error())
			return
		}
		if decoder.Decode(&struct{}{}) != io.EOF {
			writeError(w, 400, "Send one JSON object.")
			return
		}
		s := cohortCompareSpec{cohortCompareRequest: body.cohortCompareRequest, Method: cohort.Version}
		var err error
		s.Options, err = s.Options.Normalize()
		if err != nil {
			writeError(w, 400, err.Error())
			return
		}
		p, _, hash, err := a.cohortCompareParent(s)
		if err != nil {
			writeError(w, 409, err.Error())
			return
		}
		s.SHA256 = hash
		for _, id := range s.Options.Features {
			found := false
			for _, f := range p.Table.Catalog {
				if f.ID == id {
					found = true
				}
			}
			if !found {
				writeError(w, 400, "Unknown selected feature: "+id)
				return
			}
		}
		id := s.artifactID()
		if saved, err := a.artifacts.Get(id); err == nil && saved.Kind == cohortCompareKind && saved.State == artifacts.Complete {
			var back cohortCompareDocument
			if _, _, err = a.readCheckedResult(id, cohortCompareKind, &back); err != nil {
				writeError(w, 409, err.Error())
				return
			}
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
		j, _, err := a.jobs.Submit(cohortCompareKind, "Group comparison · "+p.Selection.Name, id, s, a.cohortCompareWork(s))
		if err != nil {
			writeError(w, 503, err.Error())
			return
		}
		writeJSON(w, map[string]any{"artifact": id, "ready": false, "job": j.ID})
	})
	mux.HandleFunc("GET /api/cohort-compare/{id}", func(w http.ResponseWriter, r *http.Request) {
		var doc cohortCompareDocument
		m, _, err := a.readCheckedResult(r.PathValue("id"), cohortCompareKind, &doc)
		if err != nil {
			writeError(w, 409, err.Error())
			return
		}
		writeJSON(w, map[string]any{"artifact": m, "result": doc})
	})
	mux.HandleFunc("GET /api/cohort-compare/{id}/download", func(w http.ResponseWriter, r *http.Request) {
		var doc cohortCompareDocument
		m, _, err := a.readCheckedResult(r.PathValue("id"), cohortCompareKind, &doc)
		if err != nil {
			writeError(w, 409, err.Error())
			return
		}
		notice := doc.Features.Notice
		notice.What = doc.Description
		notice.Method += " " + cohort.Citation
		notice.Derived = true
		notice.Made = m.Created
		for _, software := range m.Lineage.Software {
			if software.Name == "clonoloom" {
				notice.Version, notice.Commit = software.Version, software.Commit
			}
		}
		if notice.Restricted() && r.URL.Query().Get("accept") != "1" {
			restricted(w, notice)
			return
		}
		a.writeZip(w, m.ID, func(z *zip.Writer) error {
			for _, file := range []struct {
				name  string
				value any
			}{{"manifest.json", m}, {"cohort-compare.json", doc}} {
				f, err := z.Create(file.name)
				if err != nil {
					return err
				}
				if _, err = f.Write(mustJSON(file.value)); err != nil {
					return err
				}
			}
			f, err := z.Create("comparisons.csv")
			if err != nil {
				return err
			}
			if err = cohort.CSV(f, doc.Result); err != nil {
				return err
			}
			f, err = z.Create("subject-features.csv")
			if err != nil {
				return err
			}
			if err = study.FeatureCSV(f, doc.Features.Table); err != nil {
				return err
			}
			return writeNotice(z, notice)
		})
	})
}
func (a *app) cohortCompareWork(s cohortCompareSpec) jobs.Func {
	return func(ctx context.Context, run *jobs.Run) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if s.Method != cohort.Version || s.SHA256 == "" {
			return errors.New("comparison method or pinned parent unavailable")
		}
		run.Progress(0, "Verifying saved subject features")
		parent, pm, _, err := a.cohortCompareParent(s)
		if err != nil {
			return err
		}
		run.Progress(.1, "Computing subject-level tests and effects")
		result, err := cohort.Compute(ctx, parent.Design.Input, parent.Table, s.Options)
		if err != nil {
			return err
		}
		description := fmt.Sprintf("%s Feature parent %s (SHA-256 %s); design %s version %d (%s). Dataset %s (%s); definition %s: %s; QC: %s; abundance %s. %s", result.Description, s.Features, s.SHA256, parent.Selection.Name, parent.Selection.Revision, parent.Design.ID, parent.Parent.DatasetName, parent.Parent.Dataset, parent.Parent.DefinitionID, parent.Parent.Describe, parent.Parent.Filter.Describe(), parent.Parent.Unit, "Saved feature measurements remain at their observed sample depths; no diversity refitting.")
		for _, finding := range parent.Design.Findings {
			description += " Design finding: " + finding.Title + ". " + finding.Detail
		}
		doc := cohortCompareDocument{Format: "clonoloom-cohort-compare", Version: cohort.Version, Description: description, Features: parent, ParentManifest: pm, Result: result}
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
		file := cohortCompareKind + ".json"
		if err = os.WriteFile(filepath.Join(dir, file), raw, 0644); err != nil {
			return err
		}
		if err = ctx.Err(); err != nil {
			return err
		}
		run.Validating()
		m := artifacts.Manifest{ID: id, Kind: cohortCompareKind, Name: "Group comparison · " + parent.Selection.Name, Dataset: parent.Parent.Dataset, Lineage: artifacts.Lineage{Parents: []artifacts.Parent{{Kind: cohortFeaturesKind, ID: s.Features, SHA256: s.SHA256}}, Job: &artifacts.Job{ID: run.ID(), Kind: cohortCompareKind, Spec: mustJSON(s)}, Parameters: mustJSON(s.cohortCompareRequest), Method: artifacts.Method{Name: "Subject-level group comparison", Version: fmt.Sprint(cohort.Version), Citation: cohort.Citation}, Software: []artifacts.Software{{Name: "clonoloom", Version: version, Commit: buildCommit()}}}, Files: []artifacts.File{{Path: file, Role: cohortCompareKind}}, Summary: mustJSON(map[string]any{"family": result.Family, "tested": result.Tested, "features": len(result.Comparisons)})}
		saved, err := a.artifacts.Promote(m, func(dir string, _ *artifacts.Manifest) []artifacts.Finding {
			bytes, err := os.ReadFile(filepath.Join(dir, file))
			var back cohortCompareDocument
			if err == nil {
				err = json.Unmarshal(bytes, &back)
			}
			if err != nil || back.Result == nil || string(mustJSON(back)) != string(raw) {
				return []artifacts.Finding{{Severity: "fatal", Code: "cohort-compare.unreadable", Message: "Comparison failed read-back validation."}}
			}
			return nil
		})
		if err != nil {
			return err
		}
		if saved.State != artifacts.Complete {
			return errors.New("comparison failed validation")
		}
		run.Log("info", "Saved %d testable tests in a family of %d: %s.", result.Tested, result.Family, id)
		return nil
	}
}
func (a *app) handleCohortCompareJobs() {
	a.jobs.Handle(cohortCompareKind, func(raw json.RawMessage) (jobs.Func, error) {
		var s cohortCompareSpec
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		if s.Method != cohort.Version || s.SHA256 == "" {
			return nil, errors.New("comparison method or pinned parent unavailable")
		}
		if _, err := s.Options.Normalize(); err != nil {
			return nil, err
		}
		return a.cohortCompareWork(s), nil
	}, nil)
}
