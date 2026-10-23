// Package specificity matches receptor chains to published specificity records.
// VDJdb's legacy vdjdb.txt is read by header name, independently of its tools.
// See Shugay et al. 2018, doi:10.1093/nar/gkx760. A record is evidence for a
// chain, not proof of a paired receptor's specificity.
package specificity

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type Record struct {
	Row            int    `json:"row"`
	Complex        string `json:"complex"`
	Locus          string `json:"locus"`
	CDR3           string `json:"cdr3"`
	V              string `json:"v"`
	J              string `json:"j"`
	Species        string `json:"species"`
	MHCA           string `json:"mhcA"`
	MHCB           string `json:"mhcB"`
	MHCClass       string `json:"mhcClass"`
	Epitope        string `json:"epitope"`
	Antigen        string `json:"antigen"`
	AntigenSpecies string `json:"antigenSpecies"`
	Reference      string `json:"reference"`
	Score          int    `json:"score"`
	Method         string `json:"method"`
}

// Parse preserves every evidence row, including repeats and complex IDs. JSON
// columns in these unquoted TSV files are not interpreted as CSV quoting.
func Parse(r io.Reader) ([]Record, error) { return ParseContext(context.Background(), r) }

// ParseContext permits cancellation while reading a large release.
func ParseContext(ctx context.Context, r io.Reader) ([]Record, error) {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 65536), 8<<20)
	if !s.Scan() {
		return nil, fmt.Errorf("VDJdb: missing header: %v", s.Err())
	}
	cols := strings.Split(strings.TrimPrefix(s.Text(), "\ufeff"), "\t")
	index := map[string]int{}
	for i, c := range cols {
		if _, ok := index[c]; ok {
			return nil, fmt.Errorf("VDJdb: duplicate column %s", c)
		}
		index[c] = i
	}
	required := []string{"gene", "cdr3", "v.segm", "j.segm", "species", "mhc.a", "mhc.b", "mhc.class", "antigen.epitope", "antigen.gene", "antigen.species", "reference.id", "vdjdb.score"}
	for _, c := range required {
		if _, ok := index[c]; !ok {
			return nil, fmt.Errorf("VDJdb: missing column %s", c)
		}
	}
	out := []Record{}
	for row := 1; s.Scan(); row++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		fields := strings.Split(s.Text(), "\t")
		if len(fields) != len(cols) {
			return nil, fmt.Errorf("VDJdb row %d: got %d fields, want %d", row, len(fields), len(cols))
		}
		get := func(k string) string {
			if i, ok := index[k]; ok {
				return strings.TrimSpace(fields[i])
			}
			return ""
		}
		score, err := strconv.Atoi(get("vdjdb.score"))
		if err != nil || score < 0 || score > 3 {
			return nil, fmt.Errorf("VDJdb row %d: invalid score", row)
		}
		out = append(out, Record{Row: row, Complex: get("complex.id"), Locus: get("gene"), CDR3: get("cdr3"), V: get("v.segm"), J: get("j.segm"), Species: get("species"), MHCA: get("mhc.a"), MHCB: get("mhc.b"), MHCClass: get("mhc.class"), Epitope: get("antigen.epitope"), Antigen: get("antigen.gene"), AntigenSpecies: get("antigen.species"), Reference: get("reference.id"), Score: score, Method: get("method")})
	}
	return out, s.Err()
}
