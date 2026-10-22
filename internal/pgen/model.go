// Package pgen computes the generation probability of a receptor's CDR3 under a model of V(D)J
// recombination (clonoloom-spec/requirements.md, R6), and draws receptors from such a model: the
// probability, summed over every recombination event that makes it, that recombination alone
// makes a given amino-acid (or nucleotide) CDR3, the IMGT junction from the conserved cysteine to
// the phenylalanine or tryptophan, both included (Sethna et al. 2019, Bioinformatics 35:2974,
// doi:10.1093/bioinformatics/btz035; models in IGoR's format, Marcou et al. 2018, Nat Commun
// 9:561; clonoloom-spec/research.md §5.9). ClonoLoom's own implementation, from the publications
// and the models' file format, computes it as OLGA does, its choices included, so that the two
// agree (OLGA is run only to validate it).
//
// A model gives the probability of each event: the V gene, and the D and J genes (a T-cell β
// chain's D given its J; for IGH, OLGA's P(D, J) summed over V, the model's V–(D, J) correlation
// dropped); each gene end's deletion (palindromic nucleotides as negative deletions, the gene's end
// extended by the reverse complement of as many of its nucleotides as the most negative deletion
// adds); and each insertion's length and nucleotides, a Markov chain on the previous nucleotide
// whose first nucleotide is drawn from its stationary distribution, the DJ insertion read from its
// J end. A VJ chain (TRA, IGK, IGL) has one insertion and P(V, J).
package pgen

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

// Gene is a germline gene of a model: its name, its sequence (the whole gene as the model has
// it), its anchor (a V's: the first nucleotide of the conserved cysteine's codon; a J's: of the
// phenylalanine's or tryptophan's) and whether the model uses it.
type Gene struct {
	Name   string
	Seq    string
	Anchor int
	// Used: OLGA's rule, a gene of its anchor file with an anchor above 0 whose function,
	// stripped of spaces and parentheses, is F; a gene not used makes nothing, and its probability
	// stays in the model (the others' are not made to sum to 1 again). D genes are always used.
	Used bool
}

// Model is a recombination model.
type Model struct {
	// Name says which model it is ("human_T_beta"); Locus, the chain it models.
	Name, Locus string
	VDJ         bool
	V, D, J     []Gene
	// PV: P(V) (a VDJ chain's); PJD[j][d]: P(J, D); PVJ[v][j]: a VJ chain's P(V, J).
	PV  []float64
	PJD [][]float64
	PVJ [][]float64
	// Deletions by gene, indexed by the number of nucleotides trimmed from the palindrome-extended
	// end: DelV[v][t], DelJ[j][t], DelD5[d][t5], DelD3[d][t5][t3]. PalV, PalJ, PalD5 and PalD3 are
	// the palindromic nucleotides an end is extended by: the most negative deletion's.
	DelV, DelJ, DelD5          [][]float64
	DelD3                      [][][]float64
	PalV, PalJ, PalD5, PalD3   int
	InsVD, InsDJ               []float64
	MarkovVD, MarkovDJ         [4][4]float64
	StationaryVD, StationaryDJ [4]float64

	vExt, jExt, dExt [][]uint8
}

// nucleotides in the models' order: A, C, G, T.
const nucleotides = "ACGT"

// code is a nucleotide's index (4 for anything else).
func code(b byte) uint8 {
	switch b {
	case 'A', 'a':
		return 0
	case 'C', 'c':
		return 1
	case 'G', 'g':
		return 2
	case 'T', 't':
		return 3
	}
	return 4
}

func encode(s string) []uint8 {
	out := make([]uint8, len(s))
	for i := 0; i < len(s); i++ {
		out[i] = code(s[i])
	}
	return out
}

// reverseComplement of encoded nucleotides (4 stays 4).
func reverseComplement(s []uint8) []uint8 {
	out := make([]uint8, len(s))
	for i, b := range s {
		c := uint8(4)
		if b < 4 {
			c = 3 - b
		}
		out[len(s)-1-i] = c
	}
	return out
}

