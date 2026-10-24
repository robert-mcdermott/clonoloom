"""Independently check the artificial B-cell simulation with stdlib CSV and the genetic code.

uv run --no-project validation/reference/bcell_families.py [--study /path/to/study]
Without --study, makes the default preset in a temporary directory with `go run . simulate`.
Checks every ordered event, ancestor, paired cell, supplied alignment/region, probability and
realized count. Writes reduced reference values; no ClonoLoom Python imports or external code.
This validates bookkeeping and sampling, not biological accuracy of the artificial model.
"""
import argparse
from collections import Counter
import csv
import hashlib
from itertools import product
import json
from pathlib import Path
import platform
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[2]
# NCBI standard genetic code, TCAG order. Translation is independent of the Go simulator.
CODE = dict(zip((''.join(c) for c in product('TCAG', repeat=3)),
                'FFLLSSSSYY**CC*WLLLLPPPPHHQQRRRRIIIMTTTTNNKKSSRRVVVVAAAADDEEGGGG'))


def translate(sequence):
    assert len(sequence) % 3 == 0
    return ''.join(CODE[sequence[i:i+3]] for i in range(0, len(sequence), 3))


def rows(path):
    with path.open(newline='') as handle:
        return list(csv.DictReader(handle, delimiter='\t'))


