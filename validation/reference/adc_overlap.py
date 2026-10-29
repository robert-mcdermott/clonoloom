"""Independent set oracle and simulated ADC evidence; no downloaded human records.
Run with uv run --offline --no-project validation/reference/adc_overlap.py [--write].
Standard-library sets/Fraction calculate presence overlap independently of native code.
"""
import csv
import hashlib
import json
import random
import sys
from fractions import Fraction
from pathlib import Path
from adc_download import audit, exact

ROOT = Path(__file__).resolve().parents[2]
DEST = ROOT / 'testdata/public-overlap'
CANONICAL = set('ACDEFGHIKLMNPQRSTVWY')


def dumps(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':'), ensure_ascii=False)


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def generate():
    DEST.mkdir(exist_ok=True)
    rng = random.Random(90109)
    junctions = ['CASS' + ''.join(rng.choice('AGQSTNDR') for _ in range(5 + i % 5)) + 'EQYF' for i in range(90)]
    assert len(set(junctions)) == len(junctions)
    query = [{'sequence_id': f'SIM-Q-{i+1}', 'repertoire_id': 'SIM-query', 'locus': 'TRB', 'junction_aa': junctions[i % 40], 'productive': 'T', 'duplicate_count': 1 + (i * 7) % 31} for i in range(80)]
    for i, changes in enumerate([{'productive': 'F'}, {'locus': 'TRA'}, {'junction_aa': 'CASSXEQYF'}]):
        query.append(dict(query[i], sequence_id=f'SIM-excluded-{i}', **changes))
    with (DEST / 'query.tsv').open('w', newline='') as stream:
        writer = csv.DictWriter(stream, fieldnames=query[0], delimiter='\t', lineterminator='\n')
        writer.writeheader(); writer.writerows(query)
    selections = [('A', list(range(18)) + list(range(40, 58)), 'SIM-subject-A', True),
                  ('B', list(range(12, 24)) + list(range(58, 76)), 'SIM-subject-B', True),
                  ('C', list(range(5)), 'SIM-subject-C', False),
                  ('D', list(range(4)) + list(range(76, 80)), '', True)]
    for letter, indices, subject, productive in selections:
        folder = DEST / f'adc-download-sim-{letter.lower()}'
        folder.mkdir(exist_ok=True)
        rep = f'SIM-repertoire-{letter}'
        spec = {'repository': 'ipa1.ireceptor.org', 'repertoire': rep, 'processing': 'SIM-process', 'license': 'CC0 artificial test evidence; no provider request or human records.', 'citation': 'ClonoLoom simulated public-overlap fixture; not a public study.', 'pages': 2 if letter == 'B' else 1, 'size': 100, 'accept': True}
        meta = {'Repertoire': [{'repertoire_id': rep, 'study': {'study_id': 'SIM-study', 'study_title': 'Artificial TRB evidence'}, 'subject': {'subject_id': subject}, 'data_processing': [{'data_processing_id': 'SIM-process'}], 'fixture_integer': 9007199254740993}]}
        rows = [{'sequence_id': f'SIM-{letter}-{i}', 'repertoire_id': rep, 'data_processing_id': 'SIM-process', 'locus': 'TRB', 'junction_aa': junctions[j], 'productive': productive, 'duplicate_count': 1 + (i * 3) % 17} for i, j in enumerate(indices + (indices[:12] if letter in 'AB' else []))]
        pages = []
        docs = [('metadata.json', meta), ('page-001.json', {'Rearrangement': rows})]
        if letter == 'B':
            docs.append(('page-002.json', {'Rearrangement': []}))
        for i, (name, doc) in enumerate(docs):
            pred = lambda field, value: {'op': '=', 'content': {'field': field, 'value': value}}
            request = {'filters': pred('repertoire_id', rep), 'format': 'json', 'from': 0, 'size': 1} if i == 0 else {'filters': {'op': 'and', 'content': [pred('repertoire_id', rep), pred('data_processing_id', 'SIM-process')]}, 'format': 'json', 'from': 0 if i == 1 else len(rows), 'size': 100}
            raw = (dumps(doc) + '\n').encode()
            (folder / name).write_bytes(raw)
            req = dumps(request)
            pages.append({'file': name, 'requestJSON': req, 'requestSHA256': sha(req.encode()), 'endpoint': 'https://ipa1.ireceptor.org/airr/v1/' + ('repertoire' if i == 0 else 'rearrangement'), 'retrievedAt': '2026-10-01T00:00:00Z', 'sha256': sha(raw), 'bytes': len(raw), 'records': len(doc['Repertoire' if i == 0 else 'Rearrangement'])})
        manifest = {'version': 1, 'contract': 'ADC 1.2.0 /airr/v1 (AIRR Standards 1.5.1)', 'registryRevision': '4ae159f2799594a2c73669527f6fba97fc8995d7', 'spec': spec, 'pages': pages, 'records': len(rows), 'stop': 'empty-page' if letter == 'B' else 'page-budget', 'completeness': 'not-established', 'notice': 'Simulated incomplete evidence. No live repository contacted.'}
        (folder / 'manifest.json').write_text(json.dumps(manifest, indent=2) + '\n')


def keys(rows):
    eligible = [r for r in rows if r.get('productive') is True and r.get('locus') == 'TRB' and isinstance(r.get('junction_aa'), str) and 3 <= len(r['junction_aa']) <= 256 and r['junction_aa'].startswith('C') and r['junction_aa'][-1] in 'FW' and set(r['junction_aa']) <= CANONICAL]
    return {r['junction_aa'] for r in eligible}, {'rows': len(rows), 'eligible': len(eligible), 'unique': len({r['junction_aa'] for r in eligible}), 'excluded': len(rows) - len(eligible)}


def reference():
    with (DEST / 'query.tsv').open() as stream:
        rows = list(csv.DictReader(stream, delimiter='\t'))
    for row in rows:
        row['productive'] = row['productive'] == 'T'
    q, elig = keys(rows)
    out = {'query': elig, 'pairs': [], 'queryKeysShared': 0, 'assignedSubjects': 3, 'evaluableSubjects': 2, 'missingSubjects': 1, 'unassignedCaptures': 1}
    shared = set()
    support = {}
    for folder in sorted(DEST.glob('adc-download-*')):
        audit(folder)
        manifest = exact((folder / 'manifest.json').read_bytes())
        rows = [row for page in manifest['pages'][1:] for row in exact((folder / page['file']).read_bytes())['Rearrangement']]
        p, eligibility = keys(rows)
        both, union = q & p, q | p
        shared |= both
        for key in both:
            counts = support.setdefault('TRB:' + key, {'captures': 0, 'subjects': 0})
            counts['captures'] += 1
            counts['subjects'] += bool(exact((folder / 'metadata.json').read_bytes())['Repertoire'][0]['subject']['subject_id'])
        ratio = Fraction(len(both), len(union)) if q and p else None
        out['pairs'].append({'capture': folder.name, 'eligibility': eligibility, 'shared': len(both), 'union': len(union), 'jaccard': float(ratio) if ratio is not None else None, 'fraction': str(ratio) if ratio is not None else None})
    out['sharedKeys'] = [dict(key=key, **support[key]) for key in sorted(support)]
    out['queryKeysShared'] = len(shared)
    return out


if __name__ == '__main__':
    if '--write' in sys.argv:
        generate()
    result = reference()
    target = DEST / 'expected.json'
    if '--write' in sys.argv:
        target.write_text(json.dumps(result, indent=2) + '\n')
    else:
        assert json.loads(target.read_text()) == result
    print('Independent public-overlap set controls passed; no prevalence or specificity inference.')
