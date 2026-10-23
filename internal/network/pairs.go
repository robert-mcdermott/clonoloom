// Package network finds sequence neighbors for clonotype networks (clonoloom-spec/requirements.md,
// S4). Identity compares whole junctions; Hamming counts substitutions between equal lengths;
// Levenshtein counts substitutions, insertions and deletions, each of cost one (Levenshtein 1966,
// Soviet Physics Doklady 10:707–710). These distances include the junction's conserved anchors.
// They are distinct from TCRdist's trimmed, weighted CDR3 distance.
// The edit-distance recurrence is Wagner and Fischer (1974, J ACM 21:168–173,
// doi:10.1145/321796.321811); indexing and bounds are ClonoLoom's choices (research.md §5.10).
package network

import (
	"context"
	"errors"
	"fmt"
	"sort"
)

// Pair is an unordered pair of input sequences (A < B) and their exact distance.
type Pair struct {
	A, B, Distance int
}

// Search describes an exact search. IndexEntries bounds the index's postings, not its output:
// exceeding the budget returns an error, never a graph with silently missing edges. The caller
// must discard any partial output on error. Zero uses the default of 20 million postings.
type Search struct {
	Metric       string
	Cutoff       int
	IndexEntries int
}

// SequencePairs visits every pair within the cutoff, once, in increasing B then A order.
// The input should be one locus's unique junctions (duplicates are nevertheless supported).
// The callback may stop the search, for example when the graph's own edge budget is exhausted.
// No all-pairs distance matrix or complete edge list is kept here.
//
// Identity uses a hash. With k substitutions, at least one of k+1 disjoint segments agrees:
// Hamming candidates share a segment at the same position and length. With k edits, deleting
// at most k characters from each string leaves a common subsequence: Levenshtein candidates
// share a deletion signature. Candidates are checked by the actual distance; the signatures
// alone are not evidence of an edge. These indexes are ClonoLoom's implementation choices.
func SequencePairs(ctx context.Context, sequences []string, o Search, visit func(Pair) error) error {
	if o.Metric != "identity" && o.Metric != "hamming" && o.Metric != "levenshtein" {
		return errors.New("network: choose identity, Hamming or Levenshtein distance")
	}
	if o.Cutoff < 0 || o.Cutoff > 2 || (o.Metric == "identity" && o.Cutoff != 0) {
		return errors.New("network: the cutoff is 0 for identity, or from 0 to 2 for edit distance")
	}
	if o.IndexEntries < 0 || visit == nil {
		return errors.New("network: a nonnegative index budget and a pair callback are required")
	}
	if o.IndexEntries == 0 {
		o.IndexEntries = 20_000_000
	}
	// The quadratic deletion neighborhoods are only appropriate for junctions, not full contigs.
	// Check all inputs before emitting any pair, so a malformed sequence cannot look like a node
	// with no neighbors. Ambiguous ASCII residues remain literal symbols for these metrics.
	for i, s := range sequences {
		if err := ctx.Err(); err != nil {
			return err
		}
		if len(s) == 0 || len(s) > 256 {
			return fmt.Errorf("network: junction %d must contain from 1 to 256 symbols", i+1)
		}
		for j := range s {
			if s[j] < 'A' || s[j] > 'Z' {
				return fmt.Errorf("network: junction %d must use uppercase sequence letters", i+1)
			}
		}
	}
	type signature struct {
		length, part int
		text         string
	}
	index := map[signature][]int{}
	seen := make([]int, len(sequences))
	entries := 0
	var candidates []int
	for b, s := range sequences {
		if err := ctx.Err(); err != nil {
			return err
		}
		var keys []signature
		switch {
		case o.Cutoff == 0:
			keys = []signature{{text: s}}
		case o.Metric == "hamming":
			for part := 0; part <= o.Cutoff; part++ {
				lo, hi := len(s)*part/(o.Cutoff+1), len(s)*(part+1)/(o.Cutoff+1)
				keys = append(keys, signature{len(s), part, s[lo:hi]})
			}
		case o.Metric == "levenshtein":
			for _, d := range deletions(s, o.Cutoff) {
				keys = append(keys, signature{text: d})
			}
		}
		if len(keys) > o.IndexEntries-entries {
			return errors.New("network: the sequence index exceeds its budget; use a smaller dataset or cutoff")
		}
		candidates = candidates[:0]
		for _, key := range keys {
			if err := ctx.Err(); err != nil {
				return err
			}
			for n, a := range index[key] {
				if n%1024 == 0 {
					if err := ctx.Err(); err != nil {
						return err
					}
				}
				if seen[a] != b+1 {
					seen[a] = b + 1
					candidates = append(candidates, a)
				}
			}
		}
		sort.Ints(candidates)
		for _, a := range candidates {
			if err := ctx.Err(); err != nil {
				return err
			}
			d := 0
			if o.Cutoff > 0 {
				if o.Metric == "hamming" {
					d = hamming(sequences[a], s, o.Cutoff)
				} else {
					d = levenshtein(sequences[a], s, o.Cutoff)
				}
			}
			if d <= o.Cutoff {
				if err := visit(Pair{a, b, d}); err != nil {
					return err
				}
			}
		}
		for _, key := range keys {
			index[key] = append(index[key], b)
		}
		entries += len(keys)
	}
	return ctx.Err()
}

// deletions returns distinct strings made by deleting up to cutoff characters, in lexical order.
// Repeated residues can produce the same signature many times; keep its posting only once.
func deletions(s string, cutoff int) []string {
	set := map[string]bool{s: true}
	for i := range s {
		set[s[:i]+s[i+1:]] = true
		if cutoff == 2 {
			for j := i + 1; j < len(s); j++ {
				set[s[:i]+s[i+1:j]+s[j+1:]] = true
			}
		}
	}
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func hamming(a, b string, cutoff int) int {
	if len(a) != len(b) {
		return cutoff + 1
	}
	d := 0
	for i := range a {
		if a[i] != b[i] {
			d++
			if d > cutoff {
				return d
			}
		}
	}
	return d
}

// levenshtein uses only the diagonal band reachable within cutoff edits. Values beyond the
// cutoff are represented by cutoff+1; only distances within the cutoff are returned exactly.
func levenshtein(a, b string, cutoff int) int {
	if len(a) > len(b) {
		a, b = b, a
	}
	far := cutoff + 1
	if len(b)-len(a) > cutoff {
		return far
	}
	// SequencePairs checks the junction length before calling this function.
	var previous, current [257]int
	for j := 0; j <= len(b); j++ {
		previous[j] = min(j, far)
	}
	for i := 1; i <= len(a); i++ {
		lo, hi := max(1, i-cutoff), min(len(b), i+cutoff)
		current[0] = min(i, far)
		if lo > 1 {
			current[lo-1] = far
		}
		best := far
		for j := lo; j <= hi; j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			current[j] = min(far, previous[j]+1, current[j-1]+1, previous[j-1]+cost)
			best = min(best, current[j])
		}
		if best > cutoff {
			return far
		}
		if hi < len(b) {
			current[hi+1] = far
		}
		previous, current = current, previous
	}
	return previous[len(b)]
}
