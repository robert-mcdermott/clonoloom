# Remote control

A program on your computer (a script, a notebook, the documentation's screenshot capture) can
drive the open ClonoLoom window: open data, choose a clonotype definition, show a view, write a
dataset's methods paragraph. Each action is performed in the window, where you see it, and what it
changes goes into the project's history like any edit of yours, so it can be undone.

## Turn it on

```sh
clonoloom --remote-control
```

ClonoLoom then writes `remote.json` to its data folder (`~/Library/Application Support/ClonoLoom`
on macOS, `~/.config/ClonoLoom` on Linux, `%AppData%\ClonoLoom` on Windows, or `--data-dir`):

```json
{ "url": "http://127.0.0.1:8860", "token": "…", "version": "0.2.0", "pid": 12345 }
```

The file is readable by you only, and removed when ClonoLoom stops. Keep a ClonoLoom window open:
actions are performed by the window most recently opened.

## Send an action

POST `{"action": …, "args": {…}, "client": "what your program calls itself"}` to
`/api/remote/action`. The answer is `{"ok": true|false, "message": …, "data": …}`.

```sh
curl -s http://127.0.0.1:8860/api/remote/action -d '{"action": "get_state", "client": "my script"}'
```

```python
import json, pathlib, urllib.request

info = json.loads(pathlib.Path("~/Library/Application Support/ClonoLoom/remote.json").expanduser().read_text())

def act(action, **args):
    request = urllib.request.Request(info["url"] + "/api/remote/action",
        data=json.dumps({"action": action, "args": args, "client": "my notebook"}).encode(),
        headers={"Content-Type": "application/json", "X-ClonoLoom-Token": info["token"]})
    return json.load(urllib.request.urlopen(request))

act("open_data", paths=["/data/study/airr_rearrangement.tsv", "/data/study/repertoires.airr.json"])
act("set_definition", preset="Nucleotide junction")
print(act("show_methods")["data"]["text"])
```

`GET /api/remote/tools` lists every action with its arguments as JSON Schema.

| Action | What it does |
| --- | --- |
| `get_state` | The project: its datasets (format, unit, rows, samples, findings, filter, definition, license and citation of public data), the view shown, the example datasets. |
| `new_project` | Starts a new project (`name`). |
| `open_data` | Opens files or folders as one dataset (`paths`, `name`). Needs the token. |
| `open_example` | Opens an example dataset (`example`, `opening`); `download: true` downloads openly licensed data first. |
| `show` | Shows a view: `start`, `data`, `qc`, `clonotypes` or `jobs` (`view`, `dataset`). |
| `set_definition` | Chooses a dataset's clonotype definition: a `preset` by name, or a `definition` in full. |
| `select_clonotype` | Chooses a clonotype by its `rank` in the table or its `id`. |
| `show_examples` | Opens the Example data dialog. |
| `show_methods` | Shows a dataset's methods paragraph and answers it with its references. |
| `set_theme` | `light`, `dark` or `system`. |
| `close_dialogs` | Closes dialogs, menus and messages. |

## What is allowed

- Only programs on this computer: requests from other computers, and web pages that reach the
  loopback port under another name, are refused.
- Reading files by path (`open_data`) needs the `X-ClonoLoom-Token` header with the token in
  `remote.json`, so another user of a shared computer cannot make your ClonoLoom read your files.
- Data under a provider's terms of use (the immuneACCESS example) are never downloaded by a
  program: only you can accept their terms, in the window.
- One action runs at a time, in the order they come; a page opened from an earlier run of
  ClonoLoom does not take actions (reload it).

Agents (an MCP server whose changes you accept or reject) come in a later release.
