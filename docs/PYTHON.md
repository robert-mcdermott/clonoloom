# Python client

Wave 8 development, slice 8. The client requires action **API 2**, catalog **1**, permission
policy **1** and Python **3.10 or later**. Package and application version remain **0.7.0** until
the wave release. Published 0.7.0 hosts predate this contract: use the development executable
and install this checkout. Nothing is published to a package index by these commands.

## Install and connect

```sh
uv add /absolute/path/to/clonoloom/clients/python
# Optional pandas helper:
uv add '/absolute/path/to/clonoloom/clients/python[pandas]'
# Or run one script in an isolated environment:
uv run --with /absolute/path/to/clonoloom/clients/python my_analysis.py
```

Start ClonoLoom separately. Default remote access is read-only:

```sh
clonoloom --remote-control
# For local data, tables and reviewed edits, choose explicit grants:
clonoloom --remote-control --allow edits,compute,files
```

Keep the window open. Python uses the normal local HTTP API and the same action catalog as
[MCP](MCP.md); it does not start an MCP subprocess or execute a second scientific implementation.
The most recently connected window receives the action. Opening files, choosing a definition or
other edits require visible human review. Host grants do not imply acceptance. Network access
requires its own explicit grant; `--offline` still blocks downloads. See [Automation](AUTOMATION.md).

```python
import clonoloom

cl = clonoloom.connect(client='Research notebook')
print(cl.version())
print(cl.permissions())
state = cl.get_state().data
print(state['project'])
# If the host has edits/compute/files grants, review this in its window:
cl.open_data(['/absolute/path/to/rearrangements.tsv'], name='My repertoire')
```

Client names are caller-supplied labels. Accepted changes use normal History and Undo/Redo;
there is no client approval function. Generated action arguments called `accept`, where present,
refer to existing provider acknowledgements, not automation proposal acceptance.

## Connection precedence

| Choice | URL and token |
| --- | --- |
| Explicit `url=` | Uses only that URL and the explicitly supplied `token=`; never borrows a discovered token |
| Explicit `data_dir=` | Reads that library's remote.json; ignores environment URL/token |
| `CLONOLOOM_URL` | Uses that URL and `CLONOLOOM_TOKEN`, if supplied |
| `CLONOLOOM_DATA_DIR` | Reads that library's remote.json |
| Default library | Uses the host's platform default data directory and its remote.json |
| Missing default remote.json | Tries http://127.0.0.1:8860 without a token |

An explicit token overrides the discovered connection token. `token=None` or `token=''` disables
it; the default `UNSET` uses the matching connection token. A missing explicitly selected file,
unreadable/malformed file or invalid URL fails rather than silently selecting another host.
`CLONOLOOM_TOKEN` is used only with `CLONOLOOM_URL`; a standalone token variable is ignored.

Only numeric loopback HTTP addresses or `localhost` are accepted; `localhost` normalizes to
127.0.0.1. Credentials, non-root paths, queries and fragments are refused. Environment proxies
and redirects are disabled, so connection tokens are not forwarded elsewhere. This is a local
client restriction, not an operating-system sandbox. It does not authenticate other processes
running as the same user. `repr(connection)` does not include its token.

`connect()` checks compatibility immediately. Constructing `ClonoLoom(...)` defers that network
check. Every action rechecks the live API/catalog/policy versions before POST; incompatible or
missing versions fail without sending the action. The package version does not substitute for
that contract. Tokens cannot widen host grants. A host replacement between requests is still a
race; use a stable host and inspect the project before dependent edits.

## Exact JSON and omitted arguments

Responses use Python `int` for integer tokens and `decimal.Decimal` for fractional/exponent
tokens. Raw HTTP response bytes remain in `Result.raw_json`; `Result.data['resultJSON']` retains
the original native JSON string returned by typed queries. `Result.native()` decodes that string
with the same exact-value policy. Numeric spelling, whitespace and integer negative-zero tokens
are retained only in raw text, not promised by reserialization of parsed values.

```python
from decimal import Decimal

result = cl.query_clonotypes(
    where={'field': 'total', 'op': 'gte', 'value': Decimal('2')},
    limit=10,
)
evidence = result.native()
rows = result.records()
print(evidence['count'], evidence['matched'], evidence['definitionId'])
print(rows[0]['total'], rows[0]['frequency'])
```

Finite Decimal arguments are encoded as JSON numbers without conversion to float or quoted
strings. Python floats retain only the precision they already have; wrapping an already-rounded
float in Decimal cannot recover lost evidence. Host scientific argument bounds still apply,
including its safe-integer limit. The client does not coerce an invalid value into a permitted one.

