# ClonoLoom Python client

Wave 8 development: generated methods for the local action API 2, catalog 1, permission policy 1.
The package/application version stays 0.7.0 until the wave release; published 0.7.0 hosts predate
this contract. Install this checkout, not a published package with the same version number.

```sh
uv add /absolute/path/to/clonoloom/clients/python
# Optional exact-value DataFrame helper:
uv add '/absolute/path/to/clonoloom/clients/python[pandas]'
```

Start the development host separately with `clonoloom --remote-control` (read-only) or explicitly
choose grants, for example `--allow edits,compute,files`. Keep its window open. The client neither
launches the host nor widens grants or accepts proposals.

```python
import clonoloom

cl = clonoloom.connect(client='Research notebook')
print(cl.permissions())
print(cl.get_state().data)
# With host compute/files grants and an open dataset:
result = cl.query_clonotypes(limit=10)
rows = result.records()  # Python int / decimal.Decimal / None; full-table frequencies
frame = result.frame()  # optional pandas, object columns to retain exact values
```

Methods follow the host catalog; Python snake_case parameters map to the host's names.
`UNSET` omits optional arguments; `None` sends JSON null, which the host may reject.
`connect(data_dir=...)` reads that library's remote.json. An explicit URL never borrows a token
from a discovered library: pass its matching token explicitly. Compatibility is checked before
every action. Errors retain codes/status/action; ambiguous transport failures never auto-retry.

The client preserves original HTTP bytes in `Result.raw_json` and native query text in
`Result.data['resultJSON']`. `Result.native()` decodes that text exactly. Frames do not infer
numeric dtypes; absent keys become None only in the convenience frame. Later pandas arithmetic,
float conversion and plotting are the caller's responsibility. No new scientific engine or
certificate replay mode is included. See the complete guide at
https://github.com/robert-mcdermott/clonoloom/blob/main/docs/PYTHON.md.

Host/client patterns are adapted from Apache-2.0 CytoWeave and MaveScape, by Robert McDermott.
