package main

// Neighborhoods of a dataset's receptors against a background (internal/neighborhood;
// clonoloom-spec/requirements.md, S2), as an analysis (api_analysis.go): each receptor of one
// locus is a centroid, whose radius is the largest at which the background within it is a small
// enough share of the background's receptors, and whose neighborhood is the dataset's receptors
// within that radius. The background is drawn from a recombination model with the dataset's V–J
// usage (internal/pgen, as api_pgen.go chooses the models) or made by moving the dataset's own
// CDR3s among its genes. The V genes' loops are part of the distance, as they are for TCRdist
// (api_tcrdist.go), unless the definition does not resolve V genes.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"

	"clonoloom/internal/artifacts"
	"clonoloom/internal/clonotype"
	"clonoloom/internal/jobs"
	"clonoloom/internal/neighborhood"
	"clonoloom/internal/tcrdist"
)

var neighborhoodAnalysis = analysis{kind: "neighborhoods", prefix: "nbh-", version: 1, title: "Neighborhoods", over: "neighborhoods",
	work: (*app).neighborhoodWork, prepare: (*app).neighborhoodPrepare}

// backgroundSize is how many receptors a drawn background holds by default (Mayer-Blackwell et
// al.'s), and the most it may hold.
const (
	backgroundSize    = 100000
	maxBackgroundSize = 1000000
	// shownCenters: the centroids a result keeps, the most enriched first; shownMembers, the
	// receptors each neighborhood names.
	shownCenters = 500
	shownMembers = 100
)

// neighborhoodOptions are how neighborhoods are found: the Background ("model", receptors drawn
// from a recombination model with the dataset's V–J usage, the default; "shuffled", the dataset's
// own chains with their CDR3s moved among them), its Size and Seed; the Locus whose chains are
// compared ("" the dataset's commonest); for a model background, which Models and whose Organism,
// as generation probabilities choose them (api_pgen.go), with their Checksum; and the neighborhood
// itself: the Control share of the background a radius may hold, the MaxRadius considered and the
// MinNeighbors a centroid is reported with. Loops names the V genes' loops used.
type neighborhoodOptions struct {
	Background   string  `json:"background,omitempty"`
	Size         int     `json:"size,omitempty"`
	Seed         uint64  `json:"seed,omitempty"`
	Locus        string  `json:"locus,omitempty"`
	Models       string  `json:"models,omitempty"`
	Organism     string  `json:"organism,omitempty"`
	Checksum     string  `json:"checksum,omitempty"`
	Control      float64 `json:"control,omitempty"`
	MaxRadius    int     `json:"maxRadius,omitempty"`
	MinNeighbors int     `json:"minNeighbors,omitempty"`
	CDR3Only     bool    `json:"cdr3Only,omitempty"`
	Loops        string  `json:"loops,omitempty"`
}

// defaultNeighborhoodSeed: a drawn background is the same for the same dataset and options.
const defaultNeighborhoodSeed = 20261027

func (a *app) neighborhoodPrepare(spec *analysisSpec) error {
	o := spec.Neighborhoods
	def := spec.Definition
	if def.Vendor || def.Junction != "aa" {
		return errTCRdistJunction
	}
	switch o.Background {
	case "":
		o.Background = "model"
	case "model", "shuffled":
	default:
		return errors.New("background: model or shuffled")
	}
	if o.Size == 0 {
		o.Size = backgroundSize
	}
	if o.Size < 1000 || o.Size > maxBackgroundSize {
		return fmt.Errorf("the background holds from 1,000 to %d receptors", maxBackgroundSize)
	}
	if o.Seed == 0 {
		o.Seed = defaultNeighborhoodSeed
	}
	d := neighborhood.DefaultOptions()
	if o.Control == 0 {
		o.Control = d.Control
	}
	if o.MaxRadius == 0 {
		o.MaxRadius = d.MaxRadius
	}
	if o.MinNeighbors == 0 {
		o.MinNeighbors = d.MinNeighbors
	}
	if err := (neighborhood.Options{Control: o.Control, MaxRadius: o.MaxRadius, Step: d.Step, MinNeighbors: o.MinNeighbors}).Check(); err != nil {
		return err
	}
	if o.Locus != "" && !clonotype.KnownLocus(o.Locus) {
		return fmt.Errorf("locus: %s is not a locus ClonoLoom knows", o.Locus)
	}
	// The V genes' loops, as TCRdist uses them.
	if def.V == "family" || def.V == "none" {
		o.CDR3Only = true
	}
	if o.CDR3Only {
		o.Organism, o.Loops = "", ""
	} else {
		loops, file, err := a.tcrdistLoops()
		if err != nil {
			return err
		}
		if o.Organism != "" && loops.Alleles(o.Organism) == 0 {
			return fmt.Errorf("organism: the loops' database has %v", loops.Organisms())
		}
		o.Loops = file.SHA256
	}
	// A drawn background's models, as generation probabilities choose them.
	if o.Background == "model" {
		set, err := a.modelsFor(pgenOptions{Models: o.Models, Organism: o.Organism})
		if err != nil {
			return err
		}
		if o.Models == "" {
			if set.Checksum == clonoloomModel {
				o.Models = "clonoloom"
			} else {
				o.Models = "olga"
			}
		}
		o.Checksum = set.Checksum
	} else {
		o.Models, o.Checksum = "", ""
	}
	return nil
}

