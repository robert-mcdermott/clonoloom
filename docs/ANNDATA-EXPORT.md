# AnnData export with receptor links

Available in ClonoLoom 0.6.0.

In **Cells**, compute or open **Clonotype phenotypes**, then choose that result under
**AnnData export** and press **Prepare AnnData export**. Both supplied-coordinate phenotypes
and computed-map phenotypes are supported. The original expression sources must be available;
keep the original receptor dataset open to capture the linked clonotype definitions. Download
contains `cells.h5ad`, the export record, checksum manifest and `NOTICE.txt`. Saved exports can
be reopened and downloaded without their source files or parent results. Restricted provider
terms use the same acknowledgment as other data exports.

The export does not compute a new statistic or embedding. It preserves the saved evidence and
its population. The cell-state example exports all **360 cells × 24 features**, including the
one cell excluded from PCA. Its 359 retained computed positions and 358 supplied positions
have separate names; neither is a physical coordinate system or evidence of specificity.

## File contract, version 1

The [AnnData on-disk specification](https://anndata.readthedocs.io/en/stable/fileformat-prose.html)
defines the storage encodings. ClonoLoom's choices of population, column names and JSON records
below are an application contract, not an AnnData or scirpy biological method.

| Location | Content and meaning |
| --- | --- |
| `X` | Cell × feature CSR, exact signed int64 raw counts. No normalization, logarithm or float32 conversion. |
| `obs_names` | Every input expression barcode, exactly once, in original order. No suffix stripping or cross-sample matching. |
| `var_names` | Every input feature ID, in original order. |
| `var.feature_name`, `var.feature_type` | Names (which may repeat) and modalities; antibody counts are not renamed gene expression. |
| `obs.clonoloom_clone_id` | Unique assigned clone ID, missing for absent/unresolved/ambiguous assignments. |
| `obs.clonoloom_assignment`, `obs.clonoloom_chain_status` | Saved eligibility/assignment and productive-chain evidence, including exclusion reasons. |
| `obs.clonoloom_clone_size`, `obs.clonoloom_size_category` | Saved sample-scoped distinct-cell size and bin. Missing size is NaN, never zero. Its population is specified by the saved phenotype options. |
| `obs.clonoloom_sample` | The exact receptor sample binding. Subject/timepoint/tissue and the numeric sample index are retained in the phenotype document and parent job record. |
| `obs.annotation_<n>`, `var.annotation_<n>` | Supplied annotations as read: categories of exact text, with empty values missing. Original names are mapped in `uns/clonoloom`. Reader text semantics are preserved, not original pandas dtypes. |
| `obsm.X_supplied` | Selected source coordinates aligned to every expression cell, when any are available. Both coordinates are NaN when unavailable. The source key/axes remain in input provenance. |
| `obsm.X_clonoloom_pca` | All saved retained PC scores, aligned to input barcodes; excluded cells have an entirely NaN row. Only present for a computed-map parent. |
| `obsm.X_clonoloom_umap` | Saved computed UMAP coordinates with the same alignment and missing rows. |
| `obs.clonoloom_map_retained`, `obs.clonoloom_map_exclusion` | Computed-map inclusion and the saved exclusion reason. |
| `uns.clonoloom` | Schema, matrix semantics, method description, citations, software, NOTICE, annotation-name mappings and JSON provenance described below. |

Categorical missing values use code −1. Literal labels such as `Missing annotation` remain
ordinary categories, distinct from a missing value. UTF-8 strings support non-ASCII barcodes,
feature names and annotations. Explicit sparse zeros remain stored; a missing barcode is not
invented as a zero-count row. Expression exclusions never drop raw observations. Receptor-only
cells remain in the phenotype coverage record and are not invented as expression observations.

`uns['clonoloom']` contains:

- `schema`: `clonoloom-anndata-1`.
- `observation_columns_json`, `variable_columns_json`: exported-key to original-name mappings.
  Numeric suffixes avoid conflicts with reserved, duplicate or path-like source names.
- `phenotypes_json`: the complete phenotype document, including cell ambiguity candidates,
  source fingerprints, sample metadata, definition, QC, clone coverage and denominators.
- `phenotype_manifest_json`: the parent manifest, including job/source selection, filter-record
  identity/version and method/software lineage.
- `receptors_json`: linked clone IDs and chain definitions (plus vendor IDs when applicable),
  covering expression assignments and their ambiguity candidates. No pooled abundance is
  substituted for the phenotype result's sample-scoped size.
- `expression_map_json`, `expression_map_manifest_json`: when computed, the complete saved map
  and its manifest, including preprocessing, selected features, PCA variances, exact neighbor
  graph, UMAP parameters, seed, exclusions and citations.
- `software_json`, `description`, `citation`, `notice`, `matrix_semantics`.

This is **not** scirpy's `obsm['airr']` raw-chain schema. It does not reconstruct absent contig
records from clone definitions or adopt imported assignments as ClonoLoom results. It exports
only the selected expression input, not every modality/layer in an original MuData/AnnData
container. Empty `layers`, `obsp`, `varm` and `varp` mappings make no promise of normalized counts,
loadings or a graph aligned to all observations; the exact computed graph remains in its
saved JSON with its own retained-cell order.

## Read and inspect

Install analysis packages in your own environment as usual; they are not runtime dependencies
of ClonoLoom. For example, save this as `inspect_export.py` and run it using `uv run` with AnnData
and NumPy installed in that environment:

```python
import json
import anndata as ad
import numpy as np

a = ad.read_h5ad("cells.h5ad")
assert a.X.dtype == np.dtype("int64")
provenance = a.uns["clonoloom"]
phenotypes = json.loads(provenance["phenotypes_json"])
receptors = json.loads(provenance["receptors_json"])
print(a.shape, phenotypes["annotation"], len(receptors))

# Select evaluable computed coordinates explicitly before a downstream plot.
if "X_clonoloom_umap" in a.obsm:
    keep = a.obs["clonoloom_map_retained"].to_numpy(dtype=bool)
    mapped = a[keep].copy()
    assert np.isfinite(mapped.obsm["X_clonoloom_umap"]).all()
```

Keep missing annotations gray and preserve equal coordinate-axis scaling in downstream plots.
Do not treat UMAP distances as expression distances or cells as independent biological subjects.
A downstream analysis of the exported raw counts must choose its own preprocessing explicitly.

To read the file back into ClonoLoom, choose **Count matrix** `X`, **Feature name column**
`feature_name`, **Feature type column** `feature_type`, and optionally select `X_supplied` or
`X_clonoloom_umap` as the supplied embedding. Leave **AIRR key** empty. Reimport treats those
coordinates as imported evidence; it does not recreate saved project analyses or adopt the
exported clone-ID annotation as a new clonotype assignment.

## Verification and bounds

Requests pin the phenotype JSON hash and, when present, the exact map hash already pinned by
that phenotype. Source selection comes from the parent job; raw counts are reopened and must
have the same content identity. Receptor source records must match the parent's original
snapshot. Jobs keep the required clone definitions and source NOTICE, so resuming does not need
the receptor cache. Source changes, inconsistent map populations and mismatched coordinates
fail rather than being joined by position. Every output file receives SHA-256 verification.
Before promotion, ClonoLoom rereads the HDF5 file and compares all matrix pointers, indices,
counts, cell IDs and feature records. Downloads verify the exact bytes they serve.

Limits: 100,000 cells, 200,000 features, 2,000,000 stored sparse entries, 32 supplied observation
columns, 126 supplied variable columns, 4,000,000 values per output annotation frame, 16,000,000 embedding values and 512 MiB encoded
output. Source-reader text/annotation limits also apply. HDF5 datasets are contiguous and
uncompressed. No silent subsampling, truncation or dense conversion of counts is performed.

`validation/reference/anndata_export.py` runs the Go fixture and checks it through **AnnData
0.12.6, h5py 3.15.1, NumPy 2.4.2, SciPy 1.17.0 and pandas 2.3.3**, using public reader/writer APIs.
It checks all sparse entries, ordering, Unicode, missing values, counts beyond float32 integer
precision, clone links and every embedding row, then rewrites with AnnData and compares again.
The committed hash makes that independently read object a deterministic Go regression fixture.
Run with `uv run --offline --no-project validation/reference/anndata_export.py --check` when
those pinned packages are cached. The HDF5 writer also tests multiple global-heap collections,
long strings, exact large integers, empty arrays and malformed names/text.

The writer is adapted from SituCraft's Apache-2.0 HDF5 writer, with UTF-8 variable-length
strings, booleans, float64 arrays and dataset attributes implemented against the
[HDF Group specification](https://support.hdfgroup.org/documentation/hdf5/latest/_f_m_t3.html).
No third-party implementation source was used for independent validation.
