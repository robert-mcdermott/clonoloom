# Computed expression maps

Open expression evidence in **Cells**, then choose **Compute expression map**. The artificial
cell-state study provides a useful first run. Its imported Gaussian coordinates are illustrative;
the computed UMAP uses only the count matrix. Missing supplied coordinates do not exclude a cell
from this analysis. Missing annotations stay missing and appear gray when used for coloring.

## Population and preprocessing

Each result analyzes one expression input. All its expression barcodes are eligible, including
cells without receptor matches. The active receptor QC filter and clonotype definition do not
enter this expression analysis. Results do not pool samples or correct batches. Link a result to
receptors later through its expression identity and an explicit receptor dataset/sample.

1. Keep cells with positive Gene Expression counts and at least the selected number of detected
   genes (default **1**). A detected gene has a positive count; stored zeros do not count.
2. Normalize each retained cell to a total of **10000** (configurable), using **all Gene Expression
   features before feature selection**. Other feature types, including antibody capture, do not
   enter the total. Transform each value with the natural `log1p`.
3. Keep genes detected in at least **1** retained cell (configurable) with positive sample
   variance. Select up to **500** by decreasing variance of the transformed values; ties use
   feature ID. This is a simple variance ranking, not a named highly-variable-gene recipe or a
   significance test. Repeated feature names remain separate IDs.
4. Center selected features without variance scaling, clipping, whitening, regression or batch
   correction. Compute exact PCA using the selected-feature covariance and deterministic
   Householder/QL eigendecomposition. Keep up to **30** PCs, bounded by cells minus one, selected
   features and eigenvalues above `1e-12` times the largest. At least two nonzero PCs are required.
   Each component's largest absolute loading is positive; ties follow feature-ID order.
5. Find each cell's **15 nearest other cells** by exact Euclidean distance in all retained PCs.
   Neighbor count is capped at retained cells minus one. Self is excluded; ties follow source
   barcode order. Zero-distance neighbors remain valid. UMAP's convention including self is
   therefore this count plus one. Save both directed neighbors/distances and their weighted
   fuzzy-union graph; the graph is separate from the two-dimensional display.
6. Compute a two-dimensional UMAP using PCA initialization, minimum distance **0.5**, spread **1**,
   **500** epochs, five negative samples, learning rate **1**, repulsion **1** and seed **20261024**.
   Minimum distance, epochs and seed are configurable. Optimization is serial and uses ClonoLoom's
   deterministic PCG stream; layouts are not expected to equal umap-learn coordinates. Every
   fixed numerical choice and fitted curve coefficient is saved with the result.

Four retained cells are required. The bounded implementation accepts at most 100000 source
barcodes, 10000 retained cells and 1000 selected features. The product of retained cells and
squared selected-feature count cannot exceed two billion. Larger jobs fail explicitly; choose
fewer features or a smaller source. No cells are silently sampled. These limits do not reduce
what the input readers can inspect.

## Read and keep the result

The result shows the computed axes as **UMAP 1 / UMAP 2**, with equal display scale. Color by a
supplied annotation to compare it with the count-derived map. Use the mapped-cell, excluded-cell
and selected-feature tables to inspect exact values and exclusion reasons. Selecting a point
or a cell's table button shows its source barcode and annotation values.

**Method, settings and provenance** contains the population, transformation, feature selection,
PCA and graph rules, effective dimensions, source fingerprints and citations. **Download result
and provenance** saves JSON containing the manifest, methods, selected features and loadings,
PCA scores/variances, neighbor indices/distances, weighted graph, annotations, exclusions and
computed coordinates. Indices refer to the saved retained-cell order, whose exact barcodes are
included. The graph may be disconnected; its component count is reported. Distances between
islands on the map do not establish biological differences.

Jobs appear in Tasks and may be cancelled. Interrupted jobs can be resumed after the original
sources are restored; changed source bytes are refused. A successful map is immutable and kept
in the project's Results list. Saved maps reopen without the expression sources or receptor
dataset. Their content checksum is verified on read. Source data are never changed.

This is exploratory expression structure. A map is not a cell-type assignment, receptor
phenotype, proof of antigen specificity, differential-expression test or subject-level effect.
Clonotype overlays and phenotype summaries are the next wave 6 slice.

## Methods and validation

- Wolf FA, Angerer P, Theis FJ. SCANPY. *Genome Biology*. 2018;19:15.
  [doi:10.1186/s13059-017-1382-0](https://doi.org/10.1186/s13059-017-1382-0).
- Jolliffe IT, Cadima J. Principal component analysis: a review and recent developments.
  *Philosophical Transactions A*. 2016;374:20150202.
  [doi:10.1098/rsta.2015.0202](https://doi.org/10.1098/rsta.2015.0202).
- McInnes L, Healy J, Melville J. UMAP. 2018.
  [arXiv:1802.03426](https://arxiv.org/abs/1802.03426).

The normalization/PCA/graph/map implementation follows these methods and adapts the Apache-2.0
SituCraft 0.9.0 numerical code. Variance ranking, bounds, no variance scaling and always using
PCA initialization are ClonoLoom policies, not defaults attributed to those publications.
`validation/reference/expression_map.py` pins independent NumPy, SciPy, scikit-learn and
umap-learn calculations. It checks selected features, means, variances, loadings, scores, exact
neighbors, fuzzy weights and UMAP curve parameters. Known simulated states and neighborhood
trustworthiness assess the layout; deterministic result bytes are pinned on arm64 and amd64.
The Python reference disables Numba JIT because its fast-math compilation produced infinite
smooth-kNN bandwidths in the tested environment; finite bandwidths are explicitly required.
