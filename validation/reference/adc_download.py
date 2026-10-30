"""Independent byte/scope audit of an ADC capture, not scientific or publisher validation.
Usage: uv run --offline --no-project validation/reference/adc_download.py CAPTURE_DIRECTORY
"""
import hashlib
import json
import sys
from decimal import Decimal
from pathlib import Path


def unique(pairs):
    out = {}
    for key, value in pairs:
        assert key not in out, 'Duplicate JSON key'
        out[key] = value
    return out


def exact(raw):
    def invalid(value):
        raise ValueError('Non-JSON number: ' + value)
    return json.loads(raw, parse_float=Decimal, object_pairs_hook=unique, parse_constant=invalid)


def audit(folder):
    folder = Path(folder)
    manifest = exact((folder / 'manifest.json').read_bytes())
    assert manifest['version'] == 1
    assert manifest['completeness'] == 'not-established'
    assert manifest['stop'] in ('empty-page', 'page-budget')
    spec = manifest['spec']
    assert spec['accept'] and spec['license'].strip() and spec['citation'].strip()
    assert 1 <= spec['pages'] <= 10 and 1 <= spec['size'] <= 1000
    assert len(manifest['pages']) <= spec['pages'] + 1
    seen = set()
    total = 0
    for index, page in enumerate(manifest['pages']):
        expected = 'metadata.json' if index == 0 else f'page-{index:03d}.json'
        assert page['file'] == expected
        source = folder / expected
        assert not source.is_symlink()
        raw = source.read_bytes()
        assert len(raw) == page['bytes'] <= 8 * 1024 * 1024
        assert hashlib.sha256(raw).hexdigest() == page['sha256']
        assert hashlib.sha256(page['requestJSON'].encode()).hexdigest() == page['requestSHA256']
        request = exact(page['requestJSON'])
        doc = exact(raw)
        if index == 0:
            assert len(doc['Repertoire']) == 1
            assert doc['Repertoire'][0]['repertoire_id'] == spec['repertoire']
            continue
        assert request['filters'] == {'op': 'and', 'content': [
            {'op': '=', 'content': {'field': 'repertoire_id', 'value': spec['repertoire']}},
            {'op': '=', 'content': {'field': 'data_processing_id', 'value': spec['processing']}}]}
        rows = doc['Rearrangement']
        assert request['from'] == total and request['size'] == spec['size']
        assert len(rows) == page['records'] <= spec['size']
        for row in rows:
            assert row['repertoire_id'] == spec['repertoire']
            assert row['data_processing_id'] == spec['processing']
            assert isinstance(row['sequence_id'], str) and row['sequence_id']
            assert row['sequence_id'] not in seen
            seen.add(row['sequence_id'])
        total += len(rows)
    assert total == manifest['records']
    if manifest['stop'] == 'empty-page':
        assert manifest['pages'][-1]['records'] == 0
    else:
        assert len(manifest['pages']) == spec['pages'] + 1
        assert manifest['pages'][-1]['records'] > 0
    print(f'Checked {total} captured rows; completeness remains unestablished.')


if __name__ == '__main__':
    audit(sys.argv[1])
