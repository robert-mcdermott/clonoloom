# Remote control

A program on your computer (a script, a notebook, the documentation's screenshot capture) can
drive the open ClonoLoom window: open data, choose a clonotype definition, show a view, write a
dataset's methods paragraph. Each action is performed in the window, where you see it, and what it
changes goes into the project's history like any edit of yours, so it can be undone.

## Turn it on

```sh
clonoloom --remote-control
```

ClonoLoom then writes `remote.json` to its data folder (`~/Library/Application Support/ClonoLoom`
on macOS, `~/.config/ClonoLoom` on Linux, `%AppData%\ClonoLoom` on Windows, or `--data-dir`):

```json
{ "url": "http://127.0.0.1:8860", "token": "…", "version": "0.2.0", "pid": 12345 }
```

The file is readable by you only, and removed when ClonoLoom stops. Keep a ClonoLoom window open:
actions are performed by the window most recently opened.

## Send an action

POST `{"action": …, "args": {…}, "client": "what your program calls itself"}` to
`/api/remote/action`. The answer is `{"ok": true|false, "message": …, "data": …}`.

```sh
curl -s http://127.0.0.1:8860/api/remote/action -d '{"action": "get_state", "client": "my script"}'
```

```python
import json, pathlib, urllib.request

info = json.loads(pathlib.Path("~/Library/Application Support/ClonoLoom/remote.json").expanduser().read_text())

def act(action, **args):
    request = urllib.request.Request(info["url"] + "/api/remote/action",
        data=json.dumps({"action": action, "args": args, "client": "my notebook"}).encode(),
        headers={"Content-Type": "application/json", "X-ClonoLoom-Token": info["token"]})
    return json.load(urllib.request.urlopen(request))

act("open_data", paths=["/data/study/airr_rearrangement.tsv", "/data/study/repertoires.airr.json"])
act("set_definition", preset="Nucleotide junction")
print(act("show_methods")["data"]["text"])

# A VDJtools table, through a mapping of its columns.
table = "/data/vdjtools/patient7_day0.txt"
act("open_data", paths=[table], mappings={table: {
    "columns": {"junction_aa": "cdr3aa", "junction": "cdr3nt", "v_call": "v", "j_call": "j", "count": "count"},
    "unit": "reads"}})
```

`GET /api/remote/tools` lists every action with its arguments as JSON Schema.

| Action | What it does |
| --- | --- |
| `get_state` | The project: its datasets (format, unit, rows, samples, findings, filter, definition, license and citation of public data), the view shown, the example datasets. |
| `new_project` | Starts a new project (`name`). |
| `open_data` | Opens files or folders as one dataset (`paths`, `name`). Another tool's CSV or TSV opens through a mapping of its columns, by path in `mappings`: `{columns: {junction_aa, junction, v_call, d_call, j_call, c_call, count, sample, locus, productive, cell_id, clone_id, sequence_id: a column's name}, unit: reads, umis, templates or cells (with a count), sample: one name for every row}` ([FORMATS.md](FORMATS.md#tables-with-mapped-columns)). Needs the token. |
| `open_example` | Opens an example dataset (`example`, `opening`); `download: true` downloads openly licensed data first. |
| `show` | Shows a view: `start`, `data`, `qc`, `clonotypes`, `repertoire`, `tracking`, `specificity` or `jobs` (`view`, `dataset`). |
| `set_definition` | Chooses a dataset's clonotype definition: a `preset` by name, or a `definition` in full. |
| `annotate_samples` | Sets samples' annotation (`dataset`; `samples`: by name, `{subject, timepoint, reference, tissue, cellSubset, condition, batch, hla}`, `""` to remove a field), as the Data view's samples table holds it. |
| `describe_subjects` | Shows the Tracking view and answers the dataset's subjects (`dataset`): each one's samples in time order with their time points read as days, its reference samples and replicates, and the checks' findings with their codes. |
| `track_clonotypes` | Shows the Tracking view's Clonotypes part and follows clonotypes through each subject's samples in time order (a job; the result kept): `by` `reference` (each reference sample's `n` largest, 10 by default; the default), `largest` (each sample's `n` largest, 5 by default), `list` (`list`: clonotype IDs or junctions), `test` (the `n` most significant calls of the dataset's comparison, 20 by default: `test_expansion`) or `class` (`class`: `emerging`, `transient`, `contracting` or `persistent`; the `n` most significant against the dataset's base time point, 20 by default: `class_clonotypes`). Answers each subject's time points and each clonotype's count, frequency and 95% interval (Wilson's) at each, the samples taken at one time pooled (`dataset`, `subject`: the one shown). |
| `class_clonotypes` | Shows the Tracking view's Classes part and, in each subject, tests each clonotype between a base time point and every later one by the dataset's test, and classes it: emerging, transient, contracting or persistent (a job; the result kept). Answers each subject's clonotypes by class, and its `count` (10 by default) most significant of a `class` (emerging by default) with their frequencies and calls (`dataset`; `base`: a time point's key, the first other than the reference sample's by default; `subject`: the one shown). |
| `attribute_samples` | Shows the Tracking view's Attribution part and measures each sample of each subject with a reference sample (a job; the result kept): its share in the reference sample's clonotypes, and, where the reference shares few clonotypes with the samples before the event (a donor), the share derived from it, each with its 95% interval. Answers each subject's time points, its share before the event and the reference's overlap with it (`dataset`). |
| `find_neighbors` | Shows the Specificity view's Neighbors part and compares each T-cell clonotype with every other of the same chains by TCRdist, as tcrdist3 computes it (a job; the result kept): with the V genes' CDR1, CDR2 and CDR2.5 (the reference data `tcrdist3-loops`, downloaded first) or the CDR3s alone. The options given are kept with the dataset. Answers how many were compared and have a neighbor within the radius, and the `top` clonotypes with the most neighbors; with `select`, that one's nearest is shown (`dataset`, `radius`, `cdr3Only`, `organism`, `top`, `select`). |
| `match_specificity` | Shows Specificity’s Database part and matches chain evidence. Arguments: `dataset`; `database` (`vdjdb`, `simulated-vaccine`); `metric` (`exact`, `tcrdist`); `genes`; `radius` (0–1000); `species` (`HomoSapiens`, `MusMusculus`, `MacacaMulatta`); `minScore` (0–3); `cdr3Only`; optional `follow` (epitope/antigen group number, from 1). Returns artifact, match count, sample names and epitope shares. Never downloads implicitly; prepare Reference data or generate the vaccine example first. |
| `find_networks` | Shows Specificity's Networks part and finds the full graph and connected components; options are kept with the dataset. Returns the artifact ID and node, edge and component counts. Arguments: `dataset`; `metric` (`identity`, `hamming`, `levenshtein`, `tcrdist`); `cutoff`; `arms` (`all`, `any`); `sameV`, `sameJ`, `cdr3Only`; `select` (cluster number, from 1). Exports contain all nodes and edges independently of the drawing limit. |
| `find_neighborhoods` | Shows the Specificity view's Neighborhoods part and finds, for each receptor of one locus, the largest radius at which a background holds few enough receptors and the dataset's receptors within it (a job; the result kept), as Mayer-Blackwell et al. 2021 define a neighborhood. The background is drawn from a recombination model with the dataset's V and J usage, or made by moving the dataset's own CDR3s among its genes. Answers how many receptors have a neighborhood and the `top` in the `order` asked for, each with its radius, neighbors, enrichment and motif; with `select`, that one's receptors (`dataset`; `background`: `model`, the default, or `shuffled`; `locus`; `control`: `1e-5`, `1e-6`, the default, or `1e-7`; `minNeighbors`; `order`: `enriched`, the default, `neighbors` or `abundant`; `top`; `select`). |
| `measure_generation` | Shows the Specificity view's Generation part and computes each clonotype's generation probability, the probability that recombination alone makes its CDR3s, as OLGA computes it (a job; the result kept): under OLGA's default models (the reference data `olga-models`, downloaded first) or ClonoLoom's own model of human TRB, a pair's the product of its chains'. The options given are kept with the dataset. Answers the models, how many clonotypes have a probability, each sample's quartiles (log₁₀) and the `top` clonotypes in the `order` asked for (`dataset`; `models`: `olga`, `clonoloom` or `""`, OLGA's where downloaded; `organism`: `human`, `mouse` or `""`, guessed; `genes`: with the V and J genes; `order`: `abundant`, the default, `probable` or `improbable`; `top`). |
| `test_expansion` | Shows the Tracking view's Tests part and tests each clonotype between two time points of each subject, the samples at one time pooled (a job; the result kept): `from` and `to`, the time points' keys (`day -7`, `baseline`, `reference`); `test` `auto` (the beta-binomial test where its variance can be fitted, else the binomial test; the default), `betabinomial`, `binomial` or `fisher`; `stable`, two time points to fit the beta-binomial test's variance on instead of the replicate samples (`["day 28", "day 90"]`; `[]` for the replicates); `alternative` `two-sided` (the default), `increase` or `decrease`; `correction` `bh` (the default) or `storey`; `alpha` 0.01 (the default), 0.05 or 0.1. Clonotypes with fewer than 5 in the two together are not tested; reads are refused. Answers the test used and the variance fitted, each subject's clonotypes tested, left out, increased and decreased, and its `calls` most significant calls with their counts, p-values and adjusted p-values (`dataset`, `subject`: the one shown). |
| `select_clonotype` | Chooses a clonotype by its `rank` in the table or its `id`. |
| `measure_diversity` | Measures each sample's diversity (a job; the result kept) and answers the main measures with their 95% intervals (`dataset`). |
| `gene_usage` | Counts each sample's genes, junction lengths and amino acids (a job; the result kept, the samples grouped by condition) and answers, for each sample or condition, the most used genes of a segment, the commonest lengths and the mean length, with their 95% intervals (`dataset`; `part`: `genes` or `junctions`; `segment`: `V`, `D`, `J` or `C`; `level`: `gene` or `family`; `weight`: `clonotypes` or `abundance`; `rows`: `samples` or `groups`). |
| `measure_overlap` | Measures every pair of samples' overlap (a job; the result kept, the samples' subjects and conditions from their annotation or metadata) and answers the pairs of one subject and of two summarized, the most overlapping pairs by a measure with their 95% intervals, and the clonotypes by how many subjects hold them (`dataset`; `measure`: `morisita`, the default, `morisitaHorn`, `jaccard`, `jaccardRarefied`, `sorensen`, `sorensenRarefied`, `overlap`, `overlapRarefied`, `shared`, `sharedRarefied` or `abundance`; `pairs`: how many). |
| `rarefy` | Rarefies and extrapolates each sample (a job; the result kept) and answers the levels the samples are best compared at and each sample's clonotypes, Shannon and Simpson diversity at one of them, with their 95% intervals (`dataset`; `level`: `coverage`, the default, `size`, `extrapolatedCoverage` or `extrapolatedSize`). |
| `show_examples` | Opens the Example data dialog. |
| `show_methods` | Shows a dataset's methods paragraph and answers it with its references. |
| `set_theme` | `light`, `dark` or `system`. |
| `close_dialogs` | Closes dialogs, menus and messages. |

## What is allowed

- Only programs on this computer: requests from other computers, and web pages that reach the
  loopback port under another name, are refused.
- Reading files by path (`open_data`) needs the `X-ClonoLoom-Token` header with the token in
  `remote.json`, so another user of a shared computer cannot make your ClonoLoom read your files.
- Data under a provider's terms of use (the immuneACCESS example) are never downloaded by a
  program: only you can accept their terms, in the window.
- One action runs at a time, in the order they come; a page opened from an earlier run of
  ClonoLoom does not take actions (reload it).

Agents (an MCP server whose changes you accept or reject) come in a later release.
