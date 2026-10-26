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
| `show` | Shows a view: `start`, `data`, `qc`, `clonotypes`, `repertoire`, `tracking`, `specificity`, `cohort` or `jobs` (`view`, `dataset`). |
| `set_definition` | Chooses a dataset's clonotype definition: a `preset` by name, or a `definition` in full. |
| `annotate_samples` | Sets samples' annotation (`dataset`; `samples`: by name, `{subject, timepoint, reference, tissue, cellSubset, condition, batch, hla}`, `""` to remove a field), as the Data view's samples table holds it. |
| `build_cohort_features` | Shows Cohort and joins explicit `design` (saved design record ID including version suffix) and `diversity` (saved artifact ID from the same dataset). Discover both with `get_state`, which lists `cohortDesigns` and `artifacts`. Returns the kept artifact ID, subject/observation counts and method description. Requires source opening only for the initial binding; saved results/export are source-free. No pooling or group test. |
| `check_cohort_design` | Shows Cohort and checks `dataset` with `spec: {unit: "subject", layout: "independent" or "paired", groupField: "condition", levels: ["control", "expanded"], exclude: {sample: "reason"}}`. Group fields also include `timepoint`, `tissue`, `cellSubset`. Returns every sample’s disposition, subject counts, findings, sources’ dataset identity and citations; no inferential test. Optional `save: true` and `name` keep a usable named design version in the project. |
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

### B-cell input evidence (wave 5)

`inspect_bcell_inputs({dataset, select})` opens B cells → Inputs and returns evidence counts by
sample and locus, plus the first page (at most 50 rearrangements) under the active QC filter.
`dataset` uses the same dataset selector as the other actions; optional `select` is a one-based
row on that page. It displays that row's original sequence annotations and findings in the
inspector. The response contains bounded summaries, not all sequences. This inspection neither
reconstructs a germline nor computes mutation or family membership.


### B-cell clonal families

`find_bcell_families` opens Families and keeps the result. Optional arguments: `dataset`,
`mode` (`auto`, `heavy`, `paired`), `scope` (`subject`, `sample`), and `threshold` (0–1).
Omitting the threshold measures nearest distances without assigning families; zero means exact
identity. Subject scope uses Data annotations or supplied subject metadata. Paired distance is the larger normalized chain
distance, within both chains' V/J/length partitions. The response reports eligibility, exclusions,
receptor/family counts and the artifact identity. `inspect_bcell_inputs` returns to Inputs.

### D-masked B-cell germlines

`reconstruct_bcell_germlines` opens Germlines and computes or reuses an immutable result.
Arguments: `dataset`, `families` (saved thresholded family artifact ID; the latest such result
if omitted), and `reference` (`ogrdb-human-2026-05-27`, the default, or
`simulated-bcell-immunization`). References must already be downloaded or generated;
this action does not download anything. It returns the artifact ID, reconstructed/assigned
source-row counts, family/locus count, exclusions and parent/reference provenance.

HTTP clients use `POST /api/germlines` with `{dataset, germlines: {families, reference}, start}`.
`start: false` checks availability. `GET /api/germlines/{id}` reads the full result;
`GET /api/germlines/{id}/page?kind=families|rows&family=...&locus=...&offset=0&limit=50`
reads bounded pages (maximum 200). Saved inspection needs neither open inputs nor a reference
download; computing requires the original dataset and verified family/reference bytes.

### Somatic hypermutation

`measure_bcell_shm` opens B cells → SHM and computes or reuses a saved result. Pass `dataset`
and optionally `germlines` (a saved germline artifact ID; latest for this dataset if omitted).
The response includes the artifact ID, source-row/evaluated counts, exclusions, codon exclusions
and effective options. It requires an open original dataset and a completed germline parent.

HTTP: `POST /api/shm` with `{dataset, shm: {germlines}, start}`. `start: false` checks availability.
The server resolves and pins `germlineSha256`; unrelated current QC, clonotype and diversity
choices do not affect this derived result. `GET /api/shm/{id}` returns the complete document;
`GET /api/shm/{id}/page?kind=summaries|rows&offset=0&limit=50` returns bounded pages (maximum 200).
For `kind=rows`, optional `family` and `locus` filters select source rows. The saved document can
be inspected without reopening sources or downloading references; new computation checks the
parent bytes and source-row identities. `GET /api/export?what=result&artifact=...` exports all
row/region statistics, pooled summaries, complete JSON, manifest and source notice.

### Lineage trees (wave 5, slice 7)

