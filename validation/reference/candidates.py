"""Independent descriptive candidate audit using fractions and raw source text.
Run with uv run --no-project --with biopython==1.86 validation/reference/candidates.py DIRECTORY.
No production ranking or sequence implementation is imported.
"""
import csv
import gzip
from Bio.Seq import Seq
from fractions import Fraction
from functools import cmp_to_key
import hashlib
import json
from pathlib import Path
import sys

def digest(raw):
    return hashlib.sha256(raw).hexdigest()

def check(d):
    if 'parents' in d:
        parents = {}
        for p in d['parents']:
            raw = p['text'].encode()
            assert digest(raw) == p['sha256']
            assert any(f['sha256'] == p['sha256'] for f in p['manifest']['files'])
            parents[p['manifest']['kind']] = json.loads(raw)
        f, s, iso = parents['families'], parents.get('shm', {}), parents.get('isotypes', {})
        if s:
            assert s['germlineProvenance']['families'] == d['parents'][0]['manifest']['id']
            assert s['germlineProvenance']['familySha256'] == d['parents'][0]['sha256']
        if iso:
            assert iso['isotypeOptions']['familySha256'] == d['parents'][0]['sha256']
    else:
        f, s, iso = d['families'], d.get('shm', {}), d.get('isotypes', {})
    result = d['result']
    options = result['options']
    observations = f['observations']
    current = [o for o in observations if o['sample'] == options['sample'] and o.get('family') and not o.get('excluded')]
    baseline = [o for o in observations if options['baseline'] and o['sample'] == options['baseline'] and o.get('family') and not o.get('excluded')]
    assert len(current) == result['currentTotal'] and len(baseline) == result['baselineTotal']
    assert len([o for o in observations if o['sample'] == options['sample'] and (not o.get('family') or o.get('excluded'))]) == result['currentExcluded']
    assert len([o for o in observations if options['baseline'] and o['sample'] == options['baseline'] and (not o.get('family') or o.get('excluded'))]) == result['baselineExcluded']
    assert {o['scope'] for o in current} == {result['scope']}
    if baseline:
        assert {o['subject'] for o in current} == {o['subject'] for o in baseline}
        assert {o['scope'] for o in baseline} == {result['scope']}
        assert all(o['subject'] for o in baseline)
    identities = {o['id']:o for o in iso.get('observations', [])}
    mutations = {r['id']:r for r in s.get('rows', [])}
    expected = {}
    for family in [x for x in f['families'] if x['scope'] == result['scope']]:
        key = family['id']
        members = [o for o in current if o['family'] == key]
        old = sum(o['family'] == key for o in baseline)
        values = {'size': Fraction(len(members)), 'change': Fraction(len(members),len(current))-Fraction(old,len(baseline)) if baseline else None}
        n, den, missing, heavy = 0, 0, 0, 0
        for o in members:
            for row in o['rows']:
                if row['locus'] != 'IGH':
                    continue
                heavy += 1
                m = mutations.get(row['id'], {})
                regions = [r for r in m.get('regions',[]) if r['region']=='all' and r['evaluableBases']>0 and not r.get('unavailable') and not m.get('excluded')]
                if regions:
                    n += regions[0]['mutations']; den += regions[0]['evaluableBases']
                else:
                    missing += 1
        values['mutation'] = Fraction(n,den) if s and heavy and not missing else None
        calls = [identities.get(o['id'], {}).get('class') if not identities.get(o['id'], {}).get('excluded') else None for o in members]
        unresolved = sum(c not in ['IgM','IgD','IgG','IgA','IgE'] for c in calls)
        preferred = sum(c == options['preferredClass'] for c in calls)
        values['isotype'] = Fraction(preferred,len(members)) if iso and options['preferredClass'] and members and not unresolved else None
        expected[key] = values
        row = next(r for r in result['rows'] if r['family'] == key)
        assert (row['current'],row['baseline'],row['heavyRows'],row['missingHeavyRows'],row['unresolvedIsotypes']) == (len(members),old,heavy,missing,unresolved)
        assert row['observations'] == sorted(members,key=lambda o:o['id'])
        assert (row['metrics']['mutation']['numerator'],row['metrics']['mutation']['denominator']) == (n,den)
        assert (row['metrics']['isotype']['numerator'],row['metrics']['isotype']['denominator']) == (preferred,len(members))
        for name, value in values.items():
            metric = row['metrics'][name]
            if value is None:
                assert metric['value'] is None and metric['missing']
            else:
                assert Fraction(metric['numerator'],metric['denominator']) == value
                assert abs(metric['value']-float(value)) < 1e-15
    def compare(a,b):
        for criterion in options['criteria']:
            x,y = expected[a][criterion['metric']],expected[b][criterion['metric']]
            if x is None and y is not None: return 1
            if y is None and x is not None: return -1
            if x is None or x==y: continue
            sign = 1 if x>y else -1
            return sign if criterion['direction']=='ascending' else -sign
        return 0
    order = sorted(sorted(expected),key=cmp_to_key(compare))
    assert order == [r['family'] for r in result['rows']]
    for i,key in enumerate(order):
        tied = [j for j,k in enumerate(order) if compare(key,k)==0]
        assert result['rows'][i]['rank'] == min(tied)+1
        assert result['rows'][i]['tied'] == len(tied)
    return len(order)

