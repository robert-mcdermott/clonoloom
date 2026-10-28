"""Independent exact-integer aggregation oracle; stdlib only.
Run with uv run --no-project validation/reference/pseudobulk.py.
The fixture deliberately includes missing type, unknown size, absent groups,
measured zeros, low-cell groups, repeated barcodes and repeated subjects at visits.
"""
import json
from pathlib import Path

features=[dict(id=f"g{i}",name=f"Gene {i}",type="Gene Expression") for i in range(3)]
samples=[]
for s in range(4):
    cells=[]
    for i in range(15):
        typ="A" if i<10 else "B"
        category="1" if i%3==0 else ">1"
        reason=""
        if i==1: typ="";category="";reason="Missing cell-type annotation"
        if i==2: category="";reason="Unavailable unique clonotype size"
        counts=[0,0,0] if typ=="B" else [(i+1)*(s+1),0,(i*s)%4]
        cells.append(dict(barcode=f"C{i}",type=typ,bin=category,reason=reason,counts=[dict(feature=j,count=v) for j,v in enumerate(counts) if v]))
    # A type seen only in one sample exercises unavailable, not zero, libraries.
    if s==0: cells[-1]["type"]="C"
    samples.append(dict(parent=f"p{s}",expression=f"e{s}",annotation=dict(name=f"sample{s}",subject=f"subject{s//2}",timepoint=f"visit{s%2}"),cells=cells))
inp=dict(options=dict(annotation="type",minCells=3),features=features,bins=["1",">1"],samples=samples)
types=sorted({c['type'] for s in samples for c in s['cells'] if c['type']})
groups=[];coverage=[]
for si,sample in enumerate(samples):
    excluded={}
    for c in sample['cells']:
        if c['reason']: excluded[c['reason']]=excluded.get(c['reason'],0)+1
    coverage.append(dict(cells=len(sample['cells']),included=sum(not c['reason'] for c in sample['cells']),excluded=excluded))
    for typ in types:
        for category in inp['bins']:
            selected=[c for c in sample['cells'] if not c['reason'] and c['type']==typ and c['bin']==category]
            counts=[sum(v['count'] for c in selected for v in c['counts'] if v['feature']==j) for j in range(len(features))] if selected else None
            total=sum(counts) if counts is not None else None
            reason='No eligible cells' if not selected else 'Below minimum cells' if len(selected)<3 else 'Zero count library' if total==0 else ''
            groups.append(dict(sample=si,type=typ,bin=category,cells=len(selected),counts=counts,total=total,eligible=not reason,reason=reason))
result=dict(groups=groups,coverage=coverage,subjects=2,eligible=sum(g['eligible'] for g in groups))
Path(__file__).with_suffix('.json').write_text(json.dumps(dict(input=inp,result=result),indent=2)+'\n')
