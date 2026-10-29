# AIRR Data Commons metadata queries

Development: wave 9, slice 7. In **Data → Public repositories**, choose an approved repository,
enter metadata filters, review the exact destination and JSON request, and choose **Send metadata
query**. Opening the dialog and editing filters contact only the local ClonoLoom server.
**Request service info** sends a separate GET to the displayed `/airr/v1/info` endpoint.

No dataset, sequence, receptor, project or file is read to construct these requests. Only the
values entered in the form are sent. Do not enter private identifiers into a public query.
Query results stay in the dialog's memory until a separate reviewed download captures fresh
source evidence (below). Public-cohort comparison belongs to slice 9.

## Pinned contract and destinations

The supported protocol is **ADC API 1.2.0, `/airr/v1`**, from the
[AIRR Standards 1.5.1 specification](https://github.com/airr-community/airr-standards/blob/2ccc341fc420705b1f1447bd732f9084f160fc0e/specs/adc-api.yaml)
(CC BY 4.0). Its SHA-256 is
`22ed10775672bafd5a777174bddc51b09a7b51deea4a0dc4ac7424d3ef770b58`.
This deliberately follows requirement D9's v1 endpoint. The AIRR Standards 2.0 specification
uses `/airr/v2`; reading AIRR 2.0 local files does not imply ADC v2 protocol support.

The 16 destinations come from the [approved ADC registry at revision
4ae159f](https://github.com/airr-community/adc-registry/blob/4ae159f2799594a2c73669527f6fba97fc8995d7/ADC-registry.tsv),
dated 2026-08-24, retrieved 2026-10-29. The original registry is embedded as
`internal/adc/registry.tsv`, SHA-256
`87e32b191050e6a37be24342afc65ea86a80be80b972b084a1d16cdf0b51f4e0`.
ClonoLoom does not refresh it at runtime. Updating destinations requires a reviewed source change.
Registry membership is not a promise of current availability, data quality or unrestricted reuse.

Requests use HTTPS to the exact registered origin with `/airr/v1/info` or `/airr/v1/repertoire`.
No user-supplied URL, authentication, cookies, redirect following, retry, cross-repository search
or automatic paging is supported. A relocated endpoint fails visibly until its destination can
be reviewed. Server errors and rate limits are reported without interpreting a provider's error
body as results. Service information is preserved as provider declarations: the server's own
`version`, `api.version` and `schema.version` are different concepts. No compatibility certification
or automatic fallback to another protocol follows an info request.

## Query subset and bounds

Each nonblank field becomes a string equality predicate, joined with `and`. Blank fields impose
no restriction; they do not search for missing values. Values are not trimmed or case-folded.

| Form field | ADC field |
| --- | --- |
| Species ontology ID | `subject.species.id` |
| Locus | `sample.pcr_target.pcr_target_locus` |
| Study ID | `study.study_id` |
| Repertoire ID | `repertoire_id` |

For example, species `NCBITAXON:9606` and locus `TRB` request human TCR-beta repertoire metadata.
They do not establish productive receptors, paired chains, an abundance unit or assay comparability.
See the [official v1 query examples](https://docs.airr-community.org/en/v1.5.1/api/adc_api.html).
Arbitrary filter expressions, range searches, facets, field projections and rearrangement queries
are outside this slice. The native worker creates the actual request; the browser displays it.

| Limit | Behavior |
| --- | --- |
| Page size | 1–100 repertoire records; default 20. Zero/unlimited is refused. |
| Offset | 0–10,000 records. No request is sent beyond this browsing budget. |
| Input | 8 KiB local request; each filter at most 1,024 UTF-8 bytes, no NUL/line breaks. |
| Response | At most 8 MiB after HTTP decompression, including service info. |
| Time | 45 seconds per request, including reading the body. |
| JSON | Exact parser checks duplicate keys, valid Unicode, depth ≤64 and ≤1 million values. |

**Cancel request**, closing the dialog, or changing an input aborts the local request and suppresses
late results. Cancellation cannot retract a request the repository has already received.
`--offline` refuses both outbound operations before contacting a repository; registry browsing
and local request preparation still work. A failed request clears the previous response so it
cannot appear to answer the new query.

**Prepare next page** advances the offset by the number actually returned and makes a new local
preview. It sends nothing until **Send metadata query** is pressed again. A provider may cap
responses below the requested size. An empty page stops that next-page control; a short or empty
page does **not** establish a complete cohort. Offsets have no stable-order/snapshot guarantee,
so concurrent repository changes can duplicate or skip records. Pages are never pooled here.

## Evidence and scientific limits

The worker retains the exact response body, byte count, SHA-256 and UTC completion time, together
with the endpoint, exact request and request hash. These are the JSON entity bytes exposed by
the HTTP client after transfer/content decoding, not TLS packets or compressed wire bytes.
The response hash proves byte identity only; it is not an expected publisher checksum and does
not authenticate a study. Each record's original JSON, unknown fields, numeric tokens and
absent/null/false/zero states are available through **Inspect**. Only bounded offsets/counts and
string labels are decoded for browser presentation; scientific numbers remain in source text.

Responses must be JSON objects with a case-sensitive `Repertoire` array. Records need unique,
nonempty string `repertoire_id` values within that page; supplied study/title/subject labels must
be strings or null. Over-size pages, malformed JSON and duplicate keys/IDs are refused as a whole.
These are structural checks, not full schema validation or independent verification that the
provider applied the filters. Missing labels display as “Not supplied”; their exact source state
is inspectable. Remote descriptions remain text, never executable markup or followed links.

Repertoire records are not counts of independent subjects, cells, reads, clones or experiments.
A subject may have several repertoires, visits or processing runs. Identifiers are scoped to their
repository and study; matching labels across studies do not establish a shared person. Metadata
presence does not prove MiAIRR completeness, ontology correctness, scientific validity, antigen
specificity, or permission to reuse data. No analysis or plot is calculated from this browser.
Reissuing a query is a new remote observation, not exact scientific replay.

This is a local-window workflow. Existing MCP, Python/R and remote action catalogs remain
unchanged; no new automation capability or self-approval path is added. HTTP UI requests retain
the application's origin/loopback checks and require explicit confirmation for each outbound call.

## Validation

The artificial fixture models two declared subjects with three repertoire records, including
two time points for one subject, unequal sequencing support and missing metadata. It is not a
public study, power calculation, full MiAIRR example or scientific result. Decimal/large-integer
extensions exercise exact transport, not implausible cell counts. `validation/reference/adc.py`
independently checks the request, response hashes, source-object slices and scalar types using
standard-library JSON and Decimal. Run it with `uv run --offline --no-project`.

Native/API controls exercise all approved destinations, explicit intent, offline mode, malformed
responses, byte/record bounds, precision, pagination, redirects, errors and cancellation. Browser
controls exercise both themes, raw evidence, manual paging, stale/cancelled responses, errors and
offline state. Captures inject a labeled simulated response; no live repository is required by
CI and these checks do not claim that every listed service is currently operational.

## Reviewed downloads (wave 9, slice 8)

Inspect a returned repertoire, then choose **Download this repertoire…**. Enter one exact
`data_processing_id` from its metadata, provider permission evidence (including a source URL),
and a study citation. **Review download** is local: it shows both endpoints, the exact metadata
and initial rearrangement requests, and the page/byte budget. Total source size is unknown;
the displayed budget is not an estimate. Check the permission acknowledgment before starting.
Provider terms must be reviewed by the person; neither metadata nor registry membership grants
reuse or redistribution rights. The saved statement records the person's review, not a license
inferred or verified by ClonoLoom. Unsupported processing filters fail visibly.

One action authorizes the displayed bounded sequence of requests: one fresh metadata query,
then up to 10 rearrangement pages of 1–1000 records each. The maximum decoded response budget
is 88 MiB, including metadata, with a five-minute total deadline and the existing 45-second,
8-MiB per-response limits. The metadata must contain exactly the selected repertoire and one
matching processing entry. Every rearrangement must name that exact repertoire/processing pair
and a unique string sequence ID. Repeated IDs, mismatched scope, malformed pages or failed
requests abort the capture and remove the unfinished directory. Short nonempty pages advance
by their actual record count; an empty page stops retrieval. No retry or redirect occurs.

The capture is saved in a newly created private `adc-download-*` folder in the configured
library. Remote identifiers never become paths. Existing captures are never overwritten.
`metadata.json` and `page-NNN.json` contain the original decoded response bytes, including all
unknown fields and exact JSON numbers. `manifest.json`, written last, records the contract,
registry revision, reviewed scope/permission/citation/budgets, every exact request and its hash,
endpoint, retrieval time, source file name/hash/byte count, record counts and stop reason.
Keep the manifest and original pages together. SHA-256 verifies bytes, not publisher identity.
A process crash can leave a folder without a completion manifest; it is not a finished capture.

Stop reasons distinguish **page-budget** from **empty-page**. Both retain
`completeness: not-established`: live offsets have no stable-snapshot guarantee. An empty page
does not prove a complete cohort. No subjects or processing runs are silently pooled, and no
counts, units, pairing, specificity or scientific validity are inferred. Captured JSON is a
source-evidence bundle, not an automatically opened analysis dataset, AIRR TSV conversion or
replay certificate. Use [Public overlap](ADC-OVERLAP.md) to compare selected captures with one
local sample under a separate presence-only contract. No archive extraction is needed.

Cancel, edit an input, or close the dialog to cancel locally. A request already received by a
provider cannot be retracted; if installation finished before cancellation arrived, its complete
capture may remain. `--offline` refuses download before creating a directory or contacting a
repository. No new MCP/Python/R action or automation grant is introduced.

For an independent byte/scope check of a saved capture, run:

```bash
UV_CACHE_DIR=/tmp/clonoloom-uv uv run --offline --no-project \
  validation/reference/adc_download.py /absolute/path/to/adc-download-folder
```

This check does not certify provider authenticity, license permission or scientific validity.
