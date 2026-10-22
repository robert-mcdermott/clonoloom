package main

// Classes across time points and attribution (internal/track; clonoloom-spec/requirements.md, T4
// and T5), as analyses (api_analysis.go): the study's design read from the samples' annotation
// (the spec's Study); in each subject, every clonotype tested between a base time point and every
// later one and classed by the calls (the spec's Class); and each sample's share in its reference
// sample's clonotypes, and the share derived from the reference where it can be told.

import (
	"context"

	"clonoloom/internal/artifacts"
	"clonoloom/internal/jobs"
	"clonoloom/internal/study"
	"clonoloom/internal/track"
)

var classesAnalysis = analysis{kind: "classes", prefix: "cls-", version: 1, title: "Classes", over: "class", work: (*app).classesWork}
var attributionAnalysis = analysis{kind: "attribution", prefix: "att-", version: 1, title: "Attribution", over: "attribute", work: (*app).attributionWork}

// classesDocument is a classes artifact's classes.json: the samples classed (every subject's base
// and later time points'), and the result.
type classesDocument struct {
	analysisHead
	Samples []string `json:"samples"`
	track.ClassResult
}

// attributionDocument is an attribution artifact's attribution.json: the samples attributed, and
// the result.
type attributionDocument struct {
	analysisHead
	Samples []string `json:"samples"`
	track.Attribution
}

func (a *app) classesWork(k analysis, spec analysisSpec) jobs.Func {
	return func(ctx context.Context, run *jobs.Run) error {
		ds, head, table, _, err := a.samplesOf(k, spec, run)
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		run.Progress(0.3, "Testing each clonotype between the base and every later time point")
		res, err := track.Classify(table, sampleNames(ds), study.Describe(spec.Study), *spec.Class)
		if err != nil {
			return err
		}
		doc := classesDocument{analysisHead: head, Samples: []string{}, ClassResult: res}
		var counts track.ClassCounts
		for _, s := range res.Subjects {
			doc.Samples = append(doc.Samples, s.Base.Samples...)
			for _, p := range s.Later {
				doc.Samples = append(doc.Samples, p.Samples...)
			}
			counts.Emerging += s.Counts.Emerging
			counts.Transient += s.Counts.Transient
			counts.Contracting += s.Counts.Contracting
			counts.Persistent += s.Counts.Persistent
		}
		method := expansionMethods[res.Test]
		method.Name = "Clonotypes classed across time points by their calls against the base time point: " + method.Name
		return a.keepAnalysis(k, spec, ds, run, doc, len(doc.Samples), method,
			map[string]any{"subjects": len(res.Subjects), "base": res.Classing.Base, "test": res.Test, "counts": counts, "unit": head.Unit, "definition": head.Definition.Name})
	}
}

func (a *app) attributionWork(k analysis, spec analysisSpec) jobs.Func {
	return func(ctx context.Context, run *jobs.Run) error {
		ds, head, table, _, err := a.samplesOf(k, spec, run)
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		run.Progress(0.5, "Attributing each sample to its subject's reference sample")
		res, err := track.Attribute(table, sampleNames(ds), study.Describe(spec.Study), spec.Options.Confidence)
		if err != nil {
			return err
		}
		doc := attributionDocument{analysisHead: head, Samples: []string{}, Attribution: res}
		derived := 0
		for _, s := range res.Subjects {
			for _, x := range s.Samples {
				doc.Samples = append(doc.Samples, x.Name)
			}
			if s.Derived {
				derived++
			}
		}
		return a.keepAnalysis(k, spec, ds, run, doc, len(doc.Samples),
			artifacts.Method{Name: "Each sample's share in its reference sample's clonotypes, and the share derived from the reference among the clonotypes of one side only, each with Wilson's score interval",
				Citation: "Wilson EB. Probable inference, the law of succession, and statistical inference. J Am Stat Assoc. 1927;22:209–212"},
			map[string]any{"subjects": len(res.Subjects), "samples": len(doc.Samples), "derived": derived, "unit": head.Unit, "definition": head.Definition.Name})
	}
}
