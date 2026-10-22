# Getting started

A first study in ClonoLoom 0.1.0: open data, check them, look at each sample's quality, define
clonotypes, and write down what was done. It takes a few minutes with the example data, and works
the same way with your own.

## Start ClonoLoom

```sh
clonoloom
```

ClonoLoom opens in its own window (or your default browser, with `--window browser`) on the start
page. It runs on your computer only: nothing is uploaded, and it reaches no other computer unless
you ask it to download example data (`--offline` stops even that). Stop it by closing the window,
or with Ctrl+C in the terminal that started it. The flags are in the [README](../README.md#getting-started).

## Open data

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="images/examples-dark.webp">
  <img alt="The Example data dialog: six public datasets, each with its description, source, license, citation and size, and a button to download it" src="images/examples-light.webp">
</picture>

- **Your own data.** **Open data** shows the folders of this computer: choose AIRR rearrangement
  tables, a Cell Ranger V(D)J output folder (a `vdj` run, its `outs/`, or a `multi` run's
  per-sample folders), immunoSEQ exports, or a folder holding a cohort's files. What you choose
  together opens as **one dataset**: a cohort's exports, say, or a run's T cells. You can also
  name the files on the command line (`clonoloom run1/outs`). ClonoLoom reads each file where it
  is and never changes, copies or moves it. Files dropped on the window are copied into the
  library instead, since a browser does not say where a dropped file lies.
- **Example data.** **Example data** lists public datasets: 10x Genomics' single-cell runs of human
  PBMCs and mouse splenocytes, the AIRR Community's test files, and three subjects of Emerson et
  al. 2017 in immunoSEQ's format. Each shows its source, size, license and citation before you
  download it. Data under a provider's terms of use (the immuneACCESS data) need you to read and
  accept the terms first. Every file is checked against the checksum recorded when ClonoLoom was
  released. **Open T cells** (or B cells) then opens it as a dataset, which carries its license
  and citation from then on.

To follow along, download **Human PBMC from a healthy donor, 1k cells** (2.8 MB) and open its
T cells.

## Check the data

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="images/data-dark.webp">
  <img alt="The Data view: the dataset card with its format, unit and license; the validator's findings; the samples table; and in the inspector the file's SHA-256, its source and license, and the files read beside it" src="images/data-light.webp">
</picture>

The **Data** view shows each dataset of the project:

- **The card**: its format, rows, samples, whether it is single-cell or bulk, and its **unit**:
  cells, templates, reads or UMIs. When the files do not say whether reads or molecules were
  counted (an AIRR file with `duplicate_count` alone), ClonoLoom does not guess: choose the unit,
  and until you do, results say "counts".
- **Validation**: each finding with its severity, how many rows it concerns, examples (file, line,
  value) and what to do. A **fatal** finding stops analysis; a **warning** is shown with every
  result that uses the dataset; a **note** informs. Each has a stable code
  (`junction-anchors`, `locus-mismatch`, …) you can cite.
- **Samples**: rows, productive share, abundance and loci, and the annotation that later waves
  compare by: subject, time point, tissue, cell subset, condition, batch, HLA. **Suggest from file
  names** and **Fill from metadata** (AIRR Repertoire metadata, immunoSEQ's sample tags) propose
  values; each keeps where it came from.
- **The inspector** (right): the files with their SHA-256 fingerprints, what was read beside them
  (Cell Ranger's clonotypes and metrics), and the data's source and license.

If a file later moves, **Locate the files** finds it again by its fingerprint near where it was,
or lets you choose it and checks it. If a file's contents change, the project says so instead of
reading other data under the same name.

## Look at quality

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="images/qc-dark.webp">
  <img alt="The QC view: a table of cells by their productive chains, and plots of the contigs' status, the cells' chains and the junction lengths" src="images/qc-light.webp">
</picture>

**QC** measures every sample, every number computed by ClonoLoom from the rows:

- bulk samples: abundance, the productive, out-of-frame and stop-codon shares, clonotypes,
  singletons, **sample coverage** (Chao and Jost 2012, as iNEXT computes it), the
  largest clonotype's share, how often V and J calls name one gene, and reads per template or UMI;
- single cells: cells by their productive chains (a pair, one arm, none, more than two of a
  locus), cells with each locus, UMIs per contig;
- **the vendor's numbers, recomputed**: Cell Ranger's metrics summary and immunoSEQ's sample
  columns, each beside the number ClonoLoom counts from the rows, with whether they agree.

**What these numbers mean** explains each in plain words.

To leave rows out, make a **filter**: **New filter…**, choose its rules (productive only, a
minimum count, loci, cells only, high-confidence contigs, at most so many chains of a locus in a
cell, samples left out), and say why. A filter is never deleted: editing it saves a new version,
and the rows each rule leaves out are counted. The dataset's analyses use the version you choose;
the data themselves are never changed.

## Define clonotypes

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="images/clonotypes-dark.webp">
  <img alt="The Clonotypes view: the definition stated in words above a table of clonotypes with their junctions, genes, abundance and frequency, and one clonotype's chains and cells in the inspector" src="images/clonotypes-light.webp">
</picture>

A clonotype is whatever the definition says it is, so ClonoLoom makes you choose one and states it
everywhere clonotypes are counted (also in the status bar):

- **Presets**: the amino-acid junction with V and J genes (the default for bulk data), the
  nucleotide junction with V and J genes, the amino-acid junction alone, paired αβ chains, paired
  heavy and light chains (the defaults for single cells), or the source's own clonotypes (Cell
  Ranger's, an AIRR file's `clone_id`).
- **Edit…** to choose the junction (nucleotide or amino acid), V and J at allele, gene or family
  level or not at all, which chains, and for single cells whether a cell's primary chain of each
  locus or every chain counts, and what to do with cells missing a chain. Save it under a name to
  use it for the project's other datasets.

Each clonotype has an ID made from the definition and its chains, the same in every run. Sort by
any column, search by junction, gene or ID, and choose a clonotype to see its chains, its
abundance in each sample and the contigs or rearrangements it is made of.

## Measure diversity

**Repertoire** measures each sample's diversity under the dataset's clonotype definition and
filter: **Measure diversity** runs it in the program (a job you can follow in **Jobs**) and keeps the
result with the project. Each measure has a 95% interval:

- the **Hill numbers**: clonotypes seen (order 0), the Shannon diversity (order 1) and the Simpson
  diversity (order 2), and their profile up to order 4;
- **clonality** (Adaptive's, 1 − Pielou's evenness) and **Simpson clonality** (√Σp²), evenness,
  Gini's coefficient, the largest clonotypes' shares, and **clonal space homeostasis**;
- estimates of the whole repertoire each sample was drawn from: its richness (**Chao1**, ACE) and
  its Shannon and Simpson diversity.

Choose a measure to see it across the samples, colored by condition. Read the coverage first:
where a sample's coverage is below about 0.9, its Chao1 and estimated Shannon diversity are lower
bounds, and the view says so.

## Compare samples at equal coverage

A deeper sample holds more clonotypes because it is deeper. **Repertoire › Rarefaction** computes
what each sample would show at any size up to twice its own, and at any **sample coverage** (the
share of the repertoire's cells whose clonotype the sample holds): **Rarefy and extrapolate** runs
it in the program and keeps the result. Then:

- **Each sample at** a level: by default the lowest coverage among the samples, every sample
  rarefied to it, none extrapolated (or the smallest sample's size). The clonotypes, Shannon
  diversity and Simpson diversity there, each with its 95% interval, are the samples compared
  fairly. Comparing at equal coverage is better than at equal size: a sample twice as deep is not
  just more of the same repertoire.
- The curves by size and by coverage: solid where rarefied, dashed where extrapolated, a dot at the
  sample itself; and how each sample's coverage rises with size.
- Two more levels extrapolate to twice a sample's size, as iNEXT does by default. In ClonoLoom's
  simulations of repertoires with many rare clonotypes, values extrapolated that far were within
  3% of the truth, but their intervals were narrower than that; the view says so when a sample is
  extrapolated.

The curves and their intervals come from the same bootstrap replicates as the measures. They need
counts of cells or molecules: in **reads**, each cell is counted many times over, so few clonotypes
are seen once, every sample looks complete, and the view warns.

## See what the clonotypes are made of

**Repertoire › Genes** and **Junctions** show what each sample's clonotypes are made of; **Count
genes and junctions** runs it in the program and keeps the result. Each share is of the clonotypes
(each counted once) or of the abundance, with its 95% interval, for each sample or for each
condition (the mean of its samples, each counted once, with their range):

- **Genes**: the V, D, J or C genes, or their families, as a heatmap of every sample; one gene
  across the samples; and the V–J pairs of one sample. A call between several genes gives each an
  equal part, and what was not called is said.
- **Junctions**: the spectratype (each junction length's share) and the mean length; each amino
  acid's share of the junctions; and, for the commonest lengths, the amino acids at each IMGT
  position, from the conserved cysteine at 104 to the phenylalanine or tryptophan at 118.

## Compare samples with each other

**Repertoire › Overlap** measures how much every pair of samples overlaps; **Measure overlap** runs
it in the program and keeps the result. Each value has its 95% interval.

- **Morisita's index** weighs each clonotype by its abundance: 1 for one repertoire sampled twice,
  0 for repertoires that share nothing, at any depth. The Morisita–Horn index, which immunarch and
  vegan give, is offered too, but it reads two samples of one repertoire below 1 when they are
  shallow.
- **Jaccard's index**, **Sørensen–Dice's** and the **overlap coefficient** count clonotypes seen,
  and grow with depth: take them **rarefied**, as expected with both samples brought down to the
  smallest sample's size.
- The pairs of one subject and of two (or of one condition and of two) are summarized, every pair
  is listed, and the **public clonotypes** (found in more than one subject) are counted and listed.
  Subjects come from the samples' annotation (or their AIRR metadata): without them, each sample
  counts as its own subject, and the view says so.

## Write down what was done

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="images/methods-dark.webp">
  <img alt="A methods paragraph with numbered references, ready to copy" src="images/methods-light.webp">
</picture>

- **The project** saves itself to the library as you work; **Open a saved project** (⇧⌘O) lists
  them. Undo and redo cover every change.
- **History and log** (the status bar, or ⌘J) lists what was done to the project, and the
  session's messages.
- **Methods paragraph** (a dataset's menu or the inspector) writes a paragraph for a manuscript
  from the project's record: the data's source and license, how ClonoLoom read and checked them,
  the unit, the filter with what it kept, the clonotype definition, the diversity measured, the
  rarefaction and the levels the samples were compared at, the genes and junctions counted, the
  overlap measured, and ClonoLoom's version, with numbered references. Only what was done is said. **Copy** takes it with its references.

## Next

ClonoLoom grows release by release ([roadmap](../clonoloom-spec/roadmap.md)): figures and AIRR
exports complete 0.2.0; following
clonotypes over time in 0.3.0; specificity, B-cell lineages, cells and cohorts after that.
