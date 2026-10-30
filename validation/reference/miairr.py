"""Independent MiAIRR 2.0 inventory and hand-specified presence controls.

uv run --no-project validation/reference/miairr.py \
  --inventory /tmp/clonoloom-miairr-inventory.json \
  --evidence /tmp/clonoloom-miairr-evidence.json

Optional --source verifies the official TSV bytes and extracted inventory. The
TSV is standard documentation, not software. No reference runtime is bundled.
"""
import argparse
import csv
import hashlib
import json
from pathlib import Path

HERE = Path(__file__).resolve().parent
parser = argparse.ArgumentParser()
parser.add_argument('--source', type=Path)
parser.add_argument('--inventory', type=Path, required=True)
parser.add_argument('--evidence', type=Path, required=True)
args = parser.parse_args()
reference = json.loads((HERE / 'miairr_fields.json').read_text())
if args.source:
    assert hashlib.sha256(args.source.read_bytes()).hexdigest() == reference['sha256'], 'Official table bytes changed; review instead of accepting new defaults'
    with args.source.open(newline='') as handle:
        official = [dict(field=r['Field'], level=r['Level'], type=r['Type'], subset=r['Subset'])
                    for r in csv.DictReader(handle, delimiter='\t') if r['Set'] != '6']
    assert official == reference['fields']
native = json.loads(args.inventory.read_text())
assert set(native) == {r['field'] for r in reference['fields']}
for row in reference['fields']:
    item = native[row['field']]
    assert item['level'] == row['level'], row
    kind = row['type'] if row['type'] in ('string', 'boolean', 'integer') else 'array' if row['type'].startswith('array') else 'object'
    assert item['kind'] == kind, row
reports = json.loads(args.evidence.read_text())
assert len(reports) == 2
# Hand-specified controls from testdata/metadata/miairr.airr.json; these are not
# native expected-output snapshots. They distinguish repeated scopes/reasons.
controls = [
    {'/study/study_id': ('not_collected', 'important'),
     '/study/contributors': ('empty', 'essential'),
     '/study/pub_ids': ('null', 'important'),
     '/study/grants': ('not_applicable', 'important'),
     '/subject/synthetic': ('value-present', 'essential'),
     '/subject/species': ('structure-review', 'essential'),
     '/subject/age': ('null', 'important'),
     '/subject/diagnosis': ('absent', 'scope'),
     '/sample/0/single_cell': ('value-present', 'important'),
     '/sample/0/template_class': ('value-present', 'essential'),
     '/sample/0/pcr_target/0/pcr_target_locus': ('value-present', 'important'),
     '/sample/0/pcr_target/0/forward_pcr_primer_target_location': ('not_collected', 'important'),
     '/sample/0/sequencing_files/paired_filename': ('not_applicable', 'important'),
     '/sample/0/cell_label': ('absent', 'defined'),
     '/data_processing/0/software_versions': ('value-present', 'important'),
     '/data_processing/1/software_versions': ('unknown-marker', 'important'),
     '/data_processing/0/quality_thresholds': ('not_applicable', 'important'),
     '/data_processing/1/quality_thresholds': ('null', 'important')},
    {'/subject/species': ('null', 'essential'), '/sample': ('empty', 'scope'),
     '/data_processing': ('empty', 'scope'), '/study/contributors': ('absent', 'essential')},
]
for report, expected in zip(reports, controls):
    assert report['version'] == 'clonoloom-miairr-2.0-v1'
    assert report['status'] == 'review'
    assert 'set 6' in report['notice'] and 'not MiAIRR compliance or scientific validity' in report['notice']
    for path, want in expected.items():
        matches = [r for r in report['items'] if r['path'] == path]
        assert matches, path
        assert all((r['state'], r['level']) == want for r in matches), (path, matches)
    counts = {}
    for row in report['items']:
        counts[row['state']] = counts.get(row['state'], 0) + 1
    assert report['counts'] == counts
print('82 official field names, levels and types; 22 hand-specified evidence controls; native counts verified')
