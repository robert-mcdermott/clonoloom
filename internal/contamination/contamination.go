// Package contamination reports descriptive cross-subject nucleotide sharing.
// Scientific motivation: AIRR Community guide (2022), doi:10.1007/978-1-0716-2115-8_15;
// Li et al. (2023), doi:10.1093/bib/bbad230. This is not TCR-Broom or a calibrated
// contamination classifier. All thresholds and choices are documented in research §5.37.
package contamination

import (
	"context"
	"crypto/sha256"
	"fmt"
	"math"
	"sort"
	"strings"

	"clonoloom/internal/diversity"
	"clonoloom/internal/genes"
	"clonoloom/internal/seq"
)

const Version = 1
const Citation = "AIRR Community. Guide to Planning and Performing AIRR-Seq Experiments. 2022. doi:10.1007/978-1-0716-2115-8_15. Li R et al. A novel statistical method for decontaminating T-cell receptor sequencing data. Brief Bioinform. 2023;24:bbad230. doi:10.1093/bib/bbad230. Sethna Z et al. OLGA. Bioinformatics. 2019;35:2974–2981. doi:10.1093/bioinformatics/btz035."
const Description = "Descriptive contamination evidence between declared unrelated human subjects in bulk TRB repertoires: exact full nucleotide junction plus resolved V/J genes under the recorded QC and productive-only policy. Every positive exact cross-subject match is retained; repeated samples of one subject are not cross-subject evidence. Fractions use each sample's complete retained productive TRB abundance, including unevaluable sequence/gene rows. Review thresholds are user-declared heuristics, not p-values, contamination probabilities or validated cutoffs. Unequal depth and incomplete sequence/gene evidence affect detection. High nucleotide generation probability under a compatible saved model is a separate plausible-publicness explanation, not proof of independent generation or a clean sample. Missing Pgen is unknown; model zero is not impossibility. Shared expansions, common exposure, relatedness, transplantation, cell therapy, sample mislabeling and laboratory carryover can produce sharing. Subject independence is declared, not established by distinct IDs. Batch labels are contextual; no source, direction or rate of contamination is inferred. No sequence is removed and no statistical independence, decontamination or clinical conclusion is claimed."

type Options struct {
	MinCount     float64 `json:"minCount"`
	HighFraction float64 `json:"highFraction"`
	Ratio        float64 `json:"ratio"`
	HighPgen     float64 `json:"highPgen"`
}

func (o Options) Normalize() (Options, error) {
	if o.MinCount == 0 {
		o.MinCount = 2
	}
	if o.HighFraction == 0 {
		o.HighFraction = .01
	}
	if o.Ratio == 0 {
		o.Ratio = 20
	}
	if o.HighPgen == 0 {
		o.HighPgen = 1e-8
	}
	for _, x := range []float64{o.MinCount, o.HighFraction, o.Ratio, o.HighPgen} {
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return o, fmt.Errorf("finite thresholds required")
		}
	}
	if o.MinCount < 1 || o.MinCount > 1e9 || o.HighFraction <= 0 || o.HighFraction > 1 || o.Ratio < 1 || o.Ratio > 1e9 || o.HighPgen <= 0 || o.HighPgen > 1 {
		return o, fmt.Errorf("invalid count, fraction, ratio or Pgen threshold")
	}
	return o, nil
}

