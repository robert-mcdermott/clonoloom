// Package usage measures what each sample's clonotypes are made of (clonoloom-spec/requirements.md,
// R3): the V, D, J and C genes they use, and their V–J pairs, at the gene and the family level;
// their junctions' lengths in amino acids (the spectratype) and mean length; and their junctions'
// amino acids, overall and, for the lengths most clonotypes have, by IMGT position. Each is a share
// of the sample's clonotypes (each counted once) and of its abundance.
//
// A clonotype's genes and junction are those of its rearrangements in the sample, in proportion to
// their counts: a call that ties several genes gives each an equal part, and a rearrangement
// without a call counts as not called. The shares of genes, lengths and amino acids, and the mean
// length, have intervals from a bootstrap of the sample's clonotypes (each replicate S clonotypes
// drawn with replacement from the sample's S), the same in every run. A group of samples (a
// condition) is summarized by the mean of its samples' shares, each sample counted once, with
// their lowest and highest. clonoloom-spec/research.md §5.5 has the choices and their sources.
package usage

import (
	"math"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"clonoloom/internal/clonotype"
	"clonoloom/internal/diversity"
	"clonoloom/internal/genes"
	"clonoloom/internal/random"
	"clonoloom/internal/rep"
	"clonoloom/internal/seq"
)

// Segments and Levels are those whose usage is measured.
var (
	Segments = []string{"V", "D", "J", "C"}
	Levels   = []string{"gene", "family"}
)

// AminoAcids are the twenty, in the order composition lists them; any other residue is counted as
// X.
const AminoAcids = "ACDEFGHIKLMNPQRSTVWY"

// Profiled lengths: those of at least this share of a locus's clonotypes over the samples, at most
// this many.
const (
	ProfileShare   = 0.05
	ProfileLengths = 6
)

// Number and Pair are as the diversity package writes them: a value that may be missing (null),
// and an interval.
type (
	Number = diversity.Number
	Pair   = [2]Number
)

// Share is a gene's (a length's, an amino acid's) share of a sample's clonotypes and of its
// abundance, each with its interval where one was computed. For a group, the share is the mean of
// its samples' shares and the interval their lowest and highest. Name "" is the share not called.
type Share struct {
	Name               string `json:"name"`
	Clonotypes         Number `json:"clonotypes"`
	Abundance          Number `json:"abundance"`
	ClonotypesInterval *Pair  `json:"clonotypesInterval,omitempty"`
	AbundanceInterval  *Pair  `json:"abundanceInterval,omitempty"`
}

// Genes are the shares of one segment's genes at one level: V, D, J or C, or VJ for V–J pairs
// (named "V+J", a part not called left empty, and without intervals).
type Genes struct {
	Segment string  `json:"segment"`
	Level   string  `json:"level"`
	Shares  []Share `json:"shares"`
}

// Profile is the amino acids at each IMGT position of the junctions of one length (without
// intervals).
type Profile struct {
	Length    int        `json:"length"`
	Positions []Position `json:"positions"`
}

type Position struct {
	IMGT   string  `json:"imgt"`
	Shares []Share `json:"shares"`
}

// Locus is what a sample's (or group's) clonotypes with a chain of one locus are made of.
// MeanLength is not a share but the mean junction length in amino acids.
type Locus struct {
	Locus       string    `json:"locus"`
	Clonotypes  int       `json:"clonotypes"`
	Abundance   float64   `json:"abundance"`
	Genes       []Genes   `json:"genes"`
	Lengths     []Share   `json:"lengths"`
	MeanLength  Share     `json:"meanLength"`
	Composition []Share   `json:"composition"`
	Profiles    []Profile `json:"profiles"`
}

type Sample struct {
	Name       string  `json:"name"`
	Clonotypes int     `json:"clonotypes"`
	Abundance  float64 `json:"abundance"`
	Loci       []Locus `json:"loci"`
}

// Group is a group of samples, summarized: Clonotypes and Abundance its samples' totals.
type Group struct {
	Name    string   `json:"name"`
	Samples []string `json:"samples"`
	Loci    []Locus  `json:"loci"`
}

