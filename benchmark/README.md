# Agent contract benchmark

Wave 8, slice 13 development. Benchmark/fixture version 1; application version remains 0.7.0.
This repository-only runner evaluates eight bounded tasks against a real ClonoLoom host and
normal window. It follows CytoWeave's Apache-2.0 task, code-grader, reference and empty-control
pattern. HTTP, stdio MCP and the generated Python/R clients reach the same native methods.

**The supplied reference is a script, not a model.** Its results validate the task harness.
No model service, paid agent, credentials or external candidate runs by default. These public
fixtures are not a held-out test of research competence. See [the guide](../docs/AGENT-BENCHMARK.md)
for the independent truth, scientific limits and interpretation.

## Run a control

Run from the repository root with Go, Node 22+, Chrome/Chromium and, for the relevant transport,
uv or R with `curl` and `jsonlite`. `CHROME` may select a browser executable. No new application
runtime dependency is added. Python always runs through uv. Choose **new** output filenames:

```sh
node --test benchmark/benchmark.test.mjs
node benchmark/run.mjs --agent reference --transport http \
  --allow edits,compute,files --review scripted \
  --out /tmp/clonoloom-reference.json
node benchmark/run.mjs --agent none --transport http \
  --allow edits,compute,files --review scripted \
  --out /tmp/clonoloom-none.json
```

The runner builds a temporary binary unless `--binary /absolute/path/to/clonoloom` is supplied.
`--transport` is `http`, `mcp`, `python` or `r`. R installs the local package into a temporary
library. Every task/repetition gets a new host, browser profile, library, cache and generated
inputs. It never connects to a personal library. The chosen free port is only a hint: startup
must produce the new library's own connection file; it never adopts an existing connection.

`--tasks population,denominator` selects a documented subset; omitted tasks remain listed.
`--variant 1` (1–100) multiplies positive fixture counts by that integer. `--repeat 1` (1–10)
repeats the same fixture in fresh hosts. Neither creates independent biological observations.
The JSON records task attempts, errors, answer/evidence/permission grades, exact native text,
raw transport responses, hashes, source revision/dirty state, binary and benchmark/client
fingerprints, runtime versions and explicit permission policy. A self-contained HTML report
shows separate numerator/denominator scores and expandable evidence in either system theme.
Outputs use exclusive creation and private file permissions; choose a private destination if
recording a candidate's responses. The report is an evaluation record, not a replay certificate
or an authenticated log. It may contain generated temporary paths and candidate-supplied text.

A failing reference, unexpected empty-control pass, or run error returns a nonzero exit status.
For an external candidate, ordinary task failures are recorded scores, not a runner failure;
inspect every grade. Setup/build failures stop the run before tasks, or become failed task
attempts when encountered during a task. Tasks are never silently dropped or retried.

## Explicit permissions and review

Both `--allow edits,compute,files` and `--review scripted` are mandatory. The host is offline;
network is not granted. Setup reviews only its exact new-project, fixture-import and annotation
proposals. Candidate calls are restricted to `get_state`, `query_clonotypes`, `set_definition`
and `open_example`; the last is the intentionally denied network probe. Other actions receive
`benchmark-policy` without forwarding and fail permission grading.

For the definition task, the test reviewer accepts exactly one
`set_definition({preset:"Amino-acid junction only"})` proposal. For the rejection task it rejects
exactly one `set_definition({preset:"Nucleotide junction"})`. It verifies the visible caller,
action title and exact arguments before clicking the normal window button. Unexpected proposals
are rejected and fail the run. This fixture reviewer is not exposed to candidates or added to
the application. Human approval in ordinary research use remains required.

The harness checks original input bytes, unchanged grants, project equality for read/refusal
tasks, and exact Undo/Redo restoration for the accepted edit. Setup and post-run checks are
separate from scored candidate calls. Undo checks project state, not deletion of cache/files.

## Optional trusted candidate program

Only `--agent command --command-file /path/to/argv.json` starts a candidate. The file is a JSON
array of executable and arguments, not a shell string. `--label` records operator-supplied model,
version and configuration information; it does not authenticate them. Record any model revision,
sampling settings, system prompt and provider cost separately when doing an actual evaluation.

**The candidate is a trusted local program with the invoking user's OS privileges and inherited
environment.** Host grants apply to ClonoLoom actions, not to the candidate's filesystem, subprocess
or network access. This runner is not a sandbox for untrusted agents, an exfiltration test, or a
prompt-injection benchmark. It does not provision credentials or invoke a model service for you.

The process receives one JSON line on stdin:

```json
{"type":"task","id":"population","prompt":"…","answerShape":{"clonotypes":"integer"},"tools":[],"permissions":{"read":true},"reviewPolicy":{"maximum":0}}
```

The real message contains the complete task prompt, answer shape, permitted live tool schemas,
grants and exact fixture review policy. It contains no truth object or reference call sequence.
Write a call as one JSON line on stdout and await its reply before sending another:

```json
{"id":1,"action":"query_clonotypes","args":{"limit":1000}}
```

stdin replies with `{"id":1,"response":{…}}`. Native `data.resultJSON` is original Go text.
Keep that string intact. Successful Python/R calls retain the client's original response bytes;
refusals are explicit typed-client exception records, not invented native response bytes.
When finished, write one `{"answer":{…}}` line, consume stdin EOF and exit successfully.
stdout is only protocol; stderr is diagnostic (bounded, not saved in the report).

There are at most 16 sequential calls, 120 seconds for the candidate process, 1 MiB each of
stdout and stderr, and 30 seconds per host request/idle wait. Responses are capped at 16 MiB.
Duplicate IDs, concurrent calls, malformed messages, extra answers, incomplete output, nonzero
exits, oversize output and timeouts fail explicitly. Arguments/answers must use canonical JSON
number tokens; rounded decimal tokens and unsafe integers are refused before forwarding.
The exactly representable integer 9007199254740992 is allowed only as the designated host-refusal
probe; it is outside the host's consecutive exact-integer range. The adapter is not a replacement
for the general precision-preserving Python/R clients. A deadline terminates the direct child;
it does not claim containment or termination of descendants of an arbitrary trusted program.

## Validate the implementation

```sh
go build -o /tmp/clonoloom-benchmark .
node validation/fetch.mjs axe-core
node validation/agent-benchmark.mjs /tmp/clonoloom-benchmark /tmp/new-benchmark-validation
# Add --capture to refresh docs/images/agent-benchmark-{light,dark}.webp.
```

This checks all eight reference tasks on four transports, eight no-action controls, three
scripted JSONL adapter controls, and three tasks twice with scaled counts. Byte parity across
transports is separate from independent arithmetic grading. Both themes and desktop/phone
layouts are checked with axe-core. An additional live negative control verifies that unexpected
proposals and actions outside the fixture policy fail even with correct answers and native
evidence. `reference-agent.mjs` is the scripted adapter control, not
an autonomous agent. CI runs these controls without any model invocation.

After changing only report presentation, recheck a saved control without rerunning actions:

```sh
node validation/agent-benchmark-report.mjs /tmp/new-benchmark-validation/http.json --capture
```

This verifies recorded grades and evidence hashes, regenerates the HTML from the unchanged JSON,
and checks desktop/phone layouts in both themes. It is a development check, not a signature or
verification of an external agent's identity.
