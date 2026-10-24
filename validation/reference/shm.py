"""Independent net SHM oracle from known ancestors, not reconstructed alignments.

uv run --no-project validation/reference/shm.py /path/to/default-v2-study
Uses Python stdlib and the NCBI standard code. Deliberate reversals are netted by
comparing final sequence with root, never by summing the recorded branch events.
The fixed artificial regions are a simulation contract, not biological numbering.
"""
import csv
import hashlib
from itertools import product
import json
from pathlib import Path
import platform
import sys
from collections import Counter

study = Path(sys.argv[1])
truth = json.loads((study / 'truth/truth.json').read_text())
assert truth['model'] == 'clonoloom-artificial-bcell-v2'
assert truth['study']['seed'] == 20261027
code = dict(zip(map(''.join, product('TCAG', repeat=3)),
                'FFLLSSSSYY**CC*WLLLLPPPPHHQQRRRRIIIMTTTTNNKKSSRRVVVVAAAADDEEGGGG'))
roots = {n['family']: n for n in truth['nodes'] if not n['parent']}
observations = {(r['sample_id'], r['cell_id']): r for r in csv.DictReader((study / 'truth/observations.tsv').open(), delimiter='\t')}
regions = {'all': (0, 360), 'V': (0, 285), 'J': (318, 360),
           'fwr1': (0, 78), 'cdr1': (78, 114), 'fwr2': (114, 165),
           'cdr2': (165, 195), 'fwr3': (195, 285), 'cdr3': (285, 318), 'fwr4': (318, 360)}
lines, totals, sources = [], Counter(), []
row_count = 0
for path in sorted(study.glob('*.tsv')):
    sources.append({'file': path.name, 'sha256': hashlib.sha256(path.read_bytes()).hexdigest()})
    for row in csv.DictReader(path.open(), delimiter='\t'):
        row_count += 1
        family = observations[row['repertoire_id'], row['cell_id']]['node_id'].rsplit('-N', 1)[0]
        reference = roots[family]['heavy' if row['locus'] == 'IGH' else 'light']['sequence']
        query = row['sequence']
        assert query[282:321] == row['junction'] and row['vj_in_frame'] == 'T'
        for region, (start, end) in regions.items():
            positions = set(range(start, end)) - set(range(285, 318))
            differences = {p for p in positions if query[p] != reference[p]}
            n = Counter(evaluableBases=len(positions), mutations=len(differences), masked=end-start-len(positions))
            for a in range(start, end, 3):
                if a not in positions:
                    continue
                q, g = query[a:a+3], reference[a:a+3]
                assert code[q] != '*' and code[g] != '*'
                n['evaluableCodons'] += 1
                hits = len(differences & set(range(a, a+3)))
                if hits:
                    effect = 'silent' if code[q] == code[g] else 'replacement'
                    n[effect+'Codons'] += 1
                    if hits == 1:
                        n[effect] += 1
                    else:
                        n['multiHitCodons'] += 1
            n['unclassified'] = n['mutations'] - n['replacement'] - n['silent']
            fields = ['evaluableBases', 'mutations', 'masked', 'evaluableCodons', 'replacement', 'silent', 'unclassified', 'multiHitCodons', 'replacementCodons', 'silentCodons']
            lines.append('\t'.join([row['repertoire_id'], row['sequence_id'], row['locus'], region, *[str(n[k]) for k in fields]]))
            if region == 'all':
                totals.update(n)
result = {'python': platform.python_version(), 'method': 'Final observed sequences versus known roots, fixed artificial regions, NCBI table 1; single-hit substitution attribution and endpoint codons',
          'model': truth['model'], 'rows': row_count, 'regionRows': len(lines),
          'valuesSHA256': hashlib.sha256(('\n'.join(sorted(lines))+'\n').encode()).hexdigest(),
          'totals': dict(totals), 'sources': sources}
Path(__file__).with_name('shm.json').write_text(json.dumps(result, indent=2)+'\n')
print(json.dumps({k: v for k, v in result.items() if k != 'sources'}, indent=2))
