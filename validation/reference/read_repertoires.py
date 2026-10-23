# The development files as independent readers see them, for validation/readers_test.go: AIRR
# files read by the AIRR Community's own library (airr); Cell Ranger contig annotations, immunoSEQ
# exports, MiXCR clone tables, OAS and OTS data units and tcrdist3 clone tables read by pandas;
# each column ClonoLoom reads reduced to its row count and a SHA-256 digest
# (validation/reference/readers.json), and a data unit's metadata kept as given.
#
#   uv run --with airr --with pandas python -I validation/reference/read_repertoires.py
#
# A column's digest: each row's value as text, rows in file order joined by "\n", a missing value
# written as "\x00". Booleans are T or F; counts are written as integers when integral; junctions
# in upper case. Where a format leaves a choice to the reader, the choice ClonoLoom documents is
# made here from the raw columns (clonoloom-spec/research.md §1): AIRR counts are umi_count, else
# duplicate_count, else consensus_count; an immunoSEQ file is counted in templates when every row
# has a positive template count, else in reads. Gene names of immunoSEQ files are kept as Adaptive
# writes them (the resolved name, or the ties when the gene is not resolved): the test maps them
# with the dataset's own record of names. A row's locus is the file's locus column, else the locus
# its J call names (its first three letters), else its V call's, else its C call's. The readers of
# wave 4 make the choices their files document (internal/reads: mixcr.go, oas.go, tcrdist3.go):
# MiXCR's calls are the best-scoring hits without the allele *00; a paired unit's cell is its
# contigs' barcode; a tcrdist3 clone of n cells is n cells.
import csv, gzip, hashlib, json, math, os, sys
import airr
import pandas as pd

here = os.path.dirname(os.path.abspath(__file__))
cache = os.environ.get('CLONOLOOM_DATA') or os.path.join(here, '..', 'cache')
MISSING = '\x00'
LOCI = {'IGH', 'IGK', 'IGL', 'TRA', 'TRB', 'TRD', 'TRG'}

def digest(values):
    h = hashlib.sha256()
    h.update('\n'.join(MISSING if v is None else v for v in values).encode())
    return h.hexdigest()

def number(v):
    if v is None or v == '' or (isinstance(v, float) and math.isnan(v)):
        return None
    f = float(v)
    if f < 0:
        return None
    return str(int(f)) if f == int(f) else repr(f)

def flag(v):
    if v is None or v == '':
        return None
    if isinstance(v, bool):
        return 'T' if v else 'F'
    s = str(v).lower()
    return {'true': 'T', 't': 'T', '1': 'T', 'false': 'F', 'f': 'F', '0': 'F'}.get(s)

def text(v, nulls=('',)):
    if v is None or (isinstance(v, float) and math.isnan(v)):
        return None
    s = str(v).strip()
    return None if s in nulls else s

def upper(v, nulls=('',)):
    s = text(v, nulls)
    return s.upper() if s is not None else None

def locus_of(given, *calls):
    if given in LOCI:
        return given
    for call in calls:
        # An IMGT name: a locus, then its segment (V, D, J, C; the heavy chain's constant genes
        # are named by isotype, IGHM, IGHG1…).
        if call and call[:3].upper() in LOCI and len(call) > 3 and (call[3].upper() in 'VDJC' or (call[:3].upper() == 'IGH' and call[3].upper() in 'MGAE')):
            loci = {c.strip()[:3].upper() for c in call.split(',') if c.strip()}
            if len(loci) == 1:
                return loci.pop()
    return None

