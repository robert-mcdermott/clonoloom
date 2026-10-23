package network

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"clonoloom/internal/random"
)

// The reference uses the full dynamic-programming matrix, without the candidate index, band
// or early exit used by the implementation. Exhaustive short strings exercise repeated letters,
// shifts, unequal lengths and strings shorter than the number of Hamming segments.
func editReference(a, b string) int {
	m := make([][]int, len(a)+1)
	for i := range m {
		m[i] = make([]int, len(b)+1)
		m[i][0] = i
	}
	for j := range m[0] {
		m[0][j] = j
	}
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			m[i][j] = min(m[i-1][j]+1, m[i][j-1]+1, m[i-1][j-1]+cost)
		}
	}
	return m[len(a)][len(b)]
}

func referencePairs(sequences []string, metric string, cutoff int) []Pair {
	var out []Pair
	for b, s := range sequences {
		for a := 0; a < b; a++ {
			d := 0
			if metric == "levenshtein" {
				d = editReference(sequences[a], s)
			} else {
				if len(sequences[a]) != len(s) {
					continue
				}
				for i := range s {
					if sequences[a][i] != s[i] {
						d++
					}
				}
			}
			if d <= cutoff {
				out = append(out, Pair{a, b, d})
			}
		}
	}
	return out
}

func checkPairs(t *testing.T, sequences []string) {
	t.Helper()
	for _, metric := range []string{"identity", "hamming", "levenshtein"} {
		for cutoff := 0; cutoff <= 2; cutoff++ {
			if metric == "identity" && cutoff != 0 {
				continue
			}
			var got []Pair
			err := SequencePairs(context.Background(), sequences, Search{Metric: metric, Cutoff: cutoff}, func(p Pair) error {
				got = append(got, p)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			want := referencePairs(sequences, metric, cutoff)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("%s, cutoff %d: pairs %v, want %v", metric, cutoff, got, want)
			}
		}
	}
}

func TestSequencePairsAgainstExhaustiveDistances(t *testing.T) {
	var sequences []string
	var words func(string, int)
	words = func(s string, left int) {
		if len(s) > 0 {
			sequences = append(sequences, s)
		}
		if left > 0 {
			words(s+"A", left-1)
			words(s+"C", left-1)
		}
	}
	words("", 6)
	sequences = append(sequences, "A", "AC", "CACACA") // duplicate junctions too
	checkPairs(t, sequences)
	checkPairs(t, nil)
}

func TestJunctionNeighbors(t *testing.T) {
	checkPairs(t, []string{
		"CASSLGGNTEAFF", "CASSLGNTEAFF", "CASSLAGGNTEAFF", "CASSLAGANTEAF",
		"CASSLGGNTEAFF", "CASSPRQGAYEQYF", "CASSQETQYF", "XASSLGGNTEAFF",
	})
}

func TestSequenceSearchStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sequences := []string{"AAAA", "AAAA", "AAAA", "AAAA"}
	calls := 0
	err := SequencePairs(ctx, sequences, Search{Metric: "identity"}, func(p Pair) error {
		calls++
		cancel()
		return nil
	})
	if err != context.Canceled || calls != 1 {
		t.Fatalf("cancellation: %d callbacks, %v", calls, err)
	}
	stop := errors.New("edge budget exhausted")
	err = SequencePairs(context.Background(), sequences, Search{Metric: "identity"}, func(p Pair) error { return stop })
	if err != stop {
		t.Fatalf("callback error lost: %v", err)
	}
	err = SequencePairs(context.Background(), sequences, Search{Metric: "identity", IndexEntries: 2}, func(p Pair) error { return nil })
	if err == nil {
		t.Fatal("index budget silently truncated the graph")
	}
}

func TestSequenceSearchChecksInputs(t *testing.T) {
	visit := func(Pair) error { t.Fatal("callback on invalid input"); return nil }
	for _, o := range []Search{
		{Metric: "other"}, {Metric: "identity", Cutoff: 1}, {Metric: "hamming", Cutoff: -1},
		{Metric: "levenshtein", Cutoff: 3}, {Metric: "identity", IndexEntries: -1},
	} {
		if err := SequencePairs(context.Background(), []string{"A", "A"}, o, visit); err == nil {
			t.Errorf("accepted %+v", o)
		}
	}
	for _, s := range []string{"", "cassf", "A C", "é", string(make([]byte, 257))} {
		if err := SequencePairs(context.Background(), []string{"A", "A", s}, Search{Metric: "identity"}, visit); err == nil {
			t.Errorf("accepted %q", s)
		}
	}
}

func FuzzSequencePairs(f *testing.F) {
	f.Add([]byte("CASSLGGNTEAFFCASSPRQGAYEQYF"))
	f.Add([]byte{0, 0, 0, 0})
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 100 {
			raw = raw[:100]
		}
		var sequences []string
		for len(raw) > 0 {
			n := min(1+int(raw[0])%8, len(raw))
			s := make([]byte, n)
			for i := range s {
				s[i] = "ACGT"[raw[i]%4]
			}
			sequences = append(sequences, string(s))
			raw = raw[n:]
		}
		checkPairs(t, sequences)
	})
}

func BenchmarkSequencePairs(b *testing.B) {
	for _, metric := range []string{"identity", "hamming", "levenshtein"} {
		for _, n := range []int{10000, 100000} {
			b.Run(fmt.Sprintf("%s/%d", metric, n), func(b *testing.B) {
				r := random.New(27, 0)
				sequences := make([]string, n)
				for i := range sequences {
					s := []byte("CASSAAAAAAAAAFF")
					for j := 4; j < len(s)-2; j++ {
						s[j] = "ACDEFGHIKLMNPQRSTVWY"[r.IntN(20)]
					}
					sequences[i] = string(s)
				}
				o := Search{Metric: metric, Cutoff: 2}
				if metric == "identity" {
					o.Cutoff = 0
				}
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					err := SequencePairs(context.Background(), sequences, o, func(Pair) error { return nil })
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
