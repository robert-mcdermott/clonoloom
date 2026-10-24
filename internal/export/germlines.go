package export

import (
	"clonoloom/internal/bcell"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
)

func GermlineRows(w io.Writer, r bcell.GermlineResult) error {
	c := csv.NewWriter(w)
	if err := c.Write([]string{"row_id", "family_id", "observation_id", "sample", "locus", "source", "source_sha256", "line", "sequence_id", "v_call", "j_call", "excluded", "sequence_alignment", "germline_alignment_d_mask", "segment_labels"}); err != nil {
		return err
	}
	for _, x := range r.Rows {
		if err := c.Write([]string{x.ID, x.Family, x.Observation, x.Sample, x.Locus, x.Source.Name, x.Source.SHA256, strconv.FormatUint(uint64(x.Line), 10), x.SequenceID, x.V, x.J, x.Excluded, x.Query, x.Germline, x.Regions}); err != nil {
			return err
		}
	}
	c.Flush()
	return c.Error()
}
func GermlineFASTA(w io.Writer, r bcell.GermlineResult) error {
	for _, x := range r.Rows {
		if x.Excluded != "" {
			continue
		}
		if _, err := fmt.Fprintf(w, ">%s family=%s locus=%s\n%s\n", x.ID, x.Family, x.Locus, x.Germline); err != nil {
			return err
		}
	}
	return nil
}
