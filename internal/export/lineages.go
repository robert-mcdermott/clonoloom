package export

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"clonoloom/internal/bcell"
)

func LineageTrees(w io.Writer, r bcell.LineageResult) error {
	c := csv.NewWriter(w)
	if err := c.Write([]string{"tree_id", "family_id", "locus", "v_call", "j_call", "source_rows", "included_rows", "panel_sites", "omitted_reference_sites", "taxa", "changes", "search", "examined", "optimal_binary_trees", "budget_limited", "excluded"}); err != nil {
		return err
	}
	for _, t := range r.Trees {
		ties := ""
		if t.OptimalTrees != nil {
			ties = strconv.Itoa(*t.OptimalTrees)
		}
		score := ""
		if t.Excluded == "" {
			score = strconv.Itoa(t.Score)
		}
		if err := c.Write([]string{t.ID, t.Family, t.Locus, t.V, t.J, strconv.Itoa(t.Rows), strconv.Itoa(t.Included), strconv.Itoa(t.Sites), strconv.Itoa(t.Omitted), strconv.Itoa(t.Taxa), score, t.Search, strconv.Itoa(t.Examined), ties, strconv.FormatBool(t.BudgetLimited), t.Excluded}); err != nil {
			return err
		}
	}
	c.Flush()
	return c.Error()
}
func LineageNodes(w io.Writer, r bcell.LineageResult) error {
	c := csv.NewWriter(w)
	if err := c.Write([]string{"tree_id", "node_id", "parent_id", "kind", "changes", "depth", "distance", "source_rows", "split_trees", "chosen_panel_sequence", "possible_panel_states", "observed_annotations_json"}); err != nil {
		return err
	}
	for _, t := range r.Trees {
		for _, n := range t.Nodes {
			split := ""
			if n.SplitTrees != nil {
				split = strconv.Itoa(*n.SplitTrees)
			}
			ann, err := json.Marshal(n.Annotations)
			if err != nil {
				return err
			}
			if err = c.Write([]string{t.ID, n.ID, n.Parent, n.Kind, strconv.Itoa(n.Changes), strconv.Itoa(n.Depth), strconv.Itoa(n.Distance), strconv.Itoa(n.Rows), split, n.Sequence, n.Possible, string(ann)}); err != nil {
				return err
			}
		}
	}
	c.Flush()
	return c.Error()
}
func LineageMembers(w io.Writer, r bcell.LineageResult) error {
	c := csv.NewWriter(w)
	if err := c.Write([]string{"row_id", "tree_id", "node_id", "family_id", "observation_id", "sample", "locus", "source", "source_sha256", "line", "sequence_id", "isotype", "timepoint", "tissue", "excluded"}); err != nil {
		return err
	}
	for _, r := range r.Members {
		if err := c.Write([]string{r.ID, r.Tree, r.Node, r.Family, r.Observation, r.Sample, r.Locus, r.Source.Name, r.Source.SHA256, strconv.FormatUint(uint64(r.Line), 10), r.SequenceID, r.Isotype, r.Timepoint, r.Tissue, r.Excluded}); err != nil {
			return err
		}
	}
	c.Flush()
	return c.Error()
}
func LineageFASTA(w io.Writer, r bcell.LineageResult) error {
	for _, t := range r.Trees {
		for _, n := range t.Nodes {
			if _, err := fmt.Fprintf(w, ">%s tree=%s kind=%s panel-only chosen-assignment\n%s\n", n.ID, t.ID, n.Kind, n.Sequence); err != nil {
				return err
			}
		}
	}
	return nil
}
func LineageNewick(w io.Writer, r bcell.LineageResult) error {
	for _, t := range r.Trees {
		if t.Newick != "" {
			if _, err := fmt.Fprintln(w, t.Newick); err != nil {
				return err
			}
		}
	}
	return nil
}
