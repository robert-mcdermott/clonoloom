# The development files as independent readers see them, for validation/readers_test.go: AIRR
# files read by the AIRR Community's own library (airr), Cell Ranger contig annotations and
# immunoSEQ exports read by pandas, each column ClonoLoom reads reduced to its row count and a
# SHA-256 digest (validation/reference/readers.json).
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
# its J call names (its first three letters), else its V call's, else its C call's.
import hashlib, json, math, os, sys
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

def kind(path):
    with open(path, 'rb') as f:
        head = f.readline().decode('utf-8', 'replace')
    if '\t' in head:
        cols = head.rstrip('\r\n').split('\t')
        if 'rearrangement' in cols or 'aminoAcid' in cols:
            return immunoseq_file
        if 'sequence_id' in cols:
            return airr_file
    elif 'contig_id' in head and 'barcode' in head:
        return cellranger_file
    return None

out = {}
for dataset in sorted(os.listdir(cache)):
    root = os.path.join(cache, dataset)
    if not os.path.isdir(root):
        continue
    for dirpath, _, files in sorted(os.walk(root)):
        for name in sorted(files):
            path = os.path.join(dirpath, name)
            if name.endswith('.gz') or name.endswith('.part'):
                continue
            reader = kind(path)
            if reader is None:
                continue
            rel = os.path.relpath(path, cache)
            print(rel, file=sys.stderr)
            out[rel] = reader(path)
json.dump({'generated_by': f'airr {airr.__version__}, pandas {pd.__version__}', 'files': out}, open(os.path.join(here, 'readers.json'), 'w'), indent=1, sort_keys=True)
