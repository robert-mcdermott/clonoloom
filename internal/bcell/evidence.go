// Package bcell inspects supplied sequence evidence, not inferred germlines or mutations.
// AIRR Rearrangement schema 2.0, https://docs.airr-community.org/en/v2.0.0/datarep/rearrangements.html
// Vander Heiden et al. 2018, Front Immunol 9:2206, doi:10.3389/fimmu.2018.02206.
package bcell

import (
	"fmt"
	"strconv"
	"strings"

	"clonoloom/internal/airr"
	"clonoloom/internal/rep"
)

var fields = func() map[string]int {
	m := map[string]int{}
	for i, name := range rep.EvidenceFields {
		m[name] = i
	}
	return m
}()

type Issue struct {
	Code    string `json:"code"`
	Field   string `json:"field"`
	Value   string `json:"value"`
	Message string `json:"message"`
}

type Evidence struct {
	Sequence bool    `json:"sequence"`
	Paired   bool    `json:"paired"`
	Valid    bool    `json:"valid"`
	Regions  bool    `json:"regions"`
	Issues   []Issue `json:"issues"`
}

// Interval translates a positive, closed input interval only after checking its bounds.
// length < 0 means the corresponding reference string is not available.
func Interval(start, end, length int64) (int64, int64, error) {
	if start < 1 || end < start || (length >= 0 && end > length) {
		return 0, 0, fmt.Errorf("interval %d–%d is outside the available sequence", start, end)
	}
	return start - 1, end, nil
}

// Cigar retains offsets and consumed spans, excluding optional trailing S/N.
// AIRR allows = X M D I and terminal S/N; S precedes N at either end.
// IMGT spacers in alignment strings are not CIGAR operations.
type Cigar struct{ QueryStart, ReferenceStart, QueryLength, ReferenceLength, QueryTotal int64 }

func ParseCigar(s string) (Cigar, error) {
	var c Cigar
	bad := func() (Cigar, error) {
		return Cigar{}, fmt.Errorf("invalid AIRR CIGAR: positive lengths, =/X or M, D/I, and terminal S then N only")
	}
	var n int64
	digits, core, tail, m, eq, leadingN, trailingN := false, false, false, false, false, false, false
	const limit = int64(1<<31 - 1)
	add := func(p *int64, v int64) bool {
		if v > limit-*p {
			return false
		}
		*p += v
		return true
	}
	for _, op := range s {
		if op >= '0' && op <= '9' {
			if n > (limit-int64(op-'0'))/10 {
				return bad()
			}
			n = n*10 + int64(op-'0')
			digits = true
			continue
		}
		if !digits || n == 0 {
			return bad()
		}
		switch op {
		case 'S', 'N':
			if !core {
				if op == 'S' {
					if leadingN || !add(&c.QueryStart, n) {
						return bad()
					}
				} else {
					leadingN = true
					if !add(&c.ReferenceStart, n) {
						return bad()
					}
				}
			} else {
				tail = true
				if op == 'S' {
					if trailingN {
						return bad()
					}
				} else {
					trailingN = true
				}
			}
			if op == 'S' && !add(&c.QueryTotal, n) {
				return bad()
			}
		case 'M', '=', 'X', 'D', 'I':
			if tail {
				return bad()
			}
			core = true
			if op == 'M' {
				m = true
			}
			if op == '=' || op == 'X' {
				eq = true
			}
			if m && eq {
				return bad()
			}
			if op != 'D' {
				if !add(&c.QueryLength, n) || !add(&c.QueryTotal, n) {
					return bad()
				}
			}
			if op != 'I' && !add(&c.ReferenceLength, n) {
				return bad()
			}
		default:
			return bad()
		}
		n = 0
		digits = false
	}
	if digits || !core || c.QueryLength == 0 || c.ReferenceLength == 0 {
		return bad()
	}
	return c, nil
}

