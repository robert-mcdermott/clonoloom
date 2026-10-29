"""Independently check replay report digests against the original archived bytes.

Run: uv run --no-project validation/reference/certificate_replay.py DIRECTORY
The Go test TestCertificateReplayReleaseCohortControls writes certificates and
reports when CLONOLOOM_REPLAY_ORACLE_DIR explicitly names an existing directory.
This oracle checks evidence bindings, not statistical calibration; the independent
cohort/count-model references remain separate validation.
"""
import hashlib
import json
from pathlib import Path
import sys
import zipfile


def digest(raw):
    return hashlib.sha256(raw).hexdigest()


def raw_field(raw, key):
    # Keep original numeric lexemes: a Python float re-encoding is not Go JSON.
    text = raw.decode("utf-8")
    decoder = json.JSONDecoder()
    pos = text.index("{") + 1
    while True:
        while text[pos].isspace() or text[pos] == ",":
            pos += 1
        name, pos = decoder.raw_decode(text, pos)
        while text[pos].isspace():
            pos += 1
        assert text[pos] == ":"
        pos += 1
        while text[pos].isspace():
            pos += 1
        start = pos
        _, pos = decoder.raw_decode(text, pos)
        if name == key:
            return text[start:pos].encode("utf-8")
        assert text[pos] != "}", "missing field"


paths = sorted(Path(sys.argv[1]).glob("*.clz"))
assert len(paths) == 12, "expected all four release studies and three result kinds"
for path in paths:
    report = json.loads(path.with_suffix(".report.json").read_bytes())
    assert report["format"] == "clonoloom-replay" and report["version"] == 1
    assert report["scope"] == "captured-cohort-evidence"
    assert report["verdict"] == "replay-match" and report["recomputed"]
    with zipfile.ZipFile(path) as archive:
        manifest = json.loads(archive.read("manifest.json"))
        assert report["original"] == manifest
        record = json.loads(archive.read("certificate.json"))
        assert report["integrity"]["fingerprint"] == record["fingerprint"]
        for member in record["files"]:
            raw = archive.read(member["path"])
            assert len(raw) == member["size"] and digest(raw) == member["sha256"]
        result_bytes = archive.read("artifact/" + manifest["kind"] + ".json")
    result = json.loads(result_bytes)
    expected = {manifest["id"]: digest(result_bytes)}
    if manifest["kind"] != "cohort-features":
        parent = result["parentManifest"]
        feature_bytes = raw_field(result_bytes, "features")
        expected[parent["id"]] = digest(feature_bytes)
        assert expected[parent["id"]] == parent["files"][0]["sha256"]
        assert expected[parent["id"]] == manifest["lineage"]["job"]["spec"]["sha256"]
    assert {s["artifact"] for s in report["steps"]} == set(expected)
    for step in report["steps"]:
        assert step["verdict"] == "replay-match" and step["metadataMatched"]
        assert step["expectedSHA256"] == step["actualSHA256"] == expected[step["artifact"]]
print("Independent archive bindings, original provenance and replay digests passed: 12 results.")
