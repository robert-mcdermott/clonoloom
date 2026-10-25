# /// script
# requires-python = ">=3.12"
# dependencies = ["scipy==1.17.0"]
# ///
"""Independent graphs: fixed-length CDR3 substitution arithmetic, SciPy Euclidean/hypergeom.
No ClonoLoom results or external implementation source is read. Run with uv run --offline.
"""
import itertools
import json
import math
from pathlib import Path
import scipy
from scipy.spatial.distance import euclidean
from scipy.stats import hypergeom

motifs = ['AAAA', 'AAAS', 'AASS', 'ASSS', 'CCCC', 'CCCT', 'CCTT', 'CTTT', 'TTTT']
# BLOSUM62: A-S and S-T score 1; all other distinct pairs in A,S,C,T score <=0.
def residue(a, b):
    return 0 if a == b else 3 if set((a, b)) in ({'A','S'}, {'S','T'}) else 4

def receptor_distance(a, b):
    return sum(3 * sum(residue(x,y) for x,y in zip(c['junction'][3:-2],d['junction'][3:-2])) for c,d in zip(a,b))

cases=[]
for name, order in [('aligned', list(range(9))), ('permuted', [0,5,2,7,4,1,6,3,8]), ('all-expression-tied', [0]*9)]:
    nodes=[]
    for i, motif in enumerate(motifs):
        chains=[dict(locus='TRA',junction='CAV'+motif+'AF',v='TRAV1',j='TRAJ1'),dict(locus='TRB',junction='CAS'+motif+'TF',v='TRBV1',j='TRBJ1')]
        if i==8: chains[0]['junction']='CAVAAAAAF' # shared alpha with node 0
        scores=[[order[i]*2-1,0], [order[i]*2+1,0]]
        nodes.append(dict(id=f'C{i}',chains=chains,scores=scores))
    means=[[sum(c[q] for c in n['scores'])/len(n['scores']) for q in range(2)] for n in nodes]
    tests=[]; candidates=[]
    for i,a in enumerate(nodes):
        pool=[j for j,b in enumerate(nodes) if i!=j and all(c['junction']!=d['junction'] for c,d in zip(a['chains'],b['chains']))]
        candidates.append(len(pool))
        gd={j:euclidean(means[i],means[j]) for j in pool};td={j:receptor_distance(a['chains'],nodes[j]['chains']) for j in pool}
        for k in [1,2,3,9]:
            if k>=len(pool):continue
            gs=sorted(j for j in pool if gd[j]<=sorted(gd.values())[k-1]);ts=sorted(j for j in pool if td[j]<=sorted(td.values())[k-1]);overlap=len(set(gs)&set(ts))
            p=float(hypergeom.sf(overlap-1,len(pool),len(gs),len(ts)))
            # Small exact subset enumeration independently checks SciPy's probability.
            exhaustive=sum(len(set(s)&set(gs))>=overlap for s in itertools.combinations(pool,len(ts)))/math.comb(len(pool),len(ts))
            assert abs(p-exhaustive)<1e-14
            tests.append(dict(node=i,k=k,expression=gs,receptor=ts,overlap=overlap,expected=len(gs)*len(ts)/len(pool),p=p))
    for t in tests:t['adjusted']=min(1,t['p']*len(tests))
    cases.append(dict(name=name,nodes=nodes,means=means,candidates=candidates,tests=tests))

# Entire small null distribution, including ties, must have super-uniform upper-tail p-values.
for n in range(2,10):
    for a in range(n+1):
        for b in range(n+1):
            subs=list(itertools.combinations(range(n),b))
            values=[len(set(s)&set(range(a))) for s in subs]
            ps=[sum(v>=x for v in values)/len(values) for x in values]
            for alpha in [.01,.05,.1,.5,1]:assert sum(p<=alpha for p in ps)/len(ps)<=alpha+1e-14

tails=[]
for n,a,b,x in [(7,2,3,0),(7,2,3,1),(7,2,3,2),(99,10,12,8),(1999,200,200,200),(2000,1000,1000,1000),(2000,2000,1000,1000)]:
    tails.append(dict(n=n,a=a,b=b,x=x,logP=float(hypergeom.logsf(x-1,n,a,b))))
Path(__file__).with_suffix('.json').write_text(json.dumps(dict(reference=f'SciPy {scipy.__version__}; itertools exhaustive subsets; BLOSUM62 restricted alphabet arithmetic',cases=cases,tails=tails),indent=2)+'\n')
