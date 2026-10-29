# Typed Parquet dataset exports

ClonoLoom 0.8.0 provides native and explicitly pseudonymized Parquet dataset exports.
Use matching clients: action API **2**, catalog **4** (64 actions), permission policy **1**.
This is an export representation, not a new analysis.

## Export and inspect

Open **Reports → Typed Parquet exports**, choose a dataset and either clonotypes or
rearrangements, and select **Export Parquet ZIP**. The Clonotypes export menu offers the same
choices. The active dataset definition, QC and invariant-species setting apply. Acknowledge
provider restrictions when required; acknowledgment does not grant redistribution rights.

Ordinary Parquet retains original identifiers, source paths and fingerprints. For a separate
pseudonymized copy, choose **Typed Parquet** under **De-identification export**, make the
annotation-removal choices, then prepare and review before downloading. The private identity
map is still a separate download; see [the de-identification guide](DEIDENTIFICATION.md).
Changing format invalidates that review. Parquet itself is not a de-identification method.

No original file, saved result, project or history is rewritten. Cancellation or a changed
project prevents a pending window download; cancellation cannot retract an already saved file.

## Version 1 tables and types

Only the following native table layouts are mapped. There is no general result-to-Parquet
converter or Parquet import in this slice. Native row order and column order remain intact.

| Table | Typed fields | Remaining evidence |
| --- | --- | --- |
| `clonotypes.parquet` | `total`, `samples`, and sample abundances: optional signed INT64 | Clonotype ID, chain loci/junctions/gene calls and optional invariant-rule statuses: UTF-8 text |
| `rearrangements.parquet` | `duplicate_count`, `umi_count`, `templates`: optional INT64; `productive`, `vj_in_frame`, `stop_codon`: optional BOOLEAN | Every other retained native AIRR field: UTF-8 text, including coordinates, invalid annotation text and nested annotation JSON |
| `invariant-samples.parquet` | `total`, `evaluable`, `matched`, `rule_version`: optional INT64; `share`: optional DOUBLE | Sample, species and population: UTF-8 text. Included with ordinary clonotypes when a species is selected. |
| `samples.parquet` | All fields optional UTF-8 text | Only in the pseudonymized workflow: aliased sample/subject and explicitly retained annotations |

Clonotype sample columns use stable positional names such as `sample_000001`. The schema's
`sampleColumns` maps each key to its sample; each field also records its `nativeName`. This
avoids collisions when a sample is named `total` or another scientific field. Keys are stable
for the same ordered sample list, not permanent sample identifiers across different exports.

**Missing is not zero.** Empty native fields become Parquet null; explicit zero remains zero.
A missing sample abundance means no stored abundance in the native table, not an invented
measurement. Native text exports do not distinguish an empty string from a missing text field;
this mapping cannot reconstruct that distinction. Boolean T/F becomes true/false.

Native counts currently use float64. This export accepts only nonnegative, finite, integral
values at most **9,007,199,254,740,991 (2^53−1)** and writes them exactly as INT64. Negative,
fractional, infinite and larger counts refuse the whole export. Native NaN is the missing-value
sentinel and remains null. The restriction is deliberately conservative: although INT64 can
hold larger integers, Parquet cannot recover precision already lost during reading or native
computation. Do not cast nullable integer columns to floating point in downstream software if
exactness matters.

Rearrangement units and clonotype abundance units are recorded separately in ordinary provenance;
they can differ for single-cell data. The count column names retain their native meanings.
Counts are not normalized, rescaled, rounded or silently replaced. Source reader normalization,
active QC and the chosen clonotype definition still determine the exported population.
The native exporter places retained reads in `duplicate_count` and template abundance in
ClonoLoom's `templates` extension; Parquet preserves those existing meanings. Orientation
(`rev_comp`) remains retained annotation text, including invalid input, rather than being
coerced into a newly validated boolean.

Invariant shares retain the native denominator: all retained sample abundance, with evaluable
abundance reported separately. Rule populations may overlap. These sequence rules are not
established cell phenotypes or specificity assignments. Retained AIRR text is not newly
validated biology, and this flat representation is not a replacement AIRR schema.

## Provenance and integrity

The ordinary ZIP contains the selected Parquet table, optional invariant sample table,
`schema.json`, `NOTICE.txt` and `inventory.json`. Each footer includes:

- `clonoloom`: the exact corresponding `clonoloom-parquet-table` version 1 schema JSON,
  including row count, field types/roles, sample mapping, provenance and scientific limits.
- `NOTICE`: the same notice text as the bundle, including method/provider attribution.

Ordinary provenance captures software/version/commit, dataset identity, normalized definition
and ID, effective QC, units, source and metadata fingerprints, and provider credits/restrictions.
Source fingerprints are those recorded when loaded, not a new verification of files on disk.
The `clonoloom-parquet-export` receipt inventories member paths, sizes and SHA-256 digests;
its fingerprint hashes the serialized sorted member list. The inventory excludes itself.
This detects changed recorded bytes, not authorship, scientific validity or successful replay.

