package validation

import (
	"clonoloom/internal/clonotype"
	"clonoloom/internal/cohort"
	"clonoloom/internal/rep"
	"clonoloom/internal/robustness"
	"clonoloom/internal/study"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"testing"
)

func TestDefinitionRobustnessIndependentReference(t *testing.T) {
	raw, err := os.ReadFile("reference/robustness.json")
	if err != nil {
		t.Fatal(err)
	}
	var ref struct {
		Rows []struct {
			Sample, NT, AA, V, J string
			Count                float64
		}
		Features []string
		Variants []struct {
			Definition  clonotype.Definition
			Values      []map[string]float64
			Comparisons []struct {
				Feature                      string
				Mean, Lower, Upper, P, RankP float64
			}
		}
		BY []float64
	}
	if err = json.Unmarshal(raw, &ref); err != nil {
		t.Fatal(err)
	}
	b := rep.NewBuilder(rep.UMIs)
	design := study.CohortInput{Dataset: "reference", Spec: study.CohortSpec{Unit: "subject", Layout: "independent", GroupField: "condition", Levels: []string{"A", "B"}}}
	names := []string{}
	for i := 0; i < 8; i++ {
		name := fmt.Sprintf("S%d", i)
		names = append(names, name)
		group := "A"
		if i >= 4 {
			group = "B"
		}
		design.Samples = append(design.Samples, study.CohortSample{Name: name, Subject: name, Condition: group, Batch: "shared"})
	}
	for _, v := range ref.Rows {
		r := rep.NewRow()
		r.Sample = v.Sample
		r.Locus = "TRB"
		r.Junction = v.NT
		r.JunctionAA = v.AA
		r.V = v.V
		r.J = v.J
		r.Count = v.Count
		r.Productive = rep.True
		if err = b.Append(&r); err != nil {
			t.Fatal(err)
		}
	}
	inputs := []robustness.Input{}
	for _, v := range ref.Variants {
		def := v.Definition
		def.ProductiveOnly = true
		table, e := clonotype.Compute(b.Table(), nil, def)
		if e != nil {
			t.Fatal(e)
		}
		in, e := robustness.Capture(table, names)
		if e != nil {
			t.Fatal(e)
		}
		inputs = append(inputs, in)
	}
	baseline, err := robustness.Features(context.Background(), design, inputs[0], "umis")
	if err != nil {
		t.Fatal(err)
	}
	result, err := robustness.Compute(context.Background(), design, "umis", inputs, cohort.Options{Features: ref.Features, Permutations: 999}, baseline, nil)
	if err != nil {
		t.Fatal(err)
	}
	check := func(name string, got, want float64) {
		t.Helper()
		if math.IsNaN(got) || math.Abs(got-want) > 1e-10*math.Max(1, math.Abs(want)) {
			t.Errorf("%s %.17g != %.17g", name, got, want)
		}
	}
	for i, v := range result.Variants {
		for s, row := range v.Table.Rows {
			for j, f := range v.Table.Catalog {
				if want, ok := ref.Variants[i].Values[s][f.ID]; ok {
					check(f.ID, float64(row.Values[j].Value), want)
				}
			}
		}
		for j, c := range v.Analysis.Comparisons {
			want := ref.Variants[i].Comparisons[j]
			check("mean", float64(c.Mean.Estimate), want.Mean)
			check("lower", float64(c.Mean.Lower), want.Lower)
			check("upper", float64(c.Mean.Upper), want.Upper)
			check("p", float64(c.Parametric.P), want.P)
			check("rank p", float64(c.Rank.P), want.RankP)
			k := i*2*len(ref.Features) + 2*j
			check("BY mean", float64(v.Across[j].Mean), ref.BY[k])
			check("BY rank", float64(v.Across[j].Rank), ref.BY[k+1])
		}
	}
	bytes, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	got := fmt.Sprintf("%x", sha256.Sum256(bytes))
	if got != "5c0a473f584cbb3f187c60bca747b16d24fdd262445a28ae02d68c412f0d47b7" {
		t.Fatal("result digest changed", got)
	}
	t.Logf("complete result SHA256 %s", got)
}