def airr_file(path):
    reader = airr.read_rearrangement(path)
    fields = reader.fields
    rows = list(reader)
    # A row's count: its UMIs, else its duplicates, else its consensus reads, whichever it has.
    has_counts = any(c in fields for c in ('umi_count', 'duplicate_count', 'consensus_count'))
    def count_of(r):
        for c in ('umi_count', 'duplicate_count', 'consensus_count'):
            if c in fields and number(r.get(c)) is not None:
                return number(r.get(c))
        return None if has_counts else '1'
    def reads(r):
        if 'umi_count' in fields and number(r.get('umi_count')) is not None:
            order = ('consensus_count', 'duplicate_count')
        elif 'duplicate_count' in fields and number(r.get('duplicate_count')) is not None:
            order = ('consensus_count',)
        else:
            order = ('consensus_count',)
        for c in order:
            if c in fields and number(r.get(c)) is not None:
                return number(r.get(c))
        return None
    columns = {
        'sequence_id': [text(r.get('sequence_id')) for r in rows],
        'locus': [locus_of(upper(r.get('locus')), text(r.get('j_call')), text(r.get('v_call')), text(r.get('c_call'))) for r in rows],
        'v_call': [text(r.get('v_call')) for r in rows],
        'd_call': [text(r.get('d_call')) for r in rows],
        'j_call': [text(r.get('j_call')) for r in rows],
        'c_call': [text(r.get('c_call')) for r in rows],
        'junction': [upper(r.get('junction')) for r in rows],
        'junction_aa': [upper(r.get('junction_aa')) for r in rows],
        'productive': [flag(r.get('productive')) for r in rows],
        'cell': [text(r.get('cell_id')) for r in rows],
        'clone': [text(r.get('clone_id')) for r in rows],
        'count': [count_of(r) for r in rows],
        'reads': [reads(r) for r in rows],
    }
    return {'format': 'airr', 'reader': f'airr {airr.__version__}', 'rows': len(rows), 'columns': {k: digest(v) for k, v in columns.items()}}

def cellranger_file(path):
    df = pd.read_csv(path, dtype=str, keep_default_na=False)
    none = ('', 'None', 'none', 'NA')
    get = lambda c: df[c].tolist() if c in df.columns else [None] * len(df)
    columns = {
        'sequence_id': [text(v) for v in get('contig_id')],
        'cell': [text(v) for v in get('barcode')],
        'locus': [locus_of(upper(c, none), text(j, none), text(v, none), text(k, none)) for c, j, v, k in zip(get('chain'), get('j_gene'), get('v_gene'), get('c_gene'))],
        'v_call': [text(v, none) for v in get('v_gene')],
        'd_call': [text(v, none) for v in get('d_gene')],
        'j_call': [text(v, none) for v in get('j_gene')],
        'c_call': [text(v, none) for v in get('c_gene')],
        'junction': [upper(v, none) for v in get('cdr3_nt')],
        'junction_aa': [upper(v, none) for v in get('cdr3')],
        'productive': [flag(v) for v in get('productive')],
        'clone': [text(v, none) for v in get('raw_clonotype_id')],
        'count': [number(v) for v in get('umis')],
        'reads': [number(v) for v in get('reads')],
        'is_cell': [flag(v) for v in get('is_cell')],
        'high_confidence': [flag(v) for v in get('high_confidence')],
        'full_length': [flag(v) for v in get('full_length')],
    }
    if 'sample' in df.columns:
        columns['sample'] = [text(v) for v in get('sample')]
    return {'format': 'cellranger-contigs', 'reader': f'pandas {pd.__version__}', 'rows': len(df), 'columns': {k: digest(v) for k, v in columns.items()}}

ADAPTIVE_NULLS = ('', 'null', 'NULL', 'na', 'NA', 'unresolved', 'unknown')

def immunoseq_file(path):
    df = pd.read_csv(path, sep='\t', dtype=str, keep_default_na=False)
    v3 = 'rearrangement' in df.columns
    col = (lambda a, b: a if v3 else b)
    get = lambda c: df[c].tolist() if c in df.columns else [''] * len(df)
    templates = [number(text(v, ADAPTIVE_NULLS)) for v in get(col('templates', 'estimatedNumberGenomes'))]
    reads_col = next((c for c in ('seq_reads', 'reads') if c in df.columns), None) if v3 else None
    reads = [number(text(v, ADAPTIVE_NULLS)) for v in (get(reads_col) if reads_col else [''] * len(df))]
    all_templates = all(t is not None and float(t) > 0 for t in templates)
    count = templates if all_templates else reads
    junctions = []
    cdr3 = get('cdr3_rearrangement') if v3 else [''] * len(df)
    seqs, starts, lengths = get(col('rearrangement', 'nucleotide')), get(col('v_index', 'vIndex')), get(col('cdr3_length', 'cdr3Length'))
    for c, s, i, n in zip(cdr3, seqs, starts, lengths):
        c = text(c, ADAPTIVE_NULLS)
        if c:
            junctions.append(c.upper())
            continue
        try:
            i, n = int(i), int(n)
        except ValueError:
            junctions.append(None)
            continue
        s = text(s, ADAPTIVE_NULLS) or ''
        junctions.append(s[i:i + n].upper() if i >= 0 and n > 0 and i + n <= len(s) else None)
    frame = {'in': 'T', 'out': 'F', 'stop': 'F'}
    def calls(segment):
        if v3:
            resolved, gene, ties = get(f'{segment}_resolved'), get(f'{segment}_gene'), get(f'{segment}_gene_ties')
        else:
            resolved, gene, ties = get(f'{segment}MaxResolved'), get(f'{segment}GeneName'), get(f'{segment}GeneNameTies')
        out = []
        for r, g, t in zip(resolved, gene, ties):
            r, g, t = text(r, ADAPTIVE_NULLS), text(g, ADAPTIVE_NULLS), text(t, ADAPTIVE_NULLS)
            out.append(t if (g is None and t) else r)
        return out
    raw = {s: calls(s) for s in ('v', 'd', 'j')}
    columns = {
        'junction': junctions,
        'junction_aa': [upper(v, ADAPTIVE_NULLS) for v in get(col('amino_acid', 'aminoAcid'))],
        'productive': [frame.get((text(v, ADAPTIVE_NULLS) or '').lower()) for v in get(col('frame_type', 'sequenceStatus'))],
        'count': count,
        'reads': reads,
    }
    names = {}
    for segment, values in raw.items():
        tally = {}
        for v in values:
            if v is not None:
                tally[v] = tally.get(v, 0) + 1
        names[segment] = dict(sorted(tally.items()))
    return {'format': 'immunoseq-v3' if v3 else 'immunoseq-v2', 'reader': f'pandas {pd.__version__}', 'rows': len(df),
            'unit': 'templates' if all_templates else 'reads', 'columns': {k: digest(v) for k, v in columns.items()}, 'gene_names': names}

