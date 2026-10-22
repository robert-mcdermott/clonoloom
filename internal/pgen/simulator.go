package pgen

import (
	"strings"

	"clonoloom/internal/dmath"
	"clonoloom/internal/simulate"
)

// Simulator is ClonoLoom's own model of human TRB recombination, the simulator's
// (internal/simulate): GRCh38's gene segments with the usage of 10x Genomics' 10k PBMC data, and
// its deletions and insertions as it draws them, in a model's terms. Built in, so that generation
// probabilities need no download; the simulated studies are made from it, so it is their truth.
// Its probabilities are before the simulator's selection on junction length.
//
// What the simulator does, as a model: a deletion is palindromic (−k with the chance given) or
// else a negative binomial of its mean and size, at most its maximum and the gene's limit (a V's
// cysteine codon and a J's phenylalanine codon are never reached; a D's 3′ end is not deleted past
// its 5′ deletion), the probability beyond piled at that limit; each insertion's length the same,
// at most its maximum, and its nucleotides independent, of the composition given; TRBD2 used with
// a TRBJ2 gene by its chance, TRBD1 otherwise. One thing differs: a D's 3′ palindrome is of the
// gene's end, where the simulator, after a 5′ deletion that leaves fewer nucleotides than the
// palindrome, takes it from what is left (a rare event: a D nearly deleted whole).
func Simulator() *Model {
	locus := simulate.HumanTRB()
	rec := simulate.DefaultRecombination()
	m := &Model{Name: "clonoloom_human_T_beta", Locus: "TRB", VDJ: true}
	pal := func(d simulate.Deletions) int { return len(d.Palindromic) }
	m.PalV, m.PalJ, m.PalD5, m.PalD3 = pal(rec.VDeletions), pal(rec.JDeletions), pal(rec.D5Deletions), pal(rec.D3Deletions)
	usage := func(segs []simulate.Segment) []float64 {
		out := make([]float64, len(segs))
		sum := 0.0
		for _, s := range segs {
			sum += s.Usage
		}
		for i, s := range segs {
			out[i] = s.Usage / sum
		}
		return out
	}
	for _, s := range locus.V {
		m.V = append(m.V, Gene{Name: s.Name, Seq: s.Seq, Anchor: s.Anchor, Used: true})
		m.DelV = append(m.DelV, deletions(rec.VDeletions, len(s.Seq)-s.Anchor-3))
	}
	for _, s := range locus.J {
		m.J = append(m.J, Gene{Name: s.Name, Seq: s.Seq, Anchor: s.Anchor, Used: true})
		m.DelJ = append(m.DelJ, deletions(rec.JDeletions, s.Anchor))
	}
	d1, d2 := -1, -1
	for i, s := range locus.D {
		m.D = append(m.D, Gene{Name: s.Name, Seq: s.Seq, Used: true})
		m.DelD5 = append(m.DelD5, deletions(rec.D5Deletions, len(s.Seq)))
		// The 3′ deletion given the 5′: its limit what the 5′ deletion left.
		var d3 [][]float64
		for t5 := range m.DelD5[i] {
			v5 := t5 - m.PalD5
			d3 = append(d3, deletions(rec.D3Deletions, len(s.Seq)-max(v5, 0)))
		}
		m.DelD3 = append(m.DelD3, d3)
		switch s.Name {
		case "TRBD1":
			d1 = i
		case "TRBD2":
			d2 = i
		}
	}
	m.PV = usage(locus.V)
	pj := usage(locus.J)
	m.PJD = make([][]float64, len(m.J))
	for j, s := range locus.J {
		m.PJD[j] = make([]float64, len(m.D))
		if strings.HasPrefix(s.Name, "TRBJ2") && d2 >= 0 {
			m.PJD[j][d2] = float64(pj[j] * rec.D2WithJ2)
			m.PJD[j][d1] = float64(pj[j] * (1 - rec.D2WithJ2))
		} else {
			m.PJD[j][d1] = pj[j]
		}
	}
	m.InsVD = insertions(rec.VDInsertion)
	m.InsDJ = insertions(rec.DJInsertion)
	composition := func(c [4]float64) ([4][4]float64, [4]float64) {
		sum := c[0] + c[1] + c[2] + c[3]
		var p [4]float64
		for i := range p {
			p[i] = c[i] / sum
		}
		return [4][4]float64{p, p, p, p}, p
	}
	m.MarkovVD, m.StationaryVD = composition(rec.VDInsertion.Composition)
	m.MarkovDJ, m.StationaryDJ = composition(rec.DJInsertion.Composition)
	m.prepare()
	return m
}

// negBinomial is the probability of k under a negative binomial of this mean and size (a
// Poisson whose mean is gamma distributed, as random.NegBinomial draws it).
func negBinomial(k int, mean, size float64) float64 {
	if mean <= 0 {
		if k == 0 {
			return 1
		}
		return 0
	}
	kf := float64(k)
	logP := dmath.Lgamma(kf+size) - dmath.Lgamma(size) - dmath.Lgamma(kf+1) + float64(size*dmath.Log(size/(size+mean))) + float64(kf*dmath.Log(mean/(size+mean)))
	return dmath.Exp(logP)
}

// deletions is a deletion's distribution by nucleotides trimmed from the palindrome-extended end
// (palindromic nucleotides first): −k with its chance, else the negative binomial at most
// min(its maximum, limit), the rest piled there.
func deletions(d simulate.Deletions, limit int) []float64 {
	p := len(d.Palindromic)
	top := min(d.Max, limit)
	out := make([]float64, p+top+1)
	rest := 1.0
	for k, x := range d.Palindromic {
		out[p-(k+1)] = x
		rest -= x
	}
	below := 0.0
	for v := 0; v < top; v++ {
		x := negBinomial(v, d.Mean, d.Size)
		out[p+v] = float64(rest * x)
		below += x
	}
	out[p+top] = float64(rest * (1 - below))
	return out
}

// insertions is an insertion's length distribution: the negative binomial at most its maximum, the
// rest piled there.
func insertions(in simulate.Insertions) []float64 {
	out := make([]float64, in.Max+1)
	below := 0.0
	for l := 0; l < in.Max; l++ {
		out[l] = negBinomial(l, in.Mean, in.Size)
		below += out[l]
	}
	out[in.Max] = 1 - below
	return out
}
