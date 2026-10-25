# /// script
# requires-python = ">=3.11"
# dependencies = ["anndata==0.12.6", "h5py==3.15.1", "numpy==2.4.2", "scipy==1.17.0", "pandas==2.3.3"]
# ///
"""Independently read the complete 0.6.0 export and compare its original source and parents.
Uses public reader/writer APIs only. Run with uv run --offline --no-project.
"""
from pathlib import Path
import csv, gzip, hashlib, json, tempfile
import anndata as ad
import numpy as np
from scipy.io import mmread
from scipy import sparse

root = Path(__file__).resolve().parents[1] / 'projects/0.6.0'
project = json.loads((root / 'project.json').read_text())
records = {r['kind']: r for r in project['artifacts']}
def document(kind):
    return json.loads(gzip.decompress((root / 'artifacts' / records[kind]['id'] / (kind + '.json.gz')).read_bytes()))
for r in records.values():
    directory = root / 'artifacts' / r['id']
    manifest = json.loads((directory / 'manifest.json').read_text())
    for f in manifest['files']:
        raw = gzip.decompress((directory / (f['path'] + '.gz')).read_bytes())
        assert hashlib.sha256(raw).hexdigest() == f['sha256']
source = root / 'data/expression/S01-1'
counts = mmread(source / 'matrix.mtx').T.tocsr()
barcodes = (source / 'barcodes.tsv').read_text().splitlines()
features = [line.split('\t') for line in (source / 'features.tsv').read_text().splitlines()]
annotations = {r['barcode']: r for r in csv.DictReader((source / 'cells.tsv').open(), delimiter='\t')}
p = document('phenotypes')['analysis']
analysis = document('expression-map')['analysis']
with tempfile.TemporaryDirectory(prefix='clonoloom-release-anndata-') as temp:
    path = Path(temp) / 'cells.h5ad'
    path.write_bytes(gzip.decompress((root / 'artifacts' / records['anndata-export']['id'] / 'cells.h5ad.gz').read_bytes()))
    a = ad.read_h5ad(path)
    assert a.shape == (360, 24) and sparse.isspmatrix_csr(a.X) and a.X.dtype == np.dtype('int64')
    assert a.X.nnz == 7514 and (a.X != counts).nnz == 0
    assert list(a.obs_names) == barcodes
    assert list(a.var_names) == [f[0] for f in features]
    assert list(a.var.feature_name) == [f[1] for f in features]
    assert list(a.var.feature_type) == [f[2] for f in features]
    by_barcode = {c['barcode']: c for c in analysis['cells']}
    pheno = {c['barcode']: c for c in p['cells']}
    columns = json.loads(a.uns['clonoloom']['observation_columns_json'])
    for i, barcode in enumerate(barcodes):
        row = annotations.get(barcode, {})
        for key, original in columns.items():
            value = row.get(original, '')
            if value == '': assert a.obs[key].isna().iloc[i]
            else: assert str(a.obs[key].iloc[i]) == value
        cell = pheno[barcode]
        assert a.obs.clonoloom_assignment.iloc[i] == cell['status']
        assert a.obs.clonoloom_chain_status.iloc[i] == cell['chains']
        if cell.get('clone'): assert a.obs.clonoloom_clone_id.iloc[i] == cell['clone']
        else: assert a.obs.clonoloom_clone_id.isna().iloc[i]
        if cell.get('size') is None: assert np.isnan(a.obs.clonoloom_clone_size.iloc[i])
        else: assert a.obs.clonoloom_clone_size.iloc[i] == cell['size']
        assert a.obs.clonoloom_sample.iloc[i] == 'S01-1'
        if barcode in by_barcode:
            np.testing.assert_array_equal(a.obsm['X_clonoloom_pca'][i], by_barcode[barcode]['scores'])
            np.testing.assert_array_equal(a.obsm['X_clonoloom_umap'][i], by_barcode[barcode]['position'])
        else:
            assert np.isnan(a.obsm['X_clonoloom_pca'][i]).all()
            assert np.isnan(a.obsm['X_clonoloom_umap'][i]).all()
        if row.get('map_x'):
            np.testing.assert_array_equal(a.obsm['X_supplied'][i], [float(row['map_x']), float(row['map_y'])])
        else: assert np.isnan(a.obsm['X_supplied'][i]).all()
    assert np.isfinite(a.obsm['X_supplied']).all(axis=1).sum() == 358
    assert np.isfinite(a.obsm['X_clonoloom_umap']).all(axis=1).sum() == 359
    assert (a.X[-1].toarray() == 0).all()
    for name, kind in [('phenotypes_json', 'phenotypes'), ('expression_map_json', 'expression-map')]:
        assert json.loads(a.uns['clonoloom'][name]) == document(kind)
    rewritten = Path(temp) / 'rewritten.h5ad'
    a.write_h5ad(rewritten)
    b = ad.read_h5ad(rewritten)
    assert (a.X != b.X).nnz == 0 and a.obs.equals(b.obs) and a.var.equals(b.var)
    for key in a.obsm: np.testing.assert_array_equal(a.obsm[key], b.obsm[key])
print('0.6.0 AnnData: all 360 × 24 counts, identities, annotations, clone links and supplied/computed coordinates agree; independent rewrite agrees.')
