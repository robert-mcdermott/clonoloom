// Package validate checks a dataset before any analysis uses it, and says what it found as
// findings: each with a stable code, a severity, how many rows it concerns, a few examples with
// their place in the source, and a remedy. Fatal findings stop analysis; warnings are shown with
// every result that uses the data; notes inform. The codes are part of ClonoLoom's interface
// (clonoloom-spec/requirements.md, D8) and never change meaning.
package validate

import (
	"fmt"
	"math"
	"sort"

	"clonoloom/internal/genes"
	"clonoloom/internal/reads"
	"clonoloom/internal/rep"
	"clonoloom/internal/seq"
)

// Severity of a finding.
type Severity string

const (
	Fatal   Severity = "fatal"
	Warning Severity = "warning"
	Note    Severity = "note"
)

// Example is one place a finding was seen.
type Example struct {
	Source     int    `json:"source"`
	Line       uint32 `json:"line,omitempty"`
	SequenceID string `json:"sequenceId,omitempty"`
	Sample     string `json:"sample,omitempty"`
	Column     string `json:"column,omitempty"`
	Value      string `json:"value,omitempty"`
}

// Finding is one kind of problem and how often it occurs.
type Finding struct {
	Code     string    `json:"code"`
	Severity Severity  `json:"severity"`
	Title    string    `json:"title"`
	Detail   string    `json:"detail"`
	Count    int       `json:"count"`
	Of       int       `json:"of,omitempty"`
	Examples []Example `json:"examples,omitempty"`
	Remedy   string    `json:"remedy,omitempty"`
}

// Report is a dataset's findings, most severe first.
type Report struct {
	Findings []Finding `json:"findings"`
	Fatal    int       `json:"fatal"`
	Warnings int       `json:"warnings"`
	Notes    int       `json:"notes"`
}

// Usable reports whether analysis may use the dataset: no fatal finding.
func (r Report) Usable() bool { return r.Fatal == 0 }

// Limits the validator uses, recorded with every report.
const (
	examplesPerFinding = 5
	// NonProductiveWarn: a sample whose share of non-productive rearrangements exceeds this is
	// flagged. Genomic DNA (immunoSEQ) typically has 15–30% out-of-frame rearrangements and RNA
	// fewer, since nonsense-mediated decay removes them; above 40% suggests a failed library or a
	// filter not applied (ClonoLoom's choice of threshold, stated with the finding).
	NonProductiveWarn = 0.40
	// ChainsPerCell: more productive chains of one locus in a cell than this suggests a doublet
	// (two α chains, or two light chains, occur in single cells).
	ChainsPerCell = 2
)

type collector struct {
	finding  Finding
	examples []Example
}

type checker struct {
	t       *rep.Table
	sources []reads.Source
	found   map[string]*collector
}

func (c *checker) add(code string, severity Severity, title, detail, remedy string, of int, ex Example) {
	f, ok := c.found[code]
	if !ok {
		f = &collector{finding: Finding{Code: code, Severity: severity, Title: title, Detail: detail, Remedy: remedy, Of: of}}
		c.found[code] = f
	}
	f.finding.Count++
	if len(f.examples) < examplesPerFinding {
		if len(ex.Value) > 120 {
			ex.Value = ex.Value[:117] + "..."
		}
		f.examples = append(f.examples, ex)
	}
}

func (c *checker) example(row int, column, value string) Example {
	t := c.t
	return Example{Source: int(t.Source[row]), Line: t.Line[row], SequenceID: t.IDs.Get(t.SequenceID[row]), Sample: t.Samples[t.Sample[row]].Name, Column: column, Value: value}
}

// Check validates a dataset read by internal/reads.
func Check(res *reads.Result) Report {
	c := &checker{t: res.Table, sources: res.Sources, found: map[string]*collector{}}
	c.readerProblems(res.Problems)
	c.rows()
	c.cells()
	c.samples()
	c.unit(res.Unit)
	return c.report()
}

