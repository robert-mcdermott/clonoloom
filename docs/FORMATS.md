# Formats

What ClonoLoom reads and writes, and how it reads each format. The research behind these choices,
with sources, is in [clonoloom-spec/research.md](../clonoloom-spec/research.md).

Every rearrangement table can be gzipped (`.gz`); ClonoLoom recognizes gzip by its first bytes, not by the
name. Lines may end in LF or CRLF, and a UTF-8 byte-order mark is ignored. A file's format is
decided from its header, whatever its name, except IMGT's multi-file bundle: its documented
filenames identify table roles and headers are then checked.

## What every rearrangement reader produces

Wave 9 development also reads [Cell Ranger detailed contig JSON](CELLRANGER-JSON.md),
with exact source-object inspection, explicit layout recognition and missing/mixed-chain limits.

[ADC metadata browsing](ADC.md) is separate from local format import: it previews explicit
bounded public queries and inspects exact responses without installing a dataset.

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

**Repertoire metadata** (`.json`, `.yaml`, `.yml`, optionally `.gz`) opens with the rearrangement tables:
an AIRR Data File containing `Repertoire: [...]`, a list of repertoire objects, or one object.
Annotation extraction supports the field shapes used in schemas 1.3–2.0; it is not full schema
validation. `Info.version`, when present, is shown as the supplied declaration.

**Data → Inspect AIRR metadata** lists record IDs, matched sample counts and match basis, with
original JSON and the source-file SHA-256 available for each record. Exact case-sensitive
`repertoire_id` matches take precedence. The legacy sample-name match applies only when a table
sample has no explicit ID. Matching uses the table reader's parsed IDs; AIRR TSV fields already
have surrounding whitespace trimmed. Metadata IDs that need trimming are refused. Unmatched
records remain available. Neither coverage nor a match
establishes MiAIRR completeness, ontology validity, biological independence or a particular
sample/data-processing record.

Original JSON object bytes retain unknown fields, numeric tokens, null, false and zero.
YAML uses a documented [bounded YAML 1.2 subset](AIRR-YAML.md): decoded record JSON and the
original YAML document are shown separately, both as text, without floating-point conversion. Summary
annotations (study, subject, species, sex, age, diagnosis, tissue, cell subset, time point,
template and HLA) are display conveniences; multiple processing values may be combined.
Conflicting explicit single-cell flags do not become one boolean. The source hash covers the
whole captured file, including its wrapper; for gzip, it covers the compressed bytes. Inspection
uses the opened snapshot. It does not reread a subsequently edited source.

