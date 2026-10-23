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

func TestSHMAgainstKnownAncestors(t *testing.T) {
	var reference struct {
		Rows         int
		ValuesSHA256 string
		Totals       bcell.SHMCounts
		Sources      []struct{ File, SHA256 string }
	}
	raw, err := os.ReadFile("reference/shm.json")
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
	if result.Reconstructed != reference.Rows || len(result.Families) != 72 || len(result.Excluded) != 0 {
		t.Fatalf("rows %d groups %d exclusions %v", result.Reconstructed, len(result.Families), result.Excluded)
	}
	measured, err := bcell.SHM(context.Background(), res.Table, sources, result)
	if err != nil || measured.Evaluated != reference.Rows || len(measured.Excluded) != 0 || len(measured.CodonExclusions) != 0 {
		t.Fatal(err, measured.Excluded, measured.CodonExclusions)
	}
	var lines []string
	var total bcell.SHMCounts
	for _, r := range measured.Rows {
		for _, v := range r.Regions {
			if v.Unavailable != "" || v.Ambiguous != 0 || v.Gaps != 0 || v.StopCodons != 0 {
				t.Fatal(v)
			}
			if v.EvaluableBases == 0 {
				if v.Frequency != nil {
					t.Fatal(v)
				}
			} else if v.Frequency == nil || *v.Frequency != float64(v.Mutations)/float64(v.EvaluableBases) {
				t.Fatal(v)
			}
			lines = append(lines, fmt.Sprintf("%s\t%s\t%s\t%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d", r.Sample, r.SequenceID, r.Locus, v.Region, v.EvaluableBases, v.Mutations, v.Masked, v.EvaluableCodons, v.Replacement, v.Silent, v.Unclassified, v.MultiHitCodons, v.ReplacementCodons, v.SilentCodons))
			if v.Region == "all" {
				total.EvaluableBases += v.EvaluableBases
				total.Mutations += v.Mutations
				total.Replacement += v.Replacement
				total.Silent += v.Silent
				total.Unclassified += v.Unclassified
			}
		}
	}
	if got := familyDigest(lines); got != reference.ValuesSHA256 {
		t.Fatalf("all-row regional digest %s, want %s", got, reference.ValuesSHA256)
	}
	if total.EvaluableBases != reference.Totals.EvaluableBases || total.Mutations != reference.Totals.Mutations || total.Replacement != reference.Totals.Replacement || total.Silent != reference.Totals.Silent || total.Unclassified != reference.Totals.Unclassified {
		t.Fatal(total, reference.Totals)
	}
	pooledBases, pooledMutations := 0, 0
	for _, s := range measured.Summaries {
		if s.AvailableRows != s.Rows {
			t.Fatal(s)
		}
		if s.EvaluableBases > 0 {
			if s.Frequency == nil || *s.Frequency != float64(s.Mutations)/float64(s.EvaluableBases) {
				t.Fatal(s)
			}
		} else if s.Frequency != nil {
			t.Fatal(s)
		}
		if s.Region == "all" {
			pooledBases += s.EvaluableBases
			pooledMutations += s.Mutations
			if s.CodonRows != s.Rows {
				t.Fatal(s)
			}
		}
	}
	if pooledBases != reference.Totals.EvaluableBases || pooledMutations != reference.Totals.Mutations {
		t.Fatal(pooledBases, pooledMutations)
	}
	t.Logf("%d source rows, %d regions: %d substitutions / %d evaluable bases; %d R, %d S, %d unclassified", len(measured.Rows), len(lines), total.Mutations, total.EvaluableBases, total.Replacement, total.Silent, total.Unclassified)
}
