package main

// Diversity (internal/diversity; clonoloom-spec/requirements.md, R1 and R5), as an analysis
// (api_analysis.go): every sample's measures, each with its interval.

import (
	"context"

	"clonoloom/internal/artifacts"
	"clonoloom/internal/diversity"
	"clonoloom/internal/jobs"
)

var diversityAnalysis = analysis{kind: "diversity", prefix: "div-", version: 1, title: "Diversity", work: (*app).diversityWork}

// diversityDocument is a diversity artifact's diversity.json.
type diversityDocument struct {
	analysisHead
	Samples []diversitySample `json:"samples"`
}

type diversitySample struct {
	Name   string           `json:"name"`
	Result diversity.Result `json:"result"`
}

// diversityWork measures each sample of the dataset and keeps the result as an artifact.
func (a *app) diversityWork(k analysis, spec analysisSpec) jobs.Func {
	return func(ctx context.Context, run *jobs.Run) error {
		ds, head, abundances, err := a.samplesOf(k, spec, run)
		if err != nil {
			return err
		}
		doc := diversityDocument{analysisHead: head}
		for i, s := range ds.Result.Table.Samples {
			if err := ctx.Err(); err != nil {
				return err
			}
			run.Progress(float64(i)/float64(len(abundances)), "Measuring "+s.Name)
			opt := spec.Options
			opt.Stream = uint64(i)
			doc.Samples = append(doc.Samples, diversitySample{Name: s.Name, Result: diversity.Measure(abundances[i], opt)})
		}
		return a.keepAnalysis(k, spec, ds, run, doc, len(doc.Samples),
			artifacts.Method{Name: "Hill numbers, Chao1, ACE and the asymptotic Shannon and Simpson diversity, with bootstrap intervals",
				Citation: "Chao A, Gotelli NJ, Hsieh TC, et al. Rarefaction and extrapolation with Hill numbers. Ecol Monogr. 2014;84:45–67. doi:10.1890/13-0133.1"},
			map[string]any{"samples": len(doc.Samples), "unit": head.Unit, "definition": head.Definition.Name})
	}
}
