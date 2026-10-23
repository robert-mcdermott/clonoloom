"""Independent CSV evidence checks; no ClonoLoom output is consumed.

uv run --no-project validation/reference/bcell_inputs.py > validation/reference/bcell-inputs.json
AIRR Rearrangement schema 2.0. Python's standard csv module; no external packages.
Digests cover every selected field, in row order, including absent/invalid values.
"""
import csv
import hashlib
import json
import platform
from pathlib import Path

fields = ['sequence', 'rev_comp', 'sequence_alignment', 'germline_alignment',
          'v_cigar', 'd_cigar', 'j_cigar', 'germline_database']
fields += [f'{g}_{space}_{end}' for g in 'vdj' for space in ('sequence', 'germline', 'alignment') for end in ('start', 'end')]
fields += [f'{r}_{end}' for r in ('fwr1', 'cdr1', 'fwr2', 'cdr2', 'fwr3', 'cdr3', 'fwr4') for end in ('start', 'end')]
fields += ['np1_length', 'np2_length']
root = Path('validation/cache')
files = [Path('testdata/bcell-inputs.tsv')]
for name in ('10x-pbmc-1k', '10x-pbmc-10k-v2', '10x-mouse-splenocytes-1k', '10x-mouse-splenocytes-5k-gemx'):
    paths = list((root / name).glob('*b_airr_rearrangement.tsv'))
    assert len(paths) == 1, name
    files += paths
out = {'made': f'Python {platform.python_version()} csv.DictReader', 'fields': fields, 'files': []}
for path in files:
    with path.open() as f:
        rows = list(csv.DictReader(f, delimiter='\t'))
    digest = hashlib.sha256()
    present = [0] * len(fields)
    for row in rows:
        values = [(row.get(name) or '').strip() for name in fields]
        digest.update(('\t'.join(values) + '\n').encode())
        present = [a + bool(b) for a, b in zip(present, values)]
    out['files'].append({'path': str(path), 'source': hashlib.sha256(path.read_bytes()).hexdigest(),
                         'rows': len(rows), 'present': present, 'digest': digest.hexdigest()})
print(json.dumps(out, indent=2))
