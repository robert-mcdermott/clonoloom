# /// script
# requires-python = ">=3.12"
# dependencies = ["scipy==1.17.0"]
# ///
"""Independent raw-row grouping, entropy and SciPy Welch/BY reference.

Public statistical APIs only; no external implementation sources. Exhaustive
conditional rank tails follow the declared absolute-centered convention.
uv run --no-project validation/reference/robustness.py
"""
import itertools
import json
import math
from collections import defaultdict
from pathlib import Path
import scipy
from scipy import stats

rows = []
seqs = [('TGTGCTTTT', 'CAF', 'TRBV1*01'), ('TGCGCTTTT', 'CAF', 'TRBV1*02'),
        ('TGTGCTTTC', 'CAF', 'TRBV2*01'), ('TGTGGTTTT', 'CGF', 'TRBV1*01'),
        ('TGTGATTTT', 'CDF', 'TRBV1*01'), ('TGTTCTTTT', 'CSF', 'TRBV1*01')]
for i in range(8):
    for j, (nt, aa, v) in enumerate(seqs):
        count = (i+2)*(j+1) + (i*j)%7
        if j >= 3 and j > 3 + i%3:
            count = 0
        if i >= 4 and j < 3:
            count *= 3
        rows.append(dict(sample=f'S{i}', nt=nt, aa=aa, v=v, j='TRBJ1*01', count=count))
defs = [dict(junction='aa',v='gene',j='gene'), dict(junction='nt',v='gene',j='gene'),
        dict(junction='aa',v='allele',j='allele'),dict(junction='aa',v='none',j='none')]
features = ['abundance','hill1','richness','shannon']
variants=[]
for definition in defs:
    values=[]
    for i in range(8):
        bins=defaultdict(int)
        for row in rows:
            if row['sample']!=f'S{i}' or row['count']==0:
                continue
            calls=[row[g] if definition[g]=='allele' else row[g].split('*')[0] if definition[g]=='gene' else '' for g in ['v','j']]
            bins[tuple([row[definition['junction']]]+calls)]+=row['count']
        xs=sorted(bins.values()); total=sum(xs); entropy=-sum(x/total*math.log(x/total) for x in xs)
        values.append(dict(abundance=total,hill1=math.exp(entropy),richness=len(xs),shannon=entropy))
    comparisons=[]
    for f in features:
        a=[v[f] for v in values[:4]];b=[v[f] for v in values[4:]]
        test=stats.ttest_ind(b,a,equal_var=False);ci=test.confidence_interval(.95)
        ranks=[round(2*x) for x in stats.rankdata(a+b)]; center=4*9
        score=abs(sum(ranks[4:])-center)
        null=[abs(sum(ranks[i] for i in indexes)-center) for indexes in itertools.combinations(range(8),4)]
        comparisons.append(dict(feature=f,mean=sum(b)/4-sum(a)/4,lower=float(ci.low),upper=float(ci.high),p=float(test.pvalue),rankP=sum(x>=score for x in null)/len(null)))
    variants.append(dict(definition=definition,values=values,comparisons=comparisons))
ps=[c[k] for v in variants for c in v['comparisons'] for k in ['p','rankP']]
by=[float(q) for q in stats.false_discovery_control(ps,method='by')]
Path(__file__).with_suffix('.json').write_text(json.dumps(dict(scipy=scipy.__version__,rows=rows,variants=variants,features=features,by=by),indent=2,allow_nan=False)+'\n')
