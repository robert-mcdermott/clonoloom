package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"

	"clonoloom/internal/artifacts"
	"clonoloom/internal/clonotype"
	"clonoloom/internal/examples"
	"clonoloom/internal/jobs"
	"clonoloom/internal/specificity"
	"clonoloom/internal/tcrdist"
)

var specificityAnalysis = analysis{kind: "specificity", prefix: "spc-", version: 1, title: "Specificity database matches", over: "specificity", work: (*app).specificityWork, prepare: (*app).specificityPrepare}
var errNoDatabase = errors.New("the specificity database is not ready: download VDJdb in Reference data, or generate the vaccine example for its simulated database")

type specificityOptions struct {
	specificity.Options
	Database string `json:"database"`
	SHA256   string `json:"sha256"`
	Loops    string `json:"loops,omitempty"`
}
type specificityDocument struct {
	analysisHead
	Samples    []string              `json:"samples"`
	Options    specificityOptions    `json:"specificity"`
	Database   loopsFile             `json:"database"`
	Loops      *loopsFile            `json:"loops,omitempty"`
	Clonotypes []clonotype.Clonotype `json:"clonotypes"`
	specificity.Result
}

// Only catalogued databases are opened, never arbitrary paths supplied over HTTP.
// A generated example is identified by the bytes actually read, too.
func (a *app) specificityDatabase(name string) (*os.File, loopsFile, error) {
	library := a.references
	rel := "vdjdb.txt"
	if name == "simulated-vaccine" {
		library = a.examples
		rel = "database/vdjdb.txt"
	}
	if library == nil {
		return nil, loopsFile{}, errNoDatabase
	}
	var state examples.State
	for _, s := range library.List() {
		if s.Name == name {
			state = s
			break
		}
	}
	if state.State != "ready" {
		return nil, loopsFile{}, errNoDatabase
	}
	f, err := os.Open(filepath.Join(state.Path, filepath.FromSlash(rel)))
	if err != nil {
		return nil, loopsFile{}, err
	}
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		f.Close()
		return nil, loopsFile{}, err
	}
	file := loopsFile{Reference: name, File: rel, SHA256: hex.EncodeToString(h.Sum(nil)), Source: state.Source.Source, License: state.License, Citation: state.Citation}
	if !state.Generated {
		recorded := ""
		for _, x := range state.Files {
			if x.Path == rel {
				recorded = x.SHA256
			}
		}
		if recorded != file.SHA256 {
			f.Close()
			return nil, loopsFile{}, errors.New("the specificity database changed; download it again")
		}
	}
	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		f.Close()
		return nil, loopsFile{}, err
	}
	return f, file, nil
}
func (a *app) specificityPrepare(s *analysisSpec) error {
	o := s.Specificity
	if o.Database == "" {
		o.Database = "vdjdb"
	}
	if o.Database != "vdjdb" && o.Database != "simulated-vaccine" {
		return errors.New("database: vdjdb or simulated-vaccine")
	}
	var err error
	o.Options, err = o.Options.Normalize()
	if err != nil {
		return err
	}
	if s.Definition.Vendor || s.Definition.Junction != "aa" {
		return errors.New("specificity requires amino-acid junction clonotypes")
	}
	if o.Genes && (s.Definition.V == "none" || s.Definition.V == "family" || s.Definition.J == "none" || s.Definition.J == "family") {
		return errors.New("V/J matching requires a definition resolving genes or alleles")
	}
	o.Loops = ""
	if o.Metric == "tcrdist" && !o.CDR3Only {
		if s.Definition.V == "none" || s.Definition.V == "family" {
			return errors.New("V-gene loops require a definition resolving V genes or alleles")
		}
		loops, f, err := a.tcrdistLoops()
		if err != nil {
			return err
		}
		if loops.Alleles(o.Organism()) == 0 {
			return errors.New("the reference has no loops for this species; choose CDR3-only comparison")
		}
		o.Loops = f.SHA256
	}
	f, record, err := a.specificityDatabase(o.Database)
	if err != nil {
		return err
	}
	f.Close()
	o.SHA256 = record.SHA256
	return nil
}
func (a *app) specificityWork(k analysis, s analysisSpec) jobs.Func {
	return func(ctx context.Context, run *jobs.Run) error {
		ds, head, table, _, err := a.samplesOf(k, s, run)
		if err != nil {
			return err
		}
		o := *s.Specificity
		f, database, err := a.specificityDatabase(o.Database)
		if err != nil {
			return err
		}
		defer f.Close()
		if database.SHA256 != o.SHA256 {
			return errors.New("the specificity database changed; request the analysis again")
		}
		run.Progress(.2, "Reading specificity evidence")
		records, err := specificity.ParseContext(ctx, f)
		if err != nil {
			return err
		}
		var loops *tcrdist.Loops
		var file *loopsFile
		if o.Metric == "tcrdist" && !o.CDR3Only {
			l, f, err := a.tcrdistLoops()
			if err != nil {
				return err
			}
			if f.SHA256 != o.Loops {
				return errors.New("the V-gene loops changed; request the analysis again")
			}
			loops = l
			file = &f
		}
		run.Progress(.4, "Matching receptor chains to specificity records")
		result, err := specificity.Find(ctx, table, records, o.Options, loops)
		if err != nil {
			return err
		}
		doc := specificityDocument{analysisHead: head, Samples: sampleNames(ds), Options: o, Database: database, Loops: file, Clonotypes: table.Clonotypes, Result: result}
		return a.keepAnalysis(k, s, ds, run, doc, len(doc.Samples), artifacts.Method{Name: specificity.Words(o.Options), Citation: database.Citation + "; Dash et al. 2017, doi:10.1038/nature22383; Mayer-Blackwell et al. 2021, doi:10.7554/eLife.68605"}, map[string]any{"matches": len(result.Matches), "epitopes": len(result.Epitopes), "database": database})
	}
}
