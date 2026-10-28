package main

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"sort"
	"time"

	"clonoloom/internal/clonotype"
	"clonoloom/internal/export"
	"clonoloom/internal/invariant"
	"clonoloom/internal/qc"
)

type parquetRequest struct {
	Dataset    string               `json:"dataset"`
	What       string               `json:"what"`
	Definition clonotype.Definition `json:"definition"`
	Filter     qc.Filter            `json:"filter"`
	Species    string               `json:"species"`
	Accept     bool                 `json:"accept"`
}

func (a *app) registerParquet(mux *http.ServeMux) {
	gate := make(chan struct{}, 1)
	mux.HandleFunc("POST /api/parquet/export", func(w http.ResponseWriter, r *http.Request) {
		select {
		case gate <- struct{}{}:
			defer func() { <-gate }()
		default:
			writeError(w, 409, "A Parquet export is already being prepared.")
			return
		}
		raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		var req parquetRequest
		if e == nil {
			e = strictRunJSON(raw, &req)
		}
		if e != nil {
			writeError(w, 400, "Send bounded Parquet export settings.")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
		defer cancel()
		bundle, receipt, notice, e := a.prepareParquet(ctx, req)
		if e != nil {
			if notice.Restricted() && !req.Accept {
				restricted(w, notice)
			} else {
				writeError(w, 409, e.Error())
			}
			return
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Disposition", `attachment; filename="clonoloom-`+req.What+`-parquet.zip"`)
		w.Header().Set("X-ClonoLoom-Parquet-Receipt", string(mustJSON(receipt)))
		http.ServeContent(w, r, "export.zip", time.Time{}, bytes.NewReader(bundle))
	})
}

type parquetReceipt struct {
	Format      string      `json:"format"`
	What        string      `json:"what"`
	Rows        int         `json:"rows"`
	Unit        string      `json:"unit"`
	Files       []runOutput `json:"files"`
	Fingerprint string      `json:"fingerprint"`
}

func (a *app) prepareParquet(ctx context.Context, req parquetRequest) ([]byte, parquetReceipt, export.Notice, error) {
	var notice export.Notice
	var receipt parquetReceipt
	fail := func(e error) ([]byte, parquetReceipt, export.Notice, error) { return nil, receipt, notice, e }
	if req.What != "clonotypes" && req.What != "rearrangements" {
		return fail(errors.New("Parquet supports native clonotypes or rearrangements, not arbitrary saved results"))
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	ds, entry, _, e := a.clonotypeTable(clonotypeRequest{Dataset: req.Dataset, Definition: req.Definition, Filter: req.Filter, Species: req.Species})
	if e != nil {
		return fail(e)
	}
	def, _ := req.Definition.Normalize()
	notice = a.notice(ds, string(entry.table.Unit), req.Filter, def.Describe(), entry.table.DefinitionID)
	if notice.Restricted() && !req.Accept {
		return fail(errors.New("provider acknowledgment required"))
	}
	notice.What = "Native " + req.What + " represented as typed Parquet. Original identifiers and source provenance are retained: this export is NOT de-identified. " + parquetLimits
	notice.Method = "ClonoLoom Parquet table schema 1; Apache Parquet v1 data pages. AIRR native evidence: Vander Heiden et al. (2018), doi:10.3389/fimmu.2018.02206."
	if req.Species != "" && req.What == "clonotypes" {
		notice.Method += " Invariant sequence rules v1 (not established phenotype/specificity): Schattgen et al., doi:10.1038/s41587-021-00989-2; Van Rhijn et al., doi:10.4049/jimmunol.1401380. Invariant shares use all retained abundance; populations can overlap."
	}
	provenance := mustJSON(map[string]any{"software": map[string]string{"name": "clonoloom", "version": version, "commit": buildCommit()}, "dataset": ds.ID, "name": ds.Name, "definition": def, "definitionId": entry.table.DefinitionID, "filter": req.Filter, "unit": string(entry.table.Unit), "rearrangementUnit": string(ds.Result.Table.Unit), "sources": ds.Result.Sources, "metadata": ds.Result.Metadata, "credits": notice.Credits, "restricted": notice.Restricted(), "deidentified": false})
	if e := checkParquetCounts(ctx, ds.Result.Table, entry.table, req.Filter, req.What == "rearrangements"); e != nil {
		return fail(e)
	}
	names := sampleNames(ds)
	if len(names) > 1000 {
		return fail(errors.New("at most 1000 samples in a Parquet export"))
	}
	native := map[string]*tableBuffer{req.What: {ctx: ctx}}
	if req.What == "rearrangements" {
		keep, _ := qc.Apply(ds.Result.Table, req.Filter)
		e = export.Rearrangements(native[req.What], ds.Result.Table, keep, entry.table)
	} else {
		var inv *invariant.Result
		if req.Species != "" {
			inv, e = invariant.Compute(entry.table, req.Species)
			if e != nil {
				return fail(e)
			}
		}
		e = export.ClonotypesWithInvariant(native[req.What], entry.table, names, inv)
		if e == nil && inv != nil {
			native["invariant-samples"] = &tableBuffer{ctx: ctx}
			e = export.InvariantSamples(native["invariant-samples"], inv, names)
		}
	}
	if e != nil {
		return fail(e)
	}
	files := map[string][]byte{"NOTICE.txt": []byte(notice.Text())}
	schemas := map[string]parquetSchema{}
	keys := []string{}
	for key := range native {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		data, schema, err := nativeParquet(ctx, native[key].Bytes(), key, names, provenance, notice.Text())
		if err != nil {
			return fail(err)
		}
		files[key+".parquet"] = data
		schemas[key] = schema
	}
	files["schema.json"] = mustJSON(schemas)
	receipt = parquetReceipt{Format: "clonoloom-parquet-export", What: req.What, Rows: schemas[req.What].Rows, Unit: string(entry.table.Unit)}
	keys = nil
	for name, data := range files {
		keys = append(keys, name)
		receipt.Files = append(receipt.Files, runOutput{name, int64(len(data)), replayDigest(data)})
	}
	sort.Strings(keys)
	sort.Slice(receipt.Files, func(i, j int) bool { return receipt.Files[i].Path < receipt.Files[j].Path })
	receipt.Fingerprint = replayDigest(mustJSON(receipt.Files))
	files["inventory.json"] = mustJSON(receipt)
	keys = append(keys, "inventory.json")
	out := &tableBuffer{ctx: ctx}
	z := zip.NewWriter(out)
	for _, key := range keys {
		f, err := z.CreateHeader(&zip.FileHeader{Name: key, Method: zip.Store})
		if err != nil {
			return fail(err)
		}
		if _, err = f.Write(files[key]); err != nil {
			return fail(err)
		}
	}
	if e = z.Close(); e != nil {
		return fail(e)
	}
	return out.Bytes(), receipt, notice, ctx.Err()
}
