package bcell

// Observed somatic divergence follows the sequence/reference comparison framework
// of Gupta et al. 2015, doi:10.1093/bioinformatics/btv359. Coordinates follow AIRR
// Rearrangement schema 2.0; translation uses NCBI table 1. Conservative evaluability,
// CDR3-anchored phase and single-hit R/S attribution are ClonoLoom policies (§5.18).
import (
	"context"
	"errors"
	"sort"
	"strings"

	"clonoloom/internal/airr"
	"clonoloom/internal/rep"
	"clonoloom/internal/seq"
)

const SHMVersion = 1
const SHMDescription = "Observed nucleotide differences from the saved D-masked V/J reference, per source chain row; not historical mutation events or a mutation rate. Only paired A/C/G/T bases enter the denominator. Masks, ambiguities and gaps are separate. Regions use supplied oriented-query coordinates. Codons require checked CDR3/junction phase, vj_in_frame=true and an indel-free projection; complete non-stop codons within one segment are compared using NCBI table 1. Replacement/silent substitutions are attributed only in single-hit codons; multiple-hit codons have endpoint amino-acid classifications but their substitutions remain unclassified. Sample/locus summaries pool counts across source rows, without abundance weighting, deduplication or inferential statistics."

var shmRegions = []string{"fwr1", "cdr1", "fwr2", "cdr2", "fwr3", "cdr3", "fwr4"}

type SHMCounts struct {
	EvaluableBases    int      `json:"evaluableBases"`
	Mutations         int      `json:"mutations"`
	Frequency         *float64 `json:"frequency"`
	Masked            int      `json:"masked"`
	Ambiguous         int      `json:"ambiguous"`
	Gaps              int      `json:"gaps"`
	EvaluableCodons   int      `json:"evaluableCodons"`
	Replacement       int      `json:"replacement"`
	Silent            int      `json:"silent"`
	Unclassified      int      `json:"unclassified"`
	MultiHitCodons    int      `json:"multiHitCodons"`
	ReplacementCodons int      `json:"replacementCodons"`
	SilentCodons      int      `json:"silentCodons"`
	StopCodons        int      `json:"stopCodons"`
}
type SHMRegion struct {
	Region      string `json:"region"`
	Unavailable string `json:"unavailable,omitempty"`
	SHMCounts
}
type SHMRow struct {
	FamilyRow
	Family         string      `json:"family"`
	Observation    string      `json:"observation"`
	Sample         string      `json:"sample"`
	V              string      `json:"v"`
	J              string      `json:"j"`
	Excluded       string      `json:"excluded,omitempty"`
	CodonExclusion string      `json:"codonExclusion,omitempty"`
	FrameAnchor    int         `json:"frameAnchor,omitempty"` // 1-based CDR3 start, not reference phase.
	Regions        []SHMRegion `json:"regions"`
}
type SHMSummary struct {
	Sample        string `json:"sample"`
	Locus         string `json:"locus"`
	Region        string `json:"region"`
	Rows          int    `json:"rows"`
	AvailableRows int    `json:"availableRows"`
	CodonRows     int    `json:"codonRows"`
	SHMCounts
}
type SHMResult struct {
	Rows            []SHMRow       `json:"rows"`
	Summaries       []SHMSummary   `json:"summaries"`
	Evaluated       int            `json:"evaluated"`
	Excluded        map[string]int `json:"excluded"`
	CodonExclusions map[string]int `json:"codonExclusions"`
	Description     string         `json:"description"`
}

func (c *SHMCounts) finish() {
	c.Unclassified = c.Mutations - c.Replacement - c.Silent
	c.Frequency = nil
	if c.EvaluableBases > 0 {
		f := float64(c.Mutations) / float64(c.EvaluableBases)
		c.Frequency = &f
	}
}
func (c *SHMCounts) add(x SHMCounts) {
	c.EvaluableBases += x.EvaluableBases
	c.Mutations += x.Mutations
	c.Masked += x.Masked
	c.Ambiguous += x.Ambiguous
	c.Gaps += x.Gaps
	c.EvaluableCodons += x.EvaluableCodons
	c.Replacement += x.Replacement
	c.Silent += x.Silent
	c.MultiHitCodons += x.MultiHitCodons
	c.ReplacementCodons += x.ReplacementCodons
	c.SilentCodons += x.SilentCodons
	c.StopCodons += x.StopCodons
	c.finish()
}

