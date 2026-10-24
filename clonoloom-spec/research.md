# ClonoLoom: methods, formats and data research

The facts ClonoLoom is built on: the formats it reads (checked against the files themselves and
the tools' sources), gene nomenclature, the methods it implements (with their publications), the
public data it develops and validates on, and the licenses of everything it touches. Facts were
checked in October 2026; each section says where they come from. A choice the sources leave open
is marked **ClonoLoom's choice**, with the reason.

## 0. Summary

- **Inputs, wave 1:** AIRR-C Rearrangement TSV (schema 1.3 to 2.0), Cell Ranger V(D)J contig
  annotations (Cell Ranger 3 to 9), immunoSEQ exports (v2 and v3). Every one of them, except the
  AIRR schema itself, calls the **junction** "CDR3": ClonoLoom keeps the IMGT junction internally
  (§3).
- **Gene names** follow IMGT nomenclature. Vendors' names are mapped by published rules, with
  the list of human genes taken from HGNC (CC0) to tell subgroups with one gene from those with
  several (§2).
- **Abundance** is reads, UMIs, templates or cells, and the formats do not always say which
  (§4).
- **Methods** are reimplemented from their publications and checked against reference
  implementations used only as validation tools (§5).
- **Data:** 10x Genomics V(D)J datasets (CC BY 4.0) and the AIRR Community's test files (CC BY
  4.0) for per-commit CI; immunoSEQ and AIRR Data Commons samples under their providers' terms
  for the maintainer's validation only; nothing real committed to the repository (§6).
- **Licenses:** no reference data under a non-commercial or no-derivatives license is bundled;
  germline sets, recombination models and specificity databases are downloaded on request with
  their license shown and recorded (§7).

## 1. Formats

### 1.1 AIRR Community Rearrangement TSV

Source: the airr-standards schema YAML at versions 1.3.1, 1.4.1, 1.5.1, 1.6.1 and 2.0.0
(https://github.com/airr-community/airr-standards; CC BY 4.0), and its test files. Vander Heiden
et al. 2018, *Front Immunol* 9:2206, doi:10.3389/fimmu.2018.02206.

- Tab-separated, one header line, no quoting, no comment lines. Missing values are empty
  fields. Booleans are written `T`/`F`; the reference libraries read `T t TRUE True true 1` and
  `F f FALSE False false 0`. Integers are base 10; positions are 1-based and closed.
- The fourteen required columns have not changed from 1.3 to 2.0: `sequence_id`, `sequence`,
  `rev_comp`, `productive`, `v_call`, `d_call`, `j_call`, `sequence_alignment`,
  `germline_alignment`, `junction`, `junction_aa`, `v_cigar`, `d_cigar`, `j_cigar`. Real files
  often lack some; ClonoLoom reads what it needs and the validator says what is missing.
- No Rearrangement field has been removed since 1.3. 2.0 (June 2026) adds `locus_species`,
  `rearrangement_type` and reactivity references.
- Calls list ties separated by commas (`TRBV6-2*01,TRBV6-3*01`).
- `junction`/`junction_aa` are the IMGT junction; `cdr3`/`cdr3_aa` exclude the anchors.
- Counts: `duplicate_count` (identical observations; from 1.4 defined as reads, earlier also
  allowed to count UMIs), `umi_count` (1.4+), `consensus_count` (reads behind a consensus).
  Cell Ranger's AIRR file still puts UMIs in `duplicate_count`.
- `v_identity` is a fraction by the schema; IgBLAST's own AIRR output, OAS and TRUST4 write a
  percent.
- Repertoire metadata (MiAIRR; Rubelt et al. 2017, *Nat Immunol* 18:1274,
  doi:10.1038/ni.3873) comes as JSON or YAML: `Repertoire` objects with `subject`, `sample`
  (a list), `data_processing` and `study`. 2.0 turned several scalar fields into objects (age,
  time point, template amount). The AIRR Data Commons still serves 1.3-shaped metadata.

**ClonoLoom's choices.** A sample is a `repertoire_id`, else a `sample_processing_id`, else the
file. The dataset's unit is cells when rows name cells; UMIs with `umi_count`; reads with
`consensus_count` alone; **unknown** with `duplicate_count` alone, since files written before
1.4 and Cell Ranger's do not count reads in it; the user confirms it.

### 1.2 Cell Ranger V(D)J outputs (10x Genomics)

Sources: Cell Ranger's source code (the support pages are behind a bot check), and the public
datasets of §6.

- `filtered_contig_annotations.csv` / `all_contig_annotations.csv`: one row per contig of a
  barcode. Columns `barcode, is_cell, contig_id, high_confidence, length, chain, v_gene, d_gene,
  j_gene, c_gene, full_length, productive, cdr3, cdr3_nt, reads, umis, raw_clonotype_id,
  raw_consensus_id`; Cell Ranger 6 adds `fwr1…fwr4` and `cdr1`, `cdr2` with their nucleotides
  and `exact_subclonotype_id`; Cell Ranger 9's multi runs add a first column `sample`.
- Booleans `True`/`False` and missing `None` up to Cell Ranger 5; `true`/`false` and empty from
  6 (but `chain` may still be `None`). In 3.x the filtered file kept non-productive contigs.
- `cdr3`/`cdr3_nt` are the IMGT junction (Cell Ranger's own AIRR writer maps `cdr3` to
  `junction_aa`).
- Genes have no allele (10x's reference alleles are `00`); shared and orphon genes are written
  with slashes (`TRAV29/DV5`); old references wrote `TRAV29DV5`.
- `clonotypes.csv`: `clonotype_id, frequency, proportion, cdr3s_aa, cdr3s_nt` (chains as
  `TRA:…;TRB:…`), plus `inkt_evidence`, `mait_evidence` in recent versions.
- `airr_rearrangement.tsv`: AIRR columns plus `is_cell`; no `locus`; `duplicate_count` is UMIs
  and `consensus_count` reads; `sequence_alignment` and `germline_alignment` are not aligned to
  each other.
- Layouts: `outs/` of a `vdj` run; a `multi` run's `outs/per_sample_outs/<sample>/vdj_t`,
  `vdj_b`, `vdj_t_gd` (filtered contigs, clonotypes, consensus, AIRR) and `outs/multi/vdj_*`
  (all contigs). Public downloads prefix each file with the dataset's name.
- `metrics_summary.csv`: values are formatted strings (`"3,304"`, `96.4%`); a multi run's is in
  long format (its header changed from `Library or Sample` to `Category`).

### 1.3 immunoSEQ exports (Adaptive Biotechnologies)

Sources: Adaptive's TechNote MRK-00342 (v3 columns), the Analyzer manual (v2), the Emerson 2017
files of §6, and the readers of LymphoSeq2, immuneML and VDJtools (read for their column maps).

- **v3** (snake_case): `rearrangement, amino_acid, frame_type, rearrangement_type, templates,
  reads` or `seq_reads`, `frequency, productive_frequency, cdr3_length, v_resolved, v_family,
  v_gene, v_allele, v_gene_ties…, v_index…`, often with the sample's metadata repeated on every
  row (`sample_name, species, locus, … productive_clonality, productive_entropy, … sample_tags`).
- **v2** (camelCase): `nucleotide, aminoAcid, count (templates/reads), frequencyCount (%),
  cdr3Length, vMaxResolved, vFamilyName, vGeneName, vGeneAllele, vGeneNameTies…, vIndex…,
  estimatedNumberGenomes, sequenceStatus…`. `frequencyCount (%)` is a fraction.
- `amino_acid`/`aminoAcid` is the junction, empty out of frame. `cdr3_length` is the junction's
  length in nucleotides; the junction is the `cdr3_length` bases of `rearrangement` from
  `v_index` (0-based); newer exports also give `cdr3_rearrangement`.
- `frame_type`: `In` (productive), `Out` (out of frame), `Stop` (in frame with a stop).
- Templates are immunoSEQ's estimate of input molecules (counting methods v3 and later); reads
  are post-amplification. Some published samples have missing or negative templates (−1).
- Missing values: `null`, `na`, `unresolved`, `unknown`, `no data`; `-1` for absent indexes.

**ClonoLoom's choices.** A file is counted in templates when every row has a positive template
count; otherwise every row of it in reads, with a finding, so that one file is never counted in
two units. A gene not resolved beyond its family is called by its ties (the genes it could be),
as an AIRR call lists them.

### 1.4 Formats of later waves

VDJdb releases (re-packaged in 2026: a normalized primary zip, the legacy files `vdjdb.txt`,
`vdjdb.slim.txt` and `vdjdb_full.txt` in a zip of their own, and an AIRR zip; tab-separated, parsed
by header name), TRUST4, IMGT/HighV-QUEST and VDJtools. Each is researched again when its wave is
planned. Until then, VDJtools' tables are read through a mapping of their columns (§1.5). MiXCR's
clone tables, OAS and OTS data units and tcrdist3's clone tables are read from wave 4 (§1.6).

### 1.5 Tables with mapped columns (wave 2)

Tools write clone tables of their own: VDJtools (`count`, `freq`, `cdr3nt`, `cdr3aa`, `v`, `d`,
`j`; Shugay et al. 2015, *PLoS Comput Biol* 11:e1004503, doi:10.1371/journal.pcbi.1004503), MiXCR
(`cloneCount` or `readCount`, `nSeqCDR3`, `aaSeqCDR3`, `allVHitsWithScore`), and the spreadsheets
labs keep. immunarch reads such tables by naming each tool's columns (`repLoad`'s parsers);
Immcantation's `ParseDb` renames columns to AIRR's. ClonoLoom reads them through a mapping the user
confirms (requirement D7, `internal/reads/mapped.go`):

- **The guess is a proposal.** Each field is guessed from the columns' names, compared without case
  or punctuation, from the names those tools and the AIRR schema use. A bare `CDR3` is amino acids
  or nucleotides by its values (letters outside ACGTN). The user sees the guess beside the file's
  first rows and changes it.
- **The unit is said, never read.** A count column says neither whether reads or molecules were
  counted (§4), so the mapping states it. Its name only proposes one (`umi`, `template`, `read`).
- **The whole file is checked first.** For each field: how many values are valid, examples of the
  others, and notes (the share of junctions with IMGT's anchors, of nucleotide junctions in frame,
  of counts that are not whole numbers, which diversity's estimators need). A table that does not
  read as the user mapped it is refused before it becomes a dataset.
- **Productivity**, where no column gives it, is inferred: a junction of whole codons without a stop
  codon is productive. This is what a VDJtools or MiXCR table's own convention marks (`~` or `*` in
  the amino acids of an out-of-frame junction).
- **The mapping is part of the dataset**, as a format is: it is kept with the project, enters the
  dataset's ID (one file under two mappings is two datasets) and its cache key, and is stated in the
  methods paragraph.

### 1.6 MiXCR, OAS and OTS, and tcrdist3's tables (wave 4)

Checked against the files themselves (PyClonal's MiXCR 3 tables, MiLaboratories' benchmark tables
of MiXCR 4, OTS and OAS units of five studies, tcrdist3's `dash.csv` and `dash_human.csv`) and the
tools' documentation; how each is read is in `docs/FORMATS.md`.

- **MiXCR** (`mixcr exportClones`): MiXCR 3 names counts `cloneCount`, MiXCR 4 `readCount`, with
  `uniqueMoleculeCount` where molecules were tagged; MiXCR 2 writes readable names (`Clone count`,
  `All V hits`, `AA. Seq. CDR3`). `nSeqCDR3` and `aaSeqCDR3` are the IMGT junction; `_` marks a
  frameshift, `*` a stop, lower case an imputed residue. Hits are listed best first with their
  scores; `*00` is MiXCR's allele for a library without alleles. MiXCR 4 splits files by chain
  (`<sample>.clones_TRB.tsv`) and, for single cells, writes each clone's chain in each cell
  (`tagValueCELL`), MiXCR's clonotype of cells as `cellGroup`, and the chain as `topChains`.
  **ClonoLoom's choice:** the call is every hit with the best score (a tie kept as a set, as AIRR
  calls list ties), without its score and without `*00`; a table is counted in molecules only when
  every clone has them.
- **OAS and OTS** (Olsen et al. 2022, *Protein Sci* 31:141, doi:10.1002/pro.4205; Raybould et al.
  2024, *Cell Rep* 43:114704, doi:10.1016/j.celrep.2024.114704; both CC BY 4.0): gzipped CSV,
  the first line the unit's metadata as JSON in one CSV field (a link may hold a zero-width
  space), then IgBLAST's AIRR columns; OAS's unpaired units have no `sequence_id` and count each
  sequence by `Redundancy`; paired units hold each AIRR column once per chain (`_heavy`, `_light`;
  `_beta`, `_alpha`, the β block first), one row per cell, with 10x Genomics' contig names
  (`<barcode>_contig_1`), each barcode once a unit. Loci are one letter, booleans `T`/`F`,
  integers decimals. **ClonoLoom's choice:** `Redundancy` counts sightings of a sequence, reads or
  molecules as the study tagged them, so the user confirms the unit.
- **tcrdist3's clone tables** (Mayer-Blackwell et al. 2021; tcrdist3 is MIT): a row per clone,
  each chain's `v_b_gene`, `j_b_gene`, `cdr3_b_aa` (the IMGT junction) and `cdr3_b_nucseq` (chain
  letters `a`, `b`, `g`, `d`), the clone's `count`, and `subject`, `epitope` and `clone_id` where
  given. tcrdist3 itself counts cells, templates or reads alike in `count`. **ClonoLoom's choice:**
  a table of paired chains is single-cell data, so a clone of n cells is read as n cells (the
  clonotype tables count cells, not counts); a table of one chain is bulk, its unit for the user
  to confirm.

## 2. Gene nomenclature

- IMGT nomenclature (WHO-IUIS; Lefranc 2014, *Front Immunol* 5:22,
  doi:10.3389/fimmu.2014.00022): locus (`TRA TRB TRG TRD IGH IGK IGL`), segment (`V D J C`),
  subgroup number, gene number when the subgroup has several genes, and `*NN` allele. Shared
  TRA/TRD genes carry both names (`TRAV29/DV5`), orphons their chromosome (`TRBV20/OR9-2`),
  duplicated kappa genes a `D` (`IGKV1D-39`). Heavy-chain constant genes are named by isotype
  (`IGHM`, `IGHG1`); `IGHD` alone is the δ constant gene.
- **Human gene list.** HGNC approves the same names without IMGT's slashes (`TRAV29DV5`,
  `TRBV20OR9-2`). ClonoLoom builds its list of the 676 human TR and IG genes (and which are
  pseudogenes) from HGNC's complete set (CC0, https://www.genenames.org) with
  `internal/genes/generate-human.mjs`, putting the slashes back.
- **Adaptive names** (`TCRBV05-01*01`): locus spelled `TCRB`, numbers zero-padded, a gene
  number even for a subgroup with one gene (`TCRBV09-01` is `TRBV9`), orphons as
  `TCRBV20-or09_02`, combined calls as `TCRBV12-03/12-04`, an unresolved gene as `TCRBV20-X` or
  the family alone. Mapped by those rules; the gene number is dropped where the human list has
  the subgroup without one. Each dataset records every vendor name with the IMGT name it was
  read as.
- **ClonoLoom's choice** for mouse immunoSEQ names: gene numbers are kept until a mouse gene
  list is added (MGI), so a mouse subgroup with one gene keeps a `-1` IMGT does not write.
