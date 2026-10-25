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
	{Name: "show", Title: "Show a view", Description: "Shows a view (start, data, qc, clonotypes, repertoire, tracking, specificity, jobs), with a dataset in focus.", InputSchema: schema(map[string]any{
		"view":    textArg("start, data, qc, clonotypes, repertoire, tracking, specificity or jobs."),
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
	{Name: "track_clonotypes", Title: "Follow clonotypes over time", Description: "Shows the Tracking view's Clonotypes part and follows clonotypes through each subject's samples in time order, under the dataset's clonotype definition and filter (a job; the result kept with the project): each subject's reference sample's largest clonotypes (the default; each sample's largest for a subject without one), each sample's largest, a list, or those a test called. Answers each subject's time points and, for each clonotype followed, why it was chosen and its count, frequency and 95% interval (Wilson's) at each time point, the samples taken at one time pooled.", InputSchema: schema(map[string]any{
		"dataset": datasetArg,
		"by":      textArg("reference (each reference sample's n largest; the default), largest (each sample's n largest), list, test (the n clonotypes the dataset's comparison called, the most significant first: see test_expansion), or class (the n clonotypes of a class against the dataset's base time point: see class_clonotypes)."),
		"class":   textArg("With by class: emerging (the default), transient, contracting or persistent."),
		"n":       numberArg("How many: 10 of a reference sample, 5 of each sample and 20 of a test's calls by default, at most 100."),
		"list":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "With by list: the clonotypes, by ID (c-…) or junction (CASSLAGGPEAFF; a paired clonotype's chains joined by \"; \")."},
		"subject": textArg("The subject to show (the first by default)."),
	})},
	{Name: "test_expansion", Title: "Test clonotypes between two time points", Description: "Shows the Tracking view's Tests part and tests each clonotype between two time points of each subject (the samples at one time pooled), under the dataset's clonotype definition and filter (a job; the result kept with the project): the beta-binomial test conditional on the clonotype's total, its variance fitted on the study's replicate samples or two time points named as stable (the default where it can be fitted; else the binomial test), the binomial test, or Fisher's exact test, two-sided or one-sided, the false discovery rate controlled within each subject by Benjamini–Hochberg (the default) or Storey's q-values; clonotypes with fewer than 5 cells or molecules in the two together are not tested. Answers the test used and the variance model fitted, each subject's clonotypes tested, left out, increased and decreased, and the most significant calls. The tests refuse reads.", InputSchema: schema(map[string]any{
		"dataset":     datasetArg,
		"from":        textArg("The first time point, by its key as the subjects' samples give it (describe_subjects): day 0, day -7, baseline, reference (the reference sample), visit 2."),
		"to":          textArg("The second time point."),
		"test":        textArg("auto (the beta-binomial test where its variance can be fitted, else the binomial test; the default), betabinomial, binomial or fisher."),
		"stable":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Two time points between which the clonotypes are taken not to change, to fit the beta-binomial test's variance on instead of the replicate samples: [\"day -7\", \"day 0\"]; [] for the replicate samples."},
		"alternative": textArg("two-sided (the default), increase (more at the second time point) or decrease."),
		"correction":  textArg("bh (Benjamini–Hochberg; the default) or storey (Storey's q-values)."),
		"alpha":       numberArg("The false discovery rate at which a clonotype is called: 0.01 by default."),
		"subject":     textArg("The subject to show (the first by default)."),
		"calls":       numberArg("How many of each subject's calls to answer (10 by default)."),
	}, "from", "to")},
	{Name: "class_clonotypes", Title: "Class clonotypes across time points", Description: "Shows the Tracking view's Classes part and, in each subject, tests each clonotype between a base time point and every later one by the dataset's test (as test_expansion chose it; the beta-binomial test where its variance can be fitted), and classes it by its calls: emerging (called increased at the last time point), transient (called changed at a time point between, not at the last), contracting (called decreased at the last) or persistent (never called, found at every time point). A job; the result kept with the project. Answers each subject's clonotypes by class and the most significant of a class.", InputSchema: schema(map[string]any{
		"dataset": datasetArg,
		"base":    textArg("The base time point, by its key (day -7, baseline): the first other than the reference sample's by default."),
		"subject": textArg("The subject to show (the first by default)."),
		"class":   textArg("The class to show and answer: emerging (the default), transient, contracting or persistent."),
		"count":   numberArg("How many of the class to answer in each subject (10 by default)."),
	})},
	{Name: "attribute_samples", Title: "Attribute samples to the reference", Description: "Shows the Tracking view's Attribution part and measures each sample of each subject with a reference sample (an infusion product, a donor) under the dataset's clonotype definition and filter (a job; the result kept with the project): its share in the clonotypes the reference sample holds, and, where the subject has samples before the reference event and the reference shares few clonotypes with them (a donor), the share derived from the reference, each with its 95% interval (Wilson's). Answers each subject's time points, the share before the event and the reference's overlap with it.", InputSchema: schema(map[string]any{
		"dataset": datasetArg,
	})},
	{Name: "find_neighbors", Title: "Find similar receptors by TCRdist", Description: "Shows the Specificity view's Neighbors part and compares each T-cell clonotype of the dataset with every other of the same chains by TCRdist, as tcrdist3 computes it, under the dataset's clonotype definition and filter (a job; the result kept with the project): with the V genes' CDR1, CDR2 and CDR2.5 (the reference data tcrdist3-loops, which must be downloaded) or the CDR3s alone. The options given are kept with the dataset. Answers how many clonotypes were compared and have a neighbor within the radius, and the clonotypes with the most neighbors; with select, the chosen clonotype's nearest, loop by loop.", InputSchema: schema(map[string]any{
		"dataset":  datasetArg,
		"radius":   numberArg("The radius within which two clonotypes are neighbors, in TCRdist units (default: 24 a chain)."),
		"cdr3Only": flagArg("Compare the CDR3s alone, without the V genes' loops."),
		"organism": textArg("Whose loops: human, mouse or rhesus (default: guessed from the V genes)."),
		"top":      numberArg("How many of the clonotypes with the most neighbors to answer (default 10)."),
		"select":   numberArg("A clonotype to show the nearest of, by its rank among those with the most neighbors (from 1)."),
	})},
	{Name: "match_specificity", Title: "Match specificity evidence", Description: "Matches retained chains against a downloaded VDJdb release or the generated vaccine database, and shows per-epitope sample shares. Never downloads a database implicitly. Optionally follows one epitope's matched clonotypes in Tracking.", InputSchema: schema(map[string]any{
		"dataset":  datasetArg,
		"database": textArg("vdjdb or simulated-vaccine"),
		"metric":   textArg("exact or tcrdist"),
		"genes":    flagArg("Require matching V and J calls"),
		"radius":   numberArg("TCRdist radius, 0–1000"),
		"species":  textArg("HomoSapiens, MusMusculus or MacacaMulatta"),
		"minScore": numberArg("Minimum evidence score, 0–3"),
		"cdr3Only": flagArg("Compare CDR3s alone for TCRdist"),
		"follow":   numberArg("Epitope/antigen group to follow, from 1"),
	})},
	{Name: "measure_bcell_shm", Title: "Measure somatic hypermutation", Description: "Compare saved D-masked germlines with source rows. Reports evaluable bases/codons, regional substitutions, single-hit R/S and unclassified changes.", InputSchema: schema(map[string]any{"dataset": datasetArg, "germlines": textArg("Saved germlines artifact ID; latest result if omitted")})},
	{Name: "summarize_bcell_isotypes", Title: "Summarize B-cell isotypes", Description: "Count observed heavy constant calls by sample and family, retaining ambiguity and missing abundance. Unordered family co-occurrence is not switching-event inference.", InputSchema: schema(map[string]any{"dataset": datasetArg, "families": textArg("Saved thresholded families artifact ID; latest result if omitted")})},
	{Name: "build_bcell_lineages", Title: "Build B-cell lineage trees", Description: "Build family/locus parsimony trees from saved germlines, with conditional ancestral states and observed metadata. Exact through 8 states; bounded heuristic through 64.", InputSchema: schema(map[string]any{"dataset": datasetArg, "germlines": textArg("Saved germlines artifact ID; latest result if omitted")})},
	{Name: "reconstruct_bcell_germlines", Title: "Reconstruct D-masked B-cell germlines", Description: "Uses saved thresholded families and checked ungapped references. Preserves indels, masks D/N/P, and records exclusions and provenance.", InputSchema: schema(map[string]any{"dataset": datasetArg, "families": textArg("Saved families artifact ID; latest thresholded result if omitted"), "reference": textArg("ogrdb-human-2026-05-27 (default) or simulated-bcell-immunization")})},
	{Name: "find_bcell_families", Title: "Find B-cell clonal families", Description: "Measures nearest distinct-receptor distances, and assigns single-linkage families only with an explicit threshold. Respects subject or sample boundaries; paired mode uses both chains. Keeps a cited immutable result.", InputSchema: schema(map[string]any{"dataset": datasetArg, "mode": textArg("auto, heavy or paired"), "scope": textArg("subject (default) or sample"), "threshold": numberArg("Inclusive normalized distance 0–1. Omit for distances only; zero means exact identity.")})},
	{Name: "compare_receptor_expression", Title: "Compare receptor and expression neighborhoods", Description: "CoNGA-style paired alpha-beta clonotype graph comparison with tied neighbors, conditional overlap tests and all-scale Bonferroni correction.", InputSchema: schema(map[string]any{"phenotypes": textArg("Saved computed-map phenotype artifact; displayed result if omitted"), "options": map[string]any{"type": "object", "properties": map[string]any{"neighbors": map[string]any{"type": "array", "items": map[string]any{"type": "integer"}, "description": "1–8 distinct k values, 1–200; default [5,15]"}, "cdr3Only": map[string]any{"type": "boolean", "description": "Explicitly omit V-germline loops; false requires organism and reference"}, "organism": textArg("human or mouse; required for full TCRdist")}}})},
	{Name: "compute_clonotype_modularity", Title: "Compute clonotype modularity", Description: "Test within-clone connectivity on a saved expression graph, using size-matched subsets and BY-adjusted p-values.", InputSchema: schema(map[string]any{"phenotypes": textArg("Saved computed-map phenotype artifact; the displayed phenotype result if omitted"), "options": map[string]any{"type": "object", "properties": map[string]any{"permutations": numberArg("Monte Carlo draws: 99–99999; default 9999"), "seed": numberArg("Positive integer seed; default 20261024")}}})},
	{Name: "compute_cell_phenotypes", Title: "Compute clonotype phenotypes", Description: "Save sample-scoped clonotype sizes, map overlays and supplied phenotype counts for the open expression input and its receptor binding.", InputSchema: schema(map[string]any{"map": textArg("Computed expression-map artifact ID, latest for the open input, or empty for imported coordinates"), "population": textArg("receptor (default) or mapped"), "annotation": textArg("Supplied phenotype column; empty summarizes the sample"), "breakpoints": map[string]any{"type": "array", "items": map[string]any{"type": "integer"}, "description": "Inclusive size category bounds; default [1,2,5]"}})},
	{Name: "compute_expression_map", Title: "Compute expression map", Description: "Normalize the open expression input, compute PCA, exact neighbors and UMAP, and save an immutable result. Open expression evidence first.", InputSchema: schema(map[string]any{"options": map[string]any{"type": "object", "properties": map[string]any{"minGenes": numberArg("Minimum detected genes per cell; default 1"), "minCells": numberArg("Minimum cells per gene; default 1"), "features": numberArg("Variance-ranked features, 2–1000; default 500"), "components": numberArg("Principal components, 2–100; default 30"), "neighbors": numberArg("Other neighbors, 2–100; default 15"), "target": numberArg("Normalized count total; default 10000"), "seed": numberArg("Seed; default 20261024"), "epochs": numberArg("UMAP epochs, 10–1000; default 500"), "minDist": numberArg("UMAP minimum distance, 0–1; default 0.5")}}})},
	{Name: "inspect_expression", Title: "Inspect expression evidence", Description: "Open a MEX, 10x HDF5, AnnData or MuData input and check exact barcode coverage in an explicit receptor sample. Saves source fingerprints; no normalization or clonotype inference.", InputSchema: schema(map[string]any{"path": textArg("MEX directory or 10x HDF5 file on this computer"), "anndata": map[string]any{"type": "object", "description": "Explicit AnnData/MuData selections; layer is X or layers/<name>", "properties": map[string]any{"layer": textArg("Count matrix: X or layers/<name>"), "modality": textArg("Required expression modality for MuData"), "embedding": textArg("Optional obsm embedding name"), "airrModality": textArg("Required AIRR modality for MuData when AIRR key is given"), "airrKey": textArg("Optional obsm AIRR key"), "featureName": textArg("Optional var name column"), "featureType": textArg("Optional var type column")}, "required": []string{"layer"}}, "anndataExample": map[string]any{"type": "boolean", "description": "Use the generated artificial AnnData example"}, "cellsPath": textArg("Optional cell annotation TSV for MEX/10x HDF5 on this computer"), "hdf5Example": map[string]any{"type": "boolean", "description": "Use the already generated artificial HDF5 expression example"}, "studyPreset": textArg("cell-states (default) or cell-states-clouds; requires studyExample"), "studyExample": map[string]any{"type": "boolean", "description": "Use the generated cell-states study, first expression sample and its annotations"}, "example": map[string]any{"type": "boolean", "description": "Use the already generated artificial expression example"}, "dataset": datasetArg, "sample": numberArg("Explicit zero-based receptor sample index; dataset empty for expression only")})},
	{Name: "inspect_bcell_inputs", Title: "Inspect B-cell inputs", Description: "Shows B cells, the filtered rearrangement evidence counts and a bounded first page. Optionally inspects one row. Does not reconstruct germlines or measure mutation.", InputSchema: schema(map[string]any{
		"dataset": datasetArg, "select": numberArg("Rearrangement on the first page to inspect, from 1 (at most 50)."),
	})},
	{Name: "flag_invariant", Title: "Flag invariant populations", Description: "Evaluate published MAIT, iNKT and GEM sequence rules for an explicit species. Shows Clonotypes, optionally narrowed to a matched population, and answers sample totals, evaluable abundance and shares. Records the species with the dataset; flags do not establish phenotype or specificity.", InputSchema: schema(map[string]any{
		"dataset": datasetArg, "species": textArg("human or mouse (required)"), "population": textArg("MAIT, iNKT, GEM, or empty for all clonotypes"),
	})},
	{Name: "find_networks", Title: "Find clonotype networks", Description: "Finds connected components of clonotypes joined within a sequence-distance cutoff, retaining all edges and memberships independently of the drawing. Shows Networks in Specificity and answers component counts. Options are kept with the dataset.", InputSchema: schema(map[string]any{
		"dataset":  datasetArg,
		"metric":   textArg("identity, hamming, levenshtein or tcrdist"),
		"cutoff":   numberArg("0 for identity, 0–2 for edit distance, 0–1000 for TCRdist"),
		"arms":     textArg("all retained chains or any shared chain"),
		"sameV":    flagArg("Require the same V gene at the definition's resolution"),
		"sameJ":    flagArg("Require the same J gene at the definition's resolution"),
		"cdr3Only": flagArg("TCRdist on CDR3s alone"),
		"select":   numberArg("Cluster number to select, from 1"),
	})},
	{Name: "find_neighborhoods", Title: "Find neighborhoods against a background", Description: "Shows the Specificity view's Neighborhoods part and finds, for each receptor of one locus, the largest radius at which a background holds few enough receptors and the dataset's receptors within it (a job; the result kept with the project), as Mayer-Blackwell et al. 2021 define a neighborhood. The background is drawn from a recombination model with the dataset's V and J usage, or made by moving the dataset's own CDR3s among its genes. The options given are kept with the dataset. Answers how many receptors have a neighborhood and the first by enrichment, each with its radius, its neighborhood's receptors, how much commoner it is than in the background, and its motif.", InputSchema: schema(map[string]any{
		"dataset":      datasetArg,
		"background":   textArg("model (drawn from a recombination model; the default) or shuffled (the dataset's own CDR3s moved among its genes)."),
		"locus":        textArg("The locus whose chains are compared (default: the dataset's commonest)."),
		"control":      numberArg("The share of the background a radius may hold: 1e-5, 1e-6 (the default) or 1e-7."),
		"minNeighbors": numberArg("How many receptors a neighborhood needs to be reported, its centroid counted (default 2)."),
		"order":        textArg("Which neighborhoods first: enriched (the default), neighbors or abundant."),
		"top":          numberArg("How many neighborhoods to answer (default 10)."),
		"select":       numberArg("A neighborhood to show the receptors of, by its rank in the order asked for (from 1)."),
	})},
	{Name: "measure_generation", Title: "Compute generation probabilities", Description: "Shows the Specificity view's Generation part and computes each clonotype's generation probability, the probability that V(D)J recombination alone makes its CDR3s, as OLGA computes it, under the dataset's clonotype definition and filter (a job; the result kept with the project): under OLGA's default models (the reference data olga-models, which must be downloaded) or ClonoLoom's own model of human TRB; a pair's is the product of its chains'. The options given are kept with the dataset. Answers the models and how many clonotypes have a probability, each sample's quartiles (log10), and the first clonotypes in the order asked for.", InputSchema: schema(map[string]any{
		"dataset":  datasetArg,
		"models":   textArg("Whose models: olga, clonoloom, or \"\" for OLGA's where downloaded, else ClonoLoom's (the default)."),
		"organism": textArg("human or mouse, or \"\" to guess from the V genes (the default)."),
		"genes":    flagArg("The probability of each CDR3 with its V and J genes, not of the CDR3 alone."),
		"order":    textArg("Which clonotypes first: abundant (the default), probable or improbable."),
		"top":      numberArg("How many clonotypes to answer (default 10)."),
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
