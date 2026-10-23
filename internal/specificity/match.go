package specificity

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"clonoloom/internal/clonotype"
	"clonoloom/internal/diversity"
	"clonoloom/internal/genes"
	"clonoloom/internal/tcrdist"
)

type Options struct {
	Metric   string `json:"metric"`
	Genes    bool   `json:"genes"`
	Radius   int    `json:"radius"`
	Species  string `json:"species"`
	MinScore int    `json:"minScore"`
	CDR3Only bool   `json:"cdr3Only"`
}

func (o Options) Normalize() (Options, error) {
	if o.Metric == "" {
		o.Metric = "exact"
	}
	if o.Metric != "exact" && o.Metric != "tcrdist" {
		return o, errors.New("match by exact junction or tcrdist")
	}
	if o.Species == "" {
		o.Species = "HomoSapiens"
	}
	if o.Species != "HomoSapiens" && o.Species != "MusMusculus" && o.Species != "MacacaMulatta" {
		return o, errors.New("choose HomoSapiens, MusMusculus or MacacaMulatta")
	}
	if o.Radius < 0 || o.Radius > 1000 || o.MinScore < 0 || o.MinScore > 3 {
		return o, errors.New("radius: 0–1000; minimum score: 0–3")
	}
	if o.Metric == "exact" {
		o.Radius = 0
		o.CDR3Only = false
	}
	return o, nil
}
func (o Options) Organism() string {
	return map[string]string{"HomoSapiens": "human", "MusMusculus": "mouse", "MacacaMulatta": "rhesus"}[o.Species]
}

type Match struct {
	Clonotype int `json:"clonotype"`
	Chain     int `json:"chain"`
	Record    int `json:"record"` // index into Result.Records; Row preserves the input identity
	Distance  int `json:"distance"`
}
type Epitope struct {
	Epitope    string             `json:"epitope"`
	Antigen    string             `json:"antigen"`
	Species    string             `json:"species"` // antigen species, not the receptor's organism
	Clonotypes []string           `json:"clonotypes"`
	Counts     []float64          `json:"counts"`
	Shares     []diversity.Number `json:"shares"`
}
type Result struct {
	Records  []Record       `json:"records"`
	Matches  []Match        `json:"matches"`
	Epitopes []Epitope      `json:"epitopes"`
	Excluded map[string]int `json:"excluded"`
}

