// Package batchqc labels unusual sample QC values within declared batches.
// NIST/SEMATECH e-Handbook §1.3.5.17, Iglewicz and Hoaglin (1993).
// This is descriptive labeling, not a calibrated test or batch correction.
package batchqc

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"

	"clonoloom/internal/diversity"
	"clonoloom/internal/dmath"
)

const Version = 1
const Citation = "Iglewicz B, Hoaglin DC. How to Detect and Handle Outliers. ASQC Quality Press; 1993. NIST/SEMATECH e-Handbook §1.3.5.17, https://www.itl.nist.gov/div898/handbook/eda/section3/eda35h.htm. OSCA Advanced, Quality control, redux, https://bioconductor.org/books/release/OSCA.advanced/quality-control-redux.html."
const Description = "Descriptive sample-level QC within explicitly declared batches. Each included subject contributes at most one sample per batch; subjects may recur across batches, so batch summaries are not independent. Modified z = 0.6745 * (transformed value - median) / raw MAD, including the observation in its reference batch. Absolute scores strictly above the declared threshold are review flags, not p-values, calibrated error rates, bad-sample diagnoses or biological effects. Small groups and zero MAD remain unscored, with no epsilon or pooled-batch fallback. Positive abundance and retained-row metrics use log10 without pseudocounts; fractions use their original scale. Missing values are unavailable, not zero. Most observations in each batch must form a meaningful reference: systematic batch failure, heterogeneity, biological confounding and masking can evade or distort flags. Inspect the common-scale distributions and exclusions. No sample is removed and no batch correction or causal inference is performed."

type Assignment struct {
	Name    string `json:"name"`
	Subject string `json:"subject"`
	Batch   string `json:"batch"`
	Exclude string `json:"exclude"`
}
type Options struct {
	Threshold  float64 `json:"threshold"`
	MinSamples int     `json:"minSamples"`
}

func (o Options) Normalize() (Options, error) {
	if o.Threshold == 0 {
		o.Threshold = 3.5
	}
	if o.MinSamples == 0 {
		o.MinSamples = 5
	}
	if math.IsNaN(o.Threshold) || math.IsInf(o.Threshold, 0) || o.Threshold < 1 || o.Threshold > 20 || o.MinSamples < 5 || o.MinSamples > 100 {
		return o, fmt.Errorf("threshold must be 1–20 and minimum samples 5–100")
	}
	return o, nil
}

type Metric struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Scale       string `json:"scale"`
	Denominator string `json:"denominator"`
}

var Metrics = []Metric{
	{"abundance", "Retained abundance", "log10", "All retained counts (bulk) or distinct retained nonempty barcodes (single cell)."},
	{"rows", "Retained rows", "log10", "Every row passing source QC; rows are not independent replicates."},
	{"productive", "Productive fraction", "identity", "All weights passing QC except its productive-only rule; unknown productivity remains in the denominator."},
	{"unknown", "Unknown productivity fraction", "identity", "The same status universe as productive fraction; only absent productive flags are unknown."},
	{"vResolved", "Resolved V-gene fraction", "identity", "All retained explicitly productive weights, including rows missing junctions or gene calls."},
	{"jResolved", "Resolved J-gene fraction", "identity", "All retained explicitly productive weights, including rows missing junctions or gene calls."},
}

type Value struct {
	Value       diversity.Number `json:"value"`
	Numerator   diversity.Number `json:"numerator"`
	Denominator diversity.Number `json:"denominator"`
	Reason      string           `json:"reason"`
}
type Sample struct {
	Assignment Assignment     `json:"assignment"`
	Rows       int            `json:"rows"`
	Kept       int            `json:"kept"`
	Removed    map[string]int `json:"removed"`
	Values     []Value        `json:"values"`
}
type Input struct {
	Options    Options  `json:"options"`
	Unit       string   `json:"unit"`
	SingleCell bool     `json:"singleCell"`
	Samples    []Sample `json:"samples"`
}
type Observation struct {
	Sample      int              `json:"sample"`
	Value       diversity.Number `json:"value"`
	Transformed diversity.Number `json:"transformed"`
	Z           diversity.Number `json:"z"`
	Flag        bool             `json:"flag"`
	Reason      string           `json:"reason"`
}
type Group struct {
	Batch        string           `json:"batch"`
	Metric       string           `json:"metric"`
	Samples      int              `json:"samples"`
	Subjects     int              `json:"subjects"`
	Valid        int              `json:"valid"`
	Scored       int              `json:"scored"`
	Flagged      int              `json:"flagged"`
	Median       diversity.Number `json:"median"`
	MAD          diversity.Number `json:"mad"`
	Reason       string           `json:"reason"`
	Observations []Observation    `json:"observations"`
}
type Result struct {
	Description string   `json:"description"`
	Evidence    Input    `json:"evidence"`
	Metrics     []Metric `json:"metrics"`
	Included    int      `json:"included"`
	Excluded    int      `json:"excluded"`
	Subjects    int      `json:"subjects"`
	Groups      []Group  `json:"groups"`
}

