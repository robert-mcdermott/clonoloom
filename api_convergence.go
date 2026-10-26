package main

// Checksum-pinned motif signatures become subject-level binary evidence. Original
// motif scores are not combined with HLA probabilities; research §5.36.
import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"

	"clonoloom/internal/artifacts"
	"clonoloom/internal/convergence"
	"clonoloom/internal/export"
	"clonoloom/internal/jobs"
	"clonoloom/internal/motifs"
)

const convergenceKind = "convergence"

type convergenceRequest struct {
	Motifs    string                `json:"motifs"`
	Selection convergence.Selection `json:"selection"`
}
type convergenceSpec struct {
	convergenceRequest
	SHA256            string             `json:"sha256"`
	Input             convergence.Input  `json:"input"`
	ParentManifest    artifacts.Manifest `json:"parentManifest"`
	ParentDescription string             `json:"parentDescription"`
	Notice            export.Notice      `json:"notice"`
	Method            int                `json:"method"`
}

func (s convergenceSpec) artifactID() string {
	s.Notice.Made = time.Time{}
	return "cv-" + fmt.Sprintf("%x", sha256.Sum256(mustJSON(s)))[:24]
}

type convergenceDocument struct {
	Format      string              `json:"format"`
	Version     int                 `json:"version"`
	Description string              `json:"description"`
	Evidence    convergenceSpec     `json:"evidence"`
	Analysis    *convergence.Result `json:"analysis"`
}

