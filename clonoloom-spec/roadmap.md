# ClonoLoom roadmap

The plan for building ClonoLoom. Each **wave** is one release: wave 1 is 0.1.0, wave 2 is 0.2.0,
and so on. Each wave is split into **slices**, bite-sized units of work. A slice ends with:
- something that runs;
- tests or validation that check it;
- its documentation;
- an entry in `CHANGELOG.md`.

Slices are done in order within a wave unless noted. Each slice names the requirements
(`requirements.md`) it serves. When a wave is reached, it is planned in detail: its slices are
confirmed or re-cut, and the decisions taken are recorded here. Waves 1–8 are released.

**Progress** is marked here as each slice is committed: done, next or to come. Each
requirement's status is in `requirements.md`; release changes are in `CHANGELOG.md`.
**Where things stand:** v0.8.0 and its product site are published. Wave 8 merged in PR #8
(`719a73c`); the follow-up AI-agent documentation is committed as `6f71662`. Wave 9 is planned
below; slice 1 is committed and pushed as `e3f8b49`; slice 2 is implemented and validated on `wave9`, awaiting maintainer review and commit.
No 0.9.0 release is made.

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

**Done:** committed as `b4fe1c2`, merged in PR #7 and released as 0.7.0. E3, O2, N7. Research §5.41 records the
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



## 0.8.0: record and reach (wave 8)

The record must distinguish **unchanged saved evidence**, **recomputed results** and
**scientific interpretation**. A digest proves none of the study's assumptions and is not an
authenticity signature. Each verifier reports exactly what it checked, what differs and what
it could not check. Automation uses the same recorded actions and Go computations as the UI.

### Wave 8 sequence

| Slice | Scope and requirements | Acceptance before the next slice |
| --- | --- | --- |
| 1 | Saved-result evidence certificates and offline integrity checks (O5 foundation, O9, X5, N2/N4) | Bounded versioned archive; exact saved manifest/recipe/results and provider NOTICE; external fingerprint pinning; native Go `verify --integrity`; explicit no-recomputation verdict; Reports inspection/export/checking; cohort features/comparisons/counts first; corruption, bounds, traversal, cancellation and provider tests |
| 2 | Scientific certificate replay (O5) | First replay the three cohort kinds through real workers in an isolated temporary store; verify original source/parent bindings where needed; preserve original provenance separately; exact bytes and explicit differs/unavailable outcomes; no missing-input success or tolerance fallback |
| 3 | Headless recipes and broader replay coverage (A6, O5) | Versioned effective recipe, named inputs and output folder, no overwrite by default, cancellation/exit/report contracts; shared Go workers; one study or explicit batch task; publish a tested per-kind support matrix and refuse unsupported kinds rather than silently omitting results |
| 4 | Checkpoints and analysis differences (O7) | Immutable project checkpoints; plain-language changes to sources, definitions, QC, parameters, populations and result identities; exact evidence beneath descriptions; undo/redo and version compatibility |
| 5 | Self-checking HTML reports (O6, O1/O2) | Offline report of explicitly selected results, figures, methods, references and limits; complete denominators and accessible tables; distinguish checksum checks from recomputation; no external resources or unescaped metadata |
| 6 | Automation policy, proposals and typed queries (A2/A3, A1 foundation) | One versioned action catalog with explicit read/edit/compute/file/network effects; no arbitrary SQL/shell; read-only defaults; visible named proposals, acceptance/rejection, undo and audit history; cancellation and stale-project handling |
| 7 | MCP transport (A1, N5) | Stdio JSON-RPC, supported protocol negotiation, tools from the action catalog, explicit capability grants, bounded output/progress/cancellation, logs on stderr; tests against the current official specification and sibling transport patterns |
| 8 | Generated Python client (A4 first part) | Generate from the same catalog; connection discovery and explicit overrides, version negotiation, typed errors, faithful arrays/nulls and exact JSON numbers; optional dataframe helpers; fake-server and real-host tests using uv |
| 9 | Generated R client (A4 completion) | Same connection/action/error contract as Python, generated help and wrappers, explicit vector/list/null handling and precision; package checks, fake-server and shared real-host parity fixtures |
| 10 | Portable project archives (D12, O9, N4) | Project/history/artifact inventories, safe bounded extraction, portable path rebinding by fingerprints; source inclusion only when permitted; explicit omitted/restricted/missing sources and complete cross-release reopening |
| 11 | Export de-identification (O10) | Explicit subject pseudonyms, requested free-text/HLA removal, provenance of transformations and separate identity map; do not mutate source projects or imply anonymization; no hidden identifiers in manifests, plots or nested evidence |
| 12 | Parquet exports (O3) | Explicit schemas, units, definition/QC/provenance, exact integer counts and nulls; deterministic bounded writer with independent reader checks; provider notices and export-policy integration |
| 13 | Agent benchmark (A5) | Known-truth tasks over the visible/MCP/client contracts, code-based grading, proposal/refusal and error cases, reproducible local runner; separate task success from scientific validity |
| 14 | Container and cluster execution (P6, A6) | Small headless build, non-root execution, explicit read-only inputs/output mounts, signal handling, no browser requirement for Go computation, offline and reproducible example jobs |
| 15 | Complete record-and-reach study and release 0.8.0 | Worked cross-interface study, support matrix and all limits reviewed; full scientific/platform/accessibility/performance validation; new compatibility fixture and reviewed screenshots; versions changed only here; maintainer commits, merges, tags and publishes |

Later slices may be re-cut at their boundary when evidence warrants a smaller increment;
acceptance requirements and changes to this sequence are recorded here before implementation.
No source from incompatible-licensed implementations is used.

### Sister-project patterns and decisions

- Inspected Apache-2.0 CytoWeave 0.8.1 (`19d47bb`) and MaveScape 0.8.0 (`c997e39`) locally: `mcp.go`, `verify.go`, client
  transports and `clients/generate.mjs`. Preserve one action catalog, versioned API contracts,
  generated wrappers/help, `remote.json` discovery, explicit connection overrides, structured
  errors, stdout reserved for MCP, cancellation and visible named proposals. Generated-client
  drift checks belong in CI; host settings remain opt-in rather than inferred from clients.
- The siblings drive browser computations for verification. ClonoLoom deliberately uses its
  existing Go workers directly, without introducing a browser requirement or a second scientific
  implementation. Bit-for-bit arithmetic remains mandatory; no approximate-equality success. The current
  official MCP transport specification (2026-07-28; research §5.42) differs from the siblings'
  older handshake model; slice 7 rechecks protocol requirements and explicit compatibility.
- Slice 1's certificate is a single-result `.certificate.clz`, separate from the later portable
  project archive. Its scope is `recorded-artifact-bytes`. It contains the immutable saved
  manifest, recipe, outputs and provider notice; recorded original-source/parent fingerprints
  are evidence, not checks performed. A separate fingerprint can be pinned with `--expect`.
- Begin with the three cohort result kinds that already retain complete source-free evidence
  and provider notices. Other kinds are refused explicitly until their contracts are mapped.
  The first slice does not claim O5 complete and never presents integrity as scientific replay.
- Verify ZIP members in place with strict inventory, size, duplicate/path and checksum checks;
  extract nothing and never follow a recipe's source paths. Unsupported versions fail closed.
  Use existing restricted-provider confirmation and mark captured raw evidence as data.
- Keep the application version at 0.7.0 throughout development. Wave 8 release metadata and
  published-site updates wait for slice 15 and the maintainer's normal release sequence.

### Slice 1: saved-result evidence certificates

**Committed and pushed as `b459f61`.** O5 foundation, O9, X5, N2/N4.
Reports prepares and inspects cohort feature/comparison/count certificates. A browser-upload
check and `clonoloom verify --integrity` use the same Go verifier, return explicit inventory
findings, and never claim source verification, authenticity, recomputation or scientific validity.
The CLI writes an optional verification report only to a new path. Provider restrictions remain
visible even after the source dataset and parent artifacts are absent. The archive retains the
exact original bytes; neither source data nor earlier release fixtures are modified.

Validation:

- Application and internal-package race tests passed (application 373.549 s), including
  earlier-release compatibility. Final focused race tests cover certificate export/checking,
  source-free CLI/API agreement, provider terms, command usage, configuration and action parity.
- Native arm64 and amd64 checks agree on the pinned certificate fingerprint and source-free
  API/CLI outcomes. An independent Python-standard-library ZIP/JSON/SHA-256 oracle, run with
  `uv`, checks original numeric/null/zero tokens and recipe bytes. Thirty seconds of fuzzing
  completed 1,437 executions without failure. Regression cases cover corruption, duplicate
  keys/members, unknown versions, traversal, links, cancellation, changed inputs, prefixed path
  lengths and the separate inventory/metadata size bounds.
- Targeted cohort and count-model scientific references plus the no-fused-operations check
  passed (4.574 s). This slice adds no estimator or statistical plot; it preserves saved
  evidence without numeric conversion. The complete scientific and performance suites remain
  part of release validation, rather than being claimed by these targeted checks.
- Go vet and six CGO-free platform builds passed. All browser-module syntax checks and
  151 JavaScript tests passed, alongside 14 contrast/color-vision checks.
- Both-theme browser workflows check visible/remote export identity, uploaded integrity,
  wrong fingerprints, truncated archives and saved-project reopening. Both new screenshots
  were reviewed. The site builds 48 pages with 58 screenshots; the final accessibility/layout
  audit passed 188 checks (47 pages, two themes, two widths), after correcting a phone-width
  command-example overflow with the existing wrapping code-block style.

Keep version 0.7.0. Scientific replay is slice 2; no certificate in this slice claims that
sources or parents were independently checked, that statistics were recomputed, or that a
checksum establishes authenticity or biological validity. Nothing is committed, tagged,
released or published by the implementation workflow.


### Slice 2: exact scientific replay from captured cohort evidence

**Committed and pushed as `62b088b`.** O5, N2/N4/N7.

- Reuse the existing feature, comparison and count-model job handlers in a disposable
  native-Go artifact store. Integrity verification is a prerequisite, never a substitute.
- Accept the existing version-1 certificates without rewriting them. Verify the embedded
  parent result's exact bytes against both its own manifest and the recorded recipe binding.
  Replay feature parents before dependent comparisons/count models, then restore their
  verified original manifests inside the disposable store so new timestamps/job IDs do not
  change downstream scientific bytes. Never install the expected target output before running
  its worker. Check the recipe identity and scientific manifest metadata as well as every
  output byte; no numeric tolerance, rounding or missing-input fallback.
- The explicit computation boundary is **captured cohort evidence**: checked design and
  subject features from embedded diversity measurements; comparison tests/effects from those
  features; negative-binomial refitting from the captured count matrix. Original receptor files,
  source-file fingerprints and upstream diversity fits are not independently verified or
  recomputed. Their recorded provenance is retained, not relabeled as new verification.
- `verify --replay` and Reports upload use the same host path. Keep `--integrity` explicit and
  backward compatible. Report `replay-match`, `differs` or `unavailable`, with per-step hashes,
  scientific-metadata comparison, original manifest and current verifier software/platform.
  Exit 0 only for the requested match, 1 for a difference/invalid archive and 2 for unavailable
  replay or operational failure. Report files never overwrite existing paths.
- Unknown kinds/method versions, incomplete captured inputs or failed worker checks cannot
  succeed. Only allowlisted workers run; no saved source path, shell command or network resource
  is opened. Recheck input bytes when reading them after integrity verification. Propagate
  cancellation and remove the temporary store on success, failure or interruption.
- Keep scientific validity distinct from computational agreement: matching output does not
  establish model calibration, sound design, authorship, replication with new data or biology.
  Cite NASEM (2019) alongside the unchanged method references in research §5.43.
- Validate every supported result from the four 0.7.0 cohort controls, including positive/null,
  unbalanced and paired designs, on native arm64 and amd64. Add independent evidence/report
  digest checks, one-bit output changes, parent/metadata/recipe refusal, cancellation and
  cleanup controls. Exercise both UI modes and both themes and review the updated screenshots.

Validation completed:

- Application/internal race suite passed with the repository's thirty-minute timeout
  (application 1,038.178 s). Final targeted race checks passed (32.538 s), covering later
  verdict refinements, changed parent manifests, duplicate recipe keys, unsupported inputs,
  cancellation, cleanup, CLI usage, fail-fast report-path refusal and action parity.
- All twelve release cohort controls reproduced exact output bytes and scientific metadata
  on native arm64 and amd64. The amd64 replay/byte checks passed in 56.128 s. CLI/API/native
  reports agree; original manifests remain unchanged. Original release fixtures were not edited.
- The independent Python ZIP/report oracle, run through `uv`, checked all twelve archives,
  original provenance and step digests. It preserves raw nested JSON tokens rather than
  re-encoding floats. Targeted cohort/count-model reference and no-fused-operations checks
  passed (3.391 s); existing scientific estimators are unchanged.
- Final verdict controls passed (11.583 s), including one-bit effect differences, scientific
  metadata changes, missing captured data, altered parent bindings and worker refusal after
  a completed parent step. During validation a duplicate-key test initially assumed compact
  fixture JSON and made no change; the corrected control prepends the duplicate regardless
  of indentation and passes. A redundant focused race run hit Go's default ten-minute timeout;
  the successful full run and final targeted checks use the existing thirty-minute allowance.
- Archive/evidence fuzzing completed 113,515 executions in thirty seconds without failure.
  Direct tests cover input changes after integrity checking, caller read bounds and cancellation.
- Go vet, formatting, browser syntax, six CGO-free builds, 152 JavaScript tests and 14
  contrast/color-vision checks passed. Both-theme browser workflows cover visible mode/file
  controls, exact replay, cancellation without a verdict, corrupt input, saved reopening and
  exact report downloads, including an integer beyond JavaScript's exact range. Both updated
  screenshots were reviewed; no statistical plot or estimator is added.
- The site builds 48 pages with 58 screenshots. Its final audit passed 188 checks across
  47 pages, two themes and two widths. The complete scientific/performance release suite is
  not claimed by this slice's targeted reference checks.

Keep version 0.7.0. Broader result support and headless recipes remain slice 3; O5 remains partial.
No commit, tag, release or publication is performed by this slice's implementation workflow.


### Slice 3: headless recipes and broader replay

**Committed and pushed as `59b8868`.** A6, O5, O9, N2/N4/N7.

- Add native `clonoloom run --recipe --input NAME=PATH --out NEW_DIR`, following the
  compatible CytoWeave/MaveScape named-input, private execution and run-record patterns;
  no browser, listening port, shell, arbitrary action or network dependency.
- Recipe v1 selects one study/task. Multiple tasks require `--task ID`; never pool them or
  infer scheduler/environment selection. Each selected input declares a logical name,
  filename, size and SHA-256. Snapshot exact regular-file bytes before reading them and
  refuse missing/extra/changed bindings. Preserve explicit input ordering and metadata.
- Start source computation with batch QC and motif screens through their existing capture
  functions and workers. Accept explicitly listed AIRR TSV (optionally gzip) and AIRR
  Repertoire metadata, not implicit directory traversal or unmapped vendor formats.
  This is a bounded initial headless interface, not a claim that every UI action is portable.
- Add certificate export/replay for batch QC and motifs, keeping the three cohort kinds.
  Batch replay recomputes descriptive scores from captured sample QC; motif replay recomputes
  the existing GLIPH2-style ClonoLoom adaptation from captured sequence units. No source
  recapture or upstream GLIPH2 equivalence is implied. Keep kind-specific limits and scopes.
- Preserve certificate v1 and old cohort report scopes. Compare exact result bytes and
  scientific metadata through the same replay worker path. A changed, unsupported or
  incomplete result is never a match. Publish a tested per-kind matrix for all registered
  workers; unsupported kinds are visible, not silently omitted or treated as resumable replay.
- Effective recipes record normalized parameters and the resolved clock. A fixed recipe time
  pins notice, artifact and job provenance for repeatable output bytes; absent time is resolved
  once and written into the effective recipe. Keep actual input snapshots private and temporary.
