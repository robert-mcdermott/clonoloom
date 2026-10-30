"""Independent JC69 oracle: sum all ancestral assignments, no pruning code.
uv run --with numpy --with scipy validation/reference/lineage_likelihood.py DIRECTORY
"""
from functools import cache
from itertools import product
from collections import Counter
import hashlib
import json
from pathlib import Path
import sys
import numpy as np
from scipy.linalg import expm
from scipy.optimize import minimize

@cache
def trees(leaves):
    if len(leaves) == 1:
        return (leaves[0],)
    out = []
    for bits in range(1 << (len(leaves)-1)):
        left = (leaves[0],) + tuple(x for i, x in enumerate(leaves[1:]) if bits & (1 << i))
        right = tuple(x for x in leaves if x not in left)
        if right:
            out.extend(product(trees(left), trees(right)))
    return tuple(out)

def topology(tree, n):
    edges, next_node = [], n
    def visit(node):
        nonlocal next_node
        if isinstance(node, int):
            return node, f't{node:02}'
        here = next_node
        next_node += 1
        labels = []
        for child in node:
            kid, label = visit(child)
            edges.append((here, kid))
            labels.append(label)
        return here, '(' + ','.join(sorted(labels)) + ')'
    child, key = visit(tree)
    edges.append((0, child))
    return '(' + key + ')', edges, next_node

class Enumeration:
    def __init__(self, seqs, edges, nodes):
        self.edges = edges
        self.n = len(seqs)
        patterns = Counter(zip(*seqs))
        self.patterns = list(patterns)
        self.weight = np.array(list(patterns.values()))
        ancestral = list(product(range(4), repeat=nodes-self.n))
        self.states = np.zeros((nodes, len(ancestral), len(patterns)), dtype=int)
        for tip in range(self.n):
            self.states[tip] = np.array(['ACGT'.index(p[tip]) for p in patterns])[None, :]
        for k in range(nodes-self.n):
            self.states[self.n+k] = np.array([p[k] for p in ancestral])[:, None]
        self.c = np.array([(self.states[u] == self.states[v]).astype(float)-.25 for u, v in edges])
    def evaluate(self, lengths, posterior=False):
        decay = np.exp(-4*np.array(lengths)/3)
        factors = .25 + decay[:, None, None]*self.c
        joint = .25 * np.prod(factors, axis=0)
        probability = joint.sum(axis=0)
        if np.any(probability <= 0):
            return (1e100, np.zeros(len(lengths)))
        ll = np.dot(self.weight, np.log(probability))
        if posterior:
            return [[(joint*(state == k)).sum(axis=0)/probability for k in range(4)] for state in self.states]
        gradient = []
        for i in range(len(lengths)):
            derivative = .25*(-4/3)*decay[i]*self.c[i]*np.prod(np.delete(factors, i, axis=0), axis=0)
            gradient.append(np.dot(self.weight, derivative.sum(axis=0)/probability))
        return -ll, -np.array(gradient)

# Matrix exponential independently checks transition units and the finite bound.
Q = np.full((4, 4), 1/3)
np.fill_diagonal(Q, -1)
for t in [0, .001, .5, 3]:
    expected = .25 + np.exp(-4*t/3)*(np.eye(4)-.25)
    np.testing.assert_allclose(expm(Q*t), expected, atol=1e-14, rtol=0)

