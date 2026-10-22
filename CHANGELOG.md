# Changelog

## 0.3.0 (unreleased)

Wave 3 of the [roadmap](clonoloom-spec/roadmap.md): tracking. Built slice by slice; this section
grows with each.

### Added

- **The website (wave 3, slice 1).** <https://robert-mcdermott.github.io/clonoloom/>, built from
  `docs/site/` and published on GitHub Pages from a `gh-pages` branch holding only the built site
  ([docs/site/README.md](docs/site/README.md)); its generator, audit and styles adapted from
  SituCraft 0.9.0's:
  - a home page, install, the methods with their validation and citation, and the user guide a
    page a subject (thirteen pages, from opening data to the record), with the screenshots in both
    themes;
  - `docs/GETTING-STARTED.md` now points to the guide, so that it is written once; the
    references (formats, remote control, simulated studies, installing) stay in `docs/`;
  - a link-preview image made from a screenshot (`docs/site/og-image.mjs`);
  - axe-core listed in `validation/sources.json` as a tool, fetched for the audit only.

### Validation

- **The website:** the build checks every local link, anchor and screenshot, and every link into
  the repository's files (CI runs it on every commit); every page passes axe-core 4.13.0's WCAG 2.1
  A and AA rules in both themes at a desktop's and a phone's width, none wider than a phone's
  screen (`node docs/site/audit.mjs`: 16 pages, 64 checks, no problem).

## 0.2.0 (2026-10-21)

Wave 2 of the [roadmap](clonoloom-spec/roadmap.md): repertoire basics. Each sample's diversity,
rarefaction and extrapolation, gene usage and junctions, and every pair's overlap, each with its
uncertainty and checked against reference tools and simulated truth; figures and exports that
carry their definition, unit and license; simulated studies, remote control and jobs.

### Added

- **Remote control (wave 2, slice 1).** With `--remote-control`, programs on this computer drive the
  open window through `/api/remote/action` (`remote.go`, `actions.go`, `web/ui/remote.js`, adapted
  from SituCraft 0.9.0): eleven actions to describe the project, open data or an example, show a
  view, define clonotypes, choose a clonotype, write the methods paragraph, and set the theme.
  Actions are performed in the window, one at a time, and recorded in the project's history like
  any edit. Only programs on this computer are accepted; reading files by path needs the token in
  `remote.json`, written to the data folder (readable by the user only) and removed on exit; data
  under terms of use are never downloaded by a program. `GET /api/remote/tools` lists the actions
  with JSON Schemas. [docs/REMOTE-CONTROL.md](docs/REMOTE-CONTROL.md).
- The documentation's pictures are captured through remote control.
- **Simulated studies (wave 2, slice 2).** `clonoloom simulate` (`internal/simulate`,
  [docs/SIMULATION.md](docs/SIMULATION.md)) writes bulk TRB studies whose truth is known:
  - rearrangements from a V(D)J recombination model of ClonoLoom's own: GRCh38's TRB gene segments
    at Ensembl's annotation; V and J usage from 10x Genomics' 10k PBMC data (CC BY 4.0); deletions,
    palindromic nucleotides and non-templated insertions; productive rearrangements selected on
    junction length so their lengths are the data's (mean 14.5 amino acids, SD 1.7); no OLGA or
    IGoR model file used;
  - each subject's repertoire a population of power-law clone sizes (with expanded clones where
    asked); each sample templates drawn from it, sequenced into reads;
  - written as AIRR rearrangement tables (templates as `umi_count`, reads as `duplicate_count`)
    with AIRR Repertoire metadata (subject, condition), and the truth beside them: every clone of
    every population, and each population's clonotypes, Hill numbers, Shannon, Simpson, evenness,
    clonality, Gini, top-ten share and homeostasis; each sample's expected clonotypes and coverage
    at its depth; the clonotypes subjects share;
  - presets `standard` (control subjects and subjects with expanded clones), `null` (no difference
    between groups) and `small`; parameters as JSON; the same seed gives the same files, byte for
    byte;
  - two of them in the **Example data** dialog, made on this computer in a few seconds (offline
    too); a simulated dataset's methods paragraph says how it was made, citing the model's sources;
  - a folder opened as one dataset leaves out a simulated study's `truth/` folder.
- **Jobs, artifacts and the table cache (wave 2, slice 3).**
  - **Jobs** (`internal/jobs`, adapted from SituCraft 0.9.0): long work runs in the program, one
    job at a time, with an immutable spec, progress, a log and cancellation; jobs are kept in the
    library, survive a reload of the window, and one ClonoLoom's stopping interrupted can be
    resumed or discarded. Example downloads and simulated studies run as jobs. The **Jobs** view
    lists them (running, interrupted, finished) with their logs and specs; the page follows them
    over server-sent events (`/api/jobs/events`).
  - **Artifacts** (`internal/artifacts`, adapted from SituCraft 0.9.0): the store of results kept
    with their lineage (parents, job and spec, method and version, parameters, software), every
    file's SHA-256 and their check's findings, which the analyses of the next slices fill.
  - **The table cache** (`internal/cache`): a dataset's tables, once read, are kept and answered
    again without being read, keyed by the readers' version and each file's SHA-256, size and
    name (each file hashed again first, so a changed file is never answered from it); entries are
    written whole and checked by a CRC-32; least recently used removed first beyond
    `--cache-limit` (10 GB); `--cache-dir` chooses the folder. The Jobs view shows the cache and
    clears it.
