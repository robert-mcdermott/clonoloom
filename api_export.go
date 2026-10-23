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
	"clonoloom/internal/invariant"
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
			ds, entry, status, err := a.clonotypeTable(clonotypeRequest{Species: q.Get("species"), Dataset: q.Get("dataset"), Definition: def, Filter: filter})
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
					notice.What = "The rearrangements of " + ds.Name + " as an AIRR Rearrangement TSV, the filter's rows only, each with its clonotype under the definition (clone_id). Retained AIRR sequence evidence is exported as supplied, including invalid annotations; it is not a reconstructed germline or a mutation measurement. AIRR Rearrangement schema 2.0; Vander Heiden et al. 2018, doi:10.3389/fimmu.2018.02206."
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
					var inv *invariant.Result
					if q.Get("species") != "" {
						inv, _ = invariant.Compute(entry.table, q.Get("species"))
					}
					if err := export.ClonotypesWithInvariant(f, entry.table, names, inv); err != nil {
						return err
					}
					if inv != nil {
						notice.What += " Invariant-population sequence rules v1 for " + inv.Species + ": MAIT/iNKT (Schattgen et al., doi:10.1038/s41587-021-00989-2), GEM (Van Rhijn et al., doi:10.4049/jimmunol.1401380). Flags are not established phenotype or specificity. Each clonotype counts once per rule; shares use all retained abundance, including unevaluable receptors. Missing shares are blank."
						f, err := z.Create("invariant-samples.csv")
						if err != nil {
							return err
						}
						if err := export.InvariantSamples(f, inv, names); err != nil {
							return err
						}
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
	case "tracking":
		var doc trackingDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		intervals := fmt.Sprintf("with their %g%% intervals (Wilson's)", 100*doc.Confidence)
		if !doc.Intervals {
			intervals = "without intervals (" + doc.Caveat + ")"
		}
		notice.What = "The clonotypes followed through each subject's samples of " + ds.Name + ": each one's count, frequency and its interval in each sample (tracking.csv) and at each time point, the samples taken at one time pooled (time-points.csv), " + intervals + "."
		files["tracking.csv"] = func(f io.Writer) error { return export.Tracking(f, doc.Result) }
		files["time-points.csv"] = func(f io.Writer) error { return export.TrackingPoints(f, doc.Result) }
	case "expansion":
		var doc expansionDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		c := doc.Comparison
		correction := map[string]string{"bh": "Benjamini and Hochberg's adjusted p-values", "storey": "Storey's q-values"}[c.Correction]
		test := map[string]string{"betabinomial": "beta-binomial", "binomial": "binomial", "fisher": "Fisher's exact"}[doc.Test]
		fitted := ""
		if doc.Fit != nil {
			fitted = fmt.Sprintf(", its variance v(n) = 2 + (b ln n + a)² fitted on %d clonotypes of %d pairs of samples (a = %.6g, b = %.6g; fit.csv)", doc.Fit.Clonotypes, len(doc.Fit.Pairs), doc.Fit.Model.A, doc.Fit.Model.B)
			files["fit.csv"] = func(f io.Writer) error { return export.ExpansionFit(f, doc.Fit) }
		}
		notice.What = fmt.Sprintf("Each clonotype of %s tested between two time points of each subject (%s and %s; the samples at one time pooled) by the %s test%s, %s, with %s and the calls at %g (tests.csv); each subject's comparison in sum (subjects.csv). %s", ds.Name, c.From, c.To, test, fitted, c.Alternative, correction, c.Alpha, doc.Caveat)
		files["tests.csv"] = func(f io.Writer) error { return export.Expansion(f, doc.Tests) }
		files["subjects.csv"] = func(f io.Writer) error { return export.ExpansionSubjects(f, doc.Tests) }
	case "classes":
		var doc classesDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		notice.What = fmt.Sprintf("The clonotypes of %s classed across time points in each subject by their calls against %s, the base (by the %s test): each classed clonotype at each time point, with its frequency and call (classes.csv), and each subject's clonotypes by class (class-counts.csv). %s", ds.Name, doc.Classing.Base, doc.Test, doc.Caveat)
		files["classes.csv"] = func(f io.Writer) error { return export.Classes(f, doc.ClassResult) }
		files["class-counts.csv"] = func(f io.Writer) error { return export.ClassCounts(f, doc.ClassResult) }
		if doc.Fit != nil {
			files["fit.csv"] = func(f io.Writer) error { return export.ExpansionFit(f, doc.Fit) }
		}
	case "attribution":
		var doc attributionDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		notice.What = fmt.Sprintf("Each sample and time point of %s attributed to its subject's reference sample: its share in the reference's clonotypes and, where it can be told, the share derived from the reference, each with its %g%% interval (Wilson's) (attribution.csv). %s", ds.Name, 100*doc.Confidence, doc.Caveat)
		files["attribution.csv"] = func(f io.Writer) error { return export.Attribution(f, doc.Attribution) }
	case "tcrdist":
		var doc tcrdistDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		compared := "the CDR3s alone"
		if doc.Loops != nil {
			compared = fmt.Sprintf("the CDR3s and the V genes' loops (%s, %s, SHA-256 %s)", doc.Organism, doc.Loops.File, doc.Loops.SHA256)
		}
		notice.What = fmt.Sprintf("Each T-cell clonotype of %s compared by TCRdist with every other of the same chains, comparing %s: its neighbors within %d TCRdist units and its nearest (tcrdist.csv).", ds.Name, compared, doc.Radius)
		rows := make([]export.Neighbor, len(doc.Clonotype))
		for i, r := range doc.Clonotype {
			rows[i] = export.Neighbor{ID: r.ID, Chains: r.Chains, Total: r.Total, Samples: r.Samples, Neighbors: r.Neighbors, Nearest: r.Nearest, Distance: r.Distance}
		}
		files["tcrdist.csv"] = func(f io.Writer) error { return export.TCRdist(f, doc.Radius, rows) }
	case "pgen":
		var doc pgenDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		with := "the CDR3 alone"
		if doc.Genes {
			with = "the CDR3 with its V and J genes"
		}
		notice.What = fmt.Sprintf("Each clonotype of %s's generation probability (pgen.csv: each chain's, %s, under %s, %s; a pair's the product of its chains'), and each sample's distribution of them (samples.csv). Models: %s (checksum %s); %s", ds.Name, with, doc.Set.Name, doc.Organism, doc.Set.Source, doc.Set.Checksum, doc.Set.Citation)
		rows := make([]export.Generated, len(doc.Rows))
		for i, r := range doc.Rows {
			rows[i] = export.Generated{ID: r.ID, Chains: r.Chains, Total: r.Total, Samples: r.Samples, Chain: r.Chain, Pgen: r.Pgen}
		}
		samples := make([]export.PgenSample, len(doc.Samples))
		for i, x := range doc.Samples {
			samples[i] = export.PgenSample{Name: x.Name, With: x.With, Zero: x.Zero, Quartiles: x.Quartiles}
		}
		files["pgen.csv"] = func(f io.Writer) error { return export.Pgen(f, rows) }
		files["samples.csv"] = func(f io.Writer) error { return export.PgenSamples(f, samples) }
	case "specificity":
		var doc specificityDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		notice.What = fmt.Sprintf("Chain specificity evidence for %s. Database %s, SHA-256 %s, source %s, license %s, citation %s. Matches are evidence, not proof of receptor specificity. Epitope shares count each clonotype once per group and can overlap.", ds.Name, doc.Database.Reference, doc.Database.SHA256, doc.Database.Source, doc.Database.License, doc.Database.Citation)
		files["matches.csv"] = func(f io.Writer) error { return export.SpecificityMatches(f, doc.Clonotypes, doc.Result) }
		files["epitopes.csv"] = func(f io.Writer) error { return export.SpecificityShares(f, doc.Samples, doc.Result) }
	case "networks":
		var doc networkDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		notice.What = fmt.Sprintf("Clonotype networks of %s: %s at cutoff %d, %s chains; same V %t, same J %t. clusters.csv assigns every clonotype; edges.csv contains every edge, independent of the drawing limit. %s", ds.Name, doc.Options.Metric, doc.Options.Cutoff, doc.Options.Arms, doc.Options.SameV, doc.Options.SameJ, doc.Describe)
		files["clusters.csv"] = func(f io.Writer) error { return export.NetworkMembership(f, doc.Clonotypes, doc.Graph, doc.Unit) }
		files["edges.csv"] = func(f io.Writer) error { return export.NetworkEdges(f, doc.Clonotypes, doc.Graph) }

	case "neighborhoods":
		var doc neighborhoodDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		against := "the dataset's own chains with their CDR3s moved among them"
		if doc.Source.Kind == "model" {
			against = fmt.Sprintf("%d receptors drawn from %s with the dataset's V–J usage, each weighted by what recombination makes over what the dataset holds", doc.Source.Receptors, doc.Source.Name)
		}
		notice.What = fmt.Sprintf("The neighborhoods of %s's %s receptors against a background of %s (neighborhoods.csv: each centroid's radius, its neighborhood and its enrichment; members.csv: each neighborhood's receptors). A centroid's radius is the largest at which the background within it is at most a share %g of the background's receptors.", ds.Name, doc.Locus, against, doc.Options.Control)
		centers := make([]export.Neighborhood, len(doc.Rows))
		var members []export.NeighborhoodMember
		for i, r := range doc.Rows {
			centers[i] = export.Neighborhood{ID: r.ID, Locus: r.Chain.Locus, Junction: r.Chain.Junction, V: r.Chain.V, J: r.Chain.J,
				Total: r.Total, Samples: r.Samples, Radius: r.Radius, Neighbors: r.Neighbors, Abundance: r.Abundance,
				Background: r.Background, Weighted: r.Weighted, Rate: r.Rate, BackgroundRate: r.BackgroundRate,
				RelativeRate: r.RelativeRate, OddsRatio: r.OddsRatio, ChiSquare: r.ChiSquare, Motif: r.Motif}
			for _, m := range r.Members {
				members = append(members, export.NeighborhoodMember{Centroid: r.ID, ID: m.ID, Junction: m.Junction, V: m.V, J: m.J, Total: m.Total, Samples: m.Samples, Distance: m.Distance})
			}
		}
		files["neighborhoods.csv"] = func(f io.Writer) error { return export.Neighborhoods(f, centers) }
		files["members.csv"] = func(f io.Writer) error { return export.NeighborhoodMembers(f, members) }
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
