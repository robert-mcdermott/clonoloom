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
	"clonoloom/internal/invariant"
	"clonoloom/internal/overlap"
	"clonoloom/internal/rep"
	"clonoloom/internal/track"
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
// required fields (including retained sequence evidence), calls and junctions as read, counts,
// and the clonotype each row belongs
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
	fields := append([]string(nil), RearrangementFields...)
	evidenceAt := make(map[int]int)
	for i, name := range rep.EvidenceFields {
		if rt.Evidence[i] == nil {
			continue
		}
		at := -1
		for j, f := range fields {
			if f == name {
				at = j
				break
			}
		}
		if at < 0 {
			at = len(fields)
			fields = append(fields, name)
		}
		evidenceAt[i] = at
	}
	b.WriteString(strings.Join(fields, "\t"))
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
		for len(values) < len(fields) {
			values = append(values, "")
		}
		for field, at := range evidenceAt {
			values[at] = rt.EvidenceValue(row, field)
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
	return ClonotypesWithInvariant(w, ct, names, nil)
}

// ClonotypesWithInvariant adds the explicit species and versioned rule statuses, when requested.
func ClonotypesWithInvariant(w io.Writer, ct *clonotype.Table, names []string, inv *invariant.Result) error {
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
	if inv != nil {
		header = append(header, "invariant_species", "invariant_rule_version", "MAIT", "iNKT", "GEM")
	}
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
		if inv != nil {
			copy(row[len(row)-5:], []string{inv.Species, strconv.Itoa(inv.Version), inv.Flags[i][0], inv.Flags[i][1], inv.Flags[i][2]})
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

// chainsText is a clonotype's chains as one cell: each chain's locus, junction, V and J, chains
// joined by " / ".
func chainsText(chains []clonotype.Chain) string {
	var out []string
	for _, ch := range chains {
		out = append(out, strings.TrimSpace(strings.Join([]string{ch.Locus, ch.Junction, ch.V, ch.J}, " ")))
	}
	return strings.Join(out, " / ")
}

// whyText is why a clonotype was followed, as one cell: "rank 2 in P1-product; listed as CASS…".
func whyText(why []track.Reason) string {
	var out []string
	for _, r := range why {
		if r.Listed != "" {
			out = append(out, "listed as "+r.Listed)
		} else {
			out = append(out, fmt.Sprintf("rank %d in %s", r.Rank, r.Sample))
		}
	}
	return strings.Join(out, "; ")
}

func trackValue(prefix []string, v track.Value) []string {
	return append(prefix, number(v.Count), number(v.Total), number(float64(v.Frequency)), number(float64(v.Lower)), number(float64(v.Upper)))
}

// Tracking writes each clonotype followed, in each of its subject's samples (tracking.csv): its
// count of the sample's total, its frequency and the frequency's interval.
func Tracking(w io.Writer, res track.Result) error {
	rows := [][]string{{"subject", "clonotype_id", "chains", "why", "sample", "time_point", "days", "reference", "count", "total", "frequency", "lower", "upper"}}
	for _, s := range res.Subjects {
		for _, c := range s.Clonotypes {
			for k, v := range c.Samples {
				p := s.Points[s.Samples[k].Point]
				rows = append(rows, trackValue([]string{s.ID, c.ID, chainsText(c.Chains), whyText(c.Why), s.Samples[k].Name, p.Label, daysText(p.Days), p.Reference}, v))
			}
		}
	}
	return writeAll(w, rows)
}

// TrackingPoints writes each clonotype followed at each of its subject's time points
// (time-points.csv), the samples taken at one time pooled.
func TrackingPoints(w io.Writer, res track.Result) error {
	rows := [][]string{{"subject", "clonotype_id", "chains", "time_point", "days", "reference", "samples", "count", "total", "frequency", "lower", "upper"}}
	for _, s := range res.Subjects {
		for _, c := range s.Clonotypes {
			for k, v := range c.Points {
				p := s.Points[k]
				rows = append(rows, trackValue([]string{s.ID, c.ID, chainsText(c.Chains), p.Label, daysText(p.Days), p.Reference, strings.Join(p.Samples, " ")}, v))
			}
		}
	}
	return writeAll(w, rows)
}

func daysText(days *float64) string {
	if days == nil {
		return ""
	}
	return number(*days)
}

// Expansion writes each clonotype tested in each subject (tests.csv): its counts and frequencies at
// the two time points, the log₂ of their ratio, its p-value, adjusted p-value and call.
func Expansion(w io.Writer, res track.Tests) error {
	rows := [][]string{{"subject", "from", "to", "clonotype_id", "chains", "count_from", "count_to", "total_from", "total_to", "frequency_from", "frequency_to", "log2_fold", "p", "q", "call"}}
	for _, s := range res.Subjects {
		for _, c := range s.Clonotypes {
			rows = append(rows, []string{s.Subject, s.From.Label, s.To.Label, c.ID, chainsText(c.Chains), number(c.A), number(c.B), number(s.TotalA), number(s.TotalB),
				number(c.FreqA), number(c.FreqB), number(float64(c.Log2Fold)), number(c.P), number(c.Q), c.Call})
		}
	}
	return writeAll(w, rows)
}

// ExpansionFit writes the beta-binomial test's variance model and the pairs of samples it was
// fitted on (fit.csv).
func ExpansionFit(w io.Writer, fit *track.Fit) error {
	rows := [][]string{
		{"fitted_on", "a", "b", "log_likelihood", "clonotypes", "called_by_sampling_at_0.05", "called_by_model_at_0.05"},
		{fit.On, number(fit.Model.A), number(fit.Model.B), number(fit.LogLik), strconv.Itoa(fit.Clonotypes), number(fit.CalledBySampling), number(fit.CalledByModel)},
		{},
		{"subject", "samples_a", "samples_b", "theta", "clonotypes"},
	}
	for _, p := range fit.Pairs {
		rows = append(rows, []string{p.Subject, strings.Join(p.A, " "), strings.Join(p.B, " "), number(p.Theta), strconv.Itoa(p.Clonotypes)})
	}
	return writeAll(w, rows)
}

// ExpansionSubjects writes each subject's comparison in sum (subjects.csv): its two time points
// and their samples, the clonotypes tested and left out, and those called each way.
func ExpansionSubjects(w io.Writer, res track.Tests) error {
	rows := [][]string{{"subject", "from", "to", "samples_from", "samples_to", "total_from", "total_to", "tested", "left_out", "increased", "decreased", "pi0"}}
	for _, s := range res.Subjects {
		rows = append(rows, []string{s.Subject, s.From.Label, s.To.Label, strings.Join(s.From.Samples, " "), strings.Join(s.To.Samples, " "), number(s.TotalA), number(s.TotalB),
			strconv.Itoa(s.Tested), strconv.Itoa(s.LeftOut), strconv.Itoa(s.Increased), strconv.Itoa(s.Decreased), number(float64(s.Pi0))})
	}
	return writeAll(w, rows)
}

// Classes writes each clonotype classed in each subject (classes.csv), one row a time point: its
// class, its frequency there, and its call against the base.
func Classes(w io.Writer, res track.ClassResult) error {
	rows := [][]string{{"subject", "clonotype_id", "chains", "class", "q", "time_point", "days", "frequency", "call"}}
	for _, s := range res.Subjects {
		for _, c := range s.Clonotypes {
			rows = append(rows, []string{s.Subject, c.ID, chainsText(c.Chains), c.Class, number(c.Q), s.Base.Label, daysText(s.Base.Days), number(c.Frequencies[0]), "base"})
			for k, p := range s.Later {
				rows = append(rows, []string{s.Subject, c.ID, chainsText(c.Chains), c.Class, number(c.Q), p.Label, daysText(p.Days), number(c.Frequencies[k+1]), c.Calls[k]})
			}
		}
	}
	return writeAll(w, rows)
}

// ClassCounts writes each subject's clonotypes by class (class-counts.csv).
func ClassCounts(w io.Writer, res track.ClassResult) error {
	rows := [][]string{{"subject", "base", "later", "emerging", "transient", "contracting", "persistent", "unclassed"}}
	for _, s := range res.Subjects {
		var later []string
		for _, p := range s.Later {
			later = append(later, p.Label)
		}
		n := s.Counts
		rows = append(rows, []string{s.Subject, s.Base.Label, strings.Join(later, " "), strconv.Itoa(n.Emerging), strconv.Itoa(n.Transient), strconv.Itoa(n.Contracting), strconv.Itoa(n.Persistent), strconv.Itoa(n.Unclassed)})
	}
	return writeAll(w, rows)
}

func attributedRow(prefix []string, x track.AttributedValue) []string {
	row := append(prefix, x.Point.Label, daysText(x.Days), number(x.Value.Count), number(x.Value.Total), number(float64(x.Value.Frequency)), number(float64(x.Value.Lower)), number(float64(x.Value.Upper)))
	if x.From == nil {
		return append(row, "", "", "", "", "")
	}
	return append(row, number(x.From.Count), number(x.From.Total), number(float64(x.From.Frequency)), number(float64(x.From.Lower)), number(float64(x.From.Upper)))
}

// Attribution writes each sample's and time point's measures (attribution.csv): its cells in the
// reference's clonotypes of its total, the share and its interval; and, where told, its cells in
// the reference's clonotypes of one side only, of those in either side's, the share derived from
// the reference and its interval.
func Attribution(w io.Writer, res track.Attribution) error {
	rows := [][]string{{"subject", "reference", "sample", "time_point", "days", "in_reference", "total", "share", "lower", "upper", "from_reference", "informative", "derived_share", "derived_lower", "derived_upper"}}
	for _, s := range res.Subjects {
		for _, x := range s.Samples {
			rows = append(rows, attributedRow([]string{s.Subject, strings.Join(s.Reference, " "), x.Name}, x))
		}
		for _, x := range s.Points {
			rows = append(rows, attributedRow([]string{s.Subject, strings.Join(s.Reference, " "), strings.Join(x.Point.Samples, " ")}, x))
		}
	}
	return writeAll(w, rows)
}

// Neighbor is a clonotype TCRdist compared: its neighbors within the radius, and its nearest other
// clonotype of the same chains ("" when it has none) with their distance.
type Neighbor struct {
	ID        string
	Chains    []clonotype.Chain
	Total     float64
	Samples   int
	Neighbors int
	Nearest   string
	Distance  int
}

// TCRdist writes each clonotype TCRdist compared (tcrdist.csv): its chains, abundance and samples,
// its neighbors within the radius, and its nearest clonotype with their distance.
func TCRdist(w io.Writer, radius int, rows []Neighbor) error {
	out := [][]string{{"clonotype_id", "chains", "total", "samples", "radius", "neighbors", "nearest_id", "nearest_distance"}}
	for _, r := range rows {
		distance := ""
		if r.Nearest != "" {
			distance = strconv.Itoa(r.Distance)
		}
		out = append(out, []string{r.ID, chainsText(r.Chains), number(r.Total), strconv.Itoa(r.Samples), strconv.Itoa(radius), strconv.Itoa(r.Neighbors), r.Nearest, distance})
	}
	return writeAll(w, out)
}

// Generated is a clonotype's generation probabilities: each chain's (−1 when its locus has no
// model) and their product (−1 unless every chain has one).
type Generated struct {
	ID      string
	Chains  []clonotype.Chain
	Total   float64
	Samples int
	Chain   []float64
	Pgen    float64
}

// Pgen writes each clonotype's generation probabilities (pgen.csv): its chains, abundance and
// samples, the product, and each chain's (locus:probability, separated by spaces); a probability
// missing is empty.
func Pgen(w io.Writer, rows []Generated) error {
	out := [][]string{{"clonotype_id", "chains", "total", "samples", "pgen", "chain_pgen"}}
	text := func(p float64) string {
		if p < 0 {
			return ""
		}
		return strconv.FormatFloat(p, 'g', 10, 64)
	}
	for _, r := range rows {
		var chains []string
		for k, ch := range r.Chains {
			if k < len(r.Chain) {
				chains = append(chains, ch.Locus+":"+text(r.Chain[k]))
			}
		}
		out = append(out, []string{r.ID, chainsText(r.Chains), number(r.Total), strconv.Itoa(r.Samples), text(r.Pgen), strings.Join(chains, " ")})
	}
	return writeAll(w, out)
}

// PgenSample is a sample's clonotypes' generation probabilities: how many have one, how many the
// model cannot make, and the quartiles of the others' log₁₀.
type PgenSample struct {
	Name      string
	With      int
	Zero      int
	Quartiles [3]float64
}

// PgenSamples writes each sample's distribution (samples.csv).
func PgenSamples(w io.Writer, rows []PgenSample) error {
	out := [][]string{{"sample", "clonotypes_with_pgen", "pgen_zero", "log10_pgen_q1", "log10_pgen_median", "log10_pgen_q3"}}
	for _, r := range rows {
		q := []string{"", "", ""}
		if r.With > r.Zero {
			for i, v := range r.Quartiles {
				q[i] = strconv.FormatFloat(v, 'f', 4, 64)
			}
		}
		out = append(out, append([]string{r.Name, strconv.Itoa(r.With), strconv.Itoa(r.Zero)}, q...))
	}
	return writeAll(w, out)
}

// Neighborhood is a centroid of a neighborhoods result, for neighborhoods.csv.
type Neighborhood struct {
	ID                                           string
	Locus, Junction, V, J                        string
	Total                                        float64
	Samples, Radius, Neighbors                   int
	Abundance                                    float64
	Background                                   int
	Weighted, Rate, BackgroundRate, RelativeRate float64
	OddsRatio, ChiSquare                         float64
	Motif                                        string
}

// Neighborhoods writes each centroid: its chain, radius, neighborhood and enrichment.
func Neighborhoods(w io.Writer, rows []Neighborhood) error {
	out := [][]string{{"centroid_id", "locus", "junction_aa", "v_call", "j_call", "total", "samples", "radius",
		"neighbors", "neighbor_abundance", "background", "background_weighted", "rate", "background_rate",
		"relative_rate", "odds_ratio", "chi_square", "motif"}}
	for _, r := range rows {
		out = append(out, []string{r.ID, r.Locus, r.Junction, r.V, r.J, number(r.Total), strconv.Itoa(r.Samples),
			strconv.Itoa(r.Radius), strconv.Itoa(r.Neighbors), number(r.Abundance), strconv.Itoa(r.Background),
			strconv.FormatFloat(r.Weighted, 'g', 10, 64), strconv.FormatFloat(r.Rate, 'g', 10, 64),
			strconv.FormatFloat(r.BackgroundRate, 'g', 10, 64), strconv.FormatFloat(r.RelativeRate, 'g', 10, 64),
			strconv.FormatFloat(r.OddsRatio, 'g', 10, 64), strconv.FormatFloat(r.ChiSquare, 'g', 10, 64), r.Motif})
	}
	return writeAll(w, out)
}

// NeighborhoodMember is a receptor of a neighborhood, for members.csv.
type NeighborhoodMember struct {
	Centroid, ID   string
	Junction, V, J string
	Total          float64
	Samples        int
	Distance       int
}

// NeighborhoodMembers writes each neighborhood's receptors, its centroid first.
func NeighborhoodMembers(w io.Writer, rows []NeighborhoodMember) error {
	out := [][]string{{"centroid_id", "clonotype_id", "junction_aa", "v_call", "j_call", "total", "samples", "distance"}}
	for _, r := range rows {
		out = append(out, []string{r.Centroid, r.ID, r.Junction, r.V, r.J, number(r.Total), strconv.Itoa(r.Samples), strconv.Itoa(r.Distance)})
	}
	return writeAll(w, out)
}

// InvariantSamples writes complete sample denominators and evaluable abundance alongside flags.
func InvariantSamples(w io.Writer, r *invariant.Result, names []string) error {
	rows := [][]string{{"sample", "species", "rule_version", "population", "total", "evaluable", "matched", "share"}}
	for s, x := range r.Samples {
		for p, name := range invariant.Populations {
			share := ""
			if x.Share[p] != nil {
				share = number(*x.Share[p])
			}
			rows = append(rows, []string{names[s], r.Species, strconv.Itoa(r.Version), name, number(x.Total), number(x.Evaluable[p]), number(x.Matched[p]), share})
		}
	}
	return writeAll(w, rows)
}