- **Diversity (wave 2, slice 4).** The **Repertoire** view (`internal/diversity`, `api_diversity.go`):
  - each sample measured under its dataset's clonotype definition and filter, as a job, the result
    kept as an artifact named for what it measured (asked again, the kept result answers), listed
    with the project's results;
  - observed: the Hill numbers of order 0 to 4, Shannon's entropy, Simpson's index, evenness,
    clonality (Adaptive's 1 − evenness) and Simpson clonality (√Σp²), Gini's coefficient, the 10,
    100 and 1,000 largest clonotypes' shares, the largest frequency, clonal space homeostasis;
  - estimated: Chao1 (bias-corrected) with its log-transformed interval, ACE, the Shannon diversity
    (Chao, Wang and Jost 2013) and the Simpson diversity (Chao et al. 2014) of the repertoire each
    sample was drawn from, and sample coverage;
  - every measure with a 95% interval from 200 bootstrap replicates of each sample drawn from its
    estimated repertoire (Chao et al. 2014, Appendix G), the same on every computer (each
    replicate its own random stream);
  - a measure across the samples with its intervals, the Hill profile and homeostasis plotted,
    colored by condition (the annotation's, else the metadata's), the measures explained, and a
    warning where a sample's coverage makes Chao1 and the estimated Shannon diversity lower bounds;
  - the methods paragraph says how diversity was measured, with its references; remote control's
    `measure_diversity` measures it and answers the main measures.
- **Rarefaction and extrapolation (wave 2, slice 5).** The Repertoire view's **Rarefaction** part
  (`internal/diversity`, `api_rarefaction.go`):
  - each sample's clonotypes, Shannon diversity and Simpson diversity (the Hill numbers of order 0,
    1 and 2) and its sample coverage at any size up to twice its own, rarefied below it and
    extrapolated above (Chao et al. 2014), and the Hill numbers at any sample coverage (Chao and
    Jost 2012); each with a 95% interval from the same 200 bootstrap replicates as the measures;
  - the samples compared at a level suggested for them, reached by rarefaction alone: the lowest
    sample coverage among them, or the smallest sample's size. iNEXT's defaults, extrapolated to
    twice a sample's size, are offered beside them, with a warning where a sample is extrapolated;
  - the curves by size and by coverage (solid where rarefied, dashed where extrapolated, a dot at
    the sample, bands for the intervals), how each sample's coverage rises with size, on a
    logarithmic size axis where the samples' sizes differ more than tenfold, and a table of each
    sample at the level chosen;
  - a million-cell sample's curves in about 2 s: each clonotype's subsample expectation is summed
    outward from its distribution's mode, where iNEXT sums every size;
  - the methods paragraph says how the samples were rarefied and compared, with its references;
    remote control's `rarefy` computes the curves and answers each sample at a level.
- Both parts of the Repertoire view warn when abundance is in reads: each cell is counted many times
  over, so few clonotypes are seen once and every sample looks complete.
- Diversity and rarefaction are asked for, computed, kept and read back alike (`api_analysis.go`);
  diversity results kept before keep their ids.
- **Gene usage, spectratypes and composition (wave 2, slice 6).** The Repertoire view's **Genes**
  and **Junctions** parts (`internal/usage`, `api_usage.go`, `web/ui/repertoire-usage.js`):
  - each sample's V, D, J and C genes and their families (IMGT's subgroups), V–J pairs, junction
    lengths and their mean, and amino acids, overall and at each IMGT position of the commonest
    lengths (Lefranc et al. 2003); each as a share of its clonotypes (each counted once) and of its
    abundance;
  - a clonotype's genes are its rearrangements', in proportion to their counts; a call tied between
    genes gives each an equal part; what was not called is said;
  - 95% intervals from 200 bootstrap replicates of each sample's clonotypes, the same in every run;
  - each condition (from the annotation or the metadata) summarized by the mean of its samples, each
    counted once, with their range;
  - heatmaps of the genes and of the amino acids, one gene across the samples, the V–J pairs of one
    sample, the spectratypes, the mean lengths, and the amino acids by position;
  - the methods paragraph says what was counted and how; remote control's `gene_usage` counts it
    and answers each sample's most used genes, commonest lengths and mean length.
- **Overlap (wave 2, slice 7).** The Repertoire view's **Overlap** part (`internal/overlap`,
  `api_overlap.go`, `web/ui/repertoire-overlap.js`):
  - every pair of samples: the clonotypes they share, Jaccard's index, Sørensen–Dice's and the
    overlap coefficient, as sampled and as expected with both samples rarefied to the smallest
    sample's size (exactly); Morisita's index and the Morisita–Horn index over their abundances;
    each sample's share of its abundance in clonotypes the other holds too;
  - 95% intervals from a bootstrap of the samples' cells, widened for the chance that decides which
    clonotypes two samples share (Horvitz–Thompson), on the logarithmic scale;
  - a heatmap of every pair by a measure, the pairs of one subject and of two (and of one condition
    and of two) summarized, every pair listed;
  - the public clonotypes: how many are found in one subject, two, and more (subjects from the
    annotation or the AIRR metadata; without them each sample its own, said so), and the most
    public listed;
  - the methods paragraph says what was measured, with its references; remote control's
    `measure_overlap` measures it and answers the pairs and the public clonotypes.
