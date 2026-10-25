# Formats

What ClonoLoom reads and writes, and how it reads each format. The research behind these choices,
with sources, is in [clonoloom-spec/research.md](../clonoloom-spec/research.md).

Every rearrangement table can be gzipped (`.gz`); ClonoLoom recognizes gzip by its first bytes, not by the
name. Lines may end in LF or CRLF, and a UTF-8 byte-order mark is ignored. A file's format is
decided from its header, whatever its name, except IMGT's multi-file bundle: its documented
filenames identify table roles and headers are then checked.

## What every rearrangement reader produces

For repertoire formats, ClonoLoom reads rearrangements into one table with these fields (named as
the AIRR Rearrangement schema names them):

| Field | Meaning |
| --- | --- |
| sample | The repertoire the row belongs to (below, per format) |
| `sequence_id` | The row's identifier as given (a contig ID for Cell Ranger; none for immunoSEQ and unpaired OAS units) |
| `locus` | IGH, IGK, IGL, TRA, TRB, TRD or TRG |
| `v_call`, `d_call`, `j_call`, `c_call` | Gene calls in IMGT nomenclature, ties separated by commas |
| `junction`, `junction_aa` | The IMGT junction: the CDR3 with its conserved cysteine and phenylalanine or tryptophan |
| `productive`, `vj_in_frame`, `stop_codon` | As given; missing when not given |
| count | The abundance in the dataset's unit |
| reads, UMIs | Reads and UMIs where the format gives them beside the count |
| cell, clone | Single cell: the barcode, and the source's own clonotype or clone ID |

A missing value stays missing: a count that was not given is not a count of one. AIRR sequence evidence is retained separately: sequence, orientation, query and germline
alignments, V/D/J CIGARs and their sequence/germline/alignment coordinates, FWR/CDR coordinates,
N/P lengths and the named germline database. Original text (including invalid values) is
inspectable and exportable. Other source columns stay in the source; exports do not re-read
arbitrary columns.

### Abundance units

Each dataset has one unit, said with every result:

| Unit | When |
| --- | --- |
| cells | Single-cell data: a clonotype's abundance is its number of cells |
| templates | immunoSEQ files with a positive template count on every row |
| reads | immunoSEQ files without templates on every row (each such file is counted in reads throughout, with a finding); AIRR files with `consensus_count` alone; MiXCR tables without a count of molecules on every clone |
| UMIs | AIRR files with `umi_count`; MiXCR tables with `uniqueMoleculeCount` on every clone |
| unknown | AIRR files with `duplicate_count` alone, immunoSEQ v2's `count (templates/reads)` without `estimatedNumberGenomes`, unpaired OAS units (`Redundancy`) and tcrdist3 tables of one chain (`count`): the format does not say what was counted, so the user confirms it in the Data view |
| sequences | No count at all: each row counts once |

## AIRR Rearrangement TSV

The AIRR Community's standard (schema 1.3 to 2.0), written by IgBLAST, Change-O, MiXCR's AIRR
export, TRUST4, Cell Ranger (`airr_rearrangement.tsv`) and AIRR Data Commons downloads.

- Recognized by a tab-separated header with `sequence_id` and `junction`, `junction_aa` or
  `v_call`.
- Every column the schema types is checked on every row: booleans (`T`, `F`, and the spellings
  the AIRR libraries accept), integers and numbers. Values that are not of their type are read as
  missing and counted by the validator, with the line, column and value of the first few.
- Columns the schema requires but the file lacks are reported; the file is still read when it
  has a junction.
- The sample is the row's `repertoire_id`, else its `sample_processing_id`, else the file's name.
- Counts: `umi_count`, else `duplicate_count`, else `consensus_count`; with `umi_count`, reads
  are `consensus_count` (or `duplicate_count`). Rows that name a cell (`cell_id`) make the
  dataset single-cell.

**Repertoire metadata** (`.json`, the AIRR Data File or a list of Repertoire objects, as the ADC
API answers) named with the tables annotates their samples by `repertoire_id`: study, subject,
species, sex, age, diagnosis, tissue, cell subset, time point, template and HLA. Schema 1.3's
text fields and 2.0's objects are both read. YAML metadata arrives in a later release.

### Sequence evidence (0.5.0)

B cells → Inputs counts IGH/IGK/IGL rearrangements after the active QC filter, separately for
sequence present, paired alignments, structurally valid evidence and supplied region coordinates.
It neither groups rows into families nor computes a mutation rate. Missing evidence stays missing.

AIRR coordinates are 1-based closed intervals in their named query, reference or alignment
string. FWR/CDR coordinates use the query sequence, not an IMGT position in a padded alignment.
Orientation is preserved; alignments are not reverse-complemented again. The validator checks
positions, nucleotide symbols, paired alignment lengths and AIRR CIGAR operations. Findings
retain source locations and do not discard otherwise usable junctions. Cell Ranger AIRR
alignments may have unequal lengths; these are preserved and reported, never padded to match.

AIRR export fills the retained evidence fields and appends available optional columns in a fixed
order. It preserves invalid evidence for inspection; a file with such evidence remains invalid
until corrected at its source. Abundance and clone IDs retain their existing export semantics.


## IMGT/HighV-QUEST

Open an extracted output folder, or select these four tab-separated files together (optionally
with `.gz`): `1_Summary.txt`, `2_IMGT-gapped-nt-sequences.txt`, `3_Nt-sequences.txt` and
`6_Junction.txt`. `.tsv` and a shared sample prefix before the numbered name also work.
`11_Parameters.txt` is optional; its text and checksum are retained, with a 64 KiB limit.
Other reports in the folder are not imported. Extract ZIP/TXZ downloads before opening them;
Excel workbooks and individual detailed result files are not supported by this reader.

The reader joins by **Sequence number and Sequence ID**, independently of table row order.
Both must agree. Duplicate sequence numbers, missing rows, conflicting gene calls and incomplete
bundles are errors. Repeated sequence IDs with distinct numbers remain distinct. A folder's name
(or the shared filename prefix) names the sample. Uploaded unprefixed tables use `IMGT`; open one
such run at a time. Every contributing table is fingerprinted, including Parameters. Stored group
names survive cache reopening and project relocation. Source files remain unchanged.

