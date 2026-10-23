# TCRdist as tcrdist3 computes it, for validation/tcrdist_test.go: every pairwise distance between
# the clones of Dash et al. 2017's epitope-specific TCRs (tcrdist3's dash.csv, mouse, and
# dash_human.csv), each clone one row, in the file's order (tcrdist3 not deduplicating them), by
# tcrdist3's defaults: the α and β chains' distances, and each loop's (CDR1, CDR2, CDR2.5 and CDR3)
# alone, with the loops tcrdist3 takes from its database of genes (combo_xcr_2024-03-05.tsv).
# Each matrix is reduced to a digest (each row's distances joined by ",", the rows by "\n",
# SHA-256), with its first row kept whole; the loops tcrdist3 inferred for each clone are reduced
# the same way (validation/reference/tcrdist3.json).
#
# tcrdist3 needs parasail, whose wheels are built for x86_64 only on macOS, and numba:
#
#   uv run --python cpython-3.11-macos-x86_64 --with tcrdist3 --with numba==0.60.0 \
#     --with llvmlite==0.43.0 --with 'numpy<2.1' python -I validation/reference/tcrdist3_distances.py
import hashlib, json, os, sys, warnings
import importlib.metadata as metadata

warnings.filterwarnings('ignore')
import pandas as pd
import tcrdist
from tcrdist.repertoire import TCRrep

here = os.path.dirname(os.path.abspath(__file__))
cache = os.environ.get('CLONOLOOM_DATA') or os.path.join(here, '..', 'cache')

def digest(rows):
    return hashlib.sha256('\n'.join(rows).encode()).hexdigest()

def matrix(m):
    rows = [','.join(str(int(v)) for v in row) for row in m]
    return {'digest': digest(rows), 'first': [int(v) for v in m[0]]}

db = os.path.join(os.path.dirname(tcrdist.__file__), 'db', 'combo_xcr_2024-03-05.tsv')
out = {
    'made': f"tcrdist3 {metadata.version('tcrdist3')}, pwseqdist {metadata.version('pwseqdist')}, numpy {metadata.version('numpy')}, pandas {metadata.version('pandas')}",
    'loops': {'file': 'combo_xcr_2024-03-05.tsv', 'sha256': hashlib.sha256(open(db, 'rb').read()).hexdigest()},
    'files': {},
}
for name, organism in (('dash.csv', 'mouse'), ('dash_human.csv', 'human')):
    path = os.path.join(cache, 'dash-2017-tcrdist3', name)
    df = pd.read_csv(path)
    cols = ['count', 'v_a_gene', 'j_a_gene', 'cdr3_a_aa', 'v_b_gene', 'j_b_gene', 'cdr3_b_aa']
    tr = TCRrep(cell_df=df[cols].copy(), organism=organism, chains=['alpha', 'beta'], deduplicate=False, store_all_cdr=True)
    c = tr.clone_df
    loops = [f"{r.v_a_gene}\t{r.cdr1_a_aa}\t{r.cdr2_a_aa}\t{r.pmhc_a_aa}\t{r.v_b_gene}\t{r.cdr1_b_aa}\t{r.cdr2_b_aa}\t{r.pmhc_b_aa}" for r in c.itertuples()]
    entry = {
        'organism': organism, 'clones': int(c.shape[0]), 'loops': digest(loops),
        'alpha': matrix(tr.pw_alpha), 'beta': matrix(tr.pw_beta), 'paired': matrix(tr.pw_alpha + tr.pw_beta),
    }
    for chain in ('a', 'b'):
        for loop in ('cdr1', 'cdr2', 'pmhc', 'cdr3'):
            entry[f'{loop}_{chain}'] = matrix(getattr(tr, f'pw_{loop}_{chain}_aa'))
    out['files'][name] = entry
    print(name, entry['clones'], 'clones', file=sys.stderr)
json.dump(out, open(os.path.join(here, 'tcrdist3.json'), 'w'), indent=1, sort_keys=True)
