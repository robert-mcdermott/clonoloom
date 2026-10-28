package main

import "clonoloom/internal/examples"

// These steps call the same visible actions as a researcher. They prescribe a teaching
// analysis, not an automatic analysis of arbitrary datasets (research.md §5.41).
func cohortSteps(preset string) []examples.Step {
	step := func(stage, title, words string) examples.Step {
		return examples.Step{Title: title, Words: words, Action: "cohort_example_step", Args: map[string]any{"preset": preset, "step": stage}}
	}
	steps := []examples.Step{step("design", "Record subjects, groups and batches", "Apply the batches recorded by this artificial generator and save a complete design check. Independent subjects are the units; the paired study keeps both observations per subject. No sample is dropped.")}
	if preset == "counts-confounded" {
		steps = append(steps, examples.Step{Title: "Keep the failed check and its scientific limits", Action: "show_methods", Words: "Every control is in B1 and every stimulated subject in B2. This check is saved, but is unavailable for inference. A batch-adjusted model cannot distinguish these effects. More cells or deeper sequencing cannot supply the missing comparison."})
		return steps
	}
	return append(steps,
		step("features", "Save observed-depth subject features", "Measure diversity and bind it to this exact saved design. Unequal UMI depths remain visible; observed richness is not depth-standardized. Each sample keeps its own measurements."),
		step("compare", "Compare three declared subject features", "Compare richness, Shannon entropy and Hill diversity q=1 with mean and rank tests, 95% marginal intervals and a declared BH family. Feature tests are unadjusted for batch; interpretation remains conditional on design and sampling. A null generator does not guarantee no discoveries."),
		step("counts", "Fit counts with an explicit batch term", "Fit raw clonotype UMI counts with total-count offsets, subject-level replication, an explicit batch term and unmoderated NB2/Cox–Reid dispersion. Retain the count filter, failed fits, BH family and conditional intervals. Relative repertoire effects need not equal generating absolute effects."),
		examples.Step{Title: "Keep methods, references and saved evidence", Action: "show_methods", Words: "Results and their parents are saved automatically. Inspect all observations and export each result with its source NOTICE. These artificial examples demonstrate workflow and refusal behavior; calibration is assessed separately by the scientific validation suite."})
}
