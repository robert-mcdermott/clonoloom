"""Independent readers, not linked into ClonoLoom.

Generate fixtures:
  CLONOLOOM_PARQUET_FIXTURES=/tmp/parquet-controls go test -run 'TestParquet|TestWriterIndependentFixture' . ./internal/parquet
Read with pinned validators:
  uv run --no-project --with pyarrow==26.0.0 --with duckdb==2.0.0 validation/reference/parquet_exports.py /tmp/parquet-controls
Apache Arrow (Apache-2.0), DuckDB (MIT): only their public reader APIs are used.
"""
import csv
import hashlib
import io
import json
import math
import sys
import zipfile
from decimal import Decimal
from pathlib import Path
import duckdb
import pyarrow as pa
import pyarrow.parquet as pq

folder = Path(sys.argv[1])
assert pa.__version__ == '26.0.0', pa.__version__
assert duckdb.__version__ == '2.0.0', duckdb.__version__
connection = duckdb.connect()
checks = 0

def read_both(path):
    global checks
    table = pq.read_table(path)
    arrow = table.to_pylist()
    columns = table.column_names
    other = connection.execute('SELECT * FROM read_parquet(?)', [str(path)]).fetchall()
    assert [tuple(row[c] for c in columns) for row in arrow] == other, path
    checks += 1
    return table

control = read_both(folder / 'writer-control.parquet')
expected_counts = [0, 9007199254740993, 9223372036854775807, -9223372036854775808, None] * 2
assert control['count'].to_pylist() == expected_counts
assert control['flag'].to_pylist() == [False, True, None, True, False] * 2
assert control['text'].to_pylist() == ['', 'αβ\t\n"quoted"', None, 'HLA-A*02:01', 'NaN'] * 2
assert math.copysign(1, control['value'][1].as_py()) == -1
assert control.schema.metadata == {b'a': b'first', b'z': b'last'}
assert len(read_both(folder / 'empty-control.parquet')) == 0
assert len(read_both(folder / 'empty-native.parquet')) == 0
boundary = read_both(folder / 'native-count-boundary.parquet')
assert boundary['total'].to_pylist() == [9007199254740991, 0]
assert boundary['sample_000001'].to_pylist() == [9007199254740991, 0]
assert boundary['sample_000002'].to_pylist() == [None, None]
assert len(read_both(folder / 'multiple-groups.parquet')) == 16400
assert pq.ParquetFile(folder / 'multiple-groups.parquet').num_row_groups == 3

