// Package qc measures each sample's quality before analysis (clonoloom-spec/requirements.md, Q1–Q3
// and Q6): its abundance, the shares of productive, out-of-frame and stop-codon rearrangements, its
// junction lengths, how often V and J genes were resolved, its singletons and sample coverage, and
// for single-cell data the chains each cell carries. Filters keep or leave out rows by named rules
// and say how many each rule left out; they never change the data.
package qc

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"clonoloom/internal/genes"
	"clonoloom/internal/rep"
	"clonoloom/internal/seq"
)

// Filter is the rules a QC filter version applies. The zero Filter keeps every row.
type Filter struct {
	// ProductiveOnly leaves out rearrangements not marked productive.
	ProductiveOnly bool `json:"productiveOnly,omitempty"`
	// MinCount leaves out bulk rearrangements counted fewer times (in the dataset's unit).
	MinCount float64 `json:"minCount,omitempty"`
	// Loci keeps only these loci (all when empty).
	Loci []string `json:"loci,omitempty"`
	// CellsOnly keeps only contigs of barcodes called cells (Cell Ranger's is_cell).
	CellsOnly bool `json:"cellsOnly,omitempty"`
	// HighConfidenceOnly keeps only Cell Ranger's high-confidence contigs.
	HighConfidenceOnly bool `json:"highConfidenceOnly,omitempty"`
	// MaxChains leaves out cells with more productive chains of a locus than this (0: keeps them).
	MaxChains int `json:"maxChains,omitempty"`
	// ExcludeSamples leaves out whole samples (by name).
	ExcludeSamples []string `json:"excludeSamples,omitempty"`
}

// Reasons a filter leaves a row out, in the order they are tested.
const (
	ReasonSample        = "sample left out"
	ReasonLocus         = "another locus"
	ReasonNotCell       = "not a cell"
	ReasonLowConfidence = "low confidence"
	ReasonNotProductive = "not productive"
	ReasonBelowCount    = "below the minimum count"
	ReasonManyChains    = "cell with too many chains"
)

// Apply answers which rows a filter keeps, and how many of each sample's rows each rule left out.
func Apply(t *rep.Table, f Filter) (keep []bool, removed []map[string]int) {
	n := t.Len()
	keep = make([]bool, n)
	removed = make([]map[string]int, len(t.Samples))
	for i := range removed {
		removed[i] = map[string]int{}
	}
	excluded := map[string]bool{}
	for _, s := range f.ExcludeSamples {
		excluded[s] = true
	}
	loci := map[uint8]bool{}
	for _, l := range f.Loci {
		if c := rep.LocusCode(l); c != 0 {
			loci[c] = true
		}
	}
	// Cells with too many productive chains of a locus, by (sample, cell).
	var crowded map[uint64]bool
	if f.MaxChains > 0 && t.Cell != nil {
		chains := map[[2]uint64]int{}
		crowded = map[uint64]bool{}
		for row := 0; row < n; row++ {
			if t.Cell[row] == 0 || t.Productive[row] != rep.True {
				continue
			}
			cell := uint64(t.Sample[row])<<32 | uint64(t.Cell[row])
			k := [2]uint64{cell, uint64(t.Locus[row])}
			chains[k]++
			if chains[k] > f.MaxChains {
				crowded[cell] = true
			}
		}
	}
	for row := 0; row < n; row++ {
		s := t.Sample[row]
		reason := ""
		switch {
		case excluded[t.Samples[s].Name]:
			reason = ReasonSample
		case len(loci) > 0 && !loci[t.Locus[row]]:
			reason = ReasonLocus
		case f.CellsOnly && t.IsCell != nil && t.IsCell[row] == rep.False:
			reason = ReasonNotCell
		case f.HighConfidenceOnly && t.HighConfidence != nil && t.HighConfidence[row] == rep.False:
			reason = ReasonLowConfidence
		case f.ProductiveOnly && t.Productive[row] != rep.True:
			reason = ReasonNotProductive
		case f.MinCount > 0 && t.Cell == nil && !(t.Count[row] >= f.MinCount):
			reason = ReasonBelowCount
		case crowded != nil && t.Cell[row] != 0 && crowded[uint64(s)<<32|uint64(t.Cell[row])]:
			reason = ReasonManyChains
		}
		if reason == "" {
			keep[row] = true
		} else {
			removed[s][reason]++
		}
	}
	return keep, removed
}

