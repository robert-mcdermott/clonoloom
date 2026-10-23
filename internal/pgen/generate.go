package pgen

import (
	"strings"

	"clonoloom/internal/random"
)

// Draw is a receptor drawn from a model: its CDR3's nucleotides and amino acids (none when out of
// frame), its V and J alleles, and whether it is productive (in frame, without a stop codon).
type Draw struct {
	NT, AA, V, J string
	Productive   bool
}

// Generator draws receptors from a model, as recombination makes them: from its used genes (the
// others' probability left out), every event drawn as the model gives it; an event a model allows
// but that makes no CDR3 (a deletion longer than its segment, a D deleted past itself) is drawn
// again. A CDR3 is drawn with its generation probability over the probability of the events that
// make one (1 for ClonoLoom's own model).
type Generator struct {
	m         *Model
	v, jd, vj []float64
	// dGivenJ[j]: the D genes' cumulative weights given J, for a draw of named genes; gene[seg]:
	// each V or J gene's used alleles, by the gene's name.
	dGivenJ        [][]float64
	vGene          map[string][]int
	jGene          map[string][]int
	delV, delJ     [][]float64
	delD5          [][]float64
	delD3          [][][]float64
	insVD, insDJ   []float64
	chainVD        [4][]float64
	chainDJ        [4][]float64
	firstVD, endDJ []float64
}

func NewGenerator(m *Model) *Generator {
	g := &Generator{m: m}
	cum := random.Cumulate
	if m.VDJ {
		w := make([]float64, len(m.V))
		for v := range m.V {
			if m.V[v].Used && m.vExt[v] != nil {
				w[v] = m.PV[v]
			}
		}
		g.v = cum(w)
		jd := make([]float64, len(m.J)*len(m.D))
		for j := range m.J {
			for d := range m.D {
				if m.J[j].Used && m.jExt[j] != nil {
					jd[j*len(m.D)+d] = m.PJD[j][d]
				}
			}
		}
		g.jd = cum(jd)
	} else {
		vj := make([]float64, len(m.V)*len(m.J))
		for v := range m.V {
			for j := range m.J {
				if m.V[v].Used && m.J[j].Used && m.vExt[v] != nil && m.jExt[j] != nil {
					vj[v*len(m.J)+j] = m.PVJ[v][j]
				}
			}
		}
		g.vj = cum(vj)
	}
	all := func(rows [][]float64) [][]float64 {
		out := make([][]float64, len(rows))
		for i, r := range rows {
			out[i] = cum(r)
		}
		return out
	}
	g.delV, g.delJ = all(m.DelV), all(m.DelJ)
	if m.VDJ {
		g.delD5 = all(m.DelD5)
		for _, rows := range m.DelD3 {
			g.delD3 = append(g.delD3, all(rows))
		}
		g.insDJ = cum(m.InsDJ)
		for b := 0; b < 4; b++ {
			g.chainDJ[b] = cum(m.MarkovDJ[b][:])
		}
		g.endDJ = cum(m.StationaryDJ[:])
	}
	g.insVD = cum(m.InsVD)
	for b := 0; b < 4; b++ {
		g.chainVD[b] = cum(m.MarkovVD[b][:])
	}
	g.firstVD = cum(m.StationaryVD[:])
	if m.VDJ {
		g.dGivenJ = make([][]float64, len(m.J))
		for j := range m.J {
			g.dGivenJ[j] = cum(m.PJD[j])
		}
	}
	g.vGene, g.jGene = usedByGene(m.V, m.vExt), usedByGene(m.J, m.jExt)
	return g
}

// usedByGene lists each gene's used alleles (that the model can make a CDR3 from), by the gene's
// name and by each allele's.
func usedByGene(genes []Gene, ext [][]uint8) map[string][]int {
	out := map[string][]int{}
	for i, g := range genes {
		if !g.Used || ext[i] == nil {
			continue
		}
		out[g.Name] = append(out[g.Name], i)
		if gene, _, ok := strings.Cut(g.Name, "*"); ok {
			out[gene] = append(out[gene], i)
		}
	}
	return out
}

// Usage answers the probability of each pair of V and J genes the model uses, by their gene names
// (each gene's alleles summed), over the pairs it can make.
func (m *Model) Usage() map[[2]string]float64 {
	gene := func(name string) string {
		g, _, _ := strings.Cut(name, "*")
		return g
	}
	out := map[[2]string]float64{}
	total := 0.0
	for v := range m.V {
		if !m.V[v].Used || m.vExt[v] == nil {
			continue
		}
		for j := range m.J {
			if !m.J[j].Used || m.jExt[j] == nil {
				continue
			}
			p := 0.0
			if m.VDJ {
				for d := range m.D {
					p += float64(m.PV[v] * m.PJD[j][d])
				}
			} else {
				p = m.PVJ[v][j]
			}
			if p > 0 {
				out[[2]string{gene(m.V[v].Name), gene(m.J[j].Name)}] += p
				total += p
			}
		}
	}
	if total > 0 {
		for k, p := range out {
			out[k] = p / total
		}
	}
	return out
}

// Genes answers whether the model uses a V gene and a J gene of these names (a gene, or one of
// its alleles), which DrawWith can draw.
func (g *Generator) Genes(v, j string) bool {
	for _, vi := range g.vGene[v] {
		for _, ji := range g.jGene[j] {
			if !g.m.VDJ {
				if g.m.PVJ[vi][ji] > 0 {
					return true
				}
			} else if g.m.PV[vi] > 0 {
				for _, p := range g.m.PJD[ji] {
					if p > 0 {
						return true
					}
				}
			}
		}
	}
	return false
}

