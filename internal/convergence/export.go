package convergence

import (
	"encoding/csv"
	"io"
	"math"
	"strconv"
	"strings"
)

func number(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return ""
	}
	return strconv.FormatFloat(v, 'g', -1, 64)
}
func AssociationsCSV(w io.Writer, r *Result) error {
	c := csv.NewWriter(w)
	if e := c.Write([]string{"group", "allele_two_field", "locus", "typed_subjects", "group_subjects", "carriers", "shared", "excluded_subjects", "group_carrier_fraction", "carrier_wilson95_lower", "carrier_wilson95_upper", "group_noncarrier_fraction", "noncarrier_wilson95_lower", "noncarrier_wilson95_upper", "frequency_difference", "p", "log_p", "by", "log_by", "reason"}); e != nil {
		return e
	}
	for _, x := range r.Associations {
		row := []string{x.Group, x.Allele, x.Locus, strconv.Itoa(x.N), strconv.Itoa(x.GroupSubjects), strconv.Itoa(x.Carriers), strconv.Itoa(x.Shared), strconv.Itoa(x.Excluded)}
		for _, v := range []float64{float64(x.Carrier.Estimate), float64(x.Carrier.Lower), float64(x.Carrier.Upper), float64(x.Noncarrier.Estimate), float64(x.Noncarrier.Lower), float64(x.Noncarrier.Upper), float64(x.Difference), float64(x.P), float64(x.LogP), float64(x.Q), float64(x.LogQ)} {
			row = append(row, number(v))
		}
		row = append(row, x.Reason)
		if e := c.Write(row); e != nil {
			return e
		}
	}
	c.Flush()
	return c.Error()
}
func RecurrenceCSV(w io.Writer, r *Result) error {
	c := csv.NewWriter(w)
	if e := c.Write([]string{"group", "signature", "subjects_with_group", "observed_subjects", "fraction", "subject_ids", "receptor", "v_gene", "junction_aa", "subjects_with_receptor", "receptor_subject_ids"}); e != nil {
		return e
	}
	for _, g := range r.Recurrence {
		prefix := []string{g.ID, g.Label, strconv.Itoa(len(g.Subjects)), strconv.Itoa(g.ObservedSubjects), number(float64(g.Frequency)), strings.Join(g.Subjects, ";")}
		if len(g.Receptors) == 0 {
			if e := c.Write(append(prefix, "", "", "", "0", "")); e != nil {
				return e
			}
		}
		for _, rec := range g.Receptors {
			if e := c.Write(append(append([]string{}, prefix...), rec.ID, rec.V, rec.Junction, strconv.Itoa(len(rec.Subjects)), strings.Join(rec.Subjects, ";"))); e != nil {
				return e
			}
		}
	}
	c.Flush()
	return c.Error()
}
