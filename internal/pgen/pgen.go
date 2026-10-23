package pgen

import (
	"context"
	"errors"
	"fmt"
	"math"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
)

// The CDR3 is read nucleotide by nucleotide, 3L of them for L codons, each codon allowed a set of
// codons: an amino acid's (or an ambiguous symbol's), or one codon for a nucleotide CDR3. A
// position's state is what its codon holds so far: at a codon's first nucleotide, that nucleotide
// (4 states); at its second, the two (16); at its third, the codon is checked and only its last
// nucleotide kept (4), which the next insertion's Markov chain needs. A vector holds the summed
// probability of the events so far in each state.
//
// Forward, from the CDR3's start: each V gene's nucleotides from its anchor to where its deletion
// ends it (at least one), then the first insertion, then (VDJ) each D gene's nucleotides between
// its two deletions, or none (a D deleted whole, only by deletions that sum to its extended
// length), then the second insertion. Backward, from the CDR3's end: each J gene's nucleotides
// from where its deletion starts it (at least one) to the end of its anchor's codon. The two meet
// where the J starts.

type vec [16]float64

// codonOf is each codon's amino acid (codons indexed n0·16 + n1·4 + n2, A C G T).
var codonOf = func() (out [64]byte) {
	const bases, aas = "TCAG", "FFLLSSSSYY**CC*WLLLLPPPPHHQQRRRRIIIMTTTTNNKKSSRRVVVVAAAADDEEGGGG"
	i := 0
	for _, a := range bases {
		for _, b := range bases {
			for _, c := range bases {
				out[int(code(byte(a)))*16+int(code(byte(b)))*4+int(code(byte(c)))] = aas[i]
				i++
			}
		}
	}
	return
}()

// symbols are each amino-acid symbol's codons: the 20 amino acids, * (a stop), and OLGA's
// ambiguous B (D or N), J (I or L), Z (E or Q) and X (any amino acid, no stop).
var symbols = func() map[byte]uint64 {
	out := map[byte]uint64{}
	for c, aa := range codonOf {
		out[aa] |= 1 << c
	}
	out['B'] = out['D'] | out['N']
	out['J'] = out['I'] | out['L']
	out['Z'] = out['E'] | out['Q']
	var any uint64
	for aa, m := range out {
		if aa >= 'A' && aa <= 'Z' && aa != 'B' && aa != 'J' && aa != 'Z' && aa != 'X' {
			any |= m
		}
	}
	out['X'] = any
	return out
}()

// Translate translates whole codons of nucleotides (X for a codon with another character).
func Translate(nt string) string {
	b := make([]byte, 0, len(nt)/3)
	for i := 0; i+3 <= len(nt); i += 3 {
		x, y, z := code(nt[i]), code(nt[i+1]), code(nt[i+2])
		if x > 3 || y > 3 || z > 3 {
			b = append(b, 'X')
			continue
		}
		b = append(b, codonOf[int(x)*16+int(y)*4+int(z)])
	}
	return string(b)
}

// AminoAcids reads an amino-acid CDR3 into its codons' sets.
func AminoAcids(cdr3 string) ([]uint64, error) {
	out := make([]uint64, len(cdr3))
	for i := 0; i < len(cdr3); i++ {
		c := cdr3[i]
		if c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		m, ok := symbols[c]
		if !ok {
			return nil, fmt.Errorf("%q is not an amino acid", cdr3[i])
		}
		out[i] = m
	}
	return out, nil
}

// Nucleotides reads a nucleotide CDR3 (whole codons) into its codons, one each.
func Nucleotides(cdr3 string) ([]uint64, error) {
	if len(cdr3)%3 != 0 {
		return nil, errors.New("a nucleotide CDR3 must be whole codons")
	}
	out := make([]uint64, len(cdr3)/3)
	for i := range out {
		x, y, z := code(cdr3[3*i]), code(cdr3[3*i+1]), code(cdr3[3*i+2])
		if x > 3 || y > 3 || z > 3 {
			return nil, fmt.Errorf("%q is not A, C, G or T", cdr3[3*i:3*i+3])
		}
		out[i] = 1 << (int(x)*16 + int(y)*4 + int(z))
	}
	return out, nil
}

// run is one CDR3's computation: its positions' transitions.
type run struct {
	m *Model
	n int
	// tr[i][s][b]: the state at position i after nucleotide b from state s at i − 1, or −1.
	tr [][16][4]int8
}

