# Projects that keep opening

Every release saves a project here, in a folder named for its version
(`clonoloom-spec/roadmap.md`, "Projects that keep opening"). Every later version must open each one
and make its results again. `go test -run TestEveryReleasesProjectStillOpens .` checks this
(`projects_test.go`):
- each dataset opens from the fixture's files under the ID the project records;
- each result is made again from the parameters in its manifest, under the same artifact ID;
- each result document matches the one that release wrote.

`web/lib/project.test.mjs` checks that each project still parses and saves back to the same bytes.

## The fixtures

| Version | Data | Results |
| --- | --- | --- |
| 0.2.0 | A small simulated study (two samples) and a VDJtools table read through a mapping | Diversity (two), rarefaction, gene usage, overlap |
| 0.3.0 | A small simulated CAR-T series: two patients, two samples a week before the infusion and four after it, with their infusion products (14 samples, gzipped, 330 KB) | Diversity, tracking (each product's 10 largest), tests (the week before against two weeks after, the beta-binomial test fitted on the replicates), classes (against the week before), attribution |
| 0.4.0 | Twelve synthetic paired T cells exercising invariant rules, and 80 simulated beta-chain rows from the 0.2.0 fixture | TCRdist on CDR3s, Hamming networks, generation probabilities under ClonoLoom’s model; saved human invariant rules and sample counts |
| 0.5.0 | 60 paired artificial B cells in six samples of two subjects, and joined synthetic IMGT annotations | Nearest distances, thresholded families, germlines, SHM, lineages, isotypes; a separate IMGT family result |

| 0.6.0 | Four artificial paired receptor/expression samples, per-sample matrices, annotations and separate truth | Computed expression map, phenotypes, modularity, receptor/expression comparison, AnnData export |

## What a fixture holds

| Path | What it is |
| --- | --- |
| `project.json` | The project as the release saved it. Its files' paths are relative to the fixture's folder; everything else is as saved. |
| `data/` | The datasets' files, as the project's checksums record them. |
| `artifacts/<id>/manifest.json` | Each result's manifest: what it was made from, by which method and version, and its documents' checksums. |
| `artifacts/<id>/<kind>.json.gz` | Each result's document, gzipped. |

## Checksums, and 0.2.0's values

Since 0.3.0, results are the same on every processor, bit for bit (`clonoloom-spec/conventions.md`),
so a fixture's documents are compared by their checksums: the bytes made again must be the bytes
the release wrote.

0.2.0's fixture is compared value by value instead: numbers to a relative 1e-9 (an absolute 1e-12
near zero). 0.2.0's results were not the same on every processor. Go fused multiplications and
additions on arm64 but not on amd64, and its logarithm and exponential differ between processors,
so the last digits of a number differed. The 0.2.0 fixture was saved on arm64; the amd64 build of
the same commit wrote the same overlap document byte for byte, but different bytes for the others,
its largest difference under 0.1% of the tolerance. Results made since (with `internal/dmath` and
every product rounded on its own) differ from 0.2.0's in the same last digits, on both processors,
and within the same tolerance. The test logs whether each document's bytes are the same.

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


### 0.4.0

Created by `node validation/create-project-0.4.mjs` through remote actions and an isolated
browser/library. The script refuses to overwrite an existing fixture; pass a new output path
when checking its generation. Synthetic cells are constructed from rule examples, not
experimental data. `invariant.json` pins all sample totals and rule outcomes after saving the
species in the project. The fixture needs no downloaded references; the generated model is
built in. `projects_test.go` remakes all three result documents and the invariant summary;
the project parser's test checks the entire saved project round trip.

### 0.5.0

Created by `node validation/create-project-0.5.mjs` through the browser and remote actions,
using a fresh artificial immunization study generated by the 0.5.0 build (model v2, seed
20261027). No downloaded data are required. The script refuses to overwrite a fixture.
It selects the first two cells of each observed state/sample in families S01-F001, S01-F002
and S02-F001: 60 paired cells, 120 chain rows, six samples, both light loci and three isotypes.
`observations.tsv` records this subset's known assignments. The original synthetic IMGT
bundle includes all four joined tables and Parameters, with its native annotation provenance.
All input data are artificial and have no data-use restrictions.

