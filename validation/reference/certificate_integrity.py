"""Independent certificate v1 inventory oracle; Python standard library only.

Run through uv: uv run --no-project validation/reference/certificate_integrity.py FILE
Reads ZIP members without extraction. Checks canonical record bytes and SHA-256,
manifest inventory, and exact JSON number/null tokens in the small Go test fixture.
This is integrity validation, not scientific replay or authenticity verification.
"""
import hashlib
import json
import sys
import zipfile


def digest(raw):
    return hashlib.sha256(raw).hexdigest()


with zipfile.ZipFile(sys.argv[1]) as archive:
    names = archive.namelist()
    assert len(names) == len(set(names))
    record_raw = archive.read("certificate.json")
    record = json.loads(record_raw)
    assert record["format"] == "clonoloom-certificate"
    assert record["version"] == 1
    assert record["scope"] == "recorded-artifact-bytes"
    assert json.dumps(record, separators=(",", ":"), ensure_ascii=False).encode() == record_raw
    fingerprint = record.pop("fingerprint")
    assert digest(json.dumps(record, separators=(",", ":"), ensure_ascii=False).encode()) == fingerprint
    inventory = record["files"]
    assert [f["path"] for f in inventory] == sorted(f["path"] for f in inventory)
    assert set(names) == {"certificate.json", *(f["path"] for f in inventory)}
    for f in inventory:
        raw = archive.read(f["path"])
        assert len(raw) == f["size"] and digest(raw) == f["sha256"], f["path"]
    manifest = json.loads(archive.read("manifest.json"))
    assert manifest["id"] == record["artifact"]
    assert manifest["kind"] == record["kind"]
    for f in manifest["files"]:
        saved = next(v for v in inventory if v["path"] == "artifact/" + f["path"])
        assert saved["sha256"] == f["sha256"] and saved["size"] == f["size"]
    assert archive.read("artifact/result.json") == b'{"value":1.2345678901234567,"missing":null,"zero":0}'
    assert manifest["lineage"]["job"]["spec"] == {"seed": 42, "source": "/never/read/this"}
    assert b"not a signature" in archive.read("README.txt")
print(f"Independent ZIP, JSON-token and SHA-256 checks passed: {fingerprint}")