type kind struct {
	severity      Severity
	title, remedy string
}

// readerKinds are the readers' problems as findings: severity, title and remedy.
var readerKinds = map[string]kind{
	"airr-missing-required": {Warning, "Required AIRR columns are missing", "The file is not fully AIRR-compliant. ClonoLoom reads what it needs; tools that require the full schema may refuse it. Export again with the annotation tool's AIRR option."},
	"no-junction":           {Fatal, "No junction column", "Clonotypes are defined from junctions: export the rearrangements with junction or junction_aa (CDR3 with its conserved residues)."},
	"field-count":           {Warning, "Rows with the wrong number of fields", "A row with fewer fields than the header is not read. Check the file was not truncated or edited by a spreadsheet program."},
	"not-boolean":           {Warning, "Booleans that are not T or F", "The AIRR schema writes booleans as T or F. These values are read as missing."},
	"not-integer":           {Warning, "Integers that are not integers", "These values are read as missing."},
	"integer-as-decimal":    {Note, "Integers written as decimals", "Read as the integers they equal (12.0 as 12)."},
	"not-number":            {Warning, "Numbers that are not numbers", "These values are read as missing."},
	"bad-csv":               {Warning, "Rows with unbalanced quotes", "These rows are not read."},
	"negative-templates":    {Warning, "Negative template counts", "immunoSEQ wrote −1 where it could not estimate templates. These rows have no template count; their file is counted in reads."},
	"templates-missing":     {Warning, "A file counted in reads, for want of templates", "Some of the file's rows have no template count, so every row of it is counted in reads, which amplification distorts more than templates. Compare its samples with others counted in reads."},
	"mixed-units":           {Warning, "Files that count different things", "The dataset's files are counted in one unit wherever they can be (immunoSEQ files in reads when some lack templates). Otherwise compare samples only within a unit."},
	"gene-name-unmapped":    {Warning, "Vendor gene names not mapped to IMGT's", "These names are kept as given; clonotypes still tell them apart, but they match no IMGT name. Report them to ClonoLoom's developers."},
	"molecules-missing":     {Warning, "A MiXCR table counted in reads, for want of molecules", "Some clones have no count of molecules (uniqueMoleculeCount), so every clone of the file is counted in reads, which amplification distorts more than molecules. Compare its samples with others counted in reads."},
	"cell-count":            {Warning, "Clone counts that are not whole numbers of cells", "A table of paired chains counts each clone's cells. These clones are read as one cell each: check the count column."},
	"unit-metadata":         {Note, "A data unit without its metadata", "The first line of an OAS or OTS data unit is its metadata, as JSON; this one could not be read, so the samples have no annotation from it. Download the unit again."},
}

// readerProblems: what the readers could not read as the format says.
func (c *checker) readerProblems(p *reads.Problems) {
	for _, s := range p.Summaries() {
		k, ok := readerKinds[s.Code]
		if !ok {
			k = kind{Warning, s.Code, ""}
		}
		detail := ""
		if len(s.Examples) > 0 {
			detail = s.Examples[0].Message
		}
		f := &collector{finding: Finding{Code: s.Code, Severity: k.severity, Title: k.title, Detail: detail, Remedy: k.remedy, Count: s.Count}}
		for _, e := range s.Examples {
			f.examples = append(f.examples, Example{Source: e.Source, Line: e.Line, Column: e.Column, Value: e.Value})
		}
		c.found[s.Code] = f
	}
}

