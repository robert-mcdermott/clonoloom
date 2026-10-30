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
		simulatedExample("pseudobulk-de-study", nil, "Paired pseudobulk expression study", "28 artificial subjects at one baseline, uneven clone sizes, 40 fictional genes and subject/category expression variability with known positive controls.", "Save all paired TRA/TRB phenotypes using cell_type, receptor sizes and breakpoints [1]. Aggregate with minimum 10 cells; compare Artificial T cells at 0 day, 1 versus >1, minCount 10 and minSamples 25. Full generating truth is separate.", 12<<20),
		simulatedExample("pseudobulk-de-null", nil, "Null pseudobulk expression study", "28 artificial subjects with no generating category expression effect; shared subject effects and category variability remain.", "Use the same declared paired pseudobulk workflow as the positive study. A single null example does not establish calibration or guarantee zero discoveries.", 12<<20),
		simulatedExample("pseudobulk-study", nil, "Pseudobulk count study", "Four artificial subjects at two visits, 420 cells per sample, 24 genes and negative-binomial expression with known states and sample-scoped paired receptors.", "In Cells, save phenotypes for each expression/<sample> and cells.tsv with explicit subject/timepoint, receptor sizes, state annotation and breakpoints [1,5,10]; aggregate selected parents with minimum 10 cells. Inspect absent and low-cell categories before any comparison.", 8<<20),
		simulatedExample("batch-qc-study", nil, "Batch quality-control study", "Four artificial batches with uneven depths, a shifted distribution, low-productivity and missing-count observations, small and constant groups, and explicit exclusions.", "Use truth/samples.json to inspect every sample and distinguish review flags from correction.", 8<<20),
		simulatedExample("contamination-study", nil, "Cross-subject nucleotide sharing study", "Eight artificial subjects, a repeated sample, uneven UMI depths, shared controls and planted carryover with complete sequence truth.", "Review exact NT sharing under explicit subject and batch assignments; flags are evidence for investigation.", 8<<20),
		simulatedExample("convergence-study", nil, "Subject recurrence and HLA study", "Thirty-two artificial subjects with repeated samples, uneven UMI counts, planted sequence recurrence and explicit complete or unresolved HLA typing. Full truth distinguishes association from antigen recognition.", "Use the declared query/background arms and subject typing from truth/. Select QGHT and ILMV signatures before testing the three-allele panel.", 8<<20),
		simulatedExample("motif-study", nil, "Query and background motif study", "Four artificial query subjects and eight background subjects, variable junction lengths, biased residues and uneven UMI counts. Planted local and conservative global signatures have complete truth; similarity is not proven specificity.", "Select S01–S04 as query and S05–S12 as explicit background in Specificity → Motifs.", 8<<20),
		simulatedExample("definition-study", nil, "Definition-sensitive repertoire study", "Thirty-two artificial subjects with long-tailed UMI counts and uneven depths. Explicit synonymous junctions and V/allele distinctions make grouping change across definitions; full partition and count truth is saved. This controlled example is not a fitted biological model.", "32 independent subjects. Compare control and stimulated observed-depth features under declared junction and V/J alternatives.", 12<<20),
		simulatedExample("counts-paired", cohortSteps("counts-paired"), "Replicated paired clonotype counts", "Twenty-eight artificial subjects before and after stimulation, uneven depths, shared and private receptors, subject effects, two batches and heterogeneous negative-binomial variation. Sparse increases/decreases and a large unchanged background; complete count truth is saved.", "56 samples in UMIs. Compare day 14 against day 0 within subjects; annotate recorded batches B1/B2.", 12<<20),
		simulatedExample("counts-independent", cohortSteps("counts-independent"), "Replicated independent clonotype counts", "Thirty-two artificial subjects, sixteen control and sixteen stimulated, with uneven depths, a private long tail and heterogeneous biological variation. Complete generator truth is saved.", "32 samples in UMIs. Compare stimulated against control; annotate recorded batches B1/B2.", 12<<20),
		simulatedExample("counts-null", cohortSteps("counts-null"), "Replicated counts with no group effect", "Thirty-two artificial subjects with heterogeneous variability and uneven depths; every group effect is zero. Batch variation is retained in the known truth.", "32 samples in UMIs. A negative control, not a promise that every fitted p-value exceeds 0.05.", 12<<20),
		simulatedExample("counts-unbalanced", cohortSteps("counts-unbalanced"), "Unbalanced cohort study", "Twelve artificial control and twenty stimulated subjects, with both batches in each group, variable depths and full count truth.", "Follow the guided design, feature and count analyses. Unequal group sizes do not imply confounding or justify dropping subjects.", 12<<20),
		simulatedExample("counts-confounded", cohortSteps("counts-confounded"), "Fully confounded cohort study", "Sixteen artificial controls all in B1 and sixteen stimulated subjects all in B2. The generator applies both batch and group effects; they cannot be separated by this design.", "Save the failed design check and inspect batch membership. No group analysis is run; neither adjustment nor more cells can recover the missing comparison.", 12<<20),
		bcellInputExample(),
		imgtInputExample(),
		expressionInputExample(),
		expressionHDF5Example(),
		annDataExample(),
		cellStateInputExample(),
		cellStateCloudExample(),
		simulatedExample("bcell-immunization", bcellSteps, "An artificial B-cell immunization study with known families",
			"Three subjects sampled before a simulated immunization and at days 14 and 28: paired heavy/light chains from twelve families per subject, with known ancestors, substitutions, isotypes and cell counts. Artificial coding sequences and SIM gene calls, not biological reference alleles; a simple benchmark, not a fitted model of affinity maturation. Truth is beside the observations in truth/.",
			"Nine samples, 240 paired cells each. Inspect the supplied ancestor alignments and artificial region coordinates.", 6<<20),
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
	bcellSteps = []examples.Step{
		{Title: "See each subject's samples in time", Action: "describe_subjects",
			Words: "Three artificial subjects, sampled at days -7, 14 and 28 of an immunization. Each sample holds 240 paired cells; the sampling probabilities and actual counts are recorded in truth/."},
		{Title: "Inspect an artificial sequence and its ancestor", Action: "inspect_bcell_inputs", Args: map[string]any{"select": 1},
			Words: "Every row retains a coding sequence, its known rearranged ancestor and artificial region coordinates. SIM calls are fictional. These are supplied annotations; mutation and lineage inference remain later steps. Count cells once, even though each has two sequence rows."},
		{Title: "Inspect distances before choosing a threshold", Action: "find_bcell_families", Words: "Each distinct paired nucleotide receptor contributes once to nearest distances within its subject and both chains' V/J/length partitions. Missing neighbors are not zero; no families are assigned yet."},
		{Title: "Find families at an explicit threshold", Action: "find_bcell_families", Args: map[string]any{"threshold": 0.1}, Words: "At 0.1 the artificial study recovers its 36 known families. This is a demonstration threshold, not a recommendation for biological data. Paired distance is the larger heavy/light distance; single-linkage endpoints can exceed the threshold."},
		{Title: "Reconstruct D-masked germlines", Action: "reconstruct_bcell_germlines", Args: map[string]any{"reference": simulatedGermlines}, Words: "The v2 artificial reference reconstructs all 4,320 assigned chain rows across 72 family/locus groups. The 33 intervening bases are masked, including the light-chain junction. These are reference projections, not the known full ancestors; mutation statistics are a separate result."},
		{Title: "Measure somatic hypermutation", Action: "measure_bcell_shm", Words: "Compare all 4,320 source chain rows with their D-masked references. Each has 327 evaluable bases and 109 evaluable codons. Inspect source rows for regional counts, single-hit replacement/silent substitutions and separately classified multi-hit codons. These net differences are not the simulator's historical event counts."},
		{Title: "Build lineage trees", Action: "build_bcell_lineages", Words: "Build 72 family/locus parsimony trees on 327 shared V/J sites. Inspect an inferred node, its conditional IUPAC states and the observed isotype, timepoint and tissue annotations. Many optimal topologies tie; the selected tree is not established ancestry or evidence of switching or migration. For a bounded likelihood comparison, select a seven-state tree and choose Review JC69 likelihood, then Fit JC69 likelihood. Inspect fitted substitutions/site, zero branches and conditional ancestral probabilities; the equal-rate baseline does not model SHM context. Save its separate exact evidence before closing."},
		{Title: "Summarize isotypes", Action: "summarize_bcell_isotypes", Words: "Count 2,160 cells in 36 saved families. Compare sample and family/sample class or constant-gene usage. Each unordered IGHM/IGHG1/IGHA1 pair occurs in all 36 families; these descriptive counts do not reconstruct switching events. Inspect a cell for its original heavy and light C calls. For descriptive prioritization, return to Families and choose Review B-cell candidates: select a sample, matching SHM/isotype parents and explicit ordered criteria. Any baseline must belong to the same subject. Counts, divergence, preferred class and observed share change do not predict binding. Save the separate exact evidence. These original example inputs lack sequence_aa, so source-chain FASTA is unavailable; the documented candidate fixture supplies separate artificial upstream annotations for that workflow."},
		{Title: "Keep the methods and references", Action: "show_methods", Words: "The methods paragraph records the family threshold, reference checksums, evaluable panels, uncertainty and citations for every saved result. Export result tables from each view; germlines and lineages also offer FASTA, and lineages Newick. The project saves automatically; keep its source files available so these results can be reopened."},
	}
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
