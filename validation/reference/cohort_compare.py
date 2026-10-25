# /// script
# requires-python = ">=3.12"
# dependencies = ["scipy==1.17.0"]
# ///
"""Independent SciPy t inference and exhaustive conditional rank tails.

No implementation sources read. Public scipy.stats APIs supply t tests, CIs,
and midranks; itertools enumerates all label/sign assignments, including ties.
The absolute centered tail is ClonoLoom's declared two-sided convention.
uv run --no-project validation/reference/cohort_compare.py
"""
import itertools
import json
import math
import warnings
from pathlib import Path
import scipy
from scipy import stats

cases = [
    ('independent-normal', False, [1, 3, 4, 6], [4, 5, 7, 9, 12]),
    ('independent-ties-unbalanced', False, [0, 0, 2], [0, 1, 1, 2, 4]),
    ('all-ties', False, [2, 2, 2], [2, 2, 2]),
    ('separated-constants', False, [1, 1], [5, 5, 5]),
    ('one-constant-group', False, [2, 2], [1, 3, 8]),
    ('two-per-group', False, [1, 2], [4, 9]),
    ('paired-ties-and-zeros', True, [1, 3, 4, 6, 8, 10], [1, 5, 2, 7, 11, 9]),
    ('paired-all-zero', True, [1, 2, 3], [1, 2, 3]),
    ('paired-constant-shift', True, [1, 3, 7], [3, 5, 9]),
    ('paired-positive', True, [1, 3, 6, 10, 15], [2, 6, 11, 17, 26]),
    ('paired-two', True, [1, 3], [2, 7]),
    ('independent-monte-carlo', False, list(range(10)), list(range(3, 13))),
    ('paired-monte-carlo', True, list(range(17)), [i + (i % 7 - 3 or 1) for i in range(17)]),
]
rows = []
for name, paired, a, b in cases:
    with warnings.catch_warnings():
        warnings.simplefilter('ignore')
        t = stats.ttest_rel(b, a) if paired else stats.ttest_ind(b, a, equal_var=False)
        ci = t.confidence_interval(.95)
    mean = sum(y-x for x,y in zip(a,b))/len(a) if paired else sum(b)/len(b)-sum(a)/len(a)
    df, statistic, p, lo, hi = [float(x) for x in [t.df, t.statistic, t.pvalue, ci.low, ci.high]]
    if not math.isfinite(statistic):
        df = statistic = p = lo = hi = None
    if paired:
        diff = [y-x for x,y in zip(a,b)]
        nonzero = [x for x in diff if x]
        ranks = [round(2*x) for x in stats.rankdata([abs(x) for x in nonzero])]
        score = abs(sum(r if x > 0 else -r for r,x in zip(ranks,nonzero)))
        null = [abs(sum(r*s for r,s in zip(ranks,sign))) for sign in itertools.product([-1,1], repeat=len(ranks))]
        rank_stat = sum(r for r,x in zip(ranks,nonzero) if x>0)/2
        probability = sum(1 if x>0 else .5 if x==0 else 0 for x in diff)/len(diff)
        scale = 1/len(a)
    else:
        ranks = [round(2*x) for x in stats.rankdata(a+b)]
        center = len(b)*(len(a)+len(b)+1)
        score = abs(sum(ranks[len(a):])-center)
        null = [abs(sum(ranks[i] for i in subset)-center) for subset in itertools.combinations(range(len(ranks)),len(b))]
        rank_stat = float(stats.mannwhitneyu(b,a,method='asymptotic').statistic)
        # Independently count pairwise wins, not rank sums.
        probability = sum(1 if y>x else .5 if y==x else 0 for x in a for y in b)/(len(a)*len(b))
        scale = 1/len(a)+1/len(b)
    tail = sum(v>=score for v in null)
    radius=math.sqrt(math.log(40)*scale/2)
    rows.append(dict(name=name, paired=paired, a=a, b=b, mean=mean, t=statistic, df=df, p=p, lower=lo, upper=hi,
        rank=rank_stat, rankP=tail/len(null), tail=tail, draws=len(null), probability=probability,
        probabilityLower=max(0,probability-radius), probabilityUpper=min(1,probability+radius)))
# P values with untestable slots conservatively retained as one in the full family.
bh_input=[.001, .02, 1, .25, .04, .001]
bh=[float(x) for x in stats.false_discovery_control(bh_input,method='bh')]
tails = [dict(df=df, t=t, p=float(2*stats.t.sf(t, df)))
         for df in [.5, 1, 2, 3.5, 10, 200, 9998] for t in [0, .1, 2, 10, 1000]]
quantiles = [dict(df=df, p=p, value=float(stats.t.ppf(p, df)))
             for df in [.5, 1, 2, 3.5, 10, 200, 9998] for p in [.005, .1, .5, .9, .975, .995]]
Path(__file__).with_suffix('.json').write_text(json.dumps(dict(scipy=scipy.__version__,cases=rows,bhInput=bh_input,bh=bh,tails=tails,quantiles=quantiles),indent=2,allow_nan=False)+'\n')
