package main

// TCRdist between a dataset's clonotypes (internal/tcrdist; clonoloom-spec/requirements.md, S1),
// as an analysis (api_analysis.go): each T-cell clonotype's neighbors within a radius among the
// dataset's clonotypes of the same chains, and its nearest other one. The V genes' loops come from
// the reference data tcrdist3-loops (api_references.go), whose checksum is part of the result's
// id; without them, or for a definition that does not resolve V genes, the CDR3s are compared
// alone, and the result says so. A clonotype's nearest clonotypes, with their distances loop by
// loop, are answered on request (POST /api/tcrdist/nearest).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"

	"clonoloom/internal/artifacts"
	"clonoloom/internal/clonotype"
	"clonoloom/internal/jobs"
	"clonoloom/internal/tcrdist"
)

var tcrdistAnalysis = analysis{kind: "tcrdist", prefix: "tcd-", version: 1, title: "TCRdist neighbors", over: "tcrdist",
	work: (*app).tcrdistWork, prepare: (*app).tcrdistPrepare}

// tcrdistOptions are how the clonotypes are compared: the radius within which two are neighbors
// (TCRdist units; 24 a chain when not given), the CDR3s alone or with the V genes' loops, the
// organism whose loops are used ("" guessed from the V genes), and the loops' database's SHA-256
// (set from the reference data, not by the page).
type tcrdistOptions struct {
	Radius   int    `json:"radius"`
	CDR3Only bool   `json:"cdr3Only,omitempty"`
	Organism string `json:"organism,omitempty"`
	Loops    string `json:"loops,omitempty"`
}

// errTCRdistJunction: TCRdist compares amino acids.
var errTCRdistJunction = errors.New("TCRdist compares the junctions' amino acids: choose a definition by amino-acid junction")

// tcrdistPrepare completes the options: the radius's default, the CDR3s alone for a definition
// whose V genes give no loops (by family, or none), and the loops' database's checksum.
func (a *app) tcrdistPrepare(spec *analysisSpec) error {
	o := spec.TCRdist
	def := spec.Definition
	if def.Vendor || def.Junction != "aa" {
		return errTCRdistJunction
	}
	if o.Radius == 0 {
		o.Radius = 24 * max(1, len(def.Chains))
	}
	if o.Radius < 0 || o.Radius > 1000 {
		return errors.New("radius: from 1 to 1000 TCRdist units")
	}
	if def.V == "family" || def.V == "none" {
		o.CDR3Only = true
	}
	if o.CDR3Only {
		o.Organism, o.Loops = "", ""
		return nil
	}
	loops, file, err := a.tcrdistLoops()
	if err != nil {
		return err
	}
	if o.Organism != "" && loops.Alleles(o.Organism) == 0 {
		return fmt.Errorf("organism: the loops' database has %v", loops.Organisms())
	}
	o.Loops = file.SHA256
	return nil
}

// prepareCode is a refused spec's error code: "no-loops" when the V genes' loops are not
// downloaded (the page offers them).
func prepareCode(err error) string {
	if errors.Is(err, errNoLoops) {
		return "no-loops"
	}
	return ""
}

// tcrdistDocument is a TCRdist artifact's tcrdist.json: the samples (every one of the dataset's),
// how the clonotypes were compared, and each compared clonotype's neighborhood.
type tcrdistDocument struct {
	analysisHead
	Samples  []string `json:"samples"`
	Radius   int      `json:"radius"`
	CDR3Only bool     `json:"cdr3Only"`
	// Why the CDR3s were compared alone, when the user did not ask.
	Why       string            `json:"why,omitempty"`
	Organism  string            `json:"organism,omitempty"`
	Loops     *loopsFile        `json:"loops,omitempty"`
	Groups    []tcrdist.Group   `json:"groups"`
	Excluded  map[string]int    `json:"excluded"`
	Alleles   map[string]string `json:"alleles,omitempty"`
	Compared  int               `json:"compared"`
	WithAny   int               `json:"withNeighbors"`
	Clonotype []tcrdistRow      `json:"clonotypes"`
}

// tcrdistRow is a compared clonotype: its chains, abundance and samples, its neighbors within the
// radius, and its nearest other clonotype of the same chains (none when it is the only one).
type tcrdistRow struct {
	ID        string            `json:"id"`
	Chains    []clonotype.Chain `json:"chains"`
	Total     float64           `json:"total"`
	Samples   int               `json:"samples"`
	Neighbors int               `json:"neighbors"`
	Nearest   string            `json:"nearest,omitempty"`
	Distance  int               `json:"distance"`
}

