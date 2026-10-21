# ClonoLoom roadmap

The plan for building ClonoLoom. Each **wave** is one release: wave 1 is 0.1.0, wave 2 is 0.2.0,
and so on. Each wave is split into **slices**, bite-sized units of work. A slice ends with:
- something that runs;
- tests or validation that check it;
- its documentation;
- an entry in `CHANGELOG.md`.

Slices are done in order within a wave unless noted. Each slice names the requirements
(`requirements.md`) it serves. When a wave is reached, it is planned in detail: its slices are
confirmed or re-cut, and the decisions taken are recorded here. Waves 1–3 are planned to the slice
now; later waves are outlined and refined as they come.

The order follows the siblings' principle:
1. **Trust first.** Readers that agree with independent readers on every value, and a validator
   that says when not to trust the data.
2. **Then the basic questions, with their uncertainty.** Diversity, overlap and the clones that
   change, with the clonotype definition stated.
3. **Then the receptors' meaning.** Specificity, B-cell families and phenotypes.
4. **Then reach.** Cohorts, a provable record, agents and scripts, public data.

## Decisions taken in this plan

| Decision | Choice |
| --- | --- |
| Product and executable | **ClonoLoom**, `clonoloom`; Go module `clonoloom`; project archives `.clz` |
| Architecture | One Go binary (standard library) serving an embedded vanilla-JS app in a desktop window, as the siblings; Go computes every number, the browser draws (`design.md`) |
| First inputs | AIRR-C TSV, Cell Ranger V(D)J and immunoSEQ in wave 1, bulk TRB first (the CAR-T kinetics case), single-cell paired chains alongside |
| Development data | Public sources that need no approval (`research.md`, "Datasets"); only openly licensed ones in per-commit CI; nothing in the repository but small synthetic files |
| Remote control | Wave 2, slice 1, as in SituCraft, so documentation screenshots are captured from the start |
| CI | Once per commit; releases check CI's result on the tagged files instead of testing again (`design.md`) |
| Default port | 8860 (8860–8879) |

## In every wave, from wave 2

- **Examples.** Each new capability arrives with an example that uses it, on public data or a
  simulation (labeled simulated). Each example states its question, data source and license,
  what to expect, the view it opens and its guided steps, and the validation suite checks it.
- **Documentation.** The slice that builds a feature also writes its guide page and the capture
  scenes for its screenshots. `docs/FORMATS.md` grows with every format read or written. The
  README is the front page, laid out as the siblings' are. What each release brought goes in
  `CHANGELOG.md`.
- **Release.** Screenshots re-captured, binaries drafted by the release workflow.
- **Projects that keep opening.** Every release saves a project as a fixture under
  `validation/projects/`. Every later version must open each fixture and reproduce its artifacts'
  recorded checksums, or say why not.

## 0.1.0: trust the data (wave 1)

A researcher opens the output of their annotation pipeline and gets:
- a validation report that says what is there and whether to trust it;
- per-sample and single-cell QC;
- clonotypes under a definition they chose and can see;
- a project that reopens, recording where the data came from.

### Wave 1

1. **The program and its window (P1, P2, P3, P4, P5, P7, P8, N1).** Ported from SituCraft 0.9.0
   with renames:
   - **Go host:** `main.go`, `security.go`, `local.go`, `store.go`, `records.go`, `window.go`
     and their Go tests; file kinds for repertoire files and vendor folders.
   - **Web shell:** `index.html`, `styles.css`, `app.js`, `ui/dom.js`, `overlays.js`,
     `store.js`, `palette.js`, `icons.js`, `sidebar.js`, `inspector.js`, `drawer.js`,
     `storage.js`. The left rail (Data, QC, Clonotypes, Repertoire, Tracking, Specificity,
     B cells, Cells, Cohort, Reports, Jobs), each view saying what it will do until its slice
     builds it; the inspector, the drawer, the status bar with the active clonotype definition,
     light and dark themes, color-vision-friendly colors.
   - **Pure modules:** `web/lib/sha256.js`, `colormaps.js`, `colorvision.js`, `project.js` with
     tests.
   - **CI** with the single-run policy; the **release workflow**; installers; `LICENSE`,
     `NOTICE`, `CITATION.cff`, `CHANGELOG.md`, `README.md`, `docs/INSTALLING.md`,
     `.claude/launch.json`.
   - `--version` prints one line, `clonoloom X.Y.Z`, which the installers compare.
   - Validation: Go tests (security headers, foreign hosts and cross-origin requests refused,
     library round trips, every web module embedded, the commit reported, ports clear of the
     siblings'); the accessibility suite (contrast of both themes, palettes apart in each color
     vision); a release dry run builds six binaries with `SHA256SUMS`.
