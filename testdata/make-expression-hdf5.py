# /// script
# requires-python = ">=3.11"
# dependencies = ["h5py==3.15.1", "numpy==2.4.2", "scipy==1.17.0"]
# ///
"""Independent artificial 10x HDF5 fixtures. No donor data or third-party source code.

uv run --no-project testdata/make-expression-hdf5.py
Uses only the documented h5py API and 10x feature-barcode schema. h5py writes HDF5;
SciPy independently reconstructs every value. The Go reader never writes its own oracle.
Fixed fixture bytes are checked in: HDF5 metadata timestamps can vary on regeneration.
"""
import hashlib
import json
from pathlib import Path

import h5py
import numpy as np
import scipy
from scipy.sparse import csc_matrix

root = Path(__file__).parent / 'expression-hdf5'
root.mkdir(exist_ok=True)
# Same observations as expression-mex, deliberately unsorted within the first cell.
base = {
    'shape': np.array([3, 4], dtype='<u8'),
    'data': np.array([11, 3, 2, 0, 4, 7], dtype='<u4'),
    'indices': np.array([2, 1, 0, 0, 1, 1], dtype='<u4'),
    'indptr': np.array([0, 3, 5, 6, 6], dtype='<u4'),
    'barcodes': np.array(['AA-1', 'BB-1', 'CC-1', 'DD-1'], dtype='S4'),
    'features/id': np.array(['g1', 'g2', 'ab1'], dtype='S3'),
    'features/name': np.array(['Shared name', 'Shared name', 'Artificial antibody'], dtype='S19'),
    'features/feature_type': np.array(['Gene Expression', 'Gene Expression', 'Antibody Capture'], dtype='S16'),
}

def write(name, *, mode='contiguous', values=None, modern=False, userblock=False, extra=None):
    values = dict(base if values is None else values)
    version = modern if isinstance(modern, str) else 'v108'
    options = {'libver': (version, version)} if modern else {'libver': 'earliest'}
    if userblock:
        options['userblock_size'] = 512
    with h5py.File(root / (name + '.h5'), 'w', **options) as f:
        f.attrs['description'] = 'Artificial format fixture, unrestricted'
        g = f.create_group('matrix')
        for key, data in values.items():
            kwargs = {'track_times': False}
            if mode in ('gzip', 'shuffle', 'checksum', 'chunks', 'fletcher') and len(data):
                kwargs['chunks'] = (min(3 if mode == 'fletcher' else 2, len(data)),)
                if mode not in ('chunks', 'fletcher'): kwargs['compression'] = 'gzip'
                if mode in ('shuffle', 'checksum'): kwargs['shuffle'] = True
                if mode in ('checksum', 'fletcher'): kwargs['fletcher32'] = True
            g.create_dataset(key, data=data, **kwargs)
        if extra: extra(g)

for mode in ('contiguous', 'gzip', 'shuffle', 'checksum', 'chunks'):
    write(mode, mode=mode)
write('v108', mode='checksum', modern=True)
write('v110-contiguous', modern='v110')
write('userblock', mode='gzip', userblock=True)
write('big-endian', mode='checksum', values={k: v.astype('>i8') if v.dtype.kind == 'u' else v for k, v in base.items()})
write('small-integers', values={k: v.astype('u1') if v.dtype.kind == 'u' else v for k, v in base.items()})
# Thousands of chunk records force internal B-tree nodes; extra group links force symbol nodes.
large = dict(base)
large.update(shape=np.array([3, 300], dtype='u8'),
             barcodes=np.array([f'C{i:04}' for i in range(300)], dtype='S5'),
             indices=np.tile(np.array([2, 0, 1], dtype='u4'), 300),
             data=np.arange(900, dtype='u4') % 17,
             indptr=np.arange(0, 901, 3, dtype='u4'))
def extra_links(g):
    for i in range(80): g.create_group(f'ignored-{i:03}')
write('btree', mode='checksum', values=large, extra=extra_links)
empty = dict(base)
empty.update(data=np.array([], dtype='u4'), indices=np.array([], dtype='u4'), indptr=np.zeros(5, dtype='u4'))
write('zero-counts', values=empty)

def continuation(g):
    for i in range(30): g['data'].attrs[f'note-{i}'] = 'Artificial metadata ' * 20
write('continuation', extra=continuation)
def compact(g):
    del g['shape']
    creation = h5py.h5p.create(h5py.h5p.DATASET_CREATE)
    creation.set_layout(h5py.h5d.COMPACT)
    d = h5py.h5d.create(g.id, b'shape', h5py.h5t.py_create(base['shape'].dtype), h5py.h5s.create_simple((2,)), dcpl=creation)
    d.write(h5py.h5s.ALL, h5py.h5s.ALL, base['shape'])
write('compact', extra=compact)
def skipped_filters(g):
    g['data'].id.write_direct_chunk((0,), base['data'][:2].tobytes(), filter_mask=7)
