package countmodel

import (
	"encoding/csv"
	"io"
	"strconv"
)

func num(x float64) string {
	if !finite(x) {
		return ""
	}
	return strconv.FormatFloat(x, 'g', -1, 64)
}
func CSV(w io.Writer, r *Result) error {
	c := csv.NewWriter(w)
	if e := c.Write([]string{"clonotype", "eligible", "reason", "mean_cpm", "log2_fold", "se_log2", "lower_log2", "upper_log2", "confidence", "dispersion", "likelihood_ratio", "p", "bh_p", "family_slots", "residual_df"}); e != nil {
		return e
	}
	for _, v := range r.Rows {
		if e := c.Write([]string{v.ID, strconv.FormatBool(v.Eligible), v.Reason, num(v.MeanCPM), num(float64(v.Log2Fold)), num(float64(v.SE)), num(float64(v.Lower)), num(float64(v.Upper)), num(r.Options.Confidence), num(float64(v.Dispersion)), num(float64(v.LR)), num(float64(v.P)), num(float64(v.Adjusted)), strconv.Itoa(r.Family), strconv.Itoa(r.Design.ResidualDF)}); e != nil {
			return e
		}
	}
	c.Flush()
	return c.Error()
}
func MatrixCSV(w io.Writer, r *Result) error {
	c := csv.NewWriter(w)
	head := []string{"sample", "subject", "group", "batch", "library_total", "log_offset"}
	head = append(head, r.Design.Columns...)
	if e := c.Write(head); e != nil {
		return e
	}
	for i, row := range r.Design.Rows {
		v := []string{row.Name, row.Subject, row.Group, row.Batch, num(r.Totals[i]), num(r.Offsets[i])}
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
	head := []string{"clonotype"}
	for _, s := range r.Design.Rows {
		head = append(head, s.Name)
	}
	if e := c.Write(head); e != nil {
		return e
	}
	for _, row := range r.Rows {
		v := []string{row.ID}
		for _, y := range row.Values {
			v = append(v, num(y))
		}
		if e := c.Write(v); e != nil {
			return e
		}
	}
	c.Flush()
	return c.Error()
}
