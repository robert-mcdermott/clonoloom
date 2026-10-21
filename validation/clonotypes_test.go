// Single-cell clonotypes on the development files against scirpy's: each Cell Ranger run's cells
// grouped by the paired definition (primary chains of both arms, by nucleotide or amino-acid
// junction, with and without the V gene; a cell with one arm grouped with cells of that arm
// alone), compared with scirpy's define_clonotypes and define_clonotype_clusters by their cells,
// clonotypes and the partition itself (validation/reference/clonotypes.json, made by
// scirpy_clonotypes.py).
package validation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"clonoloom/internal/clonotype"
	"clonoloom/internal/reads"
	"clonoloom/internal/rep"
)

// partition digests a single-cell clonotype table as scirpy_clonotypes.py does.
func partition(rt *rep.Table, t *clonotype.Table) (cells, clonotypes int, digest string) {
	lines := make([]string, 0, len(t.Clonotypes))
	for i := range t.Clonotypes {
		seen := map[string]bool{}
		var barcodes []string
		for _, row := range t.MembersOf(i) {
			b := rt.Dict.Get(rt.Cell[row])
			if !seen[b] {
				seen[b] = true
				barcodes = append(barcodes, b)
			}
		}
		sort.Strings(barcodes)
		cells += len(barcodes)
		lines = append(lines, strings.Join(barcodes, ","))
	}
	sort.Strings(lines)
	h := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return cells, len(t.Clonotypes), hex.EncodeToString(h[:])
}

func TestSingleCellClonotypesAgreeWithScirpy(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("reference", "clonotypes.json"))
	if err != nil {
		t.Fatal(err)
	}
	var ref struct {
		GeneratedBy string `json:"generated_by"`
		Files       map[string]map[string]struct {
			Cells      int    `json:"cells"`
			Clonotypes int    `json:"clonotypes"`
			SHA256     string `json:"sha256"`
		} `json:"files"`
	}
	if err := json.Unmarshal(raw, &ref); err != nil {
		t.Fatal(err)
	}
	compared := 0
	for rel, definitions := range ref.Files {
		dataset := strings.Split(filepath.ToSlash(rel), "/")[0]
		path := filepath.Join(dataRoot(), filepath.FromSlash(rel))
		if _, err := os.Stat(path); err != nil {
			if required(dataset) {
				t.Errorf("%s is required and missing", rel)
			}
			continue
		}
		res, err := reads.ReadFiles(context.Background(), []string{path})
		if err != nil {
			t.Fatal(err)
		}
		chains := []string{"TRA", "TRB"}
		if strings.Contains(rel, "_b_") {
			chains = []string{"IGH", clonotype.Light}
		}
		for name, want := range definitions {
			v, junction := "none", "aa"
			if strings.HasSuffix(name, "_v_gene") {
				v = "gene"
			}
			if strings.HasPrefix(name, "junction_nt") {
				junction = "nt"
			}
			table, err := clonotype.Compute(res.Table, nil, clonotype.Definition{Chains: chains, Junction: junction, V: v, J: "none", Cells: "primary", ProductiveOnly: true})
			if err != nil {
				t.Fatal(err)
			}
			cells, n, digest := partition(res.Table, table)
			if cells != want.Cells || n != want.Clonotypes || digest != want.SHA256 {
				t.Errorf("%s, %s: %d cells in %d clonotypes; scirpy %d in %d (partitions equal: %v)", filepath.Base(rel), name, cells, n, want.Cells, want.Clonotypes, digest == want.SHA256)
			}
			compared++
		}
	}
	t.Logf("%d groupings compared with %s", compared, ref.GeneratedBy)
}

