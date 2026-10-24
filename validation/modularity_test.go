package validation

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"testing"

	"clonoloom/internal/graph"
	"clonoloom/internal/modularity"
)

func TestModularityAgainstExhaustiveReference(t *testing.T) {
	var ref struct {
		Cases []struct {
			Name                   string
			N                      int
			Edges                  [][2]int
			Labels                 []string
			Eligible, InducedEdges int
			Clones                 []modularity.Clone
			Nulls                  []modularity.Null
		}
	}
	raw, err := os.ReadFile("reference/modularity.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &ref); err != nil {
		t.Fatal(err)
	}
	for _, c := range ref.Cases {
		t.Run(c.Name, func(t *testing.T) {
			g := &graph.CSR{N: c.N, Offsets: []int{0}}
			cells := make([]modularity.Cell, c.N)
			for i := 0; i < c.N; i++ {
				cells[i] = modularity.Cell{Barcode: fmt.Sprint(i), Clone: c.Labels[i]}
				for j := 0; j < c.N; j++ {
					for _, e := range c.Edges {
						if e == [2]int{i, j} || e == [2]int{j, i} {
							g.Targets = append(g.Targets, int32(j))
							g.Weights = append(g.Weights, 1)
						}
					}
				}
				g.Offsets = append(g.Offsets, len(g.Targets))
			}
			r, err := modularity.Compute(context.Background(), g, cells, modularity.Options{Permutations: 99999}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if r.Eligible != c.Eligible || r.Edges != c.InducedEdges || len(r.Clones) != len(c.Clones) {
				t.Fatalf("population: %+v", r)
			}
			close := func(label string, got, want float64, tolerance float64) {
				t.Helper()
				if math.Abs(got-want) > tolerance {
					t.Errorf("%s: %.16g want %.16g", label, got, want)
				}
			}
			ps := []float64{}
			for i, want := range c.Clones {
				got := r.Clones[i]
				if got.ID != want.ID || got.Cells != want.Cells || got.Observed != want.Observed {
					t.Fatal(got, want)
				}
				if want.P == nil {
					if got.P != nil || got.Score != nil || got.Adjusted != nil || got.Reason == "" {
						t.Fatal(got)
					}
					continue
				}
				close("expectation", *got.Expected, *want.Expected, 1e-14)
				close("score", *got.Score, *want.Score, 1e-14)
				ps = append(ps, *want.P)
				tolerance := 1e-14
				if c.Name == "monte-carlo-target" {
					tolerance = .01
				}
				close("p", *got.P, *want.P, tolerance)
				close("adjusted p", *got.Adjusted, *want.Adjusted, tolerance)
			}
			// SciPy BY comparison uses exact p-values independently of the Monte Carlo stream.
			q, _ := modularity.AdjustBY(ps)
			qi := 0
			for _, want := range c.Clones {
				if want.P != nil {
					close("SciPy BY", q[qi], *want.Adjusted, 1e-14)
					qi++
				}
			}
			for i, want := range c.Nulls {
				got := r.Nulls[i]
				close("analytic expected vs enumerated mean", got.Expected, want.Expected, 1e-14)
				frequencies := map[int]int{}
				for _, b := range got.Histogram {
					frequencies[b.Edges] = b.Count
				}
				if c.Name != "monte-carlo-target" {
					if got.Method != "Exhaustive" || got.Draws != want.Draws {
						t.Fatal(got)
					}
					for _, b := range want.Histogram {
						if frequencies[b.Edges] != b.Count {
							t.Fatal("exact histogram", got, want)
						}
					}
				} else {
					if got.Method != "Monte Carlo" {
						t.Fatal(got)
					}
					for _, b := range want.Histogram {
						close("Monte Carlo frequency vs exhaustive probability", float64(frequencies[b.Edges])/float64(got.Draws), float64(b.Count)/float64(want.Draws), .01)
					}
				}
				// Inclusive exact p-values are super-uniform, including discrete ties.
				for _, alpha := range []float64{.01, .05, .1, .5} {
					rejections := 0
					for _, b := range want.Histogram {
						tail := 0
						for _, v := range want.Histogram {
							if v.Edges >= b.Edges {
								tail += v.Count
							}
						}
						if float64(tail)/float64(want.Draws) <= alpha {
							rejections += b.Count
						}
					}
					if float64(rejections)/float64(want.Draws) > alpha {
						t.Fatal("invalid exact test level")
					}
				}
			}
		})
	}
}
