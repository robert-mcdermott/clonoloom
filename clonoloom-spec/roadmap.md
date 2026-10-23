# ClonoLoom roadmap

The plan for building ClonoLoom. Each **wave** is one release: wave 1 is 0.1.0, wave 2 is 0.2.0,
and so on. Each wave is split into **slices**, bite-sized units of work. A slice ends with:
- something that runs;
- tests or validation that check it;
- its documentation;
- an entry in `CHANGELOG.md`.

Slices are done in order within a wave unless noted. Each slice names the requirements
(`requirements.md`) it serves. When a wave is reached, it is planned in detail: its slices are
confirmed or re-cut, and the decisions taken are recorded here. Waves 1–4 are complete; wave 5
has a sequence of slices and a detailed plan for slice 1. Later slices are refined as they come.

**Progress** is marked here as each slice is committed: done, with what it settled or changed from
the plan; next; or to come. Each requirement's own status is in `requirements.md`, and what each
release brought in `CHANGELOG.md`. **Where things stand:** wave 1 is merged (0.1.0 not tagged); waves
2–4 are merged and released as 0.2.0–0.4.0, and the 0.4.0 product site is published.
Wave 5 is underway on `wave5`; slice 1 is implemented and awaiting the maintainer's commit.

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

Planned on `wave5`. As in SituCraft's reader and reference slices, preserve and check the input
before building on it, keep the source's identity with derived results, and test against
independent values. ClonoLoom continues to compute every reported number in Go. The order below
puts a simulation with known families before the analyses it validates. Slices 1 and 2 are implemented; later slices need their methods, data and acceptance criteria settled
in detail before implementation.

| Slice | Status | Scope |
| --- | --- | --- |
| 1. B-cell sequence evidence | Committed | Retain AIRR sequences, alignments and coordinates; inspect their availability and validity |
| 2. IMGT/HighV-QUEST input | Implemented; awaiting commit | Join the documented output tables by sequence identity, preserving aligned sequences and annotation provenance (D5) |
| 3. Simulated B-cell families | To come | An immunization study with known ancestors, families, mutations, isotypes and sampled abundance (E1) |
| 4. Clonal families | To come | Distance-to-nearest distributions, explicit thresholds, heavy/light pairing and subject boundaries (C4, B2) |
| 5. Germlines | To come | Checksum-pinned reference sets and per-family reconstruction with D masking (B1) |
| 6. Somatic hypermutation | To come | Evaluable bases and codons, mutation by region, replacement and silent counts (B3) |
| 7. Lineage trees | To come | Maximum parsimony, ancestral sequences and a viewer with stated uncertainty and limits (B4) |
| 8. Isotypes and class switching | To come | Usage and family-level isotype observations; distinguish observations from inferred switching (B5) |
| 9. Examples and release 0.5.0 | To come | Complete guided immunization workflow, compatibility fixture, captures and release validation (E3, O2, N7) |

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


## Later waves (outlined)

| Wave | Release | Theme | Main content |
| --- | --- | --- | --- |
| 6 | 0.6.0 | Single cell | Expression linked by barcode, the map, phenotypes per clonotype, clonotype modularity and CoNGA-style analysis; AnnData, MuData and 10x matrices |
| 7 | 0.7.0 | Cohorts and robustness | Study design, group comparisons with the subject as unit, robustness to the definition, GLIPH2-style groups, HLA association, contamination and batch QC, pseudobulk |
| 8 | 0.8.0 | Record and reach | Certificates and `verify`, `run`, reports, MCP, Python and R clients, agent benchmark, archives, de-identification, container file |
| 9 | 0.9.0 | Suite and public data | ADC queries, overlap with public cohorts, Proteoscope and CytoWeave links, MiAIRR checklist, Excel and Prism, likelihood trees, candidates, remaining readers |
| 10 | 1.0.0 | Hardening | Budgets met, accessibility audit, documentation complete, external beta feedback addressed |