func newRun(m *Model, codons []uint64) *run {
	r := &run{m: m, n: 3 * len(codons), tr: make([][16][4]int8, 3*len(codons))}
	for i := 0; i < r.n; i++ {
		allowed := codons[i/3]
		for s := 0; s < 16; s++ {
			for b := 0; b < 4; b++ {
				next := int8(-1)
				switch i % 3 {
				case 0:
					if s < 4 || i == 0 {
						for c := 0; c < 16; c++ {
							if allowed&(1<<(b*16+c)) != 0 {
								next = int8(b)
								break
							}
						}
					}
				case 1:
					if s < 4 {
						for c := 0; c < 4; c++ {
							if allowed&(1<<(s*16+b*4+c)) != 0 {
								next = int8(s*4 + b)
								break
							}
						}
					}
				case 2:
					if allowed&(1<<(s*4+b)) != 0 {
						next = int8(b)
					}
				}
				r.tr[i][s][b] = next
			}
		}
	}
	return r
}

// last is the last nucleotide a state at position i holds.
func last(i, s int) int {
	if i%3 == 1 {
		return s & 3
	}
	return s
}

// step moves a vector at position i − 1 through a germline nucleotide at i.
func (r *run) step(v *vec, i int, b uint8) (out vec, alive bool) {
	if b > 3 {
		return out, false
	}
	for s, x := range v {
		if x == 0 {
			continue
		}
		if t := r.tr[i][s][b]; t >= 0 {
			out[t] += x
			alive = true
		}
	}
	return out, alive
}

// insert extends the vectors at each start position by an insertion of 0 or more nucleotides
// (every length the model gives: OLGA 1.3.0 sums them all, where earlier versions left out the
// longest by their codon phase): each length weighted by ins, its nucleotides by a Markov chain
// (the first by first, each next by chain[previous][next]), and the last by final when given. It
// answers the vectors at each end.
func (r *run) insert(from []vec, ins []float64, first [4]float64, chain *[4][4]float64, final *[4]float64) []vec {
	out := make([]vec, r.n)
	for start := range from {
		if isZero(&from[start]) {
			continue
		}
		addScaled(&out[start], &from[start], ins[0])
		cur := from[start]
		for l := 1; l < len(ins) && start+l < r.n-1; l++ {
			i := start + l
			var next vec
			moved := false
			for s, x := range cur {
				if x == 0 {
					continue
				}
				prev := last(i-1, s)
				for b := 0; b < 4; b++ {
					t := r.tr[i][s][b]
					if t < 0 {
						continue
					}
					w := first[b]
					if l > 1 {
						w = chain[prev][b]
					}
					next[t] += float64(x * w)
					moved = true
				}
			}
			if !moved {
				break
			}
			cur = next
			if ins[l] == 0 {
				continue
			}
			if final == nil {
				addScaled(&out[i], &cur, ins[l])
				continue
			}
			for s, x := range cur {
				if x != 0 {
					out[i][s] += float64(float64(x*final[last(i, s)]) * ins[l])
				}
			}
		}
	}
	return out
}

func isZero(v *vec) bool {
	for _, x := range v {
		if x != 0 {
			return false
		}
	}
	return true
}

func addScaled(dst, src *vec, w float64) {
	if w == 0 {
		return
	}
	for s, x := range src {
		if x != 0 {
			dst[s] += float64(x * w)
		}
	}
}

func dot(a, b *vec) float64 {
	sum := 0.0
	for s := range a {
		if a[s] != 0 && b[s] != 0 {
			sum += float64(a[s] * b[s])
		}
	}
	return sum
}

// vStage answers the V genes' vectors at each position where a V can end: Σ_V weight(V) ×
// P(deletion | V), weight(V) as given (P(V), or 1 to keep each V apart).
func (r *run) vStage(v int, weight float64, into []vec) {
	m := r.m
	ext := m.vExt[v]
	s := 0
	for p := 0; p < len(ext) && p < r.n-1; p++ {
		b := ext[p]
		if b > 3 {
			return
		}
		t := r.tr[p][s][b]
		if t < 0 {
			return
		}
		s = int(t)
		if w := at(m.DelV[v], len(ext)-(p+1)); w != 0 {
			into[p][s] += float64(weight * w)
		}
	}
}

