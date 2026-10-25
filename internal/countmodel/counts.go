package countmodel

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"

	"clonoloom/internal/clonotype"
	"clonoloom/internal/diversity"
	"clonoloom/internal/dmath"
	"clonoloom/internal/expansion"
	"clonoloom/internal/rep"
	"clonoloom/internal/study"
)

const Version = 1
const Citation = "McCarthy DJ, Chen Y, Smyth GK. Differential expression analysis of multifactor RNA-Seq experiments with respect to biological variation. Nucleic Acids Res. 2012;40:4288–4297. doi:10.1093/nar/gks042. Benjamini Y, Hochberg Y. JRSS B. 1995;57:289–300. doi:10.1111/j.2517-6161.1995.tb02031.x."
const Description = "Replicated NB2 log-link model: variance = mean + dispersion × mean². Offset is log of all assigned clonotype counts in each selected sample, before feature filtering; effects are relative repertoire abundance, not absolute cell expansion. No composition normalization, shrinkage or Poisson fallback. Separate per-clonotype Cox–Reid dispersion estimates under the full design; nested-model likelihood-ratio p-values use an asymptotic chi-square with one degree of freedom at that fitted dispersion. Wald intervals are marginal and conditional on estimated dispersion; small-sample and dispersion-estimation uncertainty can affect calibration. Subject fixed effects block paired observations; this is not a random-effects model and assumes conditional independence after blocking. Optional batch adjustment requires a full-rank design. BH covers every count/prevalence-eligible clonotype, including failed fits as p=1 placeholders that remain displayed as unavailable. BH assumes independence or suitable positive dependence; no correction across other results or exploratory choices. Zero counts are observations, not missing data. Whole nonnegative counts and positive library totals are required. All-zero groups/subject blocks, dispersion boundaries and nonconvergence remain explicit failures. Do not infer antigen specificity or causality; independent biological units and an appropriate sampling model remain required."

type Options struct {
	MinCount    int     `json:"minCount"`
	MinSamples  int     `json:"minSamples"`
	AdjustBatch bool    `json:"adjustBatch"`
	Confidence  float64 `json:"confidence"`
}

func (o Options) Normalize() (Options, error) {
	if o.MinCount == 0 {
		o.MinCount = 10
	}
	if o.MinSamples == 0 {
		o.MinSamples = 12
	}
	if o.Confidence == 0 {
		o.Confidence = .95
	}
	if o.MinCount < 1 || o.MinCount > 1000000 || o.MinSamples < 2 || o.MinSamples > 96 || !(o.Confidence >= .8 && o.Confidence <= .99) {
		return o, errors.New("choose minCount 1–1000000, minSamples 2–96 and confidence 0.8–0.99")
	}
	return o, nil
}

type Counts struct {
	ID     string            `json:"id"`
	Chains []clonotype.Chain `json:"chains"`
	Values []float64         `json:"values"`
}
type Input struct {
	Samples    []string  `json:"samples"`
	Totals     []float64 `json:"totals"`
	Clonotypes []Counts  `json:"clonotypes"`
}
type Row struct {
	Counts
	CPM        []float64        `json:"cpm"`
	Eligible   bool             `json:"eligible"`
	Reason     string           `json:"reason"`
	MeanCPM    float64          `json:"meanCPM"`
	Log2Fold   diversity.Number `json:"log2Fold"`
	SE         diversity.Number `json:"se"`
	Lower      diversity.Number `json:"lower"`
	Upper      diversity.Number `json:"upper"`
	Dispersion diversity.Number `json:"dispersion"`
	LR         diversity.Number `json:"lr"`
	P          diversity.Number `json:"p"`
	Adjusted   diversity.Number `json:"adjusted"`
	Full       *Fit             `json:"full,omitempty"`
	Null       *Fit             `json:"null,omitempty"`
}
type Result struct {
	Options     Options   `json:"options"`
	Design      Matrix    `json:"design"`
	Totals      []float64 `json:"totals"`
	Offsets     []float64 `json:"offsets"`
	Family      int       `json:"family"`
	Tested      int       `json:"tested"`
	Description string    `json:"description"`
	Rows        []Row     `json:"rows"`
}

