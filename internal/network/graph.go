package network

// Connected components use disjoint sets (Tarjan, 1975, J ACM 22:215–225,
// doi:10.1145/321879.321884). Chain matching is ClonoLoom's choice: all requires
// a bijection of retained chains within each locus, any requires one matching
// chain. Thus missing matches missing only in all mode; extra chains are never
// silently discarded. The all distance is the minimum possible maximum, any
// the minimum. Components express sequence similarity, not proven specificity.

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"clonoloom/internal/clonotype"
	"clonoloom/internal/tcrdist"
)

type Options struct {
	Metric   string `json:"metric"`
	Cutoff   int    `json:"cutoff"`
	Arms     string `json:"arms"`
	SameV    bool   `json:"sameV"`
	SameJ    bool   `json:"sameJ"`
	MaxEdges int    `json:"maxEdges"`
}

func (o Options) Normalize() (Options, error) {
	if o.Metric == "" {
		o.Metric = "identity"
	}
	if o.Arms == "" {
		o.Arms = "all"
	}
	if o.Arms != "all" && o.Arms != "any" {
		return o, errors.New("network: chains must be all or any")
	}
	if o.Metric != "identity" && o.Metric != "hamming" && o.Metric != "levenshtein" && o.Metric != "tcrdist" {
		return o, errors.New("network: unknown distance metric")
	}
	limit := 2
	if o.Metric == "identity" {
		limit = 0
	}
	if o.Metric == "tcrdist" {
		limit = 1000
	}
	if o.Cutoff < 0 || o.Cutoff > limit {
		return o, fmt.Errorf("network: cutoff must be from 0 to %d", limit)
	}
	if o.MaxEdges == 0 {
		o.MaxEdges = 2_000_000
	}
	if o.MaxEdges < 1 || o.MaxEdges > 2_000_000 {
		return o, errors.New("network: edge budget must be from 1 to 2000000")
	}
	return o, nil
}

type Edge struct {
	A        int `json:"a"`
	B        int `json:"b"`
	Distance int `json:"distance"`
}

type Component struct {
	ID        int      `json:"id"`
	Nodes     []int    `json:"nodes"`
	Abundance float64  `json:"abundance"`
	Samples   []uint32 `json:"samples"`
	Subjects  []string `json:"subjects"`
}

type Graph struct {
	Edges      []Edge            `json:"edges"`
	Components []Component       `json:"components"`
	Membership []int             `json:"membership"`
	Excluded   map[string]int    `json:"excluded"`
	Alleles    map[string]string `json:"alleles,omitempty"`
	Organism   string            `json:"organism,omitempty"`
}

type pairKey struct{ a, b int }

func key(a, b int) pairKey {
	if a > b {
		a, b = b, a
	}
	return pairKey{a, b}
}

