# CytoWeave phenotype links

Wave 9 slice 13 adds **Data → CytoWeave phenotype links**. This is unreleased source
functionality, not part of the published 0.8.0 binary. It joins an index-sort wells CSV
from CytoWeave to supplied native AIRR TSV receptor cell IDs in **one explicitly selected dataset/sample**.
It preserves phenotype observations; it does not infer a phenotype or biological pairing.

## Prepare and review

1. In CytoWeave, inspect one acquisition's index-sort plate and choose **Export wells (CSV)**.
   Keep the original FCS, CytoWeave workspace, compensation matrix, gating history and sort
   records. The supported exporter was checked in Apache-2.0 CytoWeave 0.8.1 (`19d47bb`).
2. Make a separate CSV from your experimental identity records, with the exact header
   `well,cell_id`. Give each mapped well one exact ClonoLoom cell barcode, including case,
   leading zeros and any suffix. For example:

   ```csv
   well,cell_id
   A1,BC00001-1
   A2,BC00002-1
   ```

   Do not derive this mapping from CSV row order, similar filenames, gate membership or
   matching numbers of cells. A well label is local to its acquisition. A barcode may recur
   in another sample. Retain the records that establish this mapping experimentally.
3. Prepare a UTF-8 JSON declaration with these **string** fields. Supply `unknown` when context
   is unavailable; do not invent compensation or acquisition details. `evidenceText` can hold
   original workspace/metadata JSON as a string, preserving its numeric tokens and whitespace.

   ```json
   {
     "exporterVersion": "CytoWeave 0.8.1",
     "sample": "Plate A",
     "acquisition": "Sort run identifier and acquisition date",
     "sourceSHA256": "unknown",
     "compensation": "unknown",
     "valueScale": "linear",
     "gating": "unknown",
     "evidenceText": "unknown"
   }
   ```

   `sourceSHA256` is the original FCS SHA-256 (64 lowercase hex characters), or `unknown`.
   `valueScale` is `linear` or `unknown`. Other fields accept nonempty text. This declaration
   records what you supplied; ClonoLoom does not verify it against a supplied FCS or workspace.
   It does independently hash the exact uploaded CSV, mapping and declaration bytes.
4. Open a native AIRR TSV receptor dataset in ClonoLoom. Choose **Data → CytoWeave phenotype links**, explicitly
   select the receptor sample, and select all three files. Choose **Review exact links**.
   No sample is selected automatically. Review the counts, channel values, original provenance
   and receptor-only IDs. Switching the sample or inputs invalidates the earlier review.
5. Choose **Save exact link evidence** before closing. Restricted provider data require explicit
   acknowledgment of applicable permissions. This does not grant redistribution rights.
   The JSON contains the original input text and hashes, explicit scope, source fingerprints,
   all observations, exact channel strings, matched receptor row indices and scientific limits.

The evidence record is separate from the project: it does not attach expression annotations,
change QC or clonotypes, alter paired-chain calls, enter a report certificate or become a
scientific replay recipe. Keep it with the input files. To reproduce the join, reopen the same
fingerprinted receptor dataset, select the same sample and upload the three original inputs.
The join is deterministic; the complete export also includes the loaded dataset's provenance
clock and application build. Files are uploaded only to the local ClonoLoom server. No CytoWeave
server, public service, MCP permission, Python/R action or remote-control enablement is involved.

## Supported format and exactness

Selected receptor sources must be unchanged native AIRR TSV (optionally gzip), with exact
`cell_id` and, when present, `repertoire_id` columns. Source hashes are rechecked for each review
and export. If the general reader trimmed an identifier, this join refuses it instead of silently
linking normalized identities. Source files must be regular files, each within 64 MiB; combined
decoded selected sources must be within 64 MiB, with source lines within 1 MiB. Mapped/vendor
formats, including direct Cell Ranger input, are outside this first source-identity contract.
Export native AIRR upstream with documented identities, then reopen and review. General readers
and historical caches are unchanged.

- Wells header: `well,row,column,event,population`, then zero or more channel display labels.
  Headers must be unique and nonempty; ambiguous repeated labels require upstream correction.
- Wells use uppercase letters and a positive column without leading zeros (`A1`, `AA12`). Row,
  column and event are canonical positive decimal integer tokens. Row/column must agree with
  the well and be within 1024 × 1024. Event tokens are preserved as strings, never floating point.
  CytoWeave exports **one-based** event numbers, in well order; that order is not a receptor key.
- Repeated wells or event IDs refuse the entire join, including repeated wells that have no
  mapping. Multiple events in a well are unresolved identities. Duplicate mapping wells or
  cell targets, empty targets and mapping wells absent from the acquisition are refused.
- Identifiers are never trimmed, case-folded, suffix-stripped or matched across samples. Distinct
  chain/rearrangement rows with the same cell ID count once as a receptor cell identifier.
  Loaded-table receptor row indices in JSON are **zero-based**, unlike exported event numbers.
