package main

// The example datasets (internal/examples, D13 and E2): listed, downloaded in the background
// when asked (the page polls the list for progress), cancelled, removed. The list is
// validation/sources.json, the same the validation suites download. Adapted from SituCraft 0.9.0
// api_examples.go (without its jobs).

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"clonoloom/internal/examples"
	"clonoloom/internal/jobs"
	"clonoloom/internal/simulate"
)

//go:embed validation/sources.json
var exampleSources []byte

func newExamples(dir string, offline bool) *examples.Library {
	sources, err := examples.Parse(exampleSources)
	if err != nil {
		log.Printf("the example list cannot be read: %v", err)
	}
	recorded, _ := examples.ParseAll(exampleSources)
	return &examples.Library{Dir: dir, Sources: sources, Recorded: recorded, Offline: offline, Generated: simulatedExamples(),
		UserAgent: "ClonoLoom/" + version + " (+https://github.com/robert-mcdermott/clonoloom)"}
}

// simulatedExamples are the simulated studies (internal/simulate) offered as examples, made on
// this computer in seconds.
func simulatedExamples() []examples.Generated {
	return []examples.Generated{
		bcellInputExample(),
		imgtInputExample(),
		simulatedExample("standard", nil, "A simulated TRB study with known truth",
			"Bulk T-cell receptor β repertoires simulated by ClonoLoom: three control subjects and three whose repertoires hold a few expanded clones, two blood samples of each at different depths. Every clonotype of every subject is known (truth/), so diversity, rarefaction and overlap can be checked against the truth. Made on this computer in a few seconds.",
			"The twelve samples, with their subjects and conditions, in templates.", 33<<20),
		simulatedExample("null", nil, "A simulated TRB study with no difference between its groups",
			"Six subjects of one kind in two groups (control and treated), two samples each: whatever a comparison of the groups finds is chance. Made on this computer in a few seconds, its truth beside it.",
			"The twelve samples, with their subjects and groups, in templates.", 38<<20),
		simulatedExample("car-t", carTSteps, "A simulated CAR-T study with known truth",
			"Three patients' T-cell receptor β repertoires (two who respond, one who does not) sampled a week before infusion (twice), then 7, 14, 28 and 90 days after it, with their infusion products, made from each patient's own T cells. Some of the product's clones expand after infusion and some contract, and the product's share of the blood rises and falls; every clone's trajectory, and the product's share of every sample, is known (truth/). Made on this computer in a few seconds.",
			"The 21 samples, with their subjects, time points and infusion products, in templates.", 33<<20),
		simulatedExample("transplant", transplantSteps, "A simulated transplant study with known truth",
			"Three recipients' T-cell receptor β repertoires sampled before an allogeneic transplant and 30, 100 and 365 days after it (twice), with their donors' blood. The donor's share of the recipient's blood grows from 60% to 97%, and in the recipient with graft-versus-host disease a few donor clones expand; every clone's trajectory, and the donor's share of every sample, is known (truth/). Made on this computer in a few seconds.",
			"The 18 samples, with their subjects, time points and donors, in templates.", 35<<20),
		simulatedExample("vaccine", vaccineSteps, "A simulated vaccination study with known specificities",
			"Five subjects' T-cell receptor β repertoires sampled a week before a vaccination (twice), then 7, 14, 28 and 90 days after it: three vaccinated, two given a placebo. Receptors that recognize three of the vaccine's epitopes (made up), in families around founders the subjects share and of each subject's own, rise from 0.1% of the blood to 3% at day 14 in the vaccinated. Every receptor's epitope, or none, is known (truth/), and a specificity database of the epitopes in VDJdb's format is beside the samples (database/). Made on this computer in a few seconds.",
			"The 30 samples, with their subjects and time points, in templates.", 58<<20),
	}
}

