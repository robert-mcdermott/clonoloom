package clonotype

import (
	"errors"
	"math"
	"sort"
	"strings"

	"clonoloom/internal/genes"
	"clonoloom/internal/rep"
)

// Chain is one chain of a clonotype: its locus, junction and V and J genes at the definition's
// resolution ("" when the definition does not use them).
type Chain struct {
	Locus    string `json:"locus"`
	Junction string `json:"junction"`
	V        string `json:"v,omitempty"`
	J        string `json:"j,omitempty"`
}

// Clonotype is one clonotype of a table.
type Clonotype struct {
	ID     string  `json:"id"`
	Chains []Chain `json:"chains"`
	// Vendor: the source's own clonotype ID, for a vendor definition.
	Vendor string `json:"vendor,omitempty"`
	// Total abundance over the samples, in the dataset's unit (cells for single-cell data).
	Total float64 `json:"total"`
	// Samples in which it was found.
	Samples int `json:"samples"`
}

// Reasons rearrangements (or cells) belong to no clonotype.
const (
	ReasonFiltered      = "left out by the filter"
	ReasonNoJunction    = "no junction"
	ReasonNotProductive = "not productive"
	ReasonOtherLocus    = "another locus"
	ReasonMissingChain  = "cell without every chain"
	ReasonNoVendorID    = "no clonotype ID in the source"
	ReasonNoCount       = "no count"
)

// Table is a dataset's clonotypes under a definition: each with its abundance in every sample
// (stored sparsely), and its members (rows of the rearrangement table).
type Table struct {
	Definition   Definition  `json:"definition"`
	DefinitionID string      `json:"definitionId"`
	Unit         rep.Unit    `json:"unit"`
	Clonotypes   []Clonotype `json:"-"`
	// Abundance of clonotype i in its samples: SampleOf[Start[i]:Start[i+1]] with Abundance.
	Start     []int32   `json:"-"`
	SampleOf  []uint32  `json:"-"`
	Abundance []float64 `json:"-"`
	// SampleTotals: each sample's abundance among the clonotypes (frequencies' denominator).
	SampleTotals []float64 `json:"sampleTotals"`
	// Members of clonotype i: rows Members[MemberStart[i]:MemberStart[i+1]].
	MemberStart []int32  `json:"-"`
	Members     []uint32 `json:"-"`
	// Excluded: what is in no clonotype, by reason: rearrangements (bulk, and single-cell rows a
	// filter left out) or cells (single cells without the chains the definition needs).
	Excluded map[string]int `json:"excluded"`
	// Cells grouped (single cell).
	Cells      int  `json:"cells,omitempty"`
	SingleCell bool `json:"singleCell"`
}

// ErrPairedBulk: paired chains are defined only for single-cell data.
var ErrPairedBulk = errors.New("paired chains need single-cell data: bulk sequencing does not say which chains share a cell")

// Abundances answers clonotype i's samples and abundances.
func (t *Table) Abundances(i int) ([]uint32, []float64) {
	return t.SampleOf[t.Start[i]:t.Start[i+1]], t.Abundance[t.Start[i]:t.Start[i+1]]
}

// MembersOf answers clonotype i's rows.
func (t *Table) MembersOf(i int) []uint32 {
	return t.Members[t.MemberStart[i]:t.MemberStart[i+1]]
}

// In is clonotype i's abundance in a sample (0 when absent).
func (t *Table) In(i int, sample uint32) float64 {
	samples, values := t.Abundances(i)
	for k, s := range samples {
		if s == sample {
			return values[k]
		}
	}
	return 0
}

type keyer struct {
	t      *rep.Table
	d      Definition
	vLevel genes.Level
	jLevel genes.Level
	keys   map[uint32]string
	jkeys  map[uint32]string
}

func (k *keyer) gene(code uint32, level genes.Level, cache map[uint32]string) string {
	if level == genes.LevelNone || code == 0 {
		return ""
	}
	if v, ok := cache[code]; ok {
		return v
	}
	v := k.t.Call(code).Key(level)
	cache[code] = v
	return v
}

