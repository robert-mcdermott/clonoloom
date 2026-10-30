"""Independent ADC request/evidence control. Standard-library JSON with Decimal, no floats.

Run: UV_CACHE_DIR=/tmp/clonoloom-uv uv run --offline --no-project validation/reference/adc.py
The fixture is artificial metadata, not a public cohort or a schema-compliance claim.
"""
from decimal import Decimal
from hashlib import sha256
import json
from pathlib import Path

root = Path(__file__).resolve().parents[2]
raw = (root / 'internal/adc/testdata/simulated-page.json').read_text()
decoder = json.JSONDecoder(parse_float=Decimal)
doc = decoder.decode(raw)
records = doc['Repertoire']
assert len(records) == 3
assert len({r['subject']['subject_id'] for r in records}) == 2
assert records[0]['lab_extension']['exact_decimal'] == Decimal('0.123456789012345678901')
assert records[0]['lab_extension']['numeric_identifier'] == 9007199254740993
assert records[0]['sample'][0]['single_cell'] is False
assert records[2]['sample'][0]['single_cell'] is None
assert records[0]['sample'][0]['collection_time_point_relative'] == 0

# raw_decode independently locates the original object slices without reserializing numbers.
start = raw.index('[', raw.index('"Repertoire"')) + 1
rows = []
for record in records:
    while raw[start] in ' \n\t\r,':
        start += 1
    parsed, end = decoder.raw_decode(raw, start)
    assert parsed == record
    rows.append(dict(id=record['repertoire_id'], study=record['study']['study_id'],
                     title=record['study']['study_title'], subject=record['subject']['subject_id'],
                     rawJSON=raw[start:end]))
    start = end

# Hand-specified protocol example, independent of the Go request builder.
request = {'filters': {'op': 'and', 'content': [
    {'op': '=', 'content': {'field': 'subject.species.id', 'value': 'NCBITAXON:9606'}},
    {'op': '=', 'content': {'field': 'sample.pcr_target.pcr_target_locus', 'value': 'TRB'}},
    {'op': '=', 'content': {'field': 'study.study_id', 'value': 'SIM-longitudinal-TRB'}}]},
    'format': 'json', 'from': 0, 'size': 20}
request_text = json.dumps(request, sort_keys=True, separators=(',', ':'))
out = dict(requestJSON=request_text, requestSHA256=sha256(request_text.encode()).hexdigest(),
           responseJSON=raw, responseSHA256=sha256(raw.encode()).hexdigest(),
           responseBytes=len(raw.encode()), records=rows, nextOffset=3, completeness='not-established')
(root / 'validation/reference/adc.json').write_text(json.dumps(out, indent=2) + '\n')
print('ADC control: 3 metadata records, 2 declared subject IDs, exact bytes/types and request verified.')