func (a *app) tcrdistWork(k analysis, spec analysisSpec) jobs.Func {
	return func(ctx context.Context, run *jobs.Run) error {
		ds, head, table, _, err := a.samplesOf(k, spec, run)
		if err != nil {
			return err
		}
		o := *spec.TCRdist
		set, file, err := a.tcrdistSet(table, o)
		if err != nil {
			return err
		}
		run.Progress(0.2, fmt.Sprintf("Comparing %d receptors", set.Len()))
		hoods, err := set.Neighbors(ctx, o.Radius, 0, func(done, of int) {
			run.Progress(0.2+0.75*float64(done)/float64(max(of, 1)), fmt.Sprintf("Compared %d of %d receptors with every other", done, of))
		})
		if err != nil {
			return err
		}
		doc := tcrdistDocument{analysisHead: head, Samples: sampleNames(ds), Radius: o.Radius, CDR3Only: o.CDR3Only, Organism: set.Organism,
			Loops: file, Groups: set.Groups, Excluded: set.Excluded, Alleles: set.Alleles, Compared: set.Len(), Clonotype: make([]tcrdistRow, 0, set.Len())}
		if o.CDR3Only && (spec.Definition.V == "family" || spec.Definition.V == "none") {
			doc.Why = "the definition does not resolve V genes, whose loops are looked up by gene"
		}
		for r, h := range hoods {
			c := table.Clonotypes[set.Clonotype[r]]
			row := tcrdistRow{ID: c.ID, Chains: c.Chains, Total: c.Total, Samples: c.Samples, Neighbors: h.Neighbors, Distance: h.Distance}
			if h.Nearest >= 0 {
				row.Nearest = table.Clonotypes[set.Clonotype[h.Nearest]].ID
			}
			if h.Neighbors > 0 {
				doc.WithAny++
			}
			doc.Clonotype = append(doc.Clonotype, row)
		}
		return a.keepAnalysis(k, spec, ds, run, doc, len(doc.Samples),
			artifacts.Method{Name: "TCRdist between clonotypes (Dash et al. 2017) as tcrdist3 computes it, each clonotype's neighbors within a radius",
				Citation: "Dash P, et al. Quantifiable predictive features define epitope-specific T cell receptor repertoires. Nature. 2017;547:89–93; Mayer-Blackwell K, et al. eLife. 2021;10:e68605"},
			map[string]any{"compared": doc.Compared, "withNeighbors": doc.WithAny, "radius": o.Radius, "cdr3Only": o.CDR3Only, "organism": set.Organism, "definition": head.Definition.Name})
	}
}

// tcrdistSet makes the receptors of a clonotype table as the options say, with the loops'
// database's record when its loops are used.
func (a *app) tcrdistSet(table *clonotype.Table, o tcrdistOptions) (*tcrdist.Set, *loopsFile, error) {
	if o.CDR3Only {
		return tcrdist.NewSet(table.Clonotypes, nil, ""), nil, nil
	}
	loops, file, err := a.tcrdistLoops()
	if err != nil {
		return nil, nil, err
	}
	if file.SHA256 != o.Loops {
		return nil, nil, errors.New("the V genes' loops changed since the comparison was asked for: ask again")
	}
	return tcrdist.NewSet(table.Clonotypes, loops, o.Organism), &file, nil
}

// tcrdistSets keeps the receptors last made for a nearest-clonotypes request, by the analysis's
// id, so that clicking through a table's clonotypes does not make them again.
type tcrdistSets struct {
	mu    sync.Mutex
	id    string
	table *clonotype.Table
	set   *tcrdist.Set
	index map[string]int
}

var nearestSets tcrdistSets

// registerTCRdist: POST /api/tcrdist/nearest {dataset, definition, filter, tcrdist, clonotype, n}
// answers the clonotype's n nearest clonotypes of the same chains (at most 100), each with its
// distance chain by chain and loop by loop.
func (a *app) registerTCRdist(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/tcrdist/nearest", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			analysisSpec
			Clonotype string `json:"clonotype"`
			N         int    `json:"n"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil || body.Clonotype == "" {
			writeError(w, http.StatusBadRequest, "Send {\"dataset\", \"definition\", \"filter\", \"tcrdist\", \"clonotype\", \"n\"}.")
			return
		}
		if body.TCRdist == nil {
			body.TCRdist = &tcrdistOptions{}
		}
		body.Study, body.Track, body.Compare, body.Class, body.Groups, body.Subjects = nil, nil, nil, nil, nil, nil
		spec, err := body.analysisSpec.normalized(tcrdistAnalysis.version)
		if err == nil {
			err = a.tcrdistPrepare(&spec)
		}
		if err != nil {
			writeErrorCode(w, http.StatusBadRequest, prepareCode(err), err.Error())
			return
		}
		n := min(max(body.N, 1), 100)
		id := spec.artifactID(tcrdistAnalysis.prefix)
		nearestSets.mu.Lock()
		defer nearestSets.mu.Unlock()
		if nearestSets.id != id {
			_, entry, status, err := a.clonotypeTable(clonotypeRequest{Dataset: spec.Dataset, Definition: spec.Definition, Filter: spec.Filter})
			if err != nil {
				writeError(w, status, err.Error())
				return
			}
			set, _, err := a.tcrdistSet(entry.table, *spec.TCRdist)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			index := make(map[string]int, set.Len())
			for r, c := range set.Clonotype {
				index[entry.table.Clonotypes[c].ID] = r
			}
			nearestSets.id, nearestSets.table, nearestSets.set, nearestSets.index = id, entry.table, set, index
		}
		set, table := nearestSets.set, nearestSets.table
		receptor, ok := nearestSets.index[body.Clonotype]
		if !ok {
			writeError(w, http.StatusNotFound, "This clonotype is not compared by TCRdist (a chain of another locus, without its junction, or with a V gene the loops' database lacks).")
			return
		}
		type near struct {
			ID       string            `json:"id"`
			Chains   []clonotype.Chain `json:"chains"`
			Total    float64           `json:"total"`
			Distance int               `json:"distance"`
			Regions  [][4]int          `json:"regions"`
		}
		var out []near
		for _, p := range set.Nearest(receptor, n) {
			c := table.Clonotypes[set.Clonotype[p.Receptor]]
			out = append(out, near{ID: c.ID, Chains: c.Chains, Total: c.Total, Distance: p.Distance, Regions: set.Regions(receptor, p.Receptor)})
		}
		writeJSON(w, map[string]any{"clonotype": body.Clonotype, "radius": spec.TCRdist.Radius, "cdr3Only": spec.TCRdist.CDR3Only, "organism": set.Organism, "nearest": out})
	})
}
