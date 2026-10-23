# ClonoLoom engineering conventions

ClonoLoom is a workbench for adaptive immune receptor repertoire (AIRR) data: bulk and
single-cell T- and B-cell receptor sequencing. A Go program, built on the standard library, serves
an embedded web app and does all the data work: reading files, validating them, defining
clonotypes, and every statistic that reaches a result. The browser shows results and lets the user
explore them. License: Apache-2.0. No third-party runtime dependencies, in Go or JavaScript.

ClonoLoom is a sibling of Proteoscope, CytoWeave, MaveScape and SituCraft (`~/mycode/…`). It copies
their host, security model, conventions and test approach on purpose, so that one person can
maintain all five. A copied file notes its origin in its header comment ("Adapted from SituCraft
0.9.0 security.go").

## Layout

```
main.go, security.go, local.go, store.go,      Go host (embeds web/), adapted from SituCraft
records.go, window.go
api_*.go                                       HTTP handlers: datasets, QC, clonotypes, results
remote.go, actions.go, output.go,              automation: remote control and its actions,
connection.go, mcp.go, run.go, verify.go       exports to paths, MCP, headless runs (later waves)
internal/airr/       AIRR-C schema, Rearrangement TSV and Repertoire JSON reader and writer
internal/seq/        nucleotides, codons, translation, junction checks
internal/genes/      gene names: parsing, IMGT nomenclature, vendor names mapped, loci
internal/reads/      readers of vendor and tool formats into one rearrangement table
internal/rep/        the rearrangement table (columnar), samples and cells
internal/bcell/      B-cell evidence, clonal families, D-masked germlines, SHM, lineage trees and observed isotypes (wave 5)
internal/validate/   findings with stable codes
internal/clonotype/  definitions, clonotype tables for bulk and single-cell data
internal/qc/         per-sample and single-cell QC
internal/stats/      distributions, tests, multiple testing (adapted from SituCraft)
internal/dmath/      logarithms, exponentials, powers, log-gamma and erfc with the same bits on
                     every processor (wave 3)
internal/random/     seeded random streams
internal/jobs/       durable background jobs (wave 2)
internal/artifacts/  results kept with their lineage (wave 2)
internal/cache/      the table cache (wave 2)
internal/diversity/  diversity, rarefaction and extrapolation (wave 2)
internal/usage/      gene usage, junction lengths and amino-acid composition (wave 2)
internal/overlap/    overlap between samples and public clonotypes (wave 2)
internal/simulate/   seeded simulated repertoires with their truth (wave 2)
internal/export/     exports: AIRR TSV, CSV, and the notice of their data's source and license (wave 2)
web/index.html, web/styles.css, web/app.js     app shell
web/lib/*.js         pure modules: no DOM, no globals, importable by Node
web/lib/*.test.mjs   node:test unit tests, one file per module
web/ui/*.js          DOM components (only these touch document/window)
validation/          checks against independent references and known truth
validation/reference/  committed reference numbers, each with the script that made it
validation/projects/   a project saved by each release, which every later one must open
testdata/            small synthetic files made by scripts in the repository (no real data)
docs/                installation, formats, MCP, network
docs/site/           the website: its generator, pages and audit (published from gh-pages)
clonoloom-spec/      requirements, design, roadmap, research, these conventions
```

## Names

- Product: **ClonoLoom** in prose and the interface. Executable, Go module, repository and client
  packages: `clonoloom`.
- Default port 8860 (tries 8860–8879). Proteoscope 8765–8814, CytoWeave 8770–8789, MaveScape
  8820–8839, SituCraft 8840–8859.
- Data directory `<UserConfigDir>/ClonoLoom`; derived-data cache `<UserCacheDir>/clonoloom`.
- Headers `X-ClonoLoom-Token`.
- Project archive extension `.clz` (a ZIP of the project, its events and small artifacts; source
  data only where their licenses allow).
- The project document's format is `clonoloom-project`.

## Go

- Standard library only. `gofmt`; `go vet` clean; `go test -race ./...` passes.
- Readers take an `io.Reader` (streamed, row by row) or an `io.ReaderAt` and a size, so the same
  code reads files, zip and gzip members and in-memory test data. A reader never loads a whole
  large file to answer a small question.
- Sources are opened read-only. Nothing under a source is ever written, renamed or deleted.
- Untrusted input (every file) is bounds-checked. Sizes declared in headers are checked against
  the actual size and against limits before allocation. Every reader has a fuzz test
  (`go test -fuzz`) with a committed corpus of small cases.
- Long operations take a `context.Context`, check it often, and report progress through a
  callback. Errors a user can fix name the file, line, column and value.
- Parallel code is deterministic: work is split into fixed blocks, each with its own seeded
  random stream (`internal/random`), and results are combined in a fixed order.
- What an artifact holds is the same on every processor. Go may fuse `a*b + c` into one
  multiply-add on some processors (arm64) and not others (amd64), which rounds differently: a
  floating-point product summed into a value an artifact keeps or decides on is rounded on its own
  (`float64(a*b)`). Logarithms, exponentials and powers that reach a result go through
  `internal/dmath`. Sums over a map are taken in sorted order. `validation/fused_test.go` fails on
  any fused multiply-add the compiler makes of ClonoLoom's code for arm64, and the results of every
  analysis and of the simulator are pinned by digests that arm64 and amd64 (CI) both give.
- Processes are started only with argument arrays (`exec.Command(name, args...)`), never via a
  shell.

## JavaScript

- Plain ES modules, modern JavaScript, no build step, no TypeScript. Two-space indent, single
  quotes, semicolons, trailing commas in multi-line literals.
- The page computes no number that reaches a result, export or methods text: those come from the
  program. The page lays out, draws, sorts and filters what it is given.
- Typed arrays for anything per rearrangement, cell or clonotype. No per-row object allocation in
  hot loops.

## Science

- **Cite what you implement.** A file implementing a published method names it in its header
  (authors, year, journal, DOI), and `research.md` lists it. Where the publication leaves a choice
  open, the code comment and `research.md` say "ClonoLoom's choice" and why.
- **The clonotype definition travels with every result.** A table, figure, export or methods
  sentence that counts clonotypes names the definition it used (chains; nucleotide or amino-acid
  junction; V and J resolution; single-cell rules).
- **Units are explicit.** Abundance is in reads, UMIs, templates or cells, named in field names
  and metadata (`templates`, `umi_count`, `cells`), never inferred from values. A frequency says
  what it is a fraction of (all reads, productive templates, cells with a β chain).
- **Junction, not CDR3, unless said.** Internally ClonoLoom keeps the IMGT junction (AIRR
  `junction`, `junction_aa`): the CDR3 with the conserved cysteine (2nd-CYS 104) and
  phenylalanine or tryptophan (J-PHE/J-TRP 118). Vendors' CDR3 columns are converted and the
  conversion recorded. Labels say "junction" or "CDR3", whichever is shown.
- **Gene names follow IMGT nomenclature.** Vendors' names (Adaptive's `TCRBV05-01`) are mapped to
  IMGT's (`TRBV5-1`) by published rules; the original is kept beside it. Alleles are kept as given
  and dropped only by a clonotype definition that says so.