// prepare makes each gene's palindrome-extended segment, the part of it the CDR3 may hold: a V's
// from its anchor, with the reverse complement of that segment's last PalV nucleotides added; a
// J's to the end of its anchor's codon, after the reverse complement of its first PalJ; a D's
// whole, between its two palindromes. A segment shorter than its palindrome gives a palindrome of
// its own length (as OLGA's do: a V allele whose gene ends at its anchor's codon, TRAV20*03).
func (m *Model) prepare() {
	m.vExt = make([][]uint8, len(m.V))
	for i, g := range m.V {
		s := encode(g.Seq)
		if g.Anchor >= 0 && g.Anchor <= len(s) {
			seg := s[g.Anchor:]
			p := min(m.PalV, len(seg))
			m.vExt[i] = append(append([]uint8{}, seg...), reverseComplement(seg[len(seg)-p:])...)
		}
	}
	m.jExt = make([][]uint8, len(m.J))
	for i, g := range m.J {
		s := encode(g.Seq)
		if end := g.Anchor + 3; g.Anchor >= 0 && end <= len(s) {
			seg := s[:end]
			p := min(m.PalJ, len(seg))
			m.jExt[i] = append(reverseComplement(seg[:p]), seg...)
		}
	}
	m.dExt = make([][]uint8, len(m.D))
	for i, g := range m.D {
		s := encode(g.Seq)
		p5, p3 := min(m.PalD5, len(s)), min(m.PalD3, len(s))
		m.dExt[i] = append(append(reverseComplement(s[:p5]), s...), reverseComplement(s[len(s)-p3:])...)
	}
}

// at answers a deletion array's probability of trimming t nucleotides from the extended end: 0
// outside it.
func at(del []float64, t int) float64 {
	if t < 0 || t >= len(del) {
		return 0
	}
	return del[t]
}

// markov makes a dinucleotide Markov chain from IGoR's 16 weights ([previous][next]): each row
// made to sum to 1, and its stationary distribution.
func markov(w [16]float64) ([4][4]float64, [4]float64, error) {
	var r [4][4]float64
	for b := 0; b < 4; b++ {
		sum := 0.0
		for a := 0; a < 4; a++ {
			sum += w[4*b+a]
		}
		if sum <= 0 {
			return r, [4]float64{}, errors.New("a Markov chain's row is empty")
		}
		for a := 0; a < 4; a++ {
			r[b][a] = w[4*b+a] / sum
		}
	}
	return r, stationary(r), nil
}

// stationary solves π R = π, Σπ = 1, by Gaussian elimination with partial pivoting on (Rᵀ − I)
// with its last row replaced by the sum.
func stationary(r [4][4]float64) [4]float64 {
	var a [4][5]float64
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			a[i][j] = r[j][i]
			if i == j {
				a[i][j]--
			}
		}
	}
	for j := 0; j < 4; j++ {
		a[3][j] = 1
	}
	a[3][4] = 1
	for c := 0; c < 4; c++ {
		p := c
		for i := c + 1; i < 4; i++ {
			if abs(a[i][c]) > abs(a[p][c]) {
				p = i
			}
		}
		a[c], a[p] = a[p], a[c]
		for i := 0; i < 4; i++ {
			if i == c || a[c][c] == 0 {
				continue
			}
			f := a[i][c] / a[c][c]
			for j := c; j < 5; j++ {
				a[i][j] -= float64(f * a[c][j])
			}
		}
	}
	var out [4]float64
	for i := 0; i < 4; i++ {
		out[i] = a[i][4] / a[i][i]
	}
	return out
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// --- IGoR's files --------------------------------------------------------------------------------

// event is an event of a model_params.txt: its type and its realizations (a gene's name and
// sequence, a deletion's or insertion's value, a nucleotide), by realization index.
type event struct {
	kind  string
	names []string
	seqs  []string
	value []int
}

// marginal is an event's probabilities in model_marginals.txt: its dimensions (its parents', then
// its own), its parents' nicknames, and the values row-major.
type marginal struct {
	dims    []int
	parents []string
	values  []float64
}

func (m marginal) get(idx ...int) float64 {
	k := 0
	for i, x := range idx {
		k = k*m.dims[i] + x
	}
	return m.values[k]
}

