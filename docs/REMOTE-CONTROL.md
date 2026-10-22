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

# A VDJtools table, through a mapping of its columns.
table = "/data/vdjtools/patient7_day0.txt"
act("open_data", paths=[table], mappings={table: {
    "columns": {"junction_aa": "cdr3aa", "junction": "cdr3nt", "v_call": "v", "j_call": "j", "count": "count"},
    "unit": "reads"}})
```

`GET /api/remote/tools` lists every action with its arguments as JSON Schema.

| Action | What it does |
| --- | --- |
| `get_state` | The project: its datasets (format, unit, rows, samples, findings, filter, definition, license and citation of public data), the view shown, the example datasets. |
| `new_project` | Starts a new project (`name`). |
| `open_data` | Opens files or folders as one dataset (`paths`, `name`). Another tool's CSV or TSV opens through a mapping of its columns, by path in `mappings`: `{columns: {junction_aa, junction, v_call, d_call, j_call, c_call, count, sample, locus, productive, cell_id, clone_id, sequence_id: a column's name}, unit: reads, umis, templates or cells (with a count), sample: one name for every row}` ([FORMATS.md](FORMATS.md#tables-with-mapped-columns)). Needs the token. |
| `open_example` | Opens an example dataset (`example`, `opening`); `download: true` downloads openly licensed data first. |
| `show` | Shows a view: `start`, `data`, `qc`, `clonotypes`, `repertoire` or `jobs` (`view`, `dataset`). |
| `set_definition` | Chooses a dataset's clonotype definition: a `preset` by name, or a `definition` in full. |
| `select_clonotype` | Chooses a clonotype by its `rank` in the table or its `id`. |
| `measure_diversity` | Measures each sample's diversity (a job; the result kept) and answers the main measures with their 95% intervals (`dataset`). |
| `gene_usage` | Counts each sample's genes, junction lengths and amino acids (a job; the result kept, the samples grouped by condition) and answers, for each sample or condition, the most used genes of a segment, the commonest lengths and the mean length, with their 95% intervals (`dataset`; `part`: `genes` or `junctions`; `segment`: `V`, `D`, `J` or `C`; `level`: `gene` or `family`; `weight`: `clonotypes` or `abundance`; `rows`: `samples` or `groups`). |
| `measure_overlap` | Measures every pair of samples' overlap (a job; the result kept, the samples' subjects and conditions from their annotation or metadata) and answers the pairs of one subject and of two summarized, the most overlapping pairs by a measure with their 95% intervals, and the clonotypes by how many subjects hold them (`dataset`; `measure`: `morisita`, the default, `morisitaHorn`, `jaccard`, `jaccardRarefied`, `sorensen`, `sorensenRarefied`, `overlap`, `overlapRarefied`, `shared`, `sharedRarefied` or `abundance`; `pairs`: how many). |
| `rarefy` | Rarefies and extrapolates each sample (a job; the result kept) and answers the levels the samples are best compared at and each sample's clonotypes, Shannon and Simpson diversity at one of them, with their 95% intervals (`dataset`; `level`: `coverage`, the default, `size`, `extrapolatedCoverage` or `extrapolatedSize`). |
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
