package reads

import (
	"math"
	"strings"

	"clonoloom/internal/airr"
	"clonoloom/internal/rep"
)

// The AIRR Rearrangement TSV (airr.Fields), as IgBLAST, Change-O, MiXCR's AIRR export, TRUST4,
// Cell Ranger's airr_rearrangement.tsv and AIRR Data Commons downloads write it. Every column the
// schema types is checked against its type; ClonoLoom keeps those of the rearrangement table and
// leaves the others in the file.

// airrLayout is where the columns ClonoLoom reads are in a header.
type airrLayout struct {
	cols                                columns
	header                              []string
	sequenceID, locus                   int
	v, d, j, c                          int
	junction, junctionAA, cdr3, cdr3AA  int
	productive, vjInFrame, stopCodon    int
	duplicateCount, consensusCount, umi int
	cellID, cloneID                     int
	repertoireID, sampleProcessingID    int
	missingRequired                     []string
}

func newAIRRLayout(header []string) *airrLayout {
	c := newColumns(header)
	l := &airrLayout{
		cols: c, header: header,
		sequenceID: c.index("sequence_id"), locus: c.index("locus"),
		v: c.index("v_call"), d: c.index("d_call"), j: c.index("j_call"), c: c.index("c_call"),
		junction: c.index("junction"), junctionAA: c.index("junction_aa"), cdr3: c.index("cdr3"), cdr3AA: c.index("cdr3_aa"),
		productive: c.index("productive"), vjInFrame: c.index("vj_in_frame"), stopCodon: c.index("stop_codon"),
		duplicateCount: c.index("duplicate_count"), consensusCount: c.index("consensus_count"), umi: c.index("umi_count"),
		cellID: c.index("cell_id"), cloneID: c.index("clone_id"),
		repertoireID: c.index("repertoire_id"), sampleProcessingID: c.index("sample_processing_id"),
	}
	for _, name := range airr.Required {
		if _, ok := c[name]; !ok {
			l.missingRequired = append(l.missingRequired, name)
		}
	}
	return l
}

// isAIRRHeader: a tab-separated header with sequence_id and a junction or V call, the columns no
// other format names so.
func isAIRRHeader(header []string) bool {
	c := newColumns(header)
	_, id := c["sequence_id"]
	_, junction := c["junction"]
	_, junctionAA := c["junction_aa"]
	_, v := c["v_call"]
	return id && (junction || junctionAA || v)
}

// unit is what an AIRR file's counts are, from the columns present: cells when it has cell IDs
// (single-cell contigs), UMIs when it has umi_count, reads for consensus_count alone; for
// duplicate_count alone the schema's "copy number or number of duplicate observations" does not
// say whether reads or molecules were counted, and the unit is left unknown for the user to
// confirm; sequences when no count is given.
func (l *airrLayout) unit(hasCells bool) rep.Unit {
	switch {
	case hasCells:
		return rep.Cells
	case l.umi >= 0:
		return rep.UMIs
	case l.duplicateCount >= 0:
		return rep.UnitUnknown
	case l.consensusCount >= 0:
		return rep.Reads
	}
	return rep.Sequences
}

// readAIRR reads an AIRR TSV's rows (after its header) into b, as source number src, with name the
// sample name used when the file names no repertoire. It answers how many rows named a cell.
func readAIRR(ls *lines, l *airrLayout, b *rep.Builder, src int, name string, p *Problems) (cells int, err error) {
	if len(l.missingRequired) > 0 {
		p.addf("airr-missing-required", src, 1, strings.Join(l.missingRequired, ", "), "", "the AIRR schema requires these columns, which the file lacks: %s", strings.Join(l.missingRequired, ", "))
	}
	if l.junction < 0 && l.junctionAA < 0 {
		p.addf("no-junction", src, 1, "junction", "", "the file has neither junction nor junction_aa")
	}
	// Columns the schema types, checked on every row.
	type typed struct {
		index int
		name  string
		kind  airr.Type
	}
	var checks []typed
	for i, h := range l.header {
		if t, ok := airr.Fields[strings.TrimSpace(h)]; ok && t != airr.String {
			checks = append(checks, typed{i, strings.TrimSpace(h), t})
		}
	}
	width := len(l.header)
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
		for _, c := range checks {
			v := field(fields, c.index)
			switch c.kind {
			case airr.Boolean:
				if _, _, ok := airr.ParseBool(v); !ok {
					p.addf("not-boolean", src, ls.n, c.name, v, "%s is not a boolean (T or F)", c.name)
				}
			case airr.Integer:
				if _, _, exact, ok := airr.ParseInt(v); !ok {
					p.addf("not-integer", src, ls.n, c.name, v, "%s is not an integer", c.name)
				} else if !exact {
					p.addf("integer-as-decimal", src, ls.n, c.name, v, "%s is an integer written as a decimal", c.name)
				}
			case airr.Number:
				if _, _, ok := airr.ParseNumber(v); !ok {
					p.addf("not-number", src, ls.n, c.name, v, "%s is not a number", c.name)
				}
			}
		}
		r := rep.NewRow()
		r.Source, r.Line = src, ls.n
		r.SequenceID = field(fields, l.sequenceID)
		r.Locus = strings.ToUpper(field(fields, l.locus))
		r.V, r.D, r.J, r.C = field(fields, l.v), field(fields, l.d), field(fields, l.j), field(fields, l.c)
		r.Junction = strings.ToUpper(field(fields, l.junction))
		r.JunctionAA = strings.ToUpper(field(fields, l.junctionAA))
		r.Productive = flag(field(fields, l.productive))
		r.VJInFrame = flag(field(fields, l.vjInFrame))
		r.StopCodon = flag(field(fields, l.stopCodon))
		r.Cell = field(fields, l.cellID)
		if r.Cell != "" {
			cells++
		}
		r.Clone = field(fields, l.cloneID)
		r.Sample = field(fields, l.repertoireID)
		r.RepertoireID = r.Sample
		if r.Sample == "" {
			r.Sample = field(fields, l.sampleProcessingID)
		}
		if r.Sample == "" {
			r.Sample = name
		}
		umi := count(field(fields, l.umi))
		duplicate := count(field(fields, l.duplicateCount))
		consensus := count(field(fields, l.consensusCount))
		switch {
		case l.umi >= 0:
			r.Count, r.UMIs = umi, umi
			r.Reads = firstNumber(consensus, duplicate)
		case l.duplicateCount >= 0:
			r.Count = duplicate
			r.Reads = consensus
		case l.consensusCount >= 0:
			r.Count, r.Reads = consensus, consensus
		default:
			r.Count = 1
		}
		if err := b.Append(&r); err != nil {
			return cells, err
		}
	}
	return cells, ls.err
}

func flag(s string) rep.Flag {
	v, missing, ok := airr.ParseBool(s)
	if missing || !ok {
		return rep.Missing
	}
	return rep.FlagOf(v)
}

// count reads a count; anything that is not a number, or is negative, is missing (the schema
// check reports it).
func count(s string) float64 {
	v, missing, ok := airr.ParseNumber(s)
	if missing || !ok || v < 0 {
		return math.NaN()
	}
	return v
}

func firstNumber(values ...float64) float64 {
	for _, v := range values {
		if !math.IsNaN(v) {
			return v
		}
	}
	return math.NaN()
}
