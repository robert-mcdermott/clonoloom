# ClonoLoom

ClonoLoom is an open-source, local-first workbench for adaptive immune receptor repertoires: T- and
B-cell receptor sequencing, bulk and single-cell. It takes the output of an annotation pipeline
(Cell Ranger, immunoSEQ, MiXCR, IgBLAST through Change-O, TRUST4, or any AIRR Community file) to
results others can check: the data validated, clonotypes defined explicitly, diversity and overlap
measured with their uncertainty, clonotypes followed over time, receptors grouped by likely
specificity, B-cell families traced, and figures, methods and certificates written, each step kept
on record.

It is one self-contained program for macOS, Linux and Windows. It opens as a desktop window, needs
no Python, R or account, and keeps your data on your computer.

**Status:** in development; the first release, 0.1.0, is being built
([changelog](CHANGELOG.md), [roadmap](clonoloom-spec/roadmap.md)).

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/images/clonotypes-dark.webp">
  <img alt="ClonoLoom's Clonotypes view: 10x Genomics' 10k PBMC T cells grouped into paired αβ clonotypes by amino-acid junction, V and J genes, the definition stated above the table, and the largest clonotype's chains and cells in the inspector" src="docs/images/clonotypes-light.webp">
</picture>

## In 0.1.0

The first release opens a study's receptor data, checks them and defines clonotypes explicitly,
with a project that records where every number came from:

- **Data.** AIRR Rearrangement TSV, Cell Ranger V(D)J outputs and immunoSEQ exports, read by
  ClonoLoom's own readers, each file fingerprinted by its SHA-256; a validator whose findings say
  how many rows, show examples and say what to do; the abundance unit (cells, templates, reads,
  UMIs) stated, never guessed; samples annotated by hand, from file names or from metadata.
- **QC.** Each sample's productive share, clonotypes, singletons, sample coverage, the largest
  clonotype's share and gene-call resolution; for single cells, the chains of each cell; the
  vendor's own numbers recomputed from the rows; filters saved as versions, each with its reason.
- **Clonotypes.** Named definitions (junction, V and J levels, chains, single-cell rules) stated
  in words with every table; a million clonotypes sorted and searched as they scroll.
- **The record.** Example data with their licenses and citations, and a methods paragraph per
  dataset with numbered references, written from what was done.

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/images/data-dark.webp">
  <img alt="The Data view of 10x Genomics' PBMC 1k T cells: the dataset with its format, unit and example-data license, the validator's note on junctions without their anchors, the sample, and the file's SHA-256, source and license in the inspector" src="docs/images/data-light.webp">
</picture>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/images/qc-dark.webp">
  <img alt="QC of 10x Genomics' 10k PBMC T cells: cells by their productive chains, the share of productive contigs and the junction lengths of the clonotypes" src="docs/images/qc-light.webp">
</picture>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/images/methods-dark.webp">
  <img alt="A dataset's methods paragraph: the data's source and license, how ClonoLoom read and checked them, and the clonotype definition, with numbered references" src="docs/images/methods-light.webp">
</picture>

<sub>Pictures: 10x Genomics' Human PBMC 1k (v1.1) and 10k PBMC (5′ v2.0) datasets, CC BY 4.0,
opened from ClonoLoom's Example data. Made by `node docs/capture/capture.mjs`.</sub>

## What it is for

- **Trust the data.** AIRR-C, Cell Ranger and immunoSEQ files read by ClonoLoom's own readers,
  checked by a validator that says what is wrong, where, and what to do about it.
- **State the clonotype definition.** Nucleotide or amino-acid junction, V and J genes or not, one
  chain or paired chains: a named choice shown with every table and figure.
- **Measure repertoires with their uncertainty.** Hill numbers, richness estimators, clonality
  and overlap with intervals; rarefaction and extrapolation to compare at equal depth or coverage.
- **Follow clonotypes.** Each clonotype a thread through a subject's samples, with expansion tests
  that account for sampling and biological variance.
- **Find likely specificities.** TCRdist neighborhoods, specificity groups and database matches,
  with their evidence and backgrounds.