// Result is every sample's, every group's, and the lengths profiled by locus.
type Result struct {
	Samples    []Sample         `json:"samples"`
	Groups     []Group          `json:"groups"`
	Profiled   map[string][]int `json:"profiled"`
	Replicates int              `json:"replicates"`
}

// Measure measures every sample of a clonotype table (of rt's rows). groups names each sample's
// group (by sample name; samples without one are in none). opt is the bootstrap's, each sample its
// own stream (opt.Stream + its index).
func Measure(rt *rep.Table, t *clonotype.Table, groups map[string]string, opt diversity.Options) Result {
	if opt.Confidence <= 0 || opt.Confidence >= 1 {
		opt.Confidence = 0.95
	}
	z := diversity.NormalQuantile(0.5 + opt.Confidence/2)
	aa := junctionsAA(rt)
	bySample := rowsBySample(rt, t)
	res := Result{Profiled: profiled(rt, t, bySample, aa), Replicates: max(opt.Replicates, 0)}
	if opt.Replicates < 2 {
		res.Replicates = 0
	}
	for s := range rt.Samples {
		o := opt
		o.Stream = opt.Stream + uint64(s)
		res.Samples = append(res.Samples, measureSample(rt, t, uint32(s), bySample[s], aa, res.Profiled, o, z))
	}
	res.Groups = summarize(res.Samples, groups)
	return res
}

// junctionsAA answers each distinct row junction's amino acids: its own, else its nucleotides
// translated where they are in frame.
func junctionsAA(rt *rep.Table) func(row int) string {
	var mu sync.Mutex
	translated := map[uint32]string{}
	return func(row int) string {
		if s := rt.JunctionsAA.Get(rt.JunctionAA[row]); s != "" {
			return s
		}
		code := rt.Junction[row]
		mu.Lock()
		defer mu.Unlock()
		if s, ok := translated[code]; ok {
			return s
		}
		nt := rt.Junctions.Get(code)
		s := ""
		if nt != "" && len(nt)%3 == 0 {
			s = seq.Translate(strings.ToUpper(nt))
		}
		translated[code] = s
		return s
	}
}

// member is a clonotype's rows in one sample.
type member struct {
	clonotype int
	abundance float64
	rows      []uint32
}

// rowsBySample answers each sample's clonotypes with their rows there.
func rowsBySample(rt *rep.Table, t *clonotype.Table) [][]member {
	out := make([][]member, len(rt.Samples))
	for i := range t.Clonotypes {
		samples, values := t.Abundances(i)
		rows := t.MembersOf(i)
		for k, s := range samples {
			m := member{clonotype: i, abundance: values[k]}
			for _, r := range rows {
				if rt.Sample[r] == s {
					m.rows = append(m.rows, r)
				}
			}
			out[s] = append(out[s], m)
		}
	}
	return out
}

// rowWeights are a clonotype's rows' parts of it in a sample, by locus: each row's count over the
// locus's total (a row without a count counts one).
func rowWeights(rt *rep.Table, rows []uint32) map[string][]float64 {
	w := map[string][]float64{}
	total := map[string]float64{}
	for k, r := range rows {
		l := rt.LocusName(int(r))
		c := rt.Count[r]
		if math.IsNaN(c) || c <= 0 {
			c = 1
		}
		if w[l] == nil {
			w[l] = make([]float64, len(rows))
		}
		w[l][k] = c
		total[l] += c
	}
	for l, ws := range w {
		for k := range ws {
			ws[k] /= total[l]
		}
	}
	return w
}

// profiled answers, by locus, the junction lengths of at least ProfileShare of its clonotypes over
// the samples (each clonotype counted once in each sample), at most ProfileLengths, ascending.
func profiled(rt *rep.Table, t *clonotype.Table, bySample [][]member, aa func(int) string) map[string][]int {
	counts := map[string]map[int]float64{}
	totals := map[string]float64{}
	for _, members := range bySample {
		for _, m := range members {
			for l, ws := range rowWeights(rt, m.rows) {
				for k, r := range m.rows {
					if ws[k] == 0 {
						continue
					}
					j := aa(int(r))
					if j == "" {
						continue
					}
					if counts[l] == nil {
						counts[l] = map[int]float64{}
					}
					counts[l][len(j)] += ws[k]
					totals[l] += ws[k]
				}
			}
		}
	}
	out := map[string][]int{}
	for l, c := range counts {
		var lengths []int
		for n, v := range c {
			if v >= ProfileShare*totals[l] {
				lengths = append(lengths, n)
			}
		}
		sort.Slice(lengths, func(a, b int) bool {
			if c[lengths[a]] != c[lengths[b]] {
				return c[lengths[a]] > c[lengths[b]]
			}
			return lengths[a] < lengths[b]
		})
		if len(lengths) > ProfileLengths {
			lengths = lengths[:ProfileLengths]
		}
		sort.Ints(lengths)
		if l != "" {
			out[l] = lengths
		}
	}
	return out
}