Ambiguous duplicate JSON keys or repertoire IDs within/across files, invalid UTF-8/unpaired
Unicode escapes and non-string, empty, padded or control-character repertoire IDs are refused.
These are ClonoLoom import rules. Limits: 64 MiB per source/decompressed file and combined decoded
metadata, 100,000 repertoires per file/dataset, nesting depth 64, 1,000,000 JSON values per file
and 4,096 bytes per ID. Original-record previews over 4 MiB are refused explicitly, without
truncating imported evidence. Lists are paged. Additional YAML syntax/work limits are documented in the linked guide. The
Clone/Cell/Receptor support is described below. See the [AIRR Repertoire standard](https://docs.airr-community.org/en/v2.0.0/datarep/metadata.html).

Wave 9 slice 3 adds a [MiAIRR reporting checklist](MIAIRR.md) for declared AIRR 2.0 metadata.
It distinguishes essential/important/defined levels, absent keys and explicit missingness,
with conditional review, evidence paths and remedies. Reporting completeness, schema/ontology
conformance and scientific validity are distinct. See the guide for supported versions,
sets 1–5 coverage, unassessed set 6 and bounds; no compliance certificate is issued.

### AIRR Clone, Cell and Receptor objects (wave 9 development)

Open AIRR 2.0 Data Files alongside rearrangements, including supporting Node objects and
mixed Repertoire collections. **Inspect AIRR objects** shows exact source objects and explicit
link states. Imported memberships, counts and pairing flags remain source assertions; they do
not change the active clonotype definition. Read the [supported contract and scientific
limits](AIRR-OBJECTS.md), including version requirements, identity/cardinality checks, receptor
hash validation, unresolved references and the bounded YAML subset.

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
  no filtered file. Wave 9 slice 6 adds detailed JSON as the fallback when neither CSV exists.
  `clonotypes.csv`, `consensus_annotations.csv` and `metrics_summary.csv` are
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
- **Detailed JSON (wave 9 development).** The [nested contig contract](CELLRANGER-JSON.md)
  retains original objects for inspection. Missing flags remain unknown and mixed-chain
  annotations remain unresolved; producer version is not inferred. Do not combine alternate
  representations of the same contigs as additional observations.

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

Wave 9 slice 5 adds native [TRUST4 reports and VDJtools tables](NATIVE-REPORTS.md), with separate
marker conventions, unknown units where required, exact source-row inspection and explicit
count/frequency limits. AIRR exports keep the AIRR reader; saved mappings keep their interpretation.

A CSV or TSV that no reader recognizes (a custom clone table or spreadsheet) is read
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

## 10x feature-barcode MEX (0.6.0)

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
AnnData/MuData and 10x HDF5 are described below. See the
[Cells guide](site/pages/docs/cells.html) and `testdata/expression-mex/` for a synthetic example.

## 10x feature-barcode HDF5 (0.6.0)

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

### Cell annotations and imported coordinates (0.6.0)

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

## AnnData and MuData (0.6.0)

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

## Clonotype phenotype results (0.6.0)

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

## Clonotype modularity results (0.6.0)

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


## AnnData export (0.6.0)

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

## Cohort design snapshots

Projects may contain `cohortDesigns`: named, numbered versions of a checked two-group
subject design check (passing or failed). Failed checks retain `ok: false`, all findings and
annotations, and are unavailable as analysis inputs. Each record binds its dataset identity, source and metadata fingerprints,
save time, and a complete `clonoloom-cohort-design` version 1 result. Results contain
canonical inputs, a SHA-256 design ID, every sample's disposition, distinct-subject and
sample counts, findings, descriptions and citations. Excluded samples retain annotations
and reasons. Structural checks do not perform statistical inference or pool samples.
The JSON export preserves the entire record; it is not a fitted model or an AIRR file.
Old projects without this optional field continue to open. Source files are needed for new
checks but not to inspect a saved snapshot. Group levels are ordered reference/comparison.

## Subject feature tables

`cohort-features` artifacts contain `cohort-features.json`, format
`clonoloom-cohort-features` version 1. `selection` names the project design record,
name, revision, canonical design ID and full input; `design` is its server-rechecked
snapshot. `parent` is the complete saved diversity document; `parentManifest` preserves
its lineage. `table.catalog` lists the ordered 13 features with IDs, names, units and
formulas. `table.rows` contains each included sample's full design annotations, group
and `values` in catalog order, each `{value, missing}`. Null is missing, never zero;
`missing` is empty for an available value. All excluded annotations remain in `design`.

The result manifest pins the design identity and diversity ID/SHA-256. Its durable job
spec records the complete selection, parent hash, method version and source NOTICE.
The result identity excludes only the NOTICE creation timestamp. New tables require an
opened source dataset and exact complete sample-set agreement; saved tables need neither
sources nor parents. The worker verifies the selected parent again before promotion.

`POST /api/cohort-features` accepts `{design: {record, name, revision, id, input}, diversity,
start?}`. `input` is the saved cohort input, not trusted cached checks. `start:false` probes
without starting a job. `GET /api/cohort-features/{id}` returns manifest and result;
`GET /api/cohort-features/{id}/download` produces the CSV, catalog, full result JSON,
manifest and NOTICE ZIP. Restricted provider terms require `accept=1`, as for other exports.
CSV is wide, one selected subject/group observation per row, with each numeric feature
followed by its `_missing` reason column. Units/formulas are in `catalog.json`; definition,
QC, input fingerprints and citations travel in the full JSON and NOTICE. No averaging,
pooling, depth standardization, imputation or group inference occurs here.

## Subject group comparisons

`cohort-compare` artifacts contain `cohort-compare.json`, format `clonoloom-cohort-compare`
version 1. `features` embeds the complete subject-feature document and `parentManifest` its
manifest. `analysis` contains normalized `options`, `layout`, ordered `levels`, `family`,
`tested`, method description and one `comparisons` entry per selected feature, in ID order.
Each entry retains feature ID/unit/formula, selected reference/comparison n, complete pair
count, missing count, every subject/sample/group/value, mean estimates, mean and probability
effects, and the parametric/rank tests. Effects retain their own units, limits, interval meaning
and unavailability reason. Tests retain statistic, degrees of freedom, raw/adjusted p, reason,
randomization method, draws, tail count and nonzero-pair count. Inapplicable values are null;
rank-only counters are zero where unused. Intervals are marginal, not BH-adjusted.

`POST /api/cohort-compare` accepts `{features, options: {features, confidence?, permutations?,
seed?}, start?}`. Outer `features` is the feature-artifact ID; inner `features` is 1–13 distinct
catalog IDs. Confidence defaults to 0.95 (0.8–0.99), permutations to 9999 (999–99999), seed to
20261025 (positive integer at most 2^53−1; zero selects the default). The normalized options,
method version, parent ID and verified parent SHA-256 determine the `cc-` artifact identity.
The server rebuilds the parent table from its embedded measurements and checks the design.
No raw source or separate diversity artifact is required. `start:false` probes the result/job
without starting work; durable replay rechecks the parent hash before computing.

`GET /api/cohort-compare/{id}` verifies and returns manifest and result. The `/download`
suffix returns `comparisons.csv` (two test/effect rows per selected feature), the complete
`subject-features.csv`, full comparison JSON, manifest and NOTICE. Source provider terms
require `accept=1` when restricted. Saved reads/exports need neither sources nor parents.
Unavailable numbers become blank CSV cells with reason columns, never zero. The full JSON
and NOTICE preserve design/exclusions, definition, QC, source fingerprints and method citations.
The family is exactly 2K slots; untestable p=1 placeholders affect BH but remain null in output.
Research §5.32 specifies the hypotheses, effect estimands, interval derivation and bounds.

## Replicated clonotype count models

`cohort-counts` artifacts contain `cohort-counts.json` (format `clonoloom-cohort-counts`,
version 1). They embed the checksum-pinned subject-feature parent and manifest, canonical design
matrix/column names/contrast, assigned totals/log offsets, all observed clonotype counts and CPM,
filter eligibility/reasons, full/null fits and covariance, dispersion, log2 effects/Wald intervals,
LRT/raw/BH p-values, family size, effective options and method. Missing inference is JSON null
or blank CSV, with a reason; it is never replaced by an observed zero or fabricated p-value.

`POST /api/cohort-counts` accepts `{features, options: {minCount?, minSamples?, adjustBatch?,
confidence?}, start?}`. Defaults are 10, 12, false, 0.95. Creation requires the original dataset
open to capture counts under the parent's exact definition/QC; integer source members are
checked before aggregation. Missing retained counts refuse capture. Count totals must equal
the parent's assigned abundance. A `start:false` probe returns identity/readiness without
fitting. Job identity hashes the complete captured matrix, parent checksum, effective options
and method version. The durable worker rechecks its parent and operates on that matrix without
source reads. Saved results and exports need neither sources nor separate parent artifacts.

`GET /api/cohort-counts/{id}` verifies the result. `/download` returns `comparisons.csv`,
`design.csv`, `counts.csv`, full JSON, manifest and NOTICE; restricted provider terms require
`accept=1`. CSV comparisons retain every filtered and failed clonotype; counts retain all
assigned denominators. Design CSV lists sample, subject, group, batch, library total, log offset
and every design column; JSON retains the contrast, exclusions, definition, QC and count unit.
This is a bounded, two-group, unmoderated NB2 model with subject blocks for paired studies and
optional batch; the exact statistical contract and limits are in research §5.33 and the
[replicated count guide](site/pages/docs/cohort-counts.html).

## Definition robustness

`definition-robustness` artifacts store `definition-robustness.json`, format
`clonoloom-definition-robustness`, method version 1. The document embeds its entire
checksum-pinned `cohort-compare` parent and manifest. Each declared variant retains the
normalized definition, exclusions, captured positive abundance vectors for every source sample,
subject feature table, cohort result and across-audit BY values (null when unavailable).
Failed alternatives keep their reason and family slots; baseline failure aborts.

`POST /api/definition-robustness` takes `{comparison, definitions, start?}`; `definitions`
contains 1–5 alternative definition objects and the saved baseline is prepended. Only
junction/V/J resolution may vary; normalized policy changes and duplicates are refused.
The original dataset must be open initially. Source counts must reproduce baseline features
exactly; the immutable worker also reproduces baseline inference. The `rb-` identity hashes
parent ID/checksum, declared definitions, captured evidence and method version. Replay uses
that evidence plus the comparison parent, not raw files or feature/diversity grandparents.
GET `/api/definition-robustness/{id}` verifies the artifact checksum. `/download` returns
`sensitivity.csv`, complete JSON/manifest, per-definition feature CSVs and inherited NOTICE;
provider terms require the established acceptance flow.

The CSV keeps all definition/feature rows, both tests, raw p, within-definition BH,
across-audit BY, effects/intervals, failures and descriptive summaries. Missing numbers are
blank in CSV and null in JSON. Two tests × selected features × all definitions form the BY
family, including unavailable slots. Marginal intervals are not multiplicity-adjusted;
summary ranges are descriptive. No inference treats definitions as independent replications.
Bounds and interpretive limits are in the [definition guide](site/pages/docs/definition-robustness.html)
and research §5.34. This format does not equate individual clonotype identities across definitions.

## Motif and sequence-signature screens

`motifs` artifacts store `motifs.json` (format `clonoloom-motifs`, method version 1).
`evidence` embeds the complete request, captured query/background TRBV-gene/full-AA-junction
units, source sample list, exclusions, NOTICE, abundance unit and method version. Each unit
retains its sample/subject/clonotype sets and descriptive summed abundance. Its identity is
shared across arms when the same V/junction occurs in both; membership is unique within an arm.
`analysis` retains options, both receptor arrays, shared-identity count, every matching stratum,
the full pooled signature family, scores and reporting decisions. Group member indexes refer
to their respective arm arrays. `start`/`end` are inclusive one-based motif-start windows;
for global signatures they are the single variable position, marked `%` in the full junction.
`allowed` contains its pairwise-positive BLOSUM62 residue set.

Probabilities and scores are JSON null when untestable. `logP`/`logQ` are natural logarithms
and remain informative if `p`/`q` underflow to zero; `score` is −log10(BY). A zero background
match count gives null fold and an explicit reason. Untestable family slots remain present.
The complete family is defined before reporting thresholds, including background-only groups.

`POST /api/motifs` takes `{dataset, definition, filter, query, background, subjects?, options?,
start?}`. Query/background are disjoint source sample names. Options default to
`{minK:2,maxK:5,minMembers:3,q:0.05,minFold:2}`. `start:false` captures and probes readiness
without submitting a job. Verified GET `/api/motifs/{id}` returns the manifest and document;
`/group/{group}` returns the signature and Go-computed residue counts/frequencies.
`/download` supplies `motifs.json`, `manifest.json`, complete `signatures.csv`, `receptors.csv`
and NOTICE, preserving provider-term acknowledgement. CSV unavailable numbers are blank.

Creation needs source evidence; saved inspection, export and durable replay do not. Captured
request/arm/subject membership is revalidated in workers. Bounds, exact GLIPH2 adaptations and
conditional scientific limits are in [the guide](site/pages/docs/motifs.html) and research §5.35.

## Subject recurrence and HLA artifacts (wave 7, slice 7)

`POST /api/convergence` takes `{motifs, selection}`. `motifs` is a checked saved motif
artifact ID; selection contains `species:"human"`, explicit `samples`, `subjects`,
`groups` and `alleles`. Each subject is `{id,samples,typing:{A:[call,call],B:[call,call]}}`;
each sample belongs to exactly one named subject. Groups are 1–128 saved signature IDs.
Alleles are 0–64 declared targets; an empty panel requests descriptive recurrence only.
Use two explicit copies per tested locus, including duplicated homozygous calls. Calls are
compared at two fields. Unresolved typing is retained as unknown, never inferred non-carriage.
See [research §5.36](../clonoloom-spec/research.md#536-subject-recurrence-and-hla-association-wave-7-slice-7).

`start:false` validates capture and probes readiness without submitting a job. The result ID
is `cv-` plus 24 hex digits of the canonical captured-spec SHA-256. Samples/subjects sort
canonically; group and target order remain declared. Target names normalize to two fields;
absent/empty typing arrays represent the same unknown state. Nonempty raw calls are retained.
The spec pins the parent manifest, checked parent result hash and method, source notice,
selected membership and all eligible receptor/sample identities. It does not recalculate or
combine motif scores. Durable replay needs neither original sources nor the parent directory.

`GET /api/convergence/{id}` returns checked artifact metadata and `clonoloom-convergence`
version 1 JSON. Analysis retains every subject, typed-locus status/reason, observed group and
exact receptor recurrence, all declared group × allele slots, counts, frequencies, marginal
Wilson bounds, descriptive differences, p/BY and natural-log probabilities. An unavailable
number is JSON null. Each locus has its own complete typed/receptor-evaluable universe,
including group-negative subjects. Untestable slots stay in the BY family with p=1 internally
and null output; no displayed reporting subset changes multiplicity.

`GET /api/convergence/{id}/download` writes `convergence.json`, `manifest.json`, `subjects.json`,
`associations.csv`, `recurrence.csv` and `NOTICE.txt`. CSVs retain complete rows and reasons;
unavailable numeric fields are blank. The subject declaration includes raw HLA typing and
identifiers. Provider terms require the existing `?accept=1` acknowledgement. The result and
its source/definition/QC provenance are checksum-verified before reading or exporting.

## Contamination-evidence artifacts (wave 7, slice 8)

`POST /api/contamination` takes `{dataset,species:"human",unrelated:true,samples,filter,options,pgen}`.
Samples are 2–64 explicit `{name,subject,batch}` records; batch is optional and repeated samples
reuse a subject ID. The declaration states that distinct IDs represent unrelated subjects.
Options default to `{minCount:2,highFraction:0.01,ratio:20,highPgen:1e-8}`. An optional `pgen`
is a compatible saved artifact ID, not an arbitrary probability mapping. `start:false`
validates capture and probes readiness without submitting a job. Unknown JSON fields and
multiple top-level objects are refused.

The fixed identity is productive human bulk TRB, full nucleotide junction plus resolved V/J
genes; the project definition is unchanged. Source QC is captured. Counts must be known
nonnegative integers in reads, UMIs or templates. Retained productive TRB abundance supplies
each sample's denominator, including rows with unevaluable nucleotide/gene evidence; reasons,
retained rows and eligible abundance accompany it. Missing counts refuse capture. Single-cell,
sequence-only and unknown-unit sources are outside this version.

A `cnt-` content identity pins the canonical assignments, thresholds, source notice/definition,
captured count matrix and optional Pgen manifest, checked hash and model metadata. Pgen parents
must match dataset, QC, human organism and productive NT TRB V/J-gene definition. Their joint-
gene setting remains explicit. Missing probabilities are null, distinct from model zero.
No probabilities are inferred from sharing or supplied directly by a caller.

`GET /api/contamination/{id}` returns checksum-verified `clonoloom-contamination` version 1
JSON, with captured evidence, every cross-subject pair and every positive shared identity.
Same-subject pairs are counted as excluded. Pair totals, fractions, review counts, high/unknown
Pgen counts and missingness reasons remain complete. Review labels never remove matches.
The capture is bounded to 5000 eligible identities and 200,000 shared pair/identity records;
oversized jobs fail rather than truncate. No p-value or contamination probability is returned.

`GET /api/contamination/{id}/download` supplies `contamination.json`, `manifest.json`,
`samples.json`, `matches.csv`, `pairs.csv` and `NOTICE.txt`. Complete CSVs retain identifiers,
NT/AA, counts, fractions, ratios, review/model-context labels and reasons; missing numeric
fields are blank. The JSON also retains all private/unshared eligible identities. Subject IDs
and sequences are included; provider terms use the existing `?accept=1` acknowledgement.
Reopening and durable replay do not need source files or the Pgen parent directory.

### Batch QC (`batch-qc`, wave 7 slice 9)

`POST /api/batch-qc` accepts `dataset`, the saved QC `filter`, all `samples` as
`{name,subject,batch,exclude}`, and `options:{threshold:3.5,minSamples:5}`.
An empty exclusion reason includes the sample; a nonempty reason preserves an explicit
exclusion. Every source sample must appear once. Included samples need subject/batch labels;
at most one sample per subject per batch is allowed. Bounds are 2–256 source samples and
32 included batches. Unknown fields and trailing JSON are refused. `start:false` checks
whether the computed artifact is ready without starting a job.

The immutable `bqc-…` identity includes normalized options/assignments, all captured metric
values and numerators/denominators, source QC/unit/fingerprints, provider notice and method
version. `batch-qc.json` has format `clonoloom-batch-qc`, version 1, a cited `description`,
`evidence` and `analysis`. The analysis records the six fixed metric definitions, included
sample and unique-subject counts, exclusions, and every batch × metric reference and sample
observation. Reference medians/MADs use the declared transformed scale. A null transformed
value or score has an explicit reason. Read a JSON `flag` only with its score/reason:
unscored observations have a null score and do not constitute a below-threshold verdict.

`GET /api/batch-qc/{id}` verifies the saved result's checksum. Durable jobs replay from
captured metrics without original source files. `/download` returns a ZIP containing
`batch-qc.json`, `manifest.json`, `samples.json`, `observations.csv` and `NOTICE.txt`.
CSV contains every included sample × metric slot; unavailable numeric fields and unscored
flags are blank. Excluded samples and all denominators remain in JSON. Provider restrictions
use the existing 409/explicit `accept=1` flow. Subject IDs are retained; export is not
implicitly de-identified. No source sample or QC filter is modified by a flag.

## Pseudobulk count evidence (wave 7, slice 10)

`POST /api/pseudobulk` accepts `phenotypes` (1–32 explicit saved IDs), `options` with a
required supplied `annotation` column and `minCells` (default 10), and optional `start:false`.
Unknown fields and trailing JSON are refused. All parents must share one receptor dataset,
definition, QC, receptor size population and breakpoints. Original expression sources are
fingerprint-verified for each new capture. Feature IDs/names/types/order must match exactly.
No files, barcodes, cell types or samples are inferred from paths.

The `pb-…` artifact, kind `pseudobulk`, contains `clonoloom-pseudobulk` version 1:
`evidence` pins source/parent fingerprints, policies, subject/sample/time annotations, sparse
raw Gene Expression counts and cell inclusion reasons; `analysis` contains all sample × type
× bin groups, all feature sums, library totals, eligibility/reasons and cell denominators.
Absent groups have null counts/total. Measured zeros are numeric zero. Low-cell groups keep
raw counts but are ineligible. Counts and library totals are bounded at 9e15.

`GET /api/pseudobulk/{id}` verifies saved checksums. `/download` includes `pseudobulk.json`,
`manifest.json`, `cells.json`, complete `observations.csv` and `NOTICE.txt`. Every group × feature
is exported; unavailable counts are blank. Provider terms require `accept=1`. Reopening and
durable replay use captured evidence without source or parent files. Subject identifiers remain
present. This format makes no differential-expression, normalization or significance claim.

## Paired pseudobulk differential expression (wave 7, slice 11)

`POST /api/pseudobulk-de` accepts `pseudobulk` (saved count artifact ID), `options`, and optional
`start:false` for checked preparation without fitting. Strict JSON refuses unknown/trailing data.
Options: exact `type`, literal `timepoint`, different `reference`/`comparison` size-bin labels;
`exclude` maps sample names to reasons; `minCount` default 10, `minSamples` default 25,
`confidence` default .95. At least 25 complete pairs; no automatic missing-pair deletion.

The `pde-…` artifact, kind `pseudobulk-de`, contains `clonoloom-pseudobulk-de` version 1:
`evidence` binds the normalized request, count-parent manifest/checksum, dataset, raw gene
features/groups, sample annotations, minimum-cell rule, source description/NOTICE and method
version. `analysis` records all sample dispositions, paired observations, full design/contrast,
size factors and log offsets, normalization gene IDs, family/tested counts and all gene rows.
Rows retain raw/normalized counts, mean normalized count, eligibility/reason, log2 effect,
standard error, conditional interval, dispersion, LR, p/BH and successful full/null fits.
Unavailable inference is JSON null and CSV blank; measured zero counts remain zero.

New analyses verify the count parent but need no original sources. Durable replay uses captured
raw groups without parents or sources. `GET /api/pseudobulk-de/{id}` checks result checksums.
`/download` retains provider terms (`accept=1` only after acceptance) and includes
`pseudobulk-de.json`, `manifest.json`, `parent-manifest.json`, `raw-evidence.json`, `genes.csv`,
`design.csv`, `counts.csv`, `NOTICE.txt`. The raw evidence includes excluded groups and all
features. Selected counts and design CSV columns share the saved canonical observation order;
subject fixed-effect coding and comparison orientation are explicit. Cell membership remains
in the pinned count parent. This is unmoderated NB2 with median-ratio offsets, not the full
edgeR/DESeq2 pipeline; research §5.40 records assumptions, bounds and independent references.

## Evidence certificates (0.8.0)

A single supported cohort, batch-QC or motif result can be exported as `.certificate.clz`: a ZIP containing
`certificate.json` (format `clonoloom-certificate`, version 1), exact saved `manifest.json`,
artifact files under `artifact/`, provider NOTICE and interpretation limits. This is not a
project archive. Scope `recorded-artifact-bytes` and report `recomputed: false` explicitly
exclude scientific replay. See [CERTIFICATES.md](CERTIFICATES.md) for the canonical record,
fingerprint, size/path limits, integrity verdicts and native CLI. No source files are added,
but captured evidence can contain raw data and identifiers; provider restrictions still apply.

Wave 8 slice 2 keeps certificate v1 unchanged and adds a separate `clonoloom-replay` v1
report, scope `captured-cohort-evidence`. It embeds the integrity report, original manifest,
current verifier software/platform, findings and ordered step digests/metadata comparisons.
Only explicitly supported workers execute in a temporary store; exact output/metadata agreement
is distinct from source verification and biological validity. See [replay scope and verdicts](CERTIFICATES.md#exact-replay-boundary).

Wave 8 slice 3 adds `captured-batch-qc-evidence` and `captured-motif-evidence` replay scopes
without changing certificate v1 or existing cohort scopes. Headless recipe v1
(`clonoloom-run-recipe`) pins named input filenames, byte sizes and SHA-256, a selected task,
normalized parameters and an optional clock. The final `clonoloom-run` v1 record inventories
outputs and preserves completed/failed/differing/unavailable/cancelled outcomes. Neither is a
project archive. See [HEADLESS.md](HEADLESS.md) for exact fields, bounds, support and exit codes.


## Analysis checkpoints (0.8.0)

`clonoloom-checkpoint` v1 is local immutable evidence: original `project.json` plus the original
manifests and inventoried files of attached completed artifacts. Its `cp-<SHA256>` identity pins
a sorted inventory; source paths are never opened. `clonoloom-checkpoint-diff` v1 contains complete
raw before/after JSON strings beneath descriptive changes. Recovery restores analysis values and
exact attached result bytes, retaining current project identity/history. Neither format is a
certificate, portable archive or running-job state. See [CHECKPOINTS.md](CHECKPOINTS.md) for
bounds, version checks, conflict refusal, cancellation, numeric precision and replay limits.

## Self-checking HTML reports (0.8.0)

`clonoloom-html-report` v1 inventories exact embedded content, geometry source, stylesheet,
original result JSON and native certificates for 1–8 explicitly selected supported results.
Its scope is `embedded-report-bytes`; its SHA-256 fingerprint hashes the exact Go-encoded
inventory bytes. It is an offline HTML envelope, not a new certificate or scientific replay
format. Go renders exact numeric tokens in tables; the browser checks bytes and places existing
figure marks. Unsupported/oversized evidence is refused without row truncation. See
[HTML-REPORTS.md](HTML-REPORTS.md) for fields, limits, provider gates and the trusted-checker boundary.


## Portable project archives (0.8.0)

Version 1 `.clz` ZIPs retain original project/history, inventoried results and available parents,
checkpoint closure, notices and optionally permitted sources. A sorted SHA-256 inventory binds
all members; the record fingerprint excludes its own fingerprint field. Import preserves those
bytes under `imports/<fingerprint>` and creates a separate working project with verified local
source paths. No scientific computation runs. Unsupported paths/versions, missing mandatory
evidence, altered members and destination conflicts refuse import. See
[PROJECT-ARCHIVES.md](PROJECT-ARCHIVES.md) for the exact schema, bounds, source permissions,
checkpoint bindings and interruption limits.

## De-identification table exports (0.8.0)

Slice 11 adds `clonoloom-deidentified-tables` v1, an allowlisted ZIP of native scientific tables,
pseudonymized identities, explicitly retained sample annotations and fresh inventory/notice.
The separate `clonoloom-private-identity-map` v1 never enters that ZIP. Retained table values
match ordinary native exports under the same definition and QC; original scientific artifacts
and source files stay unchanged. Unsupported nested provenance refuses AIRR export. Existing
archives, reports and certificates retain their identities and are not transformed. See
[DEIDENTIFICATION.md](DEIDENTIFICATION.md) for exact fields, linkage/removal rules, bounds,
private review/download behavior and scientific/privacy limits.

## Typed Parquet dataset exports (wave 8, slice 12)

The [Parquet guide](PARQUET.md) defines the version 1 mapped native-table format. Ordinary
exports contain clonotypes or AIRR rearrangements, optional invariant sample denominators,
`schema.json`, NOTICE and a SHA-256 inventory. Nullable INT64 counts distinguish zero and
missing; conversion refuses fractions and native counts beyond 2^53−1. Native AIRR flags
become BOOLEAN; other evidence stays UTF-8 text, including invalid annotations. Original
identities, source provenance, effective definition/QC and units remain explicit.

Clonotype sample columns use stable positional keys plus a sample-name map. Invariant shares
retain DOUBLE native precision, full retained-abundance denominators and rule limitations.
Parquet footers repeat the corresponding schema and notice. This is not a fresh verification
of source files, a general result converter, a Parquet reader or a scientific replay certificate.

The [pseudonymized export](DEIDENTIFICATION.md) optionally accepts `storageFormat:"parquet"`: its
existing mapping/removal policy runs before conversion, text tables become typed files and
`schema.json` is added. Public footers contain sanitized provenance only; the private map
remains separate. Existing text output stays the default and preserves its v1 contract.

## Excel evidence workbooks (wave 9, slice 10)

`clonoloom-workbook.zip` contains `results.xlsx`, unchanged result/manifest JSON, provider notices,
native certificates and a version 1 `clonoloom-workbook` inventory with SHA-256 hashes and sheet
mapping. Five saved-result kinds are mapped. Every JSON node has an explicit type/path; each
scalar retains its original token as text. Numeric admission is conservative and never guesses
string types, dates or formulas. Null, empty and absent values remain distinct from zero.
See [EXCEL.md](EXCEL.md) for exact precision policy, bounds, provider acknowledgment, spreadsheet
editing limits and the distinction between node rows and biological experimental units.

## Prism subject-table interchange (wave 9, slice 11)

`clonoloom-prism.zip` contains data-only `data.pzfx`, a version 1 `clonoloom-prism-mapping`
record, unchanged result/manifest JSON, provider notice, native certificate and a version 1
`clonoloom-prism` inventory. Each recorded feature becomes a two-column, one-subcolumn Column
table. Paired subject identities align across groups; independent subjects occupy their own
rows. Null measurements and structurally empty positions remain distinct in the mapping, while
both are blank in PZFX; zero stays numeric. No analysis or graph template is generated. See
[PRISM.md](PRISM.md) for supported kinds, exact tokens, import precision, exclusions, bounds,
method differences and the explicit limits of native/schema validation.