- V/D/J/C calls lose the species and functionality decorations; all tied gene/allele names remain
  as a sorted set. Original calls are retained. Productive/unproductive map to T/F; unknown and
  unsuccessful annotations remain missing. No reads, UMIs, cells or isotypes are inferred.
  Each annotated row contributes one **sequence** (the same unit as a count-free AIRR table).
- `6_Junction` supplies the un-restored nucleotide JUNCTION and, where supplied, its un-restored
  `JUNCTION (AA)`. The Summary AA field is a fallback only without a frameshift marker and without
  an out-of-frame junction. Corrected translations remain native annotations, not canonical AA
  junctions. The conserved anchors are included; CDR3 alone is not substituted.
- `sequence` is Summary's already oriented Sequence, preserved with case. Exported `rev_comp=F`
  describes that oriented sequence, not the originally submitted strand. Native `Orientation`
  retains the original sign. No second reverse complement is applied.
- `sequence_alignment` is table 2's V-D-J-REGION, else V-J-REGION, else V-REGION, with IMGT dots
  preserved. No padding, germline alignment, CIGAR, N/P length or germline database is invented.
- V/D/J and FWR/CDR start/end pairs from table 3 become AIRR query coordinates only when the
  uncorrected region exactly matches the Summary substring (case-insensitive, 1-based closed).
  Reported insertions or deletions withhold all query mappings. Incomplete, inconsistent or
  unmatched endpoints also remain native positions with a finding. IMGT-numbered alignment
  positions are never confused with query coordinates.
- `imgt_annotations`, an optional JSON-valued extension column, retains the sequence number,
  contributing table lines, original calls, functionality/comments, orientation, Summary AA,
  indel annotations, table 6's junction/frame/restoration fields and native region coordinates.
  B cells → Inputs exposes it under **Native IMGT annotations**, and AIRR export/re-import retains
  it. The Data inspector shows the supplied Parameters text. Remaining source columns stay in
  their files; vendor mutation statistics are not imported as ClonoLoom results.