// neighborhoodDocument is a neighborhoods artifact's neighborhoods.json.
type neighborhoodDocument struct {
	analysisHead
	Samples []string            `json:"samples"`
	Options neighborhoodOptions `json:"neighborhoods"`
	Locus   string              `json:"locus"`
	// Loci: how many of the dataset's clonotypes have a chain of each locus, so that the page can
	// offer the others.
	Loci     map[string]int       `json:"loci"`
	Source   neighborhood.Source  `json:"source"`
	Set      *modelSet            `json:"set,omitempty"`
	Loops    *loopsFile           `json:"loops,omitempty"`
	Organism string               `json:"organism,omitempty"`
	Result   neighborhoodSummary  `json:"result"`
	Rows     []neighborhoodRow    `json:"centers"`
	Members  []neighborhoodMember `json:"-"`
}

// neighborhoodSummary is what was compared: the receptors of the locus, those the loops' database
// has no V gene for, the clonotypes left out and why, the centroids found and those kept.
type neighborhoodSummary struct {
	Receptors int               `json:"receptors"`
	NoLoops   int               `json:"noLoops"`
	Excluded  map[string]int    `json:"excluded"`
	Alleles   map[string]string `json:"alleles,omitempty"`
	Centers   int               `json:"centers"`
	Shown     int               `json:"shown"`
	// Largest: the receptors of the largest neighborhood, and LargestRadius its centroid's radius.
	// A relative rate depends on how large the background is beside the dataset, and a radius is
	// chosen to leave almost no background within it, so neither says by itself which centroids
	// are worth reading: their neighborhoods' size does, and the χ² orders them.
	Largest       int `json:"largest"`
	LargestRadius int `json:"largestRadius"`
}

// neighborhoodRow is a centroid: its clonotype and chain, its radius, its neighborhood's receptors
// and their abundance, the background within the radius, the enrichment, the motif, and the
// neighborhood's receptors (the first shownMembers, nearest first).
type neighborhoodRow struct {
	ID             string               `json:"id"`
	Chain          clonotype.Chain      `json:"chain"`
	Total          float64              `json:"total"`
	Samples        int                  `json:"samples"`
	Radius         int                  `json:"radius"`
	Neighbors      int                  `json:"neighbors"`
	Abundance      float64              `json:"abundance"`
	Background     int                  `json:"background"`
	Weighted       float64              `json:"weighted"`
	Rate           float64              `json:"rate"`
	BackgroundRate float64              `json:"backgroundRate"`
	RelativeRate   float64              `json:"relativeRate"`
	OddsRatio      float64              `json:"oddsRatio"`
	ChiSquare      float64              `json:"chiSquare"`
	Motif          string               `json:"motif"`
	Members        []neighborhoodMember `json:"members"`
}

// neighborhoodMember is a receptor of a neighborhood: its clonotype, chain and distance from the
// centroid.
type neighborhoodMember struct {
	ID       string  `json:"id"`
	Junction string  `json:"junction"`
	V        string  `json:"v,omitempty"`
	J        string  `json:"j,omitempty"`
	Total    float64 `json:"total"`
	Samples  int     `json:"samples"`
	Distance int     `json:"distance"`
}