// chain is a row's chain under the definition, or the reason it has none.
func (k *keyer) chain(row int) (Chain, string) {
	t := k.t
	if k.d.ProductiveOnly && t.Productive[row] != rep.True {
		return Chain{}, ReasonNotProductive
	}
	var junction string
	if k.d.Junction == "nt" {
		junction = t.Junctions.Get(t.Junction[row])
	} else {
		junction = t.JunctionsAA.Get(t.JunctionAA[row])
	}
	if junction == "" {
		return Chain{}, ReasonNoJunction
	}
	return Chain{Locus: t.LocusName(row), Junction: junction, V: k.gene(t.V[row], k.vLevel, k.keys), J: k.gene(t.J[row], k.jLevel, k.jkeys)}, ""
}

func chainKey(c Chain) string {
	return c.Locus + "\x1f" + c.Junction + "\x1f" + c.V + "\x1f" + c.J
}

// slot is the definition's chain a locus fills ("light" for IGK and IGL), or "" when none.
func (d Definition) slot(locus string) string {
	if len(d.Chains) == 0 {
		return locus
	}
	for _, c := range d.Chains {
		if c == locus || (c == Light && (locus == "IGK" || locus == "IGL")) {
			return c
		}
	}
	return ""
}

// builder collects clonotypes by key.
type builder struct {
	canonical string
	index     map[string]int
	chains    [][]Chain
	vendor    []string
	keys      []string
	abundance []map[uint32]float64
	members   [][]uint32
}

func (b *builder) add(key string, chains []Chain, vendor string, sample uint32, w float64, rows []uint32) {
	i, ok := b.index[key]
	if !ok {
		i = len(b.keys)
		b.index[key] = i
		b.keys = append(b.keys, key)
		b.chains = append(b.chains, chains)
		b.vendor = append(b.vendor, vendor)
		b.abundance = append(b.abundance, map[uint32]float64{})
		b.members = append(b.members, nil)
	}
	b.abundance[i][sample] += w
	b.members[i] = append(b.members[i], rows...)
}

// Compute groups a table's rows that keep (a filter's verdict; nil keeps every row) into
// clonotypes under a definition.
func Compute(t *rep.Table, keep []bool, d Definition) (*Table, error) {
	d, err := d.Normalize()
	if err != nil {
		return nil, err
	}
	single := t.Cell != nil
	if len(d.Chains) == 2 && !single && !d.Vendor {
		return nil, ErrPairedBulk
	}
	vLevel, _ := Level(d.V)
	jLevel, _ := Level(d.J)
	k := &keyer{t: t, d: d, vLevel: vLevel, jLevel: jLevel, keys: map[uint32]string{}, jkeys: map[uint32]string{}}
	out := &Table{Definition: d, DefinitionID: d.ID(), Unit: t.Unit, Excluded: map[string]int{}, SingleCell: single}
	if single {
		out.Unit = rep.Cells
	}
	b := &builder{canonical: d.Canonical(), index: map[string]int{}}
	kept := func(row int) bool { return keep == nil || keep[row] }
	switch {
	case d.Vendor:
		vendorClonotypes(t, kept, b, out)
	case single:
		cellClonotypes(t, kept, d, k, b, out)
	default:
		for row := 0; row < t.Len(); row++ {
			if !kept(row) {
				out.Excluded[ReasonFiltered]++
				continue
			}
			if d.slot(t.LocusName(row)) == "" {
				out.Excluded[ReasonOtherLocus]++
				continue
			}
			c, reason := k.chain(row)
			if reason != "" {
				out.Excluded[reason]++
				continue
			}
			w := t.Count[row]
			if math.IsNaN(w) {
				out.Excluded[ReasonNoCount]++
				continue
			}
			b.add(chainKey(c), []Chain{c}, "", t.Sample[row], w, []uint32{uint32(row)})
		}
	}
	finish(b, out, len(t.Samples))
	return out, nil
}

// candidate is one of a cell's chains.
type candidate struct {
	chain     Chain
	umis, rds float64
	id        string
	row       uint32
}

