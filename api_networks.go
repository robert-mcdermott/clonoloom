package main

// Clonotype networks as immutable analyses: all scientific edges and memberships
// are retained separately from the bounded drawing. See internal/network.

import (
	"context"
	"errors"
	"fmt"

	"clonoloom/internal/artifacts"
	"clonoloom/internal/clonotype"
	"clonoloom/internal/jobs"
	"clonoloom/internal/network"
	"clonoloom/internal/tcrdist"
)

var networkAnalysis = analysis{kind: "networks", prefix: "net-", version: 1, title: "Clonotype networks", subjects: true, over: "networks", work: (*app).networkWork, prepare: (*app).networkPrepare}

type networkOptions struct {
	network.Options
	CDR3Only  bool   `json:"cdr3Only"`
	Organism  string `json:"organism,omitempty"`
	Loops     string `json:"loops,omitempty"`
	DrawNodes int    `json:"drawNodes"`
}

func (a *app) networkPrepare(spec *analysisSpec) error {
	o := spec.Networks
	if spec.Definition.Vendor {
		return errors.New("networks require a junction-based clonotype definition")
	}
	var err error
	o.Options, err = o.Options.Normalize()
	if err != nil {
		return err
	}
	if o.SameV && (spec.Definition.V == "none" || spec.Definition.V == "family") {
		return errors.New("same V gene requires a definition resolving V genes or alleles")
	}
	if o.SameJ && (spec.Definition.J == "none" || spec.Definition.J == "family") {
		return errors.New("same J gene requires a definition resolving J genes or alleles")
	}
	if o.DrawNodes == 0 {
		o.DrawNodes = 1000
	}
	if o.DrawNodes < 1 || o.DrawNodes > 2000 {
		return errors.New("draw from 1 to 2000 nodes; all nodes remain in the result")
	}
	if o.Metric == "tcrdist" {
		// Reuse the established loop lookup/checksum policy, while preserving cutoff 0.
		t := &tcrdistOptions{Radius: 1, CDR3Only: o.CDR3Only, Organism: o.Organism}
		copy := *spec
		copy.TCRdist = t
		if err := a.tcrdistPrepare(&copy); err != nil {
			return err
		}
		o.CDR3Only, o.Organism, o.Loops = t.CDR3Only, t.Organism, t.Loops
	} else {
		o.CDR3Only = false
		o.Organism = ""
		o.Loops = ""
	}
	return nil
}

type networkDocument struct {
	analysisHead
	Samples    []string              `json:"samples"`
	Options    networkOptions        `json:"networks"`
	Clonotypes []clonotype.Clonotype `json:"clonotypes"`
	Graph      *network.Graph        `json:"graph"`
	Drawing    network.Drawing       `json:"drawing"`
	Loops      *loopsFile            `json:"loops,omitempty"`
}

func (a *app) networkWork(k analysis, spec analysisSpec) jobs.Func {
	return func(ctx context.Context, run *jobs.Run) error {
		ds, head, table, _, err := a.samplesOf(k, spec, run)
		if err != nil {
			return err
		}
		o := *spec.Networks
		var loops *tcrdist.Loops
		var file *loopsFile
		if o.Metric == "tcrdist" && !o.CDR3Only {
			l, f, err := a.tcrdistLoops()
			if err != nil {
				return err
			}
			if f.SHA256 != o.Loops {
				return errors.New("the V genes' loops changed; request the network again")
			}
			loops = l
			file = &f
		}
		subjects := map[uint32]string{}
		names := sampleNames(ds)
		for i, s := range names {
			subjects[uint32(i)] = spec.Subjects[s]
		}
		run.Progress(.15, "Finding chain neighbors")
		graph, err := network.Build(ctx, table, o.Options, loops, o.Organism, subjects, func(done, total int) {
			run.Progress(.15+.65*float64(done)/float64(max(total, 1)), fmt.Sprintf("Comparing chains: %d of %d", done, total))
		})
		if err != nil {
			return err
		}
		run.Progress(.85, "Drawing connected components")
		drawing, err := network.Layout(ctx, graph, o.DrawNodes)
		if err != nil {
			return err
		}
		doc := networkDocument{analysisHead: head, Samples: names, Options: o, Clonotypes: table.Clonotypes, Graph: graph, Drawing: drawing, Loops: file}
		return a.keepAnalysis(k, spec, ds, run, doc, len(names), artifacts.Method{Name: "Clonotype sequence neighbors and connected components; deterministic force drawing", Citation: "Levenshtein 1966, Soviet Physics Doklady 10:707–710; Wagner and Fischer 1974, doi:10.1145/321796.321811; Dash et al. 2017, doi:10.1038/nature22383; Tarjan 1975, doi:10.1145/321879.321884; Fruchterman and Reingold 1991, doi:10.1002/spe.4380211102"}, map[string]any{"nodes": len(table.Clonotypes), "edges": len(graph.Edges), "components": len(graph.Components), "definition": head.Definition.Name})
	}
}
