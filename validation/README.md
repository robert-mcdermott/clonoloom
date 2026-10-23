# Validation

`node validation/run.mjs [suite …] [--verbose] [--require-data]` runs the validation suites and
exits with status 1 when a check fails. Each check states what is compared, the value found and
the tolerance required.

| Suite | What it checks | Data |
| --- | --- | --- |
| `accessibility` | WCAG AA contrast of every text-on-surface pair in both themes, with color-vision-friendly colors off and on; palettes, status colors and diverging maps apart in protanopia, deuteranopia and tritanopia (Machado et al. 2009, CIEDE2000) | `web/styles.css` |

Go tests in this folder check whole results against independent references (`go test
./validation/`):

| Test | What it checks | Data |
| --- | --- | --- |
| `TestReadersAgreeWithIndependentReaders` | Every column ClonoLoom reads from every development file equals what an independent reader reads: the AIRR Community's `airr` library for AIRR files, pandas for Cell Ranger, immunoSEQ, MiXCR, OAS and OTS and tcrdist3 files; immunoSEQ gene names mapped as the dataset records them, a data unit's metadata as given | `reference/readers.json` (made by `reference/read_repertoires.py`), the files of `sources.json` |
| `TestTCRdistEqualsTCRdist3` | Every TCRdist between Dash et al. 2017's 1,924 mouse and 415 human clones, α, β and paired and each loop's alone, equals tcrdist3's; every clone's neighbors and nearest, found with pruning in parallel, equal those of every distance | `reference/tcrdist3.json` (made by `reference/tcrdist3_distances.py`), `dash-2017-tcrdist3`, `tcrdist3-loops` |
| `TestTCRdistNeighborsWithinBudget` | The neighbors of 100,000 paired TCRs (the OTS donors' chains paired at random) within 2 minutes on 10 cores and 8 GB (not under the race detector) | `ots-healthy-pbmc`, `tcrdist3-loops` |
| `TestPgenEqualsOLGA` | The generation probability of 99,949 CDR3s under OLGA's seven default models (the OTS, Dash and OAS chains, CDR3s OLGA draws from each model, nucleotide junctions, CDR3s with their V and J genes and with residues of any amino acid) within a relative 10⁻⁹ of OLGA 1.3.0's; where OLGA leaves out the longest insertions at one codon position (mouse TRA), against OLGA on a copy of the model that counts them, and never below OLGA's value on the model as shipped | `reference/pgen.tsv.gz` and `pgen.json` (made by `reference/olga_pgen.py`), `olga-models` |
| `TestPgenOfTheSimulatorsModel` | Under ClonoLoom's own model of human TRB, 200 junctions (chosen from a pilot run) as frequent among 20 million rearrangements of the simulator as their probabilities say (mean z near 0, mean squared z near 1); and the generator's receptors likewise | the simulator |
| `TestGeneratorAgreesWithPgen` | Receptors drawn from OLGA's human TRB model as frequent as their probabilities say, relative to the junctions compared | `olga-models` |
| `TestPgenWithinBudget` | A million TRB CDR3s' probabilities (productive ones ClonoLoom's generator draws from OLGA's human TRB model) within 2 minutes on 10 cores (not under the race detector) | `olga-models` |
| `TestSpecificityPandas`, `TestSpecificityVaccineShares` | Exact evidence row identities and all 150 sample/epitope shares | `reference/specificity.py`, pandas 2.3.3 on the simulated database; truth distinguishes database coverage from known specificity |
| `TestSpecificityVDJdbMaintainer` | 4,000 exact row-identity lookups in VDJdb 2026-06-03 | Local-only pandas output; no real-data fixtures distributed |
| `TestNetworksEqualScirpy` | All 420 complete cell partitions under identity, Hamming, Levenshtein and TCRdist, all/any arms and with/without V genes | `reference/networks.json`, made by `scirpy_networks.py`; eight 10x runs and three OTS units |
| `TestNetworksConnectSimulatedFamilies` | All 34 observed families connected at TCRdist 48; 29 of 34 at two edits (97 observed members) | `simulated-vaccine/S01-d14.tsv` and its truth |
| `TestNeighborhoodsEqualTCRdist3` | Every centroid's radius, its neighbors in the dataset and in the background, and the rates, odds ratio and χ² computed from them, equal to tcrdist3 0.3's `bkgd_cntl_nn2` on Dash et al. 2017's 415 human β chains against 5,204 OTS β chains with weights | `reference/tcrdist3_neighborhoods.json` (made by `reference/tcrdist3_neighborhoods.py`), `dash-2017-tcrdist3`, `ots-healthy-pbmc`, `tcrdist3-loops` |
| `TestNeighborhoodsFindTheSimulatedFamilies` | On the simulated vaccination study against a background drawn from the model it was made with, the twenty most enriched neighborhoods of a vaccinated subject's day-14 sample are receptors of the vaccine's epitopes, and every receptor in them is; in a subject given the placebo, none is | `tcrdist3-loops` |
| `TestNeighborhoodsWithinBudget` | Ten thousand receptors' neighborhoods against a background of 100,000 within 2 minutes on 10 cores (not under the race detector) | `ots-healthy-pbmc`, `tcrdist3-loops` |
| `TestVaccineReceptorsAsNearAsDash` | The simulated vaccine study's receptors that recognize an epitope as near each other as Dash et al. 2017's epitope-specific β chains: each one's nearest of the same epitope by TCRdist with the V genes' loops, the shares within 12, 24 and 48 units within 0.1 of the range Dash et al.'s human and mouse chains span (their shares in each sample against the truth: `TestSimulatedSeriesAgainstTheirTruth`) | `dash-2017-tcrdist3`, `tcrdist3-loops` |
| `TestOpenAMillionRearrangementsWithinBudget` | A 1M-row immunoSEQ file read and validated within 5 s, its table within 64 bytes a row plus its junctions (not under the race detector) | generated |

Data sets are required by naming them in `CLONOLOOM_REQUIRE_DATA` (a comma-separated list, or 1
for all); otherwise a missing one is skipped. CI requires the openly licensed ones.

The other Go tests here check QC and clonotypes against pandas and scirpy, and each statistic
against its reference implementation and the simulator's truth (`qc_test.go`,
`clonotypes_test.go`, `diversity_test.go`, `rarefaction_test.go`, `coverage_test.go`,
`usage_test.go`, `overlap_test.go`, `simulated_test.go`). Each test's header says what it compares
and to what tolerance; the CHANGELOG's Validation sections give the results.

