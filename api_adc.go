package main

import (
	"errors"
	"io"
	"net/http"

	"clonoloom/internal/adc"
	"clonoloom/internal/airr"
)

func (a *app) registerADC(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/adc/download", a.adcDownload)
	mux.HandleFunc("POST /api/adc/download/preview", a.adcDownload)
	mux.HandleFunc("GET /api/adc/repositories", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"repositories": adc.Repositories(), "contract": adc.Contract, "registryRevision": adc.RegistryRevision, "registrySHA256": adc.RegistrySHA256, "offline": a.adc.Offline, "maxPage": adc.MaxPage, "maxOffset": adc.MaxOffset})
	})
	mux.HandleFunc("POST /api/adc/{operation}", a.adcRequest)
}

// ADC browsing is a local-window operation, not a new automation action. It cannot derive
// queries from a dataset or install results. Every outbound request must be explicit.
func (a *app) adcRequest(w http.ResponseWriter, r *http.Request) {
	if !isLoopbackRequest(r) {
		writeError(w, http.StatusForbidden, "Only this computer may request ADC metadata.")
		return
	}
	operation := r.PathValue("operation")
	if operation != "prepare" && operation != "query" && operation != "info" {
		writeError(w, http.StatusNotFound, "Unknown ADC operation.")
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, (8<<10)+1))
	if err != nil || len(raw) > 8<<10 {
		writeError(w, http.StatusBadRequest, "ADC request exceeds 8 KiB or could not be read.")
		return
	}
	var body struct {
		Query   adc.Query `json:"query"`
		Confirm bool      `json:"confirm"`
	}
	if err := airr.CheckExactJSON(r.Context(), raw); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := strictJSON(raw, &body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	preview, err := adc.Prepare(body.Query)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if operation == "prepare" {
		writeJSON(w, preview)
		return
	}
	if !body.Confirm {
		writeErrorCode(w, http.StatusForbidden, "confirm", "Review the endpoint and query, then explicitly confirm this outbound request.")
		return
	}
	var result any
	if operation == "info" {
		result, err = a.adc.Info(r.Context(), body.Query.Repository)
	} else {
		result, err = a.adc.Query(r.Context(), body.Query)
	}
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, adc.ErrOffline) {
			writeErrorCode(w, http.StatusForbidden, "offline", err.Error())
			return
		}
		writeError(w, status, err.Error())
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, result)
}