`build_bcell_lineages({dataset, germlines})` opens B cells → Lineages and computes or reuses
an immutable result. Omit `germlines` to use the latest saved germline result for that dataset.
The action uses current sample timepoint/tissue annotations and returns artifact ID, built-tree
and source-row counts, exclusions and normalized limits. Annotations are pinned in identity.

`POST /api/lineages` takes `{dataset, lineages:{germlines}, study?, start?}`. Omitted `study`
uses imported repertoire timepoint/tissue; an explicit array overrides that metadata, with
unspecified samples empty. Duplicate and unknown names are refused. `start:false` checks the
cache without starting work. The server pins parent SHA-256 and fixed search limits.
`GET /api/lineages/{id}` returns the complete result. `/page?kind=trees|rows&offset=0&limit=50`
pages summaries or members (limit 1–200); row filters are `family`, `locus`, `tree`, `node`.
`/page?kind=detail&tree=ID` returns one bounded tree with panel and nodes. Saved pages require
neither original input nor parent availability. Export uses the standard result endpoint.

### Isotypes (wave 5, slice 8)

`summarize_bcell_isotypes({dataset, families})` opens B cells → Isotypes and computes or
reuses the result. Omit `families` to use the latest saved thresholded family result for the
dataset. It returns artifact ID, assigned observations, resolved class/gene counts, family
count, exclusions, issues and pinned options. It does not infer class-switching events.

`POST /api/isotypes` takes `{dataset, isotypes:{families}, study?, start?}`. The family parent
fixes membership, QC and subject scope. Omitted `study` uses imported timepoint/tissue metadata;
explicit project annotations override it, including explicit missing values. Duplicate/unknown
sample annotations are refused. Normalized labels and the verified family checksum determine
identity. `start:false` probes the cache. The default starts a cancellable job when needed.

`GET /api/isotypes/{id}` returns the complete saved result.
`/api/isotypes/{id}/page?kind=usage|families|pairs|observations&offset=0&limit=50` pages through
it (limit 1–200). Usage defaults to sample-wide totals; `family` selects family/sample totals,
`sample` selects a sample, and `level=class|gene` selects resolution. Observations accept
`family` and `sample`. Families accept `family` and `left`/`right` constant-gene membership
filters, allowing an unordered pair's supporting families to be inspected. Pairs are paged
without filters. Page summaries omit full arrays; rows and count describe the selected view.
Saved inspection uses pinned labels and requires neither parent nor source files.

`GET /api/export?what=result&artifact={id}` exports all tables, full JSON, manifest and notice,
independent of page filters, under the normal source-access and data-use conditions.

### Expression evidence (0.6.0)

`inspect_expression` opens a MEX directory or 10x HDF5 file in Cells and saves its source fingerprints and
binding. Give `path`, `dataset` (ID or name) and an explicit zero-based `sample` index. Give
`dataset: ""` for expression-only inspection. The answer includes the expression ID and raw
barcode coverage. To use the artificial example, first call
`open_example {"example":"synthetic-expression","download":true}`, then
`inspect_expression {"example":true,"sample":0}`. Counts are source coverage before QC or
clonotyping. No expression analysis is inferred. A path and `example:true` are mutually exclusive.

Slice 2 adds optional `cellsPath` for a plain/gzip cell annotation TSV (see `FORMATS.md`). The
response's `annotations` reports matching rows, annotation-only rows, missing rows and mapped
expression barcodes. The source is saved and checked on reopening. To try the paired study:
`open_example {"example":"simulated-cell-states","download":true}`, then
`inspect_expression {"studyExample":true,"sample":0}`. This explicitly selects the generated
`expression/S01-1` matrix and its `cells.tsv`; choose the matching first receptor sample.
`studyExample` and `example` cannot be combined with each other or explicit paths. Other study
samples use explicit `path`, `cellsPath` and receptor `sample`. The Cells view offers Annotations
and Imported map; its coordinates are supplied evidence, never a computed embedding.

For the larger cloud study, call
`open_example {"example":"simulated-cell-states-clouds","download":true}`, then
`inspect_expression {"studyExample":true,"studyPreset":"cell-states-clouds","sample":0}`.
`studyPreset` requires `studyExample` and accepts only `cell-states` or `cell-states-clouds`.
Omitting it preserves the original grid example. Saved records reopen their explicit paths;
changing the preferred visible example never silently redirects an existing saved input.

Slice 3 accepts a 10x feature-barcode HDF5 file in `path`. To use the compressed artificial
example, call `open_example {"example":"synthetic-expression-hdf5","download":true}`, then
`inspect_expression {"hdf5Example":true,"sample":0}`. `hdf5Example`, `example` and `studyExample`
are mutually exclusive and cannot accompany explicit source paths. Saved records always
reopen their selected paths. The response's source role is `matrix-hdf5`; counts and receptor
coverage have the same semantics as MEX. `POST /api/expression/open` accepts the same HDF5
path/example selector and optional `expectedId`; a changed source returns 409 without replacing
the previously verified input. Reading boundaries and unsupported encodings are in `FORMATS.md`.


