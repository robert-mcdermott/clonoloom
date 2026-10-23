package main

// The reference data the analyses use (validation/sources.json's "references"; clonoloom-spec/
// roadmap.md, wave 4): the V genes' loops TCRdist compares, and later wave 4's recombination
// models and specificity databases. Each is listed with its source, size, license and citation,
// downloaded from its publisher only when asked (data under terms of use only once they are
// accepted), checked against its recorded SHA-256, and kept in <data dir>/references, as the
// examples are (api_examples.go).

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"clonoloom/internal/examples"
	"clonoloom/internal/jobs"
	"clonoloom/internal/tcrdist"
)

func newReferences(dir string, offline bool) *examples.Library {
	sources, err := examples.ParseReferences(exampleSources)
	if err != nil {
		log.Printf("the reference data's list cannot be read: %v", err)
	}
	return &examples.Library{Dir: dir, Sources: sources, Recorded: sources, Offline: offline,
		UserAgent: "ClonoLoom/" + version + " (+https://github.com/robert-mcdermott/clonoloom)"}
}

// referenceJobKey: one job per reference at a time.
func referenceJobKey(name string) string { return "reference\x00" + name }

// referenceJob is a reference's latest job: its unfinished one, else its last; nil when none.
func (a *app) referenceJob(name string) *jobs.Job {
	var last *jobs.Job
	for _, j := range a.jobs.List(1) {
		if j.Key != referenceJobKey(name) {
			continue
		}
		j := j
		if !j.State.Finished() {
			return &j
		}
		last = &j
	}
	return last
}

// referenceSpec is a reference's download job's spec.
type referenceSpec struct {
	Reference string `json:"reference"`
	Accept    bool   `json:"accept"`
}

func (a *app) referenceWork(spec referenceSpec) jobs.Func {
	return func(ctx context.Context, run *jobs.Run) error {
		run.Log("info", "Downloading %s into %s.", spec.Reference, a.references.Dir)
		path, err := a.references.Download(ctx, spec.Reference, spec.Accept, func(f float64, message string) { run.Progress(f, message) })
		if err != nil {
			return err
		}
		run.Log("info", "Ready at %s.", path)
		return nil
	}
}

func (a *app) registerReferences(mux *http.ServeMux) {
	a.jobs.Handle("reference", func(spec json.RawMessage) (jobs.Func, error) {
		var s referenceSpec
		if json.Unmarshal(spec, &s) != nil || s.Reference == "" {
			return nil, errors.New("its spec names no reference data")
		}
		return a.referenceWork(s), nil
	}, nil)
	mux.HandleFunc("GET /api/references", func(w http.ResponseWriter, r *http.Request) {
		list := a.references.List()
		out := make([]exampleState, len(list))
		for i, s := range list {
			out[i].State = s
			j := a.referenceJob(s.Name)
			switch {
			case j == nil:
			case !j.State.Finished():
				out[i].State.State = "downloading"
				out[i].Download = &download{Job: j.ID, Fraction: j.Progress, Message: j.Message}
			case j.State == jobs.Failed || j.State == jobs.Orphaned:
				out[i].Download = &download{Job: j.ID, Error: j.Error}
			case j.State == jobs.Cancelled:
				out[i].Download = &download{Job: j.ID, Error: "Cancelled."}
			}
		}
		writeJSON(w, map[string]any{"dir": a.references.Dir, "offline": a.references.Offline, "hosts": a.references.Hosts(), "examples": out})
	})
	mux.HandleFunc("POST /api/references/{name}", func(w http.ResponseWriter, r *http.Request) {
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
		for i := range a.references.Sources {
			if a.references.Sources[i].Name == name {
				source = &a.references.Sources[i]
			}
		}
		switch {
		case source == nil:
			writeError(w, http.StatusNotFound, "No such reference data.")
			return
		case a.references.Offline:
			writeErrorCode(w, http.StatusForbidden, "offline", examples.ErrOffline.Error()+".")
			return
		case source.LicenseClass != "open" && !body.Accept:
			writeErrorCode(w, http.StatusForbidden, "terms", "These data are under "+source.License+": read the terms and accept them to download.")
			return
		}
		spec := referenceSpec{Reference: name, Accept: body.Accept}
		job, existing, err := a.jobs.Submit("reference", "Download "+source.Title, referenceJobKey(name), spec, a.referenceWork(spec))
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, err.Error())
			return
		}
		writeJSON(w, map[string]any{"started": !existing, "job": job.ID})
	})
	mux.HandleFunc("POST /api/references/{name}/cancel", func(w http.ResponseWriter, r *http.Request) {
		j := a.referenceJob(r.PathValue("name"))
		if j == nil || j.State.Finished() {
			writeError(w, http.StatusNotFound, "It is not being downloaded.")
			return
		}
		writeJSON(w, map[string]bool{"cancelled": a.jobs.Cancel(j.ID)})
	})
	mux.HandleFunc("DELETE /api/references/{name}", func(w http.ResponseWriter, r *http.Request) {
		if !isLoopbackRequest(r) {
			writeError(w, http.StatusForbidden, "Only this computer may remove files.")
			return
		}
		name := r.PathValue("name")
		if j := a.referenceJob(name); j != nil && !j.State.Finished() {
			writeError(w, http.StatusConflict, "It is being downloaded.")
			return
		}
		if err := a.references.Remove(name); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, map[string]bool{"removed": true})
	})
}

// loopsReference names the reference data of the V genes' loops.
const loopsReference = "tcrdist3-loops"

// loopsFile is the loops' database as a result records it: its file, checksum, source, license
// and citation.
type loopsFile struct {
	Reference string `json:"reference"`
	File      string `json:"file"`
	SHA256    string `json:"sha256"`
	Source    string `json:"source"`
	License   string `json:"license"`
	Citation  string `json:"citation"`
}

// loopsCache keeps the loops' database read once, by its file's checksum.
type loopsCache struct {
	mu    sync.Mutex
	file  loopsFile
	loops *tcrdist.Loops
}

// errNoLoops: the loops' database is not downloaded.
var errNoLoops = errors.New("the V genes' loops are not downloaded: download them (Reference data), or compare the CDR3s alone")

// tcrdistLoops answers the V genes' loops, read from the downloaded database (once, while its
// file is unchanged), with the file's record.
func (a *app) tcrdistLoops() (*tcrdist.Loops, loopsFile, error) {
	var state examples.State
	for _, s := range a.references.List() {
		if s.Name == loopsReference {
			state = s
		}
	}
	if state.State != "ready" || len(state.Files) == 0 {
		return nil, loopsFile{}, errNoLoops
	}
	path := filepath.Join(state.Path, filepath.FromSlash(state.Files[0].Path))
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, loopsFile{}, err
	}
	sum := sha256.Sum256(raw)
	file := loopsFile{Reference: loopsReference, File: filepath.Base(path), SHA256: hex.EncodeToString(sum[:]), Source: state.Source.Source, License: state.License, Citation: state.Citation}
	if file.SHA256 != state.Files[0].SHA256 {
		return nil, loopsFile{}, errors.New("the V genes' loops on this computer are not the file ClonoLoom records: download them again")
	}
	a.loops.mu.Lock()
	defer a.loops.mu.Unlock()
	if a.loops.loops != nil && a.loops.file.SHA256 == file.SHA256 {
		return a.loops.loops, a.loops.file, nil
	}
	loops, err := tcrdist.ParseLoops(bytes.NewReader(raw))
	if err != nil {
		return nil, loopsFile{}, err
	}
	a.loops.loops, a.loops.file = loops, file
	return loops, file, nil
}
