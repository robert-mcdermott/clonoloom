# /// script
# requires-python = ">=3.12"
# ///
"""Independent literal set intersections and exact rational abundance fractions.
Run: uv run --no-project validation/reference/contamination.py
No repertoire implementation is imported or replicated; there are no inferential tests.
"""
import hashlib
import itertools
import json
from fractions import Fraction
from pathlib import Path
nts=['TGTGCTAGCAGCCAGGGTCATACCGCTAACGAGCAGTACTTT','TGTGCTAGCAGCGATCATCGTGGTGCTAACGAGCAGTACTTT','TGTGCTAGCAGCAATCAGGGTCATACCAACGAGCAGTACTTT']
counts=[[100,2,80,0,0],[5,4,6,10,0],[7,0,8,2,0]]
pgens=[None,1e-3,0]
samples=[dict(sample=dict(name=n,subject=sub,batch='lane-1'),total=t,eligible=sum(c[i] for c in counts),rows=3,excluded={})for i,(n,sub,t)in enumerate(zip(['a','b','repeat','c','empty'],['one','two','one','three','four'],[200,1000,250,100,0]))]
records=[dict(id='nt-'+hashlib.sha256(('TRBV5-1\nTRBJ2-7\n'+nt).encode()).hexdigest()[:24],v='TRBV5-1',j='TRBJ2-7',nt=nt,counts=c,pgen=p)for nt,c,p in zip(nts,counts,pgens)]
records.sort(key=lambda x:x['id'])
pairs=[];matches=[];same=0
for a,b in itertools.combinations(range(len(samples)),2):
    if samples[a]['sample']['subject']==samples[b]['sample']['subject']:same+=1;continue
    shared={r['id'] for r in records if r['counts'][a]>0}&{r['id'] for r in records if r['counts'][b]>0}
    pair=dict(a=a,b=b,shared=len(shared),review=0,highPgen=0,unknownPgen=0,sharedA=0,sharedB=0)
    for r in records:
        if r['id'] not in shared:continue
        ca,cb=r['counts'][a],r['counts'][b];fa=Fraction(ca,samples[a]['total']);fb=Fraction(cb,samples[b]['total']);ratio=max(fa,fb)/min(fa,fb)
        review=min(ca,cb)>=2 and max(fa,fb)>=Fraction(1,100)
        pattern='Below review thresholds' if not review else 'Asymmetric observed fractions' if ratio>=20 else 'Shared high abundance'
        pc='Unknown Pgen' if r['pgen'] is None else 'High Pgen: plausible publicness' if r['pgen']>=1e-8 else 'Below declared high-Pgen threshold'
        matches.append(dict(receptor=r['id'],a=a,b=b,countA=ca,countB=cb,fractionA=float(fa),fractionB=float(fb),ratio=float(ratio),review=review,pattern=pattern,pgenClass=pc))
        pair['review']+=review;pair['highPgen']+=r['pgen'] is not None and r['pgen']>=1e-8;pair['unknownPgen']+=r['pgen'] is None;pair['sharedA']+=ca;pair['sharedB']+=cb
    available=samples[a]['eligible']>0 and samples[b]['eligible']>0
    pair['fractionA']=pair['sharedA']/samples[a]['total'] if available else None
    pair['fractionB']=pair['sharedB']/samples[b]['total'] if available else None
    pairs.append(pair)
Path(__file__).with_suffix('.json').write_text(json.dumps(dict(input=dict(samples=samples,receptors=records),pairs=pairs,matches=matches,sameSubjectPairs=same),indent=2)+'\n')