- Output directories must be new. Export complete JSON results, original methods/provenance,
  NOTICE, certificate, validation findings, effective recipe and a checksum inventory in
  `run.json`; replay tasks write their full replay report. Partial/failed/cancelled runs never
  get success status. Retain failure diagnostics, not a misleading success marker.
- Preserve known provider terms plus explicitly supplied source credits. Restricted source
  diagnostics/evidence require the existing explicit private-study acknowledgment, represented
  by `--accept-terms`. Replay of an already supplied certificate retains the existing verifier
  contract. Do not claim de-identification or relax method assumptions.
- Test application/headless capture and output identity, input relocation, fixed-time repeat
  runs, explicit batch selection, hashes/limits/unknown fields, provider terms, cancellation,
  no overwrite, numeric tokens and exact replay for both new kinds. Reuse the realistic
  artificial batch-QC and motif studies and review both-theme certificate workflows.

Validation completed:

- Full application/internal race suite passed with the thirty-minute allowance (application
  594.896 s). Final targeted race controls passed in 69.151 s after the manifest-byte fix.
  Earlier targeted checks passed in 80.186 s. No existing release fixtures were changed.
- Both realistic artificial source studies match application capture and worker output bytes.
  Relocated input bindings and fixed effective-recipe reruns preserve all six inventoried
  outputs. Scientific result digests are pinned on arm64 and amd64; the amd64 final controls
  passed in 18.724 s. Source/certificate provider gates, explicit task selection, malformed
  inputs, changed hashes/results, unsupported methods, cancellation and no-overwrite pass.
- The documented recipe helper was exercised with freshly generated batch-QC and motif studies.
  Both CLI computations and native certificate replays pass. The independent Python ZIP/hash
  oracle, run through `uv`, checks six outputs per run, archived bytes and replay digests.
  It caught a re-encoded standalone manifest; exports now preserve the original manifest bytes,
  with an added regression test and successful independent checks for both studies.
- Existing independent batch-QC/motif method references and the no-fused-operations check
  passed (2.746 s). No estimator, inference method, plotting geometry or release fixture was
  changed. This slice does not claim the complete scientific/performance release suite.
- Recipe parser fuzzing completed 742,638 executions in thirty seconds without failure.
  Go vet, gofmt, browser/helper syntax and six CGO-free platform builds pass. All 152 JavaScript
  tests and 14 contrast/color-vision checks pass.
- The existing cohort and both new certificate workflows pass in light and dark themes;
  all six screenshots were reviewed. The underlying batch/motif scenes still check source
  units, planted controls, missingness/exclusions, figure geometry, methods and source-free
  saved reopening. A capture-script fingerprint lookup initially addressed the response wrapper;
  using its certificate field corrected the harness, and all new scenes then passed.
- Site build passes with 49 pages and 60 screenshot references. The audit passes 192 checks
  across 48 pages, two themes and two widths. Documentation describes the source/replay
  distinction, fixed provenance time, output-status/exit contracts, forced-kill cleanup limits
  and the narrower recipe input bounds.

Keep version 0.7.0. A6/O5 remain partial outside the published matrix. Checkpoints are slice 4;
additional source/parent contracts must be mapped explicitly before expanding support.


### Slice 4: checkpoints and analysis differences

**Committed and pushed as `1874fc7`.** O7, O9, N2/N4/N7.

- Add named immutable local checkpoints in Reports, using the compatible CytoWeave pattern
  of snapshot, compare and undoable recovery. Preserve raw project JSON and every inventoried
  file of each directly attached completed local result; include its original manifest and
  captured notices. Publish staged, synced files only after all input checks succeed.
- Record a versioned SHA-256 inventory with a content-derived checkpoint ID. Preserve source
  paths and fingerprints as records; never open or copy original receptor/expression sources.
  This is local recovery evidence, not a portable project/archive or a running-job checkpoint.
- Bound requests/project evidence to 16 MiB, 128 attached results, 256 inventory files,
  existing certificate member bounds and 512 MiB total. Refuse unavailable, changed, incomplete,
  external-output or unsupported-version evidence rather than omit it. Separate unattached
  parents/reference installations remain outside recovery; supported replay uses its existing
  captured-input contracts.
- Compare a checkpoint against the current analysis or another checkpoint of the same project.
  Go reports changes to source records, definitions, QC, sample annotations/populations, cohort
  designs, parameters, result identities and saved manifest/file fingerprints. Exact raw JSON
  evidence underlies each description, retaining large integers, missing versus null, and array
  order. Do not infer effect sizes, significance or biological consequences from configuration.
- Recovery preflights all existing result destinations, refuses conflicts without overwrite,
  and reinstalls missing result directories with original bytes. Retry is idempotent. Commit
  analysis values through the existing browser undo store only after recovery succeeds; keep
  current identity/name, history and checkpoint references. Invalidate pending analysis callbacks
  on recovery and undo/redo. Refuse browser numeric-token loss and stale-project/edit responses.
- Checkpoint creation/reference changes and recovery are undoable; immutable evidence remains
  in the local library when a reference is undone. Interrupted recovery can leave verified
  unreferenced results, never a falsely completed project restoration. A force-kill/power loss
  is not a transactional whole-library backup guarantee.
- Keep existing certificate format, replay scopes, scientific workers and version 0.7.0.
  Check recovered certificate fingerprints and exact replay against the original. Exercise
  malformed/corrupt/version/conflict/cancellation controls, old-project compatibility,
  independent inventory checks and both-theme artificial-study workflows.

Validation completed:

- Full application/internal race suite passed with the thirty-minute allowance (application
  581.904 s). Final checkpoint race controls passed in 2.282 s after the metadata-size guard.
  No scientific worker, estimator, plotting geometry or existing release fixture changed.
- Deleted-result recovery preserves every original manifest/output byte and the original
  certificate fingerprint, then passes exact native scientific replay. Retry is idempotent.
  The independent Python inventory/hash oracle, run through `uv`, confirms checkpoint identity,
  all three fixture files and restored replay digests. amd64 recovery/replay controls passed
  in 1.471 s alongside native arm64 validation.
- Corrupt files, conflicting destinations, incomplete/external evidence, future versions,
  cross-project requests, cancellation and stale edits are refused. Preflight checks all
  destinations before installing any missing result. Old release project envelopes still parse;
  minimal version-1 envelopes retain existing core-list defaults without changing saved bytes.
  Large numeric tokens, missing/null and array order remain distinct. An exact-limit regression
  verifies 4096 differences followed by unchanged fields succeeds and 4097 refuses truncation.
- Parser fuzzing completed 1,099,342 executions in thirty seconds without failure. Go vet,
  gofmt, browser/helper syntax and six CGO-free platform builds pass. All 155 JavaScript tests
  and 14 contrast/color-vision checks pass. Independent cohort comparison/feature, batch-QC
  and no-fused-operations controls pass (3.417 s); this is not the complete release suite.
- Both-theme checkpoint workflows pass on the existing realistic artificial batch-QC study:
  changed definition/QC/population records, deleted-result recovery, undo/redo, saved reopening,
  unchanged certificate identity, exact replay, stale-edit refusal and cancellation. Both new
  screenshots were reviewed. The initial capture assumed optional sample annotations existed;
  its fixture now supplies those records explicitly. Existing cohort, batch-QC and motif
  certificate workflows also pass in both themes; their unchanged screenshots are retained.
- Site build passes with 50 pages and 61 screenshot references. The audit passes 196 checks
  across 49 pages, two themes and two widths. Guides distinguish exact inventoried bytes from
  recovered analysis values, and document local-only scope, numeric refusal, undo semantics,
  interruption limits and unchanged scientific replay support. Version remains 0.7.0.

Slice 5 remains self-checking reports. Portable transfer of checkpoint evidence belongs to the
explicit project-archive contract in slice 10; no commit, tag, release or publication here.

### Slice 5: self-checking HTML reports

**Implemented and validated; awaiting the user's commit.** O6, O1/O2, O9, N2/N4/N7.

- Export one offline HTML file for one to eight explicitly selected saved results from the
  five existing certificate kinds. Reuse original result/manifest bytes, provider gates,
  notices, methods and references; do not open source paths or imply project-wide coverage.
- Go renders complete exact-value tables and recorded evidence without float conversion.
  Embed native replayable certificates and result JSON downloads. Bound exports and refuse
  unsupported, changed, malformed or oversized evidence rather than silently drop records.
- Reuse existing batch-QC, cohort comparison and count-model figure geometry. Show all
  applicable plots within declared bounds, never select discoveries or compute new statistics.
  Feature tables and motif screens retain complete tables; no new visualization method here.
- A standalone browser checker verifies the exact report-content bytes and embedded download
  bytes before displaying them. Allow an independently retained report fingerprint. No network
  resources, source paths, external fonts or unescaped metadata. State unsupported/failed checks
  explicitly; checksums are not scientific replay, signatures or biological validation.
- Preserve exact numeric tokens in tables/downloads; browser numbers are used only for existing
  figure geometry, with explicit refusal of unsafe integer geometry. Carry full denominators,
  missing/exclusion reasons, correction families, methods and kind-specific scientific limits.
- Exercise offline checks, tampered evidence/content, independently pinned fingerprints,
  source-free reopening, provider gates, stale edits/cancellation, exact certificate replay,
  hostile labels, complete tables and existing figure geometry. Review both-theme screenshots.

Validation completed:

- Full application/internal race suite passed with the thirty-minute allowance (application
  715.737 s). Final targeted report/action race controls passed in 136.523 s; amd64 controls
  passed in 19.254 s. Exact original result/certificate bytes and native replay pass for all
  five kinds, including the full release count-model result with about 19,000 clonotypes.
  Existing release fixtures, scientific workers and estimator/plotting modules are unchanged.
- Repeated exports with fixed selection/order, title, evidence and build are byte-identical.
  Multi-result selection stays explicit. Provider gates, cancellation, stale edits, corrupt
  files, unsupported requests, numeric tokens, missing/null and table limits are checked.
  The full large motif study exceeds the bounded report contract; native certificates remain
  available. A smaller captured artificial motif fixture validates its report/replay path.
- The independent Python oracle, run through `uv`, checks five reports' inventories, exact
  result bytes, ZIP members, certificate fingerprints and native replay digests. The offline
  browser harness opens all five as local files, with no server or network requests, and
  verifies figures, independent fingerprints, changed evidence, altered geometry/styles and
  hidden failed content. Final regenerated exports passed in 5.598 s.
- Table-parser fuzzing completed 506,908 executions in thirty seconds without failure.
  Go vet, gofmt, browser/helper syntax and six CGO-free platform builds pass. All 158 JavaScript
  tests and 14 contrast/color-vision checks pass. Independent cohort feature/comparison,
  batch-QC/motif and no-fused-operations controls pass (7.369 s), plus the independent
  statsmodels count-model reference (0.274 s). This is not the complete scientific release suite.
- Application export and offline-figure workflows pass in both themes, including accessibility,
  visible remote selection/title, cancellation and stale-response refusal. All four screenshots
  were reviewed. A capture assertion initially counted all included samples rather than only
  available metric values; it now checks every plotted sample and coordinate against recorded
  evidence. Missing values remain in the complete tables. Initial export bounds were adjusted
  to retain full release count-model rows while avoiding duplicate large captured recipes.
- Site build passes with 51 pages and 63 screenshot references. The audit passes 200 checks
  across 50 pages, two themes and two widths. Guides document numeric precision, geometry
  coverage/unavailable states, all bounds, original recipe downloads, provider restrictions,
  the trusted-checker boundary and the distinction from scientific replay and validity.

Keep version 0.7.0. Broader result contracts and portable project archives remain separate;
automation policy is slice 6. No commit, tag, release or publication in this slice.

### Slice 6: automation policy, proposals and typed queries

**Committed and pushed by the maintainer (`1b38572`).** A2/A3 and the A1 foundation. API 2 changes automation from permissive
remote control to read-only by default. One catalog declares conservative effect bounds;
`--allow edits,compute,files,network` grants only the named capabilities. Non-read calls
also require the local connection token. Grants never imply acceptance of a project edit.
The window reviews named proposals against the exact current project; rejection, cancellation
and stale proposals cannot execute. Accepted changes use normal Undo and a bounded session
audit records requests and outcomes, including refusals. Cancellation is best effort for
already-started file operations and workers; it cannot erase downloaded bytes or completed
side effects. No automatic retry or rollback of scientific files.

A bounded typed clonotype filter tree adds conjunction/disjunction/negation and declared
numeric/string predicates, stable pagination and full-population denominators. There is no
SQL, shell, expression evaluation, inferred missing value or statistical inference. Raw native
JSON is retained for precise client decoding. Existing certificate replay and its five-kind
support matrix remain unchanged; automation does not introduce another scientific engine.
MCP and generated clients remain slices 7–9. Version stays 0.7.0.

Validation completed:

- Full application/internal race suite passed (application 1000.407 s, within the existing
  thirty-minute limit). Final focused policy/query race checks passed in 2.034 s. The first
  full run caught a new test fixture that collapsed to one productive clonotype; a second
  productive identity supplies a literal 8/12 and 4/12 abundance oracle. No scientific
  tolerance, method or existing fixture was changed.
- Native query controls retain QC rules, full-table denominators, definition and source unit,
  stable pagination, case-sensitive and multi-chain semantics. Strict schemas, duplicate keys,
  numeric precision bounds, no SQL/shell, unknown fields, depth/node limits and bounded audit
  are tested. Final decoder fuzzing passed 113,557 executions in twenty seconds.
- amd64 policy/query and five-kind HTML-certificate replay controls passed in 15.066 s.
  Existing native worker and certificate replay cases pass in the full race suite. This is
  not the complete scientific release-validation campaign, which remains slice 15.
- 163 JavaScript tests and 14 contrast checks pass. Both-theme browser workflows pass for
  proposals/audit, reports and checkpoint recovery. They exercise existing-job refusal,
  explicit acceptance/rejection, stale proposals, cancellation before and after acceptance,
  blocked late writes, actor history, Undo/Redo and preservation of read-only/UI-only views.
  Four new screenshots were reviewed. Initial browser checks caught lifecycle invalidation
  and modal/drawer accessibility issues; these were corrected before the final runs.
- Go vet, formatting, JavaScript syntax and six CGO-free platform builds pass. Site build
  produces 52 pages and 65 screenshot references; its 204 desktop/mobile/theme audit checks
  report no violations or overflow. Existing release fixtures and scientific workers remain
  unchanged. Capture/fixture drivers explicitly grant capabilities, wait for existing work
  and click visible acceptance controls; they never auto-retry failed edits.

Audit is bounded to the last 500 host-session events, not persistent forensic history.
Cancellation cannot undo already written/downloaded bytes; project Undo remains bounded to
300 steps. Non-read automation refuses existing jobs/imports. Read-only query leases reject
project writes without resetting the active scientific view. No commit, push, tag, release or
site publication was performed. Slice 7 (MCP transport) is next after maintainer review.

### Slice 7: MCP transport

**Committed and pushed by the maintainer (`315f398`).** A1/N5. `clonoloom mcp` follows
Apache-2.0 CytoWeave/MaveScape's stdio subprocess, normal local host, lazy window and shared
catalog patterns. MCP 2026-07-28 uses per-request metadata/discovery; 2025-11-25 has explicit
initialization compatibility. Modern calls never inherit an earlier client's identity or
capabilities. stdout is exclusively JSON-RPC; diagnostics/help/version use stderr.

HTTP and MCP share grants, argument validation, window dispatch and bounded audit. Tool lists
include only actions allowed by the fixed launch grants; hidden actions cannot bypass them.
Edits retain human proposal review, stale-state refusal, caller-labelled History and Undo/Redo.
There is no approval tool, arbitrary SQL/shell endpoint, new estimator or replay engine.

