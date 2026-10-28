# ClonoLoom roadmap

The plan for building ClonoLoom. Each **wave** is one release: wave 1 is 0.1.0, wave 2 is 0.2.0,
and so on. Each wave is split into **slices**, bite-sized units of work. A slice ends with:
- something that runs;
- tests or validation that check it;
- its documentation;
- an entry in `CHANGELOG.md`.

Slices are done in order within a wave unless noted. Each slice names the requirements
(`requirements.md`) it serves. When a wave is reached, it is planned in detail: its slices are
confirmed or re-cut, and the decisions taken are recorded here. Waves 1–6 are complete. Wave 7 is underway below; the first four slices cover subject designs, features, comparisons and replicated counts.

**Progress** is marked here as each slice is committed: done, next or to come. Each
requirement's status is in `requirements.md`; release changes are in `CHANGELOG.md`.
**Where things stand:** wave 1 is merged (0.1.0 not tagged); waves 2–6 are merged and
released as 0.2.0–0.6.0, and the 0.6.0 product site is published. Wave 7 starts on
`wave7`. Slices 1–11 are committed and pushed. Slice 12 is implemented and validated, awaiting maintainer commit; no wave 7 release has been made.

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
- **Documentation.** The slice that builds a feature also writes its guide page on the website
  (`docs/site/pages/docs/`, from wave 3) and the capture scenes for its screenshots.
  `docs/FORMATS.md` grows with every format read or written. The README is the front page, laid
  out as the siblings' are. What each release brought goes in `CHANGELOG.md`.
- **Release.** Screenshots re-captured, binaries drafted by the release workflow, and the website
  published with the release's pages (`docs/site/README.md`).
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

**Done**: every slice committed, then a walk through the whole wave fixed what it found (AIRR
Repertoire metadata kept with their dataset across a restart; coverage not estimated under a
minimum count above one). Merged as robert-mcdermott/clonoloom#1. **Still to do:** tag v0.1.0,
which drafts the release.

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
   - *Done:* six binaries of about 7 MB each.
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
   - *Done:* 1.5 s for the million rows. Repertoire YAML and Cell Ranger's JSON annotations are
     not read (`requirements.md`, D2 and D3).
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
   - *Done:* 28 finding codes, each planted fault found once.
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
   - *Done:* equal to an independent R implementation on six bulk samples; Cell Ranger's counts of
     cells, pairs and loci reproduced exactly on six runs.
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
   - *Done:* 37 ms a page; equal to scirpy's partitions of eight Cell Ranger runs under four
     definitions.
6. **Public data (D13, E2, P4).** The Example data dialog:
   - the catalog of the wave's public datasets with source, size, license class and citation;
     downloads only after the user confirms, checked against their checksums, into
     `--examples-dir`; `--offline` turns them off;
   - an opened example becomes a dataset like any other, with its license and citation in its
     record.
   - *Done:* six datasets, of 10x Genomics, the AIRR Community and immuneACCESS.
