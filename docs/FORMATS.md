# Formats

What ClonoLoom reads and writes, and how it reads each format. The research behind these choices,
with sources, is in [clonoloom-spec/research.md](../clonoloom-spec/research.md).

Every table can be gzipped (`.gz`); ClonoLoom recognizes gzip by its first bytes, not by the
name. Lines may end in LF or CRLF, and a UTF-8 byte-order mark is ignored. A file's format is
decided from its header, whatever its name, except IMGT's multi-file bundle: its documented
filenames identify table roles and headers are then checked.

## What every reader produces

Whatever the format, ClonoLoom reads rearrangements into one table with these fields (named as
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

### Sequence evidence (wave 5, in development)

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
AIRR input round trips. No mutation count is calculated in this slice.
