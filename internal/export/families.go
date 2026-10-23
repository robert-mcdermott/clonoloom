package export

import (
	"clonoloom/internal/bcell"
	"encoding/csv"
	"io"
	"strconv"
)

func familyNumber(v *float64) string {
	if v == nil {
		return ""
	}
	return number(*v)
}

// Each source row retains its observation and family identity, including exclusions.
// Abundance belongs to the observation; it must not be summed over chain rows.
func FamilyMembers(w io.Writer, r bcell.FamilyResult) error {
	c := csv.NewWriter(w)
	if e := c.Write([]string{"observation_id", "family_id", "representative_id", "sample", "subject", "scope", "cell_id", "observation_abundance", "unit", "excluded", "source", "source_sha256", "line", "sequence_id", "locus"}); e != nil {
		return e
	}
	for _, o := range r.Observations {
		for _, row := range o.Rows {
			if e := c.Write([]string{o.ID, o.Family, o.Representative, o.Sample, o.Subject, o.Scope, o.Cell, familyNumber(o.Abundance), r.Unit, o.Excluded, row.Source.Name, row.Source.SHA256, strconv.FormatUint(uint64(row.Line), 10), row.SequenceID, row.Locus}); e != nil {
				return e
			}
		}
	}
	c.Flush()
	return c.Error()
}
func FamilyNearest(w io.Writer, r bcell.FamilyResult) error {
	c := csv.NewWriter(w)
	if e := c.Write([]string{"representative_id", "family_id", "scope", "observations", "heavy_v", "heavy_j", "heavy_junction", "light_locus", "light_v", "light_j", "light_junction", "nearest_distance", "neighbor_id"}); e != nil {
		return e
	}
	for _, o := range r.Representatives {
		light := bcell.FamilyChain{}
		if o.Light != nil {
			light = *o.Light
		}
		if e := c.Write([]string{o.ID, o.Family, o.Scope, strconv.Itoa(o.Observations), o.Heavy.V, o.Heavy.J, o.Heavy.Junction, light.Locus, light.V, light.J, light.Junction, familyNumber(o.Nearest), o.Neighbor}); e != nil {
			return e
		}
	}
	c.Flush()
	return c.Error()
}
func FamilyAmounts(w io.Writer, r bcell.FamilyResult) error {
	c := csv.NewWriter(w)
	if e := c.Write([]string{"family_id", "scope", "representatives", "sample", "observations", "abundance", "unit"}); e != nil {
		return e
	}
	for _, o := range r.Families {
		for _, s := range o.Samples {
			if e := c.Write([]string{o.ID, o.Scope, strconv.Itoa(o.Representatives), s.Sample, strconv.Itoa(s.Observations), familyNumber(s.Abundance), r.Unit}); e != nil {
				return e
			}
		}
	}
	c.Flush()
	return c.Error()
}
