package export

import (
	"encoding/csv"
	"io"
	"strconv"

	"clonoloom/internal/clonotype"
	"clonoloom/internal/specificity"
)

// SpecificityMatches writes every chain–record match, retaining database row identity.
func SpecificityMatches(w io.Writer, cs []clonotype.Clonotype, r specificity.Result) error {
	c := csv.NewWriter(w)
	if err := c.Write([]string{"clonotype", "chain", "locus", "junction", "distance", "database_row", "complex", "database_cdr3", "v", "j", "receptor_species", "epitope", "antigen", "antigen_species", "mhc_a", "mhc_b", "mhc_class", "score", "reference", "method"}); err != nil {
		return err
	}
	for _, m := range r.Matches {
		x := r.Records[m.Record]
		ch := cs[m.Clonotype].Chains[m.Chain]
		if err := c.Write([]string{cs[m.Clonotype].ID, strconv.Itoa(m.Chain + 1), ch.Locus, ch.Junction, strconv.Itoa(m.Distance), strconv.Itoa(x.Row), x.Complex, x.CDR3, x.V, x.J, x.Species, x.Epitope, x.Antigen, x.AntigenSpecies, x.MHCA, x.MHCB, x.MHCClass, strconv.Itoa(x.Score), x.Reference, x.Method}); err != nil {
			return err
		}
	}
	c.Flush()
	return c.Error()
}

// SpecificityShares writes per-sample counts and shares; empty denominators stay blank.
func SpecificityShares(w io.Writer, names []string, r specificity.Result) error {
	c := csv.NewWriter(w)
	if err := c.Write([]string{"epitope", "antigen", "antigen_species", "sample", "count", "share"}); err != nil {
		return err
	}
	for _, e := range r.Epitopes {
		for i, n := range names {
			if err := c.Write([]string{e.Epitope, e.Antigen, e.Species, n, number(e.Counts[i]), number(float64(e.Shares[i]))}); err != nil {
				return err
			}
		}
	}
	c.Flush()
	return c.Error()
}
