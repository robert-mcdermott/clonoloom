# AI agents (MCP)

Use an AI assistant to inspect repertoire data, propose analysis settings, run ClonoLoom's
methods and retrieve the recorded results. The assistant works through the same local research
window you use. You review proposed changes there before they execute.

ClonoLoom **0.8.0 or later** includes the MCP server in the normal executable; no separate MCP
module, Python package or Node.js server is needed. Action API **2**, catalog **4** (64 actions)
and automation policy **1** are independently versioned. This guide covers local stdio clients.

## What an agent can do

With the corresponding startup grants, an agent can:

- open local repertoire files and inspect source units, QC findings and clonotype definitions;
- query clonotypes without changing their full-table abundance denominators;
- propose definitions and sample annotations, measure diversity, rarefaction and overlap, and
  run the supported cell, B-cell and cohort workflows;
- retrieve methods and references, create or compare checkpoints, and export supported saved
  results as certificates or self-checking reports;
- inspect and transfer portable project archives under the existing source-inclusion rules.

The [action reference](REMOTE-CONTROL.md) describes arguments and scientific scope. Tool discovery
shows only the tools allowed by the host's grants. Agent availability is not a claim that every
analysis is appropriate for your study; check replication, units and method assumptions.

## Choose the program, project storage and dataset

These are three separate paths:

| Path | Example | Purpose |
| --- | --- | --- |
| Executable | `/Users/you/.local/bin/clonoloom` | The installed ClonoLoom program that the client starts |
| Project storage (`--data-dir`) | `/Users/you/clonoloom-projects` | Persistent saved projects, results, checkpoints and jobs; can hold multiple projects |
| Dataset | `/Users/you/data/study/airr_rearrangement.tsv` | An input file opened later through the reviewed `open_data` tool |

**`--data-dir` is not a dataset path or a software-library/module path.** ClonoLoom creates its
storage directory if needed. Source repertoire files stay where they are; projects record their
paths and fingerprints. Choosing a new empty storage directory will not display projects from
another directory. To use an existing library, stop the ClonoLoom instance using it, then point
the MCP configuration at that library and open the saved project in the window. Alternatively,
transfer a project using a [portable archive](PROJECT-ARCHIVES.md).

Use one host per storage directory. MCP starts its own host; it does not attach to an already
running application. Use separate storage directories for simultaneous agents. Omitting
`--data-dir` uses the normal application library, so the examples choose an explicit directory.

[Install ClonoLoom](INSTALLING.md) first. On macOS/Linux the installer normally places the program
at `~/.local/bin/clonoloom`; on Windows it uses `%LOCALAPPDATA%\Programs\ClonoLoom\clonoloom.exe`.
Use an absolute executable path. Shell examples below expand `$HOME`; JSON/TOML examples require
you to replace `/Users/you/...` with real absolute paths. Do not assume `~`, `$HOME` or `%...%`
expands inside JSON. Windows JSON paths may use forward slashes, for example
`C:/Users/you/AppData/Local/Programs/ClonoLoom/clonoloom.exe` and `C:/Users/you/clonoloom-projects`.

## Connect a client

Choose **one** of the following setups. They enable local data access, computation and reviewed
edits with `--allow edits,compute,files`. Remove that option and its value for read-only state
inspection (`get_state`). Grants apply to every tool call from that process; restarting the
server is required after changing them. Your AI client's tool approval is separate from
ClonoLoom's in-window proposal review.

### Claude Code

Run in a macOS/Linux shell:

```sh
claude mcp add --transport stdio --scope user clonoloom -- \
  "$HOME/.local/bin/clonoloom" mcp \
  --data-dir "$HOME/clonoloom-projects" --allow edits,compute,files
claude mcp get clonoloom
```

