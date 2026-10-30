package main

// Local descriptive review with native evidence. No implicit remote action.
import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"

	"clonoloom/internal/airr"
	"clonoloom/internal/artifacts"
	"clonoloom/internal/bcell"
)

type candidateRequest struct {
	Families       string                 `json:"families"`
	SHM            string                 `json:"shm"`
	Isotypes       string                 `json:"isotypes"`
	Options        bcell.CandidateOptions `json:"options"`
	Review         string                 `json:"review"`
	Observation    string                 `json:"observation"`
	Row            string                 `json:"row"`
	Selection      string                 `json:"selection"`
	AcceptTerms    bool                   `json:"acceptTerms"`
	AcceptSequence bool                   `json:"acceptSequence"`
}
type candidateParent struct {
	Manifest artifacts.Manifest `json:"manifest"`
	Text     string             `json:"text"`
	SHA256   string             `json:"sha256"`
}
type candidateDocument struct {
	Format      string                `json:"format"`
	Version     int                   `json:"version"`
	Software    string                `json:"software"`
	Dataset     string                `json:"dataset"`
	DatasetName string                `json:"datasetName"`
	Parents     []candidateParent     `json:"parents"`
	Result      bcell.CandidateResult `json:"result"`
	Notice      string                `json:"notice"`
	Restricted  bool                  `json:"restricted"`
	Limits      string                `json:"limits"`
}

const candidateSequenceLimits = "One unchanged supplied AIRR sequence_aa chain, not a complete receptor or synthesis-ready construct. complete_vdj describes the annotated V(D)J region, not full-protein completeness. No translation, repair, consensus, chain assembly, codon optimization or inferred ancestor. A cell-associated heavy/light assignment is the saved family's operational definition, not independent validation of biological pairing. FASTA uses an opaque safe identifier; original identities and annotations remain in the companion evidence. Proteoscope review requires fresh local/public/provider permissions and sends nothing on opening. This review is separate from project artifacts and scientific replay."

func (a *app) candidateParent(ctx context.Context, id, kind string, target any) (candidateParent, error) {
	var out candidateParent
	m, e := a.artifacts.Get(id)
	if e != nil || m.Kind != kind || m.State != artifacts.Complete {
		return out, errors.New("Choose a completed " + kind + " result")
	}
	path, e := a.artifacts.Path(id, kind+".json")
	if e != nil {
		return out, e
	}
	f, e := os.Open(path)
	if e != nil {
		return out, e
	}
	defer f.Close()
	raw, e := io.ReadAll(io.LimitReader(runContextReader{ctx, f}, (16<<20)+1))
	if e != nil {
		return out, e
	}
	if len(raw) > 16<<20 {
		return out, errors.New("Candidate parent exceeds 16 MiB")
	}
	hash := ""
	for _, v := range m.Files {
		if v.Path == kind+".json" {
			hash = v.SHA256
		}
	}
	if replayDigest(raw) != hash {
		return out, errors.New("Candidate parent checksum changed")
	}
	if e = airr.CheckExactJSON(ctx, raw); e != nil {
		return out, e
	}
	if e = json.Unmarshal(raw, target); e != nil {
		return out, e
	}
	return candidateParent{m, string(raw), hash}, nil
}
func (a *app) candidateReview(ctx context.Context, req candidateRequest) (candidateDocument, error) {
	out := candidateDocument{Format: "clonoloom-bcell-candidates", Version: bcell.CandidateVersion, Software: "ClonoLoom " + version + "; " + buildCommit(), Parents: []candidateParent{}, Limits: bcell.CandidateLimits + " " + candidateSequenceLimits}
	var f familyDocument
	p, e := a.candidateParent(ctx, req.Families, "families", &f)
	if e != nil {
		return out, e
	}
	if f.Format != "clonoloom-families" || f.Version != bcell.FamilyVersion || f.Dataset != p.Manifest.Dataset {
		return out, errors.New("Unsupported family parent")
	}
	out.Parents = append(out.Parents, p)
	out.Dataset = f.Dataset
	out.DatasetName = f.DatasetName
	ds, ok := a.datasets.get(f.Dataset)
	if !ok {
		return out, errors.New("Open the family result's dataset to review provider terms")
	}
	if req.Options.Baseline != "" && (f.Subjects[req.Options.Sample] == "" || f.Subjects[req.Options.Sample] != f.Subjects[req.Options.Baseline]) {
		return out, errors.New("Baseline requires the same explicit saved subject")
	}
	var mutation *bcell.SHMResult
	var iso *bcell.IsotypeResult
	if req.SHM != "" {
		var s shmDocument
		q, e := a.candidateParent(ctx, req.SHM, "shm", &s)
		if e != nil {
			return out, e
		}
		if s.Format != "clonoloom-shm" || s.Version != bcell.SHMVersion || s.Dataset != f.Dataset || q.Manifest.Dataset != f.Dataset || s.Germlines.Families != req.Families || s.Germlines.FamilySHA256 != p.SHA256 {
			return out, errors.New("SHM result must derive from this exact family parent")
		}
		var g germlineDocument
		gp, e := a.candidateParent(ctx, s.Options.Germlines, "germlines", &g)
		if e != nil {
			return out, e
		}
		if gp.SHA256 != s.Options.GermlineSHA256 || g.Format != "clonoloom-germlines" || g.Version != bcell.GermlineVersion || g.Dataset != f.Dataset || gp.Manifest.Dataset != f.Dataset || g.Options.Families != req.Families || g.Options.FamilySHA256 != p.SHA256 {
			return out, errors.New("SHM germline provenance differs from the selected family result")
		}
		out.Parents = append(out.Parents, gp, q)
		mutation = &s.SHMResult
	}
	if req.Isotypes != "" {
		var i isotypeDocument
		q, e := a.candidateParent(ctx, req.Isotypes, "isotypes", &i)
		if e != nil {
			return out, e
		}
		if i.Format != "clonoloom-isotypes" || i.Version != bcell.IsotypeVersion || i.Dataset != f.Dataset || q.Manifest.Dataset != f.Dataset || i.Options.Families != req.Families || i.Options.FamilySHA256 != p.SHA256 {
			return out, errors.New("Isotypes must derive from this exact family parent")
		}
		out.Parents = append(out.Parents, q)
		iso = &i.IsotypeResult
	}
	total := 0
	for _, p := range out.Parents {
		total += len(p.Text)
	}
	if total > 32<<20 {
		return out, errors.New("Combined candidate parents exceed 32 MiB")
	}
	out.Result, e = bcell.CandidateReview(ctx, f.FamilyResult, mutation, iso, req.Options)
	if e != nil {
		return out, e
	}
	n := a.notice(ds, string(ds.Result.Table.Unit), f.Filter, f.Description, "")
	n.Made = p.Manifest.Created
	n.Derived = true
	n.What = bcell.CandidateLimits
	n.Method = "Descriptive ordered criteria (ClonoLoom policy); AIRR Rearrangement 2.0. Family, reference, mutation and isotype methods/citations are retained in the parent manifests. No calibrated binding or efficacy model."
	out.Notice, out.Restricted = n.Text(), n.Restricted()
	return out, nil
}

