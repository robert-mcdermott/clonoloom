package tcrdist

import (
	"context"
	"errors"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"clonoloom/internal/clonotype"
)

// Set is a table's clonotypes as TCRdist compares them: each a receptor of its T-cell chains, in
// the definition's order, with its V genes' loops where they are used. Receptors are compared
// only with receptors of the same chains (a clonotype of a β chain alone, under a paired
// definition, with the others of a β chain alone).
type Set struct {
	// Organism whose loops are used ("" for the CDR3 alone).
	Organism string `json:"organism,omitempty"`
	// Loops: the V genes' loops are part of the distance.
	Loops bool `json:"loops"`
	// Groups: the receptors of each set of chains.
	Groups []Group `json:"groups"`
	// Clonotype: the clonotype (its index in the table) of each receptor, receptors grouped.
	Clonotype []int `json:"-"`
	// Excluded: clonotypes not compared, by reason.
	Excluded map[string]int `json:"excluded"`
	// Alleles: V calls read as an allele they do not name (a gene without its allele, or one of a
	// tie), and the allele whose loops were used.
	Alleles map[string]string `json:"alleles,omitempty"`

	group  []int32
	chains int // the most chains of a group
	// For receptor r and chain c (of its group): cdr3[c][r], its CDR3; v[c][r], its V allele's
	// index in loops[c] (-1 for the CDR3 alone).
	cdr3 [][][]uint8
	v    [][]int32
	// loop[c][a][b]: the loops' distance between V alleles a and b of chain c; alleles[c][a], the
	// allele's loops, encoded.
	loop    [][][]int
	alleles [][]Chain
}

// Group is the receptors of one set of chains.
type Group struct {
	Loci      []string `json:"loci"`
	Start     int      `json:"start"`
	Receptors int      `json:"receptors"`
}

// Reasons clonotypes are not compared.
const (
	ReasonNotTCell   = "not a T-cell receptor"
	ReasonNoJunction = "a chain without its junction"
	ReasonNoLoops    = "a V gene the loops' database lacks"
)

// tLoci are the T-cell receptor loci.
var tLoci = map[string]bool{"TRA": true, "TRB": true, "TRG": true, "TRD": true}

