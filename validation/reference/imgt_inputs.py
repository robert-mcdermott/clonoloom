"""Independent stdlib CSV inspection of artificial IMGT tables (no ClonoLoom imports).
Run with uv run --no-project validation/reference/imgt_inputs.py.
The fixture has hand-specified expected coordinate eligibility: numbers 1 and 2 only.
Official format: https://www.imgt.org/IMGT_vquest/user_guide, tables 1, 2, 3, 6.
"""
import csv
import hashlib
import json
import platform
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
DIR = ROOT / 'testdata/imgt-inputs'
NAMES = ['1_Summary', '2_IMGT-gapped-nt-sequences', '3_Nt-sequences', '6_Junction']
tables = []
sources = {}
for name in NAMES + ['11_Parameters']:
    path = DIR / f'{name}.txt'
    sources[path.name] = hashlib.sha256(path.read_bytes()).hexdigest()
    if name in NAMES:
        with path.open(newline='') as handle:
            rows = list(csv.DictReader(handle, delimiter='\t'))
        tables.append({(r['Sequence number'], r['Sequence ID']): r for r in rows})
assert all(set(t) == set(tables[0]) for t in tables)
result = []
for key, summary in tables[0].items():
    gap, nt, junction = (t[key] for t in tables[1:])
    evidence = {'sequence': summary['Sequence'], 'rev_comp': 'F',
                'sequence_alignment': gap['V-D-J-REGION'] or gap['V-J-REGION'] or gap['V-REGION']}
    if key[0] in ('1', '2'):
        for src, dst in [('V-REGION', 'v_sequence'), ('J-REGION', 'j_sequence'), ('CDR3-IMGT', 'cdr3')]:
            start, end = int(nt[f'{src} start']), int(nt[f'{src} end'])
            assert summary['Sequence'][start-1:end] == nt[src]
            evidence[f'{dst}_start'] = str(start)
            evidence[f'{dst}_end'] = str(end)
    calls = [word for word in summary['V-GENE and allele'].replace(',', ' ').split() if word.startswith(('IGHV', 'IGKV', 'IGLV'))]
    result.append({'number': key[0], 'id': key[1], 'v': ','.join(sorted(set(calls))),
                   'junction': junction['JUNCTION'].upper(), 'junctionAA': junction['JUNCTION (AA)'].upper(),
                   'evidence': evidence, 'orientation': summary['Orientation']})
out = {'python': platform.python_version(), 'method': 'CPython stdlib csv; documented IMGT columns and hand-specified coordinate eligibility',
       'sources': sources, 'rows': result}
Path(__file__).with_name('imgt-inputs.json').write_text(json.dumps(out, indent=2) + '\n')