- **Identifiers are stable.** Never row numbers. Vendor IDs (sequence IDs, barcodes, contig IDs)
  are kept as given. A clonotype's ID is derived from its definition and key, so the same
  clonotype has the same ID in every run.
- **Missing is not zero.** An unobserved value is missing; an explicit 0 is 0. A count that was not
  given is not a count of 1.
- **Seeds.** Every stochastic computation (bootstrap, rarefaction draws, permutations, simulation)
  takes a seed (default fixed) and records it.
- **Defaults are serialized.** A result records every effective parameter after defaults are
  resolved, and the method's version.
- **Artifacts are immutable.** Outputs are written to a temporary path, validated, then promoted.
- **Subjects are the unit.** No inferential statistic across groups until the design names its
  replicate unit, which is the subject (or sample when stated). Cells and sequences are never
  replicates.

## Terminology

ClonoLoom is for research use. It is not a medical device and makes no diagnostic claims.

| Use | Never |
| --- | --- |
| "clonotype (by definition …)", "clonal family" for B cells' thresholded groups | "clone" for a result whose definition is not stated |
| "expanded (q < 0.05, beta-binomial test)" | "expanded" without the test and threshold |
| "similar to receptors of known specificity", "match in VDJdb (score 2)", "predicted specificity group" | "specific for", "recognizes", "binds" as a finding |
| "public clonotype (in n of m subjects)" | "public" without the count |
| "generation probability under the model …" | "probability" alone |
| "simulated" on anything generated | presenting simulated data as biology |
| "reads Cell Ranger output", "reads immunoSEQ exports" | implying endorsement by 10x Genomics or Adaptive Biotechnologies |
| "research use only" | minimal residual disease or other clinical claims |

## Tests and validation

- `go test ./...` and `node --test "web/lib/**/*.test.mjs"` are fast (seconds), deterministic, and
  check numbers against independently known values: hand-computed, published, simulated truth, or
  an independent tool's output committed under `validation/reference/` with the script that
  produced it and the tool's version.
- Readers are checked against independent readers (the `airr` library, pandas, scirpy) on every
  development file. Statistics are checked against the reference libraries (immunarch, iNEXT,
  scirpy, tcrdist3, R `stats`) and analytic cases.
- Reference tools are run only by scripts in `validation/reference/`, on the maintainer's machine
  or in a scheduled workflow. They are never linked, bundled or copied from; their outputs are
  committed as numbers or checksums.
- Anything that needs large data, external tools or more than a few minutes runs outside
  per-commit CI (`design.md`, "Testing and CI").
- A slice is done when it runs, its tests and validation pass, its documentation is written, and
  `CHANGELOG.md` says what it added.

Lineage trees use checksum-pinned germline projections and metadata. All scientific scoring,
ancestral states and branch changes stay in Go; JavaScript lays out and labels the saved graph.
Keep exact and heuristic search explicit, conditional IUPAC states distinct from a consistent
assignment, and observed annotations distinct from inferred traits. See research §5.19.

Isotype counts and denominators stay in Go, using checksum-pinned family observations and
sample annotations. Keep class resolution separate from constant-gene/subclass resolution,
missing abundance separate from zero, and unordered family co-occurrence separate from
switching inference. Preserve raw heavy and light calls and count each cell once. See research §5.20.
