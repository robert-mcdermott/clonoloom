package validation

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"testing"

	"clonoloom/internal/cohort"
	"clonoloom/internal/diversity"
	"clonoloom/internal/expansion"
	"clonoloom/internal/stats"
	"clonoloom/internal/study"
)

func TestCohortComparisonsAgainstIndependentReferences(t *testing.T) {
	raw, err := os.ReadFile("reference/cohort_compare.json")
	if err != nil {
		t.Fatal(err)
	}
	var ref struct {
		Cases []struct {
			Name                                                         string
			Paired                                                       bool
			A, B                                                         []float64
			Mean                                                         float64
			T, DF, P, Lower, Upper                                       *float64
			Rank, RankP, Probability, ProbabilityLower, ProbabilityUpper float64
			Tail, Draws                                                  int
		}
		BHInput, BH []float64
	}
	if err = json.Unmarshal(raw, &ref); err != nil {
		t.Fatal(err)
	}
	for _, c := range ref.Cases {
		t.Run(c.Name, func(t *testing.T) {
			input := study.CohortInput{Dataset: "reference", Spec: study.CohortSpec{Unit: "subject", Layout: "independent", GroupField: "condition", Levels: []string{"A", "B"}}}
			if c.Paired {
				input.Spec.Layout = "paired"
			}
			table := study.FeatureTable{Catalog: []study.Feature{{ID: "measurement", Name: "Known measurement", Unit: "units"}}}
			for group, xs := range [][]float64{c.A, c.B} {
				for i, x := range xs {
					subject := fmt.Sprintf("S%02d", i)
					if !c.Paired {
						subject = fmt.Sprintf("G%d-%s", group, subject)
					}
					s := study.CohortSample{Name: fmt.Sprintf("G%d-%02d", group, i), Subject: subject, Condition: input.Spec.Levels[group], Batch: "shared"}
					input.Samples = append(input.Samples, s)
					table.Rows = append(table.Rows, study.FeatureRow{CohortSample: s, Group: s.Condition, Values: []study.FeatureValue{{Value: diversity.Number(x)}}})
				}
			}
			result, err := cohort.Compute(context.Background(), input, table, cohort.Options{Features: []string{"measurement"}, Permutations: 99999})
			if err != nil {
				t.Fatal(err)
			}
			r := result.Comparisons[0]
			check := func(name string, got float64, want *float64) {
				t.Helper()
				if want == nil {
					if !math.IsNaN(got) {
						t.Errorf("%s got %g expected null", name, got)
					}
					return
				}
				if !isFiniteCohort(got) || math.Abs(got-*want) > 1e-10*math.Max(1, math.Abs(*want)) {
					t.Errorf("%s %.17g want %.17g", name, got, *want)
				}
			}
			check("mean", float64(r.Mean.Estimate), &c.Mean)
			check("t", float64(r.Parametric.Statistic), c.T)
			check("df", float64(r.Parametric.DF), c.DF)
			check("p", float64(r.Parametric.P), c.P)
			check("lower", float64(r.Mean.Lower), c.Lower)
			check("upper", float64(r.Mean.Upper), c.Upper)
			check("rank", float64(r.Rank.Statistic), &c.Rank)
			check("probability", float64(r.Probability.Estimate), &c.Probability)
			check("probability lower", float64(r.Probability.Lower), &c.ProbabilityLower)
			check("probability upper", float64(r.Probability.Upper), &c.ProbabilityUpper)
			if c.Draws <= cohort.ExactLimit {
				check("rank p", float64(r.Rank.P), &c.RankP)
				if r.Rank.Draws != c.Draws || r.Rank.Tail != c.Tail {
					t.Fatal(r.Rank)
				}
			} else {
				if math.Abs(float64(r.Rank.P)-c.RankP) > .01 || r.Rank.Draws != 99999 {
					t.Fatalf("Monte Carlo %v vs exact %g", r.Rank, c.RankP)
				}
			}
		})
	}
	got := expansion.AdjustBH(ref.BHInput)
	for i, want := range ref.BH {
		if math.Abs(got[i]-want) > 1e-14 {
			t.Fatal(got, ref.BH)
		}
	}
}
func isFiniteCohort(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }

func TestCohortStudentDistributionAgainstSciPy(t *testing.T) {
	raw, err := os.ReadFile("reference/cohort_compare.json")
	if err != nil {
		t.Fatal(err)
	}
	var ref struct {
		Tails     []struct{ DF, T, P float64 }
		Quantiles []struct{ DF, P, Value float64 }
	}
	if err := json.Unmarshal(raw, &ref); err != nil {
		t.Fatal(err)
	}
	if len(ref.Tails) != 35 || len(ref.Quantiles) != 42 {
		t.Fatal("incomplete distribution reference")
	}
	for _, x := range ref.Tails {
		got := stats.StudentTTwoSided(x.T, x.DF)
		if !isFiniteCohort(got) || math.Abs(got-x.P) > 1e-10*math.Max(x.P, 1e-300) {
			t.Errorf("tail df=%g t=%g: %.17g want %.17g", x.DF, x.T, got, x.P)
		}
	}
	for _, x := range ref.Quantiles {
		got := stats.StudentTQuantile(x.P, x.DF)
		if !isFiniteCohort(got) || math.Abs(got-x.Value) > 1e-9*math.Max(1, math.Abs(x.Value)) {
			t.Errorf("quantile df=%g p=%g: %.17g want %.17g", x.DF, x.P, got, x.Value)
		}
	}
}
