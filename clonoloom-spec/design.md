# ClonoLoom design

This document explains how ClonoLoom is built and why. For code conventions, see
`conventions.md`. For the methods, formats and datasets behind these choices, see `research.md`.
For what ClonoLoom must do, see `requirements.md`. For the order of the work, see `roadmap.md`.

## Goals

1. **From an annotation pipeline's output to results others can check.** Open the output of
   Cell Ranger, immunoSEQ, MiXCR, IgBLAST through Change-O, TRUST4 or any AIRR-C file; check it;
   define clonotypes explicitly; measure diversity, overlap and dynamics with their uncertainty;
   group receptors by likely specificity; follow B-cell families; link clonotypes to cell
   phenotype; and write figures, methods and certificates, each step kept on record.
2. **The clonotype definition as a stated choice.** Every result names the definition and the
   abundance unit it used, and the sensitivity of a conclusion to them can be shown.
3. **Defensible numbers.** Every method is implemented in the open, cites its publication, and is
   checked automatically against an independent reference implementation or a simulated truth
   (`validation/`).
4. **Local and private.** Data stay on the computer. ClonoLoom serves only the loopback interface
   and sends nothing anywhere, except downloads and public-data queries the user asks for, to an
   allow-list of hosts.
5. **Self-contained.** One program per platform. No Python, R, containers or other software.
6. **Open.** Apache-2.0. No third-party runtime dependencies. AIRR-C formats in and out.
7. **Research use.** Not a medical device. Wording is fixed in `conventions.md`.

## Architecture

```
┌──────────────────────────── clonoloom (one Go binary, standard library only) ──────────────────────┐
│ host (adapted from SituCraft 0.9.0)                                                                │
│   main.go        flags, startup, single instance, /api/info                                        │
│   security.go    loopback only, Host-header check, same-origin API, CSP                            │
│   window.go      desktop window (Chrome/Edge/Brave --app with its own profile) or default browser  │
│   store.go       library: projects/*.json, content-addressed files, records, trash                 │
│   remote.go, actions.go, connection.go, mcp.go, run.go, verify.go   automation (later waves)       │
│ data plane                                                                                         │
│   internal/airr      AIRR-C schema; Rearrangement TSV and Repertoire JSON, read and written        │
│   internal/reads     Cell Ranger, immunoSEQ, Change-O, MiXCR, OAS/OTS, VDJdb … into one table      │
│   internal/rep       the rearrangement table (columnar, interned strings), samples, cells          │
│   internal/seq, internal/genes   translation and junction checks; IMGT gene nomenclature           │
│   internal/validate  findings with stable codes                                                    │
│ science                                                                                            │
│   internal/clonotype definitions and clonotype tables                                              │
│   internal/qc        per-sample and single-cell QC                                                 │
│   internal/diversity, overlap, expansion, tcrdist, pgen, bcr, tree … (later waves)                 │
│   internal/stats, dmath, random   distributions and tests; the same bits on every processor       │
│   internal/jobs, artifacts         durable background jobs; results with their lineage (wave 2)   │
│   api_*.go           JSON endpoints: datasets, QC, clonotypes, results                             │
│ embed web/ ─────────────────────────────────────────────────────────────────────────────────┐      │
└─────────────────────────────────────────────────────────────────────────────────────────────│──────┘
                                                                                              ▼
┌──────────────────────────────────────────── browser ──────────────────────────────────────────────┐
│ web/app.js           shell, views, commands, shortcuts, the project document                      │
│ web/ui/*.js          views and components (the only code that touches the DOM)                    │
│ web/lib/*.js         pure modules (no DOM): charts as SVG, tables, the project, formats            │
└───────────────────────────────────────────────────────────────────────────────────────────────────┘
```

### Why the program computes every number

CytoWeave and MaveScape compute in the browser because their data fit there. SituCraft splits the
work: Go for transcripts and images, the browser for cell-level analytics. ClonoLoom gives **every
number that reaches a result to Go**, and the browser lays out and draws. This departs from the
product requirements' first sketch (statistics in browser workers), for four reasons:
- **Scale.** A bulk immunoSEQ sample holds up to a few million rearrangements, a cohort tens of
  millions. TCRdist neighborhoods, generation probabilities and clustering are quadratic or heavy.
  These need goroutines across all cores and memory the browser does not give a page.
- **One implementation.** Headless runs (`clonoloom run`) and certificate verification
  (`clonoloom verify`) compute without a page. MCP uses the normal window for proposal review and
  dispatch; its scientific actions call the same Go methods. Each method exists once and is
  tested once (`go test`), with exact replay checked against supported captured evidence.
