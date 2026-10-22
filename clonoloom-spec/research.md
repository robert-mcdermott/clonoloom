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

MiXCR clone tables (v3 `cloneCount`, v4 `readCount`; `aaSeqCDR3` is the junction; scores in
calls, `*00` alleles), OAS and OTS data units (gzipped CSV whose first line is CSV-quoted JSON
metadata; paired units with `_heavy`/`_light` or `_beta`/`_alpha` suffixes; integers written as
`214.0`), VDJdb releases (re-packaged in 2026; parse by header name), TRUST4, IMGT/HighV-QUEST
and VDJtools. Each is researched again when its wave is planned.

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

Later waves add: Dash et al. 2017's epitope-specific TCRs as tcrdist3 distributes them (MIT),
OAS and OTS subsets (CC BY 4.0), DeWitt et al. 2015's longitudinal TRB repertoires (ADC), scirpy's
example datasets (Wu et al. 2020, Maynard et al. 2020), and VDJdb (on request only).

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
