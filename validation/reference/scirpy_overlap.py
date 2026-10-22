# Overlap between four single-cell T-cell runs of the development data (10x Genomics: human PBMC
# 1k and 10k, mouse splenocytes 1k and 5k), as scirpy computes it, for validation/overlap_test.go:
# each run's filtered contigs read by read_10x_vdj, the four concatenated (one sample each), chains
# indexed, clonotypes grouped by amino-acid junction identity (primary chains) of both arms (a
# missing arm matching a missing arm, as validation/reference/scirpy_clonotypes.py) and of the beta
# arm alone, and repertoire_overlap's Jaccard distance between the samples' sets of clonotypes;
# with the clonotypes each pair shares (validation/reference/overlap_sc.json).
#
#   uv run --with scirpy python -I validation/reference/scirpy_overlap.py
import glob, json, os, sys, warnings
warnings.filterwarnings('ignore')
import anndata as ad
import scirpy as ir
from importlib.metadata import version

here = os.path.dirname(os.path.abspath(__file__))
cache = os.environ.get('CLONOLOOM_DATA') or os.path.join(here, '..', 'cache')

paths = sorted(p for p in glob.glob(os.path.join(cache, '10x-*', '*filtered_contig_annotations.csv'))
               if '_t_' in os.path.basename(p) or 'vdj_t_' in os.path.basename(p))
names = [os.path.relpath(p, cache) for p in paths]
print('\n'.join(names), file=sys.stderr)
adatas = [ir.io.read_10x_vdj(p) for p in paths]
adata = ad.concat(adatas, label='sample', keys=names, index_unique='-')
ir.pp.index_chains(adata)
ir.tl.chain_qc(adata)
ir.pp.ir_dist(adata, metric='identity', sequence='aa')
out = {}
for name, arms in (('paired', 'all'), ('beta', 'VDJ')):
    ir.tl.define_clonotype_clusters(adata, sequence='aa', metric='identity', receptor_arms=arms, dual_ir='primary_only', same_v_gene=False, key_added='clone_id')
    weighted, distances, _ = ir.tl.repertoire_overlap(adata, 'sample', target_col='clone_id', overlap_measure='jaccard', inplace=False)
    present = weighted > 0
    samples = list(weighted.index)
    pairs, k = [], 0
    for i in range(len(samples)):
        for j in range(i + 1, len(samples)):
            pairs.append({'a': samples[i], 'b': samples[j], 'jaccard': 1 - float(distances[k]),
                          'shared': int((present.iloc[i] & present.iloc[j]).sum()),
                          'clonotypes': [int(present.iloc[i].sum()), int(present.iloc[j].sum())]})
            k += 1
    out[name] = pairs
json.dump({'generated_by': f"scirpy {version('scirpy')}", 'definitions': out}, open(os.path.join(here, 'overlap_sc.json'), 'w'), indent=1)
