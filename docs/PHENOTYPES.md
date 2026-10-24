# Clonotype overlays and phenotypes

In **Cells**, open expression evidence with an explicit receptor dataset and sample. Under
**Clonotypes and phenotypes**, choose imported coordinates or a saved computed map of that
same expression input. Choose a supplied annotation such as cluster, cell type or state,
a size population and inclusive size breakpoints, then **Compute clonotype phenotypes**.
The cell-state study includes a guided example. This feature is in development for 0.6.0.

## What is counted

Each result joins one expression input to one receptor dataset/sample by **exact barcode**.
It never strips barcode suffixes, merges samples, combines independently fitted maps or
infers cell types. AnnData/MuData observations can supply labels and coordinates; their AIRR
records remain raw evidence. Clonotyping uses the explicitly opened receptor dataset.

The existing clonotype engine applies the saved definition (junction, V/J resolution,
primary/all-chain selection and required pairing) and QC rules. Memberships are reduced to
**distinct sample/barcode cells**, never contig rows. Clone IDs have the same meaning as in
Clonotypes. A shared clone ID across samples does not pool those samples' sizes.

- **All eligible receptor cells in sample** (default) counts uniquely assigned cells after
  QC, including receptor-only barcodes and cells without coordinates.
- **Mapped eligible cells only** counts those uniquely assigned cells present in this map.
  Imported maps require supplied coordinates; computed maps use their retained cells.

**Clone coverage** always gives receptor, expression and mapped counts alongside the chosen
size. Default size categories are **1, 2, 3–5, >5**. Breakpoints are inclusive upper bounds,
configurable as one to five increasing positive integers. A size above one is called expanded
for this descriptive summary; it is not a statistical expansion test.

## Missingness and chain evidence

Unlinked inputs cannot receive clone sizes. Within a bound input, these cell states are distinct:

- **No receptor evidence:** no exact raw barcode match in the chosen sample.
- **QC excluded:** all its receptor rows were removed by the saved filter.
- **Unassigned clonotype:** retained rows do not yield a clone under this definition.
- **Ambiguous clonotype:** multiple distinct memberships, as can occur when counting each
  locus separately, or conflicting retained vendor clone IDs. No arbitrary clone is chosen.
- **Outside size population:** uniquely assigned, but its clone has no mapped cells when
  mapped cells are the chosen size population.

These states have no numeric size. JSON uses `null` for unavailable per-cell sizes. A clone
coverage row can have a measured mapped count of zero; this is distinct from a cell's missing
size. Candidate clone IDs are retained for ambiguous cells. Missing annotation values remain
a separate group from any literal label, including a label named “Missing”.

Chain evidence uses productive contigs **after QC**, before primary-chain selection. As in QC,
TRA/TRB, TRG/TRD and IGH/(IGK or IGL) define receptor arms; more than two productive contigs
of any locus is multi-chain. Two alpha or light chains can occur in one cell. Mixed receptor
families and unresolved productive loci have explicit states, taking precedence over multi,
paired and single-arm labels. No productive chains, absent receptor evidence and QC exclusions
are separate. These evidence labels are not cell-type calls or doublet diagnoses.

## Phenotype denominators

Summaries include **all expression barcodes**, including those without coordinates or those
excluded from a computed map. An annotation-only barcode is outside this expression population.
Choose no annotation to summarize the whole expression sample. Missing annotation rows/values
remain missing; they are never inferred from neighboring points.

| Quantity | Numerator | Denominator |
| --- | --- | --- |
| Expanded share within an annotation group | Cells whose chosen clone size exceeds one | Cells in that group with a numeric clone size |
| Within-clone phenotype fraction | Expression cells of this clone and annotation value | All expression cells uniquely assigned to this clone, including missing labels |
| Within-group clone fraction | Expression cells of this clone and annotation value | All expression cells with this annotation value, including cells without assigned clones |

The tables expose the integer numerators and denominators. A group with no sized cells has
an unavailable expansion share, not zero. Assignment and mapped coverage are reported
separately. Group counts use cells, not clones; the distinct-clone column counts clone IDs.
Each result records one sample's supplied subject and timepoint, with missing metadata explicit.
Cells are not independent biological replicates. There are no enrichment p-values, differential
expression, inferred phenotype transitions, or antigen-specificity claims in this slice.

## Explore and retain

Color the map by continuous size, size category, assignment, chain evidence, sample, subject,
timepoint or a supplied annotation. Continuous colors are linear over displayed nonmissing
sizes. Missing evidence is gray and drawn last. More than six categorical labels use a single
color with a notice; every label remains available in the table. Source axes and equal scale
are preserved; imported coordinates are never jittered. Coincident cells can overlap.

Select a clone in **Clone coverage**, or use **Show on a cell map** in the Clonotypes inspector.
The latter highlights a saved result only when dataset, definition and QC rules agree. If the
newest saved overlay does not match, choose another saved result or compute a matching one.
Click a point or drag a rectangle to inspect its cells and unresolved evidence. The searchable,
paged cell table and **Select matching cells** provide a keyboard-accessible alternative.
**Reset cell selection** and **Undo cell selection** only change highlighting; they never
recompute sizes or denominators. The inspector shows the first 100 selected cells; the table
and complete export retain every observation.

Settings above the result configure the **next** computation. The displayed result states its
own saved population, phenotype column, definition and QC fingerprint. Changing a dataset's
current definition/filter does not rewrite historical results. Multiple results remain distinct
by ID. Keeping a result is undoable through project history.

Jobs can be cancelled or resumed in Tasks. Resume requires reopening the same receptor dataset
and restoring the expression sources. Changed expression sources or computed-map checksums are
refused. Results are immutable and reopen without source files. **Download phenotypes and
provenance** saves a JSON envelope containing the manifest, source fingerprints, optional map
parent checksum, sample metadata, normalized definition and ID, QC values and fingerprint,
project filter version when supplied, effective options, coverage, every cell, clone, group,
association, coordinates, methods and software version.

Limits are 100000 expression cells and 20000 mapped cells; larger populations are refused,
never sampled. Phenotype summaries do not require a map, so inputs without coordinates can
still be summarized. Selecting a different sample means a separate binding and result.

## Methods and validation

- Sturm G et al. Scirpy: a Scanpy extension for analyzing single-cell T-cell receptor-sequencing
  data. *Bioinformatics*. 2020;36:4817–4818.
  [doi:10.1093/bioinformatics/btaa611](https://doi.org/10.1093/bioinformatics/btaa611).
- [scirpy clonal_expansion API](https://scirpy.scverse.org/en/latest/generated/scirpy.tl.clonal_expansion.html):
  sample-scoped sizes, configurable inclusive breakpoints and missing assignments.
- [10x integrated V(D)J and gene-expression tutorial](https://www.10xgenomics.com/support/software/loupe-vdj-browser/latest/tutorials/lvb-integrated-analysis-loupe-browser):
  linked exploration of clonotypes and supplied cell phenotypes.

Counting populations, ambiguity handling, chain evidence and denominators are explicit
ClonoLoom policies. No external implementation is copied. Independent SQLite queries over
synthetic AIRR rows and cell annotations check distinct-cell joins, both size populations,
all size categories, phenotype counts and fractions. Hand-enumerated edge cases cover QC,
missingness, duplicate contigs, mixed/unknown chains and conflicting assignments. A serialized
result digest checks processor repeatability. API and browser checks cover parent/source drift,
immutable identity, saved reopening, selection, brushing, accessible tables and both themes.
