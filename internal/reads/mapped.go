package reads

// Tables of other tools (clonoloom-spec/requirements.md, D7): a CSV or TSV ClonoLoom does not
// recognize is read through a mapping of its columns to the fields ClonoLoom uses, which the user
// confirms after seeing the columns checked one by one. The mapping is kept with the dataset, so the
// file is read the same way again.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"clonoloom/internal/genes"
	"clonoloom/internal/rep"
	"clonoloom/internal/seq"
)

// FormatTable is a table read through a mapping.
const FormatTable Format = "table"

// Mapping says which column holds which field (by the column's name in the header), what the count
// counts, and the sample's name when no column holds it (the file's name when empty).
type Mapping struct {
	Columns map[string]string `json:"columns"`
	Unit    rep.Unit          `json:"unit,omitempty"`
	Sample  string            `json:"sample,omitempty"`
}

// MappableFields are the fields a mapping can give, in the order offered.
var MappableFields = []struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}{
	{"junction_aa", "Junction, amino acids"}, {"junction", "Junction, nucleotides"},
	{"v_call", "V gene"}, {"d_call", "D gene"}, {"j_call", "J gene"}, {"c_call", "C gene"},
	{"count", "Count"}, {"sample", "Sample"}, {"locus", "Locus"}, {"productive", "Productive"},
	{"cell_id", "Cell barcode"}, {"clone_id", "Clone or clonotype ID"}, {"sequence_id", "Sequence ID"},
}

func mappable(field string) bool {
	for _, f := range MappableFields {
		if f.ID == field {
			return true
		}
	}
	return false
}

// Validate says what is wrong with a mapping for a header: an unknown field, a column the header
// lacks, no junction, a count without a unit.
func (m Mapping) Validate(header []string) error {
	has := map[string]bool{}
	for _, h := range header {
		has[h] = true
	}
	for field, column := range m.Columns {
		if !mappable(field) {
			return fmt.Errorf("%s is not a field ClonoLoom maps", field)
		}
		if column != "" && !has[column] {
			return fmt.Errorf("no column %q in the file", column)
		}
	}
	if m.Columns["junction_aa"] == "" && m.Columns["junction"] == "" {
		return errors.New("map a junction (amino acids or nucleotides): clonotypes are defined by it")
	}
	if m.Columns["count"] != "" {
		switch m.Unit {
		case rep.Reads, rep.UMIs, rep.Templates, rep.Cells:
		default:
			return errors.New("say what the count counts: reads, UMIs, templates or cells")
		}
	}
	return nil
}

// separator is a table's column separator, from its header: a tab, else whichever of semicolon and
// comma it has more of.
func separator(line []byte) byte {
	switch {
	case bytes.IndexByte(line, '\t') >= 0:
		return '\t'
	case bytes.Count(line, []byte(";")) > bytes.Count(line, []byte(",")):
		return ';'
	}
	return ','
}

func splitRow(line []byte, sep byte, fields []string) []string {
	if sep == '\t' {
		return splitTab(line, fields)
	}
	out, _ := splitSep(line, sep, fields)
	return out
}

// Inspection is what a table holds, to map its columns: its separator, header, first rows, and a
// mapping guessed from the columns' names.
type Inspection struct {
	Separator string     `json:"separator"`
	Header    []string   `json:"header"`
	Rows      [][]string `json:"rows"`
	Guess     Mapping    `json:"guess"`
}

// Inspect reads a table's header and first rows.
func Inspect(r io.Reader, rows int) (Inspection, error) {
	plain, _, err := decompress(r)
	if err != nil {
		return Inspection{}, err
	}
	ls := newLines(plain)
	if !ls.next() {
		if ls.err != nil {
			return Inspection{}, ls.err
		}
		return Inspection{}, errors.New("the file is empty")
	}
	sep := separator(ls.line)
	header := cleanHeader(splitRow(ls.line, sep, nil))
	in := Inspection{Separator: map[byte]string{'\t': "tab", ',': "comma", ';': "semicolon"}[sep], Header: header}
	for len(in.Rows) < rows && ls.next() {
		if len(bytes.TrimSpace(ls.line)) > 0 {
			in.Rows = append(in.Rows, append([]string(nil), splitRow(ls.line, sep, nil)...))
		}
	}
	in.Guess = guess(header, in.Rows)
	return in, nil
}

// cleanHeader trims the header's names (and a UTF-8 byte-order mark).
func cleanHeader(header []string) []string {
	for i := range header {
		header[i] = strings.TrimSpace(strings.TrimPrefix(header[i], "\uFEFF"))
	}
	return header
}

