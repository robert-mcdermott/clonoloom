package reads

import (
	"math"
	"strconv"
	"strings"

	"clonoloom/internal/rep"
)

// MiXCR's clone tables (mixcr exportClones, MiLaboratories), bulk TCR or BCR repertoires, one row
// per clone:
//
//   - MiXCR 3 (cloneId, cloneCount, cloneFraction, allVHitsWithScore…, nSeqCDR3, aaSeqCDR3);
//   - MiXCR 4 (readCount and readFraction for cloneCount and cloneFraction, uniqueMoleculeCount
//     where molecules were tagged), its columns depending on the export's preset, and its files
//     split by chain (<sample>.clones_TRB.tsv);
//   - MiXCR 2 (the same columns under readable names: Clone ID, Clone count, All V hits, N. Seq.
//     CDR3, AA. Seq. CDR3).
//
// MiXCR's CDR3 is the IMGT junction, from the conserved cysteine to the phenylalanine or
// tryptophan, both included. In aaSeqCDR3, _ marks a frameshift and * a stop codon; lower-case
// letters were imputed from the germline, and are read as the others. A segment's hits are listed
// best first, each with its alignment score in parentheses: the call is the best hit, with any
// other of the same score (a tie, as an AIRR call lists one), and an allele *00 (MiXCR's for a gene
// whose library gives no alleles) is dropped; the locus is topChains' where the table has it, else
// the calls'. Clones are counted in molecules where the table has them, else in reads.
//
// A single-cell export (tagValueCELL: each row a clone's chain in one cell, MiXCR 4) is single-cell
// data, as Cell Ranger's contigs are: the row's cell is its barcode, its sequence ID the barcode
// and the clone's ID (barcode_clone_7), and MiXCR's own clonotype of cells (cellGroup) its clone.

type mixcrLayout struct {
	cloneID, cell, cellGroup int
	reads, molecules         int
	v, d, j, c               int
	junction, aa, chains     int
}

// isMiXCRHeader: a clone table's amino-acid CDR3 and gene hits, in any of MiXCR's namings.
func isMiXCRHeader(header []string) bool {
	c := newColumns(header)
	aa := c.index("aaSeqCDR3", "AA. Seq. CDR3", "AA. seq. CDR3")
	hits := c.index("allVHitsWithScore", "allVHits", "bestVHit", "bestVGene", "All V hits", "Best V hit")
	return aa >= 0 && hits >= 0
}

func newMiXCRLayout(header []string) *mixcrLayout {
	c := newColumns(header)
	segment := func(s string) int {
		return c.index("all"+s+"HitsWithScore", "all"+s+"Hits", "best"+s+"Hit", "best"+s+"Gene", "All "+s+" hits", "Best "+s+" hit")
	}
	return &mixcrLayout{
		cloneID: c.index("cloneId", "Clone ID"), cell: c.index("tagValueCELL"), cellGroup: c.index("cellGroup"),
		reads:     c.index("readCount", "cloneCount", "Clone count"),
		molecules: c.index("uniqueMoleculeCount", "uniqueTagCountMolecule", "uniqueUMICount"),
		v:         segment("V"), d: segment("D"), j: segment("J"), c: segment("C"),
		junction: c.index("nSeqCDR3", "N. Seq. CDR3", "N. seq. CDR3"),
		aa:       c.index("aaSeqCDR3", "AA. Seq. CDR3", "AA. seq. CDR3"),
		chains:   c.index("topChains"),
	}
}

