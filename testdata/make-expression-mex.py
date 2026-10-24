"""Small artificial MEX/receptor evidence; no real donor data or biological simulation.
Run with uv run --no-project testdata/make-expression-mex.py.
"""
from pathlib import Path

root = Path(__file__).parent / 'expression-mex'
root.mkdir(exist_ok=True)
files = {
    'features.tsv': 'g1\tShared name\tGene Expression\ng2\tShared name\tGene Expression\nab1\tArtificial antibody\tAntibody Capture\n',
    'barcodes.tsv': 'AA-1\nBB-1\nCC-1\nDD-1\n',
    # Unordered coordinates, one explicit zero and one zero-count barcode.
    'matrix.mtx': '%%MatrixMarket matrix coordinate integer general\n% Artificial counts\n3 4 6\n2 3 7\n1 1 2\n3 1 11\n2 1 3\n1 2 0\n2 2 4\n',
    'receptors.tsv': 'sequence_id\tcell_id\tlocus\tv_call\tj_call\tjunction_aa\tproductive\n'
                     'a\tAA-1\tTRB\tTRBV5-1*01\tTRBJ2-7*01\tCASSLAGGPEAFF\tT\n'
                     'b\tAA-1\tTRA\tTRAV1-2*01\tTRAJ33*01\tCAVRDSNYQLIW\tT\n'
                     'c\tBB-1\tTRB\tTRBV5-1*01\tTRBJ2-7*01\tCASSLAGGPEAFF\tF\n'
                     'd\tEE-1\tTRB\tTRBV5-1*01\tTRBJ2-7*01\tCASSLAGGPEAFF\tT\n'
                     'e\t\tTRB\tTRBV5-1*01\tTRBJ2-7*01\tCASSLAGGPEAFF\tT\n',
}
for name, text in files.items():
    (root / name).write_text(text)