def sample_of(path):
    # A sample's name from its file's: without the extensions of tables and compression.
    name = os.path.basename(path)
    while True:
        stem, ext = os.path.splitext(name)
        if ext.lower() in ('.gz', '.tsv', '.csv', '.txt', '.tab', '.airr'):
            name = stem
            continue
        return name

def mixcr_call(hits):
    # The hits of the best score, each without its score and without MiXCR's allele *00.
    hits = text(hits)
    if not hits:
        return None
    parsed = []
    for part in hits.split(','):
        part = part.strip()
        if not part:
            continue
        name, score = part, float('-inf')
        if part.endswith(')') and part.rfind('(') > 0:
            try:
                name, score = part[:part.rfind('(')].strip(), float(part[part.rfind('(') + 1:-1])
            except ValueError:
                name, score = part, float('-inf')
        if name.endswith('*00'):
            name = name[:-3]
        parsed.append((name, score))
    best = max(s for _, s in parsed)
    names = []
    for name, score in parsed:
        if score == best and name not in names:
            names.append(name)
    return ','.join(names) or None

def mixcr_file(path):
    # MiXCR's clone tables: its 3 and 4 namings (cloneCount, readCount), the best hits, the
    # junction's amino acids dropped when out of frame (_), molecules as the count when every clone
    # has them, else reads; a single-cell export's rows are its cells' chains.
    df = pd.read_csv(path, sep='\t', dtype=str, keep_default_na=False)
    get = lambda *names: next((df[c].tolist() for c in names if c in df.columns), [''] * len(df))
    reads = [number(v) for v in get('readCount', 'cloneCount', 'Clone count')]
    molecules = [number(v) for v in get('uniqueMoleculeCount', 'uniqueTagCountMolecule')]
    cells = [text(v) for v in get('tagValueCELL')]
    single = any(cells)
    count = molecules if all(m is not None for m in molecules) else reads
    seg = lambda s: [mixcr_call(v) for v in get(f'all{s}HitsWithScore', f'all{s}Hits', f'best{s}Hit', f'best{s}Gene', f'All {s} hits')]
    v, d, j, c = seg('V'), seg('D'), seg('J'), seg('C')
    nt = [upper(x) for x in get('nSeqCDR3', 'N. Seq. CDR3')]
    aa_raw = [upper(x) for x in get('aaSeqCDR3', 'AA. Seq. CDR3')]
    junction_aa, productive = [], []
    for n, a in zip(nt, aa_raw):
        if a is None:
            junction_aa.append(None); productive.append(None)
        elif '_' in a or (n and len(n) % 3):
            junction_aa.append(None); productive.append('F')
        else:
            junction_aa.append(a); productive.append('F' if '*' in a else 'T')
    chains = [upper(x) for x in get('topChains')]
    ids = [text(x) for x in get('cloneId', 'Clone ID')]
    base = sample_of(path)
    for sep in ('.clones_', '.clonotypes.'):
        i = base.rfind(sep)
        if i > 0 and (base[i + len(sep):] == 'ALL' or base[i + len(sep):] in LOCI):
            base = base[:i]
            break
    columns = {
        'sequence_id': [f'{cell}_clone_{i}' if cell else i for cell, i in zip(cells, ids)],
        'locus': [ch if ch in LOCI else locus_of(None, jj, vv, cc) for ch, jj, vv, cc in zip(chains, j, v, c)],
        'v_call': v, 'd_call': d, 'j_call': j, 'c_call': c,
        'junction': nt, 'junction_aa': junction_aa, 'productive': productive,
        'count': count, 'reads': reads,
        'sample': [base] * len(df),
    }
    if single:
        columns['cell'] = cells
        columns['clone'] = [text(x) if cell else None for x, cell in zip(get('cellGroup'), cells)]
        columns['is_cell'] = ['T' if cell else None for cell in cells]
    unit = 'cells' if single else ('umis' if all(m is not None for m in molecules) and molecules else 'reads')
    return {'format': 'mixcr-clones', 'reader': f'pandas {pd.__version__}', 'rows': len(df), 'unit': unit, 'columns': {k: digest(v) for k, v in columns.items()}}

