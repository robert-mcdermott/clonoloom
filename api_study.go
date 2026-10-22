package main

// A study's design (internal/study; clonoloom-spec/requirements.md, D10 and G3) over HTTP: the page
// sends its samples' annotation (their own fields, else their metadata's) and is answered each
// subject's samples in time order, their reference samples and replicates, and the checks that
// come before tracking. Nothing is kept: the annotation, in the project, is the record.

import (
	"encoding/json"
	"io"
	"net/http"

	"clonoloom/internal/study"
)

// maxStudySamples bounds a request (a cohort's samples, with room to spare).
const maxStudySamples = 100_000

func (a *app) registerStudy(mux *http.ServeMux) {
	// POST /api/study {samples: [{name, subject, timepoint, tissue, reference}]}: the design.
	mux.HandleFunc("POST /api/study", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Samples []study.Sample `json:"samples"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 64<<20)).Decode(&body); err != nil || len(body.Samples) > maxStudySamples {
			writeError(w, http.StatusBadRequest, "Send {\"samples\": [{\"name\", \"subject\", \"timepoint\", \"tissue\", \"reference\"}]}.")
			return
		}
		writeJSON(w, study.Describe(body.Samples))
	})
}
