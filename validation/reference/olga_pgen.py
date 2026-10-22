# Generation probabilities as OLGA computes them, for validation/pgen_test.go: OLGA 1.3.0 (GPL-3.0,
# run here as a tool, never copied), under its seven default models, of
#   - the development data's CDR3s: the OTS donors' β and α chains, Dash et al.'s human and mouse
#     clones, the OAS units' heavy and light chains (each once);
#   - CDR3s OLGA draws from each model (productive, numpy seeded): 46,000 human TRB, 20,000 TRA,
#     6,000 IGH, 4,000 IGK and IGL, 5,000 mouse TRB and TRA;
#   - 2,000 OTS β and 500 α junctions as nucleotides;
#   - 1,000 OTS β CDR3s with their V and J genes as masks (the joint probability);
#   - 200 human TRB CDR3s with three positions made X (any amino acid).
# One row each in validation/reference/pgen.tsv.gz (model, aa or nt, CDR3, V mask, J mask, OLGA's
# probability to 15 digits, and OLGA's on the model as shipped where that differs), with the
# models' checksums and counts in pgen.json.
#
# Two things OLGA does are kept out of the reference (validation/README.md):
#   - Making a sequence generator from a model changes the model in place, and the probabilities
#     computed with it after (by up to 6 × 10⁻⁷ for TRB): the generator gets its own copy.
#   - OLGA leaves out an insertion of the longest length a model allows when that length is 1
#     more than a multiple of 3 and the insertion starts a codon (mouse TRA's 22 nucleotides);
#     its generator makes them. So where a model's longest insertion is such a length, OLGA is
#     run on a copy that allows one nucleotide more with probability 10⁻³⁰ (which changes no
#     probability by more than about 10⁻²⁶), and its values on the model as shipped are kept too.
#
#   uv run --with olga==1.3.0 python -I validation/reference/olga_pgen.py
import csv, gzip, hashlib, io, json, os, random, shutil, sys, tempfile
import importlib.metadata as metadata
import numpy as np
import olga
import olga.load_model as load_model
import olga.generation_probability as generation_probability
import olga.sequence_generation as sequence_generation

here = os.path.dirname(os.path.abspath(__file__))
cache = os.environ.get('CLONOLOOM_DATA') or os.path.join(here, '..', 'cache')
models_dir = os.path.join(os.path.dirname(olga.__file__), 'default_models')
MODELS = {'human_T_beta': True, 'human_T_alpha': False, 'human_B_heavy': True, 'human_B_kappa': False, 'human_B_lambda': False, 'mouse_T_beta': True, 'mouse_T_alpha': False}

def realizations(path, event):
    """An IGoR event's realizations' values, by index."""
    out, inside = [], False
    for line in open(path + '/model_params.txt'):
        line = line.rstrip('\n')
        if line.startswith('#'):
            inside = line.endswith(';' + event)
        elif inside and line.startswith('%'):
            value, index = line[1:].split(';')[:2]
            out.append((int(index), int(value)))
    return [v for _, v in sorted(out)]

scratch = tempfile.mkdtemp()

def padded(name):
    """The model's directory, or a copy whose longest insertion is one nucleotide longer (at 1e-30)
    when the longest is 1 more than a multiple of 3."""
    p = os.path.join(models_dir, name)
    lines = open(p + '/model_marginals.txt').read().split('\n')
    event, changed = None, False
    for k, line in enumerate(lines):
        if line.startswith('@'):
            event = line[1:]
        if event in ('vj_ins', 'vd_ins', 'dj_ins') and line.startswith('%'):
            values = realizations(p, event)
            probs = line[1:].split(',')
            longest = max(v for v, q in zip(values, probs) if float(q) > 0)
            if longest % 3 == 1 and longest + 1 in values:
                probs[values.index(longest + 1)] = '1e-30'
                lines[k] = '%' + ','.join(probs)
                changed = True
    if not changed:
        return None
    q = os.path.join(scratch, name)
    shutil.copytree(p, q)
    open(q + '/model_marginals.txt', 'w').write('\n'.join(lines))
    return q