// SHM preserves each source row, including excluded parents and duplicate contigs.
// It never treats chain rows as independent biological replicates or weights by reads.
func SHM(ctx context.Context, t *rep.Table, sources []FamilySource, parent GermlineResult) (SHMResult, error) {
	out := SHMResult{Rows: []SHMRow{}, Summaries: []SHMSummary{}, Excluded: map[string]int{}, CodonExclusions: map[string]int{}, Description: SHMDescription}
	indices := map[string]int{}
	for i := 0; i < t.Len(); i++ {
		if i%1024 == 0 {
			if err := ctx.Err(); err != nil {
				return out, err
			}
		}
		if int(t.Source[i]) >= len(sources) {
			return out, errors.New("source index outside dataset")
		}
		id := familyID("bcr-row-", []any{sources[t.Source[i]], t.Line[i], t.IDs.Get(t.SequenceID[i])})
		if _, exists := indices[id]; exists {
			return out, errors.New("duplicate source row identity")
		}
		indices[id] = i
	}
	seen := map[string]bool{}
	sums := map[string]*SHMSummary{}
	for _, g := range parent.Rows {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		i, ok := indices[g.ID]
		if !ok || seen[g.ID] || g.Locus != t.LocusName(i) || g.Source != sources[t.Source[i]] || g.Line != t.Line[i] || g.SequenceID != t.IDs.Get(t.SequenceID[i]) || g.Sample != t.Samples[t.Sample[i]].Name {
			return out, errors.New("saved germline rows do not match the open dataset")
		}
		seen[g.ID] = true
		r := measureSHM(g, t.EvidenceRow(i), t.Junctions.Get(t.Junction[i]), t.VJInFrame[i])
		if r.Excluded != "" {
			out.Excluded[r.Excluded]++
		} else {
			if r.Regions[0].EvaluableBases > 0 {
				out.Evaluated++
			}
			if r.CodonExclusion != "" {
				out.CodonExclusions[r.CodonExclusion]++
			}
		}
		for _, region := range r.Regions {
			key := r.Sample + "\x00" + r.Locus + "\x00" + region.Region
			s := sums[key]
			if s == nil {
				s = &SHMSummary{Sample: r.Sample, Locus: r.Locus, Region: region.Region}
				sums[key] = s
			}
			s.Rows++
			if region.Unavailable == "" {
				s.AvailableRows++
				if region.EvaluableCodons > 0 {
					s.CodonRows++
				}
				s.add(region.SHMCounts)
			}
		}
		out.Rows = append(out.Rows, r)
	}
	keys := make([]string, 0, len(sums))
	for key := range sums {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		out.Summaries = append(out.Summaries, *sums[key])
	}
	sort.Slice(out.Rows, func(i, j int) bool { return out.Rows[i].ID < out.Rows[j].ID })
	return out, nil
}