- **The same bits everywhere.** Go's arithmetic is IEEE 754 on every platform once fused
  multiply-adds are controlled and transcendental functions go through `internal/dmath`
  (`conventions.md`). JavaScript engines need the same care in a second language.
- **Interactive enough.** The program runs on the same computer: a sorted, filtered page of a
  clonotype table of a million rows is a few milliseconds of Go and one small JSON answer.

The browser still owns what makes the workbench pleasant: drawing (SVG charts, a virtualized
table, trees and networks on canvas), linked selection, the project document, undo and redo.

### The rearrangement table

Every reader produces the same thing: a **rearrangement table** (`internal/rep`), whatever the
source format. It is columnar and compact, so a cohort of tens of millions of rearrangements fits
a laptop:
- Strings that repeat (gene calls, sample names, barcodes) are interned in a dictionary and stored
  as integer codes. Junctions are stored once each in a string arena.
- Columns: sample, locus, V/D/J/C calls (as given, and as genes with alleles dropped), junction
  (nucleotides) and junction (amino acids), productive, in-frame and stop-codon flags, the
  abundance in its unit, reads and UMIs when given, cell barcode and contig ID for single-cell
  data, the source row, and any column the reader did not map (kept, as strings, so exports lose
  nothing).
- Each dataset records its **abundance unit**: `reads`, `umis`, `templates` or `cells`. A reader
  never guesses it from values; when a format leaves it open (an AIRR file with only
  `duplicate_count`), the validator says so and the user confirms it.
- Missing values are kept missing (an absent count is not a count of 1).

A **dataset** is one source (a file or a vendor's output folder) with its fingerprint (size,
modification time, SHA-256), its format and reader version, its samples and its validation
report. A **sample** is the unit of a repertoire: one immunoSEQ file, one `repertoire_id` of an
AIRR file, one Cell Ranger run. Samples carry the annotation the user gives (subject, timepoint,
tissue, cell subset, condition, batch, HLA).

### Clonotypes

A **clonotype definition** is a named, versioned value:
- the chains: one locus (TRB, TRA, IGH …) or paired (TRA+TRB, IGH+IGK/IGL);
- the junction: nucleotides or amino acids;
- V and J: by gene (alleles dropped), by allele, by family, or not used;
- productive rearrangements only (the default; amino-acid definitions require it);
- for single-cell data, the rules for cells with missing or extra chains (primary chain by UMIs,
  all chains, or the cell left out) and whether cells with one chain count;
- for B cells (wave 5), a distance threshold on junctions with the same V, J and length.