type Sample struct {
	Name    string `json:"name"`
	Subject string `json:"subject"`
	Batch   string `json:"batch"`
}
type Depth struct {
	Sample   Sample         `json:"sample"`
	Total    float64        `json:"total"`
	Eligible float64        `json:"eligible"`
	Rows     int            `json:"rows"`
	Excluded map[string]int `json:"excluded"`
}
type Receptor struct {
	ID     string           `json:"id"`
	V      string           `json:"v"`
	J      string           `json:"j"`
	NT     string           `json:"nt"`
	AA     string           `json:"aa"`
	Counts []float64        `json:"counts"`
	Pgen   diversity.Number `json:"pgen"`
}
type Input struct {
	Options   Options    `json:"options"`
	Samples   []Depth    `json:"samples"`
	Receptors []Receptor `json:"receptors"`
}
type Match struct {
	Receptor  string  `json:"receptor"`
	A         int     `json:"a"`
	B         int     `json:"b"`
	CountA    float64 `json:"countA"`
	CountB    float64 `json:"countB"`
	FractionA float64 `json:"fractionA"`
	FractionB float64 `json:"fractionB"`
	Ratio     float64 `json:"ratio"`
	Review    bool    `json:"review"`
	Pattern   string  `json:"pattern"`
	PgenClass string  `json:"pgenClass"`
}
type Pair struct {
	A           int              `json:"a"`
	B           int              `json:"b"`
	Shared      int              `json:"shared"`
	Review      int              `json:"review"`
	HighPgen    int              `json:"highPgen"`
	UnknownPgen int              `json:"unknownPgen"`
	SharedA     float64          `json:"sharedA"`
	SharedB     float64          `json:"sharedB"`
	FractionA   diversity.Number `json:"fractionA"`
	FractionB   diversity.Number `json:"fractionB"`
	Reason      string           `json:"reason"`
}
type Result struct {
	Description      string  `json:"description"`
	Evidence         Input   `json:"evidence"`
	Pairs            []Pair  `json:"pairs"`
	Matches          []Match `json:"matches"`
	SameSubjectPairs int     `json:"sameSubjectPairs"`
}