The field contract is documented in [IMGT/V-QUEST's output guide](https://www.imgt.org/IMGT_vquest/user_guide)
(tables 1, 2, 3, 6 and 11; Brochet et al. 2008, doi:10.1093/nar/gkn316).
This reader does not run IMGT, reconstruct references or establish mutation readiness.

## Cell Ranger V(D)J (10x Genomics)

- **Folders.** A `vdj` run's folder or its `outs/`, and a `multi` run's folder (its
  `outs/per_sample_outs/<sample>/vdj_t`, `vdj_b` and `vdj_t_gd`), are each opened as one
  dataset. `filtered_contig_annotations.csv` is read; `all_contig_annotations.csv` when there is
  no filtered file. `clonotypes.csv`, `consensus_annotations.csv` and `metrics_summary.csv` are
  kept beside it for QC to compare with.
- **Contig annotations** (Cell Ranger 3 to 9): `barcode`, `contig_id`, `chain`, `v_gene` …
  `c_gene`, `productive`, `cdr3` and `cdr3_nt` (the junction), `reads`, `umis`,
  `raw_clonotype_id`, and the flags `is_cell`, `high_confidence` and `full_length`. Booleans
  written `True`/`False` (Cell Ranger 3 to 5) or `true`/`false` (6 and later); `None` and empty
  values are missing. Cell Ranger 9's `sample` column names each contig's sample.
- The sample is named by the `sample` column, else the prefix 10x's downloads give the file
  (`sc5p_v2_hs_PBMC_10k_t_filtered_contig_annotations.csv`), else the run's folder.
- A contig's count is its UMIs; its reads are kept beside them. The dataset's unit is cells.
- A clonotype summary or metrics file opened on its own is refused with a message saying to open
  its run's folder or its contig annotations.

## immunoSEQ (Adaptive Biotechnologies)

Exports of the immunoSEQ Analyzer and immuneACCESS downloads:

- **v3** (`rearrangement`, `amino_acid`, `frame_type`, `templates`, `seq_reads` or `reads`,
  `v_resolved`, `v_gene`, `v_gene_ties` …), with the sample's metadata repeated on each row.
- **v2** (`nucleotide`, `aminoAcid`, `count (templates/reads)`, `estimatedNumberGenomes`,
  `sequenceStatus`, `vMaxResolved`, `vGeneName`, `vGeneNameTies` …).

How they are read:

- `amino_acid` (`aminoAcid`) is the junction. The nucleotide junction is `cdr3_rearrangement`
  where the export gives it, else the `cdr3_length` bases of `rearrangement` (`nucleotide`) from
  `v_index` (`vIndex`).
- `frame_type` (`sequenceStatus`): `In` is productive; `Stop` is in frame with a stop codon;
  `Out` is out of frame.
- Counts are templates when every row of the file has a positive template count; otherwise every
  row is counted in reads, and the validator says how many rows lacked templates (some published
  samples have −1).
- `null`, `na`, `unresolved` and `unknown` are missing values.
- The sample is `sample_name`, else the file's name.

**Gene names** are mapped from Adaptive's to IMGT's: `TCRBV05-01*01` is `TRBV5-1*01`;
`TCRBV09-01` is `TRBV9` (IMGT writes no number for a subgroup with one gene, decided from the list
of human genes); `TCRBV20-or09_02` is `TRBV20/OR9-2`; `TCRBD01-01` is `TRBD1`. A gene resolved only
to its family is called by its ties: `TCRBV20` with ties `TCRBV20-01,TCRBV20-or09_02` is
`TRBV20-1,TRBV20/OR9-2`; `TCRBV12-03/12-04` is `TRBV12-3,TRBV12-4`. Every name a file uses is
recorded with the IMGT name it was read as. Mouse names keep their gene numbers.

## MiXCR clone tables (MiLaboratories)

The clone tables `mixcr exportClones` writes (MiXCR 2, 3 and 4), bulk or single-cell. MiXCR's AIRR
export is an AIRR file, read as one.

- Recognized by a tab-separated header with the junction's amino acids (`aaSeqCDR3`, MiXCR 2's
  `AA. Seq. CDR3`) and a segment's hits (`allVHitsWithScore`, `bestVHit`, `bestVGene`, `All V
  hits`).
- **Calls.** A segment's hits are listed best first with their scores (`TRBV7-3*00(248),
  TRBV7-2*00(200)`): the call is the best hit and any other of the same score, without its score.
  MiXCR's allele `*00` (its library gives no alleles) is dropped: `TRBV7-3`. The locus is
  `topChains` where the table has it, else the calls'.
- **Junction.** `nSeqCDR3` and `aaSeqCDR3`: MiXCR's CDR3 is the IMGT junction. A `_` in the amino
  acids (a frameshift) or nucleotides that are not whole codons make the clone out of frame, with
  no amino-acid junction; a `*` is a stop codon. Lower-case letters (imputed from the germline)
  are read as upper case.
- **Counts.** Molecules (`uniqueMoleculeCount`) when every clone has them; otherwise reads
  (`readCount`, MiXCR 3's `cloneCount`, MiXCR 2's `Clone count`), with a finding when some clones
  had molecules. A clone's reads are kept beside its molecules. The `cloneId` is its sequence ID.
- **Single cell.** A table with `tagValueCELL` (MiXCR 4's single-cell exports, each row a clone's
  chain in one cell) is single-cell data: the cell is its barcode, the sequence ID the barcode
  and the clone's ID (`GAGTCC_clone_7`), MiXCR's clonotype of cells (`cellGroup`) the clone, and
  the dataset's unit cells.
- **The sample** is the file's name less the chain MiXCR 4 splits files by (`S1.clones_TRB.tsv`
  and `S1.clones_TRA.tsv` are sample `S1`; MiXCR 3's `S1.clonotypes.TRB.txt` too).

## OAS and OTS data units (Oxford Protein Informatics Group)

The data units of the Observed Antibody Space and the Observed T cell receptor Space, as their
search pages give them (`.csv.gz`), CC BY 4.0.

- Recognized by their first line: the unit's metadata, a JSON object written as one CSV field.
  The second line is the header: IgBLAST's AIRR columns.
- **Metadata** (`Run`, `Subject`, `Species`, `BSource` or `TSource`, `BType` or `TType`,
  `TSubtype`, `Disease`, `Vaccine`, `Treatment`, `Age` …) are kept as the sample's, as given (a
  zero-width space some links hold dropped). **Fill from metadata** in the Data view offers the
  subject, the tissue (`BSource`, `TSource`), the cells sorted (`BType`, `TType` with `TSubtype`)
  as the cell subset and the disease as the condition; `no`, `none` and `unknown` say nothing.
- **Unpaired units** (OAS): one row per sequence of a bulk repertoire, without `sequence_id`,
  counted by `Redundancy` (the times the sequence was seen, which the unit does not say are reads
  or molecules: the user confirms the unit).
- **Paired units** (OAS's `_heavy` and `_light`, OTS's `_beta` and `_alpha` columns): one row per
  cell, read as the cell's two chains. The cell is its contigs' barcode (their `sequence_id` less
  `_contig_n`, 10x Genomics' names, which the units keep), else the row's line; each chain counts
  one, and the dataset's unit is cells.
- The locus is written as one letter (`H`, `K`, `L`; `B`, `A`), booleans as `T` and `F`, and
  integers as decimals (`214.0`). The sample is the file's name.

## tcrdist3 clone tables

The clone tables tcrdist3 reads (`dash.csv`, the epitope-specific TCRs of Dash et al. 2017, among
them), comma- or tab-separated.

- Recognized by a chain's junction and V gene in tcrdist3's naming: `cdr3_b_aa` and `v_b_gene`
  (chains `a`, `b`, `g` and `d`: TRA, TRB, TRG and TRD). Each chain's `j_b_gene` and nucleotide
  junction (`cdr3_b_nucseq`) are read where given; tcrdist3's CDR3 is the IMGT junction.
- **Paired chains** are single-cell data: a clone's `count` is its cells, and the clone is read as
  that many cells with the same chains (`tcr7#1`, `tcr7#2`; a count that is not a whole number
  from 1 to 100,000 is one cell, with a finding). The `clone_id` is each cell's clone.
- **One chain** is a bulk repertoire: each clone a rearrangement counted by `count`, whose unit
  the user confirms (tcrdist3 counts cells, templates or reads alike); without `count`, each clone
  counts once.
- **The sample** is the clone's `subject`, one for each subject and `epitope`
  (`mouse_subject0050_PA`), the subject and epitope kept as its metadata (Fill from metadata
  offers the epitope as the condition); without `subject`, the file's name.
- A junction is productive when its amino acids hold no stop codon and its nucleotides, where
  given, are whole codons.

## Tables with mapped columns

