package contamination

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
func MatchesCSV(w io.Writer, r *Result) error {
	c := csv.NewWriter(w)
	if e := c.Write([]string{"sample_a", "subject_a", "sample_b", "subject_b", "receptor", "v_gene", "j_gene", "junction_nt", "junction_aa", "count_a", "count_b", "fraction_a", "fraction_b", "fraction_ratio", "review_threshold", "pattern", "nt_pgen", "pgen_context"}); e != nil {
		return e
	}
	recs := map[string]Receptor{}
	for _, x := range r.Evidence.Receptors {
		recs[x.ID] = x
	}
	for _, m := range r.Matches {
		a, b := r.Evidence.Samples[m.A].Sample, r.Evidence.Samples[m.B].Sample
		x := recs[m.Receptor]
		if e := c.Write([]string{a.Name, a.Subject, b.Name, b.Subject, x.ID, x.V, x.J, x.NT, x.AA, num(m.CountA), num(m.CountB), num(m.FractionA), num(m.FractionB), num(m.Ratio), strconv.FormatBool(m.Review), m.Pattern, num(float64(x.Pgen)), m.PgenClass}); e != nil {
			return e
		}
	}
	c.Flush()
	return c.Error()
}
func PairsCSV(w io.Writer, r *Result) error {
	c := csv.NewWriter(w)
	if e := c.Write([]string{"sample_a", "sample_b", "shared_nt_identities", "review_threshold", "high_pgen", "unknown_pgen", "shared_abundance_a", "shared_abundance_b", "shared_fraction_a", "shared_fraction_b", "reason"}); e != nil {
		return e
	}
	for _, p := range r.Pairs {
		if e := c.Write([]string{r.Evidence.Samples[p.A].Sample.Name, r.Evidence.Samples[p.B].Sample.Name, strconv.Itoa(p.Shared), strconv.Itoa(p.Review), strconv.Itoa(p.HighPgen), strconv.Itoa(p.UnknownPgen), num(p.SharedA), num(p.SharedB), num(float64(p.FractionA)), num(float64(p.FractionB)), p.Reason}); e != nil {
			return e
		}
	}
	c.Flush()
	return c.Error()
}
