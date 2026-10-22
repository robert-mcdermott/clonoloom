// Package export writes a dataset's data and results out (clonoloom-spec/requirements.md, O3 and
// O9): its rearrangements as an AIRR Rearrangement TSV, each with the clonotype it belongs to; its
// clonotype table as CSV, one column of abundance per sample; and each analysis's result as CSV.
// Every export goes with a NOTICE saying what it holds, where it came from (the files' fingerprints,
// the filter, the clonotype definition, the unit), how it was made, and under what license and
// citation each public data set it holds may be used: data under a provider's terms are said not to
// be shared.
package export

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"clonoloom/internal/airr"
	"clonoloom/internal/clonotype"
	"clonoloom/internal/diversity"
	"clonoloom/internal/overlap"
	"clonoloom/internal/rep"
	"clonoloom/internal/usage"
)

// Credit is a public data set an export holds, with its license and citation.
type Credit struct {
	Title        string
	Source       string
	License      string
	LicenseURL   string
	LicenseClass string // open, terms or unknown (validation/sources.json)
	Citation     string
}

// File is a source file of the dataset, by its fingerprint.
type File struct {
	Name   string
	SHA256 string
}

// Notice is what an export says of itself.
type Notice struct {
	What         string
	Dataset      string
	DatasetID    string
	Files        []File
	Samples      int
	Unit         string
	Filter       string
	Definition   string
	DefinitionID string
	// Method: for an analysis, how it was made and the result it was kept as; Derived: the export
	// holds results computed from the data, not the data.
	Method  string
	Derived bool
	Credits []Credit
	Version string
	Commit  string
	Made    time.Time
}

// Restricted reports whether the export holds data that may not be shared: a public data set under
// a provider's terms, or under no stated license.
func (n Notice) Restricted() bool {
	for _, c := range n.Credits {
		if c.LicenseClass != "open" {
			return true
		}
	}
	return false
}

// Text is the NOTICE.
func (n Notice) Text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", n.What)
	fmt.Fprintf(&b, "Dataset: %s (%s), %d sample%s, abundance in %s.\n", n.Dataset, n.DatasetID, n.Samples, plural(n.Samples), n.Unit)
	for _, f := range n.Files {
		fmt.Fprintf(&b, "  File: %s, SHA-256 %s\n", f.Name, f.SHA256)
	}
	if n.Filter != "" {
		fmt.Fprintf(&b, "Filter: %s.\n", n.Filter)
	}
	if n.Definition != "" {
		fmt.Fprintf(&b, "Clonotypes: %s (ClonoLoom definition %s).\n", n.Definition, n.DefinitionID)
	}
	if n.Method != "" {
		fmt.Fprintf(&b, "Method: %s.\n", n.Method)
	}
	made := n.Version
	if n.Commit != "" {
		made += " (" + n.Commit + ")"
	}
	fmt.Fprintf(&b, "Made with ClonoLoom %s on %s.\n\n", made, n.Made.UTC().Format("2006-01-02 15:04 UTC"))
	switch {
	case n.Restricted() && n.Derived:
		b.WriteString("These results were computed from public data under a provider's terms of use, or under no\nstated license (below). The data themselves may not be shared: read the terms before sharing\nthese results.\n\n")
	case n.Restricted():
		b.WriteString("THESE DATA MAY NOT BE SHARED. They hold public data under a provider's terms of use, or\nunder no stated license (below): keep this export to yourself and your study, as those terms\nsay; publish results computed from it, not the data.\n\n")
	}
	if len(n.Credits) == 0 {
		b.WriteString("Data: no public data set ClonoLoom knows: the data are yours, under your own terms.\n")
	}
	for _, c := range n.Credits {
		fmt.Fprintf(&b, "Data: %s, from %s.\n  License: %s", c.Title, c.Source, c.License)
		if c.LicenseURL != "" {
			fmt.Fprintf(&b, " (%s)", c.LicenseURL)
		}
		fmt.Fprintf(&b, ".\n  Cite: %s\n", c.Citation)
	}
	return b.String()
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// number writes a value as the shortest decimal that reads back the same, missing as empty.
func number(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return ""
	}
	return strconv.FormatFloat(v, 'g', -1, 64)
}