// rows: each rearrangement's junction, genes, counts and identifiers.
func (c *checker) rows() {
	t := c.t
	n := t.Len()
	if n == 0 {
		c.add("empty", Fatal, "No rearrangements", "The file has a header and no rows ClonoLoom could read.", "Check the file is the one meant (an export with rows, not a summary).", 0, Example{})
		return
	}
	// Per distinct junction, the checks of its text are made once.
	type junctionCheck struct{ checked, nucleotide, frame bool }
	ntChecked := make([]junctionCheck, t.Junctions.Len()+1)
	type aaCheck struct{ checked, protein, cys, phe, stop bool }
	aaChecked := make([]aaCheck, t.JunctionsAA.Len()+1)
	seen := make(map[uint64]int, n) // (sample, sequence id) → first row
	for row := 0; row < n; row++ {
		nt, aa := t.Junction[row], t.JunctionAA[row]
		if nt != 0 {
			jc := &ntChecked[nt]
			if !jc.checked {
				s := t.Junctions.Get(nt)
				jc.checked, jc.nucleotide, jc.frame = true, seq.IsNucleotide(s), len(s)%3 == 0
			}
			if !jc.nucleotide {
				c.add("junction-not-nucleotides", Warning, "Junctions that are not nucleotide sequences", "These junctions have characters other than A, C, G, T or N.", "Check the column holds nucleotides (amino acids belong in junction_aa).", n, c.example(row, "junction", t.Junctions.Get(nt)))
			}
			if t.Productive[row] == rep.True && !jc.frame {
				c.add("productive-out-of-frame", Warning, "Productive rearrangements with an out-of-frame junction", "Their junction's length is not a multiple of three, which a productive rearrangement's cannot be.", "The productive flag or the junction is wrong; ClonoLoom keeps the flag as given and counts these rows.", n, c.example(row, "junction", t.Junctions.Get(nt)))
			}
			if aa != 0 && jc.nucleotide && jc.frame {
				if got, want := seq.Translate(t.Junctions.Get(nt)), t.JunctionsAA.Get(aa); got != want {
					c.add("junction-translation", Warning, "Amino-acid junctions that do not translate from the nucleotides", "junction_aa should be the translation of junction.", "One of the two columns is wrong, or they describe different regions (a CDR3 in one, a junction in the other).", n, c.example(row, "junction_aa", fmt.Sprintf("%s (translates to %s)", want, got)))
				}
			}
		}
		if aa != 0 {
			ac := &aaChecked[aa]
			if !ac.checked {
				s := t.JunctionsAA.Get(aa)
				ac.checked, ac.protein, ac.stop = true, seq.IsProtein(s), seq.HasStop(s)
				ac.cys, ac.phe = seq.Anchors(s)
			}
			if !ac.protein {
				c.add("junction-aa-not-protein", Warning, "Amino-acid junctions with characters that are not amino acids", "Only the twenty amino acids, X and * are read as amino acids.", "Check the column holds amino acids.", n, c.example(row, "junction_aa", t.JunctionsAA.Get(aa)))
			}
			if t.Productive[row] == rep.True && ac.stop {
				c.add("productive-with-stop", Warning, "Productive rearrangements with a stop codon in the junction", "A productive rearrangement has no stop codon.", "The productive flag or the junction is wrong; ClonoLoom keeps the flag as given.", n, c.example(row, "junction_aa", t.JunctionsAA.Get(aa)))
			}
			if ac.protein && !ac.stop && (!ac.cys || !ac.phe) {
				c.add("junction-anchors", Note, "Junctions without the conserved cysteine or phenylalanine/tryptophan", "An IMGT junction begins with the conserved cysteine (2nd-CYS 104) and ends with phenylalanine or tryptophan (J-PHE/J-TRP 118). Some germline genes have other residues there; many such junctions instead suggest a CDR3 without its anchors.", "If the column holds CDR3s without anchors, clonotypes are still defined consistently, but matches against databases of junctions will fail.", n, c.example(row, "junction_aa", t.JunctionsAA.Get(aa)))
			}
		}
		if nt == 0 && aa == 0 && t.Productive[row] != rep.False {
			c.add("no-junction-row", Warning, "Rearrangements without a junction", "These rows have neither junction nor junction_aa and cannot belong to any clonotype.", "They are left out of clonotypes and counted in QC.", n, c.example(row, "junction", ""))
		}
		// Genes: readable, and of one locus.
		vCall, jCall := t.Call(t.V[row]), t.Call(t.J[row])
		if t.V[row] != 0 && len(vCall.Names) == 0 {
			c.add("gene-unreadable", Warning, "Gene calls that are not IMGT gene names", "These V, D, J or C calls could not be read as immune receptor gene names.", "Map vendor names to IMGT nomenclature before export, or report the names to ClonoLoom's developers.", n, c.example(row, "v_call", t.Dict.Get(t.V[row])))
		}
		if t.J[row] != 0 && len(jCall.Names) == 0 {
			c.add("gene-unreadable", Warning, "Gene calls that are not IMGT gene names", "These V, D, J or C calls could not be read as immune receptor gene names.", "Map vendor names to IMGT nomenclature before export, or report the names to ClonoLoom's developers.", n, c.example(row, "j_call", t.Dict.Get(t.J[row])))
		}
		vl, jl := vCall.Locus(), jCall.Locus()
		if vl != "" && jl != "" && vl != jl && !sharedAD(vCall, jl) {
			c.add("locus-mismatch", Warning, "V and J genes of different loci", "A rearrangement joins V and J genes of one locus.", "Check the annotation; chimeric calls are left out of paired-chain analyses.", n, c.example(row, "v_call", t.Dict.Get(t.V[row])+" / "+t.Dict.Get(t.J[row])))
		} else if t.Locus[row] != 0 && jl != "" && t.LocusName(row) != jl && !sharedAD(vCall, t.LocusName(row)) {
			c.add("locus-mismatch", Warning, "V and J genes of different loci", "The locus column disagrees with the J gene's locus.", "Check the annotation.", n, c.example(row, "locus", t.LocusName(row)+" / "+t.Dict.Get(t.J[row])))
		}
		// Counts.
		if v := t.Count[row]; math.IsNaN(v) {
			if t.Unit != rep.Cells && t.Unit != rep.Sequences {
				c.add("count-missing", Warning, "Rearrangements without a count", "These rows give no count in the dataset's unit.", "They count as missing, not as one, in every analysis of abundance.", n, c.example(row, "count", ""))
			}
		} else if v == 0 {
			c.add("count-zero", Note, "Rearrangements with a count of zero", "A count of zero means the rearrangement was not observed in the unit counted.", "They are kept and contribute nothing to abundances.", n, c.example(row, "count", "0"))
		}
		// Identifiers: unique within a sample.
		if id := t.SequenceID[row]; id != 0 {
			key := uint64(t.Sample[row])<<32 | uint64(hashID(t.IDs.Bytes(id)))
			if first, dup := seen[key]; dup && t.IDs.Get(t.SequenceID[first]) == t.IDs.Get(id) {
				c.add("duplicate-sequence-id", Warning, "Sequence IDs used more than once in a sample", "Each rearrangement's sequence_id should be unique.", "Rows are told apart by their place in the file; exports keep the IDs as given.", n, c.example(row, "sequence_id", t.IDs.Get(id)))
			} else if !dup {
				seen[key] = row
			}
		}
	}
}