The study keeps nearest-only and thresholded families, six family/locus germline groups,
SHM, six lineage trees and isotypes; IMGT keeps a separate heavy-only family result.
`references/` holds the artificial FASTA, and `reference.json` pins its source, license and
citation as recorded at release time. Replay uses this provenance, not a future program's
catalog citation, and verifies the reference through the resulting artifact identity and bytes.
The API test reopens IMGT files with their saved grouping and recreates parents before their
children; the JavaScript test retains all project fields and checks the saved choices and links.

### 0.6.0

Created by `node validation/create-project-0.6.mjs` through the actual seven guided browser
steps in an isolated library. The complete `cell-states-clouds` study retains two subjects,
four samples, 1440 expression cells, 2880 receptor chain rows, four matrices, annotations and
separate truth. Analysis uses S01-1 only: 360 input cells, 359 retained PCA/UMAP cells,
358 uniquely assigned retained cells and ten paired clones. Five saved results form the chain:
expression map, phenotypes, modularity, receptor/expression neighborhoods (k = 2, 4, CDR3-only),
and raw-count AnnData. `cells.h5ad.gz` stores the binary alongside the JSON result documents.

`projects_cells_test.go` runs the current durable workers with each manifest's original job
specification. `locations.json` maps the original generation directory to `data/`; project
reopening paths are relative. Replay verifies source hashes and identities before restoring
only recorded source paths and receptor file modification timestamps for comparison. No counts,
annotations, geometry, assignments or statistics are replaced. Verified original parent files
and manifests are then installed for each child, preserving original job IDs, timestamps,
software version and build identity. AnnData embeds that recorded provenance, so its complete
binary can be compared byte for byte too. These provenance fields describe the original release,
not when or where the compatibility test runs. All 23 generated source/truth files are also checked against the study's recorded hashes.
The JavaScript check retains every project
field and verifies sample binding, expression identity and the complete ordered parent chain.

Independently check the complete export with:

```sh
uv run --offline --no-project validation/reference/single_cell_release.py
```

This uses pinned AnnData/h5py/SciPy public APIs to compare raw MEX counts, annotations, receptor
links and every supplied/computed embedding row, then verifies an AnnData rewrite.

## 0.7.0 — complete cohort studies

`node validation/create-project-0.7.mjs` generates an isolated real-browser project with five
studies: independent positive, independent null, 12/20 unbalanced, 28-subject paired and 16/16
fully confounded. All 184 source samples and their complete generating truth are retained.
The four usable designs each have diversity → subject features → comparison + count model;
the confounded design retains its failed check and creates no analysis artifact. Five saved
design checks and sixteen artifacts are kept. Original source paths map from `locations.json`
to `data/`; project and design-source reopening paths are portable, scientific records unchanged.

`projects_cohort_test.go` rechecks designs and all truth/source hashes and replays real durable
workers under the recorded IDs, comparing every result byte. Only verified original parent
manifests are restored, retaining job/time/software provenance used by child records. All new
fixture artifact files, including `manifest.json`, are stored with a `.gz` suffix because count
job specs capture raw evidence. Decompression preserves exact original bytes. Earlier fixtures
retain their original storage conventions. JavaScript also checks project byte round-tripping,
all usable result chains, and retention of the failed confounding check in cited Methods.

## 0.8.0 — record and reach

Created by `node validation/create-project-0.8.mjs` through an isolated MCP/HTTP host,
normal window and generated Python/R clients. The full artificial batch-QC study retains
26 observations, complete source/truth files and one native batch-QC result. All four transports
return identical native query JSON. A reviewed definition change is compared and recovered;
certificate and headless replay are exact. Archive import/recovery preserves evidence after
removing original source paths.

`project.json` changes only reopening paths to fixture-relative locations; `original-project.json`,
checkpoint files, certificate, HTML report and portable archive retain their original bytes.
Scientific artifact files and manifests use lossless gzip storage. `projects_record_test.go`
replays the captured native worker, checks source truth, verifies the pinned certificate/archive,
and restores removed result files from the recorded checkpoint without recomputation. The common
release tests also reopen the source dataset and transfer the project through the archive contract.
Saved query text checks transport fidelity, not an independent biological result.
