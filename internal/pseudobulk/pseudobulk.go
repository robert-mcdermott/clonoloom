// Package pseudobulk preserves raw subject/sample count evidence, without inference.
// Aggregation: Crowell et al., Nature Communications (2020), doi:10.1038/s41467-020-19894-4;
// OSCA Multi-Sample, chapter 4. ClonoLoom policies are in research §5.39.
package pseudobulk

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"clonoloom/internal/expression"
	"clonoloom/internal/phenotype"
	"clonoloom/internal/study"
)

const Version = 1
const MaxCount uint64 = 9000000000000000
const MaxEntries = 2000000
const Citation = "Crowell HL et al. muscat detects subpopulation-specific state transitions from multi-sample multi-condition single-cell transcriptomics data. Nature Communications 2020;11:6077. doi:10.1038/s41467-020-19894-4. OSCA Multi-Sample, chapter 4, Creating pseudo-bulk samples: https://bioconductor.org/books/release/OSCA.multisample/multi-sample-comparisons.html. ClonoLoom adds sample-scoped receptor size categories and explicit missingness policies; no differential expression test."
const Description = "Exact raw Gene Expression count sums per subject, sample, literal timepoint, supplied cell type and declared clonotype-size category. No normalization, averaging, imputation, pooling across samples, or differential-expression test. Clone size counts uniquely assigned receptor cells within the source sample; expanded categories do not establish antigen specificity. Cells are not biological replicates. Minimum cells is an evidence rule, not a power guarantee. Absent groups have unavailable counts; measured zeros remain zero. Low-cell and zero-library groups retain raw counts but are not eligible."

type Options struct {
	Annotation string `json:"annotation"`
	MinCells   int    `json:"minCells"`
}

func (o Options) Normalize() (Options, error) {
	o.Annotation = strings.TrimSpace(o.Annotation)
	if o.Annotation == "" {
		return o, errors.New("choose a supplied cell-type annotation column")
	}
	if o.MinCells == 0 {
		o.MinCells = 10
	}
	if o.MinCells < 1 || o.MinCells > 100000 {
		return o, errors.New("minimum cells must be 1–100000")
	}
	return o, nil
}

type Entry struct {
	Feature int    `json:"feature"`
	Count   uint64 `json:"count"`
}
type Cell struct {
	Barcode string  `json:"barcode"`
	Type    string  `json:"type"`
	Bin     string  `json:"bin"`
	Reason  string  `json:"reason"`
	Counts  []Entry `json:"counts"`
}
type Sample struct {
	Parent     string       `json:"parent"`
	Expression string       `json:"expression"`
	Annotation study.Sample `json:"annotation"`
	Cells      []Cell       `json:"cells"`
}
type Input struct {
	Options  Options              `json:"options"`
	Features []expression.Feature `json:"features"`
	Bins     []string             `json:"bins"`
	Samples  []Sample             `json:"samples"`
}
type Group struct {
	Sample   int      `json:"sample"`
	Type     string   `json:"type"`
	Bin      string   `json:"bin"`
	Cells    int      `json:"cells"`
	Counts   []uint64 `json:"counts"`
	Total    *uint64  `json:"total"`
	Eligible bool     `json:"eligible"`
	Reason   string   `json:"reason"`
}
type Coverage struct {
	Cells    int            `json:"cells"`
	Included int            `json:"included"`
	Excluded map[string]int `json:"excluded"`
}
type Result struct {
	Groups   []Group    `json:"groups"`
	Coverage []Coverage `json:"coverage"`
	Subjects int        `json:"subjects"`
	Eligible int        `json:"eligible"`
}

