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

//go:embed testdata/expression-hdf5/checksum.h5
var expressionHDF5Data []byte

func expressionHDF5Example() examples.Generated {
	e := expressionInputExample()
	e.Source.Name = "synthetic-expression-hdf5"
	e.Source.Title = "Artificial HDF5 expression and receptor links"
	e.Source.Description = "The same four artificial cells as the MEX example, stored as a compressed 10x HDF5 matrix with shuffle and chunk checksums. Compare counts, repeated feature names, antibody evidence, explicit zeros and exact receptor links."
	e.Source.Source = "Made independently with h5py by testdata/make-expression-hdf5.py"
	e.Source.Citation = "10x Genomics HDF5 feature-barcode documentation; The HDF Group File Format Specification 3.0"
	e.Source.Opens[0].Summary = "Inspect a compressed HDF5 matrix in Cells."
	e.Source.Opens[0].Steps[0].Args = map[string]any{"hdf5Example": true, "sample": 0}
	e.Size += int64(len(expressionHDF5Data))
	makeMEX := e.Make
	e.Make = func(ctx context.Context, dir string, progress examples.Progress) error {
		if err := makeMEX(ctx, dir, progress); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dir, "matrix.h5"), expressionHDF5Data, 0o644)
	}
	// Mark availability only once the matrix itself is present.
	e.Marker = "matrix.h5"
	return e
}

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

func cellStateInputExample() examples.Generated {
	steps := []examples.Step{{
		Title: "Inspect known cell states and supplied coordinates", Action: "inspect_expression",
		Args:  map[string]any{"studyExample": true, "sample": 0},
		Words: "Inspect S01-1: 120 expression barcodes, 119 receptor matches, one expression-only and one receptor-only barcode. The selected cells.tsv has 119 matching rows, one annotation-only row and 118 mapped expression cells. Open Imported map; color by state and inspect missing labels. This artificial layout is not UMAP or evidence of specificity. Other samples have separate matrices and repeated barcodes; select each sample explicitly.",
	}}
	return simulatedExample("cell-states", steps, "A paired T-cell and expression study with known states",
		"Two artificial subjects, two samples each, paired TRA/TRB receptors and 24 artificial genes in three known states. Includes deliberate missing evidence, supplied layout coordinates and separate truth. Generated locally from a fixed seed; a controlled benchmark, not a fitted biological model.",
		"Four samples of 120 expression cells, with paired receptor evidence and exact sample-scoped barcodes.", 1<<20)
}

func cellStateCloudExample() examples.Generated {
	steps := []examples.Step{{Title: "Inspect supplied cell-state clouds", Action: "inspect_expression",
		Args:  map[string]any{"studyExample": true, "studyPreset": "cell-states-clouds", "sample": 0},
		Words: "S01-1 has 360 expression cells, 359 receptor matches and 358 mapped cells. Open Imported map and color by state: missing labels are neutral gray. Coordinates are illustrative Gaussian clouds, not UMAP or a fitted biological model. The separate legacy grid example remains available for exact overlap checks. Select each receptor sample explicitly; barcodes repeat across samples.",
	}, {Title: "Compute expression neighborhoods and UMAP", Action: "compute_expression_map", Args: map[string]any{}, Words: "Compute a new UMAP from S01-1 counts, independently of supplied coordinates. Color by state, inspect missing annotations, selected features and excluded cells, then read the saved method and download the graph/PCA/provenance JSON. Exact neighborhoods use PCA space; the map is exploratory and does not infer receptor specificity."}}
	return simulatedExample("cell-states-clouds", steps, "A paired cell-state study with supplied Gaussian clouds",
		"Two artificial subjects, two samples each, 360 expression cells per sample and ten clones per subject. Three states have supplied Gaussian coordinates with different spreads and unequal expected cell numbers. Separate truth, paired TRA/TRB chains and deliberately missing evidence. An illustrative benchmark, not an inferred embedding or a fitted biological model.",
		"Four samples of 360 expression cells, with paired receptors and supplied continuous coordinates.", 3<<20)
}

//go:embed testdata/anndata/gzip-csr.h5ad
var annDataExampleData []byte

func annDataExample() examples.Generated {
	e := expressionInputExample()
	e.Source.Name = "synthetic-anndata"
	e.Source.Title = "Artificial AnnData counts, annotations and AIRR evidence"
	e.Source.Description = "Four artificial cells with a selected count layer, categorical and nullable observations, variable annotations, supplied coordinates and unfiltered AIRR chain records. Coordinates are illustrative, not a computed UMAP."
	e.Source.Source = "Made independently through AnnData and Awkward public writer APIs by testdata/make-anndata.py"
	e.Source.Citation = "AnnData on-disk format; scirpy data structure; Awkward ak.to_buffers documentation"
	e.Source.Opens[0].Summary = "Inspect selected AnnData counts and supplied evidence in Cells."
	e.Source.Opens[0].Steps[0].Title = "Inspect AnnData evidence"
	e.Source.Opens[0].Steps[0].Args = map[string]any{"anndataExample": true, "sample": 0}
	e.Source.Opens[0].Steps[0].Words = "The selected layers/counts has four barcodes and three features. Only two Gene Expression features contribute to counts (5, 4, 7, 0). Inspect nullable observations, repeated feature names, a missing coordinate pair and three supplied AIRR chains. The source calls its illustrative map X_umap; ClonoLoom does not compute these coordinates or infer clonotypes."
	e.Size = int64(len(annDataExampleData)) + 512
	e.Marker = "cells.h5ad"
	e.Make = func(ctx context.Context, dir string, progress examples.Progress) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		receptor := "sequence_id\tsequence\trev_comp\tproductive\tv_call\td_call\tj_call\tsequence_alignment\tgermline_alignment\tjunction\tjunction_aa\tv_cigar\td_cigar\tj_cigar\tcell_id\tlocus\tumi_count\n" +
			"artificial-1\t\tF\tT\t\t\t\t\t\t\tCASSF\t\t\t\tCELL1\tTRA\t2\n" +
			"artificial-2\t\tF\t\t\t\t\t\t\t\tCASRF\t\t\t\tCELL1\tTRB\t1\n" +
			"artificial-3\t\tF\tF\t\t\t\t\t\t\t\t\t\t\tCELL3\tTRB\t\n"
		if err := os.WriteFile(filepath.Join(dir, "receptors.tsv"), []byte(receptor), 0644); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dir, "cells.h5ad"), annDataExampleData, 0644)
	}
	return e
}
