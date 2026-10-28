# Explicit de-identification exports

Wave 8 development, slice 11. Use a development host and matching clients: action API **2**,
catalog **4** (64 actions), permission policy **1**. Published v0.7.0 predates this workflow.

## Prepare, review and download

1. Review the dataset's effective subject annotations, clonotype definition and QC first.
   Imported metadata supplies unset annotation fields; review those effective values before export.
2. Open **Reports → De-identification export**, choose a dataset and either AIRR rearrangements
   plus clonotypes or clonotypes only. Choose whether to remove free-text annotations and HLA.
   Choose text tables (the default) or Typed Parquet. Both removal options start selected.
   Acknowledge provider restrictions when applicable.
3. Select **Prepare and review pseudonyms**. Inspect the complete subject correspondence,
   retained population, units, removal choices, inventory and fingerprint. Original identifiers
   in this local review are private. No file has been downloaded yet.
4. Select **Download reviewed export** for the table ZIP. If needed, use **Download PRIVATE
   identity map separately** and store that JSON under your study's access controls. Never
   distribute it with the ZIP. A browser or remote caller can retain private review responses;
   the application cannot retract those copies.

Changing window choices clears the reviewed receipt: prepare again. Preparations last
15 minutes and only one is retained in host memory. A new preparation invalidates the previous
one and creates fresh pseudonyms. Project changes invalidate the browser's receipt. Host restart
loses preparations. No map is written to the library; downloading is an explicit file action.
Do not rely on memory clearing as secure erasure or on pseudonyms as encryption.

## What changes, and what stays exact

The version 1 ZIP's default text representation contains only these named files:

| File | Contract |
| --- | --- |
| `clonotypes.csv` | Ordinary native clonotype CSV with pseudonymized sample-column headers; all scientific cells, stable clonotype IDs, counts and missing values unchanged. |
| `rearrangements.tsv` | Optional ordinary native AIRR export with only `sequence_id`, `cell_id` and `repertoire_id` replaced. Every other field and row order matches the ordinary export under the same QC and definition. |
| `samples.csv` | One row per sample, with `sample_id`, `subject_id` and explicitly retained annotation columns. |
| `transformation.json` | Format/version, fresh export identity, software, effective definition ID/rules, QC with aliased sample exclusions, clonotype abundance unit, population counts, removals, binding counts, provider credits, scientific limits and SHA-256/size inventory. |
| `NOTICE.txt` | Fresh scientific provenance and provider attribution/restrictions; no original source paths or fingerprints. |

With `storageFormat:"parquet"`, the same policy runs before conversion: `.parquet` files replace
CSV/TSV and `schema.json` records typed fields and aliased sample mappings. Public footer
metadata contains only the sanitized transformation record (without its file list) and notice.
The private map remains separate. Missing fields become null and explicit zero stays zero;
fractional, non-finite or native counts above 2^53−1 refuse instead of being rounded.
See [Parquet schema, bounds and scientific limits](PARQUET.md). The optional format field is
omitted for the default text representation, preserving the existing version 1 contract.

The record format is `clonoloom-deidentified-tables`, version 1, scope
`native-table-copy-with-pseudonymized-identifiers`. Its exact JSON bytes have the reviewed SHA-256
fingerprint. Its sorted inventory covers the other files; it does not hash itself. This is an
export inventory, not a certificate of scientific replay, authenticity or de-identification risk.

Subjects receive one pseudonym across their samples. Sample names receive distinct pseudonyms;
cell and sequence identifiers are scoped by original sample, preserving within-sample equality
without merging repeated barcodes from different samples. Missing subject, sequence and cell IDs
remain empty in text or null in Parquet. Repertoire
IDs become the sample pseudonym, so even ambiguous vendor repertoire labels cannot merge samples.
The separate map records their original values. Pseudonyms use HMAC-SHA-256 with a fresh random
256-bit key per preparation, structured kind/scope/value inputs and 96-bit suffixes; a detected
collision refuses preparation. The key is never exported. ClonoLoom does not infer missing
subjects, merge subjects or create independent biological replicates.

**Free-text removal** omits `timepoint`, `tissue`, `reference`, `cellSubset`, `condition` and `batch`
from the sample table. **HLA removal** omits `hla`. Subjects are pseudonymized regardless of those
choices. Retained annotations remain exact strings; a known original subject/sample identifier
inside one refuses export instead of being silently rewritten. When HLA removal is selected,
retained text containing “HLA” also refuses export. These conservative checks can reject an
innocent substring, and are not a general detector of names, identifiers or allele notation.

