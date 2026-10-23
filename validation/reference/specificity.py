"""Independent pandas lookups, without importing a reference tool's source.

uv run --with pandas==2.3.3 validation/reference/specificity.py
For maintainer-only real data (never write its output into the repository):
uv run --with pandas==2.3.3 validation/reference/specificity.py --database /path/vdjdb.txt --output /tmp/vdjdb-lookup.json
"""
import argparse
import csv
import hashlib
import json
from pathlib import Path
import pandas as pd

parser = argparse.ArgumentParser()
parser.add_argument('--database', type=Path)
parser.add_argument('--output', type=Path)
args = parser.parse_args()
root = Path(__file__).resolve().parents[1]
real = args.database is not None
path = args.database or root / 'cache/simulated-vaccine/database/vdjdb.txt'
output = args.output or Path(__file__).with_name('specificity.json')
if real and (args.output is None or output.resolve().is_relative_to(root.parent)):
    raise SystemExit('Real database results must stay outside the repository; pass --output /tmp/…')
db = pd.read_csv(path, sep='\t', dtype=str, keep_default_na=False, quoting=csv.QUOTE_NONE)
db['row'] = db.index + 1
for col in ['v.segm', 'j.segm']:
    db[col] = db[col].str.split('*').str[0]
if real:
    queries = db[['gene', 'cdr3', 'v.segm', 'j.segm']].drop_duplicates().iloc[::101].head(1000)
else:
    q = pd.read_csv(root / 'cache/simulated-vaccine/S01-d14.tsv', sep='\t', dtype=str, keep_default_na=False)
    q = q[q.productive == 'T']
    queries = q[['locus', 'junction_aa', 'v_call', 'j_call']].drop_duplicates()
    queries.columns = ['gene', 'cdr3', 'v.segm', 'j.segm']
    # Keep every positive and a deterministic selection of negative queries.
    queries = queries[queries.cdr3.isin(db.cdr3) | (queries.index % 101 == 0)]
results = []
for genes in [False, True]:
    for score in [0, 2]:
        reference = db[(db.species == 'HomoSapiens') & (db['vdjdb.score'].astype(int) >= score)]
        keys = ['gene', 'cdr3'] + (['v.segm', 'j.segm'] if genes else [])
        grouped = reference.groupby(keys, sort=False)['row'].apply(list).to_dict()
        for q in queries.to_dict('records'):
            key = tuple(q[k] for k in keys)
            results.append(dict(locus=q['gene'], cdr3=q['cdr3'], v=q['v.segm'], j=q['j.segm'], genes=genes, score=score, rows=grouped.get(key, [])))
result = dict(generatedBy=f'pandas {pd.__version__}', sha256=hashlib.sha256(path.read_bytes()).hexdigest(), queries=results)
output.write_text(json.dumps(result, indent=2) + '\n')
print(f'{len(db)} database rows; {len(results)} lookups; {sum(bool(q["rows"]) for q in results)} positive queries')

if not real:
    truth = pd.read_csv(root / 'cache/simulated-vaccine/truth/specificity.tsv', sep='\t', dtype=str, keep_default_na=False)
    truth_keys = {}
    for row in truth.to_dict('records'):
        truth_keys.setdefault((row['subject'], row['v_call'], row['j_call'], row['junction_aa']), set()).add(row['epitope'])
    lookup = db.groupby(['v.segm', 'j.segm', 'cdr3'])['antigen.epitope'].apply(set).to_dict()
    shares = []
    for sample in sorted((root / 'cache/simulated-vaccine').glob('S*-*.tsv')):
        q = pd.read_csv(sample, sep='\t', dtype=str, keep_default_na=False)
        q = q[(q.productive == 'T') & (q.junction_aa != '')]
        total = int(q.umi_count.astype(int).sum())
        subject = sample.stem.split('-')[0]
        matched = {}; true_matched = {}; true_counts = {}
        for r in q.to_dict('records'):
            key = (r['v_call'], r['j_call'], r['junction_aa'])
            evidence = lookup.get(key, set())
            actual = truth_keys.get((subject,) + key, set())
            count = int(r['umi_count'])
            for ep in actual: true_counts[ep] = true_counts.get(ep, 0) + count
            for ep in evidence:
                matched[ep] = matched.get(ep, 0) + count
                if ep in actual: true_matched[ep] = true_matched.get(ep, 0) + count
        for ep in sorted(set(db['antigen.epitope']) | set(true_counts)):
            shares.append(dict(sample=sample.stem, epitope=ep, total=total, matched=matched.get(ep, 0), trueMatched=true_matched.get(ep, 0), truth=true_counts.get(ep, 0)))
    output.with_name('specificity-shares.json').write_text(json.dumps(shares, indent=2) + '\n')
    print(f'{len(shares)} sample/epitope shares; matched counts {sum(s["matched"] for s in shares)}, truth-supported matches {sum(s["trueMatched"] for s in shares)}, all truth counts {sum(s["truth"] for s in shares)}')