// NewSet makes the receptors of these clonotypes, whose chains' junctions are amino acids. With
// loops (nil for the CDR3 alone), each chain's V gene's loops are looked up in the organism's
// (guessed from the V genes when organism is "").
func NewSet(clonotypes []clonotype.Clonotype, loops *Loops, organism string) *Set {
	s := &Set{Excluded: map[string]int{}, Alleles: map[string]string{}}
	if loops != nil {
		s.Loops = true
		if organism == "" {
			var calls []string
			for _, c := range clonotypes {
				for _, ch := range c.Chains {
					calls = append(calls, ch.V)
				}
			}
			organism, _ = loops.Guess(calls)
		}
		s.Organism = organism
	}
	type receptor struct {
		clonotype int
		chains    []clonotype.Chain
		alleles   []string
	}
	bySignature := map[string][]receptor{}
	var signatures []string
	for i, c := range clonotypes {
		r := receptor{clonotype: i}
		var loci []string
		ok := len(c.Chains) > 0
		for _, ch := range c.Chains {
			if !tLoci[ch.Locus] {
				s.Excluded[ReasonNotTCell]++
				ok = false
				break
			}
			if ch.Junction == "" {
				s.Excluded[ReasonNoJunction]++
				ok = false
				break
			}
			allele := ""
			if s.Loops {
				_, a, found := loops.Lookup(organism, ch.V)
				if !found {
					s.Excluded[ReasonNoLoops]++
					ok = false
					break
				}
				if a != ch.V {
					s.Alleles[ch.V] = a
				}
				allele = a
			}
			loci = append(loci, ch.Locus)
			r.chains = append(r.chains, ch)
			r.alleles = append(r.alleles, allele)
		}
		if !ok {
			continue
		}
		sig := strings.Join(loci, "+")
		if _, seen := bySignature[sig]; !seen {
			signatures = append(signatures, sig)
		}
		bySignature[sig] = append(bySignature[sig], r)
	}
	// Groups by their number of receptors, the largest first (then by their chains).
	sort.SliceStable(signatures, func(a, b int) bool {
		na, nb := len(bySignature[signatures[a]]), len(bySignature[signatures[b]])
		if na != nb {
			return na > nb
		}
		return signatures[a] < signatures[b]
	})
	for _, sig := range signatures {
		s.chains = max(s.chains, len(strings.Split(sig, "+")))
	}
	s.cdr3 = make([][][]uint8, s.chains)
	s.v = make([][]int32, s.chains)
	alleles := make([]map[string]int32, s.chains)
	names := make([][]string, s.chains)
	for c := range alleles {
		alleles[c] = map[string]int32{}
	}
	for g, sig := range signatures {
		rs := bySignature[sig]
		s.Groups = append(s.Groups, Group{Loci: strings.Split(sig, "+"), Start: len(s.Clonotype), Receptors: len(rs)})
		for _, r := range rs {
			s.Clonotype = append(s.Clonotype, r.clonotype)
			s.group = append(s.group, int32(g))
			for c := 0; c < s.chains; c++ {
				var cdr3 []uint8
				v := int32(-1)
				if c < len(r.chains) {
					cdr3 = Encode(r.chains[c].Junction)
					if s.Loops {
						// Alleles are indexed per chain position and locus, so that a table's loops
						// compare only alleles of one locus.
						key := r.chains[c].Locus + " " + r.alleles[c]
						idx, ok := alleles[c][key]
						if !ok {
							idx = int32(len(names[c]))
							alleles[c][key] = idx
							names[c] = append(names[c], r.alleles[c])
						}
						v = idx
					}
				}
				s.cdr3[c] = append(s.cdr3[c], cdr3)
				s.v[c] = append(s.v[c], v)
			}
		}
	}
	if s.Loops {
		s.loop = make([][][]int, s.chains)
		s.alleles = make([][]Chain, s.chains)
		for c := range names {
			n := len(names[c])
			s.loop[c] = make([][]int, n)
			encoded := make([]Chain, n)
			s.alleles[c] = encoded
			for a, name := range names[c] {
				l, _, _ := loops.Lookup(organism, name)
				encoded[a] = NewChain("", l[:])
			}
			for a := 0; a < n; a++ {
				s.loop[c][a] = make([]int, n)
				for b := 0; b < n; b++ {
					s.loop[c][a][b] = LoopDistance(encoded[a], encoded[b])
				}
			}
		}
	}
	return s
}

// Len is the number of receptors.
func (s *Set) Len() int { return len(s.Clonotype) }

// Distance is the TCRdist between receptors a and b of one group.
func (s *Set) Distance(a, b int) int {
	d := 0
	for c := range s.cdr3 {
		if s.cdr3[c][a] == nil {
			break
		}
		d += distance(s.cdr3[c][a], s.cdr3[c][b], cdr3Region)
		if s.Loops {
			d += s.loop[c][s.v[c][a]][s.v[c][b]]
		}
	}
	return d
}

// Regions are receptors a and b's distance chain by chain and loop by loop (CDR1, CDR2, CDR2.5
// and CDR3, weighted, as Regions answers them); they sum to Distance.
func (s *Set) Regions(a, b int) [][4]int {
	var out [][4]int
	for c := range s.cdr3 {
		if s.cdr3[c][a] == nil {
			break
		}
		x, y := Chain{CDR3: s.cdr3[c][a]}, Chain{CDR3: s.cdr3[c][b]}
		if s.Loops {
			x.Loops, x.HasLoops = s.alleles[c][s.v[c][a]].Loops, true
			y.Loops, y.HasLoops = s.alleles[c][s.v[c][b]].Loops, true
		}
		out = append(out, Regions(x, y))
	}
	return out
}

