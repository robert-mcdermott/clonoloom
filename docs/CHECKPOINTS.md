# Analysis checkpoints (Wave 8 development)

Reports can keep a named, immutable **local checkpoint** before you revise a study. It preserves
recorded analysis settings and attached completed result files. Recovery restores that evidence;
it does not recompute results, resume an interrupted worker or verify original source files.
Published 0.7.0 binaries predate this development feature; version remains 0.7.0 until release preparation.

## Create, compare and recover

1. Open **Reports → Analysis checkpoints → Create checkpoint** and name the analysis state.
   Every attached result must be available, complete and internally checksummed. An unavailable,
   changed, external-output or unsupported-format result refuses the whole checkpoint.
2. Select a checkpoint and compare it with **Current analysis** or another checkpoint in the
   same project. Expand each difference to see the exact before/after JSON. Export the full
   host-generated difference JSON if needed; the browser does not re-encode its numbers.
3. Select **Recover checkpoint**. The host verifies every checkpoint file and preflights every
   existing result destination. Missing results are installed from the saved bytes; conflicts
   are refused without overwriting anything. The browser then replaces analysis values through
   the ordinary undo store. **Undo** restores the preceding analysis; **Redo** reapplies recovery.
4. Save/reopen normally. Checkpoint references persist in the project and their immutable
   evidence remains in this computer's library. For a supported result, use its ordinary
   [certificate replay](CERTIFICATES.md) to recompute from captured evidence after recovery.

Changing analysis settings does not retroactively update a saved result. Its original parameters,
method, populations, notice and provenance remain in its own evidence. Run a new analysis to
produce a result under revised settings. A checkpoint does not select a statistically appropriate
threshold, background, population or study design for you.

## What is preserved

A checkpoint stores exact project JSON and copies **only files inventoried by the manifests of
directly attached completed results**, including each original manifest. The result's captured
source/provider notices remain in those unchanged files. There is no scientific JSON conversion.
This is private local persistence, not permission to redistribute provider data.

Recovered analysis includes datasets and recorded source bindings, expression records, definitions,
QC versions/selections, sample annotations, cohort designs, saved views, decisions, attached-result
references and other analysis fields in the supported project envelope. Recovery preserves the
current project ID, name, creation time, history and checkpoint references, updates its modification
time and appends a recovery history entry. Thus the **whole current project JSON is not replaced
byte for byte**; its analysis values and inventoried result bytes are restored. The original full
project bytes remain in the checkpoint as inspectable evidence. Older minimal version-1 projects
receive the existing empty defaults for datasets, artifacts, decisions and views; the original
project bytes are still retained unchanged.

The browser refuses recovery if decoding/re-encoding an analysis would change a JSON numeric token
(for example an integer beyond its exact range). Go comparisons and saved result files retain those
tokens exactly. Recovery, undo and redo invalidate pending analysis callbacks; an operation whose
project or analysis changed while it was running cannot overwrite the newer state.

Creation and recovery have normal undo entries. Undoing creation removes its reference, not its
immutable evidence directory. Undoing recovery changes project values, not immutable artifact files.
Checkpoints are not deleted, renamed or overwritten by this interface. Keep the whole local library
backed up; a project JSON alone does not carry checkpoint evidence. Portable transfer belongs to the
[portable archive contract](PROJECT-ARCHIVES.md). Archives preserve checkpoint originals and
apply verified local source bindings during recovery; later local checkpoints record those
bindings normally. The current portable archive marker survives recovery.

## Read differences precisely

Go compares analysis JSON and attached results' recorded kind, parameters, method, parents,
manifest fingerprint and output inventory. Descriptions group changes into sources, definitions,
QC, populations/annotations, cohort design, saved views, result identities and other settings.
They are **descriptive changes**, not estimated biological effects or significance tests.