// The guided steps of the studies over time.
var (
	carTSteps = []examples.Step{
		{Title: "See each patient's samples in time", Action: "describe_subjects",
			Words: "Three patients, sampled twice a week before the infusion and four times after it, each with the infusion product as its reference sample."},
		{Title: "Follow the infusion product's largest clonotypes", Action: "track_clonotypes", Args: map[string]any{"by": "reference", "n": 10},
			Words: "Each one's frequency in the blood through the time points, with its interval: the product's largest clonotypes are the blood's largest too, as the product was made from it."},
		{Title: "Test the week before against two weeks after", Action: "test_expansion", Args: map[string]any{"from": "day -7", "to": "day 14"},
			Words: "The beta-binomial test, its variance fitted on the two samples taken before the infusion: the clonotypes that expanded two weeks after it, in the two patients who respond."},
		{Title: "Class every clonotype against the week before", Action: "class_clonotypes", Args: map[string]any{"base": "day -7", "class": "transient"},
			Words: "Tested at every later time point: the product's expanding clones are transient, up at day 14 and back down by day 90."},
		{Title: "Follow the transient clonotypes", Action: "track_clonotypes", Args: map[string]any{"by": "class", "class": "transient"},
			Words: "Their threads rise to day 14 and fall back; most were not found before the infusion."},
		{Title: "Attribute each sample to the product", Action: "attribute_samples",
			Words: "About 60% of the blood is in the product's clonotypes before the infusion already: a product made from the patient's own blood cannot be told apart from it by clonotype."},
	}
	vaccineSteps = []examples.Step{
		{Title: "See each subject's samples in time", Action: "describe_subjects",
			Words: "Five subjects, sampled twice a week before the vaccination and 7, 14, 28 and 90 days after it: three vaccinated, two given a placebo."},
		{Title: "Test the week before against two weeks after", Action: "test_expansion", Args: map[string]any{"from": "day -7", "to": "day 14"},
			Words: "The beta-binomial test, its variance fitted on the two samples taken before the vaccination: the few clonotypes it calls are receptors that recognize the vaccine's epitopes, in the vaccinated subjects alone. Most of the response is spread over too many small clones to call one by one."},
		{Title: "Class every clonotype against the week before", Action: "class_clonotypes", Args: map[string]any{"base": "day -7", "class": "transient"},
			Words: "The response peaks at day 14 and falls back: its clonotypes called are transient."},
		{Title: "Follow the transient clonotypes", Action: "track_clonotypes", Args: map[string]any{"by": "class", "class": "transient"},
			Words: "Their threads rise from a few templates, or none, before the vaccination to a peak at day 14, and fall back."},
		{Title: "Match the simulated specificity database", Action: "match_specificity", Args: map[string]any{"database": "simulated-vaccine", "metric": "exact", "genes": true, "species": "HomoSapiens", "minScore": 0},
			Words: "Compare exact CDR3 and V/J calls with the simulated evidence. Each sample's epitope shares count a clonotype once. Other donors' receptors leave much of the true response unmatched; an exact match alone does not prove specificity."},
		{Title: "Follow one matched epitope", Action: "match_specificity", Args: map[string]any{"database": "simulated-vaccine", "metric": "exact", "genes": true, "species": "HomoSapiens", "minScore": 0, "follow": 2},
			Words: "Follow the second evidence group, a vaccine epitope, through each subject's samples. The selection records its source result. Compare vaccinated subjects with placebo; these matched receptors represent only part of the response."},
	}
	transplantSteps = []examples.Step{
		{Title: "See each recipient's samples in time", Action: "describe_subjects",
			Words: "Three recipients, sampled before the transplant and 30, 100 and 365 days after it (twice), each with the donor's blood as its reference sample."},
		{Title: "Attribute each sample to the donor", Action: "attribute_samples",
			Words: "The donor's share of the recipient's blood, from the clonotypes found in only one of the donor's blood and the recipient's before the transplant: about 60% at day 30, 85% at day 100 and 97% at day 365, as simulated."},
		{Title: "Test day 30 against day 100", Action: "test_expansion", Args: map[string]any{"from": "day 30", "to": "day 100"},
			Words: "The beta-binomial test, its variance fitted on the two samples taken at day 365: in the recipient with graft-versus-host disease, a few donor clones expand."},
		{Title: "Class every clonotype against day 30", Action: "class_clonotypes", Args: map[string]any{"base": "day 30", "class": "transient"},
			Words: "The alloreactive clones peak at day 100: transient."},
	}
)

