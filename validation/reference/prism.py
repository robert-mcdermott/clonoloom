"""Independent JSON/Decimal/XML audit of native subject-table Prism exports.
Use uv run --no-project prism.py DIRECTORY [--schema /path/to/Prism8XMLSchema.xsd] [--r].
The optional schema is GraphPad's published format specification, kept outside the repo.
"""
import argparse
from decimal import Decimal
import hashlib
import json
from pathlib import Path
import subprocess
import tempfile
import xml.etree.ElementTree as ET
import zipfile

class Number(str):
    pass

def read(raw):
    return json.loads(raw, parse_int=Number, parse_float=Number)

def pointer(doc, path):
    for key in path.split('/')[1:]:
        key = key.replace('~1', '/').replace('~0', '~')
        doc = doc[int(key)] if isinstance(doc, list) else doc[key]
    return doc

def audit(folder, schema=None, r=False):
    count = tables = missing = structural = 0
    schema_check = None
    if schema:
        from lxml import etree
        schema_check = etree.XMLSchema(etree.parse(schema))
    with tempfile.TemporaryDirectory(prefix='clonoloom-prism-reference-') as tmp:
        tmp = Path(tmp)
        for bundle in sorted(Path(folder).glob('*.zip')):
            with zipfile.ZipFile(bundle) as archive:
                record = json.loads(archive.read('prism.json'))
                assert record['format'] == 'clonoloom-prism' and record['version'] == 1
                for entry in record['files']:
                    raw = archive.read(entry['path'])
                    assert len(raw) == entry['size'] and hashlib.sha256(raw).hexdigest() == entry['sha256']
                source = read(archive.read('result.json'))
                mapping = read(archive.read('mapping.json'))
                feature = source['features'] if record['kind'] == 'cohort-compare' else source
                design = feature['design']; spec = design['input']['spec']
                assert spec['unit'] == mapping['unit'] == 'subject'
                assert spec['layout'] == mapping['layout']
                assert spec['levels'] == mapping['levels']
                included = {row['name']: row for row in design['rows'] if row['included']}
                assert mapping['excluded'] == [row for row in design['rows'] if not row['included']]
                subjects = sorted({row['subject'] for row in included.values()})
                fields = {f['id']: (i, f) for i, f in enumerate(feature['table']['catalog'])}
                measurements = {(row['subject'], row['group']): row for row in feature['table']['rows']}
                assert len(measurements) == len(included)
                for row in measurements.values():
                    assert included[row['name']]['subject'] == row['subject']
                    assert included[row['name']]['group'] == row['group']
                ids = source['analysis']['options']['features'] if record['kind'] == 'cohort-compare' else list(fields)
                raw = archive.read('data.pzfx')
                if schema_check:
                    schema_check.assertValid(etree.fromstring(raw))
                root = ET.fromstring(raw)
                ns = {'p': 'http://graphpad.com/prism/Prism.htm'}
                assert root.tag == '{http://graphpad.com/prism/Prism.htm}GraphPadPrismFile'
                assert not any(e.tag.split('}')[-1] in ('Script', 'Template', 'Analysis', 'Graph', 'XColumn') for e in root.iter())
                xml_tables = root.findall('p:Table', ns)
                assert len(xml_tables) == len(mapping['tables']) == len(ids)
                assert [e.attrib['ID'] for e in root.findall('p:TableSequence/p:Ref', ns)] == [e.attrib['ID'] for e in xml_tables]
                expected_r = []
                for table, xml_table in zip(mapping['tables'], xml_tables):
                    fid = table['feature']['id']; assert fid in ids
                    col, definition = fields[fid]; assert table['feature'] == definition
                    assert table['id'] == xml_table.attrib['ID']
                    assert xml_table.attrib['TableType'] == 'OneWay' and xml_table.attrib['Replicates'] == '1'
                    assert xml_table.attrib['XFormat'] == 'none'
                    assert [e.text for e in xml_table.findall('p:RowTitlesColumn/p:Subcolumn/p:d', ns)] == subjects
                    assert [row['subject'] for row in table['rows']] == subjects
                    columns = xml_table.findall('p:YColumn', ns)
                    assert [c.findtext('p:Title', namespaces=ns) for c in columns] == spec['levels']
                    expected_columns = []
                    for g, column in enumerate(columns):
                        assert len(column.findall('p:Subcolumn', ns)) == 1
                        cells = column.findall('p:Subcolumn/p:d', ns); assert len(cells) == len(subjects)
                        values = []
                        for subject, row, cell in zip(subjects, table['rows'], cells):
                            assert cell.attrib.get('Excluded', '0') == '0'
                            metadata = row['cells'][g]
                            expected = measurements.get((subject, spec['levels'][g]))
                            if expected is None:
                                assert spec['layout'] == 'independent'
                                assert metadata == {'state': 'structural-empty'} and cell.text is None
                                structural += 1; values.append(None); continue
                            observed = expected['values'][col]
                            assert metadata['sample'] == expected['name']
                            assert pointer(source, metadata['pointer']) == observed['value'] == metadata['value']
                            count += 1
                            if observed['value'] is None:
                                assert metadata['state'] == 'missing' and metadata['missing'] == observed['missing'] and cell.text is None
                                missing += 1; values.append(None)
                            else:
                                assert metadata['state'] == 'observed' and not observed['missing']
                                assert cell.text == observed['value']  # lexical token, not a float comparison
                                assert Decimal(cell.text).is_finite()
                                values.append(str(observed['value']))
                        expected_columns.append(values)
                    expected_r.append({'title': table['title'], 'subjects': subjects, 'groups': spec['levels'], 'values': expected_columns})
                    tables += 1
                target = tmp / bundle.stem; target.mkdir()
                (target / 'data.pzfx').write_bytes(raw)
                (target / 'expected.json').write_text(json.dumps(expected_r))
                print(f'{bundle.name}: exact tokens, subjects, group order, structural blanks, missingness, exclusions and hashes agree.')
        assert tables > 0 and missing > 0 and structural > 0
        if r:
            subprocess.run(['Rscript', str(Path(__file__).with_suffix('.R')), str(tmp)], check=True)
    print(f'{tables} tables; {count} declared subject/group measurements; {missing} missing measurements; {structural} structural blanks.')

if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('folder'); parser.add_argument('--schema'); parser.add_argument('--r', action='store_true')
    args = parser.parse_args(); audit(args.folder, args.schema, args.r)