// sharedAD: TRA and TRD are one locus on chromosome 14 (TRD lies within TRA): V genes are shared
// ("TRAV29/DV5"), and a TRDV gene joined to a TRAJ gene (TRDV1 with TRAJ, in α chains) is
// biology, not an error. A pair of TRA and TRD names is therefore never a mismatch.
func sharedAD(c genes.Call, locus string) bool {
	if locus != "TRA" && locus != "TRD" {
		return false
	}
	for _, n := range c.Names {
		if n.Locus == "TRA" || n.Locus == "TRD" {
			return true
		}
	}
	return false
}

func hashID(b []byte) uint32 {
	h := uint32(2166136261)
	for _, x := range b {
		h ^= uint32(x)
		h *= 16777619
	}
	return h
}

// cells: single-cell data's chains per cell.
func (c *checker) cells() {
	t := c.t
	if t.Cell == nil {
		return
	}
	type key struct {
		sample, cell uint32
		locus        uint8
	}
	chains := map[key]int{}
	first := map[key]int{}
	for row := 0; row < t.Len(); row++ {
		if t.Cell[row] == 0 || t.Productive[row] != rep.True || t.Locus[row] == 0 {
			continue
		}
		k := key{t.Sample[row], t.Cell[row], t.Locus[row]}
		if chains[k] == 0 {
			first[k] = row
		}
		chains[k]++
	}
	keys := make([]key, 0, len(chains))
	for k, n := range chains {
		if n > ChainsPerCell {
			keys = append(keys, k)
		}
	}
	sort.Slice(keys, func(i, j int) bool { return first[keys[i]] < first[keys[j]] })
	for _, k := range keys {
		row := first[k]
		c.add("cell-many-chains", Warning, "Cells with more than two productive chains of one locus", "Two α chains (or two light chains) occur in single cells; more suggests two cells captured together (a doublet).", "Choose how clonotype definitions treat these cells: keep the two chains with most UMIs, keep all, or leave the cells out.", len(chains), c.example(row, "cell_id", fmt.Sprintf("%s: %d %s chains", t.Dict.Get(k.cell), chains[k], t.LocusName(row))))
	}
}

