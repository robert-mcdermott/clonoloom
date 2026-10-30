// Package cytolink links supplied CytoWeave index-sort observations to receptor
// cell IDs in one explicitly selected sample. It performs no phenotype inference.
package cytolink

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"clonoloom/internal/rep"
)

const MaxBytes = 4 << 20
const MaxRows = 10000
const Limits = "One declared acquisition and one receptor sample; exact well-to-cell mapping only. Links identify supplied observations, not biological singlets, paired receptors, validated phenotypes or independent subjects. Repeated chain rows count once per cell ID. No QC filter, compensation, transformation, gating, statistical test or embedding is computed. Empty, NaN and other supplied channel tokens stay distinct. Population is the exported deepest gate label, not all gate memberships. Provenance is supplied, not independently verified. No cross-sample fallback, barcode rewriting or scientific replay. Keep original FCS, workspace and experimental records."

var positive = regexp.MustCompile(`^[1-9][0-9]*$`)
var wellPattern = regexp.MustCompile(`^[A-Z]+[1-9][0-9]*$`)

// CSV preserves quoted CR/LF bytes as well as the original text stored by the API.
// Reject ambiguous malformed quoting, blank records, duplicate headers and ragged rows.
func CSV(text string) ([][]string, error) {
	if len(text) > MaxBytes || !utf8.ValidString(text) || strings.ContainsRune(text, 0) {
		return nil, errors.New("CSV must be UTF-8 without NUL, at most 4 MiB")
	}
	s := strings.TrimPrefix(text, "\ufeff")
	records := [][]string{}
	for i := 0; i < len(s); {
		if s[i] == '\r' || s[i] == '\n' {
			return nil, errors.New("Blank CSV record")
		}
		row := []string{}
		for {
			value := ""
			if i < len(s) && s[i] == '"' {
				i++
				var b strings.Builder
				closed := false
				for i < len(s) {
					if s[i] == '"' {
						i++
						if i < len(s) && s[i] == '"' {
							b.WriteByte('"')
							i++
							continue
						}
						closed = true
						break
					}
					b.WriteByte(s[i])
					i++
				}
				if !closed {
					return nil, errors.New("Unclosed CSV quote")
				}
				value = b.String()
			} else {
				start := i
				for i < len(s) && s[i] != ',' && s[i] != '\r' && s[i] != '\n' {
					if s[i] == '"' {
						return nil, errors.New("Quote in unquoted CSV field")
					}
					i++
				}
				value = s[start:i]
			}
			row = append(row, value)
			if len(row) > 128 {
				return nil, errors.New("CSV exceeds 128 columns")
			}
			if i == len(s) {
				break
			}
			c := s[i]
			i++
			if c == ',' {
				continue
			}
			if c == '\r' {
				if i >= len(s) || s[i] != '\n' {
					return nil, errors.New("Bare CR outside CSV field")
				}
				i++
				break
			}
			if c == '\n' {
				break
			}
			return nil, errors.New("Unexpected text after CSV quote")
		}
		if len(records) > 0 && len(row) != len(records[0]) {
			return nil, errors.New("CSV row width differs from header")
		}
		records = append(records, row)
		if len(records) > MaxRows+1 {
			return nil, errors.New("CSV exceeds 10000 data rows")
		}
	}
	if len(records) < 2 {
		return nil, errors.New("CSV needs a header and data")
	}
	seen := map[string]bool{}
	for _, h := range records[0] {
		if strings.TrimSpace(h) == "" || seen[h] {
			return nil, errors.New("Empty or duplicate CSV header")
		}
		seen[h] = true
	}
	return records, nil
}

type Observation struct {
	Values       []string `json:"values"`
	Cell         string   `json:"cell"`
	Status       string   `json:"status"`
	ReceptorRows []int    `json:"receptorRows"` // zero-based loaded source row indices
}
type Result struct {
	Columns            []string      `json:"columns"`
	Observations       []Observation `json:"observations"`
	ReceptorOnly       []string      `json:"receptorOnly"`
	ReceptorCells      int           `json:"receptorCells"`
	MissingBarcodeRows int           `json:"missingBarcodeRows"`
	Matched            int           `json:"matched"`
	Unmapped           int           `json:"unmapped"`
	AbsentTargets      int           `json:"absentTargets"`
}

