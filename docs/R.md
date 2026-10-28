# R client

The wave 8 development R package follows CytoWeave/MaveScape: generated `cl_*` functions and
help, S3 connections/results, a small curl transport, remote.json discovery and record frames.
It shares the Python client's action API **2**, catalog **2** and permission policy **1**.
Package/application version remains **0.7.0** until the wave release; published 0.7.0 hosts
predate this contract. Use a development host and matching checkout.

## Install and connect

R 4.1 or later is required. Install dependencies, then the local package:

```sh
Rscript -e 'install.packages(c("curl", "jsonlite"), repos="https://cloud.r-project.org")'
R CMD INSTALL clients/r
clonoloom --remote-control --allow edits,compute,files
```

Start the host separately and keep its window open. Read-only is the host default. Tokens do
not grant permissions; non-read actions require explicit launch grants and the connection token.
Edits require human proposal acceptance in the window and use named History and Undo/Redo.
The package never launches a host, broadens grants, accepts a proposal or retries an action.
See [automation policy](AUTOMATION.md) for effects, review and completed-file limits.

```r
library(clonoloom)
cl <- cl_connect(client = "Research notebook")
cl_permissions(cl)
# Open a dataset in the window first; query_clonotypes requires compute.
result <- cl_query_clonotypes(
  where = list(field = "total", op = "gte", value = cl_number("2")),
  limit = 10L, cl = cl)
evidence <- cl_native(result)
rows <- cl_records(result)
cat(as.character(evidence$count), as.character(evidence$matched), evidence$definitionId)
frame <- as.data.frame(result)  # list columns; no numeric inference
```

Generated functions use snake_case argument names mapped to the host's original camelCase names.
Required arguments remain required; optional arguments default to `cl_omit()`.
`cl_call("action_name", ..., .args=list(), cl=cl)` accepts original host names. Duplicate and
unnamed arguments are refused. `?cl_query_clonotypes` describes effects, review and arguments;
`cl_tools(cl)` inspects the live catalog, including actions for which grants are absent.
The default connection is the last successful `cl_connect()`; pass `cl=` for explicit workflows.

## Exact numbers and faithful JSON

**Every decoded JSON number is a `clonoloom_number` token**, stored as character data with a
class, not an R double. This preserves large integers, full decimal digits, exponents and
negative-zero spelling. `cl_number("9007199254740993")` constructs an argument emitted as an
unquoted JSON number. Use original text; lost digits in an already rounded R value cannot be
recovered. The host still enforces its numeric bounds, including safe-integer argument limits.

`result$raw_json` retains original HTTP response bytes. `result$data$resultJSON` retains the
native JSON string from typed queries; `cl_native(result)` decodes it with the same token policy.
Raw text preserves whitespace and string escaping, which parsed values do not promise to retain.

`as.character(token)` exposes exact text. Arithmetic, comparisons, Math and Summary operations on
number tokens fail rather than treating strings as numbers or silently rounding. Explicit
`as.double(token)` / `as.numeric(token)` warns that conversion may round, overflow or underflow.
This class is an evidence representation, not an arbitrary-precision arithmetic library.
Subsetting/unclassing/combining values using base R can strip classes; retain original tokens
in records/list columns when exactness matters. Later calculation or plotting is the caller's
explicit choice, with its own precision requirements.

Plain finite R doubles are serialized with 17 significant digits, preserving only precision
already present in that double. Integer-valued doubles outside ±(2^53−1) are refused; supply
`cl_number()` from original text instead. R integers are encoded exactly. NA, NaN, infinities,
factors, dates and unsupported classes are refused rather than treated as missing evidence.

| R value | JSON meaning |
| --- | --- |
| Omitted optional argument or `cl_omit()` | No top-level argument key |
| `NULL` | Explicit null; the host may reject it for that argument |
| `cl_array("one")` or `list("one")` | One-element array |
| `cl_array()` / `list()` / empty plain vector | Empty array |
| `cl_object()` | Empty object |
| Uniquely named plain list | Object, retaining NULL members |
| Plain vector with multiple elements | Array |
| Plain atomic length-one value | Scalar |

Generated array parameters always wrap vectors/lists as arrays, including singleton or empty
vectors. Use `cl_array()` for nested arrays where shape matters. Nested omission markers and
named atomic vectors are refused. Objects may have empty string keys, but action argument names
must be nonempty. Duplicate keys, malformed UTF-8, invalid escapes and unpaired surrogates are
refused. R cannot represent embedded NUL strings: those responses fail explicitly, without
silently dropping the character. Number tokens are syntax-checked without an exponent-range
conversion; their practical limit is the containing JSON byte bound.

## Records and data frames