// samples: each sample's share of non-productive rearrangements.
func (c *checker) samples() {
	t := c.t
	if t.Cell != nil {
		return // single-cell contigs: QC reports productive shares per cell instead
	}
	productive := make([]float64, len(t.Samples))
	total := make([]float64, len(t.Samples))
	firstRow := make([]int, len(t.Samples))
	for i := range firstRow {
		firstRow[i] = -1
	}
	for row := 0; row < t.Len(); row++ {
		s := t.Sample[row]
		if firstRow[s] < 0 {
			firstRow[s] = row
		}
		w := t.Count[row]
		if math.IsNaN(w) {
			w = 1
		}
		if t.Productive[row] == rep.Missing {
			continue
		}
		total[s] += w
		if t.Productive[row] == rep.True {
			productive[s] += w
		}
	}
	for s := range t.Samples {
		if total[s] == 0 {
			continue
		}
		share := 1 - productive[s]/total[s]
		if share > NonProductiveWarn {
			ex := c.example(firstRow[s], "productive", fmt.Sprintf("%.1f%% non-productive", 100*share))
			ex.Line, ex.SequenceID = 0, ""
			c.add("nonproductive-share", Warning, "Samples with a high share of non-productive rearrangements", fmt.Sprintf("More than %.0f%% of the sample's abundance is non-productive (out of frame or with a stop codon).", 100*NonProductiveWarn), "A failed library, contamination, or a productive filter not applied upstream. Check the sample's QC before comparing it with others.", len(t.Samples), ex)
		}
	}
}

func (c *checker) unit(u rep.Unit) {
	if u == rep.UnitUnknown && c.t.Len() > 0 {
		c.add("unit-unknown", Note, "The abundance unit is not stated", "The file's counts (duplicate_count) do not say whether reads or molecules (UMIs) were counted.", "Confirm the unit in the Data view; until then results say \"duplicate_count\" rather than reads or UMIs.", 0, Example{Column: "duplicate_count"})
	}
}

func (c *checker) report() Report {
	var r Report
	for _, f := range c.found {
		f.finding.Examples = f.examples
		r.Findings = append(r.Findings, f.finding)
	}
	rank := map[Severity]int{Fatal: 0, Warning: 1, Note: 2}
	sort.Slice(r.Findings, func(i, j int) bool {
		a, b := r.Findings[i], r.Findings[j]
		if rank[a.Severity] != rank[b.Severity] {
			return rank[a.Severity] < rank[b.Severity]
		}
		return a.Code < b.Code
	})
	for _, f := range r.Findings {
		switch f.Severity {
		case Fatal:
			r.Fatal++
		case Warning:
			r.Warnings++
		case Note:
			r.Notes++
		}
	}
	if r.Findings == nil {
		r.Findings = []Finding{}
	}
	return r
}
