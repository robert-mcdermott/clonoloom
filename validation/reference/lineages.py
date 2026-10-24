"""Independent exact parsimony reference from the artificial study's known families.

uv run --no-project validation/reference/lineages.py /path/to/default-v2-study
Rooted bipartition enumeration and four-state dynamic programming; no production
alignment/tree code or external implementation is read. Scientific panel states
come directly from sampled sequences and known roots, excluding the fixed mask.
"""
from collections import Counter, defaultdict
from functools import cache
from itertools import product
import csv
import hashlib
import json
from pathlib import Path
import platform
import sys

folder = Path(sys.argv[1])
truth = json.loads((folder / 'truth/truth.json').read_text())
assert truth['model'] == 'clonoloom-artificial-bcell-v2' and truth['study']['seed'] == 20261027
roots = {n['family']: n for n in truth['nodes'] if not n['parent']}
observations = {(r['sample_id'], r['cell_id']): r for r in csv.DictReader((folder / 'truth/observations.tsv').open(), delimiter='\t')}
groups = defaultdict(list)
sources = []
for path in sorted(folder.glob('*.tsv')):
    sources.append({'file': path.name, 'sha256': hashlib.sha256(path.read_bytes()).hexdigest()})
    for r in csv.DictReader(path.open(), delimiter='\t'):
        family = observations[r['repertoire_id'], r['cell_id']]['node_id'].rsplit('-N', 1)[0]
        groups[family, r['locus']].append(r)

@cache
def topologies(leaves):
    if len(leaves) == 1:
        return (leaves[0],)
    trees = []
    # Keep the smallest leaf in the left subset to remove unordered duplicates.
    for bits in range(1 << (len(leaves)-1)):
        left = (leaves[0],) + tuple(x for i, x in enumerate(leaves[1:]) if bits & (1 << i))
        right = tuple(x for x in leaves if x not in left)
        if right:
            trees.extend(product(topologies(left), topologies(right)))
    return tuple(trees)

def exact(sequences):
    if len(sequences) == 1:
        return 0, 1
    patterns = Counter(zip(*sequences))
    patterns = {p: n for p, n in patterns.items() if len(set(p)) > 1}
    @cache
    def costs(tree, pattern):
        if isinstance(tree, int):
            return tuple(0 if b == pattern[tree] else 10000 for b in 'ACGT')
        a, b = (costs(child, pattern) for child in tree)
        return tuple(min(a[j] + (i != j) for j in range(4)) +
                     min(b[j] + (i != j) for j in range(4)) for i in range(4))
    scores = []
    for tree in topologies(tuple(range(1, len(sequences)))):
        score = sum(weight * min(cost + (b != pattern[0]) for b, cost in zip('ACGT', costs(tree, pattern)))
                    for pattern, weight in patterns.items())
        scores.append(score)
    return min(scores), scores.count(min(scores))

output = []
for (family, locus), rows in sorted(groups.items()):
    root = roots[family]['heavy' if locus == 'IGH' else 'light']['sequence']
    panel = lambda s: s[:285] + s[318:]
    reference = panel(root)
    states = sorted({panel(r['sequence']) for r in rows} - {reference})
    sequences = [reference] + states
    score, ties = exact(sequences)
    # Known generating states give an upper bound, not an asserted recoverable topology.
    nodes = {n['id']: n for n in truth['nodes'] if n['family'] == family}
    chain = 'heavy' if locus == 'IGH' else 'light'
    known_score = sum(sum(a != b for a, b in zip(panel(n[chain]['sequence']), panel(nodes[n['parent']][chain]['sequence'])))
                      for n in nodes.values() if n['parent'])
    assert score <= known_score
    ids = '\n'.join(sorted(r['sequence_id'] for r in rows)) + '\n'
    output.append({'family': family, 'locus': locus, 'membersSHA256': hashlib.sha256(ids.encode()).hexdigest(),
                   'rows': len(rows), 'sites': 327, 'taxa': len(sequences), 'score': score,
                   'optimalTrees': ties, 'generatingTreeScore': known_score})
result = {'python': platform.python_version(), 'method': 'Rooted bipartition enumeration; four-state dynamic programming; known roots and fixed artificial V/J panel',
          'model': truth['model'], 'sources': sources, 'trees': output}
Path(__file__).with_name('lineages.json').write_text(json.dumps(result, indent=2)+'\n')
print(f"{len(output)} trees; {sum(t['score'] for t in output)} changes; {sum(t['optimalTrees'] > 1 for t in output)} tied searches")