var notWord = regexp.MustCompile(`[^a-z0-9]+`)

// guess maps columns by their names, as the tools that write such tables name them.
func guess(header []string, rows [][]string) Mapping {
	m := Mapping{Columns: map[string]string{}}
	names := map[string][]string{
		"junction_aa": {"junctionaa", "cdr3aa", "aaseqcdr3", "aminoacid", "cdr3amino", "cdr3aaseq", "cdr3aminoacid", "aacdr3"},
		"junction":    {"junctionnt", "cdr3nt", "nseqcdr3", "cdr3nucleotide", "cdr3ntseq", "ntcdr3", "nucleotide"},
		"v_call":      {"vcall", "vgene", "vsegment", "bestvgene", "allvhitswithscore", "vhit", "vgenename", "v"},
		"d_call":      {"dcall", "dgene", "dsegment", "bestdgene", "allvdhitswithscore", "alldhitswithscore", "dhit", "d"},
		"j_call":      {"jcall", "jgene", "jsegment", "bestjgene", "alljhitswithscore", "jhit", "j"},
		"c_call":      {"ccall", "cgene", "csegment", "bestcgene", "allchitswithscore", "isotype", "c"},
		"count":       {"count", "clonecount", "readcount", "reads", "templates", "umicount", "umis", "duplicatecount", "uniquemoleculecount", "consensuscount"},
		"sample":      {"sample", "samplename", "sampleid", "repertoireid", "repertoire"},
		"locus":       {"locus", "chain", "chaintype"},
		"productive":  {"productive", "frametype", "functional", "functionality"},
		"cell_id":     {"cellid", "barcode", "cellbarcode"},
		"clone_id":    {"cloneid", "clonotypeid", "rawclonotypeid", "clonoid"},
		"sequence_id": {"sequenceid", "contigid", "seqid", "readid"},
	}
	normal := make([]string, len(header))
	for i, h := range header {
		normal[i] = notWord.ReplaceAllString(strings.ToLower(h), "")
	}
	used := map[int]bool{}
	for _, f := range MappableFields {
		for _, want := range names[f.ID] {
			for i, n := range normal {
				if n == want && !used[i] {
					m.Columns[f.ID] = header[i]
					used[i] = true
					break
				}
			}
			if m.Columns[f.ID] != "" {
				break
			}
		}
	}
	// A column named "cdr3" (or "junction") holds amino acids or nucleotides: as its values say.
	for i, n := range normal {
		if used[i] || (n != "cdr3" && n != "junction") {
			continue
		}
		field := "junction_aa"
		if looksNucleotide(columnValues(rows, i)) {
			field = "junction"
		}
		if m.Columns[field] == "" {
			m.Columns[field] = header[i]
			used[i] = true
		}
	}
	if c := strings.ToLower(m.Columns["count"]); c != "" {
		switch {
		case strings.Contains(c, "umi") || strings.Contains(c, "molecule"):
			m.Unit = rep.UMIs
		case strings.Contains(c, "template"):
			m.Unit = rep.Templates
		case strings.Contains(c, "read") || strings.Contains(c, "duplicate"):
			m.Unit = rep.Reads
		}
	}
	return m
}

func columnValues(rows [][]string, i int) []string {
	var out []string
	for _, r := range rows {
		if i < len(r) && strings.TrimSpace(r[i]) != "" {
			out = append(out, strings.TrimSpace(r[i]))
		}
	}
	return out
}

var (
	nucleotides = regexp.MustCompile(`^[ACGTUNacgtun.-]+$`)
	aminoAcids  = regexp.MustCompile(`^[ACDEFGHIKLMNPQRSTVWYXacdefghiklmnpqrstvwyx*_]+$`)
)

func looksNucleotide(values []string) bool {
	if len(values) == 0 {
		return false
	}
	n := 0
	for _, v := range values {
		if nucleotides.MatchString(v) {
			n++
		}
	}
	return n*10 >= len(values)*9
}

// ColumnCheck is a mapped column checked: of its rows with a value, how many are valid for the
// field, a few that are not, and a note.
type ColumnCheck struct {
	Field   string   `json:"field"`
	Column  string   `json:"column"`
	Values  int      `json:"values"`
	Valid   int      `json:"valid"`
	Invalid []string `json:"invalid,omitempty"`
	Note    string   `json:"note,omitempty"`
}

