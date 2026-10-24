"""Independent cell-level constant-gene tallies from the default v2 study's truth.

uv run --no-project validation/reference/isotypes.py /path/to/default-v2-study
Uses only Python stdlib, raw AIRR rows and generating family identities. No
ClonoLoom family, isotype or gene parser is called. This fixture has unambiguous
calls; ambiguity and bulk abundance policies have separate hand-calculated tests.
"""
import csv
import hashlib
import itertools
import json
from pathlib import Path
import platform
import sys
from collections import Counter, defaultdict

study = Path(sys.argv[1])
truth = json.loads((study / 'truth/truth.json').read_text())
assert truth['model'] == 'clonoloom-artificial-bcell-v2'
assert truth['study']['seed'] == 20261027
nodes = {n['id']: n for n in truth['nodes']}
observations = {(r['sample_id'], r['cell_id']): nodes[r['node_id']] for r in csv.DictReader((study / 'truth/observations.tsv').open(), delimiter='\t')}
sources, seen = [], set()
members, family_genes = defaultdict(list), defaultdict(set)
usage = Counter()
classes = {'IGHM': 'IgM', 'IGHG1': 'IgG', 'IGHA1': 'IgA'}
for path in sorted(study.glob('*.tsv')):
    sources.append({'file': path.name, 'sha256': hashlib.sha256(path.read_bytes()).hexdigest()})
    for row in csv.DictReader(path.open(), delimiter='\t'):
        if row['locus'] != 'IGH':
            continue
        key = row['repertoire_id'], row['cell_id']
        assert key not in seen
        seen.add(key)
        node = observations[key]
        family = node['family']
        gene = row['c_call'].split('*')[0]
        assert gene == node['isotype'] and gene in classes
        members[family].append(row['sequence_id'])
        family_genes[family].add(gene)
        for group in ['', family]:
            usage[group, key[0], 'gene', gene] += 1
            usage[group, key[0], 'class', classes[gene]] += 1
assert len(seen) == len(observations) == 2160
pairs = Counter(pair for genes in family_genes.values() for pair in itertools.combinations(sorted(genes), 2))
digests = {family: hashlib.sha256(('\n'.join(sorted(rows))+'\n').encode()).hexdigest() for family, rows in members.items()}
output = {'python': platform.python_version(), 'method': 'Direct cell tallies and unordered gene pairs in generating families', 'sources': sources,
          'observations': len(seen), 'families': len(members),
          'usage': [{'membersSHA256': digests.get(family, ''), 'sample': sample, 'level': level, 'state': state, 'count': count} for (family, sample, level, state), count in sorted(usage.items())],
          'pairs': [{'left': a, 'right': b, 'families': count} for (a, b), count in sorted(pairs.items())]}
Path(__file__).with_suffix('.json').write_text(json.dumps(output, indent=2)+'\n')
print(f'{len(seen)} cells, {len(members)} families, {len(usage)} usage rows, {dict(pairs)}')
