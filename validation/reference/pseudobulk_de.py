# /// script
# requires-python = ">=3.12"
# dependencies = ["scipy==1.17.0", "statsmodels==0.14.6"]
# ///
"""Independent paired gene-count oracle: NumPy ratios, public statsmodels GLM,
SciPy likelihood optimization and BH. No ClonoLoom outputs/source are read.
Anders & Huber 2010; McCarthy et al. 2012. All genes and subjects are artificial.
"""
import json
import sys
from pathlib import Path
import numpy as np
from scipy import optimize, stats
import statsmodels.api as sm
controls="--controls" in sys.argv
genes=300 if controls else 40
rng=np.random.default_rng(20261105 if controls else 20261103)
n=56
group=np.tile([0,1],28)
x=np.column_stack([np.ones(n),group,np.eye(28)[np.repeat(np.arange(28),2),1:]])
exposure=rng.uniform(.6,1.8,n)
subject=rng.normal(0,.25,(genes,28))
fold=np.zeros(genes);changed=15 if controls else 2;fold[:changed]=np.log(3);fold[changed:2*changed]=-np.log(3)
mu=np.exp(np.log(80+np.arange(genes)*8)[:,None]+np.repeat(subject,2,axis=1)+fold[:,None]*group)*exposure
y=rng.negative_binomial(5,5/(5+mu))
y[-1]=0  # filtered zero gene, excluded only from normalization
y[-2]=1  # low-count gene still belongs to normalization
y[-3,0:2]=0 # eligible all-zero subject block: failure retained in BH
positive=np.all(y>0,axis=1)
gm=np.exp(np.log(y[positive]).mean(axis=1))
factors=np.median(y[positive]/gm[:,None],axis=0);factors/=np.exp(np.log(factors).mean())
offset=np.log(factors)
rows=[]
for j,counts in enumerate(y):
    row=dict(id=f'SIM-G{j+1:02}',eligible=int(np.sum(counts>=10))>=25,normalized=(counts/factors).tolist(),mean=float(np.mean(counts/factors)))
    if row['eligible'] and np.all(counts.reshape(28,2).sum(axis=1)>0):
        def fit(alpha,design=x):
            return sm.GLM(counts,design,family=sm.families.NegativeBinomial(alpha=alpha),offset=offset).fit(maxiter=200,tol=1e-11)
        def objective(z):
            a=np.exp(z);f=fit(a);w=f.mu/(1+a*f.mu)
            return -(f.llf-.5*np.linalg.slogdet(x.T@(w[:,None]*x))[1])
        opt=optimize.minimize_scalar(objective,bounds=(np.log(1e-5),np.log(100)),method='bounded',options={'xatol':1e-9})
        a=float(np.exp(opt.x));full=fit(a);null=fit(a,np.delete(x,1,axis=1));lr=max(0.,2*(full.llf-null.llf))
        effect=float(full.params[1]/np.log(2));se=float(np.sqrt(full.cov_params()[1,1])/np.log(2));half=stats.norm.ppf(.975)*se
        row.update(dispersion=a,log2Fold=effect,se=se,lower=effect-half,upper=effect+half,lr=lr,p=float(stats.chi2.sf(lr,1)),beta=full.params.tolist(),covariance=full.cov_params().tolist(),mu=full.mu.tolist(),logLikelihood=float(full.llf),nullLogLikelihood=float(null.llf))
    rows.append(row)
ps=[v.get('p',1.) for v in rows if v['eligible']]
qs=iter(stats.false_discovery_control(ps))
for row in rows:
    if row['eligible']:
        q=next(qs)
        if 'p' in row:row['adjusted']=float(q)
samples=[dict(name=f'S{i+1:02}-baseline',subject=f'S{i+1:02}',timepoint='baseline') for i in range(28)]
groups=[]
for k in range(n):
    counts=y[:,k].tolist()
    groups.append(dict(sample=k//2,type='T cells',bin=['1','>1'][k%2],cells=40+k%7,counts=counts,total=sum(counts),eligible=True,reason=''))
data=dict(method='NumPy median-of-ratios; statsmodels 0.14.6 GLM; scipy 1.17.0 optimization, chi2, norm and BH',input=dict(features=[dict(id=f'SIM-G{i+1:02}',name=f'Artificial gene {i+1:02}',type='Gene Expression') for i in range(genes)],bins=['1','>1'],samples=samples,minCells=10,groups=groups),options=dict(type='T cells',timepoint='baseline',reference='1',comparison='>1',exclude={},minCount=10,minSamples=25,confidence=.95),sizeFactors=factors.tolist(),offsets=offset.tolist(),normalizationGenes=[f'SIM-G{i+1:02}' for i in np.flatnonzero(positive)],x=x.tolist(),rows=rows)
if controls:
    data['generatingLog2Fold']=(fold/np.log(2)).tolist()
    valid=[(i,r) for i,r in enumerate(rows) if 'p' in r and fold[i]==0]
    data['diagnostics']=dict(nullFitted=len(valid),nullRejected=sum(r['p']<=.05 for i,r in valid),nullCovered=int(sum(r['lower']<=0<=r['upper'] for i,r in valid)),positiveFitted=sum('p' in rows[i] for i in range(2*changed)),positiveCorrectDirection=int(sum('p' in rows[i] and rows[i]['log2Fold']*fold[i]>0 for i in range(2*changed))))
    print(data['diagnostics'])
    for row in rows:
        for key in ['beta','mu','covariance','logLikelihood','nullLogLikelihood']:row.pop(key,None)
path=Path(__file__).with_name('pseudobulk_de_controls.json') if controls else Path(__file__).with_suffix('.json')
path.write_text(json.dumps(data,indent=2)+'\n')
