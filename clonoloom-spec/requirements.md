# ClonoLoom requirements

What ClonoLoom must do, and the wave that delivers each requirement.
- `research.md` explains the methods, formats and data behind them.
- `design.md` explains how they are met.
- `roadmap.md` orders them into waves; wave *N* is released as 0.*N*.0.

Status: **planned (wave N)** until built. Then **done**, **partial** (the gap noted) or
**parked**, with the validation that shows it. Requirement IDs are stable and never reused.

## Users

- **Bench immunologists** who track clones in patients, compare repertoires and need figures and
  methods for a paper, without writing code.
- **Core-facility analysts** who process many runs consistently and hand standard QC records and
  certificates to the labs they serve.
- **Clinical-trial immunologists** who compare cohorts with the subject as the unit, with complete
  metadata and an audit trail.
- **Computational biologists** who need deterministic headless runs, open exports, documented
  formats and numbers that agree with the reference tools they already trust.
- **Antibody-discovery scientists** who need clonal families, mutation, lineage trees and
  candidates.
- **Instructors** who need data whose answers are known.
- **AI agents** acting for any of them, through a typed and auditable interface.

## Distribution and platform

| # | Requirement | Status |
| --- | --- | --- |
| P1 | One self-contained program for macOS, Linux and Windows (x64 and ARM64); one-line install with checksum and version checks, no administrator rights; installs as `$HOME/.local/bin/clonoloom` and `%LocalAppData%\Programs\ClonoLoom` | partial (wave 1, slice 1: six binaries of about 7 MB built by the release workflow, installers checking SHA-256 and version; the first release is drafted at the end of wave 1) |
| P2 | Opens as a desktop window (a Chromium-family browser in app mode with its own profile), else the default browser; a second launch hands its paths to the running ClonoLoom | done (wave 1, slice 1: from SituCraft 0.9.0; slice 3: what a launch names opens as datasets) |
| P3 | Ports 8860–8879, clear of the siblings' (Proteoscope 8765–8814, CytoWeave 8770–8789, MaveScape 8820–8839, SituCraft 8840–8859), checked by a test | done (wave 1, slice 1) |
| P4 | `--offline`, `--data-dir`, `--cache-dir`, `--cache-limit`, `--examples-dir`, `--remote-control`, `--host`, `--port`, `--window`, `--version` (prints `clonoloom X.Y.Z`) | done (wave 1: `--offline`, `--data-dir`, `--examples-dir`, `--host`, `--port`, `--window`, `--version`; wave 2: `--remote-control`, `--cache-dir`, `--cache-limit`) |
| P5 | Light and dark themes, color-vision-friendly palettes, keyboard operation, WCAG AA contrast, plots described in text | planned (wave 1, every wave) |
| P6 | A container build file for the headless commands (AIRR Software WG guidance) | planned (wave 8) |
| P7 | Apache-2.0; no third-party runtime dependencies in Go or JavaScript; nothing under an incompatible license copied in or bundled | done (wave 1, slice 1; every slice keeps it: the only data built in is HGNC's gene list, CC0) |
| P8 | Data never leave the computer; loopback only; no telemetry; the only outbound requests are downloads and public-data queries the user starts, to an allow-list of hosts | planned (wave 1) |

## Data and import

| # | Requirement | Status |
| --- | --- | --- |
| D1 | Read AIRR-C Rearrangement TSV (schema 1.3 to 2.x), typed by the schema; unknown columns kept | done (wave 1, slice 2: every typed column checked on every row; required columns reported; the other columns left in the file for exports; every column read equals the `airr` library's on 11 development files) |
| D2 | Read AIRR-C Repertoire metadata (JSON and YAML, the AIRR Data File of schema 1.4+) with MiAIRR fields; Clone, Cell and Receptor objects | partial (wave 1, slice 2: Repertoire JSON of schema 1.3 to 2.0, matched to samples by `repertoire_id`; YAML, Clone, Cell and Receptor in wave 9) |
| D3 | Read 10x Cell Ranger `vdj` and `multi` outputs: `filtered_contig_annotations.csv`, `all_contig_annotations.csv` and `.json`, `airr_rearrangement.tsv`, `clonotypes.csv`, `consensus_annotations.csv`, `metrics_summary.csv`; recognize the `outs/` and `per_sample_outs/*/vdj_t|vdj_b|vdj_t_gd` layouts | partial (wave 1, slice 2: contig annotations of Cell Ranger 3 to 9 and the AIRR file, equal to pandas' and `airr`'s reading of 14 files; the layouts; clonotypes, consensus and metrics kept beside the contigs for QC; the JSON annotations not read) |
| D4 | Read Adaptive immunoSEQ Analyzer exports (v2 and v3 column sets), including immuneACCESS downloads, with templates and reads kept apart | done (wave 1, slice 2: v2 and v3; templates, else reads for a whole file; Adaptive's gene names mapped to IMGT's and recorded; equal to pandas' reading of three Emerson 2017 samples) |
| D5 | Read MiXCR clone tables (exported TSV), TRUST4 AIRR output and report, Change-O/IgBLAST AIRR output, IMGT/HighV-QUEST summaries, VDJtools tables, OAS/OTS data units, VDJdb release tables | partial (wave 1: Change-O's AIRR output; wave 4, slice 1: MiXCR clone tables (MiXCR 2 to 4, bulk and single-cell), OAS and OTS data units (unpaired and paired) and tcrdist3's clone tables, every column equal to pandas' reading of ten files; slice 7: VDJdb legacy tables by header name; wave 5, slice 1: retain and inspect 42 AIRR sequence-evidence fields; slice 2: joined IMGT/HighV-QUEST tables with native annotations, source fingerprints and checked query-coordinate mapping; wave 9: TRUST4, VDJtools) |
| D6 | Read AnnData `.h5ad` and MuData `.h5mu` (GEX and scirpy's `airr` structures) and 10x feature-barcode matrices (MEX, `.h5`) | done within documented encoding limits (wave 6, slices 1–4: MEX, 10x HDF5, explicitly selected AnnData/MuData dense/CSR/CSC counts, obs/var, supplied embeddings and scirpy AIRR evidence; unsupported HDF5/Awkward encodings fail explicitly) |
| D7 | Generic CSV/TSV import with a column mapping, checked column by column | done (wave 2, slice 8: a table no reader recognizes opens a dialog with its first rows and a column for each field, guessed from the columns' names (VDJtools', MiXCR's and others'); the count's unit said, never guessed; the whole file checked column by column before it opens; the mapping kept with the dataset, part of its ID, and stated in the methods) |
| D8 | Validator: findings (fatal, warning, note) with stable codes, counts, examples and remedies | done (wave 1, slices 2 and 3: 28 codes, each planted fault found once; the report in the Data view; wave 5, slice 1: sequence-evidence findings with coordinates, CIGARs and paired alignment checks; grows in every wave) |
| D9 | Query and download public data from AIRR Data Commons repositories (ADC API v1, unauthenticated), recorded with repository, query, date and checksums | planned (wave 9) |
| D10 | Sample annotation: subject, timepoint, tissue, cell subset, condition, batch, HLA typing; suggested from file names; MiAIRR metadata mapped when present | done (wave 1, slice 3: typed, suggested from names (after CytoWeave), or filled from AIRR Repertoire metadata and immunoSEQ sample tags, each value's origin kept; wave 3, slice 3: a reference field (an infusion product, a donor), and time points read as days from the subject's reference event, ordered and checked by the program) |
| D11 | Sources read in place, never modified; fingerprints (size and SHA-256) recorded; a moved source found again and checked | done (wave 1, slice 3: SHA-256 taken while reading, equal to the recorded checksums of every development file; a moved file found by its size and SHA-256 near where it was, or chosen and checked; a changed file said) |
| D12 | A library of projects; project archives (`.clz`) bundling the project and, where licenses allow, its data | partial (wave 1: the library, saved automatically; wave 8: archives) |
| D13 | Public dataset catalog: license, size and citation shown before a download the user confirms; checksums verified; license status carried into exports and methods | partial (wave 1, slice 6: the catalog of six datasets, downloads confirmed, checked by SHA-256 and put in place whole, terms of use accepted first, `--offline`; a dataset whose files are an example's carries its license and citation; methods in slice 7; exports in wave 2, slice 8; every wave adds datasets) |

## Quality control

| # | Requirement | Status |
| --- | --- | --- |
| Q1 | Per-sample QC: total abundance in its unit, unique rearrangements, productive fraction, out-of-frame and stop-codon fractions, CDR3 length distribution, gene-resolution rates, singleton fraction | done (wave 1, slice 4: abundance, rows, productive, out-of-frame, stop-codon and unexplained non-productive shares, clonotypes, singletons, doubletons, the largest clonotype's share, V and J resolution, reads per template or UMI, junction lengths; equal to an independent R implementation on six bulk samples; immunoSEQ's own row counts reproduced) |
| Q2 | Single-cell QC: cells with 0, 1, 2 or more chains of each locus, pairing rate, multi-chain cells, UMI support per contig, `is_cell` and `high_confidence` handling | done (wave 1, slice 4: cells by their productive chains, paired, one arm, none, more than two of a locus, UMIs per contig; Cell Ranger's own counts of cells, pairs and loci reproduced exactly on six runs of Cell Ranger 4, 6 and 9) |
| Q3 | Depth and saturation: richness against depth, sample coverage (Good–Turing, Chao and Jost), a suggested common depth or coverage for a cohort | done (wave 1, slice 4: sample coverage by Chao and Jost, equal to iNEXT's to 10⁻¹²; wave 2, slice 5: the curves of clonotypes, Shannon and Simpson diversity and coverage against depth, and a common coverage and size suggested, reached by rarefaction alone, with iNEXT's extrapolated ones beside them) |
| Q4 | Cross-sample contamination check: identical nucleotide rearrangements at high abundance in unrelated subjects flagged, with public clonotypes (high generation probability) set apart | implemented for human bulk TRB (wave 7 slice 8: exact NT junction/V/J sharing, declared subjects, complete abundance/evaluability, heuristic review and optional saved NT Pgen context; no removal or contamination verdict; other loci and single-cell scope remain deferred) |
| Q5 | Batch QC: metrics by batch or run, outliers flagged by robust z-scores | implemented: bounded sample metrics, explicit batches/subjects, median/MAD policy and unscored reasons; descriptive flags only |
| Q6 | QC never deletes data: filters are named versions with recorded reasons | done (wave 1, slice 4: productive only, minimum count, loci, cells only, high confidence only, most chains of a locus, samples left out; each version kept with its reason; what each rule left out counted) |

## Clonotype definition

| # | Requirement | Status |
| --- | --- | --- |
| C1 | Explicit, named clonotype definitions: nucleotide or amino-acid junction; V and J by gene, family or not at all; one chain or paired chains; for BCR, a distance-thresholded definition (C4) | partial (wave 1, slice 5: nucleotide or amino-acid junction; V and J by allele, gene, family or not at all; each locus, one locus, or paired αβ, γδ and heavy–light chains; the source's own clonotypes; presets, an editor and definitions saved for reuse; IDs made from the definition and the key; equal to scirpy's partitions of eight Cell Ranger runs under four definitions, and to Cell Ranger's own clonotype sizes; C4 in wave 5) |
| C2 | The definition is part of every result's lineage and is shown with every table and figure | partial (wave 1, slice 5: said in words above the clonotype table, in the inspector and in the status bar with the unit; wave 2: in every result's lineage, and with the unit and samples in every figure's caption and every export's notice) |
| C3 | Single-cell rules for cells with extra or missing chains: primary pair by UMIs, all chains, or excluded; orphan chains kept or dropped | done (wave 1, slice 5: the primary chain of each locus by UMIs, then reads, then contig ID; or every chain; cells without both chains grouped with cells of the same chain alone, or left out; QC's filter leaves out cells with too many chains) |
| C4 | BCR clonal families: same V, J and junction length, junction distance under a threshold chosen from the distance-to-nearest distribution; light chains used when paired | implemented (wave 5, slice 4: normalized nucleotide Hamming, nearest distributions and explicit single-linkage thresholds; separate from junction clonotypes) |
| C5 | Sensitivity: key results recomputed under alternative definitions, with a plain verdict on whether the conclusion holds | done for saved aggregate subject-feature contrasts (wave 7, slice 5); individual-clonotype correspondence is outside this bounded audit |
| C6 | The abundance unit is explicit: reads, UMIs, templates or cells, never inferred silently | done (wave 1, slices 2–5: every dataset's unit from its format, confirmed by the user when the format leaves it open, said with QC and every clonotype table) |

## Repertoire statistics

| # | Requirement | Status |
| --- | --- | --- |
| R1 | Diversity: Hill numbers of order 0, 1, 2 and the profile; Chao1 and ACE; Shannon and Simpson with their effective numbers; Pielou evenness and clonality; Gini; top-N share; each with a bootstrap interval | done (wave 2, slice 4: with Simpson clonality, the asymptotic Shannon and Simpson diversities and coverage; agreeing with iNEXT, immunarch and vegan on 18 samples; interval coverage measured on simulations) |
| R2 | Rarefaction and extrapolation by sample size and by coverage (Chao et al. 2014), to compare samples at equal depth or equal coverage | done (wave 2, slice 5: the Hill numbers of order 0, 1 and 2 and the coverage, with bootstrap intervals; equal to iNEXT's on 18 samples and two cohorts; the intervals' coverage measured on simulations) |
| R3 | Gene usage (V, D, J, C and V–J pairs) by sample and group; CDR3 length spectratypes; amino-acid composition | done (wave 2, slice 6: genes and families of each segment, V–J pairs, junction lengths and their mean, amino acids overall and by IMGT position, by clonotypes and by abundance, with bootstrap intervals; conditions by the mean of their samples; equal to an independent R implementation on 15 samples, and the intervals checked against the simulator's truth) |
| R4 | Overlap: shared clonotypes, Jaccard, overlap coefficient, Morisita–Horn, rarefied versions; public clonotypes across subjects, including against public cohorts | partial (wave 2, slice 7: within a project, with Sørensen–Dice and Morisita's own index, the rarefied forms exact, intervals checked against the simulator's truth; equal to immunarch, vegan and scirpy; wave 9: against public cohorts) |
| R5 | Clonal space homeostasis: the shares of rare, small, medium, large and hyperexpanded clonotypes | done (wave 2, slice 4: immunarch's bins, with bootstrap intervals) |
| R6 | Generation probability of CDR3s under published V(D)J recombination models (OLGA's algorithm, IGoR models downloaded on request) | done (wave 4, slice 3: amino-acid or nucleotide CDR3s, alone or with their V and J genes, under OLGA's seven default models downloaded on request or ClonoLoom's own model of human TRB; each sample's distribution, the clonotypes by probability, the inspector's; 99,949 CDR3s within 3 × 10⁻¹⁵ of OLGA 1.3.0's; a million in 32 s on 10 processors) |
| R7 | k-mer and motif summaries | implemented (wave 7 slice 6: bounded TRB query/background signatures and residue frequencies) |

## Tracking and dynamics

| # | Requirement | Status |
| --- | --- | --- |
| T1 | Chosen clonotypes (top N, a reference sample's, matched to a specificity, a list) followed across a subject's samples, with frequency intervals | implemented (wave 3, slice 5: a reference sample's largest, each sample's largest or a list, with Wilson's intervals; slice 6: those a test called; wave 4, slice 7: IDs matched to an epitope, with source-result provenance) |
| T2 | Two-sample differential abundance by Fisher's exact or binomial test (DeWitt et al. 2015), with false discovery rate | done (wave 3, slice 6: between two time points of each subject; Benjamini–Hochberg or Storey's q-values) |
| T3 | Beta-binomial differential abundance with variance from replicates or a time-dependent model (Rytlewski et al. 2019); the default when a variance model exists | done (wave 3, slice 7: fitted on replicate samples or two time points named as stable) |
| T4 | Several timepoints: clonotypes classed as persistent, emerging or contracting; a negative-binomial model for designs with replicates | done, bounded (wave 3, slice 8: emerging, transient, contracting or persistent from calls against a base time point; wave 7, slice 4: two-group NB2 models with per-clonotype Cox–Reid dispersion, assigned-total offsets, paired subject blocks, optional estimable batch, explicit count filter/BH family and conditional Wald intervals; at least 12 subjects/group and 24 residual df, no moderation or arbitrary multigroup designs; research §5.33) |
| T5 | Source attribution: the share of a sample's repertoire found in a reference sample (infusion product, donor), with the definition stated | done (wave 3, slice 8: the share in the reference sample's clonotypes, and, for a reference sharing few clonotypes with the samples before the event, the share derived from it, each with its interval) |
| T6 | Alluvial plots and per-clonotype trajectories ("threads") linked to the clonotype table | done (wave 3, slice 5) |

## Specificity

| # | Requirement | Status |
| --- | --- | --- |
| S1 | TCRdist (Dash et al. 2017; tcrdist3's defaults), single or paired chains | done (wave 4, slice 2: as tcrdist3 computes it, with the V genes' loops from tcrdist3's database of genes downloaded on request, or the CDR3s alone; each clonotype's neighbors within a radius and its nearest, and a clonotype's nearest loop by loop; every distance between Dash et al.'s 2,339 clones equal to tcrdist3's; 100,000 paired TCRs in 25 s on 10 processors) |
| S2 | TCRdist neighborhoods with enrichment against a background (V/J/length-matched synthetic, shuffled, or a public paired background) | done (wave 4, slice 5: each receptor of one locus a centroid, its radius controlled at a rate of a background drawn from a recombination model with the dataset's V–J usage and weighted to what recombination makes, or of its own CDR3s shuffled; each neighborhood's receptors, enrichment and motif; every centroid equal to tcrdist3 0.3's on Dash et al.'s chains; a public background waits for a later wave) |
| S3 | GLIPH2-style specificity groups (global and local similarity, motif enrichment, scores) | implemented (wave 7 slice 6: documented adaptation; matched TRBV/length, Fisher/BY, no N/P boost or GLIPH2 composite score) |
| S4 | Clonotype networks by CDR3 identity, edit distance or TCRdist | planned (wave 4) |
| S5 | Matching against specificity databases (VDJdb first; McPAS-TCR and IEDB as their terms allow) by exact or TCRdist-near match, with species, MHC and evidence; database versions recorded; downloaded on request, never bundled | partial (wave 4, slice 7: VDJdb and the simulated vaccine database; exact/TCRdist chain matching, evidence, per-sample shares, Tracking, download terms and checksums; McPAS-TCR and IEDB later) |
| S6 | Cross-subject convergence; HLA association (hypergeometric) when HLA types are given | implemented (wave 7 slice 7: observed recurrence of selected saved TRB motif groups; unique subjects, explicit complete two-field typed universes, conditional hypergeometric/BY and marginal Wilson intervals; no convergent-recombination or HLA-restriction inference) |
| S7 | Invariant populations flagged (MAIT, iNKT, GEM) by published gene and junction rules | implemented (wave 4, slice 8: explicit species; MAIT/iNKT gene and junction-length rules, human GEM motif; flags, sample counts and evaluability, CSV and methods) |

## B cells

| # | Requirement | Status |
| --- | --- | --- |
| B1 | Germline reconstruction per clonal family, D masked | done (wave 5 slice 5): explicit allele intersection, ungapped AIRR CIGAR projection and provenance; unsupported evidence excluded |
| B2 | Clonal families (C4) with heavy and light pairing in single-cell data | implemented (wave 5, slice 4: conservative paired maximum distance, explicit boundaries/thresholds, exclusions, memberships and source provenance) |
| B3 | Somatic hypermutation: mutation frequency by sequence and region (FWR/CDR), replacement and silent counts; selection (BASELINe-style) later | implemented (wave 5, slice 6: saved D-masked parents, evaluable-base frequencies by row/region, explicit codon phase, single-hit R/S substitutions and multi-hit endpoint codons, descriptive source-row pooling and independent ancestor validation; selection parked) |
| B4 | Lineage trees by maximum parsimony with ancestral sequences, maximum likelihood later; a tree viewer with isotype, timepoint and tissue on the nodes | partial (wave 5, slice 7: shared V/J panels, exhaustive parsimony through eight states, bounded heuristic through 64, conditional ancestral states and observed metadata viewer; likelihood remains wave 9) |
| B5 | Isotype and subclass usage, class switching by clonal family | partial (wave 5, slice 8: sample/family usage, unresolved calls and unordered family co-occurrence; switching events, ancestral traits and rates are not inferred) |
| B6 | Candidates ranked by size, mutation, isotype and expansion; FASTA for synthesis; sent to Proteoscope | planned (wave 9) |

## Single cell

| # | Requirement | Status |
| --- | --- | --- |
| M1 | V(D)J linked to gene expression by barcode; clonotypes on a map (imported or computed) and phenotypes per clonotype | done (wave 6, slices 1–6: exact sample-scoped links, imported/computed maps, immutable clonotype overlays, supplied phenotypes and explicit missingness; raw AnnData/MuData AIRR evidence remains separate from the receptor dataset) |
| M2 | Clonal expansion by cluster or cell type; clonotype size on the map | done (wave 6, slice 6: distinct-cell receptor/mapped sizes, configurable categories, descriptive supplied-annotation counts and fractions, sample/subject boundaries, linked selection and complete provenance; no cell-level significance tests) |
| M3 | Clonotype modularity and CoNGA-style graph-versus-graph analysis | done (wave 6, slices 7–8: clonotype modularity and bounded paired-alpha-beta CoNGA-style KNN comparison, explicit conditional nulls and multiple testing; not the complete CoNGA pipeline) |
| M4 | Antigen-capture (BEAM, dextramer) specificity calls per clonotype | parked |
| M5 | Pseudobulk differential expression between expanded and other clonotypes per cell type, subject as the unit | implemented with bounds (wave 7, slices 10–11: exact raw sums; one supplied type/timepoint and paired size-category contrast, 25–32 subject pairs, median-ratio offsets, unmoderated NB2/Cox–Reid, explicit filter/BH family, conditional intervals, source-free replay and complete exports; no moderation or quasi-likelihood) |

## Cohorts

| # | Requirement | Status |
| --- | --- | --- |
| G1 | Study design: subjects, groups, timepoints, pairing; checked to support the requested test | done, bounded two-group independent/paired designs (wave 7: explicit exclusions, complete pairs, repeated-sample/batch-confounding checks, test-specific estimability; failed checks retained but unavailable for inference; arbitrary multi-group formulas remain outside scope) |
| G2 | Group comparisons of per-sample metrics with the test chosen from the design; nonparametric counterparts; effect sizes and intervals; Benjamini–Hochberg | done, bounded two-group designs (wave 7, slice 3: Welch/paired t and conditional Mann–Whitney/signed ranks, mean and probability effects with distinct marginal intervals, explicit two-test-per-feature BH family, missingness refusal and subject/pair plots; multigroup and adjusted models remain outside this implementation) |
| G3 | The subject is the unit; cells and sequences are never replicates | partial (wave 2: overlap's pairs of one subject and of two, and public clonotypes by subject; wave 3, slice 3: every sample's subject checked before tracking, which is within a subject; wave 7, slice 1: explicit subject counts and refusal of pseudoreplication in cohort designs; slice 3: subject-level independent/paired tests and plots, retaining incomplete-feature observations without silent deletion; every wave) |
| G4 | Feature tables (receptor, sample, cohort) exported for external modeling | partial — wave 7 slice 2: design-bound sample/subject diversity features, units/missingness, CSV/JSON and provenance; receptor and other feature families remain |

## Output, record and reproducibility

| # | Requirement | Status |
| --- | --- | --- |
| O1 | Figures (SVG, PNG, PDF) with the clonotype definition, abundance unit and n in the legend | partial (wave 2, slice 8: every plot of the QC and Repertoire views saved as SVG, PNG at 300 dpi or PDF, on white with its title, legend and a caption stating what was measured, the clonotype definition, the unit, the samples, the filter and ClonoLoom's version; wave 7, slice 12: saved passing/failed cohort checks and every kept cohort analysis with exact descriptions/citations; grown in every wave) |
| O2 | Methods paragraph with numbered references, written from the record (versions, references and their versions, public datasets with accession and download date, parameters) | partial (wave 1, slice 7: a paragraph per dataset, from the record: the data's source, license and date, the format and reader, the unit, the validator's findings, the filter with what it kept, the clonotype definition and its ID, ClonoLoom's version and commit, with numbered references; wave 2: diversity, rarefaction, gene usage, overlap and tables read through a column mapping; wave 5: family thresholds, reference hashes, mutation denominators, tree uncertainty and observed isotypes with cited methods; wave 6, slice 10: correct paired cell-state simulation description and all saved Cells analyses with effective descriptions/references, linked to their explicit dataset/input bindings; grown in every wave) |
| O3 | Exports: AIRR-C Rearrangement, Clone, Cell and Repertoire; AnnData; Parquet; CSV; FASTA; Newick | partial (wave 2, slice 8: the rearrangements as AIRR Rearrangement TSV with each one's clonotype, the clonotype table and every analysis's result as CSV; wave 5, slice 1: retained AIRR sequence evidence round-trips with missing and invalid values preserved; slice 2: IMGT evidence and native annotation JSON round-trip; AIRR Clone, Cell and Repertoire to come; wave 5: germline/lineage FASTA and lineage Newick implemented; wave 6, slice 9: AnnData raw counts, receptor links, annotations, embeddings and provenance; wave 8: Parquet) |
| O4 | Excel workbooks with provenance sheets; GraphPad Prism export | planned (wave 9) |
| O5 | Certificates: inputs' fingerprints, the recipe, every reported number; `clonoloom verify` recomputes and compares bit for bit | partial (wave 8, slice 1: bounded evidence archives for cohort features/comparisons/counts; exact byte inventory, provider notices, Reports and native integrity-only verification; slice 2 adds exact native worker replay of those three kinds and embedded feature parents, with explicit captured-input boundary and differs/unavailable outcomes; slice 3 adds batch-QC/motif certificates and exact replay with kind-specific captured-input boundaries; broader source/result contracts remain unmapped) |
| O6 | Self-checking HTML reports that open in any browser | planned (wave 8) |
| O7 | Checkpoints with plain-language differences between analysis versions | planned (wave 8) |
| O8 | MiAIRR completeness checklist | planned (wave 9) |
| O9 | Exports carry each public input's license and citation; an export that would redistribute restricted data warns first | partial (wave 2, slice 8: every export's NOTICE.txt names the data's files and fingerprints, the filter, definition and unit, the method and ClonoLoom's version, and each public data set's license and citation; rows of data under terms of use are written only after their terms are shown and accepted, and say not to share them; every wave's exports alike) |
| O10 | De-identification on export: subject IDs replaced, free-text metadata and HLA dropped on request | planned (wave 8) |

## Automation

| # | Requirement | Status |
| --- | --- | --- |
| A1 | `clonoloom mcp`: an MCP server, read-only by default; `--allow edits,compute,files,network` | planned (wave 8) |
| A2 | Agent changes are proposals the user accepts or rejects; undoable; every call logged with the agent's name | planned (wave 8) |
| A3 | A typed query tool: "which clonotypes meet these conditions" as a filter tree, never SQL or shell | planned (wave 8) |
| A4 | Python and R clients generated from the tool list | planned (wave 8) |
| A5 | An agent benchmark with tasks whose answers are known, graded by code | planned (wave 8) |
| A6 | `clonoloom run --recipe --input --out`: headless runs, one dataset or a batch, friendly to cluster array tasks | partial (wave 8, slice 3: native named-input recipes, explicit batch task selection, source batch QC/motifs and five certificate replay kinds, checksum snapshots, effective recipe/run inventory, cancellation and no overwrite; other source kinds and cluster packaging remain) |
| A7 | Remote control from local programs (the documentation's screenshots, later agents) | done (wave 2, slice 1: eleven actions, token for reading files, `remote.json`; the screenshots use it; agents in wave 8) |

## Examples and learning

| # | Requirement | Status |
| --- | --- | --- |
| E1 | Simulated studies with their truth (`clonoloom simulate`) | partial (wave 2, slice 2: bulk TRB studies from ClonoLoom's own recombination model, with each population's truth; presets standard, null and small; offered as examples; wave 3, slice 4: studies over time, CAR-T and transplant with an infusion product or a donor mixed into the blood in known shares, clones on known trajectories and biological variance between samples, offered as examples, and a null series; wave 4, slice 4: receptors of known specificity, families around shared founders and private receptors as near each other as Dash et al.'s, a vaccination study offered as an example, and a specificity database in VDJdb's format; wave 5, slice 3: artificial paired B-cell families with known ancestors, ordered substitutions, fixed isotype histories and sampled cells; offline immunization example, independently checked truth; wave 6, slice 2: seeded paired TRA/TRB and expression study with known states, supplied coordinates and separate observation truth) |
| E2 | Public example datasets downloaded on request with license and citation shown | done (wave 1, slice 6: six datasets of 10x Genomics, the AIRR Community and immuneACCESS; wave 4: Dash et al. 2017's epitope-specific TCRs and a healthy donor's OTS unit; every wave adds its own) |
| E3 | Guided tours and worked examples per view | partial (wave 3, slice 8: the simulated CAR-T and transplant studies' guided steps in the Tracking view, each done by its remote control action; wave 4, slice 8: human/mouse epitope networks and vaccine database/Tracking steps; wave 5, slice 9: nine-step artificial immunization workflow through inputs, families, germlines, SHM, lineages, isotypes and cited methods; wave 6, slice 10: complete seven-step artificial single-cell workflow through maps, phenotypes, graph tests, AnnData and cited methods, with expected populations, exports and saved reopening; wave 7, slice 12: five complete cohort controls, explicit saved design versions, sixteen replayable analyses and confounded-design refusal) |
| E4 | Exercises graded against simulated truth, with class codes | parked |

## Cross-suite

| # | Requirement | Status |
| --- | --- | --- |
| X1 | Send a TCR or antibody to Proteoscope | planned (wave 9) |
| X2 | Index-sort and FCS phenotypes from CytoWeave linked to single-cell V(D)J | planned (wave 9) |
| X3 | MaveScape effect maps aligned to a clone's sequence and mutations | parked |
| X4 | Clone labels mapped to cells in tissue (SituCraft) | parked |
| X5 | Suite-wide certificate, methods-reference and public-data catalog conventions | partial (wave 8, slice 1: sibling certificate/transport/client patterns reviewed; shared inventory, fingerprint and explicit verdict conventions; ClonoLoom computes verification in Go; broader conventions follow) |

## Quality, security and accessibility

| # | Requirement | Status |
| --- | --- | --- |
| N1 | Loopback only, Host-header check, same-origin API, strict CSP, only named paths read, safe archive extraction | planned (wave 1) |
| N2 | Every reader fuzzed; hostile files refused with a message, never a crash; sizes checked before allocation | partial (wave 1, slice 2: every reader of wave 1 fuzzed; wave 4, slice 1: the readers of MiXCR's tables, OAS and OTS units and tcrdist3's tables, with 21 million inputs; later readers as they arrive) |
| N3 | Performance budgets checked automatically (`design.md`, "Budgets") | planned (wave 1, every wave) |
| N4 | Stable, versioned project and export formats; later versions open earlier projects and reproduce their artifacts | planned (every wave; promised at 1.0) |
| N5 | Documentation and site: installing, a guide per view, formats, network, MCP, the methods with their validation | partial (wave 1: `docs/INSTALLING.md`, `docs/FORMATS.md`, `docs/GETTING-STARTED.md` with pictures made by `docs/capture/capture.mjs`, the validation in the changelog; wave 3, slice 1: the website (`docs/site/`), with a home page, install, the methods with their validation, and the guide a page a subject, its links checked in CI and its pages by axe-core; network and MCP pages with their waves) |
| N6 | Research use only: not a medical device, no diagnostic claims; wording fixed in `conventions.md` | done (wave 1: on the start page, in the status bar and the inspector, the README and the guide) |
| N7 | Results the same, bit for bit, on every processor and operating system: the same data, settings and seed give the same bytes (`conventions.md`) | done (wave 3, slice 2: `internal/dmath` and every product rounded on its own; a test fails on any fused multiply-add in the arm64 assembly; every analysis's results and a simulated study pinned on arm64 and amd64; project fixtures compared by checksum from 0.3.0, including the 0.5.0 B-cell parent chain and IMGT inputs and the 0.6.0 single-cell chain/AnnData binary with recorded provenance; 0.7.0 cohort fixture includes independent/null/unbalanced/paired chains, full truth and a failed confounded check) |
