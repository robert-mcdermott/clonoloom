package embed

import (
	"clonoloom/internal/graph"
	"context"
	"math"
	"reflect"
	"testing"
)

func TestUMAPDeterministicFiniteAndCancellable(t *testing.T) {
	ctx := context.Background()
	nb, err := graph.KNN(ctx, []float64{0, 0, 1, 0, 0, 1, 1, 1}, 4, 2, 3, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	g, err := graph.Fuzzy(nb)
	if err != nil {
		t.Fatal(err)
	}
	weights := append([]float64(nil), g.Weights...)
	p := DefaultParams()
	p.Epochs = 20
	initial := make([]float64, 8) // coincident initialization still receives deterministic noise
	a, info, err := Layout(ctx, g, initial, p, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := Layout(ctx, g, initial, p, 4, nil)
	if err != nil || !reflect.DeepEqual(a, b) {
		t.Fatal("determinism", err)
	}
	if info.Init != "principal components" || info.Epochs != 20 || !reflect.DeepEqual(weights, g.Weights) {
		t.Fatal("provenance or graph mutation")
	}
	for i, v := range a {
		if math.IsNaN(v) || math.IsInf(v, 0) || initial[i] != 0 {
			t.Fatal("nonfinite or mutated input")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := Layout(ctx, g, initial, p, 1, nil); err != context.Canceled {
		t.Fatal(err)
	}
	p.MinDist = math.NaN()
	if _, _, err := Layout(context.Background(), g, initial, p, 1, nil); err == nil {
		t.Fatal("NaN minDist accepted")
	}
}
