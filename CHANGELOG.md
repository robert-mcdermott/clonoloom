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
  from names and metadata.

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
