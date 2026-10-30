"""Make artificial upstream annotations on COPIES of the 0.5.0 controls.
Never used by the product's candidate export. NCBI genetic code table 1.
Run from repository root: uv run --no-project validation/reference/make_candidates.py
"""
import csv
import gzip
import hashlib
import itertools
from pathlib import Path

bases='TCAG'
aas='FFLLSSSSYY**CC*WLLLLPPPPHHQQRRRRIIIMTTTTNNKKSSRRVVVVAAAADDEEGGGG'
assert len(aas)==64
code=dict(zip((''.join(c) for c in itertools.product(bases, repeat=3)),aas))
root=Path('validation/projects/0.5.0/data')
out=Path('testdata/candidates')
out.mkdir(exist_ok=True)
for sample in ['S01-d14','S01-d28','S01-pre','S02-d14','S02-d28','S02-pre']:
    source=root/(sample+'.tsv.gz')
    with gzip.open(source,'rt') as handle:
        reader=csv.DictReader(handle,delimiter='\t');header=list(reader.fieldnames);rows=list(reader)
    assert not set(['sequence_aa','complete_vdj','candidate_fixture']) & set(header)
    for i,row in enumerate(rows):
        sequence=row['sequence']
        assert len(sequence)%3==0 and set(sequence)<=set('ACGT') and row['rev_comp']=='F'
        aa=''.join(code[sequence[j:j+3]] for j in range(0,len(sequence),3))
        assert '*' not in aa
        row['sequence_aa']=aa
        row['complete_vdj']=''  # coverage/full-protein completeness is not inferred here
        row['candidate_fixture']='Artificial upstream translation; no binding or construct claim'
    # One deliberate missing amino-acid observation: no repair or fallback in product.
    if sample=='S01-d28':
        rows[0]['sequence_aa']=''
    with (out/(sample+'.tsv')).open('w',newline='') as handle:
        writer=csv.DictWriter(handle,fieldnames=header+['sequence_aa','complete_vdj','candidate_fixture'],delimiter='\t',lineterminator='\n')
        writer.writeheader();writer.writerows(rows)
    print(sample,hashlib.sha256(source.read_bytes()).hexdigest(),len(rows),'source rows')
(out/'repertoires.airr.json').write_bytes((root/'repertoires.airr.json').read_bytes())
