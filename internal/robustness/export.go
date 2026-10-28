package robustness

import (
	"encoding/csv"
	"io"
	"math"
	"strconv"
)

func CSV(w io.Writer, r *Result) error {
	c := csv.NewWriter(w)
	if e := c.Write([]string{"definition", "definition_id", "definition_rules", "feature", "unit", "mean_difference", "mean_lower", "mean_upper", "mean_p", "within_definition_bh", "across_audit_by", "probability", "probability_lower", "probability_upper", "rank_p", "rank_within_bh", "rank_across_by", "mean_reason", "rank_reason", "definition_failure", "audit_family", "direction_summary", "interval_summary"}); e != nil {
		return e
	}
	for _, v := range r.Variants {
		for j, s := range r.Summaries {
			row := []string{v.Definition.Name, v.Definition.ID(), v.Definition.Describe(), s.Feature.ID, s.Feature.Unit}
			values := make([]string, 14)
			if v.Analysis != nil {
				a := v.Analysis.Comparisons[j]
				xs := []float64{float64(a.Mean.Estimate), float64(a.Mean.Lower), float64(a.Mean.Upper), float64(a.Parametric.P), float64(a.Parametric.Adjusted), float64(v.Across[j].Mean), float64(a.Probability.Estimate), float64(a.Probability.Lower), float64(a.Probability.Upper), float64(a.Rank.P), float64(a.Rank.Adjusted), float64(v.Across[j].Rank)}
				for i, x := range xs {
					if !math.IsNaN(x) && !math.IsInf(x, 0) {
						values[i] = strconv.FormatFloat(x, 'g', -1, 64)
					}
				}
				values[12] = a.Mean.Reason
				values[13] = a.Rank.Reason
			}
			row = append(row, values...)
			row = append(row, v.Failure, strconv.Itoa(r.Family), s.Direction, s.Uncertainty)
			if e := c.Write(row); e != nil {
				return e
			}
		}
	}
	c.Flush()
	return c.Error()
}