- **Figures (wave 2, slice 8).** Every plot of the QC and Repertoire views has a button that saves
  it as a figure standing alone (`web/lib/plot.js`, `web/ui/figures.js`):
  - **SVG** (vector), **PNG** at 300 dpi (recorded in its `pHYs` chunk) or **PDF** (one page of
    the figure's size, written by ClonoLoom: `POST /api/figure/pdf`, `api_figures.go`);
  - on white whatever the theme, with color-vision-friendly colors when they are on, its title and
    legend, and a caption: what was measured, the clonotype definition, the abundance unit, the
    samples, the filter, and the ClonoLoom version and day it was made.
- **Exports (wave 2, slice 8).** `internal/export`, `api_export.go`, `web/ui/exports.js`:
  - a dataset's rearrangements (those its filter keeps) as an **AIRR Rearrangement TSV**, each with
    its clonotype under the definition as `clone_id`, which reads back into the same clonotypes;
  - its **clonotype table** as CSV, one column of abundance per sample;
  - each **result kept** (diversity, rarefaction, gene usage, overlap and public clonotypes) as CSV,
    every value with its interval;
  - each in a zip with a `NOTICE.txt`: the files and their SHA-256, the filter, the definition, the
    unit, the method, ClonoLoom's version and commit, and each public data set's license and
    citation;
  - rows of data under a provider's terms of use (or no stated license) are written only after the
    window shows the terms and they are accepted, and the notice says not to share them; results
    computed from such data say to read the terms first;
  - from the Clonotypes view's and the Repertoire view's **Export** buttons, the File menu and the
    command palette.
- **Tables of other tools, through a mapping of their columns (wave 2, slice 8).** A CSV or TSV
  that no reader recognizes (`internal/reads/mapped.go`, `web/ui/mapping.js`):
  - opens a dialog with its first rows and a column chosen for each field (junction, genes, count,
    sample, locus, productivity, cell, clone, sequence ID), guessed from the columns' names
    (VDJtools', MiXCR's, AIRR's and others'); the count's unit said, never guessed;
  - **Check** reads the whole file through the mapping: each field's valid values, examples of
    the others, and notes (junctions with IMGT's anchors, in frame, counts not whole); **Open**
    checks, then opens;
  - productivity, where no column gives it, inferred from the junction (whole codons, no stop);
  - the mapping is kept with the dataset (reopened with it), is part of its ID and its cache key,
    and is stated in its methods paragraph;
  - `POST /api/datasets/inspect` and `/api/datasets/check`; remote control's `open_data` takes
    `mappings` by path.
- **Every release's project keeps opening.** `validation/projects/0.2.0` holds a project 0.2.0 saved
  (the small simulated study and a VDJtools table, with diversity, rarefaction, gene usage and
  overlap) with its data and results. A test opens it again with every later version, under the
  same dataset and result IDs, and compares each result with the one 0.2.0 wrote
  ([validation/projects/README.md](validation/projects/README.md)).
- The capture scenes of the Repertoire view's five parts; the README's and the guide's pictures
  captured again.
- **immunoSEQ's own clonality, Simpson clonality, entropy and largest frequency** are recomputed in
  QC's vendor checks: from each productive rearrangement's reads, as Adaptive computes them.
- `internal/random` (from the simulator) draws binomial and multinomial variates too.
- AIRR files written by R, every field in double quotes (the AIRR R package's example file among
  them), are read as the `airr` library and pandas read them.
- `NOTICE` names the data the program includes: HGNC's gene names (CC0), GRCh38's TRB segments
  (Ensembl, no restrictions), and usage and junction lengths computed from 10x Genomics' data
  (CC BY 4.0).

### Known limitations

- Results are not yet the same, bit for bit, on every processor: arm64 builds round some sums and
  logarithms differently from amd64's, in their last digits. Nothing a result reports changes; certificates that compare bit for bit
  (wave 8) need them the same.

### Fixed

- A plot's axis could end below its largest value, so that the tallest column (QC's junction
  lengths), the top of a line or curve, or the end of an interval was cut off: every axis now holds
  its values.
- A clonotype chosen before the Clonotypes table's first page arrived was not described in the
  inspector.
- The last tick label of a line or curve plot's horizontal axis (80,000, say) was cut off.
- Remote control's answers said "1 samples" and "1 pairs".

### Validation

- **The simulator against its targets and truth** (Go tests): every rearrangement well formed (its
  junction from its V's cysteine to its J's phenylalanine, a productive one in frame without a
  stop); productive junction lengths within 0.1 amino acids of the 10x data's mean and 0.15 of its
  standard deviation, and V usage within 0.03 of the model's in total variation, over 150,000
  rearrangements; the clonotypes samples show equal on average the truth's expectation at their
  depth (60 replicate samples); every row of every sample is its population clone, each sample's
  rows, templates and clonotypes are the truth's, and the truth's Simpson index and Hill numbers
  are those of its population tables; the same seed gives the same bytes, another seed other
  files; the random draws' means and variances (Poisson, negative binomial, gamma, uniform).
- **QC on the simulated standard study against its truth** (`validation/simulated_test.go`): twelve
  samples, 109,002 rows, no validator finding, every sample matched to its metadata; each sample's
  clonotypes the truth's count, and its sample coverage (Chao and Jost 2012) within 0.005 of the
  coverage a sample of its depth has on average (the largest difference 0.0047).
- **Diversity against iNEXT, immunarch and vegan** (`validation/diversity_test.go`,
  `validation/reference/diversity.R`): on 18 samples (Emerson et al. 2017's three immunoSEQ
  subjects and its AIRR Data Commons copy, the AIRR Community's test files, and the simulated
  standard study's twelve), every observed measure equals R's; the Hill numbers, Gini,
  Gini–Simpson, inverse Simpson and Chao1 with its interval equal immunarch's own functions';
  Chao1 with its interval, the asymptotic Shannon and Simpson diversities and the coverage equal
  iNEXT 3.0.2's to its three printed decimals; ACE equals vegan 2.7.6's. The bootstrap standard
  errors are 0.86 to 1.13 times iNEXT's (median 0.98). iNEXT's spider example reproduced (Girdled
  43.893, 13.826, 8.175; Logged 61.403, 16.337, 6.920).
- **Interval coverage on simulations** (`validation/coverage_test.go`): over 200 samples of a
  well-sampled repertoire (coverage 0.99), the 95% intervals held the truth in 96.0% (Chao1), 93.5%
  and 91.5% (the estimated Shannon and Simpson diversity) and 92.5–99.5% (those seen); of an
  undersampled one (coverage 0.62), in 92.0% for the estimated Simpson diversity and 91.5–92.5% for
  those seen, and never for Chao1 and the estimated Shannon diversity, which are lower bounds
  there. iNEXT's floor on the estimated Simpson diversity's interval (at the value seen) made it
  hold the truth in under half the samples: ClonoLoom does not apply it (research.md §5.3).
- **Rarefaction against iNEXT** (`validation/rarefaction_test.go`,
  `validation/reference/rarefaction.R`): on the same 18 samples, at iNEXT 3.0.2's 40 sizes along
  each curve, every Hill number and coverage equals iNEXT's to 10⁻⁹, but the extrapolated Shannon
  diversity, within 8·10⁻⁸ (its extrapolation magnifies iNEXT's rounding of the value seen, which
  ClonoLoom computes to 10⁻¹⁵); at each coverage, the same to what iNEXT's numerical minimization
  leaves of the size there. Two cohorts compared at iNEXT's default levels: the same levels and
  values. The bootstrap standard errors are 0.85 to 1.26 times iNEXT's (median 1.01, over 4,240).
- **The curves against the truth on simulations** (`validation/coverage_test.go`, 200 samples of
  each repertoire): rarefied to half a sample's size, every value's 95% interval held the truth in
  90–98% of samples, in a nearly complete repertoire and in one of many rare clonotypes sampled to
  coverages of 0.62, 0.83 and 0.97. Extrapolated to twice the sample, the values were within 3% of
  the truth on average (the Simpson diversity within 0.3%), but where rare clonotypes abound only
  the Simpson diversity's intervals held the truth as often as they say (92–95.5%); those of the
  coverage, the clonotypes and the Shannon diversity held it in 0–87%. Hence ClonoLoom suggests
  levels reached by rarefaction first (research.md §5.4).
- The interpolation equal to its definition summed term by term (to 10⁻¹⁰) and to every term of each
  hypergeometric summed in 256-bit arithmetic (to 10⁻¹³); the extrapolation approaching the
  asymptotes and equal to Chao et al. 2014's Eq. 9 for clonotypes; size and coverage inverting each
  other; the curves the same with one processor or many.
- **Gene usage against R** (`validation/usage_test.go`, `validation/reference/usage.R`): on the 15
  development samples in AIRR format (the ADC copy of Emerson et al. 2017's subject, the AIRR
  Community's test files, the simulated study's twelve), all 8,277 shares (genes and families of
  each segment, V–J pairs, lengths, the mean length, amino acids; by clonotypes and by abundance)
  equal an independent R implementation's to 10⁻¹², with no name either lacks.
- **Gene usage against the simulator's truth**: each sample's expected shares from its subject's
  clones and the templates drawn; of 2,308 intervals (V, D and J genes, lengths, amino acids, both
  weightings), 97.4–100% held them. The largest development sample (69,535 clonotypes) is counted,
  with its bootstrap, in 0.7 s.
- The IMGT positions of junctions of 5, 11, 13, 14, 15, 17 and 18 residues; shares, families, ties,
  pairs and conditions by hand on a small table; the bootstrap the same with one processor or many;
  diversity's and rarefaction's result ids unchanged by the shared analysis code.
- **Overlap against immunarch, vegan, R and scirpy** (`validation/overlap_test.go`,
  `validation/reference/overlap.R`, `validation/reference/scirpy_overlap.py`): on the simulated
  study's 66 pairs and Emerson et al. 2017's three subjects' 3, every measure equals immunarch's
  functions' (Jaccard, Sørensen–Dice, the overlap coefficient, Morisita–Horn), vegan's (Morisita–Horn
  and Morisita's index; vegan stops the latter at 1) or R's (shared clonotypes, the rarefied
  measures, the shares of abundance) to 10⁻⁹, and the clonotypes by number of subjects equal R's; on
  four single-cell T-cell runs opened as one dataset, every pair's shared clonotypes and Jaccard index
  equal scirpy 0.25.1's `repertoire_overlap`, by both chains and by the beta chain alone.
- **Overlap against the simulator's truth** (five simulated studies, 330 pairs): the 95% intervals
  held the expected shared clonotypes in 95.2% of pairs, the rarefied ones in 97.0%, and the
  repertoires' overlap in 93.9% for Morisita's index (every pair of one subject) and 85.8% for the
  Morisita–Horn index, which reads two samples of one repertoire about 0.98 (4 of 30 pairs of one
  subject). A bootstrap of the cells alone had held the shared clonotypes in 64% of pairs.
- The rarefied shared clonotypes equal 200,000 random subsamples' mean; overlap by hand on a small
  table; the bootstrap the same with one processor or many; the frequencies the repertoire is
  estimated to hold sum to the sample coverage.
- **Exports read back** (Go tests): the AIRR TSV of a dataset read again by ClonoLoom's AIRR reader
  gives the same clonotypes with the same abundances; the clonotype CSV's columns and values, and a
  diversity result's; the filter's rows kept; the notice of the user's own data and of data under
  terms; such rows refused without `accept=1` (409, with the licenses) and written with it.
- **Figures** (Go and Node tests): the PDF's page size and image, every object at the offset its
  cross-reference table gives, its title in UTF-16 where it is not ASCII; a body that is not a PNG,
  or no size, refused; the PNG's resolution chunk written once, after its header; the figure's SVG
  with its title, legend, caption and colors written in.
- **Mapped tables** (Go tests): VDJtools', MiXCR's and a semicolon-separated sheet's columns guessed;
  the check's counts and notes; the rows read, their productivity inferred and their unit as said;
  one file under two mappings two datasets; 2.8 million fuzzed tables read without a crash.
- **The 0.2.0 project opened again** (`projects_test.go`): its two datasets under their IDs, its five
  results under theirs, every value equal to the fixture's (the same bytes on arm64; on amd64, within
  0.1% of a relative 10⁻⁹); the project parses and saves back to the same bytes
  (`web/lib/project.test.mjs`).
- **immunoSEQ's clonality reproduced**: Adaptive's productive clonality, Simpson clonality, entropy
  and largest frequency, from the rows' reads, on Emerson et al. 2017's three exports (to seven
  significant digits; templates give other values).
- **A cohort of 100 samples (a million rearrangements) reopened from the cache in 118 ms**, its
  files hashed again, against 1.24 s to read them (budget 2 s; `validation/budget_test.go`).
- **The cache's tables are the tables read** (Go tests): every column, every string, the
  sources, problems and unit, on synthetic files and on the development files of every format;
  a moved file answered from the cache at its new place, a renamed or changed one read again; a
  damaged entry removed and written again; the cache kept within its limit; reopening a dataset
  through the API uses it.
- The jobs and artifacts packages' own tests, ported with them (state, persistence, resumption,
  cancellation; staging, promotion, checks, verification).
- Go tests: the page performs exactly the actions the program lists; actions reach the page and
  its answers the caller; no page, an unknown action, a page of another run, a missing or wrong
  token and another computer are refused; remote control is off unless asked for; `remote.json` is
  private and removed only by the run that wrote it.

## 0.1.0 (2026-10-21)

ClonoLoom's first release, wave 1 of the [roadmap](clonoloom-spec/roadmap.md): open the output of
an annotation pipeline, check it, see each sample's quality, and define clonotypes explicitly,
with a project that records where the data came from.

### Added

- **The program and its window (wave 1, slice 1).**
  - One self-contained program (`clonoloom`) for macOS, Linux and Windows, x64 and ARM64.
  - It serves its embedded web app on 127.0.0.1, port 8860 or the next free one up to 8879, and
    opens it in a desktop window.
  - Its ports are clear of the sibling programs' (Proteoscope 8765–8814, CytoWeave 8770–8789,
    MaveScape 8820–8839, SituCraft 8840–8859), and a test keeps them so.
  - A second launch hands its paths to the running ClonoLoom.
  - The host and its security are adapted from SituCraft 0.9.0: a Host-header check against DNS
    rebinding, a same-origin API, a Content-Security-Policy and cross-origin isolation.
  - The content-addressed library is adapted from SituCraft 0.9.0 too, and holds projects.
