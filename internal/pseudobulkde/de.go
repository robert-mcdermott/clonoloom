// Package pseudobulkde fits paired raw gene counts from immutable pseudobulk evidence.
// Anders & Huber (2010), doi:10.1186/gb-2010-11-10-r106 (size factors);
// McCarthy et al. (2012), doi:10.1093/nar/gks042 (NB2/Cox–Reid).
// This unmoderated implementation is not DESeq2 or edgeR. See research §5.40.
package pseudobulkde

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"

	"clonoloom/internal/countmodel"
	"clonoloom/internal/diversity"
	"clonoloom/internal/dmath"
	"clonoloom/internal/expansion"
	"clonoloom/internal/expression"
	"clonoloom/internal/pseudobulk"
	"clonoloom/internal/study"
)

const Version = 1
const Citation = "Crowell HL et al. Nature Communications 2020;11:6077. doi:10.1038/s41467-020-19894-4. Anders S, Huber W. Genome Biology 2010;11:R106. doi:10.1186/gb-2010-11-10-r106. McCarthy DJ, Chen Y, Smyth GK. Nucleic Acids Res. 2012;40:4288–4297. doi:10.1093/nar/gks042. Benjamini Y, Hochberg Y. JRSS B. 1995;57:289–300. doi:10.1111/j.2517-6161.1995.tb02031.x."
const Description = "Paired pseudobulk gene expression: one supplied cell type, literal timepoint and two declared receptor-size categories within each subject. Raw integer sums; log median-of-ratios size-factor offsets computed before gene filtering, with geometric-mean centering. Requires most normalization genes unchanged or balanced changes; global RNA shifts and composition bias are not identified. Subject fixed effects and a comparison indicator; no cell-level replication, repeated visits, pooling or automatic complete-case deletion. NB2 variance = mean + dispersion × mean²; separate unmoderated Cox–Reid dispersion per gene. Nested likelihood-ratio p-values use asymptotic chi-square(1) at the fitted full-model dispersion. Marginal Wald normal intervals are conditional on fitted dispersion and size factors, not BH-adjusted; dispersion/normalization uncertainty and small-sample calibration are not fully modeled. At least 25 complete subject pairs (24 residual df) is an implementation guard, not a power guarantee. BH covers every count/prevalence-eligible gene, with failed fits retained as p=1 family placeholders and displayed unavailable. No correction across types, timepoints, contrasts or exploratory choices. No empirical-Bayes moderation, quasi-likelihood, shrinkage, Poisson fallback or equivalence to edgeR/DESeq2. Category-associated relative expression does not establish antigen specificity, causality, absolute RNA per cell or generalization beyond sampled subjects. Expression-defined labels and size selection may introduce selection bias; subject blocking does not remove category-specific confounding."

type Options struct {
	Type       string            `json:"type"`
	Time       string            `json:"timepoint"`
	Reference  string            `json:"reference"`
	Comparison string            `json:"comparison"`
	Exclude    map[string]string `json:"exclude"`
	MinCount   int               `json:"minCount"`
	MinSamples int               `json:"minSamples"`
	Confidence float64           `json:"confidence"`
}

func (o Options) Normalize() (Options, error) {
	if strings.TrimSpace(o.Type) == "" || strings.TrimSpace(o.Time) == "" || strings.TrimSpace(o.Reference) == "" || strings.TrimSpace(o.Comparison) == "" || o.Reference == o.Comparison {
		return o, errors.New("choose an exact supplied cell type, literal timepoint and two different declared size categories")
	}
	if o.MinCount == 0 {
		o.MinCount = 10
	}
	if o.MinSamples == 0 {
		o.MinSamples = 25
	}
	if o.Confidence == 0 {
		o.Confidence = .95
	}
	if o.MinCount < 1 || o.MinCount > 1000000 || o.MinSamples < 2 || o.MinSamples > 64 || !(o.Confidence >= .8 && o.Confidence <= .99) {
		return o, errors.New("choose minCount 1–1000000, minSamples 2–64 and confidence 0.8–0.99")
	}
	ex := map[string]string{}
	for k, v := range o.Exclude {
		if strings.TrimSpace(v) == "" {
			return o, errors.New("every explicit sample exclusion needs a reason")
		}
		ex[k] = strings.TrimSpace(v)
	}
	o.Exclude = ex
	return o, nil
}

