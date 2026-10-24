package expression

import (
	"context"
	"fmt"

	"clonoloom/internal/rep"
)

type Links struct {
	Sample             int    `json:"sample"`
	SampleName         string `json:"sampleName"`
	ExpressionCells    int    `json:"expressionCells"`
	ReceptorCells      int    `json:"receptorCells"`
	Matched            int    `json:"matched"`
	ExpressionOnly     int    `json:"expressionOnly"`
	ReceptorOnly       int    `json:"receptorOnly"`
	MissingBarcodeRows int    `json:"missingBarcodeRows"`
	ChainRows          []int  `json:"-"`
}

// Link is source coverage before QC or clonotyping. Duplicate chain rows count once as a receptor
// barcode. Neither barcode suffixes nor sample names are rewritten. No cross-sample fallback.
func Link(ctx context.Context, m *Matrix, t *rep.Table, sample int) (Links, error) {
	result := Links{Sample: sample, ExpressionCells: len(m.Barcodes)}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if sample < 0 || sample >= len(t.Samples) {
		return result, fmt.Errorf("select a receptor sample")
	}
	if t.Cell == nil {
		return result, fmt.Errorf("the receptor dataset has no cell barcodes")
	}
	result.SampleName = t.Samples[sample].Name
	counts := map[string]int{}
	for row, s := range t.Sample {
		if row%65536 == 0 {
			if err := ctx.Err(); err != nil {
				return result, err
			}
		}
		if int(s) != sample {
			continue
		}
		barcode := t.Dict.Get(t.Cell[row])
		if barcode == "" {
			result.MissingBarcodeRows++
			continue
		}
		counts[barcode]++
	}
	result.ReceptorCells = len(counts)
	result.ChainRows = make([]int, len(m.Barcodes))
	for i, b := range m.Barcodes {
		if i%65536 == 0 {
			if err := ctx.Err(); err != nil {
				return result, err
			}
		}
		result.ChainRows[i] = counts[b]
		if counts[b] > 0 {
			result.Matched++
		}
	}
	result.ExpressionOnly = result.ExpressionCells - result.Matched
	result.ReceptorOnly = result.ReceptorCells - result.Matched
	return result, nil
}
