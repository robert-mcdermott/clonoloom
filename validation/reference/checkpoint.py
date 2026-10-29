"""Independently check local checkpoint inventories and recovered replay evidence.

mkdir -p /tmp/checkpoint-oracle
CLONOLOOM_CHECKPOINT_ORACLE_DIR=/tmp/checkpoint-oracle go test . -run TestCheckpointRecoveryExactAndReplay
UV_CACHE_DIR=/tmp/clonoloom-uv uv run --no-project validation/reference/checkpoint.py /tmp/checkpoint-oracle

Only standard-library hashes/JSON and literal file bytes are used. This validates
preservation, not statistical calibration, authorship or original source contents.
"""
from pathlib import Path
import hashlib
import json
import sys


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def check(root):
    folder = root / "evidence"
    record = json.loads((folder / "checkpoint.json").read_bytes())
    assert record["format"] == "clonoloom-checkpoint" and record["version"] == 1
    fingerprint = dict(record, id="")
    # Record metadata/inventories contain no floating-point values. Match Go's
    # documented HTML-sensitive JSON escaping without re-encoding scientific files.
    encoded = json.dumps(fingerprint, separators=(",", ":"), ensure_ascii=False)
    for literal, escaped in [("<", "\\u003c"), (">", "\\u003e"), ("&", "\\u0026"), ("\u2028", "\\u2028"), ("\u2029", "\\u2029")]:
        encoded = encoded.replace(literal, escaped)
    assert record["id"] == "cp-" + sha(encoded.encode())
    inventory = {f["path"]: f for f in record["files"]}
    assert len(inventory) == len(record["files"])
    assert list(inventory) == sorted(inventory)
    actual = {str(f.relative_to(folder)) for f in folder.rglob("*") if f.is_file()}
    assert actual == {*inventory, "checkpoint.json"}
    for name, item in inventory.items():
        data = (folder / name).read_bytes()
        assert len(data) == item["size"] and sha(data) == item["sha256"]
    project = json.loads((folder / "project.json").read_bytes())
    assert project["id"] == record["project"]
    replay = json.loads((root / "replay.json").read_bytes())
    assert replay["verdict"] == "replay-match" and replay["recomputed"] is True
    target = replay["original"]["id"]
    manifest = json.loads((folder / "artifacts" / target / "manifest.json").read_bytes())
    assert manifest == replay["original"]
    for artifact in project["artifacts"]:
        path = folder / "artifacts" / artifact["id"]
        m = json.loads((path / "manifest.json").read_bytes())
        for item in m["files"]:
            data = (path / item["path"]).read_bytes()
            assert sha(data) == item["sha256"] and len(data) == item["size"]
    step = replay["steps"][-1]
    output = folder / "artifacts" / target / (manifest["kind"] + ".json")
    assert step["metadataMatched"] is True
    assert step["expectedSHA256"] == step["actualSHA256"] == sha(output.read_bytes())
    print(f"Checkpoint identity, {len(inventory)} exact files and recovered replay digests agree")


if __name__ == "__main__":
    check(Path(sys.argv[1]))