Slice 4 adds explicit AnnData/MuData selections to `inspect_expression` and
`POST /api/expression/open`. For example:

```json
{
  "path": "/data/study.h5mu",
  "anndata": {
    "modality": "gex",
    "layer": "layers/counts",
    "embedding": "X_umap",
    "airrModality": "receptors",
    "airrKey": "airr",
    "featureName": "gene_name",
    "featureType": "feature_type"
  }
}
```

Only `layer` is mandatory for AnnData; MuData also requires `modality`, and `airrModality`
when `airrKey` is supplied. All choices are persisted. No omitted layer/modality/map/AIRR
key is inferred. An external `cellsPath` cannot accompany AnnData/MuData. Source counts must
be nonnegative integral values; selecting a normalized layer is not a count conversion.
The summary's `anndata` records selections, indices, variable columns, embedding provenance
and AIRR population counts. `unit` is `counts`; UMI semantics are not inferred.

`POST /api/expression/page` adds `airr` and `airr-cells` views (same offset/limit rules).
`airr` returns scalar source fields with null for missing, barcode and one-based chain number.
`airr-cells` returns all AIRR observation barcodes, a nullable chain count and expression
presence; null is a missing AIRR record, zero is a present empty chain list. Feature pages
include `variableValues` aligned to the summary's `variableColumns`. Existing receptor
bindings still require an explicit dataset and sample and are separate from AIRR observations.

Generate `synthetic-anndata` through `open_example`, then use `{"anndataExample":true}`
to inspect its selected count layer, annotations, supplied map and AIRR records. This selector
is exclusive with paths, explicit `anndata` choices and other example selectors. `expectedId`
verifies the full source and selection on reopen; failed opens retain saved evidence.


### Computed expression maps

After `inspect_expression`, `compute_expression_map` analyzes the active expression input:

```json
{"options":{"minGenes":1,"minCells":1,"features":500,"components":30,"neighbors":15,"target":10000,"seed":20261024,"epochs":500,"minDist":0.5}}
```

Options are optional; the server resolves and records every default. Neighbors excludes self.
The HTTP equivalent is `POST /api/expression-map` with `{id, options, start}`; `id` is the open
expression ID and `start:false` only checks for a saved/running result. It returns an artifact ID
and either `ready:true` or a durable job ID. Standard Tasks endpoints handle cancellation and
resumption. `GET /api/expression-map/{id}` verifies and returns the manifest and complete saved
result, without requiring the source files. See [EXPRESSION.md](EXPRESSION.md) for methods,
populations, numerical limits and the meaning of the exported graph.

### Clonotype overlays and phenotypes (0.6.0)

After `inspect_expression` opens an input with a receptor dataset/sample binding, call
`compute_cell_phenotypes` with optional `map` (a computed expression-map artifact ID; empty for
imported coordinates), `population` (`receptor` or `mapped`), `annotation` (a supplied column;
empty for the whole expression sample), and `breakpoints` (default `[1,2,5]`). It uses the
project's current clonotype definition, QC version and sample metadata, saves the immutable
result, and returns its artifact ID, coverage and method description. `GET /api/phenotypes/{id}`
exports the full result with provenance. The corresponding POST endpoint accepts explicit
`id`, `map`, `dataset`, `sample`, `definition`, `filter`, `annotation` (sample metadata including
its exact `name`), `options` and optional `start:false` for lookup. The `options.annotation`
column is distinct from the request's sample `annotation` metadata. No samples are pooled.

### Clonotype modularity (0.6.0)

`compute_cell_phenotypes` also accepts `map:"latest"` to choose the latest saved computed map
for the open expression input; it fails if none exists. The returned artifact still pins its
actual map ID and checksum. The cloud-study guide uses this to prepare a modularity parent.

`compute_clonotype_modularity` accepts `{phenotypes, options}`. `phenotypes` is a saved phenotype
artifact made with a computed map; omission uses the displayed phenotype result. Imported-map
parents fail explicitly. Options are `{permutations:9999, seed:20261024}` by default; exact null
spaces of at most 10000 subsets are enumerated regardless of the Monte Carlo draw setting.