7. **The record and the release (O2, N5, N6).** The project's history in the drawer; a methods
   paragraph per dataset (format, reader, validation, filters, definition, the data's source and
   license, ClonoLoom's version); `docs/FORMATS.md` and `docs/GETTING-STARTED.md`; README with
   screenshots; release 0.1.0 drafted.
   - *Done*, but for the release: 0.1.0 is drafted when v0.1.0 is tagged.

## 0.2.0: repertoire basics (wave 2)

Built on the branch `wave2`, one commit per slice. **Done**: merged as robert-mcdermott/clonoloom#2
and released as 0.2.0.

| Slice | Status |
| --- | --- |
| 1. Remote control and screenshots | Done |
| 2. Simulated repertoires | Done |
| 3. Artifacts, jobs and the cache | Done |
| 4. Diversity | Done |
| 5. Rarefaction and extrapolation | Done |
| 6. Gene usage and spectratypes | Done |
| 7. Overlap | Done |
| 8. Figures, exports and methods; release 0.2.0 | Done |

1. **Remote control and screenshots (A7).** SituCraft's hub and capture scenes.
   - *Done:* eleven actions (a twelfth, `measure_diversity`, came with slice 4). Reading files by
     path needs the token in `remote.json`; data under terms of use are never downloaded by a
     program. Wave 1's pictures are captured through it.
2. **Simulated repertoires (E1).** `clonoloom simulate`: bulk TRB studies from a V(D)J
   recombination model of ClonoLoom's own (gene usage, deletions, N insertions), power-law clone
   sizes, sampling of templates and reads; the truth written beside the data.
   - *Done:* GRCh38 TRB segments from Ensembl; V and J usage and junction lengths from 10x
     Genomics' 10k PBMC data (CC BY 4.0); palindromic nucleotides and selection on junction length
     added to the model. Presets standard, null and small, the same bytes for the same seed,
     offered in Example data.
3. **Artifacts, jobs and the cache.** SituCraft's `internal/jobs` and `internal/artifacts`; the
   rearrangement-table cache.
   - *Done:* example downloads and simulations run as jobs, shown in the Jobs view and resumable
     after a restart. The cache is keyed by each file's SHA-256, size and name and the readers'
     version, checked by CRC, and trimmed least-recently-used beyond `--cache-limit` (10 GB). A
     100-sample cohort reopens in 118 ms (budget 2 s).
4. **Diversity (R1, R5).** Hill numbers, Chao1, ACE, Shannon, Simpson, evenness, clonality, Gini,
   top-N share, homeostasis, each with a bootstrap interval; validated against immunarch, iNEXT
   and closed forms; interval coverage on simulations.
   - *Done:* with the asymptotic Shannon and Simpson diversities (Chao et al. 2013, 2014), Simpson
     clonality and sample coverage, in the Repertoire view. Validated against vegan (ACE) and R as
     well, on 18 samples.
   - *Decided:* the estimated Simpson diversity's interval is not floored at the value seen, as
     iNEXT's is: with the floor it held the truth in under half the simulated samples
     (`research.md` §5.3).
   - *Found:* immunoSEQ's clonality columns are computed from reads, not templates; QC now
     reproduces them.
   - *Moved to slice 8:* the Repertoire view's capture scene and picture.
5. **Rarefaction and extrapolation (R2, Q3).** Chao et al. 2014 by size and coverage; curves and a
   suggested common depth; validated against iNEXT.
   - *Done:* the Hill numbers of order 0, 1 and 2 and the coverage, by size and by coverage, in the
     Repertoire view's Rarefaction part, from the same bootstrap replicates as the measures; the
     interval coverage measured on simulations as well. A million-cell sample in about 2 s:
     iNEXT's subsample sums take every size, ClonoLoom's start at each distribution's mode.
   - *Decided:* the samples are compared first at levels reached by rarefaction alone (the lowest
     coverage among them, the smallest sample's size); iNEXT's defaults, extrapolated to twice a
     sample's size, come second. In simulations of repertoires with many rare clonotypes, values
     extrapolated that far were within 1–3% of the truth, but their intervals were narrower than
     that (`research.md` §5.4).
   - *Found:* in reads, few clonotypes are seen once, so every sample looks complete; both parts of
     the view now warn. Sizes spaced on a logarithmic scale added to iNEXT's, so the curves draw on
     a logarithmic axis.
   - *Moved to slice 8:* the Rarefaction part's capture scene, with the Repertoire view's.
6. **Gene usage and spectratypes (R3).** V, D, J, C and V–J usage; junction lengths; amino-acid
   composition.
   - *Done:* the Repertoire view's Genes and Junctions parts, from one analysis: each segment's
     genes and families, V–J pairs, junction lengths and their mean, amino acids overall and at
     each IMGT position of the commonest lengths; by clonotypes or by abundance, with intervals
     from a bootstrap of each sample's clonotypes; conditions as the mean of their samples, each
     counted once. Validated against an independent R implementation (15 samples) and the
     simulator's truth.
   - *Decided:* a call tied between genes gives each an equal part (immunarch counts the tie as a
     gene of its own); a clonotype's genes are its rearrangements', in proportion to their counts.
   - *Found:* plot axes could end below their largest value, clipping the tallest column, line or
     interval (QC's junction lengths since wave 1): every axis now holds its values.
   - *Moved to slice 8:* the Genes and Junctions parts' capture scenes.
7. **Overlap (R4).** Shared clonotypes, Jaccard, overlap coefficient, Morisita–Horn, rarefied;
   public clonotypes within the project; validated against immunarch and scirpy.
   - *Done:* the Repertoire view's Overlap part: every pair of samples by each measure, with
     Sørensen–Dice and Morisita's own index besides, the rarefied forms as exact expectations, 95%
     intervals; the pairs of one subject and of two summarized; the public clonotypes by subject.
     Validated against immunarch, vegan and R (69 pairs) and scirpy (four single-cell runs), and on
     five simulated studies against their truth.
   - *Decided:* Morisita's own index is the measure shown first: the Morisita–Horn index (immunarch's
     and vegan's) is biased low in shallow samples, and two samples of one simulated repertoire read
     about 0.98 by it.
   - *Found:* a bootstrap of the samples' cells alone gave intervals that held the expected shared
     clonotypes in two pairs of three: which clonotypes two samples share is itself random. With
     that variance added (Horvitz–Thompson) and the intervals on the logarithmic scale, 95%.
   - *Moved to slice 8:* the Overlap part's capture scene.
8. **Figures, exports and methods (O1, O2, O3, O9, D7).** SVG/PNG/PDF figures with the
   definition and unit in their legends; AIRR and CSV exports with license notes; generic CSV
   import; the methods paragraph for each analysis; release 0.2.0.
   - Also: the capture scenes of the views wave 2 built, the README's pictures re-captured, and
     the first project fixture under `validation/projects/` (see "In every wave").
   - *Done:* every plot of the QC and Repertoire views saved as SVG, PNG (300 dpi) or PDF, its
     caption stating the definition, unit, samples, filter and version. The rearrangements exported
     as AIRR TSV, the clonotypes and each result as CSV, each in a zip with a NOTICE of the data's
     files, method, license and citation; rows under terms of use only once the terms are accepted.
     A table no reader recognizes opened through a mapping of its columns, checked column by column
     first, kept with the dataset and stated in its methods. The Repertoire view's five parts
     captured. The first project fixture, `validation/projects/0.2.0`, made through remote control
     and opened again by a test, its datasets under the same IDs and its five results made again.
   - *Decided:* the fixture holds its data (the small simulated study, gzipped, and a VDJtools
     table: 200 KB in all) rather than simulating them again, so a later simulator may change. A PDF
     figure holds the 300-dpi PNG: a vector PDF would need a PDF writer for text and paths, and
     the SVG is the vector form.
   - *Found:* results are not yet the same on every processor, as `conventions.md` requires. The
     arm64 build fuses 98 multiply-adds in diversity, rarefaction, usage, overlap, QC and the random
     draws, and these call Go's `math.Log`, `Exp` and `Pow`, which differ between processors
     (`internal/dmath` was never written). On amd64 the fixture's diversity, rarefaction and usage
     documents differ from arm64's in their last digits (at most 0.1% of a relative 10⁻⁹), and its
     overlap document is the same. So the fixture is compared value by value until results are made
     the same, which wave 8's certificates (bit for bit) need.
   - Plot axes leave room for their last tick's label, which was cut off at the right of line
     and curve plots.

## 0.3.0: tracking (wave 3)

A researcher follows clonotypes through each subject's samples over time and gets:
- the clonotypes that expanded or contracted between two samples, tested with the sampling and,
  where replicates allow, the biological variance accounted for, with false discoveries controlled;
- each chosen clonotype's trajectory with its intervals, as threads and alluvial plots;
- what share of a sample comes from a reference sample (an infusion product, a donor);
- results that are the same, bit for bit, on every computer.

Built on the branch `wave3`, one commit per slice. Planned in detail at the wave's start, after
slice 1: the website was moved here from wave 8, so that each wave from now on adds its own pages
to it, and making results the same on every processor was added, so that the wave's new methods
are written under that rule from the start. **Done**: merged as robert-mcdermott/clonoloom#3 and
released as 0.3.0.

| Slice | Status |
| --- | --- |
| 1. The website | Done |
| 2. Results the same on every processor | Done |
| 3. Subjects and time points | Done |
| 4. Simulated longitudinal studies | Done |
| 5. Tracking | Done |
| 6. Two-sample tests | Done |
| 7. Beta-binomial expansion | Done |
| 8. Classes and attribution; release 0.3.0 | Done |

### Decisions for wave 3

| Decision | Choice |
| --- | --- |
| Time | A sample's time point is a number of days relative to a subject's reference event (an infusion, a transplant, a vaccination: day 0), read from the annotation (`day 28`, `week 4`, `month 3`, `baseline`, `pre`); its unit kept; samples without one are left out of tracking, and the view says so |
| Reference samples | A subject's sample marked as its reference (an infusion product, a donor, a baseline), from the annotation's tissue or a field of its own, used by tracking (its clonotypes followed) and attribution |
| Default expansion test | The beta-binomial test (Rytlewski et al. 2019) when the study has replicate pairs (two samples of one subject at one time, or a pair the user names as stable) to fit its variance; otherwise the binomial test, labeled in every result and the methods as ignoring biological variance. No published variance parameters exist to build in: Rytlewski et al. fit them per study, and their reference code carries no license |
| False discoveries | Benjamini–Hochberg at 0.01 by default (Rytlewski et al.'s); Storey's q-values offered, to reproduce DeWitt et al. 2015 |
| What is tested | Clonotypes with at least 5 cells or molecules in the two samples together (Rytlewski et al.'s `minTotal`); fewer are left out of the test and of its false discovery count, and counted |
| Units | Tests need counts of cells or molecules (templates, UMIs, cells); in reads, each molecule is counted many times over, and the test refuses, saying why |
| Validation data | Simulated longitudinal studies with known trajectories and nulls (public longitudinal data with known expansions are scarce); DeWitt et al. 2015's yellow fever vaccination series (immuneACCESS, terms of use) for maintainer validation where its terms allow |

1. **The website (N5).** The product and documentation site, built from `docs/site/` and published
   on GitHub Pages from a `gh-pages` branch that holds only the built site, as the siblings' are:
   a home page, install, the methods with their validation, and the user guide a page a subject.
   - *Done:* SituCraft 0.9.0's generator, audit and styles adapted (`docs/site/build.mjs`,
     `audit.mjs`, `assets/`); thirteen guide pages made from `docs/GETTING-STARTED.md`, which now
     points to them; every local link, anchor and screenshot checked by the build in CI, and every
     page checked with axe-core in both themes at a desktop's and a phone's width.
   - *Decided:* the guide lives on the site only, so that it is written once; the references a
     script or another tool needs (formats, remote control, simulated studies, installing) stay in
     `docs/`. The site is published by hand from a checkout of `gh-pages` when a release is out,
     as the siblings' are.
2. **Results the same on every processor (N7).** What `conventions.md` already requires, and what
   0.2.0 found was not so (wave 2, slice 8): the same data and settings give the same bytes on
   arm64 and amd64.
   - `internal/dmath` from SituCraft 0.9.0: fdlibm's logarithm, exponential, power, `log1p`,
     log-gamma and `erfc`, and `expm1` added, every product rounded on its own; every call of Go's
     `math.Log`, `Exp`, `Pow`, `Log1p`, `Expm1`, `Lgamma`, `Log2` and `Erfc` that reaches a result,
     a random draw or a simulated study replaced.
   - Every product added into a sum that reaches a result rounded on its own, in diversity,
     rarefaction, usage, overlap, QC, the random draws and the simulator.
   - Tested: a test searches the compiler's arm64 output of every package that makes results for
     fused multiply-adds, and fails on one not known to be exact; the digests of every analysis's
     result on the small simulated study are pinned, and CI checks them on amd64 against those
     made on arm64; the project fixtures from 0.3.0 on are compared by checksum (0.2.0's, saved
     before, by value).
   - The simulated studies, and every reference comparison, are checked to be unchanged.
   - *Done:* `internal/dmath` (fdlibm's log, exp, pow, erfc, log-gamma and log1p from SituCraft,
     whose fingerprint they reproduce bit for bit, and `expm1` ported from fdlibm, within an ulp of
     256-bit values); 102 fused multiply-adds removed from diversity, rarefaction, usage, overlap,
     QC, the random draws and the simulator, and every call of Go's logarithms and exponentials
     there replaced. `validation/fused_test.go` searches the arm64 assembly of every function of the
     module (1,360) and finds none; the 0.2.0 project's five results and the small simulated study
     are pinned by digests that an amd64 build gives too.
   - *Found:* the simulator's samples and truth populations came out byte for byte as before (every
     reference comparison, made on them, still holds); only its truth's expectations moved, in
     their thirteenth digit. Products a helper returns can fuse with the caller's sum once it is
     inlined (rarefaction's Stirling series did), and a product kept in a variable can fuse with a
     later statement: both are rounded where they are made.
   - *Decided:* 0.2.0's fixture stays compared by value (its results were made with the fused
     arithmetic); fixtures from 0.3.0 on, by checksum. No method's version changed: the results
     changed in their last digits only, and results kept under 0.2.0 are still answered from the
     cache.
3. **Subjects and time points (D10, G3).** Each sample's subject, time point and reference role,
   from the annotation (`Time`, above); a subject's samples in order, checked before tracking:
   samples without a time point, two samples of one subject at one time (replicates, offered for
   the variance), subjects without a reference sample. Remote control sets the annotation.
   - *Done:* `internal/study` reads time points (days, weeks, months, years and hours in their
     common spellings, AIRR's "14 day (vaccination)", baseline and pre, post, visits) into days from
     the subject's reference event, orders each subject's samples, finds its reference samples (the
     annotation's new Reference field, else a tissue such as infusion product or donor) and its
     replicates, and checks the design (eight finding codes); `POST /api/study`; the **Tracking**
     view's first part, Subjects, with a timeline of each subject's samples (a figure) and a table;
     remote control's `annotate_samples` and `describe_subjects`; the guide's page.
   - *Decided:* the design is read by the program from the annotation each time, and not kept: the
     annotation, in the project, is the record. A month is a twelfth of 365.25 days. A number
     without a unit is days, and the check says so. Baseline comes before every dated sample, and
     a time after the event without its day after every one. Visits are ordered among themselves,
     not placed among days.
   - *Moved to slice 4:* the Subjects part's capture scene, on a simulated longitudinal study (no
     example has time points yet).
4. **Simulated longitudinal studies (E1).** `clonoloom simulate` presets with each clone's
   trajectory known: CAR-T kinetics (an infusion product and post-infusion samples, some of its
   clones expanding, persisting or contracting), transplant (a donor and a recipient's
   reconstitution), and a null series with biological variance between replicate samples (each
   clone's frequency drawn around its mean, as Rytlewski et al. measured between replicates);
   offered as examples with their truth.
   - *Done:* a `series` in a study's parameters (`internal/simulate/series.go`): time points with
     their replicates, trajectories (classes of clones and their folds), a reference population
     (an infusion product made from the subject's own blood, or a donor) in a share of the blood at
     each time, and the variance between samples; presets `car-t`, `transplant` and `null-series`,
     the first two in the Example data dialog. Each sample's time point and the reference's tissue
     are in its AIRR Repertoire metadata, so the Tracking view places every sample without
     annotation. The truth adds each sample's day, the reference's share of it and the expected
     share in the reference's clonotypes, and each clonotype's expected frequency at every time
     point. The simulated series' methods paragraph says how they were made. The Subjects part's
     capture scene, moved here from slice 3, on the CAR-T example.
   - *Validation:* every preset reads with no finding and is placed as made (subjects, days,
     references, replicates); attribution within 0.016 of the truth in every one of 33 samples;
     the CAR-T product's expanding and contracting clonotypes, pooled, within 6% of their expected
     share at every time point, and the transplant's 80 alloreactive ones within 16%;
     replicates differing by 0.218 beyond sampling over 298 clonotypes, the model's 0.210; the null
     series' expectations the same at every time.
   - *Decided:* the cross-sectional presets come out byte for byte as before: a series is made by
     its own path. Every sample, replicates at one time included, draws its own biological
     variance, so that replicates measure what the expansion tests must allow for.
   - *Found:* a product made from the subject's own blood holds its largest clonotypes, so about
     three quarters of the blood is in the product's clonotypes before the infusion: attribution by
     clonotype is not the infused share, and the truth gives both (slice 8 must say so).
5. **Tracking (T1, T6).** The **Tracking** view: a subject's samples over time; the clonotypes
   chosen (each sample's largest, a reference sample's, a list, or those a test called), followed
   with each frequency's 95% interval; threads (each clonotype's trajectory) and an alluvial plot,
   linked to the clonotype table; figures, exports, the methods paragraph and remote control's
   `track_clonotypes`.
   - *Done:* `internal/track` follows each reference sample's largest clonotypes (each sample's
     largest for a subject without one), each sample's largest, or a list by ID or junction through
     each subject's time points: the samples taken at one time pooled, a reference sample its own
     point, each frequency with Wilson's score interval; `api_tracking.go`, an analysis whose ID
     holds the samples' annotation and the choice; the Tracking view's Clonotypes part with
     threads, an alluvial plot and a table, linked to each other and to the Clonotypes view (whose
     inspector adds a clonotype to the list with Follow over time); CSV export, the methods
     paragraph's sentence, `track_clonotypes`, the guide's page and capture scene.
   - *Validation:* Wilson's interval equal to R's `prop.test` (without continuity correction) to
     10⁻¹⁴; on the CAR-T example made without biological variance, the intervals held the true
     frequency in 95.2% of 2,700 blood-sample checks; with the example's variance, in 76.9%.
   - *Decided:* time points are equally spaced in the threads and the alluvial plot, in time order
     (the Subjects part's timeline keeps the days' scale); a clonotype not found is drawn on its
     own lane, never at zero on a logarithmic axis; the choice is kept with the dataset (a list is
     kept whatever is followed), and a choice the user makes is followed at once; in reads, no
     interval is given; a listed clonotype is followed in every subject, found or not.
   - *Found:* Wilson's interval is for sampling alone, and blood samples vary beyond it: the view,
     the methods paragraph and the Science page say so, and the tests of slices 6 and 7 are where
     that variance is modeled. A script's edit is noticed by the view a moment later, so a script
     waiting for the view must wait for its latest load, not the first (remote control's
     `track_clonotypes` does).
6. **Two-sample tests (T2).** DeWitt et al. 2015's test of each clonotype between two samples:
   Fisher's exact test, or the binomial test it equals conditional on the clonotype's total, with
   the share of the first sample's depth as its null; two-sided by default; Benjamini–Hochberg and
   Storey's q-values. Validated against R's `fisher.test` and `binom.test` and, on null
   simulations, false discoveries counted.
   - *Done:* `internal/expansion` (the binomial test conditional on the clonotype's total, at the
     share of the depth, and Fisher's exact test, each two-sided by R's rule or one-sided;
     Benjamini–Hochberg; Storey's π₀ and q-values); `track.Test`, the comparison of two time points
     in each subject, the samples at one time pooled (each placed sample now has a key, `day 28`
     for week 4, `baseline`, `reference`, naming its time across subjects); `api_expansion.go`, an
     analysis whose ID holds the comparison; the Tracking view's Tests part (`tracking-tests.js`):
     each clonotype at its two frequencies with the calls colored, a table of the calls, and
     Follow the calls, which the Clonotypes part follows as a new choice (`by: test`); CSV export,
     the methods paragraph's sentence, `test_expansion`, the guide's page and capture scene.
   - *Validation:* both tests against R on 215 cases, two-sided and one-sided, to 4 × 10⁻¹²; BH and
     Storey against R; on the null series, no call of 6,634 clonotypes without variance between
     samples beyond sampling, 13 of 6,599 (0.2%) with the series' variance (Rytlewski et al.: 0.28%
     between blood samples two weeks apart); on the CAR-T example, all 66 calls in the true
     direction without that variance, 6 of 73 against the truth with it; 48 of the 138 expanding
     clones tested called (most are small).
   - *Decided:* the binomial test is the default until slice 7 (Rytlewski et al.'s, and the one the
     beta-binomial extends); Fisher's is offered, as DeWitt et al. used it. Storey's π₀ is estimated
     at λ = 0.5 (Storey 2002), not by the qvalue package's spline, which is not reproduced; its
     q-values are π₀ times BH's, as that package's are. False discoveries are controlled within each
     subject. A comparison takes every subject with both time points, and names those without. The
     tests refuse reads.
   - *Found:* where one clonotype grows to a large share, every other one's share shrinks, so the
     tests call some unchanged clonotypes decreased (DeWitt et al. assume the changed clonotypes'
     total change is small); and the tests account for sampling alone, which the view, the export
     and the methods paragraph say.
7. **Beta-binomial expansion (T3).** Rytlewski et al. 2019: the variance model
   v(n) = 2 + (b ln n + a)² (the 2, which keeps the beta unimodal, is in their reference code and not
   their paper) fitted by maximum likelihood on the study's replicate pairs, and the
   beta-binomial test with it, two-sided by summing the outcomes no more likely than the one seen;
   the default where a variance model exists. Validated against an independent R
   implementation and, on simulated series with biological variance, false discoveries
   controlled where the binomial test's are not.
   - *Done:* `internal/expansion/betabinomial.go`: the beta-binomial p-value (two-sided by R's rule,
     or one-sided), its log-likelihood, and the fit (Nelder–Mead from (1, 1) with scipy's
     coefficients and simplex, restarted from its best point until it no longer improves; b made
     positive, as (a, b) and (−a, −b) are one model; v at most 10⁷, beyond which it is the
     binomial). `track.Test` fits once for the study, on every pair of each subject's replicate
     samples or on two time points named as stable (each subject's samples at them pooled), each
     pair's θ the share of its clonotypes' total in the first, as Rytlewski et al.'s code takes it;
     the test `auto`, the new default, is the beta-binomial where it can be fitted, else the
     binomial, and says so. The Tests part's variance card; the fit exported, in the NOTICE and the
     methods paragraph; `test_expansion`'s `stable`.
   - *Validation:* p-values against an independent R implementation (150 cases) to 2 × 10⁻¹²; the
     fit against R's `optim` (Nelder–Mead, then BFGS) on two sets of 3,000 clonotypes drawn from the
     model with known (a, b): the same likelihood to 10⁻⁶, the same v(n) to 10⁻³. On the null
     series with the variance 0.1 between samples, no call (the binomial test: 13); between its
     replicates, a test at 0.05 called 8.2% by sampling alone, 2.7% with the model. On the CAR-T
     example, none of the beta-binomial test's 20 calls against the truth (the binomial test's: 6
     of 73), and 19 of the 133 expanding clones tested called (the binomial test: 45).
   - *Decided:* one model for the study, from every subject's replicate pairs (all pairs of a group
     of more than two), taken to hold at every time point and in every subject; the user may name
     two time points as stable instead. Clonotypes with fewer than minTotal in a pair are left out
     of the fit, as of the tests. The scale is capped at 10⁷.
   - *Found:* on the null series, the simulator's variance (a gamma of variance 0.1 on each clone's
     frequency) is fitted by v(n) near 20 at every n (b close to 0); without it, v is large (10³ to
     10⁵ over the counts seen), and the beta-binomial test calls what the binomial does. Allowing for the variance costs power: the
     beta-binomial test calls fewer of the clones that did expand.
8. **Classes and attribution (T4, T5).** Across several time points, each clonotype classed as
   persistent, emerging, contracting or transient from the tests' calls; the share of a sample's
   repertoire found in a reference sample (an infusion product, a donor), with its interval and
   the definition stated; the CAR-T and transplant examples' guided steps; release 0.3.0 with its
   project fixture.
   - *Done:* `track.Classify`: in each subject, each clonotype tested between the base time point
     and every later one (each key once, reference samples aside) by the comparison's test, its
     variance fitted once for the study, the false discovery rate controlled within each
     comparison, and classed: emerging, transient, contracting or persistent (the others counted,
     not classed); `track.Attribute`: each sample's and time point's share in the reference
     sample's clonotypes, and, where the subject has samples before the event and at most a quarter
     of the reference's cells are in clonotypes found before it, the share derived from the
     reference, from the clonotypes found on one side only, each with Wilson's interval; the share
     before the event, and that overlap. `api_classes.go`, two analyses; the Tracking view's Classes
     and Attribution parts; the Clonotypes part follows a class; exports, the methods paragraph,
     `class_clonotypes` and `attribute_samples`. The simulated CAR-T and transplant examples' guided
     steps, listed on their cards and done in the Tracking view by the remote control actions
     (`app.actions`). Release 0.3.0: its project fixture (a small simulated CAR-T series, 448 KB,
     with its diversity, tracking, tests, classes and attribution), every picture re-captured, the
     website's pages.
   - *Validation:* on the CAR-T example with its variance, all 20 clonotypes classed by their calls
     classed the way the truth moved; attribution within 0.023 of the share the truth expects in the
     reference sample's clonotypes, at every sample of the CAR-T and transplant examples; the share
     derived from the donor within 0.065 of the truth (60% to 97%), withheld for the CAR-T products;
     the 0.3.0 fixture's five results made again byte for byte.
   - *Decided:* attribution counts the clonotypes the reference *sample* holds, the only ones
     observed; the truth's attribution, over the whole reference population, is not what can be
     measured, and the share derived from the reference is offered where it means something (a
     donor) and withheld, with the reason, where it does not (a product made from the patient's
     blood). Its interval is Wilson's, the limit of a bootstrap of the sample's cells, so no random
     draws. A class is decided by the last time point first; several comparisons make false calls
     likelier, and the view says so. Guided steps are the remote control actions, so a step and a
     script do the same.
   - *Found:* the share in the reference sample's clonotypes runs 0.17 to 0.28 below the share in
     the whole reference's, its sample missing some of them; and for a product made from the
     patient's blood, 74–79% of its cells are in clonotypes found before the infusion. On the
     CAR-T example, the beta-binomial test finds the product's expansions transient (up at day 14,
     back by day 90); none emerging or contracting at day 90.

## 0.4.0: specificity (wave 4)

A researcher asks which of their T-cell receptors may recognize the same antigen, and gets:
- distances between receptors (TCRdist) equal to tcrdist3's, on one chain or both;
- each receptor's generation probability under a named recombination model;
- neighborhoods of similar receptors, tested against a background of what recombination alone
  makes, and networks of the clonotypes that resemble each other;
- the receptors whose specificity is published (VDJdb) and the invariant populations (MAIT, iNKT,
  GEM), which Tracking follows over time like any chosen clonotypes.

Built on the branch `wave4`, one commit per slice. Planned in detail at the wave's start. The
readers come first, as in wave 1; TCRdist and generation probability before the methods built on
them; and a simulated response to an antigen, whose truth says which receptors share a
specificity, before the neighborhoods and networks validated on it, as wave 3's simulated series
came before its tests. Overlap with public cohorts (R4) moves to wave 9, whose AIRR Data Commons
queries make a public cohort more than one example's files.

| Slice | Status |
| --- | --- |
| 1. Readers of wave 4 | Done |
| 2. TCRdist | Done |
| 3. Generation probability | Done |
| 4. Simulated antigen-specific responses | Done |
| 5. Neighborhoods and backgrounds | Done |
| 6. Clonotype networks | Done |
| 7. Specificity databases | Done |
| 8. Invariant populations and the examples; release 0.4.0 | Done; released and site published |

### Decisions for wave 4

| Decision | Choice |
| --- | --- |
| TCRdist | As tcrdist3 computes it (Mayer-Blackwell et al. 2021) where it differs from Dash et al. 2017's description: the CDR3's gap placed where it costs least, 12 for each residue of difference in the CDR3s' lengths (the paper says 8), and the gap character of the V gene's aligned loops costing nothing against a residue. Distances are integers, equal to tcrdist3's chain by chain; a pair's distance is the sum of its chains'. The paper's variant is not offered |
| The V gene's loops | CDR1, CDR2 and CDR2.5 of each V allele as tcrdist3's database aligns them (`combo_xcr_2024-03-05.tsv`: MIT; human, mouse and rhesus; made from IMGT's germlines, CC BY 4.0 since July 2026), downloaded on request with its checksum, as an example is: germline data are not bundled until IMGT's terms are confirmed (`research.md` §7.1, §8). Without it, TCRdist is on the CDR3 alone, and every result says which. A V gene called without its allele takes its \*01 allele's loops, and says so |
| Recombination models | The simulator's own human TRB model, built in (the one the simulated studies are made from), and OLGA's default models (human TRB, TRA, IGH, IGK and IGL; mouse TRB and TRA), downloaded on request from OLGA's repository at a pinned commit with their GPL-3.0 notice: their parameters are not bundled (`research.md` §7.1). Generation probability is computed as OLGA 1.3.0 computes it, its choices included (every insertion length the model allows, the palindrome made from the gene's part of the CDR3, the genes its anchor files mark functional), so that it agrees with OLGA to 10⁻⁹; but not OLGA's omission of the longest insertion at one codon position (slice 3, *Found*). Every result names its model and the model's checksum |
| Backgrounds | A neighborhood is compared with receptors drawn from a recombination model with the sample's V and J usage (each weighted by its V–J pair's share in the sample over its share in the draws, as Mayer-Blackwell et al.'s are), with the sample's own receptors with their CDR3s shuffled, or with a public repertoire the user opens (an OTS unit). A neighborhood's radius is the largest at which at most one background receptor in a million is a neighbor (Mayer-Blackwell et al.'s 10⁻⁶; tcrdist3's default is 10⁻⁵) |
| Specificity databases | VDJdb, downloaded on request (its legacy release files) once its license (CC BY-NC-ND 4.0 since September 2026: non-commercial use) is shown and accepted, the release recorded; never bundled, and no fixture derived from it, so CI uses a simulated database. McPAS-TCR (no license published) and IEDB (CC BY 4.0) later, as their terms allow |
| Units and definitions | Specificity works on clonotypes, whatever the unit: distances, generation probabilities and networks need no counts. TCRdist needs each clonotype's amino-acid junction and V gene: a definition without V gives TCRdist on the CDR3 alone, and a paired definition paired distances |
| Development data | Dash et al. 2017's epitope-specific TCRs as tcrdist3 distributes them (mouse, `dash.csv`, 1,924 clones; human, `dash_human.csv`, 415; MIT), for exact TCRdist against tcrdist3 run in `validation/reference/`; OTS paired units (CC BY 4.0) for the paired readers, the budgets and a public background; PyClonal's MiXCR clone tables (MIT), and MiXCR 4 tables without a license for the maintainer only; OLGA, run in `validation/reference/`, for generation probabilities; scirpy for networks |

1. **Readers of wave 4 (D5, E2).** ClonoLoom's own readers of:
   - MiXCR clone tables (MiXCR 2's, 3's and 4's column sets; a file per chain or all chains): the
     junction (`nSeqCDR3`, `aaSeqCDR3`), each segment's best hit (its score dropped, MiXCR's `*00`
     read as no allele), and reads (`cloneCount`, `readCount`) or molecules
     (`uniqueMoleculeCount`) as the unit;
   - OAS and OTS data units: the metadata line kept as the sample's (subject, species, tissue,
     cell type, disease, vaccine, treatment, age) and offered as its annotation; unpaired units
     as bulk rearrangements, paired units (`_heavy` and `_light`, `_beta` and `_alpha`) as cells
     with their two chains;
   - tcrdist3's clone tables (the columns of `dash.csv`): a TCR's α and β chains on each row
     with its count of cells, a sample for each subject and epitope, the epitope kept with the
     sample.
   - Examples: Dash et al. 2017's mouse and human epitope-specific TCRs (tcrdist3's files) and a
     healthy donor's OTS unit.
   - Validation: every value read equal to pandas' reading of PyClonal's MiXCR tables, three OTS
     units, an OAS unit and Dash et al.'s files, and, for the maintainer, of MiXCR 4 tables; each
     reader fuzzed.
   - *Done:* `internal/reads/mixcr.go`, `oas.go` and `tcrdist3.go`, each format recognized by its
     header (an OAS or OTS unit by its metadata line); MiXCR 4's single-cell exports read as cells
     (each row a clone's chain in a cell, MiXCR's clonotype of cells as its clone); three new
     findings (`molecules-missing`, `cell-count`, `unit-metadata`); Fill from metadata offers the
     units' subject, tissue, cells sorted and disease and tcrdist3 tables' subjects and epitopes;
     the methods paragraph says how each was read, citing OAS or OTS by the units' loci; the
     examples, Dash et al.'s mouse and human TCRs and a healthy donor's OTS unit (4,424 cells);
     `docs/FORMATS.md`, the guide's Opening data page and `research.md` §1.6.
   - *Validation:* every column of ten more files equal to pandas' reading (PyClonal's two MiXCR 3
     tables, three OTS units, two OAS units, Dash et al.'s two tables, and MiLaboratories' MiXCR 4
     single-cell table on the maintainer's computer), each unit's metadata as given; 35 files in
     all; the readers fuzzed with 21 million inputs without a fault.
   - *Decided:* a tcrdist3 clone of n cells is read as n cells, since clonotype tables of single
     cells count cells; a table of one chain is bulk. A MiXCR call is every hit of the best score,
     as an AIRR call lists ties; a MiXCR table is counted in molecules only when every clone has
     them. OAS's `Redundancy` does not say what it counted, so the user confirms the unit. Dash et
     al.'s epitope is offered as each sample's condition. The example's OTS unit is a healthy,
     untreated donor's (a unit of donors given thymosin α1 was left aside).
   - *Found:* the reference script read every file under `validation/cache/`, the simulated studies
     the tests make included, had it been run again: it now reads the data sets of `sources.json`
     only. Real tables hold junctions without their conserved residues (62 of Dash et al.'s 6,810
     mouse chains, 25 of the OTS donor's 8,848), which the validator notes. In the clonotype table,
     a paired clonotype's long α junction ran over its β chain: each is now cut with an ellipsis.
2. **TCRdist (S1).** `internal/tcrdist`: the substitution matrix (4 less BLOSUM62's score, at most
   4), the CDR3's distance (its first three and last two residues left out, its gap where it costs
   least), the V gene's loops from tcrdist3's database, downloaded on request (a Reference data
   dialog beside Example data); single chains and pairs. The neighbors of a dataset's clonotypes
   within a radius, computed in parallel; the Specificity view's first part, each clonotype's
   nearest neighbors, and the inspector's.
   - Validation: every distance between Dash et al.'s clones (about 1.85 million pairs of mouse
     TCRs and 86,000 of human), α, β and paired, equal to tcrdist3's; the budget measured on OTS
     units (the neighbors of 100,000 paired TCRs in under 2 minutes on 10 cores).
   - *Done:* `internal/tcrdist` (the matrix from BLOSUM62, the CDR3's and the loops' distances,
     tcrdist3's database of genes read and looked up, the receptors of a clonotype table grouped by
     their chains, neighbors in parallel with a lower bound that skips far pairs, a receptor's
     nearest and the distance loop by loop); the reference data `tcrdist3-loops` in
     `sources.json`'s new `references`, downloaded on request through a Reference data dialog
     (`api_references.go`, the example dialog's code); the analysis `tcrdist` (`api_tcrdist.go`)
     and `POST /api/tcrdist/nearest`; the Specificity view's Neighbors part; CSV export, the methods
     paragraph's sentence, `find_neighbors`, the guide's page and capture scene.
   - *Validation:* every distance between Dash et al.'s 1,924 mouse and 415 human clones (3.87
     million pairs, rather than the half the plan counted: each pair both ways), α, β and paired,
     and each loop's alone, equal to tcrdist3 0.3's; every clone's neighbors and nearest at radii 0,
     24, 48 and 96 equal to those of every distance; the neighbors of 100,000 paired TCRs (the OTS
     donors' α and β chains paired at random) in 25 s on 10 processors, within 0.1 GB.
   - *Decided:* the radius 24 TCRdist units a chain by default (48 for a pair: two CDR3
     substitutions of the most different residues); the organism whose database names the most of
     the V genes, unless chosen; a V gene called without its allele (a definition by gene keeps
     none) read as its \*01, a tie as its first gene, and the result lists them; a definition by V
     family, or without V, compares the CDR3s alone; B-cell clonotypes are left out, counted.
     A clonotype's nearest are computed when asked, not kept. Reference data are listed in
     `sources.json` beside the development data and the tools, so that CI fetches them too.
   - *Found:* tcrdist3 keeps each loop's matrix before its weight, the CDR3's a third of what
     enters the chain's sum. Paralogous V genes with the same loops (mouse TRAV3-3 and TRAV3D-3)
     make receptors of different V genes 0 apart, as tcrdist3 does. On a healthy donor's 3,576
     paired clonotypes, 1% had a neighbor within 48; on Dash et al.'s epitope-sorted mouse TCRs,
     54%. tcrdist3 needs parasail, whose macOS wheels are for x86_64 only: its reference is made
     under Rosetta, with numba 0.60.
3. **Generation probability (R6).** `internal/pgen`: IGoR's model files read; OLGA's dynamic
   programming for amino-acid and nucleotide CDR3s, with V and J masks; receptors drawn from a
   model (for backgrounds); the simulator's model in the same terms. Each clonotype's generation
   probability in the clonotype table and the inspector, and their distribution in each sample in
   the Specificity view.
   - Validation: against OLGA (its version pinned) on 100,000 CDR3s of OTS units and simulated
     samples, to a relative 10⁻⁹; the simulator model's probabilities against the frequencies of
     junctions among the 10⁸ rearrangements it draws; the budget (a million CDR3s in under 2
     minutes on 10 cores).
   - *Done:* `internal/pgen` (IGoR's model files read, OLGA's anchors and gene rules; the
     probability of an amino-acid or nucleotide CDR3 with V and J masks, forward from the V and
     backward from the J over codon states, in parallel; receptors drawn from a model; the
     simulator's model in the same terms, `pgen.Simulator`); the reference data `olga-models` (OLGA's
     seven default models at a pinned commit); the analysis `pgen` (`api_pgen.go`) and `POST
     /api/pgen/chains`; the Specificity view's Generation part (each sample's median and middle
     half, the distribution, the clonotypes by abundance or probability) and the inspector's line;
     CSV export, the methods paragraph's sentence, `measure_generation`, the guide's page and
     capture scene.
   - *Validation:* 99,949 CDR3s under OLGA's seven models (the OTS, Dash and OAS development
     data's chains, CDR3s OLGA draws from each model, 2,094 nucleotide junctions, 1,000 with their
     genes, 200 with residues of any amino acid) within a relative 3 × 10⁻¹⁵ of OLGA 1.3.0's;
     under the simulator's model, 200 junctions' frequencies in 20 million rearrangements as their
     probabilities say (mean squared z 0.96), and the generator's under its model and OLGA's human
     TRB (1.08, 0.89); a million TRB CDR3s in 32 s on 10 processors.
   - *Decided:* OLGA's models where downloaded, else ClonoLoom's model of human TRB, unless chosen;
     the organism whose models name the most V genes, unless chosen; with the V and J genes as an
     option (a gene without its allele stands for each allele), off for a definition by family or
     without genes; a pair's probability the product of its chains', none unless each has one. The
     clonotype table has no column of probabilities (each page would need a job): the Generation
     part's table orders the clonotypes by it, and the inspector shows the one chosen. 20 million
     simulated rearrangements rather than 10⁸ give each of the 200 junctions enough counts.
   - *Found:* OLGA 1.3.0 sums every insertion length (no truncation, which `research.md` §5.9 had
     assumed), builds a gene's palindrome from its part of the CDR3 (a V whose CDR3 part is three
     nucleotides gets at most three), and reads the DJ insertion from its J end. It leaves out an
     insertion of the longest length a model allows when that length is one more than a multiple
     of three and the insertion starts a codon; its generator makes them. ClonoLoom counts them: of
     the seven models, only mouse TRA's probabilities change by more than 10⁻¹⁵ (insertions up to 22
     nucleotides): 4,660 of its 5,583 CDR3s, by up to 13% of a probability. The reference runs
     OLGA on a copy of such a model that allows one nucleotide more with probability 10⁻³⁰, and
     keeps OLGA's values on the model as shipped beside them. Making OLGA's sequence generator from a
     model object changes that object in place, and probabilities computed with it after (by up to
     6 × 10⁻⁷ for TRB): the reference gives the generator its own copy.
4. **Simulated antigen-specific responses (E1).** `clonoloom simulate` presets in which some
   clonotypes recognize an epitope: families of receptors around a few founders, as close as Dash
   et al. found epitope-specific receptors to be; a vaccination series in which the families
   expand in a repertoire; and a specificity database of the founders' epitopes (for slice 7). The
   truth gives each clonotype's epitope, or none. Offered as the vaccine example.
   - Plan: a series' `response` (`internal/simulate/response.go`). Each epitope's founders are
     rearrangements the generator makes, the same in every subject; a subject's family of a founder
     are receptors of its V, D and J genes and deletions whose non-templated nucleotides differ, so
     that their CDR3s differ by a few residues, mostly ones alike (a substitution d TCRdist units
     away chosen in proportion to e^(−κd)); a family's size drawn for each founder, so that some are
     large in every subject and many small. The families are a share of each subject's blood at
     each time point by its condition, beside its own repertoire (and a reference, where a study
     has one), with the series' variance between samples. The preset `vaccine`: three vaccinated
     subjects and two given a placebo, a week before (twice) and 7, 14, 28 and 90 days after; three
     epitopes of a simulated antigen whose families expand from 0.1% of the blood to 3% at day 14
     in the vaccinated; and a database in VDJdb's format (`database/vdjdb.txt`) of each epitope's
     founders and receptors of other donors around them, with two epitopes no subject responds to.
     The truth adds each subject's epitope-specific clonotypes, their epitope and founder.
   - Validation: the families as close as Dash et al.'s epitope-specific β chains (each one's
     nearest of the same epitope, by TCRdist with the V genes' loops: the shares within 12, 24 and
     48 units); each sample's share of templates in the families against the truth's expectation;
     the database's every founder and the subjects' families within reach of it; the same bytes for
     the same seed, and the other presets unchanged.
   - *Done:* as planned, with private receptors (below); `truth/specificity.tsv` and
     `truth/founders.tsv`, the series files' `epitope` column and `response` trajectory; the
     study's folder opened without its `database` folder; the vaccine example with four guided
     steps (its subjects, the test of day −7 against day 14, the classes, the transient clonotypes
     followed); SIMULATION.md and the guide's page.
   - *Validation:* pooled across subjects, 58%, 63% and 68% of the receptors have another of the
     same epitope within 12, 24 and 48 TCRdist units (with the V genes' loops; median 12), against
     Dash et al.'s 54%, 59% and 69% (human, 415 β chains) and 61%, 71% and 85% (mouse, 1,924); the
     response's share of the samples within a quarter of the truth's at every time point (day 14:
     0.102 pooled, the truth 0.107); replicate samples' variance as the series'; each family member
     its founder's genes and deletions, differing only at codons its insertions made, by the
     residues counted; the small response's bytes pinned on arm64 and checked on amd64; standard,
     car-t, transplant and null-series byte for byte as before.
   - *Decided:* members differ from their founder only where its non-templated nucleotides make a
     codon, so that each is a rearrangement of its own (the truth's deletions and insertions make its
     junction), and keep its junction's length (Dash et al.'s nearest neighbors are mostly of one
     length); a substitution d TCRdist units away is chosen in proportion to e^(−0.5d). The epitopes
     are peptides made up for the study, the database's MHC `simulated`, so that nothing reads as a
     real specificity. The database lists gene-level calls, as the simulator makes them.
   - *Found:* families alone are nearer than Dash et al.'s receptors (90% within 12, 99% within 24):
     a third or more of an epitope's receptors have no near neighbor, so each subject has receptors
     of its own besides its families, twice as many as the epitope's founders on average. On the
     example, the beta-binomial test between day −7 and day 14 calls 10 clonotypes, each a receptor
     of the vaccine's epitopes, and none in the placebo group; most of the response is too many small
     clones to call one by one, and TCRdist's neighbor counts over the whole dataset are led by
     common background CDR3s (CASSLGGNTEAFF): what slice 5's backgrounds are for.
