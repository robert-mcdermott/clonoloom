package validation

import (
	"clonoloom/internal/clonotype"
	"clonoloom/internal/invariant"
	"clonoloom/internal/reads"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// A biological sanity check, not a phenotype ground truth. The often reported 1–10% is of
// blood T cells, not all PBMCs (van Wilgenburg et al. Nat Commun 2018,
// doi:10.1038/s41467-018-07207-9). We use retained primary TRA/TRB cells, no B-cell file.
func TestInvariantPopulationsInPBMC(t *testing.T) {
	var reference map[string]struct {
		Cells   float64    `json:"cells"`
		Matched [3]float64 `json:"matched"`
	}
	raw, err := os.ReadFile("reference/invariant.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &reference); err != nil {
		t.Fatal(err)
	}
	for _, set := range []string{"10x-pbmc-1k", "10x-pbmc-10k-v2"} {
		t.Run(set, func(t *testing.T) {
			paths, _ := filepath.Glob(filepath.Join(dataRoot(), set, "*t_filtered_contig_annotations.csv"))
			if len(paths) != 1 {
				if required(set) {
					t.Fatal("missing contig table")
				}
				t.Skip("public data not cached")
			}
			res, err := reads.ReadFiles(context.Background(), paths)
			if err != nil {
				t.Fatal(err)
			}
			ct, err := clonotype.Compute(res.Table, nil, clonotype.Definition{Chains: []string{"TRA", "TRB"}, Junction: "aa", V: "gene", J: "gene", Cells: "primary", ProductiveOnly: true})
			if err != nil {
				t.Fatal(err)
			}
			r, err := invariant.Compute(ct, "human")
			if err != nil {
				t.Fatal(err)
			}
			for _, s := range r.Samples {
				if s.Total != reference[set].Cells || s.Matched != reference[set].Matched {
					t.Fatalf("independent cell-level CSV check: got %+v want %+v", s, reference[set])
				}
				if s.Share[0] == nil || s.Total <= 0 || s.Evaluable[0] <= 0 {
					t.Fatal(s)
				}
				t.Logf("MAIT %g / %g retained T cells = %.4f%%; alpha-evaluable %g; iNKT %g; GEM %g; published blood T-cell range 1–10%%", s.Matched[0], s.Total, 100**s.Share[0], s.Evaluable[0], s.Matched[1], s.Matched[2])
				// Report outside the literature's typical range, never force a donor into it.
				if *s.Share[0] < .01 || *s.Share[0] > .10 {
					t.Log("Outside the typical range; donor, capture and sequence-rule ascertainment differ from tetramer phenotyping.")
				}
			}
		})
	}
}
