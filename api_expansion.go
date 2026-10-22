package main

// Two-sample tests between time points (internal/expansion, internal/track; clonoloom-spec/
// requirements.md, T2), as an analysis (api_analysis.go): the study's design read from the samples'
// annotation (the spec's Study), and in each subject, every clonotype tested between its samples
// at two time points (the spec's Compare), the false discovery rate controlled within the subject;
// the beta-binomial test's variance fitted once for the study.

import (
	"context"

	"clonoloom/internal/artifacts"
	"clonoloom/internal/expansion"
	"clonoloom/internal/jobs"
	"clonoloom/internal/study"
	"clonoloom/internal/track"
)

var expansionAnalysis = analysis{kind: "expansion", prefix: "exp-", version: 1, title: "Expansion", over: "compare", work: (*app).expansionWork}

// expansionDocument is an expansion artifact's expansion.json: the samples compared (every
// subject's two time points'), and the result.
type expansionDocument struct {
	analysisHead
	Samples []string `json:"samples"`
	track.Tests
}

// The methods, as the record names and cites them.
var expansionMethods = map[string]artifacts.Method{
	expansion.BetaBinomial: {Name: "Each clonotype tested between two time points by the beta-binomial test conditional on its total, at the share of the depth, its variance v(n) = 2 + (b ln n + a)² fitted by maximum likelihood",
		Citation: "Rytlewski J, Deng S, Xie T, et al. Model to improve specificity for identification of clinically-relevant expanded T cells in peripheral blood. PLoS One. 2019;14:e0213684"},
	expansion.Binomial: {Name: "Each clonotype tested between two time points by the binomial test conditional on its total, at the share of the depth",
		Citation: "Rytlewski J, Deng S, Xie T, et al. Model to improve specificity for identification of clinically-relevant expanded T cells in peripheral blood. PLoS One. 2019;14:e0213684"},
	expansion.Fisher: {Name: "Each clonotype tested between two time points by Fisher's exact test",
		Citation: "DeWitt WS, Emerson RO, Lindau P, et al. Dynamics of the cytotoxic T cell response to a model of acute viral infection. J Virol. 2015;89:4517–4526"},
}

func (a *app) expansionWork(k analysis, spec analysisSpec) jobs.Func {
	return func(ctx context.Context, run *jobs.Run) error {
		ds, head, table, _, err := a.samplesOf(k, spec, run)
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		run.Progress(0.3, "Fitting the variance and testing each clonotype between the two time points")
		res, err := track.Test(table, sampleNames(ds), study.Describe(spec.Study), *spec.Compare)
		if err != nil {
			return err
		}
		doc := expansionDocument{analysisHead: head, Samples: []string{}, Tests: res}
		tested, increased, decreased := 0, 0, 0
		for _, s := range res.Subjects {
			doc.Samples = append(append(doc.Samples, s.From.Samples...), s.To.Samples...)
			tested += s.Tested
			increased += s.Increased
			decreased += s.Decreased
		}
		c := res.Comparison
		summary := map[string]any{"subjects": len(res.Subjects), "from": c.From, "to": c.To, "test": res.Test, "correction": c.Correction, "alpha": c.Alpha,
			"tested": tested, "increased": increased, "decreased": decreased, "unit": head.Unit, "definition": head.Definition.Name}
		if res.Fit != nil {
			summary["fit"] = map[string]any{"on": res.Fit.On, "pairs": len(res.Fit.Pairs), "clonotypes": res.Fit.Clonotypes, "a": res.Fit.Model.A, "b": res.Fit.Model.B}
		}
		return a.keepAnalysis(k, spec, ds, run, doc, len(doc.Samples), expansionMethods[res.Test], summary)
	}
}
