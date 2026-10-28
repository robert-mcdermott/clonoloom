package robustness

import (
	"bytes"
	"clonoloom/internal/clonotype"
	"clonoloom/internal/cohort"
	"clonoloom/internal/diversity"
	"clonoloom/internal/study"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
)

func fixture(t *testing.T) (study.CohortInput, []Input, study.FeatureTable) {
	t.Helper()
	d := study.CohortInput{Dataset: "fixture", Spec: study.CohortSpec{Unit: "subject", Layout: "independent", GroupField: "condition", Levels: []string{"A", "B"}}}
	base, _ := (clonotype.Definition{Junction: "aa", V: "gene", J: "gene", ProductiveOnly: true}).Normalize()
	alt := base
	alt.Junction = "nt"
	ins := []Input{{Definition: base}, {Definition: alt}}
	for i := 0; i < 8; i++ {
		name := fmt.Sprintf("S%d", i)
		g := "A"
		if i >= 4 {
			g = "B"
		}
		d.Samples = append(d.Samples, study.CohortSample{Name: name, Subject: name, Condition: g, Batch: "shared"})
		xs := []float64{float64(i + 1), 2, 3}
		for j := range ins {
			ins[j].Samples = append(ins[j].Samples, Sample{Name: name, Abundances: xs})
		}
	}
	table, err := Features(context.Background(), d, ins[0], "umis")
	if err != nil {
		t.Fatal(err)
	}
	return d, ins, table
}
func TestDefinitionAuditMissingFailureAndReplay(t *testing.T) {
	d, ins, table := fixture(t)
	opts := cohort.Options{Features: []string{"abundance", "shannon"}, Permutations: 999}
	ins[1].Failure = "declared alternative cannot be computed"
	r, err := Compute(context.Background(), d, "umis", ins, opts, table, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Family != 8 || r.Variants[1].Analysis != nil || !math.IsNaN(float64(r.Variants[1].Across[0].Mean)) || !strings.HasPrefix(r.Summaries[0].Direction, "Incomplete") {
		t.Fatal(r)
	}
	var out bytes.Buffer
	if err = CSV(&out, r); err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(&out).ReadAll()
	if err != nil || len(rows) != 5 || rows[3][5] != "" || rows[3][19] != ins[1].Failure {
		t.Fatal(rows, err)
	}
	raw, _ := json.Marshal(ins)
	var replay []Input
	if err = json.Unmarshal(raw, &replay); err != nil {
		t.Fatal(err)
	}
	back, err := Compute(context.Background(), d, "umis", replay, opts, table, nil)
	a, _ := json.Marshal(r)
	b, _ := json.Marshal(back)
	if err != nil || !bytes.Equal(a, b) {
		t.Fatal("replay differs", err)
	}
	ins[0].Samples[0].Abundances = []float64{100}
	if _, err = Compute(context.Background(), d, "umis", ins, opts, table, nil); err == nil {
		t.Fatal("stale baseline accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = Compute(ctx, d, "umis", replay, opts, table, nil); err != context.Canceled {
		t.Fatal(err)
	}
}
func TestDefinitionAuditPolicies(t *testing.T) {
	d, ins, _ := fixture(t)
	b := ins[0].Definition
	good := ins[1].Definition
	if _, err := Definitions(b, []clonotype.Definition{good}); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*clonotype.Definition){func(d *clonotype.Definition) { d.Junction = "aa" }, func(d *clonotype.Definition) { d.ProductiveOnly = false }, func(d *clonotype.Definition) { d.Cells = "all" }, func(d *clonotype.Definition) { d.Chains = []string{"TRB"} }, func(d *clonotype.Definition) { d.RequireAll = true }, func(d *clonotype.Definition) { d.Vendor = true }, func(d *clonotype.Definition) { d.Name = strings.Repeat("x", 161) }} {
		a := good
		change(&a)
		if _, err := Definitions(b, []clonotype.Definition{a}); err == nil {
			t.Fatal(a)
		}
	}
	if _, err := Definitions(b, []clonotype.Definition{good, good}); err == nil {
		t.Fatal("duplicate")
	}
	for _, xs := range [][]float64{{0}, {-1}, {math.NaN()}, {math.Inf(1)}, {1e13}} {
		bad := Input{Samples: []Sample{{Name: "S0", Abundances: xs}}}
		if _, err := Features(context.Background(), d, bad, "umis"); err == nil {
			t.Fatal(xs)
		}
	}
	bad := ins[0]
	bad.Samples = append(bad.Samples, bad.Samples[0])
	if _, err := Features(context.Background(), d, bad, "umis"); err == nil {
		t.Fatal("duplicate sample")
	}
	bad = Input{Samples: make([]Sample, 257)}
	if _, err := Features(context.Background(), d, bad, "umis"); err == nil {
		t.Fatal("source bound")
	}
}
func TestDefinitionSummaryDoesNotUsePThresholds(t *testing.T) {
	makeVariant := func(effect, lo, hi, p float64) Variant {
		return Variant{Analysis: &cohort.Result{Comparisons: []cohort.Comparison{{Mean: cohort.Effect{Estimate: diversity.Number(effect), Lower: diversity.Number(lo), Upper: diversity.Number(hi)}, Parametric: cohort.Test{P: diversity.Number(p)}, Rank: cohort.Test{P: diversity.Number(1 - p)}}}}}
	}
	// Only effects and intervals enter the summary, regardless of multiplicity or test availability.
	vs := []Variant{makeVariant(2, 1, 3, .001), makeVariant(-1, -3, 1, .9)}
	s := summarize(vs, 0)
	if s.Direction != "Effect direction changes across definitions" || s.Span != 3 || s.Uncertainty != "Marginal intervals differ in their relation to zero" {
		t.Fatal(s)
	}
	vs[1] = makeVariant(1, -2, 4, .99)
	s = summarize(vs, 0)
	if s.Direction != "Positive estimates across all declared definitions" {
		t.Fatal(s)
	}
	vs[1].Analysis.Comparisons[0].Mean.Lower = missing()
	if !strings.HasPrefix(summarize(vs, 0).Uncertainty, "Incomplete") {
		t.Fatal("unavailable interval hidden")
	}
}

func TestDefinitionAuditRetainsPairingAndCancelsBetweenDefinitions(t *testing.T) {
	d, ins, _ := fixture(t)
	d.Spec.Layout = "paired"
	for i := range d.Samples {
		d.Samples[i].Subject = fmt.Sprintf("P%d", i%4)
	}
	table, err := Features(context.Background(), d, ins[0], "umis")
	if err != nil {
		t.Fatal(err)
	}
	options := cohort.Options{Features: []string{"shannon"}, Permutations: 999}
	r, err := Compute(context.Background(), d, "umis", ins, options, table, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range r.Variants {
		c := v.Analysis.Comparisons[0]
		if v.Analysis.Layout != "paired" || c.Pairs != 4 || len(c.Points) != 8 {
			t.Fatal(v.Analysis)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, err = Compute(ctx, d, "umis", ins, options, table, func(p float64) {
		if p > 0 {
			cancel()
		}
	}); err != context.Canceled {
		t.Fatal(err)
	}
	bad := ins[1]
	bad.Samples[0].Abundances = make([]float64, 400001)
	if _, err = Features(context.Background(), d, bad, "umis"); err == nil || !strings.Contains(err.Error(), "budget") {
		t.Fatal(err)
	}
	if _, err = Capture(&clonotype.Table{Excluded: map[string]int{clonotype.ReasonNoCount: 1}}, nil); err == nil {
		t.Fatal("missing counts accepted")
	}
}