Bound frames (2 MiB), arguments (1 MiB), output (16 MiB), active calls (16) and session IDs
(65,536). Preserve native numeric tokens in text and structured results; refuse oversized
responses explicitly rather than truncating scientific evidence. Progress is elapsed waiting
time, not percent complete. Cancellation and EOF cancel pending calls without promising rollback
of completed files or artifacts. Use a dedicated library; known live hosts are refused, with
best-effort detection rather than a global process lock. Protocol choices and sources are in
research §5.48 and `docs/MCP.md`. Version remains 0.7.0.

Validation completed:

- Full application/internal race suite passed (application 863.599 s). Final focused MCP,
  shared-policy and HTTP race checks passed in 2.880 s. Cases cover modern/legacy negotiation,
  metadata isolation, strict framing/IDs, grants before window opening, catalog/review parity,
  progress/cancellation/EOF, bounded concurrency/output and live-library refusal.
- Decoder fuzzing passed 297,319 executions in twenty seconds. Native JSON transport retains
  integer/decimal tokens beyond ordinary double precision; oversized output returns an error
  rather than a partial success. Scientific input schemas and workers are unchanged.
- amd64 MCP/policy/query and five-kind HTML-certificate exact-replay controls passed in
  14.641 s. Existing native worker and certificate controls passed in the full race suite.
  Full scientific release calibration remains slice 15; this slice changes no replay limits.
- Real stdio subprocess/browser checks pass for both protocol modes, explicit grants, human
  rejection/acceptance, cancellation, actor History, Undo/Redo and pending-proposal EOF cleanup.
  HTTP and MCP return identical native query strings for the same simulated repertoires.
  Filtered rows retain abundance, full-table frequencies and definition; displayed rank changes
  with membership. The initial test incorrectly expected rank to stay fixed; its oracle was
  corrected without changing scientific behavior.
- 163 JavaScript tests, 14 contrast checks and both-theme browser accessibility checks pass.
  Four new screenshots were reviewed, with simulated data and no invented biological inference.
  The audit workflow exposed missing clonotype grid cell/header roles; these were repaired.
- Go vet, formatting, syntax checks and all six CGO-free platform builds pass. The site builds
  53 pages with 67 screenshot references. All 208 checks across 52 pages, two themes and two
  widths pass without accessibility violations or overflow. No site publication was performed.

No commit, push, tag or release was performed. Slice 8 (Python client) follows maintainer review.

### Slice 8: generated Python client

**Committed as `f086392` and pushed by the maintainer.** A4 first part. Follow the Apache-2.0
CytoWeave/MaveScape package layout, generated methods/help, remote.json discovery, standard-library
HTTP transport and optional pandas helpers. Generate all 58 methods from a Go-checked catalog
snapshot; CI checks both snapshot and generated-file drift. Python 3.10 is the minimum supported
version. Package and application version remain 0.7.0; the published host predates this contract.

Check action API 2, catalog 1 and permission policy 1 before every action. Keep explicit URL,
token and library overrides separate; no discovered token goes to an unrelated explicit URL.
Disable proxies/redirects and require loopback HTTP. Preserve authoritative host grants and human
proposal review. Typed errors retain status/action/code and ambiguous completion; never retry,
launch a host, expand a grant or accept a proposal automatically.

Decode numbers as int/Decimal and retain raw HTTP bytes and native resultJSON. Encode finite
Decimal values as number tokens without float conversion. Distinguish omitted UNSET from explicit
None/null, arrays and empty containers. Optional pandas object columns preserve original values;
missing record keys become None only in that convenience view. Bound inputs/responses/nesting,
reject malformed and non-finite evidence, and document Python's numeric representation limits.
No scientific worker, plotting method, replay tolerance or replay kind changes. Original
certificate bytes remain authoritative. Sources and choices are in research §5.49.

Validation completed:

- Full application/internal race suite passed (application 789.063 s). Native scientific worker
  and exact replay controls remain unchanged. amd64 catalog and five-kind HTML-certificate replay
  controls passed in 26.735 s. Full scientific release calibration remains slice 15.
- Seventeen Python tests pass on both Python 3.10 and 3.13 with pandas, covering all generated
  methods, precision, omission/null/arrays, object frames, discovery, versions, typed failures,
  bounds, malformed responses, redirects, timeout and no retry. One thousand seeded exact-value
  round trips compare the encoder to the standard JSON decoder with Decimal. The standard-library
  run passes with only the optional pandas check skipped. Disabled Decimal traps cannot turn
  invalid exponents into accepted NaN evidence.
- Real host/window validation generates a seeded vaccine study (five artificial subjects,
  thirty samples) in a temporary directory and opens three observations of one subject. Python
  and direct HTTP return identical native query strings; filtered rows retain scientific values,
  definition and full-table frequencies while displayed rank changes. Token/grant/null/unsafe-
  number refusals, human rejection, timeout cancellation, named History and Undo/Redo pass.
  The documented query example executes against the same host; the test driver alone clicks
  proposal controls. No public data download or user library is involved.
- Wheel and source distributions build with uv and install in isolated environments. Installed
  package checks verify version, generated methods and exact values. CI adds Python 3.10/3.13,
  catalog/generator drift, fake-server, wheel-install and real-host checks. No package publication.
- Go vet, formatting, JavaScript syntax, all six CGO-free platform builds, 163 JavaScript tests
  and 14 contrast checks pass. The site builds 54 pages with 67 existing screenshot references;
  all 212 checks across 53 pages, two themes and two widths pass without accessibility violations
  or overflow. No application visual or scientific plot changes; no new screenshots are needed.

No commit, push, tag, release or publication was performed. Slice 9 (generated R client) follows
maintainer review; Python and R will share the action/permission/error contract and parity cases.

### Slice 9: generated R client

**Committed as `a7efd18` and pushed by the maintainer.** Complete A4 using the Apache-2.0 CytoWeave/MaveScape
R package pattern: generated `cl_*` functions, Rd help, S3 connections/results, curl transport
and remote.json discovery. Share Python's catalog/API/policy, discovery precedence, explicit
token isolation, typed failures, bounds, no retries and authoritative host grants/window review.

Preserve JSON number tokens in a dedicated character-backed class rather than converting them
to R doubles. Retain raw response bytes and native resultJSON. Distinguish omission, explicit
NULL, empty objects and arrays, including singleton vectors in array parameters. Data frames
use list columns preserving values; any lossy numeric conversion must be explicit and warned.
No scientific computation, replay tolerance, new plot or inferred permission belongs here.

Generate all functions/help and check drift. Validate numeric tokens, malformed/bounded JSON,
vectors/nulls, discovery and refusal cases with fake HTTP fixtures and seeded parity cases.
Build/install/check the R package, run Python regressions, and extend the isolated real-host
driver to compare native R/Python/HTTP evidence and reviewed edits. Add CI and documented runnable
examples; check the site in both themes. Keep version 0.7.0; portable archives remain slice 10.

Validation completed:

- All 58 R functions and Rd pages generate from the same Go-checked snapshot as Python;
  generation drift, obsolete help, names and package version checks pass. The existing Python
  generated files are unchanged. R CMD build/install/check passes with no errors, warnings or
  notes on R 4.6.1 (curl 8.0.0, jsonlite 2.0.0); the Apache license and adaptation notice ship
  in the package. CI adds release and oldrel-1 R checks; those remote matrix runs follow push.
- Seven R test groups pass all 1,290 assertions, including 1,000 seeded exact-token round trips,
  generated calls, faithful scalar/vector/null/object values, exact list-column frames, raw/native
  evidence, discovery precedence/token isolation, version refusals and malformed/bounded JSON.
  Unicode surrogates/NUL are refused explicitly. Millisecond timeout controls prevent accidental
  unlimited timeouts from fractional seconds. HTTP failures, redirects, proxy isolation,
  oversized results, ambiguous submitted outcomes and no-retry controls pass.
- The installed R client and Python use one temporary, seeded vaccine project (five artificial
  subjects, thirty samples; three observations of subject S01 opened). R/Python/direct HTTP
  native query bytes match exactly at the same project state. Filtered rows retain scientific
  values and full-table denominators while ranks change. Token/grant/null/unsafe-number refusals,
  human rejection, timeout cancellation, named History and normal Undo/Redo pass. Both runnable
  query examples execute. Only the test driver clicks review controls; no user library is used.
- All seventeen Python regression tests pass. The full application/internal Go race suite and
  amd64 catalog/five-kind HTML-certificate replay controls pass using valid cached results;
  no native scientific code changed. Go vet, six CGO-free builds, JavaScript syntax, 163 browser
  module tests and 14 contrast checks pass. CI YAML and the R matrix parse correctly.
- The site builds 55 pages with 67 existing screenshot references. Its 216 accessibility/layout
  checks across 54 pages, two themes and two widths pass with no violations or overflow.
  No application plot or visual changes; no new scientific screenshots are needed. The R guide,
  package help and research §5.50 explain representation, permissions, timeouts and scientific
  limits. Exact certificate replay and release fixtures remain unchanged.

No commit, push, tag, release or publication was performed. Slice 10 (portable project archives)
follows maintainer review and commit.

### Slice 10: portable project archives

**Committed as `205045c` and pushed by the maintainer.**

**Plan recorded before implementation.** Complete D12's portable archive contract with a
versioned `.clz` ZIP containing original project/history bytes, attached immutable artifact
files, available artifact parents, checkpoint evidence, provider notices and a sorted SHA-256
inventory. Preserve source documents and scientific numeric tokens without decode-to-float.
Import creates local path bindings separately; originals remain inspectable evidence.

Source bytes are opt-in, fingerprint-checked and included only for open providers or explicitly
authorized user-owned data. Restricted/unknown-provider sources remain omitted; missing,
changed and unsupported sources have explicit inventory statuses. Captured restricted evidence
requires the existing private-use acknowledgment and retains notices; an archive is not a
redistribution license or de-identification. Never follow recorded source paths on import.

Validate all ZIP names, types, counts, compressed/expanded bounds, inventory hashes, project
versions and dependency identities before promotion. Reject traversal, links, case collisions,
special files, unlisted content and destination conflicts. Stage under the local library; never
overwrite projects/results/checkpoints. Partial interruption may leave verified unreferenced
files, not a successful project import. Preserve checkpoint bytes and apply portable source
bindings when recovering imported checkpoints. No recipes or workers execute during inspection.

Expose export, inspection and import in the window and shared reviewed action catalog, regenerate
both clients, and document the new catalog version. Validate hostile/corrupt archives, exact
round trips, cross-release projects, source policy, cancellation/conflicts, real browser reopening
and both-theme visuals. Keep version 0.7.0 and existing scientific/replay methods unchanged.

Validation completed:

- Version 1 archives retain original project/history, result and checkpoint bytes, including
  scientific large integers, decimal tokens, negative zero and valid checkpoint-record whitespace.
  Sorted SHA-256 inventories and pinned fingerprints verify every member. Missing mandatory
  results/checkpoints refuse export/import; available artifact parents are retained recursively.
- Source inclusion is off by default. Open provider licenses or explicit authority for user-owned
  sources are required; restricted original files remain excluded. Captured restricted evidence
  requires private-use acknowledgment and retains credits/notices. Omitted, restricted, missing,
  changed and unsupported sources have explicit statuses and empty working paths. No source path
  from an original machine is followed during import.
- ZIP tests reject traversal, links, devices, reserved/ambiguous/nonportable names, case and
  file/directory collisions, unsupported methods/encryption, corrupt hashes, duplicate JSON keys,
  wrong versions, forged source pointers, unlisted files and oversized counts/metadata. Metadata
  duplication is bounded before constructing a large inventory. Existing projects and conflicting
  or incomplete results/checkpoints are refused before writes. Cancellation and partial-file
  limits are documented; this is not a transactional whole-library backup.
- Original evidence remains under imports/<fingerprint>; only the working project receives local
  paths and archive status. Imported checkpoints recover with verified bindings; new local
  checkpoints work normally; re-export retains old checkpoint sources after original paths vanish.
  Every v0.2–v0.7 project reopens with original receptor dataset identities and artifact bytes;
  the v0.6 expression matrix/annotations also retain their original input identity.
- A seeded artificial vaccination study transfers between two isolated hosts after moving the
  original files away. Native clonotype query JSON, saved diversity evidence and checkpoint files
  are byte-identical. Rejected import proposals write no project; stale inspection/cancellation
  controls pass. Both new screenshots were inspected; both-theme application accessibility passes.
  The transfer test is added to CI. No new scientific figure, estimator or replay tolerance.
- Catalog 2 exposes 61 actions through HTTP, MCP and generated Python/R wrappers. Source export
  requires review even without project edits. Import requires read/edits/compute/files because
  opening the project may rebuild dataset tables; each missing grant is tested. Saved analyses
  are not recomputed. Native import/export integrity is distinct from five-kind certificate replay.
- The complete Go suite passes, including the scientific validation package. Internal scientific
  race checks, archive/cross-release race controls and focused application permission/MCP checks
  pass. Initial broad race checks exposed obsolete catalog/review assertions and concurrent
  generated-catalog drift; the catalog was regenerated, assertions updated and all affected
  checks rerun successfully. No scientific validation failure remained. Bounded decoder smoke
  fuzzing, Go vet and six CGO-free platform builds pass.
- All 167 browser module tests and 14 contrast checks pass. Python's 17 regression tests pass;
  R passes 1,296 assertions and package build/check without errors, warnings, notes or skipped
  tests. Live R/Python/HTTP parity, reviewed edits, rejection, cancellation, History/Undo and both
  query examples pass. Client generation drift checks pass with catalog 2.
- The site builds 56 pages with 68 screenshot references; all 220 accessibility/layout checks
  across 55 pages, two themes and two widths pass. Guides, format/source/recovery limits,
  requirements, research §5.51 and changelog are updated. Version remains 0.7.0.

No commit, push, tag, release or publication was performed. Slice 11 (export de-identification)
follows maintainer review and commit.

### Slice 11: explicit de-identification exports

**Status: committed as `1c10c3e`; maintainer reports pushed.**

**Plan recorded before implementation.** Implement O10 as a dedicated, versioned dataset-table
export, following CytoWeave's allowlisted-copy approach. Do not relabel ordinary archives,
certificates, HTML reports, figures or arbitrary nested artifacts as de-identified. Those formats
retain original evidence and require separately mapped transformations before support can expand.
This slice's ZIP contains only native clonotype CSV, optional AIRR rearrangements, a sample
annotation table, a fresh transformation inventory and provider notice. No original project,
history, paths, source documents, manifests, recipes, checkpoints, embedded images or binary
attachments may enter it. Unsupported nested scientific annotations refuse the AIRR export
rather than being silently stripped; clonotype-only export remains an explicit alternative.

Prepare and review before download. Generate fresh cryptographically random pseudonyms for
subjects and samples; scope cell/sequence aliases by sample and preserve missing identities.
Display the complete subject correspondence locally. Require explicit choices for dropping
free-text annotations and HLA (both selected initially). Preserve sample membership, counts,
units, QC, clonotype definition, missingness and all retained native scientific field values.
Keep the original dataset/project/artifacts untouched. A separately requested identity-map
file retains original identifiers, metadata and source fingerprints for private audit; it is
never included in the shareable ZIP or its public inventory. Pseudonyms are stable within one
prepared export, fresh across preparations, and not derived from names without a secret key.

Retained sequences, stable scientific identifiers, rare measurements and optional annotations
can enable linkage. No anonymity, privacy-risk measurement, legal certification or exact replay
of a transformed artifact is claimed. Provider terms remain authoritative. Refuse unsafe or
unsupported metadata instead of silently broadening a privacy claim. Bound preparation, files,
identity maps and ephemeral storage; fingerprint-pin both downloads and reject stale browser
state. Add reviewed shared actions and regenerate both clients with the next catalog version.