func readParams(r io.Reader) (map[string]*event, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<16), 1<<22)
	events := map[string]*event{}
	var cur *event
	section := ""
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		switch {
		case strings.HasPrefix(line, "@"):
			section, cur = line, nil
		case section != "@Event_list":
		case strings.HasPrefix(line, "#"):
			f := strings.Split(line[1:], ";")
			if len(f) < 5 {
				return nil, fmt.Errorf("model_params.txt: an event %q without its nickname", line)
			}
			cur = &event{kind: f[0]}
			events[strings.TrimSpace(f[len(f)-1])] = cur
		case strings.HasPrefix(line, "%") && cur != nil:
			f := strings.Split(line[1:], ";")
			index, err := strconv.Atoi(strings.TrimSpace(f[len(f)-1]))
			if err != nil || index < 0 || index > 1<<16 {
				return nil, fmt.Errorf("model_params.txt: a realization %q without its index", line)
			}
			for len(cur.names) <= index {
				cur.names, cur.seqs, cur.value = append(cur.names, ""), append(cur.seqs, ""), append(cur.value, 0)
			}
			cur.names[index] = strings.TrimSpace(f[0])
			switch cur.kind {
			case "GeneChoice":
				if len(f) < 3 {
					return nil, fmt.Errorf("model_params.txt: a gene %q without its sequence", line)
				}
				cur.seqs[index] = strings.ToUpper(strings.TrimSpace(f[1]))
			case "Deletion", "Insertion":
				v, err := strconv.Atoi(strings.TrimSpace(f[0]))
				if err != nil {
					return nil, fmt.Errorf("model_params.txt: %q is not a number of nucleotides", line)
				}
				cur.value[index] = v
			}
		}
	}
	return events, sc.Err()
}

func readMarginals(r io.Reader) (map[string]*marginal, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<16), 1<<24)
	out := map[string]*marginal{}
	var cur *marginal
	var row []int
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		switch {
		case strings.HasPrefix(line, "@"):
			cur = &marginal{}
			out[strings.TrimSpace(line[1:])] = cur
		case cur == nil:
		case strings.HasPrefix(line, "$Dim["):
			n := 1
			for _, f := range strings.Split(strings.TrimSuffix(strings.TrimPrefix(line, "$Dim["), "]"), ",") {
				d, err := strconv.Atoi(strings.TrimSpace(f))
				if err != nil || d <= 0 || d > 1<<12 {
					return nil, fmt.Errorf("model_marginals.txt: dimensions %q", line)
				}
				cur.dims = append(cur.dims, d)
				n *= d
				if n > 1<<24 {
					return nil, errors.New("model_marginals.txt: an event too large")
				}
			}
			cur.values = make([]float64, n)
		case strings.HasPrefix(line, "#"):
			row = row[:0]
			var parents []string
			for _, part := range strings.Split(strings.TrimPrefix(line, "#"), "],[") {
				part = strings.Trim(part, "[] ")
				if part == "" {
					continue
				}
				name, idx, ok := strings.Cut(part, ",")
				i, err := strconv.Atoi(strings.TrimSpace(idx))
				if !ok || err != nil {
					return nil, fmt.Errorf("model_marginals.txt: a row's parents %q", line)
				}
				parents = append(parents, strings.TrimSpace(name))
				row = append(row, i)
			}
			if cur.parents == nil {
				cur.parents = parents
			}
		case strings.HasPrefix(line, "%"):
			if cur.dims == nil || len(row) != len(cur.dims)-1 {
				return nil, fmt.Errorf("model_marginals.txt: values without their dimensions or parents")
			}
			k := 0
			for i, x := range row {
				if x < 0 || x >= cur.dims[i] {
					return nil, fmt.Errorf("model_marginals.txt: a parent's index %d out of range", x)
				}
				k = k*cur.dims[i] + x
			}
			last := cur.dims[len(cur.dims)-1]
			values := strings.Split(line[1:], ",")
			if len(values) != last {
				return nil, fmt.Errorf("model_marginals.txt: %d values for %d realizations", len(values), last)
			}
			for i, v := range values {
				x, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
				if err != nil || x < 0 {
					return nil, fmt.Errorf("model_marginals.txt: %q is not a probability", v)
				}
				cur.values[k*last+i] = x
			}
		}
	}
	return out, sc.Err()
}

// readAnchors reads an anchor file of OLGA's (gene,anchor_index,function, a header first): each
// gene's anchor and function.
func readAnchors(r io.Reader) (map[string]int, map[string]string, error) {
	sc := bufio.NewScanner(r)
	anchors, functions := map[string]int{}, map[string]string{}
	first := true
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if first {
			first = false
			continue
		}
		f := strings.Split(line, ",")
		if len(f) < 2 {
			continue
		}
		a, err := strconv.Atoi(strings.TrimSpace(f[1]))
		if err != nil {
			continue
		}
		anchors[strings.TrimSpace(f[0])] = a
		if len(f) > 2 {
			functions[strings.TrimSpace(f[0])] = strings.TrimSpace(f[2])
		}
	}
	return anchors, functions, sc.Err()
}

