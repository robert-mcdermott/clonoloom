package pseudobulkde

import (
	"encoding/csv"
	"io"
	"math"
	"strconv"
)

func num(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return ""
	}
	return strconv.FormatFloat(v, 'g', -1, 64)
}
func CSV(w io.Writer, r *Result) error {
	c := csv.NewWriter(w)
	if e := c.Write([]string{"feature_id", "feature_name", "eligible", "reason", "mean_normalized_count", "log2_fold", "se_log2", "lower_log2", "upper_log2", "confidence", "dispersion", "likelihood_ratio", "p", "bh_p", "family_slots", "residual_df"}); e != nil {
		return e
	}
	for _, v := range r.Rows {
		if e := c.Write([]string{v.Feature.ID, v.Feature.Name, strconv.FormatBool(v.Eligible), v.Reason, num(v.Mean), num(float64(v.Log2Fold)), num(float64(v.SE)), num(float64(v.Lower)), num(float64(v.Upper)), num(r.Options.Confidence), num(float64(v.Dispersion)), num(float64(v.LR)), num(float64(v.P)), num(float64(v.Adjusted)), strconv.Itoa(r.Family), strconv.Itoa(r.Design.ResidualDF)}); e != nil {
			return e
		}
	}
	c.Flush()
	return c.Error()
}
func MatrixCSV(w io.Writer, r *Result) error {
	c := csv.NewWriter(w)
	head := []string{"sample", "subject", "size_category", "cells", "raw_library_total", "size_factor", "log_offset"}
	head = append(head, r.Design.Columns...)
	if e := c.Write(head); e != nil {
		return e
	}
	for i, o := range r.Observations {
		v := []string{o.Sample, o.Subject, o.Bin, strconv.Itoa(o.Cells), strconv.FormatUint(o.Total, 10), num(r.SizeFactors[i]), num(r.Offsets[i])}
		for _, x := range r.Design.X[i] {
			v = append(v, num(x))
		}
		if e := c.Write(v); e != nil {
			return e
		}
	}
	c.Flush()
	return c.Error()
}
func CountsCSV(w io.Writer, r *Result) error {
	c := csv.NewWriter(w)
	head := []string{"feature_id", "feature_name"}
	for _, o := range r.Observations {
		head = append(head, strconv.Quote(o.Sample)+" / "+strconv.Quote(o.Bin))
	}
	if e := c.Write(head); e != nil {
		return e
	}
	for _, row := range r.Rows {
		v := []string{row.Feature.ID, row.Feature.Name}
		for _, y := range row.Counts {
			v = append(v, strconv.FormatUint(y, 10))
		}
		if e := c.Write(v); e != nil {
			return e
		}
	}
	c.Flush()
	return c.Error()
}