// Feature kinds: genes, V–J pairs, lengths, the mean length, amino acids, amino acids by position,
// and the denominators: clonotypes with the locus (genes and pairs), with an amino-acid junction
// (lengths, mean), residues (composition), and clonotypes of a profiled length (positions).
const (
	kindGene = iota
	kindPair
	kindLength
	kindMean
	kindAA
	kindPosition
	denLocus
	denJunction
	denResidues
	denProfile
)

type featureKey struct {
	locus  string
	kind   int
	seg    string
	level  string
	name   string
	length int
	pos    int
}

type feature struct {
	key featureKey
	den int // the denominator's index; -1 for a denominator
}

// tracked: the kinds with intervals, and the denominators they need.
func tracked(kind int) bool {
	switch kind {
	case kindGene, kindLength, kindMean, kindAA, denLocus, denJunction, denResidues:
		return true
	}
	return false
}

// sampleFeatures collects a sample's features and each clonotype's values of them.
type sampleFeatures struct {
	features []feature
	index    map[featureKey]int
	items    []item
}

// item is a clonotype's values: all of them (for the estimates) and those tracked (for the
// bootstrap).
type item struct {
	abundance  float64
	idx        []int32
	val        []float64
	trackedIdx []int32
	trackedVal []float64
}

func (sf *sampleFeatures) id(k featureKey, den int) int {
	if i, ok := sf.index[k]; ok {
		return i
	}
	i := len(sf.features)
	sf.index[k] = i
	sf.features = append(sf.features, feature{key: k, den: den})
	return i
}

