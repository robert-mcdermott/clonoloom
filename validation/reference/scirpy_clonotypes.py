# Single-cell clonotypes of the development files as scirpy defines them, for
# validation/clonotypes_test.go: each Cell Ranger contig file read by scirpy's read_10x_vdj, its
# chains indexed (productive chains with a junction, the primary chain of each arm by UMIs), and
# cells grouped with both arms required to match (a missing arm matching a missing arm), primary
# chains only: by nucleotide junction identity (define_clonotypes), and by amino-acid junction
# identity (define_clonotype_clusters with sequence aa), each with and without the V gene.
# Each grouping is reduced to its number of cells and clonotypes and a digest of its partition:
# each clonotype's cell barcodes sorted and joined by ",", the clonotypes sorted, joined by "\n",
# SHA-256 (validation/reference/clonotypes.json).
#
#   uv run --with scirpy python -I validation/reference/scirpy_clonotypes.py
import glob, hashlib, json, os, sys, warnings
warnings.filterwarnings('ignore')
import scirpy as ir
from importlib.metadata import version

here = os.path.dirname(os.path.abspath(__file__))
cache = os.environ.get('CLONOLOOM_DATA') or os.path.join(here, '..', 'cache')

def partition(adata):
    groups = {}
    for cell, clone in adata.obs['clone_id'].items():
        if clone != clone or clone is None:  # NaN: no clonotype
            continue
        groups.setdefault(str(clone), []).append(cell)
    lines = sorted(','.join(sorted(cells)) for cells in groups.values())
    return {'cells': sum(len(c) for c in groups.values()), 'clonotypes': len(groups), 'sha256': hashlib.sha256('\n'.join(lines).encode()).hexdigest()}

out = {}
for path in sorted(glob.glob(os.path.join(cache, '10x-*', '*filtered_contig_annotations.csv'))):
    rel = os.path.relpath(path, cache)
    print(rel, file=sys.stderr)
    entry = {}
    for name, sequence, same_v in (('junction_nt', 'nt', False), ('junction_nt_v_gene', 'nt', True), ('junction_aa', 'aa', False), ('junction_aa_v_gene', 'aa', True)):
        adata = ir.io.read_10x_vdj(path)
        ir.pp.index_chains(adata)
        ir.tl.chain_qc(adata)
        ir.pp.ir_dist(adata, metric='identity', sequence=sequence)
        if sequence == 'nt':
            ir.tl.define_clonotypes(adata, receptor_arms='all', dual_ir='primary_only', same_v_gene=same_v)
        else:
            ir.tl.define_clonotype_clusters(adata, sequence='aa', metric='identity', receptor_arms='all', dual_ir='primary_only', same_v_gene=same_v, key_added='clone_id')
        entry[name] = partition(adata)
    out[rel] = entry
json.dump({'generated_by': f"scirpy {version('scirpy')}", 'files': out}, open(os.path.join(here, 'clonotypes.json'), 'w'), indent=1, sort_keys=True)
