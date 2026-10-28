"""Independent raw-abundance reference; Python stdlib only, no ClonoLoom imports.

Run: UV_CACHE_DIR=/tmp/clonoloom-uv uv run --offline --no-project python validation/reference/cohort_features.py
Jost 2006 doi:10.1111/j.2006.0030-1299.14714.x; Chao & Jost 2012
 doi:10.1890/11-1952.1. Gini uses pairwise differences, not sorted ranks.
"""
import json
import math
from pathlib import Path

cases = {
    "empty": [], "one": [8], "one_singleton": [1], "equal": [4, 4, 4, 4],
    "skew": [1, 1, 2, 7, 19], "singletons": [1, 1, 1, 1],
    "no_doubletons": [1, 1, 4, 8], "fractional": [0.5, 1.5, 3.25],
    "more_than_ten": list(range(1, 14)), "scaled": [10, 10, 20, 70, 190],
}
rows = []
for name, counts in cases.items():
    n, s = sum(counts), len(counts)
    out = dict.fromkeys(["shannon", "hill1", "hill2", "evenness", "clonality", "simpson", "giniSimpson", "gini", "maxFrequency", "top10", "coverage"])
    out.update(abundance=n, richness=s)
    if n:
        p = [x / n for x in counts]
        entropy = -math.fsum(x * math.log(x) for x in p)
        concentration = math.fsum(x * x for x in p)
        out.update(shannon=entropy, hill1=math.exp(entropy), hill2=1 / concentration,
                   simpson=concentration, giniSimpson=1 - concentration,
                   gini=sum(abs(x-y) for x in counts for y in counts)/(2*s*n),
                   maxFrequency=max(p), top10=sum(sorted(counts, reverse=True)[:10])/n)
        if s > 1:
            out.update(evenness=entropy/math.log(s), clonality=1-entropy/math.log(s))
        if all(float(x).is_integer() for x in counts):
            f1, f2 = counts.count(1), counts.count(2)
            correction = ((n-1)*f1/((n-1)*f1+2*f2) if f2 else
                          (n-1)*(f1-1)/((n-1)*(f1-1)+2))
            out["coverage"] = 1-f1/n*correction
    rows.append({"name": name, "counts": counts, "expected": out})
Path(__file__).with_suffix('.json').write_text(json.dumps({"cases": rows}, indent=2, allow_nan=False)+'\n')
