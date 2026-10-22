# Projects that keep opening

Every release saves a project here, in a folder named for its version
(`clonoloom-spec/roadmap.md`, "Projects that keep opening"). Every later version must open each one
and make its results again. `go test -run TestEveryReleasesProjectStillOpens .` checks this
(`projects_test.go`):
- each dataset opens from the fixture's files under the ID the project records;
- each result is made again from the parameters in its manifest, under the same artifact ID;
- each result document matches the one that release wrote.

`web/lib/project.test.mjs` checks that each project still parses and saves back to the same bytes.

## What a fixture holds

| Path | What it is |
| --- | --- |
| `project.json` | The project as the release saved it. Its files' paths are relative to the fixture's folder; everything else is as saved. |
| `data/` | The datasets' files, as the project's checksums record them. |
| `artifacts/<id>/manifest.json` | Each result's manifest: what it was made from, by which method and version, and its documents' checksums. |
| `artifacts/<id>/<kind>.json.gz` | Each result's document, gzipped. |

## Why values, not checksums

The documents are compared value by value: numbers to a relative 1e-9 (an absolute 1e-12 near
zero). Their checksums are not compared, because 0.2.0's results are not yet the same on every
processor, as `clonoloom-spec/conventions.md` requires. Go fuses a multiplication and an addition
into one instruction on arm64 but not on amd64, and Go's logarithm and exponential differ between
processors. So the last digits of a number can differ between the computer that saved a fixture
and the one testing it.

The 0.2.0 fixture was saved on arm64. The amd64 build of the same commit writes the same overlap
document byte for byte, but different bytes for the other documents. Its largest difference was
under 0.1% of the tolerance. The test logs whether each document's bytes are the same. Once results
are the same on every processor (`roadmap.md`, wave 2, slice 8), the test compares checksums.

A later version may add to a document, but not change what it said. A result made again under
another ID fails the test: its method's version changed. The release that changes a method must
say why in its CHANGELOG and teach the test which results it no longer remakes. The fixtures
themselves are never edited.

## The fixtures

### 0.2.0

The fixture holds two datasets, each with the results 0.2.0 keeps:
- **Small simulated study.** `clonoloom simulate --preset small` (seed 7) by ClonoLoom 0.2.0: two
  subjects, one TRB sample each, 1,561 rearrangements. The tables are gzipped AIRR TSV, with
  their Repertoire metadata. Results: diversity, rarefaction, gene usage and overlap.
- **A VDJtools table.** The first 80 rows of `S02-1`, rewritten as VDJtools columns and opened
  through a column mapping, counted in UMIs. Result: diversity.

Every result uses the default definition (amino-acid junction, V and J genes), no filter, 200
bootstrap replicates and seed 20140101.

It was made through remote control on an empty library, with these actions:
`new_project`, `open_data`, `measure_diversity`, `rarefy`, `gene_usage` and `measure_overlap`.
The project and artifacts were then copied here.
