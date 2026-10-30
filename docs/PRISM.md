# GraphPad Prism subject-table exports

ClonoLoom 0.9.0 adds **Reports → GraphPad Prism subject tables**. Choose one saved **cohort feature** or **cohort comparison** result,
enter a title and choose **Export Prism tables**. Selection starts empty. Restricted evidence
requires explicit acknowledgment of its provider terms; that does not grant redistribution rights.

Keep the entire `clonoloom-prism.zip`:

| File | Purpose |
| --- | --- |
| `data.pzfx` | Data-only Prism Column tables and an Info sheet with methods, citations and limits |
| `mapping.json` | Declared design, units, group order, subject rows, exact cell tokens, source JSON paths, missingness and exclusions |
| `result.json`, `manifest.json` | Unchanged saved native result and complete provenance/recipe |
| `certificate.clz`, `NOTICE.txt` | Existing native evidence certificate and provider/source notice |
| `README.txt`, `prism.json` | Import guidance and version 1 inventory with SHA-256 hashes and exporter identity |

The filename uses the published legacy PZFX interchange, with XML version 5.00 and a declared
minimum Prism version of 7. The export contains no analysis, graph, script or template. It does
not create the newer `.prism` format, reconstruct Prism's proprietary graph/analysis records,
or imply statistical equivalence between applications. No original source files are reopened.

## Experimental units and pairing

One table represents one recorded feature, with its name and measurement unit in the title.
Feature results export their complete 13-feature catalog; comparison results export only the
features in that saved comparison. The two columns keep the declared **reference, comparison**
order. Each has one subcolumn. Rows are sorted by exact subject identifier, also used as row titles.

- **Paired:** the same subject's two measurements occupy the same row. Sample order cannot
  change pairing. Both selected samples must exist structurally; a missing measurement is blank.
- **Independent:** each subject has a distinct row and a value only in its own group column.
  The opposite blank is structural: that subject was not observed in that group. It is not a
  missing measurement, and it does not make that row a matched pair.
- **Excluded/outside contrast:** those samples remain in `mapping.json` with their original
  disposition/reason, and in the native evidence. They do not become observations in Prism.

The experimental unit remains the declared subject. An observation can measure abundance in
cells, reads, UMIs, templates or sequences without making those items independent experimental
units. The exporter does not average technical replicates, pool visits, expand clone/cell rows,
convert bootstrap draws into subjects, or convert model coefficients into raw observations.
Only the native checked two-group independent/paired subject designs are mapped. General
longitudinal, nested, multi-factor and other result layouts are outside this contract.

**Pairing is preserved by row identity, not by a preconfigured Prism test.** Inspect the Info sheet
and choose the appropriate paired or independent analysis yourself. Keep row titles and both
columns together when sorting. Do not independently sort columns or compact away blank values.
The exported structure cannot establish that subjects are biologically independent or that a
study has adequate power, an appropriate model or a causal interpretation.

## Missing values and statistical differences

The mapping has three explicit cell states:

| State | PZFX cell | Mapping evidence |
| --- | --- | --- |
| `observed` | Original numeric token | Sample identity and exact source JSON pointer/value |
| `missing` | Empty `<d/>` | Sample identity, source null and its recorded reason |
| `structural-empty` | Empty `<d/>` | No observation exists in this independent subject/group position |

Zero is written as numeric zero. Missing measurements are never zero-filled, imputed or silently
dropped. Conflicting values/reasons, duplicate subject/group observations, changed design identities,
missing points and inconsistent denominators are refused. A comparison's points must agree with
its captured feature measurements; all original statistical results remain in the JSON companion.

GraphPad documents that paired tests can omit a subject whose pair is incomplete. ClonoLoom's
cohort comparison instead leaves that feature untested when any selected value is missing.
**Export does not make a complete-case analysis equivalent to the saved ClonoLoom analysis.**
Review missingness, independence/matching, effect definitions, interval assumptions, rank/permutation
methods, tie handling and multiple-testing families before performing new analysis in Prism.
No test, p value, confidence interval, normalization or new graph is computed by this exporter.

## Precision, bounds and evidence

Numeric tokens are written directly from the original JSON, without browser-number conversion
or decimal rounding. Prism's numeric storage/display and later saving can still change their
representation. Its current rounding guide describes double-precision storage; older versions
before 7 use lower precision. **An imported or re-saved Prism document is not the exact native
record.** Preserve `result.json`, the mapping and certificate when exact evidence matters.

