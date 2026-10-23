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
	"strconv"
	"strings"
	"testing"
)

func TestGermlinesAgainstIndependentCoordinates(t *testing.T) {
	var reference struct {
		Rows                               int
		ReferenceSHA256, ProjectionsSHA256 string
	}
	raw, err := os.ReadFile("reference/germlines.json")
	if err != nil || json.Unmarshal(raw, &reference) != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "study")
	_, err = simulate.Run(context.Background(), simulate.Presets["bcell-immunization"], dir, "validation", nil)
	if err != nil {
		t.Fatal(err)
	}
	paths, _ := filepath.Glob(filepath.Join(dir, "*.tsv"))
	res, err := reads.Open(context.Background(), paths)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(filepath.Join(dir, "reference/germlines.fasta"))
	if err != nil || fmt.Sprintf("%x", sha256.Sum256(raw)) != reference.ReferenceSHA256 {
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
	ints := func(values []int) string {
		out := make([]string, len(values))
		for i, v := range values {
			out[i] = strconv.Itoa(v)
		}
		return strings.Join(out, ",")
	}
	var lines []string
	for _, r := range result.Rows {
		lines = append(lines, strings.Join([]string{r.Sample, r.SequenceID, r.Locus, r.V, r.J, r.Query, r.Germline, r.Regions, ints(r.QueryPositions), ints(r.ReferencePositions)}, "\t"))
	}
	if got := familyDigest(lines); got != reference.ProjectionsSHA256 {
		t.Fatalf("all-row projection digest %s, want %s", got, reference.ProjectionsSHA256)
	}
	t.Logf("All %d projections and coordinate maps agree with independent CSV/FASTA slicing and ancestor check", result.Reconstructed)
}

// Public data are a format/coverage check, not known germline truth. Checksums come
// from the publisher downloads recorded in sources.json; no reference software is imported.
func TestGermlinePublicReferenceAndAIRRCoverage(t *testing.T) {
	raw, err := os.ReadFile("sources.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		References map[string]struct {
			Files []struct{ Path, SHA256 string }
		}
	}
	if err = json.Unmarshal(raw, &catalog); err != nil {
		t.Fatal(err)
	}
	ref := bcell.GermlineReference{}
	for _, file := range catalog.References["ogrdb-human-2026-05-27"].Files {
		raw, err := os.ReadFile(filepath.Join(dataRoot(), "ogrdb-human-2026-05-27", file.Path))
		if os.IsNotExist(err) {
			if required("ogrdb-human-2026-05-27") {
				t.Fatal(err)
			}
			t.Skip("download pinned OGRDB data for public coverage check")
		}
		if err != nil {
			t.Fatal(err)
		}
		if fmt.Sprintf("%x", sha256.Sum256(raw)) != file.SHA256 {
			t.Fatal("reference checksum differs", file.Path)
		}
		part, err := bcell.ReadGermlineFASTA(strings.NewReader(string(raw)))
		if err != nil {
			t.Fatal(file.Path, err)
		}
		for k, v := range part {
			ref[k] = v
		}
		t.Logf("%s: %d alleles", file.Path, len(part))
	}
	for _, pattern := range []string{"vdjserver-ramos-libra/*.tsv", "10x-pbmc-1k/*b_airr_rearrangement.tsv", "10x-pbmc-10k-v2/*b_airr_rearrangement.tsv"} {
		paths, _ := filepath.Glob(filepath.Join(dataRoot(), pattern))
		if len(paths) == 0 {
			if required(strings.Split(pattern, "/")[0]) {
				t.Fatal("required AIRR data missing", pattern)
			}
			continue
		}
		res, err := reads.Open(context.Background(), paths)
		if err != nil {
			t.Fatal(err)
		}
		sources := make([]bcell.FamilySource, len(res.Sources))
		for i, s := range res.Sources {
			sources[i] = bcell.FamilySource{Name: s.Name, SHA256: s.SHA256}
		}
		threshold := 0.0
		f, err := bcell.Families(context.Background(), res.Table, nil, sources, nil, bcell.FamilyOptions{Mode: "heavy", Scope: "sample", Threshold: &threshold}, nil)
		if err != nil {
			t.Fatal(err)
		}
		r, err := bcell.Germlines(context.Background(), res.Table, sources, f, ref)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%s: %d reconstructed of %d assigned heavy rows; exclusions %v", pattern, r.Reconstructed, len(r.Rows), r.Excluded)
		if strings.Contains(pattern, "10x") && r.Reconstructed != 0 {
			t.Fatal("gene-only calls must not become allele-specific germlines")
		}
		for _, row := range r.Rows {
			if row.Excluded == "" && (len(row.Query) != len(row.Germline) || len(row.Query) != len(row.QueryPositions)) {
				t.Fatal("invalid projection")
			}
		}
	}
}
