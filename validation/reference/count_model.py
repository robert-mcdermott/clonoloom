# /// script
# requires-python = ">=3.12"
# dependencies = ["scipy==1.17.0", "statsmodels==0.14.6"]
# ///
"""Independent public statsmodels NB fits and SciPy Cox-Reid optimization.
No ClonoLoom outputs or third-party implementation sources are read.
"""
import json
from pathlib import Path
import numpy as np
from scipy import optimize, stats
import statsmodels.api as sm

rng=np.random.default_rng(20261026)
cases=[]
for name,paired,batch in [('independent',False,False),('independent-batch',False,True),('paired',True,False),('paired-batch',True,True),('unbalanced',False,False),('unbalanced-batch',False,True)]:
    n=56 if paired else 32
    first=12 if name.startswith('unbalanced') else n//2
    group=np.array([0,1]*(n//2)) if paired else np.array([0]*first+[1]*(n-first))
    X=np.column_stack([np.ones(n),group])
    if paired:
        X=np.column_stack([X,np.eye(n//2)[np.repeat(np.arange(n//2),2),1:]])
    if batch:
        X=np.column_stack([X, np.array([0,1,1,0]*(n//4))])
    offset=np.log(rng.uniform(40000,150000,n))
    beta=np.zeros(X.shape[1]);beta[0]=-7;beta[1]=.65
    if paired:beta[2:1+n//2]=rng.normal(0,.3,n//2-1)
    if batch:beta[-1]=.3
    mu=np.exp(offset+X@beta)
    y=rng.negative_binomial(5,5/(5+mu))
    def fit(alpha,x=X):
        return sm.GLM(y,x,family=sm.families.NegativeBinomial(alpha=alpha),offset=offset).fit(maxiter=200,tol=1e-11)
    def objective(z):
        f=fit(np.exp(z)); w=f.mu/(1+np.exp(z)*f.mu)
        return -(f.llf-.5*np.linalg.slogdet(X.T@(w[:,None]*X))[1])
    opt=optimize.minimize_scalar(objective,bounds=(np.log(1e-5),np.log(100)),method='bounded',options={'xatol':1e-9})
    alpha=float(np.exp(opt.x));f=fit(alpha);null=fit(alpha,np.delete(X,1,axis=1));lr=2*(f.llf-null.llf)
    cases.append(dict(name=name,x=X.tolist(),y=y.tolist(),offset=offset.tolist(),alpha=alpha,beta=f.params.tolist(),mu=f.mu.tolist(),covariance=f.cov_params().tolist(),logLikelihood=float(f.llf),nullLogLikelihood=float(null.llf),lr=float(lr),p=float(stats.chi2.sf(lr,1)),fixed=dict(alpha=.2,beta=fit(.2).params.tolist(),logLikelihood=float(fit(.2).llf))))
Path(__file__).with_suffix('.json').write_text(json.dumps(dict(method='statsmodels 0.14.6 public GLM and scipy 1.17.0 minimize_scalar',cases=cases),indent=2)+'\n')
