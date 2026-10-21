package main

// The table cache (internal/cache) and the artifacts (internal/artifacts) over HTTP: what the cache
// holds, and clearing it; the results kept, and one with its record.

import (
	"errors"
	"net/http"

	"clonoloom/internal/artifacts"
)

func (a *app) registerCache(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/cache", func(w http.ResponseWriter, r *http.Request) {
		if a.cache == nil {
			writeJSON(w, map[string]any{"enabled": false})
			return
		}
		u, err := a.cache.Usage()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, map[string]any{"enabled": true, "dir": u.Dir, "limit": u.Limit, "size": u.Size, "tables": len(u.Items)})
	})
	mux.HandleFunc("DELETE /api/cache", func(w http.ResponseWriter, r *http.Request) {
		if !isLoopbackRequest(r) {
			writeError(w, http.StatusForbidden, "Only this computer may clear the cache.")
			return
		}
		if a.cache == nil {
			writeError(w, http.StatusNotFound, "ClonoLoom has no cache.")
			return
		}
		if err := a.cache.Clear(); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, map[string]bool{"cleared": true})
	})
	mux.HandleFunc("GET /api/artifacts", func(w http.ResponseWriter, r *http.Request) {
		list, err := a.artifacts.List()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if list == nil {
			list = []artifacts.Manifest{}
		}
		writeJSON(w, map[string]any{"artifacts": list})
	})
	mux.HandleFunc("GET /api/artifacts/{id}", func(w http.ResponseWriter, r *http.Request) {
		m, err := a.artifacts.Get(r.PathValue("id"))
		switch {
		case errors.Is(err, artifacts.ErrNotFound):
			writeError(w, http.StatusNotFound, "No such result.")
		case err != nil:
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeJSON(w, m)
		}
	})
}