// jStage answers a J gene's backward vectors: at position i (the J starting at i + 1), the
// probability of its deletion for each state whose codon the J completes.
func (r *run) jStage(j int) ([]vec, bool) {
	m := r.m
	ext := m.jExt[j]
	off := len(ext) - r.n
	nt := func(i int) uint8 {
		if off+i < 0 {
			return 4
		}
		return ext[off+i]
	}
	codons := r.n / 3
	// whole[c]: codons c and after, all of the J's nucleotides, allowed.
	whole := make([]bool, codons+1)
	whole[codons] = true
	for c := codons - 1; c >= 0; c-- {
		x, y, z := nt(3*c), nt(3*c+1), nt(3*c+2)
		whole[c] = whole[c+1] && x < 4 && y < 4 && z < 4 && r.tr[3*c+2][int(x)*4+int(y)][z] >= 0 && r.tr[3*c+1][int(x)][y] >= 0 && r.tr[3*c][0][x] >= 0
	}
	out := make([]vec, r.n)
	found := false
	for start := max(1, -off); start < r.n; start++ {
		w := at(m.DelJ[j], off+start)
		if w == 0 {
			continue
		}
		c := (start + 2) / 3
		if !whole[c] {
			continue
		}
		i := start - 1
		phase := i % 3
		states := 4
		if phase == 1 {
			states = 16
		}
		for s := 0; s < states; s++ {
			ok, cur := true, s
			for k := start; k < 3*c; k++ {
				b := nt(k)
				if b > 3 || r.tr[k][cur][b] < 0 {
					ok = false
					break
				}
				cur = int(r.tr[k][cur][b])
			}
			if ok {
				out[i][s] += w
				found = true
			}
		}
	}
	return out, found
}

// prob is the generation probability of a CDR3 given as its codons' sets, with V and J masks
// (nil: every gene).
func (m *Model) prob(codons []uint64, vMask, jMask []bool) float64 {
	if len(codons) == 0 {
		return 0
	}
	r := newRun(m, codons)
	n := r.n
	usedJ := func(j int) bool { return m.J[j].Used && (jMask == nil || jMask[j]) && m.jExt[j] != nil }
	usedV := func(v int) bool { return m.V[v].Used && (vMask == nil || vMask[v]) && m.vExt[v] != nil }
	// The J genes first: most cannot end this CDR3, and are skipped.
	bj := make([][]vec, len(m.J))
	for j := range m.J {
		if usedJ(j) {
			if b, found := r.jStage(j); found {
				bj[j] = b
			}
		}
	}
	if !m.VDJ {
		total := 0.0
		// Each V's own vector, then for each J the V's weighted by P(V, J).
		vs := make([][]vec, len(m.V))
		for v := range m.V {
			if !usedV(v) {
				continue
			}
			vs[v] = make([]vec, n)
			r.vStage(v, 1, vs[v])
		}
		for j := range m.J {
			if bj[j] == nil {
				continue
			}
			from := make([]vec, n)
			found := false
			for v := range m.V {
				if vs[v] == nil || m.PVJ[v][j] == 0 {
					continue
				}
				for p := range from {
					if !isZero(&vs[v][p]) {
						addScaled(&from[p], &vs[v][p], m.PVJ[v][j])
						found = true
					}
				}
			}
			if !found {
				continue
			}
			ends := r.insert(from, m.InsVD, m.StationaryVD, &m.MarkovVD, nil)
			for i := range ends {
				total += dot(&ends[i], &bj[j][i])
			}
		}
		return total
	}
	// VDJ: the V genes and the first insertion, shared by every D.
	av := make([]vec, n)
	for v := range m.V {
		if usedV(v) && m.PV[v] != 0 {
			r.vStage(v, m.PV[v], av)
		}
	}
	n1 := r.insert(av, m.InsVD, m.StationaryVD, &m.MarkovVD, nil)
	// The DJ insertion is read from its J end: its chain's previous nucleotide is the next one
	// along the CDR3.
	var reversed [4][4]float64
	for a := 0; a < 4; a++ {
		for b := 0; b < 4; b++ {
			reversed[a][b] = m.MarkovDJ[b][a]
		}
	}
	ones := [4]float64{1, 1, 1, 1}
	total := 0.0
	for d := range m.D {
		// The J genes' backward vectors weighted by P(J, D).
		back := make([]vec, n)
		found := false
		for j := range m.J {
			if bj[j] == nil || m.PJD[j][d] == 0 {
				continue
			}
			for i := range back {
				if !isZero(&bj[j][i]) {
					addScaled(&back[i], &bj[j][i], m.PJD[j][d])
					found = true
				}
			}
		}
		if !found {
			continue
		}
		dEnds := r.dStage(d, n1)
		ends := r.insert(dEnds, m.InsDJ, ones, &reversed, &m.StationaryDJ)
		for i := range ends {
			total += dot(&ends[i], &back[i])
		}
	}
	return total
}