- **Items named on this computer.**
  - On the command line or through **Open data** (a path), ClonoLoom recognizes:
    - rearrangement tables (`.tsv`, `.csv`, `.txt`, gzipped or not), AIRR metadata (`.json`,
      `.yaml`), gene expression (`.h5ad`, `.h5mu`, `.h5`), matrices, FASTA files and project
      archives (`.clz`);
    - Cell Ranger V(D)J outputs by their folder: a `vdj` run's folder or its `outs/`, and a
      `multi` run's per-sample `vdj_t`, `vdj_b` and `vdj_t_gd` folders;
    - folders of these, in natural order.
  - An output folder counts as one item: ClonoLoom never walks into it, serves it whole or copies
    it into the library.
  - The items are listed under **On this computer** in the project tree. Reading them comes in
    the next slice.
- **The workbench shell.**
  - A view rail in the order of the work: Data, QC, Clonotypes, Repertoire, Tracking,
    Specificity, B cells, Cells, then Cohort, Reports and Jobs. Each view says what it will do
    until it is built.
  - A start page with recent projects.
  - The project tree, an inspector, and a bottom drawer with the project's history and the
    session's log.
  - A status bar, the command palette (⌘K), undo and redo.
  - Light and dark themes, and color-vision-friendly colors.
  - Without its program, the page says how to start ClonoLoom: a web page alone cannot read data
    where it lies on the computer.
