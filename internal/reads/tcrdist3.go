package reads

import (
	"math"
	"strconv"
	"strings"

	"clonoloom/internal/rep"
)

// tcrdist3's clone tables (Mayer-Blackwell et al. 2021, eLife 10:e68605; the columns of the
// epitope-specific TCRs of Dash et al. 2017 as tcrdist3 distributes them, dash.csv), comma- or
// tab-separated, one row per clone: for each chain it has (a, b, g, d: TRA, TRB, TRG, TRD) its V
// and J genes (v_b_gene, j_b_gene), its junction's amino acids (cdr3_b_aa: tcrdist3's CDR3 is the
// IMGT junction) and, where given, nucleotides (cdr3_b_nucseq); the clone's count, and the
// subject, epitope and clone_id where given.
//
//   - A table of paired chains is single-cell data: each clone's count is its cells, and the clone
//     is read as that many cells with the same chains (a count that is not a whole number from 1
//     to 100,000 is read as one cell, and reported). The clone_id is each cell's clone, so that the
//     source's own clonotypes are its clones.
//   - A table of one chain is a bulk repertoire, each clone a rearrangement counted by its count,
//     whose unit the table does not say (tcrdist3 counts cells, templates or reads alike): the user
//     confirms it.
//
// A clone's sample is its subject's, one for each subject and epitope, the subject and epitope
// kept as the sample's metadata; without a subject column, the file's. Junctions are productive
// when their amino acids hold no stop codon (and their nucleotides, where given, whole codons).

// tcrdistChains are the chain letters of tcrdist3's columns, and their loci.
var tcrdistChains = []struct{ letter, locus string }{{"a", "TRA"}, {"b", "TRB"}, {"g", "TRG"}, {"d", "TRD"}}

type tcrdistChain struct {
	locus                  string
	v, j, junctionAA, junc int
}

type tcrdistLayout struct {
	chains                  []tcrdistChain
	count, subject, epitope int
	cloneID                 int
}

// isTCRdistHeader: a junction and V gene column of tcrdist3's naming for some chain.
func isTCRdistHeader(header []string) bool {
	return len(newTCRdistLayout(header).chains) > 0
}

func newTCRdistLayout(header []string) *tcrdistLayout {
	c := newColumns(header)
	l := &tcrdistLayout{count: c.index("count"), subject: c.index("subject"), epitope: c.index("epitope"), cloneID: c.index("clone_id")}
	for _, ch := range tcrdistChains {
		aa, v := c.index("cdr3_"+ch.letter+"_aa"), c.index("v_"+ch.letter+"_gene")
		if aa < 0 || v < 0 {
			continue
		}
		l.chains = append(l.chains, tcrdistChain{locus: ch.locus, v: v, j: c.index("j_" + ch.letter + "_gene"), junctionAA: aa, junc: c.index("cdr3_" + ch.letter + "_nucseq")})
	}
	return l
}

// maxCloneCells: the most cells one row of a paired table is read as.
const maxCloneCells = 100000

// readTCRdist reads a tcrdist3 clone table's rows into b, the fields split by split. It answers
// the unit: cells for paired chains, unknown for one chain.
func readTCRdist(ls *lines, l *tcrdistLayout, width int, split func([]byte, []string) ([]string, bool), b *rep.Builder, src int, sample string, p *Problems) (rep.Unit, error) {
	paired := len(l.chains) > 1
	declared := map[string]bool{}
	var fields []string
	for ls.next() {
		if len(ls.line) == 0 {
			continue
		}
		var ok bool
		fields, ok = split(ls.line, fields)
		if !ok {
			p.addf("bad-csv", src, ls.n, "", "", "the row's quotes are unbalanced")
			continue
		}
		if len(fields) != width {
			p.addf("field-count", src, ls.n, "", "", "the row has %d fields, the header %d", len(fields), width)
			if len(fields) < width {
				continue
			}
		}
		name := sample
		subject, epitope := field(fields, l.subject), field(fields, l.epitope)
		if subject != "" {
			name = subject
			if epitope != "" {
				name += "_" + epitope
			}
		}
		if !declared[name] {
			declared[name] = true
			meta := map[string]string{}
			if subject != "" {
				meta["subject"] = subject
			}
			if epitope != "" {
				meta["epitope"] = epitope
			}
			b.AddSample(rep.Sample{Name: name, Source: src, Metadata: meta})
		}
		clone := field(fields, l.cloneID)
		raw := field(fields, l.count)
		n := count(raw)
		if l.count < 0 {
			n = 1
		}
		if !paired {
			r := tcrdistRow(l.chains[0], fields, src, ls.n, name)
			r.SequenceID, r.Clone, r.Count = clone, clone, n
			if err := b.Append(&r); err != nil {
				return "", err
			}
			continue
		}
		cells := 1
		if n >= 1 && n <= maxCloneCells && n == math.Trunc(n) {
			cells = int(n)
		} else {
			p.addf("cell-count", src, ls.n, "count", raw, "a clone's count of cells is %q: read as one cell", raw)
		}
		id := clone
		if id == "" {
			id = "row " + strconv.Itoa(int(ls.n))
		}
		for k := 1; k <= cells; k++ {
			cell := id
			if cells > 1 {
				cell = id + "#" + strconv.Itoa(k)
			}
			for _, ch := range l.chains {
				r := tcrdistRow(ch, fields, src, ls.n, name)
				if r.V == "" && r.JunctionAA == "" {
					continue // a cell without this chain
				}
				r.Cell, r.IsCell, r.Clone, r.Count = cell, rep.True, clone, 1
				r.SequenceID = cell + "_" + strings.ToLower(r.Locus[2:])
				if err := b.Append(&r); err != nil {
					return "", err
				}
			}
		}
	}
	if ls.err != nil {
		return "", ls.err
	}
	if paired {
		return rep.Cells, nil
	}
	if l.count < 0 {
		return rep.Sequences, nil
	}
	return rep.UnitUnknown, nil
}

// tcrdistRow is one chain of a clone.
func tcrdistRow(ch tcrdistChain, fields []string, src int, line uint32, sample string) rep.Row {
	r := rep.NewRow()
	r.Source, r.Line, r.Sample, r.Locus = src, line, sample, ch.locus
	r.V, r.J = field(fields, ch.v), field(fields, ch.j)
	r.JunctionAA = strings.ToUpper(field(fields, ch.junctionAA))
	r.Junction = strings.ToUpper(field(fields, ch.junc))
	switch {
	case r.JunctionAA == "":
	case r.Junction != "" && len(r.Junction)%3 != 0:
		r.Productive, r.VJInFrame = rep.False, rep.False
	case strings.Contains(r.JunctionAA, "*"):
		r.Productive, r.VJInFrame, r.StopCodon = rep.False, rep.True, rep.True
	default:
		r.Productive, r.VJInFrame, r.StopCodon = rep.True, rep.True, rep.False
	}
	return r
}
