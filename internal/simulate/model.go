// Package simulate makes immune repertoire studies whose truth is known (clonoloom-spec/
// requirements.md, E1): T-cell receptor β rearrangements from a V(D)J recombination model of
// ClonoLoom's own, each subject's repertoire a population of clonotypes with power-law sizes, and
// each sample templates drawn from it and sequenced into reads. The data are written as AIRR
// rearrangement tables with their Repertoire metadata, and the truth beside them: every
// clonotype of every subject's population, and the numbers the analyses estimate.
//
// The model generates rearrangements as the recombination machinery is understood to (Murugan et
// al. 2012; Marcou et al. 2018): a V, a D given the J, and a J; deletions at each end, with
// palindromic nucleotides added where none is deleted; non-templated insertions between V and D
// and between D and J. Its germline segments are GRCh38's (germline.go); its usage, the 10x
// Genomics 10k PBMC data's; its deletion and insertion distributions are chosen to give that
// data's junction lengths. It is not an inferred model like IGoR's: it is a simple, documented
// one, for testing methods against a known truth.
package simulate

import (
	"math"
	"strings"

	"clonoloom/internal/dmath"
	"clonoloom/internal/random"
)

// Segment is a germline gene segment.
type Segment struct {
	Name string
	Seq  string
	// Anchor: the first nucleotide of the conserved cysteine's codon (a V) or of the conserved
	// phenylalanine's or tryptophan's (a J), in Seq.
	Anchor int
	// Usage: how often it is chosen (Vs and Js).
	Usage      float64
	Transcript string
}

// Locus is a locus's germline segments and where they come from, and the lengths of the
// productive junctions of a repertoire of it (amino acids: chains), which selection gives.
type Locus struct {
	Locus, Source   string
	V, D, J         []Segment
	JunctionLengths map[int]int
}

// Deletions: how many nucleotides are deleted from a segment's end. A negative count adds that
// many palindromic nucleotides instead (the reverse complement of the end, as hairpin opening
// leaves them): Palindromic[k-1] is the chance of k of them; the rest is a negative binomial of
// this mean and size, at most Max.
type Deletions struct {
	Mean        float64   `json:"mean"`
	Size        float64   `json:"size"`
	Max         int       `json:"max"`
	Palindromic []float64 `json:"palindromic"`
}

// Insertions: the number of non-templated nucleotides added (a negative binomial of this mean
// and size, at most Max), each drawn from Composition (A, C, G, T).
type Insertions struct {
	Mean        float64    `json:"mean"`
	Size        float64    `json:"size"`
	Max         int        `json:"max"`
	Composition [4]float64 `json:"composition"`
}

// Recombination is the model's parameters.
type Recombination struct {
	VDeletions  Deletions  `json:"vDeletions"`
	D5Deletions Deletions  `json:"d5Deletions"`
	D3Deletions Deletions  `json:"d3Deletions"`
	JDeletions  Deletions  `json:"jDeletions"`
	VDInsertion Insertions `json:"vdInsertions"`
	DJInsertion Insertions `json:"djInsertions"`
	// D2WithJ2: the chance a rearrangement to a TRBJ2 gene uses TRBD2 rather than TRBD1 (one to
	// a TRBJ1 gene can only use TRBD1, which lies between them). The 10x data cannot tell (most
	// of their junctions have no D called): an even chance is assumed.
	D2WithJ2 float64 `json:"d2WithJ2"`
	// SelectLengths: productive rearrangements are selected on their junction's length, so that
	// their lengths are the locus's JunctionLengths (Draw).
	SelectLengths bool `json:"selectLengths"`
}

// HumanTRB is the human TRB locus the simulator recombines (germline.go): its gene segments and
// their usage. Its slices are shared: read them, do not change them.
func HumanTRB() Locus { return humanTRB }

