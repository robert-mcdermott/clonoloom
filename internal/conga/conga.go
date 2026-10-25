// Package conga compares clonotype expression and receptor neighborhoods. It implements the
// explicitly bounded CoNGA-style method in docs/CONGA.md, not the complete CoNGA pipeline.
package conga

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"clonoloom/internal/clonotype"
	"clonoloom/internal/dmath"
	"clonoloom/internal/expression"
	"clonoloom/internal/phenotype"
	"clonoloom/internal/tcrdist"
)

const Version = 1
const MaxClones = 2000
const MaxCells = 10000
const MaxEntries = 2000000 // stored directed neighbor entries, including repeated scales
const MaxWork int64 = 500000000

type Options struct {
	Neighbors []int  `json:"neighbors"`
	CDR3Only  bool   `json:"cdr3Only"`
	Organism  string `json:"organism,omitempty"`
}

func (o Options) Normalize() (Options, error) {
	if len(o.Neighbors) == 0 {
		o.Neighbors = []int{5, 15}
	}
	if len(o.Neighbors) > 8 {
		return o, errors.New("choose at most 8 neighborhood sizes")
	}
	o.Neighbors = append([]int(nil), o.Neighbors...)
	sort.Ints(o.Neighbors)
	for i, k := range o.Neighbors {
		if k < 1 || k > 200 || i > 0 && k == o.Neighbors[i-1] {
			return o, errors.New("neighborhood sizes must be distinct integers from 1 to 200")
		}
	}
	if o.CDR3Only {
		o.Organism = ""
	} else if o.Organism != "human" && o.Organism != "mouse" {
		return o, errors.New("choose human or mouse for full TCRdist, or explicitly choose CDR3 only")
	}
	return o, nil
}
func CheckDefinition(d clonotype.Definition) error {
	if d.Vendor || d.Junction != "aa" || len(d.Chains) != 2 || d.Chains[0] != "TRA" || d.Chains[1] != "TRB" || d.Cells != "primary" || (d.V != "gene" && d.V != "allele") || (d.J != "gene" && d.J != "allele") {
		return errors.New("choose paired TRA/TRB amino-acid clonotypes, primary chains, V and J genes or alleles; vendor, pooled-locus and incomplete definitions cannot define this comparison")
	}
	return nil
}

type Cell struct {
	Barcode string `json:"barcode"`
	Clone   string `json:"clone,omitempty"`
	Reason  string `json:"reason,omitempty"`
}
type Node struct {
	ID          string            `json:"id"`
	Chains      []clonotype.Chain `json:"chains"`
	Barcodes    []string          `json:"barcodes"`
	Scores      []float64         `json:"scores"`
	Candidates  int               `json:"candidates"`
	SharedChain int               `json:"sharedChain"`
}
type Test struct {
	Node        int      `json:"node"`
	K           int      `json:"k"`
	Expression  []int    `json:"expression"`
	Receptor    []int    `json:"receptor"`
	Overlap     *int     `json:"overlap"`
	Expected    *float64 `json:"expected"`
	P           *float64 `json:"p"`
	Adjusted    *float64 `json:"adjusted"`
	LogP        *float64 `json:"logP"`
	LogAdjusted *float64 `json:"logAdjusted"`
	Reason      string   `json:"reason,omitempty"`
}
type Result struct {
	Options    Options           `json:"options"`
	Cells      []Cell            `json:"cells"`
	Nodes      []Node            `json:"nodes"`
	Tests      []Test            `json:"tests"`
	Eligible   int               `json:"eligible"`
	Tested     int               `json:"tested"`
	Entries    int               `json:"entries"`
	Components int               `json:"components"`
	Alleles    map[string]string `json:"alleles,omitempty"`
	Rules      map[string]any    `json:"rules"`
}

