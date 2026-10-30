# Cell Ranger detailed JSON annotations

ClonoLoom 0.9.0 adds the detailed array layout of
`all_contig_annotations.json`.
Open the file in Data, then choose **Inspect native report**. Gzip and named uploads
are supported; renamed `.json` files are recognized by their first contig object.

## Layout and version recognition

The supported structural contract is `cellranger-nested-v1`: one top-level JSON array
of detailed contig objects with `contig_name`, `barcode`, `sequence` and an `annotations`
array. Annotation features name `chain`, `gene_name` and `region_type`. Supported
regions are `5'UTR`, `L-REGION+V-REGION`, `D-REGION`, `J-REGION` and `C-REGION`.
Unknown fields, including nested annotations and exact numbers, remain source evidence.

The producer does not declare a schema/software version in these arrays. ClonoLoom
recognizes this layout, not a guessed Cell Ranger version. The implementation is
checked against the public Cell Ranger 4.0.0 output listed below and the documented
detailed JSON fields in Cell Ranger 7.2. Synthetic controls cover explicit `full_length`
and `sample` fields as well as their omission. This does not claim support for every
historical/future producer version or a full vendor schema validator.

Flat JSON copies of CSV rows (`contig_id`, `v_gene`, `umis`, etc.), wrapper objects,
consensus JSON and non-null legacy `clonotype` structures are unsupported. Use a
documented CSV/AIRR export for these layouts. Known conflicting flat aliases are
refused even if nested fields are also present. Missing feature identities, unknown
chain/region names and competing gene calls for the same segment are refused; the
reader does not reproduce vendor scoring or select a preferred hit.

Folder opening keeps existing precedence: filtered contig CSV, then all-contig CSV,
then all-contig JSON. It selects one representation per recognized folder, including
`outs/` and `per_sample_outs/*/vdj_t`, `vdj_b` and `vdj_t_gd`. JSON does not automatically
replace a previously opened CSV. Open the JSON file explicitly to inspect its additional
evidence. Loading alternate representations together cannot duplicate the same
sample/contig identity silently.

## Scientific mapping

| Source | Imported meaning |
| --- | --- |
| `contig_name`, `barcode` | Exact contig identity and barcode; no suffix removal or generated pairing |
| `sample` | Explicit sample scope when supplied; otherwise the established filename/run-folder rule |
| Feature `gene_name` | V/D/J/C calls by explicit region; UTR retained only as evidence |
| Feature `chain` | A single locus only when all annotations agree; mixed or absent chains remain unresolved |
| `cdr3_seq`, `cdr3` | Anchored nucleotide/amino-acid junctions, without adding residues or translating missing peptides |
| `read_count`, `umi_count` | Separate per-contig reads and UMIs; the table's contig support is UMIs |
| `is_cell`, `high_confidence`, `productive`, `full_length` | Supplied boolean calls; absent/null is unknown |
| `info.raw_clonotype_id` | Source label, separate from the active ClonoLoom clonotype definition |
| `sequence` | Supplied contig nucleotide evidence; not a reconstructed germline or alignment |

All contigs are imported, including background barcodes, nonproductive records and
multiple chains. The active QC filter and explicit single-cell definition determine
which rows/cells enter a result. **Clonotype abundance counts distinct cells, not sums
of contig UMIs or reads.** A barcode can name a background droplet; its presence is not
a confirmed cell call. A sample/barcode link does not establish a single biological
cell, receptor pairing, antigen specificity or independence of biological replicates.

Identifiers are case-sensitive strings with no padding/control characters, up to 4,096
bytes. Samples separate otherwise identical barcodes. Review the scope before combining
runs; equal sample names are not independent subjects. Duplicate contig identities and
contradictory nonmissing `is_cell` or source clonotype calls within one sample/barcode
are errors. Missing calls do not become false or overwrite known calls. Gene alternatives
and structural conflicts require an explicit supported export rather than guessed pairing.

Counts must be nonnegative integer JSON tokens at most `2^53 − 1`; strings, fractions,
exponents and larger counts are refused. Zero remains zero and missing remains missing.
Other numeric annotations retain their original tokens even beyond JavaScript's exact
integer range. No count or scientific evidence passes through browser number conversion.