// simulatedExample is a preset's study as an example, with its guided steps.
func simulatedExample(preset string, steps []examples.Step, title, description, opening string, size int64) examples.Generated {
	study := simulate.Presets[preset]
	return examples.Generated{
		Source: examples.Source{
			Name: "simulated-" + preset, Title: title, Description: description,
			Source: fmt.Sprintf("Made on this computer: clonoloom simulate --preset %s (seed %d)", preset, study.Seed), Publisher: "ClonoLoom",
			License: "Simulated data: no restrictions", LicenseClass: "open",
			Citation: "McDermott R. ClonoLoom, version " + version + ". https://github.com/robert-mcdermott/clonoloom",
			Opens:    []examples.Opening{{Title: "the study", Name: "Simulated study (" + preset + ")", Summary: opening, Steps: steps}},
		},
		Size: size, Marker: "truth/truth.json",
		Make: func(ctx context.Context, dir string, progress examples.Progress) error {
			_, err := simulate.Run(ctx, study, dir, version, func(f float64, message string) { progress(f, message) })
			return err
		},
		// Every sample's table, with their Repertoire metadata, as one dataset.
		Files: func(dir string) [][]string {
			var files []string
			entries, _ := os.ReadDir(dir)
			for _, e := range entries {
				if strings.HasSuffix(e.Name(), ".tsv") {
					files = append(files, e.Name())
				}
			}
			return [][]string{append(files, "repertoires.airr.json")}
		},
	}
}

// download is an example's job as the Example data dialog shows it: one running, or the last
// that failed or was cancelled.
type download struct {
	Job      string  `json:"job"`
	Fraction float64 `json:"fraction"`
	Message  string  `json:"message"`
	Error    string  `json:"error,omitempty"`
}

type exampleState struct {
	examples.State
	Download *download `json:"download,omitempty"`
}

// exampleJobKey: one job per example at a time.
func exampleJobKey(name string) string { return "example\x00" + name }

// exampleJob is an example's latest job: its unfinished one, else its last; nil when it has none.
func (a *app) exampleJob(name string) *jobs.Job {
	var last *jobs.Job
	for _, j := range a.jobs.List(1) {
		if j.Key != exampleJobKey(name) {
			continue
		}
		j := j
		if !j.State.Finished() {
			return &j
		}
		last = &j
	}
	return last
}

func (a *app) registerExamples(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/examples", func(w http.ResponseWriter, r *http.Request) {
		list := a.examples.List()
		out := make([]exampleState, len(list))
		for i, s := range list {
			out[i].State = s
			j := a.exampleJob(s.Name)
			switch {
			case j == nil:
			case !j.State.Finished():
				out[i].State.State = "downloading"
				out[i].Download = &download{Job: j.ID, Fraction: j.Progress, Message: j.Message}
			case j.State == jobs.Failed || j.State == jobs.Orphaned:
				out[i].Download = &download{Job: j.ID, Error: j.Error}
			case j.State == jobs.Cancelled:
				out[i].Download = &download{Job: j.ID, Error: "Cancelled."}
			}
		}
		writeJSON(w, map[string]any{"dir": a.examples.Dir, "offline": a.examples.Offline, "hosts": a.examples.Hosts(), "examples": out})
	})
	mux.HandleFunc("POST /api/examples/{name}", a.downloadExample)
	mux.HandleFunc("POST /api/examples/{name}/cancel", func(w http.ResponseWriter, r *http.Request) {
		j := a.exampleJob(r.PathValue("name"))
		if j == nil || j.State.Finished() {
			writeError(w, http.StatusNotFound, "It is not being downloaded.")
			return
		}
		writeJSON(w, map[string]bool{"cancelled": a.jobs.Cancel(j.ID)})
	})
	mux.HandleFunc("DELETE /api/examples/{name}", func(w http.ResponseWriter, r *http.Request) {
		if !isLoopbackRequest(r) {
			writeError(w, http.StatusForbidden, "Only this computer may remove files.")
			return
		}
		name := r.PathValue("name")
		if j := a.exampleJob(name); j != nil && !j.State.Finished() {
			writeError(w, http.StatusConflict, "It is being downloaded.")
			return
		}
		if err := a.examples.Remove(name); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, map[string]bool{"removed": true})
	})
}

