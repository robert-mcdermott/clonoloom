package pseudobulkde

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"strings"
	"testing"
)

func fixture(t *testing.T) (Input, Options) {
	t.Helper()
	b, e := os.ReadFile("../../validation/reference/pseudobulk_de.json")
	if e != nil {
		t.Fatal(e)
	}
	var v struct {
		Input   Input
		Options Options
	}
	if e = json.Unmarshal(b, &v); e != nil {
		t.Fatal(e)
	}
	return v.Input, v.Options
}
func TestRefusals(t *testing.T) {
	for name, change := range map[string]func(*Input, *Options){
		"duplicate subject": func(i *Input, o *Options) { i.Samples[1].Subject = i.Samples[0].Subject },
		"insufficient pairs": func(i *Input, o *Options) {
			for _, s := range i.Samples[:4] {
				o.Exclude[s.Name] = "declared exclusion"
			}
		},
		"absent type":       func(i *Input, o *Options) { o.Type = "missing" },
		"unknown category":  func(i *Input, o *Options) { o.Reference = "invented" },
		"equal categories":  func(i *Input, o *Options) { o.Comparison = o.Reference },
		"unknown time":      func(i *Input, o *Options) { o.Time = "day 0" },
		"unknown exclusion": func(i *Input, o *Options) { o.Exclude["unknown"] = "reason" },
		"empty reason":      func(i *Input, o *Options) { o.Exclude[i.Samples[0].Name] = " " },
		"low coverage":      func(i *Input, o *Options) { i.Groups[0].Cells = 1; i.Groups[0].Eligible = false },
		"missing counts":    func(i *Input, o *Options) { i.Groups[0].Counts = nil },
		"total mismatch":    func(i *Input, o *Options) { i.Groups[0].Counts[0]++ },
		"false eligibility": func(i *Input, o *Options) { i.Groups[0].Eligible = false },
		"feature modality":  func(i *Input, o *Options) { i.Features[0].Type = "Antibody Capture" },
		"duplicate feature": func(i *Input, o *Options) { i.Features[0].ID = i.Features[1].ID },
		"duplicate group":   func(i *Input, o *Options) { i.Groups = append(i.Groups, i.Groups[0]) },
		"missing normalization": func(i *Input, o *Options) {
			for j := range i.Groups[0].Counts {
				i.Groups[0].Counts[j] = 0
			}
			*i.Groups[0].Total = 0
			i.Groups[0].Eligible = false
		},
		"nonfinite confidence": func(i *Input, o *Options) { o.Confidence = math.NaN() },
	} {
		t.Run(name, func(t *testing.T) {
			i, o := fixture(t)
			change(&i, &o)
			if _, e := Prepare(i, o); e == nil {
				t.Fatal("accepted")
			}
		})
	}
	i, o := fixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := Compute(ctx, i, o, nil); e == nil {
		t.Fatal("ignored cancel")
	}
}
func TestNormalizationIndependentOfFilter(t *testing.T) {
	i, o := fixture(t)
	a, e := Prepare(i, o)
	if e != nil {
		t.Fatal(e)
	}
	o.MinCount = 1000000
	b, e := Prepare(i, o)
	if e != nil {
		t.Fatal(e)
	}
	if b.Family != 0 {
		t.Fatal("filter")
	}
	for j, v := range a.SizeFactors {
		if b.SizeFactors[j] != v {
			t.Fatal("filter changed normalization")
		}
	}
}
func TestExclusionsAndTimeAreExplicit(t *testing.T) {
	i, o := fixture(t)
	i.Groups[0].Cells = 1
	i.Groups[0].Eligible = false
	o.Exclude[i.Samples[0].Name] = "low coverage before fitting"
	r, e := Prepare(i, o)
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Observations) != 54 || r.Dispositions[0].Included || !strings.Contains(r.Dispositions[0].Reason, "low coverage") {
		t.Fatal("exclusion lost")
	}
	i, o = fixture(t)
	i.Samples[0].Time = "later"
	r, e = Prepare(i, o)
	if e != nil {
		t.Fatal(e)
	}
	if r.Dispositions[0].Included || len(r.Observations) != 54 {
		t.Fatal("time selection")
	}
}
func TestContrastReversal(t *testing.T) {
	i, o := fixture(t)
	a, e := Compute(context.Background(), i, o, nil)
	if e != nil {
		t.Fatal(e)
	}
	o.Reference, o.Comparison = o.Comparison, o.Reference
	b, e := Compute(context.Background(), i, o, nil)
	if e != nil {
		t.Fatal(e)
	}
	for j, v := range a.Rows {
		w := b.Rows[j]
		if !math.IsNaN(float64(v.P)) && (math.Abs(float64(v.Log2Fold+w.Log2Fold)) > 1e-5 || math.Abs(float64(v.P-w.P)) > 1e-5) {
			t.Fatal("contrast reversal", v.Feature.ID)
		}
	}
}

func TestNormalizationAndFailurePolicies(t *testing.T) {
	in, o := fixture(t)
	for j := range in.Groups[0].Counts {
		if j >= 8 {
			*in.Groups[0].Total -= in.Groups[0].Counts[j]
			in.Groups[0].Counts[j] = 0
		}
	}
	if _, e := Prepare(in, o); e == nil || !strings.Contains(e.Error(), "at least ten") {
		t.Fatal("normalization refusal", e)
	}
	in, o = fixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, e := Compute(ctx, in, o, func(float64) { cancel() }); e != context.Canceled {
		t.Fatal("mid-fit cancellation", e)
	}
	in, o = fixture(t)
	r, e := Compute(context.Background(), in, o, nil)
	if e != nil {
		t.Fatal(e)
	}
	if r.Family != 38 || !r.Rows[37].Eligible || !strings.Contains(r.Rows[37].Reason, "All-zero subject block") || !math.IsNaN(float64(r.Rows[37].P)) || !math.IsNaN(float64(r.Rows[37].Adjusted)) {
		t.Fatal("failed family slot")
	}
	// Identical display names cannot merge feature identities.
	in.Features[1].Name = in.Features[0].Name
	if r, e = Prepare(in, o); e != nil || len(r.Rows) != 40 {
		t.Fatal("merged gene names", e)
	}
}
