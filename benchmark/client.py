"""Benchmark transport only: original successful response bytes, typed refusal metadata."""
import json
import sys
import clonoloom
from clonoloom import ClonoLoomError

request = json.load(sys.stdin)
try:
    client = clonoloom.connect(data_dir=sys.argv[1], client="Benchmark candidate", timeout=30)
    result = client.call(request["action"], **request["args"])
    sys.stdout.buffer.write(result.raw_json)
except ClonoLoomError as error:
    print(json.dumps({"ok": False, "code": error.code, "outcomeUnknown": error.outcome_unknown,
                      "evidenceOrigin": "python-client-exception"}))
