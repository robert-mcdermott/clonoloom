package main

// Exports (internal/export; clonoloom-spec/requirements.md, O3 and O9) over HTTP: a dataset's
// rearrangements (AIRR TSV) or clonotypes (CSV) under a definition and filter, or a result kept
// (CSV), each in a zip with its NOTICE. Rows of public data under a provider's terms (or no stated
// license) are written only when the request says it has read them (accept=1): otherwise the
// answer is 409 with the terms, which the page shows before asking again.

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"clonoloom/internal/artifacts"
	"clonoloom/internal/clonotype"
	"clonoloom/internal/diversity"
	"clonoloom/internal/export"
	"clonoloom/internal/overlap"
	"clonoloom/internal/qc"
)

func (a *app) registerExports(mux *http.ServeMux) {
	// GET /api/export?what=rearrangements|clonotypes&dataset=&definition=<JSON>&filter=<JSON>[&accept=1]
	// GET /api/export?what=result&artifact=<id>[&accept=1]
	mux.HandleFunc("GET /api/export", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		accept := q.Get("accept") == "1"
		switch what := q.Get("what"); what {
		case "rearrangements", "clonotypes":
			var def clonotype.Definition
			var filter qc.Filter
			if err := json.Unmarshal([]byte(q.Get("definition")), &def); err != nil {
				writeError(w, http.StatusBadRequest, "Send the clonotype definition (JSON).")
				return
			}
			if f := q.Get("filter"); f != "" {
				if err := json.Unmarshal([]byte(f), &filter); err != nil {
					writeError(w, http.StatusBadRequest, "The filter is not JSON ClonoLoom reads.")
					return
				}
			}
			ds, entry, status, err := a.clonotypeTable(clonotypeRequest{Dataset: q.Get("dataset"), Definition: def, Filter: filter})
			if err != nil {
				writeErrorCode(w, status, "not-open", err.Error())
				return
			}
			norm, _ := def.Normalize()
			notice := a.notice(ds, string(entry.table.Unit), filter, norm.Describe(), entry.table.DefinitionID)
			if notice.Restricted() && !accept {
				restricted(w, notice)
				return
			}
			names := sampleNames(ds)
			a.writeZip(w, ds.Name+"-"+what, func(z *zip.Writer) error {
				if what == "rearrangements" {
					notice.What = "The rearrangements of " + ds.Name + " as an AIRR Rearrangement TSV, the filter's rows only, each with its clonotype under the definition (clone_id)."
					keep, _ := qc.Apply(ds.Result.Table, filter)
					f, err := z.Create("rearrangements.tsv")
					if err != nil {
						return err
					}
					if err := export.Rearrangements(f, ds.Result.Table, keep, entry.table); err != nil {
						return err
					}
				} else {
					notice.What = fmt.Sprintf("The clonotypes of %s as CSV: each clonotype's chains, its total abundance and samples, and its abundance (in %s) in each sample.", ds.Name, notice.Unit)
					f, err := z.Create("clonotypes.csv")
					if err != nil {
						return err
					}
					if err := export.Clonotypes(f, entry.table, names); err != nil {
						return err
					}
				}
				return writeNotice(z, notice)
			})
		case "result":
			a.exportResult(w, q.Get("artifact"), accept)
		default:
			writeError(w, http.StatusBadRequest, "Export what: rearrangements, clonotypes or result.")
		}
	})
}

