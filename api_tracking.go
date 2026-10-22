package main

// Tracking (internal/track; clonoloom-spec/requirements.md, T1 and T6), as an analysis
// (api_analysis.go): the study's design read from the samples' annotation (the spec's Study;
// internal/study), and the clonotypes chosen (the spec's Track) followed through each subject's
// samples, each one's frequency with its interval at every time point.

import (
	"context"

	"clonoloom/internal/artifacts"
	"clonoloom/internal/jobs"
	"clonoloom/internal/study"
	"clonoloom/internal/track"
)

var trackingAnalysis = analysis{kind: "tracking", prefix: "trk-", version: 1, title: "Tracking", over: "track", work: (*app).trackingWork}

// trackingDocument is a tracking artifact's tracking.json: the samples followed (every subject's),
// and the result.
type trackingDocument struct {
	analysisHead
	Samples []string `json:"samples"`
	track.Result
}

func (a *app) trackingWork(k analysis, spec analysisSpec) jobs.Func {
	return func(ctx context.Context, run *jobs.Run) error {
		ds, head, table, _, err := a.samplesOf(k, spec, run)
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		run.Progress(0.5, "Following the clonotypes through each subject's samples")
		res, err := track.Follow(table, sampleNames(ds), study.Describe(spec.Study), *spec.Track, spec.Options.Confidence)
		if err != nil {
			return err
		}
		doc := trackingDocument{analysisHead: head, Samples: []string{}, Result: res}
		followed := 0
		for _, s := range res.Subjects {
			for _, x := range s.Samples {
				doc.Samples = append(doc.Samples, x.Name)
			}
			followed += len(s.Clonotypes)
		}
		return a.keepAnalysis(k, spec, ds, run, doc, len(doc.Samples),
			artifacts.Method{Name: "Clonotypes followed through each subject's samples over time, each frequency with Wilson's score interval",
				Citation: "Wilson EB. Probable inference, the law of succession, and statistical inference. J Am Stat Assoc. 1927;22:209–212"},
			map[string]any{"subjects": len(res.Subjects), "samples": len(doc.Samples), "clonotypes": followed, "by": res.Choice.By, "n": res.Choice.N, "unit": head.Unit, "definition": head.Definition.Name})
	}
}
