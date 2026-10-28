"""Executed only by clients/test-clients.mjs against an isolated normal host/window."""
import json
import os
import time
import urllib.request
from decimal import Decimal
from pathlib import Path
import clonoloom as cl

folder = os.environ['CLONOLOOM_TEST_LIBRARY']
study = Path(os.environ['CLONOLOOM_TEST_STUDY'])
connection = cl.find_connection(folder)
client = cl.connect(data_dir=folder, client='Python research notebook')
assert client.permissions() == {'read': True, 'edits': True, 'compute': True, 'files': True}
assert len(client.tools()) == 61


def idle():
    for _ in range(300):
        state = client.get_state().data
        if not state['importing'] and not state['runningJobs']:
            return
        time.sleep(.1)
    raise AssertionError('Existing work did not finish')


client.new_project(name='Artificial vaccination repertoire · Python')
idle()
paths = [str(study / name) for name in ['S01-pre-1.tsv', 'S01-d14.tsv', 'S01-d28.tsv']]
client.open_data(paths, name='Simulated vaccination · subject S01')
idle()
result = client.query_clonotypes(limit=1000)
native = result.native()
assert native['count'] > 100
assert isinstance(native['rows'][0]['total'], int)
assert isinstance(native['rows'][0]['frequency'], Decimal)
selected = native['rows'][3]
filtered = client.query_clonotypes(where={'field': 'id', 'op': 'eq', 'value': selected['id']}, limit=1).native()
assert filtered['count'] == native['count'] and filtered['definitionId'] == native['definitionId']
assert filtered['matched'] == 1 and filtered['rows'][0]['rank'] == 1
assert {**filtered['rows'][0], 'rank': selected['rank']} == selected
# Separate raw HTTP request: original native query text, not float-decoded comparisons.
request = urllib.request.Request(connection['url'] + '/api/remote/action', data=json.dumps({'action':'query_clonotypes','args':{'limit':1000},'client':'HTTP parity control'}).encode(), headers={'Content-Type':'application/json','X-ClonoLoom-Token':connection['token']})
with urllib.request.urlopen(request) as response:
    direct = json.loads(response.read(), parse_float=Decimal)
assert direct['data']['resultJSON'] == result.data['resultJSON']
frame = result.frame()
assert frame.at[0, 'frequency'] == native['rows'][0]['frequency']
assert frame.at[0, 'total'] == native['rows'][0]['total']
assert all(str(dtype) == 'object' for dtype in frame.dtypes)
for call, expected, error_type in [
    (lambda: cl.connect(connection['url'], '').query_clonotypes(), 'unauthorized', cl.PermissionDenied),
    (lambda: client.open_example(example='simulated-vaccine'), 'permission-denied', cl.PermissionDenied),
    (lambda: client.query_clonotypes(limit=None), 'invalid-arguments', cl.ActionError),
    (lambda: client.query_clonotypes(where={'field':'total','op':'gte','value':9007199254740993}), 'invalid-arguments', cl.ActionError),
    (lambda: client.call('no_such_action'), 'unknown-action', cl.ActionError),
]:
    try:
        call()
        raise AssertionError('Expected refusal')
    except error_type as error:
        assert error.code == expected, (error.code, expected)
rejected = cl.connect(data_dir=folder, client='Python rejection control')
try:
    rejected.set_definition(preset='Nucleotide junction')
    raise AssertionError('Proposal was not rejected')
except cl.ActionError as error:
    assert error.code == 'rejected' and not error.outcome_unknown
cancelled = cl.connect(data_dir=folder, client='Python cancellation control', timeout=1)
try:
    cancelled.set_definition(preset='Nucleotide junction')
    raise AssertionError('Proposal did not time out')
except cl.ConnectionError as error:
    assert error.outcome_unknown
# Wait for the window's cancellation to settle; never resubmit the timed-out edit.
time.sleep(.5)
idle()
client.set_definition(preset='Nucleotide junction')
if os.environ.get('CLONOLOOM_TEST_PARITY'):
    final_native = client.query_clonotypes(limit=1000).data['resultJSON']
    with urllib.request.urlopen(request) as response:
        assert json.loads(response.read(), parse_float=Decimal)['data']['resultJSON'] == final_native
    Path(os.environ['CLONOLOOM_TEST_PARITY']).write_bytes(final_native.encode('utf-8'))
print('Python live checks passed: exact native HTTP parity, denominators, frames, grants/token/null/numeric refusals, human rejection, timeout and reviewed edit.', flush=True)