def bundle(name, expected_tables=None, private=False):
    global checks
    with zipfile.ZipFile(folder / name) as archive:
        names = archive.namelist()
        assert len(names) == len(set(names))
        record_name = 'transformation.json' if private else 'inventory.json'
        record = json.loads(archive.read(record_name))
        for member in record['files']:
            data = archive.read(member['path'])
            assert len(data) == member['size']
            assert hashlib.sha256(data).hexdigest() == member['sha256']
        schemas = json.loads(archive.read('schema.json'))
        parsed = {}
        for member in names:
            if not member.endswith('.parquet'):
                continue
            # The native writer generates only these known files; no arbitrary archive extraction.
            assert member in ('clonotypes.parquet', 'rearrangements.parquet', 'samples.parquet', 'invariant-samples.parquet')
            local = folder / (name + '-' + member)
            local.write_bytes(archive.read(member))
            table = read_both(local)
            schema = json.loads(table.schema.metadata[b'clonoloom'])
            key = member.removesuffix('.parquet')
            assert schema == schemas[key]
            assert schema['rows'] == len(table)
            assert table.schema.metadata[b'NOTICE'] == archive.read('NOTICE.txt')
            assert table.column_names == [f['name'] for f in schema['fields']]
            # Assert the public native contract independently of the emitted schema: a
            # writer/schema agreeing on the wrong type must not pass this validation.
            if key == 'rearrangements':
                for count in ('duplicate_count', 'umi_count', 'templates'):
                    assert table.schema.field(count).type == pa.int64(), count
                for flag in ('productive', 'vj_in_frame', 'stop_codon'):
                    assert table.schema.field(flag).type == pa.bool_(), flag
            elif key == 'clonotypes':
                for count in ('total', 'samples', *(s['column'] for s in schema['sampleColumns'])):
                    assert table.schema.field(count).type == pa.int64(), count
            for f, field in zip(schema['fields'], table.schema):
                assert field.nullable == f['nullable']
                expected_type = {'INT64': pa.int64(), 'DOUBLE': pa.float64(), 'BOOLEAN': pa.bool_(), 'BYTE_ARRAY': pa.string()}[f['type']]
                assert field.type == expected_type
            if expected_tables and key in expected_tables:
                source = list(csv.reader(io.StringIO((folder / expected_tables[key]).read_text()), delimiter='\t' if key == 'rearrangements' else ','))
                assert len(source) - 1 == len(table)
                assert source[0] == [f['nativeName'] for f in schema['fields']]
                for raw, row in zip(source[1:], table.to_pylist()):
                    for text, f in zip(raw, schema['fields']):
                        expected = None if text == '' else (int(Decimal(text)) if f['type'] == 'INT64' else float(text) if f['type'] == 'DOUBLE' else text == 'T' if f['type'] == 'BOOLEAN' else text)
                        assert row[f['name']] == expected, (name, f, row, expected)
            if private:
                all_values = json.dumps(table.to_pylist(), ensure_ascii=False) + json.dumps(schema) + table.schema.metadata[b'NOTICE'].decode()
                assert 'PRIVATE-' not in all_values
                assert schema['provenance']['storageFormat'] == 'parquet'
                assert 'sources' not in schema['provenance']
            else:
                assert schema['provenance']['deidentified'] is False
                assert schema['provenance']['sources'][0]['sha256']
            parsed[key] = table
        if private:
            assert 'PRIVATE-' not in archive.read('schema.json').decode()
            assert not any('map' in name or 'project' in name for name in names)
            identity = json.loads((folder / 'PRIVATE-map.json').read_text())
            assert identity['bundleFingerprint'] == hashlib.sha256(archive.read('transformation.json')).hexdigest()
            inverse = {b['pseudonym']: b['original'] for b in identity['bindings'] if b['kind'] in ('sequence', 'cell', 'subject', 'sample')}
            ordinary = list(csv.DictReader(io.StringIO((folder / 'expected-rearrangements.txt').read_text()), delimiter='\t'))
            transformed = parsed['rearrangements'].to_pylist()
            assert len(ordinary) == len(transformed)
            for source, target in zip(ordinary, transformed):
                for field in ('sequence_id', 'cell_id'):
                    assert (inverse.get(target[field], '') if target[field] else '') == source[field]
                for field, text in source.items():
                    if field in ('sequence_id', 'cell_id', 'repertoire_id'):
                        continue
                    value = None if text == '' else int(Decimal(text)) if field in ('duplicate_count', 'umi_count', 'templates') else text == 'T' if field in ('productive', 'vj_in_frame', 'stop_codon') else text
                    assert target[field] == value, (field, target[field], value)
            sample_rows = parsed['samples'].to_pylist()
            assert sample_rows[0]['subject_id'] == sample_rows[1]['subject_id']
            assert sample_rows[2]['subject_id'] is None
            assert parsed['clonotypes']['total'].to_pylist() == native_clones['clonotypes']['total'].to_pylist()
        checks += 1
        return parsed

native_clones = bundle('native-clonotypes.zip', {'clonotypes': 'expected-clonotypes.txt'})
bundle('native-rearrangements.zip', {'rearrangements': 'expected-rearrangements.txt'})
bundle('native-invariant.zip', {'clonotypes': 'expected-invariant.txt', 'invariant-samples': 'expected-invariant-samples.txt'})
bundle('deidentified-parquet.zip', private=True)
print(f'Parquet independent validation: {checks} reader/bundle checks passed; PyArrow {pa.__version__}, DuckDB {duckdb.__version__}. Exact integers, nulls, native evidence, schemas and identity separation verified.')
