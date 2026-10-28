"""Independent NIST modified-z oracle: Fraction medians/MADs and stdlib logs.
Run with uv run --no-project validation/reference/batch_qc.py.
No application implementation is imported.
"""
from fractions import Fraction as F
from statistics import median
import json, math
from pathlib import Path
samples=[]
for batch, values in [('A',[70,72,74,76,78,98]),('B',[80]*5),('C',[40,60,90])]:
    for i,value in enumerate(values):
        raw=[10**(i+1),100+i*10,value/100,(100-value)/100,.9,.95]
        samples.append(dict(assignment=dict(name=f'{batch}{i}',subject=f'D{i}',batch=batch,exclude=''),rows=100+i*10,kept=100+i*10,removed={},values=[dict(value=x,numerator=x,denominator=1,reason='') for x in raw]))
samples.append(dict(assignment=dict(name='excluded',subject='',batch='',exclude='Not comparable'),rows=0,kept=0,removed={},values=[dict(value=None,numerator=None,denominator=None,reason='No evidence') for _ in range(6)]))
# Missing values do not silently become zero or join the reference.
samples[0]['values'][5]=dict(value=None,numerator=None,denominator=None,reason='Missing evidence')
# A true zero is retained, but is not transformed with a pseudocount.
samples[0]['values'][0]=dict(value=0,numerator=0,denominator=1,reason='')
metrics=['abundance','rows','productive','unknown','vResolved','jResolved']
expected=[]
for batch in ['A','B','C']:
 for mi,metric in enumerate(metrics):
    entries=[]
    for si,s in enumerate(samples):
        if s['assignment']['batch']!=batch:continue
        raw=s['values'][mi]['value'];reason=s['values'][mi]['reason'];v=None
        if raw is not None:
            if mi<2:
                if raw==0:reason='Zero has no log10 value; no pseudocount'
                else:v=F(str(math.log10(raw)))
            else:v=F(str(raw))
        entries.append(dict(sample=si,value=raw,transformed=v,reason=reason))
    values=[e['transformed'] for e in entries if e['transformed'] is not None]
    med=median(values) if values else None
    mad=median([abs(v-med) for v in values]) if values else None
    reason='Too few evaluable subjects for the declared minimum' if len(values)<5 else 'Zero MAD: reference spread is undefined' if mad==0 else ''
    scored=flagged=0
    for e in entries:
        e['z']=None;e['flag']=False
        if not e['reason']:
            e['reason']=reason
            if not reason:
                e['z']=F(6745,10000)*(e['transformed']-med)/mad
                e['flag']=abs(e['z'])>F(7,2);scored+=1;flagged+=e['flag']
    expected.append(dict(batch=batch,metric=metric,valid=len(values),median=med,mad=mad,reason=reason,scored=scored,flagged=flagged,observations=entries))
def convert(x):
    if isinstance(x,F):return float(x)
    raise TypeError(x)
Path(__file__).with_suffix('.json').write_text(json.dumps(dict(input=dict(options=dict(threshold=3.5,minSamples=5),unit='umis',singleCell=False,samples=samples),groups=expected),default=convert,indent=2)+'\n')
