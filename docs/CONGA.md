# Receptor / expression neighborhood comparison

Available in ClonoLoom 0.6.0.

## Workflow

Compute an expression map in **Cells**, then a phenotype overlay using that saved map and a
paired αβ clonotype definition. Under **Receptor / expression neighborhoods**, choose the
phenotype parent, neighborhood sizes and receptor distance. Keep its original receptor dataset
open to prepare a new comparison; the submitted job captures the required receptor sequences.
Full TCRdist requires an explicit human/mouse choice and the V-loop reference in **Reference
data**. **CDR3-only TCRdist** is an explicit approximation, never an automatic fallback. The
fictional cloud-study guide uses CDR3-only distances and k = 2, 4 without a reference download.

Inspect the selected clone's observed/expected overlap and p-values at every requested size.
The table exposes actual degrees, which can exceed k because ties are included. Expand the
neighbor identities or retained-cell eligibility tables; select another clone or open its parent
phenotype overlay. Filtering and selection do not change the test family. Saved comparisons,
undo/redo, cancellation, resumable jobs and full JSON download follow the other Cells analyses.
Completed results reopen without raw sources or parents. Computing requires checksum-valid
parents; full-distance jobs also require the pinned V-loop reference.

## Method and its relationship to CoNGA

Schattgen et al.'s CoNGA compares receptor and expression graphs at **clonotype** level, using
hypergeometric neighborhood-overlap probabilities. Its expression profiles can use a
representative cell or averaged PCs. ClonoLoom implements a bounded KNN-only adaptation with
explicit population, tie and correction policies below. It does not reproduce the published
multi-graph CoNGA score, its clustering, graph-versus-feature tests, or its full preprocessing
pipeline. An adjusted p-value here is not interchangeable with a published CoNGA score.

### Population and representation (ClonoLoom policies)

- One explicitly bound receptor dataset/sample from the immutable phenotype parent; exact
  barcode matching only. Preserve its definition, QC, subject/timepoint and source fingerprints.
  Samples are not pooled. The parent may use primary chains chosen from dual-chain evidence;
  this comparison does not infer or resolve a different pairing.
- Require a non-vendor paired **TRA then TRB**, amino-acid, primary-chain definition, with V
  and J at gene or allele resolution. Each included clone must contain exactly those two
  chains, nonempty V/J calls and 6–80 standard amino acids per junction. Other definitions are
  refused; incomplete clones, invalid sequences and unavailable V loops have explicit reasons.
- Intersect retained expression-PC cells with uniquely assigned, paired receptor evidence.
  Preserve each retained cell's eligibility reason. Cells outside expression preprocessing
  remain in the embedded phenotype evidence and a separate count. Unmatched, ambiguous,
  QC-excluded, single-arm, mixed and unresolved evidence cannot acquire a numeric test.
- Each eligible clone becomes **one vertex**, regardless of its cell abundance. Take the
  arithmetic mean of its retained cells' scores in **all saved PCs**, summing in barcode order.
  The saved expression preprocessing/PCA is not refitted; no clone weighting, standardization,
  batch correction or feature selection is added. A mean can obscure within-clone heterogeneity;
  use the parent overlay to inspect it. The expression graph is rebuilt on these clone profiles,
  not induced from the parent cell graph. Imported maps alone lack the required PCs.
  No receptor genes are removed by this comparison: saved PCs may include them, so association
  can directly reflect expression of the measured receptors. Inspect the saved feature list;
  the test does not isolate a receptor-independent transcriptional program.
- Expression distance is Euclidean in that PC space. Receptor distance sums the two chains'
  TCRdist under the project's independently validated **TCRdist3 conventions** (see research
  §5.9). Full mode includes V-germline loops; CDR3-only mode omits them. Allele resolutions and
  reference checksum are recorded. No two-dimensional UMAP coordinates enter either graph.

### Candidate pools, ties and test

For each focal clone i, exclude itself and any clone sharing its exact α **or** β CDR3 amino-acid
sequence, regardless of V/J. This conservative shared-CDR3 rule is a ClonoLoom policy, not an
assertion that all such cells share an ancestor. Apply the same exclusions to both graphs.
Let M_i be the remaining candidate count. For each requested integer k with 1 <= k < M_i,
construct **directed** nearest-neighbor sets in each space, including **every exact tie** at
the kth distance. No ID/index tie breaker chooses which tied clones enter the test. Do not
symmetrize or replace these sets with fuzzy graph weights. If k >= M_i, retain an untested row
with null statistics and a reason; do not silently shrink k.

Let A and B be the actual expression and receptor neighbor counts and O their intersection.
The conditional null treats the expression-neighbor labels as a uniform size-A subset of the
M_i candidates, independently of the fixed size-B receptor set. Thus:

```
X ~ Hypergeom(M_i, A, B)
E[X] = A B / M_i
P(X = x) = choose(A, x) choose(M_i − A, B − x) / choose(M_i, B)
p = P(X >= O)
```

