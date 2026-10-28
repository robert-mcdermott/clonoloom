// Package robustness audits subject-feature comparisons across declared definitions.
// Descriptive sensitivity, informed by Simonsohn, Simmons & Nelson (2020),
// doi:10.1038/s41562-020-0912-z. This is not their joint specification-curve test.
// Existing diversity/cohort engines supply all estimates and marginal intervals.
// Across-audit correction: Benjamini & Yekutieli (2001), doi:10.1214/aos/1013699998.
// Definition restrictions, descriptive verdicts and bounds are ClonoLoom choices;
// research.md §5.34 records the complete contract.
package robustness

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"clonoloom/internal/clonotype"
	"clonoloom/internal/cohort"
	"clonoloom/internal/diversity"
	"clonoloom/internal/modularity"
	"clonoloom/internal/study"
)

const Version = 1
const Citation = "Simonsohn U, Simmons JP, Nelson LD. Specification curve analysis. Nature Human Behaviour. 2020;4:1208–1214. doi:10.1038/s41562-020-0912-z. Descriptive inspiration only; no joint specification-curve test. Benjamini Y, Yekutieli D. Ann Stat. 2001;29:1165–1188. doi:10.1214/aos/1013699998. " + study.FeatureCitation + " " + cohort.Citation
const Description = "Descriptive sensitivity of the same subject-feature contrasts to declared clonotype definitions. Only junction and V/J resolution vary; subjects, exclusions, chain/cell/productivity rules, QC, features, tests, confidence and randomization settings remain fixed. Recompute observed-depth features from captured abundances, without pooling, depth normalization or within-sample bootstrap intervals. Definition changes can alter assigned evidence and the biological meaning of diversity; inspect each sample's abundance and richness. Report effect direction, magnitude range and marginal interval behavior separately; no robustness verdict uses p-value thresholds, and agreement does not establish biological truth, practical equivalence or a joint confidence statement. Definitions are correlated analyses of the same subjects, not independent replications. Within-definition BH remains as in the parent; additional BY covers both tests for every feature and declared definition, including failed slots as p=1 internally and unavailable in output. BY requires valid input p-values and does not correct outcome-driven specification selection or unknown analyses. No meta-analysis, pooled effect or joint specification-curve test. This audit covers aggregate subject features, not correspondence of individual clonotypes across definitions."

type Sample struct {
	Name       string    `json:"name"`
	Abundances []float64 `json:"abundances"`
}
type Input struct {
	Definition clonotype.Definition `json:"definition"`
	Samples    []Sample             `json:"samples"`
	Excluded   map[string]int       `json:"excluded"`
	Failure    string               `json:"failure"`
}
type Variant struct {
	Input
	Table    *study.FeatureTable `json:"table,omitempty"`
	Analysis *cohort.Result      `json:"analysis,omitempty"`
	Across   []Adjusted          `json:"across"`
}
type Adjusted struct {
	Feature string           `json:"feature"`
	Mean    diversity.Number `json:"mean"`
	Rank    diversity.Number `json:"rank"`
}
type Summary struct {
	Feature     study.Feature    `json:"feature"`
	Definitions int              `json:"definitions"`
	Available   int              `json:"available"`
	Intervals   int              `json:"intervals"`
	Minimum     diversity.Number `json:"minimum"`
	Maximum     diversity.Number `json:"maximum"`
	Span        diversity.Number `json:"span"`
	Direction   string           `json:"direction"`
	Uncertainty string           `json:"uncertainty"`
}
type Result struct {
	Description string         `json:"description"`
	Options     cohort.Options `json:"options"`
	Family      int            `json:"family"`
	Variants    []Variant      `json:"variants"`
	Summaries   []Summary      `json:"summaries"`
}

func missing() diversity.Number      { return diversity.Number(math.NaN()) }
func finite(v diversity.Number) bool { return !math.IsNaN(float64(v)) && !math.IsInf(float64(v), 0) }

