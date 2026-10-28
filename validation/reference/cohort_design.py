# Independent SQL oracle for subject membership. No ClonoLoom code is imported.
# Run: uv run --no-project validation/reference/cohort_design.py
import itertools
import json
import pathlib
import sqlite3
import sys

connection = sqlite3.connect(':memory:')
connection.execute('create table observations (subject text, level integer)')
cases = []
for assignment in itertools.product(range(3), repeat=8):
    connection.execute('delete from observations')
    connection.executemany('insert into observations values (?, ?)',
                           [(f'S{i // 2}', level) for i, level in enumerate(assignment) if level])
    per_group = dict((level, (n, subjects)) for level, n, subjects in connection.execute(
        'select level, count(*), count(distinct subject) from observations group by level'))
    subjects = connection.execute('select count(distinct subject) from observations').fetchone()[0]
    pairs = connection.execute('''select count(*) from
        (select subject from observations group by subject
         having count(*) = 2 and count(distinct level) = 2)''').fetchone()[0]
    repeated = connection.execute('''select count(*) from
        (select subject, level from observations group by subject, level having count(*) > 1)''').fetchone()[0]
    shared = connection.execute('''select count(*) from
        (select subject from observations group by subject having count(distinct level) = 2)''').fetchone()[0]
    incomplete = subjects - shared
    an, au = per_group.get(1, (0, 0))
    bn, bu = per_group.get(2, (0, 0))
    independent = not repeated and not shared and au >= 2 and bu >= 2
    paired = not repeated and not incomplete and pairs >= 2
    cases.append([''.join(map(str, assignment)), an, bn, au, bu, subjects, pairs, repeated, shared, incomplete, independent, paired])
output = {'tools': {'python': sys.version.split()[0], 'sqlite': sqlite3.sqlite_version}, 'method': 'Independent SQLite GROUP BY over all 3^8 absent/A/B assignments of two observations for each of four subjects.',
          'columns': ['assignment', 'aSamples', 'bSamples', 'aSubjects', 'bSubjects', 'subjects', 'completePairs', 'repeatedGroups', 'sharedSubjects', 'incompleteSubjects', 'independentOK', 'pairedOK'],
          'cases': cases}
pathlib.Path(__file__).with_suffix('.json').write_text(json.dumps(output, separators=(',', ':')) + '\n')
print(f'{len(cases)} designs tabulated independently with SQLite')
