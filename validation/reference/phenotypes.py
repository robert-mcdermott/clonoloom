# Independent SQL tabulation of artificial TRB clonotypes, distinct-cell sizes and phenotypes.
# Uses only Python's SQLite public API, not ClonoLoom code or result files.
# Run: uv run --no-project validation/reference/phenotypes.py
import csv
import hashlib
import json
from pathlib import Path
import sqlite3

root = Path(__file__).resolve().parents[2] / 'testdata' / 'phenotypes'
db = sqlite3.connect(':memory:')
db.row_factory = sqlite3.Row
db.executescript('''
CREATE TABLE receptor(sample TEXT, barcode TEXT, clone TEXT, productive INTEGER);
CREATE TABLE cell(sample TEXT, barcode TEXT, value TEXT, mapped INTEGER);
''')
sources = {}
inputs = {}
for sample in ('S1', 'S2'):
    for name in (f'{sample}.tsv', f'{sample}-cells.tsv'):
        sources[name] = hashlib.sha256((root / name).read_bytes()).hexdigest()
    with (root / f'{sample}.tsv').open() as f:
        for row in csv.DictReader(f, delimiter='\t'):
            db.execute('INSERT INTO receptor VALUES(?,?,?,?)', (sample, row['cell_id'], row['junction_aa'], row['productive'] == 'T'))
    inputs[sample] = []
    with (root / f'{sample}-cells.tsv').open() as f:
        for row in csv.DictReader(f, delimiter='\t'):
            mapped = row['map_x'] != ''
            db.execute('INSERT INTO cell VALUES(?,?,?,?)', (sample, row['barcode'], row['state'], mapped))
            inputs[sample].append({'barcode': row['barcode'], 'values': [row['state']], 'position': [float(row['map_x']), float(row['map_y'])] if mapped else None})
db.executescript('''
CREATE VIEW assigned AS SELECT DISTINCT sample,barcode,clone FROM receptor WHERE productive=1;
CREATE VIEW linked AS SELECT c.*,a.clone FROM cell c LEFT JOIN assigned a USING(sample,barcode);
CREATE VIEW clones AS SELECT a.sample,a.clone,COUNT(DISTINCT a.barcode) AS receptor,
 COUNT(DISTINCT c.barcode) AS expression,COUNT(DISTINCT CASE WHEN c.mapped=1 THEN c.barcode END) AS mapped
 FROM assigned a LEFT JOIN cell c USING(sample,barcode) GROUP BY a.sample,a.clone;
''')
results = []
for sample in inputs:
    for population in ('receptor', 'mapped'):
        # Column is one of the two fixed SQL identifiers above, never user data.
        rows = db.execute(f'''SELECT l.barcode,l.clone,CASE WHEN k.{population}>0 THEN k.{population} END AS size,
         CASE WHEN k.{population}=1 THEN '1' WHEN k.{population}=2 THEN '2'
         WHEN k.{population} BETWEEN 3 AND 5 THEN '3–5' WHEN k.{population}>5 THEN '>5' ELSE '' END AS bin
         FROM linked l LEFT JOIN clones k USING(sample,clone) WHERE l.sample=? ORDER BY l.barcode''', (sample,)).fetchall()
        groups = db.execute(f'''SELECT l.value,COUNT(*) AS cells,SUM(l.mapped) AS mapped,COUNT(l.clone) AS assigned,
         SUM(CASE WHEN k.{population}>0 THEN 1 ELSE 0 END) AS sized,
         SUM(CASE WHEN k.{population}>1 THEN 1 ELSE 0 END) AS expanded,COUNT(DISTINCT l.clone) AS clonotypes
         FROM linked l LEFT JOIN clones k USING(sample,clone) WHERE l.sample=? GROUP BY l.value ORDER BY l.value''', (sample,)).fetchall()
        associations = db.execute('''SELECT l.clone,l.value,COUNT(*) AS cells,SUM(l.mapped) AS mapped,
         k.expression AS cloneExpression,(SELECT COUNT(*) FROM cell c WHERE c.sample=l.sample AND c.value=l.value) AS groupExpression
         FROM linked l JOIN clones k USING(sample,clone) WHERE l.sample=? GROUP BY l.clone,l.value ORDER BY l.clone,l.value''', (sample,)).fetchall()
        results.append({'sample': sample, 'population': population, 'cells': [dict(r) for r in rows], 'groups': [dict(r) for r in groups], 'associations': [dict(r) for r in associations]})
out = {'generatedBy': f'SQLite {sqlite3.sqlite_version}; distinct-cell joins and GROUP BY', 'sources': sources, 'inputs': inputs, 'results': results}
Path(__file__).with_suffix('.json').write_text(json.dumps(out, indent=2, sort_keys=True) + '\n')