The HTTP equivalent is `POST /api/modularity` with `{phenotypes, options, start}`. Parent IDs are
explicit here. `start:false` looks up the deterministic result/running job without starting work.
The response has `artifact` and `ready`, with `job` when submitted. Tasks handles cancellation
and resumption. Both saved parents are verified; raw source files need not be reopened.
`GET /api/modularity/{id}` verifies and returns the complete saved manifest/result without
requiring parents or sources. See [MODULARITY.md](MODULARITY.md) for the tested population,
exchangeability assumption, binary edges, exact/Monte Carlo tails, BY correction and work bounds.

### Receptor / expression comparison (0.6.0)

`compare_receptor_expression` accepts `{phenotypes, options}`. Omitted `phenotypes` uses the
displayed phenotype result. It must use a computed map and paired primary αβ amino-acid
clonotypes with V/J genes or alleles. Keep the parent receptor dataset open for a new request.

Options: `neighbors` is an array of 1–8 distinct integers in 1–200 (default `[5,15]`);
`cdr3Only: true` explicitly omits germline loops. Otherwise choose `organism: "human"` or
`"mouse"` and install the V-loop reference in Reference data. No automatic fallback or species
inference. The fictional cloud-study step uses `{neighbors:[2,4],cdr3Only:true}`.

HTTP: `POST /api/conga` with `{phenotypes, options, start}`. `start:false` resolves identity/cache
without submitting a job. The server pins both parent hashes, captures required receptor
sequences from the original dataset snapshot and pins the loop reference when used. Jobs
support cancellation/resumption without reopening raw expression/receptor files.
`GET /api/conga/{id}` returns a checksum-verified standalone result. Full methods, population,
conditional null, tie handling, Bonferroni family and bounds: [CONGA.md](CONGA.md).


### AnnData export (0.6.0)

`export_anndata` accepts `{phenotypes}`; omission uses the displayed phenotype result. It
prepares and saves the export, returning its artifact ID and population. Download it in Cells.
Imported or computed-map phenotypes are accepted. Keep original expression sources available
and reopen the original receptor dataset before preparing a new export.

HTTP: `POST /api/anndata-export` with `{phenotypes, start}`; `start:false` resolves the saved
export/running job without submitting work. Jobs support cancellation/resumption, capture
linked clone definitions and verify expression-source and parent hashes. `GET
/api/anndata-export/{id}` returns the checked export record. `GET
/api/anndata-export/{id}/download` returns a ZIP with `cells.h5ad`, export record, manifest and
NOTICE; restricted source terms require `accept=1` after acknowledgment. These GET endpoints
work without parent/source files. See [ANNDATA-EXPORT.md](ANNDATA-EXPORT.md) for the schema.

### Subject group comparisons (wave 7)

`compare_cohort` accepts `{features, options: {features, confidence?, permutations?, seed?}}`.
The outer `features` is a saved `cohort-features` artifact ID from `get_state`; the inner array
selects catalog IDs, for example `{features:"cf-…", options:{features:["hill1","shannon"]}}`.
Use real IDs from the open project. Both tests for each selected feature enter one explicit
BH family. Independent designs use Welch/Mann–Whitney; paired designs use paired t/signed ranks.
No subjects or pairs are silently deleted for missing values. The response includes the result
artifact, family size, testable count and full method description.

HTTP: `POST /api/cohort-compare` with the same fields and optional `start:false` to probe.
Defaults: confidence 0.95, 9999 Monte Carlo draws, seed 20261025. Exact rank null spaces
through 50000 assignments are enumerated; larger spaces use seeded Monte Carlo. Options and
parent checksums determine identity. Tasks supports cancellation and replay. Creation uses
only the saved feature parent; reopening/downloading a saved comparison needs no parents.
`GET /api/cohort-compare/{id}/download` returns complete CSV/JSON/provenance with provider-term
acknowledgment as for other exports. See FORMATS and research §5.32 for methods and limits.

### Replicated clonotype counts (wave 7)

`fit_cohort_counts` accepts a saved subject-feature parent and effective model settings:

```json
{"action":"fit_cohort_counts","args":{"features":"cf-…","options":{"minCount":10,"minSamples":12,"adjustBatch":true,"confidence":0.95}}}
```

The action captures counts from the original open receptor dataset, fits under the saved design
and focuses the kept artifact. It returns artifact ID, fitted count, eligible family and method
description. Paired subjects are blocked; batch adjustment must be requested and estimable.
At least 12 subjects/group and 24 residual df are required. Raw and BH p-values accompany log2
comparison/reference effects and conditional marginal Wald intervals. No Poisson fallback,
composition normalization, automatic discovery labels or silent sample deletion is performed.

