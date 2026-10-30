package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"clonoloom/internal/adc"
	"clonoloom/internal/airr"
	"clonoloom/internal/genes"
	"clonoloom/internal/reads"
	"clonoloom/internal/rep"
)

func (a *app) adcOverlapCatalog(w http.ResponseWriter, r *http.Request) {
	entries, err := os.ReadDir(a.store.dir)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	names := []string{}
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "adc-download-") {
			names = append(names, e.Name())
		}
	}
	ds, ok := a.datasets.get(r.URL.Query().Get("dataset"))
	if !ok {
		writeError(w, 400, "Open a local query dataset first.")
		return
	}
	writeJSON(w, map[string]any{"captures": names, "samples": ds.Result.Table.Samples})
}
func (a *app) adcOverlap(w http.ResponseWriter, r *http.Request) {
	if !isLoopbackRequest(r) {
		writeError(w, 403, "Only this computer may compare captured evidence.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), time.Minute)
	defer cancel()
	r = r.WithContext(ctx)
	raw, err := io.ReadAll(io.LimitReader(r.Body, 16385))
	if err != nil || len(raw) > 16384 {
		writeError(w, 400, "Request exceeds 16 KiB.")
		return
	}
	var spec struct {
		Dataset  string   `json:"dataset"`
		Sample   *int     `json:"sample"`
		Locus    string   `json:"locus"`
		Captures []string `json:"captures"`
		Accept   bool     `json:"accept"`
	}
	if err = airr.CheckExactJSON(r.Context(), raw); err == nil {
		err = strictJSON(raw, &spec)
	}
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	if !spec.Accept || len(spec.Captures) < 1 || len(spec.Captures) > 20 {
		writeError(w, 400, "Review the separate presence-matching contract and select 1–20 captures.")
		return
	}
	ds, ok := a.datasets.get(spec.Dataset)
	if !ok {
		writeError(w, 400, "Query dataset is not open.")
		return
	}
	table := ds.Result.Table
	if spec.Sample == nil || *spec.Sample < 0 || *spec.Sample >= len(table.Samples) || len(table.Sample) > 1000000 {
		writeError(w, 400, "Choose one sample from a dataset with at most one million rows.")
		return
	}
	captures := map[string]adc.Capture{}
	remaining := 64 << 20
	for _, name := range spec.Captures {
		if filepath.Base(name) != name || !strings.HasPrefix(name, "adc-download-") || strings.ContainsAny(name, "/\\") {
			writeError(w, 400, "Choose a capture in this library.")
			return
		}
		if _, exists := captures[name]; exists {
			writeError(w, 400, "Repeated capture.")
			return
		}
		path := filepath.Join(a.store.dir, name)
		st, e := os.Lstat(path)
		if e != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
			writeError(w, 400, "Capture is not a local directory.")
			return
		}
		c, e := adc.ReadCapture(r.Context(), path, remaining)
		if e != nil {
			writeError(w, 400, name+": "+e.Error())
			return
		}
		remaining -= c.Bytes
		captures[name] = c
	}
	rows := []adc.Sequence{}
	for i, sample := range table.Sample {
		if i%1024 == 0 {
			if err := ctx.Err(); err != nil {
				writeError(w, 400, err.Error())
				return
			}
		}
		if int(sample) != *spec.Sample {
			continue
		}
		locus := ""
		if table.Locus[i] > 0 && int(table.Locus[i]) <= len(genes.Loci) {
			locus = genes.Loci[table.Locus[i]-1]
		}
		rows = append(rows, adc.Sequence{Locus: locus, Junction: table.JunctionsAA.Get(table.JunctionAA[i]), Productive: table.Productive[i] == rep.True})
	}
	result, err := adc.CompareCaptures(r.Context(), rows, spec.Locus, captures)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	unit := string(table.Unit)
	if unit == "" {
		unit = "unknown"
	}
	sources := []map[string]any{}
	for _, src := range ds.Result.Sources {
		sources = append(sources, map[string]any{"sha256": src.SHA256, "bytes": src.Size, "format": src.Format, "mapping": src.Mapping})
	}
	metadata := []map[string]any{}
	for _, src := range ds.Result.Metadata {
		metadata = append(metadata, map[string]any{"sha256": src.SHA256, "bytes": src.Size})
	}
	doc := map[string]any{
		"version": 1, "queryDataset": ds.ID, "querySample": table.Samples[*spec.Sample].Name,
		"querySources": sources, "queryMetadata": metadata, "readerIdentity": reads.IdentityVersion,
		"querySampleIndex": *spec.Sample, "querySourceUnit": unit,
		"publicSourceUnit": "not inferred; presence only", "result": result,
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	if err = r.Context().Err(); err != nil {
		return
	}
	f, err := os.CreateTemp(a.store.dir, "adc-overlap-*.json")
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	_, err = f.Write(append(data, '\n'))
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(f.Name())
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]any{"document": doc, "path": f.Name()})
}