Validate field-by-field equality with ordinary scientific exports, subject/sample/cell linkage,
missingness, disclosure canaries across every ZIP member, source immutability, restricted-data
policy, separate-map retrieval, cancellation/bounds and UI stale state. Use representative seeded
studies and inspect both themes; no new scientific estimator or figure is introduced. Document
supported scope, removed fields, scientific limits and cited rationale. Keep version 0.7.0.


**Implementation.** Added the native v1 allowlisted table ZIP and separate private map, fresh
per-preparation HMAC pseudonyms, native scientific-field parity, effective annotation removal,
provider attribution, bounded in-memory lifetime and fingerprint-pinned separate downloads.
The window shows every subject correspondence and clears review when choices or project change.
Both shared actions require explicit capabilities and human review; catalog 3 generates 63
Python/R wrappers without changing their numeric-token or permission contracts. Existing source
files, project/history, saved artifacts, replay methods, scientific plots and version 0.7.0 remain
unchanged. Unsupported nested provenance refuses AIRR export; clonotype-only scope is explicit.

**Validation.** Native boundary and focused race tests pass, as do 169 JavaScript tests,
17 Python tests, 1,300 R assertions and R CMD build/install/check without errors, warnings or
notes. Live R/Python/HTTP native-evidence checks, Go vet and all six CGO-free platform builds
pass. The browser workflow verifies 46,447 retained rearrangements and 27,215 clonotypes across
six observations of two simulated vaccination subjects, unchanged native queries/project bytes,
review rejection, separate downloads, changed choices, stale edits and cancellation. Both-theme
screenshots were reviewed; browser accessibility, 14 contrast/color-vision checks and 224 site
checks across 56 pages/two themes/two widths pass. The complete
`go test -race -timeout 30m ./...` suite passes (application 1,093.950 s; scientific validation
940.147 s); existing performance budgets intentionally skip under race instrumentation.
No scientific workers or performance-budget definitions changed.

Guides, format contract, implementation citations (research §5.52), scoped requirement O10,
site page, screenshots, CI and changelog are updated. No commit, push, tag, release or site
publication was performed. Slice 12 (Parquet exports) follows maintainer review and commit. Release preparation remains slice 15.


### Slice 12: typed Parquet dataset exports

**Committed and pushed as `9ed6767`.**

**Plan recorded before implementation.** Follow the wave sequence: this slice implements the
Parquet portion of O3, not release preparation. Correct the stale slice-11 closing note; keep
application/client versions at 0.7.0. Adapt only the Apache-2.0 SituCraft v0.9.0 writer and compact
protocol emission, extending it for optional columns, cancellation, failure handling and explicit
bounds. Do not import a general reader or runtime dependency. Consult the Apache Parquet/Thrift
specifications; validate generated files with independent PyArrow and DuckDB readers.

Export mapped native dataset tables (clonotypes and rearrangements) with flat versioned schemas,
metadata for effective definition/QC, abundance units, native source fingerprints, provider notices,
and exact table population. Use native CSV/AIRR exporters as the scientific source so this adds no
second computation. Assign stable sample-column keys with an explicit name map to avoid collision
with scientific column names. Write missing values as null, explicit zero as zero, flags as bool,
and counts as signed INT64 only when native float64 values are finite, nonnegative, integral and
within the consecutive exact-integer range (2^53−1). Refuse ambiguous/fractional counts, never
round or claim to restore input precision already lost before export. Keep other retained AIRR
evidence as native text, including invalid annotations; this is not an AIRR schema replacement.

Add explicit Parquet options to the window and a reviewed shared export action; regenerate the
Python/R catalog. Integrate an optional Parquet representation into slice 11's pseudonymized
export: run the existing identifier/removal policy first, keep sanitized provenance in every
footer/schema, and never put its private map or original paths/fingerprints in the public bundle.
Existing raw Parquet exports retain original identities and are clearly labeled as such. Saved
analysis artifacts and unsupported nested table types remain outside this mapped export contract.

Use deterministic GZIP/PLAIN v1 pages, RLE definition levels, fixed column/row-group ordering and
sorted metadata. Bound input, pages, groups, metadata and output; complete validation before any
HTTP download succeeds. Preserve provider acknowledgment, cancellation and stale-project guards.
Test exact independent integer/null/boolean/text decoding, empty/multiple groups, byte stability,
invalid input/writer failures, native-table parity, identity-map separation and provider refusal.
Exercise a seeded representative vaccination dataset and inspect both-theme presentation. Update
formats, guides, research, requirements, changelog and CI; no commit/tag/release/publication.

**Implementation.** Added the bounded standard-library writer, native typed-table ZIP route,
schema/notice footer metadata and exact member inventories. Preserve native clonotype and AIRR
table order, nullable counts/flags, explicit sample-column mappings, original source provenance
and optional invariant-rule denominators/shares. Ordinary exports clearly retain identifiers.
The existing pseudonymized workflow optionally emits Parquet after its removal/mapping policy;
footers contain only sanitized public provenance and the private map remains separate. Text
output stays the default. Negative, fractional, infinite and ambiguous large counts refuse
without rounding; native missing values stay null. No scientific worker or replay rule changes.

Added Reports and Clonotypes export choices, stale-project/cancellation guards and reviewed
`export_parquet` automation. Catalog 4 generates 64 Python/R wrappers; API 2, permission policy 1
and application/client version 0.7.0 remain unchanged. Updated guides, formats, scoped O3,
implementation citations (§5.53), NOTICE, CI and changelog. Validation uses independent PyArrow
26.0.0 and DuckDB 2.0.0, including explicit physical-type assertions for native AIRR count names
instead of relying solely on the emitted schema.

**Validation.** Final native export/boundary and focused race checks pass, including nullable
AIRR `duplicate_count`, `umi_count` and `templates`, refusal of unsafe counts, source immutability,
provider policy, sanitized footers, separate identity maps and writer resource/failure bounds.
PyArrow 26.0.0 and DuckDB 2.0.0 pass 16 independent reader/bundle checks against native evidence;
the bounded adapter fuzz run completed 11,288 executions without failure. The browser verifies
46,447 rearrangements and 27,215 clonotypes across six observations of two seeded vaccination
subjects, unchanged query/project evidence, reviewed rejection, stale state and cancellation.
Native/pseudonymized Parquet and existing text-export screenshots were reviewed in both themes.

Pass 169 JavaScript tests, 17 Python tests, 1,302 R assertions and R CMD build/install/check
without errors, warnings or notes; live Python/R/HTTP exact-evidence checks; Go vet; all six
CGO-free platform builds; 14 contrast/color-vision checks; and 228 site accessibility/layout
checks across 57 pages, two themes and two widths. The local site builds 58 pages; no publication.

Complete Go race coverage finished in batches. The initial `go test -race -timeout 30m ./...`
hit its cumulative timeout while portable-release reopening and pseudobulk controls were actively
computing, without assertion failures or race reports. These application/reference tests run
sequentially. Retain the preceding successful checks and rerun every interrupted and remaining
entry (31 application/test-seed entries and 23 scientific-reference entries) with verbose race
instrumentation: both batches pass (986.531 s and 898.286 s respectively). Final Parquet changes
also pass their focused race suite. Existing costly performance/simulation controls intentionally
skip under race instrumentation and remain in CI's non-race data job; no scientific assertions,
numeric tolerances, performance budgets or release fixtures were changed.

Ready for maintainer review and commit. No commit, push, tag, release or publication was performed.
Slice 13 (agent benchmark) follows; container/cluster work is slice 14 and release preparation
remains slice 15.

### Slice 13: known-truth agent benchmark

**Plan recorded before implementation.** Implement A5 as a versioned local task suite, following
CytoWeave's Apache-2.0 task/grader/reference/empty-control pattern and ClonoLoom's existing real
host/window and Python/R validation drivers. Keep the benchmark outside the shipped runtime.
Use a small deterministic artificial AIRR study with independently enumerated receptor groups,
UMI counts, missingness and repeated-subject observations. Grade known arithmetic and state
changes from code, not an LLM or the application result being treated as its own truth.

Cover full-table counts, filtered frequencies with unchanged denominators, missing versus zero,
definition-dependent grouping, explicit proposal rejection and denied/invalid calls. Distinguish
answer correctness, observed task completion and permission behavior from scientific validity.
Do not infer antigen specificity, biological expansion, independent replication from repeated
samples or exact replay from successful tool execution. Keep scientific numeric evidence as
original native JSON text, with strict structured answers and no approximate-success fallback.

Run reference and empty baselines through the real local contracts; exercise HTTP, stdio MCP
and generated Python/R clients. Provide a bounded external-program adapter with argument arrays,
explicit opt-in, a documented JSON-lines tool/answer contract and no shell invocation. No paid
model or external agent is run as part of implementation. Record command identity, software and
benchmark/fixture fingerprints, task selection, repetitions, tool evidence, failures and review
policy. Reference controls validate the harness, not agent capability; open fixtures are not a
held-out generalization test. Never publish a model leaderboard without actual model runs.

Require explicit benchmark grants and scripted-review opt-in. Use isolated temporary libraries
and generated inputs, no network grant, bounded calls/output/deadlines and no automatic retries.
The test reviewer may click only the exact declared fixture proposal, with explicit rejection
tasks; it is not an agent approval API or an operating-system sandbox. Separate setup from scored
actions. Test graders with wrong, incomplete and fabricated answers, missing evidence, changed
state, malformed protocol and failure controls. Show task denominators and limits in a local
accessible report, review both themes, update documentation/CI/research/requirements/changelog,
and keep application/client version 0.7.0 and action catalog 4 unchanged. No commit or release.


**Completed implementation.** Benchmark and fixture format 1 contain eight code-graded tasks
on 32 artificial AIRR rows (two subjects/four observations). Literal per-group/per-sample truth
is independently aggregated from the generated TSVs. The normal native results must retain
five groups/248 UMIs, explicit exclusions and zero counts, and full-population denominators;
the reviewed amino-acid-only grouping has four groups and the same abundance. The fixture is
not a receptor-generation model, specificity validation or biological treatment simulation.

The isolated runner supports HTTP, stdio MCP, Python and R with exact native text and raw
transport evidence. HTTP pre-dispatch refusals retain their original 400/403 envelopes; client
exceptions identify their own origin. Structured answer, evidence and permission grades remain
separate. All selected attempts, including failures, remain in denominators. The optional
trusted JSONL candidate adapter uses explicit argv, sequential bounded calls, strict numeric
tokens, output/deadline limits and no shell or automatic retries. It is not an OS sandbox.
Both host grants and exact fixture-only scripted review require explicit opt-in. Existing
application/client versions, catalog 4, scientific methods and replay scopes remain unchanged.

Validation completed locally:

- All **32/32 reference tasks** passed through the four live transports. Exact native query
  bytes match between transports; independent arithmetic grading is a separate check.
- **0/8 no-action controls** passed, as intended. **3/3 scripted JSONL controls** and **6/6
  scaled/repeated controls** passed. Repeated native evidence is identical. These are scripts,
  not evaluated models; neither repetitions nor count scaling establish independent samples.
- A live negative control refused an out-of-policy action and rejected an unexpected proposal,
  preserving project/input state. Correct answers and native evidence did not override its
  permission failure. Unit controls cover malformed/duplicate/concurrent/oversize protocol,
  timeouts, call limits, numeric rounding, incorrect science, omitted evidence and state drift.
- **209 JavaScript tests** passed (169 existing plus 40 benchmark checks). Focused Go race tests
  for MCP/remote/client contracts, Go vet and the 64-action generated-client drift check passed.
  No Go runtime or scientific worker changed; the earlier full scientific suite was not rerun.
- The report's **four desktop/phone, light/dark layouts** passed axe-core and overflow checks.
  The mobile scrolling table was made keyboard-focusable. Both-theme screenshots were inspected.
  A saved-report validation command rechecks grades/hashes and presentation without rerunning
  scientific actions. The site build checks 59 pages and 72 screenshot assets; all **232**
  site accessibility/layout checks passed. Final focused R definition/precision checks passed
  after tightening exact grant validation and runtime-version provenance.

The runner guide, scientific interpretation, site page, automation guide, conventions,
requirements, research §5.54, NOTICE, changelog and CI are updated. CI runs only local controls,
never a paid model. No commit, tag, release or publication was made. Slice 14 (container/cluster)
is next; release preparation remains slice 15.

**Slice 13 committed and pushed as d63cc53.**

### Slice 14: container and cluster execution

**Plan recorded before implementation.** Package the existing native `run` and `verify`
commands in a multi-stage, digest-pinned Go build and small non-root OCI image. Preserve
source and replay support, exact scientific bytes, provider terms and failure statuses.
No browser, package manager or alternate scientific implementation belongs in the runtime.
Use an allowlisted build context; include only compilation inputs, embedded fixtures and
license notices. Record the supplied source revision honestly and keep version 0.7.0.

Document explicit read-only input, writable output and temporary-scratch mounts, numeric
user/group ownership, no network, dropped capabilities, resource limits and read-only rootfs.
Use exec-form process startup and SIGTERM forwarding with the existing cancellation contract.
Image defaults do not substitute for runtime permissions; do not grant host directories,
remote-control capabilities, provider acknowledgment or network implicitly.

Provide a portable, explicit Slurm replay-array example over versioned recipes and individually
pinned certificates. Generate a new private input bundle without rewriting scientific evidence;
select exactly one named task per array index and reserve a distinct new output folder. Keep
native execution available for sites without a container runtime. Scheduler allocation,
container availability and site policies remain operator choices; no submission, image push or
release automation is added. Document Apptainer integration limits honestly if it cannot be
validated here rather than claiming untested runtime support.

Validate the image's non-root/configuration/filesystem boundary, source computation for batch
QC and motifs, exact native/container output parity and certificate replay, refused changed
inputs and existing outputs, explicit provider terms, unsupported kinds, cancellation and
unchanged sources. Test array selection/refusal without a scheduler, preserving native exit
codes and task isolation. Use existing independent inventory/science controls, inspect both
Linux architecture builds where feasible, and add a bounded CI container job plus guides,
requirements, research citations and roadmap results. No commit, tag, release or publication.

**Completed implementation.** The digest-pinned official Go 1.26.6 build produces a CGO-free
native image with a numeric non-root default, exec-form entrypoint, SIGTERM and only the program,
mount points and ClonoLoom/Go notices in the runtime filesystem. An explicit generated allowlist
contains 345 compilation/embedded-asset files. CI checks list drift and the actual builder/runtime
inventories; injected excluded files must remain absent. No registry publication is configured.

The container command helper prints a shell-quoted command using an inspected local image ID,
disjoint read-only inputs/writable output parent, explicit bounded scratch, no network, no new
privileges, dropped capabilities and non-root numeric ownership. It refuses existing outputs,
linked inputs and unsafe bindings, and never executes the command or acknowledges provider terms.
Original recipe parameters remain native JSON. Rootless Podman, SELinux and remote-engine mount
ownership are documented operator choices; image defaults do not enforce the runtime policy.

A private replay-bundle helper copies bounded regular certificate files without rewriting their
bytes, pins each file and writes a versioned recipe plus task index. The native Slurm example
selects exactly one listed task and a distinct new output per array job/task, quotes every path,
uses exec for signal forwarding and propagates the native status. It submits nothing and never
silently resumes an existing output. Array selection is locally tested; actual Slurm allocation,
log policy and Apptainer conversion/runtime integration remain unvalidated site concerns.

Validation completed locally:

- Linux arm64 and emulated Linux amd64 passed both artificial source workflows and all five
  certificate replay kinds through Podman. Every inventoried scientific source-run output is
  byte-identical to native output; the independent Python ZIP/inventory checker also passed.
  These checks establish the existing software replay contract, not biological validity.
- Valid-checksum one-bit scientific changes report differs; unsupported methods report
  unavailable. Changed input pins, existing destinations, read-only input/rootfs writes and
  missing scratch are refused. Restricted source computation requires explicit acknowledgment.