HTTP uses `POST /api/cohort-counts` with the same fields, optionally `start:false`; the returned
job can be polled/cancelled through `/api/jobs`. `GET /api/cohort-counts/{id}` and `/download`
verify the saved result, with provider-term acknowledgment on restricted downloads. Durable
replay needs the pinned feature parent but no raw source; saved reopening/export is standalone.
See research §5.33 for assumptions, filtering, bounds and calibration limits.

### Definition robustness (wave 7)

`audit_definitions` starts from a saved `cohort-compare` artifact. Send `comparison` and
1–5 `definitions`, retaining the parent's chain, cell, require-all and productivity policies.
Only junction and V/J resolution may differ. For an AA productive bulk baseline, for example:

```json
{"action":"audit_definitions","args":{"comparison":"cc-<saved-id>","definitions":[{"name":"NT with V/J genes","junction":"nt","v":"gene","j":"gene","chains":[],"cells":"primary","productiveOnly":true,"requireAll":false,"vendor":false}]}}
```

The response gives the artifact, full BY family, per-feature direction/interval summaries and
method description. The original dataset supplies the initial abundance snapshots under the
pinned QC. Subjects, exclusions, features, tests, confidence and seeds stay fixed; choose
reasonable alternatives before examining outcomes. Neither a p threshold nor across-definition
pooling determines a robustness verdict. The audit covers aggregate features, not changing
individual-clonotype identities. See the [guide](site/pages/docs/definition-robustness.html).

HTTP accepts the same fields at `POST /api/definition-robustness`, optionally `start:false`.
Use `/api/jobs` to poll/cancel. Verified GET `/api/definition-robustness/{id}` and `/download`
work from saved results; ZIPs preserve source terms and full evidence. Durable replay needs
the saved comparison and captured job specification, without original raw data.

### Motifs and sequence-signature groups (wave 7)

Call `screen_motifs` with `dataset` (name or ID; optional for the current dataset), explicit
`query` and `background` sample-name arrays and optional `options`:

```json
{"query":["S01","S02","S03","S04"],"background":["S05","S06","S07","S08","S09","S10","S11","S12"],"options":{"minK":2,"maxK":5,"minMembers":3,"q":0.05,"minFold":2}}
```

The current AA definition, QC and project subject annotations are captured. The response gives
the saved artifact, pooled family size, reported count and complete method description. The
visible Specificity → Motifs controls use the same route. Samples must be distinct/disjoint;
missing or ineligible source evidence is refused. Reporting settings do not change the family.

HTTP uses `POST /api/motifs` with dataset ID, definition, filter, query/background, optional
subject map/options and `start:false` for readiness probing. Poll/cancel through `/api/jobs`.
GET `/api/motifs/{id}`, `/group/{group}` and `/download` inspect/export checksum-verified saved
results. The ZIP retains all signatures, receptors, methods and source terms; sources are not
needed to reopen or replay captured evidence. See [the motif guide](site/pages/docs/motifs.html)
for bounds, GLIPH2 adaptations, conditional Fisher/BY assumptions and lack of specificity proof.

### Subject recurrence and HLA association (wave 7)

`analyze_convergence` accepts a saved `motifs` artifact ID and an explicit `selection`:

```json
{
  "motifs": "saved-motif-artifact-id",
  "selection": {
    "species": "human",
    "samples": ["sample-1", "sample-2"],
    "subjects": [
      {"id": "subject-1", "samples": ["sample-1"], "typing": {"A": ["HLA-A*02:01", "HLA-A*03:01"]}},
      {"id": "subject-2", "samples": ["sample-2"], "typing": {"A": ["HLA-A*01:01", "HLA-A*24:02"]}}
    ],
    "groups": ["saved-signature-id"],
    "alleles": ["HLA-A*02:01"]
  }
}
```

Replace illustrative IDs with saved IDs and use the complete intended subject universe;
two subjects illustrate the schema, not a testable study. Each sample must come from the
parent's captured arms and belong to exactly one named subject. Multiple samples of one
subject are unioned. Supply both copies at each tested locus; missing/ambiguous typing is
unknown. Empty alleles gives recurrence only. Declare signatures and alleles before inspecting
outcomes. The result returns artifact ID, evaluable subjects, full family, tested count and
method; Specificity → HLA uses the same route and saved artifact identity.

HTTP uses `POST /api/convergence` with the same body (`start:false` validates/probes without
submission). GET `/api/convergence/{id}` and `/download` reopen/export checked evidence.
ZIPs include raw subject IDs/HLA typing, complete association and recurrence tables, and NOTICE;
provider terms use `?accept=1`. Replay is source/parent-free. See the
[guide](site/pages/docs/convergence.html) for typing rules, conditional tests and limits.
