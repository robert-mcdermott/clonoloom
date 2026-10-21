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
- **One implementation.** Headless runs (`clonoloom run`), certificates (`clonoloom verify`) and the
  MCP server compute without a page. With all science in Go, each method exists once and is tested
  once (`go test`), and a headless run cannot differ from the window.
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
the cache (`<UserCacheDir>/clonoloom`, size-capped, least recently used removed first), keyed by
the source's fingerprint and the reader's version, so that a cohort reopens in seconds. Entries
are written to a temporary name and renamed into place.

### The host, from the siblings

| From | What | How much changes |
| --- | --- | --- |
| SituCraft 0.9.0 | `security.go`, `window.go`, `records.go`, `browser_*.go` | renames only |
| SituCraft 0.9.0 | `main.go`, `local.go`, `store.go` | renames; file kinds (AIRR tables, vendor outputs, folders) |
| SituCraft 0.9.0 | `remote.go`, `output.go`, `connection.go`, `mcp.go` framework, `run.go`/`verify.go` pattern | transport kept; actions and tools new (waves 2 and 8) |
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
| HDF5 | SituCraft's own reader of the subset AnnData and 10x matrices use (wave 6) |
| Default port | 8860 (8860–8879) |
| Remote control and screenshots | From wave 2, as in SituCraft: the documentation's capture scenes and headless runs drive the page through it |
