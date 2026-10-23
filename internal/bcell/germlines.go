package bcell

// Reference-based, D-masked germlines: Gupta et al. 2015,
// doi:10.1093/bioinformatics/btv359; AIRR Rearrangement schema 2.0 CIGAR semantics.
// This independent implementation uses the intersection of explicit allele calls
// within each family/locus (not a fitted genotype or a majority vote). See research.md §5.17.
import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"clonoloom/internal/airr"
	"clonoloom/internal/genes"
	"clonoloom/internal/rep"
)

const GermlineVersion = 1
const maxGermlineLength = 10000
const maxGermlineBases = 50000000

// GermlineReference contains ungapped gene segments, keyed by exact allele names.
// Only data are read; no alignment program or external implementation is used.
type GermlineReference map[string]string

func ReadGermlineFASTA(r io.Reader) (GermlineReference, error) {
	out := GermlineReference{}
	limited := &io.LimitedReader{R: r, N: (10 << 20) + 1}
	scanner := bufio.NewScanner(limited)
	scanner.Buffer(make([]byte, 4096), maxGermlineLength+1024)
	name, seq := "", ""
	flush := func() error {
		if name == "" {
			return nil
		}
		if seq == "" || len(seq) > maxGermlineLength || strings.Trim(seq, "ACGTRYSWKMBDHVN") != "" {
			return fmt.Errorf("invalid ungapped reference sequence for %s", name)
		}
		if _, exists := out[name]; exists {
			return fmt.Errorf("duplicate reference allele %s", name)
		}
		out[name] = seq
		return nil
	}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, ">") {
			if err := flush(); err != nil {
				return nil, err
			}
			name = strings.TrimPrefix(line, ">")
			n := genes.Parse(name)
			if strings.ContainsAny(name, " \t\r\n") || n.Allele == "" || n.Gene+"*"+n.Allele != name || (n.Locus != "IGH" && n.Locus != "IGK" && n.Locus != "IGL") || (n.Segment != genes.V && n.Segment != genes.D && n.Segment != genes.J) {
				return nil, fmt.Errorf("reference header must be an exact IG V/D/J allele: %s", name)
			}
			seq = ""
		} else {
			if name == "" {
				return nil, errors.New("reference sequence before FASTA header")
			}
			seq += strings.ToUpper(line)
			if len(seq) > maxGermlineLength {
				return nil, errors.New("reference segment exceeds 10000 bases")
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if limited.N == 0 {
		return nil, errors.New("germline reference exceeds 10 MiB")
	}
	if err := flush(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, errors.New("empty germline reference")
	}
	return out, nil
}

type GermlineFamily struct {
	Family        string `json:"family"`
	Locus         string `json:"locus"`
	V             string `json:"v"`
	J             string `json:"j"`
	Rows          int    `json:"rows"`
	Reconstructed int    `json:"reconstructed"`
	Excluded      string `json:"excluded,omitempty"`
}

// Query/Germline are a newly projected alignment, NOT the supplied IMGT-numbered
// alignment. Regions labels V, J and masked junction N; query positions are 1-based
// oriented-sequence coordinates, zero for a deletion. Reference positions reset
// in each segment and are zero for insertions and masked junction bases.
type GermlineRow struct {
	FamilyRow
	Family             string `json:"family"`
	Observation        string `json:"observation"`
	Sample             string `json:"sample"`
	V                  string `json:"v"`
	J                  string `json:"j"`
	Query              string `json:"query,omitempty"`
	Germline           string `json:"germline,omitempty"`
	Regions            string `json:"regions,omitempty"`
	QueryPositions     []int  `json:"queryPositions,omitempty"`
	ReferencePositions []int  `json:"referencePositions,omitempty"`
	Excluded           string `json:"excluded,omitempty"`
}
type GermlineResult struct {
	Families      []GermlineFamily `json:"families"`
	Rows          []GermlineRow    `json:"rows"`
	Reconstructed int              `json:"reconstructed"`
	Excluded      map[string]int   `json:"excluded"`
	Description   string           `json:"description"`
}

func alleleIntersection(t *rep.Table, rows []int, segment genes.Segment, ref GermlineReference) (string, string) {
	var common map[string]bool
	for _, row := range rows {
		code := t.V[row]
		if segment == genes.J {
			code = t.J[row]
		}
		call := t.Call(code)
		next := map[string]bool{}
		if call.Unreadable != 0 || len(call.Names) == 0 {
			return "", "missing or unreadable allele call"
		}
		for _, n := range call.Names {
			if n.Allele == "" || n.Segment != segment || n.Locus != t.LocusName(row) {
				return "", "requires explicit consistent V/J allele calls"
			}
			name := n.Gene + "*" + n.Allele
			if common == nil || common[name] {
				next[name] = true
			}
		}
		common = next
	}
	if len(common) == 0 {
		return "", "conflicting family allele calls"
	}
	if len(common) != 1 {
		return "", "unresolved family allele ambiguity"
	}
	for name := range common {
		if ref[name] == "" {
			return "", "allele absent from selected reference"
		}
		return name, ""
	}
	return "", "missing allele call"
}

// Germlines uses only rows assigned by the saved family result. A changed table
// cannot silently satisfy a different row's identity. Family evidence is never
// borrowed across subjects or across heavy/light loci.
func Germlines(ctx context.Context, t *rep.Table, sources []FamilySource, families FamilyResult, ref GermlineReference) (GermlineResult, error) {
	out := GermlineResult{Families: []GermlineFamily{}, Rows: []GermlineRow{}, Excluded: map[string]int{}, Description: "D-masked reference reconstruction within each saved family and locus. Explicit V/J allele calls must have one common allele. AIRR CIGARs project ungapped reference segments onto the oriented query; all intervening D/N/P bases are N. Indels remain gaps. This is not an inferred genotype, unmutated junction or lineage ancestor; no mutation rate is computed."}
	if families.Options.Threshold == nil {
		return out, errors.New("choose an explicit family threshold before reconstructing germlines")
	}
	indices := map[string]int{}
	for row := 0; row < t.Len(); row++ {
		if row%1024 == 0 {
			if err := ctx.Err(); err != nil {
				return out, err
			}
		}
		if int(t.Source[row]) >= len(sources) {
			return out, errors.New("source index outside dataset")
		}
		id := familyID("bcr-row-", []any{sources[t.Source[row]], t.Line[row], t.IDs.Get(t.SequenceID[row])})
		if _, exists := indices[id]; exists {
			return out, errors.New("duplicate source row identity")
		}
		indices[id] = row
	}
	groups := map[string][]int{}
	seen := map[string]bool{}
	for _, o := range families.Observations {
		if o.Family == "" {
			continue
		}
		for _, row := range o.Rows {
			i, ok := indices[row.ID]
			if !ok || row.Locus != t.LocusName(i) || seen[row.ID] {
				return out, errors.New("saved family rows do not match the open dataset")
			}
			seen[row.ID] = true
			out.Rows = append(out.Rows, GermlineRow{FamilyRow: row, Family: o.Family, Observation: o.ID, Sample: o.Sample})
			key := o.Family + "\x00" + row.Locus
			groups[key] = append(groups[key], len(out.Rows)-1)
		}
	}
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	bases := 0
	for _, key := range keys {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		members := groups[key]
		rows := make([]int, len(members))
		for i, m := range members {
			rows[i] = indices[out.Rows[m].ID]
		}
		f := GermlineFamily{Family: out.Rows[members[0]].Family, Locus: out.Rows[members[0]].Locus, Rows: len(members)}
		f.V, f.Excluded = alleleIntersection(t, rows, genes.V, ref)
		if f.Excluded == "" {
			f.J, f.Excluded = alleleIntersection(t, rows, genes.J, ref)
		}
		for _, m := range members {
			if err := ctx.Err(); err != nil {
				return out, err
			}
			r := &out.Rows[m]
			r.V, r.J, r.Excluded = f.V, f.J, f.Excluded
			if r.Excluded == "" {
				r.Excluded = projectGermline(t.EvidenceRow(indices[r.ID]), ref[f.V], ref[f.J], r)
			}
			if r.Excluded != "" {
				out.Excluded[r.Excluded]++
			} else {
				f.Reconstructed++
				out.Reconstructed++
				bases += len(r.Query)
				if bases > maxGermlineBases {
					return out, errors.New("germline output exceeds 50 million alignment columns; restrict the family dataset")
				}
			}
		}
		out.Families = append(out.Families, f)
	}
	sort.Slice(out.Rows, func(i, j int) bool {
		a, b := out.Rows[i], out.Rows[j]
		if a.Family != b.Family {
			return a.Family < b.Family
		}
		if a.Locus != b.Locus {
			return a.Locus < b.Locus
		}
		return a.ID < b.ID
	})
	return out, nil
}

func projectGermline(raw [rep.EvidenceCount]string, v, j string, out *GermlineRow) string {
	get := func(name string) string { return raw[fields[name]] }
	query := strings.ToUpper(get("sequence"))
	if len(query) == 0 || len(query) > maxGermlineLength || strings.Trim(query, "ACGTRYSWKMBDHVN") != "" {
		return "requires an ungapped IUPAC query of 1–10000 bases"
	}
	reverse, missing, ok := airr.ParseBool(get("rev_comp"))
	if !ok || missing {
		return "missing or invalid query orientation"
	}
	if reverse {
		b := []byte(query)
		const from, to = "ACGTRYSWKMBDHVN", "TGCAYRSWMKVHDBN"
		for i := range b {
			b[i] = to[strings.IndexByte(from, query[len(query)-1-i])]
		}
		query = string(b)
	}
	cigars := make([]Cigar, 2)
	for i, part := range []string{"v", "j"} {
		cigar := get(part + "_cigar")
		if cigar == "" {
			return "missing V/J CIGAR; export AIRR alignment evidence"
		}
		c, err := ParseCigar(cigar)
		if err != nil {
			return "invalid V/J CIGAR"
		}
		reference := v
		if i == 1 {
			reference = j
		}
		if c.QueryStart+c.QueryLength > int64(len(query)) || c.QueryTotal > int64(len(query)) || c.ReferenceStart+c.ReferenceLength > int64(len(reference)) || c.QueryLength+c.ReferenceLength > 2*maxGermlineLength {
			return "CIGAR exceeds query or selected reference"
		}
		for _, bound := range []struct {
			name string
			want int64
		}{{"sequence_start", c.QueryStart + 1}, {"sequence_end", c.QueryStart + c.QueryLength}, {"germline_start", c.ReferenceStart + 1}, {"germline_end", c.ReferenceStart + c.ReferenceLength}} {
			if text := get(part + "_" + bound.name); text != "" {
				n, err := integer(text)
				if err != nil || n != bound.want {
					return "coordinates disagree with ungapped-reference CIGAR"
				}
			}
		}
		cigars[i] = c
	}
	if cigars[0].QueryStart+cigars[0].QueryLength > cigars[1].QueryStart {
		return "V and J query spans overlap or are reversed"
	}
	var q, g, regions strings.Builder
	var qp, rp []int
	emit := func(a, b, region byte, x, y int) {
		q.WriteByte(a)
		g.WriteByte(b)
		regions.WriteByte(region)
		qp = append(qp, x)
		rp = append(rp, y)
	}
	for i, part := range []string{"v", "j"} {
		if i == 1 {
			for p := cigars[0].QueryStart + cigars[0].QueryLength; p < cigars[1].QueryStart; p++ {
				emit(query[p], 'N', 'N', int(p)+1, 0)
			}
		}
		c := cigars[i]
		x, y := int(c.QueryStart), int(c.ReferenceStart)
		reference, region := v, byte('V')
		if i == 1 {
			reference, region = j, 'J'
		}
		n := 0
		for _, op := range get(part + "_cigar") {
			if op >= '0' && op <= '9' {
				n = n*10 + int(op-'0')
				continue
			}
			if op != 'S' && op != 'N' {
				for range n {
					switch op {
					case 'I':
						emit(query[x], '-', region, x+1, 0)
						x++
					case 'D':
						emit('-', reference[y], region, 0, y+1)
						y++
					default:
						// =/X are checked against unambiguous bases only. M carries no assertion.
						if strings.ContainsRune("ACGT", rune(query[x])) && strings.ContainsRune("ACGT", rune(reference[y])) && ((op == '=' && query[x] != reference[y]) || (op == 'X' && query[x] == reference[y])) {
							return "CIGAR match assertion disagrees with selected reference"
						}
						emit(query[x], reference[y], region, x+1, y+1)
						x++
						y++
					}
				}
			}
			n = 0
		}
	}
	out.Query, out.Germline, out.Regions = q.String(), g.String(), regions.String()
	out.QueryPositions, out.ReferencePositions = qp, rp
	return ""
}