// Check never aligns or repairs the supplied strings. Valid means structural checks passed,
// not that the germline, numbering scheme or region mapping is biologically established.
func Check(raw [rep.EvidenceCount]string) Evidence {
	get := func(name string) string { return raw[fields[name]] }
	out := Evidence{Sequence: get("sequence") != "", Paired: get("sequence_alignment") != "" && get("germline_alignment") != "", Issues: []Issue{}}
	issue := func(code, field, message string) {
		out.Issues = append(out.Issues, Issue{code, field, get(field), message})
	}
	for _, name := range []string{"sequence", "sequence_alignment", "germline_alignment"} {
		for _, c := range strings.ToUpper(get(name)) {
			if !strings.ContainsRune("ACGTRYSWKMBDHVN.-", c) {
				issue("evidence-alphabet", name, "Use nucleotide IUPAC symbols and documented gap/spacer characters; the original value is retained.")
				break
			}
		}
	}
	if out.Paired && len(get("sequence_alignment")) != len(get("germline_alignment")) {
		issue("evidence-alignment", "germline_alignment", "The supplied query and germline alignments have different lengths; export a paired alignment from the annotation tool.")
	}
	if v := get("rev_comp"); v != "" {
		if _, _, ok := airr.ParseBool(v); !ok {
			issue("evidence-orientation", "rev_comp", "Supply a valid AIRR orientation flag; no sequence is reversed here.")
		}
	}
	coord := func(name string) (int64, bool) {
		v := get(name)
		if v == "" {
			return 0, false
		}
		// AIRR accepts an integer written as a decimal, but large integers must not round.
		value, err := integer(v)
		if err != nil || value < 1 {
			issue("evidence-coordinate", name, "A position must be a positive integer in its named coordinate system.")
			return 0, false
		}
		return value, true
	}
	for i, name := range rep.EvidenceFields {
		if !strings.HasSuffix(name, "_start") {
			continue
		}
		endName := rep.EvidenceFields[i+1]
		start, hasStart := coord(name)
		end, hasEnd := coord(endName)
		length := int64(-1)
		if strings.Contains(name, "_sequence_") || strings.HasPrefix(name, "cdr") || strings.HasPrefix(name, "fwr") {
			if out.Sequence {
				length = int64(len(get("sequence")))
			}
		} else if strings.Contains(name, "_alignment_") {
			if v := get("sequence_alignment"); v != "" {
				length = int64(len(v))
			}
			if v := get("germline_alignment"); v != "" {
				if length < 0 || int64(len(v)) < length {
					length = int64(len(v))
				}
			}
		}
		if (hasStart && length >= 0 && start > length) || (hasEnd && length >= 0 && end > length) || (hasStart && hasEnd && end < start) {
			issue("evidence-coordinate", name, "The interval is reversed or exceeds its corresponding supplied string; check the annotation coordinates.")
		}
		if strings.HasPrefix(name, "cdr") || strings.HasPrefix(name, "fwr") {
			out.Regions = out.Regions || get(name) != "" || get(endName) != ""
		}
	}
	for _, name := range []string{"np1_length", "np2_length"} {
		if v := get(name); v != "" {
			n, err := integer(v)
			if err != nil || n < 0 {
				issue("evidence-coordinate", name, "An N/P length must be a nonnegative integer; zero is distinct from missing.")
			}
		}
	}
	for _, gene := range []string{"v", "d", "j"} {
		name := gene + "_cigar"
		if get(name) == "" {
			continue
		}
		cigar, err := ParseCigar(get(name))
		if err != nil {
			issue("evidence-cigar", name, err.Error())
			continue
		}
		if out.Sequence && cigar.QueryTotal > int64(len(get("sequence"))) {
			issue("evidence-cigar", name, "CIGAR consumes more query positions than the supplied sequence.")
		}
		for _, p := range []struct {
			space         string
			start, length int64
		}{{"sequence", cigar.QueryStart, cigar.QueryLength}, {"germline", cigar.ReferenceStart, cigar.ReferenceLength}} {
			for _, x := range []struct {
				suffix string
				want   int64
			}{{"start", p.start + 1}, {"end", p.start + p.length}} {
				value := get(gene + "_" + p.space + "_" + x.suffix)
				if value == "" {
					continue
				}
				n, e := integer(value)
				if e == nil && n > 0 && n != x.want {
					issue("evidence-cigar", name, "CIGAR offsets or consumed spans disagree with the supplied "+p.space+" coordinates.")
					break
				}
			}
		}
	}
	out.Valid = out.Paired && len(out.Issues) == 0
	return out
}

// Preserve integral decimal spellings without passing through float64 (large values round).
func integer(value string) (int64, error) {
	if at := strings.IndexByte(value, '.'); at >= 0 {
		tail := value[at+1:]
		if tail == "" || strings.Trim(tail, "0") != "" {
			return 0, fmt.Errorf("not an integer")
		}
		value = value[:at]
	}
	return strconv.ParseInt(value, 10, 64)
}