Each entry contains `category`, JSON-pointer `path`, `summary`, `beforeJSON` and `afterJSON`.
The last two are strings holding exact JSON values, so they preserve numeric tokens. `(absent)`
is a missing field, distinct from JSON `null` or a literal string containing that word. Array
indices start at zero; ordering matters. Object key order and insignificant whitespace do not
create analysis differences; full-file hashes still identify exact stored bytes. Numeric spellings
such as `1` and `1.0` are distinct tokens, without tolerance-based equivalence.

The report has format `clonoloom-checkpoint-diff`, version 1, before/after identities, complete
`changes` and `limits`. More than 4096 changes refuses the comparison instead of truncating it.
History, project naming/timestamps and checkpoint references are outside analysis differences.
Original source paths and hashes are compared as recorded values; the sources are never opened
by this operation. Unchanged settings do not prove unchanged external data or scientific validity.

## Storage, bounds and recovery limits

The library holds `checkpoints/cp-<SHA256>/checkpoint.json`, `project.json` and
`artifacts/<artifact-id>/<inventoried-path>`. The record has format `clonoloom-checkpoint`,
version 1, name, creation time, project identity, software, sorted file inventory and limits.
The ID hashes Go's compact JSON record with `id` set to the empty string; each inventory item
contains relative `path`, byte `size` and `sha256`. The record does not inventory itself.
This is a versioned ClonoLoom encoding, not a signature or a general canonical-JSON standard.
A coordinated replacement of all evidence and hashes can create a new internally consistent
checkpoint; retain the expected ID separately when its identity matters.

Requests/project JSON are bounded to 16 MiB, with strict duplicate-key checking and JSON nesting
at most 64. At most 128 directly attached results, 256 inventoried files, 256 MiB per result file,
64 MiB per result manifest and 512 MiB total are accepted. Checkpoint record metadata is at most
1 MiB. The host requires supported complete artifact v1 manifests and local files; file links,
unsafe paths, changed hashes, missing files, future versions and cross-project recovery are refused.
An unsupported scientific replay kind can be stored as evidence when it meets this local artifact
contract; it does **not** gain replay support. See the [replay support matrix](HEADLESS.md#supported-workflows).

Staged files are synced and their completed directory is promoted only after successful writes.
Cancellation removes staging files during ordinary cleanup. Recovery checks all destinations before
installing missing directories; an existing partial/corrupt result is a conflict, not silently repaired.
Cancellation, a concurrent destination appearing, a force-kill or power loss can leave staged files
or verified, unreferenced restored results. Retry after inspection is idempotent; the project analysis
is committed only after the complete recovery request succeeds. This is not a transactional
whole-library backup or a guarantee against filesystem/hardware failure.

Not captured or restored:

- Original receptor/expression files, their physical availability, caches or source-reading state.
- Separate parents not directly attached to the project, installed reference datasets and reusable
  library records outside the project. Their recorded identities may remain in saved evidence.
- Running jobs, transient selections, unsaved panel drafts or a process execution position.
- New scientific calculations, model calibration, data de-identification or redistribution rights.

Exact scientific replay remains a separate operation for the five supported certificate kinds.
Its source-capture and method limits remain unchanged. Checkpoint copying alone never produces a
`replay-match` verdict. Identifiers, sequences and paths may remain in the evidence.

## API and shared actions

The existing protected local HTTP host accepts:

- `POST /api/checkpoints` with `{name, project}` → immutable checkpoint record.
- `POST /api/checkpoints/{id}/compare` with `{project, against?}` → exact difference report.
- `POST /api/checkpoints/{id}/recover` with `{project}` → checkpoint record, `analysisJSON` string,
  `recomputed: false` and limits, after verifying/reinstalling attached result files.

These routes do not save the mutable project themselves: the visible application commits the
returned recovery through its undo store. Malformed requests return 400; missing, corrupt,
conflicting or incompatible evidence returns 409. No partial success report is returned.
Request cancellation follows the existing host context.

The shared action catalog exposes `create_checkpoint {name}`, `compare_checkpoint {checkpoint,
against?}` and `recover_checkpoint {checkpoint}`. They use the same visible Reports operations.
MCP and generated clients remain later slices; these actions do not add arbitrary paths or shell
execution. See research §5.45 for rationale and validation boundaries.