// locusOfTable counts the clonotypes with a chain of each locus, and names the commonest.
func locusOfTable(table *clonotype.Table) (string, map[string]int) {
	counts := map[string]int{}
	for _, c := range table.Clonotypes {
		for _, ch := range c.Chains {
			if ch.Locus != "" && ch.Junction != "" {
				counts[ch.Locus]++
			}
		}
	}
	best, most := "", 0
	for _, locus := range sortedCounts(counts) {
		if counts[locus] > most {
			best, most = locus, counts[locus]
		}
	}
	return best, counts
}

func sortedCounts(counts map[string]int) []string {
	out := make([]string, 0, len(counts))
	for k := range counts {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// repertoireOf makes the receptors of a clonotype table's chosen locus: each clonotype's chain of
// that locus, with its abundance and samples. A clonotype without one, or without a junction, is
// counted and left out.
func repertoireOf(table *clonotype.Table, locus string) (*neighborhood.Repertoire, map[string]int) {
	out := &neighborhood.Repertoire{Locus: locus}
	excluded := map[string]int{}
	for i, c := range table.Clonotypes {
		found := -1
		for k, ch := range c.Chains {
			if ch.Locus == locus {
				found = k
				break
			}
		}
		if found < 0 {
			excluded["no "+locus+" chain"]++
			continue
		}
		ch := c.Chains[found]
		if ch.Junction == "" {
			excluded["a chain without its junction"]++
			continue
		}
		out.Receptors = append(out.Receptors, neighborhood.Receptor{
			CDR3: ch.Junction, V: ch.V, J: ch.J, Clonotype: i, Count: c.Total, Samples: c.Samples,
		})
	}
	return out, excluded
}

func (a *app) neighborhoodWork(k analysis, spec analysisSpec) jobs.Func {
	return func(ctx context.Context, run *jobs.Run) error {
		ds, head, table, _, err := a.samplesOf(k, spec, run)
		if err != nil {
			return err
		}
		o := *spec.Neighborhoods
		locus, loci := locusOfTable(table)
		if o.Locus != "" {
			locus = o.Locus
		}
		if locus == "" || loci[locus] == 0 {
			return fmt.Errorf("the dataset has no clonotypes with a %s chain", orAny(locus))
		}
		sample, excluded := repertoireOf(table, locus)
		if sample.Len() < 2 {
			return fmt.Errorf("the dataset has %d receptors of %s: neighborhoods need two or more", sample.Len(), locus)
		}
		doc := neighborhoodDocument{analysisHead: head, Samples: sampleNames(ds), Options: o, Locus: locus, Loci: loci}
		// The loops, as TCRdist uses them.
		loops, file, err := a.neighborhoodLoops(o)
		if err != nil {
			return err
		}
		doc.Loops = file
		// The background.
		run.Progress(0.15, "Making the background")
		var background *neighborhood.Repertoire
		var source *neighborhood.Source
		switch o.Background {
		case "shuffled":
			background, source = neighborhood.ShuffledBackground(sample, o.Seed)
		default:
			set, err := a.modelsFor(pgenOptions{Models: o.Models, Organism: o.Organism})
			if err != nil {
				return err
			}
			if set.Checksum != o.Checksum {
				return errors.New("the models changed since the neighborhoods were asked for: ask again")
			}
			organism := o.Organism
			if organism == "" {
				var calls []string
				for _, r := range sample.Receptors {
					calls = append(calls, r.V)
				}
				organism = organismOf(set, calls)
			}
			model := set.models[organism+" "+locus]
			if model == nil {
				return fmt.Errorf("no model of %s %s: choose the dataset's own chains as the background, or another locus", organism, locus)
			}
			doc.Set = set
			background, source, err = neighborhood.ModelBackground(ctx, sample, model, model.Name, set.Checksum, o.Size, o.Seed, func(done, of int) {
				run.Progress(0.15+0.25*float64(done)/float64(max(of, 1)), fmt.Sprintf("Drew %d of %d background receptors", done, of))
			})
			if err != nil {
				return err
			}
		}
		doc.Source = *source
		run.Progress(0.4, fmt.Sprintf("Comparing %d receptors with %d", sample.Len(), background.Len()))
		found, err := neighborhood.Find(ctx, sample, background, loops, o.Organism, neighborhood.Options{
			Control: o.Control, MaxRadius: o.MaxRadius, Step: neighborhood.DefaultOptions().Step, MinNeighbors: o.MinNeighbors,
		}, 0, func(done, of int) {
			run.Progress(0.4+0.55*float64(done)/float64(max(of, 1)), fmt.Sprintf("Found %d of %d neighborhoods", done, of))
		})
		if err != nil {
			return err
		}
		doc.Organism = found.Organism
		doc.Result = neighborhoodSummary{Receptors: found.Receptors, NoLoops: found.NoLoops, Excluded: excluded, Alleles: found.Alleles, Centers: len(found.Centers)}
		member := func(at, distance int) neighborhoodMember {
			r := sample.Receptors[at]
			c := table.Clonotypes[r.Clonotype]
			return neighborhoodMember{ID: c.ID, Junction: r.CDR3, V: r.V, J: r.J, Total: r.Count, Samples: r.Samples, Distance: distance}
		}
		for _, c := range found.Centers {
			if c.Neighbors > doc.Result.Largest {
				doc.Result.Largest, doc.Result.LargestRadius = c.Neighbors, c.Radius
			}
			if len(doc.Rows) >= shownCenters {
				continue
			}
			r := sample.Receptors[c.Receptor]
			cl := table.Clonotypes[r.Clonotype]
			row := neighborhoodRow{
				ID: cl.ID, Chain: clonotype.Chain{Locus: locus, Junction: r.CDR3, V: r.V, J: r.J},
				Total: r.Count, Samples: r.Samples, Radius: c.Radius, Neighbors: c.Neighbors, Abundance: c.Abundance,
				Background: c.Background, Weighted: c.Weighted, Rate: c.Rate, BackgroundRate: c.BackgroundRate,
				RelativeRate: c.RelativeRate, OddsRatio: c.OddsRatio, ChiSquare: c.ChiSquare, Motif: c.Motif,
			}
			for i, m := range c.Members {
				if i >= shownMembers {
					break
				}
				row.Members = append(row.Members, member(m, c.Distances[i]))
			}
			doc.Rows = append(doc.Rows, row)
		}
		doc.Result.Shown = len(doc.Rows)
		return a.keepAnalysis(k, spec, ds, run, doc, len(doc.Samples),
			artifacts.Method{Name: "Neighborhoods of similar receptors against a background, each centroid's radius controlled at a rate of the background (Mayer-Blackwell et al. 2021)",
				Citation: "Mayer-Blackwell K, Schattgen S, Cohen-Lavi L, et al. TCR meta-clonotypes for biomarker discovery with tcrdist3 enabled identification of public, HLA-restricted clusters of SARS-CoV-2 TCRs. eLife. 2021;10:e68605"},
			map[string]any{"receptors": doc.Result.Receptors, "centers": doc.Result.Centers, "largest": doc.Result.Largest,
				"background": o.Background, "backgroundReceptors": source.Receptors, "locus": locus, "definition": head.Definition.Name})
	}
}

// tcrdistSetLoops answers the V genes' loops a neighborhood's options use (nil for the CDR3s
// alone), with the database's record.
func (a *app) neighborhoodLoops(o neighborhoodOptions) (*tcrdist.Loops, *loopsFile, error) {
	if o.CDR3Only {
		return nil, nil, nil
	}
	loops, file, err := a.tcrdistLoops()
	if err != nil {
		return nil, nil, err
	}
	if file.SHA256 != o.Loops {
		return nil, nil, errors.New("the V genes' loops changed since the neighborhoods were asked for: ask again")
	}
	return loops, &file, nil
}

func orAny(locus string) string {
	if locus == "" {
		return "comparable"
	}
	return locus
}

// registerNeighborhoods: POST /api/neighborhoods/loci {dataset, definition, filter} answers the
// loci the dataset's clonotypes have chains of, with how many have each, so that the page can
// offer them before a result is there.
func (a *app) registerNeighborhoods(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/neighborhoods/loci", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			analysisSpec
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "Send {\"dataset\", \"definition\", \"filter\"}.")
			return
		}
		_, entry, status, err := a.clonotypeTable(clonotypeRequest{Dataset: body.Dataset, Definition: body.Definition, Filter: body.Filter})
		if err != nil {
			writeError(w, status, err.Error())
			return
		}
		best, counts := locusOfTable(entry.table)
		writeJSON(w, map[string]any{"locus": best, "loci": counts})
	})
}
