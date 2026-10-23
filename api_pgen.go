package main

// Generation probabilities of a dataset's clonotypes (internal/pgen; clonoloom-spec/
// requirements.md, R6), as an analysis (api_analysis.go): each chain's CDR3 (or, with its genes,
// the CDR3 with its V and J genes) under the recombination model of its locus, and a pair's the
// product of its chains'; each sample's distribution. The models are OLGA's default models (the
// reference data olga-models, api_references.go), where downloaded, or ClonoLoom's own model of
// human TRB, built in; their checksums are part of the result's id. A clonotype's chains are
// answered on request too (POST /api/pgen/chains), for the Clonotypes view's inspector.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"clonoloom/internal/artifacts"
	"clonoloom/internal/clonotype"
	"clonoloom/internal/dmath"
	"clonoloom/internal/examples"
	"clonoloom/internal/jobs"
	"clonoloom/internal/pgen"
)

var pgenAnalysis = analysis{kind: "pgen", prefix: "pgn-", version: 1, title: "Generation probabilities", over: "pgen",
	work: (*app).pgenWork, prepare: (*app).pgenPrepare}

// pgenOptions are which models compute the probabilities ("": OLGA's where downloaded, else
// ClonoLoom's; "olga"; "clonoloom"), whose organism's ("" guessed from the V genes), and whether
// each chain's V and J genes are part of it (the joint probability of the CDR3 and its genes);
// Checksum names the models (set from them, not by the page).
type pgenOptions struct {
	Models   string `json:"models,omitempty"`
	Organism string `json:"organism,omitempty"`
	Genes    bool   `json:"genes,omitempty"`
	Checksum string `json:"checksum,omitempty"`
}

// olgaReference names the reference data of OLGA's models.
const olgaReference = "olga-models"

// clonoloomModel is ClonoLoom's own model's checksum: its version, which changes when the
// simulator's recombination does.
const clonoloomModel = "clonoloom_human_T_beta 1"

// modelSet is the models a result is computed with: OLGA's, by organism and locus, or
// ClonoLoom's; with their record.
type modelSet struct {
	Name     string `json:"name"`
	Checksum string `json:"checksum"`
	Source   string `json:"source"`
	License  string `json:"license,omitempty"`
	Citation string `json:"citation"`
	models   map[string]*pgen.Model
}

// olgaDirs: OLGA's model folders by organism and locus.
var olgaDirs = map[string]string{
	"human TRB": "human_T_beta", "human TRA": "human_T_alpha", "human IGH": "human_B_heavy", "human IGK": "human_B_kappa",
	"human IGL": "human_B_lambda", "mouse TRB": "mouse_T_beta", "mouse TRA": "mouse_T_alpha",
}

var (
	ownModelOnce sync.Once
	ownModel     *pgen.Model
	olgaCache    struct {
		mu       sync.Mutex
		checksum string
		set      *modelSet
	}
)

// clonoloomModels is ClonoLoom's own model: human TRB.
func clonoloomModels() *modelSet {
	ownModelOnce.Do(func() { ownModel = pgen.Simulator() })
	return &modelSet{Name: "ClonoLoom's model of human TRB recombination (the simulator's)", Checksum: clonoloomModel,
		Source:   "ClonoLoom's simulator: GRCh38's TRB gene segments at Ensembl, V and J usage of 10x Genomics' 10k PBMC data",
		Citation: "McDermott R. ClonoLoom. https://github.com/robert-mcdermott/clonoloom", models: map[string]*pgen.Model{"human TRB": ownModel}}
}

// errNoModels: OLGA's models asked for and not downloaded.
var errNoModels = errors.New("OLGA's recombination models are not downloaded: download them (Reference data), or use ClonoLoom's own model of human TRB")

