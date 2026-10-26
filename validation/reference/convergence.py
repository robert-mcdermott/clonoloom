# /// script
# requires-python = ">=3.12"
# dependencies = ["scipy==1.17.0"]
# ///
"""Independent literal subject/typing aggregation and public SciPy inference.
Run: uv run --no-project validation/reference/convergence.py
No external repertoire-tool implementation source is used.
"""
import json
from pathlib import Path
from scipy import stats
subjects=[]
for i in range(12):
    subjects.append(dict(id=f'S{i:02}', samples=[f's{i}']+(['repeat'] if i==0 else []), typing={'A':['HLA-A*02:01:01','A*03:01'] if i<7 else ['A*01:01','A*24:02'],'B':['B*07:02','B*08:01'] if i%2==0 else ['B*08:01','B*08:01']}))
subjects[10]['typing']['A']=['A*02']  # Incomplete, not a noncarrier.
subjects[11]['typing']['A']=['A*02:01N','A*24:02']
# Subject 9 has no eligible receptor observation; exclude even if HLA-typed.
records=[dict(v='TRBV5-1',junction='CASSQGHTANEQYF',samples=['s0','repeat','s1','s2','s3','s4','s7']),
         dict(v='TRBV5-1',junction='CASSDHRGANEQYF',samples=['s0','s2','s4','s6','s8','s10']),
         dict(v='TRBV5-1',junction='CASSASGTANEQYF',samples=[f's{i}' for i in range(12) if i!=9]),
         dict(v='TRBV5-1',junction='CASSNQGHTNEQYF',samples=['s0','repeat','s1'])]
groups=[dict(id='local-q',label='QGHT signature',members=[0,3]),dict(id='local-d',label='DHRG signature',members=[1]),dict(id='all',label='All observed subjects',members=[2]),dict(id='empty',label='Empty signature',members=[])]
alleles=['HLA-A*02:01','HLA-B*07:02','HLA-A*99:99']
by_sample={sample:s['id'] for s in subjects for sample in s['samples']}
record_subjects=[sorted({by_sample[x] for x in rec['samples']}) for rec in records]
observed=set().union(*map(set,record_subjects))
recurrences=[]; results=[]
for group in groups:
    positive=set().union(*(set(record_subjects[i]) for i in group['members']))
    recurrences.append(dict(id=group['id'],subjects=sorted(positive),observedSubjects=len(observed),frequency=len(positive)/len(observed)))
    for allele in alleles:
        locus=allele.split('*')[0].replace('HLA-','')
        typed={}
        for s in subjects:
            raw=s['typing'].get(locus,[])
            if s['id'] not in observed or len(raw)!=2: continue
            # Literal fixture's known problematic calls; no Go parser is replicated.
            if any(x in ['A*02','A*02:01N'] for x in raw):continue
            typed[s['id']]={'HLA-'+x.removeprefix('HLA-').split(':')[0]+':'+x.removeprefix('HLA-').split(':')[1] for x in raw}
        carriers={s for s,a in typed.items() if allele in a};in_group=positive&set(typed)
        n,c,g,a=len(typed),len(carriers),len(in_group),len(carriers&positive)
        valid=n>=4 and 0<c<n and 0<g<n
        p=float(stats.hypergeom.sf(a-1,n,c,g)) if valid else None
        def freq(k,total):
            if not total:return dict(matches=k,total=total,estimate=None,lower=None,upper=None)
            ci=stats.binomtest(k,total).proportion_ci(confidence_level=.95,method='wilson')
            return dict(matches=k,total=total,estimate=k/total,lower=float(ci.low),upper=float(ci.high))
        results.append(dict(group=group['id'],allele=allele,n=n,carriers=c,groupSubjects=g,shared=a,excluded=len(subjects)-n,p=p,carrier=freq(a,c),noncarrier=freq(g-a,n-c),difference=a/c-(g-a)/(n-c) if 0<c<n else None))
qs=stats.false_discovery_control([r['p'] if r['p'] is not None else 1 for r in results],method='by')
for r,q in zip(results,qs):r['q']=float(q) if r['p'] is not None else None
Path(__file__).with_suffix('.json').write_text(json.dumps(dict(subjects=subjects,records=records,groups=groups,alleles=alleles,recurrence=recurrences,associations=results),indent=2)+'\n')
