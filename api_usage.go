package main

// Gene usage, junction lengths and amino-acid composition (internal/usage;
// clonoloom-spec/requirements.md, R3), as an analysis (api_analysis.go): every sample's, every
// group's (the spec's groups, each sample counted once), with intervals from a bootstrap of each
// sample's clonotypes.

import (
	"context"

	"clonoloom/internal/artifacts"
	"clonoloom/internal/jobs"
	"clonoloom/internal/usage"
)

var usageAnalysis = analysis{kind: "usage", prefix: "use-", version: 1, title: "Gene usage", grouped: true, work: (*app).usageWork}

// usageDocument is a usage artifact's usage.json: Grouping the samples' groups as asked.
type usageDocument struct {
	analysisHead
	Grouping   map[string]string `json:"grouping,omitempty"`
	Samples    []usage.Sample    `json:"samples"`
	Groups     []usage.Group     `json:"groups"`
	Profiled   map[string][]int  `json:"profiled"`
	Replicates int               `json:"replicates"`
}

func (a *app) usageWork(k analysis, spec analysisSpec) jobs.Func {
	return func(ctx context.Context, run *jobs.Run) error {
		ds, head, table, _, err := a.samplesOf(k, spec, run)
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		run.Progress(0.1, "Counting genes, lengths and amino acids")
		res := usage.Measure(ds.Result.Table, table, spec.Groups, spec.Options)
		doc := usageDocument{analysisHead: head, Grouping: spec.Groups, Samples: res.Samples, Groups: res.Groups, Profiled: res.Profiled, Replicates: res.Replicates}
		return a.keepAnalysis(k, spec, ds, run, doc, len(doc.Samples),
			artifacts.Method{Name: "V, D, J and C gene usage, V–J pairs, junction lengths and amino-acid composition, with bootstrap intervals over clonotypes",
				Citation: "Lefranc MP, Pommié C, Ruiz M, et al. IMGT unique numbering for immunoglobulin and T cell receptor variable domains and Ig superfamily V-like domains. Dev Comp Immunol. 2003;27:55–77. doi:10.1016/S0145-305X(02)00039-3"},
			map[string]any{"samples": len(doc.Samples), "unit": head.Unit, "definition": head.Definition.Name, "groups": len(doc.Groups)})
	}
}