// Length is a junction length's count of clonotypes and their abundance.
type Length struct {
	Length     int     `json:"length"`
	Clonotypes int     `json:"clonotypes"`
	Abundance  float64 `json:"abundance"`
}

// Chains is a single-cell sample's cells by the productive chains they carry.
type Chains struct {
	Cells int `json:"cells"`
	// Configurations: cells by their chains, as "TRA 1, TRB 1" or "IGH 1, light 2" (counts of 3 or
	// more written "3+").
	Configurations map[string]int `json:"configurations"`
	// Paired: cells with a productive chain of each arm (TRA and TRB, TRG and TRD, IGH and IGK or
	// IGL).
	Paired int `json:"paired"`
	// Single: cells with a productive chain of one arm only.
	Single int `json:"single"`
	// None: barcodes whose contigs are all not productive.
	None int `json:"none"`
	// Multi: cells with more than two productive chains of a locus (possible doublets).
	Multi int `json:"multi"`
	// ByLocus: cells with at least one productive chain of each locus.
	ByLocus map[string]int `json:"byLocus"`
	// UMIs per productive contig: quartiles.
	UMIs *Quartiles `json:"umis,omitempty"`
}

// Quartiles of a distribution.
type Quartiles struct {
	Q1     float64 `json:"q1"`
	Median float64 `json:"median"`
	Q3     float64 `json:"q3"`
}

// Sample is one sample's QC.
type Sample struct {
	Name string `json:"name"`
	Rows int    `json:"rows"`
	Kept int    `json:"kept"`
	// Removed: rows each filter rule left out.
	Removed map[string]int `json:"removed"`
	// Abundance of the rows kept, in the dataset's unit (for single-cell data, cells).
	Abundance float64 `json:"abundance"`
	// Productive, out of frame, with a stop codon, not productive for a reason not given, or with
	// no productive status at all: shares of the abundance (contigs for single-cell data) of every
	// row before the filter's productive rule.
	Productive float64 `json:"productive"`
	OutOfFrame float64 `json:"outOfFrame"`
	Stop       float64 `json:"stop"`
	Unknown    float64 `json:"unknown"`
	NotGiven   float64 `json:"notGiven"`
	// Clonotypes: distinct productive (locus, amino-acid junction, V gene, J gene) among the rows
	// kept (bulk), with their singletons and doubletons, sample coverage and the largest one's
	// share.
	Clonotypes int     `json:"clonotypes"`
	Singletons int     `json:"singletons"`
	Doubletons int     `json:"doubletons"`
	Coverage   Number  `json:"coverage"`
	TopShare   float64 `json:"topShare"`
	// VGene, JGene: the share of productive abundance whose call names one gene; VFamily, one
	// family.
	VGene   float64 `json:"vGene"`
	VFamily float64 `json:"vFamily"`
	JGene   float64 `json:"jGene"`
	// ReadsPer: reads per unit (template, UMI) where the data give both.
	ReadsPer Number             `json:"readsPer"`
	Loci     map[string]float64 `json:"loci"`
	Lengths  []Length           `json:"lengths"`
	Chains   *Chains            `json:"chains,omitempty"`
}

// Report is a dataset's QC under a filter.
type Report struct {
	Unit       rep.Unit `json:"unit"`
	SingleCell bool     `json:"singleCell"`
	Filter     Filter   `json:"filter"`
	// IntegerCounts: whether every count is a whole number (coverage needs it).
	IntegerCounts bool `json:"integerCounts"`
	// NoCoverage says why sample coverage is not estimated, when it is not (bulk data).
	NoCoverage string   `json:"noCoverage,omitempty"`
	Samples    []Sample `json:"samples"`
	// Definition is how QC counts clonotypes.
	Definition string `json:"definition"`
}

// Definition of the clonotypes QC counts (bulk data).
const Definition = "productive rearrangements by locus, amino-acid junction, V gene and J gene (alleles dropped)"

