package export

import (
	"clonoloom/internal/bcell"
	"encoding/csv"
	"io"
	"strconv"
	"strings"
)

// Isotypes writes complete tables, independent of the displayed page or filters.
func Isotypes(w io.Writer, r bcell.IsotypeResult, kind string) error {
	c := csv.NewWriter(w)
	write := func(values ...string) { _ = c.Write(values) }
	n := strconv.Itoa
	switch kind {
	case "usage":
		write("family_id", "sample", "timepoint", "tissue", "resolution", "state", "observations", "total_assigned_observations", "observation_share", "abundance", "total_abundance", "abundance_share", "missing_abundance", "total_missing_abundance", "observation_unit", "abundance_unit")
		for _, u := range r.Usage {
			write(u.Family, u.Sample, u.Timepoint, u.Tissue, u.Level, u.State, n(u.Observations), n(u.Total), strconv.FormatFloat(u.Share, 'g', -1, 64), familyNumber(u.Abundance), familyNumber(u.TotalAbundance), familyNumber(u.AbundanceShare), n(u.MissingAbundance), n(u.TotalMissingAbundance), r.Unit, r.AbundanceUnit)
		}
	case "families":
		write("family_id", "scope", "observations", "resolved_class", "resolved_gene", "samples", "classes", "constant_genes", "evidence")
		for _, f := range r.Families {
			write(f.ID, f.Scope, n(f.Observations), n(f.ResolvedClass), n(f.ResolvedGene), strings.Join(f.Samples, ";"), strings.Join(f.Classes, ";"), strings.Join(f.Genes, ";"), f.Evidence)
		}
	case "cooccurrence":
		write("left_gene", "right_gene", "families", "total_families", "families_with_resolved_gene")
		for _, p := range r.Pairs {
			write(p.Left, p.Right, n(p.Families), n(p.TotalFamilies), n(p.EvaluableFamilies))
		}
	case "observations":
		write("observation_id", "family_id", "sample", "subject", "scope", "cell_id", "timepoint", "tissue", "class", "constant_gene", "issue", "excluded", "abundance")
		for _, o := range r.Observations {
			write(o.ID, o.Family, o.Sample, o.Subject, o.Scope, o.Cell, o.Timepoint, o.Tissue, o.Class, o.Gene, o.Issue, o.Excluded, familyNumber(o.Abundance))
		}
	case "sources":
		write("observation_id", "family_id", "row_id", "sample", "locus", "source", "source_sha256", "line", "sequence_id", "c_call")
		for _, o := range r.Observations {
			for _, s := range o.Rows {
				write(o.ID, o.Family, s.ID, o.Sample, s.Locus, s.Source.Name, s.Source.SHA256, strconv.FormatUint(uint64(s.Line), 10), s.SequenceID, s.Call)
			}
		}
	}
	c.Flush()
	return c.Error()
}
