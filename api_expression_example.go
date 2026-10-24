package main

import (
	"context"
	"embed"
	"os"
	"path/filepath"

	"clonoloom/internal/examples"
)

//go:embed testdata/expression-mex/*.tsv testdata/expression-mex/*.mtx
var expressionExampleData embed.FS

func expressionInputExample() examples.Generated {
	entries, _ := expressionExampleData.ReadDir("testdata/expression-mex")
	var size int64
	for _, e := range entries {
		info, _ := e.Info()
		size += info.Size()
	}
	return examples.Generated{
		Source: examples.Source{Name: "synthetic-expression", Title: "Artificial expression and receptor links", Description: "Four artificial barcodes, three features and five receptor rows. Check exact matching, repeated chains, a nonproductive receptor, an unmatched barcode and a zero-count cell. This is a format/linkage example, not a biological expression model.", Source: "Made by testdata/make-expression-mex.py", Publisher: "ClonoLoom", License: "Synthetic data: no restrictions", LicenseClass: "open", Citation: "10x Genomics MEX documentation; Boisvert et al. 1996, NISTIR 5935", Opens: []examples.Opening{{Title: "the receptor evidence", Name: "Artificial receptor links", Summary: "Inspect a matched MEX matrix in Cells.", Steps: []examples.Step{{Title: "Inspect the expression links", Action: "inspect_expression", Args: map[string]any{"example": true, "sample": 0}, Words: "Two of four expression barcodes match three distinct receptor barcodes. AA-1 has two chains; BB-1 is nonproductive but still matches raw evidence. One receptor barcode is absent from expression and one receptor row has no barcode. Gene-expression UMIs are 5, 4, 7 and 0; the antibody count is kept separately."}}}}},
		Size:   size, Marker: "receptors.tsv",
		Make: func(ctx context.Context, dir string, progress examples.Progress) error {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			for _, e := range entries {
				if err := ctx.Err(); err != nil {
					return err
				}
				data, err := expressionExampleData.ReadFile("testdata/expression-mex/" + e.Name())
				if err != nil {
					return err
				}
				if err = os.WriteFile(filepath.Join(dir, e.Name()), data, 0o644); err != nil {
					return err
				}
			}
			return nil
		}, Files: func(string) [][]string { return [][]string{{"receptors.tsv"}} },
	}
}