// MappingCheck is a table checked through a mapping, column by column.
type MappingCheck struct {
	Rows    int           `json:"rows"`
	Columns []ColumnCheck `json:"columns"`
	Samples []string      `json:"samples"`
}

// Check reads a whole table through a mapping and checks each mapped column's values for its field.
func Check(ctx context.Context, r io.Reader, name string, m Mapping) (MappingCheck, error) {
	plain, _, err := decompress(r)
	if err != nil {
		return MappingCheck{}, err
	}
	ls := newLines(plain)
	ls.ctx = ctx
	if !ls.next() {
		return MappingCheck{}, errors.New("the file is empty")
	}
	sep := separator(ls.line)
	header := cleanHeader(splitRow(ls.line, sep, nil))
	if err := m.Validate(header); err != nil {
		return MappingCheck{}, err
	}
	index := map[string]int{}
	for i, h := range header {
		index[h] = i
	}
	var out MappingCheck
	checks := map[string]*ColumnCheck{}
	var anchored, inFrame, whole float64
	samples := map[string]bool{}
	for _, f := range MappableFields {
		if c := m.Columns[f.ID]; c != "" {
			checks[f.ID] = &ColumnCheck{Field: f.ID, Column: c}
		}
	}
	var fields []string
	for ls.next() {
		if len(bytes.TrimSpace(ls.line)) == 0 {
			continue
		}
		fields = splitRow(ls.line, sep, fields)
		out.Rows++
		sample := m.Sample
		if sample == "" {
			sample = sampleName(name)
		}
		for field, c := range checks {
			i := index[c.Column]
			v := ""
			if i < len(fields) {
				v = strings.TrimSpace(fields[i])
			}
			if field == "sample" && v != "" {
				sample = v
			}
			if v == "" || missingValue(v) {
				continue
			}
			c.Values++
			ok := true
			switch field {
			case "junction_aa":
				ok = aminoAcids.MatchString(v)
				if ok && (v[0] == 'C' || v[0] == 'c') && strings.ContainsAny(v[len(v)-1:], "FWfw") {
					anchored++
				}
			case "junction":
				ok = nucleotides.MatchString(v)
				if ok && len(v)%3 == 0 {
					inFrame++
				}
			case "v_call", "d_call", "j_call", "c_call":
				ok = len(genes.ParseCall(v).Names) > 0
			case "count":
				x, err := strconv.ParseFloat(v, 64)
				ok = err == nil && x >= 0 && !math.IsInf(x, 0)
				if ok && x == math.Trunc(x) {
					whole++
				}
			case "productive":
				_, ok = parseFlag(v)
			case "locus":
				ok = rep.LocusCode(strings.ToUpper(v)) > 0 || strings.HasPrefix(strings.ToUpper(v), "TC")
			}
			if ok {
				c.Valid++
			} else if len(c.Invalid) < 3 {
				c.Invalid = append(c.Invalid, v)
			}
		}
		samples[sample] = true
	}
	if ls.err != nil {
		return out, ls.err
	}
	for _, f := range MappableFields {
		c, ok := checks[f.ID]
		if !ok {
			continue
		}
		switch f.ID {
		case "junction_aa":
			if c.Valid > 0 {
				c.Note = fmt.Sprintf("%.0f%% begin with C and end with F or W, as IMGT junctions do (a CDR3 without them is read as given, and the validator says so)", 100*anchored/float64(c.Valid))
			}
		case "junction":
			if c.Valid > 0 {
				c.Note = fmt.Sprintf("%.0f%% are whole codons", 100*inFrame/float64(c.Valid))
			}
		case "count":
			if c.Valid > 0 && whole < float64(c.Valid) {
				c.Note = fmt.Sprintf("%d are not whole numbers: diversity's estimators need counts", c.Valid-int(whole))
			}
		}
		if c.Values < out.Rows && (f.ID == "junction_aa" || f.ID == "junction" || f.ID == "count") {
			if c.Note != "" {
				c.Note += "; "
			}
			c.Note += fmt.Sprintf("%d rows have none", out.Rows-c.Values)
		}
		out.Columns = append(out.Columns, *c)
	}
	if _, ok := checks["productive"]; !ok {
		out.Columns = append(out.Columns, ColumnCheck{Field: "productive", Note: "no column: a row is productive when its junction is whole codons without a stop"})
	}
	for s := range samples {
		out.Samples = append(out.Samples, s)
	}
	sort.Strings(out.Samples)
	return out, nil
}

