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
| P4 | `--offline`, `--data-dir`, `--cache-dir`, `--cache-limit`, `--examples-dir`, `--remote-control`, `--host`, `--port`, `--window`, `--version` (prints `clonoloom X.Y.Z`) | partial (wave 1: `--offline`, `--data-dir`, `--examples-dir`, `--host`, `--port`, `--window`, `--version`; wave 2, slice 1: `--remote-control`; slice 3: the cache's flags) |
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
| D5 | Read MiXCR clone tables (exported TSV), TRUST4 AIRR output and report, Change-O/IgBLAST AIRR output, IMGT/HighV-QUEST summaries, VDJtools tables, OAS/OTS data units, VDJdb release tables | planned (wave 1: Change-O; wave 4: MiXCR, OAS/OTS, VDJdb; wave 5: IMGT/HighV-QUEST; wave 9: TRUST4, VDJtools) |
| D6 | Read AnnData `.h5ad` and MuData `.h5mu` (GEX and scirpy's `airr` structures) and 10x feature-barcode matrices (MEX, `.h5`) | planned (wave 6) |
| D7 | Generic CSV/TSV import with a column mapping, checked column by column | planned (wave 2) |
| D8 | Validator: findings (fatal, warning, note) with stable codes, counts, examples and remedies | done (wave 1, slices 2 and 3: 28 codes, each planted fault found once; the report in the Data view; grows in every wave) |
| D9 | Query and download public data from AIRR Data Commons repositories (ADC API v1, unauthenticated), recorded with repository, query, date and checksums | planned (wave 9) |
| D10 | Sample annotation: subject, timepoint, tissue, cell subset, condition, batch, HLA typing; suggested from file names; MiAIRR metadata mapped when present | done (wave 1, slice 3: typed, suggested from names (after CytoWeave), or filled from AIRR Repertoire metadata and immunoSEQ sample tags, each value's origin kept) |
| D11 | Sources read in place, never modified; fingerprints (size and SHA-256) recorded; a moved source found again and checked | done (wave 1, slice 3: SHA-256 taken while reading, equal to the recorded checksums of every development file; a moved file found by its size and SHA-256 near where it was, or chosen and checked; a changed file said) |
| D12 | A library of projects; project archives (`.clz`) bundling the project and, where licenses allow, its data | partial (wave 1: the library, saved automatically; wave 8: archives) |
| D13 | Public dataset catalog: license, size and citation shown before a download the user confirms; checksums verified; license status carried into exports and methods | partial (wave 1, slice 6: the catalog of six datasets, downloads confirmed, checked by SHA-256 and put in place whole, terms of use accepted first, `--offline`; a dataset whose files are an example's carries its license and citation; methods in slice 7, exports in wave 2; every wave adds datasets) |

## Quality control

| # | Requirement | Status |
| --- | --- | --- |
| Q1 | Per-sample QC: total abundance in its unit, unique rearrangements, productive fraction, out-of-frame and stop-codon fractions, CDR3 length distribution, gene-resolution rates, singleton fraction | done (wave 1, slice 4: abundance, rows, productive, out-of-frame, stop-codon and unexplained non-productive shares, clonotypes, singletons, doubletons, the largest clonotype's share, V and J resolution, reads per template or UMI, junction lengths; equal to an independent R implementation on six bulk samples; immunoSEQ's own row counts reproduced) |
| Q2 | Single-cell QC: cells with 0, 1, 2 or more chains of each locus, pairing rate, multi-chain cells, UMI support per contig, `is_cell` and `high_confidence` handling | done (wave 1, slice 4: cells by their productive chains, paired, one arm, none, more than two of a locus, UMIs per contig; Cell Ranger's own counts of cells, pairs and loci reproduced exactly on six runs of Cell Ranger 4, 6 and 9) |
| Q3 | Depth and saturation: richness against depth, sample coverage (Good–Turing, Chao and Jost), a suggested common depth or coverage for a cohort | partial (wave 1, slice 4: sample coverage by Chao and Jost, equal to iNEXT's to 10⁻¹²; curves and suggestions in wave 2) |
| Q4 | Cross-sample contamination check: identical nucleotide rearrangements at high abundance in unrelated subjects flagged, with public clonotypes (high generation probability) set apart | planned (wave 7) |
| Q5 | Batch QC: metrics by batch or run, outliers flagged by robust z-scores | planned (wave 7) |
| Q6 | QC never deletes data: filters are named versions with recorded reasons | done (wave 1, slice 4: productive only, minimum count, loci, cells only, high confidence only, most chains of a locus, samples left out; each version kept with its reason; what each rule left out counted) |

## Clonotype definition

| # | Requirement | Status |
| --- | --- | --- |
| C1 | Explicit, named clonotype definitions: nucleotide or amino-acid junction; V and J by gene, family or not at all; one chain or paired chains; for BCR, a distance-thresholded definition (C4) | partial (wave 1, slice 5: nucleotide or amino-acid junction; V and J by allele, gene, family or not at all; each locus, one locus, or paired αβ, γδ and heavy–light chains; the source's own clonotypes; presets, an editor and definitions saved for reuse; IDs made from the definition and the key; equal to scirpy's partitions of eight Cell Ranger runs under four definitions, and to Cell Ranger's own clonotype sizes; C4 in wave 5) |
| C2 | The definition is part of every result's lineage and is shown with every table and figure | partial (wave 1, slice 5: said in words above the clonotype table, in the inspector and in the status bar with the unit; results' lineage from wave 2) |
| C3 | Single-cell rules for cells with extra or missing chains: primary pair by UMIs, all chains, or excluded; orphan chains kept or dropped | done (wave 1, slice 5: the primary chain of each locus by UMIs, then reads, then contig ID; or every chain; cells without both chains grouped with cells of the same chain alone, or left out; QC's filter leaves out cells with too many chains) |
| C4 | BCR clonal families: same V, J and junction length, junction distance under a threshold chosen from the distance-to-nearest distribution; light chains used when paired | planned (wave 5) |
| C5 | Sensitivity: key results recomputed under alternative definitions, with a plain verdict on whether the conclusion holds | planned (wave 7) |
| C6 | The abundance unit is explicit: reads, UMIs, templates or cells, never inferred silently | done (wave 1, slices 2–5: every dataset's unit from its format, confirmed by the user when the format leaves it open, said with QC and every clonotype table) |

## Repertoire statistics

| # | Requirement | Status |
| --- | --- | --- |
| R1 | Diversity: Hill numbers of order 0, 1, 2 and the profile; Chao1 and ACE; Shannon and Simpson with their effective numbers; Pielou evenness and clonality; Gini; top-N share; each with a bootstrap interval | planned (wave 2) |
| R2 | Rarefaction and extrapolation by sample size and by coverage (Chao et al. 2014), to compare samples at equal depth or equal coverage | planned (wave 2) |
| R3 | Gene usage (V, D, J, C and V–J pairs) by sample and group; CDR3 length spectratypes; amino-acid composition | planned (wave 2) |
| R4 | Overlap: shared clonotypes, Jaccard, overlap coefficient, Morisita–Horn, rarefied versions; public clonotypes across subjects, including against public cohorts | planned (wave 2: within a project; wave 4: against public cohorts) |
| R5 | Clonal space homeostasis: the shares of rare, small, medium, large and hyperexpanded clonotypes | planned (wave 2) |
| R6 | Generation probability of CDR3s under published V(D)J recombination models (OLGA's algorithm, IGoR models downloaded on request) | planned (wave 4) |
| R7 | k-mer and motif summaries | planned (wave 7) |

## Tracking and dynamics

| # | Requirement | Status |
| --- | --- | --- |
| T1 | Chosen clonotypes (top N, a reference sample's, matched to a specificity, a list) followed across a subject's samples, with frequency intervals | planned (wave 3) |
| T2 | Two-sample differential abundance by Fisher's exact or binomial test (DeWitt et al. 2015), with false discovery rate | planned (wave 3) |
| T3 | Beta-binomial differential abundance with variance from replicates or a time-dependent model (Rytlewski et al. 2019); the default when a variance model exists | planned (wave 3) |
| T4 | Several timepoints: clonotypes classed as persistent, emerging or contracting; a negative-binomial model for designs with replicates | planned (wave 3: classes; wave 7: model) |
| T5 | Source attribution: the share of a sample's repertoire found in a reference sample (infusion product, donor), with the definition stated | planned (wave 3) |
| T6 | Alluvial plots and per-clonotype trajectories ("threads") linked to the clonotype table | planned (wave 3) |

## Specificity

| # | Requirement | Status |
| --- | --- | --- |
| S1 | TCRdist (Dash et al. 2017; tcrdist3's defaults), single or paired chains | planned (wave 4) |
| S2 | TCRdist neighborhoods with enrichment against a background (V/J/length-matched synthetic, shuffled, or a public paired background) | planned (wave 4) |
| S3 | GLIPH2-style specificity groups (global and local similarity, motif enrichment, scores) | planned (wave 7) |
| S4 | Clonotype networks by CDR3 identity, edit distance or TCRdist | planned (wave 4) |
| S5 | Matching against specificity databases (VDJdb first; McPAS-TCR and IEDB as their terms allow) by exact or TCRdist-near match, with species, MHC and evidence; database versions recorded; downloaded on request, never bundled | planned (wave 4) |
| S6 | Cross-subject convergence; HLA association (hypergeometric) when HLA types are given | planned (wave 7) |
| S7 | Invariant populations flagged (MAIT, iNKT, GEM) by published gene and junction rules | planned (wave 4) |

## B cells

| # | Requirement | Status |
| --- | --- | --- |
| B1 | Germline reconstruction per clonal family, D masked | planned (wave 5) |
| B2 | Clonal families (C4) with heavy and light pairing in single-cell data | planned (wave 5) |
| B3 | Somatic hypermutation: mutation frequency by sequence and region (FWR/CDR), replacement and silent counts; selection (BASELINe-style) later | planned (wave 5; selection parked) |
| B4 | Lineage trees by maximum parsimony with ancestral sequences, maximum likelihood later; a tree viewer with isotype, timepoint and tissue on the nodes | planned (wave 5: parsimony; wave 9: likelihood) |
| B5 | Isotype and subclass usage, class switching by clonal family | planned (wave 5) |
| B6 | Candidates ranked by size, mutation, isotype and expansion; FASTA for synthesis; sent to Proteoscope | planned (wave 9) |

## Single cell

| # | Requirement | Status |
| --- | --- | --- |
| M1 | V(D)J linked to gene expression by barcode; clonotypes on a map (imported or computed) and phenotypes per clonotype | planned (wave 6) |
| M2 | Clonal expansion by cluster or cell type; clonotype size on the map | planned (wave 6) |
| M3 | Clonotype modularity and CoNGA-style graph-versus-graph analysis | planned (wave 6) |
| M4 | Antigen-capture (BEAM, dextramer) specificity calls per clonotype | parked |
| M5 | Pseudobulk differential expression between expanded and other clonotypes per cell type, subject as the unit | planned (wave 7) |

## Cohorts

| # | Requirement | Status |
| --- | --- | --- |
| G1 | Study design: subjects, groups, timepoints, pairing; checked to support the requested test | planned (wave 7) |
| G2 | Group comparisons of per-sample metrics with the test chosen from the design; nonparametric counterparts; effect sizes and intervals; Benjamini–Hochberg | planned (wave 7) |
| G3 | The subject is the unit; cells and sequences are never replicates | planned (wave 2, every wave) |
| G4 | Feature tables (receptor, sample, cohort) exported for external modeling | planned (wave 7) |

## Output, record and reproducibility

| # | Requirement | Status |
| --- | --- | --- |
| O1 | Figures (SVG, PNG, PDF) with the clonotype definition, abundance unit and n in the legend | planned (wave 2, every wave) |
| O2 | Methods paragraph with numbered references, written from the record (versions, references and their versions, public datasets with accession and download date, parameters) | partial (wave 1, slice 7: a paragraph per dataset, from the record: the data's source, license and date, the format and reader, the unit, the validator's findings, the filter with what it kept, the clonotype definition and its ID, ClonoLoom's version and commit, with numbered references; wave 2: analyses; grown in every wave) |
| O3 | Exports: AIRR-C Rearrangement, Clone, Cell and Repertoire; AnnData; Parquet; CSV; FASTA; Newick | planned (wave 2: AIRR, CSV; wave 5: FASTA, Newick; wave 6: AnnData; wave 8: Parquet) |
| O4 | Excel workbooks with provenance sheets; GraphPad Prism export | planned (wave 9) |
| O5 | Certificates: inputs' fingerprints, the recipe, every reported number; `clonoloom verify` recomputes and compares bit for bit | planned (wave 8) |
| O6 | Self-checking HTML reports that open in any browser | planned (wave 8) |
| O7 | Checkpoints with plain-language differences between analysis versions | planned (wave 8) |
| O8 | MiAIRR completeness checklist | planned (wave 9) |
| O9 | Exports carry each public input's license and citation; an export that would redistribute restricted data warns first | planned (wave 2, every wave) |
| O10 | De-identification on export: subject IDs replaced, free-text metadata and HLA dropped on request | planned (wave 8) |

## Automation

| # | Requirement | Status |
| --- | --- | --- |
| A1 | `clonoloom mcp`: an MCP server, read-only by default; `--allow edits,compute,files,network` | planned (wave 8) |
| A2 | Agent changes are proposals the user accepts or rejects; undoable; every call logged with the agent's name | planned (wave 8) |
| A3 | A typed query tool: "which clonotypes meet these conditions" as a filter tree, never SQL or shell | planned (wave 8) |
| A4 | Python and R clients generated from the tool list | planned (wave 8) |
| A5 | An agent benchmark with tasks whose answers are known, graded by code | planned (wave 8) |
| A6 | `clonoloom run --recipe --input --out`: headless runs, one dataset or a batch, friendly to cluster array tasks | planned (wave 8) |
| A7 | Remote control from local programs (the documentation's screenshots, later agents) | done (wave 2, slice 1: eleven actions, token for reading files, `remote.json`; the screenshots use it; agents in wave 8) |

## Examples and learning

| # | Requirement | Status |
| --- | --- | --- |
| E1 | Simulated studies with their truth (`clonoloom simulate`) | partial (wave 2, slice 2: bulk TRB studies from ClonoLoom's own recombination model, with each population's truth; presets standard, null and small; offered as examples; wave 3: longitudinal; wave 4: specificity; wave 5: BCR lineages; wave 6: single cell with expression) |
| E2 | Public example datasets downloaded on request with license and citation shown | done (wave 1, slice 6: six datasets of 10x Genomics, the AIRR Community and immuneACCESS; every wave adds its own) |
| E3 | Guided tours and worked examples per view | planned (wave 2, every wave) |
| E4 | Exercises graded against simulated truth, with class codes | parked |

## Cross-suite

| # | Requirement | Status |
| --- | --- | --- |
| X1 | Send a TCR or antibody to Proteoscope | planned (wave 9) |
| X2 | Index-sort and FCS phenotypes from CytoWeave linked to single-cell V(D)J | planned (wave 9) |
| X3 | MaveScape effect maps aligned to a clone's sequence and mutations | parked |
| X4 | Clone labels mapped to cells in tissue (SituCraft) | parked |
| X5 | Suite-wide certificate, methods-reference and public-data catalog conventions | planned (wave 8) |

## Quality, security and accessibility

| # | Requirement | Status |
| --- | --- | --- |
| N1 | Loopback only, Host-header check, same-origin API, strict CSP, only named paths read, safe archive extraction | planned (wave 1) |
| N2 | Every reader fuzzed; hostile files refused with a message, never a crash; sizes checked before allocation | partial (wave 1, slice 2: every reader of wave 1 fuzzed; later readers as they arrive) |
| N3 | Performance budgets checked automatically (`design.md`, "Budgets") | planned (wave 1, every wave) |
| N4 | Stable, versioned project and export formats; later versions open earlier projects and reproduce their artifacts | planned (every wave; promised at 1.0) |
| N5 | Documentation and site: installing, a guide per view, formats, network, MCP, the methods with their validation | partial (wave 1: `docs/INSTALLING.md`, `docs/FORMATS.md`, `docs/GETTING-STARTED.md` with pictures made by `docs/capture/capture.mjs`, the validation in the changelog; the site later) |
| N6 | Research use only: not a medical device, no diagnostic claims; wording fixed in `conventions.md` | done (wave 1: on the start page, in the status bar and the inspector, the README and the guide) |