- Ambiguous calls are kept as sets. A clonotype definition at gene level keys a call by its
  distinct genes (`TRBV6-2|TRBV6-3`), so ties never merge clonotypes silently.

## 3. Junction and CDR3

The IMGT junction runs from the conserved 2nd-CYS 104 to J-PHE or J-TRP 118, inclusive; CDR3-IMGT
is 105–117 (Lefranc et al. 2003, *Dev Comp Immunol* 27:55, doi:10.1016/S0145-305X(02)00039-3).
10x's `cdr3`, Adaptive's `amino_acid`, MiXCR's `aaSeqCDR3`, VDJdb's `cdr3`, TRUST4's and
VDJtools' CDR3 columns are all junctions; only the AIRR schema's (and IgBLAST's, Change-O's and
OAS's) `cdr3` excludes the anchors. ClonoLoom reads every one of these as a junction, checks that
it begins with C and ends with F or W, and counts the exceptions as a note: germline genes such as
TRAJ35 end otherwise, and short partial junctions occur.

## 4. Abundance units

| Source | Field | Unit |
| --- | --- | --- |
| AIRR ≥ 1.4 | `duplicate_count` | reads (identical observations) |
| AIRR ≥ 1.4 | `umi_count` | UMIs |
| AIRR | `consensus_count` | reads behind a consensus |
| AIRR ≤ 1.3 | `duplicate_count` | reads or UMIs |
| Cell Ranger contigs | `reads`, `umis` | per contig; a clonotype's abundance is cells |
| Cell Ranger AIRR | `duplicate_count`, `consensus_count` | UMIs, reads |
| immunoSEQ | `templates`, `reads`/`seq_reads`, `count (templates/reads)` | templates, reads, either |
| MiXCR | `readCount`/`cloneCount`, `uniqueMoleculeCount` | reads, UMIs |

A frequency is always said with its denominator (all, or productive, abundance of the sample).

## 5. Methods

Each method is implemented from its publication, cited in the file that implements it, and
checked against the reference implementation named (run only by scripts in
`validation/reference/`) or against simulated truth. The formulas, defaults and points where
reference implementations differ are recorded here when the wave that builds the method is
planned in detail.

| Wave | Area | Method | Publication | Reference implementation |
| --- | --- | --- | --- | --- |
| 1 | QC | Sample coverage (Good–Turing, with Chao and Jost's correction) | Chao & Jost 2012, *Ecology* 93:2533, doi:10.1890/11-1952.1 | iNEXT |
| 2 | Diversity | Hill numbers of order q; Shannon and Simpson; Pielou evenness; clonality | Hill 1973, *Ecology* 54:427; Jost 2006, *Oikos* 113:363 | immunarch, scRepertoire, closed forms |
| 2 | Diversity | Chao1 (bias-corrected), ACE | Chao 1984, *Scand J Stat* 11:265; Chao & Lee 1992, *JASA* 87:210 | iNEXT, immunarch |
| 2 | Diversity | Rarefaction and extrapolation by size and coverage; asymptotic estimators; bootstrap intervals | Chao et al. 2014, *Ecol Monogr* 84:45, doi:10.1890/13-0133.1; Hsieh, Ma & Chao 2016, *Methods Ecol Evol* 7:1451 | iNEXT |
| 2 | Overlap | Jaccard, overlap coefficient, Morisita–Horn | Horn 1966, *Am Nat* 100:419 | immunarch, scirpy |
| 3 | Expansion | Fisher's exact and binomial tests between two samples | DeWitt et al. 2015, *J Virol* 89:4517, doi:10.1128/JVI.03474-14 | R `stats` |
| 3 | Expansion | Beta-binomial test with biological variance | Rytlewski et al. 2019, *PLoS One* 14:e0213684, doi:10.1371/journal.pone.0213684 | a reference script in R |
| 4 | Specificity | TCRdist | Dash et al. 2017, *Nature* 547:89, doi:10.1038/nature22383; Mayer-Blackwell et al. 2021, *eLife* 10:e68605 | tcrdist3 |
| 4 | Pgen | Generation probability by dynamic programming over recombination models | Sethna et al. 2019, *Bioinformatics* 35:2974, doi:10.1093/bioinformatics/btz035; models: Marcou et al. 2018, *Nat Commun* 9:561 | OLGA |
| 5 | B cells | Clonal families by distance to nearest; mutation by region; germlines | Gupta et al. 2015, *Bioinformatics* 31:3356; Gupta et al. 2017, *J Immunol* 198:2489; Nouri & Kleinstein 2018, *Bioinformatics* 34:i341 | Change-O, SHazaM, SCOPer |
| 5 | B cells | Lineage trees by maximum parsimony | Fitch 1971, *Syst Zool* 20:406; Hoehn et al. 2022, *PLoS Comput Biol* 18:e1009885 | Dowser (phangorn) |
| 6 | Single cell | Clonotype modularity; graph-versus-graph | Sturm et al. 2020, *Bioinformatics* 36:4817; Schattgen et al. 2022, *Nat Biotechnol* 40:54 | scirpy, conga |
| 7 | Specificity | Specificity groups | Huang et al. 2020, *Nat Biotechnol* 38:1194, doi:10.1038/s41587-020-0505-4 | GLIPH2 (outputs only) |

### 5.1 Clonotypes (wave 1)

- A clonotype definition names its chains (each locus separately, one locus, or a pair), its
  junction (amino acids, productive rearrangements only; or nucleotides), and how V and J genes
  are compared (allele, gene, family or not at all). Gene-level keys drop alleles and keep ties as
  sets, so two calls between the same genes match and a tie never matches a resolved gene.
- Single cells: the primary chain of each locus is the one with most UMIs; **ClonoLoom's choice**
  for ties is most reads, then the contig ID first in order, so the choice is the same everywhere
  (scirpy breaks ties by its junction strings). Cells with one arm of a pair are grouped with cells
  of that arm alone, as scirpy's `receptor_arms="all"` does, unless the definition requires both.
- scirpy's `define_clonotypes` always compares nucleotide junctions; amino-acid grouping is its
  `define_clonotype_clusters(sequence="aa", metric="identity")`. ClonoLoom's paired definitions
  give the same partitions as these on every development run (`validation/clonotypes_test.go`).
- A clonotype's ID is a hash of the definition's meaning (not its name) and the clonotype's key.

### 5.2 Simulated studies (wave 2)

`clonoloom simulate` (`internal/simulate`) makes bulk TRB studies whose truth is known, so that
diversity, rarefaction and overlap can be checked against it as well as against reference
implementations.

- **Germline.** GRCh38's sequence of each functional TRB V, D and J gene at Ensembl's annotation
  (release 116), generated into `internal/simulate/germline.go` by its script from Ensembl's REST
  API. Ensembl "imposes no restrictions on access to, or use of, the data provided"; the sequence
  is the Genome Reference Consortium's. The conserved cysteine is the C of a Y-X-C motif ending a
  V; the conserved phenylalanine, the F of a J's F-G-X-G. Forty-six V genes have one (TRBV7-3
  and TRBV17, ORFs whose cysteine is mutated, do not); thirteen J genes and both D genes. Calls
  are at gene level: the reference genome's sequence is not necessarily IMGT's allele *01. IMGT's
  data (CC BY 4.0 since 2026-07-01) are not used, pending the confirmation §7 describes.
- **Recombination** as Murugan et al. 2012 (*PNAS* 109:16161, doi:10.1073/pnas.1212755109)
  describe it: a V, a J, and a D given the J (TRBJ1 genes only with TRBD1; TRBJ2 genes with either,
  an even chance assumed, since the 10x data's D calls cannot tell); deletions at each end
  (negative binomial, means 4, 3, 3 and 4 nucleotides for V, D5′, D3′ and J), or palindromic
  nucleotides (one with chance 0.04, two with 0.02); non-templated insertions between V and D and
  between D and J (means 4.5 and 4, A/C/G/T 0.22/0.28/0.28/0.22). None of OLGA's or IGoR's model
  files is used (GPL-3.0); the parameters are **ClonoLoom's own choices**.
- **Usage and selection.** V and J usage are those of the 4,640 productive TRB chains of the cells
  of 10x Genomics' 10k PBMC dataset (CC BY 4.0). Productive rearrangements are selected on their
  junction's length, kept with a chance proportional to the data's share of that length over the
  model's, as Elhanati et al. 2014 (*PNAS* 111:9875, doi:10.1073/pnas.1409572111) estimate
  selection factors: their junctions then average 14.5 amino acids with a standard deviation of
  1.7, as the data's do (without selection, 15.3 and 2.7). Nonproductive rearrangements are not
  selected.
- **Populations and samples.** Each subject's productive clones have sizes from a discrete power
  law, P(size ≥ s) = s^−α; nonproductive ones (15% of clones) too; a few expanded clones may hold
  given shares of the cells. The same nucleotide rearrangement drawn twice in a subject is one
  clone. A sample draws templates in proportion to the clones' cells; each template's reads are
  negative binomial (mean 8, size 2), and a template with none is not seen.
- **Truth.** For each subject, the population's productive clonotypes (amino-acid junction, V gene,
  J gene) and their Hill numbers, Shannon and Simpson indices, evenness, clonality, Gini
  coefficient, top-ten share and homeostasis bins; for each sample, the clonotypes and coverage a
  sample of its depth shows on average (Σ 1−(1−p)^m and 1 − Σ p(1−p)^m); for each pair of
  subjects, the clonotypes their populations share.
- **Over time (wave 3, slice 4; `internal/simulate/series.go`).** A subject's blood at a time point
  is its own population, each clone's cells multiplied by its trajectory's fold there (a class of
  clones chosen at random), mixed with a reference population in a given share: an infusion
  product (cells collected from the subject's own blood by a multinomial draw, each clone then
  expanded ex vivo by a log-normal fold) or a donor's population. In each sample, each clone's
  cells are then multiplied by a gamma of mean 1 and variance v (0.1 in the presets), drawn anew
  for every sample: the biological variance between blood samples, which a binomial test of two
  samples ignores (Rytlewski et al. 2019 found it between samples two weeks apart). The log of such
  a gamma has variance ψ′(1/v) (0.105 for v = 0.1), so two samples' log ratio for a large clone
  varies by 2ψ′(1/v) beyond sampling. Templates are drawn from each sample's blood by a multinomial.
  The truth gives each productive clonotype's expected share of the productive cells at each time
  point (the variance left out), the reference's share of each sample, and the expected share of
  each sample's productive cells in clonotypes the reference population holds.

### 5.3 Diversity (wave 2)

`internal/diversity` measures each sample from its frequency counts (how many clonotypes were seen
once, twice, …), over its clonotypes' abundances in the dataset's unit; clonotypes without a count
are left out. The estimators and the bootstrap need whole counts; without them only the observed
measures are given.

- **Observed**: the Hill numbers of order q (Hill 1973, *Ecology* 54:427, doi:10.2307/1934352),
  a profile from 0 to 4; Shannon's entropy (natural log, so that exp(H) is the Hill number of
  order 1; immunarch's `entropy` defaults to log2); Simpson's index Σp² with replacement, its
  inverse and Gini–Simpson; Pielou's evenness H/ln S and **clonality** 1 − evenness (Adaptive's
  "clonality", MRK-00355); **Simpson clonality** √Σp² (Adaptive's, which it recommends as less
  sensitive to depth); Gini's coefficient over the clonotypes' abundances sorted ascending
  (immunarch's and immApex's, without the S/(S−1) correction); the 10, 100 and 1,000 largest
  clonotypes' shares; the largest frequency; clonal space homeostasis in immunarch's bins (rare
  ≤ 10⁻⁵ < small ≤ 10⁻⁴ < medium ≤ 10⁻³ < large ≤ 10⁻² < hyperexpanded).
- **Estimated** (the repertoire the sample was drawn from): **Chao1**, bias-corrected with
  (n−1)/n and its f2 = 0 form (Chao 1984; Chao et al. 2014 Eq. B.5), its 95% interval
  log-transformed on the clonotypes not seen; where none is estimated unseen (no singleton; or one
  and no doubleton), iNEXT's interval: the variance Σ f_k(e^−k − e^−2k) − (Σ k e^−k f_k)²/n and
  S/(1−P) ± 1.96·SE/(1−P) with P = Σ f_k e^−k/S (the form immunarch writes for f1 = f2 = 0 alone),
  which may lie above the estimate (immunarch gives none there, or another);
  **ACE** with the rare clonotypes those seen at most 10 times (Chao & Lee 1992, *JASA* 87:210,
  doi:10.1080/01621459.1992.10475194), not estimable where every rare clonotype is a singleton (as
  vegan says; immApex answers the clonotypes seen); the **Shannon diversity** by Chao, Wang and
  Jost's estimator (2013, *Methods Ecol Evol* 4:1091, doi:10.1111/2041-210X.12108), its tail
  Σ_{r≥n}(1−A)^r/r computed by the Euler–Maclaurin formula where direct summation would take
  millions of terms; the **Simpson diversity** n(n−1)/ΣX(X−1) (Chao et al. 2014); sample coverage
  (Chao & Jost 2012).
