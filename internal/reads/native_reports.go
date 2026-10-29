package reads

// Format contracts, not vendor algorithms:
// https://github.com/liulab-dfci/TRUST4/blob/master/README.md
// https://vdjtools-doc.readthedocs.io/en/master/input.html
import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"unicode/utf8"

	"clonoloom/internal/rep"
)

const maxNativeBytes = 64 << 20
const maxNativeRows = 1000000

// NativeTable retains source fields the analysis table cannot represent. Text is
// the original physical line, excluding its LF/CRLF. Numeric evidence stays text.
type NativeTable struct {
	Header string      `json:"header"`
	Rows   []NativeRow `json:"rows"`
}
type NativeRow struct {
	Line uint32 `json:"line"`
	Text string `json:"text"`
}

func (n *NativeTable) Size() int {
	if n == nil {
		return 0
	}
	size := len(n.Header)
	for _, r := range n.Rows {
		size += len(r.Text)
	}
	return size
}

func nativeColumn(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimPrefix(s, "#")
	switch s {
	case "freq", "frequency(proportion of read_count)":
		return "frequency"
	case "cdr3_dna":
		return "cdr3nt"
	case "cdr3_amino_acids":
		return "cdr3aa"
	case "consensus_id":
		return "cid"
	case "consensus_id_complete_vdj", "consensus_id_full_length":
		return "cid_full_length"
	}
	return s
}

func nativeReportFormat(header []string) Format {
	c := map[string]bool{}
	for _, name := range header {
		c[nativeColumn(name)] = true
	}
	if !c["cdr3nt"] || !c["cdr3aa"] || !c["v"] || !c["j"] || !c["frequency"] || !(c["count"] || c["read_count"]) {
		return ""
	}
	if c["read_count"] || c["cid"] {
		return FormatTRUST4
	}
	return FormatVDJtools
}

func readNativeReport(ls *lines, header []string, src Source, b **rep.Builder, index int, sample string, p *Problems) (Source, rep.Unit, error) {
	c := map[string]int{}
	for i, name := range header {
		key := nativeColumn(name)
		if key == "" || !utf8.ValidString(name) {
			return src, "", fmt.Errorf("native report has an empty or invalid header")
		}
		if _, ok := c[key]; ok {
			return src, "", fmt.Errorf("native report has ambiguous duplicate column %q", name)
		}
		c[key] = i
	}
	_, countGiven := c["count"]
	_, readCount := c["read_count"]
	if countGiven && readCount {
		return src, "", fmt.Errorf("native report has both count and read_count; choose an explicit column mapping")
	}
	if _, ok := c["d"]; !ok {
		return src, "", fmt.Errorf("native report requires the D column (use . for an unknown/not-applicable call)")
	}
	if src.Format == FormatVDJtools {
		if _, ok := c["c"]; ok {
			return src, "", fmt.Errorf("a C column without a TRUST4 consensus identifier has ambiguous report conventions; use an explicit column mapping or AIRR export")
		}
	} else if _, ok := c["c"]; !ok {
		return src, "", fmt.Errorf("TRUST4 report requires the C column")
	}
	unit := rep.UnitUnknown
	countColumn := "count"
	if readCount {
		countColumn, unit = "read_count", rep.Reads
	}
	if *b == nil {
		*b = rep.NewBuilder(unit)
	}
	before := (*b).Len()
	src.Native = &NativeTable{Header: string(ls.line), Rows: []NativeRow{}}
	size := len(src.Native.Header)
	at := func(fields []string, key string) string {
		i, ok := c[key]
		if !ok {
			return ""
		}
		return strings.TrimSpace(fields[i])
	}
	var fields []string
	for ls.next() {
		if ls.ctx != nil {
			if err := ls.ctx.Err(); err != nil {
				return src, "", err
			}
		}
		if len(ls.line) == 0 {
			continue
		}
		size += len(ls.line)
		if size > maxNativeBytes || len(src.Native.Rows) >= maxNativeRows {
			return src, "", fmt.Errorf("native report exceeds 64 MiB retained text or 1000000 rows; split the input explicitly")
		}
		if !utf8.Valid(ls.line) || strings.ContainsAny(string(ls.line), "\x00\r") {
			return src, "", fmt.Errorf("line %d: native report text is not valid UTF-8 tabular text", ls.n)
		}
		fields = splitTab(ls.line, fields)
		if len(fields) != len(header) {
			return src, "", fmt.Errorf("line %d: native report has %d fields, header has %d; no rows were imported", ls.n, len(fields), len(header))
		}
		r := rep.NewRow()
		r.Source, r.Line, r.Sample = index, ls.n, sample
		var err error
		r.Count, err = nativeCount(at(fields, countColumn), src.Format == FormatTRUST4)
		if err != nil {
			return src, "", fmt.Errorf("line %d, %s: %w", ls.n, countColumn, err)
		}
		if unit == rep.Reads {
			r.Reads = r.Count
		}
		if freq := at(fields, "frequency"); !nativeMissing(freq) {
			f, err := strconv.ParseFloat(freq, 64)
			if err != nil || math.IsNaN(f) || math.IsInf(f, 0) || f < 0 || f > 1 {
				return src, "", fmt.Errorf("line %d: frequency must be finite and within [0,1], or missing", ls.n)
			}
			// Bound lexical work, then check exact endpoints: float64 can round
			// 1.000000000000000001 down to 1 or a tiny negative value to zero.
			if !nativeFrequency(freq) {
				return src, "", fmt.Errorf("line %d: frequency is outside the supported decimal syntax or exact [0,1] interval", ls.n)
			}
		}
		gene := func(key string) string {
			v := at(fields, key)
			if nativeMissing(v) || v == "*" {
				return ""
			}
			return v // Keep all alternative calls; do not select a preferred gene.
		}
		r.V, r.D, r.J, r.C = gene("v"), gene("d"), gene("j"), gene("c")
		r.SequenceID = at(fields, "cid")
		if nativeMissing(r.SequenceID) || r.SequenceID == "*" {
			r.SequenceID = ""
		}
		r.Junction, r.JunctionAA = at(fields, "cdr3nt"), at(fields, "cdr3aa")
		if err := nativeJunction(&r, src.Format); err != nil {
			return src, "", fmt.Errorf("line %d: %w", ls.n, err)
		}
		if strings.EqualFold(at(fields, "cdr3aa"), "partial") && src.Format == FormatTRUST4 {
			p.addf("report-partial", index, ls.n, "CDR3aa", "partial", "TRUST4 marks this junction partial: retained in native evidence, omitted from complete-junction clonotyping")
		}
		if err := (*b).Append(&r); err != nil {
			return src, "", err
		}
		src.Native.Rows = append(src.Native.Rows, NativeRow{Line: ls.n, Text: string(ls.line)})
	}
	if ls.err != nil {
		return src, "", ls.err
	}
	src.Rows = (*b).Len() - before
	return src, unit, nil
}