def pgen_of(p, vdj):
    genomic = load_model.GenomicDataVDJ() if vdj else load_model.GenomicDataVJ()
    genomic.load_igor_genomic_data(p + '/model_params.txt', p + '/V_gene_CDR3_anchors.csv', p + '/J_gene_CDR3_anchors.csv')
    generative = load_model.GenerativeModelVDJ() if vdj else load_model.GenerativeModelVJ()
    generative.load_and_process_igor_model(p + '/model_marginals.txt')
    return (generation_probability.GenerationProbabilityVDJ if vdj else generation_probability.GenerationProbabilityVJ)(generative, genomic), genomic, generative

def load(name):
    p = os.path.join(models_dir, name)
    vdj = MODELS[name]
    shipped, genomic, _ = pgen_of(p, vdj)
    q = padded(name)
    pgen = pgen_of(q, vdj)[0] if q else shipped
    _, genomic_gen, generative_gen = pgen_of(p, vdj)
    gen = (sequence_generation.SequenceGenerationVDJ if vdj else sequence_generation.SequenceGenerationVJ)(generative_gen, genomic_gen)
    return pgen, gen, genomic, shipped if q else None

loaded = {name: load(name) for name in MODELS}
rows, seen = [], set()

def add(model, kind, cdr3, v='', j=''):
    key = (model, kind, cdr3, v, j)
    if not cdr3 or key in seen:
        return
    seen.add(key)
    pgen, _, _, shipped = loaded[model]
    def of(p):
        if kind == 'aa':
            return p.compute_aa_CDR3_pgen(cdr3, v or None, j or None, print_warnings=False)
        return p.compute_nt_CDR3_pgen(cdr3, v or None, j or None, print_warnings=False)
    value = of(pgen)
    as_shipped = of(shipped) if shipped else value
    rows.append((model, kind, cdr3, v, j, '%.15e' % value, '' if as_shipped == value else '%.15e' % as_shipped))

def unit(path):
    with gzip.open(os.path.join(cache, path), 'rt', newline='') as f:
        f.readline()
        return list(csv.DictReader(f))

AA = set('ACDEFGHIKLMNPQRSTVWY')
good = lambda s: s and set(s) <= AA

# The development data's CDR3s.
ots = []
for u in ['Sureshchandra_2023/csv_paired/SRR18365371_1_Paired_All.csv.gz', 'Bai_2023/csv_paired/SRR14045264_1_Paired_All.csv.gz', 'Poon_2023/csv_paired/SRR19760479_1_Paired_All.csv.gz']:
    ots += unit('ots-healthy-pbmc/' + u)
for r in ots:
    if good(r['junction_aa_beta']):
        add('human_T_beta', 'aa', r['junction_aa_beta'])
    if good(r['junction_aa_alpha']):
        add('human_T_alpha', 'aa', r['junction_aa_alpha'])
for name, organism in (('dash.csv', 'mouse'), ('dash_human.csv', 'human')):
    with open(os.path.join(cache, 'dash-2017-tcrdist3', name)) as f:
        for r in csv.DictReader(f):
            if good(r['cdr3_b_aa']):
                add(organism + '_T_beta', 'aa', r['cdr3_b_aa'])
            if good(r['cdr3_a_aa']):
                add(organism + '_T_alpha', 'aa', r['cdr3_a_aa'])
for r in unit('oas-units/paired/Eccles_2020/csv/SRR10358524_paired.csv.gz'):
    if good(r['junction_aa_heavy']):
        add('human_B_heavy', 'aa', r['junction_aa_heavy'])
    light = {'K': 'human_B_kappa', 'L': 'human_B_lambda'}.get(r['locus_light'])
    if light and good(r['junction_aa_light']):
        add(light, 'aa', r['junction_aa_light'])
