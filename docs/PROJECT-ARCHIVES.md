# Portable project archives (Wave 8 development)

Slice 10 adds version 1 `.clz` archives. Use the development host: published v0.7.0
predates this feature. Application/package versions remain 0.7.0; automation uses API 2,
**catalog 3**, policy 1 (63 actions). Regenerate/update both clients with this host.

## Transfer a study

1. Open **Reports → Portable project archives**, also reachable from the project menu.
2. Choose whether to include available source files. Inclusion is off by default. For sources
   without a recorded provider license, explicitly confirm your authority to include them.
   Restricted or unknown-provider sources are never bundled as original files. Captured
   restricted evidence additionally requires the private-use acknowledgment.
3. Select **Export project archive**. Keep its SHA-256 fingerprint separately if it will be
   used to verify a transferred copy. The download preserves the current project, history,
   attached results, available result parents and checkpoint closure.
4. On the receiving computer, select **Inspect project archive**, or open/drop a `.clz` file.
   Supply the independently retained fingerprint if available. Inspect the complete inventory,
   source statuses and scientific limits. Inspection imports nothing and runs no recipes.
5. Select **Import inspected project**. Import verifies the fingerprint again, installs local
   evidence and opens a working project with verified source bindings. Project identity is
   preserved. An existing project of that identity refuses import; use a separate library or
   explicitly move the existing project to trash first. Imports do not invent a new identity.
6. **Download original project evidence** retrieves the unchanged source document. The working
   project has local source paths, `archiveStatus` fields and a `portableArchive` fingerprint;
   its original history and scientific settings are retained. Checkpoint recovery uses verified
   portable bindings while preserving the original checkpoint files.

A checkpoint taken after import records its local bindings normally. Re-export can include the
verified sources of older imported checkpoints even when their original machine paths no longer
exist. Moving just the working JSON does not transfer its imported bundle: export a new archive.

## Sources and permissions

| Status | Meaning |
| --- | --- |
| `included` | Byte count and SHA-256 match; inclusion was requested and source policy permits it. |
| `omitted` | Inclusion was not requested, or authority for unlicensed user-owned sources was not asserted. |
| `restricted` | Recorded/provider-identified terms are restricted or unknown; no original source bytes included. |
| `missing` | No binding, missing file, unreadable input, link or non-regular input. |
| `changed` | Available bytes differ from the source fingerprint in the original project. |
| `unsupported` | Missing/oversized fingerprint metadata or filename outside the portable path contract. |

Omitted sources receive an **empty local path**, never a usable original-machine path. Their
fingerprints and original names remain. Use the existing locate-source workflow to identify
matching receptor files; expression inputs may require explicitly reopening their matching
source set. An archive never searches for or opens the original paths during import. Reader
names, column mappings, IMGT groups and metadata companions are retained. Expression MEX
companions are grouped together; supplied cell annotation paths are rebound separately.

Source inclusion is not a permission inference from a checksum. The user's authority assertion
and recorded licenses do not prove legal permission. Preserve provider citations and notices.
Captured results/checkpoints can contain identifiers, sequences and counts even when original
source files are excluded. Restricted captured evidence remains a private-study export, with an
explicit acknowledgment and NOTICE. Archives are **not de-identified**; review before sharing.

## Exact evidence and scientific limits

Original `project.json`, history inside it, result manifests/files and checkpoint records/files
remain byte-for-byte evidence. Binding edits touch only the working project's documented source
fields and transfer marker. Native JSON numbers, nulls, missing fields and provenance are not
converted to floating point in the archive code. Import/export performs no new scientific
computation and makes no biological claim. The browser refuses to open a project whose numeric
tokens would change in JavaScript; original files remain inspectable, and the native importer
can retain such evidence without browser loading.

Supported project version 1 fixtures from releases v0.2.0–v0.7.0 reopen with the same source
identities and result bytes. This establishes those contracts, not forward compatibility with
unknown versions or every possible external input. Unknown project/archive versions are refused.

Integrity means the recorded members match their inventory. An attacker can replace both
content and hashes; a fingerprint obtained from the same untrusted archive is not authentication.
An independently trusted fingerprint detects a different inventory. Use [evidence certificates](CERTIFICATES.md)
for the separately supported five-kind exact scientific replay. Archive copying neither expands
that support nor proves a result's validity, assumptions, interpretation or upstream pipeline.
No graph, plot, denominator, estimator or replay tolerance changes in this slice.

## Format and bounds

The ZIP contains only regular files:

```text
archive.json                          canonical versioned inventory and fingerprint
project.json                          exact original project, including history
NOTICE.txt                            limits, source statuses, provider citations/licenses
artifacts/<id>/manifest.json           exact attached/available-parent manifest
artifacts/<id>/<inventoried-path>      exact result evidence
checkpoints/<cp-id>/checkpoint.json    exact immutable checkpoint record
checkpoints/<cp-id>/<inventoried-path> exact checkpoint evidence
sources/<group>/<filename>             permitted, fingerprint-verified original bytes
```