5. **Neighborhoods and backgrounds (S2).** Each clonotype's neighbors within a radius in its
   sample and in a background (above); each radius chosen from the background's rate; the
   neighborhood's enrichment (Mayer-Blackwell et al.'s rates, odds ratio and test) and its CDR3
   motif; neighborhoods kept as meta-clonotypes, which Tracking follows (T1).
   - Validation: neighbor counts, weights and radii equal to tcrdist3's on Dash et al.'s TCRs with
     the same background; on the simulated responses, the families found against their truth
     (each family's members found, and the neighbors not of it).
   - Plan: `internal/neighborhood`, on one locus's chains (the β chains of a bulk study, as
     Mayer-Blackwell et al. work).
     - *Backgrounds* (`background.go`), each receptor with a weight: `model`, receptors drawn from a
       recombination model (`internal/pgen`) conditioned on the dataset's V–J usage, each weighted
       by the model's own probability of its V–J pair over the dataset's share of it, so that the
       background's rate is the rate in a repertoire recombination alone makes; `shuffled`, the
       dataset's own chains with their CDR3s moved among its V–J pairs (weight 1). A public repertoire
       the user opens as a background waits for a later slice: the two above need no data of their
       own.
     - *Radii* (`neighborhood.go`): each centroid's is the largest of 0, 2, …, 48 at which the weighted
       background within it is at most a share of the background's receptors (10⁻⁶ by default,
       Mayer-Blackwell et al.'s; tcrdist3's own default is 10⁻⁵), 0 where none is.
     - *Enrichment* (`neighborhood.go`): at that radius, the neighbors in the dataset (the centroid
       counted, as tcrdist3 counts it) and in the background, weighted and not; the rates
       (n + 1)/(N + 2), their ratio, the odds ratio with one added to each count, and a χ² test of
       the two with Yates's correction, as tcrdist3 computes them.
     - *Motifs* (`motif.go`): the neighbors' residues at each position of the centroid's CDR3,
       aligned as TCRdist aligned them (`tcrdist.AlignCDR3`), written as a pattern: a residue, a
       set of up to three, or `.`; a gap makes it optional.
     - *The view*: the Specificity view's third part, Neighborhoods: the background chosen and
       made (a job, kept as a result), the centroids by enrichment with their radius, neighbors,
       rate against the background and motif, and a chosen one's neighbors.
   - Validation: the radii and enrichment equal to tcrdist3 0.3's `bkgd_cntl_nn2` on Dash et al.'s
     TCRs against a background of OLGA draws, run in `validation/reference/`; on the simulated
     vaccine study, each family's members found within its founder's radius and the enrichment of
     the epitopes' centroids above the rest; a budget (10,000 centroids against 100,000 background
     receptors in under 2 minutes on 10 cores).
   - *Done:* `internal/neighborhood` (the two backgrounds, each centroid's radius and
     neighborhood found in parallel, the enrichment, the motif from the alignment TCRdist chose,
     `tcrdist.AlignCDR3` added for it); `pgen.Generator.DrawWith` and `pgen.Model.Usage`, to draw a
     background of named genes and weigh it; the analysis `neighborhoods` (`api_neighborhoods.go`)
     and `POST /api/neighborhoods/loci`; the Specificity view's Neighborhoods part with the chosen
     neighborhood's receptors; CSV export, the methods paragraph's sentence,
     `find_neighborhoods`, the guide's page and capture scene.
   - *Validation:* all 415 centroids' radii, hits, weighted hits, rates, relative rate, odds ratio
     and χ² equal to tcrdist3 0.3's on Dash et al.'s human β chains against 5,204 OTS β chains with
     weights (the background's own construction is ClonoLoom's, so the reference fixes the
     background and compares what is computed from it); on the vaccine study's S01 at day 14, the
     20 most enriched neighborhoods and all 149 receptors in them recognize an epitope, and on S04
     (placebo) none of the 20 does; 10,000 receptors against 100,000 in 1.3 s on 10 processors.
   - *Decided:* neighborhoods are of one locus's chains, as Mayer-Blackwell et al. work and as a
     background is drawn; the locus is the dataset's commonest unless chosen. A result keeps the
     500 most enriched centroids and each one's first 100 receptors, so that a large dataset's
     document stays small. The motif uses the alignment the distance chose, so that a neighbor of
     another length lines up as TCRdist lined it up, and covers the positions the distance compares
     (the first three and last two residues left out), as tcrdist3's regex does. A public
     repertoire as a background waits for a later wave.
   - *Found:* tcrdist3 falls back to a radius of 0 where no threshold holds the control rate, and
     then still counts the background at distance 0 — two of Dash et al.'s 415 centroids differ
     without it. With a background ten times the dataset and a radius chosen to leave almost none
     of it within, a relative rate above 10 and "no background within the radius" are both true of
     nearly every centroid: the summary reports the largest neighborhood instead, and the table
     orders by χ², which accounts for both counts. On the vaccine study the neighborhoods are exact
     where slice 4's plain neighbor counts were swamped by common background CDR3s.
6. **Clonotype networks (S4).** Clonotypes joined by identical CDR3s, by edit distance
   (Levenshtein or Hamming) or by TCRdist within a threshold, on one chain or both; the clusters
   (connected components), with their sizes and samples; a layout computed by ClonoLoom, the same
   on every computer; the Specificity view's Network part, linked to the clonotype table; exports.
   - Validation: clusters equal to scirpy's (`define_clonotype_clusters`) on 10x and OTS data under
     each metric scirpy offers; the simulated families found as clusters.
   - Plan: `internal/network`. The nodes are the dataset's clonotypes under its definition; two are
     joined when their junctions are within a cutoff by a metric: identity, Hamming or Levenshtein
     (0 to 2, scirpy's cutoffs) or TCRdist (the CDR3s alone, or with the V genes' loops where they
     are downloaded). A pair definition joins clonotypes on `all` their chains (a missing chain
     matching only a missing one, as scirpy's `receptor_arms="all"` does) or on `any` chain both
     have; the same V (or J) gene may be required too. The pairs within the cutoff are found per
     chain among the unique junctions, without comparing every pair: a hash for identity, segments
     that must agree for Hamming (pigeonhole), deletion neighborhoods for Levenshtein, and the
     length-and-loops lower bound for TCRdist; then the clonotypes that hold them are joined.
     - *Clusters*: the connected components (union–find), each with its clonotypes, abundance,
       samples and subjects, the largest first.
     - *Layout*: each cluster drawn by a force-directed layout from a fixed start, its float
       arithmetic rounded as N7 requires, so that the picture is the same on every computer; the
       clusters packed largest first, up to a number of nodes the page can draw.
     - *The view*: the Specificity view's fourth part, Networks: the metric and cutoff, the drawing
       (nodes sized by abundance), the clusters with their sizes and samples, a chosen cluster's
       clonotypes linked to the Clonotypes view; exports of each clonotype's cluster and the
       edges.
   - Validation (as planned): every partition equal to scirpy 0.25.1's on the 10x runs and the OTS
     units, under identity, Hamming and Levenshtein at each cutoff, and TCRdist on the CDR3s (scirpy's
     calculator with its gap searched, as tcrdist3 places it), `receptor_arms` all and any, with and
     without the same V gene; on the simulated vaccine study, each family's members in one cluster;
     the layout's bytes pinned on arm64 and amd64; a budget (the networks of 100,000 clonotypes in
     under 2 minutes on 10 cores).
   - Completed: the analysis, full graph and cluster exports, deterministic bounded drawing,
     Networks view and remote action. All 420 scirpy partitions agree with matched settings;
     layout digest matches arm64 and amd64; the 100,000-node sparse synthetic budgets pass.
     Simulation validation distinguishes thresholds: TCRdist 48 connects all 34 observed
     families in S01 day 14; two edits connect 29, with five split (the simulator permits more
     than two substitutions). See research.md §5.10 for exact semantics and limits.
