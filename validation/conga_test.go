package validation

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"reflect"
	"testing"

	"clonoloom/internal/clonotype"
	"clonoloom/internal/conga"
	"clonoloom/internal/expression"
	"clonoloom/internal/phenotype"
)

func TestCongaAgainstIndependentGraphsAndExactSubsets(t *testing.T) {
	var ref struct {
		Cases []struct {
			Name  string
			Nodes []struct {
				ID     string
				Chains []clonotype.Chain
				Scores [][]float64
			}
			Means      [][]float64
			Candidates []int
			Tests      []conga.Test
		}
		Tails []struct {
			N, A, B, X int
			LogP       float64
		}
	}
	raw, err := os.ReadFile("reference/conga.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &ref); err != nil {
		t.Fatal(err)
	}
	for _, c := range ref.Cases {
		t.Run(c.Name, func(t *testing.T) {
			d, _ := clonotype.Presets[3].Normalize()
			p := &phenotype.Result{Definition: d}
			m := &expression.AnalysisResult{Components: 2}
			cs := []clonotype.Clonotype{}
			for _, n := range c.Nodes {
				cs = append(cs, clonotype.Clonotype{ID: n.ID, Chains: n.Chains})
				for i, v := range n.Scores {
					barcode := fmt.Sprintf("%s-%d", n.ID, i)
					p.Cells = append(p.Cells, phenotype.Cell{Barcode: barcode, Clone: n.ID, Status: "Assigned", Chains: "Paired"})
					m.Cells = append(m.Cells, expression.AnalysisCell{Barcode: barcode, Scores: v})
				}
			}
			r, err := conga.Compute(context.Background(), p, m, cs, nil, conga.Options{CDR3Only: true, Neighbors: []int{1, 2, 3, 9}}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if r.Tested != len(c.Tests) || r.Eligible != 18 {
				t.Fatal(r.Tested, r.Eligible)
			}
			for i, n := range r.Nodes {
				if !reflect.DeepEqual(n.Scores, c.Means[i]) || n.Candidates != c.Candidates[i] {
					t.Fatal("mean/population", n)
				}
			}
			j := 0
			for _, got := range r.Tests {
				if got.LogP == nil {
					if got.K != 9 || got.Reason == "" {
						t.Fatal(got)
					}
					continue
				}
				want := c.Tests[j]
				j++
				if got.Node != want.Node || got.K != want.K || got.Overlap == nil || want.Overlap == nil || *got.Overlap != *want.Overlap || !reflect.DeepEqual(got.Expression, want.Expression) || !reflect.DeepEqual(got.Receptor, want.Receptor) {
					t.Fatalf("graph got %+v want %+v", got, want)
				}
				for _, v := range [][2]*float64{{got.P, want.P}, {got.Adjusted, want.Adjusted}, {got.Expected, want.Expected}} {
					if v[0] == nil || v[1] == nil || math.Abs(*v[0]-*v[1]) > 1e-13 {
						t.Fatal("statistic", got, want)
					}
				}
			}
		})
	}
	for _, x := range ref.Tails {
		got := conga.LogTail(x.N, x.A, x.B, x.X)
		if math.Abs(got-x.LogP) > 1e-9 {
			t.Fatalf("log tail %v: %.16g != %.16g", x, got, x.LogP)
		}
	}
}