func missing() diversity.Number { return diversity.Number(math.NaN()) }

// Capture copies the observed matrix under the parent's exact definition/QC. Counts
// are aligned to canonical design rows; all assigned clones enter library denominators.
func Capture(t *clonotype.Table, names []string, m Matrix, source *rep.Table) (Input, error) {
	in := Input{}
	if t.Definition.Vendor {
		return in, errors.New("choose a sequence-defined clonotype; vendor IDs are not comparable biological identities across samples")
	}
	if t.Excluded[clonotype.ReasonNoCount] > 0 {
		return in, errors.New("source has retained rearrangements with missing counts; resolve them before count modeling")
	}
	if len(names) != len(t.SampleTotals) {
		return in, errors.New("sample totals differ from source labels")
	}
	index := map[string]uint32{}
	for i, n := range names {
		if _, ok := index[n]; ok {
			return in, errors.New("duplicate source sample")
		}
		index[n] = uint32(i)
	}
	selected := []uint32{}
	for _, r := range m.Rows {
		i, ok := index[r.Name]
		if !ok {
			return in, errors.New("design sample absent from counts")
		}
		selected = append(selected, i)
		in.Samples = append(in.Samples, r.Name)
		in.Totals = append(in.Totals, t.SampleTotals[i])
	}
	if !t.SingleCell {
		if source == nil {
			return in, errors.New("bulk count capture requires original row evidence")
		}
		chosen := map[uint32]bool{}
		for _, s := range selected {
			chosen[s] = true
		}
		for _, row := range t.Members {
			y := source.Count[row]
			if chosen[source.Sample[row]] && (!finite(y) || y < 0 || math.Trunc(y) != y) {
				return in, errors.New("source rows must have whole nonnegative counts before clonotype aggregation")
			}
		}
	}
	for i, c := range t.Clonotypes {
		values := make([]float64, len(selected))
		total := 0.0
		for j, s := range selected {
			values[j] = t.In(i, s)
			total += values[j]
		}
		if total == 0 {
			continue
		}
		in.Clonotypes = append(in.Clonotypes, Counts{ID: c.ID, Chains: c.Chains, Values: values})
		if len(in.Clonotypes) > 20000 || len(in.Clonotypes)*len(selected) > 1000000 {
			return Input{}, errors.New("count snapshot exceeds 20000 clonotypes or 1000000 matrix entries")
		}
	}
	sort.Slice(in.Clonotypes, func(i, j int) bool { return in.Clonotypes[i].ID < in.Clonotypes[j].ID })
	return in, nil
}
func Compute(ctx context.Context, design study.CohortInput, in Input, options Options, progress func(float64)) (*Result, error) {
	o, err := options.Normalize()
	if err != nil {
		return nil, err
	}
	m, err := Design(design, o.AdjustBatch)
	if err != nil {
		return nil, err
	}
	n := len(m.Rows)
	if len(in.Samples) != n || len(in.Totals) != n || len(in.Clonotypes) > 20000 || len(in.Clonotypes)*n > 1000000 {
		return nil, errors.New("count snapshot dimensions exceed bounds or differ from design")
	}
	r := &Result{Options: o, Design: m, Totals: in.Totals, Offsets: make([]float64, n), Description: Description, Rows: []Row{}}
	for i, s := range in.Samples {
		if s != m.Rows[i].Name {
			return nil, errors.New("count snapshot order differs from design")
		}
		if !finite(in.Totals[i]) || in.Totals[i] <= 0 || in.Totals[i] > 1e12 || math.Trunc(in.Totals[i]) != in.Totals[i] {
			return nil, errors.New("counts need positive whole library totals no greater than 1e12")
		}
		r.Offsets[i] = dmath.Log(in.Totals[i])
	}
	sums := make([]float64, n)
	seen := map[string]bool{}
	for _, c := range in.Clonotypes {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if c.ID == "" || seen[c.ID] || len(c.Values) != n {
			return nil, errors.New("duplicate/empty clonotype or incomplete counts")
		}
		seen[c.ID] = true
		row := Row{Counts: c, Log2Fold: missing(), SE: missing(), Lower: missing(), Upper: missing(), Dispersion: missing(), LR: missing(), P: missing(), Adjusted: missing()}
		present := 0
		for i, y := range c.Values {
			if !finite(y) || y < 0 || y > 1e9 || math.Trunc(y) != y {
				return nil, errors.New("NB counts must be whole, nonnegative and no greater than 1e9; missing/fractional counts are not zeros")
			}
			sums[i] += y
			if y >= float64(o.MinCount) {
				present++
			}
			cpm := float64(y / in.Totals[i] * 1e6)
			row.CPM = append(row.CPM, cpm)
			row.MeanCPM += cpm
		}
		row.MeanCPM /= float64(n)
		row.Eligible = present >= o.MinSamples
		if row.Eligible {
			r.Family++
		} else {
			row.Reason = fmt.Sprintf("Below count filter: at least %d in %d samples required", o.MinCount, o.MinSamples)
		}
		r.Rows = append(r.Rows, row)
	}
	for i := range sums {
		if sums[i] != in.Totals[i] {
			return nil, errors.New("count matrix does not sum to the recorded library totals")
		}
	}
	p := len(m.Columns)
	if r.Family > 1000 || r.Family*n*p*p > 3000000 {
		return nil, errors.New("count fitting exceeds 1000 eligible clonotypes or design work bound; tighten the declared count filter")
	}
	nullX := make([][]float64, n)
	for i, x := range m.X {
		nullX[i] = append([]float64{x[0]}, x[2:]...)
	}
	ps := make([]float64, 0, r.Family)
	positions := []int{}
	z := diversity.NormalQuantile(1 - float64((1-o.Confidence)/2))
	for i := range r.Rows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		row := &r.Rows[i]
		if !row.Eligible {
			continue
		}
		positions = append(positions, i)
		ps = append(ps, 1)
		if progress != nil {
			progress(float64(len(positions)) / float64(r.Family))
		}
		// All-zero cells of a blocking factor yield infinite nuisance estimates. Keep
		// the row and its family slot rather than dropping those subjects to force a fit.
		groups := [2]float64{}
		subjects := map[string]float64{}
		for j, y := range row.Values {
			g := int(m.X[j][1])
			groups[g] += y
			subjects[m.Rows[j].Subject] += y
		}
		if groups[0] == 0 || groups[1] == 0 {
			row.Reason = "All-zero group: finite group coefficient unavailable"
			continue
		}
		if design.Spec.Layout == "paired" {
			zero := false
			for _, v := range subjects {
				if v == 0 {
					zero = true
				}
			}
			if zero {
				row.Reason = "All-zero subject block: no subject removed to obtain a fit"
				continue
			}
		}
		alpha, fit, e := FitDispersion(ctx, m.X, row.Values, r.Offsets)
		if e != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			row.Reason = e.Error()
			continue
		}
		null, e := FitNB(ctx, nullX, row.Values, r.Offsets, alpha)
		if e != nil {
			row.Reason = "Null fit: " + e.Error()
			continue
		}
		lr := 2 * (fit.LogLikelihood - null.LogLikelihood)
		if lr < -1e-6 || !finite(lr) {
			row.Reason = "Nested likelihoods inconsistent"
			continue
		}
		lr = math.Max(0, lr)
		se := math.Sqrt(fit.Covariance[1][1]) / dmath.Log(2)
		effect := fit.Beta[1] / dmath.Log(2)
		half := mul(z, se)
		row.Full, row.Null = fit, null
		row.Dispersion = diversity.Number(alpha)
		row.Log2Fold = diversity.Number(effect)
		row.SE = diversity.Number(se)
		row.Lower = diversity.Number(effect - half)
		row.Upper = diversity.Number(effect + half)
		row.LR = diversity.Number(lr)
		row.P = diversity.Number(dmath.Erfc(math.Sqrt(lr / 2)))
		r.Tested++
		ps[len(ps)-1] = float64(row.P)
	}
	qs := expansion.AdjustBH(ps)
	for i, pos := range positions {
		if finite(float64(r.Rows[pos].P)) {
			r.Rows[pos].Adjusted = diversity.Number(qs[i])
		}
	}
	return r, nil
}
