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
	{Name: "open_data", Title: "Open data", Description: "Opens files or folders on this computer as one dataset of the project: AIRR rearrangement tables with their Repertoire metadata (.json), Cell Ranger V(D)J output folders or contig annotations, immunoSEQ exports. Needs the token.", InputSchema: schema(map[string]any{
		"paths": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Absolute paths, read together as one dataset."},
		"name":  textArg("The dataset's name (its files' otherwise)."),
	}, "paths"), reads: true},
	{Name: "open_example", Title: "Open an example dataset", Description: "Opens one of the example datasets as a dataset of the project. One not yet downloaded is downloaded first when download is true and its data are openly licensed; data under a provider's terms of use are downloaded only by the person, who accepts the terms in the window.", InputSchema: schema(map[string]any{
		"example":  textArg("The example's name, such as 10x-pbmc-1k (get_state lists them)."),
		"opening":  textArg("What of it to open, such as \"T cells\" (its first if not given)."),
		"download": flagArg("Download it first if it is not downloaded (openly licensed data only)."),
	}, "example")},
	{Name: "show", Title: "Show a view", Description: "Shows a view (start, data, qc, clonotypes, jobs), with a dataset in focus.", InputSchema: schema(map[string]any{
		"view":    textArg("start, data, qc, clonotypes or jobs."),
		"dataset": datasetArg,
	}, "view")},
	{Name: "set_definition", Title: "Define clonotypes", Description: "Chooses a dataset's clonotype definition: a preset by name, or one stated in full.", InputSchema: schema(map[string]any{
		"dataset":    datasetArg,
		"preset":     textArg("A preset's name (or part of it), such as \"Paired αβ\" or \"Nucleotide junction\"."),
		"definition": map[string]any{"type": "object", "description": "A definition in full: {name, chains: [] | [locus] | [locus, locus|\"light\"], junction: aa|nt, v: allele|gene|family|none, j: …, cells: primary|all, requireAll, productiveOnly, vendor}."},
	})},
	{Name: "select_clonotype", Title: "Choose a clonotype", Description: "Shows the Clonotypes view and chooses a clonotype, which the inspector describes: by its rank in the table (1 for the first) or its ID.", InputSchema: schema(map[string]any{
		"dataset": datasetArg,
		"rank":    numberArg("Its rank in the table as sorted, from 1."),
		"id":      textArg("Its ID (c-…)."),
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