// ReadIGoR reads a model in IGoR's format as OLGA distributes its default models: its
// model_params.txt, model_marginals.txt and the V and J genes' anchor files.
func ReadIGoR(name string, params, marginals, vAnchors, jAnchors io.Reader) (*Model, error) {
	events, err := readParams(params)
	if err != nil {
		return nil, err
	}
	probs, err := readMarginals(marginals)
	if err != nil {
		return nil, err
	}
	va, vf, err := readAnchors(vAnchors)
	if err != nil {
		return nil, err
	}
	ja, jf, err := readAnchors(jAnchors)
	if err != nil {
		return nil, err
	}
	need := func(nick string) (*event, *marginal, error) {
		e, m := events[nick], probs[nick]
		if e == nil || m == nil {
			return nil, nil, fmt.Errorf("the model has no %s", nick)
		}
		if e.kind != "DinucMarkov" && m.dims[len(m.dims)-1] != len(e.names) {
			return nil, nil, fmt.Errorf("%s: %d realizations, %d probabilities", nick, len(e.names), m.dims[len(m.dims)-1])
		}
		return e, m, nil
	}
	m := &Model{Name: name}
	ve, vm, err := need("v_choice")
	if err != nil {
		return nil, err
	}
	je, jm, err := need("j_choice")
	if err != nil {
		return nil, err
	}
	genes := func(e *event, anchors map[string]int, functions map[string]string) []Gene {
		out := make([]Gene, len(e.names))
		for i, n := range e.names {
			g := Gene{Name: n, Seq: e.seqs[i], Anchor: -1}
			if a, ok := anchors[n]; ok {
				g.Anchor = a
				fn := strings.Trim(strings.ReplaceAll(functions[n], " ", ""), "()")
				g.Used = a > 0 && fn == "F" && a+3 <= len(g.Seq)
			}
			out[i] = g
		}
		return out
	}
	m.V, m.J = genes(ve, va, vf), genes(je, ja, jf)
	if len(vm.dims) != 1 {
		return nil, errors.New("v_choice is conditioned on another event, which no model of OLGA's does")
	}
	m.PV = vm.values
	_, hasD := events["d_gene"]
	m.VDJ = hasD
	if m.Locus = locusOf(m.V); m.Locus == "" {
		m.Locus = locusOf(m.J)
	}
	if m.VDJ {
		de, dm, err := need("d_gene")
		if err != nil {
			return nil, err
		}
		m.D = make([]Gene, len(de.names))
		for i, n := range de.names {
			m.D[i] = Gene{Name: n, Seq: de.seqs[i], Used: true}
		}
		nV, nJ, nD := len(m.V), len(m.J), len(m.D)
		m.PJD = make([][]float64, nJ)
		for j := range m.PJD {
			m.PJD[j] = make([]float64, nD)
		}
		switch {
		case len(jm.dims) == 1 && len(dm.dims) == 2 && dm.parents[0] == "j_choice":
			// P(J) P(D | J): T cells.
			for j := 0; j < nJ; j++ {
				for d := 0; d < nD; d++ {
					m.PJD[j][d] = float64(jm.get(j) * dm.get(j, d))
				}
			}
		case len(jm.dims) == 2 && jm.parents[0] == "v_choice" && len(dm.dims) == 3 && dm.parents[0] == "v_choice" && dm.parents[1] == "j_choice":
			// IGH: P(D, J) = Σ_V P(V) P(J | V) P(D | V, J), as OLGA takes it.
			for v := 0; v < nV; v++ {
				for j := 0; j < nJ; j++ {
					pvj := float64(m.PV[v] * jm.get(v, j))
					for d := 0; d < nD; d++ {
						m.PJD[j][d] += float64(pvj * dm.get(v, j, d))
					}
				}
			}
		default:
			return nil, errors.New("the model's D and J genes are conditioned in a way ClonoLoom does not read")
		}
	} else {
		if len(jm.dims) != 2 || jm.parents[0] != "v_choice" {
			return nil, errors.New("a VJ model's J gene must be conditioned on its V gene")
		}
		m.PVJ = make([][]float64, len(m.V))
		for v := range m.V {
			m.PVJ[v] = make([]float64, len(m.J))
			for j := range m.J {
				m.PVJ[v][j] = float64(m.PV[v] * jm.get(v, j))
			}
		}
	}
	// Deletions: arrays by value, from the most negative (the palindrome's length).
	deletion := func(nick, parent string) ([][]float64, int, error) {
		e, dm, err := need(nick)
		if err != nil {
			return nil, 0, err
		}
		if len(dm.dims) != 2 || dm.parents[0] != parent {
			return nil, 0, fmt.Errorf("%s is not conditioned on %s alone", nick, parent)
		}
		lo, pos, err := byValue(e)
		if err != nil {
			return nil, 0, fmt.Errorf("%s: %w", nick, err)
		}
		out := make([][]float64, dm.dims[0])
		for g := range out {
			out[g] = make([]float64, len(pos))
			for v, ri := range pos {
				out[g][v] = dm.get(g, ri)
			}
		}
		return out, -lo, nil
	}
	if m.DelV, m.PalV, err = deletion("v_3_del", "v_choice"); err != nil {
		return nil, err
	}
	if m.DelJ, m.PalJ, err = deletion("j_5_del", "j_choice"); err != nil {
		return nil, err
	}
	if m.VDJ {
		if m.DelD5, m.PalD5, err = deletion("d_5_del", "d_gene"); err != nil {
			return nil, err
		}
		e3, d3, err := need("d_3_del")
		if err != nil {
			return nil, err
		}
		if len(d3.dims) != 3 || d3.parents[0] != "d_gene" || d3.parents[1] != "d_5_del" {
			return nil, errors.New("d_3_del is not conditioned on d_gene and d_5_del")
		}
		lo3, pos3, err := byValue(e3)
		if err != nil {
			return nil, fmt.Errorf("d_3_del: %w", err)
		}
		_, pos5, _ := byValue(events["d_5_del"])
		m.PalD3 = -lo3
		m.DelD3 = make([][][]float64, len(m.D))
		for d := range m.D {
			m.DelD3[d] = make([][]float64, len(pos5))
			for v5, r5 := range pos5 {
				m.DelD3[d][v5] = make([]float64, len(pos3))
				for v3, r3 := range pos3 {
					m.DelD3[d][v5][v3] = d3.get(d, r5, r3)
				}
			}
		}
	}
	insertion := func(nick string) ([]float64, error) {
		e, im, err := need(nick)
		if err != nil {
			return nil, err
		}
		lo, pos, err := byValue(e)
		if err != nil || lo != 0 {
			return nil, fmt.Errorf("%s: insertion lengths must run from 0", nick)
		}
		out := make([]float64, len(pos))
		for l, ri := range pos {
			out[l] = im.get(ri)
		}
		return out, nil
	}
	chain := func(nick string) ([4][4]float64, [4]float64, error) {
		e, mm, err := need(nick)
		if err != nil {
			return [4][4]float64{}, [4]float64{}, err
		}
		var w [16]float64
		if len(e.names) != 4 || len(mm.values) != 16 {
			return [4][4]float64{}, [4]float64{}, fmt.Errorf("%s: not 4 nucleotides", nick)
		}
		for b := 0; b < 4; b++ {
			for a := 0; a < 4; a++ {
				w[4*int(code(e.names[b][0]))+int(code(e.names[a][0]))] = mm.values[4*b+a]
			}
		}
		return markov(w)
	}
	if m.VDJ {
		if m.InsVD, err = insertion("vd_ins"); err != nil {
			return nil, err
		}
		if m.InsDJ, err = insertion("dj_ins"); err != nil {
			return nil, err
		}
		if m.MarkovVD, m.StationaryVD, err = chain("vd_dinucl"); err != nil {
			return nil, err
		}
		if m.MarkovDJ, m.StationaryDJ, err = chain("dj_dinucl"); err != nil {
			return nil, err
		}
	} else {
		if m.InsVD, err = insertion("vj_ins"); err != nil {
			return nil, err
		}
		if m.MarkovVD, m.StationaryVD, err = chain("vj_dinucl"); err != nil {
			return nil, err
		}
	}
	m.prepare()
	return m, nil
}

// byValue answers a deletion's or insertion's realizations ordered by value: its least value, and
// the realization index of each value from it (an error unless the values are contiguous).
func byValue(e *event) (int, []int, error) {
	if e == nil || len(e.value) == 0 {
		return 0, nil, errors.New("no realizations")
	}
	order := make([]int, len(e.value))
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(a, b int) bool { return e.value[order[a]] < e.value[order[b]] })
	lo := e.value[order[0]]
	for k, ri := range order {
		if e.value[ri] != lo+k {
			return 0, nil, errors.New("the values are not contiguous")
		}
	}
	return lo, order, nil
}

// locusOf is the locus genes' names give (their first three letters).
func locusOf(genes []Gene) string {
	for _, g := range genes {
		if len(g.Name) >= 3 {
			switch l := strings.ToUpper(g.Name[:3]); l {
			case "TRA", "TRB", "TRG", "TRD", "IGH", "IGK", "IGL":
				return l
			}
		}
	}
	return ""
}
