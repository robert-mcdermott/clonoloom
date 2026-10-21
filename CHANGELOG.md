# Changelog

## 0.1.0 (unreleased)

ClonoLoom's first release, wave 1 of the [roadmap](clonoloom-spec/roadmap.md): open the output of
an annotation pipeline, check it, see each sample's quality, and define clonotypes explicitly,
with a project that records where the data came from. Built slice by slice; this section grows
with each.

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
