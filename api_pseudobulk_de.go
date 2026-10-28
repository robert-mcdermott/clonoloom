package main

// Source-free paired pseudobulk gene inference; research §5.40.
import (
	"archive/zip"
	"clonoloom/internal/artifacts"
	"clonoloom/internal/export"
	"clonoloom/internal/jobs"
	"clonoloom/internal/pseudobulk"
	"clonoloom/internal/pseudobulkde"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

const pseudobulkDEKind = "pseudobulk-de"

type pseudobulkDERequest struct {
	Pseudobulk string               `json:"pseudobulk"`
	Options    pseudobulkde.Options `json:"options"`
}
type pseudobulkDESpec struct {
	pseudobulkDERequest
	Dataset             string             `json:"dataset"`
	Parent              artifacts.Manifest `json:"parent"`
	ParentSHA256        string             `json:"parentSHA256"`
	Input               pseudobulkde.Input `json:"input"`
	Notice              export.Notice      `json:"notice"`
	EvidenceDescription string             `json:"evidenceDescription"`
	Method              int                `json:"method"`
}

func (s pseudobulkDESpec) artifactID() string {
	// Parent checksum binds the original document; runtime timestamps/job IDs are not identity.
	return "pde-" + fmt.Sprintf("%x", sha256.Sum256(mustJSON(struct {
		Request               pseudobulkDERequest
		Dataset, ParentSHA256 string
		Input                 pseudobulkde.Input
		Method                int
	}{s.pseudobulkDERequest, s.Dataset, s.ParentSHA256, s.Input, s.Method})))[:24]
}

type pseudobulkDEDocument struct {
	Format      string               `json:"format"`
	Version     int                  `json:"version"`
	Description string               `json:"description"`
	Evidence    pseudobulkDESpec     `json:"evidence"`
	Analysis    *pseudobulkde.Result `json:"analysis"`
}

func (a *app) capturePseudobulkDE(ctx context.Context, req pseudobulkDERequest) (pseudobulkDESpec, error) {
	s := pseudobulkDESpec{pseudobulkDERequest: req, Method: pseudobulkde.Version}
	var err error
	s.Options, err = s.Options.Normalize()
	if err != nil {
		return s, err
	}
	if err = ctx.Err(); err != nil {
		return s, err
	}
	var p pseudobulkDocument
	m, hash, err := a.readCheckedResult(s.Pseudobulk, pseudobulkKind, &p)
	if err != nil {
		return s, err
	}
	if p.Format != "clonoloom-pseudobulk" || p.Version != pseudobulk.Version || p.Analysis == nil {
		return s, errors.New("unsupported pseudobulk count parent")
	}
	if err = p.Evidence.check(); err != nil {
		return s, err
	}
	s.Dataset = p.Evidence.Dataset
	s.Parent = m
	s.ParentSHA256 = hash
	s.Input = pseudobulkde.Capture(p.Evidence.Input, p.Analysis)
	s.Notice = p.Evidence.Notice
	s.EvidenceDescription = p.Description
	s.Notice.What = pseudobulkde.Description
	s.Notice.Method = pseudobulkde.Citation
	s.Notice.Derived = true
	if _, err = pseudobulkde.Prepare(s.Input, s.Options); err != nil {
		return s, err
	}
	return s, nil
}
func (a *app) registerPseudobulkDE(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/pseudobulk-de", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			pseudobulkDERequest
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
		s, err := a.capturePseudobulkDE(r.Context(), body.pseudobulkDERequest)
		if err != nil {
			writeError(w, 422, err.Error())
			return
		}
		id := s.artifactID()
		if m, e := a.artifacts.Get(id); e == nil && m.Kind == pseudobulkDEKind && m.State == artifacts.Complete {
			var doc pseudobulkDEDocument
			if _, _, e = a.readCheckedResult(id, pseudobulkDEKind, &doc); e != nil {
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
		j, _, err := a.jobs.Submit(pseudobulkDEKind, "Pseudobulk differential expression · "+s.Notice.Dataset, id, s, a.pseudobulkDEWork(s))
		if err != nil {
			writeError(w, 503, err.Error())
			return
		}
		writeJSON(w, map[string]any{"artifact": id, "job": j.ID, "ready": false})
	})
	mux.HandleFunc("GET /api/pseudobulk-de/{id}", func(w http.ResponseWriter, r *http.Request) {
		var doc pseudobulkDEDocument
		m, _, err := a.readCheckedResult(r.PathValue("id"), pseudobulkDEKind, &doc)
		if err != nil {
			writeError(w, 409, err.Error())
			return
		}
		writeJSON(w, map[string]any{"artifact": m, "result": doc})
	})
	mux.HandleFunc("GET /api/pseudobulk-de/{id}/download", func(w http.ResponseWriter, r *http.Request) {
		var doc pseudobulkDEDocument
		m, _, err := a.readCheckedResult(r.PathValue("id"), pseudobulkDEKind, &doc)
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
			}{{"pseudobulk-de.json", doc}, {"manifest.json", m}, {"parent-manifest.json", doc.Evidence.Parent}, {"raw-evidence.json", doc.Evidence.Input}} {
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
				write func(io.Writer, *pseudobulkde.Result) error
			}{{"genes.csv", pseudobulkde.CSV}, {"design.csv", pseudobulkde.MatrixCSV}, {"counts.csv", pseudobulkde.CountsCSV}} {
				out, e := z.Create(f.name)
				if e != nil {
					return e
				}
				if e = f.write(out, doc.Analysis); e != nil {
					return e
				}
			}
			return writeNotice(z, n)
		})
	})
}
func (s pseudobulkDESpec) check() error {
	if s.Method != pseudobulkde.Version || s.Dataset == "" || s.Pseudobulk != s.Parent.ID || s.Parent.Kind != pseudobulkKind || s.Parent.Dataset != s.Dataset || len(s.ParentSHA256) != 64 {
		return errors.New("inconsistent captured pseudobulk differential-expression provenance")
	}
	return nil
}
func (a *app) pseudobulkDEWork(s pseudobulkDESpec) jobs.Func {
	return func(ctx context.Context, run *jobs.Run) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := s.check(); err != nil {
			return err
		}
		result, err := pseudobulkde.Compute(ctx, s.Input, s.Options, func(p float64) { run.Progress(.9*p, "Fitting paired gene counts") })
		if err != nil {
			return err
		}
		run.Progress(.95, "Saving paired gene effects and full count evidence")
		desc := fmt.Sprintf("%s Type %q; literal timepoint %q; contrast %q / %q; %d subjects, %d paired observations, %d residual df. Filter: at least %d raw counts in %d observations; %d eligible genes, %d fitted; %d normalization genes. Parent: %s (%s). %s", pseudobulkde.Description, s.Options.Type, s.Options.Time, s.Options.Comparison, s.Options.Reference, len(result.Observations)/2, len(result.Observations), result.Design.ResidualDF, s.Options.MinCount, s.Options.MinSamples, result.Family, result.Tested, len(result.NormalizationGenes), s.Pseudobulk, s.ParentSHA256, s.EvidenceDescription)
		doc := pseudobulkDEDocument{Format: "clonoloom-pseudobulk-de", Version: pseudobulkde.Version, Description: desc, Evidence: s, Analysis: result}
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
		if err = os.WriteFile(filepath.Join(dir, "pseudobulk-de.json"), raw, 0644); err != nil {
			return err
		}
		if err = ctx.Err(); err != nil {
			return err
		}
		run.Validating()
		m := artifacts.Manifest{ID: id, Kind: pseudobulkDEKind, Name: "Pseudobulk differential expression · " + s.Notice.Dataset, Dataset: s.Dataset, Lineage: artifacts.Lineage{Parents: []artifacts.Parent{{ID: s.Pseudobulk, Kind: pseudobulkKind, SHA256: s.ParentSHA256}}, Job: &artifacts.Job{ID: run.ID(), Kind: pseudobulkDEKind, Spec: mustJSON(s)}, Parameters: mustJSON(s.pseudobulkDERequest), Method: artifacts.Method{Name: "Paired pseudobulk negative-binomial expression", Version: "1", Citation: pseudobulkde.Citation}, Software: []artifacts.Software{{Name: "clonoloom", Version: version, Commit: buildCommit()}}}, Files: []artifacts.File{{Path: "pseudobulk-de.json", Role: pseudobulkDEKind}}, Summary: mustJSON(map[string]any{"subjects": len(result.Observations) / 2, "family": result.Family, "tested": result.Tested})}
		saved, err := a.artifacts.Promote(m, func(dir string, _ *artifacts.Manifest) []artifacts.Finding {
			b, e := os.ReadFile(filepath.Join(dir, "pseudobulk-de.json"))
			var back pseudobulkDEDocument
			if e == nil {
				e = json.Unmarshal(b, &back)
			}
			if e != nil || back.Analysis == nil || string(mustJSON(back)) != string(raw) {
				return []artifacts.Finding{{Severity: "fatal", Code: "pseudobulkde.unreadable", Message: "Pseudobulk differential-expression result failed read-back validation."}}
			}
			return nil
		})
		if err != nil {
			return err
		}
		if saved.State != artifacts.Complete {
			return errors.New("pseudobulk differential-expression promotion failed")
		}
		return nil
	}
}
func (a *app) handlePseudobulkDEJobs() {
	a.jobs.Handle(pseudobulkDEKind, func(raw json.RawMessage) (jobs.Func, error) {
		var s pseudobulkDESpec
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		if err := s.check(); err != nil {
			return nil, err
		}
		return a.pseudobulkDEWork(s), nil
	}, nil)
}
