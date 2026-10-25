// Package modularity tests within-clonotype connectivity on a fixed expression graph.
// The score follows scirpy's documented clonotype_modularity formula, not Newman Q.
// Population, binary-edge, analytic expectation and BY choices are ClonoLoom policies.
package modularity

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"

	"clonoloom/internal/dmath"
	"clonoloom/internal/expansion"
	"clonoloom/internal/graph"
	"clonoloom/internal/random"
)

const Version = 1
const ExactLimit = 10000
const MaxWork int64 = 500000000
const MaxCells = 10000

type Options struct {
	Permutations int    `json:"permutations"`
	Seed         uint64 `json:"seed"`
}

func (o Options) Normalize() (Options, error) {
	if o.Permutations == 0 {
		o.Permutations = 9999
	}
	if o.Seed == 0 {
		o.Seed = 20261024
	}
	if o.Permutations < 99 || o.Permutations > 99999 {
		return o, errors.New("choose 99 to 99999 Monte Carlo draws")
	}
	if o.Seed > 1<<53-1 {
		return o, errors.New("seed must be an exact positive JavaScript integer")
	}
	return o, nil
}

// Cell order is exactly the saved graph's row order; empty Clone excludes that vertex.
// Reason preserves the parent assignment status. Barcode identities are never normalized.
type Cell struct {
	Barcode string `json:"barcode"`
	Clone   string `json:"clone"`
	Reason  string `json:"reason,omitempty"`
}
type Bin struct {
	Edges int `json:"edges"`
	Count int `json:"count"`
}
type Null struct {
	Size      int     `json:"size"`
	Method    string  `json:"method"`
	Draws     int     `json:"draws"`
	Expected  float64 `json:"expected"`
	Histogram []Bin   `json:"histogram"`
}
type Clone struct {
	ID       string   `json:"id"`
	Cells    int      `json:"cells"`
	Observed int      `json:"observed"`
	Expected *float64 `json:"expected"`
	Score    *float64 `json:"score"`
	P        *float64 `json:"p"`
	Adjusted *float64 `json:"adjusted"`
	Tail     *int     `json:"tail"`
	Reason   string   `json:"reason,omitempty"`
}
type Result struct {
	Options    Options        `json:"options"`
	GraphCells int            `json:"graphCells"`
	GraphEdges int            `json:"graphEdges"`
	Eligible   int            `json:"eligible"`
	Edges      int            `json:"edges"`
	Tested     int            `json:"tested"`
	Cells      []Cell         `json:"cells"`
	Clones     []Clone        `json:"clones"`
	Nulls      []Null         `json:"nulls"`
	Correction string         `json:"correction"`
	Harmonic   float64        `json:"harmonic"`
	Rules      map[string]any `json:"rules"`
}

// ValidateGraph checks the undirected, positive fuzzy-union CSR contract before indexing.
// We deliberately discard weight magnitudes only AFTER validating symmetry and finite weights.
func ValidateGraph(g *graph.CSR) error {
	if g == nil || g.N < 1 || g.N > MaxCells || len(g.Offsets) != g.N+1 || g.Offsets[0] != 0 || len(g.Targets) != len(g.Weights) || len(g.Targets) > 2000000 || g.Offsets[g.N] != len(g.Targets) {
		return errors.New("invalid or oversized expression graph")
	}
	for i := 0; i < g.N; i++ {
		lo, hi := g.Offsets[i], g.Offsets[i+1]
		if lo < 0 || hi < lo || hi > len(g.Targets) {
			return errors.New("invalid graph offsets")
		}
		last := int32(-1)
		for k := lo; k < hi; k++ {
			j, w := g.Targets[k], g.Weights[k]
			if j < 0 || int(j) >= g.N || int(j) == i || j <= last || math.IsNaN(w) || math.IsInf(w, 0) || w <= 0 || w > 1 {
				return errors.New("invalid graph target or weight")
			}
			last = j
		}
	}
	for i := 0; i < g.N; i++ {
		for k := g.Offsets[i]; k < g.Offsets[i+1]; k++ {
			j := int(g.Targets[k])
			lo, hi := g.Offsets[j], g.Offsets[j+1]
			at := lo + sort.Search(hi-lo, func(x int) bool { return int(g.Targets[lo+x]) >= i })
			if at == hi || int(g.Targets[at]) != i || g.Weights[at] != g.Weights[k] {
				return errors.New("expression graph must be symmetric")
			}
		}
	}
	return nil
}

