package cohort

import (
	"encoding/csv"
	"io"
	"math"
	"strconv"
)

func number(x float64) string {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return ""
	}
	return strconv.FormatFloat(x, 'g', -1, 64)
}

// CSV exports both tests and their distinct effects without conflating intervals
// for mean differences with conservative probability-of-superiority intervals.
func CSV(w io.Writer, r *Result) error {
	c := csv.NewWriter(w)
	head := []string{"feature", "unit", "layout", "reference", "comparison", "n_reference", "n_comparison", "pairs", "missing", "test", "statistic", "df", "p", "bh_adjusted", "family_slots", "test_reason", "effect", "effect_unit", "estimate", "lower", "upper", "confidence", "interval_method", "effect_reason", "null_method", "draws", "tail", "nonzero_pairs"}
	if err := c.Write(head); err != nil {
		return err
	}
	for _, row := range r.Comparisons {
		for i, t := range []Test{row.Parametric, row.Rank} {
			e := row.Mean
			if i == 1 {
				e = row.Probability
			}
			values := []string{row.Feature.ID, row.Feature.Unit, r.Layout, r.Levels[0], r.Levels[1], strconv.Itoa(row.NReference), strconv.Itoa(row.NComparison), strconv.Itoa(row.Pairs), strconv.Itoa(row.Missing), t.Name, number(float64(t.Statistic)), number(float64(t.DF)), number(float64(t.P)), number(float64(t.Adjusted)), strconv.Itoa(r.Family), t.Reason, e.Name, e.Unit, number(float64(e.Estimate)), number(float64(e.Lower)), number(float64(e.Upper)), number(r.Options.Confidence), e.Interval, e.Reason, t.NullMethod, strconv.Itoa(t.Draws), strconv.Itoa(t.Tail), strconv.Itoa(t.Nonzero)}
			if err := c.Write(values); err != nil {
				return err
			}
		}
	}
	c.Flush()
	return c.Error()
}
