package reads

import (
	"math"
	"strconv"
	"strings"

	"clonoloom/internal/genes"
	"clonoloom/internal/rep"
)

// Adaptive Biotechnologies' immunoSEQ Analyzer exports (and immuneACCESS downloads), bulk TCR or
// BCR repertoires, one row per rearrangement:
//
//   - version 3 (rearrangement, amino_acid, frame_type, templates, seq_reads or reads, v_resolved,
//     v_gene_ties…, with the sample's metadata repeated on each row);
//   - version 2 (nucleotide, aminoAcid, sequenceStatus, "count (templates/reads)",
//     estimatedNumberGenomes, vMaxResolved, vGeneNameTies…).
//
// amino_acid (aminoAcid) is the IMGT junction, from the conserved cysteine to the phenylalanine,
// empty for a rearrangement out of frame; its nucleotides are the cdr3_length bases of the
// sequenced read starting at v_index (cdr3_rearrangement where the export gives it). frame_type
// (sequenceStatus) In is productive, Stop in frame with a stop codon, Out out of frame. Counts are
// templates (immunoSEQ's estimate of input molecules) where the export has them, else reads. Gene
// names are Adaptive's, mapped to IMGT's (genes.FromAdaptive); a gene not resolved beyond its
// family is called by its ties (the genes it could be), as an AIRR call lists them.

// immunoSEQSampleColumns are the sample-level columns a v3 export repeats on every row, kept with
// the sample as given: what it says of the sample (tags with HLA types and diagnoses), and
// Adaptive's own summaries, which QC compares with ClonoLoom's.
var immunoSEQSampleColumns = []string{
	"species", "locus", "product_subtype", "sample_tags", "hla_class_i", "hla_class_ii", "counting_method", "primer_set",
	"total_templates", "productive_templates", "outofframe_templates", "stop_templates", "total_rearrangements",
	"productive_rearrangements", "total_reads", "total_productive_reads", "productive_clonality", "productive_entropy",
	"productive_simpson_clonality", "sample_clonality", "sample_entropy", "sample_simpson_clonality",
	"max_productive_frequency", "fraction_productive", "sample_amount_ng", "sample_cells_mass_estimate",
}

type immunoSEQLayout struct {
	metadata                         map[string]int
	version                          int
	sample, species, locus           int
	rearrangement, cdr3Rearrangement int
	aminoAcid, frameType             int
	cdr3Length, vIndex               int
	templates, reads, count          int
	countUnit                        rep.Unit
	v, d, j                          [3]int // resolved, gene, ties
}

func isImmunoSEQHeader(header []string) (version int) {
	c := newColumns(header)
	switch {
	case c.index("rearrangement") >= 0 && c.index("amino_acid") >= 0 && c.index("frame_type") >= 0:
		return 3
	case c.index("nucleotide") >= 0 && c.index("aminoAcid") >= 0:
		return 2
	}
	return 0
}

func newImmunoSEQLayout(header []string, version int) *immunoSEQLayout {
	c := newColumns(header)
	l := &immunoSEQLayout{version: version, sample: c.index("sample_name"), species: c.index("species"), locus: c.index("locus"), metadata: map[string]int{}}
	for _, name := range immunoSEQSampleColumns {
		if i := c.index(name); i >= 0 {
			l.metadata[name] = i
		}
	}
	if version == 3 {
		l.rearrangement, l.cdr3Rearrangement = c.index("rearrangement"), c.index("cdr3_rearrangement")
		l.aminoAcid, l.frameType = c.index("amino_acid"), c.index("frame_type")
		l.cdr3Length, l.vIndex = c.index("cdr3_length"), c.index("v_index")
		l.templates, l.reads, l.count = c.index("templates"), c.index("seq_reads", "reads"), -1
		for i, seg := range []string{"v", "d", "j"} {
			cols := [3]int{c.index(seg + "_resolved"), c.index(seg + "_gene"), c.index(seg + "_gene_ties")}
			switch seg {
			case "v":
				l.v = cols
			case "d":
				l.d = cols
			case "j":
				l.j = cols
			}
			_ = i
		}
		return l
	}
	l.rearrangement, l.cdr3Rearrangement = c.index("nucleotide"), -1
	l.aminoAcid, l.frameType = c.index("aminoAcid"), c.index("sequenceStatus")
	l.cdr3Length, l.vIndex = c.index("cdr3Length"), c.index("vIndex")
	l.templates, l.reads = c.index("estimatedNumberGenomes"), -1
	l.count = -1
	for i, name := range header {
		lower := strings.ToLower(strings.TrimSpace(name))
		if strings.HasPrefix(lower, "count (") || lower == "count" {
			l.count = i
			switch {
			case strings.Contains(lower, "templates/reads"):
				l.countUnit = rep.UnitUnknown
			case strings.Contains(lower, "templates"):
				l.countUnit = rep.Templates
			case strings.Contains(lower, "reads"):
				l.countUnit = rep.Reads
			}
			break
		}
	}
	l.v = [3]int{c.index("vMaxResolved"), c.index("vGeneName"), c.index("vGeneNameTies")}
	l.d = [3]int{c.index("dMaxResolved"), c.index("dGeneName"), c.index("dGeneNameTies")}
	l.j = [3]int{c.index("jMaxResolved"), c.index("jGeneName"), c.index("jGeneNameTies")}
	return l
}