- **The project document** (`clonoloom-project` version 1): datasets, results, decisions, saved
  views and history. It is saved automatically to the library, and refused with a plain message
  when it is not a ClonoLoom project or was saved by a newer ClonoLoom.
- **The build commit** in the status bar and `/api/info`. Release builds embed it; other builds
  take Go's version-control stamp.
- **Installers** (`install.sh`, `install.ps1`) that check the download's SHA-256 and version.
- **CI and releases.**
  - CI tests each commit once (go vet, race tests, the six-platform cross-compile, the browser
    modules, validation, installer syntax). A pull request shows its branch's checks, and an
    identical merge is not tested again.
  - A release workflow builds the six binaries with their checksums once CI has passed on exactly
    the tagged files.

- **Reading repertoires (wave 1, slice 2).** ClonoLoom's own readers, every one producing the same
  columnar table of rearrangements (about 58 bytes a row, with junctions and gene calls stored
  once each):
  - **AIRR Rearrangement TSV** (schema 1.3 to 2.0): every typed column checked on every row, the
    schema's required columns reported when missing, samples by `repertoire_id`, counts by
    `umi_count`, `duplicate_count` or `consensus_count`;
  - **Cell Ranger V(D)J** contig annotations (Cell Ranger 3 to 9, both spellings of booleans,
    Cell Ranger 9's `sample` column), and output folders of `vdj` and `multi` runs, their
    clonotypes, consensus and metrics kept beside them;
  - **immunoSEQ** exports v2 and v3: the junction cut from the read by its index, `In`/`Out`/`Stop`,
    templates with a fallback to reads for a file whose templates are missing or −1, and gene
    names mapped from Adaptive's to IMGT's (ties for genes resolved only to their family);
  - **AIRR Repertoire metadata** in JSON (schema 1.3 and 2.0), matched to samples by
    `repertoire_id`;
  - gzip recognized by its first bytes; CRLF and byte-order marks; lines over 16 MB refused.
- **Gene names in IMGT nomenclature** (`internal/genes`): locus, segment, family, gene and allele;
  shared, orphon, duplicated and constant genes; ambiguous calls kept as sets; the list of 676
  human TR and IG genes generated from HGNC (CC0).
- **The validator** (`internal/validate`): findings with stable codes (fatal, warning, note), counts,
  example rows with their line and a remedy: the readers' problems; junctions that are not
  sequences, do not translate, are out of frame or have a stop while productive, or lack their
  anchors; gene calls that are not IMGT names or of different loci (TRA and TRD counted as one);
  missing and zero counts; duplicate sequence IDs; cells with more than two productive chains of a
  locus; samples with over 40% non-productive abundance; an unknown unit. Shown in the Data view
  in the next slice.
- **The abundance unit** of every dataset: cells, templates, reads, UMIs, sequences, or unknown
  when the format does not say (AIRR `duplicate_count` alone), never guessed.
- **Development data** (`validation/sources.json`, `validation/fetch.mjs`, adapted from SituCraft
  0.9.0): four 10x Genomics V(D)J datasets (Cell Ranger 4.0, 6.1 and 9.0) and the AIRR Community's
  test files, all CC BY 4.0, for CI; three Emerson et al. 2017 samples in immunoSEQ's format (taken
  by HTTP range from the 19.9 GB archive) and two AIRR Data Commons repertoires, for the
  maintainer's validation only, under their providers' terms. `--ci` fetches the open ones; an
  AIRR Data Commons repertoire is fetched by its ADC API query.