func (a *app) captureConvergence(req convergenceRequest) (convergenceSpec, error) {
	s := convergenceSpec{convergenceRequest: req, Method: convergence.Version}
	selection, err := convergence.Normalize(req.Selection)
	if err != nil {
		return s, err
	}
	s.Selection = selection
	var p motifsDocument
	m, hash, err := a.readCheckedResult(req.Motifs, motifsKind, &p)
	if err != nil {
		return s, err
	}
	if p.Format != "clonoloom-motifs" || p.Version != motifs.Version || p.Result == nil || p.Evidence.check() != nil || p.Evidence.artifactID() != m.ID || p.Evidence.Dataset != m.Dataset || m.Lineage.Job == nil {
		return s, errors.New("motif parent provenance differs")
	}
	var job motifsSpec
	if json.Unmarshal(m.Lineage.Job.Spec, &job) != nil || string(mustJSON(job)) != string(mustJSON(p.Evidence)) || string(mustJSON(p.Result.Query)) != string(mustJSON(p.Evidence.QueryRecords)) || string(mustJSON(p.Result.Background)) != string(mustJSON(p.Evidence.BackgroundRecords)) {
		return s, errors.New("motif parent capture differs")
	}
	available := map[string]bool{}
	for _, sample := range append(append([]string{}, p.Evidence.Query...), p.Evidence.Background...) {
		available[sample] = true
	}
	for _, sample := range selection.Samples {
		if !available[sample] {
			return s, errors.New("selected samples must belong to the motif parent's captured arms")
		}
	}
	// Repeated identities across arms are unioned, not duplicated. Original counts
	// remain in the parent; this result deliberately uses subject presence only.
	records := map[string]motifs.Record{}
	for _, arm := range [][]motifs.Record{p.Result.Query, p.Result.Background} {
		for _, rec := range arm {
			prev, ok := records[rec.ID]
			if !ok {
				prev = motifs.Record{ID: rec.ID, V: rec.V, Junction: rec.Junction, Samples: []string{}}
			}
			if prev.V != rec.V || prev.Junction != rec.Junction {
				return s, errors.New("inconsistent parent identity")
			}
			set := map[string]bool{}
			for _, v := range append(prev.Samples, rec.Samples...) {
				set[v] = true
			}
			prev.Samples = []string{}
			for v := range set {
				prev.Samples = append(prev.Samples, v)
			}
			sort.Strings(prev.Samples)
			records[rec.ID] = prev
		}
	}
	s.Input = convergence.Input{Selection: selection, Records: []motifs.Record{}, Groups: []convergence.Signature{}}
	ids := []string{}
	for id := range records {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		s.Input.Records = append(s.Input.Records, records[id])
	}
	groups := map[string]motifs.Group{}
	for _, g := range p.Result.Groups {
		groups[g.ID] = g
	}
	for _, id := range selection.Groups {
		g, ok := groups[id]
		if !ok {
			return s, errors.New("choose existing saved motif group IDs")
		}
		members := map[string]bool{}
		for arm, indexes := range [][]int{g.Query, g.Background} {
			rows := p.Result.Query
			if arm == 1 {
				rows = p.Result.Background
			}
			for _, i := range indexes {
				if i < 0 || i >= len(rows) {
					return s, errors.New("invalid parent member index")
				}
				members[rows[i].ID] = true
			}
		}
		rr := []string{}
		for id := range members {
			rr = append(rr, id)
		}
		sort.Strings(rr)
		allowed := ""
		if g.Allowed != "" {
			allowed = " [" + g.Allowed + "]"
		}
		label := fmt.Sprintf("%s %s%s; %s, length %d, starts %d–%d", g.Kind, g.Motif, allowed, g.V, g.Length, g.Start, g.End)
		s.Input.Groups = append(s.Input.Groups, convergence.Signature{ID: id, Label: label, Members: rr})
	}
	s.SHA256, s.ParentManifest, s.ParentDescription = hash, m, p.Description
	s.Notice = p.Evidence.Notice
	s.Notice.What = convergence.Description
	s.Notice.Method += " " + convergence.Citation
	s.Notice.Derived = true
	// Check scientific inputs before submitting a job; this is a bounded binary
	// subject screen (<=8192 slots), so readiness probing validates them too.
	if _, err = convergence.Compute(context.Background(), s.Input); err != nil {
		return s, err
	}
	return s, nil
}
func (a *app) registerConvergence(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/convergence", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			convergenceRequest
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
		s, err := a.captureConvergence(body.convergenceRequest)
		if err != nil {
			writeError(w, 422, err.Error())
			return
		}
		id := s.artifactID()
		if m, e := a.artifacts.Get(id); e == nil && m.Kind == convergenceKind && m.State == artifacts.Complete {
			var doc convergenceDocument
			if _, _, e = a.readCheckedResult(id, convergenceKind, &doc); e != nil {
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
		j, _, err := a.jobs.Submit(convergenceKind, "Convergence and HLA · "+s.Notice.Dataset, id, s, a.convergenceWork(s))
		if err != nil {
			writeError(w, 503, err.Error())
			return
		}
		writeJSON(w, map[string]any{"artifact": id, "job": j.ID, "ready": false})
	})
	mux.HandleFunc("GET /api/convergence/{id}", func(w http.ResponseWriter, r *http.Request) {
		var doc convergenceDocument
		m, _, err := a.readCheckedResult(r.PathValue("id"), convergenceKind, &doc)
		if err != nil {
			writeError(w, 409, err.Error())
			return
		}
		writeJSON(w, map[string]any{"artifact": m, "result": doc})
	})
	mux.HandleFunc("GET /api/convergence/{id}/download", func(w http.ResponseWriter, r *http.Request) {
		var doc convergenceDocument
		m, _, err := a.readCheckedResult(r.PathValue("id"), convergenceKind, &doc)
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
			}{{"convergence.json", doc}, {"manifest.json", m}, {"subjects.json", doc.Analysis.Observations}} {
				out, e := z.Create(f.name)
				if e != nil {
					return e
				}
				if _, e = out.Write(mustJSON(f.value)); e != nil {
					return e
				}
			}
			out, e := z.Create("associations.csv")
			if e != nil {
				return e
			}
			if e = convergence.AssociationsCSV(out, doc.Analysis); e != nil {
				return e
			}
			out, e = z.Create("recurrence.csv")
			if e != nil {
				return e
			}
			if e = convergence.RecurrenceCSV(out, doc.Analysis); e != nil {
				return e
			}
			return writeNotice(z, n)
		})
	})
}
func (s convergenceSpec) check() error {
	if s.Method != convergence.Version || s.Motifs == "" || s.SHA256 == "" || s.ParentManifest.ID != s.Motifs || s.ParentManifest.Kind != motifsKind || string(mustJSON(s.Selection)) != string(mustJSON(s.Input.Selection)) {
		return errors.New("inconsistent captured convergence provenance")
	}
	found := false
	for _, f := range s.ParentManifest.Files {
		if f.Path == "motifs.json" && f.SHA256 == s.SHA256 {
			found = true
		}
	}
	if !found {
		return errors.New("captured motif checksum differs")
	}
	return nil
}
func (a *app) convergenceWork(s convergenceSpec) jobs.Func {
	return func(ctx context.Context, run *jobs.Run) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := s.check(); err != nil {
			return err
		}
		result, err := convergence.Compute(ctx, s.Input)
		if err != nil {
			return err
		}
		run.Progress(.8, "Saving subject recurrence and typed universes")
		desc := fmt.Sprintf("%s Parent %s (SHA-256 %s). Definition %s; QC %s; source abundance %s is not a test weight. Selected samples %v; %d named subjects, %d with receptor evidence; %d declared group × allele slots, %d tested. Complete per-locus typing, raw calls and exclusion reasons are retained.", convergence.Description, s.Motifs, s.SHA256, s.Notice.Definition, s.Notice.Filter, s.Notice.Unit, s.Selection.Samples, len(s.Selection.Subjects), result.ObservedSubjects, result.Family, result.Tested)
		doc := convergenceDocument{Format: "clonoloom-convergence", Version: convergence.Version, Description: desc, Evidence: s, Analysis: result}
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
		if err = os.WriteFile(filepath.Join(dir, "convergence.json"), raw, 0644); err != nil {
			return err
		}
		if err = ctx.Err(); err != nil {
			return err
		}
		run.Validating()
		m := artifacts.Manifest{ID: id, Kind: convergenceKind, Name: "Convergence and HLA · " + s.Notice.Dataset, Dataset: s.ParentManifest.Dataset, Lineage: artifacts.Lineage{Parents: []artifacts.Parent{{Kind: motifsKind, ID: s.Motifs, Name: s.ParentManifest.Name}}, Job: &artifacts.Job{ID: run.ID(), Kind: convergenceKind, Spec: mustJSON(s)}, Parameters: mustJSON(s.convergenceRequest), Method: artifacts.Method{Name: "Subject recurrence and conditional HLA association", Version: "1", Citation: convergence.Citation}, Software: []artifacts.Software{{Name: "clonoloom", Version: version, Commit: buildCommit()}}}, Files: []artifacts.File{{Path: "convergence.json", Role: convergenceKind}}, Summary: mustJSON(map[string]any{"subjects": result.ObservedSubjects, "family": result.Family, "tested": result.Tested})}
		saved, err := a.artifacts.Promote(m, func(dir string, _ *artifacts.Manifest) []artifacts.Finding {
			b, e := os.ReadFile(filepath.Join(dir, "convergence.json"))
			var back convergenceDocument
			if e == nil {
				e = json.Unmarshal(b, &back)
			}
			if e != nil || back.Analysis == nil || string(mustJSON(back)) != string(raw) {
				return []artifacts.Finding{{Severity: "fatal", Code: "convergence.unreadable", Message: "Convergence result failed read-back validation."}}
			}
			return nil
		})
		if err != nil {
			return err
		}
		if saved.State != artifacts.Complete {
			return errors.New("convergence promotion failed")
		}
		return nil
	}
}
func (a *app) handleConvergenceJobs() {
	a.jobs.Handle(convergenceKind, func(raw json.RawMessage) (jobs.Func, error) {
		var s convergenceSpec
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		if err := s.check(); err != nil {
			return nil, err
		}
		return a.convergenceWork(s), nil
	}, nil)
}
