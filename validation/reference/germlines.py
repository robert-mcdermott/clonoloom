"""Independent ungapped-reference projections from CSV and FASTA, without Go results.

uv run --no-project validation/reference/germlines.py STUDY
Artificial data: direct V/J coordinate slicing (no CIGAR implementation), independently
checked against generative ancestors only after reconstruction. Public 10x data: report
CIGAR/allele availability, not a claim of biological truth. No external source code used.
"""
import csv
import hashlib
import json
from pathlib import Path
import platform
import sys

root = Path(__file__).resolve().parents[2]
study = Path(sys.argv[1])

def fasta(path):
    records = {}
    for line in path.read_text().splitlines():
        if line.startswith('>'):
            name = line[1:]
            assert name not in records
            records[name] = ''
        else:
            records[name] += line.upper()
    assert all(set(seq) <= set('ACGTNRYWSKMBDHV') for seq in records.values())
    return records

def sha(data):
    return hashlib.sha256(data).hexdigest()

reference = fasta(study / 'reference/germlines.fasta')
truth = json.loads((study / 'truth/truth.json').read_text())
ancestors = {n['family']: n for n in truth['nodes'] if not n['parent']}
assignment = {(r['sample_id'], r['cell_id']): r for r in csv.DictReader((study / 'truth/observations.tsv').open(), delimiter='\t')}
lines = []
for path in sorted(study.glob('*.tsv')):
    for row in csv.DictReader(path.open(), delimiter='\t'):
        vs, ve, js, je = [int(row[x]) for x in ('v_germline_start', 'v_germline_end', 'j_germline_start', 'j_germline_end')]
        qs, qe, rs, re = [int(row[x]) for x in ('v_sequence_start', 'v_sequence_end', 'j_sequence_start', 'j_sequence_end')]
        v, j = reference[row['v_call']][vs-1:ve], reference[row['j_call']][js-1:je]
        germline = v + 'N' * (rs-qe-1) + j
        query = row['sequence'][qs-1:re]
        regions = 'V'*len(v) + 'N'*(rs-qe-1) + 'J'*len(j)
        assert len(germline) == len(query) == 360 and germline[285:318] == 'N'*33
        # Truth does not participate in construction above.
        known = ancestors[assignment[row['repertoire_id'], row['cell_id']]['node_id'].rsplit('-N', 1)[0]]
        ancestor = known['heavy' if row['locus'] == 'IGH' else 'light']['sequence']
        assert germline[:285] == ancestor[:285] and germline[318:] == ancestor[318:]
        lines.append('\t'.join([row['repertoire_id'], row['sequence_id'], row['locus'], row['v_call'], row['j_call'], query, germline, regions, ','.join(map(str,range(1,361))), ','.join(map(str,[*range(1,286), *([0]*33), *range(1,43)]))]))
result = {'python': platform.python_version(), 'method': 'Independent CSV/FASTA coordinate slicing, then ancestor comparison',
          'model': truth['model'], 'rows': len(lines), 'referenceSHA256': sha((study / 'reference/germlines.fasta').read_bytes()),
          'projectionsSHA256': sha(('\n'.join(sorted(lines))+'\n').encode())}
output = Path(__file__).with_name('germlines.json')
output.write_text(json.dumps(result, indent=2)+'\n')
print(json.dumps(result,indent=2))