write('filter-mask', mode='checksum', extra=skipped_filters)
for name, value in [('fletcher-ones', 2**32-1), ('fletcher-zeros', 0)]:
    values = dict(base); values['data'] = np.full(6, value, dtype='u4')
    write(name, mode='fletcher', values=values)

valid = ['contiguous', 'gzip', 'shuffle', 'checksum', 'chunks', 'v108', 'v110-contiguous', 'userblock', 'big-endian', 'small-integers', 'btree', 'zero-counts', 'continuation', 'compact', 'filter-mask', 'fletcher-ones', 'fletcher-zeros']
invalid = {}
def bad(name, key, data, reason):
    values = dict(base); values[key] = data
    write(name, values=values); invalid[name + '.h5'] = reason
bad('negative', 'data', np.array([-1, 3, 2, 0, 4, 7], dtype='i4'), 'negative')
bad('float', 'data', base['data'].astype('f8'), 'datatype class')
bad('duplicate', 'indices', np.array([1, 1, 0, 0, 1, 1], dtype='u4'), 'duplicate coordinate')
bad('out-of-range', 'indices', np.array([3, 1, 0, 0, 1, 1], dtype='u4'), 'index outside')
bad('pointer', 'indptr', np.array([0, 5, 3, 6, 6], dtype='u4'), 'not monotone')
bad('shape', 'shape', np.array([3, 5], dtype='u8'), 'shape does not match')
bad('duplicate-barcode', 'barcodes', np.array(['AA-1'] * 4, dtype='S4'), 'duplicate barcode')
bad('unsafe-count', 'data', np.array([11, 3, 2**53, 0, 4, 7], dtype='u8'), 'count exceeds')
bad('unsafe-total', 'data', np.array([11, 2**53-1, 2, 0, 4, 7], dtype='u8'), 'total exceeds')
values = dict(base); values['barcodes'] = np.array(['AA-1', 'BB-1', 'CC-1', 'DD-1'], dtype=h5py.string_dtype())
write('variable-string', values=values); valid.append('variable-string')

def replace_data(g, mode):
    del g['data']
    if mode == 'soft-link': g['data'] = h5py.SoftLink('/matrix/indices')
    elif mode == 'external-link': g['data'] = h5py.ExternalLink('not-read.h5', '/matrix/data')
    elif mode == 'unallocated': g.create_dataset('data', shape=(6,), dtype='u4')
    elif mode == 'partial-chunks':
        d = g.create_dataset('data', shape=(6,), dtype='u4', chunks=(2,)); d[:2] = base['data'][:2]
    elif mode == 'unsupported-filter': g.create_dataset('data', data=base['data'], compression='lzf')
    elif mode == 'external-storage': g.create_dataset('data', shape=(6,), dtype='u4', external=[('not-read.raw', 0, 24)])
for name, reason in {'soft-link':'soft/external link', 'external-link':'soft/external link', 'unallocated':'undefined', 'partial-chunks':'missing chunks', 'unsupported-filter':'filter 32000', 'external-storage':'external/virtual'}.items():
    write(name, extra=lambda g: replace_data(g, name)); invalid[name + '.h5'] = reason
write('new-chunk-index', mode='checksum', modern='v110')
invalid['new-chunk-index.h5'] = 'v4 chunk index'
write('dense-group', modern=True, extra=extra_links)
invalid['dense-group.h5'] = 'dense group'

reference = {'reference': f'h5py {h5py.__version__}; HDF5 {h5py.version.hdf5_version}; NumPy {np.__version__}; SciPy {scipy.__version__} csc_matrix', 'valid': {}, 'invalid': invalid}
for name in valid:
    file = root / (name + '.h5')
    with h5py.File(file, 'r') as f:
        g = f['matrix']
        dense = csc_matrix((g['data'][:], g['indices'][:], g['indptr'][:]), shape=g['shape'][:]).toarray()
        types = g['features/feature_type'].asstr()[:].tolist()
        genes = dense[np.array(types) == 'Gene Expression']
        reference['valid'][file.name] = {
            'sha256': hashlib.sha256(file.read_bytes()).hexdigest(),
            'featureByCell': dense.tolist(), 'geneUMIs': genes.sum(axis=0).tolist(),
            'genes': (genes > 0).sum(axis=0).tolist(), 'entries': len(g['data']),
            'barcodes': g['barcodes'].asstr()[:].tolist(),
            'features': [dict(zip(('id', 'name', 'type'), row)) for row in zip(g['features/id'].asstr()[:], g['features/name'].asstr()[:], types)],
        }
Path('validation/reference/expression_hdf5.json').write_text(json.dumps(reference, indent=2) + '\n')
print(f'{len(valid)} valid encodings and {len(invalid)} invalid matrices; every dense value independently reconstructed')
