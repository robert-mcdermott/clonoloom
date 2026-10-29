"""Declared synthetic contig evidence. Run with uv; no participant data or vendor code.

The short sequences are illustrative coding constructs, not complete receptor transcripts.
Source full-length/productivity calls are planted import controls, not validated biology.
"""
import csv
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parent / 'cellranger-json'
ROOT.mkdir(exist_ok=True)
CODONS = dict(zip('ACDEFGHIKLMNPQRSTVWY', 'GCT TGT GAT GAA TTT GGT CAT ATT AAA CTG ATG AAT CCT CAA CGT TCT ACT GTT TGG TAT'.split()))
# Barcode index, chain, junction, reads, UMIs, cell call, productivity, full length.
TRUTH = [
    (0, 'TRA', 'CAVRDSNYQLIW', 2418, 19, True, True, True),
    (0, 'TRB', 'CASSLGQETQYF', 6194, 43, True, True, True),
    (1, 'TRA', 'CAVSDGQKLLF', 831, 7, True, True, True),
    (1, 'TRB', 'CASSQDRGNEQFF', 3549, 28, True, True, True),
    (1, 'TRA', 'CAVKDSNYQLIW', 421, 3, True, True, True),
    (2, 'TRB', 'CASSLGQETQYF', 84, 1, False, True, None),
    (3, 'TRB', None, 0, 0, None, None, False),
    (4, 'mixed', 'CASSQDRGNEQFF', None, None, False, False, None),
]
BARCODES = ['AAACCTGAGACAGGCT-1', 'AACCATGGTGACACAG-1', 'ACCGTAATCCGTTGTC-1', 'ACTGAACTCGTCTGGA-1', 'AGGTCATAGTCAAGGC-1']
GENES = {'TRA': ('TRAV12-2', '', 'TRAJ33', 'TRAC'), 'TRB': ('TRBV7-9', 'TRBD1', 'TRBJ2-5', 'TRBC2')}
regions = ['L-REGION+V-REGION', 'D-REGION', 'J-REGION', 'C-REGION']
objects, rows = [], []
for i, (bc, chain, aa, reads, umis, cell, productive, full) in enumerate(TRUTH):
    actual_chain = 'TRB' if chain == 'mixed' else chain
    calls = GENES[actual_chain]
    nt = ''.join(CODONS[x] for x in aa) if aa else None
    sequence = 'ATG' + 'GCTTCTGGT' * 30 + (nt or 'TGTAAT') + 'TTTGGTACT' * 10 + 'TAA'
    annotations = []
    for j, (region, gene) in enumerate(zip(regions, calls)):
        if not gene:
            continue
        annotations.append({'feature': {'chain': 'TRA' if chain == 'mixed' and j == 3 else actual_chain,
                                       'gene_name': 'TRAC' if chain == 'mixed' and j == 3 else gene,
                                       'region_type': region, 'feature_id': 100 + j},
                            'score': 210 - j * 25})
    cid = f'{BARCODES[bc]}_contig_{i+1}'
    clone = f'clonotype{bc+1}' if cell else None
    r = {'barcode': BARCODES[bc], 'contig_name': cid, 'sample': 'Simulated-S01',
         'sequence': sequence, 'read_count': reads, 'umi_count': umis,
         'is_cell': cell, 'high_confidence': cell, 'productive': productive,
         'full_length': full, 'cdr3': aa, 'cdr3_seq': nt, 'annotations': annotations,
         'clonotype': None, 'info': {'raw_clonotype_id': clone},
         'simulated_annotation': {'external_id': 9007199254740993, 'score': 'EXACT_DECIMAL'}}
    objects.append(r)
    rows.append(dict(sample='Simulated-S01', barcode=BARCODES[bc], contig_id=cid, chain=actual_chain if chain != 'mixed' else 'Multi',
                     v_gene=calls[0], d_gene=calls[1], j_gene=calls[2], c_gene='TRAC' if chain=='mixed' else calls[3],
                     cdr3=aa, cdr3_nt=nt, reads=reads, umis=umis, is_cell=cell, high_confidence=cell,
                     productive=productive, full_length=full, raw_clonotype_id=clone))
text = json.dumps(objects, indent=2).replace('"EXACT_DECIMAL"', '0.1234567890123456789012345') + '\n'
(ROOT / 'simulated_all_contig_annotations.json').write_text(text)
with (ROOT / 'simulated_all_contig_annotations.csv').open('w', newline='') as f:
    w = csv.DictWriter(f, fieldnames=list(rows[0])); w.writeheader(); w.writerows(rows)
airr_names = dict(sample='repertoire_id', barcode='cell_id', contig_id='sequence_id', chain='locus', v_gene='v_call', d_gene='d_call', j_gene='j_call', c_gene='c_call', cdr3='junction_aa', cdr3_nt='junction', reads='consensus_count', umis='umi_count', productive='productive', raw_clonotype_id='clone_id')
with (ROOT / 'simulated_airr_rearrangement.tsv').open('w', newline='') as f:
    w = csv.DictWriter(f, fieldnames=list(airr_names.values()), delimiter='\t'); w.writeheader()
    for row in rows:
        a = {out: row[key] for key, out in airr_names.items()}
        a['productive'] = '' if row['productive'] is None else 'T' if row['productive'] else 'F'
        a['locus'] = '' if row['chain'] == 'Multi' else row['chain']
        w.writerow(a)
# A separate legacy control intentionally omits full_length and sample.
legacy = objects[:2]
for r in legacy:
    del r['full_length']; del r['sample']
(ROOT / 'legacy_all_contig_annotations.json').write_text(json.dumps(legacy, indent=2).replace('"EXACT_DECIMAL"', '0.1234567890123456789012345') + '\n')