// Capture checks every barcode against the immutable phenotype assignment and retains sparse
// raw counts. Only Gene Expression is included; features are identified by ID, name and type.
func Capture(m *expression.Matrix, p *phenotype.Result, options Options) ([]expression.Feature, []Cell, []string, error) {
	if p == nil || p.Options.Population != "receptor" {
		return nil, nil, nil, errors.New("use receptor-population phenotype sizes; mapped populations are selection-dependent")
	}
	normalized, err := p.Options.Normalize()
	if err != nil || len(p.Options.Breakpoints) == 0 || len(normalized.Breakpoints) != len(p.Options.Breakpoints) {
		return nil, nil, nil, errors.New("invalid phenotype size policy")
	}
	if len(m.Counts) > MaxEntries {
		return nil, nil, nil, errors.New("expression matrix exceeds 2000000 sparse entries")
	}
	column := -1
	for i, c := range p.Columns {
		if c == options.Annotation {
			column = i
		}
	}
	if column < 0 {
		return nil, nil, nil, errors.New("supplied annotation column is absent from phenotypes")
	}
	if len(m.Barcodes) != len(p.Cells) || len(m.Barcodes) > 100000 {
		return nil, nil, nil, errors.New("phenotype barcodes differ or exceed 100000 cells")
	}
	features := []expression.Feature{}
	indices := map[int]int{}
	for i, f := range m.Features {
		if f.Type == "Gene Expression" {
			indices[i] = len(features)
			features = append(features, f)
		}
	}
	if len(features) == 0 || len(features) > 50000 {
		return nil, nil, nil, errors.New("require 1–50000 Gene Expression features")
	}
	bins := []string{}
	for _, b := range p.Options.Breakpoints {
		bins = append(bins, p.Options.Bin(b))
	}
	bins = append(bins, p.Options.Bin(p.Options.Breakpoints[len(p.Options.Breakpoints)-1]+1))
	cells := make([]Cell, len(p.Cells))
	for i, c := range p.Cells {
		if c.Barcode != m.Barcodes[i] || column >= len(c.Values) {
			return nil, nil, nil, errors.New("phenotype cell order or annotation shape differs")
		}
		v := Cell{Barcode: c.Barcode, Type: c.Values[column], Counts: []Entry{}}
		if v.Type == "" {
			v.Reason = "Missing cell-type annotation"
		} else if c.Size == nil || *c.Size < 1 || c.Clone == "" {
			v.Reason = "Unavailable unique clonotype size"
		} else {
			v.Bin = p.Options.Bin(*c.Size)
			if v.Bin != c.Bin {
				return nil, nil, nil, errors.New("phenotype size category is inconsistent")
			}
		}
		for j := m.Indptr[i]; j < m.Indptr[i+1]; j++ {
			if f, ok := indices[int(m.Indices[j])]; ok && m.Counts[j] > 0 {
				v.Counts = append(v.Counts, Entry{f, m.Counts[j]})
			}
		}
		cells[i] = v
	}
	return features, cells, bins, nil
}

