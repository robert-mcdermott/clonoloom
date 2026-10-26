package simulate

import (
	"clonoloom/internal/clonotype"
	"clonoloom/internal/cohort"
	"clonoloom/internal/reads"
	"clonoloom/internal/robustness"
	"clonoloom/internal/study"
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestDefinitionStudyTruth(t *testing.T) {
	s := Presets["definition-study"]
	dir := filepath.Join(t.TempDir(), "study")
	sum, err := Run(context.Background(), s, dir, "validation", nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "truth/observations.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(raw)))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 1+32*648 {
		t.Fatal(err, len(rows))
	}
	paths := []string{}
	names := []string{}
	design := study.CohortInput{Dataset: "simulated", Spec: study.CohortSpec{Unit: "subject", Layout: "independent", GroupField: "condition", Levels: []string{"control", "stimulated"}}}
	for i := 0; i < 32; i++ {
		name := fmt.Sprintf("S%02d-1", i+1)
		names = append(names, name)
		paths = append(paths, filepath.Join(dir, name+".tsv"))
		group := "control"
		if i >= 16 {
			group = "stimulated"
		}
		design.Samples = append(design.Samples, study.CohortSample{Name: name, Subject: fmt.Sprintf("S%02d", i+1), Condition: group, Batch: fmt.Sprintf("B%d", i%2)})
	}
	source, err := reads.ReadFiles(context.Background(), paths)
	if err != nil {
		t.Fatal(err)
	}
	base := clonotype.Definition{Junction: "aa", V: "gene", J: "gene", ProductiveOnly: true}
	nt, alleles, alone := base, base, base
	nt.Junction = "nt"
	alleles.V = "allele"
	alleles.J = "allele"
	alone.V = "none"
	alone.J = "none"
	defs, err := robustness.Definitions(base, []clonotype.Definition{nt, alleles, alone})
	if err != nil {
		t.Fatal(err)
	}
	inputs := []robustness.Input{}
	zero, singleton := 0, 0
	distinct := map[string]bool{}
	// Literal truth grouping is independent of the AIRR reader and clonotype engine.
	for di, d := range defs {
		bins := map[string]map[string]float64{}
		for _, name := range names {
			bins[name] = map[string]float64{}
		}
		for _, r := range rows[1:] {
			count, e := strconv.ParseFloat(r[10], 64)
			if e != nil {
				t.Fatal(e)
			}
			if di == 0 {
				if count == 0 {
					zero++
				}
				if count == 1 {
					singleton++
				}
				key := r[0] + "/" + r[5]
				if distinct[key] {
					t.Fatal("nucleotide collision")
				}
				distinct[key] = true
			}
			if count == 0 {
				continue
			}
			junction := r[13]
			if d.Junction == "nt" {
				junction = r[5]
			}
			v, j := r[11], r[12]
			if d.V == "gene" {
				v = strings.Split(v, "*")[0]
				j = strings.Split(j, "*")[0]
			}
			if d.V == "none" {
				v = ""
				j = ""
			}
			bins[r[0]][junction+"/"+v+"/"+j] += count
		}
		ct, e := clonotype.Compute(source.Table, nil, d)
		if e != nil {
			t.Fatal(e)
		}
		in, e := robustness.Capture(ct, names)
		if e != nil {
			t.Fatal(e)
		}
		for _, sample := range in.Samples {
			want := []float64{}
			for _, v := range bins[sample.Name] {
				want = append(want, v)
			}
			sort.Float64s(want)
			if fmt.Sprint(want) != fmt.Sprint(sample.Abundances) {
				t.Fatalf("%d %s regrouping differs", di, sample.Name)
			}
		}
		inputs = append(inputs, in)
	}
	if zero < 1000 || singleton < 1000 || sum.Samples != 32 {
		t.Fatal("lost realistic sparse tail", zero, singleton, sum)
	}
	baseline, err := robustness.Features(context.Background(), design, inputs[0], "umis")
	if err != nil {
		t.Fatal(err)
	}
	result, err := robustness.Compute(context.Background(), design, "umis", inputs, cohort.Options{Features: []string{"richness", "shannon", "hill1"}, Permutations: 999}, baseline, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, summary := range result.Summaries {
		t.Logf("%s: %s; %g to %g", summary.Feature.ID, summary.Direction, summary.Minimum, summary.Maximum)
		if summary.Available != 4 || summary.Intervals != 4 {
			t.Fatal(summary)
		}
	}
	if result.Summaries[1].Direction != "Effect direction changes across definitions" {
		t.Fatal("richness control lost intended reversal", result.Summaries[1])
	}
	got := digest(t, dir)
	if got != "045736de8c762da8317c3beb541a1e8869df0d06fc1bdf08b3b41218557b9519" {
		t.Fatal("study digest changed", got)
	}
	t.Logf("zero counts %d, singleton counts %d, study digest %s", zero, singleton, got)
}

func TestDefinitionStudyRejectsIncompatibleOptions(t *testing.T) {
	s := Presets["definition-study"]
	counts := *s.Counts
	s.Counts = &counts
	counts.Null = true
	if err := s.Check(); err == nil {
		t.Fatal("definition-sensitive study called all-null")
	}
	counts.Null = false
	counts.Layout = "paired"
	if err := s.Check(); err == nil {
		t.Fatal("unsupported paired definition generator")
	}
}