func Compute(ctx context.Context, g *graph.CSR, cells []Cell, o Options, progress func(float64)) (*Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var err error
	if o, err = o.Normalize(); err != nil {
		return nil, err
	}
	if err = ValidateGraph(g); err != nil {
		return nil, err
	}
	if len(cells) != g.N {
		return nil, errors.New("graph and barcode rows differ")
	}
	r := &Result{Options: o, GraphCells: g.N, GraphEdges: g.Edges(), Cells: append([]Cell(nil), cells...), Clones: []Clone{}, Nulls: []Null{}, Correction: "Benjamini–Yekutieli", Rules: map[string]any{
		"population": "uniquely assigned retained expression cells in one receptor sample; induced graph, including isolated vertices",
		"edges":      "one undirected binary edge for each positive saved fuzzy-union edge; no UMAP-coordinate distances or optimization pruning",
		"null":       "uniform size-matched vertex subsets, conditional on fixed graph and clone size; cells exchangeable within this sample",
		"score":      "log2((observed + 1) / (expected + 1)); expected = edges * size * (size - 1) / (eligible * (eligible - 1))",
		"tail":       "upper, inclusive ties; exhaustive tail / draws, Monte Carlo (tail + 1) / (draws + 1)",
		"family":     "all clones with 2 <= retained uniquely assigned cells < eligible cells, including zero observed edges",
		"exactLimit": ExactLimit, "maxWork": MaxWork, "random": "PCG, stream = clone size; independent uniform subsets with replacement across draws",
	}}
	seen := map[string]bool{}
	members := map[string][]int{}
	remap := make([]int, g.N)
	for i, c := range cells {
		if c.Barcode == "" || seen[c.Barcode] {
			return nil, errors.New("graph barcodes must be nonempty and unique")
		}
		seen[c.Barcode] = true
		remap[i] = -1
		if c.Clone != "" {
			remap[i] = r.Eligible
			members[c.Clone] = append(members[c.Clone], r.Eligible)
			r.Eligible++
		}
	}
	if r.Eligible < 2 {
		return nil, errors.New("modularity needs at least two uniquely assigned expression-graph cells")
	}
	adj := make([][]int, r.Eligible)
	maxDegree := 0
	for i, ni := range remap {
		if ni < 0 {
			continue
		}
		for k := g.Offsets[i]; k < g.Offsets[i+1]; k++ {
			nj := remap[g.Targets[k]]
			if nj >= 0 {
				adj[ni] = append(adj[ni], nj)
				if nj > ni {
					r.Edges++
				}
			}
		}
		maxDegree = max(maxDegree, len(adj[ni]))
	}
	ids := make([]string, 0, len(members))
	sizes := map[int]bool{}
	for id, v := range members {
		ids = append(ids, id)
		if len(v) >= 2 && len(v) < r.Eligible {
			sizes[len(v)] = true
		}
	}
	sort.Strings(ids)
	ordered := make([]int, 0, len(sizes))
	for s := range sizes {
		ordered = append(ordered, s)
	}
	sort.Ints(ordered)
	work := int64(0)
	for _, s := range ordered {
		draws := combinations(r.Eligible, s, ExactLimit)
		if draws > ExactLimit {
			draws = o.Permutations
		}
		work += int64(draws) * int64(4*s+min(2*r.Edges, s*maxDegree))
	}
	if work > MaxWork {
		return nil, fmt.Errorf("null work bound %d exceeds %d; reduce Monte Carlo draws or analyze a smaller predeclared population", work, MaxWork)
	}
	marked := make([]bool, r.Eligible)
	count := func(v []int) int {
		for _, i := range v {
			marked[i] = true
		}
		n := 0
		for _, i := range v {
			for _, j := range adj[i] {
				if j > i && marked[j] {
					n++
				}
			}
		}
		for _, i := range v {
			marked[i] = false
		}
		return n
	}
	bySize := map[int]int{}
	for si, s := range ordered {
		null, err := distribution(ctx, r.Eligible, s, o, count)
		if err != nil {
			return nil, err
		}
		null.Expected = float64(int64(r.Edges)*int64(s)*int64(s-1)) / float64(int64(r.Eligible)*int64(r.Eligible-1))
		bySize[s] = len(r.Nulls)
		r.Nulls = append(r.Nulls, null)
		if progress != nil {
			progress(float64(si+1) / float64(len(ordered)))
		}
	}
	ps := []float64{}
	tested := []int{}
	for _, id := range ids {
		v := members[id]
		c := Clone{ID: id, Cells: len(v), Observed: count(v)}
		switch {
		case c.Cells < 2:
			c.Reason = "Singleton in graph population"
		case c.Cells == r.Eligible:
			c.Reason = "Clone occupies entire graph population"
		default:
			n := r.Nulls[bySize[c.Cells]]
			expected := n.Expected
			score := dmath.Log((float64(c.Observed)+1)/(expected+1)) / dmath.Log(2)
			tail := 0
			for _, b := range n.Histogram {
				if b.Edges >= c.Observed {
					tail += b.Count
				}
			}
			c.Tail = &tail
			p := float64(tail) / float64(n.Draws)
			if n.Method == "Monte Carlo" {
				p = float64(tail+1) / float64(n.Draws+1)
			}
			c.Expected = &expected
			c.Score = &score
			c.P = &p
			ps = append(ps, p)
			tested = append(tested, len(r.Clones))
		}
		r.Clones = append(r.Clones, c)
	}
	r.Tested = len(ps)
	adjusted, h := AdjustBY(ps)
	r.Harmonic = h
	for i, j := range tested {
		v := adjusted[i]
		r.Clones[j].Adjusted = &v
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r, nil
}

// AdjustBY controls FDR under arbitrary dependence when the input p-values are valid.
// Applying the harmonic factor after BH is equivalent to BY's monotone step-up adjustment.
func AdjustBY(p []float64) ([]float64, float64) {
	h := 0.0
	for i := 1; i <= len(p); i++ {
		h += 1 / float64(i)
	}
	q := expansion.AdjustBH(p)
	for i := range q {
		q[i] = math.Min(1, q[i]*h)
	}
	return q, h
}
func combinations(n, k, limit int) int {
	k = min(k, n-k)
	v := int64(1)
	for i := 1; i <= k; i++ {
		v = v * int64(n-k+i) / int64(i)
		if v > int64(limit) {
			return limit + 1
		}
	}
	return int(v)
}
func distribution(ctx context.Context, n, s int, o Options, count func([]int) int) (Null, error) {
	out := Null{Size: s, Method: "Exhaustive", Draws: combinations(n, s, ExactLimit), Histogram: []Bin{}}
	hist := map[int]int{}
	v := make([]int, s)
	for i := range v {
		v[i] = i
	}
	if out.Draws <= ExactLimit {
		for draw := 0; draw < out.Draws; draw++ {
			if err := ctx.Err(); err != nil {
				return out, err
			}
			hist[count(v)]++
			j := s - 1
			for j >= 0 && v[j] == n-s+j {
				j--
			}
			if j >= 0 {
				v[j]++
				for k := j + 1; k < s; k++ {
					v[k] = v[k-1] + 1
				}
			}
		}
	} else {
		out.Method = "Monte Carlo"
		out.Draws = o.Permutations
		rng := random.New(o.Seed, uint64(s))
		pool := make([]int, n)
		swaps := make([]int, s)
		for i := range pool {
			pool[i] = i
		}
		for draw := 0; draw < out.Draws; draw++ {
			if err := ctx.Err(); err != nil {
				return out, err
			}
			// Partial Fisher–Yates, without replacement within a draw. Undo to make each draw
			// start from the same ordering; the independent stream is keyed only by size.
			for i := 0; i < s; i++ {
				j := i + rng.IntN(n-i)
				swaps[i] = j
				pool[i], pool[j] = pool[j], pool[i]
			}
			hist[count(pool[:s])]++
			for i := s - 1; i >= 0; i-- {
				j := swaps[i]
				pool[i], pool[j] = pool[j], pool[i]
			}
		}
	}
	keys := make([]int, 0, len(hist))
	for k := range hist {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	for _, k := range keys {
		out.Histogram = append(out.Histogram, Bin{k, hist[k]})
	}
	return out, nil
}
