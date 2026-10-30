package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"clonoloom/internal/airr"
	"clonoloom/internal/cytolink"
	"clonoloom/internal/qc"
	"clonoloom/internal/reads"
	"clonoloom/internal/rep"
)

type cytoProvenance struct {
	ExporterVersion string `json:"exporterVersion"`
	Sample          string `json:"sample"`
	Acquisition     string `json:"acquisition"`
	SourceSHA256    string `json:"sourceSHA256"`
	Compensation    string `json:"compensation"`
	ValueScale      string `json:"valueScale"`
	Gating          string `json:"gating"`
	EvidenceText    string `json:"evidenceText"`
}
type cytoRequest struct {
	Dataset     string `json:"dataset"`
	Sample      *int   `json:"sample"`
	Wells       string `json:"wells"`
	Mapping     string `json:"mapping"`
	Provenance  string `json:"provenance"`
	Fingerprint string `json:"fingerprint"`
	AcceptTerms bool   `json:"acceptTerms"`
}
type cytoText struct {
	Text   string `json:"text"`
	SHA256 string `json:"sha256"`
}
type cytoEvidence struct {
	Format      string          `json:"format"`
	Version     int             `json:"version"`
	Software    string          `json:"software"`
	Dataset     string          `json:"dataset"`
	SampleIndex int             `json:"sampleIndex"`
	Sample      rep.Sample      `json:"sample"`
	Sources     []reads.Source  `json:"sources"`
	Wells       cytoText        `json:"wells"`
	Mapping     cytoText        `json:"mapping"`
	Provenance  cytoText        `json:"provenance"`
	Declaration cytoProvenance  `json:"declaration"`
	Result      cytolink.Result `json:"result"`
	Notice      string          `json:"notice"`
	Restricted  bool            `json:"restricted"`
	Limits      string          `json:"limits"`
}

var cytoHash = regexp.MustCompile(`^[0-9a-f]{64}$`)

func checkCytoProvenance(text string) (cytoProvenance, error) {
	var p cytoProvenance
	if len(text) > 64<<10 || !utf8.ValidString(text) {
		return p, errors.New("Provenance must be UTF-8 within 64 KiB")
	}
	if e := airr.CheckExactJSON(context.Background(), []byte(text)); e != nil {
		return p, e
	}
	if e := strictRunJSON([]byte(text), &p); e != nil {
		return p, e
	}
	for _, v := range []string{p.ExporterVersion, p.Sample, p.Acquisition, p.SourceSHA256, p.Compensation, p.ValueScale, p.Gating, p.EvidenceText} {
		if strings.TrimSpace(v) == "" {
			return p, errors.New("Declare every provenance field, using unknown where unavailable")
		}
	}
	if p.SourceSHA256 != "unknown" && !cytoHash.MatchString(p.SourceSHA256) {
		return p, errors.New("sourceSHA256 must be a lowercase SHA-256 or unknown")
	}
	if p.ValueScale != "linear" && p.ValueScale != "unknown" {
		return p, errors.New("Wells CSV supports linear or unknown valueScale; no transformed-value inference")
	}
	return p, nil
}
func (a *app) registerCytoWeave(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/cytoweave/{operation}", localOnly(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		op := r.PathValue("operation")
		if op != "review" && op != "export" {
			writeError(w, 404, "Unknown phenotype operation")
			return
		}
		raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 12<<20))
		var req cytoRequest
		if e == nil {
			if !utf8.Valid(raw) {
				e = errors.New("UTF-8 required")
			} else {
				e = airr.CheckExactJSON(r.Context(), raw)
				if e == nil {
					e = strictRunJSON(raw, &req)
				}
			}
		}
		if e != nil {
			writeError(w, 400, "Send one bounded UTF-8 request with explicit dataset/sample and CSV/provenance text")
			return
		}
		ds, ok := a.datasets.get(req.Dataset)
		if !ok {
			writeError(w, 404, "Open the receptor dataset again")
			return
		}
		if req.Sample == nil {
			writeError(w, 400, "Select one receptor sample explicitly")
			return
		}
		if e := verifyCytoIdentities(r.Context(), ds, *req.Sample); e != nil {
			writeError(w, 400, e.Error())
			return
		}
		p, e := checkCytoProvenance(req.Provenance)
		if e != nil {
			writeError(w, 400, e.Error())
			return
		}
		result, e := cytolink.Link(r.Context(), ds.Result.Table, *req.Sample, req.Wells, req.Mapping)
		if e != nil {
			writeError(w, 400, e.Error())
			return
		}
		notice := a.notice(ds, "distinct supplied cell IDs", qc.Filter{}, "No QC or receptor pairing inferred", "")
		notice.Made = ds.Opened
		notice.Samples = 1
		text := func(s string) cytoText { return cytoText{s, replayDigest([]byte(s))} }
		doc := cytoEvidence{Format: "clonoloom-cytoweave-links", Version: 1, Software: version, Dataset: ds.ID, SampleIndex: *req.Sample, Sample: ds.Result.Table.Samples[*req.Sample], Sources: ds.Result.Sources, Wells: text(req.Wells), Mapping: text(req.Mapping), Provenance: text(req.Provenance), Declaration: p, Result: result, Notice: notice.Text(), Restricted: notice.Restricted(), Limits: cytolink.Limits}
		b, e := json.MarshalIndent(doc, "", "  ")
		if e != nil {
			writeError(w, 500, e.Error())
			return
		}
		fingerprint := replayDigest(b)
		if op == "review" {
			writeJSON(w, map[string]any{"fingerprint": fingerprint, "evidence": doc})
			return
		}
		if req.Fingerprint == "" || req.Fingerprint != fingerprint {
			writeError(w, 409, "Evidence changed; review this acquisition and sample again")
			return
		}
		if doc.Restricted && !req.AcceptTerms {
			writeError(w, 403, "Review provider terms and confirm permission to export these links")
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="clonoloom-cytoweave-links.json"`)
		w.Write(b)
	}))
}
