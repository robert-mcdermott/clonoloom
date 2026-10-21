package main

// QC (clonoloom-spec/requirements.md, Q1–Q3, Q6): a dataset's samples measured under a filter
// (internal/qc), with the numbers its vendor reported recomputed beside them: Cell Ranger's
// metrics summary, immunoSEQ's sample totals.

import (
	"encoding/json"
	"io"
	"net/http"
	"sort"

	"clonoloom/internal/qc"
	"clonoloom/internal/reads"
)

type sampleChecks struct {
	Sample string     `json:"sample"`
	Source string     `json:"source"`
	Checks []qc.Check `json:"checks"`
}

type qcAnswer struct {
	Dataset string         `json:"dataset"`
	Report  qc.Report      `json:"report"`
	Vendor  []sampleChecks `json:"vendor"`
}

func (a *app) registerQC(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/qc", a.handleQC)
}

// handleQC answers {"dataset": id, "filter": {...}} with the dataset's QC under the filter.
func (a *app) handleQC(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Dataset string    `json:"dataset"`
		Filter  qc.Filter `json:"filter"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "Send {\"dataset\": id, \"filter\": {...}}.")
		return
	}
	ds, ok := a.datasets.get(body.Dataset)
	if !ok {
		writeErrorCode(w, http.StatusNotFound, "not-open", "The dataset is not open: open it again from its files.")
		return
	}
	t := ds.Result.Table
	answer := qcAnswer{Dataset: ds.ID, Report: qc.Compute(t, body.Filter), Vendor: []sampleChecks{}}
	// The vendors' numbers are of the data as written: compared with ClonoLoom's before any filter.
	unfiltered := answer.Report
	if !isZeroFilter(body.Filter) {
		unfiltered = qc.Compute(t, qc.Filter{})
	}
	for _, c := range ds.Result.Companions {
		if c.Kind != reads.CompanionMetrics {
			continue
		}
		metrics, err := qc.CellRangerMetrics(c.Path)
		if err != nil {
			continue
		}
		for _, s := range unfiltered.Samples {
			if s.Name != c.Sample || s.Chains == nil {
				continue
			}
			var loci []string
			for l := range s.Chains.ByLocus {
				loci = append(loci, l)
			}
			sort.Strings(loci)
			if checks := qc.CompareCellRanger(metrics, s.Chains, loci); len(checks) > 0 {
				answer.Vendor = append(answer.Vendor, sampleChecks{Sample: s.Name, Source: "Cell Ranger metrics summary", Checks: checks})
			}
		}
	}
	for i := range t.Samples {
		if checks := qc.CompareImmunoSEQ(t, i); len(checks) > 0 {
			answer.Vendor = append(answer.Vendor, sampleChecks{Sample: t.Samples[i].Name, Source: "immunoSEQ sample columns", Checks: checks})
		}
	}
	writeJSON(w, answer)
}

func isZeroFilter(f qc.Filter) bool {
	return !f.ProductiveOnly && f.MinCount == 0 && len(f.Loci) == 0 && !f.CellsOnly && !f.HighConfidenceOnly && f.MaxChains == 0 && len(f.ExcludeSamples) == 0
}