func missing() diversity.Number { return diversity.Number(math.NaN()) }
func median(x []float64) float64 {
	sort.Float64s(x)
	n := len(x)
	if n%2 == 1 {
		return x[n/2]
	}
	return float64(x[n/2-1]/2) + float64(x[n/2]/2)
}
func Compute(ctx context.Context, in Input) (*Result, error) {
	o, e := in.Options.Normalize()
	if e != nil {
		return nil, e
	}
	in.Options = o
	if len(in.Samples) < 2 || len(in.Samples) > 256 {
		return nil, fmt.Errorf("capture 2–256 source samples")
	}
	if in.Unit != "reads" && in.Unit != "umis" && in.Unit != "templates" && in.Unit != "cells" {
		return nil, fmt.Errorf("known abundance unit required")
	}
	names, subjects, batches := map[string]bool{}, map[string]bool{}, map[string][]int{}
	pairs := map[[2]string]bool{}
	r := &Result{Description: Description, Evidence: in, Metrics: append([]Metric{}, Metrics...), Groups: []Group{}}
	for i, s := range in.Samples {
		a := s.Assignment
		if a.Name == "" || names[a.Name] || len(a.Name) > 256 || len(a.Subject) > 256 || len(a.Batch) > 256 || len(a.Exclude) > 2048 || len(s.Values) != len(Metrics) {
			return nil, fmt.Errorf("invalid captured sample")
		}
		names[a.Name] = true
		for _, v := range s.Values {
			x := float64(v.Value)
			if math.IsInf(x, 0) || (!math.IsNaN(x) && x < 0) || math.IsNaN(x) != (v.Reason != "") {
				return nil, fmt.Errorf("invalid metric or missingness reason")
			}
		}
		if a.Exclude != "" {
			r.Excluded++
			continue
		}
		if strings.TrimSpace(a.Subject) == "" || strings.TrimSpace(a.Batch) == "" {
			return nil, fmt.Errorf("assign subject and batch or explicitly exclude each sample")
		}
		k := [2]string{a.Subject, a.Batch}
		if pairs[k] {
			return nil, fmt.Errorf("subject %s repeats within batch %s; select one observation using explicit exclusions", a.Subject, a.Batch)
		}
		pairs[k] = true
		subjects[a.Subject] = true
		batches[a.Batch] = append(batches[a.Batch], i)
		r.Included++
	}
	if r.Included < 2 || len(batches) > 32 {
		return nil, fmt.Errorf("include at least two samples in at most 32 batches")
	}
	r.Subjects = len(subjects)
	keys := []string{}
	for k := range batches {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, batch := range keys {
		for mi, metric := range Metrics {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			g := Group{Batch: batch, Metric: metric.ID, Samples: len(batches[batch]), Subjects: len(batches[batch]), Median: missing(), MAD: missing(), Observations: []Observation{}}
			values := []float64{}
			for _, i := range batches[batch] {
				v := in.Samples[i].Values[mi]
				x := float64(v.Value)
				p := Observation{Sample: i, Value: v.Value, Transformed: missing(), Z: missing(), Reason: v.Reason}
				if p.Reason == "" && metric.Scale == "log10" {
					if x == 0 {
						p.Reason = "Zero has no log10 value; no pseudocount"
					} else {
						x = dmath.Log(x) / dmath.Log(10)
					}
				}
				if p.Reason == "" {
					if metric.Scale == "identity" && x > 1 {
						return nil, fmt.Errorf("fraction exceeds one")
					}
					p.Transformed = diversity.Number(x)
					values = append(values, x)
				}
				g.Observations = append(g.Observations, p)
			}
			g.Valid = len(values)
			if g.Valid > 0 {
				med := median(values)
				g.Median = diversity.Number(med)
				dev := make([]float64, len(values))
				for i, x := range values {
					dev[i] = math.Abs(x - med)
				}
				g.MAD = diversity.Number(median(dev))
			}
			switch {
			case g.Valid < o.MinSamples:
				g.Reason = "Too few evaluable subjects for the declared minimum"
			case g.MAD == 0:
				g.Reason = "Zero MAD: reference spread is undefined"
			}
			for i := range g.Observations {
				p := &g.Observations[i]
				if p.Reason != "" {
					continue
				}
				if g.Reason != "" {
					p.Reason = g.Reason
					continue
				}
				z := .6745 * (float64(p.Transformed) - float64(g.Median)) / float64(g.MAD)
				if math.IsNaN(z) || math.IsInf(z, 0) {
					p.Reason = "Score exceeds numerical range"
					continue
				}
				p.Z = diversity.Number(z)
				p.Flag = math.Abs(z) > o.Threshold
				g.Scored++
				if p.Flag {
					g.Flagged++
				}
			}
			r.Groups = append(r.Groups, g)
		}
	}
	return r, nil
}
func CSV(w io.Writer, r *Result) error {
	c := csv.NewWriter(w)
	if e := c.Write([]string{"batch", "metric", "sample", "subject", "raw_value", "transformed", "median", "raw_mad", "modified_z", "review_flag", "reason", "valid_subjects", "scored_subjects"}); e != nil {
		return e
	}
	num := func(x diversity.Number) string {
		if math.IsNaN(float64(x)) {
			return ""
		}
		return strconv.FormatFloat(float64(x), 'g', -1, 64)
	}
	for _, g := range r.Groups {
		for _, v := range g.Observations {
			a := r.Evidence.Samples[v.Sample].Assignment
			flag := ""
			if v.Reason == "" {
				flag = strconv.FormatBool(v.Flag)
			}
			if e := c.Write([]string{g.Batch, g.Metric, a.Name, a.Subject, num(v.Value), num(v.Transformed), num(g.Median), num(g.MAD), num(v.Z), flag, v.Reason, strconv.Itoa(g.Valid), strconv.Itoa(g.Scored)}); e != nil {
				return e
			}
		}
	}
	c.Flush()
	return c.Error()
}
