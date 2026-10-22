package reads

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"clonoloom/internal/rep"
)

// The data units of the Observed Antibody Space (OAS; Olsen et al. 2022, Protein Sci 31:141) and
// the Observed T cell receptor Space (OTS; Raybould et al. 2024, Cell Rep 43:114704), Oxford
// Protein Informatics Group, CC BY 4.0: gzipped CSV whose first line is the unit's metadata, a JSON
// object written as one CSV field, and whose columns are IgBLAST's AIRR fields.
//
//   - Unpaired units (OAS): one row per sequence of a bulk repertoire, counted by Redundancy (the
//     times the sequence was seen: reads, or molecules where the study tagged them, which the unit
//     does not say, so the user confirms it), without sequence_id.
//   - Paired units (OAS's heavy and light chains, OTS's β and α): one row per cell, each AIRR field
//     once for each chain with the chain as a suffix (v_call_heavy, v_call_light; v_call_beta,
//     v_call_alpha). A cell's barcode is its contigs' sequence_id less "_contig_n" (10x Genomics'
//     names, which the units keep), else the row's own; each chain is one of its contigs.
//
// A locus is written as one letter (H, K, L; B, A), booleans as T or F, and integers as decimals
// (214.0). The metadata (Run, Subject, Species, BSource or TSource, BType or TType, Disease,
// Vaccine, Treatment, Age…) are kept as the sample's, as given, a zero-width space a link may
// hold dropped.

// unitLoci are the units' one-letter loci.
var unitLoci = map[string]string{"H": "IGH", "K": "IGK", "L": "IGL", "B": "TRB", "A": "TRA", "G": "TRG", "D": "TRD"}

func unitLocus(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	if locus, ok := unitLoci[s]; ok {
		return locus
	}
	if rep.LocusCode(s) != 0 {
		return s
	}
	return ""
}

// isUnitMetadata: a first line that is one CSV field holding a JSON object.
func isUnitMetadata(fields []string) bool {
	return len(fields) == 1 && strings.HasPrefix(strings.TrimSpace(fields[0]), "{")
}

// unitMetadata reads a unit's metadata line into strings, as given (numbers as written).
func unitMetadata(field string) (map[string]string, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(field), &raw); err != nil {
		return nil, err
	}
	out := map[string]string{}
	for k, v := range raw {
		var s string
		if json.Unmarshal(v, &s) != nil {
			s = string(v)
		}
		s = strings.TrimSpace(strings.ReplaceAll(s, "​", ""))
		if s != "" && s != "null" {
			out[k] = s
		}
	}
	return out, nil
}

// unitChains are a paired unit's chain suffixes, in the header's order (heavy and light, beta and
// alpha), or nil for an unpaired unit.
func unitChains(header []string) []string {
	var out []string
	for _, h := range header {
		if suffix, ok := strings.CutPrefix(strings.TrimSpace(h), "v_call_"); ok && suffix != "" {
			out = append(out, suffix)
		}
	}
	return out
}

// isUnitHeader: AIRR fields, comma-separated: v_call, or v_call for each chain of a pair.
func isUnitHeader(header []string) bool {
	c := newColumns(header)
	if c.index("v_call") >= 0 && (c.index("junction_aa") >= 0 || c.index("junction") >= 0) {
		return true
	}
	chains := unitChains(header)
	return len(chains) == 2 && c.index("junction_aa_"+chains[0]) >= 0
}

type unitChain struct {
	suffix                           string
	sequenceID, locus                int
	v, d, j                          int
	junction, junctionAA             int
	productive, vjInFrame, stopCodon int
	redundancy                       int
}

func newUnitChain(c columns, suffix string) unitChain {
	name := func(n string) int {
		if suffix == "" {
			return c.index(n)
		}
		return c.index(n + "_" + suffix)
	}
	return unitChain{
		suffix: suffix, sequenceID: name("sequence_id"), locus: name("locus"),
		v: name("v_call"), d: name("d_call"), j: name("j_call"),
		junction: name("junction"), junctionAA: name("junction_aa"),
		productive: name("productive"), vjInFrame: name("vj_in_frame"), stopCodon: name("stop_codon"),
		redundancy: name("Redundancy"),
	}
}

