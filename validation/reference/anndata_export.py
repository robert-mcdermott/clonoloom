# /// script
# requires-python = ">=3.11"
# dependencies = ["anndata==0.12.6", "h5py==3.15.1", "numpy==2.4.2", "scipy==1.17.0", "pandas==2.3.3"]
# ///
"""Independent reader and rewrite validation of Go-produced AnnData, using public APIs only.
uv run --offline --no-project validation/reference/anndata_export.py
The fixture's expected cells, counts, labels and coordinates below are independently specified.
No reference implementation source is inspected. --check checks the committed oracle.
"""
from pathlib import Path
from importlib.metadata import version
import hashlib, json, os, subprocess, tempfile, sys
import anndata as ad, h5py, numpy as np, pandas as pd
from scipy import sparse
root = Path(__file__).resolve().parents[2]
reference = Path(__file__).with_suffix('.json')
with tempfile.TemporaryDirectory(prefix='clonoloom-anndata-') as directory:
    path = Path(directory) / 'cells.h5ad'
    env = dict(os.environ, CLONOLOOM_TEST_ANNDATA_EXPORT=str(path))
    subprocess.run(['go', 'test', '-count=1', './internal/anndata', '-run', '^TestExportFixture$'], cwd=root, env=env, check=True)
    a = ad.read_h5ad(path)
    assert a.shape == (3, 3) and sparse.isspmatrix_csr(a.X)
    assert a.X.dtype == np.dtype('int64')
    assert list(a.obs_names) == ['β-1', 'B-1', 'ZERO-1']
    assert list(a.var_names) == ['g1', 'g2', 'ab1']
    expected = [[16777217, 0, 7], [0, 3, 0], [0, 0, 0]]
    np.testing.assert_array_equal(a.X.toarray(), expected)
    np.testing.assert_array_equal(a.X.indptr, [0, 2, 3, 3])
    np.testing.assert_array_equal(a.X.indices, [0, 2, 1])
    assert list(a.var.feature_name) == ['Shared', 'Shared', '抗体']
    assert list(a.var.feature_type) == ['Gene Expression', 'Gene Expression', 'Antibody Capture']
    assert list(a.var.annotation_0) == ['gene', 'gene', 'protein']
    assert pd.isna(a.var.annotation_1.iloc[0])
    assert list(a.var.annotation_1.iloc[1:]) == ['é', 'yes']
    assert json.loads(a.uns['clonoloom']['variable_columns_json']) == {'annotation_0': 'class / source', 'annotation_1': 'nullable'}
    assert a.obs.annotation_0.iloc[0] == 'Mémoire'
    assert pd.isna(a.obs.annotation_0.iloc[1])
    assert a.obs.annotation_0.iloc[2] == 'Missing annotation'
    assert list(a.obs.clonoloom_assignment) == ['Assigned', 'Ambiguous clonotype', 'No receptor evidence']
    assert a.obs.clonoloom_clone_id.iloc[0] == 'c1'
    assert a.obs.clonoloom_clone_id.iloc[1:].isna().all()
    np.testing.assert_allclose(a.obs.clonoloom_clone_size, [2, np.nan, np.nan])
    assert list(a.obs.clonoloom_sample) == ['S01'] * 3
    assert list(a.obs.clonoloom_map_retained) == [True, True, False]
    assert a.obs.clonoloom_map_exclusion.iloc[2] == 'no positive gene counts'
    assert list(a.obs.clonoloom_map_exclusion.cat.codes) == [-1, -1, 0]
    expected_obsm = {'X_clonoloom_pca': [[-1, 2], [3, 4], [np.nan, np.nan]],
                     'X_clonoloom_umap': [[1.5, -2], [3, 4], [np.nan, np.nan]],
                     'X_supplied': [[3, 4], [np.nan, np.nan], [np.nan, np.nan]]}
    assert set(a.obsm) == set(expected_obsm)
    for key, values in expected_obsm.items():
        np.testing.assert_allclose(a.obsm[key], values, rtol=0, atol=0, equal_nan=True)
    assert json.loads(a.uns['clonoloom']['observation_columns_json']) == {'annotation_0': 'state / supplied'}
    assert json.loads(a.uns['clonoloom']['provenance_json']) == {'sample': 'S01', 'definition': 'paired'}
    assert a.uns['clonoloom']['schema'] == 'clonoloom-anndata-1'
    with h5py.File(path, 'r') as f:
        assert f.attrs['encoding-type'] == b'anndata'
        for key in ['obs/_index', 'var/feature_name', 'obs/annotation_0/categories', 'uns/clonoloom/provenance_json']:
            dtype = h5py.check_string_dtype(f[key].dtype)
            assert dtype.encoding == 'utf-8' and dtype.length is None
        assert f['obs/clonoloom_map_retained'].dtype == np.dtype(bool)
        assert isinstance(f['obs/annotation_0'].attrs['ordered'], np.bool_)
        assert not f['obs/annotation_0'].attrs['ordered']
    # AnnData's own writer reads and rewrites the exported object without changing its values.
    rewritten = Path(directory) / 'rewritten.h5ad'
    a.write_h5ad(rewritten)
    b = ad.read_h5ad(rewritten)
    np.testing.assert_array_equal(b.X.toarray(), expected)
    pd.testing.assert_frame_equal(a.obs, b.obs)
    pd.testing.assert_frame_equal(a.var, b.var)
    assert a.uns == b.uns
    for key in a.obsm:
        np.testing.assert_array_equal(a.obsm[key], b.obsm[key])
    result = {'software': {p: version(p) for p in ['anndata', 'h5py', 'numpy', 'scipy', 'pandas']},
              'sha256': hashlib.sha256(path.read_bytes()).hexdigest(), 'shape': list(a.shape),
              'barcodes': list(a.obs_names), 'features': list(a.var_names), 'counts': expected,
              'assertions': 'Every sparse count/index/pointer, cell and feature identity, feature modality/name, supplied annotation and missing code, assignment, clone size, sample, map inclusion/exclusion, every embedding row, UTF-8 encoding, boolean dtype, JSON provenance, and AnnData rewrite checked.'}
    if '--check' in sys.argv:
        assert json.loads(reference.read_text()) == result, 'Independent export reference changed'
    else:
        reference.write_text(json.dumps(result, indent=2, ensure_ascii=False) + '\n')
    print(json.dumps(result, indent=2, ensure_ascii=False))
