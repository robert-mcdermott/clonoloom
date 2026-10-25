package main

// A feature table preserves its checked design and exact diversity parent. Only the
// initial source binding needs an open dataset; reopening and export are source-free.
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
	"time"

	"clonoloom/internal/artifacts"
	"clonoloom/internal/diversity"
	"clonoloom/internal/export"
	"clonoloom/internal/jobs"
	"clonoloom/internal/study"
)

const cohortFeaturesKind = "cohort-features"

type cohortFeatureDesign struct {
	Record   string            `json:"record"`
	Name     string            `json:"name"`
	Revision int               `json:"revision"`
	ID       string            `json:"id"`
	Input    study.CohortInput `json:"input"`
}
type cohortFeaturesRequest struct {
	Design    cohortFeatureDesign `json:"design"`
	Diversity string              `json:"diversity"`
}
type cohortFeaturesSpec struct {
	cohortFeaturesRequest
	SHA256 string        `json:"sha256"`
	Notice export.Notice `json:"notice"`
	Method int           `json:"method"`
}

func (s cohortFeaturesSpec) artifactID() string {
	// Creation time does not change the identity of otherwise identical evidence.
	s.Notice.Made = time.Time{}
	sum := sha256.Sum256(mustJSON(s))
	return "cf-" + hex.EncodeToString(sum[:])[:24]
}

type cohortFeaturesDocument struct {
	Format         string              `json:"format"`
	Version        int                 `json:"version"`
	Description    string              `json:"description"`
	Selection      cohortFeatureDesign `json:"selection"`
	Design         study.CohortDesign  `json:"design"`
	Parent         diversityDocument   `json:"parent"`
	ParentManifest artifacts.Manifest  `json:"parentManifest"`
	Table          study.FeatureTable  `json:"table"`
	Notice         export.Notice       `json:"notice"`
}