def check(folder):
    truth = json.loads((folder / 'truth/truth.json').read_text())
    assert truth['model'] == 'clonoloom-artificial-bcell-v2'
    config = truth['study']['bcells']
    for file in truth['files']:
        assert hashlib.sha256((folder / file['path']).read_bytes()).hexdigest() == file['sha256']
    nodes = {n['id']: n for n in truth['nodes']}
    assert len(nodes) == len(truth['nodes']) == config['subjects'] * config['families'] * 7
    roots = {n['family']: n for n in nodes.values() if not n['parent']}
    effects = Counter()
    reversals = 0
    for n in nodes.values():
        root = roots[n['family']]
        assert root['subject'] == n['subject']
        parent = nodes[n['parent']] if n['parent'] else None
        for arm in ('heavy', 'light'):
            chain = n[arm]
            assert len(chain['sequence']) == 360 and '*' not in translate(chain['sequence'])
            assert translate(chain['sequence'][282:285]) == 'C'
            assert translate(chain['sequence'][318:321]) == ('W' if arm == 'heavy' else 'F')
            assert chain['vCall'] == root[arm]['vCall'] and chain['jCall'] == root[arm]['jCall']
            if parent is None:
                assert chain['mutations'] == [] and n['isotype'] == 'IGHM'
                continue
            assert parent['family'] == n['family'] and parent['subject'] == n['subject']
            assert ['IGHM', 'IGHG1', 'IGHA1'].index(parent['isotype']) <= ['IGHM', 'IGHG1', 'IGHA1'].index(n['isotype'])
            sequence = list(parent[arm]['sequence'])
            assert len(chain['mutations']) == config['mutationsPerBranch']
            for m in chain['mutations']:
                pos = m['position'] - 1
                assert sequence[pos] == m['from'] != m['to']
                assert pos not in list(range(282, 285)) + list(range(318, 321))
                codon = pos // 3 * 3
                before = translate(''.join(sequence[codon:codon+3]))
                sequence[pos] = m['to']
                after = translate(''.join(sequence[codon:codon+3]))
                assert after != '*'
                assert m['effect'] == ('silent' if before == after else 'replacement')
                effects[m['effect']] += 1
            assert ''.join(sequence) == chain['sequence']
            if n['id'].endswith('-N3'):
                last, first = parent[arm]['mutations'][-1], chain['mutations'][0]
                assert (last['position'], last['from'], last['to']) == (first['position'], first['to'], first['from'])
                reversals += 1
    observed = rows(folder / 'truth/observations.tsv')
    assignments = {(r['sample_id'], r['cell_id']): nodes[r['node_id']] for r in observed}
    assert len(assignments) == len(observed) == config['subjects'] * 3 * config['cellsPerSample']
    realized = Counter((r['sample_id'], r['node_id']) for r in observed)
    repertoires = json.loads((folder / 'repertoires.airr.json').read_text())['Repertoire']
    metadata = {r['repertoire_id']: r for r in repertoires}
    sample_ids = {s['id'] for s in truth['samples']}
    assert len(sample_ids) == len(truth['samples']) == len(repertoires) == config['subjects'] * 3
    assert set(metadata) == sample_ids
    for subject in {n['subject'] for n in nodes.values()}:
        assert sorted(s['day'] for s in truth['samples'] if s['subject'] == subject) == [-7, 14, 28]
    result = []
    for sample in truth['samples']:
        sid = sample['id']
        repertoire = metadata[sid]
        assert repertoire['subject']['subject_id'] == sample['subject']
        assert repertoire['study']['study_id'] == 'clonoloom-simulated-' + truth['model']
        supplied = repertoire['sample'][0]
        assert supplied['sample_id'] == sid and supplied['single_cell'] is True
        assert supplied['collection_time_point_relative'] == sample['day']
        assert supplied['collection_time_point_relative_unit']['label'] == 'day'
        assert supplied['collection_time_point_reference'] == 'simulated immunization'
        assert repertoire['data_processing'][0]['data_processing_files'] == [sid + '.tsv']
        table = rows(folder / f'{sid}.tsv')
        pairs = {}
        loci, isotypes = Counter(), Counter()
        net = 0
        regions = [('fwr1', 1, 78), ('cdr1', 79, 114), ('fwr2', 115, 165), ('cdr2', 166, 195),
                   ('fwr3', 196, 285), ('cdr3', 286, 318), ('fwr4', 319, 360)]
        for row in table:
            n = assignments[sid, row['cell_id']]
            assert n['subject'] == sample['subject']
            arm = 'heavy' if row['locus'] == 'IGH' else 'light'
            chain, ancestor = n[arm], roots[n['family']][arm]
            assert row['sequence'] == row['sequence_alignment'] == chain['sequence']
            assert row['germline_alignment'] == ancestor['sequence']
            assert row['germline_database'] == truth['model']
            assert row['junction'] == chain['sequence'][282:321]
            assert row['junction_aa'] == translate(row['junction'])
            assert row['v_call'] == chain['vCall'] and row['j_call'] == chain['jCall']
            assert row['productive'] == row['vj_in_frame'] == 'T' and row['stop_codon'] == row['rev_comp'] == 'F'
            assert row['repertoire_id'] == sid and row['sequence_id'] == row['cell_id'] + '-' + chain['locus']
            assert all(row[x] == '' for x in ('d_call', 'd_cigar'))
            assert (row['v_cigar'], row['j_cigar']) == ('285M75S', '318S42M')
            assert [int(row[k]) for k in ('v_germline_start', 'v_germline_end', 'j_germline_start', 'j_germline_end')] == [1, 285, 1, 42]
            assert all(x not in row for x in ('clone_id', 'umi_count', 'duplicate_count', 'consensus_count'))
            assert [int(row[k]) for k in ('v_sequence_start', 'v_sequence_end', 'j_sequence_start', 'j_sequence_end')] == [1, 285, 319, 360]
            for region, start, end in regions:
                assert (int(row[region+'_start']), int(row[region+'_end'])) == (start, end)
            constant = n['isotype'] if arm == 'heavy' else {'IGK': 'IGKC', 'IGL': 'IGLC1'}[chain['locus']]
            assert row['c_call'] == constant
            if arm == 'heavy':
                isotypes[constant] += 1
            loci[row['locus']] += 1
            pair = pairs.setdefault(row['cell_id'], {})
            assert arm not in pair
            pair[arm] = (row['locus'], row['v_call'].split('*')[0], row['j_call'].split('*')[0], row['junction_aa'])
            net += sum(a != b for a, b in zip(chain['sequence'], ancestor['sequence']))
        assert len(table) == 2 * sample['cells'] == 2 * config['cellsPerSample']
        assert len(pairs) == sample['cells'] and all(set(p) == {'heavy', 'light'} for p in pairs.values())
        assert {cell for sample_id, cell in assignments if sample_id == sid} == set(pairs)
        assert len(sample['abundance']) == config['families'] * 7
        total_prob = sum(a['probability'] for a in sample['abundance'])
        assert abs(total_prob - 1) < 1e-12
        assert sum(a['cells'] for a in sample['abundance']) == sample['cells']
        # Independent closed-form expectations for the documented family/node weights.
        stage = {-7: 0, 14: 1, 28: 2}[sample['day']]
        state = [[1, 0, 0, 0, 0, 0, 0], [1, 4, 4, 0, 0, 0, 0], [1, 1, 1, 4, 4, 4, 4]][stage]
        expanded = (config['families'] + 3) // 4
        fold = [1, 8, 3][stage]
        denominator = (expanded * fold + config['families'] - expanded) * sum(state)
        for a in sample['abundance']:
            f, state_id = int(a['node'].split('-F')[1].split('-')[0]) - 1, int(a['node'].split('-N')[1])
            expected = state[state_id] * (fold if f < expanded else 1) / denominator
            assert abs(a['probability'] - expected) < 1e-14
            assert a['cells'] == realized[sid, a['node']]
            assert a['probability'] != 0 or a['cells'] == 0
        result.append({'sample': sid, 'cells': len(pairs), 'rows': len(table), 'loci': dict(sorted(loci.items())),
                       'isotypes': dict(sorted(isotypes.items())),
                       'pairedClonotypes': len({(p['heavy'], p['light']) for p in pairs.values()}),
                       'netDifferences': net, 'sha256': hashlib.sha256((folder / f'{sid}.tsv').read_bytes()).hexdigest()})
    return {'python': platform.python_version(), 'method': 'stdlib CSV, sequential event replay, standard genetic code and closed-form sampling probabilities',
            'model': truth['model'], 'seed': truth['study']['seed'], 'config': config,
            'nodes': len(nodes), 'events': dict(sorted(effects.items())), 'deliberateReversals': reversals,
            'samples': result}


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--study', type=Path)
    parser.add_argument('--output', type=Path, default=Path(__file__).with_name('bcell-families.json'))
    args = parser.parse_args()
    with tempfile.TemporaryDirectory(prefix='clonoloom-bcell-reference-') as tmp:
        folder = args.study or Path(tmp) / 'study'
        if args.study is None:
            subprocess.run(['go', 'run', '.', 'simulate', '--preset', 'bcell-immunization', '--quiet', '--out', str(folder)], cwd=ROOT, check=True)
        reference = check(folder)
    args.output.write_text(json.dumps(reference, indent=2) + '\n')
    print(f"Checked {reference['nodes']} nodes and {sum(s['cells'] for s in reference['samples'])} paired cells; wrote {args.output}")
