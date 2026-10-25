package modularity

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"clonoloom/internal/graph"
)

func fixture(n int, edges [][2]int, labels []string) (*graph.CSR, []Cell) {
	g := &graph.CSR{N: n, Offsets: []int{0}}
	cells := make([]Cell, n)
	for i := 0; i < n; i++ {
		cells[i] = Cell{Barcode: fmt.Sprint(i), Clone: labels[i]}
		for j := 0; j < n; j++ {
			for _, e := range edges {
				if (e[0] == i && e[1] == j) || (e[1] == i && e[0] == j) {
					g.Targets = append(g.Targets, int32(j))
					g.Weights = append(g.Weights, .25)
				}
			}
		}
		g.Offsets = append(g.Offsets, len(g.Targets))
	}
	return g, cells
}
func TestKnownEdgesAndMissingness(t *testing.T) {
	g, c := fixture(8, [][2]int{{0, 1}, {0, 2}, {1, 2}, {3, 4}, {4, 5}, {0, 7}}, []string{"A", "A", "A", "B", "C", "B", "D", ""})
	c[7].Reason = "Ambiguous clonotype"
	r, err := Compute(context.Background(), g, c, Options{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Eligible != 7 || r.Edges != 5 || r.GraphEdges != 6 || r.Tested != 2 || r.Cells[7].Reason != "Ambiguous clonotype" {
		t.Fatalf("%+v", r)
	}
	a := r.Clones[0]
	if a.Observed != 3 || *a.Expected != 5.0/7 || *a.P != 1.0/35 || a.Cells != 3 {
		t.Fatalf("%+v", a)
	}
	if r.Clones[1].Observed != 0 || *r.Clones[1].P != 1 || r.Clones[2].P != nil || r.Clones[2].Score != nil || r.Clones[2].Tail != nil {
		t.Fatal(r.Clones)
	}
	for i := range g.Weights {
		g.Weights[i] = 1
	}
	again, err := Compute(context.Background(), g, c, Options{Seed: 42}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.Clones, again.Clones) || !reflect.DeepEqual(r.Nulls, again.Nulls) {
		t.Fatal("weights or seed altered exhaustive result")
	}
	for _, n := range r.Nulls {
		sum := 0
		for _, b := range n.Histogram {
			sum += b.Count
		}
		if sum != n.Draws {
			t.Fatal("histogram denominator")
		}
	}
}
func TestDegenerateGraphsAndUntestableClones(t *testing.T) {
	for _, edges := range [][][2]int{nil, {{0, 1}, {0, 2}, {0, 3}, {1, 2}, {1, 3}, {2, 3}}} {
		g, c := fixture(4, edges, []string{"A", "A", "B", "B"})
		r, err := Compute(context.Background(), g, c, Options{}, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, v := range r.Clones {
			if *v.P != 1 || *v.Score != 0 || *v.Adjusted != 1 {
				t.Fatal(v)
			}
		}
	}
	g, c := fixture(3, [][2]int{{0, 1}}, []string{"A", "A", "A"})
	r, err := Compute(context.Background(), g, c, Options{}, nil)
	if err != nil || r.Tested != 0 || r.Clones[0].Reason == "" || r.Clones[0].P != nil {
		t.Fatal(r, err)
	}
	for i := range c {
		c[i].Clone = fmt.Sprint(i)
	}
	r, err = Compute(context.Background(), g, c, Options{}, nil)
	if err != nil || r.Tested != 0 {
		t.Fatal(r, err)
	}
}
func TestModularityInvalidInputsAndCancellation(t *testing.T) {
	base, c := fixture(4, [][2]int{{0, 1}, {1, 2}}, []string{"A", "A", "B", "B"})
	for _, mutate := range []func(*graph.CSR){func(g *graph.CSR) { g.Offsets[2] = -1 }, func(g *graph.CSR) { g.Targets[0] = 0 }, func(g *graph.CSR) { g.Targets[0] = 99 }, func(g *graph.CSR) { g.Weights[0] = 0 }, func(g *graph.CSR) { g.Weights[0] = 2 }, func(g *graph.CSR) { g.Weights[0] = .5 }, func(g *graph.CSR) { g.N = MaxCells + 1 }, func(g *graph.CSR) { g.Targets = g.Targets[:1] }, func(g *graph.CSR) { g.Offsets[4]++ }} {
		raw, _ := json.Marshal(base)
		var g graph.CSR
		json.Unmarshal(raw, &g)
		mutate(&g)
		if _, err := Compute(context.Background(), &g, c, Options{}, nil); err == nil {
			t.Fatal("accepted malformed graph", g)
		}
	}
	duplicate := append([]Cell(nil), c...)
	duplicate[1].Barcode = duplicate[0].Barcode
	for _, v := range [][]Cell{c[:3], duplicate, {{Barcode: "1"}, {Barcode: "2"}, {Barcode: "3"}, {Barcode: "4"}}} {
		if _, err := Compute(context.Background(), base, v, Options{}, nil); err == nil {
			t.Fatal("accepted invalid population")
		}
	}
	for _, o := range []Options{{Permutations: 1}, {Permutations: 100000}, {Seed: 1 << 53}} {
		if _, err := o.Normalize(); err == nil {
			t.Fatal(o)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Compute(ctx, base, c, Options{}, nil); err != context.Canceled {
		t.Fatal(err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	calls := 0
	_, err := distribution(ctx, 20, 5, Options{Seed: 1, Permutations: 9999}, func([]int) int {
		calls++
		if calls == 20 {
			cancel()
		}
		return 0
	})
	if err != context.Canceled {
		t.Fatal("null cancellation", err)
	}
	// Bound is checked before any sampling, even with a dense valid graph.
	edges := [][2]int{}
	for i := 0; i < 100; i++ {
		for j := i + 1; j < 100; j++ {
			edges = append(edges, [2]int{i, j})
		}
	}
	labels := make([]string, 100)
	for i := range labels {
		if i < 50 {
			labels[i] = "A"
		} else {
			labels[i] = "B"
		}
	}
	g, c := fixture(100, edges, labels)
	if _, err := Compute(context.Background(), g, c, Options{Permutations: 99999}, nil); err == nil || !strings.Contains(err.Error(), "work bound") {
		t.Fatal(err)
	}
}
func TestModularityMonteCarloReproducible(t *testing.T) {
	edges := [][2]int{}
	for i := 0; i < 5; i++ {
		for j := i + 1; j < 5; j++ {
			edges = append(edges, [2]int{i, j})
		}
	}
	labels := make([]string, 20)
	for i := range labels {
		labels[i] = fmt.Sprint(i / 5)
	}
	g, c := fixture(20, edges, labels)
	r, err := Compute(context.Background(), g, c, Options{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Nulls) != 1 || r.Nulls[0].Method != "Monte Carlo" || r.Nulls[0].Draws != 9999 {
		t.Fatal(r.Nulls)
	}
	for _, v := range r.Clones {
		if v.Tail == nil || *v.P != float64(*v.Tail+1)/10000 || *v.P < .0001 {
			t.Fatal(v)
		}
	}
	raw, _ := json.Marshal(r)
	sum := sha256.Sum256(raw)
	digest := hex.EncodeToString(sum[:])
	const want = "1b327b45baa3c0973a0e6c4035f1a7ff3d0b6d622f4a36b9a29f13ea6d29227c"
	if digest != want {
		t.Fatalf("modularity digest %s", digest)
	}
}