`archive.json` records `format: "clonoloom-project-archive"`, version 1, project identity,
software, sorted files (`path`, byte `size`, `sha256`), sources (document, JSON pointer, original
path/name, fingerprint, status, reason, credits and optional entry), result dependencies, restricted
flag and limits. Its fingerprint is SHA-256 of Go's canonical JSON serialization of this record
with `fingerprint` omitted. It identifies this complete inventory, **not the compressed ZIP
bytes**. Export uses deterministic entry order and compression. Original source paths can cause
otherwise equivalent transfers to have different fingerprints.

Bounds: 520 MiB compressed input, 512 MiB inventoried bytes, 256 MiB per member, 4 MiB archive
record, 16 MiB per project, 2,048 payload files, 2,048 source bindings (1,024 per document),
16 checkpoints and 128 global result dependencies. Checkpoints retain their narrower 256-file
contract. Processing holds bounded evidence in memory, with additional staging/destination disk
space; the byte limit is not a promise of a 512 MiB process memory ceiling.

Member names are ASCII relative slash paths, at most 1,024 bytes with components at most 240
bytes. Reject traversal, absolute/drive/UNC paths, backslashes, control characters, Windows
reserved characters/devices, trailing dots/spaces, non-ASCII normalization aliases, duplicate
or case-colliding paths, file/directory collisions, directories, links, special files, encryption,
and compression methods other than Store/Deflate. Unicode inside documents is preserved;
original filenames outside this path contract are reported unsupported. Verify central-directory
sizes, bounded reads/CRC, exact member count, sorted inventories and SHA-256 before installing.
Unknown members, altered inventories, wrong source pointers, mismatched identities and missing
mandatory evidence refuse the whole import.

Mandatory attached artifacts/checkpoints must exist and match. Available artifact parents are
included recursively; missing unattached artifact parents are explicitly listed. Non-artifact
parents remain in original manifests. Installed references, caches, jobs, external output files
and arbitrary paths mentioned in prose/recipes are outside the archive. Manifests with external
outputs are refused. References and original inputs needed for a new analysis must be installed
or supplied separately; missing parents do not imply successful replay.

## Storage and interruption

The receiver keeps the original bundle at `imports/<fingerprint>/` in its library, alongside
normal immutable result/checkpoint locations and `projects/<id>.json`. It preflights all
conflicts, stages bytes locally, uses exclusive creation for missing evidence and publishes the
complete project last without replacing an existing destination. Existing results/checkpoints
must be complete and byte-identical; partial directories are refused. No archive-controlled
link or path escapes extraction. The library is assumed to be owned by the user, not concurrently
rewritten by another process; this is not protection against a malicious local library owner.

Cancellation or failure can leave verified unreferenced files; a process crash can also leave
staging or partial files that require inspection. No whole-library transactional rollback or
hardware-failure guarantee is claimed. An import timeout is an uncertain outcome: inspect the
library before retrying. Undoing a reviewed import changes the open project, not installed files.
No recipes, executable content, references or clients are installed or executed from archives.

## Automation and validation

The shared HTTP/MCP actions and generated clients expose:

- `export_project_archive(includeSources?, ownSources?, accept?)`: read/files grants and human
  review; downloads through the application window. It returns the inventory, not a remote path.
- `inspect_project_archive(path, expected?)`: read/files grants; absolute path local to the host;
  returns inventory, original `projectJSON` text and `recomputed: false`.
- `import_project_archive(path, expected, accept?)`: read/edits/compute/files grants and human review;
  the expected inventory fingerprint is mandatory. Opens the imported working project; source
  reopening can rebuild dataset tables.

Python/R use the generated snake_case argument names. Host grants, review, cancellation and
caller-labelled History remain authoritative. There is no automatic acceptance or retry.

Native routes are `POST /api/project-archives/export` (`{project, includeSources, ownSources,
accept, expected?}`, `?preview=1` for inventory), `/inspect` and `/import`. The latter accept raw
ZIP (`?expect=<fingerprint>&accept=1`) or JSON `{path, expected?, accept}`; import requires an
expected fingerprint. `GET /api/project-archives/<fingerprint>/original` rechecks the stored
bundle before returning original project bytes. These are same-origin application routes;
automation clients use the permission-checked shared actions.

Tests cover deterministic/lexically exact transfers, source policies, missing/changed evidence,
corrupt/hostile ZIPs, destination conflicts, immutable/new checkpoint recovery, re-export and
v0.2–v0.7 projects. `node validation/project-archives.mjs /path/to/clonoloom --screenshots` creates
a seeded artificial vaccination study and two isolated libraries, moves the original sources
away, compares exact native query and evidence bytes, tests rejection/recovery and audits both
themes. Only this validation driver clicks review controls. See research §5.51 for implementation
sources; no new scientific method is introduced.
