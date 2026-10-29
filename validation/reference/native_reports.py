"""Independent CSV/Decimal reference for the declared native-report fixtures.
Run with uv run validation/reference/native_reports.py [--write].
Only standard-library parsing; no vendor implementation or ClonoLoom code.
"""
import csv
import hashlib
import json
import sys
from decimal import Decimal
from pathlib import Path

root = Path(__file__).resolve().parents[2]
missing = '\0'
results = {}
for path in sorted((root / 'testdata/native-reports').glob('*.tsv')):
    trust = 'trust4' in path.name
    legacy = 'legacy' in path.name
    raw = path.read_text()
    header, *lines = raw.splitlines()
    reader = csv.DictReader(lines, fieldnames=next(csv.reader([header], delimiter='\t')), delimiter='\t')
    columns = {}
    for line, source in enumerate(reader, 2):
        def get(*names):
            for name in names:
                if name in source:
                    value = source[name].strip()
                    return '' if value in ('', '.', 'NA') else value
            return ''
        aa = get('CDR3_amino_acids', 'CDR3aa', 'cdr3aa')
        nt = get('CDR3_dna', 'CDR3nt', 'cdr3nt')
        flags = dict(productive=missing, vj_in_frame=missing, stop_codon=missing)
        if trust and aa == 'partial':
            aa = nt = ''
        elif (trust and aa == 'out_of_frame') or (not trust and any(marker in aa for marker in ('_', '~'))):
            aa = ''
            flags.update(productive='F', vj_in_frame='F')
        else:
            if trust:
                aa = aa.replace('_', '*').replace('?', 'X')
            aa = aa.upper()
            if aa:
                flags['stop_codon'] = 'T' if '*' in aa else missing if 'X' in aa else 'F'
                if nt:
                    flags['vj_in_frame'] = 'T' if len(nt) % 3 == 0 else 'F'
                if '*' in aa or flags['vj_in_frame'] == 'F':
                    flags['productive'] = 'F'
                elif 'X' not in aa and flags['vj_in_frame'] == 'T':
                    flags['productive'] = 'T'
        count = get('read_count', '#count', 'count')
        if count:
            d = Decimal(count)
            assert 0 <= d <= 9007199254740991
            count = str(d.normalize()) if d != d.to_integral() else str(int(d))
        freq = get('frequency(proportion of read_count)', 'frequency', 'freq')
        if freq:
            assert 0 <= Decimal(freq) <= 1
        genes = {}
        for gene in 'vdjc':
            v = get(gene, gene.upper())
            genes[gene + '_call'] = (v if v != '*' else '') or missing
        j = genes['j_call']
        locus = j[:3] if j.startswith(('TR', 'IG')) else missing
        row = dict(sequence_id=get('cid', 'consensus_id') or missing, locus=locus,
                   junction=nt.upper() or missing, junction_aa=aa or missing,
                   count=count or missing, reads=(count or missing) if legacy else missing,
                   cell=missing, clone=missing, is_cell=missing, high_confidence=missing, full_length=missing,
                   sample=path.stem, source_line=str(line), **flags, **genes)
        for k, v in row.items():
            columns.setdefault(k, []).append(v)
    results[path.name] = dict(format='trust4-report' if trust else 'vdjtools', unit='reads' if legacy else '',
        columns=columns, header=header, native_rows=lines, sha256=hashlib.sha256(path.read_bytes()).hexdigest())
output = root / 'validation/reference/native_reports.json'
if '--write' in sys.argv:
    output.write_text(json.dumps(results, indent=2) + '\n')
else:
    assert json.loads(output.read_text()) == results
    print('3 native reports: all mapped fields, source rows, exact decimals and fingerprints verified')