7. **Specificity databases (S5, T1) — done.** VDJdb downloaded on request and read (`vdjdb.txt`, by its
   header's names); clonotypes matched exactly (the CDR3, with V and J or without) or within a
   TCRdist radius, with the epitope, antigen, species, MHC, score and reference; each sample's
   share matched to each epitope; Tracking follows the clonotypes matched to an epitope.
   - Validation: the matches equal to a direct lookup (pandas) in a VDJdb release for the
     maintainer, and in the simulated database in CI; on the simulated vaccine response, each
     sample's share matched to each epitope against the truth.
   - Completed: reference download with terms, checksum-pinned evidence, exact and TCRdist
     chain matching, deduplicated epitope/antigen shares, Tracking selections with provenance,
     complete CSV exports and NOTICE, Reports methods, remote action and guide.
   - Checked: all 424 simulated and 4,000 real-release pandas row-identity lookups agree;
     all 150 simulated sample/epitope shares agree. Exact V/J matches contain 602 molecules,
     598 supported by truth, compared with 3,498 total true response molecules. Database
     coverage is incomplete and chance matches remain visible; see research.md §5.11.
8. **Invariant populations, the examples' steps and release 0.4.0 (S7, E3).** MAIT, iNKT and GEM
   T cells flagged by their published rules (their TRAV and TRAJ genes, with the CDR3α's length
   or the TRBV gene where a rule names them), in the clonotype table, and counted in each sample;
   the epitope and vaccine examples' guided steps; release 0.4.0 with its project fixture, its
   pictures and its pages.
   - Validation: each rule against the published receptors it comes from; the share of MAIT
     cells in 10x Genomics' PBMC data against the published range.
   - Implemented: human/mouse MAIT and iNKT, human GEM motif; explicit saved species,
     retained-chain flags with missing-evidence states, population filtering, complete sample
     totals/evaluable abundance/shares, exports, cited methods and remote control.
   - Checked: independent CSV cell-level rules agree with all counts; MAIT shares 10/406
     (2.46%) and 80/4,562 (1.75%) in PBMC 1k and 10k v2. Published consensus receptors and
     constructed gene/length boundaries, unsupported definitions and ambiguous calls tested.
   - Guided mouse/human epitope and vaccine steps run through the browser in both themes.
     Release 0.4.0's project fixture, version files, pictures and pages are prepared.
     Go race checks, amd64 compatibility, six platform builds, 80 JavaScript tests,
     14 accessibility checks and 104 site audit checks pass.
     Completed: merged, CI passed, 0.4.0 released and its product site published by the maintainer.

## 0.5.0: B cells (wave 5)

A researcher opens annotated B-cell repertoires, checks which sequences can support the
analysis, and follows clonal families, their mutations and isotypes through a study. Junction
clonotypes remain the existing, explicit definitions; a B-cell clonal family is a separate,
threshold-dependent result, not a replacement for those identifiers.

Implemented on `wave5`. As in SituCraft's reader and reference slices, preserve and check the input
before building on it, keep the source's identity with derived results, and test against
independent values. ClonoLoom continues to compute every reported number in Go. The order below
puts a simulation with known families before the analyses it validates. All nine slices are
committed and merged; 0.5.0 is released and its product site is published.

| Slice | Status | Scope |
| --- | --- | --- |
| 1. B-cell sequence evidence | Committed | Retain AIRR sequences, alignments and coordinates; inspect their availability and validity |
| 2. IMGT/HighV-QUEST input | Committed | Join the documented output tables by sequence identity, preserving aligned sequences and annotation provenance (D5) |
| 3. Simulated B-cell families | Committed | An immunization study with known ancestors, families, mutations, isotypes and sampled abundance (E1) |
| 4. Clonal families | Committed | Distance-to-nearest distributions, explicit thresholds, heavy/light pairing and subject boundaries (C4, B2) |
| 5. Germlines | Committed | Checksum-pinned reference sets and per-family reconstruction with D masking (B1) |
| 6. Somatic hypermutation | Committed | Evaluable bases and codons, mutation by region, replacement and silent counts (B3) |
| 7. Lineage trees | Committed | Maximum parsimony, ancestral sequences and a viewer with stated uncertainty and limits (B4) |
| 8. Isotypes and class switching | Committed | Usage and family-level isotype observations; distinguish observations from inferred switching (B5) |
| 9. Examples and release 0.5.0 | Committed and released | Complete guided immunization workflow, compatibility fixture, captures and release validation (E3, O2, N7) |

### Slice 1: B-cell sequence evidence

**Serves D5, D8, D11 and O3; prerequisite for B1 and B3.** These B-cell analyses remain planned
after this slice. The deliverable is useful on its own: open an AIRR table, see which IGH, IGK
and IGL rearrangements have sequence and alignment evidence, inspect a row and export the
retained evidence. An absent or inconsistent alignment is a reason an analysis cannot use that
row, not a reason to discard a valid junction from the existing repertoire analyses.

**Why this comes first.** `internal/airr/schema.go` already types the alignment fields, but
`internal/reads/airr.go` and `internal/rep` do not retain them. `internal/export` currently writes
empty sequence, alignment and CIGAR fields. The B cells view is still a placeholder. Building
mutation counts on this table would either lose the evidence or silently assume an alignment.

1. **Retain a bounded set of AIRR evidence fields.** Extend `rep.Row`, `rep.Table` and its builder
   with optional columns for `sequence`, `rev_comp`, `sequence_alignment`, `germline_alignment`,
   `v_cigar`, `d_cigar`, `j_cigar`, the V/D/J sequence, germline and alignment start/end fields,
   FWR1–4/CDR1–3 start/end fields, `np1_length`, `np2_length`, and `germline_database`.
   Keep strings as supplied, including gaps and ambiguity symbols; interpretation uses a
   separate checked representation. Store long sequences outside the gene-call dictionary.
   Columns absent from the input allocate nothing; mixed files backfill missing values.
   Existing non-AIRR readers continue to leave this evidence missing. Mapping OAS-specific
   fields, legacy Change-O headers and raw IgBLAST output is outside this slice.
2. **Keep coordinate systems distinct.** AIRR positions are 1-based closed intervals. Sequence,
   germline and alignment coordinates name different strings; FWR/CDR coordinates refer to the
   query sequence. Keep their names and values in the table and exports. Checked internal
   slices may use zero-based half-open intervals, with the conversion in one tested place.
   Missing, invalid and explicit zero lengths remain distinguishable. `rev_comp` is preserved;
   do not reverse an already oriented alignment a second time. No positional mapping is inferred
   by padding strings, assuming a full-length V gene, or treating the junction as a full VDJ.
3. **Validate what is actually present.** Add stable findings with source, line, field, example
   and remedy for invalid coordinates, reversed or out-of-bounds intervals, malformed CIGARs
   and incompatible paired alignment lengths. Do not bounds-check germline coordinates against
   the query or pretend the unprovided reference length is known. Parse CIGAR operations with
   overflow and allocation bounds; apply AIRR's operation semantics rather than equating a
   CIGAR span with a gapped string's length. Preserve the supplied evidence when invalid, but
   refuse to use it as a checked alignment. A missing optional field is availability information,
   not a warning for every row. Equal lengths alone do not establish IMGT numbering, valid
   region mapping or a trustworthy germline. Cell Ranger's existing unequal alignment examples
   must remain readable and show that limitation explicitly.
4. **Cache and export without changing old results.** Extend the cache's column/store inventory,
   decoding bounds and memory accounting. Increment `reads.ReaderVersion` and the cache layout
   version when the retained layout changes; rebuild disposable caches from the checked sources.
   Do not rewrite old artifacts or change the project archive version for an optional table
   extension. Populate the AIRR export's existing evidence columns and append retained optional
   coordinates deterministically, preserving missing values. Existing clone IDs, counts, filters
   and single-cell chain selection stay the same. Correct the format guide's present claim that
   exports re-read every other source column: arbitrary source-column round trips are not part
   of this slice.
5. **Make the evidence inspectable.** Replace the B cells placeholder with an Inputs part
   (`api_bcells.go`, `web/ui/mode-bcells.js`, backed by a small `internal/bcell` package).
   Show the active filter and definition, IGH/IGK/IGL row counts by sample, and separate counts
   for sequence present, both alignments present, structurally valid paired alignments and
   supplied region coordinates. Count rearrangements here, not cells or clonal families; any
   abundance shown names its unit and denominator. These are evidence summaries, not a blanket
   "ready for mutation" verdict. A paginated table opens the source sequence, alignments,
   coordinates and findings in the inspector, linked by source identity and sequence ID even
   when IDs repeat across files. Paginate from Go; do not send all sequences to the browser.
   Reuse the existing dataset/filter and inspector patterns, with a remote action
   `inspect_bcell_inputs` for the same summary and a bounded page. No new background job or
   scientific artifact is needed for this deterministic inspection of an already opened table.
6. **Example and documentation.** Supply a small labeled synthetic AIRR example with complete,
   partial, ambiguous and invalid evidence, made by a committed script. Its guided steps open
   B cells and inspect the reasons rows differ. Use the existing openly licensed B-cell AIRR
   development files as real-format checks. Add the Inputs guide page and both-theme capture
   scene, update `docs/FORMATS.md`, the README, remote-action documentation and requirements,
   and add an unreleased changelog entry when implemented. Keep the released version at 0.4.0
   until the wave's release-preparation slice.

**Validation and exit criteria.**

- Independent `airr`/CSV reads agree on every retained field and missing value in the selected
  development files; commit the comparison script, tool version and reduced reference output.
  Use `uv` for Python reference scripts. Synthetic coordinates have hand-calculated boundaries,
  including reverse orientation, leading offsets, gaps, partial segments, ambiguity symbols,
  missing endpoints, zero N/P lengths and hostile large integers/CIGAR operations.
- Cache round trips retain every new column; old cache keys are not reused; corrupt optional
  columns fail safely. AIRR export/re-import preserves the declared evidence fields. Source
  checksums remain unchanged. Fuzz the new parsers and coordinate conversions.
- Existing reader, QC, clonotype and export tests pass. Every released project fixture opens;
  its existing artifact documents reproduce byte for byte on arm64 and amd64. A changed reader
  version alone must not invalidate a saved scientific result whose inputs and method agree.
- API and browser checks cover mixed T/B data, no B cells, missing evidence, filters, repeated
  sequence IDs, paging, selection changes and remote inspection. The example's guided steps
  work in both themes; accessibility checks and the site build pass.
- Re-run the existing million-row read and page budgets and measure the added sequence memory
  on a fixed synthetic AIRR dataset. Confirm no sequence columns are allocated for junction-only
  inputs. Large reference regeneration stays outside per-commit CI; small reference checks run
  in CI. Go race tests and `go vet`, JavaScript tests and six-platform builds remain required.

**Implemented.** Optional raw evidence columns, bounded CIGAR and coordinate checks, stable
findings, cache and AIRR round trips, paginated Inputs and its inspector, remote action and
seven-row artificial example with guided steps. The new guide, captures, format notes and cited
methods describe the distinction between structural evidence and biological inference.

**Checked.** All 42 fields agree with independent CSV reads on 7 artificial and 7,733 public
B-cell rows (four 10x datasets). Cache corruption and old versions are refused; absent evidence
allocates no columns. API tests cover repeated IDs, mixed T/B data, missing evidence, filters,
paging and invalid requests. Both-theme browser checks cover the example, filtered/empty results
and stale inspector responses. Release fixtures and evidence checks pass on arm64 and amd64;
Go race validation (including the full scientific package and a root-package rerun after the
example catalog grew), `go vet`, 81 JavaScript tests, six platform builds, 14 accessibility
checks and 108 site audit checks pass. The million-row page
budget is 41 ms and cached cohort reopening 185 ms on this machine. The fixed 100,000-row
sequence-memory case adds 30.9 MiB. CIGAR and interval fuzz runs pass.

**Decided during implementation.** Keep raw numeric and orientation text as well as sequence
strings, so that missing, zero and invalid annotations can be inspected and exported distinctly.
Reader/cache version 2 rebuilds cached tables, while a separate identity version remains 1:
additional annotations do not change existing junction analyses or their dataset/artifact IDs.
The first Inputs part does not create an artifact; it inspects the current source and filter.
Rows count under the QC filter rather than a clonotype definition's chain selection. Region
presence and structural validity remain separate. IMGT input and all biological inference stay
in later slices. The version remains 0.4.0 until release preparation.

**Sources and boundary.** The field and coordinate contract is the
[AIRR Rearrangement schema 2.0](https://docs.airr-community.org/en/v2.0.0/datarep/rearrangements.html),
with the older supported schemas still accepted (Vander Heiden et al. 2018, *Front Immunol*
9:2206, doi:10.3389/fimmu.2018.02206). Before implementing the coordinate checks, pin the schema
version and document each supported field's string and orientation convention in `research.md`.
Change-O's public [germline documentation](https://changeo.readthedocs.io/en/stable/examples/germlines.html)
motivates retaining the evidence needed by later reconstruction, but slice 1 reconstructs no
germline and computes no mutation rate. No new germline download, inferred allele, family
threshold, selection statistic or lineage algorithm belongs here. Future reference comparisons
use tools as external programs; their source is neither inspected nor copied into ClonoLoom.

### Slice 2: IMGT/HighV-QUEST input

**Serves D5, D8, D11 and O3.** Read annotated output without running annotation, reconstructing
germlines or importing vendor mutation statistics as computed results. Use the documented table
contract (research.md §5.14), retaining sequence evidence and the provenance needed by later slices.

1. **Open one run as one sample.** Accept an extracted folder or explicitly selected tables
   1, 2, 3 and 6, plus optional Parameters (11). Support gzip and prefixed names. Recognize output
   folders in the picker. Check headers, exact row width and identities; join by sequence number
   and sequence ID regardless of row order. Reject incomplete bundles, duplicate numbers,
   mismatched identities and conflicting gene calls. Keep failed annotations as rows.
2. **Map only justified fields.** Preserve ties, productive/unknown states, un-restored junctions,
   the already oriented Summary sequence and table 2's gapped query. Map query coordinates only
   when their uncorrected region matches the Summary substring. Retain original positions,
   orientation, comments, corrected translations and contributing table lines in a native
   annotation extension. Leave germline alignment, CIGAR and biological inference absent.
3. **Preserve the whole input's identity.** Fingerprint every table and optional Parameters;
   record sample grouping in source records, saved projects and cache keys. A companion change
   must invalidate both the cache and dataset identity. Preserve old dataset/artifact IDs.
   Use disk-backed identity indexes for reordered tables; remove temporary spools on all exits.
4. **Make it inspectable.** Reuse B cells → Inputs, AIRR export and remote inspection. Show native
   annotations in an expandable section and Parameters in Data. Add a locally generated,
   explicitly artificial example with guided steps, format notes, cited methods and captures.

**Acceptance checks.** Independent stdlib CSV joins and hand-specified coordinate expectations;
shuffled rows, repeated IDs, heavy/light chains, ties, missing/failed annotations, reverse strand,
indel correction and malformed input. Exercise gzip, cancellation, sources unchanged, cache and
AIRR round trips, companion fingerprint changes, content-addressed relocation and project reopen.
Fuzz the bounded index reader. Check the example in both themes, release compatibility on arm64
and amd64, Go race tests and vet, JavaScript/accessibility checks, six builds and the site audit.
Keep version 0.4.0. Extracted tabular output is the supported scope; archive/workbook import,
reference reconstruction, N/P reconstruction and vendor mutation-statistics import are deferred.

**Implemented.** The four-table identity join, optional Parameters, conservative coordinate
mapping, native annotation evidence, cache/project integration, inspector, offline example,
methods and documentation. No later biological analysis has been started.

**Checked.** Independent CSV values agree for all four artificial fixture rows, including
shuffled joins, repeated IDs, calls, sequences, mapped coordinates and missing evidence. AIRR
round trips preserve the native annotation extension. Cache checks include changes in both gapped
sequence and Parameters files. Gzip, hash-sharded library storage, legacy Functionality headers,
prefixed cohorts, invalid grouping/mapping combinations and source-preserving relocation pass.
The bounded reader fuzz run completed 498,261 executions without failure. Guided browser steps
and project reopening pass in both themes. Existing release fixtures reproduce on arm64 and
amd64. The full Go race suite and final changed-package checks, vet, six platform builds,
82 JavaScript tests, 14 accessibility checks and 108 site audit checks pass.

The isolated million-row budget reads and validates in 1.488 s, reopens the cached cohort in
135 ms and answers the searched page in 39.847 ms. An earlier page timing exceeded the budget
while scientific validation and builds competed for resources; the isolated rerun passed without
a code change. No real IMGT-annotated cohort was used: the new reader's evidence is the published
format contract and independently checked artificial fixtures. Version remains 0.4.0.


### Slice 3: Simulated B-cell families

**Serves E1 and the first immunization steps of E3; prerequisite truth for C4 and B1–B5.**
Deliver a deterministic offline study with paired heavy/light observations and separate known
ancestors, family membership, ordered mutations, isotype history and sampled abundance. No
family, germline, mutation-rate or lineage inference is introduced in this slice.

**Scope and decisions.** Add `bcell-immunization` to `clonoloom simulate` and Example data. The
default is three artificial subjects, twelve families each, days −7, 14 and 28, 240 paired cells
per sample and three accepted substitutions per chain per edge. Model
`clonoloom-artificial-bcell-v1` uses artificial coding strings with fictional SIM calls and
explicit contiguous region coordinates, without importing a reference allele set. Its fixed
seven-state tree, forward M/G1/A1 transitions and family/node abundance weights are ClonoLoom's
choices, fully described in `docs/SIMULATION.md`. They make a small inspectable benchmark, not a
calibrated germinal-center or affinity model. A deliberately reversed event distinguishes path
mutation counts from net ancestor differences. States with identical sequences remain separate.

1. **Observations and units.** Write AIRR rows for one heavy and one kappa/lambda chain per cell,
   with coding sequence, known ancestor alignment, artificial query regions, source IDs and
   isotype calls. Reads/UMIs and D/CIGAR/reference-coordinate evidence remain absent. Count cells
   once under the paired definition. Preserve ordinary junction clonotype IDs and behavior.
2. **Known truth.** Keep all subject-specific states, parents, sequences and ordered events in
   `truth/truth.json`; link observed cells to states in `truth/observations.tsv`. Record every
   sample's expected node probabilities and realized cell counts, including unobserved states.
   Serialize effective parameters, seed, model/version and file checksums. Keep truth out of
   imported data. Atomic completion, cancellation cleanup and nonempty-output refusal follow
   the existing local-output conventions.
3. **User path.** Offer the study offline with Subjects and Inputs steps. Methods text identifies
   the B-cell model from metadata even if the study is renamed; it must not describe TRB genes,
   template/read sampling or extra biological variance. Add a guide capture in both themes and
   document reproduction, artificial-region semantics and the scientific limits.
4. **Independent validation.** Python stdlib CSV and genetic-code checks replay all events,
   verify every observation and coordinate, and independently calculate sampling probabilities,
   cell/isotype/paired-clonotype counts and net ancestor differences. Commit reduced expectations
   and sample hashes. Go checks them through the real readers and paired definitions. Pin output
   bytes on arm64 and amd64, retain old simulator digests, and test sampling, parameters, seeds,
   cancellation and no-overwrite behavior. Run the race suite, vet, release compatibility, six
   builds, JavaScript/accessibility checks, guided browser steps and site audit.

**Sources and boundaries.** See research.md §5.15. Yaari et al. 2013
(doi:10.3389/fimmu.2013.00358) explains the context dependence absent from this simple mutation
model; Horns et al. 2016 (doi:10.7554/eLife.16578) describes biological class-switch order.
Neither publication's model is reproduced. The ancestor alignment includes its artificial
junction and is not a reconstructed/D-masked germline. Real reference sets, indels, selection,
error/dropout, alternative trees and biological calibration stay outside this slice. Later
analyses require additional hand cases, independent implementations and real-data checks.

**Implemented.** The bounded deterministic model, paired AIRR/Repertoire output, complete state
and event truth, sample probabilities and cell assignment, SHA-256 manifest, atomic output,
offline example, guided steps, model-specific methods and documentation. Version stays 0.4.0.

**Checked.** Independent replay agrees on 252 states, 1,296 events (998 replacement, 298 silent,
including 72 deliberate reversals), and all 2,160 paired cells / 4,320 rows. Source/ancestor
alignments, regions, metadata, paired clonotypes, isotypes and net differences agree through the
real readers. All 180,000 categorical draws remain within 2.782 standard deviations of the
documented probabilities. Parameter round trips, bounds, cancellation cleanup and overwrite
refusal pass. The small study's pinned bytes agree on arm64 and amd64; old TRB digests and every
released project fixture still pass.

The full Go race suite passes (scientific validation 493.5 s), as do the final root-package rerun,
vet, six platform builds, 84 JavaScript tests, 14 accessibility checks, guided steps and methods
inspection in both themes, and the 108-check site audit with no problems. Mixed B-cell/TRB data
keep separate methods descriptions. The site builds 28 pages with 26 screenshot sets. Isolated
million-row budgets: read/validate 1.518 s, cached cohort reopen 123 ms, searched page 37.058 ms.
No later slice or release preparation has been started. Version remains 0.4.0.

### Slice 4: Clonal families

**Serves C4 and B2.** Keep these inferred families separate from junction clonotypes. The
settled method is length-normalized nucleotide Hamming distance and single-linkage connected
components at an explicit inclusive threshold, following Gupta et al. (2015, 2017), with the
conservative eligibility and paired policies documented in research §5.16.

1. Compare productive ACGT nucleotide junctions with resolved V/J genes (same-gene allele ties
   are acceptable), within equal V/J/length partitions and explicit subject boundaries. Do not
   infer subject from a sample name. Missing subjects are excluded; sample scope is an explicit
   alternative. QC applies first. A null threshold measures distances only; zero is exact identity.
2. Auto selects paired mode for cell-indexed tables. Require exactly one distinct heavy and
   one distinct kappa/lambda chain per sample-scoped cell. Collapse duplicate identical chains,
   exclude ambiguous or missing evidence, and use the maximum normalized chain distance within
   both chains' partitions. Heavy mode remains available. Bulk rows retain their count unit;
   cells count once. Unknown abundance stays missing.
3. Collapse identical compared receptors only within their scope for nearest distances. Each
   contributes once, without abundance weighting. No comparable receptor is null. Use 20 bins
   of width 0.05, with 1 included in the final bin. State transitivity and threshold sensitivity.
4. Keep cancellable, resumable jobs and immutable artifacts with normalized options, subjects,
   QC, source-row identities, membership, exclusions, nearest distances and per-sample abundance.
   Retain existing clonotype IDs. Refuse more than 250,000 observations or 200 million compared
   bases explicitly, without sampling. The UI reads pages of at most 200 rows (normally 50).
5. Add B cells → Families, saved choices, source inspection, old-result reopening, figure and
   CSV exports, methods citations, remote control and two guided steps. Export all rows and
   source hashes, not only the current page. Keep membership abundance at observation level;
   chain-row repeats must not be summed. Family IDs and artifact IDs record different scopes:
   family IDs hash members/options; artifacts additionally identify files, filters and annotations.
6. Validate boundary cases, subjects, missing evidence, duplicate chains, multi-chain cells,
   transitive links, explicit zero, cancellation and work bounds. Compare every observed-cell
   membership and nearest-distance bit pattern against an independent CSV/adjacency-graph
   calculation for eight cases. At 0.1, both heavy and paired modes exactly recover all 36 known
   artificial families. This validates artificial truth, not accuracy on biological repertoires.

**Implemented and validated.** The complete Go race suite passes (scientific validation
669.3 s), followed by the final API/core/export race rerun. Vet, six platform builds,
arm64/amd64 family and released-project compatibility checks, and five seconds of distance
fuzzing pass. All 87 JavaScript tests and 14 accessibility checks pass. Browser checks cover the
visible threshold controls, metadata subjects, paginated members/nearest receptors, source
inspection, methods and reopening an older result. All four affected B-cell guided workflows
pass in both themes. The site builds 29 pages with 27 screenshot sets; its 112-check audit finds
no problems. Isolated million-row budgets pass: read/validate 1.558 s, cached reopen 161 ms,
searched page 40.444 ms. The reference checks every membership and nearest value at eight
mode/threshold combinations. Explicit empty subject maps override metadata; unrelated clonotype
and diversity choices do not change family identity.
No germline, SHM, tree or class-switch implementation is included. Version remains 0.4.0.

### Slice 5: Germline reconstruction

**Serves B1.** Reconstruct from a saved, thresholded family result and a checksum-pinned
ungapped V/J reference. Retain the family result's checksum, original row identities,
reference bytes' checksums, versions, license and citation in the immutable result.

- Resolve one V and J allele per family/locus by intersecting explicit member calls.
  Conflicts, unresolved ties and absent alleles remain exclusions. Do not infer a genotype,
  choose the first tie, borrow another subject's evidence or substitute a reference allele.
- Project selected V/J segments using bounded AIRR CIGARs and oriented raw query sequence.
  Check supplied query/reference endpoints against CIGAR offsets. Preserve insertions and
  deletions as alignment gaps and retain per-column query/reference positions. Mask the
  entire intervening D/N/P region with N, including the V–J junction in light chains.
  Rows may cover different reference spans; keep their individual projections under the
  common family allele selection rather than inventing one padded family alignment.
- First biological catalog: human OGRDB IGH v10, IGK v5 and IGL v4 (27 May 2026), CC0,
  downloaded only on request. No raw-sequence annotation, gapped IMGT projection, gene-only
  allele inference, custom reference upload, SHM statistics or lineage reconstruction.
  Missing CIGARs are an explicit unsupported-evidence state, including native IMGT input.
- Version the artificial study as v2: export its independent V/J backbones and CIGARs;
  correct the shared J name for distinct backbone sequences. Do not change its simulated
  sequences or mutation histories. Existing v1 datasets/results remain immutable and openable.
- Add a Germlines view, bounded pages/row inspection, complete JSON/CSV/FASTA exports,
  methods and reference provenance, remote action and guided artificial workflow.

**Acceptance.** Hand-calculated reverse orientation, offsets, indels, masking, ambiguity,
missing evidence and hostile lengths; independent CSV/reference reconstruction of the
artificial study; pinned public-reference parsing and real AIRR evidence checks; parent and
reference tampering refusal; cache identity and released-project compatibility; browser
workflow in both themes; Go race/vet, JavaScript, accessibility, builds and site checks.
Keep version 0.4.0 until release preparation. No slice 6 mutation statistics in this slice.

**Implemented.** Immutable germline jobs derive from checksum-verified saved families and
on-request human OGRDB or generated artificial references. The Germlines view, bounded
family/row pages, alignment inspector, source identity, complete CSV/FASTA/JSON exports,
parent/reference lineage, cited methods and remote/guided workflow are in place. Native IMGT,
gene-only calls and missing CIGARs remain explicit exclusions. Artificial model v2 supplies
reference segments and unambiguous J names while preserving sequences and mutation histories.

**Checked.** Independent Python 3.13.3 CSV/FASTA reconstruction agrees with all 4,320 projected
alignments and coordinate maps in 72 family/locus groups. All 252 mutation histories and all
observed sequences/ancestors are unchanged from v1; existing independent family memberships
and nearest distances remain unchanged. All 421 pinned OGRDB records parse. The public Ramos
heavy row reconstructs; 56/437 assigned heavy rows in 10x PBMC 1k/10k-v2 are explicitly excluded
for gene-only calls, a coverage check rather than biological truth. Orientation, clipping,
indels, ambiguity, invalid coordinates, missing evidence, byte bounds and checksum changes
pass focused tests; projection fuzzing passes. Full Go race coverage, with affected-package
reruns after fixes, and vet pass. arm64/amd64 germline, simulation and released-project checks,
six platform builds, 89 JavaScript tests, 14 contrast checks, five browser workflows in both
themes and 116 site audit checks pass. The site builds 30 pages with 28 screenshot sets.
Isolated million-row budgets pass: read/validate 2.915 s, cached reopen 231 ms and searched
page 82.289 ms. Version remains 0.4.0. Committed by the maintainer.

### Slice 6: Somatic hypermutation

**Serves B3.** Compare the saved D-masked germline parent with the original source chain rows.
Pin the parent checksum and retain its family/reference provenance. Keep all rows, including
exclusions. Current QC and clonotype settings must not change a derived result's identity.

- Count unequal paired A/C/G/T bases; frequency is substitutions / evaluable paired bases,
  missing at denominator zero. Report masks, gaps and ambiguities separately. These are net
  differences, not historical event counts, mutation rates or sequencing-error corrections.
- Keep whole-alignment, V/J and individual FWR/CDR results distinct. Use supplied oriented-query
  coordinates. Missing regions stay missing; invalid/overlapping/out-of-order intervals suppress
  the regional partition without discarding nucleotide totals. Uncovered positions stay in
  totals; deletion gaps have no query-region assignment.
- Check query codon phase from explicit in-frame V/J annotation and CDR3/junction agreement,
  including conserved flanking codons. Never assume reference base 1 starts a codon. Suppress
  codon inference for any indel-containing projection; omit partial, ambiguous, masked,
  cross-segment and stop-containing codons. Regional codons must fit in the region.
- Classify single-hit substitutions by NCBI table 1. For multi-hit codons report endpoint
  replacement/silent codon counts but leave individual substitutions unclassified. Record
  evaluable codons and stops separately. Do not infer mutation order or selection statistics.
- Provide B cells → SHM, bounded pages, per-row region inspection, descriptive sample/locus
  pooled counts, complete CSV/JSON exports, methods, remote control and a guided example.
  Pool source rows without abundance weighting or deduplication; they are not subject replicates.

**Acceptance.** Analytic coordinate/frame/ambiguity/indel/stop cases; every artificial row and
region compared independently with known ancestors, including reversals and multi-hit codons;
source and parent identity, missing versus zero, immutable options and complete exports;
released-project and processor compatibility; Go race/vet, JavaScript, fuzz, browser workflows
in both themes, accessibility/site checks, six platform builds and isolated performance budgets.
Version remains 0.4.0. Trees and class switching remain in slices 7 and 8.

**Implemented.** The core comparison, saved jobs and lineage, SHM view and regional inspector,
source-row/sample summaries, complete exports, methods and remote/guided workflow are present.
Detailed scientific choices and limitations are recorded in research §5.18 and FORMATS.

**Checked.** Independent Python 3.13.3 ancestor comparisons agree for all 4,320
source rows and 43,200 region entries: 9,941 substitutions / 1,412,640 evaluable bases; 7,297
single-hit replacement, 2,360 silent, and 284 unclassified substitutions in 142 multi-hit codons.
Endpoint counts are 7,439 replacement and 2,360 silent codons, with 470,880 evaluable codons.
Source-file hashes and pooled summary denominators are checked. Analytic evidence/coordinate,
indel, ambiguity, stop, partial-codon and attribution tests pass, along with source/parent
identity, exports, cancellation and fuzzing (26,614 executions). The public Ramos row supports
base/codon comparisons; 56/437 gene-only 10x heavy rows retain explicit germline exclusions.
Full Go race coverage passes with loopback-enabled reruns of the two sandbox-blocked packages
and focused reruns after test refinements; the scientific validation package completes in
867.877 s. Vet, arm64/amd64 SHM and released-project compatibility, six platform builds,
90 JavaScript tests and 14 contrast checks pass. Six B-cell browser workflows pass in both
themes, including the guided SHM action, visible run control, paging, inspection, cited methods,
view disposal and reopening. The site builds 31 pages with 29 screenshot sets; its 120-check
audit across 30 pages finds no problems. Isolated million-row budgets pass: read/validate
1.563 s, cached reopen 127 ms and searched page 41.742 ms. Version remains 0.4.0.
Committed by the maintainer.

### Slice 7: Lineage trees

**Serves B4.** Build separate family/locus parsimony trees from checksum-pinned germlines.
Scientific choices and citations are detailed in research §5.19. Version remains 0.4.0.

- Intersect unambiguous, covered V/J reference positions across eligible rows; omit the
  D/N/P mask, indels and ambiguity. Retain the panel map and all excluded source rows.
- Collapse identical panel states without abundance weighting, retaining every source row
  and observed C call, sample, timepoint and tissue. Fix the reference as the root state.
- Enumerate every binary topology through eight taxa including the reference. Search 9–64
  taxa by deterministic addition/NNI with explicit candidate/work bounds; mark heuristic
  results and budget termination. Refuse larger groups rather than subsampling.
- Choose a reproducible minimum topology and consistent ancestral assignment. Show conditional
  IUPAC state sets and exact optimal-topology/split counts; never present them as probabilities.
  Retain zero edges and acknowledge non-identifiable binary resolutions.
- Add B cells → Lineages, an interactive/keyboard tree, observed annotation choices, node and
  source inspection, accessible tables, complete CSV/JSON/FASTA/Newick exports, figure exports,
  cited methods, remote control and the guided artificial study. Pin metadata in the result.
- Do not infer class switching, migration, likelihood trees, selection or event dates.
  Class-switching work remains slice 8; release preparation remains slice 9.

**Acceptance.** Exact topology counts, brute-force ancestral ambiguity, deterministic heuristic
and cancellation, partial/ambiguous panels, duplicate states and metadata mixtures; independent
exhaustive calculation of all 72 artificial family/locus trees; immutable source/parent/metadata
identity and complete exports; browser workflow in both themes; Go race/vet, processor and
released-project compatibility, six builds, JavaScript, fuzz, accessibility/site and isolated
performance checks.

**Implemented.** Core parsimony and conditional-state reconstruction, immutable jobs and
provenance, viewer and source inspection, exports, methods and remote/guided workflow are in place.
Independent exact search agrees for all 72 trees and 4,320 rows, including all optimal-tree
counts and 1,093 changes.

**Checked.** Exact topology counts and brute-force ancestral ambiguity, deterministic heuristic
search, cancellation, state/work bounds, partial panels, metadata mixtures and exclusions pass.
Source/parent/annotation identity, historical metadata, pages and all export formats pass.
The public Ramos row builds a tree; all 56/437 gene-only heavy rows in the 10x PBMC fixtures
retain their upstream exclusions. These are evidence-coverage checks, not biological truth.
The full Go race suite passes (scientific validation: 562.741 s), with focused reruns after
final memory and test refinements. Coordinate maps are processed one row at a time rather
than duplicating all saved alignments as maps. Vet, arm64/amd64 tree-serialization and released-project compatibility,
six platform builds, 93 JavaScript tests, 517,526 projection fuzz cases and 14 contrast checks
pass. Seven B-cell browser workflows pass in both themes, including keyboard node inspection,
observed annotations, source paging, methods, disposal and saved reopening. The site builds
32 pages with 30 screenshot sets; 124 audit checks across 31 pages find no problems.
Isolated million-row budgets pass: read/validate 1.503 s, cached reopen 125 ms and searched
page 47.415 ms. Version remains 0.4.0. Committed by the maintainer.

### Slice 8: Isotypes and family co-occurrence

**Serves B5's observational scope.** Summarize constant-region usage and family-level
co-occurrence from a checksum-pinned, thresholded family result. Scientific choices and
citations are in research §5.20. Version remains 0.4.0; release preparation is slice 9.

- Count each assigned sample-scoped cell once in single-cell data, or each assigned heavy
  source row once in bulk data. Retain original heavy and light C calls and source identities;
  only heavy calls define isotype. Inherit membership, QC and subject scope from the parent.
- Collapse allele ties. Resolve class separately from constant gene/subclass. All retained
  heavy calls must agree at that resolution; missing, unsupported and conflicting calls remain
  explicit. Do not choose the first candidate or treat IgM/IgD ambiguity as coexpression.
- Show class and gene usage by sample and family/sample, with all assigned observations in
  denominators, including unresolved calls. Keep parent exclusions outside the denominator.
  Preserve missing abundance, measured zero and cell-versus-row counting; never multiply cell
  abundance by contig count. Pin timepoint/tissue labels without pooling biological replicates.
- Show family coverage and unordered pairs of resolved constant genes, counting each family
  once per pair. Distinguish observations from switching inference: no event counts, direction,
  timing, ancestral states, rates, significance or species-specific genomic ordering.
- Add B cells → Isotypes, bounded pages, pair-to-family and family-to-observation navigation,
  raw-call inspection, complete CSV/JSON exports, cited methods, remote action and guided study.

**Acceptance.** Hand-calculated ambiguity, missing/zero abundance, duplicated contig, IgM/IgD,
source-identity, scope and denominator checks; independent truth-file counts for the default
artificial study; immutable parent/annotation identity and complete exports; browser workflow
in both themes; Go race/vet, processor/released-project compatibility, six builds, JavaScript,
fuzz, contrast/site and isolated performance checks.

**Implemented.** The Go counting, saved jobs, usage/family/pair/observation pages, source inspector,
exports, cited methods and remote/guided workflow are in place. Independent direct tallies agree
for all 2,160 cells, 36 families and 466 usage rows. Each of the three unordered IGHM/IGHG1/IGHA1
pairs occurs in all 36 families. This validates descriptive bookkeeping, not switching inference.

**Checked.** Hand-calculated ambiguity, missing/zero abundance, duplicate contigs, IgM/IgD,
subject/sample boundaries, empty evidence, cancellation and source identity pass. Observation
units are distinct from abundance units, including unspecified bulk units. Malformed allele
suffixes cannot conceal another candidate. API identity, annotations, bounded/filterable pages,
parent tampering and complete exports pass. Public coverage resolves the Ramos observation and
all 56/437 assigned observations in the pinned 10x PBMC fixtures, without requiring V/J alleles.
These are coverage checks, not known biological switching truth.

The application/internal race suite and complete scientific race run pass (scientific validation:
501.008 s), with focused reruns for the final constant-call suffix guard. Vet, arm64/amd64
serialization and released-project compatibility, six platform builds, 94 JavaScript tests,
378,725 final fuzz cases and 14 contrast checks pass. Eight B-cell browser workflows pass in
both themes; the isotype scene checks visible run controls, pair/family navigation, source
paging and inspection, pinned metadata, cited methods, disposal and saved reopening. The site
builds 33 pages with 31 screenshot sets; 128 audit checks across 32 pages find no problems.
Isolated million-row budgets pass: read/validate 1.630 s, cached reopen 135 ms and searched page
38.099 ms. Version remains 0.4.0. Committed by the maintainer.

### Slice 9: Examples and release 0.5.0

**Serves E3, O2 and N7.** Complete the artificial immunization guide through families,
D-masked germlines, SHM, lineage trees, observed isotypes and cited methods. Prepare 0.5.0
without changing the scientific scope: likelihood, selection and switching-event inference
remain deferred.

- Preserve a small release project with AIRR evidence, paired families across subjects,
  reference bytes and provenance, the complete parent/result chain and joined IMGT inputs.
  Reopen its datasets under their original identities and reproduce every document byte for
  byte on arm64 and amd64. Preserve earlier fixtures unchanged.
- Update version, changelog, citation, user guides and release overview. Explain how to
  inspect evidence, choose a demonstration threshold, interpret limits and export results.
- Refresh all captures in both themes and the site preview; check links, accessibility,
  viewport overflow, release version agreement, installers and six platform builds.

**Acceptance.** All released projects, full Go race/vet and scientific validation, JavaScript
and contrast checks, guided browser workflows, site audit and isolated performance budgets.
The maintainer executes the commit, PR/merge, tag, release and site publication.

**Implemented.** The nine-step guide ends with cited methods; its worked example explains
expected counts, interpretation limits, exports and returning to Tracking for the next step.
The 0.5.0 compatibility fixture retains 60 paired cells (120 chain rows) across three families,
six samples and two subjects, plus a joined synthetic IMGT bundle. Its seven results retain
nearest-only and thresholded families, germlines, SHM, six family/locus trees and isotypes.
Reference FASTA and catalog provenance are pinned; replay preserves IMGT grouping. Earlier
release fixtures are unchanged. Program, web assets and citation now name 0.5.0; changelog,
release overview, scientific validation, formats and user guides describe the completed scope.

**Checked.** Application/internal and complete scientific Go race checks pass (scientific
run: 788.685 s). Non-race scientific validation with every required CI dataset passes
(261.504 s), with all six performance budgets run separately without competing validation
or capture processes (87.704 s). The million-row read/validation takes 1.568 s, cached reopen
129 ms and a searched page 37.348 ms. One million generation probabilities take 41.703 s;
100,000 paired TCR neighborhoods take 34.591 s, both on ten processors.

All four release projects reproduce on arm64 and amd64; 0.3.0 onward compare byte for byte.
Vet, formatting, release-version agreement, six platform builds, shell installer syntax,
95 JavaScript tests and 14 contrast checks pass. All 31 browser scenes pass in both themes,
including the complete guide's saved result kinds and scientific citations. The site builds
33 pages with 31 screenshot sets; its 128 accessibility/viewport checks find no problems,
and eight focused checks cover the final guide wording. Captures and desktop/phone previews
were visually inspected. The link-preview image was regenerated and remains byte-identical.
PowerShell syntax remains a CI check because pwsh is not installed locally; optional live
publisher downloads and the maintainer-only VDJdb lookup were not requested or rerun.

Committed, merged and released by the maintainer as 0.5.0; the product site is published.

## 0.6.0: single-cell expression and receptors (wave 6)

Keep expression in a separate sparse matrix, linked to receptor evidence by an explicit dataset
and sample plus exact barcode. Never strip suffixes, match across samples, infer missing cells
as zero expression, or treat cells as biological replicates. Preserve feature IDs, names and
modalities separately. Imported coordinates and annotations remain source evidence; computed
maps and statistical results are immutable artifacts with effective parameters and citations.

SituCraft supplies useful sparse-matrix and AnnData patterns, but its HDF5 implementation is a
writer and its AnnData reader uses Zarr. The earlier design's assumption of a reusable HDF5
reader was incorrect. Reading HDF5 is explicit work here, using format documentation and
independent fixtures; unsupported encodings must fail clearly, never return partial matrices.

### Wave 6 sequence

| Slice | Scope | Requirements |
| --- | --- | --- |
| 1 | MEX expression evidence: strict sparse counts, source fingerprints, exact sample/barcode linkage, Cells inspection and saved reopening | D6 (MEX), D11, M1 foundation |
| 2 | Simulated paired receptor/expression study with known cell states; cell annotations and imported maps, explicit unmatched cells | E1, E3, M1 |
| 3 | Bounded HDF5 reader and 10x feature-barcode HDF5 input, checked against independently generated compressed/chunked fixtures | D6 (10x HDF5) |
| 4 | AnnData and MuData input: dense/CSR/CSC expression, observations, variables, embeddings and scirpy AIRR evidence; explicit modality and count-layer selection | D6 (h5ad/h5mu), M1 |
| 5 | Expression preprocessing, neighborhoods and computed map; pin transformation, feature selection, distance, seed and all effective defaults; refine algorithms from primary methods first | M1, N7 |
| 6 | Clonotype sizes on maps and phenotypes by clonotype, cluster/cell type and sample; explicit definitions, denominators and subject boundaries | M1, M2 |
| 7 | Clonotype modularity on the expression graph, with defined null model, seeds, multiple testing and independent reference validation | M3 |
| 8 | CoNGA-style receptor/expression graph comparison; paired-cell population, graph definitions, null and correction explicitly recorded | M3 |
| 9 | AnnData export with receptor links, annotations, embeddings and provenance; independent round-trip validation | O3 |
| 10 | Complete guided single-cell study, release fixture, documentation, performance/accessibility checks and release 0.6.0 preparation | E3, O2, N7 |

The later slices are planning boundaries, not promises of unreviewed algorithms. Refine each
before implementation. Differential expression with subjects as replicates remains wave 7;
antigen capture remains parked. Maps are exploratory, never evidence of antigen specificity.

### Cells feedback: concrete acceptance for the remaining slices

The map must connect expression evidence to receptor definitions, not stop at state coloring.
The following refines the existing slice boundaries after researcher review of the slice 2
screenshot; it does not imply that the pending analyses already exist.

- **Slice 4, imported embeddings:** retain the selected embedding name, source dimension names
  and provenance. Display UMAP 1 / UMAP 2 only when the source identifies UMAP; otherwise use
  the actual source names or neutral dimensions. The current TSV contract supplies only
  `map_x`/`map_y`; neither filenames nor cloud appearance establish an embedding method.
- **Slice 5, computed maps:** carry method, effective parameters, seed and axis names into the
  immutable result. Maintain equal axis scale; show source or method labels on both axes.
- **Slice 6, repertoire overlays:** provide continuous clonotype cell size and explicit size
  categories (initial proposal 1, 2, 3–5, >5; configurable breakpoints), plus receptor evidence /
  chain status. Count distinct cells using a saved clonotype definition and QC selection,
  never chain rows. Record whether size refers to all eligible receptor cells in that sample
  or only mapped cells; expose both coverage counts and the chosen population. Unassigned
  clonotypes, absent receptor evidence, unlinked input, QC exclusion and unresolved pairing
  must not be treated as zero-size clones. Distinguish single-chain, paired and multi-chain
  evidence using the existing chain-selection rules, including mixed/ambiguous cases.
- **Slice 6, linked exploration:** selecting a clonotype highlights its mapped cells and dims
  other cells; selecting/brushing map cells lists their clonotypes and unresolved evidence in
  the inspector. Preserve exact dataset/sample/barcode keys, selected definition/QC/result
  IDs, reset/undo behavior and a keyboard-accessible table/filter alternative to brushing.
  Sample/timepoint coloring or faceting must retain subject boundaries and missing metadata;
  separate embeddings cannot be concatenated merely because both have two coordinates.
- **Density overlay, optional after the core slice 6 interactions:** first define and cite the
  estimator, bandwidth, grid, weighting/normalization, eligible population, contour levels
  and minimum support. Compute in Go and record parameters. Density in embedding coordinates
  is descriptive; it is not enrichment, physical density, statistical significance or proof
  of antigen specificity. Do not add attractive contours without a defined calculation.
- **Figures across views:** missing values have a neutral style distinct from valid categories
  and literal strings resembling missing-value labels. Include overlap, empty/high-cardinality
  categories, source axes, equal scaling and both-theme checks. Never jitter imported source
  coordinates silently; retain every observation in an accessible source-value table.

**Slice 2 follow-up implemented.** Missing annotations now use outlined neutral gray and draw
last, fixing the hidden missing-label point in the grid screenshot. Explicit `map_x`/`map_y`
axis labels retain equal scale. The new `cell-states-clouds` preset supplies 360 cells per
sample with continuous Gaussian layout, unequal expected state sizes and differing spreads.
It is the visible study example; the original preset, example, action default and seeded
reference bytes remain available. This is still artificial supplied layout, not UMAP or fitted
biology. See research §5.23 for methods and the acceptance rationale.

**Follow-up checked.** Application/internal race tests and focused expression/cell-state
scientific race tests pass. The original and cloud fixtures agree with independent SciPy
references; the new reference checks all 34,560 matrix values and 1,440 cell totals. Native
and amd64 checks, released-project replay, vet, six platform builds, 100 JavaScript tests
and 14 contrast checks pass. The cell-state browser scene passes in both themes, including
missing-point selection and saved reopening; both refreshed screenshots were inspected.
The site builds 34 pages with 33 screenshot sets, with zero problems in 132 checks.
Unrelated scientific regressions, fuzzing and performance budgets were not rerun for this
follow-up. Version remains 0.5.0. Committed by the maintainer as `a081bd6`.

### Slice 1: MEX expression evidence

**Serves D6, D11 and the linking foundation of M1.** Read a selected directory containing
`matrix.mtx[.gz]`, `barcodes.tsv[.gz]` and `features.tsv[.gz]` (or legacy `genes.tsv[.gz]`).
Read integer-general coordinate matrices as nonnegative UMI counts. Preserve source order,
feature type and IDs; names may repeat. Reject ambiguous companions, duplicate identifiers or
coordinates, unsupported matrix encodings, malformed/truncated gzip, dimension mismatches,
unsafe counts and resource-limit violations. Stream text; never allocate from an unchecked
header or materialize a dense matrix. Keep zero-count barcodes and all feature modalities.

Require an explicit receptor sample when linking; count distinct barcodes, not chain rows.
Show matched, expression-only and receptor-only counts, missing receptor barcodes and bounded
cell/feature pages. This is raw source coverage, before receptor QC or clonotype definitions.
Gene-expression UMIs exclude other modalities. A matched barcode is not a called productive
cell. Save the file fingerprints and binding in the project; reopening changed files must fail
identity checks. Source files stay in place and unmodified. Remove a saved binding through undoable
project edits. No normalization, cell-type assignment, map or inferential statistic in this slice.

**Acceptance.** Hand-calculated sparse values and links, gzip/plain/legacy parity, malformed
input and resource boundaries, cancellation, fuzzing, independent SciPy Matrix Market numbers,
API paging and fingerprint drift, older project compatibility and arm64/amd64 agreement.
Exercise visible controls, error handling and saved reopening in both themes. Run Go race/vet,
JavaScript tests, six platform builds, contrast and site checks. Document limits and examples;
keep version 0.5.0. The maintainer performs all Git and release operations.

**Implemented.** The independent sparse expression model, streamed plain/gzip MEX reader,
read-only source fingerprints, exact sample/barcode coverage, bounded API pages and Cells view
are in place. Projects retain bindings and verify all three source hashes when reopened. The
artificial example and remote action exercise repeated chains, nonproductive evidence, both
unmatched populations, a missing receptor barcode and measured zero counts. Feature names may
repeat; antibody counts are retained separately and a matrix without gene features reports
unmeasured gene totals. Reading rules, limits, source citations and a development guide are
written. The HDF5 assumption is corrected and the remaining wave is split into nine later slices.

**Checked.** Application/internal race tests, focused native/amd64 expression checks,
released-project replay and processor checks, vet, six platform builds, 97 JavaScript tests,
199,350 fuzz executions and 14 contrast checks pass. SciPy independently agrees with every
artificial matrix value and gene total; set-based reference coverage agrees. The browser scene
passes in both themes, including the visible controls, errors, project saving/reopening and
remove/undo. Both screenshots were inspected. The site builds 34 pages with 32 screenshot sets;
132 accessibility/viewport checks across 33 pages find no problems. Scientific regression
validation under the race detector passes (1,159.230 s). Existing performance budgets, which
skip under the race detector, were not rerun for this reader/UI slice. Version remains 0.5.0.
Committed by the maintainer as `9f5738b`.

### Slice 2: known cell states and imported maps

**Serves E1, E3 and the imported-evidence part of M1.** Add an explicitly selected plain/gzip
TSV with first column `barcode`, optional paired `map_x`/`map_y` columns and up to 32 other
annotation columns. Empty fields mean missing; coordinate pairs must be both absent or finite
within ±1e12. Reject duplicate barcodes/headers, malformed rows and excessive input. Join exact
barcodes to the selected matrix, retain annotation-only rows, and report missing annotations
and coordinates separately. Pin the optional file in the expression identity; MEX-only IDs and
old saved inputs remain unchanged. Never discover an annotation file implicitly.

Show bounded annotation pages and a supplied-coordinate map (up to 20,000 matched points;
larger maps fail explicitly). Coordinates are source evidence, not a computed UMAP. Keep the
table as an accessible alternative with source values and missingness. Scientific counts and
map populations come from Go; JS only lays out and displays the supplied points.

Add the `cell-states` artificial study: two samples per subject, paired fictional TRA/TRB
receptors, three known states, 24 artificial genes, seeded negative-binomial counts and supplied
layout coordinates. Record parameters, sample/barcode/clone/state truth and source hashes.
Deliberately include expression-only, receptor-only, annotation-only, missing-label/map and
zero-count cases. Keep truth separate from observations and subject/sample identity explicit.
Offer the preset offline with a guided first-sample inspection. This is a controlled software
benchmark with chosen parameters, not a fitted biological model or evidence of specificity.

**Acceptance.** Strict parser, cancellation, malformed/gzip/limit and drift checks; exact joins,
missingness and source-order pages; seeded byte reproducibility, changed-seed and processor
agreement; independent SciPy and truth/set tallies; project reopening and remote controls;
visible form/map/table checks in both themes. Run relevant race tests, scientific regressions,
vet, JS tests, six builds, contrast and site checks. Keep version 0.5.0; maintainer performs Git.

**Implemented.** The explicit annotation reader, fingerprinted optional source, complete
coverage and bounded annotation/map APIs are in place. Cells displays supplied labels and
coordinates, missing evidence, annotation-only rows and an accessible source-value table;
saved reopening checks all selected sources. The `cell-states` CLI preset and offline guided
example generate four default samples with 480 expression cells, 960 paired AIRR chain rows,
separate observation truth and source manifests. Simulation, format, remote-control and
development-guide documentation describe the chosen model and cite its sampler/format sources.
MEX-only identities and released projects remain compatible. Computed maps and clonotype
phenotypes remain later slices.

**Checked.** Application/internal race tests, focused scientific race tests, full required-data
scientific regressions (246.975 s), released-project replay, native/amd64 checks, vet, six
platform builds, 99 JavaScript tests and 14 contrast checks pass. SciPy and independent truth
tallies agree on all 11,520 generated matrix values, 480 gene totals and source coverage.
Annotation fuzzing passes 169,189 executions. The Cells and cell-state browser scenes pass in
both themes, including forms, malformed paths, pagination, imported-map selection and project
saving/reopening. Screenshots were inspected and legend spacing corrected. The site builds
34 pages with 33 screenshot sets; 132 accessibility/viewport checks report zero problems.
Existing performance budgets were not rerun for this evidence/simulation slice. Version stays
0.5.0. Committed by the maintainer as `152c061`; the feedback follow-up above is separate.

### Slice 3: bounded 10x HDF5 expression input

**Serves D6 (10x HDF5) and D11.** Read an explicitly selected feature-barcode HDF5 file into
the same sparse expression model as MEX. Require `/matrix/{shape,data,indices,indptr,barcodes}`
and `/matrix/features/{id,name,feature_type}`. Preserve every feature modality, explicit zero,
barcode and repeated feature name. Validate CSC dimensions, monotone pointers, exact integers,
unique IDs/barcodes and duplicate coordinates before exposing any input. Sort paired sparse
indices/values per cell; never sum duplicates or coerce float/negative counts. Do not select a
genome, raw/filtered matrix, count layer or modality implicitly.

**Reader boundary.** Implement from the primary HDF5 specification, using Go's standard library.
Support 8-byte offsets/lengths; superblocks 0–3 without extensions; classic symbol-table and
compact-link groups; object headers 1–2 with continuations; fixed ASCII/UTF-8 strings and full-width
1/2/4/8-byte integers in either byte order; compact/contiguous arrays and v3 chunk layouts with
v1 B-tree indices. Decode deflate and shuffle, verify Fletcher32 and metadata checksums, honor
per-chunk filter masks. Every required dataset is a fully allocated vector (or empty vector).
Bound file reads, metadata, decoded bytes, groups, traversal depth and chunk count before
allocation. Unsupported encodings fail explicitly, including variable strings, dense groups,
new chunk indices, external/virtual storage and soft/external links on required paths. MEX is
the fallback for encodings outside this boundary. AnnData/MuData support expands the reader
deliberately in slice 4; this slice does not claim to be a general HDF5 implementation.

**Integration.** The existing Cells path field accepts a file or directory. Add an offline HDF5
example and remote action selector, retain optional TSV annotations, exact receptor sample
binding and undoable saved inputs. Fingerprint the entire HDF5 file before and after parsing;
reject concurrent changes. Give it a separate versioned identity domain, so MEX identities and
released projects remain unchanged. Hashes differ between encodings even when counts agree.

**Acceptance.** Compare all values, labels, totals and stored-entry counts to pinned h5py/SciPy
references. Include compressed/shuffled/checksummed multi-level chunk trees, metadata
continuations, user blocks, big-endian/small integers, filter masks, empty count arrays and
compact storage. Reject malformed CSC evidence, unsupported storage, checksum corruption,
truncation and cycles. Exercise cancellation, limits, fuzzing, native/amd64 agreement, source
relocation/drift, API linkage and visible file opening/saved reopening in both themes. Run Go
race/vet, scientific regressions, JavaScript tests, six platform builds, contrast/site checks;
refresh the Cells screenshots and format/method documentation. Version stays 0.5.0. The
maintainer performs all Git and release operations.

**Implemented.** The bounded HDF5 package and 10x CSC reader, Cells/API/remote integration,
offline example, independent fixtures and documentation are in place. Source fingerprints,
optional annotations and existing MEX behavior remain intact. Independent checksum checks
caught and corrected Fletcher32's all-one-word edge case.

**Checked.** All values, labels and gene totals agree with h5py/SciPy for 17 supported fixtures;
18 invalid/unsupported inputs fail as specified. A separate public 10x PBMC file agrees on all
2,498,225 stored entries, 33,538 features and 1,222 cells, with no real data added to the repository.
Application/internal race tests and required-data scientific regressions pass (605.894 s for
the latter); final focused race, native/amd64, released-project replay and processor checks
pass. Vet, six platform builds, 100 JavaScript tests, 391,633 HDF5 fuzz executions across two
runs, and 14 contrast checks pass. Cells, HDF5 Cells and cell-state browser scenes pass in both
themes; all six refreshed screenshots were inspected. The site builds 34 pages with 34 screenshot
sets and zero problems in 132 checks. Existing unrelated performance budgets, skipped under
the race detector, were not rerun. Version stays 0.5.0; committed as `78001bb`.

### Slice 4: selected AnnData and MuData evidence

**Serves D6 (AnnData/MuData), D11 and M1's imported-evidence foundation.** Read one `.h5ad`
or one explicitly selected `.h5mu` expression modality, with an explicit `X` or `layers/<name>`
count selection. Never infer that X is raw counts, choose a modality from its name, join global
MuData observations by row number, or combine unrelated embeddings. The selected matrix must
contain nonnegative exact integer values, with each count and gene total at most 2^53−1. Numeric
float storage is accepted only for finite integral values. AnnData units remain **counts**;
UMI semantics are not inferred. Dense arrays and CSR/CSC arrays become the existing sparse
cell-major model. Validate dimensions, pointers, indices and duplicate coordinates; sparse
explicit zeros remain evidence. Dense storage counts every supplied coordinate, retaining zero
values implicitly in memory.

**Annotations and maps.** Read the selected modality's obs/var index and declared column order,
including categorical codes, nullable integer/boolean/string arrays, text and numeric columns.
Keep exact barcode/index values, missingness and variable annotations. Require unique indices;
feature names may repeat. Feature IDs come from var's index; optional name/type columns are
explicit choices. Otherwise names equal IDs and the selected expression matrix is Gene
Expression. Up to 32 obs and 128 var columns, 4 million values and 64 MiB text per frame.
Select one two-dimensional dense obsm embedding. Record its source path and derive axis labels
from its key (`X_umap`: UMAP 1/2; `X_tsne`: t-SNE 1/2; otherwise key 1/2), without inventing a
method for generic keys. Keep supplied values and equal axis scaling. Both NaN means missing
coordinates; partial/nonfinite/oversized pairs fail. No source embeddings are concatenated.

**Scirpy AIRR evidence.** Select an obsm key and, for MuData, its modality explicitly. Decode
AnnData's Awkward form/length/buffer encoding into unfiltered per-cell chain records. Keep
source field order, exact text/numbers, missing fields, whole missing cell records and empty
chain lists separately. Join the selected modalities only by exact barcode and report both
unmatched populations. Inspect both chain records and the full AIRR cell population through
paged views. Imported chain indices, clonotypes, Python behavior and analysis results are not
executed or adopted as ClonoLoom results. This slice retains supplied evidence; linking computed
clonotypes and phenotype analyses remains slice 6.

**Reader boundary and provenance.** Extend the own standard-library HDF5 reader with inline
attributes, bounded global heaps for variable strings, IEEE float32/64, boolean enums, scalar
metadata and rank-two arrays/chunks. Existing file/read/metadata/decoded/traversal budgets
remain; unsupported dense groups/attributes, newer chunk indices, shared types, external/virtual
storage and required soft/external links fail. Awkward supports numeric/string scalars,
ListOffset, Record, Indexed/IndexedOption, Unmasked and Empty forms, with at most 16 nesting
levels, 256 forms, 128 fields, 1 million chains, 4 million values and 64 MiB field text. Unsupported
forms fail without evaluating code. Hash the entire source before/after parsing and include all
selections in `clonoloom-expression-anndata/1`. Relocation preserves identity; changed bytes or
selections do not. MEX/10x identities stay unchanged. AnnData supplies its own observations and
cannot also attach the external annotation TSV.

**Acceptance.** Independently write/read dense, CSR, CSC and compressed AnnData/MuData fixtures
through public Python APIs. Compare every count, label, annotation, map coordinate and AIRR
field; exercise reordered/unequal modality populations, empty matrices, signed/unsigned/floating
count storage, edge chunks, missing values, malformed schema, count rejection, source drift,
selection identity, cancellation and fuzzing. Exercise the visible selections, remote action,
saved reopening and failed-open preservation in both themes. Update primary format citations,
user/API documentation and screenshots. Run race tests, scientific regressions, released-project
replay, native/amd64 checks, vet, six builds, JS, contrast and site checks. Keep version 0.5.0;
the maintainer performs Git operations.

**Implemented.** Reader, Cells/API/remote selections, source-aware map labels, variable/AIRR
pages, saved provenance and an offline guided AnnData example are in place. Independent
fixture and adversarial validation is in place. The source-observation policy also rejects an
external cell TSV with an AnnData/MuData input.

**Checked.** All count values, observation/variable annotations, map coordinates and AIRR fields
agree with independent public-API references for 13 supported AnnData/MuData fixtures;
21 invalid/ambiguous inputs fail as specified. Scirpy's own AIRR cell conversion reads the
synthetic evidence and writes an additional validated fixture. Nullable string/boolean/integer
values, exact large-integer labels, compressed edge chunks, missing whole-cell AIRR records and
unequal/reordered modalities are covered. The expanded 10x suite passes 18 supported and 17
invalid cases; variable strings are now supported without changing old source identities.

Application/internal race tests and the required-data scientific suite pass (542.455 s for
the full scientific run). Final focused race and native/amd64 checks pass, including released
project replay, processor agreement, source drift, selection identity, cancellation and the
external-TSV refusal. Two bounded AnnData fuzz runs pass. Vet, gofmt, six platform builds,
102 JavaScript tests and 14 contrast checks pass. All four Cells browser scenes pass in both
themes; eight screenshots are captured, with the new AnnData map inspected in both themes.
The site builds 34 pages with 35 screenshot sets; all 132 audit checks pass. Existing unrelated
performance budgets, skipped under race detection, were not rerun. Version stays 0.5.0;
committed by the maintainer as `3a7bdd1`. Slice 5 follows below.

### Slice 5: Expression preprocessing, neighborhoods and computed map

**Serves M1 and N7.** Turn a selected expression input into an immutable, reproducible analysis.
Keep it independent of receptor QC/clonotyping, supplied layouts and annotation labels. Read
primary methods before choosing the pipeline; adapt SituCraft 0.9.0's Apache-2.0 numerical
patterns, with ClonoLoom's deterministic math and explicit bounded exact computation.

**Settled method.** Retain positive Gene Expression cells with a configurable minimum detected
feature count. Normalize against all gene counts before feature filtering to 10000 per cell,
apply natural log1p, select up to 500 features by decreasing sample variance with ID ties, and
center without variance scaling or clipping. This is explicitly variance ranking, not a named
HVG recipe. Exact covariance PCA retains up to 30 nonzero components; exact Euclidean search
finds 15 other neighbors per cell. Preserve the directed table and UMAP fuzzy-union graph.
Compute UMAP from a PCA start with seed 20261024, minimum distance 0.5, spread 1, 500 epochs,
five negative samples and serial optimization. Record all fixed rules, fitted curve parameters,
requested settings and effective dimensions/counts. See research §5.26 and `docs/EXPRESSION.md`.

**Bounds and semantics.** No hidden subsampling or approximate search: at most 100000 source
barcodes, 10000 retained cells, 1000 selected features and two billion cell-by-feature-pair
operations. Four retained cells and two nonzero PCs are required. Keep zero-count and
below-threshold exclusions distinct. Preserve exact barcode/feature identities, missing
annotations and all source fingerprints. Cells missing imported coordinates can still map.
No batch correction, cluster calling, receptor phenotype or subject-level inference is added.

**Implementation and acceptance.** A separate expression-map job verifies source identity,
reports progress/cancellation and writes a checked immutable artifact. Project Results can
reopen it without source files. Cells offers explicit settings, equal-scale computed UMAP axes,
annotation coloring, missing values in gray, paged cell/exclusion/feature tables, cited methods
and JSON download containing scores, loadings, graph and provenance. A guided step follows the
cloud study's imported-map inspection. Later receptor overlays use this expression parent;
map distances are never substituted for PCA-space neighborhoods.

**Validation.** Independent pinned NumPy/SciPy/scikit-learn/umap-learn references cover transformed
moments, variance ranking, PCA, exact neighbors, fuzzy weights and UMAP curve fitting. Artificial
three-state counts assess neighborhood purity and map trustworthiness; reference JIT is disabled
and finite bandwidths checked after finding nonfinite values under compiled fast math. Check
source drift, cache identity, cancellation, failed promotion, missing/zero values, bounds,
worker independence, native/amd64 byte identity, both UI themes, saved reopening and undo.
**Done, committed as `ef6d893`.** The application/internal race suite and complete
scientific validation pass (477.226 s for the latter). Both independent numerical cases agree;
known-state neighbor purity is 1.0, and map trustworthiness is 0.9780 and 0.9642 versus the
reference's 0.9774 and 0.9618. The optional public 10x 1k PBMC matrix completes in 1.74 s with
1222 retained cells, 500 features, 30 PCs and 13678 undirected graph edges. This is a bounded
pipeline exercise, not biological validation of a donor's cell types.

Native and amd64 checks pin identical expression result bytes; released-project replay and
existing processor invariants pass. Cancellation, source drift, altered-result rejection,
worker independence, resource bounds, degenerate inputs and missing/zero semantics pass.
Vet, gofmt, six platform builds, 103 JavaScript tests and all 14 contrast checks pass. All five
Cells capture scenes pass in light/dark themes, including computed-result save/reopen, undo,
annotation coloring and table navigation. Both new screenshots are visually inspected. The
site builds 34 pages with 36 screenshot sets; all 132 audit checks pass. Version stays 0.5.0.

### Slice 6: Clonotype overlays and phenotypes

**Scope (M1, M2).** Bind one expression input and either its imported coordinates or one saved
computed map to an explicit receptor dataset/sample. Reuse the normalized clonotype definition
and QC mask. Count distinct sample/barcode cells, never chain rows, with no suffix stripping.
Save both all uniquely assigned receptor cells in that sample and mapped assigned cells per
clonotype; the user chooses the size population (default receptor sample). Configurable inclusive
breakpoints default to 1, 2, 5. Missing receptor evidence, QC exclusion, unassigned clonotypes,
ambiguous assignments and absence from the chosen size population remain distinct, never zero.
Definitions that assign a cell to several locus-specific clonotypes cannot supply a unique clone
size; report those cells as ambiguous. Conflicting vendor labels are also ambiguous.

**Phenotypes.** Select a supplied annotation (cluster, cell type or state); do not infer labels.
Count all expression barcodes, including those without coordinates, by annotation and clonotype.
Keep missing labels as a separate group. Report assigned/sized cells, expanded cells (size > 1),
mapped coverage, within-clonotype expression-cell fractions and within-group expression-cell
fractions with their denominators. Expansion shares use sized cells only. Each result belongs
to one sample and pins its supplied subject/timepoint; no pooled donors, cell-level significance,
expression DE, batch correction or antigen-specificity claims. Sample/timepoint coloring is
available within that one map; independently fitted maps are never concatenated.

**Interaction and record.** Continuous size, size-bin, receptor/QC/assignment, chain-status and
supplied-annotation coloring; neutral missing evidence; exact source axes and equal scale.
Clonotype selection highlights mapped cells; map clicking/brushing and table selection list
clonotypes and unresolved evidence in the inspector. Reset/undo selection and table filtering
provide an accessible equivalent. Pin expression/map IDs, parent checksums, dataset fingerprints,
definition ID, filter values, sample metadata, populations and method version in immutable job
artifacts. Results reopen without source files and export their full JSON record.

**Sources.** scirpy's documented `clonal_expansion` API describes sample-scoped sizes and inclusive
breakpoints; Sturm et al. 2020, doi:10.1093/bioinformatics/btaa611. The 10x Loupe integrated V(D)J
and gene-expression tutorial documents linked clone/phenotype exploration. These are method and
interaction references, not shared implementations. Counting/ambiguity policies are ClonoLoom
choices. Chain evidence uses QC's productive-contig receptor arms and >2 chains/locus threshold,
with mixed receptor families and unresolved loci explicitly labeled.

**Validation.** Hand-enumerated cells with duplicate contigs, repeated barcodes across samples,
missing annotations/coordinates, QC exclusions, single/paired/multi/mixed chains, vendor conflicts,
all-loci definitions and both size populations. Independent tabulation validates sizes/bins and
phenotype fractions. Test artifact identity, altered inputs/parents, cancellation, saved reopening,
linked selection, both themes, undo, accessible tables and processor determinism. Finish the
application/scientific suites, browser capture and documentation checks before handoff.

**Done, committed as `3aaa344`.** Imported and computed overlays, distinct-cell
populations, explicit ambiguity/missingness, chain evidence and supplied-phenotype summaries
are implemented. Immutable jobs pin source/map checksums, normalized definitions, QC content
fingerprints and project filter versions, sample metadata, options and citations. Cells supports
continuous/category coloring, clone links, pointer brushing, searchable/paged tables, selection
reset/undo, result undo/redo, saved reopening and complete JSON export. The cloud study has a
third guided step; the remote action is `compute_cell_phenotypes`.

Independent SQL tabulations agree for both samples and both size populations, including every
category, phenotype numerator and denominator. Hand-enumerated ambiguous assignments, repeated
barcodes/contigs, QC exclusions, missing labels, mixed/unknown chains, empty populations,
resource bounds and cancellation pass. Source drift and altered parent/result checksums are
refused. Full application/internal race tests and scientific validation pass (498.369 s for the
latter). Native/amd64 phenotype digests and released-project/processor compatibility checks
agree. Vet, gofmt, six platform builds, 104 JavaScript tests and all 14 contrast checks pass.
All six Cells capture scenes pass in both themes, including actual pointer brushing and
Clonotypes-to-Cells navigation; both new screenshots are visually inspected. The site builds
34 pages with 37 screenshot sets and passes all 132 audit checks. Version remains 0.5.0.

### Slice 7: Clonotype modularity on the expression graph

**Scope (M3).** Test predefined clonotypes on one saved computed expression graph, using a
checksum-pinned phenotype parent for its exact sample/barcode assignments, normalized
clonotype definition, QC and metadata. Keep uniquely assigned retained cells and their induced
binary graph, including isolated vertices. Every positive fuzzy-union edge counts once;
weight magnitudes and two-dimensional UMAP coordinates do not enter this statistic. Imported
maps alone cannot supply it. Preserve excluded assignments and expression cells outside the graph.

**Settled method.** Use the documented scirpy connectivity score
`log2((observed + 1)/(expected + 1))`, distinct from Newman modularity Q. Compute expectation
analytically as `M s(s−1)/(N(N−1))` on the fixed induced graph. The null assigns a clone uniformly
to size-s vertex subsets, conditional on graph and size, within one sample. Enumerate every
subset when choose(N,s) <= 10000; otherwise draw 9999 independent uniform subsets by default,
with replacement between draws and PCG seed 20261024, size-keyed streams. Use inclusive
upper-tail tests: tail/draws for enumeration, (tail+1)/(draws+1) for Monte Carlo. No parametric
tail extrapolation. Benjamini–Yekutieli adjusts the complete family of clones with 2 <= s < N,
including zero-edge clones, because shared graphs/null draws do not justify independence.
Singleton and whole-population clones have explicit untested/null fields.

**Interpretation and bounds.** Exchangeability is an assumption, not a finding: this null does
not condition on cell type, batch, receptor family or degree. It tests exploratory within-sample
connectivity, not antigen specificity, causality or subject-level effects. No pooled donors.
Correction does not cover repeated choices of graph, QC, definition or sample. Retain null
histograms and attainable Monte Carlo resolution. Limits: 10000 graph cells, 99–99999 requested
draws, and a conservative 500 million work-unit bound; refuse excess rather than subsample.

**Delivery and validation.** Immutable jobs verify both parents and reopen without raw sources;
saved results reopen without parents. Cells displays observed/expected edges, score, raw and
adjusted p-values, untested reasons, a discrete null-frequency plot with zero baseline and
accessible tables. The plot is a null distribution, not an effect confidence interval. Keep
methods, definitions, population, source fingerprints, hashes, seeds and limits in JSON export.
Add remote control, a guided artificial-study step, docs and screenshots. Independently enumerate
subsets with Python itertools and check BY with SciPy's public API; compare Monte Carlo
frequencies to exhaustive truth. Check graph validation, degeneracy, exclusion, cancellation,
changed parents, processor identity, saved reopening, undo, both themes and regression suites.

**Done; committed by the maintainer as `7a991b9`.** Sample-scoped induced graph tests, analytic expected
edges, exhaustive/seeded null distributions, inclusive nonzero p-values and BY adjustment are
implemented. Results retain complete phenotype evidence, both parent hashes, effective options,
method citations, graph memberships, null frequencies and explicit untested states. Jobs verify
parent formats/versions and hashes, support cancellation/resumption and refuse changed parents.
Saved results reopen without sources or parents. Cells offers a discrete zero-based null plot,
accessible tables, unchanged test families under filtering, parent-overlay highlighting,
project undo/redo and full JSON export. Remote control and the fourth cloud-study guide step
exercise the same workflow.

Independent Python enumeration agrees on every exact histogram, edge count, expectation, score,
tail and SciPy BY adjustment; 99999 Monte Carlo draws agree with the exhaustive 15504-subset
reference within the declared probability tolerance. Hand-counted exclusions, isolated cells,
singleton/whole-population clones, empty/complete graphs, malformed CSR, work limits and
cancellation pass. Native/amd64 result digests agree, as do released-project replay and existing
processor checks. The artificial cloud workflow has 359 retained graph cells, 358 eligible
receptor-linked cells, 3963 induced edges and 10 tested clones; the unmatched cell remains an
explicit exclusion. These fixtures validate the method, not biological specificity.

The final application/internal race suite and full required-data scientific suite pass
(484.031 s for the latter). An initial assembly check found a fused operation in progress
reporting; explicit rounding fixes it, and the full final run is clean. Vet, gofmt, six platform
builds, 106 JavaScript tests and all 14 contrast checks pass. All seven Cells capture scenes
pass in both themes; final modularity screenshots are refreshed and inspected. The site builds
34 pages with 38 screenshot sets and passes all 132 audit checks. Existing unrelated performance
budgets, skipped under race detection, were not rerun. Version remains 0.5.0.

### Slice 8: Receptor / expression neighborhood comparison

**Scope (M3).** A bounded CoNGA-style comparison of paired αβ clonotypes in one sample, using
checksum-pinned computed-map phenotype evidence and all retained expression PCs. New requests
capture the original open receptor dataset's required clonotype sequences; durable jobs then
need no raw expression/receptor files. Full TCRdist requires explicit organism and a pinned
V-loop reference. CDR3-only is an explicit approximation, including in the fictional offline guide.
No imported-coordinate distances, donor pooling or treatment inference.

**Settled method.** Refine from Schattgen et al. (2022), Dash et al. (2017), TCRdist3 conventions
and conditional hypergeometric probability. Use one mean-PC profile per uniquely assigned,
paired clonotype; clone size does not multiply graph vertices. Require primary TRA/TRB amino-acid
clones resolving V/J genes or alleles; preserve invalid/missing evidence reasons. For each focal
clone, exclude self and any identical α or β CDR3 from both candidate pools. Construct exact,
directed k-nearest sets on mean PCs and paired receptor distances, including every cutoff tie.
Test overlap with an inclusive hypergeometric tail conditional on the actual two degrees and
candidate population. Correct every testable clone-by-k row with Bonferroni, including zero
matches and degenerate tied sets. Keep all scales; never silently choose the best. Insufficient
candidates produce explicit untested rows. Stable log tails preserve probabilities below float
range. These declared policies are not an exact reproduction of the original multi-graph score.

**Interpretation.** Independent, exchangeable neighbor labels are assumptions, not established
properties of a biological dataset. The null does not condition on cell type, batch or chain
groups; correction cannot repair nonexchangeability. Results are exploratory within one sample,
not specificity, causality or subject-level effects. Repeated analysis choices are outside the
correction. Means can hide within-clone heterogeneity; keep parent-overlay exploration.

**Delivery and validation.** Save sequences, clone memberships, mean PCs, neighbor identities,
actual degrees, exclusions, expectations, probabilities/log probabilities, source/parent/reference
fingerprints and cited method parameters. Expose a zero-based observed-count / expected-marker
plot and accessible complete tables; it is not a confidence interval or density. Add remote
control, a cloud-study guide step, docs and both-theme screenshots. Bounds: 10000 retained cells,
2000 assigned clones, eight scales, k <= 200, two million stored neighbor entries, 500 million
conservative distance work units; refuse excess without subsampling. Validate against independent
SciPy geometry/tails and exhaustive set enumeration, including tied/null controls, shared chains,
clone abundance, missing/ambiguous evidence, cancellation, source drift and saved reopening.
Run race/scientific regressions, processor checks, six builds, JavaScript, browser/contrast/site
checks. Keep version 0.5.0; the maintainer performs every Git operation.

**Done, committed as `c4b29dd`.** The bounded comparison, immutable job/API, Cells
controls, observed/expected plot, accessible tables, remote action and fifth cloud-study step
are implemented. New results pin source-compatible receptor snapshots, both parent hashes and
the full-distance reference when used; saved results reopen without sources or parents. Missing
full-distance references never fall back automatically. Both neighbor sets, actual degrees,
clone means/memberships, cell exclusions, conditional tails and all-scale adjustments are kept.
Untested overlaps/statistics are null. The visible method notes include nonexchangeability and
possible direct coupling through receptor genes retained in the inherited PCs.

Independent SciPy geometry and tail probabilities agree with every reference neighbor identity,
mean profile, degree, overlap, expectation and Bonferroni value. Exhaustive subsets confirm small
upper tails and super-uniform null probabilities; extreme tails retain finite log values.
Concentrated-neighborhood and fully tied controls, shared-chain exclusions, clone abundance,
source-order invariance, missing loops/evidence, malformed inputs, work/output bounds and
cancellation pass. The cloud workflow retains 358 paired cells among 359 expression-PC cells,
10 clonotype profiles and 20 tests at k = 2, 4. These artificial checks validate the specified
calculation, not biological specificity or general null exchangeability.

Application/internal race tests and the full required-data scientific suite pass (475.803 s
for the latter). Final focused race/assembly/reference checks cover the explicit null export;
API checks cover missing-reference refusal and saved-job integrity. Native/amd64 deterministic
results, released-project replay and processor checks agree. Vet, gofmt, six platform builds,
108 JavaScript tests and 14 contrast checks pass. All eight Cells browser scenes pass in both
themes, including visible/remote equivalence, unchanged test families under filtering, overlay
links, undo/redo and saved reopening. Final comparison screenshots are refreshed and visually
inspected. The site builds 34 pages with 39 screenshot sets and passes all 132 audit checks;
four focused Cells checks pass after the final methods wording. Unrelated performance budgets,
skipped under race detection, were not rerun. Version remains 0.5.0.

### Slice 9: AnnData export with receptor links

**Scope (O3).** Export a verified phenotype parent, with either supplied coordinates or a pinned
computed expression map, as a portable AnnData file. Preserve raw cell × feature counts, exact
barcodes, all selected modalities, annotations and sample-scoped receptor assignments. Retain
excluded cells with missing computed coordinates; no refitting or biological inference.

**Settled contract.** Follow the AnnData/HDF5 specifications and SituCraft's compatible HDF5
writer conventions. Use int64 CSR, UTF-8 strings, categorical missing codes, distinct supplied
and computed embeddings, explicit map inclusion/exclusion, and versioned ClonoLoom receptor
links. Save complete parent documents/manifests, clone-chain definitions, ambiguity candidates,
source fingerprints, sample metadata, QC, parameters and citations in uns/clonoloom. Do not
claim scirpy raw AIRR-chain compatibility. Original reader text/missing semantics are retained;
original pandas dtypes and unselected input layers/modalities are not exported.

**Implementation.** Bounded Go writer and export package; durable checksum-pinned jobs, source
verification, cache identity, cancellation/resumption and portable download with NOTICE. Cells
supports preparing, keeping, undoing, reopening and downloading exports; the remote action and
sixth cloud-study guide step exercise the same path. Keep the application version at 0.5.0.

**Done, committed as `6c4f06e`.** AnnData/h5py independently read every sparse count,
cell/feature identity, Unicode label, missing code and embedding row; AnnData's own writer
rewrites the object without changing those values. The oracle pins AnnData 0.12.6, h5py 3.15.1,
NumPy 2.4.2, SciPy 1.17.0 and pandas 2.3.3. Its deterministic Go fixture hash is
`48c3ed862e5084897a3ad527800e8955fb137b79400e265aa96a3ca6ea7c38a0` on native arm64 and amd64.

Tests cover long Unicode/global-heap boundaries, datatypes, empty arrays, counts beyond float32
precision, supplied variable annotations, reordered parent rows, 32 supplied observation columns,
missing receptor evidence, matrix/annotation/embedding bounds, invalid source positions and
cancellation. API checks cover changed sources/parents, checked downloads without original
sources, cached results, versioned resumption and provider-term acknowledgment. The application
and internal race suite passes; the complete required-data scientific race suite passes in
524.173 s. Focused final export/API race checks and amd64 checks pass after the last validation
and empty-receptor-snapshot refinements. Released-project compatibility and processor identity
checks pass. Vet, formatting/diff checks, six CGO-free platform builds, 108 JavaScript tests
and 14 contrast checks pass.

All nine Cells scenes pass in both themes, including visible/remote export equivalence, ZIP
download, undo/redo and source-free saved reopening. Both export screenshots were visually
inspected. The site builds 34 pages with 40 screenshot sets and passes all 132 accessibility/
overflow checks. Unrelated performance budgets, skipped under race detection, were not rerun.
Version remained 0.5.0 for slice 9.


### Slice 10: complete study and release 0.6.0 preparation

**Scope (E3, O2, N7).** Finish the artificial single-cell workflow and its methods, retain a
portable release project and the complete result chain, reconcile documentation and release
identifiers, refresh visuals, and validate scientific correctness, performance and accessibility.
The maintainer owns commit, PR/merge, tag, release and product-site publication.

**Settled contract.** Keep supplied Gaussian coordinates distinct from computed PCA/UMAP.
Document all populations and missingness: S01-1 has 360 expression cells, 359 retained map cells,
358 uniquely assigned retained cells, ten paired clone profiles and 20 comparison tests at
k = 2, 4. All 360 cells remain in raw-count AnnData. Uniform connectivity and conditional
neighbor-overlap nulls are exploratory; corrected tests do not remove confounding or establish
antigen specificity. Subject-level inference remains wave 7. No scientific algorithm changes.

**Implementation.** Seven guided steps now end with cited methods. Correct the cell-state
simulator's methods, which previously inherited the unrelated TRB model; include saved Cells
results through explicit expression bindings or this dataset's saved phenotype/graph/export
results. Preserve their Go-generated descriptions, effective choices and citations. Check
both simulation variants, real fixture metadata, empty methods and unrelated dataset isolation.
Unknown or renamed simulation identifiers no longer inherit a TRB generative-model claim.
The complete-study accessibility audit found invalid tablist separators and empty list semantics;
fix those and preserve native button roles inside list-item wrappers.

The 0.6.0 fixture is generated through the actual guide in an isolated library: all four
samples, matrices, annotations, separate truth and five saved results. Current durable workers
remake the chain under the same IDs. Source paths and file mtimes are restored to original
provenance only after content/identity checks; scientific values are never substituted.
Verified original parent manifests preserve their job IDs, timestamps and software identity.
The complete AnnData binary, including that provenance, is compared byte for byte. Independent
AnnData/SciPy validation checks the release matrix, labels, links and all embedding rows, then
rewrites it through AnnData and checks again.

Version identifiers, changelog, citation, format references and product-site sources move to
0.6.0. The worked study explains expected results, plots, null assumptions, source retention,
exports and reopening; earlier release fixtures and simulation identities remain unchanged.

**Done, awaiting the maintainer's commit.** Full application/internal race tests pass (root
17.070 s); final focused release/source/example checks pass after the fixture validation changes.
The complete required-data scientific suite passes with race detection (790.910 s) and without
it (257.710 s, performance tests separated). All released projects replay; the 0.6.0 chain and
AnnData binary agree on native arm64 and amd64. Both the small independent export oracle and
the full-release AnnData read/rewrite check pass. All 23 study source/truth hashes agree.

Six isolated performance budgets pass (69.308 s total): one million rows read/validated in
1.5 s at 58 bytes per row plus junctions; a 100-sample cached cohort reopened in 122 ms; a
searched clonotype page in 38.304 ms; 10,000 receptor neighborhoods in 1.141 s; one million
Pgen values in 30.826 s; 100,000 paired-TCR neighborhoods in 27.025 s (last three on 10
processors). The separate public 10x expression-map check passes in 4.75 s: 1222 cells,
500 selected features, 30 PCs and 13678 graph edges, with finite coordinates and complete
population accounting. These are local measurements, not universal hardware promises.

Vet, formatting/diff checks, six CGO-free platform builds, 113 JavaScript tests, 14 contrast
checks and shell-installer syntax pass. PowerShell is not installed locally; its parser check
remains CI's responsibility. All 41 browser scenes pass in both themes. The complete study
checks all guided actions, methods/citations and saved-result reopening; axe-core finds no
WCAG A/AA violations in populated and empty projects after the navigation fixes. All 82
screenshots are refreshed, and the new study plus map/graph-test visuals are inspected.
The site builds 35 pages with 41 screenshot sets and passes 136 accessibility/overflow checks.
The link-preview image is regenerated; its bytes are unchanged because its crop is unchanged.

The maintainer committed and merged wave 6, published release 0.6.0, and published the site.
The release cache-key omission was corrected with a host regression check before publication.

## 0.7.0: cohorts and robustness (wave 7)

The subject is the independent unit. A cohort result must first state the biological
question, sampled population, groups, pairing, exclusions, abundance unit, definition,
QC and multiplicity family. Cells, chains, libraries and repeated samples cannot inflate
replication. Passing structural checks is not a claim of adequate power, randomization,
independence or absence of confounding.

Slice 6 sequence-signature screens are separate conditional repertoire analyses, not cohort
comparisons: their sequence units are not independent subjects. Their exploratory scores
carry explicit exchangeability/ascertainment limits and imply no population-level inference.

### Wave 7 sequence

| Slice | Scope and requirements | Acceptance before the next slice |
| --- | --- | --- |
| 1 | Explicit two-group study designs (G1, G3 foundation) | Independent/paired subjects, exact group labels, explicit sample exclusions, refusal of repeated samples/incomplete pairs, batch-confounding check, named saved snapshots, table inspection, remote control and artificial example |
| 2 | Subject feature tables (G4; G2 inputs) | Source-pinned per-subject metrics with units, definition/QC and missingness, exports, explicit sample selection; do not average diversity across libraries or silently pool counts |
| 3 | Group comparisons (G2, G3) | Design-driven independent/paired tests and nonparametric counterparts, effect sizes and intervals, explicit BH family; subject points and within-subject links, correct interval labels, no cell-level tests; extend multi-group designs only with validated estimability checks |
| 4 | Replicated count models (T4 remainder) | Explicit offsets, design matrices and contrasts, rank/estimability checks, negative-binomial dispersion policy, repeated-subject handling, independently validated calibration and nonconvergence; no substitution of Poisson tests |
| 5 | Definition robustness (C5) | Recompute pinned key results under declared alternative definitions; effect direction/size/uncertainty and failed analyses shown; no verdict based solely on crossing a significance threshold |
| 6 | Motifs and GLIPH2-style groups (R7, S3) | Document published global/local sequence rules, background matching, motif tests and scores; distinguish adaptations, disclose selection/multiplicity, independent reference validation; groups are hypotheses, not proven specificity |
| 7 | Convergence and HLA association (S6) | Unique subjects as observations, explicit typed-subject universe and allele resolution/missingness, hypergeometric tails and declared multiplicity; population structure and ascertainment limitations |
| 8 | Contamination evidence (Q4) | Cross-subject nucleotide matches with explicit abundance/coverage; separate plausible public/high-Pgen receptors; flags are evidence for investigation, never proof of contamination |
| 9 | Batch QC (Q5) | Subject/sample denominators, robust z-score policy including zero MAD and small groups, distributions and exclusions; separate a QC flag from correction or biological inference |
| 10 | Pseudobulk count evidence (M5 foundation) | Exact raw-count sums per subject × supplied cell type × declared expansion category, feature/modality identity, sample/time boundaries, minimum-cell rule, missing versus zero, export and independent aggregation oracle |
| 11 | Pseudobulk differential expression (M5) | Within-subject category comparisons with explicit count design/offsets, estimability, filtering/dispersion/multiplicity and replicate-aware uncertainty; expanded means a stated size category, not antigen specificity; independent null/positive controls |
| 12 | Complete cohort study and release 0.7.0 | Guided artificial null/positive/unbalanced/confounded studies, all results saved and replayable, methods and references, reviewed figures, full scientific/platform/accessibility/performance checks and compatibility fixture |

Each later statistical slice rechecks its primary papers and publishes its exact methods,
limits and independent reference fixtures before implementation is considered complete.
A method that cannot be independently validated is not advertised as implemented. Runtime
remains standard-library Go and vanilla JavaScript; external tools are validation only.
No incompatible-licensed implementation source is inspected or copied.

### Decisions for wave 7

- Start with one opened dataset (which may contain many source files) and two explicitly
  chosen levels, reference then comparison. Subject IDs must be globally meaningful within
  that dataset. Multi-dataset merging and arbitrary formula models are not implicit.
- Slice 1 never infers that two libraries are technical replicates or pools repeated samples.
  Exclude samples explicitly with reasons to select a justified observation. Later count
  aggregation needs its own recorded biological and technical policy.
- Independent designs require disjoint subjects; paired designs require one sample per
  subject at each chosen level, with no automatic complete-case deletion. Two subjects per
  group or two complete pairs is a structural minimum, not a power recommendation.
- Groups use exact trimmed annotation labels. Existing Tracking time parsing is unchanged;
  cohort time labels such as `week 4` and `day 28` are not silently equated.
- Named design versions keep all effective design annotations, choices, exclusions, checks,
  citations and a Go-computed content identity bound to the source-derived dataset ID.
  Annotation edits mark old designs changed; old snapshots remain inspectable/exportable.
- Slice 1 detects complete separation of groups by recorded batch, and warns when batch
  annotations are missing or sampling contexts differ. It does not fit adjusted models or
  certify that all confounding has been ruled out.
- Adapt the siblings' named-design and undo workflows; all scientific rules/counts remain
  in Go. The first visual is an accessible membership/count table, not an inferential plot.

### Slice 1: study design before cohort inference

**Committed.** Requirements G1 and G3 (bounded first implementation).
Add the Cohort view and `POST /api/cohort/design`, requiring the complete sample set of an
opened dataset, with a bounded strict request schema. Validate exact source sample names,
subject grouping, exclusions, independent/paired structure and recorded batch overlap.
Report every sample's disposition and Go-computed group/subject counts. Save named versions
in the project, with undo, changed-annotation detection, JSON export and remote control.
Use the artificial standard study to demonstrate why twelve samples from six subjects are
not twelve independent replicates and how a justified single observation per subject is selected.

Validation: hand-specified independent/paired/missing/repeated/confounded controls; exhaustive
small membership assignments checked against independent SQL aggregation; deterministic saved
identity under row reordering; endpoint dataset membership and strict request failures; project
round-trip, immutable saved snapshots, annotation changes and undo; real-browser example,
both-theme captures and accessibility. No inferential statistics are added in this slice.

**Validation completed:** application/internal race suites pass (root 14.918 s), including
release-project replay; all 116 JavaScript tests pass. The required-data scientific suite
passes in 158.662 s, excluding unchanged performance-budget tests; no existing inferential
algorithm or budgeted data path changed. Focused race checks validate
all 6,561 independent SQL assignments in both layouts (13,122 checks), including membership,
counts, complete pairs and finding counts. Bounds, malformed requests, duplicate/unknown
samples, exact labels and source/annotation/contrast identity changes are covered.

All six CGO-free platform builds and Go vet pass. The real-browser walkthrough passes in
both themes for independent and paired designs, repeated-subject refusal, explicit exclusions,
named saving, undo/redo, project reopening, annotation changes and complete JSON export with
sources unavailable. Both empty and populated Cohort views pass axe WCAG A/AA checks; both
screenshots are captured and visually reviewed. All 14 contrast checks pass. The generated
site builds 36 pages and the 35 audited pages pass 140 theme/viewport checks without violations
or overflow. Formatting and diff checks pass. Version remains 0.6.0 until release preparation;
new work is recorded under Unreleased. No commit, push, tag, release or site publication.

### Slice 2: source-pinned subject features

**Committed and pushed.** G4 (first sample/subject catalog) and G2 inputs.
Bind one explicitly selected saved design version to one verified diversity artifact. Recheck
canonical design inputs and exact complete sample coverage server-side; pin parent bytes before
the job and verify them again in the worker. Keep one sample per subject/group, including paired
subject identity, without averaging libraries or pooling counts. Catalog 13 existing measures
with units and definitions; distinguish zero abundance/richness from empty, singleton and
fractional-count missingness. Preserve the full design/exclusions, parent measurements and
manifest, source notices, definition, QC, options and software/method provenance.

Use an accessible, searchable, paginated observation table with a feature selector; no group
summary or inferential plot. Save immutable artifacts, undo/redo, route from Results, expose
remote control and include saved descriptions in Methods. Export exact wide CSV, catalog, JSON,
manifest and NOTICE; saved evidence must reopen and export without sources or parent artifacts.
Initial creation requires opening the source dataset to bind fingerprints and provider terms.
This is a bounded sample/subject diversity catalog; receptor/usage and other feature families
remain outside the first G4 implementation. Group comparisons begin in slice 3.

Validation covers independent raw-abundance formulas, missingness/zero and unit controls,
explicit selection and pairing, malformed/changed inputs, durable replay/cancellation,
checksum tampering, source-free export, CSV/JSON round trips, browser controls, undo/reopen,
methods and both-theme accessibility/captures. Full application/scientific/platform checks
and documentation audit are complete.

**Validation completed:** application/internal race tests pass, including release-project
replay. All 117 JavaScript tests pass. The required-data scientific suite passes in
160.806 s, excluding unchanged performance-budget tests; existing numeric engines and
budgeted data paths are unchanged. Focused race checks pass the exhaustive 13,122 design
membership comparisons and independent raw-abundance feature reference. Additional API
controls verify provider-term acknowledgments/NOTICE retention and changed-parent worker
failure without promotion.

Go vet and all six CGO-free platform builds pass. Both-theme browser workflows pass visible
and remote selection equivalence, filtering, saved annotation isolation, undo/redo, sidebar
routing, project reopening, source-free ZIP export and cited Methods. Long methods dialogs
are keyboard-scrollable; accessibility checks wait for entrance animations to settle.
Both Cohort views and the methods modal pass WCAG A/AA; all 14 contrast checks pass.
The complete six-subject table screenshots are visually reviewed in both themes. The site
builds 37 pages with 43 screenshot sets; all 36 audited pages pass 144 theme/viewport checks
with no violations or overflow. Formatting and diff checks pass. Version stays 0.6.0 until
release preparation. Subsequently committed and pushed by the maintainer.

### Slice 3: subject-level group comparisons

**Committed and pushed.** G2 and G3, bounded to the existing two-group
independent/paired designs. The scientific contract is recorded in research §5.32.

- Consume one verified saved feature table, rechecking its embedded design and measurements.
  Preserve source/parent lineage; no raw data reopening or refitting of diversity is needed.
- Compute two-sided Welch/paired t and conditional Mann–Whitney/signed-rank tests for every
  explicitly selected feature. Handle ties by midranks and exact enumeration through 50,000
  assignments; otherwise use seeded Monte Carlo with inclusive, nonzero tail probabilities.
- Report mean differences and t intervals separately from probability effects and conservative
  bounded-difference intervals. Keep zero-variance inference unavailable with reasons.
  All intervals are marginal. State test assumptions and distinguish rank nulls from effects.
- Declare exactly two BH slots per selected feature. Any missing observation leaves that
  feature untested without deleting subjects or pairs; untestable slots remain in the family.
  Disclose BH dependence assumptions and additional multiplicity from exploratory selection.
- Show actual subject values and paired links, separately labeled mean-effect intervals,
  selected counts, units and missingness; no cell-level replication or parent bootstrap bars.
- Save immutable cancellable/replayable results, complete CSV/JSON/provenance and figure
  exports, remote controls, Methods and undo/reopen. Preserve provider-term acknowledgment.
  Keep version 0.6.0; do not introduce multigroup/count models or start slice 4 here.

Acceptance: independent SciPy t/interval/distribution/BH references and exhaustive tied rank
nulls, Monte Carlo agreement, exact null calibration and simulated interval coverage; missingness,
pair identity, bounds/cancellation, durable replay, source-free export and parent tampering;
processor reproducibility, full race/scientific/platform checks, reviewed both-theme plots,
real-browser workflows, accessibility and documentation audit.

**Validation completed:** full application/internal race tests pass (root 43.168 s), with
focused race reruns for the final export/provider-term controls. The complete required-data
scientific suite passes in 469.699 s, excluding unchanged performance-budget tests. The new
inference, distribution and conditional-randomization references pass, including focused race
coverage, exact null calibration and 2,000 normal-model replicates per layout. Whole-result
Monte Carlo bytes and numerical references agree on arm64 and amd64; the compiler scan finds
no fused multiply-adds. Saved jobs replay identically and missing parents/sources do not prevent
export; changed evidence is refused before promotion.

All 120 JavaScript tests, Go vet and six CGO-free platform builds pass. Independent/paired
browser workflows pass in both themes, including visible/remote equivalence, exact point/pair
counts, standalone figure captions, provider-aware ZIP export, Methods, undo/redo and saved
reopening. All four screenshots are visually reviewed; the examples state their artificial
scope and the paired teaching labels do not claim a simulated longitudinal intervention.
All 14 contrast checks pass. The site builds 38 pages with 45 screenshot sets; 37 audited
pages pass 148 theme/viewport checks with no accessibility violations or overflow. Formatting
and diff checks pass. Version remains 0.6.0. Subsequently committed and pushed by the maintainer.

### Slice 4: replicated clonotype counts

**Committed and pushed.** T4 remainder, bounded to saved two-group
independent/paired studies. The scientific contract is research §5.33.

- Capture whole observed counts under the pinned feature parent's sequence definition/QC;
  preserve all assigned denominators, filtered/private clonotypes, original sources and terms.
  Refuse unknown/missing/fractional counts and noncomparable vendor clonotype IDs.
- Fit NB2 with assigned-library offsets, per-clonotype Cox–Reid dispersion, group contrast,
  paired subject fixed effects and optional full-rank batch adjustment. Require at least
  12 subjects/group and 24 residual df; retain failures instead of substituting Poisson tests.
- Save the design/contrast, counts, offsets, dispersion, both likelihood fits/covariance,
  conditional marginal Wald log2 intervals, LRT and one explicit eligible-clonotype BH family.
  No composition normalization, empirical-Bayes moderation or automatic discovery labels.
- Add complete subject/paired observations, all-fitted MA-style effects and separate model
  intervals, with inspectable tables, failed fits, figure provenance and no p-value coloring.
- Add immutable cancellable jobs, raw-source-free durable replay, standalone reopening and
  exports, remote control, undo/redo and cited Methods with provider-term preservation.
- Provide independent, paired and null UMI simulations with unequal depths, heterogeneous
  variation, subject/batch effects, private long tails and full generating truth. Explicitly
  distinguish artificial identities and generator exposures from biological reality and
  realized-library offsets. Keep version 0.6.0 until release preparation.

Validation: six independent statsmodels/SciPy numerical references; 1,200 fixed-exposure null
trials spanning balanced, paired and unbalanced layouts; count/source/truth, estimability,
missingness, nonconvergence, boundary, family, replay and provider-term controls; both-theme
browser workflows and visual inspection; full application/scientific/platform checks.
The observed 93.25–93.5% coverage of nominal 95% conditional intervals is disclosed; this
unmoderated asymptotic method is not claimed to have exact small-sample calibration.

**Validation completed:** the required-data scientific suite passes in 488.246 s, excluding
unchanged performance-budget tests. All application/internal race packages pass (application
rerun 40.061 s after updating the example catalog; count-model calibration 456.968 s), with
focused race checks for the final missing/fractional source, separation and work-bound controls.
All six numerical references pass under race and on amd64; native/amd64 whole-result and all
three example digests agree. The compiler scan finds no fused multiply-adds. Source-free
replay/export and inherited provider-term tests pass.

The complete example pipeline fits all 48 eligible panel receptors in each positive study,
with all eight generating changes in the correct direction. One of forty unchanged panel
receptors in the independent study has BH≤0.05, versus none in the paired study. The all-null
study fits 45 eligible panel receptors, none with BH≤0.05; three fail the declared count filter.
These finite controls do not establish exact FDR or interval coverage on arbitrary data.

All 123 JavaScript tests, Go vet and six CGO-free platform builds pass. Both independent/paired
browser workflows pass in both themes: visible/remote identity, exact subject/pair counts,
saved Methods, source-free ZIP and figure exports, undo/redo and project reopening. All four
screenshots are visually reviewed. Review caught and corrected an unintended SVG axis fill;
count observations now have an explicit zero baseline. The site builds 39 pages with 47
screenshot sets; 38 audited pages pass all 152 theme/viewport checks with no accessibility
violations or overflow. All 14 contrast checks, formatting and diff checks pass. Version stays
0.6.0; no tag, release or publication. Slice 4 was subsequently committed and pushed by the maintainer.

### Slice 5: definition robustness

**Committed and pushed.** C5 is bounded to saved aggregate subject-feature
contrasts; the scientific contract is research §5.34. Individual-clonotype count-model audits
need an explicit correspondence between changing receptor partitions and are not claimed here.

- Pin and reconstruct the complete saved comparison, then capture baseline plus 1–5 declared
  junction/V/J alternatives under identical source QC, chain/cell/productivity rules and unit.
  Preserve all subjects, exclusions, features, tests, confidence and randomization settings.
  Refuse equivalent definitions, policy changes and any baseline measurement/inference drift.
- Reuse the validated diversity and subject-comparison engines. Retain missing/failed analyses
  with reasons and full test-family membership. Keep within-definition BH and add BY over
  both tests × features × definitions; marginal intervals remain unadjusted.
- Report direction, effect-size range and interval behavior separately, without a p-threshold
  verdict. Definitions reuse subjects; no pooled effect, equivalence claim, joint confidence
  interval or specification-curve inferential test is implied.
- Show every definition in a common-axis forest in declared order, complete subject/pair
  observations, assignment changes, tests and failures. Preserve methods/figure provenance,
  immutable jobs/replay, provider-aware full exports, remote control and saved reopening.
- Add a 32-subject, long-tailed gamma-Poisson UMI example with uneven depths and explicit
  synonymous junction/gene/allele distinctions. Literal truth verifies every partition and
  count; simulated observed richness reverses direction across definitions. This deliberately
  sensitive construction is not fitted biological evidence or depth-standardized inference.
- Keep version 0.6.0. Do not begin slice 6 or perform release operations in this slice.

Validation includes independently grouped Python/SciPy references for all subject values,
mean effects/intervals, conditional rank tails and BY; missingness/failure/policy/cancellation
controls; exact baseline and source-free replay/export; literal simulator truth and previous
preset byte compatibility; processor agreement; accessible browser workflows and both-theme
visual review.

**Validation completed:** the full application/internal race suite passes (application 65.189 s,
count-model calibration 470.787 s, simulator 73.689 s). The required-data scientific suite
passes in 504.456 s, excluding unchanged performance-budget tests. Final focused race checks
cover provenance/checksum refusals, paired observations, cancellation between definitions,
abundance bounds and incompatible simulation options. The compiler scan finds no fused
multiply-adds. Native/amd64 complete-result and example digests agree; earlier count presets
retain their original bytes. The four-definition example's richness contrast spans −244.125
to +64.125 observed clonotypes, explicitly a descriptive range under unequal observed depth.

All 126 JavaScript tests, Go vet and six CGO-free platform builds pass. The complete browser
workflow passes in both themes, including visible/remote identity, every forest row, undo/redo,
saved reopening, source-free ZIP, figure provenance and saved Methods. Both final screenshots
are visually reviewed. The local site builds 40 pages with 48 screenshot sets; 39 audited pages
pass all 156 theme/viewport checks with no accessibility violations or overflow. All 14
contrast checks, formatting and diff checks pass. Version stays 0.6.0; no commit, push, tag,
release or publication. Slice 5 was subsequently committed and pushed by the maintainer.

### Slice 6: motifs and sequence-signature groups

**Committed and pushed.** R7 and S3 have a bounded TRB implementation;
research §5.35 states the complete scientific contract and GLIPH2 adaptations.

- Capture explicitly selected, disjoint query/background samples under saved AA definition
  and QC. Collapse positive evidence to unique TRBV-gene/full-junction units in each arm;
  retain source hashes, subject/sample/clonotype membership, abundance units and exclusions.
  Match exact V/length strata and preserve unmatched cases explicitly.
- Enumerate overlapping local 2–5-mer windows and global one-position signatures with maximal
  pairwise-positive BLOSUM62 residue sets. Exclude three residues at each terminal; never chain
  positions or incompatible substitutions transitively. These are documented adaptations,
  including stricter local V/length matching, not claims of GLIPH2 implementation equivalence.
- Construct the complete pooled, arm-label-independent signature family before testing. Use
  stable one-sided Fisher tails and log-space BY over all slots; reporting filters never
  shrink the family. Preserve unmatched nulls, undefined zero-background folds and underflow
  log probabilities. The score is −log10(BY), not the GLIPH2 composite score.
- State exchangeable-sequence assumptions and donor, repertoire and ascertainment limitations.
  Abundance and subject annotations are descriptive; no independent-subject p-values, N/P
  boost, HLA test, antigen label or experimental specificity conclusion is supplied.
- Show complete query-member residue frequencies on one fixed 0–100% opacity scale, with
  one-based full-junction positions, exact count tables and explicit denominators. Selection
  explains conserved positions; the display is not an information-content logo.
- Save immutable cancellable jobs, source-free replay/reopening, complete provider-aware ZIPs,
  remote controls, Methods and figure provenance. Revalidate captured arm/subject evidence.
- Provide a twelve-subject example with varied junctions, uneven gamma-Poisson UMI counts,
  known local/global signatures and every source/truth record. It is an artificial positive
  control, not fitted recombination or a power/FDR benchmark. Existing presets remain stable.
- Keep version 0.6.0. Slice 7 and release operations remain outside this change.

Validation includes all 811 independently enumerated signatures with SciPy Fisher/BY values,
exhaustive conditional label allocations, nontransitive policies, abundance/family invariance,
underflow, nulls, bounds, cancellation, source/truth counts, replay, checksums and provider terms.
Native/amd64 result and example digests agree. Browser workflows cover visible/remote identity,
empty-query profiles, complete counts, undo/reopening, source-free remount/export and Methods.
Both-theme screenshots are reviewed; the plot was made more compact so its denominators and
selected evidence remain visible. Existing Networks workflows are regressed in both themes.

**Validation completed:** the full application/internal race suite passes (application
183.224 s; simulator 89.144 s), with final focused API race checks in 151.331 s and final
numeric/simulator/reference race checks passing after pinning the new fixture. The required-data
scientific suite passes in 502.815 s, excluding unchanged performance-budget tests. The compiler
scan finds no fused multiply-adds after correcting progress arithmetic. All 811 independent
reference signatures agree; native/amd64 complete-result and example digests match. Extreme
underflow retains finite log scores on both processors. The 70 exhaustive conditional label
allocations preserve the pooled family and super-uniform raw tails for the checked signature;
this finite sequence-level check is not a biological FDR claim.

All 128 JavaScript tests, Go vet and six CGO-free platform builds pass. Final motif browser
workflows pass in both themes, including background-only unavailable profiles, job updates,
visible/remote identity, source-free remount/export, undo/redo and saved Methods. Final light
and dark screenshots are visually reviewed; scientific notation stays unbroken. Existing
Networks workflows also pass in both themes. The local site builds 41 pages with 49 screenshot
sets; 40 audited pages pass 160 theme/viewport checks without accessibility violations or
overflow. All 14 contrast checks, formatting and diff checks pass. Version stays 0.6.0; no
commit, push, tag, release or publication. Slice 6 was subsequently committed and pushed by the maintainer.

### Slice 7: convergence and HLA association

**Committed and pushed.** S6 is bounded to observed recurrence of
explicitly selected saved TRB sequence groups and conditional subject-level HLA association.
Research §5.36 records the primary sources, implementation choices and scientific limits.

- Pin the saved motif parent and capture selected memberships, receptor/sample identities,
  source hashes, definition/QC, units, method and provider terms. Union both motif arms and
  selected visits by globally named subject. Require an explicit human declaration and exact
  sample assignments; sequences, cells, abundance and repeated samples cannot add replicates.
- Preserve observed group and exact V/full-AA-junction recurrence. No eligible receptor
  evidence means unevaluable, never inferred absence. An empty allele panel gives descriptive
  recurrence only; no convergent-recombination model is claimed.
- Require two explicit resolved copies at each tested locus, including homozygotes. Compare
  at two fields, retaining raw calls. Missing, incomplete, ambiguous, low-resolution and
  expression/group-suffix typing remains unknown. Do not infer typing, phase or heterodimers.
- Use a separate complete typed/receptor-evaluable universe per locus, including subjects
  without the group. Compute stable one-sided hypergeometric tails and BY over all declared
  group × allele slots. Keep constant/undersized tests null with reasons and family membership.
- Show fixed-scale subject fractions with marginal 95% Wilson intervals, exact denominators,
  descriptive differences and complete recurrence. State population structure, relatedness,
  depth, unequal sampling, ascertainment and selection limits. Association is not restriction
  or antigen specificity; original motif scores are not combined.
- Support immutable cancellable jobs, source/parent-free replay, complete provider-aware ZIPs,
  undo/redo, remote/visible identity, cited Methods and figure provenance. Raw subject IDs and
  typing remain explicit in exports.
- Add a 32-subject, 36-sample artificial study with varied junctions, uneven gamma-Poisson UMI
  counts, repeated observations and four unresolved A typings. Save all generating truth;
  the planted 17/20 versus 1/8 association demonstrates behavior, not power or biological FDR.
- Keep version 0.6.0. Slice 8 and release operations remain outside this change.

Validation includes independent SciPy hypergeometric/BY/Wilson values for every one of 12
reference slots, all 70 fixed-margin carrier assignments, repeated-sample/abundance invariance,
homozygosity, unknown typing, bounds, cancellation, source truth, tamper refusals and exact
source/parent-free replay. Browser checks verify every displayed fraction, locus denominator,
visible/remote identity, raw typing, saved reopening, export and cited Methods in both themes.

**Validation completed:** the full application/internal race suite passes (application
71.108 s; simulator 25.187 s), followed by final focused race checks for canonical empty typing,
source/parent-free replay and exports (application 38.036 s). The required-data scientific
suite passes in 198.209 s, excluding unchanged performance-budget tests; its compiler scan
finds no fused multiply-adds. All independent association/interval values agree; native/amd64
complete-result and simulator digests match. Earlier example presets retain their pinned bytes.

All 131 JavaScript tests, final compact-plot checks, Go vet and six CGO-free platform builds
pass. Browser workflows and existing Networks regressions pass in both themes; all four final
screenshots are visually reviewed. The local site builds 42 pages with 50 screenshot sets;
41 audited pages pass all 164 theme/viewport checks with no accessibility violations or
overflow. All 14 contrast checks, Go formatting and diff checks pass. Version stays 0.6.0;
no commit, push, tag, release or publication. Slice 7 was subsequently committed and pushed by the maintainer.

### Slice 8: contamination evidence

**Committed and pushed.** Q4 has a bounded human bulk-TRB evidence
screen. Research §5.37 states the sources, independent implementation and scientific limits.

- Capture explicit human/unrelated declarations, sample/subject/batch assignments, source QC,
  fixed productive NT TRB/V-and-J-gene definition, known integer abundance and fingerprints.
  Keep repeated samples under the same subject; exclude their same-subject pair. Preserve all
  other pairs, including zero-match and unavailable cases, without treating pairs as replicates.
- Match exact standard nucleotide junctions with resolved genes; never collapse synonymous
  sequences. Retained productive TRB abundance supplies every fraction denominator, including
  unevaluable sequence/gene rows. Expose eligible abundance and exclusion reasons. Missing
  counts refuse the screen; zero/no eligible evidence is unavailable, not a clean result.
- Keep every positive match. Apply declared count/fraction/asymmetry review heuristics without
  removing evidence. No p-values, calibrated classifier, automatic filtering or source,
  direction, rate or proof of contamination is claimed. Batch labels are contextual.
- Optionally capture a compatible checked NT Pgen parent, model/checksum and joint-gene setting.
  Separate high-Pgen plausible-publicness context from review status; categories can overlap.
  Missing Pgen is unknown; model zero is not impossibility. Biological sharing and sampling
  limits remain explicit. Other loci and single-cell analyses require separate future contracts.
- Show every selected pair's positive matches on equal log axes, exact denominators and
  complete tables. No pseudocounts or subsampling. Preserve scientific captions, immutable
  replay, source/parent-free reopening, provider-aware exports, remote controls and Methods.
- Provide eight artificial subjects and nine samples with length-selected simulated TRB
  junctions, uneven gamma-Poisson counts, a repeat, planted transfers, assigned shared controls,
  a synonymous control and full truth. Labels are construction assumptions, not Pgen outputs
  or claims of real-world classifier accuracy. Previous example bytes remain unchanged.
- Keep version 0.6.0. Slice 9 and release operations are outside this change.

Validation includes independent rational-arithmetic pair/match references, every simulated
truth intersection and sample denominator, threshold invariance, explicit missingness,
incompatible-model/capture refusals, cancellation, checksums and source/parent-free replay.
Browser checks cover visible/remote identity, exact scatter geometry, saved workflows, full
exports and cited Methods; existing sample QC is regressed in both themes.

**Validation completed:** the full application/internal race suite passes (application
92.719 s; simulator 28.414 s), followed by final focused race checks for evidence capture,
source/parent-free replay and exports (application 5.080 s). The required-data scientific
suite passes in 204.203 s, excluding unchanged performance-budget tests; its compiler scan
finds no fused multiply-adds. Independent rational references agree; native/amd64 complete
result and simulator digests match. Earlier example presets retain their pinned bytes.

All 133 JavaScript tests, Go vet and six CGO-free platform builds pass. Final browser checks
cover saved-result dataset restoration and clearing stale results during source changes,
as well as both-theme contamination and existing sample-QC workflows. All four final
screenshots are visually reviewed. The local site builds 43 pages with 51 screenshot sets;
42 audited pages pass all 168 theme/viewport checks without accessibility violations or
overflow. All 14 contrast checks, Go formatting and diff checks pass. Version stays 0.6.0;
no commit, push, tag, release or publication. Slice 8 was subsequently committed and pushed by the maintainer.

### Slice 9: batch QC

**Committed and pushed.** Q5 reports descriptive sample-level metrics and
within-batch review labels. Research §5.38 records sources, exact universes and scientific limits.

- Capture every source sample, explicit subject/batch assignments and exclusion reasons.
  Require one included observation per subject per batch; do not pool technical repeats.
  Preserve unique-subject and sample denominators, including recurrence across batches.
- Measure six fixed QC metrics with raw numerator/denominator evidence and source QC.
  Bulk fractions use known counts; single-cell fractions use contigs and retained abundance
  deduplicates barcodes. Preserve missing counts/barcodes and zero denominators as unavailable.
  Productive fractions use the status universe before the productive-only QC rule; gene
  fractions include all retained productive weights, including missing-junction rows.
- Use log10 for positive abundance/row counts without pseudocounts; keep fractions on their
  original scale. Compute 0.6745 × deviation/raw MAD within each batch, with strict absolute
  threshold 3.5 and at least five evaluable subjects by default. Small groups and zero MAD
  remain unscored, without epsilon, pooled reference or alternate-scale fallback.
- Show complete sample distributions on a common scale, batch median lines, exact reference
  tables and all exclusions. Vertical offsets have no quantitative meaning. Distinguish
  flags from correction, deletion or biological inference; systematic shifts and masking
  can evade a within-batch reference. No p-values or calibrated error-rate claim.
- Preserve immutable cancellable jobs, source-free replay/reopening, complete provider-aware
  exports, visible/remote identity, undo/redo and cited Methods and figure captions.
- Add 26 artificial samples from 25 subjects and 2427 rows, with uneven depths, a shifted
  distribution, a low-productivity observation, missing counts, small and constant groups,
  explicit repeat/missing-batch exclusions and complete generating truth. Keep prior presets.
- Keep version 0.6.0; slice 10 and release operations remain outside this change.

Validation covers independent median/MAD/score references, every simulated row's contribution,
threshold invariance, same-subject refusal, missingness, barcode/contig accounting, cancellation,
source-free replay, checksums, complete exports, processor agreement and both-theme workflows.

**Validation completed:** application/internal race tests pass (application 136.343 s;
simulator 41.396 s), with final capture/refusal/replay race checks passing in 6.661 s.
The required-data scientific suite passes in 294.004 s, excluding unchanged performance
budgets. Its compiler scan exposed four fused-arithmetic expressions; explicit rounding
resolves them, and a subsequent scan of 3174 functions finds no fused multiply-adds.
Independent references agree and the native/amd64 result and simulator digests are unchanged.
Earlier presets retain their pinned bytes. Canonical assignment order, strict threshold
boundaries, raw denominator limits and source-free replay/export are checked explicitly.

All 136 JavaScript tests, Go vet, six CGO-free platform builds and 14 contrast checks pass.
Batch QC, Sample QC and Contamination browser workflows pass in both themes; all six final
screenshots are visually reviewed. The final browser regression also verifies that switching
from a saved result to another QC tab loads its source assignments without breaking source-free
reopening. The site builds 44 pages with 52 screenshot sets; 43 audited pages pass all 172
theme/viewport checks without accessibility violations or overflow. Formatting and diff checks
pass. Version stays 0.6.0; no commit, push, tag, release or publication. Slice 9 was subsequently committed and pushed by the maintainer.

### Slice 10: pseudobulk count evidence

**Committed and pushed.** M5 foundation. Research §5.39 records primary sources,
exact count and membership rules, the artificial model and limits. Differential expression
remains slice 11; release preparation remains slice 12.

- Select explicit saved phenotype parents with one receptor dataset, definition, QC,
  receptor-size population and inclusive breakpoints. Pin subject, literal timepoint and sample;
  refuse duplicate samples/expression inputs or more than one sample per subject/timepoint.
- Verify expression sources and exact Gene Expression feature IDs/names/modalities/order.
  Preserve sample boundaries and every raw gene count; exclude other modalities. Never pool,
  normalize, average, impute absent features or reuse a map's display/QC subset implicitly.
- Aggregate per sample × supplied type × declared size category with captured cell membership.
  Missing type and unavailable unique clone assignment have disjoint exclusion reasons.
  Sizes count unique receptor cells in the original sample, before type-specific partitioning.
- Preserve every observed type crossed with all categories in every sample. Absent groups have
  null counts; measured zeros remain zero. Low-cell and zero-library groups retain raw evidence
  but are ineligible. Default minimum 10 cells is a configurable evidence rule, not power.
- Keep durable jobs, source-free replay/reopening, complete provider-aware exports, visible and
  remote identity, undo/redo and cited Methods. Coverage panels share a zero-based linear scale;
  bars count cells, not replicates or normalized expression. Samples sort by subject/time/name.
- Add four artificial subjects at two visits, 420 cells/matrix and 24 negative-binomial genes.
  Inverse-rank clone probabilities produce uneven sample-scoped sizes; preserve missing and
  all-zero controls and complete truth/source hashes. Earlier presets retain their identities.
- Bound samples, features, cells, sparse capture and output slots; refuse exact-count overflow.
  Keep version 0.6.0. No differential-expression fit, release or publication in this slice.

Validation includes an independent stdlib integer oracle; every literal count and generating
clone/annotation record of the eight-sample example; missingness, modality and feature identity,
barcode scoping, sample/time boundaries, threshold invariance, overflow, cancellation, source
fingerprints, provider terms, complete exports and replay without sources or parent artifacts.
Native/amd64 digests, application/internal race tests, required-data scientific regressions,
six platform builds, JavaScript and browser checks and both-theme figures are required.

**Validation completed:** the complete application/internal race suite passes (application
267.513 s); final focused capture/reference/simulator race checks pass (application 50.016 s).
The required-data scientific suite passes in 469.651 s, excluding unchanged performance budgets,
including processor and released-project compatibility and the compiler's no-FMA scan. All eight
example matrices agree with independently parsed generating clone/annotation truth for every
raw gene sum and contributing cell count. Canonical request order, relocation-independent
identity, changed-source refusal, provider terms, checksum failures and replay after removing
sources and parent files pass. Exact integer-reference and simulator digests agree on native
arm64 and amd64; existing cell-state and release fixtures retain their pinned identities.

All 139 JavaScript tests, Go vet, six CGO-free platform builds and 14 contrast checks pass.
Pseudobulk, phenotype and AnnData browser workflows pass in both themes, including visible/remote
identity, undo/redo, saved reopening, source-free exports and cited Methods/figures. Final
pseudobulk and refreshed AnnData screenshots are visually reviewed; the capture generates a
fresh study and verifies its model to prevent stale example reuse. The site builds 45 pages
with 53 screenshot sets; 44 audited pages pass all 176 theme/viewport checks with no problems.
Formatting and diff checks pass. Version stays 0.6.0; no commit, push, tag, release or publication.
The maintainer committed and pushed slice 10 as `a90782b`.

### Slice 11: paired pseudobulk differential expression

**Committed** (`d1521e7`). M5. Research §5.40 records sources, explicit
adaptations, estimands, normalization assumptions and limits. Release preparation remains slice 12.

- Consume a checked immutable count parent, one exact supplied type and literal timepoint,
  two declared size categories, and explicit whole-sample exclusions with reasons. Require
  both eligible categories for each included subject; never delete incomplete pairs silently.
- Reuse the independently validated NB2/Cox–Reid engine with a full-rank subject-blocked
  design and comparison contrast. Require 25–32 pairs (24 residual df minimum), not cells
  as replicates. Record every disposition, raw group, design column and contrast.
- Add median-of-ratios size factors from genes positive in every selected observation before
  testing filters; record the reference set and log offsets. Require ten reference genes;
  no pseudocount fallback. Document majority-unchanged/balanced-change assumptions.
- Keep raw count/prevalence filters, all eligible genes in the BH family (failed fits as
  p=1 placeholders, displayed unavailable), conditional marginal Wald intervals and failed
  fits. No moderation, quasi-likelihood, Poisson fallback or edgeR/DESeq2 equivalence claim.
- Save durable jobs with count-parent checksums, source/parent-free replay, complete raw/design/
  gene exports, provider terms, cited Methods, undo/redo and visible/remote identity.
- Show gene abundance/effects, zero-based paired subject counts and a separate conditional
  interval. Color selection, not discoveries. Retain filtered/failed genes and all observations.
- Add distinct 28-subject positive/null artificial expression presets, uneven receptor sizes,
  subject/category variability, low/zero controls and full generating truth/source hashes.
  Preserve earlier simulations. Version remains 0.6.0.

Validation includes the independent 40-gene full-fit oracle and 300-gene null/positive panel,
all generated raw-count truth, family/failure/normalization invariants, exact source-free replay,
processor-identical digests, race/scientific/JavaScript/platform checks, both-theme browser
workflows and visual inspection, complete methods/docs and a local site audit.

**Validation completed:** full application/internal race checks pass (application 262.158 s).
Final focused race checks pass for API replay/exports (231.381 s), core policies/contrast
reversal/cancellation (117.451 s), simulators (4.851 s) and numerical/example controls
(313.863 s). The full required-data scientific suite passes in 285.324 s, excluding unchanged
performance budgets; it includes released-project compatibility and the compiler's no-FMA scan.
The 40-gene oracle agrees on normalization, complete designs, full/null fits, covariance,
likelihoods, effects, conditional intervals and BH. The separate 300-gene panel has 15/267
nominal null rejections, 250/267 conditional intervals covering zero and 30/30 correct positive
directions, disclosed as finite approximation diagnostics. Every selected example raw count
matches generating truth. The positive/null examples each fit 37/38 eligible genes; respectively
four/zero have BH≤.05 for the fixed seeds, not a calibration guarantee.

Complete-result and both simulator digests agree on native arm64 and amd64. Earlier examples
and released-project identities are unchanged. Go vet, six CGO-free platform builds, 143
JavaScript tests and 14 contrast checks pass. Paired expression and count-evidence browser
workflows pass in both themes: visible/remote identity, all gene/subject points and paired
links, failed-fit display, undo/redo, reopening, source/parent-free exports and cited Methods.
Both effect and paired-observation/conditional-interval figures are visually reviewed in both
themes. The local site builds 46 pages with 55 screenshot sets; 45 audited pages pass all 180
theme/viewport checks without accessibility problems or overflow. Formatting/diff checks pass.
No commit, push, tag, release or publication; version remains 0.6.0.

### Slice 12: complete cohort studies and release 0.7.0

**Implemented and validated; awaiting maintainer commit.** E3, O2, N7. Research §5.41 records the
control designs, interpretation and release evidence. Five offline guides use the ordinary
visible actions: independent positive, null, unbalanced 12/20, paired 28-subject and fully
confounded 16/16 studies. Count truth and model identities are retained; no new inferential
method is introduced. Failed design checks remain versioned, exportable and cited, with
analysis selection and host checks excluding them.

The 0.7.0 project contains all five source studies, complete truth, five design checks and
sixteen saved results. Real durable workers replay the four usable design → diversity →
features → comparison/count chains byte for byte. Exact original parent manifests are
restored only after checksum/result verification, preserving provenance in child documents.
Old release fixtures remain unchanged. New manifest files are gzip-compressed in the fixture
because captured raw counts make plain manifests large; their decoded bytes are unchanged.

Application, citation and HTML asset-cache versions are 0.7.0. Release/tag/publication remain
the maintainer's actions after commit, CI and merge. Wave 8 covers clients and MCP.

Validation fixes include the catalog's new total of 66 guided steps, two-minute waits for
pseudobulk test jobs, and an explicit 30-minute CI race-package timeout. The application
race suite now takes longer than Go's default ten minutes; a local full-cache scientific
race run also exceeded a 20-minute package limit. These are harness deadlines, not relaxed
scientific tolerances or performance budgets. A narrow-screen table overflow in the new
guide was fixed before the final site audit.

Release checks: application/internal Go race tests passed (application 625.981 s).
The remaining scientific race cases passed in a separate 670.828 s run after the
full-cache run's package timeout, with no scientific assertion relaxed. The required-data
scientific suite passed in 328.640 s: 74 top-level tests passed, with four optional checks
skipped (live publisher downloads, two maintainer-selected HDF5 checks, and maintainer-only
VDJdb). Native and amd64 fixture replay, designs and new simulator digests agreed. Go vet,
six CGO-free platform builds, 149 JavaScript tests, syntax checks and 14 contrast checks
passed. All 112 browser scene/theme runs passed; 57 screenshot pairs were reviewed, including
the final cohort workflow/refusal recaptures. The site builds 47 pages with 57 screenshot
sets; its 184 desktop/mobile/theme audit checks report no violations or overflow. Shell
installer syntax passed; PowerShell parsing remains a CI check because `pwsh` is unavailable
locally. No old release fixture was changed.

All six performance budgets passed without race instrumentation (147.633 s total): one
million rearrangements read/validated in 1.590 s at 58 bytes/row plus junctions; 100 samples
reopened in 148 ms; a searched million-clonotype page in 38.485 ms; 10,000 neighborhood
centroids against 99,997 background receptors in 2.801 s; one million generation probabilities
in 82.793 s with no generated receptor assigned zero probability; and neighbors of 100,000
paired TCRs in 49.539 s at 1.3 GB. Parallel timings are on ten processors. The homepage's
scale timings were refreshed and the final 47-page site rebuilt locally. No commit, tag,
release or site publication was performed.


## Later waves (outlined)

| Wave | Release | Theme | Main content |
| --- | --- | --- | --- |
| 8 | 0.8.0 | Record and reach | Certificates and `verify`, `run`, reports, MCP, Python and R clients, agent benchmark, archives, de-identification, container file |
| 9 | 0.9.0 | Suite and public data | ADC queries, overlap with public cohorts, Proteoscope and CytoWeave links, MiAIRR checklist, Excel and Prism, likelihood trees, candidates, remaining readers |
| 10 | 1.0.0 | Hardening | Budgets met, accessibility audit, documentation complete, external beta feedback addressed |