count = 0
for file in sorted(Path(sys.argv[1]).rglob('*.json')):
    d = json.loads(file.read_bytes())
    if 'panel' not in d or 'result' not in d or d['result'].get('model') != 'JC69':
        continue
    panel, result = d['panel'], d['result']
    tips = [n for n in panel['nodes'] if n['kind'] == 'reference'] + sorted((n for n in panel['nodes'] if n['kind'] == 'observed'), key=lambda n:n['sequence'])
    seqs = [n['sequence'] for n in tips]
    if 'parentText' in d:
        assert hashlib.sha256(d['parentText'].encode()).hexdigest() == d['parentSHA256']
        parent = json.loads(d['parentText'])
        assert panel in parent['trees']
    fits = {f['topology']: f for f in result['fits']}
    independent = []
    # All topologies are independently generated; full optimization cross-checks
    # cover <=5 tips. At the seven-tip boundary check the selected topology,
    # its nearest competitor and the first canonical topology, plus every
    # ancestral posterior on the selected native tree.
    keys = {topology(t, len(tips))[0] for t in trees(tuple(range(1, len(tips))))}
    assert keys == set(fits)
    native_nodes = {n['id']: n for n in result['nodes']}
    children = {key: [] for key in native_nodes}
    for node in native_nodes.values():
        if node.get('parent'):
            children[node['parent']].append(node['id'])
    def native_key(key):
        node = native_nodes[key]
        if not children[key]:
            return f"t{seqs.index(node['sequence']):02}"
        return '(' + ','.join(sorted(native_key(k) for k in children[key])) + ')'
    root = next(n for n in native_nodes.values() if not n.get('parent'))
    selected = native_key(root['id'])
    assert selected in fits and abs(fits[selected]['logLikelihood']-result['logLikelihood']) < 1e-10
    maximum = max(f['logLikelihood'] for f in fits.values())
    assert selected == min(k for k in keys if maximum-fits[k]['logLikelihood'] <= 1e-8)
    competitor = min((k for k in keys if k != selected), key=lambda k:(-fits[k]['logLikelihood'], k))
    checked = keys if len(tips) <= 5 else {selected, competitor, min(keys)}
    assert all(f['converged'] and f['maxCoordinateGain'] <= 1e-7 for f in fits.values())
    assert 0 < result['work'] <= result['settings']['workBudget']
    for tree in trees(tuple(range(1, len(tips)))):
        key, edges, nodes = topology(tree, len(tips))
        if key not in checked:
            continue
        oracle = Enumeration(seqs, edges, nodes)
        solutions = [minimize(oracle.evaluate, np.full(len(edges), start), jac=True,
                             bounds=[(1e-10, 3)]*len(edges), method='L-BFGS-B',
                             options={'ftol':1e-14, 'gtol':1e-9, 'maxiter':2000, 'maxls':100})
                     for start in [.002, .1, 1]]
        best = min(solutions, key=lambda x:x.fun)
        assert abs(-best.fun-fits[key]['logLikelihood']) < 2e-6, (file, key, best.message, -best.fun, fits[key])
        assert fits[key]['converged'] and fits[key]['maxCoordinateGain'] <= 1e-7
        independent.append(-best.fun)
    assert len(fits) == result['topologies']
    assert len(independent) == len(checked)
    assert abs(max(independent)-result['logLikelihood']) < 2e-6
    # Check saved lengths and every marginal probability on the selected graph.
    nodes = result['nodes']
    by_id = {n['id']:n for n in nodes}
    ids = [n['id'] for n in sorted((x for x in nodes if x['kind'] != 'inferred'), key=lambda x: 0 if x['kind']=='reference' else seqs.index(x['sequence']))]
    ids += [n['id'] for n in nodes if n['kind']=='inferred']
    edges = [(ids.index(n['parent']), ids.index(n['id'])) for n in nodes if n.get('parent')]
    lengths = [n['branchLength'] for n in nodes if n.get('parent')]
    oracle = Enumeration(seqs, edges, len(nodes))
    assert abs(-oracle.evaluate(lengths)[0]-result['logLikelihood']) < 1e-8
    posterior = oracle.evaluate(lengths, posterior=True)
    for i, node_id in enumerate(ids):
        node = by_id[node_id]
        if node['kind'] != 'inferred':
            original = next(n for n in tips if n['id'] == node['sourceNode'])
            assert node['sequence'] == original['sequence']
            assert node['rows'] == original['rows'] and node['annotations'] == original['annotations']
        else:
            assert node['rows'] == 0 and not node['annotations']
        for col, pattern in enumerate(zip(*seqs)):
            j = oracle.patterns.index(pattern)
            np.testing.assert_allclose(node['posterior'][col], [posterior[i][a][j] for a in range(4)], atol=2e-12, rtol=0)
            v = node['posterior'][col]
            mask = sum(1 << a for a in range(4) if max(v)-v[a] <= 1e-10)
            assert node['sequence'][col] == '?ACMGRSVTWYHKDBN'[mask]
        if node.get('parent'):
            assert 0 <= node['branchLength'] <= 3
            assert node['boundary'] == ('zero' if node['branchLength'] == 0 else 'upper' if node['branchLength'] == 3 else 'interior')
            assert abs(node['distance']-by_id[node['parent']]['distance']-node['branchLength']) < 1e-12
    count += 1
    print(f'{file.name}: {len(fits)} enumerated topologies / {len(checked)} optimizer comparisons, optimized likelihood, branch-length likelihood and all marginal probabilities agree.')
assert count
print(f'{count} panels independently checked. Numerical agreement is conditional on JC69, not biological model validation.')