// exportResult writes a kept result as CSV.
func (a *app) exportResult(w http.ResponseWriter, id string, accept bool) {
	m, err := a.artifacts.Get(id)
	if err != nil || m.State != artifacts.Complete {
		writeError(w, http.StatusNotFound, "No such result.")
		return
	}
	path, err := a.artifacts.Path(id, m.Kind+".json")
	if err != nil {
		writeError(w, http.StatusNotFound, "This result has nothing to export.")
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var head analysisHead
	json.Unmarshal(raw, &head)
	ds, ok := a.datasets.get(head.Dataset)
	if !ok {
		writeErrorCode(w, http.StatusNotFound, "not-open", "The result's dataset is not open: open it again from its files.")
		return
	}
	notice := a.notice(ds, head.Unit, head.Filter, head.Describe, head.DefinitionID)
	notice.Method = fmt.Sprintf("%s (method version %s; %s), kept as %s on %s", m.Lineage.Method.Name, m.Lineage.Method.Version, m.Lineage.Method.Citation, m.ID, m.Created)
	notice.Derived = true
	if notice.Restricted() && !accept {
		restricted(w, notice)
		return
	}
	files := map[string]func(io.Writer) error{}
	switch m.Kind {
	case "diversity":
		var doc diversityDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		var names []string
		var results []diversity.Result
		for _, s := range doc.Samples {
			names, results = append(names, s.Name), append(results, s.Result)
		}
		notice.What = "Each sample's diversity measures of " + ds.Name + ", with their 95% intervals."
		files["diversity.csv"] = func(f io.Writer) error { return export.Diversity(f, names, results) }
	case "rarefaction":
		var doc rarefactionDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		var names []string
		var curves []diversity.Curves
		for _, s := range doc.Samples {
			names, curves = append(names, s.Name), append(curves, s.Curves)
		}
		notice.What = "Each sample's rarefaction and extrapolation curves of " + ds.Name + ", by size and by coverage, and at the levels the samples are compared at, with their 95% intervals."
		files["rarefaction.csv"] = func(f io.Writer) error { return export.Rarefaction(f, names, curves) }
	case "usage":
		var doc usageDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		notice.What = "Each sample's and condition's gene usage, junction lengths and amino-acid composition of " + ds.Name + ", by clonotypes and by abundance, with their intervals (a condition's: its samples' range)."
		files["usage.csv"] = func(f io.Writer) error { return export.Usage(f, doc.Samples, doc.Groups) }
	case "overlap":
		var doc overlapDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		res := overlap.Result{Samples: doc.Samples, Depth: doc.Depth, Pairs: doc.Pairs, Summaries: doc.Summaries, Public: doc.Public, Replicates: doc.Replicates}
		notice.What = "Every pair of samples' overlap in " + ds.Name + ", with their 95% intervals (overlap.csv), and the public clonotypes (public.csv)."
		files["overlap.csv"] = func(f io.Writer) error { return export.Overlap(f, res) }
		files["public.csv"] = func(f io.Writer) error { return export.Public(f, res) }
	default:
		writeError(w, http.StatusBadRequest, "This result has nothing to export as a table.")
		return
	}
	a.writeZip(w, ds.Name+"-"+m.Kind, func(z *zip.Writer) error {
		for _, name := range sortedKeys(files) {
			f, err := z.Create(name)
			if err != nil {
				return err
			}
			if err := files[name](f); err != nil {
				return err
			}
		}
		return writeNotice(z, notice)
	})
}

// notice is what an export of a dataset says of where it came from.
func (a *app) notice(ds *dataset, unit string, filter qc.Filter, definition, definitionID string) export.Notice {
	n := export.Notice{Dataset: ds.Name, DatasetID: ds.ID, Samples: len(ds.Result.Table.Samples), Unit: unitWords(unit), Filter: filter.Describe(),
		Definition: definition, DefinitionID: definitionID, Version: version, Commit: buildCommit(), Made: time.Now()}
	for _, s := range ds.Result.Sources {
		n.Files = append(n.Files, export.File{Name: s.Name, SHA256: s.SHA256})
	}
	for _, c := range ds.Credits {
		n.Credits = append(n.Credits, export.Credit{Title: c.Title, Source: c.Source, License: c.License, LicenseURL: c.LicenseURL, LicenseClass: c.LicenseClass, Citation: c.Citation})
	}
	return n
}

func unitWords(unit string) string {
	switch unit {
	case "umis":
		return "UMIs"
	case "":
		return "counts"
	}
	return unit
}

func sampleNames(ds *dataset) []string {
	var out []string
	for _, s := range ds.Result.Table.Samples {
		out = append(out, s.Name)
	}
	return out
}

// restricted answers 409 with the terms the data are under.
func restricted(w http.ResponseWriter, n export.Notice) {
	var licenses []string
	for _, c := range n.Credits {
		if c.LicenseClass != "open" {
			licenses = append(licenses, fmt.Sprintf("%s (%s)", c.Title, c.License))
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusConflict)
	json.NewEncoder(w).Encode(map[string]any{"code": "restricted", "licenses": licenses,
		"error": "These data are public data under a provider's terms of use, or under no stated license: " + strings.Join(licenses, "; ") + ". Export them only for yourself and your study, and do not share them."})
}

var unsafeName = regexp.MustCompile(`[^\w.-]+`)

// writeZip streams a zip download.
func (a *app) writeZip(w http.ResponseWriter, name string, fill func(*zip.Writer) error) {
	name = strings.Trim(unsafeName.ReplaceAllString(name, "_"), "_")
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.zip"`, name))
	z := zip.NewWriter(w)
	if err := fill(z); err != nil {
		// The download is cut short: its zip will not open.
		return
	}
	z.Close()
}

func writeNotice(z *zip.Writer, n export.Notice) error {
	f, err := z.Create("NOTICE.txt")
	if err != nil {
		return err
	}
	_, err = io.WriteString(f, n.Text())
	return err
}

func sortedKeys(m map[string]func(io.Writer) error) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
