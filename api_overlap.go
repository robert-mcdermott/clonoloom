package main

// Overlap between samples (internal/overlap; clonoloom-spec/requirements.md, R4), as an analysis
// (api_analysis.go): every pair of samples' overlap, observed and rarefied to a common depth, each
// with its interval; the pairs of one subject and of two, of one condition and of two, summarized;
// and the public clonotypes, by subject (the spec's subjects).

import (
	"context"

	"clonoloom/internal/artifacts"
	"clonoloom/internal/jobs"
	"clonoloom/internal/overlap"
)

var overlapAnalysis = analysis{kind: "overlap", prefix: "ovl-", version: 1, title: "Overlap", grouped: true, subjects: true, work: (*app).overlapWork}

// overlapDocument is an overlap artifact's overlap.json: Subjects and Grouping the samples'
// subjects and conditions as asked.
type overlapDocument struct {
	analysisHead
	Subjects   map[string]string `json:"subjects,omitempty"`
	Grouping   map[string]string `json:"grouping,omitempty"`
	Samples    []overlap.Sample  `json:"samples"`
	Depth      float64           `json:"depth"`
	Pairs      []overlap.Pair    `json:"pairs"`
	Summaries  []overlap.Summary `json:"summaries"`
	Public     overlap.Public    `json:"public"`
	Replicates int               `json:"replicates"`
}

func (a *app) overlapWork(k analysis, spec analysisSpec) jobs.Func {
	return func(ctx context.Context, run *jobs.Run) error {
		ds, head, table, _, err := a.samplesOf(k, spec, run)
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		run.Progress(0.1, "Measuring every pair of samples")
		var names []string
		for _, s := range ds.Result.Table.Samples {
			names = append(names, s.Name)
		}
		res := overlap.Measure(table, names, spec.Subjects, spec.Groups, spec.Options)
		doc := overlapDocument{analysisHead: head, Subjects: spec.Subjects, Grouping: spec.Groups, Samples: res.Samples, Depth: res.Depth,
			Pairs: res.Pairs, Summaries: res.Summaries, Public: res.Public, Replicates: res.Replicates}
		return a.keepAnalysis(k, spec, ds, run, doc, len(doc.Samples),
			artifacts.Method{Name: "Shared clonotypes, Jaccard, Sørensen–Dice and the overlap coefficient, observed and rarefied; Morisita–Horn and Morisita's index; public clonotypes",
				Citation: "Morisita M. Measuring of interspecific association and similarity between communities. Mem Fac Sci Kyushu Univ Ser E Biol. 1959;3:65–80"},
			map[string]any{"samples": len(doc.Samples), "pairs": len(doc.Pairs), "unit": head.Unit, "definition": head.Definition.Name, "depth": doc.Depth})
	}
}
