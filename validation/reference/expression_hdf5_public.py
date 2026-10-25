# /// script
# requires-python = ">=3.11"
# dependencies = ["h5py==3.15.1", "numpy==2.4.2", "scipy==1.17.0"]
# ///
"""Check a maintainer-selected 10x file, keeping sources and results outside the repository.
uv run --no-project validation/reference/expression_hdf5_public.py INPUT.h5 OUTPUT.json
Only hashes and dimensions are written; counts and labels are decoded independently by h5py.
"""
import hashlib
import json
from pathlib import Path
import sys

import h5py
import numpy as np
import scipy
from scipy.sparse import csc_matrix

source, output = map(Path, sys.argv[1:])
with h5py.File(source, 'r') as f:
    g = f['matrix']
    m = csc_matrix((g['data'][:].astype('u8'), g['indices'][:], g['indptr'][:]), shape=g['shape'][:])
    m.sort_indices()
    types = g['features/feature_type'].asstr()[:].tolist()
    gene = m[np.array(types) == 'Gene Expression', :]
    features = list(zip(g['features/id'].asstr()[:], g['features/name'].asstr()[:], types))
    def digest(a, dtype): return hashlib.sha256(np.asarray(a, dtype=dtype).tobytes()).hexdigest()
    result = {
        'reference': f'h5py {h5py.__version__}; NumPy {np.__version__}; SciPy {scipy.__version__} CSC',
        'source': hashlib.sha256(source.read_bytes()).hexdigest(),
        'features': m.shape[0], 'cells': m.shape[1], 'entries': m.nnz,
        'counts': digest(m.data, '<u8'), 'indices': digest(m.indices, '<u4'), 'indptr': digest(m.indptr, '<u8'),
        'geneUMIs': digest(np.asarray(gene.sum(axis=0)).ravel(), '<u8'),
        'genes': digest(np.asarray((gene > 0).sum(axis=0)).ravel(), '<u8'),
        'labels': hashlib.sha256(b''.join(s.encode('utf-8') + b'\0' for row in features for s in row)).hexdigest(),
        'barcodes': hashlib.sha256(b''.join(s.encode('utf-8') + b'\0' for s in g['barcodes'].asstr()[:])).hexdigest(),
    }
output.write_text(json.dumps(result, indent=2) + '\n')
print(f"Independently checked {result['features']} features, {result['cells']} cells, {result['entries']} stored entries")
