"""Independent CSV, identity, missingness and hash oracle; no network.
uv run --no-project validation/reference/cytoweave.py OUTPUT_DIRECTORY
"""
import csv
import hashlib
import io
import json
from pathlib import Path
import sys

root = Path(sys.argv[1])
count = 0
for file in sorted(root.rglob('links.json')):
    d = json.loads(file.read_bytes())
    assert d['format'] == 'clonoloom-cytoweave-links' and d['version'] == 1
    for key in ('wells', 'mapping', 'provenance'):
        assert hashlib.sha256(d[key]['text'].encode()).hexdigest() == d[key]['sha256']
    assert json.loads(d['provenance']['text']) == d['declaration']
    wells = list(csv.reader(io.StringIO(d['wells']['text'].removeprefix('\ufeff'), newline='')))
    mapping = dict(list(csv.reader(io.StringIO(d['mapping']['text'].removeprefix('\ufeff'), newline='')))[1:])
    receptors = []
    for source in d['sources']:
        raw = Path(source['path']).read_bytes()
        assert hashlib.sha256(raw).hexdigest() == source['sha256']
        receptors.extend(csv.DictReader(io.StringIO(raw.decode(), newline=''), delimiter='\t'))
    sample = d['sample']['repertoireId']
    cells = {}
    missing = 0
    for i, row in enumerate(receptors):
        if row['repertoire_id'] != sample:
            continue
        if not row['cell_id']:
            missing += 1
        else:
            cells.setdefault(row['cell_id'], []).append(i)
    result = d['result']
    assert result['columns'] == wells[0]
    assert result['receptorCells'] == len(cells) and result['missingBarcodeRows'] == missing
    assert len(result['observations']) == len(wells) - 1
    matched, unmapped, absent = set(), 0, 0
    for row, obs in zip(wells[1:], result['observations']):
        assert obs['values'] == row
        cell = mapping.get(row[0], '')
        assert obs['cell'] == cell and obs['receptorRows'] == cells.get(cell, [])
        if not cell:
            assert obs['status'] == 'unmapped'
            unmapped += 1
        elif cell not in cells:
            assert obs['status'] == 'absent receptor target'
            absent += 1
        else:
            assert obs['status'] == 'linked'
            matched.add(cell)
    assert result['matched'] == len(matched)
    assert result['unmapped'] == unmapped and result['absentTargets'] == absent
    assert result['receptorOnly'] == sorted(set(cells) - matched)
    if file.parent.name == 'sibling':
        assert d['declaration']['sourceSHA256'] == hashlib.sha256(file.with_name('artificial.fcs').read_bytes()).hexdigest()
        assert len(matched) == 95
    else:
        assert (len(matched), unmapped, absent, len(cells)-len(matched)) == (80, 6, 10, 4)
    count += 1
    print(f'{file.parent.name}: all source values, hashes, sample scope, chain rows and missing/unmatched identities agree.')
assert count, 'No exported link evidence'
print(f'{count} exact interchange records audited; no biological validity or phenotype inference is asserted.')