- **Intervals**: 95%, Chao1's from its variance; the others' from **200 bootstrap replicates** of
  each sample (Chao et al. 2014, Appendix G; iNEXT's `ChaoShannon` and `ChaoSimpson` default; its
  `iNEXT()` defaults to 50): the detected clonotypes' frequencies tuned down
  (p̂ = (X/n)(1 − λ(1 − X/n)^n)), ⌈f̂0⌉ undetected ones sharing 1 − Ĉ, each replicate an exact
  multinomial sample of the same size (conditional binomials, Hörmann's BTRS), every measure
  computed again; the interval is the estimate ± 1.96 SE, clamped to the measure's range. Each
  replicate draws from its own stream of a recorded seed, so the result is the same on every
  computer and with any number of processors. **ClonoLoom's choice**: the asymptotic Shannon
  diversity's lower bound is floored at the Shannon diversity seen (biased low), as iNEXT does; the
  Simpson diversity's is not, though iNEXT floors it too: the Simpson diversity seen is biased high,
  and deep samples see it just below the estimate, so the floor cut away the interval's lower half
  (in simulations, its intervals held the truth in 47.5% of samples with it, 91.5% without).
- **Agreement** (`validation/diversity_test.go`, `validation/reference/diversity.R`): on 18 samples
  (three immunoSEQ subjects of Emerson et al. 2017, its AIRR Data Commons copy, the AIRR Community's
  two test files and the simulated standard study's twelve), every observed measure equals R's,
  immunarch's own functions' (Hill numbers, Gini, Gini–Simpson, inverse Simpson, Chao1 and its
  interval) and iNEXT 3.0.2's (Chao1 and its interval, the asymptotic Shannon and Simpson
  diversities, coverage; to its three printed decimals), and ACE equals vegan 2.7.6's; the bootstrap
  standard errors are 0.86 to 1.13 times iNEXT's (median 0.98; each is its own random draw).
- **Interval coverage on simulations** (`validation/coverage_test.go`, 200 samples of each
  repertoire, 200 replicates each; clonotype sizes from a power law, α = 1.3):

  | Repertoire | Coverage | Chao1 | Shannon, est. | Simpson, est. | Seen: clonotypes | Shannon | Simpson |
  | --- | --- | --- | --- | --- | --- | --- | --- |
  | 5,000 clonotypes, 50,000 templates | 0.99 | 96.0% | 93.5% | 91.5% | 99.5% | 94.5% | 92.5% |
  | 50,000 clonotypes, 20,000 templates | 0.62 | 0% | 0% | 92.0% | 92.0% | 91.5% | 92.5% |

  The share of 95% intervals holding the truth (the repertoire's own value for the estimates; the
  expected value at the sample's depth for those seen). Where the coverage is low, Chao1 and the
  estimated Shannon diversity are lower bounds, as their authors say: the Repertoire view says so
  beside them, and comparisons at equal coverage (rarefaction, §5.4) are the remedy.
- **immunoSEQ's own clonality** (QC's vendor checks): Adaptive computes `productive_clonality`,
  `productive_simpson_clonality`, `productive_entropy` (log2) and `max_productive_frequency` over
  the productive rearrangements (rows) **from their reads, not their templates**: on the three
  Emerson et al. 2017 exports they are reproduced from reads to seven significant digits (the
  entropy carries about 2·10⁻⁸ of rounding), and templates give other values (P00207's clonality
  0.245 from reads, 0.202 from templates).

### 5.4 Rarefaction and extrapolation (wave 2)

`diversity.Rarefy` answers, for a sample of whole counts, its Hill numbers of order 0, 1 and 2 and
its sample coverage at other sizes, and its Hill numbers at other coverages (Chao et al. 2014;
Chao & Jost 2012, *Ecology* 93:2533, doi:10.1890/11-1952.1), as iNEXT 3.0 computes them:

- **Interpolated** (size m below the sample's n): the expectation over subsamples of m drawn
  without replacement. Clonotypes Σ_i [1 − C(n−X_i, m)/C(n, m)]; Shannon diversity
  exp(ln m − Σ_i E[Y_i ln Y_i]/m), Y_i hypergeometric; Simpson diversity
  1/(1/m + (1 − 1/m)·ΣX(X−1)/(n(n−1))); coverage 1 − Σ_i (X_i/n)·C(n−X_i, m)/C(n−1, m). Each ratio
  of binomial coefficients is a product of its smaller side's terms, or a difference of Stirling
  series, never the difference of two large log-gammas. iNEXT sums E f_k(m) over every k from 1 to
  m for every clonotype, which takes minutes for a million cells; ClonoLoom sums each clonotype's
  hypergeometric outward from its mode until the terms fall below 10⁻¹⁸ of the sum, the same
  expectation (equal to every term summed in 256-bit arithmetic to 10⁻¹³, and to the definition's
  sum to 10⁻¹⁰). A million-cell sample's curves, with 200 replicates, take about 2 s on ten cores.
- **Extrapolated** (above n, to twice n): obs + (asy − obs)(1 − (1 − β)^(m−n)) with asy Chao1 or
  the asymptotic Shannon diversity (§5.3, not floored) and β = (obs − D(n−1))/(asy − D(n−1)), as
  iNEXT 3.0 has it (for clonotypes, Chao et al. 2014's Eq. 9); the Simpson diversity by its closed
  form; coverage 1 − (f1/n)·A^(m−n+1).
- **At a coverage**: the size where the sample reaches it, below the sample's own coverage by
  bisection over whole sizes and linearly between them (iNEXT minimizes numerically, to about
  10⁻⁴ of a cell), above it by the extrapolated coverage's closed form; then the Hill numbers
  there (linearly between whole sizes, as iNEXT).
- **Sizes along a curve**: iNEXT's 40 (19 evenly from 1 to n − 1, n, 20 evenly to 2n) and 18 more
  evenly on a logarithmic scale, since repertoires' sizes span orders of magnitude and iNEXT's
  first step is a twentieth of the sample.
- **Intervals**: 95%, from the measures' 200 bootstrap replicates (§5.3, the same seed and streams,
  so the same replicates), every value computed again from each at the same size, or at the same
  coverage (iNEXT's "unconditional" variance); the estimate ± 1.96 SE.
- **Levels suggested** (ClonoLoom's choice): first, the lowest sample coverage among the samples and
  the smallest sample's size, which every sample reaches by rarefaction; then iNEXT's defaults, the
  lowest coverage any sample reaches at twice its size and twice the smallest size (after Chao et
  al. 2014 and Chao & Jost 2012), marked as extrapolated, for the reason below.
- **Agreement** (`validation/rarefaction_test.go`, `validation/reference/rarefaction.R`): on the
  18 samples of §5.3, at iNEXT 3.0.2's 40 sizes along each curve, every Hill number and coverage
  equals iNEXT's to 10⁻⁹, except the extrapolated Shannon diversity, within 8·10⁻⁸: its β divides
  by the small difference between the values at n and n − 1 (0.17 for S01-1), which magnifies
  iNEXT's rounding of the value at n (R sums a term for every clonotype: 2768.6720701115 in iNEXT,
  2768.6720701097 in ClonoLoom, 2768.67207010965 exactly). At each coverage, the same to what
  iNEXT's minimization leaves of the size there (R's `optimize` stops within √ε·m of it, about
  10⁻³ of a cell at 10⁵), except where the coverage barely changes with size (the AIRR Community's
  three-clonotype test files: 35 coverages), where the size is not determined and iNEXT's
  minimizer answers any along the flat stretch. Two cohorts, the simulated study's twelve samples
  and Emerson et al.'s three subjects, compared at iNEXT's default levels (`estimateD`): the same
  levels and the same values. The bootstrap standard errors are 0.85 to 1.26 times iNEXT's (4,240
  compared; median 1.01, 95% of them 0.90–1.14), each a random draw of its own.
- **The curves against the truth on simulations** (`validation/coverage_test.go`, 200 samples of
  each repertoire, 200 replicates each; power-law clonotype sizes, α = 1.3; the truth the value a
  sample of that size from the repertoire has, exactly for coverage, clonotypes and the Simpson
  diversity, and as exp of the mean entropy of 200 such samples for the Shannon diversity). The
  share of 95% intervals holding the truth, and the mean estimate's error relative to it:

  | Repertoire | Coverage | Size | Coverage | Clonotypes | Shannon | Simpson |
  | --- | --- | --- | --- | --- | --- | --- |
  | 5,000 clonotypes, 50,000 templates | 0.99 | n/2 | 98.0%, 0.0% | 96.5%, 0.0% | 94.0%, 0.0% | 91.5%, −0.2% |
  | | | 2n | 100%, 0.0% | 100%, 0.0% | 90.5%, +0.7% | 91.5%, −0.2% |
  | 50,000 clonotypes, 20,000 templates | 0.62 | n/2 | 92.0%, 0.0% | 93.5%, 0.0% | 92.5%, +0.3% | 92.0%, −0.3% |
  | | | 2n | 1.0%, +2.7% | 72.0%, −0.9% | 50.5%, +2.9% | 92.0%, −0.3% |
  | 50,000 clonotypes, 80,000 templates | 0.83 | n/2 | 93.5%, 0.0% | 92.5%, 0.0% | 93.0%, +0.1% | 95.5%, +0.1% |
  | | | 2n | 0.0%, +1.1% | 37.5%, −1.0% | 20.5%, +2.2% | 95.5%, +0.1% |
  | 50,000 clonotypes, 320,000 templates | 0.97 | n/2 | 97.5%, 0.0% | 95.5%, 0.0% | 90.0%, +0.1% | 94.5%, +0.1% |
  | | | 2n | 16.5%, +0.1% | 87.0%, −0.2% | 17.5%, +1.1% | 94.5%, +0.1% |

  Rarefied, every value's interval held the truth in 90–98% of samples. Extrapolated to twice the
  sample, the values were within 3% of the truth on average (the Simpson diversity within 0.3%),
  but where the repertoire has many rare clonotypes the intervals were narrower than that error,
  and only the Simpson diversity's held the truth as often as they say: the bootstrap's intervals
  measure the sampling variation of an estimator whose bias they do not see. Extrapolation is
  iNEXT's, unchanged; hence the levels reached by rarefaction come first, and the view says so
  where a sample is extrapolated.
- **Reads**: the estimators count each cell or molecule once. In reads each counts many times over,
  so few clonotypes are seen once or twice, and Chao1, ACE, the coverage, the estimated diversities
  and extrapolation all read a sample as complete (Emerson et al. 2017's three subjects, opened as
  one dataset in reads, each have a coverage of 1). The Repertoire view warns where a dataset's
  unit is reads; rarefying to equal reads compares equal sequencing depth, not equal cells.

### 5.5 Gene usage, junction lengths and composition (wave 2)

`internal/usage` counts what each sample's clonotypes (under the dataset's definition and filter)
are made of, as a share of its clonotypes (each counted once) and of its abundance:

- **Genes**: V, D, J and C, at the gene and the family level (IMGT's subgroup: the name to its
  first "-" or "/", without the "D" of a duplicated gene; a constant gene its own family), and V–J
  pairs. A clonotype's genes are those of its rearrangements in the sample, in proportion to their
  counts (a clonotype grouped without its V gene, or by family, may hold several). A call tied
  between several genes gives each an equal part, so that the shares sum to one and no clonotype is
  counted twice; immunarch's `geneUsage` instead counts the tie as a gene of its own, drops it, or
  takes its first gene. A rearrangement without a call counts as not called, reported apart.
- **Junction lengths** in amino acids of the IMGT junction (positions 104 to 118, the CDR3 and its
  two conserved residues), the spectratype, and the mean length; a junction given only in
  nucleotides is translated where it is in frame.
- **Composition**: each amino acid's share of the junctions' residues (X for any but the twenty),
  and, for the lengths of at least 5% of the clonotypes over the samples (at most six), its share at
  each IMGT position (Lefranc et al. 2003, *Dev Comp Immunol* 27:55,
  doi:10.1016/S0145-305X(02)00039-3): the CDR3 numbered from 105 and from 117 toward its middle,
  positions missing from the middle when it is shorter than 13 (111, 112, 110, 113, …) and
  insertions there when longer (111.1, 112.1, 111.2, …).
- **Intervals**: 95%, from 200 bootstrap replicates of each sample's clonotypes (each replicate the
  sample's S clonotypes drawn with replacement, every share computed again; the estimate ± 1.96
  SE), for the genes, lengths, amino acids and the mean length (not the pairs and positions, which
  are many). Each replicate has its own random stream: the same in every run.
- **Conditions**: each summarized by the mean of its samples' shares, each sample counted once, and
  their lowest and highest (requirements G3: the subject is the unit; tests between conditions come
  with cohorts, wave 7).
- **Agreement** (`validation/usage_test.go`, `validation/reference/usage.R`): on the 15 samples of
  §5.3 in AIRR format (the ADC copy of Emerson et al. 2017's subject, the AIRR Community's two test
  files and the simulated study's twelve; immunoSEQ's own files name genes Adaptive's way, mapped
  to IMGT's by the readers), all 8,277 shares (genes and families of each segment, V–J pairs,
  lengths, the mean length, amino acids) equal an independent R implementation's to 10⁻¹², and
  neither has a name the other lacks.
- **Against the simulator's truth** (`validation/usage_test.go`): each simulated sample's expected
  shares follow from its subject's clones and the templates drawn: by abundance, a gene's clones'
  share of the productive cells; by clonotypes, of the productive clones each weighted by its chance
  to be seen, 1 − (1 − q)^T. Of 2,308 intervals (V, D and J genes, lengths and amino acids, both
  weightings, twelve samples), 97.4–100% held the expected share: the clonotype bootstrap is a
  little wider than it need be.
- 69,535 clonotypes (the largest development sample) are counted, with 200 replicates, in 0.7 s.

### 5.6 Overlap (wave 2)

`internal/overlap` measures every pair of samples of a dataset, two samples sharing a clonotype when
the dataset's definition calls their rearrangements the same:

- **Over the clonotypes seen**: those shared; Jaccard's index (1912, *New Phytol* 11:37,
  doi:10.1111/j.1469-8137.1912.tb05611.x), |A∩B|/|A∪B|; Sørensen–Dice's (Dice 1945, *Ecology*
  26:297, doi:10.2307/1932409), 2|A∩B|/(|A|+|B|); the overlap coefficient, |A∩B|/min(|A|, |B|).
  Each grows with depth, so each is also given **rarefied**: as expected with both samples
  subsampled without replacement to the smallest sample's size m, exactly (a clonotype of x cells
  in a sample of n is in the subsample with chance 1 − C(n−x, m)/C(n, m), §5.4; the indices are
  ratios of these expectations), where immunarch and VDJtools draw one random subsample.
- **Over the abundances**: the Morisita–Horn index (Horn 1966, *Am Nat* 100:419,
  doi:10.1086/282436), 2Σxy/((Σx²/X² + Σy²/Y²)XY), as immunarch and vegan compute it; and
  **Morisita's own index** (1959, *Mem Fac Sci Kyushu Univ Ser E Biol* 3:65), with Σx(x−1)/(X(X−1))
  in place of Σx²/X², which estimates Σp² without bias. Two samples of one repertoire read about
  0.98 by Horn's form and 1.000 by Morisita's (the simulated study's six subjects); Morisita's can
  exceed 1 a little by chance, and is given as it is (vegan sets its dissimilarity to zero there).
  The share of each sample's abundance in the clonotypes the other holds too, which is not
  symmetric.
- **Intervals** (95%): from 200 bootstrap replicates of each sample's cells (a multinomial of its
  size over its clonotypes' frequencies), every pair measured again. Alone, they were too narrow:
  resampling keeps the clonotypes seen shared, while which clonotypes two samples share is itself
  random, rare events between different subjects, so that on the simulated study the shared
  clonotypes' intervals held the truth in 64% of pairs. Added: the variance of the sharing, a sum
  over the shared clonotypes of Bernoulli events with chances π = (1 − (1 − p)^A)(1 − (1 − q)^B),
  estimated without bias by Σ y²(1 − π) over those seen shared (Horvitz and Thompson 1952, *J Am
  Stat Assoc* 47:663, doi:10.1080/01621459.1952.10483446), less the part the bootstrap already has,
  through each measure's derivative; with p and q each clonotype's frequency in its repertoire as
  the diversity bootstrap estimates it (§5.3: a clonotype seen once is rarer than 1/n). The
  intervals are on the logarithmic scale, as fits counts and indices near zero, whose distributions
  are skewed; where a pair shares nothing, from zero to what a count of none allows (Poisson's
  −ln 0.025 ≈ 3.7 clonotypes).
- **Public clonotypes**: each clonotype by the number of subjects whose samples hold it (from the
  samples' annotation or AIRR metadata; without subjects, each sample its own, and the view warns
  that two samples of one person would then look public), and the most public listed.
- **Pairs summarized** by kind (one subject or two, one condition or two): each measure's mean,
  lowest and highest; descriptive only, the pairs not being independent.
- **Agreement** (`validation/overlap_test.go`, `validation/reference/overlap.R`,
  `validation/reference/scirpy_overlap.py`): on the simulated study's 66 pairs and Emerson et al.
  2017's three subjects' 3 (counted in reads, as a dataset of the three counts them), every measure
  equals immunarch's functions' (Jaccard, Sørensen–Dice as Tversky's with a = b = ½, the overlap
  coefficient, Morisita–Horn), vegan's (Morisita–Horn and Morisita's index) or R's (the rest, the
  rarefied measures by lchoose) to 10⁻⁹, and the clonotypes by number of subjects equal R's; on
  four single-cell T-cell runs (two human, two mouse) opened as one dataset, every pair's shared
  clonotypes and Jaccard index, and each sample's clonotypes, equal scirpy 0.25.1's
  `repertoire_overlap`, with clonotypes by both chains and by the beta chain alone.
