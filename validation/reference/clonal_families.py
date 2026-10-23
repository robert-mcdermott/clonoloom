"""Independent CSV / adjacency-graph reference for artificial B-cell observations.

Run with uv run --no-project validation/reference/clonal_families.py STUDY OUT.
No Go output or ancestor sequences enter clustering. Truth is read only afterwards
for recovery checks. Uses a breadth-first component traversal, not Go's union-find.
Gupta et al. 2017, doi:10.4049/jimmunol.1601850. Paired maximum is ClonoLoom policy.
"""
import csv
import hashlib
import itertools
import json
from pathlib import Path
import platform
import struct
import sys

root, output = map(Path, sys.argv[1:])
meta = json.loads((root / 'repertoires.airr.json').read_text())['Repertoire']
subjects = {r['repertoire_id']: r['subject']['subject_id'] for r in meta}
cells = {}
files = []
for path in sorted(root.glob('*.tsv')):
    files.append({'name': path.name, 'sha256': hashlib.sha256(path.read_bytes()).hexdigest()})
    for r in csv.DictReader(path.open(), delimiter='\t'):
        assert r['productive'] == 'T' and set(r['junction']) <= set('ACGT')
        name = r['repertoire_id'] + ':' + r['cell_id']
        cells.setdefault(name, []).append((r['locus'], r['v_call'].split('*')[0], r['j_call'].split('*')[0], r['junction']))

def digest(groups):
    return hashlib.sha256(('\n'.join(sorted('\t'.join(sorted(g)) for g in groups)) + '\n').encode()).hexdigest()

cases = []
for mode in ['heavy', 'paired']:
    represented = {}
    for name, chains in cells.items():
        chain = tuple(sorted(c for c in chains if mode == 'paired' or c[0] == 'IGH'))
        assert len(chain) == (2 if mode == 'paired' else 1)
        key = (subjects[name.split(':')[0]], chain)
        represented.setdefault(key, []).append(name)
    keys = sorted(represented)
    partitions = {}
    for i, (subject, chains) in enumerate(keys):
        partition = (subject, tuple((locus, v, j, len(seq)) for locus, v, j, seq in chains))
        partitions.setdefault(partition, []).append(i)
    edges = []
    nearest = [None] * len(keys)
    for indices in partitions.values():
        for i, j in itertools.combinations(indices, 2):
            d = max(sum(a != b for a, b in zip(c1[3], c2[3])) / len(c1[3]) for c1, c2 in zip(keys[i][1], keys[j][1]))
            edges.append((i, j, d))
            for n in [i, j]:
                nearest[n] = d if nearest[n] is None else min(nearest[n], d)
    lines = []
    for key, distance in zip(keys, nearest):
        value = 'NA' if distance is None else struct.pack('>d', distance).hex()
        lines.extend(name + '\t' + value for name in represented[key])
    nearest_digest = hashlib.sha256(('\n'.join(sorted(lines)) + '\n').encode()).hexdigest()
    for threshold in [0, 0.025, 0.05, 0.1]:
        adjacent = [set() for _ in keys]
        for i, j, d in edges:
            if d <= threshold:
                adjacent[i].add(j)
                adjacent[j].add(i)
        groups, unseen = [], set(range(len(keys)))
        while unseen:
            frontier, component = {min(unseen)}, set()
            while frontier:
                component |= frontier
                frontier = set.union(set(), *(adjacent[n] for n in frontier)) - component
            unseen -= component
            groups.append([name for n in component for name in represented[keys[n]]])
        cases.append({'mode': mode, 'threshold': threshold, 'families': len(groups), 'representatives': len(keys), 'comparisons': len(edges), 'membershipSHA256': digest(groups), 'nearestSHA256': nearest_digest})

# Ground truth is a separate evaluation, never a clustering input.
truth = json.loads((root / 'truth/truth.json').read_text())
node_family = {n['id']: n['family'] for n in truth['nodes']}
true_groups = {}
for r in csv.DictReader((root / 'truth/observations.tsv').open(), delimiter='\t'):
    true_groups.setdefault(node_family[r['node_id']], []).append(r['sample_id'] + ':' + r['cell_id'])
truth_digest = digest(true_groups.values())
for c in cases:
    c['exactTruthRecovery'] = c['membershipSHA256'] == truth_digest
out = {'python': platform.python_version(), 'method': 'Independent CSV, direct distances and breadth-first connected components; all observations', 'files': files, 'observations': len(cells), 'truthFamilies': len(true_groups), 'cases': cases}
output.write_text(json.dumps(out, indent=2) + '\n')
print(json.dumps(out, indent=2))
