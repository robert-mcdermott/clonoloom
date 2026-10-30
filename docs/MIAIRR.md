# MiAIRR reporting checklist

ClonoLoom 0.9.0 adds a read-only checklist to **Data → Inspect AIRR metadata → Inspect →
Review MiAIRR reporting completeness**. It describes the reporting evidence in one captured
Repertoire record. It does **not** certify MiAIRR compliance or scientific validity.

The native checklist version is `clonoloom-miairr-2.0-v1`, pinned to **AIRR Standards 2.0**.
Files declaring `Info.version` as `2.0` or `2.0.0` are assessed; absent or other declarations
receive **not assessed**, with the original evidence still available. Older AIRR fields differ
(e.g. study contributors and age/time structures). Review and migrate against the applicable
standard rather than simply changing a version string. A supported declaration is not proof
that the file conforms to that schema.

## Reporting levels, not a quality score

The checklist preserves AIRR's three levels. These are separate from schema-required keys,
nullability and recommendations in annotation guidelines:

| AIRR level | Reporting meaning |
| --- | --- |
| Essential | Substantive information must be reported; null-like values cannot supply it. |
| Important | Report the field, retaining an explicit null-like value when information is unavailable. |
| Defined | Reporting is optional; use this field when providing information with its meaning. |

There is no percentage, grade, green “valid” badge or overall compliance decision. Counts describe
**evidence rows** by observed state, across all displayed reporting groups. They do not count
subjects, replicates or required fields satisfied. A parent scope that cannot be traversed is one
row per affected group; its unseen children are not counted as assessed. Filtering the table by
group does not change the counts. Multiple samples, diagnoses, PCR targets and data-processing
entries are kept separate. Neither a matching repertoire ID nor a completeness checklist selects
an analysis pipeline or establishes independent subjects.

## Observed states and remedies

| State | Interpretation |
| --- | --- |
| Absent key | No explicit reporting at this path. For defined fields this is optional, not a required-field gap. |
| Explicit null | A supplied JSON/YAML null, with no reason inferred. |
| Explicit empty value | Empty/whitespace-only string or empty collection; not substantive information. |
| Explicit `not_applicable` | The author asserts that study design leaves no meaningful value. Applicability is not independently verified. |
| Explicit `not_collected` | The author reports that this information was not collected. |
| Explicit `missing` | The author reports collected information that is now unavailable. |
| Nonstandard unknown marker | `unknown`, `n/a`, `na`, `not known` or `not applicable` (case/outer-space insensitive) is flagged for review, never silently rewritten into an AIRR token. |
| Value supplied | A scalar of the expected broad type exists. False and zero are values, not missingness. |
| Structure supplied · review members | A nonempty object/array exists; its members, identifiers, units and nested completeness still need review. |
| Unexpected value shape · review | The supplied JSON shape differs from the expected broad type; no coercion is applied. |

The three AIRR reason tokens are recognized exactly and remain distinct. The UI supplies a remedy
for each row, including the stricter substantive-information requirement for essential fields.
Do not replace missing data with fabricated values to improve a checklist. In particular,
`not_applicable` in an essential field is not an exemption. A null-like reporting reason also
does not prove that its representation satisfies the field's schema type.

Each path is a JSON Pointer relative to the selected repertoire, e.g.
`/data_processing/1/software_versions`. Array indices identify evidence locations, not biological
identifiers. The record, source filename, declared schema and SHA-256 remain available alongside
the checklist. For YAML, decoded JSON and original source text stay separate. Exact scientific
numbers remain in that source/projection text; the browser never parses them to compute a score.

## Coverage and limits

The inventory contains all 82 Repertoire metadata fields in sets 1–5 of the versioned MiAIRR data
elements table, including the two **defined** cell-annotation fields. It covers study, subject,
diagnosis, sample collection, cell/nucleic-acid processing, PCR targets, sequencing-run/raw-read
metadata and data-processing descriptions. Conditional contexts remain explicit review items:

- Diagnosis/intervention: absence does not establish health or lack of intervention.
- Locus-specific PCR: review the protocol. Neither a missing PCR array nor a library-method
  label proves the condition inapplicable.
- Raw-read and paired-read metadata: absent paired fields do not establish single-end sequencing.
- Commercial library protocol version: applicability requires source review.

The tool does not create not-applicable declarations, fill fields, or infer missingness from
other attributes. It does not perform nested schema validation (including ontology IDs, time
units or contributor roles), controlled-vocabulary checks, numerical range/positivity checks,
source-publication review, raw-file/accession retrieval, experimental-design verification or
biological validation. Integer fields currently recognize integer-form JSON tokens; decimal or
exponent spelling triggers shape review without numeric conversion or loss of evidence.

**Set 6 (processed AIRR sequences with annotations) is not assessed.** A source table's import
validation and the MiAIRR metadata checklist answer different questions. Full-study reporting,
raw-read availability and the scientific suitability of the data still require review. Even a
fully populated metadata object cannot establish correct annotation, adequate sampling, valid
replication, antigen specificity or a trustworthy statistical conclusion.

Inspection is bounded to 4 MiB per repertoire record and 2,000 checklist rows. Exceeding a
checklist bound or cancellation discards the entire assessment and its counts, with an explicit
notice; no partial denominator is presented. Original captured evidence and its fingerprints
are unchanged. This feature adds no permissions, external requests, metadata writes, replay kind
or change to dataset identity.

## Validation and example

`testdata/metadata/miairr.airr.json` is a deliberately incomplete, declared simulation using the
existing small UMI-count study. It illustrates two processing descriptions, empty essential
contributors, explicit null/missingness and an unmatched repertoire; it is not a compliant study
or a biological result. Its `synthetic: false` describes the modeled biological repertoire,
not a synthetic phage/display library. No real participant, instrument, read file or accession
is asserted.

The native tests exercise exact scalar evidence, separate repeated scopes, JSON/YAML equivalence,
unsupported versions, bounds, cancellation, malformed input and source removal after opening.
`validation/reference/miairr.py` compares the native inventory with a checksum-pinned extraction
of the official table and checks hand-specified evidence states independently. The full TSV can
be checked with `--source`; it is validation-only, never a runtime dependency.

```sh
GOCACHE=/tmp/clonoloom-go-cache \
  CLONOLOOM_MIAIRR_INVENTORY=/tmp/clonoloom-miairr-inventory.json \
  CLONOLOOM_MIAIRR_EVIDENCE=/tmp/clonoloom-miairr-evidence.json \
  go test ./internal/airr -run TestMiAIRRInventoryAndReference -count=1
UV_CACHE_DIR=/tmp/clonoloom-uv uv run --no-project validation/reference/miairr.py \
  --inventory /tmp/clonoloom-miairr-inventory.json \
  --evidence /tmp/clonoloom-miairr-evidence.json
```

## Sources

Reviewed 2026-10-29:

- [AIRR Standards 2.0 MiAIRR data elements](https://docs.airr-community.org/en/v2.0.0/miairr/data_elements.html).
- [AIRR field reporting levels and null-like values](https://docs.airr-community.org/en/v2.0.0/datarep/airr_schema_requirement_levels.html).
- [Repertoire field shapes and processing relationships](https://docs.airr-community.org/en/v2.0.0/datarep/metadata.html).
- [Metadata annotation guidelines](https://docs.airr-community.org/en/v2.0.0/miairr/metadata_guidelines.html).
