package export

// Network exports retain every scientific node and edge, independent of the
// drawing. The enclosing archive supplies the definition, options and citations
// in its NOTICE (api_export.go).

import (
	"encoding/csv"
	"io"
	"strconv"

	"clonoloom/internal/clonotype"
	"clonoloom/internal/network"
)

func NetworkMembership(w io.Writer, nodes []clonotype.Clonotype, g *network.Graph, unit string) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"clonotype_id", "cluster", "junctions", "abundance", "unit", "samples"}); err != nil {
		return err
	}
	for i, c := range nodes {
		if err := cw.Write([]string{c.ID, strconv.Itoa(g.Membership[i]), c.Junctions(), number(c.Total), unit, strconv.Itoa(c.Samples)}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func NetworkEdges(w io.Writer, nodes []clonotype.Clonotype, g *network.Graph) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"clonotype_a", "clonotype_b", "distance"}); err != nil {
		return err
	}
	for _, e := range g.Edges {
		if err := cw.Write([]string{nodes[e.A].ID, nodes[e.B].ID, strconv.Itoa(e.Distance)}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}
