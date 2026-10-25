package validation

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"clonoloom/internal/expression"
)

func TestExpressionMapAgainstIndependentNumericalTools(t *testing.T) {
	var ref struct {
		Sources map[string]string
		Truth   []int
		Curve   []float64
		Cases   []struct {
			Options                            expression.AnalysisOptions
			Selected                           []string
			Means, Variances, Eigenvalues      []float64
			Loadings, Scores, Distances, Graph [][]float64
			Indices                            [][]int
			ReferenceTrustworthiness           float64
		}
	}
	raw, err := os.ReadFile("reference/expression_map.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &ref); err != nil {
		t.Fatal(err)
	}
	dir := "../testdata/expression-map"
	for name, sum := range ref.Sources {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if fmt.Sprintf("%x", sha256.Sum256(raw)) != sum {
			t.Fatal("changed fixture", name)
		}
	}
	in, err := expression.OpenWithCells(context.Background(), dir, filepath.Join(dir, "cells.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	close := func(t *testing.T, label string, got, want, tol float64) {
		t.Helper()
		if math.IsNaN(got) || math.Abs(got-want) > tol*math.Max(1, math.Abs(want)) {
			t.Fatalf("%s: got %.15g, want %.15g", label, got, want)
		}
	}
	for caseIndex, c := range ref.Cases {
		t.Run(fmt.Sprint(caseIndex), func(t *testing.T) {
			r, err := expression.Analyze(context.Background(), in, c.Options, 3, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(r.Features) != len(c.Selected) || len(r.Cells) != len(c.Scores) {
				t.Fatal("shape")
			}
			for i, f := range r.Features {
				if f.ID != c.Selected[i] {
					t.Fatal(f.ID, c.Selected[i])
				}
				close(t, "log mean", f.Mean, c.Means[i], 1e-12)
				close(t, "log variance", f.Variance, c.Variances[i], 1e-12)
				for j, l := range f.Loadings {
					close(t, "loading", l, c.Loadings[i][j], 1e-10)
				}
			}
			for i, v := range r.Variances {
				close(t, "eigenvalue", v, c.Eigenvalues[i], 1e-11)
			}
			for i, cell := range r.Cells {
				for j, v := range cell.Scores {
					close(t, "score", v, c.Scores[i][j], 1e-10)
				}
			}
			correct := 0
			for i, row := range c.Indices {
				for j, want := range row {
					at := i*r.Neighbors.K + j
					got := int(r.Neighbors.Index[at])
					if got != want {
						t.Fatal("neighbor", i, j, got, want)
					}
					close(t, "distance", r.Neighbors.Distances[at], c.Distances[i][j], 1e-10)
					if ref.Truth[i] == ref.Truth[got] {
						correct++
					}
				}
			}
			n := len(r.Cells)
			for i := 0; i < n; i++ {
				weights := make([]float64, n)
				for e := r.Graph.Offsets[i]; e < r.Graph.Offsets[i+1]; e++ {
					weights[r.Graph.Targets[e]] = r.Graph.Weights[e]
				}
				for j, v := range weights {
					close(t, fmt.Sprintf("fuzzy graph %d,%d", i, j), v, c.Graph[i][j], 1e-5)
				}
			}
			close(t, "UMAP curve a", r.Map.A, ref.Curve[0], 2e-5)
			close(t, "UMAP curve b", r.Map.B, ref.Curve[1], 2e-5)
			purity := float64(correct) / float64(n*r.Neighbors.K)
			if purity < .95 {
				t.Fatal("known-state neighbor purity", purity)
			}
			quality := expressionMapTrustworthiness(r, 7)
			t.Logf("trustworthiness %.6f (umap-learn %.6f); known-state neighbor purity %.6f", quality, c.ReferenceTrustworthiness, purity)
			if quality < .9 || quality < c.ReferenceTrustworthiness-.05 {
				t.Fatal("UMAP failed neighborhood preservation", quality)
			}
		})
	}
}

// Venna & Kaski (2001), ICANN pp485–491: ranks in PCA space of false map neighbors.
// Full pairwise ranks here are an independent test-only assessment, not a runtime estimator.
func expressionMapTrustworthiness(r *expression.AnalysisResult, k int) float64 {
	n := len(r.Cells)
	penalty := 0
	for i := 0; i < n; i++ {
		high, low := make([]float64, n), make([]float64, n)
		orderHigh, orderLow := make([]int, n), make([]int, n)
		for j := 0; j < n; j++ {
			orderHigh[j], orderLow[j] = j, j
			for d, v := range r.Cells[i].Scores {
				delta := v - r.Cells[j].Scores[d]
				high[j] += float64(delta * delta)
			}
			for d, v := range r.Cells[i].Position {
				delta := v - r.Cells[j].Position[d]
				low[j] += float64(delta * delta)
			}
		}
		high[i], low[i] = math.Inf(1), math.Inf(1)
		sort.SliceStable(orderHigh, func(a, b int) bool { return high[orderHigh[a]] < high[orderHigh[b]] })
		sort.SliceStable(orderLow, func(a, b int) bool { return low[orderLow[a]] < low[orderLow[b]] })
		ranks := make([]int, n)
		for rank, j := range orderHigh {
			ranks[j] = rank + 1
		}
		for _, j := range orderLow[:k] {
			if ranks[j] > k {
				penalty += ranks[j] - k
			}
		}
	}
	return 1 - 2*float64(penalty)/float64(n*k*(2*n-3*k-1))
}

// An optional locally supplied public 10x matrix exercises the bounded pipeline at real input
// size. No donor counts or derived coordinates are bundled; parsing validation is separate.
func TestExpressionMapPublicHDF5(t *testing.T) {
	path := os.Getenv("CLONOLOOM_EXPRESSION_MAP_HDF5")
	if path == "" {
		t.Skip("set CLONOLOOM_EXPRESSION_MAP_HDF5 to a local 10x count matrix")
	}
	in, err := expression.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	r, err := expression.Analyze(context.Background(), in, expression.AnalysisOptions{}, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Cells)+len(r.Excluded) != r.InputCells || r.InputCells != len(in.Matrix.Barcodes) {
		t.Fatal("population accounting")
	}
	for _, c := range r.Cells {
		for _, v := range c.Position {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				t.Fatal("nonfinite coordinates")
			}
		}
	}
	t.Logf("%d input cells, %d retained, %d features, %d PCs, %d graph edges", r.InputCells, len(r.Cells), len(r.Features), r.Components, r.Graph.Edges())
}