// olgaModels reads OLGA's downloaded models (once, while their files are unchanged), each checked
// against the checksum ClonoLoom records; the set's checksum is that of every file's.
func (a *app) olgaModels() (*modelSet, error) {
	var ref *examples.State
	for _, s := range a.references.List() {
		if s.Name == olgaReference && s.State == "ready" {
			s := s
			ref = &s
		}
	}
	if ref == nil {
		return nil, errNoModels
	}
	h := sha256.New()
	for _, f := range ref.Files {
		h.Write([]byte(f.Path + " " + f.SHA256 + "\n"))
	}
	checksum := hex.EncodeToString(h.Sum(nil))
	olgaCache.mu.Lock()
	defer olgaCache.mu.Unlock()
	if olgaCache.set != nil && olgaCache.checksum == checksum {
		return olgaCache.set, nil
	}
	for _, f := range ref.Files {
		raw, err := os.ReadFile(filepath.Join(ref.Path, filepath.FromSlash(f.Path)))
		if err != nil {
			return nil, err
		}
		if sum := sha256.Sum256(raw); hex.EncodeToString(sum[:]) != f.SHA256 {
			return nil, fmt.Errorf("%s is not the file ClonoLoom records: download OLGA's models again", f.Path)
		}
	}
	set := &modelSet{Name: "OLGA's default models (OLGA 1.3.0)", Checksum: checksum, Source: ref.Source.Source, License: ref.License, Citation: ref.Citation, models: map[string]*pgen.Model{}}
	for key, dir := range olgaDirs {
		var files []io.Reader
		for _, name := range []string{"model_params.txt", "model_marginals.txt", "V_gene_CDR3_anchors.csv", "J_gene_CDR3_anchors.csv"} {
			f, err := os.Open(filepath.Join(ref.Path, dir, name))
			if err != nil {
				return nil, err
			}
			defer f.Close()
			files = append(files, f)
		}
		m, err := pgen.ReadIGoR(dir, files[0], files[1], files[2], files[3])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", dir, err)
		}
		set.models[key] = m
	}
	olgaCache.checksum, olgaCache.set = checksum, set
	return set, nil
}

// modelsFor answers the models the options choose.
func (a *app) modelsFor(o pgenOptions) (*modelSet, error) {
	switch o.Models {
	case "clonoloom":
		return clonoloomModels(), nil
	case "olga":
		return a.olgaModels()
	case "":
		if set, err := a.olgaModels(); err == nil {
			return set, nil
		}
		return clonoloomModels(), nil
	}
	return nil, errors.New("models: olga or clonoloom")
}

func (a *app) pgenPrepare(spec *analysisSpec) error {
	o := spec.PGen
	if spec.Definition.Vendor {
		return errors.New("generation probabilities need clonotypes defined by their junctions")
	}
	switch o.Organism {
	case "", "human", "mouse":
	default:
		return errors.New("organism: human or mouse")
	}
	set, err := a.modelsFor(*o)
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
	if spec.Definition.V == "family" || spec.Definition.V == "none" || spec.Definition.J == "family" || spec.Definition.J == "none" {
		o.Genes = false
	}
	o.Checksum = set.Checksum
	return nil
}

// organismOf guesses the organism whose models name the most of these V genes.
func organismOf(set *modelSet, calls []string) string {
	best, found := "human", -1
	for _, organism := range []string{"human", "mouse"} {
		names := map[string]bool{}
		for key, m := range set.models {
			if strings.HasPrefix(key, organism+" ") {
				for _, g := range m.V {
					names[g.Name] = true
					names[strings.SplitN(g.Name, "*", 2)[0]] = true
				}
			}
		}
		if len(names) == 0 {
			continue
		}
		n := 0
		for _, c := range calls {
			if names[strings.SplitN(c, "*", 2)[0]] {
				n++
			}
		}
		if n > found {
			best, found = organism, n
		}
	}
	return best
}

// chainQuery is a chain as a model's query: its model, or why it has none.
func chainQuery(set *modelSet, organism string, ch clonotype.Chain, nt, genes bool) (*pgen.Model, pgen.Query, string) {
	m := set.models[organism+" "+ch.Locus]
	if m == nil {
		return nil, pgen.Query{}, fmt.Sprintf("no model of %s %s", organism, ch.Locus)
	}
	q := pgen.Query{CDR3: ch.Junction, NT: nt}
	if genes {
		var err error
		if ch.V != "" {
			if q.V, err = m.Mask('V', ch.V); err != nil {
				return nil, q, "a V gene the model lacks"
			}
		}
		if ch.J != "" {
			if q.J, err = m.Mask('J', ch.J); err != nil {
				return nil, q, "a J gene the model lacks"
			}
		}
	}
	return m, q, ""
}