- All five bundled tasks replay through the native array script and the container. Invalid
  indices/IDs and existing destinations fail. Handled SIGTERM returns exit 2 with a cancelled
  record and cleaned scratch. Original source hashes remain unchanged. Hard-kill/OOM recovery
  and actual scheduler behavior are not claimed by these tests.
- Focused Go race coverage passed for exact workers, difference/unavailable cases, all four
  cohort release-fixture designs, cancellation, report protection, both source workers, recipe
  bounds and task refusals. The initial ten-minute cumulative timeout was resolved by rerunning
  the complete selection with the project's 30-minute limit; it passed in 671 seconds. No Go
  production worker changed, and the unrelated full Go suite was not rerun for this slice.
- **228 JavaScript tests** passed, including 19 bundle/array/container-helper controls. Go vet,
  syntax/format checks and the 64-action generated-client drift check passed. Both-theme guide
  screenshots were inspected; the site builds 60 pages with 72 screenshot assets and passes
  all **236** desktop/phone accessibility/layout checks.

The container/headless guides, site page, CI, conventions, requirements, research §5.55 and
changelog are updated. Source computation remains batch QC/motifs; exact captured-evidence replay
remains the same five kinds. No scientific worker, numerical tolerance, action catalog or version
changed. No commit, push, tag, release, image publication or scheduler submission was performed.
Slice 15 (final study and release preparation) follows after maintainer review and commit.

**Slice 14 committed and pushed as `7e96917`.**

### Slice 15: complete record-and-reach study and release 0.8.0

**Plan recorded before implementation.** Complete the wave with a worked artificial batch-QC
study connecting the ordinary window, exact HTTP/MCP/Python/R query evidence, checkpoint
comparison/recovery, a self-checking HTML report, certificate verification/replay and portable
archive transfer. Use the existing simulator and native methods; retain source truth, units,
subjects, batches, unavailable scores and all samples. Descriptive QC flags are not diagnoses,
treatment effects, automatic exclusions or corrected data. Distinguish unchanged saved bytes,
recomputed captured evidence and biological interpretation at every boundary.

Create a new 0.8.0 compatibility fixture through real reviewed actions in an isolated library.
Keep portable reopening paths separate from original scientific provenance, include the new
checkpoint/certificate/archive contracts, and test exact future reopening/replay. Never edit
older release fixtures. Add a worked guide and site page, refresh representative wave-8 screenshots
in both themes and review all existing scientific scenes through the established browser driver.
Review the full support/permission matrix without expanding unsupported scientific methods.

Bump application, citation, web cache keys and generated client packages to 0.8.0 only in this
slice. Consolidate the release changelog and remove stale development-only instructions while
keeping historical release records intact. Add version/fixture drift checks where needed.
Run complete Go race coverage, required-data scientific/reference checks, processor compatibility,
six platform builds, performance budgets, JavaScript/Python/R/package/live-client checks, offline
report and archive controls, container replay, benchmark baselines and site accessibility.
Use bounded batches for slow suites if necessary; do not weaken scientific tolerances or budgets.
Record every completed check and material local limitation. The maintainer alone commits, merges,
tags, publishes the release and publishes the site; this slice performs none of those actions.


**Implementation.** The complete worked guide and 0.8.0 fixture connect the ordinary window,
HTTP, MCP, generated Python/R clients, checkpoint comparison/recovery, offline HTML reports,
certificates, pinned native replay and portable archive transfer. The artificial batch-QC
study retains 26 observations, 2,427 generating rows, declared batches/exclusions and complete
truth. Its plots retain raw denominators, missing values and unscored groups. Flags remain
descriptive; no treatment, automatic exclusion, batch-correction or specificity inference is added.

The four transports retain identical original query JSON. A reviewed definition change leaves
the saved QC result untouched; recovery restores the original query. Transfer into a fresh
library with original source paths unavailable preserves query and result bytes and supports
checkpoint recovery. The 51-file compatibility fixture keeps original project, checkpoint,
certificate, report, archive and scientific bytes alongside a separately serialized portable
project. All earlier release fixtures are unchanged. Native tests verify exact future replay,
recovery and source/truth hashes; CI runs the complete worked integration and offline report.

Release testing found that accepted automation reset loaded expression inputs and completed
views. Action startup now invalidates earlier asynchronous attachments separately from project
replacement. Cancellation, recovery and Undo/Redo still invalidate stale work; Undo/Redo notify
selection listeners to reload still-selected saved results. Unit and real browser checks cover
these boundaries. Capture scripts now await explicit saved-result reopening after history
changes, use the current action schema and isolated ports, and frame the checkpoint panel itself.
The source-run checksum test pins its original software provenance, preserving its existing
scientific hashes across the release-version change. No scientific worker or tolerance changes.

Application, citation, web cache keys and generated client packages are 0.8.0. API 2, catalog 4,
permission policy 1 and the 64-action catalog stay unchanged. The guides and support matrix
distinguish integrity, restoration, captured-evidence replay and source computation. Certificates
and HTML reports cover five kinds; source recipes cover batch QC and motifs. Broader O5/A6
contracts, actual Slurm scheduling and Apptainer integration remain explicitly outside this release.


Validation completed locally:

- **176 root test/fuzz-seed entries** passed with race instrumentation, in bounded batches;
  all **49 internal packages** passed their race suites. The large paired cohort replay first
  exceeded its two-minute job wait under competing validation load; its complete parent chain
  passed on recheck. The original source-run hashes also passed with their captured version
  pinned. No numerical tolerance or performance budget was relaxed. The container-fixture
  generator is opt-in and ran separately in the real runtime check; the optional publisher
  download test was not enabled. Seven wave-8 fuzz targets each passed 20 seconds of mutation.
- **74 scientific/reference tests** passed with all eleven configured data sources required.
  Four optional publisher/public-HDF5/VDJdb checks were not configured. Every release fixture
  reopened/replayed on arm64 and amd64; the pre-0.3 value-comparison contract stays unchanged,
  and later scientific outputs retain exact bytes. Go vet and all six CGO-free builds passed.
- **231 JavaScript tests**, **17 Python tests**, **1,302 R assertions**, Python wheel/source
  build and installed smoke checks, and R package build/install/check passed. Generated
  clients match all 64 catalog actions. Live Python/R and both MCP protocol contracts passed
  permission, precision, review/rejection/cancellation and Undo/Redo checks.
- Independent PyArrow 26.0.0 and DuckDB 2.0.0 checks passed all **16 reader/bundle controls**.
  AnnData 0.12.6 independently reread and rewrote all **360 × 24** saved counts and their
  identifiers, annotations, receptor links and supplied/computed coordinates.
- The complete worked integration passed, including clean bounded driver shutdown. All five
  supported offline report kinds passed exact fingerprints, changed-byte refusal and zero
  network requests; the worked report's four layouts retain all six native figures. Benchmark
  controls passed **32/32** transport tasks, **3/3** scripted JSONL tasks and **6/6** repeated
  controls, with **0/8** empty controls succeeding as intended. Permission failures remain
  separate from correct answers. No model or biological performance was evaluated.
- Linux arm64 Podman passed both source workflows, all five replay kinds and array tasks,
  exact native/container scientific files, independent inventories, differences/unavailable
  outcomes, explicit terms, mount/input/overwrite refusals and handled SIGTERM cleanup. The
  actual build context contains **345** allowlisted inputs; injected exclusions remain absent.
  Slice 14's Linux amd64 runtime checks remain recorded separately. This slice does not claim
  native Windows UI execution, PowerShell execution, a Slurm scheduler or Apptainer validation.
- All **126 browser scene checks** passed across both themes, including plot/native-value and
  source-free reopening controls. All **73 screenshot pairs** were reviewed; the checkpoint
  screenshot was corrected to show its own panel. The site builds **61 pages** and passes all
  **240** accessibility/layout checks, alongside **14** application contrast/color-vision checks.


- All **six performance budgets** passed in a separate non-race run on ten cores: a million
  rearrangements read/validated in **1.509 s** at **58 bytes/row** plus junctions; 100 cached
  samples reopened in **125 ms**; a searched 200-row page of 999,690 clonotypes in **38.004 ms**;
  10,000 neighborhoods against 99,997 background receptors in **1.177 s**; a million generation
  probabilities in **41.955 s**; 100,000 paired-TCR neighbors in **36.597 s**, using **1.3 GB**.
  Original limits remain unchanged; these are local timings, not universal runtime promises.

**Slice 15 committed and pushed as `4573c93`.** Wave 8 implementation and release preparation
are complete. CI run `40807882153` found two early portability failures: Docker adds
`dev/console` and `etc/mtab` to exported containers, unlike the locally tested Podman inventory;
the old-R job passed package checks but failed when setup-uv tried to save an unused cache.
The R matrix then cancelled the release-R job.

**CI follow-up, awaiting maintainer commit.** Allow those two exact engine-created paths and
report unexpected runtime member names. Disable the unused container/old-R uv caches and
let both R matrix jobs complete independently. Real Docker validation additionally exposed
the Podman-only bare `rw` bind-mount option; use explicit `readonly=false` in both the command
helper and validation, preserving writable-output/scratch and read-only-input boundaries.
Actual Docker and Podman Linux arm64 checks pass for both source kinds, all five exact replay
kinds, array tasks, refusals and SIGTERM cleanup. Docker's filtered build context contains
345 allowed inputs; exclusion canaries stay absent and an injected unexpected runtime file
is rejected by name. All 231 JavaScript tests pass. No scientific worker, tolerance, fixture
or permission scope changed. Hosted CI must pass on the follow-up before the maintainer
creates the wave-8 PR and follows the usual merge/release/site sequence. No commit, push, PR,
merge, tag, release, image publication, scheduler submission or product-site publication was
performed for this follow-up. Wave 9 planning follows that release; no wave-9 work was started.

## 0.9.0: suite and public data (wave 9)

**Plan recorded before implementation.** Build on the 0.8.0 evidence and permission contracts.
Start with exact, unambiguous metadata and complete the remaining readers before querying public
repositories or linking other applications. A local repertoire identifier is not evidence of a
shared subject, identical experimental processing, or permission to redistribute a dataset.

### Wave 9 sequence

| Slice | Scope and requirements | Acceptance before the next slice |
| --- | --- | --- |
| 1 | Exact AIRR JSON metadata and inspection (D2 foundation, D8, D11, N2/N4/N5) | Preserve numeric tokens, unknown fields and original repertoire-object bytes; reject duplicate JSON keys, ambiguous/missing IDs and malformed text; inspect matched/unmatched records and source fingerprints; no claim of MiAIRR completeness or ontology validation |
| 2 | AIRR Repertoire YAML (D2) | Dependency-free bounded reader with a published supported-syntax contract; exact scalar meaning, null/false/zero distinctions, ambiguity refusals and original source hashes; compare supported cases with an independent YAML/AIRR reader; no silent coercion or alias expansion |
| 3 | MiAIRR completeness checklist (O8, D2/D10) | Pin the checklist/schema version, distinguish required/conditional/recommended fields, missing versus explicitly unknown/not applicable, expose evidence paths and remedies; presence is not biological validity or ontology conformance |
| 4 | AIRR Clone, Cell and Receptor objects (D2) | Preserve native objects and explicit repertoire/cell/receptor/processing links; validate identifiers and cardinality; no guessed pairing or replacement of the user's clonotype definition; independent-reader fixtures |
| 5 | Remaining table readers (D5) | TRUST4 reports and VDJtools tables with native count units, missingness, junction conversion and source provenance; compare every mapped value with independent parsing; retain unsupported fields and fuzz refusals |
| 6 | Cell Ranger JSON annotations (D3) | Explicit version/layout recognition, exact barcodes/chains/flags/units, match supported CSV/AIRR representations; refuse conflicting records and unsupported layouts without inventing pairing |
| 7 | AIRR Data Commons discovery and queries (D9, D13, P8) | Pin ADC API contract and approved repository registry; user-initiated bounded queries with visible endpoint/filter/pagination and offline/cancellation/error behavior; never send local receptor data implicitly |
| 8 | ADC downloads and provenance (D9/D11/D13, O2, X5) | Review license/size/source before download; record exact request, repository, retrieval date, response/source hashes and completeness/truncation state; safe paginated/streamed installs, no overwrite, no silent pooled processing or subject identity |
| 9 | Overlap with public cohorts (R4) | Explicit matching definition, loci, source units and cohort universe; show evaluable and missing subjects and sampling/assay limitations; independently computed overlaps and partial-query controls; descriptive sharing is not antigen specificity |
| 10 | Excel workbooks (O4) | Typed values, missingness, full denominators, provenance/methods/notices, limits of spreadsheet number precision; independent workbook reader validation and no formula injection |
| 11 | GraphPad Prism export (O4) | Document a supported interchange format and table/replicate mapping; preserve subject pairing and experimental units, missing values and provenance; independent format checks and import exercise where available; no silent analysis conversion |
| 12 | Proteoscope handoff (X1) | Review exact selected receptor sequence/chain/identifier and destination before transfer; preserve annotation/provenance and explicit permissions; inspect sibling contracts and license-compatible patterns; unsupported or incomplete sequences remain explicit |
| 13 | CytoWeave phenotype links (X2) | Sample-scoped barcode/index-sort identity, explicit mapping and collision/ambiguity refusals, acquisition/compensation provenance and missingness; no fabricated pairing or pooled independent subjects; validate real sibling interchange and simulated known links |
| 14 | Maximum-likelihood lineage methods and viewer (B4) | Select and cite a defensible sequence model before implementation; independently validate likelihoods/search/branch lengths, identify boundaries and convergence failures, retain ancestral uncertainty and rooting assumptions; do not infer switching/selection/dates from observed labels; split this slice if model/search/view work needs separate review |
| 15 | B-cell candidate review (B6) | Explicit user-selected ranking criteria and complete denominators, ties/missing values and provenance; no binding/efficacy claim; export only declared available sequences with chain/pairing limits, reviewed FASTA and Proteoscope handoff |
| 16 | Complete suite/public-data study and release preparation | Worked provenance-preserving study, realistic declared simulations, unavailable/partial/ambiguous controls, both-theme presentation, reference/platform/performance/accessibility checks and 0.9.0 compatibility fixture; version changes and maintainer release sequence only here |

This order is a plan, not a claim of completed support. Each slice must record its implementation,
validation and limits before the next. Research can require a documented split or deferral rather
than an unvalidated method. In particular, lineage likelihoods and candidate ranking need their own
scientific acceptance criteria; no external algorithm/source is selected by this outline.

### Cross-cutting decisions

- Preserve the native-worker, zero third-party runtime dependency and exact-evidence conventions.
  Consult the Apache-2.0 siblings for interchange/export/UI patterns when the relevant slice starts.
- Extend automation only through the shared action catalog with explicit effects and human review;
  regenerate Python/R wrappers when that catalog changes. Local scientific replay stays at its five
  supported captured-evidence kinds unless a separately validated slice explicitly extends it.
- New scientific fields do not round-trip through JavaScript numbers for evidence preservation.
  Keep prior release fixtures and existing valid dataset/artifact identities stable; explicitly
  document new refusals for inputs whose identity or metadata were ambiguous.
- Downloads, queries and cross-application transfers require explicit user intent and preserve
  provider restrictions. A downloaded partial result cannot become a complete cohort silently.
- Visuals show actual imported/computed values, experimental units, uncertainty and missingness;
  no significance or quality color is assigned merely because metadata are present.
- Keep application/client version 0.8.0 during development. Do not commit, push, merge, tag,
  release, publish images or update the deployed site automatically.

### Slice 1: exact AIRR metadata and inspection

**Committed and pushed as `e3f8b49`.** AIRR's metadata standard
defines case-sensitive repertoire identifiers for links to rearrangements. At the start of this
slice the reader decoded through float64, re-encoded each object, and the summary silently chose
the last duplicate ID. This slice resolves those issues before YAML, MiAIRR and ADC build on them.

- Keep each original JSON repertoire object and numeric token without a float conversion;
  preserve unknown nested fields and explicit null, false and zero. Retain source-file hashes.
