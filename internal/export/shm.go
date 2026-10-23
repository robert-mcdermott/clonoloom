package export

import (
	"encoding/csv"
	"io"
	"strconv"

	"clonoloom/internal/bcell"
)

var shmHeaders = []string{"evaluable_bases", "substitutions", "frequency", "masked", "ambiguous", "gaps", "evaluable_codons", "replacement_single_hit", "silent_single_hit", "unclassified_substitutions", "multi_hit_codons", "replacement_codons", "silent_codons", "stop_codons"}

func shmValues(c bcell.SHMCounts, missing bool) []string {
	if missing {
		return make([]string, len(shmHeaders))
	}
	out := []string{}
	for _, n := range []int{c.EvaluableBases, c.Mutations} {
		out = append(out, strconv.Itoa(n))
	}
	f := ""
	if c.Frequency != nil {
		f = strconv.FormatFloat(*c.Frequency, 'g', -1, 64)
	}
	out = append(out, f)
	for _, n := range []int{c.Masked, c.Ambiguous, c.Gaps, c.EvaluableCodons, c.Replacement, c.Silent, c.Unclassified, c.MultiHitCodons, c.ReplacementCodons, c.SilentCodons, c.StopCodons} {
		out = append(out, strconv.Itoa(n))
	}
	return out
}
func SHMRows(w io.Writer, r bcell.SHMResult) error {
	c := csv.NewWriter(w)
	if err := c.Write(append([]string{"row_id", "family_id", "observation_id", "sample", "locus", "source", "source_sha256", "line", "sequence_id", "v_call", "j_call", "excluded", "codon_exclusion", "frame_anchor", "region", "unavailable"}, shmHeaders...)); err != nil {
		return err
	}
	for _, row := range r.Rows {
		for _, region := range row.Regions {
			anchor := ""
			if row.FrameAnchor != 0 {
				anchor = strconv.Itoa(row.FrameAnchor)
			}
			values := []string{row.ID, row.Family, row.Observation, row.Sample, row.Locus, row.Source.Name, row.Source.SHA256, strconv.FormatUint(uint64(row.Line), 10), row.SequenceID, row.V, row.J, row.Excluded, row.CodonExclusion, anchor, region.Region, region.Unavailable}
			if err := c.Write(append(values, shmValues(region.SHMCounts, region.Unavailable != "")...)); err != nil {
				return err
			}
		}
	}
	c.Flush()
	return c.Error()
}
func SHMSummaries(w io.Writer, r bcell.SHMResult) error {
	c := csv.NewWriter(w)
	if err := c.Write(append([]string{"sample", "locus", "region", "source_rows", "available_rows", "rows_with_evaluable_codons"}, shmHeaders...)); err != nil {
		return err
	}
	for _, s := range r.Summaries {
		if err := c.Write(append([]string{s.Sample, s.Locus, s.Region, strconv.Itoa(s.Rows), strconv.Itoa(s.AvailableRows), strconv.Itoa(s.CodonRows)}, shmValues(s.SHMCounts, s.AvailableRows == 0)...)); err != nil {
			return err
		}
	}
	c.Flush()
	return c.Error()
}
