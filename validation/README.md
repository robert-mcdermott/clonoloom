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
