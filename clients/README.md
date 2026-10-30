# ClonoLoom clients

The Python client (wave 8, slice 8) follows the sibling CytoWeave/MaveScape layout: generated
methods and help from one action catalog, a small standard-library HTTP transport, discovery
through remote.json and optional pandas helpers. The R client (slice 9) adds generated `cl_*`
functions/Rd help, curl transport, S3 results, exact number tokens and list-column frames.

See [Python guide](../docs/PYTHON.md) for installation with uv, explicit grants, discovery,
precision, errors and examples. The [R guide](../docs/R.md) describes installation with R CMD
INSTALL, number tokens and explicit vector/list/null handling. Use the matching 0.9.0 host with API 2.
The client never starts a host, broadens permissions or accepts a proposal.

## Regenerate after a catalog change

```sh
go test -run '^TestTheClientsToolListIsCurrent$' -update-clients .
node clients/generate.mjs
```

Go compares `tools.json` with `actions.go` and the current API/catalog/policy constants.
The generator creates every method and its argument/effect/review help, maps camelCase arguments
to snake_case, distinguishes omission from null and follows the application's version.
CI checks both the snapshot and generated files; `--check` never writes files.

## Validate

```sh
go test -run '^TestTheClientsToolListIsCurrent$' .
node clients/generate.mjs --check
PYTHONPATH=clients/python/src uv run --no-project clients/python/tests/test_client.py -v
PYTHONPATH=clients/python/src CLONOLOOM_REQUIRE_PANDAS=1 uv run --no-project --with pandas clients/python/tests/test_client.py -v
node clients/test-clients.mjs
Rscript -e 'install.packages(c("curl", "jsonlite", "testthat", "withr", "pkgload"), repos="https://cloud.r-project.org")'
node clients/test-r.mjs
node clients/test-clients.mjs --r
uv build clients/python --out-dir /tmp/clonoloom-client-dist
uv run --no-project --with /tmp/clonoloom-client-dist/clonoloom-0.9.0-py3-none-any.whl clients/python/tests/package_smoke.py
```

The real-host driver needs Go, Node 22, uv and Chrome/Chromium; `--r` also needs R and its curl/jsonlite
dependencies. `test-r.mjs` runs local fake HTTP fixtures, then builds/installs/checks the R source
package in a temporary directory, failing on errors, warnings or notes. It also needs testthat,
withr and pkgload; `--tests-only` runs just the fixtures. Standalone R CMD check skips only the
HTTP fixtures if no fixture server is supplied. CI uses release and oldrel-1 R. It generates its own seeded
vaccine study offline, starts an isolated host with explicit edits/compute/files grants, and
clicks the visible review controls. It compares original native JSON against direct HTTP,
checks full-table frequencies and filtered rank, pandas precision, refusals and timeout, then
checks named History and Undo/Redo. Only the test driver accepts proposals. No provider download
or existing user library is used. With `--r`, the driver installs the package in a temporary R
library, compares the same project's original R/Python native JSON bytes, exercises R grants,
review and cancellation, and runs both query examples. It adds no scientific engine or replay
tolerance. These commands do not publish either package.
