package main

// Captured sample QC and within-batch descriptive scores; research §5.38.
import (
	"archive/zip"
	"clonoloom/internal/artifacts"
	"clonoloom/internal/batchqc"
	"clonoloom/internal/export"
	"clonoloom/internal/jobs"
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
	"time"
)

const batchqcKind = "batch-qc"

type batchqcRequest struct {
	Dataset string               `json:"dataset"`
	Samples []batchqc.Assignment `json:"samples"`
	Filter  qc.Filter            `json:"filter"`
	Options batchqc.Options      `json:"options"`
}
type batchqcSpec struct {
	batchqcRequest
	Input  batchqc.Input `json:"input"`
	Notice export.Notice `json:"notice"`
	Method int           `json:"method"`
}

func (s batchqcSpec) artifactID() string {
	s.Notice.Made = time.Time{}
	return "bqc-" + fmt.Sprintf("%x", sha256.Sum256(mustJSON(s)))[:24]
}

type batchqcDocument struct {
	Format      string          `json:"format"`
	Version     int             `json:"version"`
	Description string          `json:"description"`
	Evidence    batchqcSpec     `json:"evidence"`
	Analysis    *batchqc.Result `json:"analysis"`
}

func (a *app) captureBatchQC(req batchqcRequest) (batchqcSpec, error) {
	s := batchqcSpec{batchqcRequest: req, Method: batchqc.Version}
	ds, ok := a.datasets.get(req.Dataset)
	if !ok {
		return s, errors.New("open the source dataset first")
	}
	var err error
	s.Options, err = s.Options.Normalize()
	if err != nil {
		return s, err
	}
	s.Input, err = batchqc.Capture(ds.Result.Table, s.Filter, s.Samples, s.Options)
	if err != nil {
		return s, err
	}
	s.Samples = []batchqc.Assignment{}
	for _, sample := range s.Input.Samples {
		s.Samples = append(s.Samples, sample.Assignment)
	}
	if _, err = batchqc.Compute(context.Background(), s.Input); err != nil {
		return s, err
	}
	s.Notice = a.notice(ds, s.Input.Unit, s.Filter, "Sample QC metrics; no clonotype grouping", "batch-qc-v1")
	s.Notice.What = batchqc.Description
	s.Notice.Method += " " + batchqc.Citation
	s.Notice.Derived = true
	for _, source := range ds.Result.Metadata {
		s.Notice.Files = append(s.Notice.Files, export.File{Name: source.Name, SHA256: source.SHA256})
	}
	return s, nil
}
func (a *app) registerBatchQC(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/batch-qc", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			batchqcRequest
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
		s, err := a.captureBatchQC(body.batchqcRequest)
		if err != nil {
			writeError(w, 422, err.Error())
			return
		}
		id := s.artifactID()
		if m, e := a.artifacts.Get(id); e == nil && m.Kind == batchqcKind && m.State == artifacts.Complete {
			var doc batchqcDocument
			if _, _, e = a.readCheckedResult(id, batchqcKind, &doc); e != nil {
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
		j, _, err := a.jobs.Submit(batchqcKind, "Batch QC · "+s.Notice.Dataset, id, s, a.batchqcWork(s))
		if err != nil {
			writeError(w, 503, err.Error())
			return
		}
		writeJSON(w, map[string]any{"artifact": id, "job": j.ID, "ready": false})
	})
	mux.HandleFunc("GET /api/batch-qc/{id}", func(w http.ResponseWriter, r *http.Request) {
		var doc batchqcDocument
		m, _, err := a.readCheckedResult(r.PathValue("id"), batchqcKind, &doc)
		if err != nil {
			writeError(w, 409, err.Error())
			return
		}
		writeJSON(w, map[string]any{"artifact": m, "result": doc})
	})
	mux.HandleFunc("GET /api/batch-qc/{id}/download", func(w http.ResponseWriter, r *http.Request) {
		var doc batchqcDocument
		m, _, err := a.readCheckedResult(r.PathValue("id"), batchqcKind, &doc)
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
			}{{"batch-qc.json", doc}, {"manifest.json", m}, {"samples.json", doc.Analysis.Evidence.Samples}} {
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
			if e = batchqc.CSV(out, doc.Analysis); e != nil {
				return e
			}
			return writeNotice(z, n)
		})
	})
}
func (s batchqcSpec) check() error {
	if s.Method != batchqc.Version || s.Dataset == "" || string(mustJSON(s.Options)) != string(mustJSON(s.Input.Options)) || len(s.Samples) != len(s.Input.Samples) {
		return errors.New("inconsistent captured batch QC provenance")
	}
	for i, x := range s.Samples {
		if x != s.Input.Samples[i].Assignment {
			return errors.New("captured assignments differ")
		}
	}
	return nil
}
func (a *app) batchqcWork(s batchqcSpec) jobs.Func {
	return func(ctx context.Context, run *jobs.Run) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := s.check(); err != nil {
			return err
		}
		result, err := batchqc.Compute(ctx, s.Input)
		if err != nil {
			return err
		}
		run.Progress(.8, "Saving sample QC and within-batch scores")
		desc := fmt.Sprintf("%s Dataset %s (%s); QC: %s; abundance unit %s; single cell: %t. Bulk fractions use counts; single-cell fractions use contigs. Included %d samples / %d unique subjects; excluded %d. Threshold: absolute modified z > %g; minimum evaluable subjects per batch/metric %d. Median and MAD use the displayed transformed scale.", batchqc.Description, s.Notice.Dataset, s.Dataset, s.Notice.Filter, s.Notice.Unit, s.Input.SingleCell, result.Included, result.Subjects, result.Excluded, s.Options.Threshold, s.Options.MinSamples)
		doc := batchqcDocument{Format: "clonoloom-batch-qc", Version: batchqc.Version, Description: desc, Evidence: s, Analysis: result}
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
		if err = os.WriteFile(filepath.Join(dir, "batch-qc.json"), raw, 0644); err != nil {
			return err
		}
		if err = ctx.Err(); err != nil {
			return err
		}
		run.Validating()
		m := artifacts.Manifest{ID: id, Kind: batchqcKind, Name: "Batch QC · " + s.Notice.Dataset, Dataset: s.Dataset, Lineage: artifacts.Lineage{Job: &artifacts.Job{ID: run.ID(), Kind: batchqcKind, Spec: mustJSON(s)}, Parameters: mustJSON(s.batchqcRequest), Method: artifacts.Method{Name: "Descriptive batch QC", Version: "1", Citation: batchqc.Citation}, Software: []artifacts.Software{{Name: "clonoloom", Version: version, Commit: buildCommit()}}}, Files: []artifacts.File{{Path: "batch-qc.json", Role: batchqcKind}}, Summary: mustJSON(map[string]any{"samples": result.Included, "subjects": result.Subjects, "groups": len(result.Groups)})}
		saved, err := a.artifacts.Promote(m, func(dir string, _ *artifacts.Manifest) []artifacts.Finding {
			b, e := os.ReadFile(filepath.Join(dir, "batch-qc.json"))
			var back batchqcDocument
			if e == nil {
				e = json.Unmarshal(b, &back)
			}
			if e != nil || back.Analysis == nil || string(mustJSON(back)) != string(raw) {
				return []artifacts.Finding{{Severity: "fatal", Code: "batchqc.unreadable", Message: "BatchQC result failed read-back validation."}}
			}
			return nil
		})
		if err != nil {
			return err
		}
		if saved.State != artifacts.Complete {
			return errors.New("batchqc promotion failed")
		}
		return nil
	}
}
func (a *app) handleBatchQCJobs() {
	a.jobs.Handle(batchqcKind, func(raw json.RawMessage) (jobs.Func, error) {
		var s batchqcSpec
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		if err := s.check(); err != nil {
			return nil, err
		}
		return a.batchqcWork(s), nil
	}, nil)
}