This includes ties in the upper tail. Zero-overlap tests and fully tied neighborhoods remain
in the family; a full candidate set in either graph necessarily gives p = 1. This null is a
**conditional model**, not a permutation calibration of the entire graph-building pipeline.
It assumes exchangeable independent neighbor labels; cell type, batch, receptor-chain groups,
selection and expression heterogeneity can violate it. No stratified null is fitted here.

For m testable clone-by-k rows, use **Bonferroni p_adj = min(1, m p)**. Count all testable rows
across every requested scale, including redundant tied sets and zero overlaps. This controls
family-wise error under arbitrary dependence **if each marginal null p-value is valid**;
correction cannot repair nonexchangeable populations. We show each scale separately rather
than selecting and displaying the most favorable scale. Defaults are k = 5, 15. Requested
values are sorted and must be distinct. Changing settings makes a new immutable result.

There is no sampling seed: exact distance ranking and analytic tails are deterministic. Tails
are computed in log space from relative hypergeometric weights, without factorial overflow or
subtracting a nearly-one CDF. Store natural-log raw and adjusted p-values. If exponentiation
underflows, the ordinary probability is null and the UI displays the finite log probability;
it never claims a zero p-value. Untested rows have null log probabilities too.

## Interpretation, figures and record

This is exploratory **within-sample association**, not antigen specificity, causality, lineage
transition, a treatment effect or a donor-level hypothesis test. Clonotypes/cells are not
independent biological subjects. The family correction does not cover repeated choices of
sample, definition, QC, PCs, receptor metric or separately run scales. Predeclare scientifically
appropriate choices; do not search across saved results for a favorable p-value. Artificial
fixtures and the guide establish software behavior, not biological specificity.

The figure uses solid bars for **observed shared-neighbor counts** and dashed markers for
**conditional expected counts**, with a common linear count axis starting at zero. Each requested
k is a separate category. It is not a null histogram, a density, or an uncertainty interval.
Untested rows have no plotted values. The accessible table contains every count and probability;
neighbor IDs, cell memberships and exclusions are available without interpreting the picture.

The saved manifest pins phenotype and expression-map hashes, the receptor snapshot, V-loop
checksum (full mode), effective options, method/software versions and citations. The result
embeds phenotype evidence, clone mean-PC profiles, included sequences/barcodes, both directed
neighbor lists for every scale, candidate/shared-chain counts and all tests. Keep the parent
expression export to inspect preprocessing and individual cell PCs. Full-distance reproduction
also needs the recorded loop reference. No source files are modified.

Bounds are 10000 retained expression cells, 2000 assigned clonotypes, up to eight distinct k
values from 1–200, and two million stored directed neighbor entries including repeated scales.
A conservative work estimate `n(n−1) × (PCs + 2 L² + 1000)` must not exceed 500 million, where
n is eligible clones and L is their largest CDR3 length. The allowance covers PC distances,
two-chain gap-placement scans and loop-related work. Excess is refused before distance-set
construction; output ties are bounded during construction. There is no hidden subsampling.
This is an interactive bounded implementation, not a large-repertoire approximation.

## Sources and independent checks

- Schattgen et al., *Nature Biotechnology* 2022;40:54–63,
  [doi:10.1038/s41587-021-00989-2](https://doi.org/10.1038/s41587-021-00989-2): the CoNGA framework.
- Dash et al., *Nature* 2017;547:89–93,
  [doi:10.1038/nature22383](https://doi.org/10.1038/nature22383): TCRdist.
- Mayer-Blackwell et al., *eLife* 2021;10:e68605,
  [doi:10.7554/eLife.68605](https://doi.org/10.7554/eLife.68605): TCRdist3 conventions.
- Dunn, *JASA* 1961;56:52–64,
  [doi:10.1080/01621459.1961.10482090](https://doi.org/10.1080/01621459.1961.10482090):
  simultaneous comparisons using the Bonferroni inequality.
- [SciPy hypergeom API](https://docs.scipy.org/doc/scipy/reference/generated/scipy.stats.hypergeom.html):
  public distribution parameterization and independent tail evaluation.

`validation/reference/conga.py` uses SciPy 1.17.0 public Euclidean/hypergeometric APIs and
independent set operations. Fixed-length synthetic CDR3 distances use the relevant BLOSUM62
entries directly, not ClonoLoom outputs. Exhaustive subsets independently confirm the small
upper tails and check super-uniform p-values across all degrees for populations 2–9. References
cover aligned, permuted and completely tied expression profiles, shared chains, actual degrees,
all-scale adjustment and extreme log tails. Existing TCRdist validation separately checks the
full distance implementation against public reference results. Unit/API/browser checks cover
invalid/absent evidence, clone abundance, source order, missing loops, changed parents/sources,
resource bounds, cancellation, saved reopening, plot values and both themes.
