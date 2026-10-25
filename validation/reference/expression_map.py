# /// script
# requires-python = ">=3.11"
# dependencies = ["numpy==2.3.5", "scipy==1.17.0", "scikit-learn==1.8.0", "umap-learn==0.5.11", "numba==0.63.1"]
# ///
"""Independent expression-map checks; reference tools are executed, never copied or bundled.
Run with uv run validation/reference/expression_map.py.
PCA: scipy.linalg.eigh; neighbors: scipy.spatial.distance.cdist; graph and curve:
umap-learn public numerical functions; layout quality: sklearn.manifold.trustworthiness.
Methods: Wolf et al. 2018 doi:10.1186/s13059-017-1382-0;
Jolliffe & Cadima 2016 doi:10.1098/rsta.2015.0202; McInnes et al. arXiv:1802.03426.
Feature variance ranking and centered/unscaled PCA are ClonoLoom's explicit choices.
"""
from pathlib import Path
import os
# Disable Numba JIT: in this environment its fast-math compilation produced infinite
# smooth-kNN bandwidths. Execute the same reference functions without compilation.
os.environ["NUMBA_DISABLE_JIT"] = "1"
import hashlib
import json
import importlib.metadata
import numpy as np
from scipy.linalg import eigh
from scipy.spatial.distance import cdist
from sklearn.manifold import trustworthiness
from umap import UMAP
from umap.umap_ import fuzzy_simplicial_set, find_ab_params

root = Path(__file__).resolve().parents[2]
folder = root / 'testdata/expression-map'
folder.mkdir(exist_ok=True)
rng = np.random.default_rng(731)
profiles = np.array([[70, 50, 30, 2, 1, 3, 2, 6], [2, 3, 2, 70, 50, 30, 2, 6], [3, 2, 3, 2, 3, 2, 70, 60]])
truth = np.repeat(np.arange(3), [40, 32, 24])
counts = rng.poisson(profiles[truth]).astype(np.int64)
counts = np.vstack([counts, np.zeros(8, dtype=np.int64), [1,0,0,0,0,0,0,0]])
# Antibody capture must not affect normalization or feature selection.
counts = np.column_stack([counts, np.full(len(counts), 900000)])
ids = ['G8', 'G2', 'G3', 'G4', 'G1', 'G6', 'G7', 'G5', 'ADT']
barcodes = [f'CELL{i:04d}' for i in range(len(counts))]
(folder/'features.tsv').write_text(''.join(f'{g}\tShared\t{"Antibody Capture" if g=="ADT" else "Gene Expression"}\n' for g in ids))
(folder/'barcodes.tsv').write_text('\n'.join(barcodes)+'\n')
entries = [(g+1,c+1,int(v)) for c,row in enumerate(counts) for g,v in enumerate(row) if v]
# Include an explicit sparse zero: it is not a detected gene.
entries.append((2,98,0))
(folder/'matrix.mtx').write_text('%%MatrixMarket matrix coordinate integer general\n% Artificial expression analysis reference\n'+f'{counts.shape[1]} {counts.shape[0]} {len(entries)}\n'+''.join(f'{g} {c} {v}\n' for g,c,v in entries))
(folder/'cells.tsv').write_text('barcode\tstate\tmap_x\tmap_y\n'+''.join(f'{barcode}\t{("ABC"[truth[i]] if i<len(truth) and i!=4 else "")}\t{i}\t{-i}\n' for i,barcode in enumerate(barcodes)))

cases = []
for features,components,neighbors in [(6,4,7),(8,30,15)]:
    raw = counts[:96,:8].astype(np.float64)
    transformed = np.log1p(raw / (raw.sum(axis=1)/10000)[:,None])
    means = transformed.mean(axis=0)
    variance = transformed.var(axis=0,ddof=1)
    selected = sorted(sorted(range(8),key=lambda i:(-variance[i],ids[i]))[:features],key=lambda i:ids[i])
    x = transformed[:,selected]-means[selected]
    vals, vecs = eigh(x.T@x/(len(x)-1))
    order = np.argsort(-vals,kind='stable'); vals,vecs=vals[order],vecs[:,order]
    k=min(components,int((vals>vals[0]*1e-12).sum()))
    vals,vecs=vals[:k],vecs[:,:k]
    for i in range(k):
        if vecs[np.argmax(np.abs(vecs[:,i])),i]<0: vecs[:,i]*=-1
    scores=x@vecs
    dist=cdist(scores,scores)
    np.fill_diagonal(dist,np.inf)
    indices=np.argsort(dist,axis=1,kind='stable')[:,:neighbors]
    distances=np.take_along_axis(dist,indices,axis=1)
    with_self=np.column_stack([np.arange(len(x)),indices]).astype(np.int32)
    with_zero=np.column_stack([np.zeros(len(x)),distances]).astype(np.float32)
    graph,sigmas,rhos=fuzzy_simplicial_set(scores,neighbors+1,np.random.RandomState(0),'euclidean',knn_indices=with_self,knn_dists=with_zero)
    assert np.isfinite(sigmas).all() and np.isfinite(rhos).all()
    model=UMAP(n_neighbors=neighbors+1,init=scores[:,:2].astype(np.float32),min_dist=.5,spread=1,n_epochs=500,random_state=20261024,n_jobs=1)
    layout=model.fit_transform(scores)
    cases.append(dict(options=dict(minGenes=2,minCells=2,features=features,components=components,neighbors=neighbors),selected=[ids[i] for i in selected],means=means[selected].tolist(),variances=variance[selected].tolist(),eigenvalues=vals.tolist(),loadings=vecs.tolist(),scores=scores.tolist(),indices=indices.tolist(),distances=distances.tolist(),graph=graph.toarray().tolist(),referenceTrustworthiness=float(trustworthiness(scores,layout,n_neighbors=7))))
a,b=find_ab_params(1,.5)
out=dict(numbaJIT=False,versions={p:importlib.metadata.version(p) for p in ['numpy','scipy','scikit-learn','umap-learn','numba']},sources={p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in sorted(folder.iterdir())},truth=truth.tolist(),curve=[a,b],cases=cases)
(root/'validation/reference/expression_map.json').write_text(json.dumps(out,indent=2)+'\n')
print({k:out[k] for k in ['versions','curve']})
print('Reference trustworthiness:', [c['referenceTrustworthiness'] for c in cases])