// Input retains all raw groups and sample annotations, including excluded observations.
// Cell membership remains in the content-addressed parent; replay needs only these sums.
type Input struct {
	Features []expression.Feature `json:"features"`
	Bins     []string             `json:"bins"`
	Samples  []study.Sample       `json:"samples"`
	MinCells int                  `json:"minCells"`
	Groups   []pseudobulk.Group   `json:"groups"`
}

func Capture(in pseudobulk.Input, r *pseudobulk.Result) Input {
	out := Input{Features: in.Features, Bins: in.Bins, MinCells: in.Options.MinCells, Groups: r.Groups}
	for _, s := range in.Samples {
		out.Samples = append(out.Samples, s.Annotation)
	}
	return out
}

type Disposition struct {
	Sample   study.Sample `json:"sample"`
	Included bool         `json:"included"`
	Reason   string       `json:"reason"`
}
type Observation struct {
	Sample  string `json:"sample"`
	Subject string `json:"subject"`
	Bin     string `json:"bin"`
	Cells   int    `json:"cells"`
	Total   uint64 `json:"total"`
}
type Row struct {
	Feature    expression.Feature `json:"feature"`
	Counts     []uint64           `json:"counts"`
	Normalized []float64          `json:"normalized"`
	Mean       float64            `json:"mean"`
	Eligible   bool               `json:"eligible"`
	Reason     string             `json:"reason"`
	Log2Fold   diversity.Number   `json:"log2Fold"`
	SE         diversity.Number   `json:"se"`
	Lower      diversity.Number   `json:"lower"`
	Upper      diversity.Number   `json:"upper"`
	Dispersion diversity.Number   `json:"dispersion"`
	LR         diversity.Number   `json:"lr"`
	P          diversity.Number   `json:"p"`
	Adjusted   diversity.Number   `json:"adjusted"`
	Full       *countmodel.Fit    `json:"full,omitempty"`
	Null       *countmodel.Fit    `json:"null,omitempty"`
}
type Result struct {
	Options            Options           `json:"options"`
	Dispositions       []Disposition     `json:"dispositions"`
	Observations       []Observation     `json:"observations"`
	Design             countmodel.Matrix `json:"design"`
	SizeFactors        []float64         `json:"sizeFactors"`
	Offsets            []float64         `json:"offsets"`
	NormalizationGenes []string          `json:"normalizationGenes"`
	Family             int               `json:"family"`
	Tested             int               `json:"tested"`
	Rows               []Row             `json:"rows"`
	Description        string            `json:"description"`
}

func missing() diversity.Number { return diversity.Number(math.NaN()) }

