"""Independent AIRR text/hash audit of reviewed Proteoscope handoffs, without network.
uv run --no-project validation/reference/proteoscope.py OUTPUT_DIRECTORY
"""
import csv
import hashlib
import json
from pathlib import Path
import sys

root = Path(sys.argv[1])
fixture = Path(__file__).resolve().parents[2] / 'testdata/proteoscope/simulated-airr.tsv'
raw = fixture.read_bytes()
lines = raw.decode().splitlines()
header = next(csv.reader([lines[0]], delimiter='\t'))
count = 0
for file in sorted(root.rglob('handoff.json')):
    doc = json.loads(file.read_bytes())
    command = json.loads(file.with_name('command.json').read_bytes())
    receipt = json.loads(file.with_name('receipt.json').read_bytes(), parse_int=str, parse_float=str)
    row = lines[doc['line'] - 1]
    fields = dict(zip(header, next(csv.reader([row], delimiter='\t'))))
    assert doc['format'] == 'clonoloom-proteoscope-handoff' and doc['version'] == 1
    assert doc['headerText'] == lines[0] and doc['rowText'] == row and doc['fields'] == fields
    assert doc['source']['sha256'] == hashlib.sha256(raw).hexdigest()
    assert doc['sequence'] == fields['sequence_aa'] and doc['sequenceId'] == fields['sequence_id']
    assert doc['sequenceSHA256'] == hashlib.sha256(fields['sequence_aa'].encode()).hexdigest()
    assert command == {'command': 'search ' + fields['sequence_aa']}
    assert doc['command'] == command['command'] and doc['eligible']
    assert doc['locus'] == fields['locus'] and doc['sample'] == fields['repertoire_id']
    assert receipt['sequenceSHA256'] == doc['sequenceSHA256']
    assert receipt['fingerprint'] == hashlib.sha256(file.read_bytes().removesuffix(b'\n')).hexdigest()
    assert receipt['acceptedLocal'] and receipt['acceptedPublic'] and receipt['response']['ok']
    assert receipt['destination'] == doc['destination']
    assert receipt['responseSHA256'] == hashlib.sha256(receipt['responseText'].encode()).hexdigest()
    assert json.loads(receipt['responseText'], parse_int=str, parse_float=str) == receipt['response']
    assert fields['sequence_id'] not in command['command']
    if fields['sequence_id'] == '00123':
        assert fields['exact_extension'] == '9007199254740993'
    else:
        assert fields['complete_vdj'] == 'F' and fields['exact_extension'] == '0.1234567890123456789'
    count += 1
    print(f'{file.parent.name}: exact source row, annotations, sequence, command, permissions and receipt agree.')
assert count, 'No exported evidence found'
print(f'{count} handoffs audited. This is exact interchange evidence, not biological validity or live RCSB reproducibility.')