- All CSV fields remain strings. Negative values, zeros, blanks, `NaN`, infinities, unknown tokens,
  decimal precision and integer precision are preserved. They are not converted to zero or
  coerced to measurements. Quoted commas, quotes and embedded CR/LF are retained; original UTF-8
  bytes, BOM and line endings are preserved in each evidence text/hash. No spreadsheet formulas
  are executed. JSON avoids spreadsheet auto-conversion.
- Each CSV is limited to 4 MiB, 10,000 data rows and 128 columns. The declaration is limited to
  64 KiB; the JSON HTTP request is limited to 12 MiB including escaping. Malformed quoting,
  blank records, ragged rows, invalid Unicode, duplicate JSON keys and unknown declaration/request
  fields are refused. Large files are rejected, never silently truncated.
- Empty mappings (header only) are refused: this workflow requires at least one explicit link
  attempt. A valid mapping can have no matches; that remains visible as absent receptor targets.

## What the numbers mean

**Linked** is the number of exported wells whose explicitly mapped cell ID occurs in the selected
receptor sample. **Unmapped** wells have no mapping record. **Absent receptor targets** have a
mapping but no matching cell ID in that sample. **Receptor-only** IDs occur in the selected
sample without a linked event. Receptor rows lacking cell IDs are counted separately. Coverage
uses loaded evidence before QC; it is not an estimate of recovery rate or phenotype frequency
in the original specimen.

CytoWeave's exporter uses `SampleView.column`: linear values under the active compensation for
covered channels, otherwise raw/derived channel values. It does not apply the plate display's
logicle/arcsinh transform. Negative compensated values can be meaningful; do not truncate them.
The CSV alone does **not** tell you which matrix was active, whether compensation failed upstream,
or whether channel labels represent comparable measurements. Unknown history stays unknown.

`population` is one deepest gate path chosen by the exporter. It is not the complete gate
membership matrix; equally deep overlapping gates are not resolved into exclusive cell types.
An empty population is missing/unassigned exported annotation, not a negative biological call.
The ClonoLoom review uses a table of exact observations, without inferred phenotype colors,
thresholds, distances, pooled subjects, p-values or an invented embedding.

A unique well/barcode link does not prove a biological singlet, exclude doublets, certify
physical receptor pairing or establish antigen specificity. Repeated source chain rows are not
independent experimental units. Subject-level analysis still needs an explicit experimental
design. This slice does not read arbitrary FCS files, auto-gate, recompute compensation, ingest
multi-sample event exports or AnnData as this contract, or modify the Cells view.

## Sources and validation

- [AIRR Rearrangement 2.0](https://docs.airr-community.org/en/v2.0.0/datarep/rearrangements.html)
  defines `cell_id` as a cell-of-origin identifier and provides repertoire/processing references.
  [AIRR Cell](https://docs.airr-community.org/en/latest/datarep/cell.html) specifies case-sensitive
  identifiers and study-level Cell identity. ClonoLoom's operational sample-scoped source barcode
  key does not claim that a repeated barcode is a globally unique AIRR Cell object.
- [Spidlen et al., FCS 3.1](https://doi.org/10.1002/cyto.a.20825) distinguishes event data and
  metadata, including spillover. [MIFlowCyt](https://isac-net.org/resource/resmgr/docs/miflowcyt1-0.pdf)
  supports retention of acquisition, compensation and analysis context. These are reporting
  requirements, not certification that a supplied link or phenotype is scientifically correct.
- Apache-2.0 [CytoWeave](https://github.com/robert-mcdermott/cytoweave), `web/ui/plate-view.js`,
  `web/lib/indexsort.js` and `web/lib/engine.js` establish the checked interchange behavior.
  ClonoLoom implements its own bounded parser/join; no cytometry algorithm is copied here.

`testdata/cytoweave` is explicitly artificial: 96 events, 80 linked cells, 10 absent targets,
6 unmapped wells, 4 receptor-only cells, one missing-barcode row and deliberately reused barcodes
in another sample. Intensities resemble linear cytometry values but are not biological findings.
The actual sibling exercise generates its artificial FCS, runs its FCS reader and compensation,
clicks its real plate CSV export, checks event/well truth and channel strings, and links 95 wells
through the native ClonoLoom API. It does not claim validation on a clinical or experimental study.

```sh
GOCACHE=/tmp/clonoloom-go-cache go build -o /tmp/clonoloom-cytoweave .
node validation/cytoweave-links.mjs /tmp/clonoloom-cytoweave ~/mycode/cytoweave /tmp/cytoweave-evidence
CLONOLOOM_CYTOWEAVE_OUT=/tmp/cytoweave-evidence go test -run TestCytoWeave .
uv run --no-project validation/reference/cytoweave.py /tmp/cytoweave-evidence
GOCACHE=/tmp/clonoloom-go-cache CLONOLOOM_CYTOWEAVE_OUT=/tmp/cytoweave-evidence node docs/capture/capture.mjs cytoweave --theme both
```

The independent Python oracle recomputes the join from original CSV/TSV, checks every source
value/hash and verifies sample boundaries, missingness and chain-row identities. Browser controls
cover initial empty selection, exact downloads, scope changes, ambiguity refusals, concurrent
operations, cancellation and stale projects. Full scientific regression remains separate from
this interchange check.
