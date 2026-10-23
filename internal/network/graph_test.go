package network

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"clonoloom/internal/clonotype"
	"clonoloom/internal/random"
	"clonoloom/internal/tcrdist"
)

func chain(locus, junction string) clonotype.Chain {
	return clonotype.Chain{Locus: locus, Junction: junction, V: locus + "V1", J: locus + "J1"}
}
func testTable(cs ...[]clonotype.Chain) *clonotype.Table {
	t := &clonotype.Table{Start: []int32{0}}
	for i, c := range cs {
		t.Clonotypes = append(t.Clonotypes, clonotype.Clonotype{ID: fmt.Sprint(i), Chains: c, Total: float64(i + 1), Samples: 1})
		t.SampleOf = append(t.SampleOf, uint32(i%2))
		t.Abundance = append(t.Abundance, float64(i+1))
		t.Start = append(t.Start, int32(i+1))
	}
	return t
}
func TestGraphMatching(t *testing.T) {
	a, b, c := chain("TRA", "CAAF"), chain("TRB", "CBBF"), chain("TRB", "CBCF")
	table := testTable([]clonotype.Chain{a, b}, []clonotype.Chain{a, c}, []clonotype.Chain{b}, []clonotype.Chain{b}, []clonotype.Chain{a, b, c}, []clonotype.Chain{c, a, b})
	for _, tc := range []struct {
		arms  string
		cut   int
		edges int
	}{{"all", 0, 2}, {"all", 1, 3}, {"any", 0, 13}, {"any", 1, 15}} {
		g, err := Build(context.Background(), table, Options{Metric: "hamming", Cutoff: tc.cut, Arms: tc.arms}, nil, "", map[uint32]string{0: "A", 1: "B"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(g.Edges) != tc.edges {
			t.Errorf("%+v: edges %+v", tc, g.Edges)
		}
		if tc.arms == "any" && tc.cut == 1 {
			if len(g.Components) != 1 || g.Components[0].Abundance != 21 || len(g.Components[0].Subjects) != 2 {
				t.Fatal(g.Components)
			}
		}
	}
	// Same genes cannot be satisfied by missing names.
	altered := a
	altered.V = ""
	g, err := Build(context.Background(), testTable([]clonotype.Chain{altered}, []clonotype.Chain{altered}), Options{SameV: true}, nil, "", nil, nil)
	if err != nil || len(g.Edges) != 0 {
		t.Fatalf("missing genes: %v %v", g, err)
	}
	// Identical junctions at different loci do not connect.
	altered = a
	altered.Locus = "TRB"
	g, err = Build(context.Background(), testTable([]clonotype.Chain{a}, []clonotype.Chain{altered}), Options{}, nil, "", nil, nil)
	if err != nil || len(g.Edges) != 0 {
		t.Fatal(g, err)
	}
}
func TestGraphBoundsAndCancellation(t *testing.T) {
	ch := chain("TRB", "CASSLGGNTEAFF")
	table := testTable([]clonotype.Chain{ch}, []clonotype.Chain{ch}, []clonotype.Chain{ch})
	if g, e := Build(context.Background(), table, Options{MaxEdges: 1}, nil, "", nil, nil); e == nil || g != nil {
		t.Fatal("partial graph on budget failure")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if g, e := Build(ctx, table, Options{}, nil, "", nil, nil); e != context.Canceled || g != nil {
		t.Fatal(g, e)
	}
	for _, o := range []Options{{Metric: "bad"}, {Arms: "bad"}, {Cutoff: 1}, {Metric: "hamming", Cutoff: 3}, {MaxEdges: -1}} {
		if _, e := o.Normalize(); e == nil {
			t.Fatal(o)
		}
	}
}
func TestGraphTCRdistAndLayout(t *testing.T) {
	seqs := []string{"CASSLGGNTEAFF", "CASSLGGATEAFF", "CASSPRQGAYEQYF", "CASSLGGNTEAF", "CASSLGGNTEAFFF"}
	cs := [][]clonotype.Chain{}
	for _, s := range seqs {
		cs = append(cs, []clonotype.Chain{chain("TRB", s)})
	}
	table := testTable(cs...)
	g, err := Build(context.Background(), table, Options{Metric: "tcrdist", Cutoff: 24}, nil, "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var want []Edge
	for i := range seqs {
		for j := i + 1; j < len(seqs); j++ {
			d := tcrdist.ChainDistance(tcrdist.NewChain(seqs[i], nil), tcrdist.NewChain(seqs[j], nil))
			if d <= 24 {
				want = append(want, Edge{i, j, d})
			}
		}
	}
	if !reflect.DeepEqual(want, g.Edges) {
		t.Fatal(want, g.Edges)
	}
	d, err := Layout(context.Background(), g, 1000)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(d)
	digest := fmt.Sprintf("%x", sha256.Sum256(raw))
	t.Log("layout", digest)
	const pinned = "5ffdc30d99a8b5cf328eaf332ccb7c8fb1a36373e7bf82969a9a64ba472aeab5"
	if digest != pinned {
		t.Fatalf("layout changed: %s", digest)
	}
	limited, err := Layout(context.Background(), g, 2)
	if err != nil || len(limited.Points) != 2 || limited.Omitted != 3 {
		t.Fatal(limited, err)
	}
}

// Full graph plus drawing budget on reproducible receptor-like sequences. The
// 100,000-node graph is sparse; dense graphs must explicitly fail their budget.
func BenchmarkGraph100000(b *testing.B) {
	r := random.New(27, 0)
	cs := make([][]clonotype.Chain, 100000)
	for i := range cs {
		s := []byte("CASSAAAAAAAAAFF")
		for j := 4; j < len(s)-2; j++ {
			s[j] = "ACDEFGHIKLMNPQRSTVWY"[r.IntN(20)]
		}
		cs[i] = []clonotype.Chain{chain("TRB", string(s))}
	}
	table := testTable(cs...)
	for _, metric := range []string{"identity", "hamming", "levenshtein", "tcrdist"} {
		b.Run(metric, func(b *testing.B) {
			cut := 2
			if metric == "identity" {
				cut = 0
			}
			if metric == "tcrdist" {
				cut = 24
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				g, e := Build(context.Background(), table, Options{Metric: metric, Cutoff: cut}, nil, "", nil, nil)
				if e != nil {
					b.Fatal(e)
				}
				if _, e := Layout(context.Background(), g, 1000); e != nil {
					b.Fatal(e)
				}
			}
		})
	}
}

// Independently enumerate pairs and permutations on small random multichain
// tables. This checks the indexed candidates and bottleneck matcher together.
func TestGraphAgainstEveryPair(t *testing.T) {
	r := random.New(711, 0)
	for _, metric := range []string{"identity", "hamming", "levenshtein", "tcrdist"} {
		for trial := 0; trial < 10; trial++ {
			cs := make([][]clonotype.Chain, 20)
			for i := range cs {
				for j := 0; j < 1+r.IntN(3); j++ {
					s := []byte("CAAAAAF")
					for k := 1; k < 6; k++ {
						s[k] = "ACGT"[r.IntN(4)]
					}
					locus := []string{"TRA", "TRB"}[r.IntN(2)]
					c := chain(locus, string(s))
					if r.IntN(2) == 0 {
						c.V += "X"
					}
					cs[i] = append(cs[i], c)
				}
			}
			for _, arms := range []string{"all", "any"} {
				o := Options{Metric: metric, Cutoff: 2, Arms: arms, SameV: trial%2 == 0}
				if metric == "identity" {
					o.Cutoff = 0
				}
				if metric == "tcrdist" {
					o.Cutoff = 12
				}
				g, err := Build(context.Background(), testTable(cs...), o, nil, "", nil, nil)
				if err != nil {
					t.Fatal(err)
				}
				want := []Edge{}
				distance := func(a, b clonotype.Chain) int {
					if a.Locus != b.Locus || (o.SameV && a.V != b.V) {
						return 10000
					}
					switch metric {
					case "identity":
						if a.Junction != b.Junction {
							return 10000
						}
						return 0
					case "hamming":
						d := 0
						if len(a.Junction) != len(b.Junction) {
							return 10000
						}
						for i := range a.Junction {
							if a.Junction[i] != b.Junction[i] {
								d++
							}
						}
						return d
					case "levenshtein":
						return editReference(a.Junction, b.Junction)
					default:
						return tcrdist.ChainDistance(tcrdist.NewChain(a.Junction, nil), tcrdist.NewChain(b.Junction, nil))
					}
				}
				for a := range cs {
					for b := a + 1; b < len(cs); b++ {
						best := 10000
						if arms == "any" {
							for _, x := range cs[a] {
								for _, y := range cs[b] {
									best = min(best, distance(x, y))
								}
							}
						} else if len(cs[a]) == len(cs[b]) {
							used := make([]bool, len(cs[b]))
							var permutations func(int, int)
							permutations = func(i, d int) {
								if i == len(cs[a]) {
									best = min(best, d)
									return
								}
								for j, y := range cs[b] {
									if !used[j] {
										used[j] = true
										permutations(i+1, max(d, distance(cs[a][i], y)))
										used[j] = false
									}
								}
							}
							permutations(0, 0)
						}
						if best <= o.Cutoff {
							want = append(want, Edge{a, b, best})
						}
					}
				}
				if !reflect.DeepEqual(want, g.Edges) {
					t.Fatalf("%s %s trial %d: got %v want %v", metric, arms, trial, g.Edges, want)
				}
			}
		}
	}
}