2. **Reading repertoires (D1, D2, D3, D4, D5, D11, N2).** ClonoLoom's own readers:
   - `internal/seq` (codons, translation, reverse complement), `internal/genes` (IMGT names
     parsed into locus, segment, family, gene and allele; Adaptive's names mapped; ambiguous
     calls kept as sets);
   - `internal/airr`: the Rearrangement schema's fields and types (1.3–2.x), TSV read and written,
     Repertoire JSON;
   - `internal/rep`: the columnar rearrangement table with interned strings;
   - `internal/reads`: AIRR TSV (and Change-O's), Cell Ranger contig annotations (CSV and the
     `airr_rearrangement.tsv`), `clonotypes.csv`, `metrics_summary.csv` and the `outs/` and
     `per_sample_outs/` layouts, immunoSEQ v2 and v3; the format detected from the header; gzip
     read transparently.
   - **Data:** `validation/sources.json` and `validation/fetch.mjs` download the wave's
     development datasets with checksums.
   - **Exit criterion.** On every development file, every row and value ClonoLoom reads equals an
     independent reader's: the `airr` Python library for AIRR files, pandas for the vendor tables,
     scirpy's `read_10x_vdj` for cells and chains. The comparison scripts are committed under
     `validation/reference/` with their outputs reduced to checksums.
   - **Budget:** a 1M-row immunoSEQ file read within 5 s.
   - **Fuzzing:** every reader fuzzed, with a corpus of small hostile cases.
3. **Datasets and their validation (D8, D10, D11, Q6).** The Data view:
   - files named on the command line, by **Open**, or dropped become datasets with their
     fingerprints, format and samples;
   - the validator's findings (fatal, warning, note) with stable codes, counts, example rows and
     remedies: junction not translating to `junction_aa`, junctions without the conserved
     residues, V/J calls not parseable or of another locus, duplicate sequence IDs, missing or
     non-positive counts, unknown abundance unit, cells with more than two chains of a locus,
     mixed species, productive flags contradicting the sequence, an unusual non-productive share;
   - sample annotation (subject, timepoint, tissue, cell subset, condition, batch) with
     **Suggest from file names**;
   - the project document (`clonoloom-project` version 1) records datasets by path and
     fingerprint, samples and their annotation; a moved source is found again by its
     fingerprint.
   - Validation: a broken-data set made by a script from the synthetic fixtures (each fault
     planted once) raises exactly the expected findings; the development datasets validate with
     no fatal finding; no source file changes (checksums before and after every test).
4. **QC (Q1, Q2, Q3, Q6).** The QC view:
   - per-sample metrics: abundance in its unit, rearrangements, productive, out-of-frame and
     stop-codon shares, junction length distribution, V and J resolution, singletons, sample
     coverage (Chao and Jost 2012);
   - single-cell metrics: cells by chains per locus, pairing, multi-chain cells, UMIs per contig;
   - plots linked to the samples; filters (productive only, minimum abundance, loci, cells'
     chain counts) kept as named versions with reasons; nothing deleted.
   - Validation: metrics equal pandas' on the development files and hand-computed values on the
     synthetic fixtures; Cell Ranger's own `metrics_summary.csv` numbers recomputed where the
     contigs allow, differences explained.