for r in unit('oas-units/unpaired/Bashford_2013/csv/ERR220445_Heavy_IGHG.csv.gz'):
    if good(r['junction_aa']):
        add('human_B_heavy', 'aa', r['junction_aa'])

# CDR3s each model makes.
np.random.seed(20261022)
for name, n in (('human_T_beta', 46000), ('human_T_alpha', 20000), ('human_B_heavy', 6000), ('human_B_kappa', 4000), ('human_B_lambda', 4000), ('mouse_T_beta', 5000), ('mouse_T_alpha', 5000)):
    gen = loaded[name][1]
    for _ in range(n):
        add(name, 'aa', gen.gen_rnd_prod_CDR3()[1])

# Nucleotides; masks; ambiguous residues.
nt_beta = [r['junction_beta'].upper() for r in ots if r['junction_beta'] and len(r['junction_beta']) % 3 == 0 and set(r['junction_beta'].upper()) <= set('ACGT')]
nt_alpha = [r['junction_alpha'].upper() for r in ots if r['junction_alpha'] and len(r['junction_alpha']) % 3 == 0 and set(r['junction_alpha'].upper()) <= set('ACGT')]
for s in nt_beta[:2000]:
    add('human_T_beta', 'nt', s)
for s in nt_alpha[:500]:
    add('human_T_alpha', 'nt', s)
genomic = loaded['human_T_beta'][2]
vs = {g[0].split('*')[0] for g in genomic.genV}
js = {g[0].split('*')[0] for g in genomic.genJ}
masked = 0
for r in ots:
    v, j = r['v_call_beta'].split('*')[0], r['j_call_beta'].split('*')[0]
    if masked < 1000 and good(r['junction_aa_beta']) and v in vs and j in js and ',' not in v + j:
        before = len(rows)
        add('human_T_beta', 'aa', r['junction_aa_beta'], v, j)
        masked += len(rows) - before
rng = random.Random(20261022)
for r in [x for x in rows if x[0] == 'human_T_beta' and x[1] == 'aa' and not x[3]][:200]:
    s = list(r[2])
    for i in rng.sample(range(3, len(s) - 2), 3) if len(s) > 8 else []:
        s[i] = 'X'
    add('human_T_beta', 'aa', ''.join(s))

buf = io.BytesIO()
with gzip.GzipFile(fileobj=buf, mode='wb', mtime=0) as z:
    z.write(('\n'.join('\t'.join(r) for r in rows) + '\n').encode())
open(os.path.join(here, 'pgen.tsv.gz'), 'wb').write(buf.getvalue())
shutil.rmtree(scratch)
checksums = {}
for name in MODELS:
    for f in ('model_params.txt', 'model_marginals.txt', 'V_gene_CDR3_anchors.csv', 'J_gene_CDR3_anchors.csv'):
        checksums[f'{name}/{f}'] = hashlib.sha256(open(os.path.join(models_dir, name, f), 'rb').read()).hexdigest()
counts = {}
for r in rows:
    counts[r[0] + ' ' + r[1] + (' masked' if r[3] else '')] = counts.get(r[0] + ' ' + r[1] + (' masked' if r[3] else ''), 0) + 1
padded_models = sorted(name for name in MODELS if loaded[name][3])
json.dump({'made': f"olga {metadata.version('olga')}, numpy {metadata.version('numpy')}", 'file': 'pgen.tsv.gz', 'sha256': hashlib.sha256(buf.getvalue()).hexdigest(),
           'rows': len(rows), 'counts': counts, 'models': checksums, 'padded': padded_models,
           'shipped differs': sum(1 for r in rows if r[6])}, open(os.path.join(here, 'pgen.json'), 'w'), indent=1, sort_keys=True)
print(len(rows), 'rows', counts, file=sys.stderr)