- [docs/FORMATS.md](docs/FORMATS.md): what each reader reads and how.

- **Datasets (wave 1, slice 3).** The Data view:
  - files and folders chosen in ClonoLoom's own **Open data** dialog of this computer's folders
    (adapted from SituCraft 0.9.0), named on the command line, or dropped on the window (added to
    the library and read from there) open together as one dataset: a cohort's exports, a run's T
    and B cells; a folder of tables opens as one;
  - each dataset's files with their SHA-256, taken while they are read; the project records them,
    opens the dataset again from them, says when a file changed, and finds one that moved by its
    fingerprint near where it was (or lets the user choose it, and checks it);
  - the abundance unit, confirmed by the user when the files leave it open;
  - the validator's report: findings by severity with their counts, share of rows, examples
    (file, line, sequence, sample, value), stable codes and what to do;
  - the samples, their rows, productive share, abundance and loci, and their annotation (subject,
    time point, tissue, cell subset, condition, batch, HLA), typed, suggested from file names
    (after CytoWeave's suggestions) or filled from AIRR Repertoire metadata and immunoSEQ's sample
    tags (HLA types, CMV status), with each value's origin kept;
  - the dataset's files in the inspector.
- A row's locus, when its file names none, is the one its J (else V, else C) gene names.
- **QC (wave 1, slice 4).** The QC view, every number computed by the program (`internal/qc`):
  - for bulk samples: abundance in the dataset's unit, rows, the shares of productive, out-of-frame,
    stop-codon and otherwise non-productive abundance, clonotypes (productive rearrangements by
    locus, amino-acid junction, V gene and J gene), singletons, doubletons, **sample coverage**
    (Chao and Jost 2012, as iNEXT computes it), the largest clonotype's share, how often V and J
    calls name one gene, reads per template or UMI, and junction lengths;
  - for single cells: cells by their productive chains (paired, one arm, none, more than two of a
    locus), cells with each locus, UMIs per contig;
  - plots of the status shares, junction lengths and chain configurations, each described in words;
  - **the vendor's numbers, recomputed**: Cell Ranger's metrics summary (found in a run's folder or
    beside a contig file) and immunoSEQ's sample columns, each beside ClonoLoom's count from the
    rows;
  - **filters as versions**: productive only, a minimum count, loci, cells only, high-confidence
    contigs only, the most productive chains of a locus in a cell, samples left out; saved with a
    reason, never deleted, each rule's rows left out counted; the dataset's analyses use the version
    chosen;
  - what each number measures, in plain words.

- **Clonotypes (wave 1, slice 5).** The Clonotypes view (`internal/clonotype`):
  - **definitions** named and stated in words: nucleotide or amino-acid junction; V and J by allele,
    gene, family or not at all; each locus separately, one locus, or paired αβ, γδ or heavy and
    light chains; the source's own clonotypes (Cell Ranger's `raw_clonotype_id`, an AIRR
    `clone_id`) as given; presets, an editor, and definitions saved for the project's other
    datasets;
  - **single-cell rules**: the primary chain of each locus by UMIs (then reads, then contig ID), or
    every chain; cells with one arm grouped with cells of that arm alone, or left out;
  - each clonotype's **ID** made from the definition's meaning and its chains, the same in every
    run;
  - the table sorted by total, samples, junction, genes or any sample's abundance, searched by
    junction, gene or ID, a million rows scrolling as they come into view, each sample's abundance
    in columns for up to eight samples;
  - a clonotype in the inspector: its chains, its abundance and frequency in each sample, and the
    rearrangements or contigs it is made of, with their genes as given;
  - the definition and the abundance unit of the dataset in focus in the status bar.
