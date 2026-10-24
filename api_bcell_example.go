package main

import (
	"context"
	_ "embed"
	"os"
	"path/filepath"

	"clonoloom/internal/examples"
)

//go:embed testdata/bcell-inputs.tsv
var bcellExampleData []byte

func bcellInputExample() examples.Generated {
	return examples.Generated{
		Source: examples.Source{Name: "synthetic-bcell-inputs", Title: "Synthetic B-cell sequence evidence", Description: "Seven artificial annotation cases: complete, partial, ambiguous, unequal alignments, invalid coordinates and CIGAR, and reverse orientation. These short sequences illustrate input checks, not biological receptors or germline reconstruction.", Source: "Made by testdata/make-bcell-inputs.mjs", Publisher: "ClonoLoom", License: "Synthetic data: no restrictions", LicenseClass: "open", Citation: "AIRR Rearrangement schema 2.0; doi:10.3389/fimmu.2018.02206", Opens: []examples.Opening{{Title: "the evidence", Name: "Synthetic B-cell evidence", Summary: "Seven rearrangements with deliberately different annotation evidence.", Steps: []examples.Step{
			{Title: "Inspect the complete annotation", Action: "inspect_bcell_inputs", Args: map[string]any{"select": 1}, Words: "One artificial rearrangement with a structurally consistent pair of alignments. This does not establish a biological germline."},
			{Title: "Inspect unequal alignments", Action: "inspect_bcell_inputs", Args: map[string]any{"select": 4}, Words: "The supplied query and reference have different lengths; the original values remain visible and exportable."},
		}}}},
		Size: int64(len(bcellExampleData)), Marker: "bcell-inputs.tsv",
		Make: func(ctx context.Context, dir string, progress examples.Progress) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(dir, "bcell-inputs.tsv"), bcellExampleData, 0o644)
		},
		Files: func(string) [][]string { return [][]string{{"bcell-inputs.tsv"}} },
	}
}