// Only plain decimal counts, bounded before float64 analysis. Original lexical
// values survive in NativeTable; no frequency is ever used to reconstruct counts.
func nativeCount(s string, fractional bool) (float64, error) {
	if nativeMissing(s) {
		return math.NaN(), nil
	}
	if len(s) > 128 || s == "" || strings.Trim(s, "0123456789.") != "" || strings.Count(s, ".") > 1 || s[0] == '.' || s[len(s)-1] == '.' {
		return 0, fmt.Errorf("count must be a nonnegative plain decimal or missing")
	}
	x, ok := new(big.Rat).SetString(s)
	if !ok || x.Cmp(new(big.Rat).SetInt64(9007199254740991)) > 0 || (!fractional && !x.IsInt()) {
		return 0, fmt.Errorf("count exceeds 2^53-1 or is fractional in a VDJtools table")
	}
	f, _ := x.Float64()
	return f, nil
}

func nativeMissing(s string) bool {
	return s == "" || s == "." || strings.EqualFold(s, "NA")
}

func nativeFrequency(s string) bool {
	if len(s) > 128 || strings.Trim(s, "0123456789.eE+-") != "" {
		return false
	}
	if i := strings.IndexAny(s, "eE"); i >= 0 {
		e, err := strconv.Atoi(s[i+1:])
		if err != nil || e < -308 || e > 308 {
			return false
		}
	}
	x, ok := new(big.Rat).SetString(s)
	return ok && x.Sign() >= 0 && x.Cmp(big.NewRat(1, 1)) <= 0
}

func nativeJunction(r *rep.Row, format Format) error {
	if nativeMissing(r.Junction) {
		r.Junction = ""
	}
	if nativeMissing(r.JunctionAA) {
		r.JunctionAA = ""
	}
	// ASCII before case conversion: Unicode lookalikes are not residues.
	if strings.Trim(r.Junction, "ACGTNacgtn") != "" {
		return fmt.Errorf("CDR3nt must contain only ASCII A/C/G/T/N, or be missing")
	}
	r.Junction = strings.ToUpper(r.Junction)
	if format == FormatTRUST4 {
		switch strings.ToLower(r.JunctionAA) {
		case "partial":
			r.Junction, r.JunctionAA = "", ""
			return nil
		case "out_of_frame":
			r.JunctionAA = ""
			r.Productive, r.VJInFrame = rep.False, rep.False
			return nil
		}
		if strings.Trim(r.JunctionAA, "ACDEFGHIKLMNPQRSTVWYXacdefghiklmnpqrstvwyx_*?") != "" {
			return fmt.Errorf("unsupported TRUST4 amino-acid notation")
		}
		r.JunctionAA = strings.NewReplacer("_", "*", "?", "X").Replace(r.JunctionAA)
	} else {
		if strings.Trim(r.JunctionAA, "ACDEFGHIKLMNPQRSTVWYXacdefghiklmnpqrstvwyx_*~") != "" {
			return fmt.Errorf("unsupported VDJtools amino-acid notation")
		}
		if strings.ContainsAny(r.JunctionAA, "_~") {
			r.JunctionAA = ""
			r.Productive, r.VJInFrame = rep.False, rep.False
			return nil
		}
	}
	r.JunctionAA = strings.ToUpper(r.JunctionAA)
	if r.JunctionAA == "" {
		return nil // Do not translate a missing source peptide or certify productivity.
	}
	switch {
	case strings.Contains(r.JunctionAA, "*"):
		r.StopCodon = rep.True
	case !strings.Contains(r.JunctionAA, "X"):
		r.StopCodon = rep.False
	} // X may conceal a stop: leave the flag unknown, not false.
	if r.Junction != "" {
		r.VJInFrame = rep.FlagOf(len(r.Junction)%3 == 0)
	}
	if r.StopCodon == rep.True || r.VJInFrame == rep.False {
		r.Productive = rep.False
	} else if !strings.Contains(r.JunctionAA, "X") && r.VJInFrame == rep.True {
		r.Productive = rep.True
	}
	return nil
}