- AIRR rows are counted by the first count they have (UMIs, duplicates, consensus reads): a count
  column present but empty no longer leaves every row without a count; rows counted by different
  columns leave the unit unknown, with a finding.

- **Example data (wave 1, slice 6).** The **Example data** dialog, from the start page, the Data
  view, the project menu and the command palette (`internal/examples`, adapted from SituCraft
  0.9.0):
  - six public datasets: four 10x Genomics V(D)J runs (human PBMC 1k and 10k, mouse splenocytes 1k
    and 5k; T and B cells each), the AIRR Community's valid and invalid test files (CC BY 4.0), and
    three Emerson et al. 2017 subjects in immunoSEQ's own format (immuneACCESS terms of use);
  - each listed with its source, size, license and citation, and downloaded only when asked, from
    its publisher; data under a provider's terms of use only once the user has read and accepted
    them;
  - every file checked against the size and SHA-256 that `validation/sources.json` records, and a
    download put in place only when all are: a changed or interrupted one leaves nothing behind;
  - members of a large remote zip (the 19.9 GB immuneACCESS archive, ZIP64) taken by HTTP range,
    each checked by its CRC-32 too;
  - only the publishers' hosts reached, and redirects followed only to them over HTTPS;
  - `--offline` downloads nothing; `--examples-dir` chooses the folder (default
    `<data dir>/examples`);
  - an example opens as a dataset with a short name, and **any dataset whose files are an
    example's**, however they were downloaded, carries its source, license and citation, shown on
    its card and in the inspector and kept in the project for the methods.

- **The record (wave 1, slice 7).**
  - **A methods paragraph per dataset** (`web/lib/methods.js`, adapted from SituCraft 0.9.0),
    from a dataset's menu, the inspector or the command palette: written from the project's record
    and what the program computes of the dataset (the data's source, license and date; the format
    and reader; the unit; the validator's findings; the filter with the rows it kept; the
    clonotype definition with its ID and the clonotypes it gives; ClonoLoom's version and commit),
    with numbered references (the AIRR Community's standards, IMGT's nomenclature and junction,
    HGNC's gene list, the data's own citation). Only what was done is said. **Copy** takes the
    paragraph with its references.
  - [docs/GETTING-STARTED.md](docs/GETTING-STARTED.md): a first study, view by view.
  - The README's pictures, made by `docs/capture/capture.mjs` (adapted from SituCraft 0.9.0, with
    its Chrome DevTools driver `validation/cdp.mjs`) on the open example data, in the light and
    dark themes.