func measureSample(rt *rep.Table, t *clonotype.Table, sample uint32, members []member, aa func(int) string, prof map[string][]int, opt diversity.Options, z float64) Sample {
	out := Sample{Name: rt.Samples[sample].Name, Clonotypes: len(members)}
	sf := &sampleFeatures{index: map[featureKey]int{}}
	codes := map[string][]uint32{"V": rt.V, "D": rt.D, "J": rt.J, "C": rt.C}
	levelOf := map[string]genes.Level{"gene": genes.LevelGene, "family": genes.LevelFamily}
	names := func(code uint32, level string) []string {
		if code == 0 {
			return []string{""}
		}
		c := rt.Call(code)
		if len(c.Names) == 0 {
			return []string{""}
		}
		return strings.Split(c.Key(levelOf[level]), "|")
	}
	values := map[int]float64{}
	add := func(k featureKey, den int, v float64) {
		if v != 0 {
			values[sf.id(k, den)] += v
		}
	}
	for _, m := range members {
		out.Abundance += m.abundance
		clear(values)
		for l, ws := range rowWeights(rt, m.rows) {
			nLocus := sf.id(featureKey{locus: l, kind: denLocus}, -1)
			nJunction := sf.id(featureKey{locus: l, kind: denJunction}, -1)
			nResidues := sf.id(featureKey{locus: l, kind: denResidues}, -1)
			for k, r := range m.rows {
				w := ws[k]
				if w == 0 {
					continue
				}
				values[nLocus] += w
				for _, seg := range Segments {
					col := codes[seg]
					var code uint32
					if col != nil {
						code = col[r]
					}
					for _, level := range Levels {
						ns := names(code, level)
						for _, n := range ns {
							add(featureKey{locus: l, kind: kindGene, seg: seg, level: level, name: n}, nLocus, w/float64(len(ns)))
						}
					}
				}
				for _, level := range Levels {
					vs, js := names(rt.V[r], level), names(rt.J[r], level)
					for _, v := range vs {
						for _, j := range js {
							add(featureKey{locus: l, kind: kindPair, seg: "VJ", level: level, name: v + "+" + j}, nLocus, w/float64(len(vs)*len(js)))
						}
					}
				}
				junction := aa(int(r))
				if junction == "" {
					continue
				}
				n := len(junction)
				values[nJunction] += w
				values[nResidues] += w * float64(n)
				add(featureKey{locus: l, kind: kindLength, length: n}, nJunction, w)
				add(featureKey{locus: l, kind: kindMean}, nJunction, w*float64(n))
				for i := 0; i < n; i++ {
					add(featureKey{locus: l, kind: kindAA, name: residue(junction[i])}, nResidues, w)
				}
				if containsInt(prof[l], n) {
					np := sf.id(featureKey{locus: l, kind: denProfile, length: n}, -1)
					values[np] += w
					for i := 0; i < n; i++ {
						add(featureKey{locus: l, kind: kindPosition, length: n, pos: i, name: residue(junction[i])}, np, w)
					}
				}
			}
		}
		it := item{abundance: m.abundance}
		keys := make([]int, 0, len(values))
		for k := range values {
			keys = append(keys, k)
		}
		sort.Ints(keys)
		for _, k := range keys {
			it.idx = append(it.idx, int32(k))
			it.val = append(it.val, values[k])
			if tracked(sf.features[k].key.kind) {
				it.trackedIdx = append(it.trackedIdx, int32(k))
				it.trackedVal = append(it.trackedVal, values[k])
			}
		}
		sf.items = append(sf.items, it)
	}
	// The estimates.
	nf := len(sf.features)
	cl, ab := make([]float64, nf), make([]float64, nf)
	for _, it := range sf.items {
		for k, i := range it.idx {
			cl[i] += it.val[k]
			ab[i] += it.abundance * it.val[k]
		}
	}
	share := func(t []float64, i int) float64 {
		f := sf.features[i]
		if f.den < 0 || t[f.den] == 0 {
			return math.NaN()
		}
		return t[i] / t[f.den]
	}
	// The bootstrap: each tracked feature's shares (by clonotypes, by abundance) in each replicate.
	var withInterval []int
	for i, f := range sf.features {
		if f.den >= 0 && tracked(f.key.kind) {
			withInterval = append(withInterval, i)
		}
	}
	se := map[int][2]float64{}
	if opt.Replicates > 1 && len(sf.items) > 1 {
		rows := replicate(sf, withInterval, opt)
		for k, i := range withInterval {
			se[i] = [2]float64{spread(rows, 2*k), spread(rows, 2*k+1)}
		}
	}
	// The shares, by locus.
	loci := map[string]*Locus{}
	locusOf := func(l string) *Locus {
		if loci[l] == nil {
			loci[l] = &Locus{Locus: l}
		}
		return loci[l]
	}
	genesOf := map[[3]string][]Share{}
	lengths := map[string][]Share{}
	composition := map[string][]Share{}
	positions := map[string]map[[2]int][]Share{}
	for i, f := range sf.features {
		k := f.key
		if f.den < 0 {
			if k.kind == denLocus {
				locusOf(k.locus).Clonotypes = int(math.Round(cl[i]))
				locusOf(k.locus).Abundance = ab[i]
			}
			continue
		}
		s := Share{Name: k.name, Clonotypes: Number(share(cl, i)), Abundance: Number(share(ab, i))}
		if e, ok := se[i]; ok {
			hi := 1.0
			if k.kind == kindMean {
				hi = math.Inf(1)
			}
			s.ClonotypesInterval = interval(float64(s.Clonotypes), e[0], z, hi)
			s.AbundanceInterval = interval(float64(s.Abundance), e[1], z, hi)
		}
		switch k.kind {
		case kindGene, kindPair:
			key := [3]string{k.locus, k.seg, k.level}
			genesOf[key] = append(genesOf[key], s)
		case kindLength:
			s.Name = strconv.Itoa(k.length)
			lengths[k.locus] = append(lengths[k.locus], s)
		case kindMean:
			s.Name = "mean"
			locusOf(k.locus).MeanLength = s
		case kindAA:
			composition[k.locus] = append(composition[k.locus], s)
		case kindPosition:
			if positions[k.locus] == nil {
				positions[k.locus] = map[[2]int][]Share{}
			}
			pk := [2]int{k.length, k.pos}
			positions[k.locus][pk] = append(positions[k.locus][pk], s)
		}
	}
	for key, shares := range genesOf {
		sortShares(shares, geneLess)
		l := locusOf(key[0])
		l.Genes = append(l.Genes, Genes{Segment: key[1], Level: key[2], Shares: shares})
	}
	for l, shares := range lengths {
		sortShares(shares, numberLess)
		locusOf(l).Lengths = shares
	}
	for l, shares := range composition {
		sortShares(shares, aminoLess)
		locusOf(l).Composition = shares
	}
	for l, byPos := range positions {
		for _, n := range prof[l] {
			labels := IMGTPositions(n)
			p := Profile{Length: n}
			for pos, label := range labels {
				shares := byPos[[2]int{n, pos}]
				sortShares(shares, aminoLess)
				p.Positions = append(p.Positions, Position{IMGT: label, Shares: shares})
			}
			locusOf(l).Profiles = append(locusOf(l).Profiles, p)
		}
	}
	for _, l := range loci {
		if l.Locus == "" {
			continue
		}
		sort.Slice(l.Genes, func(a, b int) bool { return genesOrder(l.Genes[a]) < genesOrder(l.Genes[b]) })
		out.Loci = append(out.Loci, *l)
	}
	sort.Slice(out.Loci, func(a, b int) bool { return out.Loci[a].Locus < out.Loci[b].Locus })
	return out
}

