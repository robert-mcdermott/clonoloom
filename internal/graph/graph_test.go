package graph

import (
	"context"
	"math"
	"reflect"
	"testing"
)

func TestExactNeighborsTiesAndWorkers(t *testing.T) {
	x := []float64{0, 0, 1, 0, 0, 1, 1, 1, 0, 0}
	a, err := KNN(context.Background(), x, 5, 2, 3, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := KNN(context.Background(), x, 5, 2, 3, 4, nil)
	if err != nil || !reflect.DeepEqual(a, b) {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a.Index[:3], []int32{4, 1, 2}) || a.Distances[0] != 0 || a.Distances[1] != 1 {
		t.Fatal(a)
	}
	g, err := Fuzzy(a)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < g.N; i++ {
		for e := g.Offsets[i]; e < g.Offsets[i+1]; e++ {
			j := g.Targets[e]
			w := g.Weights[e]
			if int(j) == i || w <= 0 || w > 1 || math.IsNaN(w) {
				t.Fatal("bad edge")
			}
			found := false
			for q := g.Offsets[j]; q < g.Offsets[j+1]; q++ {
				if g.Targets[q] == int32(i) && g.Weights[q] == w {
					found = true
				}
			}
			if !found {
				t.Fatal("asymmetric graph")
			}
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := KNN(ctx, x, 5, 2, 3, 2, nil); err != context.Canceled {
		t.Fatal(err)
	}
}
