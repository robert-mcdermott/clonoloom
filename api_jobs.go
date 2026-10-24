package main

// Jobs (internal/jobs) over HTTP: the list, one job with its log, cancelling, resuming an
// interrupted job and discarding a finished one, and a stream of server-sent events that sends the
// list whenever a job changes, so the page follows progress without polling and finds running jobs
// again after a reload. handleJobs tells the manager how each kind of job is resumed (from its
// spec). Adapted from SituCraft 0.9.0 api_jobs.go.

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"clonoloom/internal/jobs"
)

// jobLogLines is how many of each job's last log lines the list and the stream carry.
const jobLogLines = 5

func (a *app) registerJobs(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/jobs", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"jobs": a.jobs.List(jobLogLines)})
	})
	mux.HandleFunc("GET /api/jobs/events", a.jobEvents)
	mux.HandleFunc("GET /api/jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
		job, ok := a.jobs.Get(r.PathValue("id"))
		if !ok {
			writeError(w, http.StatusNotFound, "No such job.")
			return
		}
		writeJSON(w, job)
	})
	mux.HandleFunc("POST /api/jobs/{id}/resume", func(w http.ResponseWriter, r *http.Request) {
		job, err := a.jobs.Resume(r.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeJSON(w, job)
	})
	mux.HandleFunc("POST /api/jobs/{id}/discard", func(w http.ResponseWriter, r *http.Request) {
		if err := a.jobs.Discard(r.PathValue("id")); err != nil {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeJSON(w, map[string]bool{"discarded": true})
	})
	mux.HandleFunc("POST /api/jobs/{id}/cancel", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if _, ok := a.jobs.Get(id); !ok {
			writeError(w, http.StatusNotFound, "No such job.")
			return
		}
		writeJSON(w, map[string]bool{"cancelled": a.jobs.Cancel(id)})
	})
}

// Changes closer together than this are sent as one event.
const jobEventInterval = 200 * time.Millisecond

// jobEvents streams "jobs" events, each the whole list, at most every jobEventInterval, with a
// comment now and then to keep the connection open. It ends when the page goes away or the
// program stops.
func (a *app) jobEvents(w http.ResponseWriter, r *http.Request) {
	rc := http.NewResponseController(w)
	header := w.Header()
	header.Set("Content-Type", "text/event-stream")
	header.Set("Cache-Control", "no-store")
	changes, stop := a.jobs.Subscribe()
	defer stop()
	send := func() bool {
		raw, err := json.Marshal(a.jobs.List(jobLogLines))
		if err != nil {
			return false
		}
		if _, err := fmt.Fprintf(w, "event: jobs\ndata: %s\n\n", raw); err != nil {
			return false
		}
		return rc.Flush() == nil
	}
	if !send() {
		return
	}
	keepAlive := time.NewTicker(25 * time.Second)
	defer keepAlive.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-keepAlive.C:
			if _, err := fmt.Fprint(w, ": still here\n\n"); err != nil || rc.Flush() != nil {
				return
			}
		case <-changes:
			// Let a burst of progress settle into one event.
			select {
			case <-time.After(jobEventInterval):
			case <-r.Context().Done():
				return
			}
			select {
			case <-changes:
			default:
			}
			if !send() {
				return
			}
		}
	}
}

// handleJobs registers how each kind of job is resumed from its spec.
func (a *app) handleJobs() {
	a.handleExpressionMapJobs()
	a.jobs.Handle("example", func(spec json.RawMessage) (jobs.Func, error) {
		var s exampleSpec
		if json.Unmarshal(spec, &s) != nil || s.Example == "" {
			return nil, errors.New("its spec names no example")
		}
		return a.exampleWork(s), nil
	}, nil)
	for _, k := range analyses {
		a.jobs.Handle(k.kind, func(spec json.RawMessage) (jobs.Func, error) {
			var s analysisSpec
			if err := json.Unmarshal(spec, &s); err != nil || s.Dataset == "" {
				return nil, errors.New("its spec names no dataset")
			}
			if s.Method != k.version {
				return nil, fmt.Errorf("this job uses method version %d; %s now uses version %d: start a new analysis", s.Method, k.title, k.version)
			}
			return k.work(a, k, s), nil
		}, nil)
	}
}
