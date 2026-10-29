# Excel evidence workbooks

Wave 9 slice 10 adds **Reports → Excel evidence workbook**. Select one to eight saved results,
enter a title and choose **Export Excel workbook**. Selection starts empty. The supported kinds
are cohort features, cohort comparisons, replicated count models, batch QC and motif screens,
within the bounds below. This is an unreleased wave 9 feature; it is not in the 0.8.0 binary.

The download is `clonoloom-workbook.zip`. Keep it intact:

- `results.xlsx`: a Read me sheet, then one values sheet and one full provenance sheet per result.
- `R1.result.json`, `R1.manifest.json`: unchanged saved bytes, including the captured job recipe.
- `R1.NOTICE.txt`, `R1.certificate.clz`: provider notices and the existing native evidence certificate.
- `workbook.json`: version 1 inventory, SHA-256 hashes, result/sheet mapping and node/type counts.
  Later selections use `R2`, `R3`, etc.

Restricted evidence requires explicit acknowledgment of the selected sources' terms. That does
not grant redistribution rights. Identifiers, sequences and paths may remain; this is not a
de-identification workflow. Export does not reopen original sources, change a project, run a
statistical test or add an automation permission. Cancel stops preparation; changing the project
or analysis prevents a pending response from being downloaded.

## Read values without losing their meaning

Each values/provenance sheet is a **complete typed JSON-node index**, not a wide biological table.
An Excel row is a JSON node, not a participant, sample or independent observation. Do not treat
these rows as replicates. Use the recorded design, units, denominators, exclusions and correction
families when interpreting the underlying result. There is no inferred join or table flattening.

| Column | Meaning |
| --- | --- |
| JSON pointer | RFC 6901 path; the empty path denotes the document root. Object keys are sorted; array positions retain their recorded order. `/` and `~` in a key are escaped as `~1` and `~0`. |
| JSON node type | Object, array, string, number, boolean or null, from the source JSON. |
| Spreadsheet value | Typed value where supported; precision-sensitive numbers are text. Containers/null have an empty cell. |
| Exact JSON scalar | Original scalar token as text, including numeric spelling and JSON string escapes. Containers have no scalar token. |
| Child count | Number of immediate members/elements of a container; zero is an empty container. |

JSON strings always stay strings: `00123`, `1-2`, date-like identifiers and text beginning with
`=`, `+`, `-` or `@` are not interpreted as numbers, dates or formulas. Booleans use boolean cells.
A null has an explicit `null` node and exact token; an empty string has type `string` and token
`""`; an absent field has no path row. None becomes a numeric zero. All denominators, exclusions,
missingness reasons and nested parameters present in the saved files remain present.

The Read me sheet carries methods, citations, provider notices and each result kind's existing
scientific limits. Batch flags remain descriptive; count models retain their relative-abundance
and model assumptions; motif screens retain their GLIPH2-style adaptation and specificity limits.
There are no new plots, biological algorithms, significance tests or spreadsheet formulas.

## Spreadsheet precision and display limits

Excel numeric cells use binary floating point and have a 15-significant-digit precision limit.
ClonoLoom admits a numeric token only if it has at most 15 significant decimal digits and is zero
or has absolute magnitude from `1e-300` through `1e300`. This magnitude range is deliberately more
conservative than Excel's range. Negative zero, longer decimals, large precise integers and
out-of-range values stay text. For example, `9007199254740993`, `0.123456789012345678901`, `-0`
and `1e-310` remain text. The JSON number `0` stays numeric zero.

Even an admitted decimal such as `0.1` need not have an exact binary representation. A displayed
number may be rounded or use scientific notation. **Use Exact JSON scalar or the original JSON
when exact values matter.** Do not convert that column to numbers, use “precision as displayed,”
or infer extra precision from the value column. The scalar token also preserves `1.0` versus `1`.

Frozen headers, filters, wrapped text and landscape print settings help navigation. Very long
cells can exceed the visible row height even within the storage limit; inspect the formula bar
or original JSON for the whole value. Control characters may display differently across readers.
The original workbook contains no formulas, macros, hyperlinks or external relationships.

**Editing or re-saving in another spreadsheet application is outside the exact-evidence contract.**
For example, the LibreOffice interoperability exercise opened the workbook but a re-save converted
boolean cells to formulas, removed empty-string cells and normalized carriage returns. Readers
also differ in how they decode SpreadsheetML string escapes. Keep the original ZIP and its JSON
companions; do not treat a re-saved XLSX or CSV as a byte-preserving scientific record.

## Bounds and integrity

Export refuses oversized selections rather than sampling rows, omitting fields or truncating
cells. Limits are 1–8 selected results; 100,000 combined JSON nodes across results and full
manifests; 32 MiB per evidence document and per selected result's declared saved evidence;
33 MiB per compressed certificate; 64 MiB of companion evidence; 64 MiB of worksheet XML;
600,000 cells; 100,001 rows per sheet; 32 columns; and 20 sheets. Text cells allow at most
32,767 UTF-16 code units and 253 line feeds. Exact JSON validation also bounds depth to 64 and
values to one million and refuses duplicate keys or invalid Unicode. Preparation has a two-minute
request deadline. The full manifest can be much larger than the displayed analysis; select fewer
results or use native certificates when the workbook budget is exceeded.

The displayed fingerprint hashes the **whole downloaded ZIP**. The inventory hashes the XLSX
and companion files. Checksums identify bytes, not authorship, scientific validity or replication.
The workbook is a static presentation, not a self-checking report or certificate verifier.
Use the [existing native certificate checks/replay](CERTIFICATES.md) for those separate operations,
with their captured-input limits. Original sources and independent upstream validation remain
outside this export. General rearrangement/clonotype sheets and other result kinds are not mapped
by this slice; [Parquet](PARQUET.md) and existing native exports retain their own contracts.

The internal browser endpoint is `POST /api/reports/workbook` with JSON
`{"title":"Study evidence","artifacts":["saved-result-id"],"accept":false}`.
It returns a ZIP and `X-ClonoLoom-Workbook` fingerprint with `Cache-Control: no-store`.
Unknown fields, duplicate results, changed evidence and unsupported kinds/versions are refused.
Restricted sources return HTTP 409 before an explicit acknowledgment. This endpoint does not
add an MCP/client/remote-control action.

## Validation and sources

The independent [openpyxl/XML/Decimal checker](../validation/reference/workbook.py) compares every
node, original token, spreadsheet type, missingness state and companion hash for all five kinds
and the browser download. Native tests cover precise integers/decimals, negative zero, formula-like
strings, Unicode/control escaping, cancellation, provider gates, altered evidence and explicit
bounds. The artificial precision fixture is a format stress test, not realistic participant data.
Both-theme screenshots use the existing 26-sample artificial batch study (24 included, two excluded).

- [Microsoft: Excel specifications and limits](https://support.microsoft.com/en-us/excel/excel-specifications-and-limits?app=wp).
- [Microsoft: floating-point arithmetic in Excel](https://learn.microsoft.com/en-us/troubleshoot/microsoft-365-apps/excel/floating-point-arithmetic-inaccurate-result).
- [Microsoft: SpreadsheetML structure](https://learn.microsoft.com/en-us/office/open-xml/spreadsheet/structure-of-a-spreadsheetml-document).
- [RFC 6901: JSON Pointer](https://www.rfc-editor.org/rfc/rfc6901).

The native Go writer follows the Apache-2.0 CytoWeave presentation pattern (frozen headers, widths,
styles and explicit provenance), with independent exact-JSON handling and no runtime dependency.
