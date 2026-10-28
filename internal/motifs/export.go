package motifs

import (
	"encoding/csv"
	"io"
	"math"
	"strconv"
	"strings"
)

func CSV(w io.Writer, r *Result) error {
	c := csv.NewWriter(w)
	if e := c.Write([]string{"id", "kind", "v", "length", "signature", "allowed", "first_start_1based", "last_start_1based", "query_matches", "query_total", "background_matches", "background_total", "fold", "p", "log_p", "by", "log_by", "minus_log10_by", "reported", "reason", "query_ids", "background_ids"}); e != nil {
		return e
	}
	for _, g := range r.Groups {
		row := []string{g.ID, g.Kind, g.V, strconv.Itoa(g.Length), g.Motif, g.Allowed, strconv.Itoa(g.Start), strconv.Itoa(g.End), strconv.Itoa(len(g.Query)), strconv.Itoa(g.QueryTotal), strconv.Itoa(len(g.Background)), strconv.Itoa(g.BackgroundTotal)}
		for _, v := range []float64{float64(g.Fold), float64(g.P), float64(g.LogP), float64(g.Q), float64(g.LogQ), float64(g.Score)} {
			s := ""
			if !math.IsNaN(v) && !math.IsInf(v, 0) {
				s = strconv.FormatFloat(v, 'g', -1, 64)
			}
			row = append(row, s)
		}
		ids := func(indexes []int, records []Record) string {
			out := []string{}
			for _, i := range indexes {
				out = append(out, records[i].ID)
			}
			return strings.Join(out, ";")
		}
		row = append(row, strconv.FormatBool(g.Reported), g.Reason, ids(g.Query, r.Query), ids(g.Background, r.Background))
		if e := c.Write(row); e != nil {
			return e
		}
	}
	c.Flush()
	return c.Error()
}