// Definitions preserves declared order. Equivalent definitions are refused, rather
// than counted as independent supporting analyses; names have no scientific identity.
func Definitions(base clonotype.Definition, alternatives []clonotype.Definition) ([]clonotype.Definition, error) {
	if len(alternatives) < 1 || len(alternatives) > 5 {
		return nil, errors.New("choose one to five distinct alternative definitions")
	}
	b, err := base.Normalize()
	if err != nil || b.Vendor {
		return nil, errors.New("choose a sequence-defined baseline")
	}
	out := []clonotype.Definition{b}
	seen := map[string]bool{b.Canonical(): true}
	for _, raw := range alternatives {
		d, err := raw.Normalize()
		if err != nil {
			return nil, err
		}
		if d.Vendor || d.ProductiveOnly != b.ProductiveOnly || d.Cells != b.Cells || d.RequireAll != b.RequireAll || strings.Join(d.Chains, ",") != strings.Join(b.Chains, ",") {
			return nil, errors.New("vary only junction and V/J resolution; retain baseline chain, cell and productivity rules")
		}
		if seen[d.Canonical()] {
			return nil, errors.New("definitions must differ in meaning, including from the baseline")
		}
		seen[d.Canonical()] = true
		d.Name = strings.TrimSpace(d.Name)
		if len(d.Name) > 160 {
			return nil, errors.New("definition name exceeds 160 characters")
		}
		if d.Name == "" {
			d.Name = d.Describe()
		}
		out = append(out, d)
	}
	return out, nil
}

