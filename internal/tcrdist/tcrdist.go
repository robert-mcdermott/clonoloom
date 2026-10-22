// Package tcrdist measures how alike two T-cell receptors are by TCRdist (clonoloom-spec/
// requirements.md, S1): Dash et al. 2017 (Nature 547:89, doi:10.1038/nature22383), computed as
// tcrdist3 computes it (Mayer-Blackwell et al. 2021, eLife 10:e68605, doi:10.7554/eLife.68605;
// clonoloom-spec/research.md §5.9), so that its distances equal tcrdist3's.
//
// A chain's distance sums four loops: the V gene's CDR1, CDR2 and CDR2.5 (the loop between CDR2
// and CDR3 that faces the peptide–MHC), each of weight 1, and the CDR3, of weight 3. Residues
// differ by min(4, 4 − BLOSUM62) (matrix.go). The V gene's loops are aligned strings of one length
// within an organism and chain, compared position by position. The CDR3 (the IMGT junction) leaves
// out its first three and last two residues; two of different lengths are aligned at their two
// ends, the longer one's extra residues left out where the residues compared differ least, each
// extra residue costing 4. Where tcrdist3 and Dash et al.'s description differ, ClonoLoom does as
// tcrdist3 does: the CDR3's gap placed where it costs least (Dash et al., at a fixed place), the
// CDR3's weight multiplying its gaps too (12 a residue; Dash et al. say 8), and a loop's gap
// character costing nothing against a residue. A pair of chains' distance is the sum of its
// chains'. Distances are whole numbers.
package tcrdist

// region is how a stretch of a chain is compared: the weight of its distance, the cost of each
// residue by which the two lengths differ, the residues left out at its start and end, and
// whether the gap is placed at a fixed position or where it costs least.
type region struct {
	weight, gap, ntrim, ctrim int
	fixed                     bool
}

// tcrdist3's parameters for the CDR3 and for the V gene's loops.
var (
	cdr3Region = region{weight: 3, gap: 4, ntrim: 3, ctrim: 2, fixed: false}
	loopRegion = region{weight: 1, gap: 4, ntrim: 0, ctrim: 0, fixed: true}
)

// floorDiv divides rounding down, as Python's // does (Go's / rounds toward zero).
func floorDiv(a, b int) int {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

// distance compares two encoded sequences over a region.
func distance(a, b []uint8, r region) int {
	la, lb := len(a), len(b)
	if la == lb {
		d := 0
		for i := r.ntrim; i < la-r.ctrim; i++ {
			d += int(table[a[i]][b[i]])
		}
		return d * r.weight
	}
	short, diff := min(la, lb), la-lb
	if diff < 0 {
		diff = -diff
	}
	var lo, hi int
	if r.fixed {
		// After the conserved cysteine and five residues, earlier in a short sequence.
		lo = min(6, 3+floorDiv(short-5, 2))
		hi = lo
	} else {
		// Anywhere from five residues in to five from the end, widened until there is a place.
		lo, hi = 5, short-5
		for lo > hi {
			lo--
			hi++
		}
	}
	best := -1
	for gap := lo; gap <= hi; gap++ {
		d := 0
		for i := r.ntrim; i < gap; i++ {
			d += int(table[a[i]][b[i]])
		}
		for c := r.ctrim; c < short-gap; c++ {
			d += int(table[a[la-1-c]][b[lb-1-c]])
		}
		if best < 0 || d < best {
			best = d
		}
		if best == 0 {
			break
		}
	}
	return (best + diff*r.gap) * r.weight
}

// Chain is one chain of a receptor, encoded: its CDR3 (the IMGT junction) and, where its V gene's
// loops are known, its CDR1, CDR2 and CDR2.5.
type Chain struct {
	CDR3  []uint8
	Loops [3][]uint8
	// HasLoops: the loops are known (TCRdist on the CDR3 alone otherwise).
	HasLoops bool
}

// NewChain encodes a chain from its junction's amino acids and its V gene's loops (nil for the
// CDR3 alone).
func NewChain(junction string, loops []string) Chain {
	c := Chain{CDR3: Encode(junction)}
	if len(loops) == 3 {
		c.HasLoops = true
		for i, l := range loops {
			c.Loops[i] = Encode(l)
		}
	}
	return c
}

// ChainDistance is the TCRdist between two chains of one locus: the CDR3's, and the loops' where
// both chains' are known.
func ChainDistance(x, y Chain) int {
	d := distance(x.CDR3, y.CDR3, cdr3Region)
	if x.HasLoops && y.HasLoops {
		for i := range x.Loops {
			d += distance(x.Loops[i], y.Loops[i], loopRegion)
		}
	}
	return d
}

// LoopDistance is the distance between two chains' V gene loops alone (0 unless both are known).
func LoopDistance(x, y Chain) int {
	if !x.HasLoops || !y.HasLoops {
		return 0
	}
	d := 0
	for i := range x.Loops {
		d += distance(x.Loops[i], y.Loops[i], loopRegion)
	}
	return d
}

// CDR3Distance is the distance between two chains' CDR3s alone.
func CDR3Distance(x, y Chain) int {
	return distance(x.CDR3, y.CDR3, cdr3Region)
}

// Regions are two chains' distance loop by loop, each weighted: CDR1, CDR2 and CDR2.5 (0 unless
// both chains' loops are known) and CDR3. They sum to ChainDistance.
func Regions(x, y Chain) [4]int {
	var out [4]int
	if x.HasLoops && y.HasLoops {
		for i := range x.Loops {
			out[i] = distance(x.Loops[i], y.Loops[i], loopRegion)
		}
	}
	out[3] = distance(x.CDR3, y.CDR3, cdr3Region)
	return out
}

// Distance is the TCRdist between two receptors of the same chains, in the same order: the sum
// of their chains'.
func Distance(x, y []Chain) int {
	d := 0
	for i := range x {
		d += ChainDistance(x[i], y[i])
	}
	return d
}