// missingValue reports whether a value means "none" as tables write it.
func missingValue(v string) bool {
	switch strings.ToLower(v) {
	case "", "na", "nan", "null", "none", "-", ".", "n/a":
		return true
	}
	return false
}

// parseFlag reads a productive flag as the tools write it.
func parseFlag(v string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "t", "true", "yes", "y", "1", "in", "productive", "functional", "f-productive":
		return true, true
	case "f", "false", "no", "n", "0", "out", "stop", "unproductive", "nonproductive", "non-productive", "nonfunctional":
		return false, true
	}
	return false, false
}

// readMapped reads a table through its mapping into the builder.
func readMapped(ctx context.Context, r io.Reader, path string, index int, b **rep.Builder, p *Problems, m Mapping) (Source, rep.Unit, error) {
	src := Source{Path: path, Name: filepath.Base(path), Format: FormatTable, Mapping: &m}
	plain, gzipped, err := decompress(r)
	if err != nil {
		return src, "", err
	}
	src.Gzipped = gzipped
	ls := newLines(plain)
	ls.ctx = ctx
	if !ls.next() {
		if ls.err != nil {
			return src, "", ls.err
		}
		return src, "", errors.New("the file is empty")
	}
	sep := separator(ls.line)
	header := cleanHeader(splitRow(ls.line, sep, nil))
	src.Header = append([]string(nil), header...)
	if err := m.Validate(header); err != nil {
		return src, "", err
	}
	col := map[string]int{}
	for i, h := range header {
		col[h] = i
	}
	at := func(fields []string, field string) string {
		c, ok := m.Columns[field]
		if !ok || c == "" {
			return ""
		}
		i := col[c]
		if i >= len(fields) {
			return ""
		}
		v := strings.TrimSpace(fields[i])
		if missingValue(v) {
			return ""
		}
		return v
	}
	unit := m.Unit
	if m.Columns["count"] == "" {
		unit = rep.Sequences
	}
	if *b == nil {
		*b = rep.NewBuilder(unit)
	}
	before := (*b).Len()
	sample := m.Sample
	if sample == "" {
		sample = sampleName(path)
	}
	var fields []string
	for ls.next() {
		if len(bytes.TrimSpace(ls.line)) == 0 {
			continue
		}
		fields = splitRow(ls.line, sep, fields)
		row := rep.NewRow()
		row.Source, row.Line = index, ls.n
		row.Sample = sample
		if s := at(fields, "sample"); s != "" {
			row.Sample = s
		}
		row.Locus = strings.ToUpper(at(fields, "locus"))
		if rep.LocusCode(row.Locus) == 0 {
			row.Locus = ""
		}
		row.V, row.D, row.J, row.C = at(fields, "v_call"), at(fields, "d_call"), at(fields, "j_call"), at(fields, "c_call")
		row.Junction, row.JunctionAA = strings.ToUpper(at(fields, "junction")), strings.ToUpper(at(fields, "junction_aa"))
		row.Cell, row.Clone, row.SequenceID = at(fields, "cell_id"), at(fields, "clone_id"), at(fields, "sequence_id")
		if m.Columns["count"] == "" {
			row.Count = 1
		} else if v := at(fields, "count"); v != "" {
			x, err := strconv.ParseFloat(v, 64)
			if err != nil || x < 0 {
				p.addf("not-number", index, ls.n, m.Columns["count"], v, "the count %q is not a number", v)
			} else {
				row.Count = x
			}
		}
		if v := at(fields, "productive"); v != "" {
			if f, ok := parseFlag(v); ok {
				row.Productive = rep.FlagOf(f)
			}
		} else if m.Columns["productive"] == "" {
			row.Productive = inferProductive(row.Junction, row.JunctionAA)
		}
		if err := (*b).Append(&row); err != nil {
			return src, "", err
		}
	}
	if ls.err != nil {
		return src, "", ls.err
	}
	src.Rows = (*b).Len() - before
	return src, unit, nil
}

// inferProductive is a row's productivity when no column says it: productive when its junction is
// whole codons and has no stop (amino acids preferred, else the nucleotides translated).
func inferProductive(nt, aa string) rep.Flag {
	if aa == "" && nt != "" {
		if len(nt)%3 != 0 {
			return rep.False
		}
		aa = seq.Translate(nt)
	}
	if aa == "" {
		return rep.Missing
	}
	if strings.ContainsAny(aa, "*_") || (nt != "" && len(nt)%3 != 0) {
		return rep.False
	}
	return rep.True
}