A clonotype's ID is a hash of the definition's canonical form and the clonotype's key, so the same
clonotype has the same ID in every run, sample and computer. A **clonotype table** is computed on
request for a set of samples and a definition: each clonotype's key, its abundance per sample in
the dataset's unit, its frequency, its cells for single-cell data, and its rearrangements. Tables
are cached in memory by (datasets' fingerprints, filter, definition) and served to the page in
sorted, filtered pages.

### Results and their record

From wave 2, a result the user keeps (a diversity analysis, an expansion test, a figure) is an
**artifact** (`internal/artifacts`, adapted from SituCraft): an immutable folder with its outputs,
its spec (every effective parameter, the definition, the filter, the seeds), its inputs by
fingerprint, the method's and program's version, and checksums. The project document records the
artifacts it uses and the decisions taken about them. Methods text and certificates are written
from these records, never by hand. Quick, deterministic answers (QC, a clonotype table page) are
computed on demand and not stored, since their spec reproduces them exactly.

### Jobs

Computations that take longer than a second or two (TCRdist neighborhoods, generation
probabilities, lineage trees, bootstraps over a cohort, downloads) are jobs (`internal/jobs`,
adapted from SituCraft from wave 2): an immutable spec, a state (`queued → running → validating →
complete`, or `cancelled | failed | orphaned`), progress, a log and cancellation, surviving a page
reload. Their outputs are checked before they become artifacts.

### The derived-data cache

Sources are read in place and never modified. From wave 2, a parsed rearrangement table is kept in
the cache (`<UserCacheDir>/clonoloom/tables`, size-capped at 10 GB by default, least recently used
removed first), keyed by the readers' version and each source's SHA-256, size and name, so that a
cohort reopens in a fraction of a second. Each source is hashed again before the cache is asked, so
a changed file is never answered from it. Entries are written to a temporary name and renamed into
place, and carry a CRC-32 of their contents; one that fails it is removed and the files read
again.

### The host, from the siblings

| From | What | How much changes |
| --- | --- | --- |
| SituCraft 0.9.0 | `security.go`, `window.go`, `records.go`, `browser_*.go` | renames only |
| SituCraft 0.9.0 | `main.go`, `local.go`, `store.go` | renames; file kinds (AIRR tables, vendor outputs, folders) |
| SituCraft 0.9.0 | `remote.go`, `output.go`, `connection.go`, `run.go`/`verify.go` pattern | transport kept; actions and tools new (waves 2 and 8) |
| CytoWeave 0.8.1 and MaveScape 0.8.0 | `mcp.go` hosting/lazy-window pattern | modern per-request metadata plus explicit legacy compatibility; shared ClonoLoom grants/review and exact JSON bounds (wave 8, slice 7) |
| SituCraft 0.9.0 | `internal/random`, `internal/dmath`, `internal/stats`, `internal/jobs`, `internal/artifacts`, `internal/hdf5`, `internal/anndata`, `internal/parquet` | copied with tests when a slice needs them |
| SituCraft 0.9.0 | `web/ui/dom.js`, `overlays.js`, `store.js`, `palette.js`, `icons.js`, `sidebar.js`, `inspector.js`, `drawer.js`, `storage.js` | renames; ClonoLoom's views |
| SituCraft 0.9.0 | `web/lib/sha256.js`, `colormaps.js`, `colorvision.js`, `project.js` pattern, `zip.js`, `png.js`, `pdf.js`, `figures.js`, `methods.js`, `certificate.js` | copied with tests when a slice needs them |
| SituCraft 0.9.0 | CI and release workflows, installers, `validation/fetch.mjs` and `sources.json`, `docs/capture`, `clients/generate.mjs`, `benchmark/` | renames; data sources new |
| CytoWeave 0.8.1 | design-driven test choice, robustness verdicts | ported in wave 7 |

Each copied file keeps an origin comment, so fixes can be carried across.

### Network and security

As in the siblings:
- ClonoLoom listens on 127.0.0.1 only and requires a loopback Host header.
- API calls must be same-origin, under a strict CSP; the page is cross-origin isolated.
- Paths can be read only when named on the command line, by a later launch, in the Open dialog or
  by a project the user opened.
- Processes are started with argument arrays, never shell strings (only the window's browser).
- Archive extraction is guarded against path traversal and decompression bombs.

Outbound requests are only those the user starts, to an allow-list of hosts (public dataset and
reference sources: 10x Genomics, GitHub releases, OPIG, AIRR Data Commons repositories, VDJServer,
OGRDB, Adaptive's immuneACCESS). Each download shows its source, size, license and citation first,
and is checked against a recorded checksum. Redirects are followed only to the same hosts, over
HTTPS. ADC queries send only the query, never local data. `--offline` turns all of it off.

Default port 8860 (8860–8879). The siblings use 8765–8859.

## Development data

Real data never go in the repository or the binary, except small synthetic files made by scripts
in `testdata/`. Each public source is listed in `validation/sources.json` (URL, size, checksum,
license, citation, the tests that use it); `validation/fetch.mjs` downloads it into
`validation/cache/` (git-ignored; `CLONOLOOM_DATA` overrides), verifies it and records SHA-256 for
every extracted file. Only sources with an open license (CC0, CC BY) are used by per-commit CI;
those under terms of use are fetched for the maintainer's validation runs only and never
redistributed. The sources chosen for each wave, with their licenses, are in `research.md`
("Datasets").

## Testing and CI

Tests and CI follow the siblings: **each commit is tested once**.
- A push to any branch runs CI.
- A pull request from a branch of this repository is not tested again: it shows the branch's
  checks. Only pull requests from forks run CI themselves.
- A merge whose files equal its branch's (same tree) skips testing.
- A newer push to the same branch cancels an unfinished run.
- The release workflow builds and drafts the release only after checking that a CI run passed on
  exactly the tagged files. It never re-runs the tests.
- Documentation-only changes (Markdown, the spec, license, citation) do not run CI.

CI jobs (all on fixed data):
- **Go:** gofmt, vet, `go test -race ./...`, and the six-platform cross-compile.
- **Web modules:** `node --check`, `node --test`.
- **Validation:** on the open development datasets, cached by checksum, with recorded reference
  numbers.
- **Installers:** syntax checks.

Anything slow, large or external (cohort-scale runs, reference tools re-run) runs in a weekly and
on-demand workflow or on the maintainer's machine, and its outputs are committed as reference JSON.

## Budgets

Targets, confirmed by measurement in the slice that builds each piece and checked by tests
thereafter (on a 2024 laptop: Apple M-series or recent x86, 16 GB):

| Scenario | Target |
| --- | --- |
| Open and validate a 1M-rearrangement immunoSEQ sample | < 5 s |
| Memory of the rearrangement table | < 64 bytes per rearrangement plus its junctions |
| Reopen a 100-sample cohort from the cache | < 2 s |
| A sorted, filtered page of a 1M-row clonotype table | < 100 ms |
| TCRdist neighborhoods of 100k paired TCRs | < 2 min on 10 cores, < 8 GB |
| Generation probabilities of 1M CDR3s | < 2 min on 10 cores |
| Binary size | < 30 MB per platform |

## Decisions

| Decision | Choice |
| --- | --- |
| Product and executable | **ClonoLoom**, `clonoloom`; Go module `clonoloom`; project archives `.clz` |
| Go version | The siblings' minimum (`go 1.24` in `go.mod`); releases built with the current stable Go |
| Where numbers are computed | In Go, always ("Why the program computes every number") |
| Internal model | The AIRR-C Rearrangement fields, in a columnar table with interned strings; junction (IMGT) rather than CDR3; IMGT gene nomenclature |
| Reference data | Germline sets and model files downloaded on request with their license recorded; nothing under a non-commercial or no-derivatives license bundled (`research.md`, "Licenses") |
| Reference tools | Validation only, run by scripts in `validation/reference/`; never linked, bundled or copied |
| HDF5 | Own bounded reader of the AnnData/10x subset (wave 6); SituCraft has a writer, not a reusable HDF5 reader |
| Default port | 8860 (8860–8879) |
| Remote control and screenshots | From wave 2, as in SituCraft: the documentation's capture scenes and headless runs drive the page through it |


### Selected AnnData/MuData evidence (wave 6, slice 4)

The expression input owns the selected sparse count matrix, its source observation annotations,
variable annotations, one supplied embedding and optional unfiltered AIRR records. MuData
selection is modality-local; exact barcodes link selected expression and AIRR populations.
Neither global MuData row indices nor imported clonotype/chain-index analysis objects determine
ClonoLoom results. `internal/hdf5` handles bounded storage decoding; `internal/expression`
validates AnnData schema and scientific count/missing-value policies. No Python runtime is
required by the application. Python public APIs generate independent test fixtures only.

`anndata` in the open request and saved expression record pins count layer, modality, embedding,
AIRR modality/key and optional feature-name/type columns. The whole source file and those
selections determine the `clonoloom-expression-anndata/1` identity. Existing source identity
domains remain stable. Summaries contain provenance and population counts; large variable/AIRR
values are paged, and maps retain the existing population limit. Source observations stay
supplied evidence; computed preprocessing, neighborhoods and phenotypes remain separate results.


Computed expression analyses (wave 6 slice 5) use separate `expression-map` jobs and artifacts,
keyed by expression identity, normalized options and method version. They retain population
exclusions, feature moments/loadings, PCA scores, exact directed neighbors, fuzzy graph weights,
UMAP coordinates and complete methods. Source fingerprints are verified again when a job runs;
completed results are checksum-verified and reopen without the sources. They are independent
of receptor QC and clonotype definitions. See `docs/EXPRESSION.md` for explicit bounded methods.

Clonotype modularity (wave 6 slice 7) consumes a complete `phenotypes` artifact made with a
computed map and that exact `expression-map` graph. `internal/modularity` validates the graph,
induces its uniquely assigned population, computes size-conditioned edge tests and BY adjustment.
It does not reopen raw sources or refit the graph. Parent IDs/checksums, effective settings and
method version determine a separate `modularity` job/artifact. The worker verifies hashes again,
stages JSON, checks cancellation and promotes only a readable result. Resume recreates work
from the pinned spec. Completed results are self-contained for inspection; reproducing the graph
test also uses the exported expression-map parent. JS lays out supplied null frequencies and
observed/expected markers, filters tables and links the parent overlay; Go computes statistics.
See `docs/MODULARITY.md` for the inferential assumptions and populations.

### Exact repertoire metadata inspection (wave 9, slice 1)

`internal/reads/metadata.go` captures bounded source bytes once and hashes that same snapshot;
compressed metadata is decoded under a second bound. `internal/airr/metadata.go` checks JSON
ambiguity, Unicode, structural budgets and record IDs before extracting annotation summaries.
Each repertoire retains its original object bytes. IDs are unique across a dataset's metadata
files; file order and per-file record counts retain the source association.

`api_metadata.go` shares the annotation-matching rule with dataset summaries. The read-only
`GET /api/datasets/{id}/metadata` lists 20 records by default (1–50 allowed via `limit`, zero-based
`offset`). Go returns match coverage and basis. `?record=N` returns one original object as
`rawJSON`, a JSON **string**, so browser decoding cannot round numbers inside the evidence.
Detail previews over 4 MiB fail explicitly; imports retain the object. The view uses text nodes
and project-generation/cancellation guards. Inspection reads the opened snapshot, not paths
supplied by callers. Existing loopback/same-origin protections apply; no new automation grant,
MCP tool, scientific artifact kind, project schema or replay contract is added.

Import limits: 64 MiB per source and decompressed file, 64 MiB combined decoded metadata,
100,000 records per file/dataset, JSON nesting depth 64, 1,000,000 values per file, and 4,096 bytes
per repertoire ID. A declared schema version is information, not schema validation. Summary
labels can combine processing annotations; raw evidence preserves their individual identity.

### Bounded YAML projection and original source (wave 9, slice 2)

`internal/airr/yaml.go` parses a documented YAML 1.2 subset to exact typed values and then JSON,
reusing the repertoire validation/extraction rules. No floats are constructed. Depth, parser
work, line and source/projection limits bound the parser; it reports unsupported syntax rather
than guessing. The native reader dispatches using the supplied filename, including gzip and
content-addressed uploads. JSON behavior and prior dataset identity rules stay unchanged.

For YAML, `reads.MetadataFile` retains `Format` and one `SourceText` snapshot. `SourceText` has
`json:"-"` so dataset summaries and listings cannot unintentionally serialize entire documents.
The detail endpoint adds `format: "yaml"`, the decoded `rawJSON` string and the `sourceYAML`
string (or an explicit oversized-source notice). Original JSON inputs keep their original
object-byte contract. The YAML projection may be lexically indented in Go for display without
numeric decoding. Each preview remains bounded at 4 MiB. The UI labels the projection and shows
the original YAML document separately; it never round-trips evidence through JavaScript numbers.

### Versioned MiAIRR reporting review (wave 9, slice 3)

`internal/airr/miairr.go` assesses a captured repertoire with the pinned 82-field AIRR 2.0
inventory in `miairr_fields.go`. Native results contain checklist/schema/declaration versions,
explicit evidence-row states, reporting levels, conditions, JSON-pointer paths and remedies.
Unsupported/absent declarations and exceeded bounds return no partial rows or counts. Original
source/projection text is unchanged; no scientific number enters a browser checklist calculation.

Metadata detail responses add `checklist`; the UI renders a collapsed read-only review with a
group filter and native counts. Parent-scope failures show unassessed children, not fabricated
missing leaves. Each repeated sample/diagnosis/PCR/processing record is separate. The checklist
adds no persistent artifact, action, permission or replay kind and does not alter dataset identity.
Presence, reporting requirements, conditional applicability, schema conformance and scientific
validity remain distinct. See `docs/MIAIRR.md` and research §5.59 for coverage and limits.

### Imported AIRR object evidence (wave 9, slice 4)

`airr.ReadDataFile` shares the bounded JSON/YAML decoders with Repertoire import and adds
Clone/Cell/Receptor/Node records with raw evidence and explicit links. `ResolveObjectLinks`
checks the combined opening after all metadata files load. Native immutable records stay on
`reads.Result`, outside table caches, with file indices and source fingerprints. This preserves
existing table and dataset identity contracts. `MetadataFile.objects` is omitted when zero.

`GET /api/datasets/{id}/airr-objects` serves bounded pages and record details from the opened
snapshot. Scientific JSON is returned as text; the browser renders native link states. The
Data-view inspector has cancellation, project/selection guards and explicit unassessed limits.
No action/permission/replay kind is added. Imported membership never modifies the active
clonotype definition, chain pairing or analysis counts. See `docs/AIRR-OBJECTS.md` and §5.60.

### Native repertoire reports (wave 9, slice 5)

`reads.native_reports` recognizes TRUST4 and VDJtools column contracts separately from generic
mapped tables. Existing explicit mappings take precedence. Source counts retain ambiguous units;
the existing Data unit confirmation supplies a human interpretation. Marker normalization is
format-specific, with no invented chains, cells, sequences or frequency denominators.

`Source.Native` retains bounded original header/row text and physical line numbers. It is omitted
from generic source JSON; cache header versioned by reader version 4 serializes it explicitly.
`GET /api/datasets/{id}/native-report` serves pages and bounded text previews from the opened
snapshot. The browser only renders text and uses ordinary row/source indices. This adds no action,
permission or replay kind and preserves scientific identity version 1. See `docs/NATIVE-REPORTS.md`.
