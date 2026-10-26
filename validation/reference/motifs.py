# /// script
# requires-python = ">=3.12"
# dependencies = ["scipy==1.17.0", "biopython==1.86"]
# ///
"""Independent brute-force signature membership and public SciPy Fisher/BY APIs.
BLOSUM62 values are data loaded through Biopython's public API. No GLIPH source.
uv run --no-project validation/reference/motifs.py
"""
import itertools
import json
from pathlib import Path
from collections import defaultdict
from scipy import stats
from Bio.Align import substitution_matrices
matrix=substitution_matrices.load('BLOSUM62')
query=['CASSGITGQANEQYF','CASSGLTGQANEQYF','CASSGMTGQANEQYF','CASSGVTGQANEQYF','CASSQGHTANEQYF','CASSAQGHTNEQYF','CASSNQGHTAEQYF','CASSASSSGGEQYF']
background=['CASSASGTNREQYF','CASSGGGTTSEQYF','CASSNSGGTREQYF','CASSRSNNGTEQYF','CASSGITGQTNEQYF','CASSGATGQANEQYF','CASSGSTGQANEQYF','CASSQGHTGNEQYF','CASSYSGSNGEQYF']
records=[dict(v='TRBV5-1',junction=s,arm='query' if i<len(query) else 'background') for i,s in enumerate(query+background)]
# Unmatched strata stay in the family, with unavailable tests.
records += [dict(v='TRBV6-5',junction='CASSQGHTANEQYF',arm='query')]
strata=defaultdict(list)
for i,r in enumerate(records):strata[(r['v'],len(r['junction']))].append(i)
patterns=set()
for (v,length),indexes in strata.items():
    for i in indexes:
        s=records[i]['junction']
        for k in range(2,6):
            for start in range(3,length-3-k+1):
                # Enumerate each declared window, then any motif observed within it.
                for pos in range(start,min(start+3,length-3-k)+1):
                    patterns.add(('local',v,length,s[pos:pos+k],'',start+1,min(start+3,length-3-k)+1))
    globals=defaultdict(set)
    for i in indexes:
        s=records[i]['junction']
        for pos in range(3,length-3):globals[(s[:pos]+'%'+s[pos+1:],pos)].add(s[pos])
    for (mask,pos),letters in globals.items():
        # Exhaustive subsets, independently of the Go clique search.
        complete=[]
        for n in range(2,len(letters)+1):
            for subset in itertools.combinations(sorted(letters),n):
                if all(matrix[a,b]>0 for a,b in itertools.combinations(subset,2)):complete.append(set(subset))
        for c in complete:
            if not any(c<other for other in complete):patterns.add(('global',v,length,mask,''.join(sorted(c)),pos+1,pos+1))
rows=[]
for kind,v,length,motif,allowed,start,end in sorted(patterns):
    arms={'query':[], 'background':[]}; totals={'query':0,'background':0}
    for i in strata[(v,length)]:
        r=records[i];s=r['junction'];totals[r['arm']]+=1
        if kind=='local':match=any(s[p:p+len(motif)]==motif for p in range(start-1,end))
        else:match=s[start-1] in allowed and all(a=='%' or a==b for a,b in zip(motif,s))
        if match:arms[r['arm']].append(s)
    a,b=len(arms['query']),len(arms['background']);n,m=totals['query'],totals['background']
    p=float(stats.fisher_exact([[a,n-a],[b,m-b]],alternative='greater').pvalue) if n and m else None
    rows.append(dict(kind=kind,v=v,length=length,motif=motif,allowed=allowed,start=start,end=end,query=sorted(arms['query']),background=sorted(arms['background']),queryTotal=n,backgroundTotal=m,p=p))
qs=stats.false_discovery_control([r['p'] if r['p'] is not None else 1 for r in rows],method='by')
for row,q in zip(rows,qs):row['q']=float(q) if row['p'] is not None else None
Path(__file__).with_suffix('.json').write_text(json.dumps(dict(records=records,groups=rows),indent=2,allow_nan=False)+'\n')
