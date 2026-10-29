# AIRR Repertoire YAML

Development support: wave 9 slice 2, after the published 0.8.0 release. Open `.yaml`, `.yml`,
`.yaml.gz` or `.yml.gz` **with the rearrangement tables**, just as for Repertoire JSON. The
filename supplied with an upload is used even when the library stores it under a content hash.

This is a bounded subset of [YAML 1.2.2](https://yaml.org/spec/1.2.2/) for the
[AIRR Repertoire format](https://docs.airr-community.org/en/v2.0.0/datarep/metadata.html).
Unsupported syntax is refused with a location and a suggestion to export JSON. This reader is
not a general YAML processor, a full AIRR schema validator or a MiAIRR completeness assessment.
No plugin, Python installation or third-party runtime library is needed by ClonoLoom.

## Supported syntax

- One UTF-8 document, with LF or CRLF line endings. Optional `---` and `...` markers occupy
  their own unindented lines; comments may follow them. Root content starts in column 1.
- An AIRR Data File mapping with `Repertoire` array and optional `Info`, a repertoire array,
  or one repertoire mapping. Existing JSON import rules apply to the decoded structure.
- Space-indented block mappings and sequences, including compact `- key: value` mappings,
  sequences of scalars, empty sequence entries, and indentless sequences beneath mapping keys.
  Indentation may vary in width but must be consistent within a collection. Compact nested
  sequences such as `- - value` are not supported; put the child sequence on its own line.
- Nonempty string mapping keys, plain or quoted. Quote keys containing YAML punctuation.
  Duplicate decoded keys are refused, including differently quoted spellings of the same key.
- Single-line plain strings, including commas, URI colons, and internal brackets. Plain text
  cannot start with a reserved YAML indicator, contain `: `, end with `:`, or contain quotes.
  Use a quoted scalar for these cases. A `#` after whitespace starts a comment; elsewhere it
  remains part of a plain value. Trailing ASCII separation spaces are not string content; nonbreaking spaces remain content.
- Single-quoted strings with doubled apostrophes (`'researcher''s note'`). Double-quoted strings
  use single-line **JSON string escapes** (`\n`, `\t`, `\u03b1`, etc.). Other YAML escapes,
  multiline quoted strings and surrogate escapes are refused; use literal UTF-8 characters
  such as `α` or `😀`. Quoted values always remain strings, including `'001'` and `'false'`.
- Plain `null`, `Null`, `NULL`, `~`, and empty mapping/sequence values become JSON null.
  `true`/`True`/`TRUE` and `false`/`False`/`FALSE` become booleans. `yes`, `no`, `on`, and `off`
  remain strings under YAML 1.2. No date/time or YAML 1.1 boolean/sexagesimal conversion occurs.
- Finite **JSON-form decimal numbers**: `0`, `-0`, `123`, `-4.25`, `1e400`, `1.234567890123456789`.
  Their tokens pass through without binary floating-point conversion or exponent expansion.
  Arbitrarily large finite exponents are retained as evidence, not interpreted as machine
  infinities. Numeric-looking forms outside this grammar are refused: `01`, `+1`, `.5`, `1.`,
  `0x10`, `0o10`, `1_000`, `1:20`, and unquoted digit-leading dates/IDs. Use JSON decimal syntax
  for numbers and quote text identifiers/dates. `.inf` and `.nan` variants are refused.
- Single-line strict JSON objects/arrays as inline collections, with quoted string keys and
  no trailing commas or duplicate keys. YAML forms such as `{a: 1}` or `[yes, no]` are refused.
  A whole document written as JSON is also accepted, under the existing JSON size/structure
  limits; surrogate escapes are still refused for YAML inputs.
- Literal text blocks `|`, `|-`, `|+`, using indentation determined by the first nonempty
  content line. Text and `#` characters inside the block are retained, line endings decode
  to LF, and clip/strip/keep final-line-break behavior is respected. Blank lines must be empty,
  not filled with spaces. A missing final line break is not invented.

```yaml
Info:
  version: '2.0'              # schema declaration is a string
Repertoire:
- repertoire_id: 'R001'
  subject:
    subject_id: '001'        # identity remains text
  sample:
  - single_cell: false
    collection_time_point_relative:
      value: 0              # zero is not missing
      unit: {"label": "day"}
  lab_extension:
    instrument_record: 9007199254740993
    note: |-
      Supplied study note.
      No processing record is selected by this annotation alone.
```

## Deliberate limits and refusals

No directives (including `%YAML`), tags, anchors, aliases, merge keys, complex/explicit keys,
multiple documents, folded `>` blocks, explicit block-indent indicators, or wrapped plain
scalars. The block parser refuses raw tabs, BOM, bare CR and unsupported control characters; whole
JSON documents follow the JSON whitespace rules. Quoting reserved text is
appropriate only when it really is a string; exporting JSON is the fallback for unsupported
structural syntax. Export through a producer that preserves exact numeric values; a generic
binary-floating-point YAML-to-JSON conversion can round them. There is no alias expansion, code construction or implicit external access.

Limits are 64 MiB per source/decompressed file, 64 MiB combined decoded source metadata,
64 MiB per generated JSON projection, 100,000 repertoire records per file/dataset, nesting
at most 64, and at most 1,000,000 parser work units and JSON values. The block parser additionally
allows fewer than 1,000,000 LF separators and at most 1 MiB per physical line. Work accounting
includes collection/key/scalar visits, so it can reject a document before its JSON-value count
reaches one million. Parsing checks cancellation. Existing repertoire-ID rules remain in force:
nonempty strings without surrounding whitespace/control characters, at most 4,096 bytes,
unique across JSON and YAML metadata files in one dataset.

## Evidence and scientific meaning

**Data → Inspect AIRR metadata** shows a decoded record JSON projection and, separately, the
**original YAML document (all records)**. Neither is parsed into JavaScript numbers. Unknown
nested fields, string/number types, null, false and zero are preserved. The original snapshot
retains comments, spacing, quoting and original line endings; the JSON projection represents
decoded values, not original YAML spelling. Source SHA-256 covers the original file bytes,
including compressed bytes for gzip. A YAML file and a JSON serialization of the same values
have different source hashes/dataset identities, intentionally.

Each record projection and original-source preview has a 4 MiB limit. An oversized source
preview has an explicit notice; smaller record projections remain inspectable. An oversized
record preview is refused rather than truncated. Original source text is kept once per opened
metadata file, excluded from routine dataset/list summaries, and is not reread when inspected.
Reopening still requires the captured source file to be available under its fingerprint.

Annotation matching and its limits are unchanged: parsed repertoire IDs match case-sensitively;
legacy name matching applies only without an explicit ID. Metadata does not establish subject
independence, select a processing object or validate an experimental design. MiAIRR assessment
is a separate planned slice. This work changes no scientific estimator or replay kind.

The validation-only oracle uses ruamel.yaml **0.18.6**, YAML 1.2 nodes and scalar tags rather
than floating-point construction. Its exact expected JSON files are committed under
`testdata/metadata/yaml/` and compared with native output. See `validation/README.md` for commands.