reviews = {}; selections=[]; count=0
for path in sorted(Path(sys.argv[1]).rglob('*.json')):
    raw=path.read_bytes(); d=json.loads(raw)
    if d.get('format') == 'clonoloom-candidate-sequence':
        selections.append((path,d));continue
    if 'result' not in d or 'criteria' not in d['result'].get('options',{}):continue
    n=check(d);reviews[digest(raw)]=d;count+=1
    print(f'{path.name}: {n} families; exact fractions, missingness, source observations and competition ranks agree.')
for path,s in selections:
    assert s['review'] in reviews
    review=reviews[s['review']]
    assert any(s['observation'] == o for r in review['result']['rows'] for o in r['observations'])
    assert s['row'] in s['observation']['rows']
    source=Path(__file__).resolve().parents[2]/'testdata/candidates'/s['row']['source']['name']
    source_bytes=source.read_bytes()
    assert digest(source_bytes)==s['row']['source']['sha256']
    lines=source_bytes.decode().splitlines()
    assert lines[0]==s['headerText'] and lines[s['row']['line']-1]==s['rowText']
    columns=s['headerText'].lstrip('\ufeff').split('\t');values=s['rowText'].split('\t')
    assert len(columns)==len(values) and dict(zip(columns,values))==s['fields']
    assert s['fields']['sequence_aa']==s['sequence'] and digest(s['sequence'].encode())==s['sha256']
    assert s['fields']['sequence_id']==s['row']['sequenceId'] and s['fields']['locus']==s['row']['locus']
    fasta=s['fasta'].splitlines()
    if s['eligible']:
        assert len(fasta)==2 and fasta[0].startswith('>candidate-') and fasta[1]==s['sequence']
        assert set(s['sequence']) <= set('ACDEFGHIKLMNPQRSTVWY')
        assert any(p.read_text()==s['fasta'] for p in path.parent.glob('*.fasta'))
    print(f'{path.name}: exact selected source chain and companion FASTA agree.')
assert count and selections
print(f'{count} rankings and {len(selections)} source-chain exports independently checked; no biological efficacy is inferred.')

# Independently check every artificial upstream annotation, never translate in product.
fixture=Path(__file__).resolve().parents[2]/'testdata/candidates'
original=Path(__file__).resolve().parents[2]/'validation/projects/0.5.0/data'
missing=0; total=0
for source in sorted(fixture.glob('*.tsv')):
    with source.open() as handle:
        rows=list(csv.DictReader(handle,delimiter='\t',quoting=csv.QUOTE_NONE))
    with gzip.open(original/(source.name+'.gz'),'rt') as handle:
        old=list(csv.DictReader(handle,delimiter='\t',quoting=csv.QUOTE_NONE))
    assert len(old)==len(rows)
    for before,after in zip(old,rows):
        assert all(after[k]==v for k,v in before.items())
        total+=1
        if after['sequence_aa']:
            assert str(Seq(after['sequence']).translate(table=1))==after['sequence_aa']
        else: missing+=1
assert total==120 and missing==1
print('120 artificial rows retain original evidence; 119 supplied translations agree with Biopython and one remains explicitly missing.')