- Bound metadata size, nesting and structural work; refuse duplicate keys, invalid UTF-8/Unicode,
  non-string/empty/whitespace-padded repertoire IDs and repeated IDs within or across files.
  These are import checks, not full AIRR schema or MiAIRR validation.
- Match explicit repertoire IDs exactly. Preserve the historical sample-name fallback only when
  a sample has no repertoire ID, and label it as such; never use it to conceal an unmatched
  explicit ID. Summarize unmatched records/samples without treating them as study exclusions.
- Add read-only, paged Data-view inspection of record identities, match basis and original JSON
  text, with declared schema version when supplied and source fingerprint. The browser displays
  strings; it neither computes completeness nor parses/re-serializes scientific numeric evidence.
- Test large integers/precise decimals, null/false/zero, escaped keys, duplicates, cross-file
  ambiguity, mixed versions, bounds, cancellation and matching controls. Compare against an
  independent exact JSON reader, preserve existing release fixtures and review both-theme UI.
- Document original bytes versus display summaries, annotation joins versus subject replication,
  and deferred YAML/MiAIRR/Clone/Cell/Receptor support. No new inference, plot or replay kind.


**Implemented:** a bounded source snapshot and exact JSON reader; cross-file ID checks; shared
explicit-ID/legacy-name matching; native paged inspection with original JSON supplied as text;
Data-view controls, cancellation/stale-response guards and keyboard focus; a separate focus
button in dataset cards to remove nested interactive elements. Conflicting single-cell flags
remain unresolved. Declared schema versions and per-file associations are retained. Table-reader
whitespace normalization is unchanged and documented. No project/reader identity version,
scientific method, replay kind, automation policy or application/client version changes.

**Validation completed:** parser/reader race tests, source/gzip/cancellation controls,
API coverage/pagination/refusal controls, original evidence after source removal, mixed schema
versions, independent Python int/Decimal checks, amd64 reader checks, 51,288 fuzz executions with
original-byte/unique-ID invariants, all 231 JavaScript tests, Go vet, six platform builds, current
349-file container allowlist, unchanged 0.2.0–0.8.0 fixtures reopening, both-theme metadata
pagination/stale-response/keyboard/accessibility checks and reviewed screenshots, 61-page site
build/link checks and all 240 site accessibility/layout checks. The declared simulated example
uses bulk RNA UMIs and missing metadata links; it makes no biological or inferential claim.

The full Go regression run initially encountered sandbox loopback restrictions and the default
ten-minute scientific-reference timeout. Local-server tests were rerun with loopback access.
All internal-package race checks pass. Scientific references completed in two bounded race
batches (the resumed 23 tests took 399.7 seconds), retaining their standard optional-data and
non-race performance skips; no numerical comparison failed. The full root race suite passed
with the thirty-minute limit (1,090.360 seconds). Final focused API checks and browser checks
also pass after the inspection/reopening and keyboard-focus guards. No assertions, numerical
tolerances, fixtures or performance budgets were relaxed. Slice 2 is the bounded AIRR Repertoire YAML reader, after
maintainer review and commit; it is not started here.


**Slice 1 committed and pushed as `e3f8b49`.**

### Slice 2: bounded AIRR Repertoire YAML

**Acceptance recorded before validation.** Follow the YAML 1.2.2 and AIRR 2.0 format
specifications, with an explicit supported subset rather than a general-purpose YAML runtime.
Support block mappings/sequences (including indentless sequences), quoted/plain scalars,
literal blocks and JSON-form inline collections. Keep decimal numeric tokens exact; reject
ambiguous numeric forms, duplicate keys, tags, anchors/aliases, merge keys, directives,
multiple documents and unsupported folding. Never silently expand or coerce these constructs.

Keep the captured original YAML document and its source hash separate from the decoded JSON
projection. Expose both as text through metadata inspection, with bounded previews and clear
labels. Preserve gzip and uploaded-filename handling, cross-format duplicate-ID checks, existing
JSON behavior and dataset identities. No schema/completeness or biological-validity claim.

Validate against pinned ruamel.yaml 0.18.6 YAML 1.2 nodes/tags without constructing binary floats;
commit independent expected JSON, exercise malformed inputs/budgets/cancellation/fuzz invariants,
source snapshot and API limits, and both-theme presentation. Document syntax/limits in
`docs/AIRR-YAML.md`; MiAIRR remains slice 3. No version, permission or replay changes.


**Committed and pushed as `173ef55`.** The YAML reader dispatches by supplied
filename, retains exact decimal tokens and typed scalar meaning, checks original source hashes
and rejects unsupported constructs explicitly. Original YAML is stored once per opened file,
excluded from summary serialization, and shown separately from a decoded JSON projection.
Native lexical indentation makes the projection readable without numeric conversion. Existing
JSON evidence display is unchanged. The independent node oracle covers three fixtures, including
literal-block endings and nonbreaking spaces (content, not indentation). Source previews,
cancellation, budgets, mixed-format IDs, uploaded names, gzip and CRLF have native controls.

Validation completed: independent ruamel.yaml 0.18.6 references; focused reader/API race
checks; 706,263 final fuzz executions without failures; amd64 reader checks; Go vet and six
CGO-free platform builds; 231 JavaScript tests; JSON/YAML browser interaction and accessibility
checks in both themes; reviewed YAML source/projection screenshots; 61-page site build and all
240 site accessibility/layout checks; current 350-file container allowlist. Full root race coverage passed in 1,156.277 seconds; internal race suites passed, and scientific
references passed in 1,003.612 seconds with their existing optional-data/performance skips.
Final targeted YAML/API/release-compatibility race checks passed after the Unicode-space fix
(120.423 seconds for the root subset). All existing release fixtures remain unchanged. No versions, fixtures, numerical
tolerances, permissions or replay kinds changed. Slice 3 (MiAIRR completeness) follows below.


### Slice 3: versioned MiAIRR reporting completeness

**Acceptance recorded before implementation.** Use AIRR Standards 2.0's MiAIRR data-element
inventory and requirement-level definitions. Preserve the standard's essential / important /
defined terms: essential requires substantive information, important permits explicit null-like
reporting, and defined is optional but prescribes the field when that information is reported.
Do not rename these levels required / recommended or conflate them with schema required keys.
Conditional context (diagnosis, locus-specific PCR and raw-read files) remains an explicit review
item, never inferred as not applicable from absence, library labels or sequence counts.

Assess each captured Repertoire separately against checklist `clonoloom-miairr-2.0-v1`;
accept declared schema 2.0/2.0.0 only, with other or absent versions explicitly not assessed.
Retain every sample, PCR target, diagnosis and processing index in JSON-pointer evidence paths.
Distinguish absent keys, null, empty, explicit AIRR missingness reasons, nonstandard unknown
markers, typed values and structured values needing review. False and zero remain values.
Expose native counts, levels, conditions and remedies in a read-only checklist; no percentage,
quality grade, compliance certificate or automatic metadata edits. Structural content, ontology,
scientific truth, raw-file access and processed-sequence MiAIRR set 6 remain unassessed, visibly.

Validate inventory/levels independently against the official versioned table, exercise exact
numbers, nested/repeated scopes, null-like controls, unsupported versions, bounds, cancellation,
JSON/YAML equivalence and captured-source inspection. Review both-theme screenshots and
accessibility; run native race, scientific/release regressions, JS, vet, platform and site checks.
No identity/version, permissions, scientific replay or numerical-method changes.


**Committed and pushed as `cd040ae`.** The native 82-field checklist uses the
pinned AIRR 2.0 inventory and exact captured record, with independent official-table comparison.
Metadata inspection adds a collapsed, read-only review, group filtering, evidence paths and
native state counts. Missing parents leave their children unassessed. Unsupported declarations
and exceeded row budgets return no partial rows/counts. Source JSON/YAML remains unchanged.
The example explicitly declares its simulation and includes incomplete essential reporting,
null-like reasons, an unmatched repertoire and separate processing descriptions. No compliance
or scientific-validity badge, automatic applicability, new permission, replay kind or version bump.
The methods/coverage guide is `docs/MIAIRR.md`; citations and adaptations are in research §5.59.


**Validation completed:** native catalogue checks against the checksum-pinned official 82-field
inventory, 22 independent hand-specified evidence controls, JSON/YAML equivalence, false/zero and
exact-number controls, nested/repeated scopes, unsupported declarations, malformed input,
all-or-nothing budgets/cancellation and source-removal API checks. MiAIRR fuzzing passed 291,038
executions. Focused race and amd64 checks pass. Full root race coverage passed in 1,034.967
seconds; all internal race suites passed and scientific references passed in 880.864 seconds,
retaining standard optional-data/performance/container skips. No assertion or tolerance changed.

Go vet, six CGO-free platform builds, all 231 JavaScript tests, the 353-file container allowlist,
three metadata/checklist browser scenes in both themes, reviewed screenshots, the 61-page site
build and all 240 site accessibility/layout checks pass. Existing 0.2.0–0.8.0 release fixtures
remain unchanged and reopen successfully. Original evidence, scientific algorithms, replay,
permissions and application/client version 0.8.0 are preserved. No commit, push, tag, release or
publication was performed. Slice 4 (AIRR Clone/Cell/Receptor objects) follows below.


### Slice 4: native AIRR Clone, Cell and Receptor evidence

**Acceptance recorded before implementation.** Read AIRR 2.0 Data File JSON and the existing
bounded YAML subset, with Clone, Cell, Receptor and supporting Node collections beside the
rearrangement tables. Preserve object bytes/decoded values, unknown fields and full source
text, fingerprints and collection paths. Support nested Clone nodes and standalone Nodes.
Require explicit supported schema declarations for these new objects; retain legacy Repertoire
behavior. Bound combined records/links and reject duplicate IDs, malformed link cardinality,
ambiguous processing IDs and contradictory locally resolved repertoire links.

Resolve only explicit links to loaded objects and repertoire-scoped processing metadata.
Missing/external references stay unresolved, never guessed from names, barcodes, sequences or
one remaining processing record. Validate receptor type/locus pairing and supplied sequence hash
against the AIRR formula, without claiming complete variable domains, biological pairing,
reactivity or ontology validity. Imported Clone membership/counts do not replace a clonotype
definition or alter analysis counts. No raw rearrangement/sequence link or group membership is
inferred; those references remain explicitly unassessed in this slice.

Expose paged read-only object inspection, original evidence and link status in Data. Keep
existing dataset identities, release fixtures, permissions, replay contracts and version 0.8.0.
Validate exact JSON/YAML values with independent readers, linked/unresolved/cross-file and
malformed controls, cancellation/bounds/fuzz, source removal, both-theme UI and full regression.

**Committed and pushed as `7ac32bd`.** A shared exact JSON/YAML Data File
reader retains native objects and full captured documents beside rearrangements, with explicit
link resolution across files. New object import requires declared AIRR 2.0/2.0.0; previous
Repertoire forms retain their semantics, except ambiguous bare object IDs require an explicit
collection wrapper to prevent Cell/Clone references being read as Repertoire identities. IDs,
collection cardinality, repertoire contradictions, processing ambiguity and receptor hash/type/locus
constraints have explicit refusals. Unknown
fields remain source evidence. Imported memberships never change the active clonotype definition.

The paged object inspector uses live dataset summaries (object counts are not added to saved
project metadata), with read-only source text and loaded/unresolved/unscoped/unassessed links.
Both-theme screenshots are reviewed. Independent JSON/Decimal and YAML-node controls cover all
seven artificial objects and the ordered receptor hash; full source text survives source removal.
The initial oracle assertion compared JSON formatting as well as values; it was corrected to
compare exact typed values while separately requiring original JSON byte preservation. The
final reader race and amd64 suites pass. No scientific tolerance or expected value was changed.

Validation completed:

- The full application race suite passes (1,767.678 s), including unchanged release-project
  results, portable archives and exact report replay. Scientific references pass (1,365.041 s)
  with the existing optional-data/performance skips. The initial combined reference command
  reported the formatting-only assertion above; the corrected reader and complete internal race
  suites subsequently pass. A sandboxed internal rerun could not bind its local download-test
  listener; rerunning with loopback access passes without changing the test or implementation.
- Final bare-object and ASCII-alphabet guards have focused reader/API coverage; the API/metadata
  and release-opening regression run passes (185.092 s), with a further API pass after the ASCII
  refusal. Unicode lookalikes are refused even when their uppercase hash matches. Independent
  exact-object and existing YAML oracles pass; the final bounded fuzz run passes 55,803 executions.
- Go vet, amd64 readers and all six CGO-free Darwin/Linux/Windows amd64/arm64 builds pass.
  All 231 JavaScript tests pass. Both-theme object inspection, exact source text, null/false,
  loaded/unresolved links and accessibility checks pass; both screenshots were reviewed.
  The site builds 61 pages with 77 screenshot pairs and passes all 240 accessibility/layout checks.
- The explicit container context checks 356 source/assets; whitespace checks pass. Existing
  release fixtures, version 0.8.0, scientific methods, numerical tolerances, automation permissions
  and replay contracts remain unchanged. No commit, push, tag, release or publication was performed.
  Slice 5 (remaining table readers) follows below.

### Slice 5: native TRUST4 reports and VDJtools tables

**Acceptance recorded before implementation.** Recognize documented tabular report headers,
including explicit legacy TRUST4 names and VDJtools case/frequency aliases. Keep explicit saved
column mappings authoritative. Do not guess a report convention from filenames or conflate
TRUST4's stop marker with VDJtools' frameshift marker. AIRR exports retain their existing reader.

Preserve source headers and complete native row text, unsupported columns, exact numeric tokens,
source line numbers and fingerprints, including cache reopen and uploaded/gzipped sources.
Bound retained evidence and refuse ambiguous headers, malformed row widths and invalid counts.
Map anchored source CDR3s to junctions without adding residues, translating missing peptides,
constructing cell identities or merging chains. Partial TRUST4 junctions remain source evidence
but cannot become complete-junction clonotypes. Preserve missing genes/counts and unknown flags.

Count semantics must be explicit: VDJtools counts may be reads or molecules; modern TRUST4
reports can count assigned read support or barcodes. Keep ambiguous units unknown, using the
existing human unit confirmation rather than guessing. Retain frequencies with their source
denominators; never normalize counts from them. Explain decimal support and float64 analysis
limits separately from exact retained evidence. Structural import is not biological validation.

Provide paged read-only native-row inspection and both-theme simulated examples. Independently
parse every mapped field and all source values using a standard CSV reader and exact decimals;
exercise missingness, alternate marker semantics, gzip/uploads, caching, source removal,
malformed inputs, cancellation and fuzzing. Run reader/API/release regressions and the full
scientific suites. Preserve prior release fixtures, identities, explicit permissions, replay
contracts and application/client version 0.8.0. Bump only the disposable reader-cache version
if needed; do not implement slice 6 or make a commit/release.

**Committed and pushed as `0f473dc`.** Native report dispatch recognizes the documented
TRUST4 modern/legacy and VDJtools aliases while explicit mappings retain precedence. The readers
refuse ambiguous headers, malformed widths, unsupported alphabets, count precision loss and
out-of-range frequencies. Unknown units remain explicit; source frequencies never supply counts.
Partial TRUST4 junctions do not enter complete-junction clonotyping. Stop, frameshift and ambiguous
codon states remain distinct; X cannot certify a stop-free sequence or productive receptor.

Bounded source header/row snapshots retain unsupported annotations and exact numeric text across
cache reopening. Cache version 4 serializes them explicitly; generic source metadata, scientific
exports and private identity maps do not silently acquire raw annotations. The read-only inspector
pages captured rows without opening new paths. Methods paragraphs cite the format papers and
explain normalization, count units, denominators and scientific limits. Three declared simulated
fixtures have independent CSV/Decimal references for every mapped field and source row.