// bound is a lower bound of the distance between receptors a and b: their loops' distance, and
// the cost of their CDR3s' difference in length (each residue 4, weighted 3), which no alignment
// avoids.
func (s *Set) bound(a, b int) int {
	d := 0
	for c := range s.cdr3 {
		x := s.cdr3[c][a]
		if x == nil {
			break
		}
		diff := len(x) - len(s.cdr3[c][b])
		if diff < 0 {
			diff = -diff
		}
		d += diff * cdr3Region.gap * cdr3Region.weight
		if s.Loops {
			d += s.loop[c][s.v[c][a]][s.v[c][b]]
		}
	}
	return d
}

// Neighborhood is a receptor's neighbors within the radius (itself not counted), and its nearest
// other receptor of its group (-1 when it has none) with their distance.
type Neighborhood struct {
	Neighbors int
	Nearest   int
	Distance  int
}

// Neighbors answers every receptor's neighborhood, comparing each with every other of its group
// on workers goroutines (0: one for each processor). The answer does not depend on the workers:
// each receptor's is computed whole by one of them, and a tie for the nearest goes to the first
// receptor.
func (s *Set) Neighbors(ctx context.Context, radius, workers int, progress func(done, of int)) ([]Neighborhood, error) {
	if radius < 0 {
		return nil, errors.New("the radius is a distance: 0 or more")
	}
	if workers <= 0 {
		workers = runtime.GOMAXPROCS(0)
	}
	n := s.Len()
	out := make([]Neighborhood, n)
	var next, done atomic.Int64
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				if ctx.Err() != nil {
					return
				}
				i := int(next.Add(1) - 1)
				if i >= n {
					return
				}
				out[i] = s.neighborhood(i, radius)
				if d := done.Add(1); progress != nil && d%256 == 0 {
					progress(int(d), n)
				}
			}
		}()
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if progress != nil {
		progress(n, n)
	}
	return out, nil
}

func (s *Set) neighborhood(i, radius int) Neighborhood {
	g := s.Groups[s.group[i]]
	h := Neighborhood{Nearest: -1, Distance: -1}
	for j := g.Start; j < g.Start+g.Receptors; j++ {
		if j == i {
			continue
		}
		// Neither a neighbor nor nearer than the nearest so far (a tie goes to the first).
		if lb := s.bound(i, j); lb > radius && h.Nearest >= 0 && lb >= h.Distance {
			continue
		}
		d := s.Distance(i, j)
		if d <= radius {
			h.Neighbors++
		}
		if h.Nearest < 0 || d < h.Distance {
			h.Nearest, h.Distance = j, d
		}
	}
	return h
}

// Pair is a receptor and its distance from another.
type Pair struct {
	Receptor int
	Distance int
}

// Nearest answers receptor i's k nearest other receptors of its group, nearest first (ties in
// the receptors' order).
func (s *Set) Nearest(i, k int) []Pair {
	g := s.Groups[s.group[i]]
	var out []Pair
	for j := g.Start; j < g.Start+g.Receptors; j++ {
		if j == i {
			continue
		}
		if len(out) == k && s.bound(i, j) > out[k-1].Distance {
			continue
		}
		d := s.Distance(i, j)
		if len(out) == k && d >= out[k-1].Distance {
			continue
		}
		at := sort.Search(len(out), func(x int) bool { return out[x].Distance > d })
		out = append(out, Pair{})
		copy(out[at+1:], out[at:])
		out[at] = Pair{Receptor: j, Distance: d}
		if len(out) > k {
			out = out[:k]
		}
	}
	return out
}

// Receptor answers the receptor of a clonotype (by its index in the table), or -1.
func (s *Set) Receptor(clonotype int) int {
	for r, c := range s.Clonotype {
		if c == clonotype {
			return r
		}
	}
	return -1
}