func Compute(ctx context.Context, in Input) (*Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	o, e := in.Options.Normalize()
	if e != nil {
		return nil, e
	}
	if len(in.Samples) < 1 || len(in.Samples) > 32 || len(in.Features) < 1 || len(in.Features) > 50000 || len(in.Bins) < 2 || len(in.Bins) > 6 {
		return nil, errors.New("require 1–32 samples, 1–50000 features and 2–6 size categories")
	}
	bins := map[string]bool{}
	for _, b := range in.Bins {
		if b == "" || bins[b] {
			return nil, errors.New("invalid size categories")
		}
		bins[b] = true
	}
	features := map[string]bool{}
	for _, f := range in.Features {
		if f.Type != "Gene Expression" || f.ID == "" || features[f.ID] {
			return nil, errors.New("ambiguous or non-Gene Expression feature identity")
		}
		features[f.ID] = true
	}
	types := map[string]bool{}
	subjects := map[string]bool{}
	observations := map[[2]string]bool{}
	sources := map[string]bool{}
	names := map[string]bool{}
	entries, totalCells := 0, 0
	for _, s := range in.Samples {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		a := s.Annotation
		key := [2]string{a.Subject, a.Time}
		if a.Name == "" || strings.TrimSpace(a.Subject) == "" || strings.TrimSpace(a.Time) == "" || observations[key] || sources[s.Expression] || names[a.Name] || s.Parent == "" || s.Expression == "" {
			return nil, errors.New("pin unique samples/expression inputs with subject and literal timepoint; repeated subject/time observations are not pooled")
		}
		observations[key] = true
		sources[s.Expression] = true
		names[a.Name] = true
		subjects[a.Subject] = true
		barcodes := map[string]bool{}
		totalCells += len(s.Cells)
		if totalCells > 200000 {
			return nil, errors.New("at most 200000 captured cells")
		}
		for _, c := range s.Cells {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if c.Barcode == "" || barcodes[c.Barcode] {
				return nil, errors.New("empty or duplicate barcode within sample")
			}
			barcodes[c.Barcode] = true
			if c.Type != "" {
				types[c.Type] = true
			}
			if c.Reason == "" && (!bins[c.Bin] || c.Type == "") {
				return nil, errors.New("eligible cell lacks declared category or type")
			}
			if c.Reason != "" && (c.Bin != "" || (c.Reason != "Missing cell-type annotation" && c.Reason != "Unavailable unique clonotype size")) {
				return nil, errors.New("invalid cell exclusion")
			}
			last := -1
			var total uint64
			for _, v := range c.Counts {
				entries++
				if entries > MaxEntries || v.Feature <= last || v.Feature >= len(in.Features) || v.Count == 0 || v.Count > MaxCount || total > MaxCount-v.Count {
					return nil, errors.New("invalid sparse counts or count/capture bound exceeded")
				}
				last = v.Feature
				total += v.Count
			}
		}
	}
	if len(types) == 0 || len(types) > 128 || len(types)*len(in.Bins)*len(in.Samples)*len(in.Features) > MaxEntries {
		return nil, errors.New("require 1–128 supplied types and at most 2000000 output count slots")
	}
	labels := []string{}
	for t := range types {
		labels = append(labels, t)
	}
	sort.Strings(labels)
	r := &Result{Subjects: len(subjects), Groups: []Group{}, Coverage: []Coverage{}}
	for si, s := range in.Samples {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		coverage := Coverage{Cells: len(s.Cells), Excluded: map[string]int{}}
		groups := map[[2]string]*Group{}
		for _, typ := range labels {
			for _, b := range in.Bins {
				groups[[2]string{typ, b}] = &Group{Sample: si, Type: typ, Bin: b}
			}
		}
		for _, c := range s.Cells {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if c.Reason != "" {
				coverage.Excluded[c.Reason]++
				continue
			}
			coverage.Included++
			g := groups[[2]string{c.Type, c.Bin}]
			if g.Cells == 0 {
				g.Counts = make([]uint64, len(in.Features))
				g.Total = new(uint64)
			}
			g.Cells++
			for _, v := range c.Counts {
				if *g.Total > MaxCount-v.Count {
					return nil, errors.New("aggregate library exceeds exact count bound")
				}
				*g.Total += v.Count
				g.Counts[v.Feature] += v.Count
			}
		}
		for _, typ := range labels {
			for _, b := range in.Bins {
				g := groups[[2]string{typ, b}]
				switch {
				case g.Cells == 0:
					g.Reason = "No eligible cells"
				case g.Cells < o.MinCells:
					g.Reason = "Below minimum cells"
				case *g.Total == 0:
					g.Reason = "Zero count library"
				default:
					g.Eligible = true
					r.Eligible++
				}
				r.Groups = append(r.Groups, *g)
			}
		}
		r.Coverage = append(r.Coverage, coverage)
	}
	return r, nil
}

// CSV includes every group × feature, with empty count fields only for absent groups.
func CSV(w io.Writer, in Input, r *Result) error {
	c := csv.NewWriter(w)
	if e := c.Write([]string{"sample", "subject", "timepoint", "tissue", "cell_type", "size_category", "cells", "eligible", "reason", "feature_id", "feature_name", "feature_type", "count", "library_total"}); e != nil {
		return e
	}
	for _, g := range r.Groups {
		a := in.Samples[g.Sample].Annotation
		for i, f := range in.Features {
			count, total := "", ""
			if g.Counts != nil {
				count = strconv.FormatUint(g.Counts[i], 10)
				total = strconv.FormatUint(*g.Total, 10)
			}
			if e := c.Write([]string{a.Name, a.Subject, a.Time, a.Tissue, g.Type, g.Bin, fmt.Sprint(g.Cells), fmt.Sprint(g.Eligible), g.Reason, f.ID, f.Name, f.Type, count, total}); e != nil {
				return e
			}
		}
	}
	c.Flush()
	return c.Error()
}