The full application regression exposed a slice-4 legacy-array compatibility bug: initializing
the temporary RawMessage slice with the input buffer let json.Unmarshal overwrite that buffer
before the Repertoire reader used it. The guard now decodes array candidates into separate storage.
A repeated-read test verifies original bytes, precise values and two repertoire identities, while
ambiguous bare Cell/Clone refusal stays intact. No historical fixture or numerical tolerance changed.

Validation:

- The initial application race campaign completed in 1,223.888 s with only the two legacy-array
  opening failures above; every other application test passed. Both failures subsequently pass
  with native-report, mapping, metadata, uploaded-source and complete release-opening regressions
  (132.527 s). Final reader/cache/validator race checks and independent native references pass.
- The full internal/scientific race campaign passes; the scientific reference package completes
  in 1,050.949 s with the existing optional-data/performance skips. Final ambiguity controls keep
  stop status unknown when X could conceal a stop; new references agree without changing any
  historical result or tolerance.
- All 232 JavaScript tests pass, including methods citation/unit controls. Both-theme browser
  checks cover exact decimal/identifier text, partial evidence, frameshift notation, source
  switching and accessibility. Screenshots were reviewed. The site builds 61 pages with 78
  screenshot pairs and passes all 240 accessibility/layout checks.
- Bounded reader fuzzing passes 1,995,926 executions; AIRR object fuzzing passes 42,353 executions
  after the legacy-array repair. Final API race regressions, amd64 controls, Go vet and all six
  CGO-free platform builds pass. The container context checks 359 explicit sources/assets.
  Prior release fixtures, scientific identity version 1, application/client version 0.8.0,
  automation permissions and replay contracts remain unchanged. No commit, push, tag, release
  or publication was made. Slice 6 has not started.

### Slice 6: Cell Ranger JSON contig annotations

**Acceptance recorded before implementation.** Read the detailed top-level array layout of
Cell Ranger `all_contig_annotations.json`, including gzip and renamed uploads. Recognize nested
annotation features explicitly; do not mistake a JSON rendering of CSV columns or an AIRR
metadata object for this layout. State the structural contract and verified producer outputs;
absence of a software-version declaration is not evidence of a specific Cell Ranger version.

Preserve exact barcodes, sample scope, contig IDs, supplied booleans, missing counts and source
clonotype labels. Keep reads and UMIs separate; single-cell abundance remains distinct cells
under the existing QC/definition rules. Do not filter background/nonproductive contigs on import,
infer pairing or overwrite an active clonotype definition. Keep missing full-length/frame/stop
flags unknown, rather than reconstructing vendor algorithms from junctions or coordinates.
Retain mixed-chain annotation evidence without assigning a single resolved chain.

Capture bounded original JSON objects and unknown annotations for read-only inspection, cache
reopening and exact numeric display. Reject duplicate keys/contig identities, conflicting cell
flags or clone assignments, competing segment calls, malformed types and unsupported layouts.
Folder opening retains existing CSV precedence, with JSON as an explicit fallback so duplicate
representations cannot silently double counts. Compare shared fields against independent JSON,
CSV and AIRR parsing, documenting fields absent or differently scoped in each representation.

Use declared artificial examples with realistic unequal support and cell/background/multichain
states. Review both themes, exercise gzip/uploads/source removal/cache, bounds/cancellation/fuzz,
and run scientific/release regressions and platform checks. Keep earlier fixtures, scientific
identity 1, version 0.8.0, permissions and replay contracts unchanged. No slice 7, commit or release.

**Implemented and validated; awaiting maintainer commit.** The bounded nested-array reader
recognizes source structure without guessing a producer version. It retains original JSON
objects, unknown annotations and exact numeric tokens in the existing native evidence cache
and inspector. Reader version 5 invalidates disposable older caches; scientific identity 1 is
unchanged. Folder opening retains filtered/all CSV precedence and uses JSON as a fallback.

Supplied identifiers, genes, junctions, sequence, reads/UMIs and flags remain separate from
derived interpretations. Background contigs remain present. Missing full-length/frame/stop
flags stay unknown; mixed-chain annotations do not receive a single inferred locus. Exact
sample/contig IDs and barcode calls have cross-file conflict checks. Annotation objects use
case-sensitive map lookup, so an unknown `Feature` key cannot override `feature` through Go's
struct-field matching. Explicit AIRR repertoire identities retain precedence in the JSON probe,
including metadata larger than the contig probe budget. No vendor algorithm/source was used.

Independent JSON/Decimal, CSV and AIRR controls cover all mapped values in declared simulated
fixtures. The public Cell Ranger 4.0.0 comparison covers 1,835 contigs, 16,515 shared values and
gene/chain/source-clone calls, plus native source objects and hashes. All public JSON contigs
omit full_length, and 14 have mixed chains; those limits remain explicit. No public data or
per-contig references are committed. The unavailable proposed 9.0 JSON download (HTTP 403)
does not become a version-compatibility claim. The distinct-cell control retains eight source
contigs and gives two cells of abundance from the five declared-cell contigs under the explicitly
selected paired primary-chain definition; UMI/read support is not summed into cell abundance.

Validation completed:

- Full internal/scientific race coverage passes; the reference package completes in 898.484 s
  with existing optional-data/performance skips. Final exact-key/probe, native reader/cache/API,
  public-output and independent-value controls pass after the final parser checks.
- All 233 JavaScript tests pass. Both-theme browser controls check exact large numeric tokens,
  source switching, null/false/zero and absent flags; both screenshots are visually reviewed.
  The site builds 61 pages with 79 screenshot pairs and passes all 240 accessibility/layout checks.
- Bounded reader fuzzing passes 1,206,454 executions. amd64 controls, Go vet and all six CGO-free
  Darwin/Linux/Windows amd64/arm64 builds pass. The container context checks 360 explicit assets.
- Full application race coverage passes in 1,052.100 s, including complete release-project
  reopening, portable archives, self-checking reports and exact headless replay. Final JSON and
  metadata API race regressions also pass. No historical fixture, scientific tolerance,
  application/client version, permission or replay kind changed.
  No commit, push, tag, release or publication was made. Slice 7 has not started.

### Slice 7: AIRR Data Commons discovery and bounded queries

**Slice 6 committed and pushed as `1e20403`. Plan recorded before implementation.** Pin the
ADC v1 protocol (AIRR Standards 1.5.1, ADC specification 1.2.0) and an immutable approved
repository registry. Discovery lists bundled destinations without contacting them. Each service
info request and repertoire-metadata page requires a deliberate local-window action, with its
HTTPS endpoint, exact equality filters, offset and positive page limit visible before submission.

Support a documented metadata-query subset; do not send local sequences, project data or files.
Bound time, response bytes, records and pagination; reject redirects, malformed/ambiguous JSON
and out-of-contract results. Preserve original response text, unknown fields and numeric tokens.
Cancellation, offline mode and provider errors must be visible and must not leave stale results.
Do not interpret a short page as a complete cohort, independently sampled subjects, validated
metadata or a stable/replayable remote snapshot. No automatic paging, retries or installations.

Validate with independent request/evidence controls and an injected HTTP transport, including
offline/no-request, exact values, paging, bounds, malformed data, cancellation and failures.
Use clearly simulated, scientifically plausible metadata for both-theme browser checks and
screenshots. No live repository is a CI dependency. Downloads/provenance installation remain
slice 8; shared automation actions and local scientific replay are unchanged in this slice.

**Implemented and validated; awaiting maintainer commit.** The Data view now opens a bounded
public-repository browser with 16 pinned approved HTTPS origins. Native code prepares the exact
ADC v1 request from four metadata equality filters. Registry listing and preparation are local;
each outbound info/page request requires an explicit action. No local dataset, receptor or file
is consulted. Requests have a 45-second deadline, 8 MiB decoded-response limit, 1–100 records
and offset 0–10,000; redirects, automatic retries and automatic paging are refused.

Exact response text, original repertoire objects, unknown fields, numeric tokens, hashes and UTC
retrieval time remain inspectable. Missing labels remain visibly missing, long table labels are
shortened explicitly, and raw evidence is preserved. Duplicate keys/IDs, malformed values and
over-size pages fail without stale results. Offline, cancellation, close/input-change races,
provider errors and empty pages have explicit states. A returned page never certifies cohort
completeness, stable ordering, shared subjects, reuse permission or scientific validity.

Validation completed:

- Full application race coverage passes in **1,077.544 s**, including saved release projects,
  portable archives, self-checking reports and exact headless replay. The full internal/scientific
  suite passes; reference validation completes in **921.012 s** with existing optional-data and
  performance skips. Final focused ADC/API race and amd64 controls also pass.
- Independent JSON/Decimal controls verify exact request/body hashes, record slices and scalar
  types. Native controls cover all registered origins, intent/offline refusals, strict response
  bounds, HTTP failures, redirects and cancellation. Local HTTPS tests exercise transparent gzip
  limits and cancellation while reading a body. Bounded fuzzing passes **786,369 executions**.
- Both-theme browser controls pass for evidence precision, manual paging without pooling,
  cancellation, closed/stale responses, empty pages, service info, provider errors and offline
  state. Both screenshots are visually reviewed; the declared simulation shows three repertoire
  records for two subject IDs. No live repository is a CI dependency or claimed operational.
- All **233 JavaScript tests**, Go vet, six CGO-free platform builds and the **365-input** explicit
  container context check pass. The site builds **61 pages / 80 screenshot pairs** and passes
  all **240** accessibility/layout checks. `git diff --check` is clean.

The ADC contract, approved registry, citations, supported subset and scientific limits are
documented in `docs/ADC.md` and research §5.63. Application/client version 0.8.0, reader cache 5,
scientific identity 1, historical fixtures, numerical tolerances, automation permissions and
replay kinds remain unchanged. No commit, push, tag, release or publication was made. Slice 8
has not started.

### Slice 8: bounded ADC downloads and durable provenance

**Slice 7 committed as `93d4041`. Plan before implementation.** Download one explicitly
selected repository/repertoire/processing scope per bundle. Review provider permission evidence,
citation, unknown total size and fixed request/byte/page budgets before starting. Capture exact
metadata and rearrangement JSON pages, requests, timestamps and hashes. Install into a new
private directory only after validation; failures/cancellation leave no successful bundle.
Keep partial/live-pagination limits explicit even when a terminal empty page is observed.
No automatic subject pooling, analysis, redistribution grant or change to replay/automation.


**Implemented and validated; awaiting maintainer commit.** The repertoire inspector opens a
separate download review. One explicit action authorizes fresh metadata plus a bounded sequence
of rearrangement requests for one repository/repertoire/processing pair. Provider permission
and citation are recorded by the person, not inferred from a server software license. Total
size is unknown; review shows exact requests and limits of 10 pages, 1,000 rows/page, 8 MiB per
response and five minutes overall. Existing offline, origin, loopback and HTTPS restrictions apply.

Captures preserve original JSON pages and exact numeric tokens. A versioned manifest records
scope, provider permission evidence, citation, contract/registry revision, requests and hashes,
retrieval times, response hashes/bytes, row counts and stop reason. Fresh metadata must match
exactly one processing entry. Mixed scopes, repeated sequence IDs, malformed pages and request
failures abort; unfinished work is removed. New private directories prevent overwriting prior
captures; provider identifiers never become paths. A completion manifest is written last.

Page-budget and empty-page stops both retain unestablished completeness. Concurrent repository
updates can omit rows even when no duplicate is seen. No scientific validity, independent
subjects, redistribution permission or exact remote replay is certified. Original source JSON
is installed as an evidence bundle; it is not automatically converted or opened as an analysis
dataset. Public-cohort use remains slice 9. No new automation actions or replay kinds are added.

Validation completed:

- Full application race suite passes in **1,165.284 s**, including historical projects, archives,
  reports and exact headless replay. Full internal/scientific coverage passes, with reference
  validation in **999.281 s** and the existing optional-data/performance skips.
- Native/API controls pass for review, offline refusal, bounded paging, exact source bytes,
  ambiguous processing metadata, repeated IDs, mismatched scope, cancellation, failed-install
  cleanup and distinct destinations. Final focused race and amd64 controls pass.
- The independent standard-library JSON/Decimal/SHA-256 checker verifies actual native captures,
  including exact request predicates, hashes, offsets, row scopes and stop states. No external
  repository or provider implementation is needed for validation.
- Both-theme browser checks pass for acceptance, invalidated review, cancellation, provider
  failures and concurrent-submit prevention. Final screenshots are visually reviewed and clearly
  simulated. All **233 JavaScript tests**, Go vet and six CGO-free platform builds pass.
- The site builds **61 pages / 81 screenshot pairs** and passes **240** accessibility/layout
  checks. The container context includes **368** explicit inputs; `git diff --check` is clean.

Version 0.8.0, reader cache 5, scientific identity 1, historical fixtures, numerical tolerances,
automation permissions and exact replay contracts remain unchanged. No commit, push, tag,
release or publication was made. Slice 9 has not started.

### Slice 9: observed overlap with captured public cohorts

**Slice 8 committed as `3aeead4`. Plan before implementation.** Compare one local sample with
1–20 explicitly selected, checksum-verified ADC captures offline. Use a separate named presence
contract: productive, one selected locus, exact canonical anchored amino-acid junction; no V/J,
pairing, abundance weighting, rarefaction or inherited project QC. Record that contract explicitly.
Verify manifests, every request/page/hash, scope and stop evidence. Refuse duplicate scopes and
implicit pooling of visits/processing for a scoped subject. Report eligible/excluded rows,
unique keys, shared/union counts and descriptive Jaccard, with unavailable values for empty
eligible sets. Separate assigned/evaluable/missing subjects and unassigned captures; all capture
completeness remains unestablished. Save native result evidence, show source units and sampling
limits, independently check set arithmetic and review both themes. No specificity/prevalence
claim, new replay kind, automation action, version change, commit or release.


**Implemented and validated; awaiting maintainer commit.** Data → Public overlap compares one
loaded sample with 1–20 deliberately selected native ADC captures, offline. The separate
`productive-single-chain-locus-exact-junction-aa-v1` contract uses one locus, true productivity
and exact canonical anchored junction amino acids. It does not inherit project QC or V/J,
paired-chain or nucleotide clonotype rules, and does not use source counts as weights.

The native reader checks fixed regular members, exact manifest keys/types, the pinned contract,
every request/hash/endpoint/time, response bytes/hashes, repertoire/processing scope, unique
sequence IDs and stop/count evidence. It rejects symlinks, altered or inconsistent sources,
duplicate scopes and multiple observations of the same repository/study/subject. The comparison
budget is 64 MiB of capture evidence, one million loaded dataset rows and one minute. Existing
captures remain intact; each success writes a new private observational JSON result.

Report raw/eligible/excluded/unique records, exact shared/union counts, Jaccard and each shared
query key's capture/scoped-subject support. Empty eligible sets are unavailable, including an
empty query. Missing study/subject IDs stay outside the assigned/evaluable subject denominator
while their pairwise and aggregate sequence evidence remains visible. Subject identity across
studies/repositories and biological independence are not inferred. Both empty-page and
page-budget captures retain unestablished completeness; no prevalence, antigen specificity,
assay/depth equivalence, abundance comparison or exact remote replay is asserted.

Results retain the loaded query's dataset/sample identity and full source hashes, reader
identity, named match definition, selected capture manifest fingerprints, scoped identifiers,
permission/citation statements, stop reasons and all observed shared keys. The UI shows the
first 100 keys; native evidence keeps all of them. Jaccard display uses four significant digits,
with exact integer numerator/denominator and full native values retained. Input edits invalidate
review; cancellation, late responses, project changes and duplicate submissions are guarded.
Standalone results are not new project artifacts, report/certificate entries or automation actions.

Validation completed:

- Full application race suite passes in **1,114.880 s**, including historical projects, archives,
  reports and exact headless replay. Full internal/scientific coverage passes, with reference
  validation in **951.439 s** and the existing optional-data/performance skips. Final focused
  API/ADC race controls pass after the manifest/schema and evidence refinements.