func Link(ctx context.Context, t *rep.Table, sample int, wells, mapping string) (Result, error) {
	out := Result{Observations: []Observation{}, ReceptorOnly: []string{}}
	if sample < 0 || sample >= len(t.Samples) || t.Cell == nil {
		return out, errors.New("Select one receptor sample with cell IDs")
	}
	rows, e := CSV(wells)
	if e != nil {
		return out, e
	}
	m, e := CSV(mapping)
	if e != nil {
		return out, e
	}
	header := []string{"well", "row", "column", "event", "population"}
	if len(rows[0]) < len(header) {
		return out, errors.New("Expected CytoWeave wells CSV")
	}
	for i, v := range header {
		if rows[0][i] != v {
			return out, errors.New("Expected well,row,column,event,population followed by channel labels")
		}
	}
	if len(m[0]) != 2 || m[0][0] != "well" || m[0][1] != "cell_id" {
		return out, errors.New("Mapping header must be well,cell_id")
	}
	out.Columns = rows[0]
	byWell := map[string]bool{}
	events := map[string]bool{}
	for _, r := range rows[1:] {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		if !wellPattern.MatchString(r[0]) || !positive.MatchString(r[1]) || !positive.MatchString(r[2]) || !positive.MatchString(r[3]) {
			return out, errors.New("Wells and row/column/event identities must be canonical positive integers")
		}
		// Bounds apply to coordinates, never channel values or the decimal event token.
		ri, er := strconv.Atoi(r[1])
		ci, ec := strconv.Atoi(r[2])
		if er != nil || ec != nil || ri > 1024 || ci > 1024 {
			return out, errors.New("Plate coordinates exceed supported 1024 by 1024 bounds")
		}
		letters := ""
		for n := ri; n > 0; n = (n - 1) / 26 {
			letters = string(rune('A'+(n-1)%26)) + letters
		}
		if r[0] != letters+r[2] {
			return out, fmt.Errorf("Well %q contradicts row/column", r[0])
		}
		if byWell[r[0]] || events[r[3]] {
			return out, errors.New("Repeated well or event: identity is ambiguous; resolve upstream")
		}
		byWell[r[0]] = true
		events[r[3]] = true
	}
	cells := map[string][]int{}
	for i, s := range t.Sample {
		if i%65536 == 0 {
			if e := ctx.Err(); e != nil {
				return out, e
			}
		}
		if int(s) != sample {
			continue
		}
		c := t.Dict.Get(t.Cell[i])
		if c == "" {
			out.MissingBarcodeRows++
			continue
		}
		cells[c] = append(cells[c], i)
	}
	out.ReceptorCells = len(cells)
	mapped := map[string]string{}
	used := map[string]bool{}
	for _, r := range m[1:] {
		if !byWell[r[0]] {
			return out, fmt.Errorf("Mapping well %q is absent from this acquisition", r[0])
		}
		if strings.TrimSpace(r[1]) == "" || mapped[r[0]] != "" || used[r[1]] {
			return out, errors.New("Mapping needs one nonempty cell ID per well and no repeated cell targets")
		}
		mapped[r[0]] = r[1]
		used[r[1]] = true
	}
	matched := map[string]bool{}
	for _, r := range rows[1:] {
		o := Observation{Values: r, Cell: mapped[r[0]], ReceptorRows: []int{}}
		switch {
		case o.Cell == "":
			o.Status = "unmapped"
			out.Unmapped++
		case len(cells[o.Cell]) == 0:
			o.Status = "absent receptor target"
			out.AbsentTargets++
		default:
			o.Status = "linked"
			o.ReceptorRows = cells[o.Cell]
			out.Matched++
			matched[o.Cell] = true
		}
		out.Observations = append(out.Observations, o)
	}
	for c := range cells {
		if !matched[c] {
			out.ReceptorOnly = append(out.ReceptorOnly, c)
		}
	}
	sort.Strings(out.ReceptorOnly)
	return out, nil
}
