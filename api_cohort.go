package main

// Cohort design checks are computed in Go; the browser keeps versioned snapshots in
// the project, as it keeps sample annotations. Dataset identity pins source fingerprints.
import (
	"encoding/json"
	"io"
	"net/http"

	"clonoloom/internal/study"
)

func (a *app) registerCohort(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/cohort/design", func(w http.ResponseWriter, r *http.Request) {
		var input study.CohortInput
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "Send a cohort dataset, complete sample annotations and explicit design spec: "+err.Error())
			return
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			writeError(w, http.StatusBadRequest, "Send exactly one cohort design document.")
			return
		}
		ds, ok := a.datasets.get(input.Dataset)
		if !ok {
			writeError(w, http.StatusNotFound, "Open the cohort's dataset before checking its design.")
			return
		}
		names := sampleNames(ds)
		if len(input.Samples) != len(names) || len(input.Samples) > study.MaxCohortSamples {
			writeError(w, http.StatusBadRequest, "Send every sample in the dataset once (up to 10000), with explicit reasons for exclusions.")
			return
		}
		known := make(map[string]bool, len(names))
		for _, name := range names {
			known[name] = true
		}
		for _, s := range input.Samples {
			if !known[s.Name] {
				writeError(w, http.StatusBadRequest, "Sample "+s.Name+" is not in this dataset.")
				return
			}
		}
		design, err := study.CheckCohort(input)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, design)
	})
}