// Prepare validates complete pairs and captures the design, normalization and test family.
func Prepare(in Input, options Options) (*Result, error) {
	o, e := options.Normalize()
	if e != nil {
		return nil, e
	}
	if len(in.Samples) < 25 || len(in.Samples) > 32 || len(in.Features) < 10 || len(in.Features) > 50000 || len(in.Groups)*len(in.Features) > 2000000 || in.MinCells < 1 {
		return nil, errors.New("require 25–32 samples, 10–50000 genes and at most 2000000 raw group × gene slots")
	}
	if !slices.Contains(in.Bins, o.Reference) || !slices.Contains(in.Bins, o.Comparison) {
		return nil, errors.New("requested category is absent from the saved size policy")
	}
	names, ids := map[string]bool{}, map[string]bool{}
	for _, s := range in.Samples {
		if s.Name == "" || s.Subject == "" || s.Time == "" || s.Subject != strings.TrimSpace(s.Subject) || names[s.Name] {
			return nil, errors.New("invalid or duplicate sample/subject/time annotation")
		}
		names[s.Name] = true
	}
	for k := range o.Exclude {
		if !names[k] {
			return nil, fmt.Errorf("excluded sample %q is absent from the evidence", k)
		}
	}
	for _, f := range in.Features {
		if f.ID == "" || ids[f.ID] || f.Type != "Gene Expression" {
			return nil, errors.New("invalid Gene Expression feature identity")
		}
		ids[f.ID] = true
	}
	groups := map[[3]string]pseudobulk.Group{}
	for _, g := range in.Groups {
		if g.Sample < 0 || g.Sample >= len(in.Samples) || g.Type == "" || !slices.Contains(in.Bins, g.Bin) || g.Cells < 0 {
			return nil, errors.New("invalid raw group identity")
		}
		key := [3]string{in.Samples[g.Sample].Name, g.Type, g.Bin}
		if _, ok := groups[key]; ok {
			return nil, errors.New("duplicate raw group")
		}
		groups[key] = g
		if g.Cells == 0 {
			if g.Counts != nil || g.Total != nil || g.Eligible {
				return nil, errors.New("absent group must retain unavailable counts")
			}
			continue
		}
		if len(g.Counts) != len(in.Features) || g.Total == nil {
			return nil, errors.New("incomplete raw gene counts")
		}
		var total uint64
		for _, y := range g.Counts {
			if y > 1000000000 || total > 1000000000000-y {
				return nil, errors.New("gene counts exceed 1e9 or library sum exceeds 1e12")
			}
			total += y
		}
		if total != *g.Total || g.Eligible != (g.Cells >= in.MinCells && total > 0) {
			return nil, errors.New("raw total or cell eligibility disagrees with saved evidence")
		}
	}
	r := &Result{Options: o, Description: Description, Rows: []Row{}, Dispositions: []Disposition{}}
	design := study.CohortInput{Dataset: "pseudobulk", Spec: study.CohortSpec{Unit: "subject", Layout: "paired", GroupField: "condition", Levels: []string{o.Reference, o.Comparison}, Exclude: map[string]string{}}}
	byName := map[string]pseudobulk.Group{}
	obs := map[string]Observation{}
	subjects := map[string]bool{}
	for _, s := range in.Samples {
		d := Disposition{Sample: s, Included: true}
		if reason, ok := o.Exclude[s.Name]; ok {
			d.Included = false
			d.Reason = "Excluded: " + reason
		} else if s.Time != o.Time {
			d.Included = false
			d.Reason = "Outside selected literal timepoint"
		}
		r.Dispositions = append(r.Dispositions, d)
		if !d.Included {
			continue
		}
		if subjects[s.Subject] {
			return nil, errors.New("more than one selected sample per subject; repeated observations are not independent")
		}
		subjects[s.Subject] = true
		for _, bin := range []string{o.Reference, o.Comparison} {
			g, ok := groups[[3]string{s.Name, o.Type, bin}]
			if !ok || !g.Eligible {
				return nil, fmt.Errorf("sample %q, type %q, category %q lacks eligible counts; explicitly exclude the sample with a reason or revise the evidence policy", s.Name, o.Type, bin)
			}
			name := strconv.Quote(s.Name) + " / " + strconv.Quote(bin)
			byName[name] = g
			obs[name] = Observation{s.Name, s.Subject, bin, g.Cells, *g.Total}
			design.Samples = append(design.Samples, study.CohortSample{Name: name, Subject: s.Subject, Condition: bin, Timepoint: s.Time, Tissue: s.Tissue, CellSubset: o.Type})
		}
	}
	if len(subjects) < 25 {
		return nil, errors.New("unmoderated paired expression requires at least 25 complete subjects (24 residual degrees of freedom); no cell-level replication or automatic pair deletion")
	}
	r.Design, e = countmodel.Design(design, false)
	if e != nil {
		return nil, e
	}
	n := len(r.Design.Rows)
	if o.MinSamples > n {
		return nil, errors.New("count prevalence exceeds the number of paired pseudobulk observations")
	}
	for _, d := range r.Design.Rows {
		r.Observations = append(r.Observations, obs[d.Name])
	}
	for j, f := range in.Features {
		row := Row{Feature: f, Log2Fold: missing(), SE: missing(), Lower: missing(), Upper: missing(), Dispersion: missing(), LR: missing(), P: missing(), Adjusted: missing()}
		present := 0
		for _, d := range r.Design.Rows {
			y := byName[d.Name].Counts[j]
			row.Counts = append(row.Counts, y)
			if y >= uint64(o.MinCount) {
				present++
			}
		}
		row.Eligible = present >= o.MinSamples
		if row.Eligible {
			r.Family++
		} else {
			row.Reason = fmt.Sprintf("Below count filter: at least %d counts in %d paired observations required", o.MinCount, o.MinSamples)
		}
		r.Rows = append(r.Rows, row)
	}
	if r.Family > 1000 {
		return nil, errors.New("unmoderated fitting supports at most 1000 eligible genes; tighten the declared count filter or export raw counts for a larger analysis")
	}
	// Ordinary median of count/geometric-mean ratios; zeros omit a gene from normalization,
	// never add a pseudocount. Filtering does not redefine this reference set.
	ratios := make([][]float64, n)
	for _, row := range r.Rows {
		sum := 0.0
		positive := true
		for _, y := range row.Counts {
			if y == 0 {
				positive = false
				break
			}
			sum += dmath.Log(float64(y))
		}
		if !positive {
			continue
		}
		gm := dmath.Exp(sum / float64(n))
		r.NormalizationGenes = append(r.NormalizationGenes, row.Feature.ID)
		for i, y := range row.Counts {
			ratios[i] = append(ratios[i], float64(y)/gm)
		}
	}
	if len(r.NormalizationGenes) < 10 {
		return nil, errors.New("median-of-ratios normalization requires at least ten genes positive in every selected observation; no pseudocount fallback")
	}
	center := 0.0
	for _, v := range ratios {
		sort.Float64s(v)
		m := v[len(v)/2]
		if len(v)%2 == 0 {
			m = (m + v[len(v)/2-1]) / 2
		}
		r.SizeFactors = append(r.SizeFactors, m)
		center += dmath.Log(m)
	}
	center = dmath.Exp(center / float64(n))
	for i := range r.SizeFactors {
		r.SizeFactors[i] /= center
		r.Offsets = append(r.Offsets, dmath.Log(r.SizeFactors[i]))
	}
	for j := range r.Rows {
		row := &r.Rows[j]
		for i, y := range row.Counts {
			v := float64(y) / r.SizeFactors[i]
			row.Normalized = append(row.Normalized, v)
			row.Mean += v
		}
		row.Mean /= float64(n)
	}
	return r, nil
}
func Compute(ctx context.Context, in Input, options Options, progress func(float64)) (*Result, error) {
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	r, e := Prepare(in, options)
	if e != nil {
		return nil, e
	}
	nullX := make([][]float64, len(r.Design.X))
	for i, x := range r.Design.X {
		nullX[i] = append([]float64{x[0]}, x[2:]...)
	}
	ps := []float64{}
	positions := []int{}
	z := diversity.NormalQuantile(1 - float64((1-r.Options.Confidence)/2))
	for i := range r.Rows {
		if e := ctx.Err(); e != nil {
			return nil, e
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
		y := make([]float64, len(row.Counts))
		totals := map[string]uint64{}
		group := [2]uint64{}
		for j, v := range row.Counts {
			y[j] = float64(v)
			totals[r.Observations[j].Subject] += v
			group[int(r.Design.X[j][1])] += v
		}
		if group[0] == 0 || group[1] == 0 {
			row.Reason = "All-zero category: finite coefficient unavailable"
			continue
		}
		zero := false
		for _, v := range totals {
			if v == 0 {
				zero = true
			}
		}
		if zero {
			row.Reason = "All-zero subject block: no subject removed to obtain a fit"
			continue
		}
		alpha, full, err := countmodel.FitDispersion(ctx, r.Design.X, y, r.Offsets)
		if err != nil {
			row.Reason = err.Error()
			continue
		}
		null, err := countmodel.FitNB(ctx, nullX, y, r.Offsets, alpha)
		if err != nil {
			row.Reason = "Null fit: " + err.Error()
			continue
		}
		lr := 2 * (full.LogLikelihood - null.LogLikelihood)
		if math.IsNaN(lr) || math.IsInf(lr, 0) || lr < -1e-6 {
			row.Reason = "Nested likelihoods inconsistent"
			continue
		}
		lr = math.Max(0, lr)
		effect := full.Beta[1] / dmath.Log(2)
		se := math.Sqrt(full.Covariance[1][1]) / dmath.Log(2)
		half := float64(z * se)
		row.Log2Fold = diversity.Number(effect)
		row.SE = diversity.Number(se)
		row.Lower = diversity.Number(effect - half)
		row.Upper = diversity.Number(effect + half)
		row.Dispersion = diversity.Number(alpha)
		row.LR = diversity.Number(lr)
		row.P = diversity.Number(dmath.Erfc(math.Sqrt(lr / 2)))
		row.Full = full
		row.Null = null
		r.Tested++
		ps[len(ps)-1] = float64(row.P)
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	qs := expansion.AdjustBH(ps)
	for j, i := range positions {
		if !math.IsNaN(float64(r.Rows[i].P)) {
			r.Rows[i].Adjusted = diversity.Number(qs[j])
		}
	}
	return r, nil
}