- **Trace B-cell families.** Clonal families, germlines, somatic hypermutation and lineage trees.
- **Compare cohorts.** The subject, never the cell or the sequence, as the unit.
- **Write results others can check.** Figures with the definition and unit in their legends, a
  methods section written from the record, AIRR-compliant exports, and certificates.

Every method is implemented in ClonoLoom, cites its publication, and is checked against an
independent reference implementation or a simulated truth.

## Install

Releases are not published yet. Build from source with Go (the version in `go.mod` or newer):

```sh
go build -o clonoloom .
```

Once releases are published, one file per platform will be installed by a one-line installer that
checks its checksum ([docs/INSTALLING.md](docs/INSTALLING.md)).

## Getting started

[docs/GETTING-STARTED.md](docs/GETTING-STARTED.md) walks through a first study.

```sh
clonoloom                                  # the start page
clonoloom /path/to/airr_rearrangement.tsv  # or open your own data
```

ClonoLoom opens in a desktop window: a Chromium-family browser (Chrome, Edge, Brave or Chromium)
in app mode with its own profile, or else your default browser. Name files, Cell Ranger output
folders or folders of them on the command line, or use **Open data** in the window. Data are read
where they are: ClonoLoom never copies, changes or moves them. Running `clonoloom <paths>` again
hands them to the ClonoLoom already running.

To try it without your own data, choose **Example data**: public 10x Genomics, AIRR Community and
immuneACCESS datasets, each shown with its source, size, license and citation and downloaded from
its publisher only when you ask.

<details>
<summary>Options</summary>

| Flag | Default | What it does |
| --- | --- | --- |
| `--port` | 8860 | Preferred port; the next free one up to 8879 is used if it is taken |
| `--window` | `app` | `app` (a desktop window), `browser` (the default browser) or `none` |
| `--keep-running` | off | Keep serving after the window is closed |
| `--data-dir` | `<user config dir>/ClonoLoom` | Where the project library is kept |
| `--examples-dir` | `<data dir>/examples` | Where example data are downloaded, when asked |
| `--offline` | off | Download nothing: ClonoLoom then reaches no other computer |
| `--host` | 127.0.0.1 | Interface to bind |
| `--dev` | off | Serve `web/` from the working directory (development) |
| `--version` | | Print `clonoloom X.Y.Z` and exit |

</details>

## Documentation

- [Getting started](docs/GETTING-STARTED.md): a first study, view by view.
- [Installing](docs/INSTALLING.md): release files, installers, upgrading, uninstalling.
- [Formats](docs/FORMATS.md): what each reader reads and how.
- [Requirements](clonoloom-spec/requirements.md): what ClonoLoom must do.
- [Design](clonoloom-spec/design.md): how it is built and why.
- [Roadmap](clonoloom-spec/roadmap.md): the order of the work, wave by wave (wave *N* is release
  0.*N*.0).
- [Research](clonoloom-spec/research.md): the methods, formats and datasets behind it, with
  citations.
- [Conventions](clonoloom-spec/conventions.md): code, science and terminology rules.

## Development

```sh
go test -race ./...                      # the Go program
node --test "web/lib/**/*.test.mjs"      # the browser modules
node validation/run.mjs                  # checks against references (validation/README.md)
go run . --dev --window none             # serve web/ from disk at http://127.0.0.1:8860
node docs/capture/capture.mjs            # the documentation's pictures (docs/images)
```

The web app is plain ES modules with no build step and no dependencies. CI tests each commit
once. A release is a tag (`v0.1.0`) whose files CI has passed on; the release workflow builds the
six binaries and drafts the release.

## Siblings

ClonoLoom is the fifth of a family of local-first research workbenches that share their design,
security model and conventions: [Proteoscope](https://github.com/robert-mcdermott/proteoscope)
(protein structures), [CytoWeave](https://github.com/robert-mcdermott/cytoweave) (cytometry),
[MaveScape](https://github.com/robert-mcdermott/mavescape) (multiplexed assays of variant effect)
and [SituCraft](https://github.com/robert-mcdermott/situcraft) (spatial transcriptomics).

## Research use

ClonoLoom is for research use only. It is not a medical device and makes no diagnostic claims.

## License

[Apache License 2.0](LICENSE). See [NOTICE](NOTICE).