`validation/projects/` holds a project saved by each release, with its data and results; every
later version must open it and make its results again ([projects/README.md](projects/README.md),
`go test -run TestEveryReleasesProjectStillOpens .`).

Reference outputs are committed in `validation/reference/` with the script and tool version that
produced them. Public datasets are fetched and checksummed by `validation/fetch.mjs` into
`validation/cache/`, which is never stored in the repository (clonoloom-spec/design.md,
"Development data").

Network layout bytes are pinned in `internal/network/graph_test.go`; run
`go test ./internal/network -run TestGraphTCRdistAndLayout` on arm64 and amd64. The complete
100,000-node sparse synthetic graph and drawing budget is measured with
`go test ./internal/network -run '^$' -bench BenchmarkGraph100000 -benchtime=1x`.

### Specificity database validation

`uv run --with pandas==2.3.3 validation/reference/specificity.py` regenerates only simulated
fixtures. Exact matching tests both with and without V/J, at minimum scores 0 and 2.
The sample-share fixture independently joins all 30 vaccine samples to database and truth:
602 matched molecules, 598 supported by their assigned epitope, 3,498 true response molecules.
Neither incomplete coverage nor the four chance matches is treated as implementation failure.

After downloading VDJdb under its terms, maintainer-only checks keep data and results outside
version control:

```sh
uv run --with pandas==2.3.3 validation/reference/specificity.py \
  --database /tmp/vdjdb.txt --output /tmp/vdjdb-lookup.json
CLONOLOOM_VDJDB=/tmp/vdjdb.txt CLONOLOOM_VDJDB_LOOKUP=/tmp/vdjdb-lookup.json \
  go test ./validation -run TestSpecificityVDJdbMaintainer -v
CLONOLOOM_VDJDB_DOWNLOAD=1 go test . -run TestSpecificityPublishedDownload -v
```

The last check exercises the app's on-request archive download, terms gate and checksum checks
in temporary storage. The database guide capture exercises matching, retained provenance and
Tracking in an isolated library: `node docs/capture/capture.mjs database --theme both`.

For the complete scientific suite with race detection, allow more than Go’s default ten minutes
per package: `go test -race -timeout 20m ./...`. The simulated-truth checks generate their own
repertoires, and the full validation package can exceed ten minutes on a busy machine.

### Artificial B-cell families

```sh
uv run --no-project validation/reference/bcell_families.py
# Or check an existing generated study without regenerating it:
uv run --no-project validation/reference/bcell_families.py --study /tmp/bcell-study --output /tmp/bcell-check.json
go test ./validation -run '^TestSimulatedBCellFamiliesAgainstIndependentCSV$' -v
go test ./internal/simulate -run '^TestBCell' -v
GOARCH=amd64 go test ./internal/simulate -run '^TestBCellStudyTruthAndBytes$' -v
node docs/capture/capture.mjs immunization --theme both
```

The independent stdlib checker replays all ordered events and translations, checks every paired
cell, coordinate, ancestry/isotype assignment, closed-form probability and realized count, and
verifies the output manifest. `bcell-families.json` records CPython's version, sample hashes and
reduced counts. The Go validation compares the reader, structural evidence checks and paired
clonotype counts to those independent values for the default 9-sample, 2,160-cell study. Go tests
also exercise repeated events/back-mutations, model exclusivity, bounds, seeds, parameter round
trips, cancellation cleanup, overwrite refusal and 180,000 draws against their probabilities.
The pinned byte digest must agree on arm64 and amd64. These tests validate an artificial
benchmark, not SHM rates or inference on biological repertoires; model limits and citations are
in `docs/SIMULATION.md` and `clonoloom-spec/research.md` §5.15.