// dStage extends the vectors at each end of the first insertion by a D gene's nucleotides
// between its deletions (P(5′ deletion) P(3′ deletion | 5′)), or by none when it is deleted
// whole.
func (r *run) dStage(d int, from []vec) []vec {
	m := r.m
	ext := m.dExt[d]
	d3 := func(t5, t3 int) float64 {
		if t5 < 0 || t5 >= len(m.DelD3[d]) {
			return 0
		}
		return at(m.DelD3[d][t5], t3)
	}
	// Deleted whole: only by deletions that sum to the extended length.
	whole := 0.0
	for t5 := 0; t5 <= len(ext); t5++ {
		if w5 := at(m.DelD5[d], t5); w5 != 0 {
			whole += float64(w5 * d3(t5, len(ext)-t5))
		}
	}
	out := make([]vec, r.n)
	for e := range from {
		if isZero(&from[e]) {
			continue
		}
		addScaled(&out[e], &from[e], whole)
		for t5 := 0; t5 < len(ext); t5++ {
			w5 := at(m.DelD5[d], t5)
			if w5 == 0 {
				continue
			}
			cur := from[e]
			for k, i := t5, e+1; k < len(ext) && i < r.n-1; k, i = k+1, i+1 {
				var alive bool
				cur, alive = r.step(&cur, i, ext[k])
				if !alive {
					break
				}
				if w3 := d3(t5, len(ext)-(k+1)); w3 != 0 {
					addScaled(&out[i], &cur, float64(w5*w3))
				}
			}
		}
	}
	return out
}

// AA is the generation probability of an amino-acid CDR3 (the junction, both conserved residues
// included), with V and J masks (nil: every gene; see Mask).
func (m *Model) AA(cdr3 string, vMask, jMask []bool) (float64, error) {
	codons, err := AminoAcids(cdr3)
	if err != nil {
		return 0, err
	}
	return m.prob(codons, vMask, jMask), nil
}

// NT is the generation probability of a nucleotide CDR3.
func (m *Model) NT(cdr3 string, vMask, jMask []bool) (float64, error) {
	codons, err := Nucleotides(cdr3)
	if err != nil {
		return 0, err
	}
	return m.prob(codons, vMask, jMask), nil
}

// Mask answers the model's V (segment 'V') or J ('J') genes a call names: its genes (each of a
// tie, separated by commas), each by its allele when it names one, else every allele of the gene.
// A call naming none of the model's genes is an error (OLGA would quietly use every gene).
func (m *Model) Mask(segment byte, call string) ([]bool, error) {
	genes := m.V
	if segment == 'J' {
		genes = m.J
	}
	mask := make([]bool, len(genes))
	found := false
	for _, name := range strings.FieldsFunc(call, func(r rune) bool { return r == ',' || r == '|' }) {
		name = strings.TrimSpace(name)
		for i, g := range genes {
			if g.Name == name || (!strings.Contains(name, "*") && strings.HasPrefix(g.Name, name+"*")) {
				mask[i], found = true, true
			}
		}
	}
	if !found {
		return nil, fmt.Errorf("the model %s has no gene %s", m.Name, call)
	}
	return mask, nil
}

// Query is a CDR3 to compute: its sequence (amino acids, or nucleotides when NT) and its V and J
// masks (nil: every gene).
type Query struct {
	CDR3 string
	NT   bool
	V, J []bool
}

// All computes queries' generation probabilities on workers goroutines (0: one for each processor),
// in their order; a CDR3 that cannot be read is NaN. The answer does not depend on the workers.
func (m *Model) All(ctx context.Context, queries []Query, workers int, progress func(done, of int)) ([]float64, error) {
	if workers <= 0 {
		workers = runtime.GOMAXPROCS(0)
	}
	out := make([]float64, len(queries))
	var next, done atomic.Int64
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				i := int(next.Add(1) - 1)
				if i >= len(queries) {
					return
				}
				q := queries[i]
				var p float64
				var err error
				if q.NT {
					p, err = m.NT(q.CDR3, q.V, q.J)
				} else {
					p, err = m.AA(q.CDR3, q.V, q.J)
				}
				if err != nil {
					p = math.NaN()
				}
				out[i] = p
				if d := done.Add(1); progress != nil && d%1024 == 0 {
					progress(int(d), len(queries))
				}
			}
		}()
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
