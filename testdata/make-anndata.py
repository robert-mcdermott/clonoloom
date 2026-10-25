# /// script
# requires-python = ">=3.11"
# dependencies = ["anndata==0.12.6", "mudata==0.3.2", "awkward==2.8.12", "h5py==3.15.1", "numpy==2.4.2", "scipy==1.17.0", "pandas==2.3.3", "scirpy==0.25.1"]
# ///
"""Artificial AnnData/MuData fixtures through public writer APIs, never library source.
uv run --no-project testdata/make-anndata.py
AnnData reads each written source back; SciPy/NumPy provide independent dense/count oracles.
"""
from pathlib import Path
from importlib.metadata import version
import os, tempfile
os.environ.setdefault("MPLCONFIGDIR", str(Path(tempfile.gettempdir()) / "clonoloom-fixture-matplotlib"))
import hashlib, json, shutil, warnings
import anndata as ad, mudata as md, awkward as ak, h5py, numpy as np, pandas as pd, scirpy as ir
from scipy import sparse
warnings.filterwarnings('ignore', category=FutureWarning)
root = Path(__file__).resolve().parent
out = root / 'anndata'
out.mkdir(exist_ok=True)
counts = np.array([[2, 3, 0], [0, 4, 2], [7, 0, 0], [0, 0, 0]], dtype='int32')
obs = pd.DataFrame({'state': pd.Categorical(['A', None, 'B', 'Missing annotation']), 'age': pd.array([1, None, 3, 4], dtype='Int64'), 'included': [True, False, True, True], 'score': [0.5, np.nan, 1.5, 0]}, index=['CELL1', 'CELL2', 'CELL3', 'CELL4'])
var = pd.DataFrame({'gene_name': ['Shared', 'Shared', 'Antibody'], 'feature_type': ['Gene Expression', 'Gene Expression', 'Antibody Capture']}, index=['g1', 'g2', 'ab1'])
chains = [[{'locus': 'TRA', 'junction_aa': 'CASSF', 'productive': True, 'umi_count': 2}, {'locus': 'TRB', 'junction_aa': 'CASRF', 'productive': None, 'umi_count': 1}], [], [{'locus': 'TRB', 'junction_aa': None, 'productive': False, 'umi_count': None}], []]
# Include the required AIRR columns using scirpy's public schema helper. Keep our first
# four fields in their source order and leave unspecified sequence/call evidence missing.
required = ir.io.AirrCell.empty_chain_dict()
chains = [[dict(chain, **{k:v for k,v in required.items() if k not in chain}) for chain in cell] for cell in chains]
base = ad.AnnData(counts, obs=obs, var=var)
base.obsm['X_umap'] = np.array([[.1, 1.2], [3.4, 4.5], [6.7, 7.8], [np.nan, np.nan]])
base.obsm['X_tsne'] = np.array([[1,2],[3,4],[5,6],[7,8]], dtype='float32')
base.obsm['airr'] = ak.Array(chains)
base.layers['counts'] = sparse.csr_matrix(counts)
base.layers['normalized'] = counts.astype('float64') / 3
valid={}
selection={'layer':'layers/counts','embedding':'X_umap','airrKey':'airr','featureName':'gene_name','featureType':'feature_type'}
for name, matrix, compression in [('dense',counts,None),('csr',sparse.csr_matrix(counts),None),('csc',sparse.csc_matrix(counts),None),('gzip-dense',counts,'gzip'),('gzip-csr',sparse.csr_matrix(counts),'gzip'),('float-counts',counts.astype('float64'),None),('big-endian',counts.astype('>i4'),None)]:
    a=base.copy(); a.X=matrix
    path=out / (name+'.h5ad'); a.write_h5ad(path,compression=compression)
    valid[path.name]=dict(selection,layer='X')
air=ad.AnnData(obs=pd.DataFrame(index=['CELL3','AIRR_ONLY','CELL1']))
air.obsm['airr']=ak.Array([chains[2],[],chains[0]])
md.set_options(pull_on_update=False)
m=md.MuData({'gex':base,'receptors':air});m.write_h5mu(out/'multimodal.h5mu')
valid['multimodal.h5mu']=dict(selection,modality='gex',airrModality='receptors')
# Missing whole-cell AIRR records are a distinct option layout, not a zero-chain inference.
a=base.copy();a.obsm['airr']=base.obsm['airr'][[0,None,2,3]];a.write_h5ad(out/'optional-cell.h5ad');valid['optional-cell.h5ad']=selection
# Zero dimensions are valid evidence, including no detected genes.
a=base[:0].copy();a.write_h5ad(out/'empty.h5ad');valid['empty.h5ad']=selection

# Partial edge chunks, unsigned counts, nullable booleans, exact large integer labels.
a=base.copy();a.obs['flag']=pd.array([True,None,False,True],dtype='boolean');a.obs['large']=np.array([2**63-1,-2**63,0,1],dtype='int64');a.write_h5ad(out/'edge-chunks.h5ad')
with h5py.File(out/'edge-chunks.h5ad','r+') as f:
    del f['X'];d=f.create_dataset('X',data=counts.astype('uint64'),chunks=(3,2),compression='gzip',shuffle=True,fletcher32=True);d.attrs['encoding-type']='array';d.attrs['encoding-version']='0.2.0'
valid['edge-chunks.h5ad']=dict(selection,layer='X')

