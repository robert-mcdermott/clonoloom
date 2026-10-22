package main

// The remote-control actions (remote.go): what a program on this computer can ask the open
// ClonoLoom page to do. The page performs them (web/ui/remote.js, whose action table must name
// exactly these; TestEveryActionHasAPageHandler checks it). GET /api/remote/tools lists them with
// their arguments' JSON schemas. Adapted from SituCraft 0.9.0 actions.go.

// apiVersion is the version of what these actions are: their names, their arguments and what their
// results hold. It changes when an action is removed or renamed, an argument removed or made to
// mean something else, or a field of a result removed or changed; adding an action, an optional
// argument or a field does not change it.
const apiVersion = 1

type remoteAction struct {
	Name        string         `json:"name"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	// reads: opens files on this computer by their paths, so it needs the token.
	reads bool
}

func schema(properties map[string]any, required ...string) map[string]any {
	out := map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
	if len(required) > 0 {
		out["required"] = required
	}
	return out
}

func textArg(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func numberArg(description string) map[string]any {
	return map[string]any{"type": "number", "description": description}
}

func flagArg(description string) map[string]any {
	return map[string]any{"type": "boolean", "description": description}
}

var datasetArg = textArg("A dataset of the project by name or id; the one in focus, else the first, if not given.")

var remoteActions = []remoteAction{
	{Name: "get_state", Title: "Describe the project", Description: "The open project: its datasets (format, unit, rows, samples, findings, filter, clonotype definition, license and citation of public data), the view shown, the item in focus, and the example datasets with whether each is downloaded.", InputSchema: schema(map[string]any{})},
	{Name: "new_project", Title: "Start a new project", Description: "Starts a new, empty project (the open one is saved).", InputSchema: schema(map[string]any{
		"name": textArg("The new project's name."),
	})},
	{Name: "open_data", Title: "Open data", Description: "Opens files or folders on this computer as one dataset of the project: AIRR rearrangement tables with their Repertoire metadata (.json), Cell Ranger V(D)J output folders or contig annotations, immunoSEQ exports; another tool's CSV or TSV through a mapping of its columns. Needs the token.", InputSchema: schema(map[string]any{
		"paths":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Absolute paths, read together as one dataset."},
		"name":     textArg("The dataset's name (its files' otherwise)."),
		"mappings": map[string]any{"type": "object", "description": "For tables ClonoLoom does not recognize, by path: {columns: {junction_aa, junction, v_call, d_call, j_call, c_call, count, sample, locus, productive, cell_id, clone_id, sequence_id: the column's name}, unit: reads | umis | templates | cells (when a count is mapped), sample: the sample's name when no column holds it}."},
	}, "paths"), reads: true},
	{Name: "open_example", Title: "Open an example dataset", Description: "Opens one of the example datasets as a dataset of the project. One not yet downloaded is downloaded first when download is true and its data are openly licensed; data under a provider's terms of use are downloaded only by the person, who accepts the terms in the window.", InputSchema: schema(map[string]any{
		"example":  textArg("The example's name, such as 10x-pbmc-1k (get_state lists them)."),
		"opening":  textArg("What of it to open, such as \"T cells\" (its first if not given)."),
		"download": flagArg("Download it first if it is not downloaded (openly licensed data only)."),
	}, "example")},
	{Name: "show", Title: "Show a view", Description: "Shows a view (start, data, qc, clonotypes, repertoire, tracking, jobs), with a dataset in focus.", InputSchema: schema(map[string]any{
		"view":    textArg("start, data, qc, clonotypes, repertoire, tracking or jobs."),
		"dataset": datasetArg,
	}, "view")},
	{Name: "set_definition", Title: "Define clonotypes", Description: "Chooses a dataset's clonotype definition: a preset by name, or one stated in full.", InputSchema: schema(map[string]any{
		"dataset":    datasetArg,
		"preset":     textArg("A preset's name (or part of it), such as \"Paired αβ\" or \"Nucleotide junction\"."),
		"definition": map[string]any{"type": "object", "description": "A definition in full: {name, chains: [] | [locus] | [locus, locus|\"light\"], junction: aa|nt, v: allele|gene|family|none, j: …, cells: primary|all, requireAll, productiveOnly, vendor}."},
	})},
	{Name: "annotate_samples", Title: "Annotate samples", Description: "Sets samples' annotation, as the Data view's samples table holds it: each one's subject, time point (day 28, week 4, baseline), reference role (infusion product, donor), tissue, cell subset, condition, batch and HLA. An empty value removes a field; fields not named are left as they are.", InputSchema: schema(map[string]any{
		"dataset": datasetArg,
		"samples": map[string]any{"type": "object", "description": "By sample name: {subject, timepoint, reference, tissue, cellSubset, condition, batch, hla: a value, or \"\" to remove it}."},
	}, "samples")},
	{Name: "describe_subjects", Title: "Describe the subjects", Description: "Shows the Tracking view and answers the dataset's subjects as the program reads their annotation: each one's samples in time order with their time points read as days from the subject's reference event, its reference samples and the samples taken at one time (replicates), and the checks' findings (samples without a subject or a time point, time points not understood, subjects without a reference sample).", InputSchema: schema(map[string]any{
		"dataset": datasetArg,
	})},
	{Name: "select_clonotype", Title: "Choose a clonotype", Description: "Shows the Clonotypes view and chooses a clonotype, which the inspector describes: by its rank in the table (1 for the first) or its ID.", InputSchema: schema(map[string]any{
		"dataset": datasetArg,
		"rank":    numberArg("Its rank in the table as sorted, from 1."),
		"id":      textArg("Its ID (c-…)."),
	})},
	{Name: "measure_diversity", Title: "Measure diversity", Description: "Shows the Repertoire view and measures each sample's diversity under the dataset's clonotype definition and filter (a job; the result kept with the project), answering each sample's main measures with their 95% intervals: clonotypes seen, Chao1, the Shannon and Simpson diversity seen and estimated, clonality, Simpson clonality, Gini and coverage.", InputSchema: schema(map[string]any{
		"dataset": datasetArg,
	})},
	{Name: "rarefy", Title: "Rarefy and extrapolate", Description: "Shows the Repertoire view's Rarefaction part and computes each sample's rarefaction and extrapolation curves under the dataset's clonotype definition and filter (a job; the result kept with the project), answering the levels the samples are best compared at and each sample's clonotypes, Shannon diversity and Simpson diversity at the level chosen, with their 95% intervals.", InputSchema: schema(map[string]any{
		"dataset": datasetArg,
		"level":   textArg("Where the samples are compared: coverage (the lowest sample coverage among them; the default), size (the smallest sample's), extrapolatedCoverage or extrapolatedSize (iNEXT's defaults, reached by extrapolating to twice a sample's size)."),
	})},
	{Name: "gene_usage", Title: "Count genes and junctions", Description: "Shows the Repertoire view's Genes or Junctions part and counts each sample's V, D, J and C genes, junction lengths and amino acids under the dataset's clonotype definition and filter (a job; the result kept with the project, the samples grouped by condition), answering for each sample (or condition) the most used genes of a segment, the commonest junction lengths and the mean length, with their 95% intervals (for a condition, its samples' range).", InputSchema: schema(map[string]any{
		"dataset": datasetArg,
		"part":    textArg("genes (the default) or junctions."),
		"segment": textArg("V (the default), D, J or C."),
		"level":   textArg("gene (the default) or family."),
		"weight":  textArg("clonotypes (each counted once; the default) or abundance."),
		"rows":    textArg("samples (the default) or groups (the samples' conditions)."),
	})},
	{Name: "measure_overlap", Title: "Measure overlap", Description: "Shows the Repertoire view's Overlap part and measures every pair of samples' overlap under the dataset's clonotype definition and filter (a job; the result kept with the project, the samples' subjects and conditions from their annotation or metadata), answering the pairs of one subject and of two summarized, the most overlapping pairs by a measure with their 95% intervals, and how many clonotypes are found in one subject, two, and more.", InputSchema: schema(map[string]any{
		"dataset": datasetArg,
		"measure": textArg("morisita (the default), morisitaHorn, jaccard, jaccardRarefied, sorensen, sorensenRarefied, overlap, overlapRarefied, shared, sharedRarefied or abundance."),
		"pairs":   numberArg("How many of the most overlapping pairs to answer (10 by default)."),
	})},
	{Name: "show_examples", Title: "Show the example data", Description: "Opens the Example data dialog.", InputSchema: schema(map[string]any{})},
	{Name: "show_methods", Title: "Write the methods paragraph", Description: "Shows a dataset's methods paragraph and answers it with its references.", InputSchema: schema(map[string]any{
		"dataset": datasetArg,
	})},
	{Name: "set_theme", Title: "Choose the theme", Description: "Light, dark or the system's.", InputSchema: schema(map[string]any{
		"theme": textArg("light, dark or system."),
	}, "theme")},
	{Name: "close_dialogs", Title: "Close dialogs", Description: "Closes any dialog, menu or toast open in the window.", InputSchema: schema(map[string]any{})},
}

func findAction(name string) (remoteAction, bool) {
	for _, a := range remoteActions {
		if a.Name == name {
			return a, true
		}
	}
	return remoteAction{}, false
}