// tsvValue is a value an AIRR TSV can hold: tabs and line breaks are spaces.
func tsvValue(s string) string {
	if strings.ContainsAny(s, "\t\r\n") {
		return strings.NewReplacer("\t", " ", "\r", " ", "\n", " ").Replace(s)
	}
	return s
}

// flag is a row's flag as AIRR writes it (T, F, or empty when missing or not given).
func flag(c []rep.Flag, row int) string {
	if c == nil {
		return ""
	}
	return airr.FormatBool(c[row] == rep.True, c[row] == rep.Missing)
}

// RearrangementFields are the columns of the rearrangements' TSV: the AIRR Rearrangement schema's
// required fields (sequence, alignments and CIGARs empty: ClonoLoom does not keep them; the source
// files have them), the calls and junctions as read, the counts, and the clonotype each row belongs
// to under the definition (clone_id). templates holds an immunoSEQ export's templates, which the
// schema has no field for.
var RearrangementFields = []string{"sequence_id", "sequence", "rev_comp", "productive", "vj_in_frame", "stop_codon", "locus", "v_call", "d_call", "j_call", "c_call",
	"sequence_alignment", "germline_alignment", "junction", "junction_aa", "v_cigar", "d_cigar", "j_cigar", "duplicate_count", "umi_count", "templates", "cell_id", "clone_id", "repertoire_id"}

