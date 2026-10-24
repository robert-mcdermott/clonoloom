# Clonotype modularity on the expression graph

Development feature for 0.6.0 (wave 6, slice 7); the application version remains 0.5.0.

## Workflow and population

1. Open expression evidence with one explicit receptor dataset/sample binding in **Cells**.
2. **Compute expression map**, then choose that saved map under **Clonotypes and phenotypes**
   and compute the overlay. The normalized clonotype definition, QC and sample metadata are
   pinned there. An imported coordinate map has no expression-neighbor graph for this test.
3. Under **Clonotype modularity**, choose that phenotype parent, a seed and Monte Carlo draw
   count, then **Compute clonotype modularity**. The cloud-study guide includes these steps.
4. Inspect coverage, the clone table and a selected clone's null-frequency plot. **Open parent
   phenotype overlay** highlights that clone on the original saved map. Filtering or selecting
   a clone never recalculates p-values or changes the tested family.

The test population is the intersection of **retained expression-graph cells** and **uniquely
assigned receptor cells in the parent sample**. Exact sample/barcode linkage, QC, ambiguous
membership and conflicting vendor-ID policies come from [PHENOTYPES.md](PHENOTYPES.md).
Missing receptor evidence, QC exclusions, unassigned and ambiguous cells are excluded; their
statuses remain in the record. Expression cells excluded during preprocessing are counted
separately. A clone's size here is its cell count in this graph population, which may differ
from its receptor-sample size or an imported-map size. No donors or samples are pooled.

Use the **induced graph**: retain edges whose two endpoints are eligible, without fitting
neighbors again. Keep isolated eligible cells in the null population. Count each positive
undirected fuzzy-union edge once; weight magnitudes do not contribute. The parent retains the
original weights. UMAP optimizer pruning and distances on the displayed two-dimensional map
never define an edge. The original graph uses all retained PCs, as described in
[EXPRESSION.md](EXPRESSION.md).

## Statistic and hypothesis

Let N be eligible vertices, M their induced undirected edges, s a clone's graph cell count,
and O its observed internal edges. Under the null, every size-s subset of the N vertices is
equally likely. Each edge has probability s(s−1)/(N(N−1)) of being internal, so by linearity:

```
E = M × s(s−1) / (N(N−1))
score = log2((O + 1) / (E + 1))
```

The score follows the documented scirpy clonotype-modularity ratio. It is **not Newman
modularity Q**. The +1 is a pseudocount: score 1 doubles the pseudocount-adjusted quantity,
not necessarily the raw number of edges. Positive scores indicate excess connectivity relative
to this null; negative scores indicate fewer edges. Only the upper tail is tested. The analytic
expectation avoids simulation noise in the score. The binary-edge population and analytic
expectation are explicit ClonoLoom choices, not a claim of numerical equivalence to scirpy's
stochastic expectation or negative-binomial approximation.

The null assumes membership **exchangeable among eligible cells within this one sample**,
conditional on the graph and clone size. It does not condition on cell type, batch, receptor
family, graph component or vertex degree. Enrichment may reflect these factors. This is an
exploratory within-sample connectivity test; it does not establish antigen specificity,
causality, lineage transition or a population/subject-level effect. Cells are not independent
biological replicates. Choose a scientifically defensible population and fixed graph/QC/clone
definition before inspecting results; this slice does not supply stratified or donor-level tests.

## Null sampling and multiple testing

- If choose(N,s) <= 10000, enumerate every size-matched subset, including the observed subset.
  The p-value is the fraction with at least O edges; ties are included.
- Otherwise sample B independent uniform subsets. Sampling is without replacement **within**
  a subset and with replacement **between** draws. Partial Fisher–Yates uses the project's
  unbiased PCG integer generator, seed 20261024 by default and a separate stream keyed by s.
  The default B is 9999. For b draws with at least O edges, p = (b+1)/(B+1), never b/B.
- Clones of the same size share a null histogram. The result stores every occupied integer
  edge count and its frequency, the sampling method, draw count and each clone's inclusive tail.
  No parametric tail extrapolation or significance-dependent early stopping is used.
- Test every clone with 2 <= s < N, including zero-edge clones. Singletons and a clone occupying
  the whole population are explicitly **not tested**; their score, p and adjusted p are null.
  Empty/complete induced graphs give p = 1 for testable clones, not invented evidence.
