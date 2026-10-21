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

### Validation

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