// Compute measures every sample's QC under a filter.
func Compute(t *rep.Table, f Filter) Report {
	keep, removed := Apply(t, f)
	// A sample's productive share is measured on the rows the rest of the filter keeps.
	statusKeep := keep
	if f.ProductiveOnly {
		g := f
		g.ProductiveOnly = false
		statusKeep, _ = Apply(t, g)
	}
	r := Report{Unit: t.Unit, SingleCell: t.Cell != nil, Filter: f, Definition: Definition, IntegerCounts: true}
	n := t.Len()
	samples := make([]Sample, len(t.Samples))
	for i, s := range t.Samples {
		samples[i] = Sample{Name: s.Name, Removed: removed[i], Loci: map[string]float64{}, Coverage: Number(math.NaN()), ReadsPer: Number(math.NaN())}
	}
	// Gene calls resolved, by Dict code.
	type resolution struct{ gene, family bool }
	resolved := map[uint32]resolution{}
	resolve := func(code uint32) resolution {
		if v, ok := resolved[code]; ok {
			return v
		}
		c := t.Call(code)
		v := resolution{gene: c.Resolved(genes.LevelGene), family: c.Resolved(genes.LevelFamily)}
		resolved[code] = v
		return v
	}
	geneKey := map[uint32]string{}
	key := func(code uint32) string {
		if k, ok := geneKey[code]; ok {
			return k
		}
		k := t.Call(code).Key(genes.LevelGene)
		geneKey[code] = k
		return k
	}
	type tally struct {
		status                            [5]float64 // productive, out of frame, stop, unknown, not given
		productive, vGene, vFamily, jGene float64
		readsWithUnit, unitWithReads      float64
		clonotypes                        map[string]float64
		lengths                           map[int]*Length
		lengthSeen                        map[string]bool
	}
	tallies := make([]tally, len(samples))
	for i := range tallies {
		tallies[i] = tally{clonotypes: map[string]float64{}, lengths: map[int]*Length{}, lengthSeen: map[string]bool{}}
	}
	single := t.Cell != nil
	for row := 0; row < n; row++ {
		s := t.Sample[row]
		ss, tl := &samples[s], &tallies[s]
		ss.Rows++
		// Contigs count once each in single-cell data; a bulk row without a count adds nothing to
		// abundance (missing is not one), though it is counted among the rows.
		w := t.Count[row]
		switch {
		case single:
			w = 1
		case math.IsNaN(w):
			w = 0
		}
		if !single && t.Count[row] != math.Trunc(t.Count[row]) && !math.IsNaN(t.Count[row]) {
			r.IntegerCounts = false
		}
		// Status before the filter's productive rule: from the flags, or the junction.
		status := 4
		switch {
		case t.Productive[row] == rep.True:
			status = 0
		case t.VJInFrame[row] == rep.False:
			status = 1
		case t.StopCodon[row] == rep.True:
			status = 2
		case t.Productive[row] == rep.False:
			status = statusFromJunction(t, row)
		}
		if statusKeep[row] {
			tl.status[status] += w
		}
		if !keep[row] {
			continue
		}
		ss.Kept++
		if !single {
			ss.Abundance += w
		}
		if l := t.LocusName(row); l != "" {
			ss.Loci[l] += w
		}
		if !single && t.Reads != nil && !math.IsNaN(t.Reads[row]) && !math.IsNaN(t.Count[row]) && t.Unit != rep.Reads {
			tl.readsWithUnit += t.Reads[row]
			tl.unitWithReads += t.Count[row]
		}
		if t.Productive[row] != rep.True || t.JunctionAA[row] == 0 || w == 0 {
			continue
		}
		tl.productive += w
		if t.V[row] != 0 {
			v := resolve(t.V[row])
			if v.gene {
				tl.vGene += w
			}
			if v.family {
				tl.vFamily += w
			}
		}
		if t.J[row] != 0 && resolve(t.J[row]).gene {
			tl.jGene += w
		}
		junction := t.JunctionsAA.Get(t.JunctionAA[row])
		k := t.LocusName(row) + "\x00" + junction + "\x00" + key(t.V[row]) + "\x00" + key(t.J[row])
		tl.clonotypes[k] += w
		length := tl.lengths[len(junction)]
		if length == nil {
			length = &Length{Length: len(junction)}
			tl.lengths[len(junction)] = length
		}
		length.Abundance += w
		if !tl.lengthSeen[k] {
			tl.lengthSeen[k] = true
			length.Clonotypes++
		}
	}
	for i := range samples {
		ss, tl := &samples[i], &tallies[i]
		total := tl.status[0] + tl.status[1] + tl.status[2] + tl.status[3] + tl.status[4]
		if total > 0 {
			ss.Productive, ss.OutOfFrame, ss.Stop, ss.Unknown, ss.NotGiven = tl.status[0]/total, tl.status[1]/total, tl.status[2]/total, tl.status[3]/total, tl.status[4]/total
		}
		if tl.productive > 0 {
			ss.VGene, ss.VFamily, ss.JGene = tl.vGene/tl.productive, tl.vFamily/tl.productive, tl.jGene/tl.productive
		}
		if tl.unitWithReads > 0 {
			ss.ReadsPer = Number(tl.readsWithUnit / tl.unitWithReads)
		}
		ss.Clonotypes = len(tl.clonotypes)
		var sum, top float64
		// Sorted keys: sums taken in one order on every computer.
		keys := make([]string, 0, len(tl.clonotypes))
		for k := range tl.clonotypes {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v := tl.clonotypes[k]
			sum += v
			top = math.Max(top, v)
			switch v {
			case 1:
				ss.Singletons++
			case 2:
				ss.Doubletons++
			}
		}
		if sum > 0 {
			ss.TopShare = top / sum
		}
		// Coverage is estimated from the singletons and doubletons, which a minimum count above
		// one leaves out: it would always be 1.
		if !single && r.IntegerCounts && f.MinCount <= 1 && sum > 0 {
			ss.Coverage = Number(Coverage(sum, float64(ss.Singletons), float64(ss.Doubletons)))
		}
		for _, l := range tl.lengths {
			ss.Lengths = append(ss.Lengths, *l)
		}
		sort.Slice(ss.Lengths, func(a, b int) bool { return ss.Lengths[a].Length < ss.Lengths[b].Length })
		if ss.Lengths == nil {
			ss.Lengths = []Length{}
		}
	}
	if single {
		cellChains(t, keep, samples)
	}
	switch {
	case single:
	case f.MinCount > 1:
		r.NoCoverage = fmt.Sprintf("Sample coverage is not estimated: it is estimated from the clonotypes seen once and twice, and the filter leaves out rearrangements counted fewer than %g times.", f.MinCount)
	case !r.IntegerCounts:
		r.NoCoverage = "Some counts are not whole numbers, so sample coverage is not estimated."
	}
	r.Samples = samples
	return r
}