// replicate draws the bootstrap's replicates: in each, S clonotypes drawn with replacement from the
// sample's S (a multinomial), each tracked feature's shares computed again. Each replicate has its
// own stream and its own slot, so the result is the same with any number of processors.
func replicate(sf *sampleFeatures, withInterval []int, opt diversity.Options) [][]float64 {
	n := len(sf.items)
	chances := make([]float64, n)
	for i := range chances {
		chances[i] = 1 / float64(n)
	}
	nf := len(sf.features)
	out := make([][]float64, opt.Replicates)
	work := make(chan int)
	var wg sync.WaitGroup
	for range max(1, runtime.GOMAXPROCS(0)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var counts []int
			cl, ab := make([]float64, nf), make([]float64, nf)
			for b := range work {
				r := random.New(opt.Seed, opt.Stream<<24|uint64(b))
				counts = r.Multinomial(n, chances, counts)
				clear(cl)
				clear(ab)
				for i, it := range sf.items {
					m := float64(counts[i])
					if m == 0 {
						continue
					}
					for k, f := range it.trackedIdx {
						cl[f] += m * it.trackedVal[k]
						ab[f] += m * it.abundance * it.trackedVal[k]
					}
				}
				row := make([]float64, 2*len(withInterval))
				for k, i := range withInterval {
					d := sf.features[i].den
					row[2*k], row[2*k+1] = cl[i]/cl[d], ab[i]/ab[d]
				}
				out[b] = row
			}
		}()
	}
	for b := range opt.Replicates {
		work <- b
	}
	close(work)
	wg.Wait()
	return out
}

// spread is a column's standard error over the replicates, those not finite left out.
func spread(rows [][]float64, k int) float64 {
	var sum, n float64
	for _, r := range rows {
		if x := r[k]; !math.IsNaN(x) && !math.IsInf(x, 0) {
			sum += x
			n++
		}
	}
	if n < 2 {
		return math.NaN()
	}
	mean := sum / n
	var ss float64
	for _, r := range rows {
		if x := r[k]; !math.IsNaN(x) && !math.IsInf(x, 0) {
			ss += (x - mean) * (x - mean)
		}
	}
	return math.Sqrt(ss / (n - 1))
}

// interval is the estimate ± z SE, within [0, hi].
func interval(estimate, se, z, hi float64) *Pair {
	if math.IsNaN(estimate) || math.IsNaN(se) {
		return nil
	}
	return &Pair{Number(min(max(estimate-z*se, 0), hi)), Number(max(min(estimate+z*se, hi), 0))}
}