// DefaultRecombination: deletion and insertion distributions chosen so that the productive
// junctions' lengths are those of the 10x Genomics 10k PBMC data's TRB chains (mean 14.5 amino
// acids; checked by TestJunctionLengthsMatchTheOpenData), and the palindromic nucleotides
// occasional, as they are observed to be.
func DefaultRecombination() Recombination {
	pal := []float64{0.04, 0.02}
	return Recombination{
		VDeletions:    Deletions{Mean: 4, Size: 2, Max: 16, Palindromic: pal},
		D5Deletions:   Deletions{Mean: 3, Size: 2, Max: 12, Palindromic: pal},
		D3Deletions:   Deletions{Mean: 3, Size: 2, Max: 12, Palindromic: pal},
		JDeletions:    Deletions{Mean: 4, Size: 2, Max: 16, Palindromic: pal},
		VDInsertion:   Insertions{Mean: 4.5, Size: 2, Max: 25, Composition: [4]float64{0.22, 0.28, 0.28, 0.22}},
		DJInsertion:   Insertions{Mean: 4, Size: 2, Max: 25, Composition: [4]float64{0.22, 0.28, 0.28, 0.22}},
		D2WithJ2:      0.5,
		SelectLengths: true,
	}
}

// Rearrangement is one recombination event.
type Rearrangement struct {
	V, D, J *Segment
	// The deletions (negative: palindromic nucleotides) and insertions.
	DelV, DelD5, DelD3, DelJ int
	N1, N2                   string
	// Sequence: the rearranged region, from the V's 30 nucleotides before its cysteine to the J's
	// end; Junction: from the cysteine's codon to the phenylalanine's (or tryptophan's), both
	// included, at JunctionStart in Sequence.
	Sequence      string
	Junction      string
	JunctionStart int
	JunctionAA    string
	InFrame, Stop bool
	Productive    bool
}

// Generator draws rearrangements.
type Generator struct {
	locus *Locus
	model Recombination
	v, j  []float64
	// d1, d2: TRBD1 and TRBD2.
	d1, d2 *Segment
	// keep: the chance a productive rearrangement of a junction this long (amino acids) is kept.
	keep []float64
}

func NewGenerator(locus *Locus, model Recombination) *Generator {
	g := &Generator{locus: locus, model: model}
	vw := make([]float64, len(locus.V))
	for i, s := range locus.V {
		vw[i] = s.Usage
	}
	jw := make([]float64, len(locus.J))
	for i, s := range locus.J {
		jw[i] = s.Usage
	}
	g.v, g.j = random.Cumulate(vw), random.Cumulate(jw)
	for i := range locus.D {
		switch locus.D[i].Name {
		case "TRBD1":
			g.d1 = &locus.D[i]
		case "TRBD2":
			g.d2 = &locus.D[i]
		}
	}
	if model.SelectLengths && len(locus.JunctionLengths) > 0 {
		g.keep = g.selection()
	}
	return g
}

// selection answers the chance of keeping a productive rearrangement by its junction's length
// (amino acids): proportional to the target share over the generated share, at most 1 (after
// Elhanati et al. 2014's selection factors, here on length alone). The generated shares are
// measured once, on 200,000 rearrangements of a fixed seed; the target's are the locus's, each
// count with a quarter added (from two shorter to two longer than any seen), so that a length
// the data happen not to show is rare rather than impossible.
func (g *Generator) selection() []float64 {
	lo, hi := 1<<30, 0
	for l := range g.locus.JunctionLengths {
		lo, hi = min(lo, l), max(hi, l)
	}
	lo, hi = max(lo-2, 1), hi+2
	target := make([]float64, hi+1)
	for l := lo; l <= hi; l++ {
		target[l] = float64(g.locus.JunctionLengths[l]) + 0.25
	}
	generated := make([]float64, hi+1)
	r := random.New(20141014, 0)
	for range 200000 {
		x := g.Rearrange(r)
		if x.Productive && len(x.JunctionAA) <= hi {
			generated[len(x.JunctionAA)]++
		}
	}
	keep := make([]float64, hi+1)
	top := 0.0
	for l := range keep {
		if generated[l] > 0 {
			keep[l] = target[l] / generated[l]
			top = max(top, keep[l])
		}
	}
	for l := range keep {
		keep[l] /= top
	}
	return keep
}

// Draw draws a rearrangement as a repertoire holds it: a productive one selected on its
// junction's length (when the model selects), a nonproductive one, which is not expressed, as
// recombined.
func (g *Generator) Draw(r *random.Random) Rearrangement {
	for {
		x := g.Rearrange(r)
		if !x.Productive || g.keep == nil {
			return x
		}
		if l := len(x.JunctionAA); l < len(g.keep) && r.Float64() < g.keep[l] {
			return x
		}
	}
}

func deletion(r *random.Random, d Deletions, limit int) int {
	u := r.Float64()
	for k, p := range d.Palindromic {
		if u < p {
			return -(k + 1)
		}
		u -= p
	}
	n := r.NegBinomial(d.Mean, d.Size)
	return min(n, d.Max, limit)
}

