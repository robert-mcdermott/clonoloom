package cohort

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"testing"

	"clonoloom/internal/diversity"
	"clonoloom/internal/random"
	"clonoloom/internal/study"
)

func fixture(a, b []float64, paired bool) (study.CohortInput, study.FeatureTable) {
	input := study.CohortInput{Dataset: "fixture", Spec: study.CohortSpec{Unit: "subject", Layout: "independent", GroupField: "condition", Levels: []string{"A", "B"}}}
	if paired {
		input.Spec.Layout = "paired"
	}
	table := study.FeatureTable{Catalog: []study.Feature{{ID: "value", Name: "Value", Unit: "units"}}}
	for group, xs := range [][]float64{a, b} {
		for i, x := range xs {
			subject := fmt.Sprintf("S%05d", i)
			if !paired {
				subject = fmt.Sprintf("G%d-%s", group, subject)
			}
			s := study.CohortSample{Name: fmt.Sprintf("G%d-%05d", group, i), Subject: subject, Condition: input.Spec.Levels[group], Batch: "shared"}
			input.Samples = append(input.Samples, s)
			table.Rows = append(table.Rows, study.FeatureRow{CohortSample: s, Group: s.Condition, Values: []study.FeatureValue{{Value: diversity.Number(x)}}})
		}
	}
	return input, table
}
func TestCohortMissingnessPairingAndFamily(t *testing.T) {
	in, table := fixture([]float64{1, 3, 6}, []float64{2, 5, 8}, true)
	table.Catalog = append(table.Catalog, study.Feature{ID: "missing", Unit: "units"})
	for i := range table.Rows {
		table.Rows[i].Values = append(table.Rows[i].Values, study.FeatureValue{Value: 3})
	}
	table.Rows[0].Values[1] = study.FeatureValue{Value: missing(), Missing: "Not measured"}
	r, err := Compute(context.Background(), in, table, Options{Features: []string{"value", "missing"}})
	if err != nil {
		t.Fatal(err)
	}
	if r.Family != 4 || r.Tested != 2 || r.Comparisons[0].Missing != 1 || r.Comparisons[0].Pairs != 3 || len(r.Comparisons[0].Points) != 6 || !math.IsNaN(float64(r.Comparisons[0].Rank.P)) {
		t.Fatal(r)
	}
	good := r.Comparisons[1]
	if good.Pairs != 3 || math.Abs(float64(good.Mean.Estimate)-5.0/3) > 1e-14 {
		t.Fatal(good)
	}
	// Family retains both untestable slots; choosing only the complete feature cannot
	// make its raw p-values change. Results do not quietly drop the incomplete pair.
	one, err := Compute(context.Background(), in, table, Options{Features: []string{"value"}})
	if err != nil {
		t.Fatal(err)
	}
	if good.Rank.P != one.Comparisons[0].Rank.P || good.Rank.Adjusted < one.Comparisons[0].Rank.Adjusted {
		t.Fatal("family changed raw inference")
	}
	var out bytes.Buffer
	if err = CSV(&out, r); err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(&out).ReadAll()
	if err != nil || len(rows) != 5 || rows[1][12] != "" || rows[1][16] == "" {
		t.Fatal(rows, err)
	}
	// Row order is not biological pairing; subject IDs determine it.
	for i, j := 0, len(table.Rows)-1; i < j; i, j = i+1, j-1 {
		table.Rows[i], table.Rows[j] = table.Rows[j], table.Rows[i]
	}
	reversed, err := Compute(context.Background(), in, table, r.Options)
	if err != nil {
		t.Fatal(err)
	}
	x, _ := json.Marshal(r)
	y, _ := json.Marshal(reversed)
	if !bytes.Equal(x, y) {
		t.Fatal("row order changed pair alignment")
	}
	table.Rows[0].Subject = "wrong"
	if _, err = Compute(context.Background(), in, table, r.Options); err == nil {
		t.Fatal("accepted mismatched subject")
	}
}
func TestCohortRefusalsAndCancellation(t *testing.T) {
	in, table := fixture([]float64{1, 2}, []float64{3, 4}, false)
	for _, o := range []Options{{}, {Features: []string{"value", "value"}}, {Features: []string{"absent"}}, {Features: []string{"value"}, Confidence: 1}, {Features: []string{"value"}, Permutations: 12}} {
		if _, err := Compute(context.Background(), in, table, o); err == nil {
			t.Fatal(o)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Compute(ctx, in, table, Options{Features: []string{"value"}}); err != context.Canceled {
		t.Fatal(err)
	}
	budget := 1
	if _, _, err := rankTest(context.Background(), []float64{1, 2}, []float64{3, 4}, false, Options{Confidence: .95}, "x", &budget); err == nil {
		t.Fatal("ignored budget")
	}
}
func TestCohortConditionalNullCalibration(t *testing.T) {
	// Exhaust every labeling of a fixed tied pool (or every sign configuration).
	// Exact p-values must be super-uniform under their own declared null.
	for _, paired := range []bool{false, true} {
		var ps []float64
		if paired {
			for bits := 0; bits < 16; bits++ {
				a := []float64{0, 0, 0, 0}
				b := []float64{1, 1, 2, 3}
				for i := range b {
					if bits&(1<<i) != 0 {
						b[i] = -b[i]
					}
				}
				budget := WorkLimit
				r, _, err := rankTest(context.Background(), a, b, true, Options{Confidence: .95}, "null", &budget)
				if err != nil {
					t.Fatal(err)
				}
				ps = append(ps, float64(r.P))
			}
		} else {
			pool := []float64{0, 0, 1, 1, 2, 3}
			for i := 0; i < 6; i++ {
				for j := i + 1; j < 6; j++ {
					a, b := []float64{}, []float64{}
					for k, v := range pool {
						if k == i || k == j {
							b = append(b, v)
						} else {
							a = append(a, v)
						}
					}
					budget := WorkLimit
					r, _, err := rankTest(context.Background(), a, b, false, Options{Confidence: .95}, "null", &budget)
					if err != nil {
						t.Fatal(err)
					}
					ps = append(ps, float64(r.P))
				}
			}
		}
		for _, alpha := range []float64{.01, .05, .1, .25, .5} {
			n := 0
			for _, p := range ps {
				if p <= alpha {
					n++
				}
			}
			if float64(n)/float64(len(ps)) > alpha+1e-14 {
				t.Fatal(paired, alpha, ps)
			}
		}
	}
}
func TestCohortNormalModelCoverage(t *testing.T) {
	for _, paired := range []bool{false, true} {
		rng := random.New(72342, 1)
		covered, rejected := 0, 0
		for range 2000 {
			a, b := make([]float64, 8), make([]float64, 11)
			if paired {
				b = make([]float64, len(a))
			}
			for i := range a {
				a[i] = rng.Normal()
			}
			for i := range b {
				b[i] = 4 * rng.Normal()
				if paired {
					b[i] += a[i]
				}
			}
			c := Comparison{Mean: emptyEffect("mean", "units", "t"), Parametric: emptyTest("t")}
			parametric(&c, a, b, paired, .95)
			if c.Mean.Lower <= 0 && c.Mean.Upper >= 0 {
				covered++
			}
			if c.Parametric.P <= .05 {
				rejected++
			}
		}
		if covered < 1850 || covered > 1950 || rejected < 50 || rejected > 150 {
			t.Fatal(paired, covered, rejected)
		}
	}
}
func TestCohortDeterministicMonteCarlo(t *testing.T) {
	a, b := make([]float64, 18), make([]float64, 18)
	for i := range a {
		a[i] = float64(i % 6)
		b[i] = float64((i + 2) % 7)
	}
	in, table := fixture(a, b, false)
	r, err := Compute(context.Background(), in, table, Options{Features: []string{"value"}, Permutations: 999, Seed: 42})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(r)
	sum := sha256.Sum256(raw)
	got := hex.EncodeToString(sum[:])
	const want = "e3ce4cb557d74f5373ac07688e8e7021b64553deff71c63dcec119f99df49464"
	if got != want {
		t.Fatalf("comparison digest %s", got)
	}
}