// Compute uses only retained expression PCs and the pinned sample's assignments. Each clone
// has one unweighted mean PC profile. Clone size cannot multiply its vertices or hypotheses.
func Compute(ctx context.Context, p *phenotype.Result, m *expression.AnalysisResult, receptors []clonotype.Clonotype, loops *tcrdist.Loops, o Options, progress func(float64)) (*Result, error) {
	o, err := o.Normalize()
	if err != nil {
		return nil, err
	}
	if p == nil || m == nil {
		return nil, errors.New("missing phenotype or expression analysis")
	}
	if err = CheckDefinition(p.Definition); err != nil {
		return nil, err
	}
	if len(m.Cells) > MaxCells || len(m.Cells) == 0 || m.Components < 1 || m.Components > 1000 || len(p.Cells) > phenotype.MaxCells {
		return nil, errors.New("invalid or excessive expression population/components")
	}
	if !o.CDR3Only && (loops == nil || loops.Alleles(o.Organism) == 0) {
		return nil, errors.New("full TCRdist requires the selected organism's V-loop reference")
	}
	byBarcode := map[string]phenotype.Cell{}
	for _, c := range p.Cells {
		if c.Barcode == "" {
			return nil, errors.New("empty phenotype barcode")
		}
		if _, ok := byBarcode[c.Barcode]; ok {
			return nil, errors.New("duplicate phenotype barcode")
		}
		byBarcode[c.Barcode] = c
	}
	byClone := map[string]clonotype.Clonotype{}
	for _, c := range receptors {
		if _, ok := byClone[c.ID]; ok {
			return nil, errors.New("duplicate receptor clonotype")
		}
		byClone[c.ID] = c
	}
	r := &Result{Options: o, Components: m.Components, Cells: []Cell{}, Nodes: []Node{}, Tests: []Test{}, Rules: map[string]any{
		"unit":           "one paired alpha-beta clonotype in one explicitly bound sample",
		"expression":     "arithmetic mean of retained cells in every saved PC; Euclidean distance; no refit or UMAP distances",
		"receptor":       "sum of paired-chain TCRdist; CDR3-only mode must be selected explicitly",
		"candidates":     "exclude self and any clone sharing an exact alpha or beta CDR3 amino-acid sequence, regardless of V/J",
		"neighbors":      "directed k-nearest sets within the same focal candidate pool; include every exact boundary tie; no ID-based tie breaking",
		"null":           "conditional on candidate pool and both degrees, expression-neighbor labels are a uniform subset independent of receptor neighbors",
		"test":           "inclusive upper-tail hypergeometric; expected = expression degree * receptor degree / candidates",
		"correction":     "Bonferroni over every testable clonotype x requested k, including zero overlap and degenerate tied sets",
		"limits":         "10000 retained cells, 2000 paired clonotypes, 8 scales, k <= 200, 2000000 stored directed neighbor entries; 500 million distance work units",
		"interpretation": "exploratory within-sample association; exchangeability not established; no conditioning on cell type, batch or receptor-chain groups; not antigen specificity, causality or subject-level inference; repeated analysis choices not corrected",
		"adaptation":     "ClonoLoom CoNGA-style KNN comparison, not the published multi-graph CoNGA score or a complete CoNGA reproduction",
	}}
	seen := map[string]bool{}
	groups := map[string][]int{}
	for i, c := range m.Cells {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if c.Barcode == "" || seen[c.Barcode] || len(c.Scores) != m.Components {
			return nil, errors.New("invalid expression barcode or PC dimensions")
		}
		seen[c.Barcode] = true
		for _, v := range c.Scores {
			if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 1e100 {
				return nil, errors.New("nonfinite or excessive PC score")
			}
		}
		pc, ok := byBarcode[c.Barcode]
		if !ok {
			return nil, errors.New("expression barcode missing from phenotype parent")
		}
		cell := Cell{Barcode: c.Barcode, Clone: pc.Clone}
		switch {
		case pc.Status != "Assigned" && pc.Status != "Outside size population":
			cell.Reason = pc.Status
			if cell.Reason == "" {
				cell.Reason = "Unassigned"
			}
		case pc.Chains != "Paired":
			cell.Reason = "Not paired receptor evidence"
		default:
			receptor, ok := byClone[pc.Clone]
			if !ok {
				return nil, errors.New("assigned clonotype missing from pinned receptor snapshot")
			}
			cell.Reason = receptorReason(receptor, loops, o)
		}
		r.Cells = append(r.Cells, cell)
		if cell.Reason == "" {
			groups[cell.Clone] = append(groups[cell.Clone], i)
			r.Eligible++
		}
	}
	if len(groups) < 2 || len(groups) > MaxClones {
		return nil, fmt.Errorf("comparison needs 2–%d eligible paired clonotypes; found %d", MaxClones, len(groups))
	}
	ids := make([]string, 0, len(groups))
	for id := range groups {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	cs := make([]clonotype.Clonotype, 0, len(ids))
	for _, id := range ids {
		indices := groups[id]
		sort.Slice(indices, func(i, j int) bool { return m.Cells[indices[i]].Barcode < m.Cells[indices[j]].Barcode })
		n := Node{ID: id, Chains: byClone[id].Chains, Scores: make([]float64, m.Components)}
		for _, i := range indices {
			c := m.Cells[i]
			n.Barcodes = append(n.Barcodes, c.Barcode)
			for j, v := range c.Scores {
				n.Scores[j] += v
			}
		}
		for j := range n.Scores {
			n.Scores[j] /= float64(len(indices))
		}
		r.Nodes = append(r.Nodes, n)
		cs = append(cs, byClone[id])
	}
	// Conservative bound on ordered pair work, including gap-placement scans. Refuse before
	// constructing the distance set; never silently thin clones or components.
	longest := 0
	for _, c := range cs {
		for _, ch := range c.Chains {
			longest = max(longest, len(ch.Junction))
		}
	}
	work := int64(len(cs)) * int64(len(cs)-1) * int64(m.Components+2*longest*longest+1000)
	if work > MaxWork {
		return nil, errors.New("comparison exceeds 500 million conservative distance work units")
	}
	set := tcrdist.NewSet(cs, loopsFor(o, loops), o.Organism)
	if set.Len() != len(cs) || len(set.Groups) != 1 {
		return nil, errors.New("receptor distance population differs from paired clones")
	}
	r.Alleles = set.Alleles
	indices := make([]int, len(cs))
	for i, c := range set.Clonotype {
		indices[c] = i
	}
	for i := range r.Nodes {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		a := r.Nodes[i]
		var gex, tcr []neighbor
		for j, b := range r.Nodes {
			if j%64 == 0 {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
			}
			if i == j {
				continue
			}
			if a.Chains[0].Junction == b.Chains[0].Junction || a.Chains[1].Junction == b.Chains[1].Junction {
				r.Nodes[i].SharedChain++
				continue
			}
			d := 0.0
			for q, x := range a.Scores {
				delta := x - b.Scores[q]
				d += float64(delta * delta)
			}
			gex = append(gex, neighbor{j, math.Sqrt(d)})
			tcr = append(tcr, neighbor{j, float64(set.Distance(indices[i], indices[j]))})
		}
		r.Nodes[i].Candidates = len(gex)
		sort.Slice(gex, func(i, j int) bool { return gex[i].distance < gex[j].distance })
		sort.Slice(tcr, func(i, j int) bool { return tcr[i].distance < tcr[j].distance })
		for _, k := range o.Neighbors {
			t := Test{Node: i, K: k}
			if k >= len(gex) {
				t.Reason = "Too few nonshared-chain candidates for requested k"
				r.Tests = append(r.Tests, t)
				continue
			}
			t.Expression = neighbors(gex, k)
			t.Receptor = neighbors(tcr, k)
			r.Entries += len(t.Expression) + len(t.Receptor)
			if r.Entries > MaxEntries {
				return nil, errors.New("neighbor output exceeds 2000000 entries; reduce predeclared scales or population")
			}
			overlap := 0
			for _, j := range t.Expression {
				q := sort.SearchInts(t.Receptor, j)
				if q < len(t.Receptor) && t.Receptor[q] == j {
					overlap++
				}
			}
			t.Overlap = &overlap
			e := float64(len(t.Expression)*len(t.Receptor)) / float64(len(gex))
			t.Expected = &e
			lp := LogTail(len(gex), len(t.Expression), len(t.Receptor), overlap)
			t.LogP = &lp
			t.P = probability(lp)
			r.Tested++
			r.Tests = append(r.Tests, t)
		}
		if progress != nil {
			progress(float64(i+1) / float64(len(r.Nodes)))
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for i := range r.Tests {
		t := &r.Tests[i]
		if t.LogP != nil {
			lp := math.Min(0, *t.LogP+dmath.Log(float64(r.Tested)))
			t.LogAdjusted = &lp
			t.Adjusted = probability(lp)
		}
	}
	return r, nil
}
func loopsFor(o Options, l *tcrdist.Loops) *tcrdist.Loops {
	if o.CDR3Only {
		return nil
	}
	return l
}
func receptorReason(c clonotype.Clonotype, l *tcrdist.Loops, o Options) string {
	if len(c.Chains) != 2 || c.Chains[0].Locus != "TRA" || c.Chains[1].Locus != "TRB" {
		return "Incomplete paired alpha-beta clonotype"
	}
	for _, ch := range c.Chains {
		if len(ch.Junction) < 6 || len(ch.Junction) > 80 || strings.Trim(ch.Junction, "ARNDCQEGHILKMFPSTWYV") != "" {
			return "CDR3 must contain 6–80 standard amino acids"
		}
		if ch.V == "" || ch.J == "" {
			return "Missing V or J gene"
		}
		if !o.CDR3Only {
			if _, _, ok := l.Lookup(o.Organism, ch.V); !ok {
				return "V gene absent from selected organism's loop reference"
			}
		}
	}
	return ""
}

type neighbor struct {
	index    int
	distance float64
}

func neighbors(v []neighbor, k int) []int {
	out := []int{}
	cut := v[k-1].distance
	for _, n := range v {
		if n.distance > cut {
			break
		}
		out = append(out, n.index)
	}
	sort.Ints(out)
	return out
}
func probability(lp float64) *float64 {
	p := dmath.Exp(lp)
	if p == 0 {
		return nil
	}
	return &p
}

// LogTail is log P[X >= overlap] for Hypergeom(population, a, b). Relative log
// weights avoid factorial overflow and retain finite tails even below float64 probability range.
// Inputs are validated counts from two subsets of the same nonempty candidate pool.
func LogTail(population, a, b, overlap int) float64 {
	lo, hi := max(0, a+b-population), min(a, b)
	if overlap <= lo {
		return 0
	}
	w := make([]float64, hi-lo+1)
	top, tailTop := 0.0, math.Inf(-1)
	for x := lo; x < hi; x++ {
		w[x-lo+1] = w[x-lo] + dmath.Log(float64(a-x)) + dmath.Log(float64(b-x)) - dmath.Log(float64(x+1)) - dmath.Log(float64(population-a-b+x+1))
		top = math.Max(top, w[x-lo+1])
	}
	for x := overlap; x <= hi; x++ {
		tailTop = math.Max(tailTop, w[x-lo])
	}
	total, tail := 0.0, 0.0
	for x := lo; x <= hi; x++ {
		total += dmath.Exp(w[x-lo] - top)
		if x >= overlap {
			tail += dmath.Exp(w[x-lo] - tailTop)
		}
	}
	return math.Min(0, tailTop-top+dmath.Log(tail)-dmath.Log(total))
}