func measureSHM(g GermlineRow, raw [rep.EvidenceCount]string, junction string, inFrame rep.Flag) SHMRow {
	r := SHMRow{FamilyRow: g.FamilyRow, Family: g.Family, Observation: g.Observation, Sample: g.Sample, V: g.V, J: g.J, Excluded: g.Excluded, Regions: []SHMRegion{}}
	for _, name := range append([]string{"all", "V", "J"}, shmRegions...) {
		r.Regions = append(r.Regions, SHMRegion{Region: name})
	}
	if r.Excluded == "" && (len(g.Query) == 0 || len(g.Query) != len(g.Germline) || len(g.Query) != len(g.Regions) || len(g.Query) != len(g.QueryPositions) || len(g.Query) != len(g.ReferencePositions)) {
		r.Excluded = "invalid saved alignment or coordinate maps"
	}
	if r.Excluded != "" {
		for i := range r.Regions {
			r.Regions[i].Unavailable = r.Excluded
		}
		return r
	}
	query := strings.ToUpper(raw[fields["sequence"]])
	reverse, missing, valid := airr.ParseBool(raw[fields["rev_comp"]])
	if reverse {
		query = seq.ReverseComplement(query)
	}
	if missing || !valid || len(query) > maxGermlineLength {
		r.Excluded = "invalid source sequence or orientation"
	}
	last := 0
	for c, p := range g.QueryPositions {
		if p < 0 || p > len(query) || (p == 0 && g.Query[c] != '-') || (p > 0 && (p <= last || query[p-1] != g.Query[c])) || !strings.ContainsRune("VJN", rune(g.Regions[c])) {
			r.Excluded = "invalid saved query coordinate map"
			break
		}
		if p > 0 {
			last = p
		}
	}
	if r.Excluded != "" {
		for i := range r.Regions {
			r.Regions[i].Unavailable = r.Excluded
		}
		return r
	}
	// Missing individual regions remain missing. Any malformed/overlapping/out-of-order
	// interval makes the supplied regional partition unusable; totals still stand.
	bounds := make([][2]int, len(shmRegions))
	previous := 0
	badRegions := false
	for k, name := range shmRegions {
		start, end := raw[fields[name+"_start"]], raw[fields[name+"_end"]]
		if start == "" && end == "" {
			r.Regions[k+3].Unavailable = "missing region coordinates"
			continue
		}
		a, e1 := integer(start)
		b, e2 := integer(end)
		if e1 != nil || e2 != nil || a < 1 || b < a || b > int64(len(query)) || a <= int64(previous) {
			badRegions = true
			continue
		}
		bounds[k] = [2]int{int(a), int(b)}
		previous = int(b)
	}
	if badRegions {
		for k := range bounds {
			bounds[k] = [2]int{}
			r.Regions[k+3].Unavailable = "invalid or overlapping region coordinates"
		}
	}
	targets := func(c int) []int {
		list := []int{0}
		if g.Regions[c] == 'V' {
			list = append(list, 1)
		} else if g.Regions[c] == 'J' {
			list = append(list, 2)
		}
		p := g.QueryPositions[c]
		for k, b := range bounds {
			if p > 0 && b[0] > 0 && p >= b[0] && p <= b[1] {
				list = append(list, k+3)
				break
			}
		}
		return list
	}
	for c := range g.Query {
		for _, k := range targets(c) {
			n := &r.Regions[k].SHMCounts
			switch {
			case g.Regions[c] == 'N':
				n.Masked++
			case g.Query[c] == '-' || g.Germline[c] == '-':
				n.Gaps++
			case !canonical(g.Query[c]) || !canonical(g.Germline[c]):
				n.Ambiguous++
			default:
				n.EvaluableBases++
				if g.Query[c] != g.Germline[c] {
					n.Mutations++
				}
			}
		}
	}
	// Validate the frame anchor against the explicit complete junction; no searching,
	// germline-start phase assumption, or padding of truncated codons.
	cdr := bounds[5]
	switch {
	case inFrame != rep.True:
		r.CodonExclusion = "requires vj_in_frame=true"
	case cdr[0] < 4 || cdr[1]+3 > len(query) || (cdr[1]-cdr[0]+1)%3 != 0:
		r.CodonExclusion = "requires complete valid CDR3 coordinates"
	case query[cdr[0]-4:cdr[1]+3] != strings.ToUpper(junction):
		r.CodonExclusion = "CDR3 coordinates disagree with the supplied junction"
	default:
		aa := seq.Translate(strings.ToUpper(junction))
		c, w := seq.Anchors(aa)
		if !c || !w || strings.ContainsAny(aa, "*X") {
			r.CodonExclusion = "requires an unambiguous anchored junction"
		}
	}
	if r.CodonExclusion == "" && strings.ContainsAny(g.Query+g.Germline, "-") {
		r.CodonExclusion = "indel-containing projection; codon phase not inferred"
	}
	if r.CodonExclusion == "" {
		r.FrameAnchor = cdr[0]
		for c := 0; c+2 < len(g.Query); c++ {
			p := g.QueryPositions[c]
			if p == 0 || (p-r.FrameAnchor)%3 != 0 || g.Regions[c] == 'N' || g.Regions[c] != g.Regions[c+1] || g.Regions[c] != g.Regions[c+2] || g.QueryPositions[c+1] != p+1 || g.QueryPositions[c+2] != p+2 || g.ReferencePositions[c] < 1 || g.ReferencePositions[c+1] != g.ReferencePositions[c]+1 || g.ReferencePositions[c+2] != g.ReferencePositions[c]+2 {
				continue
			}
			q, ref := g.Query[c:c+3], g.Germline[c:c+3]
			if strings.Trim(q+ref, "ACGT") != "" {
				continue
			}
			qa, ga := seq.Translate(q), seq.Translate(ref)
			hits := 0
			for i := 0; i < 3; i++ {
				if q[i] != ref[i] {
					hits++
				}
			}
			for _, k := range targets(c) {
				if k >= 3 && p+2 > bounds[k-3][1] {
					continue
				} // partial regional codon
				n := &r.Regions[k].SHMCounts
				if qa == "*" || ga == "*" {
					n.StopCodons++
					continue
				}
				n.EvaluableCodons++
				if hits == 0 {
					continue
				}
				if qa == ga {
					n.SilentCodons++
				} else {
					n.ReplacementCodons++
				}
				if hits > 1 {
					n.MultiHitCodons++
				} else if qa == ga {
					n.Silent++
				} else {
					n.Replacement++
				}
			}
		}
	}
	for i := range r.Regions {
		r.Regions[i].finish()
	}
	return r
}
func canonical(b byte) bool { return b == 'A' || b == 'C' || b == 'G' || b == 'T' }
