package bcell

// Equal-cost nucleotide parsimony: Fitch 1971, doi:10.1093/sysbio/20.4.406;
// conditional ancestral reconstruction: Sankoff 1975, doi:10.1137/0128004.
// Tree enumeration, deterministic addition/NNI search and tie policies are
// ClonoLoom choices, documented in research.md §5.19. No external source used.
import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
)

const LineageExactTaxa = 8
const LineageMaxTaxa = 64
const LineageSearchBudget = 50000
const LineageWorkBudget = 200000000

var errLineageWork = errors.New("tree search exceeds 200 million pattern-node work units; use a smaller family or panel")

type treeGraph [][]int

func (g treeGraph) copy() treeGraph {
	out := make(treeGraph, len(g))
	for i := range g {
		out[i] = append([]int{}, g[i]...)
	}
	return out
}
func (g treeGraph) connect(a, b int) { g[a] = append(g[a], b); g[b] = append(g[b], a) }
func (g treeGraph) disconnect(a, b int) {
	for i, v := range g[a] {
		if v == b {
			g[a] = append(g[a][:i], g[a][i+1:]...)
			break
		}
	}
	for i, v := range g[b] {
		if v == a {
			g[b] = append(g[b][:i], g[b][i+1:]...)
			break
		}
	}
}
func (g treeGraph) edges() [][2]int {
	var out [][2]int
	for u := range g {
		for _, v := range g[u] {
			if u < v {
				out = append(out, [2]int{u, v})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { a, b := out[i], out[j]; return a[0] < b[0] || a[0] == b[0] && a[1] < b[1] })
	return out
}
func (g treeGraph) insert(edge [2]int, tip int) treeGraph {
	out := g.copy()
	u, v := edge[0], edge[1]
	mid := len(out)
	out = append(out, nil)
	out.disconnect(u, v)
	out.connect(u, mid)
	out.connect(v, mid)
	out.connect(mid, tip)
	return out
}
func (g treeGraph) rooted() (parent, order []int) {
	parent = make([]int, len(g))
	for i := range parent {
		parent[i] = -2
	}
	parent[0] = -1
	order = []int{0}
	for at := 0; at < len(order); at++ {
		u := order[at]
		for _, v := range g[u] {
			if v != parent[u] {
				parent[v] = u
				order = append(order, v)
			}
		}
	}
	return
}
func (g treeGraph) key(taxa int) string {
	var visit func(int, int) string
	visit = func(u, p int) string {
		if u < taxa && u != 0 {
			return fmt.Sprintf("t%02d", u)
		}
		var children []string
		for _, v := range g[u] {
			if v != p {
				children = append(children, visit(v, u))
			}
		}
		sort.Strings(children)
		return "(" + strings.Join(children, ",") + ")"
	}
	return visit(0, -1)
}
func (g treeGraph) clades(taxa int) []uint64 {
	p, order := g.rooted()
	bits := make([]uint64, len(g))
	for i := len(order) - 1; i >= 0; i-- {
		u := order[i]
		if u < taxa {
			bits[u] = 1 << u
		}
		for _, v := range g[u] {
			if v != p[u] {
				bits[u] |= bits[v]
			}
		}
	}
	return bits
}

type sitePattern struct {
	states []uint8
	weight int
}

func treePatterns(sequences []string) []sitePattern {
	by := map[string]int{}
	out := []sitePattern{}
	for col := range sequences[0] {
		states := make([]uint8, len(sequences))
		same := true
		for i, s := range sequences {
			states[i] = 1 << strings.IndexByte("ACGT", s[col])
			if i > 0 && states[i] != states[0] {
				same = false
			}
		}
		if same {
			continue
		}
		key := string(states)
		if i, ok := by[key]; ok {
			out[i].weight++
		} else {
			by[key] = len(out)
			out = append(out, sitePattern{states, 1})
		}
	}
	return out
}
func treeScore(g treeGraph, patterns []sitePattern) int {
	parent, order := g.rooted()
	sets := make([]uint8, len(g))
	score := 0
	for _, site := range patterns {
		cost := 0
		for i := len(order) - 1; i >= 0; i-- {
			u := order[i]
			var set uint8
			if u < len(site.states) {
				set = site.states[u]
			}
			for _, v := range g[u] {
				if v == parent[u] {
					continue
				}
				if set == 0 {
					set = sets[v]
				} else if shared := set & sets[v]; shared != 0 {
					set = shared
				} else {
					set |= sets[v]
					cost++
				}
			}
			sets[u] = set
		}
		score += cost * site.weight
	}
	return score
}

type treeSearch struct {
	graph                   treeGraph
	score, examined, optima int
	exact, limited          bool
	support                 map[uint64]int
}

func searchTree(ctx context.Context, sequences []string, forceHeuristic bool) (treeSearch, error) {
	n := len(sequences)
	out := treeSearch{score: 1 << 30, exact: n <= 2 || n <= LineageExactTaxa && !forceHeuristic, support: map[uint64]int{}}
	patterns := treePatterns(sequences)
	budget := min(LineageSearchBudget, LineageWorkBudget/(max(1, len(patterns))*max(1, 2*n-2)))
	needed := 1
	if out.exact {
		for i := 3; i <= n; i++ {
			needed *= 2*i - 5
		}
	} else {
		needed = 3 * (n - 2) * (n - 2)
	}
	if needed > budget {
		return out, errLineageWork
	}
	base := make(treeGraph, n)
	if n > 1 {
		base.connect(0, 1)
	}
	bestKey := ""
	consider := func(g treeGraph) {
		score := treeScore(g, patterns)
		out.examined++
		if score < out.score {
			out.score = score
			out.optima = 0
			out.support = map[uint64]int{}
			bestKey = ""
		}
		if score != out.score {
			return
		}
		out.optima++
		key := g.key(n)
		if bestKey == "" || key < bestKey {
			out.graph = g.copy()
			bestKey = key
		}
		if out.exact {
			for _, clade := range g.clades(n) {
				out.support[clade]++
			}
		}
	}
	if out.exact {
		var enumerate func(treeGraph, int) error
		enumerate = func(g treeGraph, next int) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if next >= n {
				consider(g)
				return nil
			}
			for _, edge := range g.edges() {
				if err := enumerate(g.insert(edge, next), next+1); err != nil {
					return err
				}
			}
			return nil
		}
		if err := enumerate(base, 2); err != nil {
			return out, err
		}
		return out, nil
	}
	// Three reproducible orders, then strict-improvement NNI. Equal-score plateaus
	// are not traversed. Exhausting the budget is explicit, never a global optimum.
	orders := [][]int{}
	for mode := 0; mode < 3; mode++ {
		var order []int
		for i := 1; i < n; i++ {
			order = append(order, i)
		}
		if mode == 1 {
			sort.Sort(sort.Reverse(sort.IntSlice(order)))
		}
		if mode == 2 {
			sort.SliceStable(order, func(i, j int) bool {
				d := func(k int) int {
					v := 0
					for c := range sequences[0] {
						if sequences[k][c] != sequences[0][c] {
							v++
						}
					}
					return v
				}
				a, b := d(order[i]), d(order[j])
				return a > b || a == b && order[i] < order[j]
			})
		}
		orders = append(orders, order)
	}
	used := 0
	for _, order := range orders {
		if used+(n-2)*(n-2) > budget {
			out.limited = true
			break
		}
		if err := ctx.Err(); err != nil {
			return out, err
		}
		g := make(treeGraph, n)
		g.connect(0, order[0])
		for _, tip := range order[1:] {
			if err := ctx.Err(); err != nil {
				return out, err
			}
			best := 1 << 30
			key := ""
			var chosen treeGraph
			for _, edge := range g.edges() {
				candidate := g.insert(edge, tip)
				score := treeScore(candidate, patterns)
				used++
				k := candidate.key(n)
				if score < best || score == best && (key == "" || k < key) {
					chosen, best, key = candidate, score, k
				}
			}
			g = chosen
		}
		consider(g)
		for {
			if err := ctx.Err(); err != nil {
				return out, err
			}
			score := treeScore(g, patterns)
			best := score
			key := ""
			var next treeGraph
			for _, edge := range g.edges() {
				u, v := edge[0], edge[1]
				if u < n || v < n {
					continue
				}
				left, right := []int{}, []int{}
				for _, a := range g[u] {
					if a != v {
						left = append(left, a)
					}
				}
				for _, b := range g[v] {
					if b != u {
						right = append(right, b)
					}
				}
				sort.Ints(left)
				sort.Ints(right)
				for _, b := range right {
					if used >= budget {
						out.limited = true
						break
					}
					candidate := g.copy()
					a := left[0]
					candidate.disconnect(u, a)
					candidate.disconnect(v, b)
					candidate.connect(u, b)
					candidate.connect(v, a)
					cost := treeScore(candidate, patterns)
					used++
					k := candidate.key(n)
					if cost < best || cost == best && cost < score && (key == "" || k < key) {
						next, best, key = candidate, cost, k
					}
				}
				if out.limited {
					break
				}
			}
			if next == nil {
				break
			}
			g = next
			consider(g)
			if out.limited {
				break
			}
		}
		if out.limited {
			break
		}
	}
	out.examined = used
	out.optima = 0
	out.support = nil
	return out, nil
}

// Inside/outside costs give every state compatible with a global optimum on
// this selected topology. Marginal ambiguity strings are NOT a joint assignment.
// The chosen sequences are one consistent optimum, parent state preferred then A/C/G/T.
func ancestralStates(g treeGraph, sequences []string) (chosen, possible []string, lengths []int) {
	n := len(g)
	taxa := len(sequences)
	parent, order := g.rooted()
	out, sets := make([][]byte, n), make([][]byte, n)
	lengths = make([]int, n)
	for i := range out {
		out[i] = make([]byte, len(sequences[0]))
		sets[i] = make([]byte, len(sequences[0]))
	}
	const inf = 1000000
	const iupac = "?ACMGRSVTWYHKDBN" // bit masks A=1,C=2,G=4,T=8.
	down, up := make([][4]int, n), make([][4]int, n)
	for col := range sequences[0] {
		local := func(u, s int) int {
			if u < taxa && sequences[u][col] != "ACGT"[s] {
				return inf
			}
			return 0
		}
		for i := len(order) - 1; i >= 0; i-- {
			u := order[i]
			for a := 0; a < 4; a++ {
				cost := local(u, a)
				for _, v := range g[u] {
					if v == parent[u] {
						continue
					}
					best := inf
					for b := 0; b < 4; b++ {
						x := down[v][b]
						if a != b {
							x++
						}
						best = min(best, x)
					}
					cost += best
				}
				down[u][a] = cost
			}
		}
		up[0] = [4]int{}
		opt := inf
		for _, v := range down[0] {
			opt = min(opt, v)
		}
		for _, u := range order {
			mask := 0
			for a := 0; a < 4; a++ {
				if down[u][a]+up[u][a] == opt {
					mask |= 1 << a
				}
			}
			sets[u][col] = iupac[mask]
			state := -1
			best := inf
			for a := 0; a < 4; a++ {
				cost := down[u][a]
				if parent[u] >= 0 && "ACGT"[a] != out[parent[u]][col] {
					cost++
				}
				if cost < best {
					best, state = cost, a
				}
			}
			if parent[u] >= 0 {
				a := strings.IndexByte("ACGT", out[parent[u]][col])
				if down[u][a] == best {
					state = a
				}
			}
			out[u][col] = "ACGT"[state]
			if parent[u] >= 0 && out[u][col] != out[parent[u]][col] {
				lengths[u]++
			}
			for _, v := range g[u] {
				if v == parent[u] {
					continue
				}
				for b := 0; b < 4; b++ {
					best := inf
					for a := 0; a < 4; a++ {
						cost := up[u][a] + local(u, a)
						if a != b {
							cost++
						}
						for _, w := range g[u] {
							if w == parent[u] || w == v {
								continue
							}
							small := inf
							for c := 0; c < 4; c++ {
								x := down[w][c]
								if a != c {
									x++
								}
								small = min(small, x)
							}
							cost += small
						}
						best = min(best, cost)
					}
					up[v][b] = best
				}
			}
		}
	}
	for i := range out {
		chosen = append(chosen, string(out[i]))
		possible = append(possible, string(sets[i]))
	}
	return
}
