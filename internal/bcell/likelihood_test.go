package bcell

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mlPanel(seqs ...string) LineageTree {
	p := LineageTree{ID: "artificial-panel", Family: "artificial-family", Locus: "IGH", Sites: len(seqs[0]), Taxa: len(seqs)}
	for i := range seqs[0] {
		p.Columns = append(p.Columns, LineageColumn{"V", i + 1})
	}
	for i, s := range seqs {
		k := "observed"
		if i == 0 {
			k = "reference"
		}
		p.Nodes = append(p.Nodes, LineageNode{ID: string(rune('a' + i)), Kind: k, Sequence: s, Rows: 1, Annotations: []LineageAnnotation{{Sample: "Artificial sample", Isotype: "IGHM", Rows: 1}}})
	}
	return p
}
func TestLikelihoodTwoTips(t *testing.T) {
	for _, diff := range []int{0, 10, 75, 100} {
		seqs := []string{strings.Repeat("A", 100)}
		if diff > 0 {
			seqs = append(seqs, strings.Repeat("C", diff)+strings.Repeat("A", 100-diff))
		}
		r, e := Likelihood(context.Background(), mlPanel(seqs...))
		if e != nil {
			t.Fatal(e)
		}
		if diff > 0 {
			want := MLMaxBranch
			if diff < 75 {
				want = -.75 * math.Log(1-4*float64(diff)/300)
			}
			if math.Abs(r.Nodes[1].BranchLength-want) > 1e-9 {
				t.Fatal(diff, r.Nodes[1].BranchLength, want)
			}
		}
		if r.Topologies != 1 || !r.Fits[0].Converged {
			t.Fatal(r)
		}
	}
}
func TestLikelihoodControls(t *testing.T) {
	cases := map[string]LineageTree{
		"quartet": mlPanel(strings.Repeat("ACGT", 30), strings.Repeat("ACGT", 25)+strings.Repeat("AAGT", 5), strings.Repeat("ACGT", 25)+strings.Repeat("AAGG", 5), strings.Repeat("ACGT", 24)+"TCGT"+strings.Repeat("ACGT", 5)),
		"star":    mlPanel("AAAAAAAAAAAAAAAA", "CAAAAAAAAAAAAAAA", "ACAAAAAAAAAAAAAA", "AACAAAAAAAAAAAAA"),
		"five":    mlPanel("ACGTACGTACGTACGTACGTACGTACGTACGT", "CCGTACGTACGTACGTACGTACGTACGTACGT", "CCGTCCGTACGTACGTACGTACGTACGTACGT", "ACGTACGTACGTTCGTACGTACGTACGTACGT", "ACGTACGTACGTTCGTTCGTACGTACGTACGT"),
	}
	for name, p := range cases {
		t.Run(name, func(t *testing.T) {
			r, e := Likelihood(context.Background(), p)
			if e != nil {
				t.Fatal(e)
			}
			want := 3
			if p.Taxa == 5 {
				want = 15
			}
			if r.Topologies != want || len(r.Nodes) != 2*p.Taxa-2 || r.LogLikelihood >= 0 {
				t.Fatal(r)
			}
			for _, n := range r.Nodes {
				for _, v := range n.Posterior {
					sum := 0.0
					for _, x := range v {
						if x < 0 || x > 1 {
							t.Fatal(v)
						}
						sum += x
					}
					if math.Abs(sum-1) > 1e-12 {
						t.Fatal(v)
					}
				}
			}
			raw, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			digest := fmt.Sprintf("%x", sha256.Sum256(raw))
			wantDigest := map[string]string{
				"quartet": "0da459f0b893ad8c014c3a6bb959ed7ab8d9087aeeca82d9c239a4a789f43cfb",
				"star":    "35adc917ffa853b2a51d6f8994d8a7e1bcdeaa64d4bb1712be277695dca832b1",
				"five":    "052ade3193136138163f2920c6e3c04c7af58df1fcf08cde9848cd24f1c65edc",
			}[name]
			if digest != wantDigest {
				t.Fatalf("%s result SHA256 %s", name, digest)
			}
			// Metadata and historical parsimony estimates cannot steer likelihood.
			altered := p
			altered.Nodes = append([]LineageNode(nil), p.Nodes...)
			altered.Score = 12345
			altered.Search = "untrusted historical score"
			for i := range altered.Nodes {
				altered.Nodes[i].Changes = 99
				altered.Nodes[i].Parent = "irrelevant"
			}
			again, e := Likelihood(context.Background(), altered)
			if e != nil || again.Newick != r.Newick || again.LogLikelihood != r.LogLikelihood {
				t.Fatal("parsimony topology leaked into fit", e)
			}
			if dir := os.Getenv("CLONOLOOM_ML_OUT"); dir != "" {
				os.MkdirAll(dir, 0700)
				b, _ := json.MarshalIndent(map[string]any{"panel": p, "result": r}, "", "  ")
				if e := os.WriteFile(filepath.Join(dir, name+".json"), b, 0600); e != nil {
					t.Fatal(e)
				}
			}
		})
	}
}
func TestLikelihoodRefusals(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := Likelihood(ctx, mlPanel("AAAA", "AAAC")); e != context.Canceled {
		t.Fatal(e)
	}
	for _, p := range []LineageTree{mlPanel("AAAA", "AAAA"), mlPanel("AAAA", "AAAN"), mlPanel("AAAA", "AAA"), mlPanel(strings.Repeat("A", MLMaxSites+1)), mlPanel("AAAA", "AAAC", "AAAG", "AAAT", "AACA", "AAGA", "AATA", "ACAA")} {
		if _, e := Likelihood(context.Background(), p); e == nil {
			t.Fatal("accepted invalid panel", p)
		}
	}
	m := mlEngine{ctx: context.Background(), patterns: mlPatterns([]string{"AAAA", "AAAC"}), work: MLWorkBudget}
	g := treeGraph{{1}, {0}}
	if _, _, e := m.fit(g, 2, .1, 1); e == nil {
		t.Fatal("work bound")
	}
	// A zero-sweep fit away from the analytical optimum must fail stationarity.
	m = mlEngine{ctx: context.Background(), patterns: mlPatterns([]string{"AAAAAAAAAA", "CAAAAAAAAA"})}
	fit, _, e := m.fit(g, 2, 1, 0)
	if e != nil || fit.Converged || fit.MaxCoordinateGain < .1 {
		t.Fatal(fit, e)
	}
}

func TestLikelihoodTieWindow(t *testing.T) {
	// Pairwise chaining would choose a, more than one tolerance below the best.
	fits := []MLFit{{Topology: "c", LogLikelihood: -1}, {Topology: "b", LogLikelihood: -1 - 0.75e-8}, {Topology: "a", LogLikelihood: -1 - 1.5e-8}}
	for _, order := range [][]int{{0, 1, 2}, {2, 1, 0}, {1, 0, 2}} {
		ordered := []MLFit{fits[order[0]], fits[order[1]], fits[order[2]]}
		if selected := mlSelect(ordered); ordered[selected].Topology != "b" {
			t.Fatal(ordered, selected)
		}
	}
}