func insertion(r *random.Random, in Insertions) string {
	n := min(r.NegBinomial(in.Mean, in.Size), in.Max)
	weights := random.Cumulate(in.Composition[:])
	var b strings.Builder
	for range n {
		b.WriteByte("ACGT"[r.Discrete(weights)])
	}
	return b.String()
}

// trim3 cuts del nucleotides from s's 3' end, or adds -del palindromic ones (at most as many as
// s has).
func trim3(s string, del int) string {
	if del >= 0 {
		return s[:len(s)-del]
	}
	return s + reverseComplement(s[len(s)-min(-del, len(s)):])
}

// trim5 cuts del nucleotides from s's 5' end, or adds -del palindromic ones (at most as many as
// s has).
func trim5(s string, del int) string {
	if del >= 0 {
		return s[del:]
	}
	return reverseComplement(s[:min(-del, len(s))]) + s
}

// Rearrange draws one rearrangement. Deletions never reach the conserved cysteine or
// phenylalanine (whose loss would make a chain no junction defines).
func (g *Generator) Rearrange(r *random.Random) Rearrangement {
	m := g.model
	v := &g.locus.V[r.Discrete(g.v)]
	j := &g.locus.J[r.Discrete(g.j)]
	d := g.d1
	if strings.HasPrefix(j.Name, "TRBJ2") && g.d2 != nil && r.Float64() < m.D2WithJ2 {
		d = g.d2
	}
	out := Rearrangement{V: v, D: d, J: j}
	out.DelV = deletion(r, m.VDeletions, len(v.Seq)-v.Anchor-3)
	out.DelJ = deletion(r, m.JDeletions, j.Anchor)
	out.DelD5 = deletion(r, m.D5Deletions, len(d.Seq))
	out.DelD3 = deletion(r, m.D3Deletions, len(d.Seq)-max(out.DelD5, 0))
	out.N1 = insertion(r, m.VDInsertion)
	out.N2 = insertion(r, m.DJInsertion)
	vPart := trim3(v.Seq, out.DelV)
	dPart := trim3(trim5(d.Seq, out.DelD5), out.DelD3)
	jPart := trim5(j.Seq, out.DelJ)
	start := max(0, v.Anchor-30)
	full := vPart + out.N1 + dPart + out.N2 + jPart
	end := len(vPart) + len(out.N1) + len(dPart) + len(out.N2) + j.Anchor - out.DelJ + 3
	out.Sequence = full[start:]
	out.JunctionStart = v.Anchor - start
	out.Junction = full[v.Anchor:end]
	out.InFrame = len(out.Junction)%3 == 0
	if out.InFrame {
		out.JunctionAA = Translate(out.Junction)
		out.Stop = strings.Contains(out.JunctionAA, "*")
	}
	out.Productive = out.InFrame && !out.Stop
	return out
}

var complement = [256]byte{'A': 'T', 'C': 'G', 'G': 'C', 'T': 'A'}

func reverseComplement(s string) string {
	b := make([]byte, len(s))
	for i := range s {
		b[len(s)-1-i] = complement[s[i]]
	}
	return string(b)
}

var codons = func() map[string]byte {
	const bases, aas = "TCAG", "FFLLSSSSYY**CC*WLLLLPPPPHHQQRRRRIIIMTTTTNNKKSSRRVVVVAAAADDEEGGGG"
	m := map[string]byte{}
	i := 0
	for _, a := range bases {
		for _, b := range bases {
			for _, c := range bases {
				m[string([]rune{a, b, c})] = aas[i]
				i++
			}
		}
	}
	return m
}()

// Translate translates whole codons.
func Translate(s string) string {
	b := make([]byte, 0, len(s)/3)
	for i := 0; i+3 <= len(s); i += 3 {
		aa, ok := codons[s[i:i+3]]
		if !ok {
			aa = 'X'
		}
		b = append(b, aa)
	}
	return string(b)
}

// powerLaw draws a clone size from a discrete power law: P(size ≥ s) = s^-alpha, s ≥ 1, at most
// limit.
func powerLaw(r *random.Random, alpha float64, limit int) int {
	s := math.Floor(dmath.Pow(1-r.Float64(), -1/alpha))
	if s > float64(limit) {
		return limit
	}
	return int(s)
}
