package main

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"clonoloom/internal/adc"
	"clonoloom/internal/airr"
)

func (a *app) adcDownload(w http.ResponseWriter, r *http.Request) {
	if !isLoopbackRequest(r) {
		writeError(w, http.StatusForbidden, "Only this computer may download ADC evidence.")
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 16385))
	if err != nil || len(raw) > 16384 {
		writeError(w, http.StatusBadRequest, "Download request exceeds 16 KiB.")
		return
	}
	if err = airr.CheckExactJSON(r.Context(), raw); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var spec adc.DownloadSpec
	if err = strictJSON(raw, &spec); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	plan, err := adc.PlanDownload(spec)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if r.URL.Path == "/api/adc/download/preview" {
		writeJSON(w, plan)
		return
	}
	if !spec.Accept || strings.TrimSpace(spec.License) == "" || strings.TrimSpace(spec.Citation) == "" {
		writeError(w, http.StatusForbidden, "Record provider permission evidence and citation, then explicitly accept the download.")
		return
	}
	result, err := a.adc.Download(r.Context(), a.store.dir, spec)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, adc.ErrOffline) {
			status = http.StatusForbidden
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, result)
}