// Rearrangements writes the rows keep keeps (nil: every row) as an AIRR Rearrangement TSV.
func Rearrangements(w io.Writer, rt *rep.Table, keep []bool, ct *clonotype.Table) error {
	cloneOf := map[uint32]string{}
	if ct != nil {
		for i, c := range ct.Clonotypes {
			for _, r := range ct.MembersOf(i) {
				cloneOf[r] = c.ID
			}
		}
	}
	str := func(s *rep.Strings, code uint32) string {
		if s == nil || code == 0 {
			return ""
		}
		return s.Get(code)
	}
	col := func(c []uint32, row int) uint32 {
		if c == nil {
			return 0
		}
		return c[row]
	}
	opt := func(c []float64, row int) float64 {
		if c == nil {
			return math.NaN()
		}
		return c[row]
	}
	var b strings.Builder
	b.WriteString(strings.Join(RearrangementFields, "\t"))
	b.WriteByte('\n')
	for row := range rt.Len() {
		if keep != nil && !keep[row] {
			continue
		}
		count := rt.Count[row]
		reads, umis, templates := opt(rt.Reads, row), opt(rt.UMIs, row), math.NaN()
		switch rt.Unit {
		case rep.Reads:
			reads = count
		case rep.UMIs:
			umis = count
		case rep.Templates:
			templates = count
		}
		s := rt.Samples[rt.Sample[row]]
		repertoire := s.RepertoireID
		if repertoire == "" {
			repertoire = s.Name
		}
		values := []string{
			str(rt.IDs, col(rt.SequenceID, row)), "", "", flag(rt.Productive, row), flag(rt.VJInFrame, row), flag(rt.StopCodon, row), rt.LocusName(row),
			str(rt.Dict, col(rt.V, row)), str(rt.Dict, col(rt.D, row)), str(rt.Dict, col(rt.J, row)), str(rt.Dict, col(rt.C, row)),
			"", "", str(rt.Junctions, rt.Junction[row]), str(rt.JunctionsAA, rt.JunctionAA[row]), "", "", "",
			number(reads), number(umis), number(templates), str(rt.Dict, col(rt.Cell, row)), cloneOf[uint32(row)], repertoire,
		}
		for i, v := range values {
			if i > 0 {
				b.WriteByte('\t')
			}
			b.WriteString(tsvValue(v))
		}
		b.WriteByte('\n')
		if b.Len() > 1<<20 {
			if _, err := io.WriteString(w, b.String()); err != nil {
				return err
			}
			b.Reset()
		}
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// Clonotypes writes a clonotype table as CSV: each clonotype's ID, its chains (locus, junction, V
// and J at the definition's levels), its total abundance and samples, and its abundance in each
// sample (names).
func Clonotypes(w io.Writer, ct *clonotype.Table, names []string) error {
	chains := 1
	for _, c := range ct.Clonotypes {
		chains = max(chains, len(c.Chains))
	}
	cw := csv.NewWriter(w)
	header := []string{"clonotype_id"}
	for k := 1; k <= chains; k++ {
		p := fmt.Sprintf("chain%d_", k)
		header = append(header, p+"locus", p+"junction", p+"v", p+"j")
	}
	header = append(header, "total", "samples")
	header = append(header, names...)
	if err := cw.Write(header); err != nil {
		return err
	}
	row := make([]string, len(header))
	for i, c := range ct.Clonotypes {
		clear(row)
		row[0] = c.ID
		for k, ch := range c.Chains {
			copy(row[1+4*k:], []string{ch.Locus, ch.Junction, ch.V, ch.J})
		}
		at := 1 + 4*chains
		row[at], row[at+1] = number(c.Total), strconv.Itoa(c.Samples)
		samples, values := ct.Abundances(i)
		for k, s := range samples {
			if int(s) < len(names) {
				row[at+2+int(s)] = number(values[k])
			}
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// writeAll writes rows of CSV.
func writeAll(w io.Writer, rows [][]string) error {
	cw := csv.NewWriter(w)
	if err := cw.WriteAll(rows); err != nil {
		return err
	}
	return cw.Error()
}

func valueRow(prefix []string, v diversity.Value) []string {
	return append(prefix, number(float64(v.Estimate)), number(float64(v.SE)), number(float64(v.Lower)), number(float64(v.Upper)))
}

// Diversity writes each sample's measures, one row each, with their intervals.
func Diversity(w io.Writer, names []string, results []diversity.Result) error {
	rows := [][]string{{"sample", "abundance", "clonotypes", "measure", "estimate", "se", "lower", "upper"}}
	for i, r := range results {
		ids := make([]string, 0, len(r.Measures))
		for id := range r.Measures {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			rows = append(rows, valueRow([]string{names[i], number(r.Abundance), strconv.Itoa(r.Clonotypes), id}, r.Measures[id]))
		}
	}
	return writeAll(w, rows)
}

// Rarefaction writes each sample's curves, one row a point: by size and by coverage, and at the
// levels the samples are compared at; the coverage and the Hill numbers of order 0, 1 and 2, each
// with its interval.
func Rarefaction(w io.Writer, names []string, curves []diversity.Curves) error {
	header := []string{"sample", "curve", "size", "method", "coverage", "coverage_lower", "coverage_upper"}
	for _, q := range []string{"q0", "q1", "q2"} {
		header = append(header, q, q+"_lower", q+"_upper")
	}
	rows := [][]string{header}
	for i, c := range curves {
		for _, part := range []struct {
			name   string
			points []diversity.Point
		}{{"by size", c.BySize}, {"by coverage", c.ByCoverage}, {"at a common size", c.AtSize}, {"at a common coverage", c.AtCoverage}} {
			for _, p := range part.points {
				row := []string{names[i], part.name, number(float64(p.Size)), p.Method, number(float64(p.Coverage.Estimate)), number(float64(p.Coverage.Lower)), number(float64(p.Coverage.Upper))}
				for q := range 3 {
					v := diversity.Value{Estimate: diversity.Number(math.NaN()), Lower: diversity.Number(math.NaN()), Upper: diversity.Number(math.NaN())}
					if q < len(p.Hill) {
						v = p.Hill[q]
					}
					row = append(row, number(float64(v.Estimate)), number(float64(v.Lower)), number(float64(v.Upper)))
				}
				rows = append(rows, row)
			}
		}
	}
	return writeAll(w, rows)
}

// Usage writes every share of every sample and group, one row each: what it is of (genes of a
// segment at a level, pairs, lengths, the mean length, amino acids, amino acids at an IMGT
// position), by clonotypes and by abundance, with their intervals (a group's: its samples' range).
func Usage(w io.Writer, samples []usage.Sample, groups []usage.Group) error {
	rows := [][]string{{"sample_or_group", "group", "locus", "kind", "segment", "level", "length", "imgt_position", "name",
		"clonotypes", "clonotypes_lower", "clonotypes_upper", "abundance", "abundance_lower", "abundance_upper"}}
	pair := func(p *usage.Pair, k int) string {
		if p == nil {
			return ""
		}
		return number(float64(p[k]))
	}
	add := func(unit, isGroup string, l usage.Locus) {
		share := func(kind, segment, level, length, position string, s usage.Share) {
			name := s.Name
			if name == "" && (kind == "genes" || kind == "pairs") {
				name = "(not called)"
			}
			rows = append(rows, []string{unit, isGroup, l.Locus, kind, segment, level, length, position, name,
				number(float64(s.Clonotypes)), pair(s.ClonotypesInterval, 0), pair(s.ClonotypesInterval, 1),
				number(float64(s.Abundance)), pair(s.AbundanceInterval, 0), pair(s.AbundanceInterval, 1)})
		}
		for _, g := range l.Genes {
			kind := "genes"
			if g.Segment == "VJ" {
				kind = "pairs"
			}
			for _, s := range g.Shares {
				share(kind, g.Segment, g.Level, "", "", s)
			}
		}
		for _, s := range l.Lengths {
			share("lengths", "", "", s.Name, "", s)
		}
		share("mean length", "", "", "", "", l.MeanLength)
		for _, s := range l.Composition {
			share("amino acids", "", "", "", "", s)
		}
		for _, p := range l.Profiles {
			for _, pos := range p.Positions {
				for _, s := range pos.Shares {
					share("amino acids by position", "", "", strconv.Itoa(p.Length), pos.IMGT, s)
				}
			}
		}
	}
	for _, s := range samples {
		for _, l := range s.Loci {
			add(s.Name, "", l)
		}
	}
	for _, g := range groups {
		for _, l := range g.Loci {
			add(g.Name, "yes", l)
		}
	}
	return writeAll(w, rows)
}

// Overlap writes every pair's measures, one row each, with their intervals.
func Overlap(w io.Writer, res overlap.Result) error {
	rows := [][]string{{"sample_a", "sample_b", "subject_a", "subject_b", "measure", "estimate", "se", "lower", "upper"}}
	for _, p := range res.Pairs {
		a, b := res.Samples[p.A], res.Samples[p.B]
		for _, id := range overlap.IDs {
			rows = append(rows, valueRow([]string{a.Name, b.Name, a.Subject, b.Subject, id}, p.Measures[id]))
		}
	}
	return writeAll(w, rows)
}

// Public writes the public clonotypes listed, and how many clonotypes are found in each number of
// subjects (or samples).
func Public(w io.Writer, res overlap.Result) error {
	rows := [][]string{{"clonotype_id", "chains", res.Public.By + "s", "samples", "abundance"}}
	for _, c := range res.Public.Top {
		var chains []string
		for _, ch := range c.Chains {
			chains = append(chains, strings.TrimSpace(strings.Join([]string{ch.Locus, ch.Junction, ch.V, ch.J}, " ")))
		}
		rows = append(rows, []string{c.ID, strings.Join(chains, " / "), strconv.Itoa(c.Units), strings.Join(c.Samples, " "), number(c.Abundance)})
	}
	rows = append(rows, []string{}, []string{res.Public.By + "s", "clonotypes"})
	for _, x := range res.Public.Incidence {
		rows = append(rows, []string{strconv.Itoa(x.Units), strconv.Itoa(x.Clonotypes)})
	}
	return writeAll(w, rows)
}