func residue(c byte) string {
	if strings.IndexByte(AminoAcids, c) >= 0 {
		return string(c)
	}
	return "X"
}

func containsInt(list []int, v int) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// IMGTPositions are the IMGT unique numbering's positions of a junction of n amino acids
// (Lefranc et al. 2003): 104 and 118 at its ends, and between them the CDR3, numbered from 105 and
// from 117 toward its middle. A CDR3 shorter than 13 lacks positions from the middle (111, 112,
// 110, 113, …); a longer one has insertions there (111.1, 112.1, 111.2, 112.2, …).
func IMGTPositions(n int) []string {
	if n < 2 {
		return nil
	}
	c := n - 2
	out := []string{"104"}
	if c <= 13 {
		left := (c + 1) / 2
		for p := 105; p < 105+left; p++ {
			out = append(out, strconv.Itoa(p))
		}
		for p := 118 - (c - left); p < 118; p++ {
			out = append(out, strconv.Itoa(p))
		}
	} else {
		for p := 105; p <= 111; p++ {
			out = append(out, strconv.Itoa(p))
		}
		extra := c - 13
		for k := 1; k <= (extra+1)/2; k++ {
			out = append(out, "111."+strconv.Itoa(k))
		}
		for k := extra / 2; k >= 1; k-- {
			out = append(out, "112."+strconv.Itoa(k))
		}
		for p := 112; p <= 117; p++ {
			out = append(out, strconv.Itoa(p))
		}
	}
	return append(out, "118")
}

// Orders: genes by their names' numbers (TRBV2 before TRBV10-1), not called last; lengths by
// number; amino acids as AminoAcids lists them, X last; segments V, D, J, C, then pairs, gene level
// before family.
func sortShares(s []Share, less func(a, b string) bool) {
	sort.Slice(s, func(a, b int) bool { return less(s[a].Name, s[b].Name) })
}

func geneLess(a, b string) bool {
	if (a == "") != (b == "") {
		return b == ""
	}
	return naturalLess(a, b)
}

// naturalLess compares names with their digit runs as numbers.
func naturalLess(a, b string) bool {
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		ca, cb := a[i], b[j]
		if isDigit(ca) && isDigit(cb) {
			si, sj := i, j
			for i < len(a) && isDigit(a[i]) {
				i++
			}
			for j < len(b) && isDigit(b[j]) {
				j++
			}
			na, _ := strconv.Atoi(a[si:i])
			nb, _ := strconv.Atoi(b[sj:j])
			if na != nb {
				return na < nb
			}
			continue
		}
		if ca != cb {
			return ca < cb
		}
		i++
		j++
	}
	return len(a)-i < len(b)-j
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func numberLess(a, b string) bool {
	x, _ := strconv.Atoi(a)
	y, _ := strconv.Atoi(b)
	return x < y
}

func aminoLess(a, b string) bool {
	ia, ib := strings.Index(AminoAcids+"X", a), strings.Index(AminoAcids+"X", b)
	return ia < ib
}

func genesOrder(g Genes) int {
	seg := map[string]int{"V": 0, "D": 1, "J": 2, "C": 3, "VJ": 4}[g.Segment]
	level := 0
	if g.Level == "family" {
		level = 1
	}
	return seg*2 + level
}

// summarize summarizes each group of samples: for every share, the mean over the group's samples
// with the locus (a sample without a gene sharing 0 of it), and their lowest and highest.
func summarize(samples []Sample, groups map[string]string) []Group {
	byName := map[string][]int{}
	var order []string
	for i, s := range samples {
		g := groups[s.Name]
		if g == "" {
			continue
		}
		if byName[g] == nil {
			order = append(order, g)
		}
		byName[g] = append(byName[g], i)
	}
	sort.Strings(order)
	var out []Group
	for _, name := range order {
		g := Group{Name: name}
		loci := map[string][]Locus{}
		for _, i := range byName[name] {
			g.Samples = append(g.Samples, samples[i].Name)
			for _, l := range samples[i].Loci {
				loci[l.Locus] = append(loci[l.Locus], l)
			}
		}
		var names []string
		for l := range loci {
			names = append(names, l)
		}
		sort.Strings(names)
		for _, l := range names {
			g.Loci = append(g.Loci, summarizeLocus(loci[l]))
		}
		out = append(out, g)
	}
	return out
}

