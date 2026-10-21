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
| `TestReadersAgreeWithIndependentReaders` | Every column ClonoLoom reads from every development file equals what an independent reader reads: the AIRR Community's `airr` library for AIRR files, pandas for Cell Ranger and immunoSEQ files; immunoSEQ gene names mapped as the dataset records them | `reference/readers.json` (made by `reference/read_repertoires.py`), the files of `sources.json` |
| `TestOpenAMillionRearrangementsWithinBudget` | A 1M-row immunoSEQ file read and validated within 5 s, its table within 64 bytes a row plus its junctions (not under the race detector) | generated |

Data sets are required by naming them in `CLONOLOOM_REQUIRE_DATA` (a comma-separated list, or 1
for all); otherwise a missing one is skipped. CI requires the openly licensed ones.

Later slices add:
- QC and clonotypes against pandas and scirpy (wave 1, slices 4 and 5);
- each statistic against its reference implementation and simulated truth (wave 2 onward).

Reference outputs are committed in `validation/reference/` with the script and tool version that
produced them. Public datasets are fetched and checksummed by `validation/fetch.mjs` into
`validation/cache/`, which is never stored in the repository (clonoloom-spec/design.md,
"Development data").