UNIT_LOCI = {'H': 'IGH', 'K': 'IGK', 'L': 'IGL', 'B': 'TRB', 'A': 'TRA', 'G': 'TRG', 'D': 'TRD'}

def unit_file(path):
    # An OAS or OTS data unit: its first line the metadata (one CSV field holding JSON; a zero-width
    # space a link holds dropped), then IgBLAST's AIRR columns; a paired unit's columns once per
    # chain, its rows cells.
    with gzip.open(path, 'rt', newline='') as f:
        meta = json.loads(next(csv.reader([f.readline()]))[0])
        df = pd.read_csv(f, dtype=str, keep_default_na=False)
    suffixes = [c[len('v_call_'):] for c in df.columns if c.startswith('v_call_')]
    paired = 'v_call' not in df.columns and len(suffixes) == 2
    sample = sample_of(path)
    columns = {k: [] for k in ('sequence_id', 'locus', 'v_call', 'd_call', 'j_call', 'junction', 'junction_aa', 'productive', 'count', 'sample')}
    if paired:
        columns.update({'cell': [], 'is_cell': []})
    rows = 0
    for line, r in enumerate(df.to_dict('records'), start=3):
        chains = suffixes if paired else ['']
        col = lambda name, s: r.get(f'{name}_{s}' if s else name, '')
        if paired:
            ids = [text(col('sequence_id', s)) for s in chains]
            ids = [i for i in ids if i]
            stems = [i[:i.find('_contig')] if i.find('_contig') > 0 else None for i in ids]
            cell = stems[0] if stems and None not in stems and len(set(stems)) == 1 else f'row {line}'
        for s in chains:
            v, d, j = text(col('v_call', s)), text(col('d_call', s)), text(col('j_call', s))
            nt, aa = upper(col('junction', s)), upper(col('junction_aa', s))
            if paired and not (v or j or nt or aa):
                continue
            rows += 1
            columns['sequence_id'].append(text(col('sequence_id', s)))
            columns['locus'].append(locus_of(UNIT_LOCI.get((text(col('locus', s)) or '').upper(), (text(col('locus', s)) or '').upper()), j, v, None))
            columns['v_call'].append(v); columns['d_call'].append(d); columns['j_call'].append(j)
            columns['junction'].append(nt); columns['junction_aa'].append(aa)
            columns['productive'].append(flag(col('productive', s)))
            if paired:
                columns['count'].append('1'); columns['cell'].append(cell); columns['is_cell'].append('T')
            else:
                columns['count'].append(number(r['Redundancy']) if 'Redundancy' in r else '1')
            columns['sample'].append(sample)
    out = {'format': 'oas-unit', 'reader': f'pandas {pd.__version__}', 'rows': rows, 'columns': {k: digest(v) for k, v in columns.items()}, 'metadata': {k: str(v).replace('\u200b', '').strip() for k, v in sorted(meta.items()) if v is not None and str(v).strip()}}
    if paired:
        out['unit'] = 'cells'
    return out

TCRDIST_CHAINS = [('a', 'TRA'), ('b', 'TRB'), ('g', 'TRG'), ('d', 'TRD')]