// adaptiveValue: immunoSEQ writes "null" (and "unresolved") for a missing value.
func adaptiveValue(s string) string {
	switch s {
	case "null", "NULL", "na", "NA", "unresolved", "unknown":
		return ""
	}
	return s
}

// call is a segment's call: the resolved name when the gene is resolved, else the ties, else the
// family.
func (l *immunoSEQLayout) call(fields []string, cols [3]int, human bool, names map[string]string, p *Problems, src int, line uint32) string {
	resolved := adaptiveValue(field(fields, cols[0]))
	gene := adaptiveValue(field(fields, cols[1]))
	ties := adaptiveValue(field(fields, cols[2]))
	raw := resolved
	if gene == "" && ties != "" {
		raw = ties
	}
	if raw == "" {
		return ""
	}
	if mapped, ok := names[raw]; ok {
		return mapped
	}
	mapped, ok := genes.FromAdaptiveCall(raw, human)
	if !ok {
		p.addf("gene-name-unmapped", src, line, "", raw, "%s is not an immunoSEQ gene name ClonoLoom maps to IMGT's", raw)
	}
	names[raw] = mapped
	return mapped
}

var adaptiveLoci = map[string]string{"TCRB": "TRB", "TCRA": "TRA", "TCRG": "TRG", "TCRD": "TRD", "IGH": "IGH", "IGK": "IGK", "IGL": "IGL", "TCRAD": ""}

// readImmunoSEQ reads an immunoSEQ export's rows into b, counted in templates. names collects the
// gene names mapped, for the dataset's record. It answers how many rows had templates and how many
// did not (the caller counts the source in reads when any did not).
func readImmunoSEQ(ls *lines, l *immunoSEQLayout, width int, b *rep.Builder, src int, sample string, names map[string]string, p *Problems) (withTemplates, withoutTemplates int, err error) {
	var fields []string
	declared := map[string]bool{}
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
		if named := adaptiveValue(field(fields, l.sample)); named != "" {
			r.Sample = named
		}
		if !declared[r.Sample] && len(l.metadata) > 0 {
			meta := map[string]string{}
			for name, i := range l.metadata {
				if v := adaptiveValue(field(fields, i)); v != "" {
					meta[name] = v
				}
			}
			b.AddSample(rep.Sample{Name: r.Sample, Source: src, Metadata: meta})
		}
		declared[r.Sample] = true
		species := strings.ToLower(adaptiveValue(field(fields, l.species)))
		human := species == "" || species == "human" || species == "homo sapiens"
		r.Locus = adaptiveLoci[strings.ToUpper(adaptiveValue(field(fields, l.locus)))]
		r.V = l.call(fields, l.v, human, names, p, src, ls.n)
		r.D = l.call(fields, l.d, human, names, p, src, ls.n)
		r.J = l.call(fields, l.j, human, names, p, src, ls.n)
		if r.Locus == "" {
			r.Locus = genes.ParseCall(r.J).Locus()
		}
		r.JunctionAA = strings.ToUpper(adaptiveValue(field(fields, l.aminoAcid)))
		junction := strings.ToUpper(adaptiveValue(field(fields, l.cdr3Rearrangement)))
		if junction == "" {
			read := adaptiveValue(field(fields, l.rearrangement))
			start, err1 := strconv.Atoi(field(fields, l.vIndex))
			length, err2 := strconv.Atoi(field(fields, l.cdr3Length))
			if err1 == nil && err2 == nil && start >= 0 && length > 0 && start+length <= len(read) {
				junction = strings.ToUpper(read[start : start+length])
			}
		}
		r.Junction = junction
		switch strings.ToLower(adaptiveValue(field(fields, l.frameType))) {
		case "in":
			r.Productive, r.VJInFrame, r.StopCodon = rep.True, rep.True, rep.False
		case "stop":
			r.Productive, r.VJInFrame, r.StopCodon = rep.False, rep.True, rep.True
		case "out":
			r.Productive, r.VJInFrame = rep.False, rep.False
		}
		// Templates where the export has them; a row's reads are kept beside them so that a source
		// without templates (or with invalid ones) can be counted in reads instead (readOne).
		rawTemplates := adaptiveValue(field(fields, l.templates))
		templates := count(rawTemplates)
		if v, err := strconv.ParseFloat(rawTemplates, 64); err == nil && v < 0 {
			p.addf("negative-templates", src, ls.n, "templates", rawTemplates, "a negative template count")
		}
		r.Reads = count(adaptiveValue(field(fields, l.reads)))
		if l.count >= 0 && math.IsNaN(r.Reads) && l.countUnit != rep.Templates {
			r.Reads = count(adaptiveValue(field(fields, l.count)))
		}
		r.Count = templates
		if templates > 0 {
			withTemplates++
		} else {
			withoutTemplates++
		}
		if err := b.Append(&r); err != nil {
			return withTemplates, withoutTemplates, err
		}
	}
	return withTemplates, withoutTemplates, ls.err
}