For the pseudonymized representation, the existing transformation policy runs first. The ZIP
substitutes `.parquet` tables for CSV/TSV and adds `schema.json`; `transformation.json` remains
the reviewed inventory. Its optional `storageFormat:"parquet"` records the representation.
Each footer carries only the sanitized public transformation record (without its file list),
aliased sample mapping and public notice. Original paths, hashes and identities remain in the
separate private map. Unsupported nested IMGT/germline evidence still refuses the AIRR option.
Retained sequences, stable clonotype IDs and measurements remain linkable; no anonymity or
replay of transformed evidence is promised.

The writer is deterministic for fixed table values, schema, metadata and notice. Fresh notice
timestamps or pseudonyms change complete export bytes; separate exports are not promised the
same fingerprint. Keep originals and saved artifacts for exact replay under its existing limits.

## Bounds and file layout

The standard-library writer emits flat optional columns, PLAIN values, RLE definition levels,
GZIP-compressed DATA_PAGE v1 pages and a Thrift Compact Protocol footer with file version 1.
No dictionary, nested schema, statistics, bloom filters or reader is implemented. Consumers
must honor the validity/definition levels, not infer missingness from numeric placeholders.

Native preparation permits one request at a time, 1,000 samples, 1,100 fields, a 1 MiB settings
body, 64 MiB per native table and 64 MiB total ordinary ZIP output. Text cells are limited to
1 MiB and AIRR lines to 8 MiB. Groups flush at 8,192 rows or an estimated 16 MiB; the writer
also caps a group at 32 MiB, 4,096 groups and 16,384 column chunks. The default PLAIN value-page
budget is 1 MiB (definition levels add overhead). Footer key/value metadata is limited to
1 MiB and the serialized footer to 8 MiB. A wide table can hit a chunk limit before a row limit.
The writer's separate 256 MiB output ceiling does not relax the export's lower 64 MiB limit.

The pseudonymized workflow retains its own additional preparation/map/bundle limits, and each
Parquet table uses the same representation bounds. Preparation has a five-minute context and
checks cancellation during conversion/writing; native clonotype construction is not interruptible
midway. Validation completes before a successful native download response. Oversized or
unsupported exports refuse without returning a partially valid file.

## Automation and independent readers

`export_parquet(dataset?, what="clonotypes", accept=false)` requires explicit `read`, `compute`
and `files` grants plus human proposal review. It returns the receipt; the ZIP downloads on
the host's browser computer. `what="rearrangements"` selects AIRR evidence. The generated
Python method is `export_parquet`, the R function is `cl_export_parquet`; neither grants
permissions or approves proposals. `prepare_deidentified_export` adds optional
`storageFormat="parquet"` (`storage_format` in Python/R); its default stays text.

The native window route is `POST /api/parquet/export`, with dataset, what, definition, filter,
species and accept. It shares existing loopback/window protections and is not a separate remote
permission mechanism. Successful responses are `no-store` ZIPs with an
`X-ClonoLoom-Parquet-Receipt` header. No scientific values are converted by browser JavaScript.

Extract a reviewed bundle to a directory, then inspect the typed file without an implicit
pandas float conversion, for example:

```sh
uv run --no-project --with pyarrow==26.0.0 --with duckdb==2.0.0 - <<'PY'
import json
import pyarrow.parquet as pq
import duckdb

table = pq.read_table("clonotypes.parquet")
schema = json.loads(table.schema.metadata[b"clonoloom"])
print(table.schema)
print(schema["sampleColumns"])
print(table["total"].to_pylist()[:5])  # Python int or None
with duckdb.connect() as db:
    print(db.execute("SELECT total FROM read_parquet(?) LIMIT 5",
                     ["clonotypes.parquet"]).fetchall())
PY
```

Independent controls use PyArrow 26.0.0 and DuckDB 2.0.0 to decode every native field and
compare against ordinary exports, check integer/null/boolean/text boundaries, empty and
multiple row groups, inventory bytes and decoded disclosure canaries. Writer-only controls
include full signed INT64 extrema and 2^53+1; those test the file encoding, not permission to
export such counts from the float64 native model. Browser checks use six observations of two
seeded simulated vaccination subjects, unchanged project/query evidence, rejected proposals,
cancellation and stale-project guards. Both themes are reviewed and accessibility-checked.

Sources: [Apache Parquet file format](https://parquet.apache.org/docs/file-format/),
[encodings](https://parquet.apache.org/docs/file-format/data-pages/encodings/),
[parquet.thrift](https://github.com/apache/parquet-format/blob/master/src/main/thrift/parquet.thrift),
[Thrift Compact Protocol](https://github.com/apache/thrift/blob/master/doc/specs/thrift-compact-protocol.md),
and the [AIRR Rearrangement standard](https://doi.org/10.3389/fimmu.2018.02206).
The writer adapts Apache-2.0 SituCraft v0.9.0; see NOTICE and research §5.53 for scope and changes.