// Cell Ranger's own clonotypes, as the contigs name them (the vendor definition), have the cells
// Cell Ranger's clonotypes.csv counts for each.
func TestCellRangerClonotypesAsGiven(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join(dataRoot(), "10x-*", "*filtered_contig_annotations.csv"))
	if len(files) == 0 {
		t.Skip("no Cell Ranger outputs (node validation/fetch.mjs --ci)")
	}
	for _, path := range files {
		summary := strings.TrimSuffix(path, "filtered_contig_annotations.csv") + "clonotypes.csv"
		raw, err := os.ReadFile(summary)
		if err != nil {
			continue
		}
		want := map[string]int{}
		lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
		for _, line := range lines[1:] {
			f := strings.Split(line, ",")
			var n int
			for _, c := range f[1] {
				n = 10*n + int(c-'0')
			}
			want[f[0]] = n
		}
		res, err := reads.ReadFiles(context.Background(), []string{path})
		if err != nil {
			t.Fatal(err)
		}
		table, err := clonotype.Compute(res.Table, nil, clonotype.Definition{Vendor: true})
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]int{}
		for _, c := range table.Clonotypes {
			got[c.Vendor] = int(c.Total)
		}
		differ := 0
		for id, n := range want {
			if got[id] != n {
				differ++
			}
		}
		if differ > 0 || len(got) != len(want) {
			t.Errorf("%s: %d of %d clonotypes' sizes differ from clonotypes.csv (%d clonotypes read)", filepath.Base(path), differ, len(want), len(got))
		}
	}
}

// Bulk clonotypes on the development files against the R implementation (qc.R): their number
// and the largest one's abundance under three definitions (amino-acid junction with V and J
// genes; nucleotide junction with V and J genes; amino-acid junction alone).
func TestBulkClonotypesAgreeWithAnIndependentImplementation(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("reference", "qc.json"))
	if err != nil {
		t.Fatal(err)
	}
	var ref struct {
		Files map[string]map[string]struct {
			Clonotypes       int     `json:"clonotypes"`
			ClonotypesNT     int     `json:"clonotypes_nt"`
			ClonotypesAAOnly int     `json:"clonotypes_aa_only"`
			LargestNT        float64 `json:"largest_nt"`
			LargestAAOnly    float64 `json:"largest_aa_only"`
		} `json:"files"`
	}
	if err := json.Unmarshal(raw, &ref); err != nil {
		t.Fatal(err)
	}
	compared := 0
	for rel, samples := range ref.Files {
		path := filepath.Join(dataRoot(), filepath.FromSlash(rel))
		if _, err := os.Stat(path); err != nil {
			continue
		}
		res, err := reads.ReadFiles(context.Background(), []string{path})
		if err != nil {
			t.Fatal(err)
		}
		type check struct {
			definition clonotype.Definition
			count      func(name string) int
			largest    func(name string) float64
		}
		for _, c := range []check{
			{clonotype.Definition{Junction: "aa", V: "gene", J: "gene"}, func(n string) int { return samples[n].Clonotypes }, nil},
			{clonotype.Definition{Junction: "nt", V: "gene", J: "gene", ProductiveOnly: true}, func(n string) int { return samples[n].ClonotypesNT }, func(n string) float64 { return samples[n].LargestNT }},
			{clonotype.Definition{Junction: "aa", V: "none", J: "none"}, func(n string) int { return samples[n].ClonotypesAAOnly }, func(n string) float64 { return samples[n].LargestAAOnly }},
		} {
			table, err := clonotype.Compute(res.Table, nil, c.definition)
			if err != nil {
				t.Fatal(err)
			}
			for s, sample := range res.Table.Samples {
				n, largest := 0, 0.0
				for i := range table.Clonotypes {
					if v := table.In(i, uint32(s)); v > 0 {
						n++
						largest = max(largest, v)
					}
				}
				if n != c.count(sample.Name) || (c.largest != nil && largest != c.largest(sample.Name)) {
					t.Errorf("%s %s, %s: %d clonotypes, largest %v; the reference %d, %v", filepath.Base(rel), sample.Name, c.definition.Describe(), n, largest, c.count(sample.Name), func() float64 {
						if c.largest == nil {
							return 0
						}
						return c.largest(sample.Name)
					}())
				}
				compared++
			}
		}
	}
	t.Logf("%d samples and definitions compared", compared)
}