// pgenDocument is a generation probabilities artifact's pgen.json: the models, each sample's
// distribution, and each clonotype's probabilities.
type pgenDocument struct {
	analysisHead
	Models   string            `json:"models"`
	Set      modelSet          `json:"set"`
	Organism string            `json:"organism"`
	Genes    bool              `json:"genes"`
	Loci     map[string]string `json:"loci"`
	Samples  []pgenSample      `json:"samples"`
	Without  map[string]int    `json:"without"`
	Rows     []pgenRow         `json:"clonotypes"`
}

// pgenSample is a sample's clonotypes' probabilities (a pair's, the product of its chains'): how
// many have one, how many of those the model cannot make (0), and the quartiles of the others'
// log₁₀.
type pgenSample struct {
	Name      string     `json:"name"`
	With      int        `json:"with"`
	Zero      int        `json:"zero"`
	Quartiles [3]float64 `json:"quartiles"`
}

// pgenRow is a clonotype: its chains, abundance and samples, each chain's probability (−1 when its
// locus has no model), and the product (−1 unless every chain has one).
type pgenRow struct {
	ID      string            `json:"id"`
	Chains  []clonotype.Chain `json:"chains"`
	Total   float64           `json:"total"`
	Samples int               `json:"samples"`
	Chain   []float64         `json:"chain"`
	Pgen    float64           `json:"pgen"`
}

func (a *app) pgenWork(k analysis, spec analysisSpec) jobs.Func {
	return func(ctx context.Context, run *jobs.Run) error {
		ds, head, table, _, err := a.samplesOf(k, spec, run)
		if err != nil {
			return err
		}
		o := *spec.PGen
		set, err := a.modelsFor(o)
		if err != nil {
			return err
		}
		if set.Checksum != o.Checksum {
			return errors.New("the models changed since the probabilities were asked for: ask again")
		}
		organism := o.Organism
		if organism == "" {
			var calls []string
			for _, c := range table.Clonotypes {
				for _, ch := range c.Chains {
					calls = append(calls, ch.V)
				}
			}
			organism = organismOf(set, calls)
		}
		nt := spec.Definition.Junction == "nt"
		doc := pgenDocument{analysisHead: head, Models: o.Models, Set: *set, Organism: organism, Genes: o.Genes, Loci: map[string]string{}, Without: map[string]int{}}
		// Each model's queries, and where their answers go.
		type slot struct{ row, chain int }
		queries := map[*pgen.Model][]pgen.Query{}
		slots := map[*pgen.Model][]slot{}
		doc.Rows = make([]pgenRow, len(table.Clonotypes))
		for i, c := range table.Clonotypes {
			row := pgenRow{ID: c.ID, Chains: c.Chains, Total: c.Total, Samples: c.Samples, Chain: make([]float64, len(c.Chains)), Pgen: -1}
			for k, ch := range c.Chains {
				row.Chain[k] = -1
				m, q, why := chainQuery(set, organism, ch, nt, o.Genes)
				if m == nil {
					doc.Without[why]++
					continue
				}
				doc.Loci[ch.Locus] = m.Name
				queries[m] = append(queries[m], q)
				slots[m] = append(slots[m], slot{i, k})
			}
			doc.Rows[i] = row
		}
		models := make([]*pgen.Model, 0, len(queries))
		total := 0
		for m := range queries {
			models = append(models, m)
			total += len(queries[m])
		}
		sort.Slice(models, func(x, y int) bool { return models[x].Name < models[y].Name })
		done := 0
		for _, m := range models {
			got, err := m.All(ctx, queries[m], 0, func(d, of int) {
				run.Progress(0.1+0.85*float64(done+d)/float64(max(total, 1)), fmt.Sprintf("Computed %d of %d chains' probabilities", done+d, total))
			})
			if err != nil {
				return err
			}
			for x, s := range slots[m] {
				p := got[x]
				if math.IsNaN(p) {
					doc.Without["a junction the model cannot read"]++
					p = -1
				}
				doc.Rows[s.row].Chain[s.chain] = p
			}
			done += len(queries[m])
		}
		for i := range doc.Rows {
			product := 1.0
			for _, p := range doc.Rows[i].Chain {
				if p < 0 {
					product = -1
					break
				}
				product = float64(product * p)
			}
			doc.Rows[i].Pgen = product
		}
		// Each sample's distribution, by its clonotypes.
		values := make([][]float64, len(ds.Result.Table.Samples))
		zero := make([]int, len(values))
		with := make([]int, len(values))
		for i := range table.Clonotypes {
			p := doc.Rows[i].Pgen
			if p < 0 {
				continue
			}
			samples, _ := table.Abundances(i)
			for _, s := range samples {
				with[s]++
				if p == 0 {
					zero[s]++
				} else {
					values[s] = append(values[s], dmath.Log(p)/math.Ln10)
				}
			}
		}
		for s, sample := range ds.Result.Table.Samples {
			x := pgenSample{Name: sample.Name, With: with[s], Zero: zero[s]}
			if v := values[s]; len(v) > 0 {
				sort.Float64s(v)
				for q, f := range []float64{0.25, 0.5, 0.75} {
					x.Quartiles[q] = quantile(v, f)
				}
			}
			doc.Samples = append(doc.Samples, x)
		}
		computed := 0
		for _, r := range doc.Rows {
			if r.Pgen >= 0 {
				computed++
			}
		}
		return a.keepAnalysis(k, spec, ds, run, doc, len(doc.Samples),
			artifacts.Method{Name: "Generation probability of each clonotype's chains under a V(D)J recombination model (Sethna et al. 2019), computed as OLGA computes it",
				Citation: "Sethna Z, et al. OLGA: fast computation of generation probabilities of B- and T-cell receptor amino acid sequences and motifs. Bioinformatics. 2019;35:2974–2981"},
			map[string]any{"clonotypes": len(doc.Rows), "computed": computed, "models": o.Models, "organism": organism, "genes": o.Genes, "definition": head.Definition.Name})
	}
}

