package validation

import (
	"clonoloom/internal/bcell"
	"clonoloom/internal/reads"
	"clonoloom/internal/simulate"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestIsotypesAgainstIndependentCellTallies(t *testing.T) {
	var ref struct {
		Sources                []struct{ File, SHA256 string }
		Observations, Families int
		Usage                  []struct {
			MembersSHA256, Sample, Level, State string
			Count                               int
		}
		Pairs []struct {
			Left, Right string
			Families    int
		}
	}
	raw, err := os.ReadFile("reference/isotypes.json")
	if err != nil || json.Unmarshal(raw, &ref) != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "study")
	if _, err = simulate.Run(context.Background(), simulate.Presets["bcell-immunization"], dir, "validation", nil); err != nil {
		t.Fatal(err)
	}
	for _, s := range ref.Sources {
		raw, err := os.ReadFile(filepath.Join(dir, s.File))
		if err != nil || fmt.Sprintf("%x", sha256.Sum256(raw)) != s.SHA256 {
			t.Fatal("source differs", s.File, err)
		}
	}
	paths, _ := filepath.Glob(filepath.Join(dir, "*.tsv"))
	data, err := reads.Open(context.Background(), paths)
	if err != nil {
		t.Fatal(err)
	}
	subjects := map[string]string{}
	for _, s := range data.Table.Samples {
		subjects[s.Name] = s.Name[:3]
	}
	sources := make([]bcell.FamilySource, len(data.Sources))
	for i, s := range data.Sources {
		sources[i] = bcell.FamilySource{Name: s.Name, SHA256: s.SHA256}
	}
	threshold := .1
	parent, err := bcell.Families(context.Background(), data.Table, nil, sources, subjects, bcell.FamilyOptions{Mode: "paired", Threshold: &threshold}, nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := bcell.Isotypes(context.Background(), data.Table, sources, parent, nil)
	if err != nil || result.Assigned != ref.Observations || result.ResolvedClass != ref.Observations || result.ResolvedGene != ref.Observations || len(result.Families) != ref.Families || len(result.Usage) != len(ref.Usage) || len(result.Pairs) != len(ref.Pairs) {
		t.Fatal(err, result.Assigned, len(result.Usage))
	}
	members := map[string][]string{}
	for _, o := range result.Observations {
		for _, r := range o.Rows {
			if r.Locus == "IGH" {
				members[o.Family] = append(members[o.Family], r.SequenceID)
			}
		}
	}
	type key struct{ Family, Sample, Level, State string }
	expected := map[key]int{}
	totals := map[[3]string]int{}
	for _, u := range ref.Usage {
		expected[key{u.MembersSHA256, u.Sample, u.Level, u.State}] = u.Count
		totals[[3]string{u.MembersSHA256, u.Sample, u.Level}] += u.Count
	}
	for _, u := range result.Usage {
		digest := ""
		if u.Family != "" {
			digest = familyDigest(members[u.Family])
		}
		k := key{digest, u.Sample, u.Level, u.State}
		total := totals[[3]string{digest, u.Sample, u.Level}]
		if u.Observations != expected[k] || u.Total != total || u.Share != float64(u.Observations)/float64(total) || u.Abundance == nil || *u.Abundance != float64(u.Observations) || u.TotalAbundance == nil || *u.TotalAbundance != float64(total) || u.AbundanceShare == nil || *u.AbundanceShare != u.Share {
			t.Fatalf("%+v; expected %d / %d", u, expected[k], total)
		}
		delete(expected, k)
	}
	if len(expected) != 0 {
		t.Fatal("unmatched usage", len(expected))
	}
	for i, p := range result.Pairs {
		want := ref.Pairs[i]
		if p.Left != want.Left || p.Right != want.Right || p.Families != want.Families || p.TotalFamilies != ref.Families || p.EvaluableFamilies != ref.Families {
			t.Fatal(p, want)
		}
	}
	serialized, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	const digest = "fb790f155d67331df1253dae9ab7a97a8f5ab0b3d53eed985c35f964ed1883ac"
	if got := fmt.Sprintf("%x", sha256.Sum256(serialized)); got != digest {
		t.Fatalf("isotype serialization differs: %s", got)
	}
	t.Logf("%d cells, %d families, %d usage rows and %d unordered pairs match independent truth tallies", result.Assigned, len(result.Families), len(result.Usage), len(result.Pairs))
}
