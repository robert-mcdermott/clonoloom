# Self-checking HTML reports

Wave 8 adds a single-file offline report for **explicitly selected saved results**. It contains
complete recorded tables, existing figures, methods/references, provider notices, original result
JSON and native evidence certificates. No source data are reopened and no statistic is recomputed
while preparing or opening it. Version 0.7.0 remains the published release; this feature currently
requires the development source.

## Export and open

1. Open **Reports → Self-checking HTML report**. Enter a title and select one to eight results.
   Selection starts empty. Only cohort features, comparisons, replicated count models, batch QC
   and motif screens have mapped report contracts. Other results are not silently included.
2. Choose **Export self-checking HTML**. Restricted sources require explicit acknowledgment of
   all selected results' provider terms; that does not grant redistribution rights.
3. Keep the displayed **report fingerprint** separately. Open `clonoloom-report.html` in a modern
   browser with JavaScript enabled. No application server, network, source file or library is needed.
4. The checker hashes the embedded content and download bytes before displaying them. Optionally
   enter the independently retained fingerprint and choose **Check embedded bytes** again.
   Failure removes the displayed report and downloads. With JavaScript disabled, the page says
   that no check was performed; it does not show a successful verdict.
5. Expand table sections to inspect every row and field. Download original JSON or a native
   certificate at the end. The report provides a `clonoloom verify --replay --expect …` command
   for each certificate. Native replay is a separate explicit operation.

Cancel stops report preparation. A changed project/analysis prevents a pending response from
being downloaded. Export does not edit the project or create a new scientific result.

## Read the science at its recorded scope

Go renders numeric table cells from **original JSON tokens**, preserving large integers and
spellings such as `1.0`. `null` is unavailable and `(absent)` is a missing field. All result fields,
rows, nested records, missing/exclusion reasons, denominators and testing families remain present.
Nested structures are displayed as complete JSON within the tables. The full original manifest
and captured job recipe remain in the certificate; the visible provenance table omits the job
record to avoid duplicating large captured input matrices.

Figures reuse the application's existing display modules:

| Kind | Figure coverage | Scientific boundary |
| --- | --- | --- |
| Cohort features | Complete tables; no new figure | Saved subject/group features, not recomputation of upstream diversity |
| Cohort comparisons | Subject observations and mean-effect intervals for every recorded feature | Recorded independent/paired design, missingness, interval assumptions and correction families still apply |
| Count models | Complete abundance/effect overview | Relative repertoire abundance; conditional NB2/Wald/LRT assumptions and failed fits remain explicit |
| Batch QC | Every recorded metric, on its existing shared batch scale | Descriptive median/MAD review flags; no diagnosis, batch correction or automatic removal |
| Motif screens | Complete tables; no new figure | The documented GLIPH2-style adaptation; no upstream equivalence, population-calibrated inference or antigen-specificity claim |

Plotting uses browser numbers only for the existing geometry, including readable rounded axis
ticks. Exact values remain in the tables/downloads. Unsafe integer geometry is refused; figures
with unsupported shapes or excessive geometry show an unavailable message. Cohort observation
plots retain their existing 500-point limit without drawing a subset. Report figure limits are
32 cohort comparisons, 64 batch metrics/20,000 batch observations, or 20,000 count-model rows.
An unavailable figure does not turn into an empty scientific finding or an integrity failure:
the saved evidence can match while its visualization is unavailable.

The screenshot study is artificial, with 26 samples, 24 included and two excluded. Unavailable
metric values and the small/constant batch controls remain explicit. The report draws precisely
the recorded plottable observations; inclusion does not imply that every metric is available.

## What a successful check means

**Integrity match** means the exact embedded report content, original result JSON, certificate
archive bytes, bundled geometry source and stylesheet match their SHA-256 inventory. The actual
executing geometry and style are compared with those inventoried bytes too. The checker does
not decode/re-encode scientific JSON to hash it and does not execute recorded recipes.

This is **not** scientific replay, a signature, authorship verification, original-source validation,
independent replication, model calibration or proof of biological validity. The report's checker
and surrounding UI are not an independently trusted verifier. Someone who replaces the checker,
content and hashes can forge an internally consistent page. A fingerprint retained separately
identifies an expected record only when checked with a trusted checker. Native certificate
verification/replay provides a separate tool and preserves its existing captured-input limits.
See [certificates and replay](CERTIFICATES.md), including the NASEM reproducibility distinction.

No external scripts, fonts, images or network resources are loaded. A content security policy
blocks connections and permits only the generated inline script. Text metadata are escaped;
checked report markup is restricted to text, tables and disclosure elements before insertion.
The report still contains sensitive scientific evidence: sequences, identifiers, counts and paths
can occur in its results/recipes. This is not de-identification or permission to share provider data.

## Versioned envelope and bounds

The file embeds base64 bytes in inert script elements, with an inventory of format
`clonoloom-html-report`, version 1, scope `embedded-report-bytes`. Its fields are `format`,
`version`, `scope`, `software`, `results`, `files`, `limits`. The report fingerprint is SHA-256
of the exact compact UTF-8 Go encoding of that record. It is a ClonoLoom encoding, not JCS.

The sorted file inventory has `path`, `size`, `sha256`. It contains `report.html`, `geometry.js`,
`report.css`, plus `<artifact>.result.json` and `<artifact>.certificate.clz` for each selected
result. Each result entry records artifact/kind, both filenames and its independent certificate
fingerprint. Certificate v1 and scientific replay contracts are unchanged. Report bytes are
repeatable for the same selected immutable evidence, order, title and application build/assets.

Bounds are one to eight distinct results, a 1–200-byte title, 16 KiB request, 32 MiB original
manifest plus output files per result, 33 MiB per compressed certificate, 32 MiB rendered content,
350,000 table cells and 64 MiB total inventoried bytes. Base64 makes the HTML larger than the
inventoried bytes. The checker limits its inventory record to 1 MiB. Limits refuse the whole
export; results/rows are never silently truncated. Large motif screens or combined studies can
exceed these limits: retain their ordinary native certificates/exports instead.

The protected local host accepts `POST /api/reports/html` with `{title, artifacts, accept?}` and
returns an HTML attachment with `X-ClonoLoom-Report`. Malformed/duplicate selections return 400;
unsupported, changed, oversized or restricted evidence returns 409. `accept: true` explicitly
acknowledges the provider terms for private study. No arbitrary output paths are accepted.

The shared action is `export_html_report {artifacts, title?, accept?}`. It uses the visible
Reports flow and returns the fingerprint, selected IDs and `recomputed: false`. MCP and generated
client work remain separate slices.