// mixcrCall reads a segment's hits ("TRBV7-3*00(248),TRBV7-2*00(200)") into a call: the hits of
// the best score, each without its score and without MiXCR's allele *00.
func mixcrCall(hits string) string {
	if hits == "" {
		return ""
	}
	type hit struct {
		name  string
		score float64
	}
	var all []hit
	best := math.Inf(-1)
	for _, part := range strings.Split(hits, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		h := hit{name: part, score: math.Inf(-1)}
		if open := strings.LastIndexByte(part, '('); open > 0 && strings.HasSuffix(part, ")") {
			if s, err := strconv.ParseFloat(part[open+1:len(part)-1], 64); err == nil {
				h.name, h.score = strings.TrimSpace(part[:open]), s
			}
		}
		h.name = strings.TrimSuffix(h.name, "*00")
		if h.score > best {
			best = h.score
		}
		all = append(all, h)
	}
	var names []string
	for _, h := range all {
		if h.score == best && !contains(names, h.name) {
			names = append(names, h.name)
		}
	}
	return strings.Join(names, ",")
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// mixcrSample is a MiXCR file's sample: its name without the chain MiXCR splits files by
// ("S1.clones_TRB" and MiXCR 3's "S1.clonotypes.TRB" are both sample S1), so that a sample's
// chains are one sample.
func mixcrSample(name string) string {
	for _, sep := range []string{".clones_", ".clonotypes."} {
		if i := strings.LastIndex(name, sep); i > 0 {
			chain := name[i+len(sep):]
			if chain == "ALL" || rep.LocusCode(chain) != 0 {
				return name[:i]
			}
		}
	}
	return name
}

// readMiXCR reads a MiXCR clone table's rows into b: cells for a single-cell export; otherwise
// counted in molecules when every clone has a count of them, else in reads.
func readMiXCR(ls *lines, l *mixcrLayout, width int, b *rep.Builder, src int, sample string, p *Problems) (rep.Unit, error) {
	sample = mixcrSample(sample)
	with, without, cells := 0, 0, 0
	var fields []string
	for ls.next() {
		if len(ls.line) == 0 {
			continue
		}
		fields = splitTab(ls.line, fields)
		if len(fields) != width {
			p.addf("field-count", src, ls.n, "", "", "the row has %d fields, the header %d", len(fields), width)
			if len(fields) < width {
				continue
			}
		}
		r := rep.NewRow()
		r.Source, r.Line, r.Sample = src, ls.n, sample
		r.SequenceID = field(fields, l.cloneID)
		if cell := field(fields, l.cell); cell != "" {
			r.Cell, r.IsCell, r.Clone = cell, rep.True, field(fields, l.cellGroup)
			r.SequenceID = cell + "_clone_" + r.SequenceID
			cells++
		}
		if chain := strings.ToUpper(field(fields, l.chains)); rep.LocusCode(chain) != 0 {
			r.Locus = chain
		}
		r.V, r.D, r.J, r.C = mixcrCall(field(fields, l.v)), mixcrCall(field(fields, l.d)), mixcrCall(field(fields, l.j)), mixcrCall(field(fields, l.c))
		r.Junction = strings.ToUpper(field(fields, l.junction))
		aa := strings.ToUpper(field(fields, l.aa))
		switch {
		case aa == "":
		case strings.Contains(aa, "_") || (r.Junction != "" && len(r.Junction)%3 != 0):
			// Out of frame: no amino-acid junction.
			r.Productive, r.VJInFrame = rep.False, rep.False
		case strings.Contains(aa, "*"):
			r.JunctionAA = aa
			r.Productive, r.VJInFrame, r.StopCodon = rep.False, rep.True, rep.True
		default:
			r.JunctionAA = aa
			r.Productive, r.VJInFrame, r.StopCodon = rep.True, rep.True, rep.False
		}
		r.Reads = count(field(fields, l.reads))
		r.UMIs = count(field(fields, l.molecules))
		r.Count = r.UMIs
		if math.IsNaN(r.UMIs) {
			without++
		} else {
			with++
		}
		if err := b.Append(&r); err != nil {
			return "", err
		}
	}
	if ls.err != nil {
		return "", ls.err
	}
	if cells > 0 {
		if without > 0 {
			b.CountReads(src)
		}
		return rep.Cells, nil
	}
	if with > 0 && without == 0 {
		return rep.UMIs, nil
	}
	if with > 0 {
		p.addf("molecules-missing", src, 0, "uniqueMoleculeCount", "", "%d of %d clones have no count of molecules: the file is counted in reads", without, with+without)
	}
	b.CountReads(src)
	return rep.Reads, nil
}