// statusFromJunction: a non-productive row's kind from its junction, when its flags do not say:
// out of frame when its length is not a multiple of three, a stop codon when it translates to one,
// else unknown.
func statusFromJunction(t *rep.Table, row int) int {
	if nt := t.Junction[row]; nt != 0 {
		j := t.Junctions.Get(nt)
		if len(j)%3 != 0 {
			return 1
		}
		if seq.HasStop(seq.Translate(j)) {
			return 2
		}
	}
	if aa := t.JunctionAA[row]; aa != 0 && seq.HasStop(t.JunctionsAA.Get(aa)) {
		return 2
	}
	return 3
}

// Arms of the receptors: a cell is paired when it has a productive chain of each.
var arms = [][2][]string{
	{{"TRA"}, {"TRB"}},
	{{"TRG"}, {"TRD"}},
	{{"IGH"}, {"IGK", "IGL"}},
}

func cellChains(t *rep.Table, keep []bool, samples []Sample) {
	type cell struct {
		loci       map[string]int
		productive bool
	}
	cells := make([]map[uint32]*cell, len(samples))
	order := make([][]uint32, len(samples))
	umis := make([][]float64, len(samples))
	for i := range cells {
		cells[i] = map[uint32]*cell{}
	}
	for row := 0; row < t.Len(); row++ {
		if !keep[row] || t.Cell[row] == 0 {
			continue
		}
		s := t.Sample[row]
		c := cells[s][t.Cell[row]]
		if c == nil {
			c = &cell{loci: map[string]int{}}
			cells[s][t.Cell[row]] = c
			order[s] = append(order[s], t.Cell[row])
		}
		if t.Productive[row] != rep.True {
			continue
		}
		c.productive = true
		if l := t.LocusName(row); l != "" {
			c.loci[l]++
		}
		if t.UMIs != nil && !math.IsNaN(t.UMIs[row]) {
			umis[s] = append(umis[s], t.UMIs[row])
		}
	}
	for i := range samples {
		ch := &Chains{Configurations: map[string]int{}, ByLocus: map[string]int{}}
		for _, id := range order[i] {
			c := cells[i][id]
			ch.Cells++
			if !c.productive {
				ch.None++
				continue
			}
			multi := false
			for l, k := range c.loci {
				ch.ByLocus[l]++
				multi = multi || k > 2
			}
			if multi {
				ch.Multi++
			}
			paired, any := false, false
			var parts []string
			for _, a := range arms {
				left, right := 0, 0
				for _, l := range a[0] {
					left += c.loci[l]
				}
				for _, l := range a[1] {
					right += c.loci[l]
				}
				if left == 0 && right == 0 {
					continue
				}
				any = true
				paired = paired || (left > 0 && right > 0)
				rightName := a[1][0]
				if len(a[1]) > 1 {
					rightName = "light"
				}
				parts = append(parts, a[0][0]+" "+atMost3(left), rightName+" "+atMost3(right))
			}
			switch {
			case paired:
				ch.Paired++
			case any:
				ch.Single++
			}
			ch.Configurations[joinParts(parts)]++
		}
		ch.UMIs = quartiles(umis[i])
		samples[i].Chains = ch
		samples[i].Abundance = float64(ch.Cells - ch.None)
	}
}

