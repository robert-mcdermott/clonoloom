package main

// Rarefaction and extrapolation (internal/diversity; clonoloom-spec/requirements.md, R2 and Q3), as
// an analysis (api_analysis.go): every sample's curves, the levels the samples are best compared
// at, and each sample's values there, each with its interval. With the same options, the bootstrap
// draws the same replicates as the diversity measures.

import (
	"context"

	"clonoloom/internal/artifacts"
	"clonoloom/internal/diversity"
	"clonoloom/internal/jobs"
)

var rarefactionAnalysis = analysis{kind: "rarefaction", prefix: "rar-", version: 1, title: "Rarefaction", work: (*app).rarefactionWork}

// rarefactionDocument is a rarefaction artifact's rarefaction.json. Levels are those suggested for
// the samples (none when no sample can be rarefied); each sample's AtSize are its values at
// Levels.Size and Levels.ExtrapolatedSize, its AtCoverage at Levels.Coverage and
// Levels.ExtrapolatedCoverage.
type rarefactionDocument struct {
	analysisHead
	Knots   int                        `json:"knots"`
	Levels  *diversity.SuggestedLevels `json:"levels"`
	Samples []rarefactionSample        `json:"samples"`
}

type rarefactionSample struct {
	Name   string           `json:"name"`
	Curves diversity.Curves `json:"curves"`
}

func (a *app) rarefactionWork(k analysis, spec analysisSpec) jobs.Func {
	return func(ctx context.Context, run *jobs.Run) error {
		ds, head, _, abundances, err := a.samplesOf(k, spec, run)
		if err != nil {
			return err
		}
		doc := rarefactionDocument{analysisHead: head, Knots: diversity.DefaultKnots}
		lv := diversity.Levels{Knots: diversity.DefaultKnots}
		if levels, ok := diversity.Suggest(abundances); ok {
			doc.Levels = &levels
			lv.Sizes = []float64{levels.Size, levels.ExtrapolatedSize}
			lv.Coverages = []float64{levels.Coverage, levels.ExtrapolatedCoverage}
		}
		for i, s := range ds.Result.Table.Samples {
			if err := ctx.Err(); err != nil {
				return err
			}
			run.Progress(float64(i)/float64(len(abundances)), "Rarefying "+s.Name)
			opt := spec.Options
			opt.Stream = uint64(i)
			doc.Samples = append(doc.Samples, rarefactionSample{Name: s.Name, Curves: diversity.Rarefy(abundances[i], lv, opt)})
		}
		return a.keepAnalysis(k, spec, ds, run, doc, len(doc.Samples),
			artifacts.Method{Name: "Rarefaction and extrapolation of the Hill numbers of order 0, 1 and 2 by size and by coverage, with bootstrap intervals",
				Citation: "Chao A, Gotelli NJ, Hsieh TC, et al. Rarefaction and extrapolation with Hill numbers. Ecol Monogr. 2014;84:45–67. doi:10.1890/13-0133.1"},
			map[string]any{"samples": len(doc.Samples), "unit": head.Unit, "definition": head.Definition.Name, "levels": doc.Levels})
	}
}