5. **Clonotypes (C1, C2, C3, C6).** The Clonotypes view:
   - named definitions with presets (bulk TRB: amino-acid junction + V gene + J gene; paired
     αβ; 10x's own clonotypes as given) and an editor;
   - clonotype tables per sample and across samples, sorted and filtered pages of a million rows,
     each clonotype's rearrangements, cells and samples in the inspector;
   - the definition and abundance unit in the status bar and on every table.
   - Validation: clonotype counts and abundances equal pandas' group-bys and scirpy's
     `define_clonotypes` on the development data; Cell Ranger's own clonotypes reproduced by the
     definition that matches theirs, differences explained; the budget of 100 ms per page on a
     1M-row table.
6. **Public data (D13, E2, P4).** The Example data dialog:
   - the catalog of the wave's public datasets with source, size, license class and citation;
     downloads only after the user confirms, checked against their checksums, into
     `--examples-dir`; `--offline` turns them off;
   - an opened example becomes a dataset like any other, with its license and citation in its
     record.
7. **The record and the release (O2, N5, N6).** The project's history in the drawer; a methods
   paragraph per dataset (format, reader, validation, filters, definition, the data's source and
   license, ClonoLoom's version); `docs/FORMATS.md` and `docs/GETTING-STARTED.md`; README with
   screenshots; release 0.1.0 drafted.

## 0.2.0: repertoire basics (wave 2)

1. **Remote control and screenshots (A7).** SituCraft's hub and capture scenes.
2. **Simulated repertoires (E1).** `clonoloom simulate`: bulk TRB studies from a V(D)J
   recombination model of ClonoLoom's own (gene usage, deletions, N insertions), power-law clone
   sizes, sampling of templates and reads; the truth written beside the data.
3. **Artifacts, jobs and the cache.** SituCraft's `internal/jobs` and `internal/artifacts`; the
   rearrangement-table cache.
4. **Diversity (R1, R5).** Hill numbers, Chao1, ACE, Shannon, Simpson, evenness, clonality, Gini,
   top-N share, homeostasis, each with a bootstrap interval; validated against immunarch, iNEXT
   and closed forms; interval coverage on simulations.
5. **Rarefaction and extrapolation (R2, Q3).** Chao et al. 2014 by size and coverage; curves and a
   suggested common depth; validated against iNEXT.
6. **Gene usage and spectratypes (R3).** V, D, J, C and V–J usage; junction lengths; amino-acid
   composition.
7. **Overlap (R4).** Shared clonotypes, Jaccard, overlap coefficient, Morisita–Horn, rarefied;
   public clonotypes within the project; validated against immunarch and scirpy.
8. **Figures, exports and methods (O1, O2, O3, O9, D7).** SVG/PNG/PDF figures with the
   definition and unit in their legends; AIRR and CSV exports with license notes; generic CSV
   import; the methods paragraph for each analysis; release 0.2.0.

## 0.3.0: tracking (wave 3)

1. **Subjects and timepoints.** The study's subjects, timepoints and reference samples from the
   annotation; checked before tracking.
2. **Simulated longitudinal studies (E1).** Expansion and contraction trajectories with biological
   variance between replicates; CAR-T kinetics and transplant examples.
3. **Tracking (T1, T6).** Chosen clonotypes followed across a subject's samples with intervals;
   alluvial plots and clonotype threads.
4. **Two-sample tests (T2).** Fisher's exact and binomial tests with Benjamini–Hochberg;
   validated against R and the simulator's truth.
5. **Beta-binomial expansion (T3).** Rytlewski et al. 2019; dispersion from replicates or the
   published time-dependent model; false discovery controlled on null simulations.
6. **Classes and attribution (T4, T5).** Persistent, emerging and contracting clonotypes; the share
   of a sample found in a reference sample; release 0.3.0.

## Later waves (outlined)

| Wave | Release | Theme | Main content |
| --- | --- | --- | --- |
| 4 | 0.4.0 | Specificity | TCRdist (exact against tcrdist3), neighborhoods with backgrounds, generation probability (OLGA's algorithm; IGoR models on request), clonotype networks, VDJdb matching, invariant populations; MiXCR, OAS/OTS and VDJdb readers; vaccine and epitope examples |
| 5 | 0.5.0 | B cells | Germline reconstruction, clonal families with thresholds from distance to nearest, mutation by region, parsimony trees, isotypes; immunization example |
| 6 | 0.6.0 | Single cell | Expression linked by barcode, the map, phenotypes per clonotype, clonotype modularity and CoNGA-style analysis; AnnData, MuData and 10x matrices |
| 7 | 0.7.0 | Cohorts and robustness | Study design, group comparisons with the subject as unit, robustness to the definition, GLIPH2-style groups, HLA association, contamination and batch QC, pseudobulk |
| 8 | 0.8.0 | Record and reach | Certificates and `verify`, `run`, reports, MCP, Python and R clients, agent benchmark, archives, de-identification, container file, the website |
| 9 | 0.9.0 | Suite and public data | ADC queries, Proteoscope and CytoWeave links, MiAIRR checklist, Excel and Prism, likelihood trees, candidates, remaining readers |
| 10 | 1.0.0 | Hardening | Budgets met, accessibility audit, documentation complete, external beta feedback addressed |