func summarizeLocus(ls []Locus) Locus {
	out := Locus{Locus: ls[0].Locus}
	genes := map[[2]string][][]Share{}
	var genesKeys [][2]string
	var lengths, composition, means [][]Share
	profiles := map[int]map[string][][]Share{}
	labels := map[int][]string{}
	for _, l := range ls {
		out.Clonotypes += l.Clonotypes
		out.Abundance += l.Abundance
		for _, g := range l.Genes {
			k := [2]string{g.Segment, g.Level}
			if genes[k] == nil {
				genesKeys = append(genesKeys, k)
			}
			genes[k] = append(genes[k], g.Shares)
		}
		lengths = append(lengths, l.Lengths)
		composition = append(composition, l.Composition)
		means = append(means, []Share{l.MeanLength})
		for _, p := range l.Profiles {
			if profiles[p.Length] == nil {
				profiles[p.Length] = map[string][][]Share{}
				for _, pos := range p.Positions {
					labels[p.Length] = append(labels[p.Length], pos.IMGT)
				}
			}
			// A sample without junctions of the length says nothing of it.
			for _, pos := range p.Positions {
				if len(pos.Shares) > 0 {
					profiles[p.Length][pos.IMGT] = append(profiles[p.Length][pos.IMGT], pos.Shares)
				}
			}
		}
	}
	n := len(ls)
	for _, k := range genesKeys {
		less := geneLess
		out.Genes = append(out.Genes, Genes{Segment: k[0], Level: k[1], Shares: meanShares(genes[k], n, less)})
	}
	sort.Slice(out.Genes, func(a, b int) bool { return genesOrder(out.Genes[a]) < genesOrder(out.Genes[b]) })
	out.Lengths = meanShares(lengths, n, numberLess)
	out.Composition = meanShares(composition, n, aminoLess)
	if m := meanShares(means, n, numberLess); len(m) > 0 {
		out.MeanLength = m[0]
	}
	var ls2 []int
	for length := range profiles {
		ls2 = append(ls2, length)
	}
	sort.Ints(ls2)
	for _, length := range ls2 {
		p := Profile{Length: length}
		for _, label := range labels[length] {
			lists := profiles[length][label]
			p.Positions = append(p.Positions, Position{IMGT: label, Shares: meanShares(lists, len(lists), aminoLess)})
		}
		out.Profiles = append(out.Profiles, p)
	}
	return out
}

// meanShares is each name's mean share over n lists (a list without the name counting 0), with the
// lowest and highest as its interval.
func meanShares(lists [][]Share, n int, less func(a, b string) bool) []Share {
	type acc struct {
		sum, lo, hi [2]float64
		seen        int
	}
	byName := map[string]*acc{}
	var names []string
	for _, list := range lists {
		for _, s := range list {
			a := byName[s.Name]
			if a == nil {
				a = &acc{lo: [2]float64{math.Inf(1), math.Inf(1)}, hi: [2]float64{math.Inf(-1), math.Inf(-1)}}
				byName[s.Name] = a
				names = append(names, s.Name)
			}
			for k, v := range [2]float64{float64(s.Clonotypes), float64(s.Abundance)} {
				if math.IsNaN(v) {
					continue
				}
				a.sum[k] += v
				a.lo[k] = min(a.lo[k], v)
				a.hi[k] = max(a.hi[k], v)
			}
			a.seen++
		}
	}
	var out []Share
	for _, name := range names {
		a := byName[name]
		if a.seen < n {
			// The lists without it share none of it.
			for k := range 2 {
				a.lo[k] = min(a.lo[k], 0)
				a.hi[k] = max(a.hi[k], 0)
			}
		}
		s := Share{Name: name, Clonotypes: Number(a.sum[0] / float64(n)), Abundance: Number(a.sum[1] / float64(n)),
			ClonotypesInterval: &Pair{Number(a.lo[0]), Number(a.hi[0])}, AbundanceInterval: &Pair{Number(a.lo[1]), Number(a.hi[1])}}
		out = append(out, s)
	}
	sortShares(out, less)
	return out
}