- **Fixed.**
  - The plots' labels inherited a stroke from the layout around them (two classes shared a name)
    and looked faint and outlined; their text is now larger, and the last tick ("100%") fits.
  - Long sample names no longer push the QC tables' columns, or the clonotype inspector, out of
    view.
  - AIRR metadata in YAML, which are read from wave 9, are refused with that reason rather than
    read as a table.
  - **AIRR Repertoire metadata are part of their dataset**: fingerprinted like its tables, recorded
    in the project, read again when the project reopens (they were lost after a restart), found
    with the tables when they move, listed in the inspector, and part of the dataset's ID. Metadata
    dropped on the window are read as metadata, not as a table.
  - **Sample coverage is not estimated under a minimum count**: coverage is estimated from the
    clonotypes seen once and twice, which a minimum count above one leaves out, so it always came
    out as 1. The QC view says why it is not given.
  - A dataset whose files are any public data set `sources.json` records (offered as an example
    or not, such as the AIRR Data Commons' copy of Emerson et al.) carries its source, license and
    citation.

### Validation

- **Readers against independent readers:** every column ClonoLoom reads from 25 development files
  (AIRR, Cell Ranger and immunoSEQ) equals what the `airr` library 2.0.0 and pandas 3.0.6 read
  (`validation/readers_test.go`, `validation/reference/read_repertoires.py`).
- **A million rearrangements** of an immunoSEQ file read and validated in 1.5 s (budget 5 s), at
  58 bytes a row plus the junctions (budget 64).
- **Fuzzing:** every reader and the metadata reader fuzzed (12 million inputs without a fault).
- **Fingerprints:** the SHA-256 each reader takes while reading equals the checksum `sources.json`
  records for every development file: every byte was read, and none was changed.
- **Datasets' API:** files opened, opened again as the same dataset, refused with their reason, a
  moved file found again by its fingerprint, another computer refused (Go tests).
- **`node --test`:** dataset records, unit confirmation, annotation with origins, suggestions
  from names and metadata; filter versions; the plots.
- **QC against independent computations:** on six bulk samples (immunoSEQ, AIRR Data Commons and
  the AIRR Community's files), rows, abundance, productive share, clonotypes, singletons,
  doubletons and the largest share equal an R implementation's (data.table), and sample coverage
  equals iNEXT 3.0.2's, to 10⁻¹² (`validation/qc_test.go`, `validation/reference/qc.R`).
- **Cell Ranger's metrics reproduced:** cells, cells with a productive pair and with a productive
  chain of each locus, as counts and shares, exactly as Cell Ranger 4.0, 6.1 and 9.0 reported them
  for six T- and B-cell runs.
- **Clonotypes against scirpy 0.25.1:** on eight Cell Ranger runs (T and B cells, Cell Ranger 4.0,
  6.1 and 9.0), ClonoLoom's paired definitions give exactly scirpy's partitions of cells, by
  nucleotide and amino-acid junction, with and without the V gene (`validation/clonotypes_test.go`,
  `validation/reference/scirpy_clonotypes.py`); Cell Ranger's own clonotypes have the sizes its
  `clonotypes.csv` gives.
- **Bulk clonotypes** equal the R implementation's in number and largest abundance on six
  samples, under three definitions (amino-acid junction with V and J genes, nucleotide junction
  with V and J genes, amino-acid junction alone).
- **A page of a million clonotypes**, sorted and searched, in 37 ms (budget 100 ms).
- immunoSEQ's own row counts reproduced; its template totals by frame type do not always sum from
  its rows (pandas sums them as ClonoLoom does), which the QC view says.
- **Example data from their publishers:** the six examples downloaded as the program downloads
  them, every file equal to its record, and each of their eleven datasets read
  (`validation/examples_test.go`, with `CLONOLOOM_FETCH_EXAMPLES=1`); the three immuneACCESS
  samples taken from the 19.9 GB archive in under 3 s.
- **Methods text** (`node --test`): each sentence present only when its step was taken, the
  references numbered in the order cited, the same inputs giving the same text.
- **The downloader** (Go tests against a local server): a changed, longer or missing file refused
  with nothing left behind; an archive of another size refused; members taken by range without
  reading the rest; a redirect to another host refused; nothing offline, nothing under terms not
  accepted, nothing from another computer.

- **Go tests of the host:**
  - security headers; foreign Host headers and cross-origin requests refused;
  - the project library's round trips; files stored under the hash the program computes; range
    requests;
  - Cell Ranger outputs and files found in folders in natural order, with an output folder's
    insides never listed, served or copied;
  - every web module embedded; the commit reported;
  - the port range clear of the siblings'.
- **`node --test` of the shared modules:** SHA-256, color-vision simulation and the project
  document.
- **`validation/run.mjs accessibility`:**
  - every text-on-surface pair of both themes reaches WCAG AA contrast, with color-vision-friendly
    colors off and on;
  - the palettes and status colors stay apart in protanopia, deuteranopia and tritanopia;
  - the diverging maps' two ends stay apart in every vision.
- **A release dry run** built the six binaries (7 MB each). The macOS one printed
  `clonoloom 0.1.0`, served the embedded app with its commit in `/api/info`, and refused a
  foreign Host.