// Build keeps all input clonotypes as nodes, including unsupported TCRdist
// receptors as explicitly counted isolated nodes. Sample subjects are annotations,
// not inferred from sample names. On any budget or cancellation error no graph is returned.
func Build(ctx context.Context, table *clonotype.Table, o Options, loops *tcrdist.Loops, organism string, subjects map[uint32]string, progress func(int, int)) (*Graph, error) {
	var err error
	o, err = o.Normalize()
	if err != nil {
		return nil, err
	}
	g := &Graph{Edges: []Edge{}, Components: []Component{}, Membership: make([]int, len(table.Clonotypes)), Excluded: map[string]int{}, Alleles: map[string]string{}}
	if loops != nil && organism == "" {
		var calls []string
		for _, c := range table.Clonotypes {
			for _, ch := range c.Chains {
				calls = append(calls, ch.V)
			}
		}
		organism, _ = loops.Guess(calls)
	}
	g.Organism = organism
	unique := []clonotype.Chain{}
	index := map[clonotype.Chain]int{}
	owners := [][]int{}
	chains := make([][]int, len(table.Clonotypes))
	encoded := []tcrdist.Chain{}
	for n, c := range table.Clonotypes {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		valid := len(c.Chains) > 0
		reason := "no retained chain"
		for _, ch := range c.Chains {
			if ch.Junction == "" || len(ch.Junction) > 256 {
				return nil, fmt.Errorf("network: clonotype %s has an empty or overlong junction", c.ID)
			}
			for _, r := range ch.Junction {
				if r < 'A' || r > 'Z' {
					return nil, fmt.Errorf("network: clonotype %s has a non-uppercase junction", c.ID)
				}
			}
			if o.Metric == "tcrdist" {
				if ch.Locus != "TRA" && ch.Locus != "TRB" && ch.Locus != "TRG" && ch.Locus != "TRD" {
					valid = false
					reason = tcrdist.ReasonNotTCell
					break
				}
				if loops != nil {
					if _, _, ok := loops.Lookup(organism, ch.V); !ok {
						valid = false
						reason = tcrdist.ReasonNoLoops
						break
					}
				}
			}
		}
		if !valid {
			g.Excluded[reason]++
			continue
		}
		for _, ch := range c.Chains {
			id, ok := index[ch]
			if !ok {
				id = len(unique)
				index[ch] = id
				unique = append(unique, ch)
				owners = append(owners, nil)
				var ls []string
				if o.Metric == "tcrdist" && loops != nil {
					l, a, _ := loops.Lookup(organism, ch.V)
					ls = l[:]
					if a != ch.V {
						g.Alleles[ch.V] = a
					}
				}
				encoded = append(encoded, tcrdist.NewChain(ch.Junction, ls))
			}
			chains[n] = append(chains[n], id)
			// A repeated retained chain has multiplicity for matching, but one owner posting.
			list := owners[id]
			if len(list) == 0 || list[len(list)-1] != n {
				owners[id] = append(list, n)
			}
		}
	}
	distances := map[pairKey]int{}
	candidates := map[pairKey]struct{}{}
	add := func(a, b, d int) error {
		x, y := unique[a], unique[b]
		if x.Locus != y.Locus || (o.SameV && (x.V == "" || y.V == "" || x.V != y.V)) || (o.SameJ && (x.J == "" || y.J == "" || x.J != y.J)) {
			return nil
		}
		distances[key(a, b)] = d
		for _, i := range owners[a] {
			for _, j := range owners[b] {
				if err := ctx.Err(); err != nil {
					return err
				}
				if i == j {
					continue
				}
				candidates[key(i, j)] = struct{}{}
				if len(candidates) > 10_000_000 {
					return errors.New("network: candidate pairs exceed 10000000; use a smaller dataset or cutoff")
				}
			}
		}
		return nil
	}
	byLocus := map[string][]int{}
	var loci []string
	for i, ch := range unique {
		if _, ok := byLocus[ch.Locus]; !ok {
			loci = append(loci, ch.Locus)
		}
		byLocus[ch.Locus] = append(byLocus[ch.Locus], i)
		if err := add(i, i, 0); err != nil {
			return nil, err
		}
	}
	sort.Strings(loci)
	for _, locus := range loci {
		ids := byLocus[locus]
		if o.Metric == "tcrdist" {
			// Length buckets avoid comparing impossible lengths; loop distances are a
			// second exact lower bound before the searched-gap CDR3 computation.
			buckets := map[int][]int{}
			for _, id := range ids {
				buckets[len(unique[id].Junction)] = append(buckets[len(unique[id].Junction)], id)
			}
			for done, b := range ids {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				length := len(unique[b].Junction)
				for l := max(1, length-o.Cutoff/12); l <= min(256, length+o.Cutoff/12); l++ {
					for _, a := range buckets[l] {
						if a >= b {
							continue
						}
						delta := length - l
						if delta < 0 {
							delta = -delta
						}
						ld := tcrdist.LoopDistance(encoded[a], encoded[b])
						if ld+12*delta > o.Cutoff {
							continue
						}
						d := ld + tcrdist.CDR3Distance(encoded[a], encoded[b])
						if d <= o.Cutoff {
							if err := add(a, b, d); err != nil {
								return nil, err
							}
						}
					}
				}
				if progress != nil && done%256 == 0 {
					progress(done, len(ids))
				}
			}
		} else {
			// Index unique junctions once even when several gene calls retain
			// separate chains with that sequence. Expand only actual neighbors.
			seqs := []string{}
			groups := [][]int{}
			at := map[string]int{}
			for _, id := range ids {
				seq := unique[id].Junction
				i, ok := at[seq]
				if !ok {
					i = len(seqs)
					at[seq] = i
					seqs = append(seqs, seq)
					groups = append(groups, nil)
				}
				for _, a := range groups[i] {
					if err := add(a, id, 0); err != nil {
						return nil, err
					}
				}
				groups[i] = append(groups[i], id)
			}
			err := SequencePairs(ctx, seqs, Search{Metric: o.Metric, Cutoff: o.Cutoff}, func(p Pair) error {
				for _, a := range groups[p.A] {
					for _, b := range groups[p.B] {
						if err := add(a, b, p.Distance); err != nil {
							return err
						}
					}
				}
				return nil
			})
			if err != nil {
				return nil, err
			}
		}
	}
	ordered := make([]pairKey, 0, len(candidates))
	for p := range candidates {
		ordered = append(ordered, p)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].a != ordered[j].a {
			return ordered[i].a < ordered[j].a
		}
		return ordered[i].b < ordered[j].b
	})
	parent := make([]int, len(chains))
	for i := range parent {
		parent[i] = i
	}
	var root func(int) int
	root = func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}
		return i
	}
	for _, p := range ordered {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		d, ok := match(chains[p.a], chains[p.b], distances, o.Arms, o.Cutoff)
		if !ok {
			continue
		}
		if len(g.Edges) >= o.MaxEdges {
			return nil, errors.New("network: edge budget exceeded; use a smaller dataset or cutoff")
		}
		g.Edges = append(g.Edges, Edge{p.a, p.b, d})
		a, b := root(p.a), root(p.b)
		if a != b {
			if a > b {
				a, b = b, a
			}
			parent[b] = a
		}
	}
	groups := map[int][]int{}
	for i := range parent {
		r := root(i)
		groups[r] = append(groups[r], i)
	}
	for _, nodes := range groups {
		c := Component{Nodes: nodes, Samples: []uint32{}, Subjects: []string{}}
		sm := map[uint32]bool{}
		su := map[string]bool{}
		for _, n := range nodes {
			c.Abundance += table.Clonotypes[n].Total
			if len(table.Start) > n+1 {
				ss, _ := table.Abundances(n)
				for _, s := range ss {
					sm[s] = true
					if subjects[s] != "" {
						su[subjects[s]] = true
					}
				}
			}
		}
		for s := range sm {
			c.Samples = append(c.Samples, s)
		}
		sort.Slice(c.Samples, func(i, j int) bool { return c.Samples[i] < c.Samples[j] })
		for s := range su {
			c.Subjects = append(c.Subjects, s)
		}
		sort.Strings(c.Subjects)
		g.Components = append(g.Components, c)
	}
	sort.Slice(g.Components, func(i, j int) bool {
		a, b := g.Components[i], g.Components[j]
		if len(a.Nodes) != len(b.Nodes) {
			return len(a.Nodes) > len(b.Nodes)
		}
		return a.Nodes[0] < b.Nodes[0]
	})
	for i := range g.Components {
		g.Components[i].ID = i + 1
		for _, n := range g.Components[i].Nodes {
			g.Membership[n] = i + 1
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return g, nil
}

// Bipartite augmenting paths test a bijection at each possible bottleneck.
// This handles every retained chain without exponential permutation enumeration.
func match(a, b []int, ds map[pairKey]int, arms string, cutoff int) (int, bool) {
	best := cutoff + 1
	for _, x := range a {
		for _, y := range b {
			if d, ok := ds[key(x, y)]; ok {
				best = min(best, d)
			}
		}
	}
	if arms == "any" {
		return best, best <= cutoff
	}
	if len(a) == 0 || len(a) != len(b) || best > cutoff {
		return 0, false
	}
	values := []int{}
	for _, x := range a {
		for _, y := range b {
			if d, ok := ds[key(x, y)]; ok {
				values = append(values, d)
			}
		}
	}
	sort.Ints(values)
	previous := -1
	for _, threshold := range values {
		if threshold == previous {
			continue
		}
		previous = threshold
		assigned := make([]int, len(b))
		for i := range assigned {
			assigned[i] = -1
		}
		var augment func(int, []bool) bool
		augment = func(i int, seen []bool) bool {
			for j, y := range b {
				d, ok := ds[key(a[i], y)]
				if !ok || d > threshold || seen[j] {
					continue
				}
				seen[j] = true
				if assigned[j] < 0 || augment(assigned[j], seen) {
					assigned[j] = i
					return true
				}
			}
			return false
		}
		ok := true
		for i := range a {
			if !augment(i, make([]bool, len(b))) {
				ok = false
				break
			}
		}
		if ok {
			return threshold, true
		}
	}
	return 0, false
}