type candidateSequence struct {
	Format      string                  `json:"format"`
	Version     int                     `json:"version"`
	Review      string                  `json:"review"`
	Observation bcell.FamilyObservation `json:"observation"`
	Row         bcell.FamilyRow         `json:"row"`
	Dataset     string                  `json:"dataset"`
	RowIndex    int                     `json:"rowIndex"`
	HeaderText  string                  `json:"headerText"`
	RowText     string                  `json:"rowText"`
	Fields      map[string]string       `json:"fields"`
	Sequence    string                  `json:"sequence"`
	SHA256      string                  `json:"sha256"`
	FASTA       string                  `json:"fasta"`
	Eligible    bool                    `json:"eligible"`
	Reason      string                  `json:"reason"`
	Notice      string                  `json:"notice"`
	Restricted  bool                    `json:"restricted"`
	Limits      string                  `json:"limits"`
}

func (a *app) candidateSequence(ctx context.Context, doc candidateDocument, req candidateRequest) (candidateSequence, error) {
	out := candidateSequence{Format: "clonoloom-candidate-sequence", Version: 1, Review: req.Review, Dataset: doc.Dataset, RowIndex: -1, Notice: doc.Notice, Restricted: doc.Restricted, Limits: candidateSequenceLimits}
	found := false
	for _, c := range doc.Result.Rows {
		for _, o := range c.Observations {
			if o.ID == req.Observation {
				out.Observation = o
				for _, r := range o.Rows {
					if r.ID == req.Row {
						out.Row = r
						found = true
					}
				}
			}
		}
	}
	if !found {
		return out, errors.New("Select one current-sample observation and its explicit source chain")
	}
	ds, ok := a.datasets.get(doc.Dataset)
	if !ok {
		return out, errors.New("Dataset closed; reopen and review again")
	}
	t := ds.Result.Table
	for i := 0; i < t.Len(); i++ {
		if i%1024 == 0 {
			if e := ctx.Err(); e != nil {
				return out, e
			}
		}
		source := ds.Result.Sources[t.Source[i]]
		if source.Name == out.Row.Source.Name && source.SHA256 == out.Row.Source.SHA256 && t.Line[i] == out.Row.Line && t.IDs.Get(t.SequenceID[i]) == out.Row.SequenceID && t.Samples[t.Sample[i]].Name == out.Observation.Sample && t.LocusName(i) == out.Row.Locus {
			if out.RowIndex >= 0 {
				return out, errors.New("Source row identity is ambiguous in the opened dataset")
			}
			out.RowIndex = i
		}
	}
	if out.RowIndex < 0 {
		return out, errors.New("Source observation differs from the opened dataset")
	}
	source := ds.Result.Sources[t.Source[out.RowIndex]]
	var e error
	out.HeaderText, out.RowText, out.Fields, e = handoffSource(ctx, source, out.Row.Line)
	if e != nil {
		return out, e
	}
	if out.Fields["sequence_id"] != out.Row.SequenceID || out.Row.SequenceID == "" || out.Fields["locus"] != out.Row.Locus || out.Fields["cell_id"] != out.Observation.Cell {
		return out, errors.New("Original source identity differs from the saved observation")
	}
	out.Sequence = out.Fields["sequence_aa"]
	out.SHA256 = replayDigest([]byte(out.Sequence))
	out.Eligible = (out.Row.Locus == "IGH" || out.Row.Locus == "IGK" || out.Row.Locus == "IGL") && len(out.Sequence) >= 25 && len(out.Sequence) <= 5000 && handoffSequence.MatchString(out.Sequence)
	if !out.Eligible {
		out.Reason = "Need an unchanged supplied sequence_aa of 25–5000 canonical uppercase amino acids and a declared B-cell locus. Missing, stopped, gapped or ambiguous sequences are not repaired."
	} else {
		out.FASTA = ">candidate-" + replayDigest(mustJSON(out.Row))[:24] + " locus=" + out.Row.Locus + " field=sequence_aa\n" + out.Sequence + "\n"
	}
	return out, nil
}
func (a *app) registerCandidates(mux *http.ServeMux) {
	running := make(chan struct{}, 2)
	mux.HandleFunc("POST /api/bcell-candidates/{operation}", localOnly(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		op := r.PathValue("operation")
		switch op {
		case "review", "export", "selection", "selection-export", "fasta":
		default:
			writeError(w, 404, "Unknown candidate operation")
			return
		}
		raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 32<<10))
		var req candidateRequest
		if e == nil {
			e = airr.CheckExactJSON(r.Context(), raw)
		}
		if e == nil {
			e = strictRunJSON(raw, &req)
		}
		if e != nil {
			writeError(w, 400, "Choose explicit candidate parents and ranking criteria")
			return
		}
		select {
		case running <- struct{}{}:
			defer func() { <-running }()
		default:
			writeError(w, 409, "Two candidate operations are running; wait or cancel one")
			return
		}
		doc, e := a.candidateReview(r.Context(), req)
		if e != nil {
			writeError(w, 400, e.Error())
			return
		}
		body, e := json.MarshalIndent(doc, "", "  ")
		if e != nil || len(body) > 64<<20 {
			writeError(w, 400, "Candidate evidence exceeds 64 MiB")
			return
		}
		hash := replayDigest(body)
		if op == "review" {
			writeJSON(w, map[string]any{"review": hash, "evidence": json.RawMessage(body)})
			return
		}
		if req.Review != hash {
			writeError(w, 409, "Candidate evidence or provider notice changed; review again")
			return
		}
		if (op == "export" || op == "selection-export" || op == "fasta") && doc.Restricted && !req.AcceptTerms {
			writeError(w, 403, "Review provider terms and confirm export permission")
			return
		}
		name := "clonoloom-bcell-candidates.json"
		if op != "export" {
			selected, e := a.candidateSequence(r.Context(), doc, req)
			if e != nil {
				writeError(w, 400, e.Error())
				return
			}
			body, e = json.MarshalIndent(selected, "", "  ")
			if e != nil {
				writeError(w, 400, e.Error())
				return
			}
			fingerprint := replayDigest(body)
			if op == "selection" {
				writeJSON(w, map[string]any{"selection": fingerprint, "evidence": json.RawMessage(body)})
				return
			}
			if req.Selection != fingerprint {
				writeError(w, 409, "Sequence review changed; select and review again")
				return
			}
			name = "clonoloom-candidate-sequence.json"
			if op == "fasta" {
				if !selected.Eligible || !req.AcceptSequence {
					writeError(w, 403, "Review sequence completeness and pairing limits before FASTA export")
					return
				}
				body = []byte(selected.FASTA)
				name = "clonoloom-candidate.fasta"
			}
		}
		if e = r.Context().Err(); e != nil {
			writeError(w, 400, e.Error())
			return
		}
		if strings.HasSuffix(name, ".fasta") {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		} else {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
		}
		w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
		w.Write(body)
	}))
}
