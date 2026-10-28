"""Independent exported-byte inventory check; no ClonoLoom imports or method reimplementation.

UV_CACHE_DIR=/tmp/clonoloom-uv uv run --no-project validation/reference/headless_run.py RUN_DIR [REPLAY_JSON]
Checks successful source runs, actual file digests and ZIP members without re-encoding results.
Scientific method references remain batch_qc.py and motifs.py.
"""
import hashlib
import json
from pathlib import Path
import sys
import zipfile


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def check(folder, replay=None):
    run = json.loads((folder / "run.json").read_bytes())
    assert run["format"] == "clonoloom-run" and run["version"] == 1
    assert run["status"] == "complete" and run["operation"] == "compute"
    expected = {"validation.json", run["kind"] + ".json", "manifest.json", "NOTICE.txt",
                "result.certificate.clz", "effective-recipe.json"}
    assert {item["path"] for item in run["outputs"]} == expected
    assert len(run["outputs"]) == len(expected)
    for item in run["outputs"]:
        raw = (folder / item["path"]).read_bytes()
        assert len(raw) == item["size"] and sha(raw) == item["sha256"]
    recipe = json.loads((folder / "effective-recipe.json").read_bytes())
    assert recipe["time"] == run["time"] and len(recipe["tasks"]) == 1
    assert recipe["tasks"][0]["inputs"] == run["inputs"]
    manifest = json.loads((folder / "manifest.json").read_bytes())
    assert manifest["id"] == run["artifact"] and manifest["kind"] == run["kind"]
    with zipfile.ZipFile(folder / "result.certificate.clz") as archive:
        record = json.loads(archive.read("certificate.json"))
        assert record["scope"] == "recorded-artifact-bytes"
        assert len(archive.namelist()) == len(set(archive.namelist()))
        assert set(archive.namelist()) == {"certificate.json", *(f["path"] for f in record["files"])}
        for item in record["files"]:
            raw = archive.read(item["path"])
            assert len(raw) == item["size"] and sha(raw) == item["sha256"]
        output = archive.read("artifact/" + run["kind"] + ".json")
        assert output == (folder / (run["kind"] + ".json")).read_bytes()
        assert archive.read("manifest.json") == (folder / "manifest.json").read_bytes()
        assert archive.read("NOTICE.txt") == (folder / "NOTICE.txt").read_bytes()
        if replay:
            report = json.loads(replay.read_bytes())
            assert report["verdict"] == "replay-match" and report["recomputed"] is True
            assert report["integrity"]["verdict"] == "integrity-match"
            assert report["original"] == manifest
            assert len(report["steps"]) == 1
            step = report["steps"][0]
            assert step["metadataMatched"] is True
            assert step["expectedSHA256"] == step["actualSHA256"] == sha(output)
    print(f"{run['kind']}: {len(expected)} output inventories, archive members and replay digests agree")


if __name__ == "__main__":
    check(Path(sys.argv[1]), Path(sys.argv[2]) if len(sys.argv) > 2 else None)