// fill sets a row's chain fields from fields.
func (u unitChain) fill(r *rep.Row, fields []string) {
	r.SequenceID = field(fields, u.sequenceID)
	r.Locus = unitLocus(field(fields, u.locus))
	r.V, r.D, r.J = field(fields, u.v), field(fields, u.d), field(fields, u.j)
	r.Junction = strings.ToUpper(field(fields, u.junction))
	r.JunctionAA = strings.ToUpper(field(fields, u.junctionAA))
	r.Productive = flag(field(fields, u.productive))
	r.VJInFrame = flag(field(fields, u.vjInFrame))
	r.StopCodon = flag(field(fields, u.stopCodon))
}

// unitBarcode is a paired row's cell: its contigs' barcode (the sequence_id before "_contig"), the
// same for both chains, else the row's line.
func unitBarcode(ids []string, line uint32) string {
	barcode := ""
	for _, id := range ids {
		i := strings.Index(id, "_contig")
		if i <= 0 {
			return "row " + strconv.Itoa(int(line))
		}
		if barcode != "" && id[:i] != barcode {
			return "row " + strconv.Itoa(int(line))
		}
		barcode = id[:i]
	}
	if barcode == "" {
		return "row " + strconv.Itoa(int(line))
	}
	return barcode
}

// readUnit reads a data unit's rows (after its metadata and header) into b. It answers the unit:
// cells for a paired unit; for an unpaired one counted by Redundancy, unknown (reads or molecules,
// for the user to confirm).
func readUnit(ls *lines, header []string, meta map[string]string, b *rep.Builder, src int, sample string, p *Problems) (rep.Unit, error) {
	c := newColumns(header)
	width := len(header)
	b.AddSample(rep.Sample{Name: sample, Source: src, Metadata: meta})
	suffixes := unitChains(header)
	paired := len(suffixes) == 2 && c.index("v_call") < 0
	var chains []unitChain
	if paired {
		for _, s := range suffixes {
			chains = append(chains, newUnitChain(c, s))
		}
	} else {
		chains = []unitChain{newUnitChain(c, "")}
	}
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
		if !paired {
			r := rep.NewRow()
			r.Source, r.Line, r.Sample = src, ls.n, sample
			chains[0].fill(&r, fields)
			r.Count = count(field(fields, chains[0].redundancy))
			if chains[0].redundancy < 0 {
				r.Count = 1
			}
			if err := b.Append(&r); err != nil {
				return "", err
			}
			continue
		}
		ids := make([]string, 0, len(chains))
		for _, ch := range chains {
			if id := field(fields, ch.sequenceID); id != "" {
				ids = append(ids, id)
			}
		}
		cell := unitBarcode(ids, ls.n)
		for _, ch := range chains {
			r := rep.NewRow()
			r.Source, r.Line, r.Sample = src, ls.n, sample
			ch.fill(&r, fields)
			if r.V == "" && r.J == "" && r.Junction == "" && r.JunctionAA == "" {
				continue // a cell without this chain
			}
			r.Cell, r.IsCell, r.Count = cell, rep.True, 1
			if err := b.Append(&r); err != nil {
				return "", err
			}
		}
	}
	if ls.err != nil {
		return "", ls.err
	}
	if paired {
		return rep.Cells, nil
	}
	if chains[0].redundancy < 0 {
		return rep.Sequences, nil
	}
	return rep.UnitUnknown, nil
}

// readUnitHeader reads a unit's metadata line (already split as fields) and its header line. A
// metadata line that is not JSON is reported, and the unit read without it.
func readUnitHeader(ls *lines, first []string, src int, p *Problems) (header []string, meta map[string]string, err error) {
	meta, jerr := unitMetadata(first[0])
	if jerr != nil {
		p.addf("unit-metadata", src, 1, "", first[0], "the data unit's first line is not its metadata as JSON: %v", jerr)
		meta = nil
	}
	if !ls.next() {
		if ls.err != nil {
			return nil, nil, ls.err
		}
		return nil, nil, fmt.Errorf("the data unit has its metadata but no header")
	}
	header, ok := splitCSV(ls.line, nil)
	if !ok {
		return nil, nil, fmt.Errorf("the data unit's header has unbalanced quotes")
	}
	for i := range header {
		header[i] = strings.TrimSpace(header[i])
	}
	return header, meta, nil
}