- **Against the simulator's truth** (five simulated studies, each its own seed, 330 pairs; the truth
  from the subjects' populations, the templates drawn that gave reads, and the depth): the 95%
  intervals held the expected shared clonotypes in 95.2% of pairs (30 of 30 of one subject, 284 of
  300 of two), the rarefied ones in 97.0%, and the populations' overlap in 93.9% for Morisita's
  index (30 of 30 of one subject) and 85.8% for the Morisita–Horn index (4 of 30 of one subject: its
  bias).

### 5.7 Figures and exports (wave 2)

**Figures** (requirement O1, `web/lib/plot.js`, `web/ui/figures.js`, `api_figures.go`). A plot
saved from the window stands alone in a manuscript. It is drawn on white whatever the theme, with
its title, legend and a caption. The caption states what was measured, the clonotype definition,
the abundance unit, the samples (n), the filter and the program's version: what a reader needs to
compare the figure with another study's, without the text.
- **SVG** keeps every mark as a vector, with its colors written in rather than taken from the
  page's style sheet, and common fonts named.
- **PNG** is drawn at 300 dpi, the resolution journals commonly ask of figures. The resolution is
  recorded in the file's `pHYs` chunk (the W3C PNG specification), so a layout program places it at
  its size.
- **PDF** is one page of the figure's size holding that PNG, written by ClonoLoom itself (ISO
  32000-1: a catalog, a page, a Flate-compressed RGB image and its drawing operator, the
  document's information with the program as Producer). Fonts are not embedded, because there is
  no text to embed.

**Exports** (requirements O3, O9, `internal/export`, `api_export.go`):
- **Rearrangements** are written as an AIRR Rearrangement TSV (the schema's required fields,
  §1.1), each row with its clonotype under the definition as `clone_id` and its sample's
  `repertoire_id`. The sequence, alignment and CIGAR fields are empty: ClonoLoom does not keep
  them, and the source files hold them. The file reads back into the same clonotypes.
- **Clonotypes and results** are written as CSV (RFC 4180), one row per clonotype or value, each
  value with its interval. Numbers are written in the shortest form that reads back to the same
  value, and a missing value is left empty (never 0: `conventions.md`, "Missing is not zero").
- **A notice travels with every export.** It names the files and their SHA-256, the filter, the
  definition and its ID, the unit, the method and the program's version and commit, and each
  public data set's license and citation. Data under a provider's terms of use (immuneACCESS:
  research use, no redistribution, §7.1) or under no stated license are exported only after their
  terms are shown and accepted, and the notice then says not to share the rows. Results computed
  from such data say to read the terms before sharing them.

### 5.8 Tracking and expansion (wave 3, planned)

- **Two samples, one clonotype at a time** (DeWitt et al. 2015, *J Virol* 89:4517,
  doi:10.1128/JVI.03474-14): with k₁ and k₂ the clonotype's cells or molecules in samples of
  N₁ and N₂, Fisher's exact test of the 2×2 table [[k₁, k₂], [N₁ − k₁, N₂ − k₂]], each clonotype
  tested on its own (the total change of the changed clonotypes assumed small). Conditional on
  the clonotype's total n = k₁ + k₂, this is the binomial test of k₁ ~ Binomial(n, θ) with
  θ = N₁/(N₁ + N₂), the share of the depth (Rytlewski et al. 2019's code uses this θ, though
  their text suggests the clonotype's pooled frequency; the depth's share is the conditional
  null, and the one that matches Fisher's test). DeWitt et al. controlled false discoveries by
  Storey's positive FDR (q ≤ 0.01 for induced clonotypes, 0.05 for putatively reactive ones;
  Storey 2002, *J R Stat Soc B* 64:479; Storey and Tibshirani 2003, *PNAS* 100:9440), on
  productive rearrangements, in templates.
- **Biological variance** (Rytlewski et al. 2019, *PLoS One* 14:e0213684,
  doi:10.1371/journal.pone.0213684): k₁ given n is beta-binomial, with mean θ and a scale
  v(n) = 2 + (b ln n + a)², β₁ = θv, β₂ = (1 − θ)v; the 2, which keeps the beta unimodal, is in
  their reference code and not in the paper's equation, and a reimplementation without it does
  not match. a and b are fitted by maximum likelihood (Nelder–Mead from (1, 1) in their code) on
  the joint counts of two replicate samples, or two samples two weeks apart; they publish no
  values to reuse. The two-sided p-value sums the beta-binomial probabilities of every outcome no
  more likely than the one seen; one-sided tests sum a tail. Their defaults: clonotypes by
  amino-acid junction, productive only, at least 5 in the two samples together (fewer are not
  tested, nor counted in the false discovery rate), two-sided, Benjamini–Hochberg at 0.01. Between
  technical replicates, the binomial test called about 1.2 clonotypes changed per pair, and 16 to
  19 over two to four weeks; the beta-binomial test, far fewer.
- **Time points** (built in slice 3, `internal/study`): read as days from the subject's reference
  event, as AIRR Repertoire metadata state them (`collection_time_point_relative`, its unit, and
  `collection_time_point_reference`, the event) and as studies write them (day 28, D+28, week 4,
  month 3, baseline); a month is a twelfth of 365.25 days. Samples of one subject at one time are
  replicates, which Rytlewski et al.'s variance model is fitted on.
- **Trajectories**: a clonotype's frequency in each sample with its 95% interval (Wilson's, for a
  count out of the sample's depth), the samples ordered by their time point; a clonotype absent
  from a sample is shown as below that sample's detection (one in its depth), never as zero.
- **Attribution**: the share of a sample's cells or molecules in clonotypes its subject's
  reference sample holds, under the definition, with an interval from a bootstrap of the sample.

### 5.9 Specificity (wave 4, planned)

- **TCRdist** (Dash et al. 2017, *Nature* 547:89, doi:10.1038/nature22383; as tcrdist3 computes
  it, Mayer-Blackwell et al. 2021, *eLife* 10:e68605, doi:10.7554/eLife.68605; tcrdist3 and its
  distance library are MIT, so their parameters may be read and matched). Two residues differ by
  min(4, 4 − BLOSUM62(a, b)), and a residue by 0 from itself; the only differences under 4 are
  AS 3, DE 2, DN 3, EK 3, EQ 2, FW 3, FY 1, HN 3, HY 2, IL 2, IM 3, IV 1, KQ 3, KR 2, LM 2, LV 3,
  MV 3, NS 3, QR 3, ST 3 and WY 2. A chain's distance sums four loops: CDR1, CDR2 and CDR2.5 (the
  loop between CDR2 and CDR3 that faces the peptide–MHC), each weight 1, and the CDR3, weight 3.
  - The V gene's three loops are strings of one length within an organism and chain, aligned
    with the gap character `.`, compared position by position. tcrdist3 encodes `.` (and `X`,
    `*`) outside its alphabet, and its matrix gives them 0 against anything: a gap opposite a
    residue costs nothing, not the gap penalty Dash et al. describe. Human loops are IMGT columns
    27–38, 56–65 and 81–86; tcrdist3's mouse α and β alignments place them at 28–39, 57–66 and
    82–88, so the loops are taken from its database as written, not recomputed.
  - The CDR3 (the IMGT junction) leaves out its first three and last two residues. Of equal
    length, its distance is the sum of the residues' differences. Of different lengths, the
    shorter is aligned to the longer's two ends with the Δ extra residues of the longer left out
    at one place, chosen where the sum of differences is least among positions 5 to (shorter
    length − 5), widened until there is one; each left-out residue costs 4. tcrdist3 multiplies the
    whole CDR3 term by 3 (3·differences + 12·Δ), its distance library's own default does not (4 per
    gap and a fixed gap position), and Dash et al. say 8: three numbers, and tcrdist3's is the
    one ClonoLoom matches.
  - Distances are integers (tcrdist3 sums them in 16-bit integers); a pair's is the sum of its
    chains'. tcrdist3 takes the single allele given; the original tcr-dist took the least
    distance over a gene's alleles.
