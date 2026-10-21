package main

// The example datasets (internal/examples, D13 and E2): listed, downloaded in the background
// when asked (the page polls the list for progress), cancelled, removed. The list is
// validation/sources.json, the same the validation suites download. Adapted from SituCraft 0.9.0
// api_examples.go (without its jobs).

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"sync"

	"clonoloom/internal/examples"
)

//go:embed validation/sources.json
var exampleSources []byte

func newExamples(dir string, offline bool) *examples.Library {
	sources, err := examples.Parse(exampleSources)
	if err != nil {
		log.Printf("the example list cannot be read: %v", err)
	}
	return &examples.Library{Dir: dir, Sources: sources, Offline: offline,
		UserAgent: "ClonoLoom/" + version + " (+https://github.com/robert-mcdermott/clonoloom)"}
}

// download is an example being downloaded, or the last that failed.
type download struct {
	Fraction float64 `json:"fraction"`
	Message  string  `json:"message"`
	Error    string  `json:"error,omitempty"`
	cancel   context.CancelFunc
}

type downloads struct {
	mu      sync.Mutex
	running map[string]*download
	failed  map[string]*download
}

type exampleState struct {
	examples.State
	Download *download `json:"download,omitempty"`
}

func (a *app) registerExamples(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/examples", func(w http.ResponseWriter, r *http.Request) {
		list := a.examples.List()
		out := make([]exampleState, len(list))
		a.downloads.mu.Lock()
		for i, s := range list {
			out[i].State = s
			if d := a.downloads.running[s.Name]; d != nil {
				out[i].State.State = "downloading"
				c := *d
				out[i].Download = &c
			} else if d := a.downloads.failed[s.Name]; d != nil {
				c := *d
				out[i].Download = &c
			}
		}
		a.downloads.mu.Unlock()
		writeJSON(w, map[string]any{"dir": a.examples.Dir, "offline": a.examples.Offline, "hosts": a.examples.Hosts(), "examples": out})
	})
	mux.HandleFunc("POST /api/examples/{name}", a.downloadExample)
	mux.HandleFunc("POST /api/examples/{name}/cancel", func(w http.ResponseWriter, r *http.Request) {
		a.downloads.mu.Lock()
		d := a.downloads.running[r.PathValue("name")]
		a.downloads.mu.Unlock()
		if d == nil {
			writeError(w, http.StatusNotFound, "It is not being downloaded.")
			return
		}
		d.cancel()
		writeJSON(w, map[string]bool{"cancelled": true})
	})
	mux.HandleFunc("DELETE /api/examples/{name}", func(w http.ResponseWriter, r *http.Request) {
		if !isLoopbackRequest(r) {
			writeError(w, http.StatusForbidden, "Only this computer may remove files.")
			return
		}
		name := r.PathValue("name")
		a.downloads.mu.Lock()
		running := a.downloads.running[name] != nil
		a.downloads.mu.Unlock()
		if running {
			writeError(w, http.StatusConflict, "It is being downloaded.")
			return
		}
		if err := a.examples.Remove(name); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, map[string]bool{"removed": true})
	})
}

// downloadExample starts downloading an example, {"accept": true} accepting its provider's terms
// of use where it has them. Only this computer may ask.
func (a *app) downloadExample(w http.ResponseWriter, r *http.Request) {
	if !isLoopbackRequest(r) {
		writeError(w, http.StatusForbidden, "Only this computer may download into its folders.")
		return
	}
	var body struct {
		Accept bool `json:"accept"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<10)).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "Send {\"accept\": true|false}.")
		return
	}
	name := r.PathValue("name")
	var source *examples.Source
	for i := range a.examples.Sources {
		if a.examples.Sources[i].Name == name {
			source = &a.examples.Sources[i]
		}
	}
	switch {
	case source == nil:
		writeError(w, http.StatusNotFound, "No such example.")
		return
	case a.examples.Offline:
		writeErrorCode(w, http.StatusForbidden, "offline", examples.ErrOffline.Error()+".")
		return
	case source.LicenseClass != "open" && !body.Accept:
		writeErrorCode(w, http.StatusForbidden, "terms", "These data are under "+source.License+": read the terms and accept them to download.")
		return
	}
	a.downloads.mu.Lock()
	defer a.downloads.mu.Unlock()
	if a.downloads.running[name] != nil {
		writeJSON(w, map[string]bool{"started": false})
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	d := &download{Message: "Starting", cancel: cancel}
	a.downloads.running[name] = d
	delete(a.downloads.failed, name)
	go func() {
		defer cancel()
		_, err := a.examples.Download(ctx, name, body.Accept, func(f float64, message string) {
			a.downloads.mu.Lock()
			d.Fraction, d.Message = f, message
			a.downloads.mu.Unlock()
		})
		a.downloads.mu.Lock()
		delete(a.downloads.running, name)
		if err != nil {
			d.Error = err.Error()
			if errors.Is(err, context.Canceled) {
				d.Error = "Cancelled."
			}
			a.downloads.failed[name] = d
			log.Printf("example %s: %v", name, err)
		}
		a.downloads.mu.Unlock()
	}()
	writeJSON(w, map[string]bool{"started": true})
}

// credit is the source, license and citation of an example a dataset was opened from (O9).
type credit struct {
	Example      string `json:"example"`
	Title        string `json:"title"`
	Source       string `json:"source"`
	Page         string `json:"page"`
	License      string `json:"license"`
	LicenseURL   string `json:"licenseUrl,omitempty"`
	LicenseClass string `json:"licenseClass"`
	Citation     string `json:"citation"`
}

// credits answers the examples a dataset's files are, by their SHA-256: wherever and however
// they were downloaded, the dataset carries their source, license and citation.
func (a *app) credits(sha256s []string) []credit {
	var out []credit
	for _, s := range a.examples.Identify(sha256s) {
		out = append(out, credit{Example: s.Name, Title: s.Title, Source: s.Source, Page: s.Page, License: s.License, LicenseURL: s.LicenseURL, LicenseClass: s.LicenseClass, Citation: s.Citation})
	}
	return out
}
