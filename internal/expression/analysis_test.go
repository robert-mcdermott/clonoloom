package expression

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"testing"
)

func analysisInput(t *testing.T) *Input {
	t.Helper()
	in, err := OpenWithCells(context.Background(), "../../testdata/expression-map", "../../testdata/expression-map/cells.tsv")
	if err != nil {
		t.Fatal(err)
	}
	return in
}
func TestExpressionAnalysisDeterministic(t *testing.T) {
	in := analysisInput(t)
	before, _ := json.Marshal(in.Matrix)
	options := AnalysisOptions{MinGenes: 2, MinCells: 2, Features: 6, Components: 4, Neighbors: 7, Epochs: 100}
	a, err := Analyze(context.Background(), in, options, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Analyze(context.Background(), in, options, 4, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatal("workers changed the result")
	}
	after, _ := json.Marshal(in.Matrix)
	if string(before) != string(after) {
		t.Fatal("source counts changed")
	}
	if len(a.Cells) != 96 || len(a.Excluded) != 2 || a.Excluded[0].Reason != "no Gene Expression counts" || a.Excluded[1].Reason != "below minimum detected genes" || len(a.Features) != 6 || a.Components != 4 {
		t.Fatal("population or dimensions differ")
	}
	if a.Cells[4].Values[0] != "" || a.Cells[0].Values[0] != "A" {
		t.Fatal("missing annotation changed")
	}
	// Supplied coordinates never enter the computed map.
	for i := range in.Cells.Rows {
		in.Cells.Rows[i].Position = &[2]float64{1e12, -1e12}
	}
	b, err = Analyze(context.Background(), in, options, 2, nil)
	if err != nil || !reflect.DeepEqual(a, b) {
		t.Fatal("source coordinates affected computed map", err)
	}
	raw, _ := json.Marshal(a)
	digest := fmt.Sprintf("%x", sha256.Sum256(raw))
	t.Log("processor digest", digest)
	const want = "7c61d2b6f4b6c51de38f2496a03e41a7d8c39aca52e32f758614d1d5ff062700"
	if digest != want {
		t.Fatalf("processor digest %s, want %s", digest, want)
	}
	options.Seed = 1234
	b, err = Analyze(context.Background(), in, options, 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(a.Cells[0].Position, b.Cells[0].Position) || !reflect.DeepEqual(a.Neighbors, b.Neighbors) || !reflect.DeepEqual(a.Graph, b.Graph) {
		t.Fatal("seed must affect UMAP only")
	}
}
func TestExpressionAnalysisBoundsAndCancellation(t *testing.T) {
	for _, o := range []AnalysisOptions{{MinGenes: -1}, {MinCells: -1}, {Features: 1001}, {Components: 1}, {Neighbors: 101}, {Target: math.NaN()}, {Target: math.Inf(1)}, {Epochs: 1001}, {MinDist: func() *float64 { x := math.NaN(); return &x }()}} {
		if _, err := o.Normalize(); err == nil {
			t.Fatal("accepted invalid options", o)
		}
	}
	in := analysisInput(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Analyze(ctx, in, AnalysisOptions{}, 1, nil); err != context.Canceled {
		t.Fatal(err)
	}
	for _, stage := range []string{"Computing principal components", "Finding exact expression neighbors", "Computing UMAP"} {
		ctx, cancel := context.WithCancel(context.Background())
		_, err := Analyze(ctx, in, AnalysisOptions{}, 2, func(_ float64, msg string) {
			if msg == stage {
				cancel()
			}
		})
		cancel()
		if err != context.Canceled {
			t.Fatal(stage, err)
		}
	}
	if _, err := Analyze(context.Background(), nil, AnalysisOptions{}, 1, nil); err == nil {
		t.Fatal("nil input accepted")
	}
	if _, err := Analyze(context.Background(), in, AnalysisOptions{MinGenes: 20}, 1, nil); err == nil {
		t.Fatal("empty population accepted")
	}
	if _, err := Analyze(context.Background(), in, AnalysisOptions{MinCells: 100}, 1, nil); err == nil {
		t.Fatal("no features accepted")
	}
	// Effective counts are capped, with all defaults kept in the result.
	r, err := Analyze(context.Background(), in, AnalysisOptions{Neighbors: 100, Components: 100, Epochs: 10}, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Neighbors.K != 96 || r.Components > 8 || r.Options.Neighbors != 100 || r.Options.Target != 10000 {
		t.Fatal("effective defaults or caps lost")
	}
}

func TestExpressionAnalysisPopulationLimitsAndDegeneracy(t *testing.T) {
	in := &Input{Matrix: &Matrix{Barcodes: make([]string, 100001)}}
	if _, err := Analyze(context.Background(), in, AnalysisOptions{}, 1, nil); err == nil {
		t.Fatal("source population bound ignored")
	}
	m := &Matrix{Barcodes: make([]string, MaxAnalysisCells+1), GeneUMIs: make([]uint64, MaxAnalysisCells+1), Genes: make([]int, MaxAnalysisCells+1)}
	for i := range m.Barcodes {
		m.GeneUMIs[i] = 1
		m.Genes[i] = 1
	}
	if _, err := Analyze(context.Background(), &Input{Matrix: m}, AnalysisOptions{}, 1, nil); err == nil {
		t.Fatal("retained population bound ignored")
	}
	// Identical expression profiles contain no principal-component variation.
	m = &Matrix{Features: []Feature{{ID: "a", Type: "Gene Expression"}, {ID: "b", Type: "Gene Expression"}}, Indptr: []int{0}}
	for i := 0; i < 4; i++ {
		m.Barcodes = append(m.Barcodes, fmt.Sprint(i))
		m.GeneUMIs = append(m.GeneUMIs, 2)
		m.Genes = append(m.Genes, 2)
		m.Counts = append(m.Counts, 1, 1)
		m.Indices = append(m.Indices, 0, 1)
		m.Indptr = append(m.Indptr, len(m.Counts))
	}
	if _, err := Analyze(context.Background(), &Input{Matrix: m}, AnalysisOptions{}, 1, nil); err == nil {
		t.Fatal("constant expression was mapped")
	}
}