A CSV or TSV that no reader recognizes (VDJtools' clone tables, a spreadsheet) is read
through a **mapping** of its columns, made in the window
([the guide](https://robert-mcdermott.github.io/clonoloom/docs/opening-data.html#mapping)) or given to remote control's `open_data`
([REMOTE-CONTROL.md](REMOTE-CONTROL.md)).

- **Separator**: a tab if the header has one; otherwise a semicolon if the header has more
  semicolons than commas; otherwise a comma. Fields may be quoted as CSV quotes them.
- **Fields**: `junction_aa`, `junction`, `v_call`, `d_call`, `j_call`, `c_call`, `count`,
  `sample`, `locus`, `productive`, `cell_id`, `clone_id` and `sequence_id`, each from one column.
  A junction (amino acids or nucleotides) is required.
- **The guess**: each field is guessed from the columns' names, compared without case, spaces or
  punctuation. For example, `cdr3aa` and `aaSeqCDR3` give `junction_aa`; `cdr3nt` and `nSeqCDR3`
  give `junction`; `v` and `allVHitsWithScore` give `v_call`; `count`, `cloneCount` and `reads`
  give `count`. A bare `CDR3` or `junction` column is amino acids or nucleotides, as its values
  look.
- **The count's unit** (reads, UMIs, templates or cells) is said with the mapping, never taken
  from the file. It is guessed from the count's name: `umi` or `molecule` gives UMIs, `template`
  gives templates, `read` or `duplicate` gives reads. Without a count, each row is one sequence.
- **The sample**: from its column, or one name for every row (the file's name by default).
- **Missing values**: empty, `NA`, `NaN`, `null`, `none`, `-`, `.` and `n/a`.
- **Productive**: as its column says (`T`/`F`, `true`/`false`, `yes`/`no`, `1`/`0`, `in`/`out`,
  `productive`/`unproductive`, `functional`/`nonfunctional`). With no such column, a rearrangement is productive when its junction is whole
  codons without a stop codon.
- **Check**: before a mapping is used, the whole file is read through it. For each field, ClonoLoom
  reports how many values are valid, with examples of those that are not, and notes: the share of
  junctions that begin with C and end with F or W, the share of nucleotide junctions that are whole
  codons, and how many counts are not whole numbers.

The mapping is part of the dataset: it is kept in the project, makes part of the dataset's ID (one
file read through two mappings is two datasets), and is stated in the methods paragraph.

## Gene calls

Calls are read as IMGT names: locus, segment, family (IMGT's subgroup), gene and allele
(`TRBV5-1*01`). Shared TRA/TRD genes (`TRAV29/DV5`), orphons (`TRBV20/OR9-2`), duplicated genes
(`IGKV1D-39`, `IGHV1-69D`) and heavy-chain constant genes (`IGHM`, `IGHG1`; `IGHD` alone is the δ
constant gene) are recognized. MiXCR's scores (`TRBV5-1*00(1250)`) are dropped. A call naming
several genes keeps them all: a clonotype definition at gene level keys it by its distinct genes
(`TRBV6-2|TRBV6-3`), so ties never merge clonotypes silently.

## What ClonoLoom writes

### Exports

**Export** writes a zip named `<dataset>-<what>.zip` holding the files and a `NOTICE.txt`.

| What | Files |
| --- | --- |
| Rearrangements | `rearrangements.tsv` |
| Clonotypes | `clonotypes.csv` |
| A diversity result | `diversity.csv` |
| A rarefaction result | `rarefaction.csv` |
| A gene usage result | `usage.csv` |
| An overlap result | `overlap.csv`, `public.csv` |

**`rearrangements.tsv`** is an AIRR Rearrangement TSV of the rows the dataset's filter keeps, with
these columns:
- the schema's required fields: `sequence_id`, `sequence`, `rev_comp`, `productive`,
  `vj_in_frame`, `stop_codon`, `v_call`, `d_call`, `j_call`, `sequence_alignment`,
  `germline_alignment`, `junction`, `junction_aa`, `v_cigar`, `d_cigar` and `j_cigar`;
- `locus`, `c_call`, `duplicate_count` (reads), `umi_count`, `templates`, `cell_id`, `clone_id`
  and `repertoire_id`.

Retained AIRR sequences, alignments, orientation and CIGARs are filled as supplied; optional
evidence fields are appended when available. Other readers leave them missing. `clone_id` is the row's clonotype under the definition. `templates` holds an
immunoSEQ export's templates, which the schema has no field for. The file reads back into the same
clonotypes.

**`clonotypes.csv`** has one row per clonotype:
- `clonotype_id`;
- for each chain, `chainN_locus`, `chainN_junction`, `chainN_v` and `chainN_j`;
- `total` (the abundance over the samples) and `samples` (how many hold it);
- one column of abundance per sample, empty where the sample does not hold it.

**Results** are written one row per value, with its 95% interval (and its standard error, for
diversity and overlap):
- `diversity.csv`: sample, measure, estimate;
- `rarefaction.csv`: each curve's points by size and by coverage, and the levels compared;
- `usage.csv`: each share by sample or condition, segment, level, length and IMGT position;
- `overlap.csv`: each pair of samples and each measure;
- `public.csv`: the public clonotypes, and how many clonotypes are found in one subject, two and
  more.

Numbers are written in their shortest exact form; a value that does not exist is empty.

**`NOTICE.txt`** says what the export holds and where it came from:
- the dataset, its ID, samples and unit;
- each file with its SHA-256;
- the filter, the clonotype definition and its ID;
- the method, ClonoLoom's version and commit, and when it was made;
- each public data set's license and citation.

Rows of public data under a provider's terms of use (or no stated license) are exported only after
the window shows their terms and you accept them. The notice then says not to share them.

### Figures

Each plot is saved on its own:
- **SVG**: vector, with its fonts named and every color written in;
- **PNG**: at 300 dpi, the resolution recorded in the file;
- **PDF**: one page of the figure's size, holding the PNG.

A figure is the plot on white, with its title, its legend and a caption. The caption says what was
measured, the clonotype definition, the unit, the samples, the filter and ClonoLoom's version.

## Development data

The public files the readers are checked on, their licenses and how to fetch them are listed in
[validation/sources.json](../validation/sources.json); `node validation/fetch.mjs --list` prints
them. `node validation/fetch.mjs --ci` fetches the openly licensed ones.

### Reconstructed B-cell germlines (wave 5, slice 5)

Germlines uses saved family membership, explicit allele calls, `sequence`, `rev_comp`, and
V/J AIRR CIGARs against a selected ungapped V/J reference. Supplied sequence and germline
endpoints must agree with CIGAR offsets. Native IMGT input, gene-only calls and missing CIGARs
remain inspectable but are excluded from reconstruction; no alignment or reference offset is
guessed. The chosen reference must match the annotation's segment boundaries.

The new projected alignment is distinct from retained `sequence_alignment` and
`germline_alignment`. CSV exports use `sequence_alignment`, `germline_alignment_d_mask` and
`segment_labels`; JSON retains per-column query and reference positions, the parent family
checksum and reference file records. Query coordinates refer to the oriented sequence;
reference coordinates reset for V and J. Zero marks absent mappings. D/N/P junction bases
are masked, including intervening light-chain bases. FASTA exports successful projections,
with stable source-row IDs; CSV includes all exclusions. These result exports are not full
AIRR input round trips. Mutation counts are a separate SHM result.

### Somatic hypermutation (wave 5, slice 6)

SHM consumes a saved germline artifact, with its checksum and reference provenance. Membership
is fixed by that parent. Each source chain row is retained, including exclusions and duplicate
contigs; counts are not weighted by reads, UMIs or cells. `shm.csv` has one row per source row
and region; `summaries.csv` pools counts by sample/locus/region. These are descriptive totals,
not independent replicates. The complete `shm.json` and manifest accompany the export.

A substitution is an unequal paired A/C/G/T base. Its frequency is substitutions divided by
paired A/C/G/T bases, or missing when there are none. Masked D/N/P columns, gap columns and
ambiguity columns are counted separately, in that order. Missing regional evidence produces
blank CSV statistics and an explicit JSON `unavailable` reason, not an observed zero.

`all`, `V`, `J` and individual `fwr1`, `cdr1`, `fwr2`, `cdr2`, `fwr3`, `cdr3`, `fwr4` entries
overlap and must not be added together. Regional intervals refer to the oriented raw query,
not the alignment or reference. Missing intervals stay missing. Incomplete, reversed,
out-of-bounds, overlapping or out-of-order intervals invalidate the regional partition while
whole-alignment and V/J counts remain available. Uncovered bases remain in the totals; deletion
gaps have no query-region assignment. CDR3 may include reference-covered V/J bases in real
inputs; only the saved D/N/P mask determines which bases are excluded from comparison.

Codon phase requires `vj_in_frame=T`, valid CDR3 bounds, a multiple-of-three CDR3 length,
and exact agreement between the query span including the flanking codons and `junction`.
The junction must translate without ambiguity/stops and retain the C and F/W anchors. Its
CDR3 start anchors query phase; no phase is inferred from reference position 1. Any alignment
indel suppresses codon inference for that row. Only complete paired A/C/G/T codons within one
V/J segment and, for a regional count, one region are evaluable. Stop-containing codons are
counted separately and excluded from the codon denominator, while their bases still contribute
to nucleotide comparisons. Truncated or ambiguous codons do not support R/S attribution.

Single-hit codons classify the sole substitution as replacement or silent using NCBI table 1.
Multi-hit codons retain endpoint replacement/silent **codon** counts; their substitutions stay
unclassified because their order is unknown. `unclassified = substitutions - replacement - silent`.
Codon exclusion reasons and the evaluable-codon denominator accompany classification counts;
zero classified substitutions without evaluable codons is not evidence for absence of SHM.
These are net observed differences, not cumulative events, evolutionary rates, selection tests,
or biological error correction. Reference choice and annotation accuracy remain prerequisites.

### Lineage trees (wave 5, slice 7)

`lineages.json` is a version-1 `clonoloom-lineages` artifact derived from checksum-verified
`germlines.json`. `lineageOptions` pins the parent hash and search limits; `germlineProvenance`
retains family and reference identities. `annotations` pins normalized sample timepoint/tissue
values. Subject/reference flags, active QC and clonotype settings do not alter this derived
analysis. Saved results can be inspected without the original input; new computation requires it.

`trees` contains each family/locus summary and its complete panel, graph and Newick. `columns`
is an ordered list of `{segment, position}` with 1-based ungapped V/J reference coordinates.
Only paired canonical bases present in every included row enter this complete-case panel.
`sites` is its length; `omitted` counts mapped V/J reference sites removed from the union, not
masked junction positions or query insertions. `taxa` counts distinct panel states including
the reference. `included` counts source rows, not abundance. Excluded groups retain reasons;
zero integer fields on excluded groups must not be read as measurements.

Nodes contain stable IDs, parent, kind (`reference`, `observed`, `inferred`), one chosen `sequence`
and marginal `possible` IUPAC states conditional on the selected topology. Both strings are
concatenated panel states, not full rearrangements. `changes` is the chosen assignment's edge
length; `distance` sums those changes from the reference; `depth` counts edges. Zero edges are
retained. `annotations` counts observed source rows by sample/isotype/timepoint/tissue. Inferred
nodes have no observed traits. `members` maps every parent source row to its tree and node or
exclusion, preserving source SHA-256, line, sequence ID and metadata. Identical panel states
collapse without losing distinct original rows; rows matching the root attach to it.

Search is exhaustive through eight taxa and a bounded addition/NNI heuristic through 64. Exact
`optimalTrees` and node `splitTrees` count tied optimal binary resolutions, not probabilities or
bootstrap support. Heuristic counts are absent, with `budgetLimited` marking early termination;
no global optimality claim is made. `examined` counts candidate scores. Limits and deterministic
choices are documented in research §5.19 and travel in the analysis identity.

The result ZIP contains `trees.csv`, `nodes.csv`, `members.csv`, `panel.fasta`, `trees.nwk`, full
`lineages.json`, `manifest.json` and `NOTICE.txt`. Tree scores are blank for excluded groups;
unknown tie counts are blank. FASTA gives the consistent chosen panel assignment. JSON retains
the coordinate map and possible states, also in node CSV. Newick uses stable node IDs and
integer change lengths, one tree per line; its root ID joins to nodes.csv and that tree's ID.
No class-switching, migration, event-date, selection or likelihood inference is encoded.

### Isotypes (wave 5, slice 8)

`isotypes.json` is version 1 of `clonoloom-isotypes`, derived from a checksum-verified,
thresholded `families.json`. `isotypeOptions` pins the family artifact and SHA-256;
`familiesOptions` and `familyFilter` record its choices. `annotations` pins sample timepoint
and tissue labels; observation subject/scope comes from the parent. Parent-retained C calls come
from the source dataset, without requiring germline alignments or a reference download.

`observations` retains each parent observation, its retained source rows, raw C calls,
class, constant gene, issue, exclusion and abundance. Class and gene can resolve differently:
IGHG1/IGHG2 ties give IgG with an unresolved gene. IGHG/IGHA without a subclass resolve only
class. All retained heavy calls must agree at the reported resolution; missing, unsupported
or conflicting calls remain explicit. Light calls are provenance only. Alleles collapse;
pseudogenes and unsupported labels do not become functional isotypes. See research §5.20
for supported labels, the 4,096-byte call bound, numeric-allele/finite-score suffix checks
and issue precedence.

`usage` has sample totals (`family` empty) and family/sample totals at `class` and `gene`
resolution. `observations`, `total` and `share` count assigned observations, including unresolved
calls. Parent exclusions are outside this denominator. Single-cell parents count one per
sample-scoped cell; bulk parents count one per heavy row. Only observed buckets are emitted.
`unit` describes observations (cells or heavy-chain rows); `abundanceUnit` separately retains
the parent count unit (for example reads; empty means unspecified). CSV usage includes both units.
`abundance`, `totalAbundance` and `abundanceShare` are separate: missing counts propagate to
bucket and group totals; a known zero denominator has a null share. Missing-count coverage
is retained. No metadata group is treated as a biological replicate.

`families` lists coverage, samples and resolved labels. `pairs` contains unordered constant-gene
pairs, counted once per family, alongside all-family and any-resolved-gene coverage counts.
These overlapping pair counts are not additive, switching events or transition estimates.
IgM/IgD-only co-occurrence is explicitly not switching evidence. Missing/ambiguous observations
cannot establish which events occurred or did not occur.

Exports include `usage.csv`, `families.csv`, `cooccurrence.csv`, `observations.csv`, `sources.csv`,
full `isotypes.json`, `manifest.json` and `NOTICE.txt`. They contain the complete artifact,
independent of displayed filters or pages. CSV missing numeric values are blank; measured zero
is `0`. `sources.csv` retains every parent-retained chain without duplicating observation abundance.
Saved inspection does not require the input or parent files; exporting follows normal source
access and data-use terms. Versioned isotype choices and normalized annotations travel in the
analysis identity; unrelated active QC, clonotype and diversity settings do not.

## 10x feature-barcode MEX (wave 6, slice 1; development)

Expression is separate from rearrangement tables. **Cells** reads a selected MEX directory:
`matrix.mtx[.gz]`, `features.tsv[.gz]` and `barcodes.tsv[.gz]`, or legacy two-column
`genes.tsv[.gz]` in place of the three-column feature table. Plain/gzip may be mixed; two
candidates for one role are ambiguous and rejected. No recursive raw/filtered choice is made.
The Matrix Market header must be `%%MatrixMarket matrix coordinate integer general`.
Features are rows, barcodes columns, coordinates one-based and values nonnegative integer UMIs.
Source order and feature type are retained. Names may repeat; IDs, barcodes and coordinates may
not. Explicit zeros are retained; missing coordinates within the matrix are zero, whereas a
barcode absent from the matrix has no measured expression. Gene-expression totals include only
`Gene Expression` features; with no such features, gene measurements are null, not zero.

This reader follows the [10x MEX specification](https://www.10xgenomics.com/support/software/cell-ranger/latest/analysis/cr-outputs-mex-matrices)
and [NIST Matrix Market](https://math.nist.gov/MatrixMarket/formats.html). It requires UTF-8
labels and LF/CRLF lines, without a byte-order mark. Counts and per-cell gene-expression totals
must be at most 2^53−1. It limits features to 200,000, barcodes to 2,000,000, coordinates to
10,000,000, label text to 64 MiB per companion, lines to 1 MiB, decompressed text to 512 MiB per
file and source bytes to 1 GiB per file. Unsupported encodings and corrupt/truncated data fail.
The source is streamed and fingerprinted read-only; a project saves its paths and SHA-256 values.
Reopening verifies identity. Barcode links require an explicit receptor dataset/sample and keep
suffixes exactly. Matching counts raw source evidence before QC, never inferred clonotypes.
AnnData/MuData and 10x HDF5 are described below. See the development
[Cells guide](site/pages/docs/cells.html) and `testdata/expression-mex/` for a synthetic example.

## 10x feature-barcode HDF5 (wave 6, slice 3; development)

Cells accepts an explicitly selected HDF5 file in the same path field as a MEX directory.
The schema follows [10x's HDF5 documentation](https://www.10xgenomics.com/support/software/cell-ranger/latest/analysis/outputs/cr-outputs-h5-matrices):
`/matrix/shape`, `data`, `indices`, `indptr`, `barcodes` and `features/id`, `features/name`,
`features/feature_type`. Shape is features by cells; CSC pointers and indices are zero-based.
All eight datasets are required vectors. Older genome-named groups, molecule-info files,
AnnData and MuData are not this schema. Extra datasets/attributes are not interpreted as counts
or annotations. IDs, names and modalities are preserved, with the same exact-label, integer,
duplicate and population rules as MEX. Unsorted CSC indices are sorted with their counts;
duplicates fail instead of being summed. Optional `cells.tsv` remains an explicit selection.

The reader is implemented from the [HDF Group format specification](https://support.hdfgroup.org/documentation/hdf5/latest/_f_m_t3.html).
Its supported subset is deliberately explicit:

- Superblocks 0–3, 8-byte addresses/lengths, optional user block, no superblock extension or
  external file driver; object headers 1–2 and bounded continuation blocks.
- Classic symbol-table groups or compact hard links. Soft/external links are never followed;
  a required dataset behind one fails. Dense/fractal-heap groups are unsupported.
- Fixed-length ASCII/UTF-8 strings with declared padding, variable strings through bounded
  global heaps (added in slice 4), and full-width signed/unsigned 1/2/4/8-byte integers in either
  byte order. 10x counts remain integer datatypes; negative/floating counts fail. The additional
  numeric/attribute types supported for AnnData are described below. Compound/reference types
  and custom bit layouts remain unsupported.
- Compact and contiguous arrays; layout-v3 chunks indexed by v1 B-trees, including internal
  nodes. Deflate, shuffle and Fletcher32 are supported, with reverse decoding order and
  per-chunk masks. Metadata and Fletcher32 checksums are verified. Newer chunk indices,
  unknown filters, external/virtual storage and missing/unallocated data fail. An empty vector
  needs no allocation; a nonempty dataset is never silently filled from an HDF5 fill value.

Resource policies: 1 GiB per file; 128 MiB per decoded array/read/chunk; 512 MiB total decoded
arrays; 64 MiB total metadata and at most 4 MiB per metadata block; 2 GiB cumulative file reads
and 1,000,000 read operations; 4,096 objects and 4,096 links per group, 65,536 messages per object/tree nodes per tree,
64 continuation blocks, depth 32 and 262,144 chunks per array. Matrix limits remain 200,000
features, 2,000,000 cells and 10,000,000 stored coordinates; feature labels and barcode labels
are bounded at 64 MiB each. Counts and gene totals remain at most 2^53−1. Exceeding any bound
fails the input, without sampling or partial values. A 10x sparse matrix is never densified. For unsupported
encodings, supply the corresponding MEX files.

The entire file is hashed before and after random-access parsing; changed size, modification
time or bytes reject that read. The `matrix-hdf5` source records its path, size and SHA-256.
Identity uses `clonoloom-expression-hdf5/1`, source size and hash; relocation retains identity,
while another encoding of the same counts has a different source identity. Existing MEX and
annotation identity domains are unchanged. Project reopening checks the recorded identity.

`testdata/make-expression-hdf5.py` uses pinned h5py/NumPy/SciPy to write artificial fixtures
and independently reconstruct every value in `validation/reference/expression_hdf5.json`.
The compressed offline example has the same evidence as the MEX example. These are format
tests, not a biological expression model or an HDF5 conformance claim.

### Cell annotations and imported coordinates (wave 6, slice 2; development)

An explicitly selected plain/gzip TSV can accompany one expression matrix. The first column
is `barcode`; other columns are text annotations except reserved `map_x` and `map_y`, which
must occur together. Header order is preserved. Up to 32 annotation columns are supported.
Empty annotation fields mean missing; literal `NA` is a label, not a missing-value sentinel.
Coordinates are either both empty or a finite pair within ±1e12. Zero is a valid coordinate.
No quoting, escaped tabs, multiline fields or BOM; LF/CRLF and valid UTF-8 are accepted.
Labels must have no surrounding whitespace, NUL, tab or newline. Headers and barcodes must
be unique; malformed rows, partial coordinates and corrupt gzip fail the whole import.

The table joins by exact barcode to the selected matrix. All rows remain available, including
annotation-only barcodes. Missing rows, missing values and missing coordinates are distinct.
The map contains only expression barcodes with coordinates, at most 20,000; larger populations
fail explicitly rather than being sampled. The complete table remains pageable. Reader limits
are 2,000,000 rows, 4,000,000 annotation values, 64 MiB of text and the MEX line/source limits.

The selected source's path, byte size and SHA-256 hash are saved. The combined identity hashes
the MEX identity, annotation byte size and hash under `clonoloom-expression-cells/1`. Files are
never discovered implicitly, so adding an unselected file leaves existing MEX-only identities
unchanged. A changed selected annotation file fails saved reopening. Coordinates are imported
evidence; no UMAP, normalization, clustering or cell-type assignment is performed.

## AnnData and MuData (wave 6, slice 4; development)

The Cells path field also accepts `.h5ad` and `.h5mu`. Open **AnnData / MuData selections**
and supply the **Count matrix** (`X` or `layers/<name>`). MuData also requires an **Expression
modality**; optionally select **Supplied embedding**, **AIRR key** and **AIRR modality**.
Feature name/type columns are optional, explicit var-column choices. No count layer, modality,
embedding or AIRR key is guessed. The format metadata must identify an AnnData or MuData
object; a filename alone does not establish compatibility. The selected modality's obs and
var are authoritative; global MuData annotations/maps and obsmap/varmap are not substituted.

The schema follows [AnnData's on-disk format](https://anndata.readthedocs.io/en/stable/fileformat-prose.html)
and [MuData's embedded AnnData contract](https://mudata.readthedocs.io/en/latest/io/output.html).
Supported encodings/versions: anndata and MuData 0.1.0; dataframe, array, string-array and
categorical 0.2.0; CSR/CSC, nullable-integer, nullable-boolean, nullable-string-array and
awkward-array 0.1.0. Unknown selected encodings or versions fail.

- **Counts:** obs × var dense arrays or CSR/CSC. Validate all dimensions, pointer/index lengths,
  monotonicity, bounds and duplicate coordinates. Nonnegative integer datatypes and finite,
  exactly integral float32/64 values are accepted up to 2^53−1, including totals. Negative,
  fractional, NaN, infinite, boolean and unsafe counts fail. X can contain normalized data:
  explicit selection is essential, and integral values alone cannot prove preprocessing history.
  The UI says **counts**, not UMIs. Sparse zeros remain stored coordinates. Dense input is bounded
  to 10 million supplied coordinates, including zeros; zero values are implicit internally.
- **Obs/var:** declared `_index` and `column-order`, with unique nonempty indices; exact text,
  numeric, boolean, categorical and nullable values. Category code −1, nullable mask true and
  numeric NaN mean missing. Empty annotation strings also display as missing; literal `NA`,
  `None` and `Missing annotation` remain categories. Signed/unsigned integer annotation text
  remains exact, even beyond JavaScript's safe integer range. At most 64 obs/128 var columns (32 obs for the separate AIRR modality),
  4 million annotation values and 64 MiB label text per frame. Features use var index as ID;
  without selected name/type columns, names equal IDs and type is Gene Expression. Missing
  values in a selected feature-name/type column fail. The full variable columns remain inspectable.
- **Embedding:** one selected dense obsm array with exactly two coordinates per observation.
  Both NaN means missing; otherwise both must be finite within ±10^12. The selected path and
  source key are retained. Axis labels derive from `X_umap`/`X_tsne` when those source keys
  identify the map; other keys retain their own name with dimensions 1/2. This does not verify
  that a producer actually ran that method. Coordinates remain supplied evidence, equal-scale
  and unmodified; the existing 20,000-point display limit applies. No maps are concatenated.
- **AIRR:** [scirpy's cell-aligned `obsm` representation](https://scirpy.scverse.org/en/latest/data-structure.html),
  decoded through the [Awkward form/length/buffer contract](https://awkward-array.org/doc/main/reference/generated/ak.to_buffers.html).
  Select the key and MuData modality explicitly. Retain every selected scalar field and chain
  in source order, with missing fields distinct from empty text. Keep missing whole-cell records
  distinct from empty chain lists. **AIRR evidence** pages chains; **AIRR cells** pages all
  source observations, including no-chain and missing records. Cross-modality coverage uses
  exact barcodes in those selected modalities; it is observation coverage, not a count of
  productive/paired receptors. No suffix stripping, row-position matching, imported clonotype
  adoption or execution of Python objects occurs. The supported built-in forms are ListOffset,
  Record, Numpy scalar, Indexed/IndexedOption, Unmasked and Empty; optional indices use −1 for
  missing. Strings use uint8 character buffers. At most 16 form levels, 256 forms, 128 fields,
  1 million chains, 4 million field values and 64 MiB field text. Other selected forms fail.

The HDF5 subset above additionally supports inline attributes v1–3, scalar metadata,
rank-two arrays and classic B-tree chunks, IEEE float32/64, signed codes, exact integer labels,
and FALSE/TRUE boolean enums. Variable string heaps count toward metadata budgets. Arbitrary
enums, shared datatypes, dense groups/attributes, newer chunk indices and external/virtual
storage remain unsupported. Required unsupported data fails rather than becoming missing or
zero. AnnData requires its own observations; an external annotation TSV cannot be attached too.
Unselected layers, embeddings and analysis dictionaries are not read as evidence.

The `anndata` source records the entire file's path, size and SHA-256. Hashes before and after
parsing must agree. Identity uses `clonoloom-expression-anndata/1`, bytes and all selections;
relocation preserves it, changing selection or source does not. Saved reopening verifies it.
Existing MEX and 10x identities are unchanged. Python is only used for independent fixtures;
the distributed application remains a standard-library Go binary.

`testdata/make-anndata.py` writes and independently reads synthetic fixtures with pinned
AnnData 0.12.6, MuData 0.3.2, Awkward 2.8.12, h5py 3.15.1, NumPy 2.4.2, SciPy 1.17.0 and
pandas 2.3.3 and scirpy 0.25.1. It uses public APIs, with no donor data or third-party implementation source.
`validation/reference/anndata.json` records source hashes and independent counts and AIRR
values. The **Use AnnData example** button works offline; its `X_umap` coordinates are
artificial illustration points, not a computed UMAP or a biological validation dataset.

## Clonotype phenotype results (wave 6, slice 6; development)

`GET /api/phenotypes/{id}` returns an `artifact` manifest and a `result` document with
`format: "clonoloom-phenotypes"`, `version: 1`. It records expression and receptor source
fingerprints, the receptor dataset, explicit sample/subject/timepoint, map parent (empty for
imported coordinates), axes/source flag, methods and the `analysis` object. Analysis includes
normalized definition and ID, QC rules and content-derived `filterId`, effective size population,
inclusive breakpoints, annotation column, coverage, cells, clone coverage, phenotype groups and
clone/phenotype associations. The manifest also records the project's filter version if supplied.

Cells retain exact barcodes, supplied annotation values and coordinates (`null` when absent),
assignment/chain states, candidate IDs and a unique clone where available. A cell's unavailable
size is `null`, never zero. Clone coverage counts may legitimately be zero. Empty annotation
strings mean missing values; literal names are never interpreted as missing. Group expansion
shares are nullable; association rows include both integer denominators and their fractions.
Source axes and positions are copied, never combined across maps or samples. Result identities
pin the input, map checksum, dataset/sample, definition meaning, QC rules, metadata, options and
method version. [Full population and denominator rules](PHENOTYPES.md).

## Clonotype modularity results (wave 6, slice 7; development)

`GET /api/modularity/{id}` returns `{artifact, result}`. The result has
`format: "clonoloom-modularity"`, `version: 1`, its description, the complete `phenotypes`
parent document, computed `map` ID, `expressionExcluded` count and `analysis`. The manifest pins
both parent hashes, method/software versions and options. The phenotype parent retains raw
source fingerprints, exact sample binding, normalized definition, QC and supplied metadata.
The separate map artifact holds the weighted graph; export it too for independent recomputation.

Analysis contains effective `{permutations, seed}`, graph/eligible cell and edge counts, tested
family size, the BY harmonic factor, explicit rules, graph-row `cells`, sorted `clones` and one
`nulls` entry per tested size. Cells retain barcode, unique clone or empty string, and parent
assignment status. Each clone has `cells`, `observed`, `expected`, `score`, `p`, `adjusted`,
inclusive `tail` count and optional untested `reason`. Singleton and whole-population clones
have null expected/score/p/adjusted/tail fields; these are not zero-valued test results.

Nulls contain size, `Exhaustive` or `Monte Carlo` method, actual draw count, analytic expectation
and sorted `{edges, count}` histogram bins. Absent integer bins have zero frequency; observed
values can exceed all sampled null values. Monte Carlo tails use an added observed configuration,
not a fabricated extra histogram bin. Graph counts use induced, positive undirected edges once;
weights and UMAP coordinates are not test inputs. Identity includes both parent IDs/hashes,
normalized options and method version. [Full method and limits](MODULARITY.md).


## AnnData export (wave 6, slice 9; development)

Cells prepares a checksum-verified `.h5ad` from a saved phenotype result. `X` is raw integer
CSR with every input cell and feature modality; excluded cells retain their counts and have
NaN computed embeddings. Namespaced observation columns preserve exact sample-scoped receptor
assignments, chain status and clone sizes. Supplied annotations use categorical missing code −1;
UTF-8 values and original column-name mappings survive. Source and computed coordinates are
separate. This is ClonoLoom's versioned receptor-link contract, not scirpy's raw AIRR-chain form.
The file includes parent documents/manifests, source fingerprints, definitions, QC, sample
metadata, method citations and software provenance in `uns/clonoloom`. Download includes its
manifest and source NOTICE. See [ANNDATA-EXPORT.md](ANNDATA-EXPORT.md) for the complete schema,
independent round-trip validation, reading example and explicit resource limits.