// quantile of sorted values (linear between the nearest, as R's type 7).
func quantile(sorted []float64, f float64) float64 {
	h := float64(f * float64(len(sorted)-1))
	lo := int(h)
	if lo+1 >= len(sorted) {
		return sorted[len(sorted)-1]
	}
	return sorted[lo] + float64((h-float64(lo))*(sorted[lo+1]-sorted[lo]))
}

// registerPgen: POST /api/pgen/chains {chains, junction ("aa" or "nt"), pgen} answers each chain's
// generation probability under the options' models, and their product.
func (a *app) registerPgen(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/pgen/chains", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Chains   []clonotype.Chain `json:"chains"`
			Junction string            `json:"junction"`
			PGen     pgenOptions       `json:"pgen"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body); err != nil || len(body.Chains) == 0 || len(body.Chains) > 4 {
			writeError(w, http.StatusBadRequest, "Send {\"chains\": [{\"locus\", \"junction\", \"v\", \"j\"}], \"junction\": \"aa\" or \"nt\", \"pgen\": {…}}.")
			return
		}
		set, err := a.modelsFor(body.PGen)
		if err != nil {
			code := ""
			if errors.Is(err, errNoModels) {
				code = "no-models"
			}
			writeErrorCode(w, http.StatusBadRequest, code, err.Error())
			return
		}
		organism := body.PGen.Organism
		if organism == "" {
			var calls []string
			for _, ch := range body.Chains {
				calls = append(calls, ch.V)
			}
			organism = organismOf(set, calls)
		}
		type answer struct {
			Locus string   `json:"locus"`
			Model string   `json:"model,omitempty"`
			Pgen  *float64 `json:"pgen"`
			Why   string   `json:"why,omitempty"`
		}
		out := make([]answer, len(body.Chains))
		product := 1.0
		for i, ch := range body.Chains {
			out[i].Locus = ch.Locus
			m, q, why := chainQuery(set, organism, ch, body.Junction == "nt", body.PGen.Genes)
			if m == nil {
				out[i].Why, product = why, -1
				continue
			}
			got, _ := m.All(r.Context(), []pgen.Query{q}, 1, nil)
			if math.IsNaN(got[0]) {
				out[i].Why, product = "a junction the model cannot read", -1
				continue
			}
			p := got[0]
			out[i].Model, out[i].Pgen = m.Name, &p
			if product >= 0 {
				product = float64(product * p)
			}
		}
		var all *float64
		if product >= 0 {
			all = &product
		}
		writeJSON(w, map[string]any{"chains": out, "pgen": all, "models": set.Name, "organism": organism, "checksum": set.Checksum})
	})
}
