"""Independent standard-library byte/ZIP oracle; never re-encodes scientific JSON."""
import base64
import hashlib
import io
import json
from pathlib import Path
import re
import sys
import zipfile


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def check(path):
    page = path.read_bytes()
    members = re.findall(rb'<script type="application/octet-stream" id="(report-record|report-file-[0-9]+)"(?: data-sha256="([a-f0-9]+)")?>([A-Za-z0-9+/=]+)</script>', page)
    raw = base64.b64decode(members[0][2], validate=True)
    assert sha(raw).encode() == members[0][1]
    record = json.loads(raw)
    assert record['format'] == 'clonoloom-html-report' and record['version'] == 1
    assert record['scope'] == 'embedded-report-bytes'
    assert len(members) == len(record['files']) + 1
    files = {}
    for i, item in enumerate(record['files']):
        assert members[i + 1][0] == f'report-file-{i}'.encode()
        data = base64.b64decode(members[i + 1][2], validate=True)
        assert len(data) == item['size'] and sha(data) == item['sha256']
        assert item['path'] not in files
        files[item['path']] = data
    assert list(files) == sorted(files)
    for result in record['results']:
        with zipfile.ZipFile(io.BytesIO(files[result['certificate']])) as archive:
            cert = json.loads(archive.read('certificate.json'))
            assert cert['fingerprint'] == result['fingerprint']
            inventory = {k: v for k, v in cert.items() if k != 'fingerprint'}
            canonical = json.dumps(inventory, separators=(',', ':'), ensure_ascii=False)
            for literal, escaped in [('&', r'\u0026'), ('<', r'\u003c'), ('>', r'\u003e'), ('\u2028', r'\u2028'), ('\u2029', r'\u2029')]:
                canonical = canonical.replace(literal, escaped)
            assert sha(canonical.encode()) == cert['fingerprint']
            assert set(archive.namelist()) == {'certificate.json', *(f['path'] for f in cert['files'])}
            for f in cert['files']:
                data = archive.read(f['path'])
                assert sha(data) == f['sha256'] and len(data) == f['size']
            scientific = archive.read(f"artifact/{result['kind']}.json")
            assert scientific == files[result['result']]
            manifest = archive.read('manifest.json')
        replay_path = path.with_suffix('.replay.json')
        if replay_path.exists():
            replay = json.loads(replay_path.read_bytes())
            assert replay['verdict'] == 'replay-match' and replay['recomputed']
            assert replay['original'] == json.loads(manifest)
            last = replay['steps'][-1]
            assert last['metadataMatched']
            assert last['expectedSHA256'] == last['actualSHA256'] == sha(scientific)
    assert b'<script>' not in files['report.html']
    print(f'{path.name}: {len(files)} exact files; certificate/result bytes and native replay agree')


for name in sys.argv[1:]:
    check(Path(name))