# Scirpy's own public conversion adds its canonical field ordering and nullable layouts.
a=base.copy(); cells=ir.io.to_airr_cells(a)
assert len(cells)==4 and sum(len(c.chains) for c in cells)==3
converted=ir.io.from_airr_cells(cells)
a.obsm['airr']=converted.obsm['airr'];a.write_h5ad(out/'scirpy.h5ad')
valid['scirpy.h5ad']=selection
ad.settings.allow_write_nullable_strings=True
a=base.copy();a.obs['note']=pd.array(['red',None,'NA',''],dtype='string');a.write_h5ad(out/'nullable-strings.h5ad',convert_strings_to_categoricals=False)
valid['nullable-strings.h5ad']=selection
invalid={}
def bad(name,mutate,reason,sel=None):
    path=out/(name+'.h5ad');shutil.copyfile(out/'dense.h5ad',path)
    with h5py.File(path,'r+') as f: mutate(f)
    invalid[path.name]={'selection':sel or dict(selection,layer='X'),'error':reason}
bad('fractional',lambda f:f['X'].__setitem__((0,0),-1),'counts')
bad('duplicate-barcode',lambda f:f['obs/_index'].__setitem__(1,'CELL1'),'duplicate index')
bad('bad-category',lambda f:f['obs/state/codes'].__setitem__(0,9),'code outside')
bad('half-missing-map',lambda f:f['obsm/X_umap'].__setitem__((0,0),np.nan),'complete finite')
bad('bad-airr-offset',lambda f:f['obsm/airr/node0-offsets'].__setitem__(1,999),'outside content')
bad('encoding-version',lambda f:f['X'].attrs.__setitem__('encoding-version','99'),'unsupported AnnData')
bad('normalized',lambda f:None,'counts',dict(selection,layer='layers/normalized'))
bad('no-selection',lambda f:None,'select a count matrix',{'layer':''})
def badptr(f):
    f['layers/counts/indptr'][2]=0
bad('bad-indptr',badptr,'monotone',selection)
def duplicate(f):
    f['layers/counts/indices'][1]=f['layers/counts/indices'][0]
bad('duplicate-coordinate',duplicate,'duplicate count',selection)
def badshape(f): f['layers/counts'].attrs['shape']=np.array([4,2])
bad('bad-shape',badshape,'shape differs',selection)

def replace_x(f,values):
    del f['X'];d=f.create_dataset('X',data=values);d.attrs['encoding-type']='array';d.attrs['encoding-version']='0.2.0'
for name,value in [('nan-count',np.nan),('infinite-count',np.inf),('fraction-count',1.25),('unsafe-count',float(2**53))]:
    values=counts.astype('float64');values[0,0]=value
    bad(name,lambda f,v=values:replace_x(f,v),'counts')
bad('boolean-counts',lambda f:replace_x(f,counts.astype('bool')),'numeric array')
bad('negative-index',lambda f:f['layers/counts/indices'].__setitem__(0,-1),'negative',selection)
def unknown_form(f):
    form=json.loads(f['obsm/airr'].attrs['form']);form['class']='UnionArray';f['obsm/airr'].attrs['form']=json.dumps(form)
bad('unknown-airr-form',unknown_form,'unsupported Awkward form')
def mismatch_form(f):
    form=json.loads(f['obsm/airr'].attrs['form']);form['content']['contents'][0]['content']['primitive']='uint32';f['obsm/airr'].attrs['form']=json.dumps(form)
bad('airr-type-mismatch',mismatch_form,'character form')
def badnullable(f):
    del f['obs/age/mask'];f['obs/age'].create_dataset('mask',data=np.array([0,1,0,0],dtype='int8'))
bad('integer-mask',badnullable,'mask must be boolean')
bad('unknown-category-code',lambda f:f['obs/state/codes'].__setitem__(0,-2),'code outside')

reference={'generated_by':{x:version(x) for x in ['anndata','mudata','awkward','h5py','numpy','pandas','scipy','scirpy']},'valid':{},'invalid':invalid}
def text_value(value):
    if pd.isna(value): return ''
    if isinstance(value,(bool,np.bool_)): return 'true' if value else 'false'
    if isinstance(value,(float,np.floating)): return format(value,'.15g')
    return str(value)
for name,sel in valid.items():
    path=out/name
    obj=md.read_h5mu(path) if path.suffix=='.h5mu' else ad.read_h5ad(path)
    a=obj.mod[sel['modality']] if 'modality' in sel else obj
    x=a.X if sel['layer']=='X' else a.layers[sel['layer'].split('/',1)[1]]
    dense=x.toarray() if sparse.issparse(x) else x
    air=obj.mod[sel['airrModality']] if 'airrModality' in sel else a
    reference['valid'][name]={'selection':sel,'sha256':hashlib.sha256(path.read_bytes()).hexdigest(),'barcodes':a.obs_names.tolist(),'features':a.var_names.tolist(),'dense':dense.astype('int64').tolist(),'gene_umis':dense[:,:2].sum(axis=1).astype('int64').tolist(),'genes':(dense[:,:2]>0).sum(axis=1).tolist(),'airr_barcodes':air.obs_names.tolist(),'airr':ak.to_list(air.obsm['airr']),'obs_columns':list(a.obs.columns),'obs_values':[[text_value(v) for v in row] for row in a.obs.itertuples(index=False,name=None)],'var_columns':list(a.var.columns),'var_values':[[text_value(v) for v in row] for row in a.var.itertuples(index=False,name=None)],'positions':[None if np.isnan(row).all() else row.tolist() for row in a.obsm[sel['embedding']]],'entries':int(x.nnz if sparse.issparse(x) else x.size)}
(root.parent/'validation/reference/anndata.json').write_text(json.dumps(reference,indent=2,allow_nan=False)+'\n')
print('Wrote',len(valid),'valid and',len(invalid),'invalid files')