- Independent CSV/JSON/Decimal/set/Fraction reference controls agree on all counts, ratios,
  subject denominators and per-key support. The fixture has 40 unique eligible query keys,
  24 observed in selected captures, two evaluable of three assigned scoped subjects and one
  unassigned capture. Varied junction lengths, repeats, unknown count units, nonproductive
  evidence and both stop reasons are declared artificial, not a biological generative model.
- Native/API race and amd64 controls pass, including disjoint/identical/empty sets, duplicate
  observations, exact manifest fields, rehashed conflicting scope, checksum changes, explicit
  acceptance, missing/null sample selection, local-only access, budgets and cancellation.
- Both-theme browser controls agree with the independent reference and pass acceptance,
  concurrent-submit, input-change, stale-response, cancellation and failure checks. Screenshots
  are visually reviewed. All 233 JavaScript tests, Go vet and six CGO-free builds pass.
- Site build: 61 pages / 82 screenshot pairs; 240 accessibility/layout checks pass. Container
  source allowlist: 372 inputs. `git diff --check` and formatting checks are clean.

Version 0.8.0, reader cache 5, scientific identity 1, historical fixtures, numerical tolerances,
automation permissions and exact replay contracts remain unchanged. No commit, push, tag,
release or publication was made. Slice 10 has not started.

### Slice 10: precision-preserving Excel evidence workbooks

**Slice 9 committed as `173ec98`. Plan before implementation.** Extend the existing explicitly
selected saved-result report workflow with a native, dependency-free XLSX presentation. Keep
the five mapped saved-result/notice contracts (cohort features/comparisons/count models, batch
QC and motif screens), not untyped inference from arbitrary CSV columns. Each result and full
manifest becomes a complete typed JSON-node worksheet, with pointer, source type, spreadsheet
value, exact original scalar token and container child count. This preserves nested denominators,
exclusions, identifiers, zero, false, null, empty strings and absent fields without guessed joins.
Use conservative numeric cells within Excel's 15-digit/range limits; otherwise text. Preserve
original result/manifest bytes and native certificates as ZIP companions; carry methods, provider
notices and complete parameters, with no new estimator/replay claim. Bound work, refuse rather
than truncate, prevent formula/link injection, test with an independent workbook reader and
review both-theme UI and a rendered workbook. No new automation action or version change.

**Implemented.**

- Native stdlib SpreadsheetML writer with explicit text/number/boolean cells, exact original
  scalar tokens, RFC 6901 paths, complete container rows and full original manifest indexing.
  Strings never become dates or formulas; null/absent/empty/zero remain distinct. Numeric
  admission is conservative (15 significant digits, zero or absolute magnitude 1e-300–1e300,
  negative zero as text). Native companions remain authoritative for original bytes.
- Reports workflow selects 1–8 mapped saved results, verifies their existing certificate/file
  contracts, carries methods/citations/provider terms, and downloads a deterministic XLSX plus
  original JSON/manifests/notices/certificates in a hashed ZIP. Full parameters, denominators,
  exclusions and source fingerprints remain present. No new estimator or replay claim.
- Explicit combined-node, cell, text, XML and companion-evidence bounds; refuse oversized
  evidence rather than omit rows. Provider acknowledgment, cancellation, concurrent-submit
  refusal and stale-project suppression. No new automation action or client/MCP permission.
- Frozen/filterable headers, visible column boundaries and landscape printing with repeated
  headers. Reviewed precision controls and an artificial batch-study overview in LibreOffice.
  Document spreadsheet display limits and loss of the exact-evidence contract after re-saving:
  a spreadsheet application can replace booleans with formulas, discard empty-string cells or
  normalize line endings. Keep original native companions; cell rows are not independent subjects.
- Independent openpyxl/XML/ZIP/Decimal reader for every node/token/type in all five mapped kinds
  and the real browser download, plus CI integration. Original release fixtures remain immutable;
  use bounded native count/motif controls for successful export and explicitly refuse an oversized
  historical result. The numerical edge-case fixture is declared artificial format data.
- Documented contract, precision policy, scientific limits, primary format sources, sibling
  attribution and both-theme screenshots. O4 remains partial: other workbook mappings are outside
  this contract, and GraphPad Prism remains slice 11.

**Validation completed.**

- Full application race suite: 1,199.532 s; full internal/scientific race suite passes, including
  the validation package in 975.153 s. Existing opt-in published-download/container checks remain
  skipped when their external inputs are not requested. Historical fixtures and replay hashes
  remain unchanged.
- Final focused workbook race controls pass (62.549 s application tests); amd64 workbook/API
  controls pass (7.825 s application, 0.962 s writer). Cover all five native kinds, mixed selections,
  original bytes, deterministic output, certificate integrity, restricted terms, altered evidence,
  cancellation, invalid JSON/Unicode, formula-like strings, numeric boundaries and explicit refusal.
- Independent openpyxl 3.1.5/XML/ZIP/Decimal audit: 66,663 typed JSON nodes across five native
  result/provenance exports, the actual browser ZIP and 33 precision-control nodes; all original
  scalar tokens, value-cell types, missingness states and companion hashes agree. Seven precision
  fixture numbers deliberately remain text. CI runs this independent audit.
- Both-theme artificial-study browser controls pass for empty initial selection, successful
  download, errors, cancellation, stale responses and concurrent-submit prevention. Both screenshots
  reviewed; independent LibreOffice rendering checked the Read me and precision sheets, including
  literal escape-looking text, identifiers and exact scalar columns. Re-save limits are documented.
- All 233 JavaScript tests, Go vet and six CGO-free darwin/linux/windows × amd64/arm64 builds pass.
  Site build: 61 pages / 83 screenshot pairs; 240 accessibility/layout checks pass. Container source
  allowlist: 375 inputs. Formatting and `git diff --check` are clean.

Version 0.8.0, reader cache 5, scientific identity 1, historical evidence, numerical tolerances,
automation permissions and scientific replay contracts remain unchanged. No commit, push, tag,
release or publication was made. Slice 11 has not started.


### Slice 11: subject-preserving Prism interchange

**Slice 10 committed as `4d57ae4`. Plan before implementation.** Export one explicitly selected
saved cohort feature/comparison result as data-only PZFX Column tables, following CytoWeave's
Apache-2.0 format pattern and GraphPad's published schema. One table per recorded feature,
two columns in declared reference/comparison order, one subcolumn, and one identified subject
per row. Pair by explicit subject identity, never input order. Independent subjects occupy only
their own group column; the opposite cell is structurally empty, not a missing measurement.
Keep missing measurements blank without removing subjects or replacing them with zero. Carry
cell-to-source mappings, reasons, exclusions, complete native JSON/manifests/notices/certificates,
methods and a hash inventory. No count-model coefficients, cells, clones, bootstrap draws or
technical replicates become independent subjects. No analysis/graph template, pooling, imputation
or silent complete-case conversion. Validate design/measurement agreement and explicit bounds;
retain original numeric tokens and document Prism import precision and method differences.
Validate independently with XML/Decimal, the published schema and R's pzfx reader, and exercise
both-theme UI with artificial paired data. A native Prism import is conditional on availability;
report that limitation explicitly. Keep versions, scientific identities and automation unchanged.

**Implemented.**

- Reports exports one explicitly selected saved cohort feature/comparison result as data-only
  Prism 7+ PZFX Column tables. One table per recorded feature, declared group order, one subcolumn
  and one identified subject per row. Paired observations align by exact subject identity;
  independent subjects occupy separate rows with explicit structural opposite-group blanks.
- Preserve numeric zero, missing measurements and reasons, all subject rows (including wholly
  missing features), excluded-sample dispositions and exact original numeric tokens. Check the
  captured design, feature catalog, measurements, comparison points and denominators without
  rerunning scientific workers. Refuse ambiguous identity, changed evidence and unsupported values.
- Package a cell-to-source mapping, unchanged native result/manifest, certificate, provider notice,
  methods/citations, import guidance and SHA-256 inventory. Keep provider acknowledgment explicit;
  support cancellation, visible errors, concurrent-submit refusal and stale-response suppression.
- Bound evidence, data positions, XML text, numeric tokens/exponents and construction time. Refuse
  negative zero, out-of-range values and integers beyond binary64's exact integer range. Other
  decimal fractions may round in Prism; native companions remain the authoritative exact evidence.
- Document the scientific boundary: data export does not configure a paired test, establish
  biological independence or translate statistical methods. Prism's omission of incomplete pairs
  can differ from ClonoLoom leaving a feature untested. No pooling, imputation, complete-case
  conversion, technical-replicate expansion, new estimator, analysis template or graph is added.
- Add independent XML/Decimal and R-reader checks to CI, both-theme artificial paired-study
  screenshots, primary format/scientific sources, sibling attribution and container source inputs.
  O4 remains partial for other layouts. Native Prism GUI import remains unverified because Prism
  is unavailable; the guide supplies a manual import exercise. The published Prism 7/8 Mac XSDs
  fail strict lxml compilation on their nondeterministic `d` content model, so no successful strict
  XSD validation is claimed. No vendor schema or stylesheet is redistributed.

**Validation completed.**

- Full application race suite passes in 1,314.842 s, including historical project reopening,
  portable archives, certificates, reports, existing workbooks and exact headless replay.
- Full internal/scientific race suite passes, including the validation package in 1,102.029 s
  and existing opt-in external-data/performance skips. Final focused Prism race controls pass
  (5.895 s application, 1.283 s writer); amd64 controls pass (2.917 s application, 0.726 s writer).
  Controls cover exact tokens, reordered paired points, experimental units, wholly missing
  features, exclusions, inconsistent denominators, provider acknowledgment, changed evidence,
  deterministic output, cancellation, XML text, numerical bounds and oversized exports.
- Independent XML/ZIP/Decimal and R pzfx 0.3.1 audits pass for seven exports, including the
  actual browser download: 59 tables, 743 declared subject/group measurements, 72 missing
  measurements and 515 structural blanks. Subject identities, group order, source pointers,
  literal numeric tokens, missingness reasons, exclusions and inventory hashes agree.
- Both-theme browser controls pass for empty initial selection, successful export, errors,
  cancellation, stale responses and concurrent-submit prevention. Screenshots visually reviewed;
  the example is explicitly artificial, with 28 paired subjects and 56 samples. No new plot or
  native Prism GUI rendering is claimed.
- All 233 JavaScript tests, Go vet and six CGO-free darwin/linux/windows × amd64/arm64 builds pass.
  Site build: 61 pages / 84 screenshot pairs; 240 accessibility/layout checks pass. Container
  source allowlist: 378 inputs. Formatting and `git diff --check` are clean.

Version 0.8.0, reader cache 5, scientific identity 1, historical evidence, numerical tolerances,
automation permissions and scientific replay contracts remain unchanged. No commit, push, tag,
release or publication was made. Slice 12 has not started.


### Slice 12: reviewed Proteoscope sequence handoff

**Slice 11 committed as `450e78d`. Plan before implementation.** Follow the Apache-2.0 sibling's
loopback startup-probe pattern, but require an explicit destination port and a reviewed single
source observation. Proteoscope 0.10 has no sequence-prefill URL: its optional remote `search`
command invokes RCSB sequence search. Name both recipients and require separate local-transfer,
public-search and applicable provider-term acceptance before sending. No background discovery,
remote-control enablement, arbitrary commands, source identifiers in the query or automatic retry.

Start with an unchanged, fingerprint-verified AIRR `sequence_aa` value. Retain exact header/row,
identifiers, locus, source hash, annotations (including completeness), notices and query bytes in
a downloadable review record. Do not translate nucleotides, strip residues, infer missing chains,
substitute junctions, or call partial V(D)J evidence a complete receptor. Bound source/record sizes,
destinations and time; refuse unsupported sequences, changed sources/destinations and reused reviews.
Use the actual Proteoscope command parser/classifier and receiver in interoperability validation,
with public search intercepted so no private or artificial sequence is submitted to a public service.
Exercise both-theme UI, exact query/provenance checks, permissions, cancellation and stale review.
Keep scientific replay, versions and automation catalog unchanged; report downstream uncertainty.

**Implemented.**

- Data → Proteoscope handoff selects one original AIRR source observation and an explicit
  loopback port. Initial selection and permission checkboxes are empty. Native source hashing
  checks the unchanged regular file before review/export/send; exact original header/row text,
  all decoded fields, identifiers, sample, locus and completeness annotations stay in local JSON.
- Accept only unchanged `sequence_aa` of 25–5000 uppercase canonical amino acids and an explicit
  receptor locus. Missing/ambiguous/stopped/gapped/lowercase sequences and other formats are
  refused without translation, trimming, junction substitution or chain inference. Eligible
  fragments retain partial/unknown annotations; no full protein or paired complex is asserted.
- Review Proteoscope 0.10.0's explicit local endpoint/version/capabilities. The supported remote
  search command can invoke public RCSB sequence search, so require separate local-transfer and
  public-disclosure approvals plus applicable provider-term acknowledgment. No background scan,
  remote enablement, token discovery, arbitrary command, hit selection or fallback public query.
- Bind a bounded five-minute, one-use review to source evidence, query, destination and notices.
  Recheck before sending; refuse changed/reused reviews and concurrent dispatch. Offline mode
  permits local evidence export without probing or sending. Closing/cancelling cannot retract a
  query already received; uncertain outcomes require inspection, never automatic retry.
- Save exact reviewed evidence before dispatch, then a permission/receiver receipt on acknowledgment.
  Keep original receiver JSON text, SHA-256 and numeric tokens; provide a manual receipt download.
  Acknowledgment is not proof of a successful search, valid structure or biological function.
  No scientific worker, project artifact, replay kind or MCP/client permission is added.
- Document explicit size/time/alphabet/format limits, primary sources, sibling attribution and
  realistic interpretation of the declared artificial format controls. X1 remains partial for
  other formats, paired-complex transfer and structure prediction. Slice 13 has not started.

**Validation completed.**

- Full application race suite passes in 1,283.991 s, including historical project reopening,
  portable archives, certificates, report/workbook exports and exact scientific replay.
- Full internal/scientific race suite passes, including the reference validation package in
  1,043.367 s and existing opt-in external-data/performance skips. Historical evidence and
  numerical tolerances remain unchanged.
- Final native handoff race controls pass in 2.289 s; amd64 controls pass in 3.687 s. Include exact
  source/receipt bytes, UTF-8 BOM and gzip, precision-sensitive annotations, malformed text/widths,
  source bounds, permission refusals, changed source/destination, offline export, redirect refusal,
  cancellation, review binding/capacity and one-use concurrent submission.
- Actual Proteoscope 0.10.0 command parser, browser receiver and sequence classifier accept two
  exact source sequences with no identifier disclosure or text-search fallback. Every search is
  intercepted before its server can contact a public service. No live RCSB query is performed.
- Independent CSV/JSON/SHA-256 audit checks five handoffs: the native permission fixture, two real
  receiver exercises and both actual browser downloads. Original source rows/annotations, query
  bytes, destination/permission records, review fingerprints and receiver response hashes agree.
- Both-theme browser controls pass for initial empty selection, unselected approvals, exact
  downloads, partial/missing evidence, cancellation, concurrent submissions and stale project
  reviews. Screenshots reviewed. All 233 JavaScript tests pass; site build has 61 pages / 85
  screenshot pairs, with 240 accessibility/layout checks and no problems. Container source
  allowlist: 380 inputs.
- Go vet and all six CGO-free darwin/linux/windows × amd64/arm64 builds pass. Formatting and
  `git diff --check` are clean.

Version 0.8.0, reader cache 5, scientific identity 1, historical evidence, numerical tolerances,
automation permissions and scientific replay contracts remain unchanged. No commit, push, tag,
release or publication was made. Slice 13 has not started.

## Later waves (outlined)

| Wave | Release | Theme | Main content |
| --- | --- | --- | --- |
| 10 | 1.0.0 | Hardening | Budgets met, accessibility audit, documentation complete, external beta feedback addressed |
