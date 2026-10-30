"""Independent JSON/Decimal and CSV/AIRR controls; no ClonoLoom/vendor imports.

uv run --no-project validation/reference/cellranger_json.py [--write]
Optional public cross-format comparison:
  ... --public JSON_FILE ALL_CONTIG_CSV OUTPUT_REFERENCE
No public source data or derived participant records are committed.
"""
import csv
import hashlib
import json
import sys
from decimal import Decimal
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
MISSING = '\x00'
def text(x):
    if x is None or x == '': return MISSING
    if x is True: return 'T'
    if x is False: return 'F'
    return str(x)

def reference(path):
    raw = path.read_bytes()
    values = json.loads(raw, parse_float=Decimal)
    fields = {k: [] for k in ['sequence_id', 'locus', 'v_call', 'd_call', 'j_call', 'c_call', 'junction', 'junction_aa', 'productive', 'cell', 'clone', 'count', 'reads', 'umis', 'sequence', 'vj_in_frame', 'stop_codon', 'is_cell', 'high_confidence', 'full_length', 'sample']}
    fallback = path.name.removesuffix('.json').removesuffix('_all_contig_annotations')
    for obj in values:
        features = [a['feature'] for a in obj['annotations']]
        chains = {f['chain'] for f in features}
        genes = {f['region_type']: f['gene_name'] for f in features}
        row = dict(sequence_id=obj['contig_name'], locus=next(iter(chains)) if len(chains)==1 else None,
                   v_call=genes.get('L-REGION+V-REGION'), d_call=genes.get('D-REGION'), j_call=genes.get('J-REGION'), c_call=genes.get('C-REGION'),
                   junction=obj.get('cdr3_seq'), junction_aa=obj.get('cdr3'), productive=obj.get('productive'), cell=obj['barcode'],
                   clone=(obj.get('info') or {}).get('raw_clonotype_id'), count=obj.get('umi_count'), reads=obj.get('read_count'),
                   umis=obj.get('umi_count'), sequence=obj.get('sequence'), vj_in_frame=None, stop_codon=None,
                   is_cell=obj.get('is_cell'), high_confidence=obj.get('high_confidence'), full_length=obj.get('full_length'), sample=obj.get('sample') or fallback)
        for key in fields: fields[key].append(text(row[key]))
    return dict(sha256=hashlib.sha256(raw).hexdigest(), columns=fields)

def compare_csv(source, csv_path):
    values=json.loads(source.read_text(), parse_float=Decimal)
    rows={r['contig_id']:r for r in csv.DictReader(csv_path.open())}
    aliases={'barcode':'barcode', 'contig_name':'contig_id', 'cdr3':'cdr3', 'cdr3_seq':'cdr3_nt', 'read_count':'reads', 'umi_count':'umis', 'is_cell':'is_cell', 'high_confidence':'high_confidence', 'productive':'productive', 'full_length':'full_length'}
    checked=0; absent_full=0; mixed=0
    for obj in values:
        row=rows[obj['contig_name']]
        for source_key, target in aliases.items():
            if source_key not in obj:
                if source_key=='full_length': absent_full+=1
                continue
            expected=str(obj[source_key]).lower() if obj[source_key] is not None else ''
            actual=row[target].lower()
            if actual in ('none','na'): actual=''
            assert actual==expected, (obj['contig_name'],source_key,actual,expected)
            checked+=1
        features=[a['feature'] for a in obj['annotations']]
        chains={f['chain'] for f in features}
        expected=next(iter(chains)) if len(chains)==1 else 'Multi' if chains else 'None'
        assert row['chain'].lower() in (expected.lower(), '' if not chains else expected.lower())
        mixed+=len(chains)>1
        for region,target in [('L-REGION+V-REGION','v_gene'),('D-REGION','d_gene'),('J-REGION','j_gene'),('C-REGION','c_gene')]:
            expected=next((f['gene_name'] for f in features if f['region_type']==region),'')
            actual=row[target]; actual='' if actual in ('None','NA') else actual
            assert actual==expected,(obj['contig_name'],target)
        clone=(obj.get('info') or {}).get('raw_clonotype_id') or ''
        actual=row['raw_clonotype_id']; actual='' if actual in ('None','NA') else actual
        assert actual==clone
    assert len(rows)==len(values)
    return dict(records=len(values),shared_values_checked=checked,missing_full_length=absent_full,mixed_chain_records=mixed)

if '--public' in sys.argv:
    path,csv_path,out=map(Path,sys.argv[2:5])
    result=reference(path); result['csv_comparison']=compare_csv(path,csv_path)
    out.write_text(json.dumps(result,indent=2)+'\n'); print(result['csv_comparison'])
else:
    folder=ROOT/'testdata/cellranger-json'
    expected={p.name:reference(p) for p in sorted(folder.glob('*.json'))}
    print(compare_csv(folder/'simulated_all_contig_annotations.json',folder/'simulated_all_contig_annotations.csv'))
    # The AIRR control independently agrees on all common represented values.
    airr=list(csv.DictReader((folder/'simulated_airr_rearrangement.tsv').open(), delimiter='\t'))
    columns=expected['simulated_all_contig_annotations.json']['columns']
    for key in ['sequence_id','v_call','d_call','j_call','c_call','junction','junction_aa','productive']:
        assert [r[key] or MISSING for r in airr]==columns[key],key
    for source,key in [('cell_id','cell'),('clone_id','clone'),('consensus_count','reads'),('umi_count','count'),('repertoire_id','sample')]:
        assert [r[source] or MISSING for r in airr]==columns[key],key
    target=Path(__file__).with_suffix('.json')
    if '--write' in sys.argv: target.write_text(json.dumps(expected,indent=2)+'\n')
    else: assert json.loads(target.read_text())==expected
    print('Exact JSON, CSV and AIRR controls agree; missing flags and mixed chains are explicit.')