func ID(v, j, nt string) string {
	return fmt.Sprintf("nt-%x", sha256.Sum256([]byte(v+"\n"+j+"\n"+nt)))[:27]
}
func Valid(v, j, nt string) bool {
	if len(nt) < 27 || len(nt) > 120 || len(nt)%3 != 0 || strings.Trim(nt, "ACGT") != "" {
		return false
	}
	for _, g := range []struct{ call, prefix string }{{v, "TRBV"}, {j, "TRBJ"}} {
		c := genes.ParseCall(g.call)
		if !c.Resolved(genes.LevelGene) || c.Key(genes.LevelGene) != g.call || !strings.HasPrefix(g.call, g.prefix) {
			return false
		}
	}
	aa := seq.Translate(nt)
	return aa[0] == 'C' && (aa[len(aa)-1] == 'F' || aa[len(aa)-1] == 'W') && !strings.ContainsAny(aa, "*X")
}
func Compute(ctx context.Context, in Input) (*Result, error) {
	o, e := in.Options.Normalize()
	if e != nil {
		return nil, e
	}
	in.Options = o
	if len(in.Samples) < 2 || len(in.Samples) > 64 || len(in.Receptors) > 5000 {
		return nil, fmt.Errorf("choose 2–64 samples and at most 5000 eligible nucleotide identities")
	}
	names, subjects := map[string]bool{}, map[string]bool{}
	totals := make([]float64, len(in.Samples))
	for _, s := range in.Samples {
		if s.Sample.Name == "" || names[s.Sample.Name] || strings.TrimSpace(s.Sample.Subject) == "" || len(s.Sample.Subject) > 256 || len(s.Sample.Name) > 256 || len(s.Sample.Batch) > 256 {
			return nil, fmt.Errorf("distinct samples and explicit subject IDs required")
		}
		names[s.Sample.Name] = true
		subjects[s.Sample.Subject] = true
		if s.Total < 0 || math.Trunc(s.Total) != s.Total || s.Total > 9e15 || math.IsNaN(s.Total) || math.IsInf(s.Total, 0) || s.Eligible < 0 || s.Eligible > s.Total {
			return nil, fmt.Errorf("invalid captured abundance")
		}
	}
	if len(subjects) < 2 {
		return nil, fmt.Errorf("select at least two declared unrelated subjects")
	}
	seen := map[string]bool{}
	for _, rec := range in.Receptors {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if !Valid(rec.V, rec.J, rec.NT) || rec.ID != ID(rec.V, rec.J, rec.NT) || seen[rec.ID] || rec.AA != seq.Translate(rec.NT) || len(rec.Counts) != len(totals) {
			return nil, fmt.Errorf("invalid captured nucleotide identity")
		}
		seen[rec.ID] = true
		p := float64(rec.Pgen)
		if !math.IsNaN(p) && (p < 0 || p > 1 || math.IsInf(p, 0)) {
			return nil, fmt.Errorf("invalid captured Pgen")
		}
		for i, c := range rec.Counts {
			if c < 0 || math.Trunc(c) != c || math.IsNaN(c) || math.IsInf(c, 0) {
				return nil, fmt.Errorf("invalid captured count")
			}
			totals[i] += c
		}
	}
	for i, s := range in.Samples {
		if totals[i] != s.Eligible {
			return nil, fmt.Errorf("eligible abundance differs from captured records")
		}
	}
	r := &Result{Description: Description, Evidence: in, Pairs: []Pair{}, Matches: []Match{}}
	for a := range in.Samples {
		for b := a + 1; b < len(in.Samples); b++ {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			sa, sb := in.Samples[a], in.Samples[b]
			if sa.Sample.Subject == sb.Sample.Subject {
				r.SameSubjectPairs++
				continue
			}
			p := Pair{A: a, B: b, FractionA: diversity.Number(math.NaN()), FractionB: diversity.Number(math.NaN())}
			if sa.Total == 0 || sb.Total == 0 {
				p.Reason = "No retained productive TRB abundance in at least one sample"
			} else if sa.Eligible == 0 || sb.Eligible == 0 {
				p.Reason = "No eligible nucleotide identities in at least one sample"
			}
			for _, rec := range in.Receptors {
				ca, cb := rec.Counts[a], rec.Counts[b]
				if ca <= 0 || cb <= 0 {
					continue
				}
				if len(r.Matches) >= 200000 {
					return nil, fmt.Errorf("more than 200000 shared pair/identity records; select fewer samples")
				}
				fa, fb := ca/sa.Total, cb/sb.Total
				ratio := math.Max(fa, fb) / math.Min(fa, fb)
				if math.IsInf(ratio, 0) {
					return nil, fmt.Errorf("frequency ratio exceeds numerical range")
				}
				m := Match{Receptor: rec.ID, A: a, B: b, CountA: ca, CountB: cb, FractionA: fa, FractionB: fb, Ratio: ratio, PgenClass: "Unknown Pgen", Pattern: "Below review thresholds"}
				if !math.IsNaN(float64(rec.Pgen)) {
					m.PgenClass = "Below declared high-Pgen threshold"
					if float64(rec.Pgen) >= o.HighPgen {
						m.PgenClass = "High Pgen: plausible publicness"
						p.HighPgen++
					}
				} else {
					p.UnknownPgen++
				}
				m.Review = ca >= o.MinCount && cb >= o.MinCount && math.Max(fa, fb) >= o.HighFraction
				if m.Review {
					p.Review++
					m.Pattern = "Shared high abundance"
					if ratio >= o.Ratio {
						m.Pattern = "Asymmetric observed fractions"
					}
				}
				p.Shared++
				p.SharedA += ca
				p.SharedB += cb
				r.Matches = append(r.Matches, m)
			}
			if p.Reason == "" {
				p.FractionA = diversity.Number(p.SharedA / sa.Total)
				p.FractionB = diversity.Number(p.SharedB / sb.Total)
			}
			r.Pairs = append(r.Pairs, p)
		}
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return r, nil
}

// Sort establishes reproducible summation order before capture totals are computed.
func Sort(records []Receptor) {
	sort.Slice(records, func(i, j int) bool { return records[i].ID < records[j].ID })
}
