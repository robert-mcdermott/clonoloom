package reads

import (
	"strings"

	"clonoloom/internal/rep"
)

// 10x Genomics Cell Ranger V(D)J contig annotations (filtered_contig_annotations.csv and
// all_contig_annotations.csv, Cell Ranger 3 to 9): one row per contig of a cell barcode, with the
// cell's flags, the contig's chain, genes, CDR3 (Cell Ranger's `cdr3` is the IMGT junction, with
// its conserved cysteine and phenylalanine or tryptophan) and its reads and UMIs. A contig is a
// rearrangement of the table; the dataset's unit is cells. Cell Ranger 3 and 4 write booleans as
// True and False and missing values as None; Cell Ranger 6 and later as true and false, and
// missing values empty; Cell Ranger 9's multi runs add a first column naming the sample.

type cellRangerLayout struct {
	sample                                           int
	barcode, isCell, contigID, highConfidence, chain int
	v, d, j, c                                       int
	fullLength, productive                           int
	cdr3, cdr3nt                                     int
	reads, umis                                      int
	rawClonotype                                     int
}

func isCellRangerContigHeader(header []string) bool {
	c := newColumns(header)
	return c.index("barcode") >= 0 && c.index("contig_id") >= 0 && c.index("chain") >= 0 && c.index("cdr3") >= 0
}

func newCellRangerLayout(header []string) *cellRangerLayout {
	c := newColumns(header)
	return &cellRangerLayout{
		sample:  c.index("sample"),
		barcode: c.index("barcode"), isCell: c.index("is_cell"), contigID: c.index("contig_id"),
		highConfidence: c.index("high_confidence"), chain: c.index("chain"),
		v: c.index("v_gene"), d: c.index("d_gene"), j: c.index("j_gene"), c: c.index("c_gene"),
		fullLength: c.index("full_length"), productive: c.index("productive"),
		cdr3: c.index("cdr3"), cdr3nt: c.index("cdr3_nt"),
		reads: c.index("reads"), umis: c.index("umis"), rawClonotype: c.index("raw_clonotype_id"),
	}
}

// cellRangerFlag reads Cell Ranger's booleans: True, False, and None or "" for unknown (Cell
// Ranger 3 and 4 write "None" for a contig whose productivity was not decided).
func cellRangerFlag(s string) rep.Flag {
	switch strings.ToLower(s) {
	case "true", "t", "1":
		return rep.True
	case "false", "f", "0":
		return rep.False
	}
	return rep.Missing
}

// noneIsEmpty: Cell Ranger writes "None" for a missing gene, chain or sequence.
func noneIsEmpty(s string) string {
	if s == "None" || s == "none" || s == "NA" {
		return ""
	}
	return s
}

func readCellRanger(ls *lines, l *cellRangerLayout, width int, b *rep.Builder, src int, sample string, p *Problems) error {
	var fields []string
	for ls.next() {
		if len(ls.line) == 0 {
			continue
		}
		var ok bool
		fields, ok = splitCSV(ls.line, fields)
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
		r := rep.NewRow()
		r.Source, r.Line, r.Sample = src, ls.n, sample
		// Cell Ranger 9's multi runs name each contig's sample.
		if named := field(fields, l.sample); named != "" {
			r.Sample = named
		}
		r.Cell = field(fields, l.barcode)
		r.SequenceID = field(fields, l.contigID)
		r.IsCell = cellRangerFlag(field(fields, l.isCell))
		r.HighConfidence = cellRangerFlag(field(fields, l.highConfidence))
		r.FullLength = cellRangerFlag(field(fields, l.fullLength))
		r.Productive = cellRangerFlag(field(fields, l.productive))
		chain := strings.ToUpper(noneIsEmpty(field(fields, l.chain)))
		if rep.LocusCode(chain) != 0 {
			r.Locus = chain
		}
		r.V = noneIsEmpty(field(fields, l.v))
		r.D = noneIsEmpty(field(fields, l.d))
		r.J = noneIsEmpty(field(fields, l.j))
		r.C = noneIsEmpty(field(fields, l.c))
		r.JunctionAA = strings.ToUpper(noneIsEmpty(field(fields, l.cdr3)))
		r.Junction = strings.ToUpper(noneIsEmpty(field(fields, l.cdr3nt)))
		r.Reads = count(field(fields, l.reads))
		r.UMIs = count(field(fields, l.umis))
		r.Count = r.UMIs
		r.Clone = noneIsEmpty(field(fields, l.rawClonotype))
		if err := b.Append(&r); err != nil {
			return err
		}
	}
	return ls.err
}
