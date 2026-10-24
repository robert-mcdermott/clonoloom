package main

import (
	"context"
	"embed"
	"os"
	"path/filepath"

	"clonoloom/internal/examples"
)

//go:embed testdata/imgt-inputs/*.txt
var imgtExampleData embed.FS

func imgtInputExample() examples.Generated {
	entries, _ := imgtExampleData.ReadDir("testdata/imgt-inputs")
	var size int64
	for _, entry := range entries {
		info, _ := entry.Info()
		size += info.Size()
	}
	return examples.Generated{
		Source: examples.Source{Name: "synthetic-imgt-inputs", Title: "Synthetic IMGT annotation tables", Description: "Four artificial rearrangements split across reordered IMGT tables: heavy and light chains, repeated sequence IDs, reverse submission, a corrected insertion and an unsuccessful annotation. No biological or reference sequences are bundled.", Source: "Made by testdata/make-imgt-inputs.mjs", Publisher: "ClonoLoom", License: "Synthetic data: no restrictions", LicenseClass: "open", Citation: "IMGT/V-QUEST output documentation; Brochet et al. 2008, doi:10.1093/nar/gkn316", Opens: []examples.Opening{{Title: "the annotations", Name: "Synthetic IMGT evidence", Summary: "Join the tables and inspect retained annotations.", Steps: []examples.Step{
			{Title: "Inspect the reverse submission", Action: "inspect_bcell_inputs", Args: map[string]any{"select": 2}, Words: "IMGT already oriented the Summary sequence. Native annotations retain the submitted orientation; no second reversal is applied."},
			{Title: "Inspect the corrected insertion", Action: "inspect_bcell_inputs", Args: map[string]any{"select": 3}, Words: "The IMGT coordinates describe corrected sequence evidence. ClonoLoom retains the original positions but withholds their mapping to the Summary query. No germline alignment is inferred."},
		}}}},
		// ReadDir is sorted: Junction is written last, after the other required tables.
		Size: size, Marker: "6_Junction.txt",
		Make: func(ctx context.Context, dir string, progress examples.Progress) error {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			for _, entry := range entries {
				if err := ctx.Err(); err != nil {
					return err
				}
				data, err := imgtExampleData.ReadFile("testdata/imgt-inputs/" + entry.Name())
				if err != nil {
					return err
				}
				if err = os.WriteFile(filepath.Join(dir, entry.Name()), data, 0o644); err != nil {
					return err
				}
			}
			return nil
		},
		Files: func(string) [][]string { return [][]string{{"."}} },
	}
}
