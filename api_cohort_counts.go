package main

// Capture the selected repertoire counts with the feature parent's definition/QC,
// then fit from an immutable snapshot. Raw sources are not needed for durable replay.
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
	"clonoloom/internal/countmodel"
	"clonoloom/internal/jobs"
)

const cohortCountsKind = "cohort-counts"

type cohortCountsRequest struct {
	Features string             `json:"features"`
	Options  countmodel.Options `json:"options"`
}
type cohortCountsSpec struct {
	cohortCountsRequest
	SHA256 string           `json:"sha256"`
	Input  countmodel.Input `json:"input"`
	Method int              `json:"method"`
}

func (s cohortCountsSpec) artifactID() string {
	sum := sha256.Sum256(mustJSON(s))
	return "cn-" + hex.EncodeToString(sum[:])[:24]
}

type cohortCountsDocument struct {
	Format         string                 `json:"format"`
	Version        int                    `json:"version"`
	Description    string                 `json:"description"`
	Features       cohortFeaturesDocument `json:"features"`
	ParentManifest artifacts.Manifest     `json:"parentManifest"`
	Result         *countmodel.Result     `json:"analysis"`
}

func (a *app) cohortCountsParent(s cohortCountsSpec) (cohortFeaturesDocument, artifacts.Manifest, string, error) {
	return a.cohortCompareParent(cohortCompareSpec{cohortCompareRequest: cohortCompareRequest{Features: s.Features}, SHA256: s.SHA256})
}
func checkCountTotals(parent cohortFeaturesDocument, in countmodel.Input) error {
	expected := map[string]float64{}
	for _, row := range parent.Table.Rows {
		expected[row.Name] = float64(row.Values[0].Value)
	}
	if len(expected) != len(in.Samples) || len(in.Samples) != len(in.Totals) {
		return errors.New("count samples differ from saved feature observations")
	}
	for i, s := range in.Samples {
		v, ok := expected[s]
		if !ok || v != in.Totals[i] {
			return errors.New("count totals differ from the saved definition/QC measurements")
		}
		delete(expected, s)
	}
	return nil
}
func (a *app) registerCohortCounts(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/cohort-counts", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			cohortCountsRequest
			Start *bool `json:"start"`
		}
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		dec.DisallowUnknownFields()
		if e := dec.Decode(&body); e != nil {
			writeError(w, 400, e.Error())
			return
		}
		if dec.Decode(&struct{}{}) != io.EOF {
			writeError(w, 400, "Send one JSON object.")
			return
		}
		s := cohortCountsSpec{cohortCountsRequest: body.cohortCountsRequest, Method: countmodel.Version}
		var e error
		s.Options, e = s.Options.Normalize()
		if e != nil {
			writeError(w, 400, e.Error())
			return
		}
		parent, _, hash, e := a.cohortCountsParent(s)
		if e != nil {
			writeError(w, 409, e.Error())
			return
		}
		s.SHA256 = hash
		matrix, e := countmodel.Design(parent.Design.Input, s.Options.AdjustBatch)
		if e != nil {
			writeError(w, 422, e.Error())
			return
		}
		ds, entry, _, e := a.clonotypeTable(clonotypeRequest{Dataset: parent.Parent.Dataset, Definition: parent.Parent.Definition, Filter: parent.Parent.Filter})
		if e != nil {
			writeError(w, 409, "Open the original receptor dataset to capture new count evidence: "+e.Error())
			return
		}
		if string(entry.table.Unit) != parent.Parent.Unit {
			writeError(w, 409, "Count units differ from saved feature evidence.")
			return
		}
		s.Input, e = countmodel.Capture(entry.table, sampleNames(ds), matrix, ds.Result.Table)
		if e == nil {
			e = checkCountTotals(parent, s.Input)
		}
		if e != nil {
			writeError(w, 422, e.Error())
			return
		}
		id := s.artifactID()
		if m, e := a.artifacts.Get(id); e == nil && m.Kind == cohortCountsKind && m.State == artifacts.Complete {
			var doc cohortCountsDocument
			if _, _, e = a.readCheckedResult(id, cohortCountsKind, &doc); e != nil {
				writeError(w, 409, e.Error())
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
		job, _, e := a.jobs.Submit(cohortCountsKind, "Replicated counts · "+parent.Selection.Name, id, s, a.cohortCountsWork(s))
		if e != nil {
			writeError(w, 503, e.Error())
			return
		}
		writeJSON(w, map[string]any{"artifact": id, "ready": false, "job": job.ID})
	})
	mux.HandleFunc("GET /api/cohort-counts/{id}", func(w http.ResponseWriter, r *http.Request) {
		var doc cohortCountsDocument
		m, _, e := a.readCheckedResult(r.PathValue("id"), cohortCountsKind, &doc)
		if e != nil {
			writeError(w, 409, e.Error())
			return
		}
		writeJSON(w, map[string]any{"artifact": m, "result": doc})
	})
	mux.HandleFunc("GET /api/cohort-counts/{id}/download", func(w http.ResponseWriter, r *http.Request) {
		var doc cohortCountsDocument
		m, _, e := a.readCheckedResult(r.PathValue("id"), cohortCountsKind, &doc)
		if e != nil {
			writeError(w, 409, e.Error())
			return
		}
		notice := doc.Features.Notice
		notice.What = doc.Description
		notice.Method += " " + countmodel.Citation
		notice.Derived = true
		notice.Made = m.Created
		for _, v := range m.Lineage.Software {
			if v.Name == "clonoloom" {
				notice.Version, notice.Commit = v.Version, v.Commit
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
			}{{"manifest.json", m}, {"cohort-counts.json", doc}} {
				out, e := z.Create(f.name)
				if e != nil {
					return e
				}
				if _, e = out.Write(mustJSON(f.value)); e != nil {
					return e
				}
			}
			for _, f := range []struct {
				name  string
				write func(io.Writer, *countmodel.Result) error
			}{{"comparisons.csv", countmodel.CSV}, {"design.csv", countmodel.MatrixCSV}, {"counts.csv", countmodel.CountsCSV}} {
				out, e := z.Create(f.name)
				if e != nil {
					return e
				}
				if e = f.write(out, doc.Result); e != nil {
					return e
				}
			}
			return writeNotice(z, notice)
		})
	})
}
func (a *app) cohortCountsWork(s cohortCountsSpec) jobs.Func {
	return func(ctx context.Context, run *jobs.Run) error {
		if e := ctx.Err(); e != nil {
			return e
		}
		if s.Method != countmodel.Version {
			return errors.New("count-model method version unavailable")
		}
		parent, pm, _, e := a.cohortCountsParent(s)
		if e != nil {
			return e
		}
		if e = checkCountTotals(parent, s.Input); e != nil {
			return e
		}
		result, e := countmodel.Compute(ctx, parent.Design.Input, s.Input, s.Options, func(v float64) { run.Progress(.1+float64(.8*v), "Fitting replicated negative-binomial counts") })
		if e != nil {
			return e
		}
		description := fmt.Sprintf("%s Design %s version %d (%s), %s: %q minus %q; %d subjects, %d selected observations, %d residual df. Feature parent %s SHA-256 %s; dataset %s; definition %s (%s), QC %s, count unit %s. Count filter >=%d in >=%d samples; BH family %d, fitted %d. %.1f%% marginal Wald intervals, conditional on fitted dispersion. Batch adjustment: %t.", countmodel.Description, parent.Selection.Name, parent.Selection.Revision, parent.Design.ID, parent.Design.Input.Spec.Layout, parent.Design.Input.Spec.Levels[1], parent.Design.Input.Spec.Levels[0], parent.Design.Subjects, parent.Design.Included, result.Design.ResidualDF, s.Features, s.SHA256, parent.Parent.Dataset, parent.Parent.DefinitionID, parent.Parent.Describe, parent.Parent.Filter.Describe(), parent.Parent.Unit, s.Options.MinCount, s.Options.MinSamples, result.Family, result.Tested, 100*s.Options.Confidence, s.Options.AdjustBatch)
		for _, f := range parent.Design.Findings {
			description += " Design finding: " + f.Title + ". " + f.Detail
		}
		doc := cohortCountsDocument{Format: "clonoloom-cohort-counts", Version: countmodel.Version, Description: description, Features: parent, ParentManifest: pm, Result: result}
		raw, e := json.Marshal(doc)
		if e != nil {
			return e
		}
		id := s.artifactID()
		dir, e := a.artifacts.Stage(id)
		if e != nil {
			return e
		}
		defer a.artifacts.Discard(id)
		file := cohortCountsKind + ".json"
		if e = os.WriteFile(filepath.Join(dir, file), raw, 0644); e != nil {
			return e
		}
		if e = ctx.Err(); e != nil {
			return e
		}
		run.Validating()
		m := artifacts.Manifest{ID: id, Kind: cohortCountsKind, Name: "Replicated counts · " + parent.Selection.Name, Dataset: parent.Parent.Dataset, Lineage: artifacts.Lineage{Parents: []artifacts.Parent{{Kind: cohortFeaturesKind, ID: s.Features, SHA256: s.SHA256}}, Job: &artifacts.Job{ID: run.ID(), Kind: cohortCountsKind, Spec: mustJSON(s)}, Parameters: mustJSON(s.cohortCountsRequest), Method: artifacts.Method{Name: "Replicated negative-binomial counts", Version: fmt.Sprint(countmodel.Version), Citation: countmodel.Citation}, Software: []artifacts.Software{{Name: "clonoloom", Version: version, Commit: buildCommit()}}}, Files: []artifacts.File{{Path: file, Role: cohortCountsKind}}, Summary: mustJSON(map[string]any{"family": result.Family, "tested": result.Tested, "observations": len(result.Design.Rows)})}
		saved, e := a.artifacts.Promote(m, func(dir string, _ *artifacts.Manifest) []artifacts.Finding {
			b, e := os.ReadFile(filepath.Join(dir, file))
			var back cohortCountsDocument
			if e == nil {
				e = json.Unmarshal(b, &back)
			}
			if e != nil || back.Result == nil || string(mustJSON(back)) != string(raw) {
				return []artifacts.Finding{{Severity: "fatal", Code: "cohort-counts.unreadable", Message: "Count model failed read-back validation."}}
			}
			return nil
		})
		if e != nil {
			return e
		}
		if saved.State != artifacts.Complete {
			return errors.New("count model failed promotion")
		}
		run.Log("info", "Saved %d fitted clonotypes in %d family slots.", result.Tested, result.Family)
		return nil
	}
}
func (a *app) handleCohortCountsJobs() {
	a.jobs.Handle(cohortCountsKind, func(raw json.RawMessage) (jobs.Func, error) {
		var s cohortCountsSpec
		if e := json.Unmarshal(raw, &s); e != nil {
			return nil, e
		}
		if s.Method != countmodel.Version || s.SHA256 == "" {
			return nil, errors.New("count-model method or parent hash unavailable")
		}
		if _, e := s.Options.Normalize(); e != nil {
			return nil, e
		}
		return a.cohortCountsWork(s), nil
	}, nil)
}