Open Claude Code and use `/mcp` to check the connection. User scope makes the configuration
available across your coding projects; it does not choose a ClonoLoom dataset. See the
[Claude Code MCP guide](https://code.claude.com/docs/en/mcp) for scopes and connection management.

### Claude Desktop

Open **Settings → Developer → Edit Config**. Merge this entry into `mcpServers` in
`claude_desktop_config.json`, preserving other configured servers, then restart Claude Desktop:

```json
{
  "mcpServers": {
    "clonoloom": {
      "command": "/Users/you/.local/bin/clonoloom",
      "args": [
        "mcp",
        "--data-dir",
        "/Users/you/clonoloom-projects",
        "--allow",
        "edits,compute,files"
      ]
    }
  }
}
```

See the [official local-server setup guide](https://modelcontextprotocol.io/docs/develop/connect-local-servers).
Use the local executable configuration, not a connector asking for a remote URL.

### Codex

Register the local server with the CLI:

```sh
codex mcp add clonoloom -- "$HOME/.local/bin/clonoloom" mcp \
  --data-dir "$HOME/clonoloom-projects" --allow edits,compute,files
codex mcp list
```

For review and longer computations, set `tool_timeout_sec = 660` in the existing
`[mcp_servers.clonoloom]` section of `~/.codex/config.toml`. The equivalent complete entry is:

```toml
[mcp_servers.clonoloom]
command = "/Users/you/.local/bin/clonoloom"
args = [
  "mcp",
  "--data-dir", "/Users/you/clonoloom-projects",
  "--allow", "edits,compute,files"
]
tool_timeout_sec = 660
```

Use either registration or the complete entry, without duplicating the table. Restart the client
connection after edits. This client timeout does not extend ClonoLoom's own ten-minute action
limit. The [Codex MCP guide](https://developers.openai.com/codex/mcp) covers configuration and
`/mcp` in the CLI.

### Cursor

Put the following in `~/.cursor/mcp.json` for user-wide access, or `.cursor/mcp.json` for one
coding project. Merge with existing servers and enable it in Cursor's MCP settings:

```json
{
  "mcpServers": {
    "clonoloom": {
      "type": "stdio",
      "command": "/Users/you/.local/bin/clonoloom",
      "args": [
        "mcp",
        "--data-dir",
        "/Users/you/clonoloom-projects",
        "--allow",
        "edits,compute,files"
      ]
    }
  }
}
```

See [Cursor's MCP configuration](https://docs.cursor.com/context/model-context-protocol).

### VS Code

For VS Code's local agent harness, run **MCP: Open User Configuration**, or use
`.vscode/mcp.json` in the coding workspace. Its configuration uses a `servers` object:

```json
{
  "servers": {
    "clonoloom": {
      "type": "stdio",
      "command": "/Users/you/.local/bin/clonoloom",
      "args": [
        "mcp",
        "--data-dir",
        "/Users/you/clonoloom-projects",
        "--allow",
        "edits,compute,files"
      ]
    }
  }
}
```

Use **MCP: List Servers** to start it and inspect output, then enable its tools in agent chat.
Other VS Code harnesses can use different configuration stores; follow the
[VS Code MCP reference](https://code.visualstudio.com/docs/agents/reference/mcp-configuration)
for the selected harness. Keep ClonoLoom, the browser window and files on the same computer for
these examples; a remote development environment has different executable and file paths.

## Try a first task

First ask the assistant:

> Use ClonoLoom's get_state tool. Tell me which project and datasets are open, their source
> units, and whether any jobs are running. Do not change anything.

The client starts ClonoLoom; the normal window opens on the first permitted tool call. An empty
new library is expected. Then use the absolute path of one of your local files:

> Use ClonoLoom to open /Users/you/data/study/airr_rearrangement.tsv. I will review the proposal
> in its window. Summarize the source units, QC findings and current clonotype definition.
> Query the ten most abundant clonotypes and report frequencies using the full table
> denominator. Keep the analysis descriptive; do not infer antigen specificity or a treatment
> effect from rank or abundance.

Open data through `open_data`, not as positional arguments to `clonoloom mcp`. Wait for each
reviewed action to finish before requesting the next. To try artificial data, open an example
manually in this MCP-owned window, then ask the agent to inspect it. The `open_example` tool
requires the additional `network` grant even for a local example because its declared effects
cover downloadable examples too. Add `network` only when that workflow is intended; `--offline`
still blocks ClonoLoom downloads. Provider acknowledgements remain separate.

For a complete scientific workflow, follow the [record study](RECORD-STUDY.md). The
[agent benchmark](AGENT-BENCHMARK.md) uses artificial data with known answers; its scripted
controls validate the harness, not general model competence.

## Review and troubleshooting

Project edits wait for a person to accept or reject their exact arguments in the window. No
MCP tool can accept them. Inspection and some exports do not need a proposal; the action catalog
specifies this per tool. Accepted project changes appear in History and use normal Undo/Redo;
Undo does not delete completed exports or downloaded files.

| Symptom | What to check |
| --- | --- |
| Server cannot start | Confirm the absolute executable path and `clonoloom --version`; inspect the client's server log |
| Only `get_state` is available | Add the required `--allow` capabilities, then restart the server and refresh its tools |
| No familiar datasets | Check `--data-dir`; open a saved project in that library or ask to open a dataset by its separate path |
| Another host uses the directory | Stop that host before reconnecting, or choose a different storage directory |
| A call waits or times out | Keep the MCP-owned window open and inspect its proposal; client timeouts may be shorter than ClonoLoom's limits |
| No window opens | Try `--window browser`; with `--window none`, open the local address in the client's stderr log |
| Busy or stale proposal | Finish/cancel existing jobs or imports, inspect current state, then submit a new proposal |
| Protocol version rejected | Use a client supporting one of the two modes below; other revisions are not claimed compatible |

Tool discovery does not open a window. The most recently connected window receives actions.
A window connection wait is bounded to 40 seconds. Closing the client's stdin stops its owned
host and cancels pending calls; closing the window alone does not stop MCP. A best-effort
live-host check is not an interprocess lock and cannot detect every concurrent launch.

## Privacy and the agent's model

The MCP server and scientific computations run locally. The assistant can send tool results
and your prompts to its configured model provider. Those results can include receptor sequences,
annotations, identifiers and scientific summaries. Choose a client/model configuration suitable
for the data you intend to expose. `--offline` controls ClonoLoom's downloads, not the AI client's
network use. Capability grants restrict ClonoLoom actions; they are not an OS filesystem sandbox.

The server is a stdio subprocess, not a network MCP endpoint. stdout is reserved for JSON-RPC;
diagnostics and the local window address go to stderr. Configuration examples follow the linked
client documentation; they do not imply that every client/version has been exercised end to end.

## Protocol versions

The implementation follows the official [2026-07-28 versioning rules](https://modelcontextprotocol.io/specification/2026-07-28/basic/versioning),
[stdio transport](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/stdio)
and [tools contract](https://modelcontextprotocol.io/specification/2026-07-28/server/tools),
checked on 2026-10-28. It supports two explicit modes:

| Mode | Start | Request identity |
| --- | --- | --- |
| 2026-07-28 | `server/discover`; no initialization handshake | Each request supplies its own protocol version and client capabilities in `_meta`; optional client info never inherits another request's identity |
| 2025-11-25 | `initialize`, then `notifications/initialized` | The initialized client label is used only for legacy requests without modern protocol metadata |

Modern example (send as one line):

```json
{"jsonrpc":"2.0","id":"state-1","method":"tools/call","params":{"name":"get_state","arguments":{},"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{},"io.modelcontextprotocol/clientInfo":{"name":"Research notebook","version":"1"}}}}
```

Missing modern metadata gives `-32602`; an unsupported per-request version gives `-32022`
with supported/requested versions. Modern results include `resultType: "complete"` and server
info; discovery and tool lists have private, zero-duration cache scope. Bad modern metadata
never falls back to an earlier client's identity. Legacy initialization negotiates 2025-11-25
(the client must stop if it cannot support the returned version), following the
[legacy lifecycle](https://modelcontextprotocol.io/specification/2025-11-25/basic/lifecycle).
Other handshake revisions are not claimed as supported. `ping` is available in both modes.

Only tools are advertised. There are no resources, prompts, sampling, elicitation, subscriptions,
tasks or server-initiated requests. A tool execution failure is `isError: true`; malformed calls
and unknown tools use protocol errors. The catalog fits one page; omit `cursor`.

## Permissions and review

`tools/list` derives from the shared action catalog and includes only actions whose complete
conservative effect set is granted. Calling a hidden action by name still fails. Each tool's
`_meta["org.clonoloom/automation"]` records effects, review, API, catalog and policy versions.
These describe the host policy; client capability metadata does not grant application effects.

The stdio process uses its launch grants without passing a token in every tool argument. Its
HTTP endpoint still requires the local token for non-read calls. Both transports share argument
validation, grants, window dispatch and the last-500-event session audit. Caller names are
self-reported labels, not authenticated identities. This is not an operating-system sandbox.

An edit proposes exact arguments against the current project. A person accepts or rejects it
in the window. No MCP tool can accept proposals. Stale proposals, existing jobs/imports and
cancelled callers are refused under the same [automation policy](AUTOMATION.md). Accepted edits
record the caller in History and use normal Undo/Redo. Provider acknowledgements remain separate.

![Reviewing a proposal from an MCP notebook](images/mcp-light.webp)

The example uses simulated vaccination repertoires from one subject at three observations. It
shows a proposed definition change, not evidence for a biological treatment effect. Query
filtering changes membership and displayed rank; it preserves original abundance and the full
clonotype-table denominator under the current definition and QC rules.

## Results, progress and cancellation

A successful call contains the same `{ok, message, data}` envelope as HTTP, serialized in a text
content block and in `structuredContent`. The transport never parses native numeric tokens
through a floating-point map. `query_clonotypes` also retains native JSON in `data.resultJSON`.
Consumers must use a precision-preserving JSON decoder for exact integers/decimal tokens;
JavaScript `JSON.parse` alone cannot represent all such values. Native certificate bytes remain
the authority for exact replay, not reserialized client objects.

Optional `params._meta.progressToken` requests a heartbeat every five seconds. Its increasing
number measures elapsed seconds waiting for the window, review or action, not percent complete
or a scientific worker's progress. Tokens must be unique among active calls. See official
[progress](https://modelcontextprotocol.io/specification/2026-07-28/basic/patterns/progress).

Send `notifications/cancelled` with `params.requestId` to cancel. After processing cancellation,
the server emits no more results/progress for that request (already-written messages may still
arrive). EOF cancels all pending calls. The window receives cancellation and refuses later
attachments; existing files/artifacts/downloaded bytes can remain. There is no automatic retry,
rollback or deletion. Reinspect state and History before submitting again. See official
[cancellation](https://modelcontextprotocol.io/specification/2026-07-28/basic/patterns/cancellation).

| Bound | Behavior |
| --- | --- |
| 2 MiB incoming frame, 1 MiB action arguments | Refuse; drain oversized lines so later messages remain readable |
| 16 active calls | Refuse excess calls; the shared window dispatcher executes one at a time |
| 65,536 request IDs per process | Stop at the limit; reconnect with a new process |
| String IDs/tokens at most 512 encoded bytes, or integers within ±(2^53−1) | Reject null/fractional/unsafe IDs; IDs cannot be reused in a session |
| 16 MiB outgoing tool response, including both content representations | Explicit `output-too-large` error; no truncated success; request fewer rows or native exports |
| 40-second window wait; 10-minute window action timeout | Explicit failure; capability and argument refusals happen before opening a window |

Dependent edits must wait for the preceding result; concurrent call order is unspecified.
Tool names are limited to 128 bytes. Batches are unsupported. Duplicate JSON keys and invalid UTF-8 are refused. Output writes are
serialized; write failures end the transport. File-backed stdout uses a ten-second write deadline
where the OS supports it. A response-size failure can occur after the action completed; do not
blindly retry. These bounds are ClonoLoom implementation choices, not scientific thresholds.

## Scientific boundary

MCP adds no estimator, embedding, plot, tolerance or scientific replay kind. It invokes the same
Go methods and reviewed project operations as HTTP and the application. Definition, QC, units,
denominators, seeds and provenance retain their existing meanings. The five supported exact
certificate replay kinds and their captured-input limits remain unchanged; see
[Certificates](CERTIFICATES.md). Protocol success, explicit permission and successful replay do
not establish biological validity, model calibration or receptor specificity.