// cellClonotypes groups single cells by their chains.
func cellClonotypes(t *rep.Table, kept func(int) bool, d Definition, k *keyer, b *builder, out *Table) {
	type cell struct {
		sample uint32
		slots  map[string][]candidate
		rows   []uint32
	}
	cells := map[uint64]*cell{}
	var order []uint64
	// Cells the filter kept a row of, whether or not one of their chains is usable.
	seen := map[uint64]bool{}
	for row := 0; row < t.Len(); row++ {
		if t.Cell[row] == 0 {
			continue
		}
		if !kept(row) {
			out.Excluded[ReasonFiltered]++
			continue
		}
		seen[uint64(t.Sample[row])<<32|uint64(t.Cell[row])] = true
		slot := d.slot(t.LocusName(row))
		if slot == "" {
			continue
		}
		c, reason := k.chain(row)
		if reason != "" {
			continue
		}
		id := uint64(t.Sample[row])<<32 | uint64(t.Cell[row])
		cl := cells[id]
		if cl == nil {
			cl = &cell{sample: t.Sample[row], slots: map[string][]candidate{}}
			cells[id] = cl
			order = append(order, id)
		}
		cand := candidate{chain: c, umis: valueOr(t.UMIs, row, t.Count[row]), rds: valueOr(t.Reads, row, 0), id: t.IDs.Get(t.SequenceID[row]), row: uint32(row)}
		cl.slots[slot] = append(cl.slots[slot], cand)
	}
	// Cells none of whose chains is usable under the definition.
	for id := range seen {
		if cells[id] == nil {
			out.Excluded[ReasonMissingChain]++
		}
	}
	slots := d.Chains
	for _, id := range order {
		cl := cells[id]
		if len(slots) == 0 {
			// Each locus separately: a cell counts once in the clonotype of each locus's chain.
			loci := make([]string, 0, len(cl.slots))
			for l := range cl.slots {
				loci = append(loci, l)
			}
			sort.Strings(loci)
			out.Cells++
			for _, l := range loci {
				chosen := choose(cl.slots[l], d.Cells)
				chains, key, rows := assemble([][]candidate{chosen})
				b.add(key, chains, "", cl.sample, 1, rows)
			}
			continue
		}
		var picked [][]candidate
		missing := false
		for _, s := range slots {
			chosen := choose(cl.slots[s], d.Cells)
			if len(chosen) == 0 {
				missing = true
			}
			picked = append(picked, chosen)
		}
		if missing && (d.RequireAll || len(slots) == 1 || allEmpty(picked)) {
			out.Excluded[ReasonMissingChain]++
			continue
		}
		out.Cells++
		chains, key, rows := assemble(picked)
		b.add(key, chains, "", cl.sample, 1, rows)
	}
}

func allEmpty(p [][]candidate) bool {
	for _, c := range p {
		if len(c) > 0 {
			return false
		}
	}
	return true
}

// choose a cell's chains of a locus: its primary one (most UMIs, then most reads, then the first
// contig ID in order: ClonoLoom's choice for ties, so that the choice is the same everywhere), or
// all of them in the order of their keys.
func choose(candidates []candidate, mode string) []candidate {
	if len(candidates) == 0 {
		return nil
	}
	if mode == "all" {
		out := append([]candidate(nil), candidates...)
		sort.Slice(out, func(x, y int) bool {
			if kx, ky := chainKey(out[x].chain), chainKey(out[y].chain); kx != ky {
				return kx < ky
			}
			return out[x].row < out[y].row
		})
		return out
	}
	best := candidates[0]
	for _, c := range candidates[1:] {
		switch {
		case c.umis != best.umis:
			if c.umis > best.umis {
				best = c
			}
		case c.rds != best.rds:
			if c.rds > best.rds {
				best = c
			}
		case c.id < best.id:
			best = c
		}
	}
	return []candidate{best}
}