The private `clonoloom-private-identity-map` v1 JSON binds the public fingerprint to original
sample/subject/cell/sequence/repertoire identities, source fingerprints and effective annotations.
It is outside the public inventory and never a ZIP member. Missing subjects cannot be recovered
from a map when they were never recorded.

Exactness means parity with the **native scientific export**, not raw input-byte preservation:
reader normalization, count representation, QC and clonotype rules still apply. Reimporting
transformed tables is not a promise of identical downstream replay; use unchanged originals for
that purpose. Rows removed by
QC stay removed; keep original files and saved artifacts for source-level evidence and replay.
Removing experimental annotations can prevent reconstructing pairing, groups or time courses;
the private map and original project remain necessary for that context. Stable clonotype IDs,
sequences, gene calls, rare counts, sample structure and retained annotations remain linkable.
This is pseudonymization, not anonymity, a privacy-risk estimate or legal certification.

## Deliberate exclusions and bounds

This workflow does not sanitize existing archives, reports, result artifacts, certificates,
checkpoints, recipes, histories, images or binary attachments. Those formats can contain original
identifiers and stay unchanged. They never enter this ZIP. Original dataset/project labels,
custom definition labels, source paths, source hashes, original filenames and raw metadata are
excluded. Required provider credits are retained; a known identifier collision in those credits
refuses export rather than removing attribution. Provider restrictions never change, and a
private-use acknowledgment does not authorize redistribution.

Nonempty native `imgt_annotations` or `germline_database` evidence refuses the AIRR option: this
version has no mapping for identifiers in that nested/free-form scientific provenance. Choose
clonotypes only explicitly or keep the original evidence privately. No annotation field is
silently dropped from an otherwise successful AIRR export.

Limits: one preparation at a time, 1,000 samples, 4 MiB settings body, 8,192 bytes per identifier
or annotation, 250,000 identity bindings, 16 MiB serialized binding budget, 32 MiB private map,
64 MiB combined table/notice bytes, 68 MiB compressed output and 8 MiB per native TSV line.
Preparation uses a five-minute context and checks cancellation around table construction and
while transforming/writing; native clonotype construction itself is not interruptible midway.
Oversized/unsupported inputs refuse without partial downloads. This is a bounded dataset export,
not a whole-library operation.

## Shared actions and native routes

`prepare_deidentified_export(dataset?, what="tables", dropText=true, dropHLA=true, storageFormat="text", accept=false)`
requires explicit host `read`, `compute`, `files` grants and human proposal review. It returns
the receipt and private local subject correspondence. `what="clonotypes"` omits AIRR rows.
`download_deidentified_export(token, expected, identityMap=false)` requires `read`, `files` and
separate review. Its default downloads only the ZIP; `identityMap=true` downloads only the private
map. The window, HTTP/MCP and generated Python/R clients use the same contract. Client parameter
names are generated in snake_case (`drop_text`, `drop_hla`, `storage_format`, `identity_map`). Clients do not grant
permissions or approve proposals, and downloads occur on the host's browser computer.

Native window routes are `POST /api/deidentification/prepare` and
`GET /api/deidentification/<token>?expected=<fingerprint>&part=bundle|identity-map`.
The native POST requires both boolean removal choices and exactly one allowlisted annotation
record for each open sample. Missing/duplicate/unknown fields and invalid QC refuse. Both
responses are `no-store`; the GET requires the reviewed fingerprint and explicit part. Native
routes share the existing loopback/window protections; they are not a separate remote API.

## Sources and validation

The privacy boundary follows [NIST SP 800-188 (2023)](https://doi.org/10.6028/NIST.SP.800-188):
identifier substitution alone does not establish that data cannot be linked or re-identified.
The scientific table and subject/repertoire distinction follow the
[AIRR Rearrangement standard](https://doi.org/10.3389/fimmu.2018.02206) and
[AIRR metadata schema](https://docs.airr-community.org/en/latest/datarep/metadata.html).
See research §5.52 for implementation choices and limits.

Native tests compare every retained scientific field against ordinary exports, test subject/cell
linkage and missingness, check disclosure canaries in every ZIP member, check unchanged source
bytes, test explicit retention/removal, unsupported evidence, provider terms, separate pinned
downloads, cancellation and bounds. `node validation/deidentification.mjs /path/to/clonoloom
--screenshots` uses six longitudinal observations of two simulated vaccination subjects in an
isolated library, verifies identical native queries and unchanged project bytes, tests rejected
and separate downloads, stale edits and aborts, and audits both themes. It adds no new estimator
or biological finding.