func (a *app) cohortFeatureParents(s cohortFeaturesSpec) (cohortFeaturesDocument, string, error) {
	var doc cohortFeaturesDocument
	d, err := study.CheckCohort(s.Design.Input)
	if err != nil {
		return doc, "", err
	}
	if !d.OK || d.ID != s.Design.ID || s.Design.Record == "" || s.Design.Name == "" || len(s.Design.Name) > 4096 || len(s.Design.Record) > 4096 || s.Design.Revision < 1 {
		return doc, "", errors.New("choose an unchanged saved design that passes structural checks")
	}
	var p diversityDocument
	m, hash, err := a.readCheckedResult(s.Diversity, "diversity", &p)
	if err != nil {
		return doc, "", err
	}
	if s.SHA256 != "" && s.SHA256 != hash {
		return doc, "", errors.New("diversity parent changed after selection")
	}
	if p.Format != "clonoloom-diversity" || p.Version != 1 || p.Dataset != d.Input.Dataset || m.Dataset != p.Dataset || p.DefinitionID != p.Definition.ID() {
		return doc, "", errors.New("diversity parent format, dataset or definition differs")
	}
	var spec analysisSpec
	if m.Lineage.Job == nil || json.Unmarshal(m.Lineage.Job.Spec, &spec) != nil || spec.Method != 1 || spec.Dataset != p.Dataset || spec.artifactID("div-") != s.Diversity || spec.Definition.Canonical() != p.Definition.Canonical() || string(mustJSON(spec.Filter)) != string(mustJSON(p.Filter)) || spec.Options != p.Options {
		return doc, "", errors.New("diversity parent provenance is inconsistent")
	}
	samples := map[string]diversity.Result{}
	for _, sample := range p.Samples {
		if _, exists := samples[sample.Name]; exists {
			return doc, "", errors.New("duplicate sample in diversity parent")
		}
		samples[sample.Name] = sample.Result
	}
	table, err := study.SubjectFeatures(d.Input, samples, p.Unit)
	if err != nil {
		return doc, "", err
	}
	description := fmt.Sprintf("%s Design %s version %d (%s): %s groups %q then %q, %d subjects, %d selected observations; diversity parent %s (SHA-256 %s), definition %s (%s), QC: %s; abundance unit %s. All design annotations and exclusions are saved snapshots.", study.FeatureDescription, s.Design.Name, s.Design.Revision, d.ID, d.Input.Spec.Layout, d.Input.Spec.Levels[0], d.Input.Spec.Levels[1], d.Subjects, d.Included, s.Diversity, hash, p.DefinitionID, p.Definition.Describe(), p.Filter.Describe(), p.Unit)
	doc = cohortFeaturesDocument{Format: "clonoloom-cohort-features", Version: study.FeatureVersion, Description: description, Selection: s.Design, Design: d, Parent: p, ParentManifest: m, Table: table, Notice: s.Notice}
	return doc, hash, nil
}
func (a *app) registerCohortFeatures(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/cohort-features", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			cohortFeaturesRequest
			Start *bool `json:"start"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&body); err != nil {
			writeError(w, 400, err.Error())
			return
		}
		if decoder.Decode(&struct{}{}) != io.EOF {
			writeError(w, 400, "Send one JSON object.")
			return
		}
		s := cohortFeaturesSpec{cohortFeaturesRequest: body.cohortFeaturesRequest, Method: study.FeatureVersion}
		doc, hash, err := a.cohortFeatureParents(s)
		if err != nil {
			writeError(w, 409, err.Error())
			return
		}
		s.SHA256 = hash
		s.Design.Input = doc.Design.Input
		ds, ok := a.datasets.get(doc.Parent.Dataset)
		if !ok {
			writeError(w, 409, "Open the source dataset to bind a new feature table; saved tables reopen without it.")
			return
		}
		names := sampleNames(ds)
		if len(names) != len(doc.Design.Input.Samples) {
			writeError(w, 409, "Design samples differ from the source dataset.")
			return
		}
		present := map[string]bool{}
		for _, v := range doc.Design.Input.Samples {
			present[v.Name] = true
		}
		for _, name := range names {
			if !present[name] {
				writeError(w, 409, "Design samples differ from the source dataset.")
				return
			}
		}
		s.Notice = a.notice(ds, doc.Parent.Unit, doc.Parent.Filter, doc.Parent.Definition.Describe(), doc.Parent.DefinitionID)
		for _, source := range ds.Result.Metadata {
			s.Notice.Files = append(s.Notice.Files, export.File{Name: source.Name, SHA256: source.SHA256})
		}
		s.Notice.What = doc.Description
		s.Notice.Method += " " + study.FeatureCitation
		s.Notice.Derived = true
		id := s.artifactID()
		if saved, err := a.artifacts.Get(id); err == nil && saved.Kind == cohortFeaturesKind && saved.State == artifacts.Complete {
			var back cohortFeaturesDocument
			if _, _, err = a.readCheckedResult(id, cohortFeaturesKind, &back); err != nil {
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
		job, _, err := a.jobs.Submit(cohortFeaturesKind, "Subject features · "+s.Design.Name, id, s, a.cohortFeaturesWork(s))
		if err != nil {
			writeError(w, 503, err.Error())
			return
		}
		writeJSON(w, map[string]any{"artifact": id, "ready": false, "job": job.ID})
	})
	mux.HandleFunc("GET /api/cohort-features/{id}", func(w http.ResponseWriter, r *http.Request) {
		var doc cohortFeaturesDocument
		m, _, err := a.readCheckedResult(r.PathValue("id"), cohortFeaturesKind, &doc)
		if err != nil {
			writeError(w, 409, err.Error())
			return
		}
		writeJSON(w, map[string]any{"artifact": m, "result": doc})
	})
	mux.HandleFunc("GET /api/cohort-features/{id}/download", func(w http.ResponseWriter, r *http.Request) {
		var doc cohortFeaturesDocument
		m, _, err := a.readCheckedResult(r.PathValue("id"), cohortFeaturesKind, &doc)
		if err != nil {
			writeError(w, 409, err.Error())
			return
		}
		if doc.Notice.Restricted() && r.URL.Query().Get("accept") != "1" {
			restricted(w, doc.Notice)
			return
		}
		a.writeZip(w, m.ID, func(z *zip.Writer) error {
			for _, file := range []struct {
				name  string
				value any
			}{{"manifest.json", m}, {"cohort-features.json", doc}, {"catalog.json", doc.Table.Catalog}} {
				out, err := z.Create(file.name)
				if err != nil {
					return err
				}
				if _, err = out.Write(mustJSON(file.value)); err != nil {
					return err
				}
			}
			out, err := z.Create("subject-features.csv")
			if err != nil {
				return err
			}
			if err = study.FeatureCSV(out, doc.Table); err != nil {
				return err
			}
			return writeNotice(z, doc.Notice)
		})
	})
}
func (a *app) cohortFeaturesWork(s cohortFeaturesSpec) jobs.Func {
	return func(ctx context.Context, run *jobs.Run) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if s.Method != study.FeatureVersion || s.SHA256 == "" {
			return errors.New("feature method or pinned parent unavailable")
		}
		run.Progress(0, "Verifying design and saved diversity measurements")
		doc, _, err := a.cohortFeatureParents(s)
		if err != nil {
			return err
		}
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
		file := cohortFeaturesKind + ".json"
		if err = os.WriteFile(filepath.Join(dir, file), raw, 0644); err != nil {
			return err
		}
		if err = ctx.Err(); err != nil {
			return err
		}
		run.Validating()
		m := artifacts.Manifest{ID: id, Kind: cohortFeaturesKind, Name: "Subject features · " + s.Design.Name, Dataset: doc.Parent.Dataset,
			Lineage: artifacts.Lineage{Parents: []artifacts.Parent{{Kind: "cohort-design", ID: doc.Design.ID, Name: s.Design.Name}, {Kind: "diversity", ID: s.Diversity, SHA256: s.SHA256}}, Job: &artifacts.Job{ID: run.ID(), Kind: cohortFeaturesKind, Spec: mustJSON(s)}, Parameters: mustJSON(s.cohortFeaturesRequest), Method: artifacts.Method{Name: "Subject feature table", Version: fmt.Sprint(study.FeatureVersion), Citation: study.FeatureCitation}, Software: []artifacts.Software{{Name: "clonoloom", Version: version, Commit: buildCommit()}}}, Files: []artifacts.File{{Path: file, Role: cohortFeaturesKind}}, Summary: mustJSON(map[string]any{"subjects": doc.Design.Subjects, "rows": len(doc.Table.Rows), "features": len(doc.Table.Catalog)})}
		saved, err := a.artifacts.Promote(m, func(dir string, _ *artifacts.Manifest) []artifacts.Finding {
			bytes, err := os.ReadFile(filepath.Join(dir, file))
			var back cohortFeaturesDocument
			if err == nil {
				err = json.Unmarshal(bytes, &back)
			}
			if err != nil || string(mustJSON(back)) != string(raw) {
				return []artifacts.Finding{{Severity: "fatal", Code: "cohort-features.unreadable", Message: "Feature table failed read-back validation."}}
			}
			return nil
		})
		if err != nil {
			return err
		}
		if saved.State != artifacts.Complete {
			return errors.New("feature table failed validation")
		}
		run.Log("info", "Saved %d subject/group rows in %s.", len(doc.Table.Rows), id)
		return nil
	}
}
func (a *app) handleCohortFeaturesJobs() {
	a.jobs.Handle(cohortFeaturesKind, func(raw json.RawMessage) (jobs.Func, error) {
		var s cohortFeaturesSpec
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		if s.Method != study.FeatureVersion || s.SHA256 == "" {
			return nil, errors.New("feature method or pinned parent unavailable")
		}
		return a.cohortFeaturesWork(s), nil
	}, nil)
}
