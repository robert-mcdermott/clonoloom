# Neighborhoods as tcrdist3 computes them, for validation/neighborhood_test.go: tcrdist3 0.3
# (MIT, run here as a tool, never copied), on Dash et al. 2017's human epitope-specific β chains
# against a background of the OTS donors' β chains, each centroid's radius controlled at the rate
# given, and its enrichment against the background.
#
# The background's weights are made up here (1, 1.25, …, 2, repeating), not drawn from a model:
# what is compared is the radius each weight gives and the statistics computed from it, not how a
# background is built. ClonoLoom's own backgrounds are checked against the simulated vaccine
# study's truth instead.
#
# Writes validation/reference/tcrdist3_neighborhoods.json: the target and background rows used (so
# that ClonoLoom compares the same receptors), and each centroid's radius, hits and statistics.
#
# tcrdist3 needs parasail, whose wheels are built for x86_64 only on macOS, and numba:
#
#   uv run --python cpython-3.11-macos-x86_64 --with tcrdist3 --with numba==0.60.0 \
#     --with llvmlite==0.43.0 --with 'numpy<2.1' python -I validation/reference/tcrdist3_neighborhoods.py
import csv, gzip, hashlib, json, os, sys, warnings
import importlib.metadata as metadata

warnings.filterwarnings('ignore')
import numpy as np
import pandas as pd
import tcrdist
from tcrdist.repertoire import TCRrep
from tcrdist.neighbors import bkgd_cntl_nn2

here = os.path.dirname(os.path.abspath(__file__))
cache = os.environ.get('CLONOLOOM_DATA') or os.path.join(here, '..', 'cache')

CONTROL = 1e-5
THRESHOLDS = [x for x in range(0, 50, 2)]
BACKGROUND = 20000

AA = set('ACDEFGHIKLMNPQRSTVWY')
good = lambda s: isinstance(s, str) and s and set(s) <= AA

# The target: Dash et al.'s human β chains, one row each.
rows = []
with open(os.path.join(cache, 'dash-2017-tcrdist3', 'dash_human.csv')) as f:
    for r in csv.DictReader(f):
        if good(r['cdr3_b_aa']):
            rows.append({'cdr3_b_aa': r['cdr3_b_aa'], 'v_b_gene': r['v_b_gene'], 'j_b_gene': r['j_b_gene'], 'epitope': r['epitope'], 'count': 1})
target_df = pd.DataFrame(rows)

# The background: the OTS donors' β chains, in the file's order, to BACKGROUND rows.
def unit(path):
    with gzip.open(os.path.join(cache, path), 'rt', newline='') as f:
        f.readline()
        return list(csv.DictReader(f))

bg_rows = []
for u in ['Sureshchandra_2023/csv_paired/SRR18365371_1_Paired_All.csv.gz',
          'Bai_2023/csv_paired/SRR14045264_1_Paired_All.csv.gz',
          'Poon_2023/csv_paired/SRR19760479_1_Paired_All.csv.gz']:
    for r in unit('ots-healthy-pbmc/' + u):
        v, j = r['v_call_beta'].split('*')[0], r['j_call_beta'].split('*')[0]
        if len(bg_rows) < BACKGROUND and good(r['junction_aa_beta']) and v.startswith('TRBV') and j.startswith('TRBJ') and ',' not in v + j:
            bg_rows.append({'cdr3_b_aa': r['junction_aa_beta'], 'v_b_gene': v + '*01', 'j_b_gene': j + '*01', 'count': 1})
bg_df = pd.DataFrame(bg_rows)
bg_df['weights'] = [1 + 0.25 * (i % 5) for i in range(bg_df.shape[0])]

tr = TCRrep(cell_df=target_df.copy(), organism='human', chains=['beta'], deduplicate=False, compute_distances=True)
tr_bkgd = TCRrep(cell_df=bg_df.copy(), organism='human', chains=['beta'], deduplicate=False, compute_distances=False)
# The rows tcrdist3 actually used, so that ClonoLoom compares the same receptors.
used_target = tr.clone_df[['cdr3_b_aa', 'v_b_gene', 'j_b_gene']].to_dict('records')
used_bkgd = tr_bkgd.clone_df[['cdr3_b_aa', 'v_b_gene', 'j_b_gene']].to_dict('records')
weights = list(tr_bkgd.clone_df['weights'])

tr.compute_rect_distances(df=tr.clone_df, df2=tr_bkgd.clone_df, store=False)
centers = bkgd_cntl_nn2(tr=tr, tr_background=tr_bkgd, weights=np.array(weights), ctrl_bkgd=CONTROL,
                        col='cdr3_b_aa', thresholds=THRESHOLDS, include_seq_info=False,
                        generate_regex=False, test_regex=False)

out = []
for i, r in centers.iterrows():
    out.append({
        'cdr3': r['cdr3_b_aa'], 'v': r['v_b_gene'], 'j': r['j_b_gene'],
        'radius': int(r['radius']),
        'targetHits': int(r['target_hits']),
        'backgroundHits': int(r['bkgd_hits']),
        'backgroundWeighted': float(r['bkgd_hits_weighted']),
        'rate': float(r['TR']),
        'backgroundRate': float(r['BR_weighted']),
        'relativeRate': float(r['RR_weighted']),
        'oddsRatio': float(r['OR_weighted']),
        'chiSquare': float(r['chi2dist']),
    })

db = os.path.join(os.path.dirname(tcrdist.__file__), 'db', 'combo_xcr_2024-03-05.tsv')
json.dump({
    'made': f"tcrdist3 {metadata.version('tcrdist3')}, pwseqdist {metadata.version('pwseqdist')}, numpy {metadata.version('numpy')}, pandas {metadata.version('pandas')}",
    'loops': {'file': 'combo_xcr_2024-03-05.tsv', 'sha256': hashlib.sha256(open(db, 'rb').read()).hexdigest()},
    'control': CONTROL, 'thresholds': THRESHOLDS,
    'target': used_target, 'background': used_bkgd, 'weights': weights,
    'centers': out,
}, open(os.path.join(here, 'tcrdist3_neighborhoods.json'), 'w'), indent=1)
print(len(out), 'centroids,', len(used_bkgd), 'background receptors', file=sys.stderr)
