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
	m              *Model
	v, jd, vj      []float64
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
	return g
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
		var b strings.Builder
		vExt, jExt := m.vExt[v], m.jExt[j]
		tv, tj := pick(r, g.delV[v]), pick(r, g.delJ[j])
		if tv < 0 || tj < 0 || tv > len(vExt) || tj > len(jExt) {
			continue
		}
		write(&b, vExt[:len(vExt)-tv])
		b.WriteString(g.insertion(r, g.insVD, g.firstVD, &g.chainVD, false))
		if m.VDJ {
			dExt := m.dExt[d]
			t5 := pick(r, g.delD5[d])
			if t5 < 0 || t5 >= len(g.delD3[d]) {
				continue
			}
			t3 := pick(r, g.delD3[d][t5])
			if t3 < 0 || t5+t3 > len(dExt) {
				continue
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
		return out
	}
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
