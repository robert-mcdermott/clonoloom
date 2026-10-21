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

```sh
clonoloom                                  # the start page
clonoloom /path/to/airr_rearrangement.tsv  # or open your own data
```

ClonoLoom opens in a desktop window: a Chromium-family browser (Chrome, Edge, Brave or Chromium)
in app mode with its own profile, or else your default browser. Name files, Cell Ranger output
folders or folders of them on the command line, or use **Open data** in the window. Data are read
where they are: ClonoLoom never copies, changes or moves them. Running `clonoloom <paths>` again
hands them to the ClonoLoom already running.

<details>
<summary>Options</summary>

| Flag | Default | What it does |
| --- | --- | --- |
| `--port` | 8860 | Preferred port; the next free one up to 8879 is used if it is taken |
| `--window` | `app` | `app` (a desktop window), `browser` (the default browser) or `none` |
| `--keep-running` | off | Keep serving after the window is closed |
| `--data-dir` | `<user config dir>/ClonoLoom` | Where the project library is kept |
| `--host` | 127.0.0.1 | Interface to bind |
| `--dev` | off | Serve `web/` from the working directory (development) |
| `--version` | | Print `clonoloom X.Y.Z` and exit |

</details>

## Documentation

- [Installing](docs/INSTALLING.md): release files, installers, upgrading, uninstalling.
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