// Find compares each retained chain with records of its locus and the explicitly
// chosen receptor species. Exact V/J matching requires equal nonempty call sets
// at the definition's resolution. Near matching reuses the validated TCRdist
// implementation (Dash 2017; Mayer-Blackwell 2021). Missing loops never fall back
// silently. Budgets fail the whole result, never truncate evidence.
func Find(ctx context.Context, t *clonotype.Table, records []Record, o Options, loops *tcrdist.Loops) (Result, error) {
	var err error
	o, err = o.Normalize()
	if err != nil {
		return Result{}, err
	}
	if t.Definition.Vendor || t.Definition.Junction != "aa" {
		return Result{}, errors.New("specificity matching requires amino-acid junction clonotypes")
	}
	if o.Genes && (t.Definition.V == "none" || t.Definition.J == "none" || t.Definition.V == "family" || t.Definition.J == "family") {
		return Result{}, errors.New("V/J matching requires genes or alleles in the definition")
	}
	if o.Metric == "tcrdist" && !o.CDR3Only && loops == nil {
		return Result{}, errors.New("TCRdist requires V-gene loops, or explicit CDR3-only comparison")
	}
	vl, _ := clonotype.Level(t.Definition.V)
	jl, _ := clonotype.Level(t.Definition.J)
	res := Result{Records: []Record{}, Matches: []Match{}, Epitopes: []Epitope{}, Excluded: map[string]int{}}
	type entry struct {
		source int
		chain  tcrdist.Chain
		v, j   string
	}
	index := map[string][]entry{}
	encode := func(cdr3, v string) (tcrdist.Chain, error) {
		var l []string
		if !o.CDR3Only {
			x, _, ok := loops.Lookup(o.Organism(), v)
			if !ok {
				return tcrdist.Chain{}, errors.New("missing V-gene loops")
			}
			l = x[:]
		}
		if len(cdr3) < 5 || strings.Trim(cdr3, "ACDEFGHIKLMNPQRSTVWY") != "" {
			return tcrdist.Chain{}, errors.New("invalid amino-acid junction")
		}
		return tcrdist.NewChain(cdr3, l), nil
	}
	key := func(locus, cdr3 string) string {
		if o.Metric == "exact" {
			return locus + "\x00" + cdr3
		}
		return locus
	}
	for i, r := range records {
		if i%1024 == 0 {
			if err := ctx.Err(); err != nil {
				return Result{}, err
			}
		}
		if r.Species != o.Species || r.Score < o.MinScore {
			continue
		}
		if r.Locus != "TRA" && r.Locus != "TRB" && r.Locus != "TRG" && r.Locus != "TRD" {
			res.Excluded["database: unsupported locus"]++
			continue
		}
		if r.CDR3 == "" || r.Epitope == "" {
			res.Excluded["database: missing CDR3 or epitope"]++
			continue
		}
		e := entry{source: i, v: genes.ParseCall(r.V).Key(vl), j: genes.ParseCall(r.J).Key(jl)}
		if o.Metric == "tcrdist" {
			e.chain, err = encode(r.CDR3, r.V)
			if err != nil {
				res.Excluded["database: "+err.Error()]++
				continue
			}
		}
		index[key(r.Locus, r.CDR3)] = append(index[key(r.Locus, r.CDR3)], e)
	}
	kept := map[int]int{}
	epitopes := map[string]*Epitope{}
	comparisons := 0
	for ci, c := range t.Clonotypes {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		seen := map[string]bool{}
		for ch, chain := range c.Chains {
			var encoded tcrdist.Chain
			if o.Metric == "tcrdist" {
				encoded, err = encode(chain.Junction, chain.V)
				if err != nil {
					res.Excluded["query: "+err.Error()]++
					continue
				}
			}
			for _, e := range index[key(chain.Locus, chain.Junction)] {
				comparisons++
				if comparisons%1024 == 0 {
					if err := ctx.Err(); err != nil {
						return Result{}, err
					}
				}
				if comparisons > 100_000_000 {
					return Result{}, errors.New("specificity comparison budget exceeded; use exact matching, a smaller dataset or a higher minimum score")
				}
				if o.Genes && (chain.V == "" || chain.J == "" || chain.V != e.v || chain.J != e.j) {
					continue
				}
				distance := 0
				if o.Metric == "tcrdist" {
					diff := len(encoded.CDR3) - len(e.chain.CDR3)
					if diff < 0 {
						diff = -diff
					}
					if diff*12 > o.Radius {
						continue
					}
					distance = tcrdist.ChainDistance(encoded, e.chain)
					if distance > o.Radius {
						continue
					}
				}
				ri, ok := kept[e.source]
				if !ok {
					ri = len(res.Records)
					kept[e.source] = ri
					res.Records = append(res.Records, records[e.source])
				}
				if len(res.Matches) >= 2_000_000 {
					return Result{}, errors.New("specificity evidence budget exceeded; narrow the matching options")
				}
				res.Matches = append(res.Matches, Match{ci, ch, ri, distance})
				r := records[e.source]
				k := strings.Join([]string{r.Epitope, r.Antigen, r.AntigenSpecies}, "\x00")
				if seen[k] {
					continue
				}
				seen[k] = true
				ep := epitopes[k]
				if ep == nil {
					ep = &Epitope{Epitope: r.Epitope, Antigen: r.Antigen, Species: r.AntigenSpecies, Clonotypes: []string{}, Counts: make([]float64, len(t.SampleTotals)), Shares: make([]diversity.Number, len(t.SampleTotals))}
					epitopes[k] = ep
				}
				ep.Clonotypes = append(ep.Clonotypes, c.ID)
				ss, aa := t.Abundances(ci)
				for i, s := range ss {
					ep.Counts[s] += aa[i]
				}
			}
		}
	}
	keys := make([]string, 0, len(epitopes))
	for k := range epitopes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		ep := epitopes[k]
		for i, total := range t.SampleTotals {
			ep.Shares[i] = diversity.Number(math.NaN())
			if total > 0 {
				ep.Shares[i] = diversity.Number(ep.Counts[i] / total)
			}
		}
		res.Epitopes = append(res.Epitopes, *ep)
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	return res, nil
}

func Words(o Options) string {
	return fmt.Sprintf("%s chain matching, V/J required %t, radius %d, receptor species %s, minimum score %d, CDR3-only %t", o.Metric, o.Genes, o.Radius, o.Species, o.MinScore, o.CDR3Only)
}
