# /// script
# requires-python = ">=3.11"
# dependencies = ["scipy==1.17.0"]
# ///
"""Independent MEX values, source coverage and generating-truth checks.

uv run validation/reference/cell_states.py /path/to/cell-states-study
Uses SciPy mmread (https://docs.scipy.org/doc/scipy/reference/generated/scipy.io.mmread.html),
Python CSV and sets. Does not call ClonoLoom's expression reader, links or summaries.
"""
from pathlib import Path
import csv
import hashlib
import json
import sys
import scipy
from scipy.io import mmread

study = Path(sys.argv[1])
truth = json.loads((study / 'truth/truth.json').read_text())
assert truth['model'] == 'clonoloom-artificial-cell-states-v1'
assert truth['study']['seed'] == 20261028
observations = list(csv.DictReader((study / 'truth/observations.tsv').open(), delimiter='\t'))
samples, sources = [], {}
pairs = {}
for matrix in sorted(study.glob('expression/*/matrix.mtx')):
    folder, sample = matrix.parent, matrix.parent.name
    counts = mmread(matrix).toarray()
    assert counts.shape == (24, 120)
    barcodes = (folder / 'barcodes.tsv').read_text().splitlines()
    cells = list(csv.DictReader((folder / 'cells.tsv').open(), delimiter='\t'))
    receptors = list(csv.DictReader((study / f'{sample}.tsv').open(), delimiter='\t'))
    for file in [*sorted(folder.iterdir()), study / f'{sample}.tsv']:
        sources[str(file.relative_to(study))] = hashlib.sha256(file.read_bytes()).hexdigest()
    expr = set(barcodes)
    ann = {r['barcode']: r for r in cells}
    rec = {r['cell_id'] for r in receptors}
    known = {r['barcode']: r for r in observations if r['sample_id'] == sample}
    mapped = {b for b, r in ann.items() if r['map_x'] != '' and r['map_y'] != ''}
    totals, genes = counts.sum(axis=0).tolist(), (counts > 0).sum(axis=0).tolist()
    for b, total in zip(barcodes, totals):
        k = known[b]
        assert k['expression'] == 'true' and total == int(k['gene_umis'])
        assert (b in rec) == (k['receptor'] == 'true')
        assert (b in ann) == (k['annotation'] == 'true')
        assert (b in mapped) == (k['coordinates'] == 'true')
        if b in ann and ann[b]['state']:
            assert ann[b]['state'] == k['state']
    assert totals[-1] == 0 and known[next(iter(rec-expr))]['gene_umis'] == ''
    # Every known clone has a single paired receptor within its subject across samples.
    for row in receptors:
        # Required AIRR fields are present, including explicitly empty unavailable alignments.
        assert row['sequence'] == row['junction'] and row['rev_comp'] == 'F' and row['productive'] == 'T'
        assert all(row[k] == '' for k in ['d_call', 'sequence_alignment', 'germline_alignment', 'v_cigar', 'd_cigar', 'j_cigar'])
        key = known[row['cell_id']]['clone_id'], row['locus']
        assert key not in pairs or pairs[key] == row['junction']
        pairs[key] = row['junction']
    assert len(receptors) == 240 and len(rec) == 120
    samples.append({'name': sample, 'denseSHA256': hashlib.sha256(counts.astype('<u8').tobytes()).hexdigest(),
                    'geneUMIs': totals, 'genes': genes, 'matched': len(expr & rec),
                    'expressionOnly': len(expr-rec), 'receptorOnly': len(rec-expr),
                    'annotations': {'columns': ['state'], 'rows': len(cells), 'matched': len(expr & ann.keys()),
                                    'annotationOnly': len(ann.keys()-expr), 'withoutAnnotation': len(expr-ann.keys()),
                                    'mapped': len(expr & mapped), 'withoutCoordinates': len(expr-mapped)}})
output = {'reference': f'SciPy {scipy.__version__} mmread; Python set joins and generating truth', 'sources': sources, 'samples': samples}
Path(__file__).with_suffix('.json').write_text(json.dumps(output, indent=2)+'\n')
print(f'{len(samples)} samples; 480 expression cells and 960 paired chain rows checked')