- Adjust the entire tested family using **Benjamini–Yekutieli (BY)**: sorted p-values are
  multiplied by m H_m / rank, then made monotone from largest rank to smallest and capped at 1,
  where H_m = sum(1/i), i=1…m. Shared graph/null draws do not justify independent tests; BY
  permits arbitrary dependence when the marginal p-values are valid under their stated nulls.
  These are adjusted p-values, not probabilities that a clone is a false discovery.

Monte Carlo resolution is at least 1/(B+1); ties can make it coarser. A p-value at that floor
is not zero. Finite resolution and conservative BY adjustment can limit power for large
families. Increasing B makes a new recorded result. The correction covers only clones within
this single result, not repeated choices of sample, graph, QC, definition or seed. Do not select
the most favorable run. Exact enumeration is independent of the seed.

## Display and saved record

The plot shows a **discrete null distribution**: horizontal axis = internal undirected edges;
vertical axis = number of subsets, starting at zero. Bars retain individual integer counts;
the solid line is observed edges and the dashed line is the analytic expectation. This is not
an effect-size confidence interval or a biological density estimate. Its null-frequency table
and complete JSON retain all values. More than 2000 occupied counts uses tables/export instead
of an unreadably dense SVG. Untested clones have no null plot.

The clone table exposes graph sizes, observed and expected edges, score, raw upper-tail p,
BY-adjusted p and untested reasons. Historical results retain their own settings. Results and
project undo/redo work as with other analyses; Tasks supports cancellation and resumption.
Raw receptor/expression files need not be open to compute from saved parents. Both parents
must pass checksum verification and match the overlay's recorded map hash; altered parents
are refused. A completed modularity result reopens without its parents or raw source files.

The JSON envelope contains manifest, software/method version, both parent IDs and SHA-256s,
parameters, full phenotype provenance (including source fingerprints, definition, QC and sample
metadata), graph-population membership/status, all clone results and null histograms. The saved
expression-map parent separately holds the complete weighted graph and PCA. Keep both exports
to independently reproduce the graph-based test.

Bounds: 10000 graph cells, 99–99999 Monte Carlo draws and 500 million conservative work units.
For each tested size, the work bound uses draws × (4s + min(2M, s × maximum degree)), with
exhaustive subset count where applicable. Oversized work is refused before sampling, without
hidden subsampling. Reconsider a predeclared population or reduce draws, noting the loss of
p-value resolution. Seed zero/omitted selects the documented default; supported explicit seeds
are positive integers through 2^53−1.

## Sources and independent validation

- Sturm et al., *Bioinformatics* 2020;36:4817–4818,
  [doi:10.1093/bioinformatics/btaa611](https://doi.org/10.1093/bioinformatics/btaa611), and the
  [documented scirpy clonotype_modularity API](https://scirpy.scverse.org/en/latest/generated/scirpy.tl.clonotype_modularity.html)
  for the connectivity-score definition and size-matched subgraphs.
- Phipson and Smyth, *Statistical Applications in Genetics and Molecular Biology* 2010;9:39,
  [doi:10.2202/1544-6115.1585](https://doi.org/10.2202/1544-6115.1585), for nonzero Monte Carlo p-values.
- Hemerik and Goeman, *TEST* 2018;27:811–825,
  [doi:10.1007/s11749-017-0571-1](https://doi.org/10.1007/s11749-017-0571-1), for random permutation
  tests with the observed configuration and the required invariance assumptions.
- Benjamini and Yekutieli, *Annals of Statistics* 2001;29:1165–1188,
  [doi:10.1214/aos/1013699998](https://doi.org/10.1214/aos/1013699998), for FDR control under dependence.

`validation/reference/modularity.py` independently enumerates subsets with Python itertools,
counts internal edges with set containment and adjusts with SciPy 1.17.0's public
`false_discovery_control(method="by")`. It reads no ClonoLoom results or third-party source code.
The committed references cover a concentrated clique, dispersed clones, isolated and excluded
cells, exact tails and every histogram frequency. A 15504-subset reference also checks seeded
Monte Carlo sampling. Tests check super-uniform exact p-values with ties, null degeneracy,
nonzero Monte Carlo p-values, deterministic result bytes, malformed graphs, bounds and
cancellation. These artificial cases validate the defined statistic, not biological specificity.
