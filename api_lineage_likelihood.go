package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"clonoloom/internal/airr"
	"clonoloom/internal/artifacts"
	"clonoloom/internal/bcell"
	"clonoloom/internal/export"
	"clonoloom/internal/qc"
)

type likelihoodRequest struct {
	Artifact    string `json:"artifact"`
	Tree        string `json:"tree"`
	Review      string `json:"review"`
	AcceptTerms bool   `json:"acceptTerms"`
}
type likelihoodDocument struct {
	Format       string             `json:"format"`
	Version      int                `json:"version"`
	Software     string             `json:"software"`
	Commit       string             `json:"commit"`
	Parent       artifacts.Manifest `json:"parent"`
	ParentText   string             `json:"parentText"`
	ParentSHA256 string             `json:"parentSHA256"`
	Panel        bcell.LineageTree  `json:"panel"`
	Result       bcell.MLResult     `json:"result"`
	Citation     string             `json:"citation"`
	Notice       string             `json:"notice"`
	Restricted   bool               `json:"restricted"`
	Limits       string             `json:"limits"`
}

const likelihoodCitation = "Jukes & Cantor 1969, Evolution of Protein Molecules, doi:10.1016/B978-1-4832-3211-9.50009-7; Felsenstein 1981, doi:10.1007/BF01734359. JC69 baseline, not a somatic-hypermutation context model."

func (a *app) likelihoodParent(id, tree string) (artifacts.Manifest, []byte, bcell.LineageTree, error) {
	m, e := a.artifacts.Get(id)
	empty := bcell.LineageTree{}
	if e != nil || m.Kind != "lineages" || m.State != artifacts.Complete {
		return m, nil, empty, errors.New("Select a completed saved lineage result")
	}
	path, e := a.artifacts.Path(id, "lineages.json")
	if e != nil {
		return m, nil, empty, e
	}
	f, e := os.Open(path)
	if e != nil {
		return m, nil, empty, e
	}
	defer f.Close()
	raw, e := io.ReadAll(io.LimitReader(f, (16<<20)+1))
	if e != nil {
		return m, nil, empty, e
	}
	if len(raw) > 16<<20 {
		return m, nil, empty, errors.New("Saved lineage parent exceeds 16 MiB")
	}
	hash := ""
	for _, v := range m.Files {
		if v.Path == "lineages.json" {
			hash = v.SHA256
		}
	}
	if replayDigest(raw) != hash {
		return m, nil, empty, errors.New("Saved lineage parent checksum changed")
	}
	var d lineageDocument
	if e = json.Unmarshal(raw, &d); e != nil {
		return m, nil, empty, e
	}
	if d.Format != "clonoloom-lineages" || d.Version != bcell.LineageVersion || d.Dataset != m.Dataset {
		return m, nil, empty, errors.New("Unsupported saved lineage parent")
	}
	for _, p := range d.Trees {
		if p.ID == tree {
			return m, raw, p, nil
		}
	}
	return m, nil, empty, errors.New("Select a tree in this saved lineage result")
}
func (a *app) registerLineageLikelihood(mux *http.ServeMux) {
	type kept struct {
		body                                   []byte
		artifact, tree, parentHash, noticeHash string
		restricted                             bool
		expires                                time.Time
	}
	cache := map[string]kept{}
	var mu sync.Mutex
	running := make(chan struct{}, 2)
	mux.HandleFunc("POST /api/lineage-likelihood/{operation}", localOnly(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		op := r.PathValue("operation")
		if op != "review" && op != "export" {
			writeError(w, 404, "Unknown likelihood operation")
			return
		}
		raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 16<<10))
		var req likelihoodRequest
		if e == nil {
			e = airr.CheckExactJSON(r.Context(), raw)
		}
		if e == nil {
			e = strictRunJSON(raw, &req)
		}
		if e != nil {
			writeError(w, 400, "Select one saved lineage artifact and tree")
			return
		}
		m, parent, panel, e := a.likelihoodParent(req.Artifact, req.Tree)
		if e != nil {
			writeError(w, 400, e.Error())
			return
		}
		notice, e := a.likelihoodNotice(m)
		if e != nil {
			writeError(w, 400, e.Error())
			return
		}
		parentKey := replayDigest(parent) + replayDigest(mustJSON(m))
		if op == "export" {
			mu.Lock()
			saved, ok := cache[req.Review]
			mu.Unlock()
			if !ok || time.Now().After(saved.expires) || saved.artifact != req.Artifact || saved.tree != req.Tree || saved.parentHash != parentKey || saved.noticeHash != replayDigest([]byte(notice.Text())) {
				writeError(w, 409, "Likelihood review expired or changed; run the review again")
				return
			}
			if saved.restricted && !req.AcceptTerms {
				writeError(w, 403, "Review provider terms and confirm permission to export")
				return
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Content-Disposition", `attachment; filename="clonoloom-lineage-likelihood.json"`)
			w.Write(saved.body)
			return
		}
		select {
		case running <- struct{}{}:
			defer func() { <-running }()
		default:
			writeError(w, 409, "Two likelihood reviews are already running; wait or cancel one")
			return
		}
		result, e := bcell.Likelihood(r.Context(), panel)
		if e != nil {
			writeError(w, 422, e.Error())
			return
		}
		doc := likelihoodDocument{Format: "clonoloom-lineage-likelihood", Version: bcell.MLVersion, Software: version, Commit: buildCommit(), Parent: m, ParentText: string(parent), ParentSHA256: replayDigest(parent), Panel: panel, Result: result, Citation: likelihoodCitation, Notice: notice.Text(), Restricted: notice.Restricted(), Limits: "Selected-panel likelihood evidence only; no project annotation or historical parsimony artifact is changed. No certificate replay or automation action is added. The parent contains original source identifiers and annotations; this is not a de-identified export. Save before closing. " + bcell.MLDescription}
		body, e := json.MarshalIndent(doc, "", "  ")
		if e != nil || len(body) > 32<<20 {
			writeError(w, 422, "Likelihood evidence exceeds 32 MiB")
			return
		}
		if e := r.Context().Err(); e != nil {
			writeError(w, 400, e.Error())
			return
		}
		key := replayDigest(body)
		mu.Lock()
		for k, v := range cache {
			if time.Now().After(v.expires) {
				delete(cache, k)
			}
		}
		if len(cache) >= 4 {
			oldest := ""
			var at time.Time
			for k, v := range cache {
				if oldest == "" || v.expires.Before(at) {
					oldest, at = k, v.expires
				}
			}
			delete(cache, oldest)
		}
		cache[key] = kept{body, req.Artifact, req.Tree, parentKey, replayDigest([]byte(notice.Text())), doc.Restricted, time.Now().Add(30 * time.Minute)}
		mu.Unlock()
		writeJSON(w, map[string]any{"review": key, "evidence": json.RawMessage(body)})
	}))
}

func (a *app) likelihoodNotice(m artifacts.Manifest) (export.Notice, error) {
	ds, ok := a.datasets.get(m.Dataset)
	if !ok {
		return export.Notice{}, errors.New("Open the saved result's dataset to review provider terms")
	}
	n := a.notice(ds, "distinct panel states", qc.Filter{}, "Saved complete-case V/J panel", "")
	n.Made = m.Created
	n.Method = likelihoodCitation
	n.Derived = true
	n.What = "Selected-panel JC69 likelihood and conditional ancestral probabilities. Original lineage parent and observed source annotations are included."
	return n, nil
}
