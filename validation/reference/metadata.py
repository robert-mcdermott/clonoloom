"""Independent exact JSON check of native API evidence; no application imports.

GOCACHE=/tmp/clonoloom-go-cache CLONOLOOM_METADATA_EVIDENCE=/tmp/metadata-evidence.json go test . -run '^TestMetadataInspectionExactAndPaged$' -count=1
uv run --offline --no-project validation/reference/metadata.py /tmp/metadata-evidence.json

This tests JSON preservation, not AIRR schema conformance or MiAIRR completeness.
"""
import json
import sys
from decimal import Decimal
from pathlib import Path


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        assert key not in result, f"Duplicate key {key}"
        result[key] = value
    return result


raw = json.loads(Path(sys.argv[1]).read_text())["rawJSON"]
record = json.loads(raw, parse_int=int, parse_float=Decimal, object_pairs_hook=unique_object)
evidence = record["extra"]
assert record["repertoire_id"] == "R1"
assert evidence == {
    "integer": 9007199254740993,
    "decimal": Decimal("1.234567890123456789"),
    "large": Decimal("1e400"),
    "null": None,
    "false": False,
    "zero": 0,
}
assert type(evidence["false"]) is bool
assert type(evidence["zero"]) is int
assert '"zero":-0' in raw  # preserve the token even when numeric equality cannot distinguish it
assert '"large":1e400' in raw
print("Independent int/Decimal JSON oracle: exact integer, decimal, exponent, null, false and negative-zero token preserved")
