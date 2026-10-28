# Automation policy and review

Wave 8 development, slice 6. Published 0.7.0 binaries predate this policy. The development
host advertises API **2**, action catalog **3** and automation policy **1**. [MCP transport](MCP.md) is available in development slice 7; [Python](PYTHON.md) and [R](R.md) are available in development slices 8–9. Application version remains 0.7.0.

## Explicit capabilities

Start with `clonoloom --remote-control` for read-only automation (`get_state`). Grant only
what the workflow needs, for example:

```sh
clonoloom --remote-control --allow compute,files
# For proposed project edits and optional example/reference downloads:
clonoloom --remote-control --allow edits,compute,files,network
```

`--allow` requires remote control; unknown, repeated or whitespace-padded capability names
fail. Read is implicit. The token in the owner-readable `remote.json` is required for every
non-read call. Possessing a token does not grant a capability. `--offline` still prevents
network downloads even when network is granted. Neither request arguments nor a client's
name can change policy.

`GET /api/remote/tools` supplies the single catalog: names, titles, descriptions, argument
schemas, `effects`, `review`, `api`, `catalogVersion`, `policyVersion` and `allowed`.
Effects are conservative upper bounds across an action's options and cache states:

| Effect | Meaning |
| --- | --- |
| `read` | Inspect the open project's state and local metadata |
| `edits` | Change project, selection or application settings; requires window review |
| `compute` | May construct tables or run existing scientific workers |
| `files` | May open source/evidence files or write artifacts/downloads |
| `network` | May request an example or reference download |

An action can require a capability even when its particular input or warm cache avoids that
effect. Read-only queries can require compute/files without edits. They cannot write project changes
or reset existing view generations. Inspection/export actions may navigate to their visible
view while leaving the project unchanged. These are restrictions on
the automation action interface, not an operating-system sandbox or authentication between
programs already running as the same user. The application's ordinary same-origin local APIs
remain its trusted window interface. No SQL, shell or arbitrary expression endpoint is added.

## Review, cancellation and history

The most recently connected window shows the caller-supplied name, action, exact arguments,
project and effects. Accept executes once; Reject or closing the proposal does nothing.
Acceptance is refused if the project changed, including an edit followed by Undo, or if the
caller has disconnected. There is no action to accept a proposal. Provider acknowledgements
remain separate and existing restrictions still apply.

Non-read actions refuse to start while existing jobs or data imports are running, including
work that starts during proposal review. `get_state` remains available and lists running jobs
and whether an import is in progress. Finish or cancel that work before submitting again.

While a non-read action runs, the application surface is unavailable for incidental edits.
Cancel prevents later project attachments and requests cancellation of newly observed workers;
the window stays busy until the action settles. HTTP cancellation/timeouts and loss of the
window's event connection also cancel the call. Already completed edits can remain and are
undoable. Already written artifacts, cached files, exports and downloads may remain. A worker
that has not yet registered when cancellation arrives may still finish without attaching its
result. There is no automatic rollback, retry, deletion, or guarantee that cancellation stops
all CPU/I/O immediately. Re-check state before deciding whether to submit another call.

Project changes carry the caller name in History and use normal Undo/Redo. Starting a project
also retains the previous project in this window's undo history. UI-only changes such as theme
and dialog selection are changed through their ordinary controls, not project Undo. As before,
Undo history is bounded to 300 steps and is not restored after reopening the application.

The Automation drawer shows the last 500 request/acceptance/outcome events in this host
session. `GET /api/remote/audit` returns the same bounded history with the connection token
(or the current window session). It records timestamp, caller name, action, status and SHA-256
of argument bytes, not source contents or result payloads. Invalid/denied calls are included;
malformed requests have a generic caller label. It is an in-memory operational audit, not a
persistent forensic log or a scientific certificate. Export it before stopping if needed.
The dispatcher also refuses integer arguments outside ±(2^53−1), duplicate JSON keys and
arguments nested beyond 32 levels, rather than silently rounding or choosing a duplicate.
Names are self-reported labels, not authenticated identities. Native result manifests retain
the scientific methods, parameters and provenance.

## Typed clonotype queries

With `compute,files` granted and a token, call:

```json
{
  "action": "query_clonotypes",
  "client": "Research notebook",
  "args": {
    "dataset": "Study",
    "where": {"all": [
      {"field": "total", "op": "gte", "value": 3},
      {"field": "locus", "op": "eq", "value": "TRB"}
    ]},
    "offset": 0,
    "limit": 200
  }
}
```

Choose exactly one boolean branch (`all`, `any`, `not`) or a leaf (`field`, `op`, `value`).
Arrays for `all`/`any` must be nonempty. Limits: 8 levels, 64 nodes, 256 bytes per text value,
1 MiB request body and at most 1000 rows per page. Omitted/zero limit uses 200. Offset is
nonnegative; fractional offsets/limits fail. Omit `where` to select all rows.

Numeric fields `total` and `samples` support `eq`, `gte`, `lte`. Thresholds must be finite,
nonnegative and at most 2^53−1. Text fields `id`, `junction`, `v`, `j`, `locus` support exact,
case-sensitive `eq` and literal `contains`. No regex or wildcard expansion. Each chain field
matches if **any** chain matches; separate predicates may match different chains. This is not
a same-chain join. Empty/unavailable fields never match a text predicate. Negation reverses
the row predicate, so `not(v equals X)` also includes rows lacking a V value. A definition
that ignores V/J leaves those fields empty; that is not evidence of missing sequencing data.

The query uses the current dataset's clonotype definition and QC rules. Results are ordered
by descending total, with the existing stable clonotype-key order for ties. `count` is the
full post-definition/QC table size; `matched` is the count before pagination. Frequencies
retain the **full table abundance denominator**, not the selected rows' sum. `total` retains
the source unit; sample presence is not independent subject replication. Selection makes no
inference about expansion, antigen specificity, significance or biological validity.

The response carries `data.resultJSON`, the original Go JSON text. Decode it using a parser
appropriate to the desired numeric precision (for example Python `json.loads` with Decimal
for fractional tokens). Do not parse and serialize it through JavaScript to archive evidence.
The native window endpoint is `POST /api/clonotypes/query`; it accepts the explicit dataset,
definition, filter and typed `where`. Unknown fields and unsupported predicates fail closed.

Structured bounded queries are also used by the [AIRR Data Commons API](https://docs.airr-community.org/en/stable/api/adc_api_requests.html).
This is a local ClonoLoom clonotype contract, **not ADC protocol compatibility**. Existing
clonotype definitions and QC methods remain the scientific basis; there is no new estimator.

## Exact replay remains separate

Automation still calls the same Go scientific workers. This slice changes no estimator,
result format, plotting method, replay tolerance or supported result kind. A successful call,
proposal acceptance, query, audit entry or checksum is not a successful scientific replay.
Use the existing [certificate replay](CERTIFICATES.md) or [headless replay](HEADLESS.md)
contracts. Five captured-evidence kinds remain supported: cohort features, comparisons,
count models, batch QC and motifs. Exact output bytes and declared provenance boundaries
are required; unavailable kinds/inputs never fall back to approximate success.
