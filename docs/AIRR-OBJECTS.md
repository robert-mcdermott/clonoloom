# Imported AIRR Clone, Cell and Receptor objects

Wave 9 slice 4 development reads **AIRR 2.0 Data Files** containing `Clone`, `Cell`,
`Receptor` and supporting `Node` arrays. Open them alongside rearrangement tables, then use
**Data → Inspect AIRR objects**. This is not part of the published 0.8.0 binary.

Objects are source assertions, not newly computed analysis. An imported Clone does not replace
the active clonotype definition, set sequence membership, change abundance units or become a
ClonoLoom clonal family. Cell and Receptor objects do not create biological cells, infer chain
pairing or establish antigen specificity. The supplied counts, pairing flags, sequences and
extensions remain available as evidence.

## Accepted files and identity

Use an AIRR Data File dictionary with `Info.version: "2.0"` (or `"2.0.0"`) and named
collection arrays, optionally alongside `Repertoire`. JSON and the existing
[bounded YAML subset](AIRR-YAML.md), including gzip and uploaded filenames, are supported.
Quote digit-leading YAML identifiers and receptor hashes. New object collections with missing
or other versions are refused; migrate according to the source schema instead of relabeling.
Legacy repertoire-only documents retain their existing behavior except ambiguous bare records
(or root-array entries) with `clone_id`, `cell_id`, `receptor_id` or `node_id`: these must use an
explicit collection. This prevents a Cell's `repertoire_id` reference from being silently
misread as a Repertoire identity. A legacy Repertoire using those names as extension fields
can still be read inside an explicit `Repertoire` wrapper.

Each object keeps its original JSON bytes or exact decoded YAML projection, collection path,
source-file association and full captured source document. Unknown object fields and wrapper
extensions remain in the source; they are not interpreted as additional supported schemas.
Fingerprints cover original file bytes, including compressed bytes for gzip. Inspection reads
the opened snapshot, not a subsequently changed source. Numeric values do not pass through
JavaScript numbers. Scalar null, false, zero and absent fields stay distinct.

ClonoLoom requires nonempty case-sensitive object IDs of at most 4,096 bytes, without surrounding
whitespace or control characters. Identifiers must be unique **per object kind across the
opening**, including nested and standalone Nodes. This is a conservative import rule: study-local
IDs are not globally unique, and opening multiple studies with colliding IDs requires separate
datasets or explicit source disambiguation. Identical repeated definitions are also refused;
they are not silently deduplicated. Repertoire identifiers retain the earlier uniqueness rules.

## Structural checks and links

| Object | Checked structure | Preserved interpretation |
| --- | --- | --- |
| Clone | ID; exactly one non-null repertoire or repertoire-group ID; `nodes` array of Node objects or explicit null; nonnegative integer-form `clone_count` when supplied | Source-defined ancestry assertion; count units/meaning are not inferred |
| Node | ID; explicit nullable repertoire ID; at most one non-null `cell_id` or `sequence_id` | May refer to observed, inferred or simulated evidence; missing member links are not invented |
| Cell | ID; explicit nullable repertoire ID; explicit boolean/null `virtual_pairing`; nullable array of unique receptor IDs | Multiple receptors are allowed; false, true and unknown pairing remain different |
| Receptor | ID; receptor type and ordered loci; two nonempty amino-acid strings; supplied hash | Domain completeness, physical pairing and function remain unverified |

Both nested `Clone.nodes` and standalone `Node` collections are supported. Links resolve only by
explicit identifiers to loaded objects. Data-processing IDs are scoped by the object's repertoire;
duplicate processing IDs in referenced repertoire metadata are refused. No fallback to a sample
name, matching sequence, repeated barcode or the sole processing record is made. Conflicting
non-null repertoire IDs across a locally resolved Clone–Node or Node–Cell link are refused.

The link table covers repertoire/data-processing references, Clone nodes/inferred ancestor,
Node cell/sequence references and Cell receptor references. Other source annotations such as
`seed_id` and external `receptor_ref` values remain in the exact object text; they are not
included in the displayed link count or resolved.

Resolution states:

- **loaded:** the explicitly named target is present; this is an identifier match, not validation
  of biological identity, processing suitability or scientific truth.
- **unresolved:** no matching target was loaded; the target may legitimately be external.
- **unscoped:** a processing reference lacks a repertoire scope and cannot be resolved safely.
- **not-assessed:** rearrangement references and repertoire-group membership are preserved but
  are not joined or verified in this slice. Rearrangement-table imports keep their own validation.

Receptor hashes are checked using AIRR's SHA-256 of the concatenated uppercase variable-domain
1 and variable-domain 2 strings, encoded as lowercase hexadecimal. Ordered locus pairs are
IGH–IGK/IGL for `Ig`, TRB–TRA or TRD–TRG for `TCR`. This reader accepts the 20 standard amino
acids and X (ASCII, case-insensitive); other alphabet symbols and unsupported receptor combinations
are refused. It does not synthesize a missing hash or infer domain boundaries from length, V/J
calls or a junction. A correct hash can describe an incomplete or biologically incorrect sequence.
External `receptor_ref` entries remain source annotations; no network request follows them.

## Bounds and limits

The existing 64 MiB combined decoded metadata/source limits, nesting depth 64 and one-million
JSON-value bounds still apply. New collections are additionally bounded to 100,000 objects across
an opening, 1,000 memberships per Clone or receptor references per Cell, and one million total
explicit links. The object inspector pages at 20 records (API maximum 50), with 4 MiB object and
full-document previews. Oversized previews are refused/not shown explicitly, without truncating
the stored evidence. Cancellation is checked during import and link resolution.

No full AIRR schema, ontology, MiAIRR-compliance, clone-ancestry, count-unit, variable-domain
completeness, pairing or reactivity certification is performed. Groups, rearrangement membership,
expression/reactivity objects and native object export are outside this slice. A table is still
required when opening a dataset. Source files can contain legitimate external references; their
absence from an opening is not automatically an invalid experiment.

## Validation

`testdata/airr-objects/` contains four simulated rearrangements (two modeled cells), source
membership assertions, two constructed domain-like sequences, an unmatched receptor/processing
reference, explicit unknown pairing, an unassessed sequence link and exact numeric extension
controls. These are artificial import fixtures, not observed receptors or biological results.
The extension's large integer/precise decimal test serialization, not realistic clone abundance.

The independent `validation/reference/airr_objects.py` uses Python's exact integer/Decimal JSON
reader and pinned ruamel.yaml 0.18.6 nodes. It compares all seven captured objects by source path,
scalar meaning and hash. The independent YAML projection is committed for ordinary native CI.

```sh
GOCACHE=/tmp/clonoloom-go-cache CLONOLOOM_OBJECT_EVIDENCE=/tmp/clonoloom-objects.json \
  go test ./internal/airr -run TestAIRRObjectsExactAndLinked -count=1
UV_CACHE_DIR=/tmp/clonoloom-uv uv run --no-project --with ruamel.yaml==0.18.6 \
  validation/reference/airr_objects.py /tmp/clonoloom-objects.json
```

## Sources

AIRR Community Standards 2.0, reviewed 2026-10-29:
[Clone and Node](https://docs.airr-community.org/en/v2.0.0/datarep/clone.html),
[Cell](https://docs.airr-community.org/en/v2.0.0/datarep/cell.html),
[Receptor](https://docs.airr-community.org/en/v2.0.0/datarep/receptor.html), and
[Repertoire processing links](https://docs.airr-community.org/en/v2.0.0/datarep/metadata.html).