- **Neighborhoods** (Mayer-Blackwell et al. 2021): a centroid's radius is the largest of
  0, 2, …, 48 at which its weighted background neighbors are at most a share 10⁻⁶ of the
  background (the paper's; tcrdist3's function defaults to 10⁻⁵); the background is 100,000
  receptors drawn from a recombination model with the antigen-enriched set's V and J genes,
  weighted by w = p(V, J) / q(V, J), the pair's share in the sample over its share in the draws,
  plus 100,000 cord-blood receptors of weight 1. Each centroid's report: its neighbors in the
  sample and the background, the weighted background rate, the relative rate
  ((n + 1)/(N + 2) over the background's), an odds ratio with one added to each count, and a χ²
  test (scipy's `chi2_contingency`, so with Yates's correction on the 2 × 2 table); a motif from
  the neighbors aligned to the centroid (ambiguity up to 3 residues in tcrdist3's code, 5 in the
  paper). Where no threshold holds the rate, tcrdist3 falls back to a radius of 0 and still counts
  the background at distance 0 (roadmap, wave 4, slice 5). ClonoLoom's backgrounds are its own
  (the paper's cord-blood set is not redistributable): receptors drawn from a recombination model
  with the dataset's V–J usage, weighted by the model's probability of the pair over the dataset's
  share of it, or the dataset's own chains with their CDR3s moved among its genes.
  The χ² is two-sided: a large value can describe depletion. ClonoLoom's ranking puts rates at
  least as large as the background first, by descending χ², then depleted neighborhoods by
  ascending χ². The reported statistic is unchanged. For a VJ model, drawing with named genes
  conditions the joint P(V, J) on those names; the alleles must not be drawn independently from
  their marginal probabilities. Neighborhood method version 2 records these corrections.
- **Generation probability** (Sethna et al. 2019, *Bioinformatics* 35:2974,
  doi:10.1093/bioinformatics/btz035; models in IGoR's format, Marcou et al. 2018, *Nat Commun*
  9:561). The probability of an amino-acid CDR3 (the junction, both conserved residues included)
  summed over every recombination event that makes it, by dynamic programming over its codons:
  P(V) P(D, J) (P(D | J) for T cells) and each segment's deletions (palindromic nucleotides as
  negative deletions), the insertions' lengths and a dinucleotide Markov chain for their
  nucleotides, the first one drawn from the chain's stationary distribution; the DJ insertion read
  from its J end. What OLGA does, and an exact reimplementation must too: genes not marked
  functional (`F` or `(F)`) or without an anchor contribute nothing and keep their share (P(V) is
  not made to sum to 1 again); only the Markov rows are normalized; every insertion length the
  model allows is summed, except that OLGA 1.3.0 leaves out an insertion of the longest length
  when that length is one more than a multiple of three and the insertion starts a codon (mouse
  TRA's 22 nucleotides; its own generator makes them, and ClonoLoom counts them: roadmap, wave 4,
  slice 3); a gene's palindrome is made from its part of the CDR3, at most as long as that part; a
  D deleted entirely only by left and right deletions that sum to its length; V and J
  each keep at least one nucleotide in the CDR3; the probability is over all recombination events,
  not only productive ones; V and J masks give joint probabilities. The default models' files carry
  no data license apart from the GPL-3.0 of their repositories (§7.1), and their versions differ
  (OLGA 1.2.4 and 1.3.0 give different IGH, IGK and IGL probabilities), so the model's version and
  checksum are part of every result.

### 5.10 Clonotype networks (wave 4)

The sequence-search foundation in `internal/network/pairs.go` compares whole junctions, including
their conserved anchors. Identity requires equality; Hamming counts substitutions between equal
lengths; Levenshtein permits substitutions, insertions and deletions, each of cost one
(Levenshtein 1966, *Soviet Physics Doklady* 10:707–710;
[original article, 1965](https://www.mathnet.ru/eng/dan31411)). The dynamic-programming recurrence
is the string correction recurrence of Wagner and Fischer (1974, *J ACM* 21:168–173,
[doi:10.1145/321796.321811](https://doi.org/10.1145/321796.321811)).

ClonoLoom's indexing choices, for cutoffs from 0 to 2:

- Identity: a hash of the whole junction.
- Hamming: k+1 disjoint segments at cutoff k; at least one must agree at its position. Lengths
  must agree too. Even junctions shorter than k+1 are covered (an empty segment gives candidates).
- Levenshtein: signatures made by deleting up to k symbols from each junction. Within k edits,
  the two junctions share at least one signature. This is only a candidate filter: the exact
  distance is checked by dynamic programming within the diagonal band reachable in k edits.

Pairs are streamed once in input-index order. The caller must discard partial output after an
error. Cancellation is checked during the search, and exceeding the index budget is an error,
never a truncated graph. Junctions must be uppercase sequence letters, from 1 to 256 symbols;
ambiguous letters are compared literally. The default budget is 20 million index postings.
These bounds are engineering choices, not biological thresholds. Unit tests compare every pair
of all binary-alphabet strings of lengths 1–6 with an independent full-matrix calculation, as
well as duplicate junctions, receptor examples, cancellation and budget failures.

Chain matching uses the maximum for `all`, the minimum for `any`, as
[scirpy's API](https://scirpy.scverse.org/en/v0.25.1/generated/scirpy.tl.define_clonotype_clusters.html)
describes for receptor arms. This differs from summing paired TCRdist distances in Neighbors.
ClonoLoom's choices are explicit: chains must have the same locus (IGK and IGL remain distinct);
`all` requires a bijection of every retained chain, with a minimum bottleneck found by bipartite
augmenting paths, while `any` needs one shared chain. Extra chains in an `all` cell definition
are retained, not reduced to the primary chain. Missing matches missing only under `all`.
Gene restrictions compare nonempty calls at the definition's gene or allele resolution; a
missing call cannot satisfy a same-gene restriction. Definitions at family resolution cannot
request a gene match. The scirpy comparisons use primary chains, not its default dual-IR policy.

TCRdist uses the established implementation (§5.9), with a length difference cost of 12 and the
V-loop distance as lower bounds. All clonotypes remain nodes: non-T-cell receptors and receptors
without required V loops are explicitly counted and left isolated for TCRdist. Loop checksums
and inferred alleles travel with the result. Sequence pairs are mapped back to their owning
clonotypes; candidate pairs are limited to 10 million and edges to 2 million. Every budget
failure discards the partial result. Connected components use disjoint sets with path compression
(Tarjan 1975, *J ACM* 22:215–225, [doi:10.1145/321879.321884](https://doi.org/10.1145/321879.321884)),
sorted by node count, then their first node in the stable clonotype table. Abundance sums follow
that order. Samples and annotated subjects are distinct sets, sorted before serialization.

The drawing is inspired by Fruchterman and Reingold (1991, *Software: Practice and Experience*
21:1129–1164, [doi:10.1002/spe.4380211102](https://doi.org/10.1002/spe.4380211102)), with ClonoLoom's
own fixed grid start, inverse-squared repulsion, linear edge attraction, bounded updates for
60 iterations and shelf packing with a width derived from the occupied area (target aspect 16:9). It draws components largest first, taking a prefix of an
oversized component when necessary, with all omitted nodes counted. The default is 1,000 nodes,
maximum 2,000; layout never changes scientific edges, memberships or exports. Coordinates carry
no scientific distance meaning. Products are rounded explicitly before accumulation, and layout
bytes are pinned on arm64 and amd64.

`validation/reference/scirpy_networks.py` runs scirpy 0.25.1 as a black box on eight 10x runs and
three OTS units. All 420 cell partitions agree: identity; Hamming and Levenshtein at 0, 1, 2;
TCRdist at 0, 12, 24, 48 on T-cell data; all/any arms, with/without V genes. OTS calls are
normalized to genes, matching the definition; TCRdist explicitly searches the gap and charges
12 per length difference, rather than scirpy's default of 4. At zero TCRdist, scirpy is asked
for cutoff 1 (distances are multiples of 3) to avoid its automatic substitution of identity.
On the simulated vaccination S01 day-14 repertoire, 97 observed members of 34 founder families
are all connected within their families at TCRdist 48 with same V and J. At two Hamming or
Levenshtein edits, 29 families connect and five split: the simulator's Poisson substitution
count is not bounded by two, and an unobserved intermediate cannot connect two nodes. Neither
family recovery nor connectedness proves a shared specificity or purity of the components.

`BenchmarkGraph100000` measures the whole sparse synthetic graph plus drawing: 0.14 s identity,
4.16 s Hamming 2, 2.97 s Levenshtein 2, 69.03 s TCRdist 24 on Apple M4. Dense inputs are subject
to the explicit budgets; the benchmark does not claim every threshold on every dataset finishes
within two minutes.

### 5.11 Specificity database evidence (wave 4, slice 7)

Read VDJdb's documented legacy `vdjdb.txt` by header names, preserving row identity, repeated
publication evidence, complex IDs, receptor and antigen species, MHC, score, method and reference.
The current publisher's [output documentation](https://github.com/antigenomics/vdjdb-db/blob/master/docs/outputs.md)
distinguishes the normalized primary bundle from the legacy table. The pinned historical
[2026-06-03 release](https://github.com/antigenomics/vdjdb-db/releases/tag/2026-06-03) already
contains that table (284,546 rows); the app downloads its table and license only on request.
Catalog terms link the current publisher license as well as retaining the historical archive's
license. No data or derived real-release fixture is bundled. Citation: Shugay et al. 2018,
Nucleic Acids Res. 46:D419–D427, doi:10.1093/nar/gkx760.

Match a retained chain at the same locus and explicit receptor species, exactly by whole CDR3
or within TCRdist (§5.9). Optional V/J matching compares identical nonempty call sets at the
clonotype definition's gene/allele resolution, without guessing absent alleles. TCRdist at zero
is not whole-junction identity. Missing required loops are counted and excluded, never silently
replaced by CDR3-only distances. A matched chain is evidence, not proof of the paired receptor's
specificity. MHC is evidence metadata; no sample HLA filter is implied. Complex IDs are retained
without claiming a paired match. Scores 0–3 are evidence categories, not calibrated probabilities.

An epitope/antigen/antigen-species group's sample count includes each matched clonotype once,
regardless of matching chains or repeated evidence. Divide by all retained sample abundance;
an empty denominator is null. Groups overlap. Tracking carries explicit IDs plus immutable
source-result and epitope provenance; existing list and display limits remain explicit.
At 100 million candidates or two million matches, fail rather than truncate the result.

Independent pandas 2.3.3 lookups agree on all record identities for 424 simulated and 4,000
real-release queries (V/J on/off; minimum score 0/2). All 150 simulated sample/epitope shares
agree. Exact V/J matching covers 602 molecules, 598 supported by the assigned epitope in truth,
out of 3,498 true response molecules. The database holds other donors' family members, so
coverage is incomplete and chance matches are possible; neither is concealed as perfect recovery.

## 6. Datasets

All are listed in `validation/sources.json` with their URL, sizes and SHA-256, and fetched by
`validation/fetch.mjs`. None is committed to the repository or embedded in the program.

| Name | Content | License | Use |
| --- | --- | --- | --- |
| `10x-pbmc-1k` | Human PBMC, about 1,000 cells, 5′ V(D)J, T and B; Cell Ranger 4.0.0 (contigs, all contigs, AIRR, clonotypes, consensus, metrics) | CC BY 4.0 (10x Genomics) | per-commit CI; reader and QC validation |
| `10x-mouse-splenocytes-1k` | C57BL/6 splenocytes, about 1,000 cells, T and B; Cell Ranger 4.0.0 | CC BY 4.0 | per-commit CI; mouse names |
| `10x-pbmc-10k-v2` | Human PBMC, about 10,000 cells, 5′ v2, Cell Ranger 6.1.0 multi (framework and CDR columns) | CC BY 4.0 | per-commit CI; Cell Ranger 6 layout |
| `10x-mouse-splenocytes-5k-gemx` | Mouse splenocytes, about 5,000 cells, GEM-X 5′ v3, Cell Ranger 9.0.0 (`sample` column) | CC BY 4.0 | per-commit CI; recent layout |
| `airr-standards-tests` | The AIRR Community's test and example files (v2.0.0) | CC BY 4.0 (AIRR Community) | per-commit CI; AIRR reader |
| `emerson-2017-immunoseq` | Three small samples of Emerson et al. 2017 (*Nat Genet* 49:659, doi:10.1038/ng.3822) in immunoSEQ v3 format, taken by HTTP range from the published 19.9 GB archive | immuneACCESS Terms of Use (research use, no redistribution) | maintainer validation only |
| `emerson-2017-adc` | The same cohort through VDJServer's ADC API as AIRR TSV | immuneACCESS Terms of Use | maintainer validation only |
| `vdjserver-ramos-libra` | One single-cell BCR repertoire of the LIBRA-seq study (Setliff et al. 2019, *Cell* 179:1636) through the ADC API | not stated | maintainer validation only |
| `dash-2017-tcrdist3` | Dash et al. 2017's epitope-specific paired TCRs as tcrdist3 distributes them: 1,924 mouse clones (3,405 cells) for seven epitopes, 415 human clones for three; tcrdist3 at a pinned commit | MIT (tcrdist3) | per-commit CI; the tcrdist3 reader; TCRdist against tcrdist3 (wave 4); an example |
| `ots-healthy-pbmc` | Three OTS paired units of healthy donors' blood (Sureshchandra et al. 2023, 4,424 cells; Bai et al. 2023; Poon et al. 2023) | CC BY 4.0 (OTS) | per-commit CI; the units' reader; an example |
| `oas-units` | Two OAS units: unpaired IgG heavy chains (Bashford-Rogers et al. 2013) and paired heavy and light chains (Eccles et al. 2020) | CC BY 4.0 (OAS) | per-commit CI; the units' reader |
| `pyclonal-mixcr` | Two bulk TCR clone tables of MiXCR 3, PyClonal's sample inputs | MIT (PyClonal) | per-commit CI; the MiXCR reader |
| `mixcr-benchmark-v4` | A single-cell B-cell clone table of MiXCR 4 (MiLaboratories' benchmark of 10x Genomics' 10k PBMC data) | not stated | maintainer validation only |

Later waves add: DeWitt et al. 2015's longitudinal TRB repertoires (ADC), scirpy's example
datasets (Wu et al. 2020, Maynard et al. 2020), and VDJdb (on request only).

## 7. Licenses

### 7.1 Reference data

| Resource | License (October 2026) | ClonoLoom |
| --- | --- | --- |
| AIRR-C germline sets (OGRDB) | CC0; immunoglobulin sets only (human, mouse strains, rhesus, *Peromyscus*, trout), as AIRR GermlineSet JSON with IMGT-gapped sequences and CDR delineations, and FASTA | may be bundled, with set id and release recorded |
| IMGT data (GENE-DB, V-QUEST reference directory) | CC BY 4.0 since 1 July 2026 (CC BY-NC-ND 4.0 before); IMGT's tools are under separate terms | downloaded on request (TR germlines come from here); retrieval date recorded |
| OLGA and IGoR recombination models | inside GPL-3.0 repositories, no separate license | downloaded on request only; Pgen implemented independently |
| Ensembl (GRCh38 at Ensembl's annotation) | no restrictions ("Ensembl imposes no restrictions on access to, or use of, the data provided") | the simulator's TRB gene segments are bundled (`internal/simulate/germline.go`), the release recorded |
| VDJdb | CC BY-NC-ND 4.0 since September 2026 (AGPL-3.0 and CC BY-ND 4.0 earlier in 2026) | downloaded on request only; never bundled or used for derived fixtures; non-commercial terms shown |
| McPAS-TCR | no license published | downloaded on request only |
| IEDB receptor data | CC BY 4.0 | downloaded on request |
| OAS, OTS | CC BY 4.0 | downloaded on request; small subsets may be used in CI |
| 10x Genomics datasets | CC BY 4.0 | downloaded on request; CI |
| immuneACCESS (Adaptive) | Terms of Use: research-only, no redistribution | downloaded on request with the terms shown; never redistributed |
| AIRR Data Commons repositories | no license stated per study | treated as unknown; maintainer validation only |
| AIRR schema and libraries | CC BY 4.0 | field names and types used with attribution |
| HGNC gene symbols | CC0 | human gene list |

### 7.2 Reference tools (validation only)

ClonoLoom copies no code from any of these; it runs them in `validation/reference/` and
compares outputs.

| Tool | License |
| --- | --- |
| airr (Python, R) | CC BY 4.0 |
| immunarch | Apache-2.0 (AGPL-3.0 before March 2024) |
| scirpy | BSD-3-Clause |
| scRepertoire | MIT |
| tcrdist3 | MIT (its copies of OLGA's models are GPL-3.0) |
| CoNGA | MIT |
| iNEXT | GPL-3.0 |
| OLGA, IGoR | GPL-3.0 |
| Change-O, SHazaM, Alakazam, SCOPer, Dowser | AGPL-3.0 |
| partis, immuneSIM | GPL-3.0 |
| GLIPH2 | binary only, no license published |
| MiXCR | proprietary, academic use only (kept out of public CI) |
| TRUST4 | MIT |

## 8. To verify before relying on it

- The behaviour of reference tools where publications leave choices open (bootstrap details of
  iNEXT, gap placement of tcrdist3), when each method's wave is planned.
- A mouse gene list (MGI) and its terms, before mouse immunoSEQ names are mapped fully.
- IMGT's written confirmation of CC BY 4.0 for its download folders, whose READMEs still carry
  the earlier wording, before any IMGT-derived file is bundled.

### 5.12 Invariant-population sequence rules (wave 4, slice 8)

The implementation reads published method descriptions, not external implementations.
Schattgen et al., Nat Biotechnol 2022;40:54–63, doi:10.1038/s41587-021-00989-2,
Methods, TCR sequence features, defines MAIT and iNKT gene/length rules. The full junction
includes its conserved cysteine and terminal F/W. Human MAIT: TRAV1-2 and TRAJ33/20/12,
length 12; mouse: TRAV1/TRAJ33, length 12. Human iNKT: TRAV10/TRAJ18/TRBV25, alpha
length 14–16; mouse: TRAV11/TRAJ18, length 15. IMGT's TRBV25-1 and the published TRBV25
name are accepted; mouse TRAV11 and its duplicated TRAV11D are accepted explicitly
(IMGT mouse TRAV allele table), not arbitrary prefix matches.

Human GEM uses TRAV1-2/TRAJ9 and CAV[RL].TGGFKTIF, as defined by Van Rhijn et al.,
J Immunol 2014;193:5338–5344, doi:10.4049/jimmunol.1401380, following the original
GEM receptors (Nat Immunol 2013;14:706–713, doi:10.1038/ni.2630, Fig. 3). The consensus
CAVRNTGGFKTIF is a positive test; the canonical human iNKT sequence CVVSDRGSTLGRLYF
is also documented by Musvosvi et al., Nat Med 2023, doi:10.1038/s41591-022-02110-9,
Extended Data Fig. 10. Additional published checks use human MAIT TRAV1-2/CAVKDSNYQLIW/TRAJ33 and mouse
MAIT TRAV1/CAVRDSNYQLIW/TRAJ33 (Ito et al., Sci Immunol 2024,
doi:10.1126/sciimmunol.ade6924, TCR transduction methods), and mouse iNKT
TRAV11/CVVGDRGSALGRLHF/TRAJ18 (Blood Adv 2020, doi:10.1182/bloodadvances.2019000977).
Other unit-test variants exercise constructed length/gene boundaries;
they are not claimed to be experimentally typed receptors. GEM's beta bias is not a rule.
No mouse GEM rule is asserted.

Species is explicit and saved, initially unevaluated. Definitions without amino-acid junctions
and gene/allele resolution cannot evaluate rules. Ambiguous genes are not resolved by choosing
a favorable candidate. An entirely matching retained chain suffices; otherwise incomplete
required evidence remains unknown. Human iNKT candidates require a retained beta chain.
Secondary chains discarded by the definition do not contribute. Each clonotype contributes
once per rule per sample; all retained abundance is the denominator, with evaluable abundance
reported separately. This annotation never establishes phenotype or antigen specificity.

On paired primary TRA/TRB clonotypes of cached 10x filtered T-cell contig tables, PBMC 1k has
10 MAIT-consistent cells / 406 retained (2.4631%; 357 alpha-evaluable), one iNKT and no GEM;
PBMC 10k v2 has 80 / 4,562 (1.7536%; 4,069 alpha-evaluable), two iNKT and no GEM.
The typical 1–10% range is of blood T cells, not all PBMCs (van Wilgenburg et al., Nat Commun
2018, doi:10.1038/s41467-018-07207-9). Both estimates lie within it. This is biological
plausibility, not phenotype ground truth; capture, donor and ascertainment differ. The test
reports departures rather than forcing every donor inside a typical range.

A separate standard-library CSV calculation (`validation/reference/invariant.py`, run with uv)
selects primary productive chains directly from Cell Ranger rows and applies the published
human predicates without calling ClonoLoom. All matched counts and both cell denominators
agree; the compact count reference is checked by `validation/invariant_test.go`.
Mouse nomenclature source: https://www.imgt.org/IMGTrepertoire/Proteins/taballeles/mouse/TRA/TRAV/Mu_TRAVall.html.


### 5.13 B-cell input evidence (wave 5, slice 1)

The first slice preserves annotations before interpreting them. Its contract is the
[AIRR Rearrangement schema 2.0](https://docs.airr-community.org/en/v2.0.0/datarep/rearrangements.html)
(accessed 23 October 2026), alongside the existing older-schema readers; Vander Heiden et al.
2018, *Front Immunol* 9:2206, doi:10.3389/fimmu.2018.02206. No external implementation's source
was used.

ClonoLoom retains 42 named fields as optional columns in a separate string store. Text is kept
as supplied by the existing TSV field reader, including invalid numbers, case, gaps and ambiguity
symbols. This is ClonoLoom's choice: conversion must not destroy the evidence that explains a
finding. Checked values are temporary. A missing column costs no per-row storage. Raw evidence
is not a license to use an invalid alignment in a later analysis.

Query, reference and aligned-string positions have separate bounds; region endpoints belong to
the query. No reference length is guessed. Intervals are checked before conversion to internal
slice bounds. AIRR CIGAR spans exclude numbering spacers and use its restricted terminal S/N
convention. Parsing limits accumulated lengths to signed 32-bit range and never expands a CIGAR
into an allocation. The orientation flag is preserved without reorienting supplied alignments.
Equal lengths and valid symbols are structural evidence only: numbering, germline correctness,
codon mapping and biological interpretation require the later analysis's checks.

Evidence warnings leave existing junction analyses available. The Inputs summary counts rows
under QC filters, not cells, clones or abundance. Presence of region coordinates is separate from
validity. Long sequences are fetched only for a selected row. Identity includes the source
checksum/name, its ordinal among the dataset's sources, source line and supplied sequence ID, avoiding collisions between repeated IDs.

The disposable table cache and reader versions advance to 2. The readers' identity version stays
at 1 because existing counts, calls, junctions and chain selection have not changed; previously
saved dataset and artifact IDs therefore remain valid. Future changes to those meanings must
advance identity separately. The retained annotations add no new scientific result artifact.

Independent Python CSV checks cover every retained value, including missing fields, in four
10x B-cell AIRR datasets and the artificial input example (`validation/reference/bcell_inputs.py`,
with its Python version and source checksums in `bcell-inputs.json`). Hand-calculated cases check
coordinate boundaries, orientation, incomplete evidence, CIGAR offsets and overflow. Round trips
check the cache and AIRR export, and release fixtures check existing results. These are input
validation checks, not a validation of germline reconstruction or somatic hypermutation.

### 5.14 IMGT/HighV-QUEST annotation input (wave 5, slice 2)

The primary contract is the [IMGT/V-QUEST output documentation](https://www.imgt.org/IMGT_vquest/user_guide),
consulted 2026-10-23, especially tables 1 (Summary), 2 (IMGT-gapped nt), 3 (nt sequences),
6 (Junction) and 11 (Parameters). The guide identifies the same columns for HighV-QUEST's
corresponding tabular files. Cite Brochet, Lefranc and Giudicelli (2008), *Nucleic Acids Research*
36:W503–508, doi:10.1093/nar/gkn316. The implementation is independently written from this
contract; no other implementation's source was inspected or copied.

ClonoLoom's choices are explicit: require the four per-sequence tables, match the pair of supplied
sequence number and sequence ID, and reject mismatched membership, duplicate numbers or conflicting
gene calls. Numbers locate annotations within a run; they do not replace the supplied sequence ID
or define a biological clone. A repeated ID with another number remains a distinct rearrangement.
One row counts one annotated sequence, including unsuccessful annotations; this asserts no read,
molecule or cell abundance. Unknown functionality remains missing, and tied calls remain sets.

Summary contains the sequence in sense orientation even for reverse submissions. Consequently,
ClonoLoom's retained/exported `sequence` is oriented and `rev_comp=F`; the original `Orientation`
sign stays in `imgt_annotations`. This avoids exporting an already reversed query with a flag
that would ask downstream consumers to reverse it again. Table 2 supplies a gapped query, not a
reconstructed germline. Its dots are retained without padding or expansion.

The guide says that insertion removal affects the other output tables while Summary retains
insertions in capitals. ClonoLoom therefore withholds all query coordinate mappings for reported
insertions/deletions. Otherwise each complete region must exactly match the indicated substring
of the oriented Summary (case-insensitive, 1-based closed). Failed or incomplete matches retain
native positions and a finding. This deliberately conservative rule does not claim that every
IMGT partial/indel output can be converted to AIRR query coordinates. No numerical mutation
statistics, reference sequence, CIGAR or N/P length is inferred. A restored frameshift translation
is never silently used as an un-restored AA junction.

The optional `imgt_annotations` evidence column holds bounded, per-row JSON with original calls,
functionality/comments, strand, indel annotations, junction restoration fields, native coordinates
and source table lines. Parameters remain text on their fingerprinted source record. All five
files participate in cache and dataset identity. Reader version 3 invalidates old caches; the
existing identity version and cache layout stay unchanged. Old non-IMGT results remain identical.
Joining indexes identities/offsets and spools bounded lines into private temporary files removed
on completion or error, so long annotations from four complete tables are not held in memory.

Validation uses constructed sequences generated by `testdata/make-imgt-inputs.mjs`, with independent
Python stdlib CSV joins (`validation/reference/imgt_inputs.py`) and hand-specified coordinate
eligibility. Source checksums and reduced expectations are committed. Cases cover shuffled tables,
repeated IDs, ties, reverse submission, corrected insertion, unsuccessful annotation, missing or
conflicting identities, malformed widths/headers, gzip, cancellation, cache invalidation and AIRR
round trips. Browser steps inspect the same cases and reopen the saved source groups. These are
format-contract tests on artificial data; no real IMGT-annotated cohort or IMGT algorithm accuracy
is claimed to have been validated in this slice.

### 5.15 Artificial B-cell families (wave 5, slice 3)

`internal/simulate/bcells.go` implements **ClonoLoom's own artificial benchmark**, version
`clonoloom-artificial-bcell-v1`. It supplies truth for later family, mutation and lineage tests;
it is not an implementation of a published germinal-center simulator. No external implementation
source or reference allele set is used. The complete probability rules, fixed topology, coordinate
geometry and parameter bounds are in `docs/SIMULATION.md` and each generated study's README.

The scientific boundary is deliberate. Yaari et al. (2013), “Models of Somatic Hypermutation
Targeting and Substitution Based on Synonymous Mutations from High-Throughput Immunoglobulin
Sequencing Data”, *Front Immunol* 4:358,
[doi:10.3389/fimmu.2013.00358](https://doi.org/10.3389/fimmu.2013.00358), establishes why a uniform
mutation proposal is not a calibrated model of SHM. We do not implement S5F, infer selection,
or use event counts as a biological mutation rate. Horns et al. (2016), “Lineage tracing of human
B cells reveals the in vivo landscape of human antibody class switching”, *eLife* 5:e16578,
[doi:10.7554/eLife.16578](https://doi.org/10.7554/eLife.16578), supplies biological context for
irreversible switching along the human constant-region order. The fixed M→G1→A1 subset here
asserts no measured switching probability, edge timing or inferred lineage. Both primary
publications were checked on 2026-10-23; neither paper's computational model is reproduced.

Each artificial family has a paired rearranged ancestor and six descendants in a fixed binary
tree. Uniform sense-codon backbones are shared by fictional V calls, with independent family
junctions; conserved junction anchors and reading frame are preserved. Accepted substitutions
are recorded in order, with silent/replacement classified against the codon at that event.
One edge deliberately reverses its parent's last event. This tests the distinction between edge
events and net ancestor differences, including a descendant with an identical sequence when
one-event branches reverse. Node IDs describe known states even when their sequences coincide.

The default three-subject study samples 240 cells at each of three times; each cell has IGH and
IGK or IGL observations. Family abundance is prescribed, and draws are independent categorical
samples. Probabilities and realized counts are separate; zero-count states remain in truth.
No read/UMI counts, ambiguous cells or observation error are fabricated. Family membership,
parents and cell-to-node assignments stay in `truth/`; opening the folder reads observations
and Repertoire metadata only. The supplied `germline_alignment` is the **known rearranged
ancestor including its junction**, not an inferred germline or a D-masked segment alignment.
SIM calls and artificial coordinate regions cannot be interpreted as biological reference
alleles or IMGT numbering. Pinned biological references remain wave 5, slice 5.

The Python stdlib reference checker replays every edge, verifies translation and event effects,
checks paired rows and artificial coordinates, recomputes closed-form probabilities and counts,
and verifies all output hashes. Its reduced sample counts are committed with its Python version
and source-file hashes. Go reads the same default study and agrees on all 4,320 rows, 2,160 cells,
isotypes, paired junction clonotypes and net ancestor differences. The 252 states hold 998
replacement and 298 silent events, including 72 deliberate reversals. This is agreement on
artificial truth, not biological validation. A separate fixed-seed sampling check compares
180,000 draws against their categorical probabilities. Byte digests cover both processor
architectures and retain the pre-existing TRB digests. Bounds, parameter serialization,
cancellation, no-overwrite behavior, offline generation and guided inspection are tested.


### 5.16 B-cell clonal families (wave 5, slice 4)

The implemented method follows the published V/J/junction-length partitioning and normalized
nucleotide Hamming/single-linkage framework in Gupta et al. (2015), *Change-O: a toolkit for
analyzing large-scale B cell immunoglobulin repertoire sequencing data*, Bioinformatics
31:3356–3358, [doi:10.1093/bioinformatics/btv359](https://doi.org/10.1093/bioinformatics/btv359),
and Gupta et al. (2017), *Hierarchical Clustering Can Identify B Cell Clones with High Confidence
in Ig Repertoire Sequencing Data*, J Immunol 198:2489–2499,
[doi:10.4049/jimmunol.1601850](https://doi.org/10.4049/jimmunol.1601850).
The primary publications and public [Change-O method documentation](https://changeo.readthedocs.io/en/1.3.1/methods/clustering.html)
and [SHazaM distance documentation](https://shazam.readthedocs.io/en/stable/topics/distToNearest/)
were checked on 2026-10-23. No external implementation source was inspected or copied.

ClonoLoom independently computes mismatches divided by nucleotide junction length. V/J calls
must resolve to one gene; ambiguous alleles of that gene are acceptable, unreadable calls are
not. Junctions must be nonempty, productive, ACGT-only, a multiple of three and at most 1,000
bases. The QC filter acts first. Subject scope requires explicit metadata/annotations; missing
subjects are excluded, never pooled as an anonymous subject. Explicit sample scope is available.

Paired mode is a **ClonoLoom policy**, not claimed equivalence to Change-O or SHazaM pairing:
exactly one distinct heavy and one distinct IGK/IGL chain per sample-scoped cell, with duplicate
identical chains collapsed. Multiple or invalid chains exclude the observation. Each chain must
match its own locus/V/J/length partition; distance is the larger of the two normalized Hamming
distances. Thus both chains must support an edge. Auto selects paired mode whenever the input
table has cell IDs. Explicit heavy mode ignores light chains but still counts cells once.

Each distinct compared receptor per scope contributes once to the distance-to-nearest histogram;
identical observations retain their membership and abundance separately. A singleton partition
has no comparable neighbor, recorded as null rather than zero. No mixture fit or automatic
threshold is supplied. A user-selected threshold joins edges at distance ≤ threshold; connected
components implement single linkage. Paths can join endpoints exceeding that threshold. Zero
joins exact receptors; null computes distances without assigning families. A threshold suitable
for the artificial example is not a biological recommendation. Inferred families neither prove
ancestry nor replace junction clonotypes.

The complete result records the filter, subjects, mode, threshold, source names/checksums/lines,
sequence and cell IDs, exclusions, nearest neighbors and per-sample family abundance. Missing
bulk abundance remains null; cells contribute one regardless of contig counts. Deterministic
sorting fixes IDs, tie breaks and summation. All-pairs comparisons are bounded to 200 million
base comparisons and 250,000 observations; exceeding either refuses the job without subsampling.
Cancellation is checked during grouping and comparisons. UI pages are bounded independently of
complete exports. Germline reconstruction, SHM and lineage inference remain separate slices.

An independent Python standard-library CSV reader and breadth-first adjacency-graph calculation
checks all 2,160 artificial cells at four thresholds in both modes. Digests cover every membership
and nearest float bit pattern; source hashes and Python version accompany the reference. Heavy
mode has 84 unique receptors; paired mode has 133. At threshold 0.05 these yield 38 and 43
families; at 0.1 both recover the exact 36 true families. Truth is read only after inference for
evaluation. These results do not establish performance on biological samples or recommend a
threshold, and no fitted SHM model is implied.

### 5.17 D-masked germline reconstruction (wave 5, slice 5)

**Sources checked 2026-10-23.** Gupta et al. (2015), *Change-O*,
[doi:10.1093/bioinformatics/btv359](https://doi.org/10.1093/bioinformatics/btv359), and its
[public CreateGermlines documentation](https://changeo.readthedocs.io/en/stable/tools/CreateGermlines.html)
describe reference reconstruction and masking the uncertain D/N/P region. The
[AIRR Rearrangement schema 2.0](https://docs.airr-community.org/en/v2.0.0/datarep/rearrangements.html)
defines query orientation, 1-based closed intervals and CIGAR offsets/operations.
Only publications, documentation and licensed reference data were consulted, not another
implementation's source code. This method is independently implemented in Go.

**ClonoLoom's choices.** A saved, explicitly thresholded family result defines membership and
subject/sample boundaries. Within each family/locus, intersect every member's explicit V/J
allele calls. Exactly one common allele per segment must remain and exist in the selected
reference. Conflicts, unresolved ties, missing calls and gene-only calls are exclusions, not
votes or opportunities to guess a genotype. This differs from Change-O's clone consensus:
ClonoLoom does not use majority abundance, choose the longest representative, infer deletions
from junction length, or transfer an inferred unmutated junction to other members.

The raw query must be ungapped IUPAC nucleotide sequence with known orientation. Reverse
complement it once when `rev_comp` is true. Project each selected ungapped V/J segment using
its AIRR CIGAR: leading S/N establishes query/reference offsets, M or =/X consumes both,
I consumes query only, D consumes reference only. Supplied sequence/germline endpoints must
agree with those offsets and consumed spans. Check bounds before allocation; =/X assertions
are checked where both symbols are unambiguous. Optional trailing S/N do not create columns.
V and J query spans must be ordered and nonoverlapping. Query insertions and reference
insertions relative to query remain '-' in the other alignment. Ambiguity symbols remain
ambiguous. All intervening query bases get reference N, including light-chain junctions.

Output is a new unnumbered alignment with V/J/N segment labels and per-column 1-based oriented
query/reference coordinates. A reference coordinate belongs to its labeled segment; zero
marks no reference position at query insertions/masked junctions, or no query position at
reference deletions. Common family allele choices accompany individual member projections,
which may have different lengths. No padding or cross-member sequence consensus is invented.
The input's supplied germline alignment is never used as the reference. Names/lengths alone
cannot prove the annotation used this reference; the user must choose compatible segment
boundaries. Native IMGT gapped coordinates and missing CIGARs are unsupported in this slice.

**References and provenance.** The human catalog contains OGRDB AIRR-C IGH_VDJ v10,
IGKappa_VJ v5 and IGLambda_VJ v4, released 27 May 2026, from version-specific `ungapped_ex`
endpoints. The 240/84/97 records include D alleles; reconstruction uses only V and J.
Files are pinned by byte count and SHA-256 in `validation/sources.json`, downloaded only on
request, and rehashed when preparing and running each job. OGRDB's
[licensing statement](https://ogrdb.airr-community.org/render_page/licensing_statement.html)
places this reference information under CC0; website software is not part of these data.
The reference-set paper is [AIRR-C IG Reference Sets](https://doi.org/10.3389/fimmu.2023.1330153).
Release records: [IGH v10](https://doi.org/10.5281/zenodo.20409587),
[IGK v5](https://doi.org/10.5281/zenodo.20409594),
[IGL v4](https://doi.org/10.5281/zenodo.20409602).

Artifacts record the parent family checksum, reference files/checksums/license/citation and
method version. Resume rejects changed bytes. Inspection of completed results needs neither
open source data nor a downloaded reference. Exports retain complete rows, exclusions and
coordinate maps, with FASTA limited to successful projections. Counts are source rows, not
cells or abundance. Query/reference segments are bounded at 10,000 bases, reference input at
10 MiB and reconstructed output at 50 million columns; limits fail without subsampling.

**Artificial model v2 and validation.** The v1 simulator had distinct J backbones under one
fictional J name. Model v2 gives each backbone its own J name, exports the independent V/J
segments, and supplies explicit CIGARs and germline coordinates. Simulated sequences, mutation
events and sampling are unchanged. Older datasets/results remain readable. Independent
CSV/FASTA slicing (`validation/reference/germlines.py`, Python 3.13.3 via uv) checks all 4,320
projections and every coordinate against generative ancestors outside the 33 masked junction
bases. All 72 family/locus groups reconstruct; the existing independent graph calculation
still recovers 36 families and the same nearest distances. Truth is not an inference input.
Hand-derived tests cover clipping, offsets, reverse orientation, I/D operations, ambiguity,
conflicting/tied calls, inconsistent coordinates, missing evidence, checksum changes and
hostile lengths; fuzzing exercises the projection boundary.

The public Ramos AIRR file gives one eligible heavy row and one successful projection with
the pinned human reference. The 10x PBMC 1k/10k-v2 AIRR examples contribute 56/437 eligible
heavy rows; all are correctly excluded for gene-only calls. These are format and coverage
checks, not biological truth or a claim that the selected set matches every original
annotation. SHM denominators, codon changes, selection and lineage inference remain later slices.

### 5.18 Somatic hypermutation: observed divergence and codon attribution (wave 5, slice 6)

**Sources consulted.** Gupta NT et al. (2015), *Change-O: a toolkit for analyzing large-scale
B cell immunoglobulin repertoire sequencing data*, Bioinformatics 31:3356–3358,
[doi:10.1093/bioinformatics/btv359](https://doi.org/10.1093/bioinformatics/btv359), establishes
the D-masked sequence/germline comparison framework already used in §5.17.
The public [SHazaM observedMutations documentation](https://shazam.readthedocs.io/en/stable/topics/observedMutations/)
describes amino-acid identity for R/S and non-N denominators, with its own ambiguity modes and
IMGT-aligned regional definitions. We consulted documentation only, not implementation source.
ClonoLoom does **not** reproduce those modes or claim numerical equivalence to SHazaM.
[AIRR Rearrangement schema 2.0](https://docs.airr-community.org/en/v2.0.0/datarep/rearrangements.html)
defines region coordinates on the query and distinguishes CDR3 from the junction including its
flanking conserved residues. Translation uses [NCBI genetic code table 1](https://www.ncbi.nlm.nih.gov/Taxonomy/Utils/wprintgc.cgi#SG1),
through the existing independent `internal/seq` implementation. Consulted 23 October 2026.

**ClonoLoom's choices.** No empirical selection model is fitted. Nucleotide divergence is the
number of unequal, aligned, unambiguous A/C/G/T pairs divided by all such pairs. The stored
parent's D/N/P mask is excluded first; gaps and remaining ambiguous symbols are separately
counted. Even an IUPAC pair that could agree is excluded. A zero denominator yields a missing
frequency. Counts compare final sequence with reference; reversals and repeated changes cannot
be reconstructed from these endpoints. They do not establish a rate or mutation history.

A saved germline parent supplies immutable family membership, V/J projections and coordinate
maps. Its file SHA-256 is checked when preparing and running an analysis. The result preserves
family and reference hashes, source, license and citation without needing another download.
Original source hashes/line/sequence ID identify each row; altered or duplicate identities fail.
Every parent row remains represented, including reconstruction exclusions. Saved SHM inspection
needs neither the parent file nor the original dataset; new computation needs both.

Regions are the input's 1-based closed intervals on the oriented query, not fixed positions in
the reconstructed alignment. Missing intervals remain unavailable. A malformed, partially
supplied, overlapping or out-of-order interval invalidates the regional partition, while `all`,
`V` and `J` comparisons remain valid. Coordinates outside the reconstructed coverage do not
create bases. Unassigned query positions stay in totals; reference deletions have no query
position and no regional gap assignment. Segment and region totals overlap and must not be
summed. The CDR3 label itself does not impose a mask: V/J-covered CDR3 positions remain eligible
if the saved reference supplies them.

An ungapped reference FASTA does not establish codon phase. Codon comparisons require explicit
`vj_in_frame=true`, a valid whole-codon CDR3 interval, exact query/junction agreement including
the two flanking codons, and unambiguous translation with C and F/W anchors and no junction stop.
The CDR3 start fixes query phase. This conservative input-consistency check does not independently
validate the annotator's frame assignment. Any indel in the projection suppresses all codon
inference, including in-frame indels; inferring how codons span them is deferred. Complete paired
A/C/G/T codons must occupy consecutive query and reference positions within one V/J segment;
regional codons also fit entirely in the interval. Truncated, masked or ambiguous codons are
omitted. Stop-containing paired codons are separately counted and omitted from the codon
denominator. Their otherwise valid nucleotide pairs still count toward base divergence.

For single-hit evaluable codons, amino-acid identity defines silent versus replacement
**substitutions**. Multi-hit codons have an endpoint amino-acid classification, recorded as
replacement/silent **codons**, but their individual substitutions remain unclassified. We do
not choose an order of changes or distribute fractional attribution across hypothetical paths.
Unclassified substitutions also include differences outside evaluable codons. They remain in
the nucleotide numerator. `replacement + silent + unclassified = substitutions`; endpoint codon
counts are a different unit. Zero classified substitutions with no evaluable codons is not
biological absence of mutation. Each row records codon exclusions and the phase anchor.

Summaries pool source-row counts by sample, locus and region, divide pooled substitutions by
pooled evaluable bases, and show available-row/codon-row coverage. They do not average per-row
frequencies, weight by abundance, collapse contigs, combine heavy/light denominators or treat
rows as independent subjects. No confidence intervals, selection statistic, family inference,
lineage, error correction or group-level inference is introduced.

**Independent validation.** `validation/reference/shm.py` uses only Python stdlib, known root
sequences, final observed sequences, NCBI table 1 and the fixed artificial region contract.
It does not consume the Go reconstruction or its SHM result. Every one of 43,200 row/region
records is pinned by a digest; original TSV hashes and Python version accompany it. All 4,320
rows have 327 evaluable bases and 109 evaluable codons. Pooled totals: 9,941 substitutions /
1,412,640 bases, 7,297 single-hit R, 2,360 single-hit S, 284 unclassified substitutions in 142
multi-hit codons. Endpoint codons: 7,439 R and 2,360 S; 470,880 codons evaluated. The ancestor
comparison naturally nets deliberate reversals rather than summing historical events.

Analytic tests use GCT/GCC (one silent), AAA/GAA (one replacement), TTA/CTG (two-hit synonymous
endpoint), TTT/GGG (three-hit replacement endpoint) and AAA/TAA (stop exclusion). They cover
clipping without resetting phase, reverse orientation, partial regional codons, ambiguity,
indels, hostile bounds, missing frame, conflicting junctions, source identity and cancellation.
This validates the stated conservative rules and artificial bookkeeping, not biological
annotation accuracy or selection inference. Native IMGT and gene-only inputs still require the
unsupported reconstruction evidence identified in §5.17; this slice does not fill it in.

The pinned public Ramos AIRR row supplies evaluable bases and codons. The 56 and 437 assigned
heavy rows in the 10x PBMC 1k and 10k-v2 fixtures remain excluded for gene-only calls inherited
from germline reconstruction; SHM does not convert those exclusions into zero-mutation rows.
These are coverage and provenance checks, not independently known biological mutation truth.

### 5.19 Lineage trees: bounded parsimony and conditional ancestors (wave 5, slice 7)

**Sources consulted.** Fitch WM (1971), *Toward Defining the Course of Evolution: Minimum
Change for a Specific Tree Topology*, Syst Zool 20:406–416,
[doi:10.1093/sysbio/20.4.406](https://doi.org/10.1093/sysbio/20.4.406), supplies the fixed-topology
minimum-change framework. Sankoff D (1975), *Minimal Mutation Trees of Sequences*, SIAM J
Appl Math 28:35–42, [doi:10.1137/0128004](https://doi.org/10.1137/0128004), supplies the
sequence-state cost framework. Hoehn KB, Pybus OG, Kleinstein SH (2022), *Phylogenetic analysis
of migration, differentiation, and class switching in B cells*, PLoS Comput Biol 18:e1009885,
[doi:10.1371/journal.pcbi.1009885](https://doi.org/10.1371/journal.pcbi.1009885), is context for
keeping observed traits distinct from inferred transitions. We implement no trait-transition
statistic from that paper. Consulted 23 October 2026. Only papers and public method descriptions
were consulted; no external tree implementation source was examined or copied.

**ClonoLoom's policies.** Each immutable germline parent's family/locus is analyzed separately.
Its SHA-256 and original source identities are checked before computation. The selected V/J
alleles and per-column reference coordinates define homology. A complete-case panel retains
only positions with paired A/C/G/T in every eligible row and a consistent reference base.
Positions are ordered V then J, each by 1-based ungapped reference position. D/N/P, insertions,
deletions, ambiguity and missing coverage do not become character states. `omitted` counts the
union of mapped V/J reference positions minus this intersection; it does not count query
insertions or masked junction bases. One poorly covered row can shorten the whole panel.
No shared sites is an exclusion, not a zero-change tree. No full V(D)J ancestor is fabricated.

Identical panel sequences collapse even when the source sequences differ outside the panel.
All source identities and observed C calls, samples, timepoints and tissues remain attached.
Rows matching the reference attach to the reference root. Distinct states, including that root,
are the taxa; abundance does not weight the search. Reference root placement is an assumption,
not evidence for a sampled ancestor. Heavy and light trees are not concatenated or reconciled.

At most eight taxa are searched by inserting each successive tip into every edge of each
unrooted binary tree. This visits each labeled topology once, up to 10,395 at eight taxa.
Equal-cost unordered nucleotide changes are scored with Fitch sets, compressed by site pattern;
constant columns contribute zero. The reference is a fixed tip used to orient the output root.
Among globally minimal trees, choose the lexicographically smallest canonical rooted subtree
encoding on sorted panel states. Record the number of tied optimal binary trees and the count
containing each selected split. Counts include zero-edge binary resolutions. They are neither
bootstrap support, posterior probabilities, nor an assertion that the selected resolution is
biologically identifiable. The one-state tree is just its reference node.

Nine through 64 taxa use stepwise addition in sorted, reverse-sorted and decreasing
reference-distance order, each followed by strict-improvement nearest-neighbor interchange.
Equal-score candidates use the same canonical tie break; equal-score plateaus are not traversed.
At most 50,000 candidate scores and 200 million estimated pattern-node work units are allowed.
A candidate's conservative work charge is `max(1, variable patterns) * max(1, 2*taxa-2)`,
including partial addition trees. Exact enumeration must fit this bound in full. The heuristic
requires capacity for the three addition passes before starting; NNI may exhaust the remaining
budget and prevent later passes. `budgetLimited` records early termination. The reported
`examined` counts addition and NNI candidate scores, not bookkeeping rescoring of incumbents.
Heuristic scores are not proven global minima and have no exhaustive tie/split counts. More
than 64 states or a work refusal excludes that group without silently subsampling or losing
other groups. These search limits and algorithms are our own implementation choices.

Conditional on the selected topology, four-state inside/outside costs enumerate each node's
marginal optimal states at every panel site. IUPAC symbols encode these sets. They do not encode
a joint assignment: arbitrarily combining allowed symbols can increase total cost. Separately,
a top-down pass chooses one consistent minimum-cost assignment, preferring the parent's state
then A/C/G/T. Integer edge lengths count changes in that assignment, including zero lengths.
They are not mutation rates, times or all historical events. Inferred nodes may repeat observed
states. The viewer draws topological depth, labels each edge's chosen change count, and exposes
all nodes in an accessible table; Newick exports the same lengths and stable node IDs.

Observed annotations never constrain topology or assign traits to unobserved nodes. Project
sample annotations override imported metadata and are normalized and pinned in analysis identity;
subject/reference flags are irrelevant to this derived analysis. Editing annotations requires a
new result to change labels; saved inspection remains historical. Mixed or missing values stay
explicit. This slice infers neither class switching, migration, event dates, selection, affinity
nor likelihood trees. Context-dependent SHM, alignment uncertainty, sequencing errors and family
threshold uncertainty are not modeled. Biological interpretation depends on those upstream inputs.

**Independent validation.** `validation/reference/lineages.py` reads only known artificial
families, root sequences and observed TSV sequences. It enumerates rooted bipartitions and
scores them with independent four-state dynamic programming, rather than the production edge
insertion and Fitch scorer. Pinned source hashes and membership digests connect its results to
the Go family/germline pipeline. All 72 default-v2 family/locus trees agree in panel size (327),
taxa, minimum score and optimal-topology count: 1,093 changes total; 71 searches have ties.
Each score is at most the known generating tree's panel score. This is not a claim of recovering
the generating topology: unobserved states, reversals and ties make that unidentifiable here.
Small analytic trees check topology counts and brute-force every internal state assignment to
verify marginal ambiguity and the consistent assignment's score. Tests also cover heuristic
repeatability, cancellation, incomplete panels, metadata mixtures, parent/source identity,
explicit exclusions, work limits and complete exports. Artificial agreement validates the
implemented objective, not biological phylogeny accuracy.

The public Ramos AIRR row produces one supported tree; the 56/437 gene-only heavy rows in the
10x PBMC 1k/10k-v2 fixtures retain upstream reconstruction exclusions. This checks format and
evidence coverage, not reference suitability or biological lineage accuracy.


### 5.20 Isotypes: observed usage and family co-occurrence (wave 5, slice 8)

**Sources and boundary.** AIRR Rearrangement schema 2.0 defines `c_call` as the constant-region
gene call, including its allele when available ([schema](https://docs.airr-community.org/en/v2.0.0/datarep/rearrangements.html)).
IMGT nomenclature supplies the gene labels (Lefranc 2014, *Front Immunol* 5:22,
[doi:10.3389/fimmu.2014.00022](https://doi.org/10.3389/fimmu.2014.00022)); the
[IMGT mouse IGHC table](https://www.imgt.org/IMGTrepertoire/LocusGenes/genetable/autotable.php?group=IGHC&species=Mus+musculus)
provides context for labels such as IGHG2A/B/C. No reference sequences are required or copied.
Horns et al. (2016), “Lineage tracing of human B cells reveals the in vivo landscape of human
antibody class switching,” *eLife* 5:e16578,
[doi:10.7554/eLife.16578](https://doi.org/10.7554/eLife.16578), supplies biological context.
Hoehn, Pybus and Kleinstein (2022), “Phylogenetic analysis of migration, differentiation, and
class switching in B cells,” *PLoS Comput Biol* 18:e1009885,
[doi:10.1371/journal.pcbi.1009885](https://doi.org/10.1371/journal.pcbi.1009885), distinguishes
observed labels from phylogenetic trait inference. Neither paper's inference algorithm or
statistic is reproduced. The following counting and ambiguity rules are ClonoLoom policies.

**Parent and unit.** A saved, thresholded family result fixes the observation identity,
assignment, QC exclusions, subject boundaries and abundance. Isotypes do not require alignments,
germline reconstruction or a tree: usable C calls can exist without that evidence. Each
assigned observation contributes once: a sample-scoped cell for single-cell inputs, or a heavy
source row for bulk inputs. All parent-retained source rows and raw C calls remain inspectable. Light
calls never determine isotype. Multiple retained heavy contigs do not multiply cell counts.

**Resolution.** Use the existing gene parser, remove alleles and normalize case. Supported
heavy constant labels are IGHM, IGHD, IGHE, IGHG, IGHG1–4, IGHG2A/B/C, IGHA and IGHA1/2.
The output has separate class and constant-gene resolutions. A gene label does not establish
species, ploidy, genomic order or a functional protein. Pseudogenes, other loci/segments and
unrecognized tokens are unsupported. Calls over 4,096 bytes are unsupported, with raw evidence
retained. The parser's documented separators and missing-token handling remain unchanged. A stricter
constant-call suffix check accepts numeric alleles and optional finite parenthesized scores;
malformed suffixes cannot conceal a second candidate (for example `IGHM*01/IGHD*01`).

All retained heavy rows must agree at the reported resolution. Alleles of one gene collapse;
ties between IGHG subclasses retain IgG class but no gene. Broad IGHG/IGHA labels retain class
only. A missing heavy call alongside a resolved call remains unresolved. An unsupported token
makes the observation unresolved even if another token is valid. Multi-class candidates,
including IGHM/IGHD, remain unresolved; a call tie is not evidence of coexpression. One primary
issue is reported in order: unsupported, missing, conflicting classes, conflicting subclasses,
class-only call. Full raw calls permit inspection of any additional limitations.

**Usage and missingness.** Calculate counts and shares separately per sample and per
family/sample at both resolutions. The denominator is all assigned observations in that group,
including unresolved calls. Parent-excluded observations are retained but outside the denominator.
Only observed buckets are emitted; an absent sample/group is not a measured zero. Observation units (cells or heavy-chain rows) are separate from the parent's abundance unit
(for example reads). In single-cell
parents abundance is one per cell; in bulk it is the parent's count or missing. Any missing count
makes that bucket's abundance and the group's abundance denominator unavailable, rather than
summing just known values. Other fully observed bucket totals remain available but have no
abundance share when the denominator is missing. A known zero denominator is zero with missing
share. Negative, non-finite or overflowing assigned abundance fails explicitly. Observation
shares remain defined independently of abundance. Sorted observation IDs fix floating-point
summation order. Labels use pinned sample timepoint/tissue annotations; no inferential comparison
or pooling of timepoints, tissues or subjects as independent replicates is performed.

**Families and co-occurrence.** Report observations, resolved-class/gene coverage, samples and
sorted observed labels per family. Count each unordered pair of resolved constant genes once
per family, regardless of cell abundance. Report all families and those with at least one
resolved gene alongside pair counts; the latter is coverage, not proof of complete sampling.
Pairs overlap and are not additive. IgM/IgD-only co-occurrence is explicitly not switching
evidence. No pair establishes a transition, its direction, timing, ancestor, rate or significance.
The slice does not reconstruct class-switch events, assign ancestral traits, impose genomic
ordering or apply the lineage search's chosen topology to C calls. Results depend on input
annotation accuracy, sampling and the selected family threshold.

**Independent validation.** `validation/reference/isotypes.py` uses only Python's standard
library, raw AIRR rows and the simulation's generating-family truth. It checks one heavy row per
sample/cell, compares each C label with its sampled truth node, then directly tallies classes,
genes and unordered family pairs. It does not call the Go parser, family or isotype implementation.
Source hashes and heavy-member digests connect the independent reference to the production
pipeline. All 2,160 default-v2 cells, 36 families and 466 sample/family usage rows agree, including
counts, denominators and observation/abundance shares. Each of the three gene pairs occurs in
36 families. A separate complete-result digest checks arm64/amd64 serialization. The simulated
calls are unambiguous; hand-calculated tests separately cover ambiguous, missing, unsupported,
duplicate-contig and bulk missing/zero-count cases. This checks bookkeeping, not biological
accuracy or recovery of the simulator's historical switching events.

Public-data coverage checks also resolve the Ramos observation and all 56/437 assigned
observations in the two pinned 10x PBMC fixtures. These C calls remain usable even where
V/J allele evidence is insufficient for germline reconstruction. This tests evidence
availability, not known biological class-switching truth.

### 5.21 MEX expression evidence and exact barcode linkage (wave 6, slice 1)

**Formats.** [10x Genomics, Cell Ranger Feature Barcode Matrices (MEX)](https://www.10xgenomics.com/support/software/cell-ranger/latest/analysis/cr-outputs-mex-matrices)
defines the feature/barcode companions and their orientation.
[Boisvert et al., Matrix Market Exchange Formats, NISTIR 5935 (1996)](https://math.nist.gov/MatrixMarket/formats.html)
defines coordinate matrices (doi:10.6028/NIST.IR.5935). Read feature × barcode integer/general counts, preserving feature
IDs, names, modalities, barcode suffixes and explicit zero coordinates. Legacy genes.tsv has
two columns and denotes Gene Expression; modern features.tsv has three. No expression value is
computed in JavaScript. No normalization, graph, phenotype or biological inference is introduced.

**ClonoLoom choices.** Keep a cell-compressed sparse matrix separate from receptor rows. Reject
ambiguous companions, duplicate IDs/barcodes/coordinates, unsupported encodings and noninteger
or negative counts. Do not sum repeated coordinates or choose one gene with a repeated name.
Keep all modalities; gene totals and detected genes include only Gene Expression. With no gene
features these measurements are null in the API. Retain measured zeros separately from missing
barcodes. Bound counts and gene totals by 2^53−1 so JSON consumers preserve integers. Reader
limits are documented in FORMATS.md. Stream and bound actual input before allocation; headers
never determine a large allocation alone. Source bytes, role and reader version determine ID.

Link against an explicitly selected receptor dataset/sample using exact barcode strings. Count
distinct barcodes, not contigs; report chain rows, missing-barcode rows and both unmatched
populations. This is before QC: nonproductive or low-confidence rows still contribute to raw
coverage. No stripping of GEM-well suffixes, case folding, cross-sample fallback or assumption
that a barcode is a productive singlet. These are data-handling policies, not a published test.

**Sibling review.** SituCraft's cell_feature_matrix MEX reader uses a sparse representation and
its AnnData reader supports Zarr. Its HDF5 code writes files; the roadmap's presumed existing
HDF5 reader does not exist. ClonoLoom's streaming MEX parser is its own implementation. The
HDF5/AnnData/MuData slices will use primary format documentation and independently produced
fixtures, preserving the standard-library-only runtime.

**Validation.** The artificial three-feature/four-barcode example has duplicated gene names,
an antibody feature, unordered coordinates, an explicit zero and a zero-count barcode. SciPy
1.17.0 `scipy.io.mmread` independently reads the dense 3×4 reference and gene totals (5, 4, 7, 0).
Python sets tally two matches, two expression-only and one receptor-only barcode; one receptor
row lacks a barcode. Files and values are pinned in `validation/reference/expression.json`,
with the script beside it. Hand cases cover sample collisions, repeated chains, malformed
counts, dimensions, duplicates, gzip corruption, cancellation and limits. Fuzzing probes the
reader. This validates format and bookkeeping, not biological cell calling or phenotypes.

### 5.22 Known cell states, annotations and imported maps (wave 6, slice 2)

**Purpose.** A controlled paired TRA/TRB and count-matrix study supplies known clone/state
relationships and explicit missing evidence for the rest of wave 6. Model
`clonoloom-artificial-cell-states-v1` uses three artificial states, 24 genes, uniform clones,
state-dependent negative-binomial counts and supplied illustrative coordinates. All means,
dispersion, sample effects and missingness cases are project choices; this is not an inferred
model of single-cell biology. The complete distributions, seeds, stream order and output
contract are in `docs/SIMULATION.md`. Fictional junctions and SIM calls use no external alleles.

**Methods.** Reuse the project's deterministic gamma–Poisson random sampler. Gamma generation:
Marsaglia and Tsang (2000), *ACM TOMS* 26:363–372,
[doi:10.1145/358407.358414](https://doi.org/10.1145/358407.358414). Poisson generation: Hörmann
(1993), *Statistics & Probability Letters* 15:43–48,
[doi:10.1016/0167-7152(93)90097-4](https://doi.org/10.1016/0167-7152(93)90097-4).
Count storage follows the 10x/NIST MEX references in §5.21. No UMAP or clustering algorithm is
implemented in this slice, and supplied state labels do not infer phenotype or specificity.
SituCraft's own simulator provides a useful separation of observations and truth; this study's
model and implementation are ClonoLoom's, reusing its existing sampler and simulation conventions.

**Annotation contract.** Explicitly opt into one plain/gzip TSV: `barcode`, arbitrary text
columns (at most 32), optional paired `map_x`/`map_y`. Preserve source row/header order and empty
values, reject duplicates and malformed coordinates, join exact matrix barcodes, retain
annotation-only rows. Go computes the complete coverage and mapped population; JS displays
coordinates at equal axis scale. The 20,000-point map limit rejects larger populations rather
than silently sampling. Table limits are 2,000,000 rows, 4,000,000 values and 64 MiB of text.
These are format/resource policies, not published statistical methods.

**Provenance.** An optional source extends the original MEX identity with its actual-byte hash
and size under a separately versioned contract. Old MEX-only IDs remain unchanged; nearby files
are never discovered implicitly. Projects pin paths and fingerprints, including annotations;
changed sources fail reopening. Source coverage and supplied maps remain evidence inspection,
not inferred-result artifacts. Zero counts, missing expression, missing annotation rows, missing
labels and missing coordinates remain distinct. Barcodes repeating in separate samples never
establish cross-sample cell identity; cells are not independent biological replicates.

**Independent checks.** `validation/reference/cell_states.py` uses SciPy 1.17.0 `mmread`, CSV
and sets, without calling ClonoLoom expression/linking code. It verifies all four generated
matrices against separate observation truth and pins source hashes, dense-value hashes, gene
totals/detection counts and linkage/annotation coverage. Go reconstructs the study and checks
those numbers on both processors. Unit tests cover byte reproducibility, changed seeds,
cancellation/staging, malformed input, optional-source identity, drift and missing values;
browser scenes exercise explicit inputs, source-value tables, imported maps and saved reopening.

### 5.23 Cells map rendering and exploration acceptance

**Observed rendering issue.** The grid fixture placed multiple observations at exactly the same
coordinates. The missing-label cell had a separate color, but a later state-colored point
covered it. Missing values now use an outlined neutral gray drawn after categorized points;
source indices remain unchanged for selection. No coordinate jitter or deduplication is added.
An empty label is distinct from a literal `Missing annotation` label, including beyond the
six-category palette limit. Legends retain missing values without taking a category color.
Both source axes are explicit and retain equal scale. These are display policies, not inference.

**A separate illustrative fixture.** The `cell-states-clouds` preset increases each sample to
360 expression cells with ten clones per subject and optional `layout: "gaussian"`. Its new
model ID and example directory preserve the original grid fixture and source identities.
Continuous normal coordinates use a separate PCG stream and the existing polar normal sampler:
Marsaglia and Bray (1964), SIAM Review 6:260–264,
[doi:10.1137/1006063](https://doi.org/10.1137/1006063). Centers, spreads and sample sizes are chosen
illustration parameters; this is not a biological fit or computed UMAP. `docs/SIMULATION.md`
records every parameter. Tests require unchanged biological observations when only layout
changes, reproducible source hashes on both processors, and independent SciPy/truth agreement.

**Planned cell-size displays.** Size categories require explicit breakpoints and population.
The [scirpy clonal_expansion documentation](https://scirpy.scverse.org/en/latest/generated/scirpy.tl.clonal_expansion.html)
defines configurable inclusive breakpoints and optional grouping; its default (1,2) differs
from the proposed (1,2,5). The latter yields 1, 2, 3–5 and >5 and is a ClonoLoom display choice,
not a universal expansion threshold or significance test. Slice 6 must record cell counting,
sample scope, selected definition/QC, mapped versus all-eligible population and unassigned cells.

**Planned interaction and density.** The
[documented linked selection workflow](https://www.10xgenomics.com/support/software/loupe-vdj-browser/latest/tutorials/lvb-integrated-analysis-loupe-browser)
illustrates clonotype-to-map highlighting and phenotype-based exploration. Slice 6 acceptance
now includes linked selection in both directions with a keyboard-accessible table alternative,
exact sample/barcode identities and explicit missingness. Optional density overlays require
an independently specified estimator, bandwidth, normalization and support criteria first;
[clonalOverlay documentation](https://bioconductor.posit.co/packages/3.22/bioc/manuals/scRepertoire/man/scRepertoire.pdf)
provides context for frequency-filtered contours, not an implementation to copy. Density on an
embedding is descriptive and does not imply enrichment or antigen specificity. Only public
method/interface documentation was consulted for these acceptance refinements.

### 5.24 Bounded 10x HDF5 expression input

**Format sources.** The [10x feature-barcode HDF5 schema](https://www.10xgenomics.com/support/software/cell-ranger/latest/analysis/outputs/cr-outputs-h5-matrices)
defines the feature-by-cell CSC arrays and feature/barcode datasets. The reader is implemented
from The HDF Group's [File Format Specification 3.0](https://support.hdfgroup.org/documentation/hdf5/latest/_f_m_t3.html)
and [earlier 1.1 structures](https://support.hdfgroup.org/documentation/hdf5/latest/_f_m_t11.html).
SituCraft's Apache-2.0 HDF5 writer supplies the lookup3 metadata-checksum helper and a useful
compact-object reference; it supplies no reader. No other implementation's source was copied
or consulted. Go standard-library zlib performs deflate decoding; no native/runtime dependency
is added. h5py is used only through its public API to generate independent validation data.

**Evidence policy.** This is format decoding, not a statistical method. Require all eight
10x vectors; retain source feature names/types, barcodes, explicit zeros and source order.
Normalize only the internal ordering of CSC indices, moving counts with them. Reject duplicate
coordinates, malformed pointers, negative/floating counts and integer/UMI overflow. Missing
storage chunks are not measured zeros. Ignore unrelated annotations/attributes rather than
guessing a count layer, barcode mapping, genome, raw/filtered input or cell type. Receptor
linkage and optional annotation TSVs reuse the exact sample-scoped rules of slices 1–2.

**Bounded format subset.** `docs/FORMATS.md` records accepted structures and limits. It covers
the classic structures emitted by ordinary h5py files and compact HDF5 1.8-era headers/groups,
including compressed B-tree chunks. Unsupported variable strings, dense groups, newer chunk
indices, shared datatypes, external/virtual storage and required soft/external links fail
explicitly. Slice 4 must extend this boundary with independent AnnData/MuData fixtures, not
claim compatibility from a filename extension. File/array/metadata/read/traversal budgets,
checked offsets and context cancellation bound malformed input. Fletcher32 requires end-around
carry: all-one words retain 0xffff, unlike a naive modulo-65535 calculation. Independent raw
chunk checks include all-zero/all-one words and odd byte lengths.

**Provenance and validation.** Hash the selected file before and after parsing and record its
whole-file SHA-256, size and path in a separate `clonoloom-expression-hdf5/1` identity domain.
MEX-only and optional-annotation identities are unchanged. Equal observations in different
encodings do not establish equal source identity; relocation of identical bytes does.
`testdata/make-expression-hdf5.py` pins h5py 3.15.1, NumPy 2.4.2 and SciPy 1.17.0. It generates
17 accepted fixtures, including internal chunk-tree nodes, compact arrays, continuations,
filter masks, differing byte orders and empty counts, plus 18 invalid/unsupported files.
The saved reference independently reconstructs every value and gene total, with labels,
stored-entry counts and source hashes. Mutation tests exercise corrupt checksums, addresses,
cycles and budgets; fuzzing checks malformed files. The browser exercises visible opening,
explicit binding, unsupported input and saved reopening in both themes. The format example
is artificial bookkeeping evidence, not an expression model or biological validation.

An additional check used the public, CC BY 4.0
[10x 1k PBMC v3 dataset](https://www.10xgenomics.com/datasets/1-k-pbm-cs-from-a-healthy-donor-v-3-chemistry-3-standard-3-0-0)
(Cell Ranger 3.0.0): all 2,498,225 stored entries, 33,538 feature records, 1,222 barcodes and
gene totals matched h5py/SciPy exactly. `expression_hdf5_public.py` and the optional Go test
make this repeatable with explicitly selected local paths. Neither the donor data nor its
derived arrays are bundled. The check supports decoding compatibility, not biological inference.