// assemble a cell's clonotype key from its chosen chains, slot by slot ("" for a slot it lacks),
// with the rows they came from.
func assemble(picked [][]candidate) ([]Chain, string, []uint32) {
	var chains []Chain
	var rows []uint32
	parts := make([]string, len(picked))
	for i, slot := range picked {
		keys := make([]string, len(slot))
		for j, c := range slot {
			keys[j] = chainKey(c.chain)
			chains = append(chains, c.chain)
			rows = append(rows, c.row)
		}
		parts[i] = strings.Join(keys, "\x1d")
	}
	return chains, strings.Join(parts, "\x1e"), rows
}

func valueOr(column []float64, row int, fallback float64) float64 {
	if column == nil || math.IsNaN(column[row]) {
		if math.IsNaN(fallback) {
			return 0
		}
		return fallback
	}
	return column[row]
}

func vendorClonotypes(t *rep.Table, kept func(int) bool, b *builder, out *Table) {
	if t.Clone == nil {
		out.Excluded[ReasonNoVendorID] = t.Len()
		return
	}
	single := t.Cell != nil
	type cell struct {
		sample uint32
		clone  uint32
		rows   []uint32
	}
	cells := map[uint64]*cell{}
	var order []uint64
	for row := 0; row < t.Len(); row++ {
		if !kept(row) {
			out.Excluded[ReasonFiltered]++
			continue
		}
		clone := t.Clone[row]
		if clone == 0 {
			out.Excluded[ReasonNoVendorID]++
			continue
		}
		if !single {
			w := t.Count[row]
			if math.IsNaN(w) {
				out.Excluded[ReasonNoCount]++
				continue
			}
			key := t.Dict.Get(clone)
			b.add(key, nil, key, t.Sample[row], w, []uint32{uint32(row)})
			continue
		}
		id := uint64(t.Sample[row])<<32 | uint64(t.Cell[row])
		cl := cells[id]
		if cl == nil {
			cl = &cell{sample: t.Sample[row], clone: clone}
			cells[id] = cl
			order = append(order, id)
		}
		cl.rows = append(cl.rows, uint32(row))
	}
	for _, id := range order {
		cl := cells[id]
		key := t.Dict.Get(cl.clone)
		out.Cells++
		b.add(key, nil, key, cl.sample, 1, cl.rows)
	}
}

// finish orders the clonotypes by their keys (the same order on every computer) and stores them.
func finish(b *builder, out *Table, samples int) {
	order := make([]int, len(b.keys))
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(x, y int) bool { return b.keys[order[x]] < b.keys[order[y]] })
	out.SampleTotals = make([]float64, samples)
	out.Clonotypes = make([]Clonotype, len(order))
	out.Start = make([]int32, len(order)+1)
	out.MemberStart = make([]int32, len(order)+1)
	for n, i := range order {
		c := Clonotype{ID: clonotypeID(b.canonical, b.keys[i]), Chains: b.chains[i], Vendor: b.vendor[i]}
		if c.Chains == nil {
			c.Chains = []Chain{}
		}
		sampleIDs := make([]uint32, 0, len(b.abundance[i]))
		for s := range b.abundance[i] {
			sampleIDs = append(sampleIDs, s)
		}
		sort.Slice(sampleIDs, func(x, y int) bool { return sampleIDs[x] < sampleIDs[y] })
		for _, s := range sampleIDs {
			v := b.abundance[i][s]
			out.SampleOf = append(out.SampleOf, s)
			out.Abundance = append(out.Abundance, v)
			c.Total += v
			out.SampleTotals[s] += v
		}
		c.Samples = len(sampleIDs)
		out.Start[n+1] = int32(len(out.SampleOf))
		members := b.members[i]
		sort.Slice(members, func(x, y int) bool { return members[x] < members[y] })
		out.Members = append(out.Members, members...)
		out.MemberStart[n+1] = int32(len(out.Members))
		out.Clonotypes[n] = c
	}
}

// Junctions of a clonotype as text: its chains' junctions joined by "; ".
func (c Clonotype) Junctions() string {
	parts := make([]string, len(c.Chains))
	for i, ch := range c.Chains {
		parts[i] = ch.Junction
	}
	return strings.Join(parts, "; ")
}