// Capture keeps positive assigned clonotype abundances, in source sample order.
// They are sufficient for aggregate diversity, not for identifying clones across definitions.
func Capture(t *clonotype.Table, names []string) (Input, error) {
	in := Input{Definition: t.Definition, Excluded: t.Excluded, Samples: []Sample{}}
	if len(names) > 256 || len(names) != len(t.SampleTotals) {
		return in, errors.New("definition audit supports at most 256 source samples with exact totals")
	}
	if t.Excluded[clonotype.ReasonNoCount] > 0 {
		return in, errors.New("retained rearrangements have missing abundance; no missing count is treated as zero")
	}
	for _, n := range names {
		in.Samples = append(in.Samples, Sample{Name: n, Abundances: []float64{}})
	}
	entries := 0
	for i := range t.Clonotypes {
		samples, values := t.Abundances(i)
		for j, s := range samples {
			y := values[j]
			if math.IsNaN(y) || math.IsInf(y, 0) || y < 0 {
				return in, errors.New("invalid assigned abundance")
			}
			if y > 0 {
				in.Samples[s].Abundances = append(in.Samples[s].Abundances, y)
				entries++
			}
			if entries > 400000 {
				return in, errors.New("a definition exceeds 400000 observed sample/clonotype abundances")
			}
		}
	}
	for i := range in.Samples {
		sort.Float64s(in.Samples[i].Abundances)
	}
	return in, nil
}
func Features(ctx context.Context, design study.CohortInput, in Input, unit string) (study.FeatureTable, error) {
	if len(in.Samples) > 256 {
		return study.FeatureTable{}, errors.New("definition audit supports at most 256 source samples")
	}
	samples := map[string]diversity.Result{}
	entries := 0
	for _, s := range in.Samples {
		if err := ctx.Err(); err != nil {
			return study.FeatureTable{}, err
		}
		if _, ok := samples[s.Name]; ok {
			return study.FeatureTable{}, errors.New("duplicate captured sample")
		}
		entries += len(s.Abundances)
		if entries > 400000 {
			return study.FeatureTable{}, errors.New("captured abundance budget exceeded")
		}
		for _, x := range s.Abundances {
			if !(x > 0) || math.IsInf(x, 0) || x > 1e12 {
				return study.FeatureTable{}, errors.New("captured abundances must be positive finite values at most 1e12")
			}
		}
		// Only point estimates feed subject inference. Replicates=0 bypasses the
		// within-sample bootstrap; its intervals would answer a different question.
		samples[s.Name] = diversity.Measure(s.Abundances, diversity.Options{Replicates: 0})
	}
	return study.SubjectFeatures(design, samples, unit)
}
func Compute(ctx context.Context, design study.CohortInput, unit string, inputs []Input, options cohort.Options, baseline study.FeatureTable, progress func(float64)) (*Result, error) {
	if len(inputs) < 2 || len(inputs) > 6 {
		return nil, errors.New("audit needs baseline and one to five alternatives")
	}
	d, err := study.CheckCohort(design)
	if err != nil || !d.OK {
		return nil, errors.New("invalid saved subject design")
	}
	if d.Included > 200 {
		return nil, errors.New("audit supports at most 200 selected subject/group observations")
	}
	alternatives := []clonotype.Definition{}
	for _, in := range inputs[1:] {
		alternatives = append(alternatives, in.Definition)
	}
	if _, err = Definitions(inputs[0].Definition, alternatives); err != nil {
		return nil, err
	}
	o, err := options.Normalize()
	if err != nil {
		return nil, err
	}
	out := &Result{Description: Description, Options: o, Family: 2 * len(o.Features) * len(inputs), Variants: []Variant{}, Summaries: []Summary{}}
	ps := make([]float64, out.Family)
	for i := range ps {
		ps[i] = 1
	}
	for i, in := range inputs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if progress != nil {
			progress(float64(i) / float64(len(inputs)))
		}
		v := Variant{Input: in, Across: []Adjusted{}}
		if in.Failure == "" {
			table, e := Features(ctx, design, in, unit)
			if e != nil {
				v.Failure = e.Error()
			} else {
				if i == 0 {
					a, _ := json.Marshal(table)
					b, _ := json.Marshal(baseline)
					if string(a) != string(b) {
						return nil, errors.New("baseline source measurements differ from pinned subject features")
					}
				}
				v.Table = &table
				v.Analysis, e = cohort.Compute(ctx, design, table, o)
				if e != nil {
					v.Failure = e.Error()
					v.Analysis = nil
				}
			}
		}
		if i == 0 && v.Failure != "" {
			return nil, fmt.Errorf("baseline unavailable: %s", v.Failure)
		}
		for j, id := range o.Features {
			v.Across = append(v.Across, Adjusted{Feature: id, Mean: missing(), Rank: missing()})
			if v.Analysis != nil {
				c := v.Analysis.Comparisons[j]
				k := i*2*len(o.Features) + 2*j
				if finite(c.Parametric.P) {
					ps[k] = float64(c.Parametric.P)
				}
				if finite(c.Rank.P) {
					ps[k+1] = float64(c.Rank.P)
				}
			}
		}
		out.Variants = append(out.Variants, v)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	qs, _ := modularity.AdjustBY(ps)
	for i := range out.Variants {
		v := &out.Variants[i]
		if v.Analysis == nil {
			continue
		}
		for j, c := range v.Analysis.Comparisons {
			k := i*2*len(o.Features) + 2*j
			if finite(c.Parametric.P) {
				v.Across[j].Mean = diversity.Number(qs[k])
			}
			if finite(c.Rank.P) {
				v.Across[j].Rank = diversity.Number(qs[k+1])
			}
		}
	}
	for j := range o.Features {
		out.Summaries = append(out.Summaries, summarize(out.Variants, j))
	}
	return out, nil
}
func summarize(variants []Variant, j int) Summary {
	s := Summary{Definitions: len(variants), Minimum: missing(), Maximum: missing(), Span: missing()}
	positive, negative, zero := 0, 0, 0
	above, below, includes := 0, 0, 0
	for _, v := range variants {
		if v.Analysis == nil {
			continue
		}
		c := v.Analysis.Comparisons[j]
		s.Feature = c.Feature
		if finite(c.Mean.Estimate) {
			x := c.Mean.Estimate
			if s.Available == 0 {
				s.Minimum = x
				s.Maximum = x
			} else {
				if x < s.Minimum {
					s.Minimum = x
				}
				if x > s.Maximum {
					s.Maximum = x
				}
			}
			s.Available++
			if x > 0 {
				positive++
			} else if x < 0 {
				negative++
			} else {
				zero++
			}
		}
		if finite(c.Mean.Lower) && finite(c.Mean.Upper) {
			s.Intervals++
			if c.Mean.Lower > 0 {
				above++
			} else if c.Mean.Upper < 0 {
				below++
			} else {
				includes++
			}
		}
	}
	if s.Available > 0 {
		s.Span = s.Maximum - s.Minimum
	}
	switch {
	case s.Available < len(variants):
		s.Direction = "Incomplete: an effect is unavailable"
	case positive > 0 && negative > 0:
		s.Direction = "Effect direction changes across definitions"
	case positive == len(variants):
		s.Direction = "Positive estimates across all declared definitions"
	case negative == len(variants):
		s.Direction = "Negative estimates across all declared definitions"
	case zero == len(variants):
		s.Direction = "All estimates are zero; this does not establish equivalence"
	default:
		s.Direction = "Direction includes a zero estimate"
	}
	switch {
	case s.Intervals < len(variants):
		s.Uncertainty = "Incomplete: a marginal interval is unavailable"
	case above == len(variants):
		s.Uncertainty = "Every marginal interval is above zero"
	case below == len(variants):
		s.Uncertainty = "Every marginal interval is below zero"
	case includes == len(variants):
		s.Uncertainty = "Every marginal interval includes zero"
	default:
		s.Uncertainty = "Marginal intervals differ in their relation to zero"
	}
	return s
}
