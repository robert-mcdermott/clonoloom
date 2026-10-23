package main

// B-cell input evidence (wave 5, slice 1): inspected rearrangements under the active QC filter.
// Counts are rows, not clonotypes or cells; the definition is carried for context only.
import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"

	"clonoloom/internal/bcell"
	"clonoloom/internal/clonotype"
	"clonoloom/internal/qc"
	"clonoloom/internal/rep"
)

type bcellRequest struct {
	Dataset    string               `json:"dataset"`
	Definition clonotype.Definition `json:"definition"`
	Filter     qc.Filter            `json:"filter"`
	Offset     int                  `json:"offset"`
	Limit      int                  `json:"limit"`
	ID         string               `json:"id"`
}
type bcellSummary struct {
	Sample   string `json:"sample"`
	Locus    string `json:"locus"`
	Rows     int    `json:"rows"`
	Sequence int    `json:"sequence"`
	Paired   int    `json:"paired"`
	Valid    int    `json:"valid"`
	Regions  int    `json:"regions"`
}
type bcellRow struct {
	ID         string            `json:"id"`
	SequenceID string            `json:"sequenceId"`
	Source     string            `json:"source"`
	Line       uint32            `json:"line"`
	Sample     string            `json:"sample"`
	Locus      string            `json:"locus"`
	Evidence   bcell.Evidence    `json:"evidence"`
	FieldOrder []string          `json:"fieldOrder,omitempty"`
	Fields     map[string]string `json:"fields,omitempty"`
}
type bcellPage struct {
	Describe string         `json:"describe"`
	Filter   qc.Filter      `json:"filter"`
	Count    int            `json:"count"`
	Offset   int            `json:"offset"`
	Limit    int            `json:"limit"`
	Samples  []bcellSummary `json:"samples"`
	Rows     []bcellRow     `json:"rows"`
}

func (a *app) registerBCells(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/bcells/inputs", a.handleBCellInputs)
	mux.HandleFunc("GET /api/families/{id}/page", a.handleFamilyPage)
	mux.HandleFunc("GET /api/germlines/{id}/page", a.handleGermlinePage)
	mux.HandleFunc("GET /api/shm/{id}/page", a.handleSHMPage)
	mux.HandleFunc("GET /api/lineages/{id}/page", a.handleLineagePage)
}
func (a *app) handleBCellInputs(w http.ResponseWriter, r *http.Request) {
	var req bcellRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, 400, "Send a dataset, definition, filter and page or sequence identity.")
		return
	}
	ds, ok := a.datasets.get(req.Dataset)
	if !ok {
		writeErrorCode(w, 404, "not-open", "Open the dataset again from its files.")
		return
	}
	def, err := req.Definition.Normalize()
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	if req.Offset < 0 || req.Limit < 0 || req.Limit > 200 {
		writeError(w, 400, "Use an offset of zero or more and a limit of 1–200.")
		return
	}
	if req.Limit == 0 {
		req.Limit = 50
	}
	t := ds.Result.Table
	keep, _ := qc.Apply(t, req.Filter)
	out := bcellPage{Describe: def.Describe(), Filter: req.Filter, Offset: req.Offset, Limit: req.Limit, Samples: []bcellSummary{}, Rows: []bcellRow{}}
	summaries := make([][3]bcellSummary, len(t.Samples))
	for i, s := range t.Samples {
		for j, locus := range []string{"IGH", "IGK", "IGL"} {
			summaries[i][j] = bcellSummary{Sample: s.Name, Locus: locus}
		}
	}
	for row := 0; row < t.Len(); row++ {
		if row%1024 == 0 && r.Context().Err() != nil {
			return
		}
		if keep != nil && !keep[row] {
			continue
		}
		locus := t.LocusName(row)
		li := -1
		switch locus {
		case "IGH":
			li = 0
		case "IGK":
			li = 1
		case "IGL":
			li = 2
		}
		if li < 0 {
			continue
		}
		raw := t.EvidenceRow(row)
		e := bcell.Check(raw)
		s := &summaries[t.Sample[row]][li]
		s.Rows++
		if e.Sequence {
			s.Sequence++
		}
		if e.Paired {
			s.Paired++
		}
		if e.Valid {
			s.Valid++
		}
		if e.Regions {
			s.Regions++
		}
		rank := out.Count
		out.Count++
		if req.ID == "" && (rank < req.Offset || rank-req.Offset >= req.Limit) {
			continue
		}
		src := ds.Result.Sources[t.Source[row]]
		idsum := sha256.Sum256([]byte(fmt.Sprintf("%s\n%s\n%d\n%d\n%s", src.SHA256, src.Name, t.Source[row], t.Line[row], t.IDs.Get(t.SequenceID[row]))))
		id := "bci-" + hex.EncodeToString(idsum[:16])
		if req.ID != "" && req.ID != id {
			continue
		}
		item := bcellRow{ID: id, SequenceID: t.IDs.Get(t.SequenceID[row]), Source: src.Name, Line: t.Line[row], Sample: s.Sample, Locus: locus, Evidence: e}
		// Page summaries never ship complete sequences; only a selected row does.
		for i := range item.Evidence.Issues {
			if len(item.Evidence.Issues[i].Value) > 120 {
				item.Evidence.Issues[i].Value = item.Evidence.Issues[i].Value[:117] + "..."
			}
		}
		if req.ID != "" {
			item.FieldOrder = rep.EvidenceFields[:]
			item.Fields = map[string]string{}
			for i, name := range rep.EvidenceFields {
				item.Fields[name] = raw[i]
			}
		}
		out.Rows = append(out.Rows, item)
	}
	for _, sample := range summaries {
		for _, s := range sample {
			if s.Rows > 0 {
				out.Samples = append(out.Samples, s)
			}
		}
	}
	if req.ID != "" && len(out.Rows) == 0 {
		writeError(w, 404, "This rearrangement is not present under the active filter.")
		return
	}
	writeJSON(w, out)
}
