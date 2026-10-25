# /// script
# requires-python = ">=3.12"
# dependencies = ["scipy==1.17.0"]
# ///
# Independent exhaustive reference: itertools subsets, set-based edges, SciPy public BY API.
# Does not read ClonoLoom implementation/results or third-party source code.
# uv run --no-project validation/reference/modularity.py
import itertools
import json
import math
from collections import Counter
from pathlib import Path
import scipy
from scipy.stats import false_discovery_control

cases = []
for name, n, edges, labels in [
    ('clique-and-path', 12, list(itertools.combinations(range(4), 2)) + [(i, i+1) for i in range(4, 11)], ['A']*4 + ['B', 'C', 'D', 'B', 'C', 'D', 'B', 'C']),
    ('isolated-and-excluded', 8, [(0,1),(0,2),(1,2),(3,4),(4,5),(0,7)], ['A','A','A','B','C','B','D','']),
    ('monte-carlo-target', 20, list(itertools.combinations(range(5), 2)), [str(i//5) for i in range(20)]),
]:
    eligible = [i for i in range(n) if labels[i]]
    included = set(eligible)
    induced = [e for e in edges if set(e) <= included]
    clones = []
    nulls = []
    sizes = sorted({labels.count(c) for c in set(labels)-{''} if 2 <= labels.count(c) < len(eligible)})
    for size in sizes:
        # Deliberately enumerate ALL subsets, including the Monte Carlo target's 15504.
        hist = Counter(sum(set(e) <= set(v) for e in induced) for v in itertools.combinations(eligible, size))
        draws = math.comb(len(eligible), size)
        expectation = sum(k*v for k, v in hist.items()) / draws
        nulls.append(dict(size=size, draws=draws, expected=expectation, histogram=[dict(edges=k,count=v) for k,v in sorted(hist.items())]))
    for clone in sorted(set(labels)-{''}):
        members = {i for i in eligible if labels[i] == clone}
        observed = sum(set(e) <= members for e in induced)
        record = dict(id=clone, cells=len(members), observed=observed, expected=None, score=None, p=None, adjusted=None)
        if 2 <= len(members) < len(eligible):
            null = next(v for v in nulls if v['size'] == len(members))
            record.update(expected=null['expected'], score=math.log2((observed+1)/(null['expected']+1)), p=sum(b['count'] for b in null['histogram'] if b['edges'] >= observed)/null['draws'])
        clones.append(record)
    tested = [v for v in clones if v['p'] is not None]
    for row, value in zip(tested, false_discovery_control([v['p'] for v in tested], method='by')):
        row['adjusted'] = float(value)
    cases.append(dict(name=name,n=n,edges=edges,labels=labels,eligible=len(eligible),inducedEdges=len(induced),clones=clones,nulls=nulls))
Path(__file__).with_suffix('.json').write_text(json.dumps(dict(reference=f'Python itertools exhaustive subsets; SciPy {scipy.__version__} false_discovery_control(method=by)',cases=cases), indent=2)+'\n')