// exampleSpec is an example job's spec: the example, and whether the user accepted its
// provider's terms of use.
type exampleSpec struct {
	Example string `json:"example"`
	Accept  bool   `json:"accept"`
}

// exampleWork downloads (or makes) an example as a job.
func (a *app) exampleWork(spec exampleSpec) jobs.Func {
	return func(ctx context.Context, run *jobs.Run) error {
		run.Log("info", "%s into %s.", map[bool]string{true: "Making " + spec.Example, false: "Downloading " + spec.Example}[isGenerated(a.examples, spec.Example)], a.examples.Dir)
		path, err := a.examples.Download(ctx, spec.Example, spec.Accept, func(f float64, message string) { run.Progress(f, message) })
		if err != nil {
			return err
		}
		run.Log("info", "Ready at %s.", path)
		return nil
	}
}

// downloadExample starts a job downloading (or making) an example, {"accept": true} accepting its
// provider's terms of use where it has them. Only this computer may ask.
func (a *app) downloadExample(w http.ResponseWriter, r *http.Request) {
	if !isLoopbackRequest(r) {
		writeError(w, http.StatusForbidden, "Only this computer may download into its folders.")
		return
	}
	var body struct {
		Accept bool `json:"accept"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<10)).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "Send {\"accept\": true|false}.")
		return
	}
	name := r.PathValue("name")
	var source *examples.Source
	for i := range a.examples.Sources {
		if a.examples.Sources[i].Name == name {
			source = &a.examples.Sources[i]
		}
	}
	for i := range a.examples.Generated {
		if a.examples.Generated[i].Name == name {
			source = &a.examples.Generated[i].Source
		}
	}
	switch {
	case source == nil:
		writeError(w, http.StatusNotFound, "No such example.")
		return
	case a.examples.Offline && !isGenerated(a.examples, name):
		writeErrorCode(w, http.StatusForbidden, "offline", examples.ErrOffline.Error()+".")
		return
	case source.LicenseClass != "open" && !body.Accept:
		writeErrorCode(w, http.StatusForbidden, "terms", "These data are under "+source.License+": read the terms and accept them to download.")
		return
	}
	title := "Download " + source.Title
	if isGenerated(a.examples, name) {
		// "Make a simulated TRB study…"
		title = "Make " + strings.ToLower(source.Title[:1]) + source.Title[1:]
	}
	spec := exampleSpec{Example: name, Accept: body.Accept}
	job, existing, err := a.jobs.Submit("example", title, exampleJobKey(name), spec, a.exampleWork(spec))
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeJSON(w, map[string]any{"started": !existing, "job": job.ID})
}

func isGenerated(l *examples.Library, name string) bool {
	for _, g := range l.Generated {
		if g.Name == name {
			return true
		}
	}
	return false
}

// credit is the source, license and citation of an example a dataset was opened from (O9).
type credit struct {
	Example      string `json:"example"`
	Title        string `json:"title"`
	Source       string `json:"source"`
	Page         string `json:"page"`
	License      string `json:"license"`
	LicenseURL   string `json:"licenseUrl,omitempty"`
	LicenseClass string `json:"licenseClass"`
	Citation     string `json:"citation"`
}

// credits answers the recorded public data sets a dataset's files are, by their SHA-256: wherever
// and however they were downloaded, the dataset carries their source, license and citation.
func (a *app) credits(sha256s []string) []credit {
	var out []credit
	for _, s := range a.examples.Identify(sha256s) {
		out = append(out, credit{Example: s.Name, Title: s.Title, Source: s.Source, Page: s.Page, License: s.License, LicenseURL: s.LicenseURL, LicenseClass: s.LicenseClass, Citation: s.Citation})
	}
	return out
}