Missing full-length, frame and stop flags are **not inferred**. In particular, an older
JSON file's absent `full_length` is not interchangeable with its CSV's supplied call.
`frame` is documented as unused. `stop_codon_pos` can refer to a position outside the
V–J interval; it is not converted into an AIRR stop flag. Alignment coordinates, CIGARs,
quality strings, translated sequence, junction support, GEX/assembler calls and other
annotations remain inspectable native evidence, without unvalidated coordinate or
orientation conversion. Existing sequence/QC findings do not certify receptor function.

## Exact evidence and limits

Each original contig object is captured as text, preserving internal whitespace, unknown
fields and numeric tokens. The source fingerprint covers the entire original file (the
compressed bytes for gzip). Array separators, outer whitespace and gzip framing are not
included in the individual object snapshots. The displayed record number is an array
ordinal, not a physical text line or biological identifier.

Captured evidence survives disposable-cache reopening and remains inspectable after
source removal. It is explicitly separate from generic source metadata, scientific exports
and private identity maps. AIRR/CSV exports contain their mapped fields, not a lossless
Cell Ranger JSON export. Keep original files to transfer unsupported annotations; portable
source inclusion retains its existing explicit permissions and provider restrictions.

Limits: 64 MiB decompressed input per file and 64 MiB combined captured native text,
100,000 contigs per JSON file, 8 MiB per object, nesting depth 64 and one million JSON
values per object. Input is decoded one contig at a time with cancellation checks.
Malformed/oversized input is refused without a partial dataset. Duplicate JSON keys,
invalid UTF-8, unpaired Unicode escapes and trailing values are rejected. Empty arrays
are supported under a recognized annotation filename; unnamed empty arrays cannot
identify their biological format.

The inspector pages 20 contigs at a time (API maximum 50). An object over 4 MiB is
retained but its preview is refused explicitly. Absence, null, false and zero are shown
separately. Reader-cache version 5 invalidates disposable older caches; scientific identity
version 1, prior valid table-reader values, release fixtures and replay contracts are unchanged.

## Validation and references

The committed fixtures are **simulated import controls**, with unequal read/UMI support,
paired/multiple-chain barcode arrangements, background contigs, missing flags, zero counts
and an unresolved mixed-chain annotation. Their coding constructs are not complete biological
receptors; planted source calls are not validated transcript functionality. Both-theme
screenshots show source evidence, not inferred biology or a research effect.

Independent Python standard-library JSON/Decimal and CSV parsing compares every mapped field,
original object and fingerprint; the shared artificial AIRR fields are also checked. No vendor
code, analysis algorithm or runtime dependency is used. Reproduce with:

```sh
UV_CACHE_DIR=/tmp/clonoloom-uv uv run --no-project validation/reference/cellranger_json.py
GOCACHE=/tmp/clonoloom-go-cache go test ./internal/reads ./validation -run TestCellRangerJSON
```

The public Cell Ranger 4.0.0 T-cell JSON/CSV comparison covers all 1,835 contigs and 16,515
shared values, plus gene/chain and source clone calls. All JSON objects lack `full_length`;
14 have mixed-chain annotations. These remain explicit differences from reconstructed or
CSV-only flags/loci. Comparison establishes format fidelity, not biological truth. Public
data and per-contig reference outputs stay outside the repository.

Sources reviewed 2026-10-29:

- [10x Genomics, Cell Ranger 7.2 detailed annotation outputs](https://www.10xgenomics.com/support/software/cell-ranger/7.2/analysis/cr-5p-outputs-annotations-vdj).
- [10x Genomics, V(D)J annotation and productivity criteria](https://www.10xgenomics.com/support/cn/software/cell-ranger/8.0/resources/cr-5p-vdj-algorithm-annotations).
- [10x Genomics, Human PBMC from a healthy donor, 1k cells, Cell Ranger 4.0.0](https://www.10xgenomics.com/datasets/human-pbmc-from-a-healthy-donor-1-k-cells-v-1-1-2-standard-4-0-0), CC BY 4.0; [detailed T-cell JSON](https://cf.10xgenomics.com/samples/cell-vdj/4.0.0/sc5p_v1p1_hs_PBMC_1k/sc5p_v1p1_hs_PBMC_1k_t_all_contig_annotations.json).
