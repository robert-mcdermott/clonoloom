# Validation

`node validation/run.mjs [suite …] [--verbose] [--require-data]` runs the validation suites and
exits with status 1 when a check fails. Each check states what is compared, the value found and
the tolerance required.

| Suite | What it checks | Data |
| --- | --- | --- |
| `accessibility` | WCAG AA contrast of every text-on-surface pair in both themes, with color-vision-friendly colors off and on; palettes, status colors and diverging maps apart in protanopia, deuteranopia and tritanopia (Machado et al. 2009, CIEDE2000) | `web/styles.css` |

Later slices add the checks against independent references:
- the readers against independent readers (the `airr` library, pandas, scirpy) on every
  development file (wave 1, slice 2);
- QC and clonotypes against pandas and scirpy (wave 1, slices 4 and 5);
- each statistic against its reference implementation and simulated truth (wave 2 onward).

Reference outputs are committed in `validation/reference/` with the script and tool version that
produced them. Public datasets are fetched and checksummed by `validation/fetch.mjs` into
`validation/cache/`, which is never stored in the repository (clonoloom-spec/design.md,
"Development data").
