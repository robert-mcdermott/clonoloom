package batchqc

import (
	"clonoloom/internal/diversity"
	"clonoloom/internal/genes"
	"clonoloom/internal/qc"
	"clonoloom/internal/rep"
	"fmt"
	"math"
	"sort"
	"strings"
)

// Capture keeps the complete assignment set and exact metric numerators/denominators.
// Bulk fractions use abundance; single-cell fractions use contigs, never cells as replicates.
func Capture(t *rep.Table, f qc.Filter, assignments []Assignment, o Options) (Input, error) {
	in := Input{Options: o, Unit: string(t.Unit), SingleCell: t.Cell != nil, Samples: []Sample{}}
	if t.Cell != nil {
		in.Unit = "cells"
	}
	if len(t.Samples) < 2 || len(t.Samples) > 256 {
		return in, fmt.Errorf("capture 2–256 source samples")
	}
	if in.Unit != "reads" && in.Unit != "umis" && in.Unit != "templates" && in.Unit != "cells" {
		return in, fmt.Errorf("known abundance unit required")
	}
	if len(assignments) != len(t.Samples) {
		return in, fmt.Errorf("assign every source sample, including explicit exclusions")
	}
	if math.IsNaN(f.MinCount) || math.IsInf(f.MinCount, 0) || f.MinCount < 0 || f.MaxChains < 0 {
		return in, fmt.Errorf("invalid QC thresholds")
	}
	byName := map[string]Assignment{}
	for _, a := range assignments {
		a.Subject = strings.TrimSpace(a.Subject)
		a.Batch = strings.TrimSpace(a.Batch)
		a.Exclude = strings.TrimSpace(a.Exclude)
		if _, exists := byName[a.Name]; exists {
			return in, fmt.Errorf("duplicate sample assignment")
		}
		byName[a.Name] = a
	}
	keep, removed := qc.Apply(t, f)
	statusFilter := f
	statusFilter.ProductiveOnly = false
	statusKeep, _ := qc.Apply(t, statusFilter)
	type tally struct {
		rows, kept                                          int
		total, status, productive, unknown, geneTotal, v, j float64
		badTotal, badStatus, badGenes, badCells             bool
		cells                                               map[uint32]bool
	}
	ts := make([]tally, len(t.Samples))
	for i := range ts {
		ts[i].cells = map[uint32]bool{}
	}
	for row := 0; row < t.Len(); row++ {
		p := &ts[t.Sample[row]]
		p.rows++
		w := t.Count[row]
		if in.SingleCell {
			w = 1
		}
		bad := math.IsNaN(w) || math.IsInf(w, 0) || w < 0 || w != math.Trunc(w)
		if statusKeep[row] {
			if bad {
				p.badStatus = true
			} else {
				p.status += w
				if t.Productive[row] == rep.True {
					p.productive += w
				}
				if t.Productive[row] == rep.Missing {
					p.unknown += w
				}
			}
		}
		if !keep[row] {
			continue
		}
		p.kept++
		if bad {
			p.badTotal = true
		} else {
			p.total += w
		}
		if in.SingleCell {
			if t.Cell[row] == 0 {
				p.badCells = true
			} else {
				p.cells[t.Cell[row]] = true
			}
		}
		if t.Productive[row] == rep.True {
			if bad {
				p.badGenes = true
			} else {
				p.geneTotal += w
				if t.Call(t.V[row]).Resolved(genes.LevelGene) {
					p.v += w
				}
				if t.Call(t.J[row]).Resolved(genes.LevelGene) {
					p.j += w
				}
			}
		}
	}
	for i, source := range t.Samples {
		a, ok := byName[source.Name]
		if !ok {
			return in, fmt.Errorf("source sample %s has no assignment", source.Name)
		}
		p := ts[i]
		for _, total := range []float64{p.total, p.status, p.geneTotal} {
			if total > 9e15 {
				return in, fmt.Errorf("abundance exceeds exact integer bound")
			}
		}
		s := Sample{Assignment: a, Rows: p.rows, Kept: p.kept, Removed: removed[i]}
		val := func(n, d float64, reason string) Value {
			v := Value{Value: missing(), Numerator: diversity.Number(n), Denominator: diversity.Number(d), Reason: reason}
			if reason != "" {
				v.Numerator = missing()
				v.Denominator = missing()
			} else if d == 0 {
				v.Reason = "No denominator evidence"
			} else {
				v.Value = diversity.Number(n / d)
			}
			return v
		}
		why := func(b bool) string {
			if b {
				return "Missing, noninteger or invalid contributing counts"
			}
			return ""
		}
		abundance := p.total
		abundanceReason := why(p.badTotal)
		if in.SingleCell {
			abundance = float64(len(p.cells))
			abundanceReason = ""
			if p.badCells {
				abundanceReason = "Retained contigs have missing barcodes"
			}
		}
		s.Values = []Value{val(abundance, 1, abundanceReason), val(float64(p.kept), 1, ""), val(p.productive, p.status, why(p.badStatus)), val(p.unknown, p.status, why(p.badStatus)), val(p.v, p.geneTotal, why(p.badGenes)), val(p.j, p.geneTotal, why(p.badGenes))}
		in.Samples = append(in.Samples, s)
	}
	sort.Slice(in.Samples, func(i, j int) bool { return in.Samples[i].Assignment.Name < in.Samples[j].Assignment.Name })
	return in, nil
}