func atMost3(k int) string {
	switch {
	case k >= 3:
		return "3+"
	default:
		return string(rune('0' + k))
	}
}

func joinParts(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}

// quartiles by linear interpolation between order statistics (R's type 7, numpy's default).
func quartiles(values []float64) *Quartiles {
	if len(values) == 0 {
		return nil
	}
	v := append([]float64(nil), values...)
	sort.Float64s(v)
	at := func(p float64) float64 {
		h := p * float64(len(v)-1)
		lo := math.Floor(h)
		if int(lo) >= len(v)-1 {
			return v[len(v)-1]
		}
		return v[int(lo)] + float64(h-lo)*(v[int(lo)+1]-v[int(lo)])
	}
	return &Quartiles{Q1: at(0.25), Median: at(0.5), Q3: at(0.75)}
}

// Number is a float64 written in JSON as null when it is not a number (a coverage that cannot be
// estimated): JSON has no NaN.
type Number float64

func (n Number) MarshalJSON() ([]byte, error) {
	f := float64(n)
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return []byte("null"), nil
	}
	return json.Marshal(f)
}

func (n *Number) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		*n = Number(math.NaN())
		return nil
	}
	var f float64
	if err := json.Unmarshal(b, &f); err != nil {
		return err
	}
	*n = Number(f)
	return nil
}

// Describe is the filter in words ("none" when it keeps every row).
func (f Filter) Describe() string {
	var parts []string
	if f.ProductiveOnly {
		parts = append(parts, "productive only")
	}
	if f.MinCount > 0 {
		parts = append(parts, fmt.Sprintf("counted at least %g times", f.MinCount))
	}
	if len(f.Loci) > 0 {
		parts = append(parts, "loci "+strings.Join(f.Loci, ", "))
	}
	if f.CellsOnly {
		parts = append(parts, "cells only")
	}
	if f.HighConfidenceOnly {
		parts = append(parts, "high-confidence contigs only")
	}
	if f.MaxChains > 0 {
		parts = append(parts, fmt.Sprintf("cells with at most %d productive chains of a locus", f.MaxChains))
	}
	if len(f.ExcludeSamples) > 0 {
		parts = append(parts, "samples left out: "+strings.Join(f.ExcludeSamples, ", "))
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, "; ")
}