ClonoLoom conservatively refuses nonfinite/non-numeric values, negative zero, nonzero magnitudes
outside `1e-300`–`1e300`, integers outside ±9,007,199,254,740,991 and exponents outside ±400.
It does not silently turn an unsupported number into a blank or round it. Decimal fractions
within those bounds may still round in Prism. Labels stay XML text, including numeric-looking
or formula-like subject IDs; XML markup is escaped and unsupported XML 1.0 control characters
are refused. No spreadsheet or Prism expression is generated.

Bounds: one result; 1–13 tables; two columns and one subcolumn; 100,000 combined numeric/blank
data positions; 10,000 design samples; 32 MiB source/saved evidence and PZFX; 33 MiB compressed
certificate; 64 MiB total uncompressed bundle; 256 bytes per numeric token; 64 KiB per XML text
item/Info line. Native exact-JSON validation also bounds depth/values and rejects duplicate keys
or malformed Unicode. Prism construction has a two-minute request deadline. Oversized data
are refused, not sampled or truncated. Use native exports for evidence outside this mapping.

Export is cancellable and cannot download a pending result after the project/analysis changes.
It does not modify the project, create a scientific result, change automation permissions or add
an MCP/client action. The browser endpoint is `POST /api/reports/prism` with
`{"artifact":"saved-result-id","title":"Study title","accept":false}`; success returns a ZIP,
`X-ClonoLoom-Prism` fingerprint and `Cache-Control: no-store`. Unknown request fields are refused.

The inventory identifies bytes, not authorship or biological validity. Keep the displayed whole-ZIP
fingerprint separately if you need to identify an expected export. Use [native certificate
verification/replay](CERTIFICATES.md) as a separate operation with its existing captured-input limits.
Identifiers and provider restrictions remain; this is not de-identification or permission to share.

## Validation and native import status

The independent [JSON/Decimal/XML checker](../validation/reference/prism.py) verifies table/column
order, exact scalar tokens, subject identity, paired alignment, structural blanks, missing reasons,
exclusions and every inventory hash. It also imports the files with R's **pzfx 0.3.1**, independently
checking row titles, numeric values and NA positions. CI runs these checks on native independent,
paired, excluded-subject and wholly missing-feature controls. The browser download uses a declared
artificial study with 28 paired subjects and 56 samples; the UI is reviewed in both themes.

GraphPad's published format schemas were consulted. The unmodified Prism 7/8 Mac XSDs fail strict
lxml compilation because their `d` content model is nondeterministic; **no successful strict XSD
validation is claimed**. The checker retains an optional `--schema` argument for a usable local
schema, and fails explicitly if schema compilation/validation fails. No vendor schema or stylesheet
is redistributed, and no incompatible application implementation source was examined.

**Native GraphPad Prism was not installed on the validation host, so a native GUI import remains
unverified.** R-reader import is useful interoperability evidence, not a substitute for that check.
On a host with Prism 7+, open `data.pzfx`, inspect the Info sheet, confirm two Column datasets with
one subcolumn, and compare all row titles/values/blanks against `mapping.json` before any analysis.
In paired controls, verify a missing value leaves its partner on the same row. In independent
controls, confirm the opposite-group blanks and choose an unpaired analysis. Check exclusions and
precision separately. Do not claim equivalent statistical results from merely opening the file.

## Sources

- [GraphPad: Column tables](https://www.graphpad.com/guides/prism/latest/user-guide/column_tables.htm).
- [GraphPad: missing values](https://www.graphpad.com/guides/prism/latest/user-guide/missing_values.htm).
- [GraphPad: PZFX integration and data-only XML](https://www.graphpad.com/support/faq/prism-integrated-saves-prism-files-instruments/).
- [GraphPad: published XML schemas](https://www.graphpad.com/support/faq/prism-xml-style-sheet-and-schema-that-define-the-pzfx-format/).
- [GraphPad: rounding](https://www.graphpad.com/guides/prism/latest/user-guide/rounding.htm) and [numeric precision/range](https://www.graphpad.com/support/faq/how-many-digits-of-precision-does-prism-store-how-accurate-are-the-results-what-are-the-largest-and-smallest-numbers-prism-can-store/).
- [GraphPad: repeated measures and pseudoreplication](https://www.graphpad.com/guides/prism/latest/statistics/repeated_measures.htm).
- Lazic SE (2010), *The problem of pseudoreplication in neuroscientific studies*, [doi:10.1186/1471-2202-11-5](https://doi.org/10.1186/1471-2202-11-5), already cited by the native cohort design.

The native writer follows CytoWeave's Apache-2.0 PZFX table/Info pattern, with ClonoLoom-specific
subject mapping, exact tokens, bounds and original evidence companions. It has no new runtime dependency.