Generated wrappers use snake_case Python parameter names, mapping to the host's original names.
Required arguments remain required; optional defaults are `clonoloom.UNSET`:

- Omit an argument or pass `UNSET` to leave it out of the top-level argument object.
- Pass `None` to send JSON null explicitly. The host may reject null for that field.
- Lists remain arrays, including empty and one-element lists. Empty dictionaries remain objects.
- Non-string dictionary keys, tuples, sets, nested UNSET, cycles, NaN and infinities are refused.

`cl.call('action_name', **args)` uses exact host argument names. The generated methods and help
come from the checked catalog; `cl.tools()` inspects the live catalog, including actions for
which grants are absent. Calling an ungranted action produces a typed refusal, never an implicit
grant. Methods do not batch or retry dependent edits; await each call before starting the next.

## Records and optional pandas

`Result.records()` selects a root record list or `rows` in the native result/data object.
`records('key')` selects a named list explicitly; it never chooses an arbitrary metadata list.
Empty lists remain empty. No record values are rounded, inferred or filtered by the helper.

`Result.frame()` requires optional pandas and creates **object columns** containing the original
int/Decimal/bool/list/None values. It avoids numeric dtype inference that could turn a large
integer with missing values into a float. Missing dictionary keys become None in this convenience
view; use records/raw JSON to distinguish absent keys from explicit null. Later `astype`, NumPy
conversion, plotting or pandas arithmetic are the caller's choices and can lose precision or
use the current Decimal arithmetic context. This helper makes no statistical calculation.

## Errors, bounds and cancellation

All typed errors inherit `ClonoLoomError` and carry `code`, `status` (0 for local errors),
`action` and `outcome_unknown`:

| Error | Meaning |
| --- | --- |
| `ConnectionError` | Discovery, connection or timeout failure |
| `CompatibilityError` | Host API/catalog/policy differs; no action submitted |
| `ProtocolError` | Malformed, incompatible-shaped or oversized response; redirect refused |
| `ArgumentError` | Local request cannot be faithfully encoded |
| `PermissionDenied` | Host refused a missing token or grant |
| `ActionError` | Other host refusal/failure, including rejection, stale project and invalid arguments |

The default socket timeout is 660 seconds (host action timeout: ten minutes). It is an I/O
operation timeout, not an overall scientific deadline. A timeout/disconnect after submission
can leave completed work. `outcome_unknown` is deliberately conservative; false is not a claim
of transactional rollback. Inspect the project, jobs and History before deciding what to do.
The client never retries, accepts a proposal, deletes output or rolls back files. Closing the
HTTP connection requests host cancellation; worker/file effects can already have completed.
There is no new Python cancellation API in this slice.

Connection files are bounded to 64 KiB, complete action requests to 1 MiB, and responses/native
result JSON to 16 MiB. JSON depth is at most 64; Python's own integer digit and Decimal exponent
limits also apply. Duplicate keys, invalid UTF-8, non-finite constants and malformed envelopes
are refused. Oversized responses are errors, not truncated successes. Limits and failures never
justify silently dropping scientific rows.

## Scientific boundary and validation

The client invokes the existing Go methods. Querying selects the current post-definition/QC
table without changing source units or renormalizing its full-table frequencies; displayed ranks
reflect filtered membership. Missing values remain missing. No new estimator, biological
interpretation, plot, certificate tolerance or replay kind is added. [Exact certificate replay](CERTIFICATES.md)
continues to use original captured evidence and its existing five-kind limits. Client-parsed
objects or pandas frames are not a replacement for original certificate bytes.

The real-host validation uses a seeded, explicitly simulated vaccination study, compares native
HTTP/Python query strings and filtered rows, and checks grants, token refusal, human rejection,
timeout, named History and Undo/Redo. Fake-server checks cover every generated method, malformed
responses, precision, discovery, bounds, compatibility and no retry. See [client maintenance](../clients/README.md)
and the runnable [query example](../clients/examples/query_clonotypes.py).

Implementation references: Python's [JSON decoder hooks](https://docs.python.org/3/library/json.html),
[Decimal representation](https://docs.python.org/3/library/decimal.html),
[urllib transport](https://docs.python.org/3/library/urllib.request.html), and pandas'
[DataFrame dtype controls](https://pandas.pydata.org/docs/reference/api/pandas.DataFrame.html).
