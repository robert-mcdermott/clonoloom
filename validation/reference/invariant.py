"""Independent cell-level check of published human gene/length/motif rules.

uv run validation/reference/invariant.py [validation/cache]
Reads Cell Ranger CSV directly; no ClonoLoom output or external implementation.
MAIT/iNKT: doi:10.1038/s41587-021-00989-2; GEM: doi:10.4049/jimmunol.1401380.
Select primary productive chains by UMI, reads, then contig ID. These cached files have
resolved gene calls; fail rather than silently resolving ambiguous calls.
"""
import csv
import json
import re
import sys
from collections import defaultdict
from pathlib import Path

root = Path(sys.argv[1] if len(sys.argv) > 1 else "validation/cache")
result = {}
for name in ("10x-pbmc-1k", "10x-pbmc-10k-v2"):
    files = list((root / name).glob("*t_filtered_contig_annotations.csv"))
    assert len(files) == 1
    cells = defaultdict(lambda: defaultdict(list))
    with files[0].open() as f:
        for row in csv.DictReader(f):
            if row["is_cell"].lower() != "true" or row["productive"].lower() != "true" or row["chain"] not in ("TRA", "TRB"):
                continue
            if not re.fullmatch("[ACDEFGHIKLMNPQRSTVWY]+", row["cdr3"]):
                continue
            cells[row["barcode"]][row["chain"]].append(row)
    counts = [0, 0, 0]
    for chains in cells.values():
        primary = {l: min(rows, key=lambda x: (-int(x["umis"]), -int(x["reads"]), x["contig_id"])) for l, rows in chains.items()}
        a, b = primary.get("TRA"), primary.get("TRB")
        if not a:
            continue
        v, j, aa = a["v_gene"].split("*")[0], a["j_gene"].split("*")[0], a["cdr3"]
        assert not any(c in v+j for c in "|,;")
        counts[0] += v == "TRAV1-2" and j in ("TRAJ33", "TRAJ12", "TRAJ20") and len(aa) == 12
        counts[1] += bool(v == "TRAV10" and j == "TRAJ18" and len(aa) in (14, 15, 16) and b and b["v_gene"].split("*")[0] in ("TRBV25", "TRBV25-1"))
        counts[2] += bool(v == "TRAV1-2" and j == "TRAJ9" and re.fullmatch(r"CAV[RL].TGGFKTIF", aa))
    result[name] = {"cells": len(cells), "matched": counts}
print(json.dumps(result, indent=2))