def tcrdist_file(path):
    # tcrdist3's clone tables: paired chains as single cells (each clone's count its cells, a count
    # that is not a whole number from 1 to 100,000 one cell), one chain as bulk clones; a sample for
    # each subject and epitope.
    with open(path) as f:
        sep = '\t' if '\t' in f.readline() else ','
    df = pd.read_csv(path, sep=sep, dtype=str, keep_default_na=False)
    chains = [(l, locus) for l, locus in TCRDIST_CHAINS if f'cdr3_{l}_aa' in df.columns and f'v_{l}_gene' in df.columns]
    paired = len(chains) > 1
    columns = {k: [] for k in ('sequence_id', 'locus', 'v_call', 'j_call', 'junction', 'junction_aa', 'productive', 'clone', 'count', 'sample')}
    if paired:
        columns.update({'cell': [], 'is_cell': []})
    for line, r in enumerate(df.to_dict('records'), start=2):
        subject, epitope = text(r.get('subject')), text(r.get('epitope'))
        sample = subject + ('_' + epitope if epitope else '') if subject else sample_of(path)
        clone = text(r.get('clone_id'))
        n = number(r['count']) if 'count' in r else '1'
        def emit(l, locus, sequence_id, cell):
            nt, aa = upper(r.get(f'cdr3_{l}_nucseq')), upper(r.get(f'cdr3_{l}_aa'))
            columns['sequence_id'].append(sequence_id)
            columns['locus'].append(locus)
            columns['v_call'].append(text(r.get(f'v_{l}_gene'))); columns['j_call'].append(text(r.get(f'j_{l}_gene')))
            columns['junction'].append(nt); columns['junction_aa'].append(aa)
            columns['productive'].append(None if aa is None else 'F' if (nt and len(nt) % 3) or '*' in aa else 'T')
            columns['clone'].append(clone)
            columns['sample'].append(sample)
            if cell is not None:
                columns['count'].append('1'); columns['cell'].append(cell); columns['is_cell'].append('T')
            else:
                columns['count'].append(n)
        if not paired:
            emit(*chains[0], clone, None)
            continue
        cells = int(float(n)) if n is not None and float(n) == int(float(n)) and 1 <= float(n) <= 100000 else 1
        ident = clone or f'row {line}'
        for k in range(1, cells + 1):
            cell = f'{ident}#{k}' if cells > 1 else ident
            for l, locus in chains:
                if not text(r.get(f'v_{l}_gene')) and not text(r.get(f'cdr3_{l}_aa')):
                    continue
                emit(l, locus, f'{cell}_{l}', cell)
    out = {'format': 'tcrdist3-clones', 'reader': f'pandas {pd.__version__}', 'rows': len(columns['sample']), 'columns': {k: digest(v) for k, v in columns.items()}}
    if paired:
        out['unit'] = 'cells'
    return out

def kind(path):
    if path.endswith('.gz'):
        with gzip.open(path, 'rt') as f:
            head = f.readline()
        return unit_file if head.startswith('"{') else None
    with open(path, 'rb') as f:
        head = f.readline().decode('utf-8', 'replace')
    cols = head.rstrip('\r\n').split('\t' if '\t' in head else ',')
    if any(f'cdr3_{l}_aa' in cols and f'v_{l}_gene' in cols for l, _ in TCRDIST_CHAINS) and 'sequence_id' not in cols:
        return tcrdist_file
    if '\t' in head:
        if 'rearrangement' in cols or 'aminoAcid' in cols:
            return immunoseq_file
        if 'aaSeqCDR3' in cols or 'AA. Seq. CDR3' in cols:
            return mixcr_file
        if 'sequence_id' in cols:
            return airr_file
    elif 'contig_id' in head and 'barcode' in head:
        return cellranger_file
    return None

# The development data sets: those sources.json lists (the cache also holds what the tests make).
with open(os.path.join(here, '..', 'sources.json')) as f:
    listed = set(json.load(f)['datasets'])
out = {}
for dataset in sorted(os.listdir(cache)):
    root = os.path.join(cache, dataset)
    if not os.path.isdir(root) or dataset not in listed:
        continue
    for dirpath, _, files in sorted(os.walk(root)):
        for name in sorted(files):
            path = os.path.join(dirpath, name)
            if name.endswith('.part'):
                continue
            reader = kind(path)
            if reader is None:
                continue
            rel = os.path.relpath(path, cache)
            print(rel, file=sys.stderr)
            out[rel] = reader(path)
json.dump({'generated_by': f'airr {airr.__version__}, pandas {pd.__version__}', 'files': out}, open(os.path.join(here, 'readers.json'), 'w'), indent=1, sort_keys=True)
