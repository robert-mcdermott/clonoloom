# Independent black-box network partitions; no reference implementation source is
# read, copied or linked. uv run --with scirpy==0.25.1 validation/reference/scirpy_networks.py
# Whole 10x runs and OTS paired units; primary chains, amino-acid junctions.
# V calls are normalized to genes for a gene-level definition. TCRdist uses
# searched gaps and cost 12 per length difference, matching tcrdist3 (not
# scirpy's default unweighted gap cost 4).
# OTS conversion below reads its published CSV fields independently of ClonoLoom.
import csv
import glob
import gzip
import hashlib
import json
import os
import sys
import warnings
from importlib.metadata import version

warnings.filterwarnings('ignore')
import scirpy as ir

here = os.path.dirname(os.path.abspath(__file__))
cache = os.environ.get('CLONOLOOM_DATA') or os.path.join(here, '..', 'cache')
output = os.path.join(here, 'networks.json')

def partition(adata):
    groups = {}
    for cell, clone in adata.obs['network'].items():
        if clone != clone or clone is None:
            continue
        groups.setdefault(str(clone), []).append(cell)
    lines = sorted(','.join(sorted(cells)) for cells in groups.values())
    return dict(cells=sum(map(len, groups.values())), components=len(groups), sha256=hashlib.sha256('\n'.join(lines).encode()).hexdigest())

def read_ots(path):
    cells = []
    with gzip.open(path, 'rt') as f:
        next(f)
        for line, row in enumerate(csv.DictReader(f), 3):
            ids = [row['sequence_id_' + arm] for arm in ('alpha', 'beta')]
            bars = [s.split('_contig')[0] if '_contig' in s else '' for s in ids]
            barcode = bars[0] if bars[0] and bars[0] == bars[1] else f'row {line}'
            cell = ir.io.AirrCell(barcode)
            for arm, locus in [('alpha', 'TRA'), ('beta', 'TRB')]:
                chain = ir.io.AirrCell.empty_chain_dict()
                for name in ('junction', 'junction_aa', 'v_call', 'd_call', 'j_call'):
                    value = row.get(name + '_' + arm) or None
                    if value and name.endswith('_call'):
                        value = '|'.join(sorted(set(g.strip().split('*')[0] for g in value.split(','))))
                    chain[name] = value
                chain.update(locus=locus, productive=row.get('productive_' + arm) == 'T', umi_count=1, consensus_count=1)
                cell.add_chain(chain)
            cells.append(cell)
    return ir.io.from_airr_cells(cells)

out = {'generated_by': f"scirpy {version('scirpy')}", 'files': {}}
paths = sorted(glob.glob(os.path.join(cache, '10x-*', '*filtered_contig_annotations.csv')) + glob.glob(os.path.join(cache, 'ots-healthy-pbmc', '*', 'csv_paired', '*.csv.gz')))
for path in paths:
    rel = os.path.relpath(path, cache)
    print(rel, file=sys.stderr, flush=True)
    adata = read_ots(path) if path.endswith('.gz') else ir.io.read_10x_vdj(path)
    ir.pp.index_chains(adata)
    ir.tl.chain_qc(adata)
    entries = {}
    for metric in ('identity', 'hamming', 'levenshtein', 'tcrdist'):
        if metric == 'tcrdist' and '_b_' in rel:
            continue
        for cutoff in ([0] if metric == 'identity' else [0, 12, 24, 48] if metric == 'tcrdist' else [0, 1, 2]):
            kw = {'fixed_gappos': False, 'gap_penalty': 12} if metric == 'tcrdist' else {}
            # TCRdist distances are multiples of 3. A cutoff of 1 keeps exactly
            # distance zero and avoids scirpy's cutoff=0 identity shortcut.
            effective_cutoff = max(1, cutoff) if metric == 'tcrdist' else cutoff
            ir.pp.ir_dist(adata, metric=metric, sequence='aa', cutoff=effective_cutoff, n_jobs=1, key_added='network_dist', **kw)
            for arms in ('all', 'any'):
                for same_v in (False, True):
                    ir.tl.define_clonotype_clusters(adata, sequence='aa', metric=metric, receptor_arms=arms, dual_ir='primary_only', same_v_gene=same_v, key_added='network', distance_key='network_dist', n_jobs=1)
                    entries[f'{metric}/{cutoff}/{arms}/{int(same_v)}'] = partition(adata)
    out['files'][rel] = entries

# Never replace the reference with a partial run after an interrupted calculation.
with open(output + '.tmp', 'w') as f:
    json.dump(out, f, indent=1, sort_keys=True)
os.replace(output + '.tmp', output)
