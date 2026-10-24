# /// script
# requires-python = ">=3.11"
# dependencies = ["scipy==1.17.0"]
# ///
"""Independent MEX parsing with scipy.io.mmread; reference values, not runtime code.
Run: uv run validation/reference/expression.py
SciPy documentation: https://docs.scipy.org/doc/scipy/reference/generated/scipy.io.mmread.html
"""
from pathlib import Path
import csv
import hashlib
import json
import scipy
from scipy.io import mmread

root = Path(__file__).resolve().parents[2]
source = root / 'testdata/expression-mex'
a = mmread(source / 'matrix.mtx').toarray()
features = list(csv.reader((source / 'features.tsv').open(), delimiter='\t'))
barcodes = (source / 'barcodes.tsv').read_text().splitlines()
gene = a[[f[2] == 'Gene Expression' for f in features]]
rows = list(csv.DictReader((source / 'receptors.tsv').open(), delimiter='\t'))
receptor = {r['cell_id'] for r in rows if r['cell_id']}
result = {
    'reference': f'SciPy {scipy.__version__} scipy.io.mmread; Python set intersection for links',
    'sources': {n: hashlib.sha256((source / n).read_bytes()).hexdigest() for n in ['matrix.mtx', 'features.tsv', 'barcodes.tsv', 'receptors.tsv']},
    'featureByCell': a.tolist(),
    'geneUMIs': gene.sum(axis=0).tolist(),
    'genes': (gene > 0).sum(axis=0).tolist(),
    'matched': len(receptor & set(barcodes)),
    'expressionOnly': len(set(barcodes) - receptor),
    'receptorOnly': len(receptor - set(barcodes)),
    'missingBarcodeRows': sum(not r['cell_id'] for r in rows),
}
(root / 'validation/reference/expression.json').write_text(json.dumps(result, indent=2) + '\n')
