package validation

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"clonoloom/internal/bcell"
	"clonoloom/internal/reads"
	"clonoloom/internal/simulate"
)

func TestLineagesAgainstIndependentExactSearch(t *testing.T) {
	var reference struct {
		Sources []struct{ File, SHA256 string }
		Trees   []struct {
			Family, Locus, MembersSHA256                                string
			Rows, Sites, Taxa, Score, OptimalTrees, GeneratingTreeScore int
		}
	}
	raw, err := os.ReadFile("reference/lineages.json")
	if err != nil || json.Unmarshal(raw, &reference) != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "study")
	_, err = simulate.Run(context.Background(), simulate.Presets["bcell-immunization"], dir, "validation", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range reference.Sources {
		raw, err := os.ReadFile(filepath.Join(dir, source.File))
		if err != nil || fmt.Sprintf("%x", sha256.Sum256(raw)) != source.SHA256 {
			t.Fatal("source fingerprint differs", source.File, err)
		}
	}
	paths, _ := filepath.Glob(filepath.Join(dir, "*.tsv"))
	res, err := reads.Open(context.Background(), paths)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(filepath.Join(dir, "reference/germlines.fasta"))
	if err != nil {
		t.Fatal("reference differs", err)
	}
	ref, err := bcell.ReadGermlineFASTA(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	subjects := map[string]string{}
	for _, s := range res.Table.Samples {
		subjects[s.Name] = s.Name[:3]
	}
	sources := make([]bcell.FamilySource, len(res.Sources))
	for i, s := range res.Sources {
		sources[i] = bcell.FamilySource{Name: s.Name, SHA256: s.SHA256}
	}
	threshold := .1
	families, err := bcell.Families(context.Background(), res.Table, nil, sources, subjects, bcell.FamilyOptions{Mode: "paired", Threshold: &threshold}, nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := bcell.Germlines(context.Background(), res.Table, sources, families, ref)
	if err != nil {
		t.Fatal(err)
	}

	measured, err := bcell.Lineages(context.Background(), res.Table, sources, result, nil)
	if err != nil || measured.Built != 72 || len(measured.Excluded) != 0 {
		t.Fatal(err, measured.Built, measured.Excluded)
	}
	byMembers := map[string][]string{}
	for _, r := range measured.Members {
		if r.Node == "" || r.Excluded != "" {
			t.Fatal(r)
		}
		byMembers[r.Tree] = append(byMembers[r.Tree], r.SequenceID)
	}
	checked, total := 0, 0
	for _, tree := range measured.Trees {
		digest := familyDigest(byMembers[tree.ID])
		found := false
		for _, want := range reference.Trees {
			if want.MembersSHA256 != digest {
				continue
			}
			found = true
			checked++
			if tree.Locus != want.Locus || tree.Rows != want.Rows || tree.Included != want.Rows || tree.Sites != want.Sites || tree.Taxa != want.Taxa || tree.Score != want.Score || tree.OptimalTrees == nil || *tree.OptimalTrees != want.OptimalTrees || tree.Score > want.GeneratingTreeScore || tree.Search != "exhaustive" {
				t.Fatalf("tree %+v; want %+v", tree.Summary(), want)
			}
		}
		if !found {
			t.Fatal("unknown membership", tree.ID, digest)
		}
		score, rows := 0, 0
		nodes := map[string]bcell.LineageNode{}
		for _, n := range tree.Nodes {
			nodes[n.ID] = n
		}
		for _, n := range tree.Nodes {
			score += n.Changes
			rows += n.Rows
			if len(n.Sequence) != tree.Sites || len(n.Possible) != tree.Sites {
				t.Fatal(n)
			}
			if n.Parent != "" {
				parent := nodes[n.Parent]
				d := 0
				for i := range n.Sequence {
					if n.Sequence[i] != parent.Sequence[i] {
						d++
					}
				}
				if d != n.Changes || n.Distance != parent.Distance+d || n.Depth != parent.Depth+1 || n.SplitTrees == nil || *n.SplitTrees < 1 || *n.SplitTrees > *tree.OptimalTrees {
					t.Fatal(n)
				}
			}
			if n.Kind != "inferred" && n.Sequence != n.Possible {
				t.Fatal("observed states changed", n)
			}
		}
		if score != tree.Score || rows != tree.Included {
			t.Fatal(score, rows, tree.Summary())
		}
		total += score
	}
	if checked != len(reference.Trees) {
		t.Fatal(checked)
	}
	// Pin the complete graph, panel, assignment, ambiguity and metadata serialization
	// for the arm64/amd64 checks, separately from the independent scientific oracle.
	graphJSON, err := json.Marshal(measured.Trees)
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(graphJSON)); got != "2149d0619a7a6a658e2b01f5e089e3828bcdf014e56a4be6a3b148c84a6701d7" {
		t.Fatalf("tree serialization differs: %s", got)
	}
	t.Logf("%d trees and %d source rows match independent exhaustive search: %d changes", checked, len(measured.Members), total)
}