// DrawWith draws a receptor of the V and J genes named (a gene, one of whose used alleles is
// chosen in proportion to the model's probability, or one allele), every other event as the model
// gives it; false when the model uses no such genes. A D gene is drawn given the J, as the model
// gives it.
func (g *Generator) DrawWith(r *random.Random, v, j string) (Draw, bool) {
	vs, js := g.vGene[v], g.jGene[j]
	if len(vs) == 0 || len(js) == 0 {
		return Draw{}, false
	}
	m := g.m
	if !m.VDJ {
		// VJ models give P(V, J), not P(V)P(J). Condition that joint distribution on
		// the requested names, including the correlation between their alleles.
		weights := make([]float64, 0, len(vs)*len(js))
		for _, vi := range vs {
			for _, ji := range js {
				weights = append(weights, m.PVJ[vi][ji])
			}
		}
		cumulative := random.Cumulate(weights)
		for range 1000 {
			at := pick(r, cumulative)
			if at < 0 {
				return Draw{}, false
			}
			if out, ok := g.draw(r, vs[at/len(js)], js[at%len(js)], 0); ok {
				return out, true
			}
		}
		return Draw{}, false
	}
	if !g.Genes(v, j) {
		return Draw{}, false
	}
	// An allele of each gene, in proportion to the model's probability of it.
	allele := func(list []int, weight func(int) float64) int {
		if len(list) == 1 {
			return list[0]
		}
		w := make([]float64, len(list))
		for i, x := range list {
			w[i] = weight(x)
		}
		c := random.Cumulate(w)
		if at := pick(r, c); at >= 0 {
			return list[at]
		}
		return list[r.IntN(len(list))]
	}
	jTotal := func(x int) float64 {
		p := 0.0
		for d := range m.D {
			p += m.PJD[x][d]
		}
		return p
	}
	vTotal := func(x int) float64 {
		return m.PV[x]
	}
	for range 1000 {
		vi, ji := allele(vs, vTotal), allele(js, jTotal)
		d := 0
		if m.VDJ {
			if d = pick(r, g.dGivenJ[ji]); d < 0 {
				continue
			}
		}
		if out, ok := g.draw(r, vi, ji, d); ok {
			return out, true
		}
	}
	return Draw{}, false
}

// pick draws an index from cumulative weights, or −1 when they are all 0.
func pick(r *random.Random, cumulative []float64) int {
	if len(cumulative) == 0 || cumulative[len(cumulative)-1] <= 0 {
		return -1
	}
	return r.Discrete(cumulative)
}

// Draw draws one receptor.
func (g *Generator) Draw(r *random.Random) Draw {
	m := g.m
	for {
		var v, j, d int
		if m.VDJ {
			v = pick(r, g.v)
			jd := pick(r, g.jd)
			if v < 0 || jd < 0 {
				return Draw{}
			}
			j, d = jd/len(m.D), jd%len(m.D)
		} else {
			vj := pick(r, g.vj)
			if vj < 0 {
				return Draw{}
			}
			v, j = vj/len(m.J), vj%len(m.J)
		}
		if out, ok := g.draw(r, v, j, d); ok {
			return out
		}
	}
}

// draw draws a receptor of these genes' every other event, or false where they make no CDR3 (a
// deletion longer than its segment, a D deleted past itself).
func (g *Generator) draw(r *random.Random, v, j, d int) (Draw, bool) {
	m := g.m
	var b strings.Builder
	vExt, jExt := m.vExt[v], m.jExt[j]
	tv, tj := pick(r, g.delV[v]), pick(r, g.delJ[j])
	if tv < 0 || tj < 0 || tv > len(vExt) || tj > len(jExt) {
		return Draw{}, false
	}
	write(&b, vExt[:len(vExt)-tv])
	b.WriteString(g.insertion(r, g.insVD, g.firstVD, &g.chainVD, false))
	if m.VDJ {
		dExt := m.dExt[d]
		t5 := pick(r, g.delD5[d])
		if t5 < 0 || t5 >= len(g.delD3[d]) {
			return Draw{}, false
		}
		t3 := pick(r, g.delD3[d][t5])
		if t3 < 0 || t5+t3 > len(dExt) {
			return Draw{}, false
		}
		write(&b, dExt[t5:len(dExt)-t3])
		b.WriteString(g.insertion(r, g.insDJ, g.endDJ, &g.chainDJ, true))
	}
	write(&b, jExt[tj:])
	nt := b.String()
	out := Draw{NT: nt, V: m.V[v].Name, J: m.J[j].Name}
	if len(nt) > 0 && len(nt)%3 == 0 {
		out.AA = Translate(nt)
		out.Productive = !strings.ContainsAny(out.AA, "*X")
	}
	return out, true
}

// insertion draws an insertion's nucleotides: its length, then its first nucleotide from first and
// each next from the chain; reversed, the chain read from its far end (the DJ insertion, read from
// its J end).
func (g *Generator) insertion(r *random.Random, lengths, first []float64, chain *[4][]float64, reversed bool) string {
	n := pick(r, lengths)
	if n <= 0 {
		return ""
	}
	out := make([]byte, n)
	b := pick(r, first)
	for i := 0; i < n; i++ {
		if i > 0 {
			b = pick(r, chain[b])
		}
		if reversed {
			out[n-1-i] = nucleotides[b]
		} else {
			out[i] = nucleotides[b]
		}
	}
	return string(out)
}

func write(b *strings.Builder, s []uint8) {
	for _, x := range s {
		if x < 4 {
			b.WriteByte(nucleotides[x])
		} else {
			b.WriteByte('N')
		}
	}
}