`cl_records(result)` selects a root record array or `rows`; `cl_records(result, "key")` selects a
named array explicitly. It never guesses among other metadata lists. Empty arrays remain empty.
`as.data.frame(result, key="key")` creates **list columns**, retaining exact number tokens,
booleans, lists and NULL. It neither rounds nor filters evidence. Missing record keys become
NULL only in this convenience frame; keep records/raw JSON to distinguish absent from null.
Use `frame$total[[1]]` to retrieve the first token without coercion. No statistical calculation
or scientific plot is added by these helpers.

## Discovery and compatibility

| Priority | Connection and token source |
| --- | --- |
| Explicit `url=` | Only the explicitly supplied token; never another library's token |
| Explicit `data_dir=` | That library's remote.json, ahead of environment settings |
| `CLONOLOOM_URL` | This URL and `CLONOLOOM_TOKEN` only |
| `CLONOLOOM_DATA_DIR` | That directory's remote.json |
| Platform default library | remote.json, or port 8860 without a token when absent |

The platform folder is `~/Library/Application Support/ClonoLoom` on macOS, `%APPDATA%/ClonoLoom`
on Windows (fallback `~/.clonoloom`), and `$XDG_CONFIG_HOME/ClonoLoom` or `~/.config/ClonoLoom`
on other systems. Explicit missing files and all malformed files fail rather than falling back.
Explicit `token=NULL` or `token=""` disables the token. Omission uses only the matching discovered
token. Only loopback HTTP root URLs are accepted; localhost is pinned to 127.0.0.1. Proxies and
redirects are disabled, and tokens are sent only on action POSTs. Printing a connection hides
its token; the connection object itself contains it and should not be shared.

`cl_connect()` checks compatibility immediately. Every action checks the live API/catalog/policy
versions again before sending the action. Missing or incompatible versions fail before POST.
This does not eliminate a host-replacement race between requests; use a stable host and inspect
state before dependent edits. Client labels are audit descriptions, not authenticated identities.

## Errors, timeouts and bounds

Connection and action errors inherit `clonoloom_error`, with `code`, `status` (0 for local errors), `action` and
`outcome_unknown`. Subclasses are `clonoloom_connection_error`, `clonoloom_compatibility_error`,
`clonoloom_protocol_error`, `clonoloom_argument_error`, `clonoloom_permission_error` and
`clonoloom_action_error`. Host refusals retain their codes, including rejected, stale-project,
busy, invalid-arguments, unauthorized and permission-denied.

The default timeout is **660 seconds for each complete HTTP transfer**, including proposal review.
R uses curl's millisecond timeout (rounded up, minimum 1 ms, maximum 2147483.647 seconds).
This differs from Python's socket-operation timeout and excludes local decoding time.
A timeout/disconnect after submission can leave completed worker or file effects.
`outcome_unknown` is conservative; false does not promise transactional rollback. Inspect the
project, jobs and History before deciding whether to resubmit. Closing the HTTP connection
requests cooperative host cancellation; the client has no separate cancellation API.

Connection files are bounded to 64 KiB, complete action requests to 1 MiB, and responses/native
JSON to 16 MiB, with at most 64 nesting levels. Responses are bounded while streaming. Oversized
results are errors, never truncated successes or an excuse to omit scientific rows. These limits
bound transport input, not total R memory or computation time for parsing adversarial structures.

## Scientific boundary and validation

R invokes the existing host workers. Query filters retain the current definition, QC, source
units and full-table frequency denominator; displayed ranks change with filtered membership.
Client number preservation does not add biological inference, change a statistical method or
establish validity. The existing five-kind [exact certificate replay](CERTIFICATES.md) remains
unchanged. Original certificate bytes remain authoritative, not reserialized client objects.

The shared real-host fixture generates a seeded artificial vaccination study, uses the same
project for Python and installed R, compares original native query bytes and filtered scientific
rows, and checks token/grant/null/unsafe-number refusals, rejection, timeout, named History and
Undo/Redo. Only the validation driver clicks proposal controls. No existing user library is used.
Fake-server tests cover every generated wrapper, exact tokens, malformed responses, discovery,
versions, limits and no retry. Package build/install/check and runnable examples are included in
[client validation](../clients/README.md). See [query_clonotypes.R](../clients/examples/query_clonotypes.R).

Implementation references: R's [double representation](https://stat.ethz.ch/R-manual/R-devel/library/base/html/double.html),
[jsonlite interface](https://cran.r-project.org/web/packages/jsonlite/refman/jsonlite.html),
[curl streaming](https://cran.r-project.org/web/packages/curl/refman/curl.html),
[transfer timeout](https://curl.se/libcurl/c/CURLOPT_TIMEOUT_MS.html), and
[package checks](https://cran.r-project.org/doc/manuals/r-release/R-exts.html).
