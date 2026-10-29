package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"time"
	"unicode/utf8"
)

func (a *app) registerDeidentification(mux *http.ServeMux) {
	shelf := &deidShelf{}
	mux.HandleFunc("POST /api/deidentification/prepare", func(w http.ResponseWriter, r *http.Request) {
		shelf.mu.Lock()
		if shelf.preparing {
			shelf.mu.Unlock()
			writeError(w, 409, "A de-identification export is already being prepared.")
			return
		}
		shelf.preparing = true
		shelf.current = nil
		shelf.mu.Unlock()
		defer func() { shelf.mu.Lock(); shelf.preparing = false; shelf.mu.Unlock() }()
		raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 4<<20))
		var body deidRequest
		if e == nil && !utf8.Valid(raw) {
			writeError(w, 400, "Settings must be valid UTF-8.")
			return
		}
		if e == nil {
			e = strictRunJSON(raw, &body)
		}
		if e != nil {
			writeError(w, 400, "Send bounded de-identification settings and sample annotations.")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
		defer cancel()
		prepared, e := a.prepareDeidentified(ctx, body)
		if e != nil {
			writeError(w, 409, e.Error())
			return
		}
		shelf.mu.Lock()
		shelf.current = prepared
		shelf.mu.Unlock()
		// Clear private correspondence even if the host remains idle. Replaced packages
		// are not kept alive by this callback; only the random token is captured.
		token := prepared.Preview.Token
		time.AfterFunc(deidLifetime, func() {
			shelf.mu.Lock()
			defer shelf.mu.Unlock()
			if shelf.current != nil && shelf.current.Preview.Token == token {
				shelf.current = nil
			}
		})
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, prepared.Preview)
	})
	mux.HandleFunc("GET /api/deidentification/{token}", func(w http.ResponseWriter, r *http.Request) {
		shelf.mu.Lock()
		p := shelf.current
		if p != nil && time.Now().After(p.Expires) {
			shelf.current = nil
			p = nil
		}
		shelf.mu.Unlock()
		if p == nil || p.Preview.Token != r.PathValue("token") {
			writeError(w, 404, "Prepared export expired or was replaced; prepare and review again.")
			return
		}
		if r.URL.Query().Get("expected") != p.Preview.Fingerprint {
			writeError(w, 409, "Supply the reviewed export fingerprint.")
			return
		}
		var raw []byte
		var name, kind string
		switch r.URL.Query().Get("part") {
		case "bundle":
			raw = p.Bundle
			name = "clonoloom-deidentified.zip"
			kind = "application/zip"
		case "identity-map":
			raw = p.IdentityMap
			name = "clonoloom-private-identity-map.json"
			kind = "application/json"
		default:
			writeError(w, 400, "Choose bundle or identity-map explicitly.")
			return
		}
		w.Header().Set("Content-Type", kind)
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
		w.Header().Set("X-ClonoLoom-Deidentification", p.Preview.Fingerprint)
		http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(raw))
	})
}
