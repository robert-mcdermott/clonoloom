package bcell

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func TestLineageExactTopologiesAndAncestralStates(t *testing.T) {
	for n, want := range []int{0, 1, 1, 1, 3, 15, 105, 945, 10395} {
		if n == 0 {
			continue
		}
		seqs := make([]string, n)
		for i := range seqs {
			seqs[i] = "A"
		}
		r, err := searchTree(context.Background(), seqs, false)
		if err != nil || r.examined != want || r.optima != want || r.score != 0 || !r.exact {
			t.Fatal(n, r, err)
		}
	}
	sequences := []string{"AAAA", "ACCG", "ACCA", "GCCA", "GTCG"}
	r, err := searchTree(context.Background(), sequences, false)
	if err != nil {
		t.Fatal(err)
	}
	chosen, possible, edges := ancestralStates(r.graph, sequences)
	total := 0
	for _, v := range edges {
		total += v
	}
	if total != r.score {
		t.Fatal(total, r.score)
	}
	// Enumerate all 4^(n-2) internal assignments per site, independently of both
	// Fitch and the inside/outside recurrence. Compare every marginal state set.
	for col := range sequences[0] {
		best := 1000
		masks := make([]int, len(r.graph))
		assignment := make([]byte, len(r.graph))
		for i, s := range sequences {
			assignment[i] = s[col]
		}
		var visit func(int)
		visit = func(u int) {
			if u == len(r.graph) {
				cost := 0
				for _, edge := range r.graph.edges() {
					if assignment[edge[0]] != assignment[edge[1]] {
						cost++
					}
				}
				if cost < best {
					best = cost
					clear(masks)
				}
				if cost == best {
					for k, b := range assignment {
						masks[k] |= 1 << strings.IndexByte("ACGT", b)
					}
				}
				return
			}
			for _, b := range []byte("ACGT") {
				assignment[u] = b
				visit(u + 1)
			}
		}
		visit(len(sequences))
		for u, mask := range masks {
			if possible[u][col] != "?ACMGRSVTWYHKDBN"[mask] {
				t.Fatalf("node %d col %d %s mask %d", u, col, possible[u], mask)
			}
		}
	}
	for i, s := range sequences {
		if chosen[i] != s || possible[i] != s {
			t.Fatal(i, chosen, possible)
		}
	}
	again, _ := searchTree(context.Background(), sequences, false)
	if r.graph.key(len(sequences)) != again.graph.key(len(sequences)) {
		t.Fatal("nondeterministic")
	}
}
func TestLineageHeuristicAndCancellation(t *testing.T) {
	sequences := []string{"AAAAAA", "CAAAAA", "CCAAAA", "CCCAAA", "CCCCAA", "CCCCCA", "CCCCCC", "GCCCCC", "GGCCCC"}
	r, err := searchTree(context.Background(), sequences, false)
	if err != nil || r.exact || r.score != 8 || r.examined <= 0 || r.limited {
		t.Fatal(r, err)
	}
	again, _ := searchTree(context.Background(), sequences, false)
	if !reflect.DeepEqual(r, again) {
		t.Fatal("heuristic differs")
	}
	// Compare heuristic with the global optimum on an analytically small panel.
	small := sequences[:7]
	a, _ := searchTree(context.Background(), small, false)
	b, _ := searchTree(context.Background(), small, true)
	if a.score != b.score {
		t.Fatal(a.score, b.score)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := searchTree(ctx, sequences, false); err != context.Canceled {
		t.Fatal(err)
	}
	if _, err := searchTree(ctx, small, false); err != context.Canceled {
		t.Fatal(err)
	}
}
