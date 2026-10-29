# Headless recipes

ClonoLoom 0.8.0 supports `clonoloom run --recipe FILE --input NAME=PATH --out NEW_DIR`.
It starts no browser or server, executes no shell commands and uses no network. Each invocation
selects one explicit task and uses the existing native workers.

## Supported workflows

| Kind | Source computation | Certificate export / exact replay | Replay boundary |
| --- | --- | --- | --- |
| `batch-qc` | Yes | Yes | Captured sample QC measurements; descriptive scores and flags |
| `motifs` | Yes | Yes | Captured query/background units; existing GLIPH2-style ClonoLoom adaptation |
| `cohort-features` | Unavailable | Yes | Captured diversity measurements |
| `cohort-compare` | Unavailable | Yes | Captured diversity measurements and embedded feature parent |
| `cohort-counts` | Unavailable | Yes | Captured count matrix and embedded feature parent |

All other registered workers are unavailable for both headless computation and certificate
export/replay: `diversity`, `rarefaction`, `usage`, `overlap`, `tracking`, `expansion`, `classes`,
`attribution`, `tcrdist`, `pgen`, `neighborhoods`, `networks`, `specificity`, `families`, `germlines`,
`shm`, `lineages`, `isotypes`, `expression-map`, `phenotypes`, `modularity`, `definition-robustness`,
`convergence`, `pseudobulk`, `pseudobulk-de`, `contamination`, `conga`, `anndata-export`, `example`
and `reference`. `GET /api/certificates/support` lists **every registered kind**, its three support
flags, replay scope and limits; tests check this matrix against the registry and browser choices.
Durable job resume alone does not establish a portable scientific replay contract.

## Try a complete artificial study

From the repository, with a development `clonoloom` binary on PATH:

```sh
clonoloom simulate --preset batch-qc-study --out /tmp/clonoloom-qc-study
node docs/examples/headless-recipe.mjs batch-qc /tmp/clonoloom-qc-study /tmp/clonoloom-qc-recipe.json /tmp/clonoloom-qc-run
```

The helper writes a new recipe and **prints** the complete command for review and execution.
It pins sorted top-level AIRR files and reads the simulator's declared sample assignments.
Use `--preset motif-study` and helper kind `motifs` for the 12-sample artificial motif study.
The helper's assignments, comparison arms and fixed clock are for these presets only; real
studies need their own declared subjects, batches, exclusions and query/background selection.
Truth files inform this example recipe, not the scientific worker or its results.

After running the printed command:

```sh
clonoloom verify --replay --report /tmp/clonoloom-qc-replay.json /tmp/clonoloom-qc-run/result.certificate.clz
```

The batch study includes uneven batches, a small batch, zero MAD, missing values and declared
exclusions. Its 26 samples yield 24 included samples from 24 subjects. The motif study includes
108 query and 1000 background sequence units with planted local/global signatures and null
controls. Neither simulation represents measured human data or validates a biological claim.
See the [batch QC](site/pages/docs/batch-qc.html) and [motif](site/pages/docs/motifs.html) method guides.

## Recipe v1

The top-level fields are `format: "clonoloom-run-recipe"`, `version: 1`, optional RFC3339 `time`,
and `tasks` (1–64). Each selected task has:

| Field | Contract |
| --- | --- |
| `id` | Unique lowercase logical name: letter then letters, digits, `_` or `-`, at most 64 characters |
| `operation` | `compute` or `replay` |
| `kind` | One of the supported kinds above |
| `inputs` | Ordered list of `{name, file, size, sha256}`; names use the same logical-name rule |
| `study` | Required for compute; study display name, at most 4096 bytes |
| `parameters` | Required compute request, with no dataset ID; normalized defaults are saved |
| `credits` | Optional compute source credits, using existing `Title`, `Source`, `License`, `LicenseURL`, `LicenseClass`, `Citation` fields |
| `fingerprint` | Optional replay-only separately retained certificate fingerprint (64 lowercase hex characters) |

Compute parameters use the existing [batch QC](REMOTE-CONTROL.md) or motif request
fields: batch QC takes `samples` and `options`; motifs takes `definition`, `filter`, `query`,
`background`, `subjects` and `options`. The dataset is bound from the declared snapshots.
Unknown fields, duplicate JSON keys and invalid science declarations are refused. Effective
recipes save typed, normalized options and one selected task. They do not record physical
input paths; the CLI supplies those separately.

```sh
clonoloom run --recipe study.json --task study --input source0=/data/S01.tsv --input source1=/data/study.airr.json --out /results/new-run
```

This illustrative command needs a recipe whose two named inputs match those files exactly.
Every input declares its byte size, lowercase SHA-256 and portable filename. Filenames begin
with a letter/digit, contain only letters/digits/dot/underscore/hyphen, are at most 128 bytes,
and cannot end in a dot; case-insensitive duplicates are refused. The filename is retained
when snapshotting, so moving physical paths does not silently change sample identity.
Input order is significant and preserved. A binding may contain `=` in its path.

Compute accepts only explicitly listed AIRR `.tsv`, `.tsv.gz` and `.airr.json` files. No directory
traversal, automatic file search, undeclared sidecars or network retrieval occurs. Inputs and
recipe must be regular files, not links or special files. At most 256 inputs, 256 MiB each and
512 MiB total are accepted; recipes are bounded to 8 MiB and 64 JSON nesting levels. Replay
recipes take exactly one certificate input and no replacement study, parameters or credits.
Their 256 MiB input limit is narrower than standalone `verify`'s 520 MiB compressed-archive limit.

Multiple tasks require explicit `--task ID`. A cluster array can map its index to a task ID
and pass only that task's bindings and a unique output directory. No scheduler variable is
interpreted and no tasks or studies are implicitly pooled. Unselected task semantics are not
executed or scientifically validated; the overall JSON schema and unique task IDs are checked.

## Outputs, reproducibility and failure

The parent output directory must exist; `--out` itself **must not exist**, even if empty.
Files are created without overwrite. Successful computation writes:

- `validation.json`: source-reader findings, including warnings and missing data.
- `<kind>.json`: complete scientific result and captured evidence.
- `manifest.json` and `NOTICE.txt`: methods, parameters, provenance, citations and provider terms.
- `result.certificate.clz`: portable evidence at the documented captured-input boundary.
- `effective-recipe.json`: the normalized selected task and resolved clock.
- `run.json`: status, requested recipe SHA-256, software, ordered input pins, artifact ID,
  findings, limits and output path/size/SHA-256 inventory (excluding itself).

Replay tasks write `replay.json`, `effective-recipe.json` and `run.json`. A mismatch in the
requested versus archived kind fails. `run.json` is a run record, not a portable project or
certificate; compute exports do not populate the application's library.

A fixed recipe `time` pins scientific notice/job/artifact provenance. If absent, the command
resolves it once and records it in the effective recipe. Reusing that effective recipe, exact
inputs and compatible software gives exact scientific output bytes in the tested contracts.
The time is recipe provenance, **not proof of wall-clock execution time**. The requested-recipe
hash in `run.json` changes if the recipe bytes change; software/build provenance can also differ.
Replay reports separately record the verifier platform. There is no tolerance fallback or claim
that whole run directories are identical across arbitrary software versions or platforms.

Default timeout is 30 minutes; `--timeout 5m` changes it. Ctrl+C/SIGTERM cancel. Snapshot reading
and worker execution honor cancellation; bounded existing capture/validation calls finish their
current operation before cancellation is observed. Temporary source snapshots and worker results
are removed when the process exits normally or handles cancellation. A forced kill/power failure
can leave temporary files and an incomplete output directory; no final `run.json` means no
completed run. Failed runs can retain diagnostics or partial exports: read status and exit code.

| Exit | Meaning |
| --- | --- |
| 0 | `complete` computation or `replay-match` |
| 1 | Scientific capture/worker failure, invalid/differing certificate or requested-kind mismatch |
| 2 | Usage/input/output failure, unavailable replay, provider acknowledgment needed or cancellation |

When output reservation succeeds, the command attempts to write a final `run.json` even on
failure. An I/O error can prevent that record too. Unsupported recipes fail before creating an
output directory. Other failures retain `failed`, `differs`, `unavailable` or `cancelled` status.

## Scientific and source boundaries

Source runs verify declared file bytes, then perform ordinary source parsing/capture and the
selected existing worker. Certificate replay begins **after capture**: it does not redo sample
QC extraction or upstream clonotyping from original files. Batch flags are descriptive, not
contamination diagnoses or an instruction to correct data. Motifs rerun ClonoLoom's documented
GLIPH2-style adaptation, not upstream GLIPH2; no population-calibrated inference or antigen
specificity is established. Methods and limits remain in the result and NOTICE; see
[certificate boundaries](CERTIFICATES.md#exact-replay-boundary) and research §§5.35, 5.38, 5.44.

Known catalog credits are retained and supplied credits are appended, never used to erase known
terms. Supply the actual source attribution for external files; arbitrary source licensing cannot
be inferred from bytes. Non-open/unknown recorded credits require `--accept-terms` before source
diagnostics or scientific exports; this acknowledges private-study restrictions, not redistribution
permission. Existing supplied certificates retain the verifier's contract. Evidence may contain
identifiers and sequences and is not de-identified. Matching bytes do not prove authorship or
scientific validity.

## Container and cluster packaging

The [container and cluster guide](CONTAINERS.md) packages these same commands in a small
non-root OCI image, with explicit read-only inputs, disjoint writable outputs, bounded scratch
and no network. Command-printing and replay-bundle helpers preserve original recipe/certificate
bytes. An explicit native Slurm array example selects one task and a new output per index.
Packaging adds no source kind, numerical tolerance or replay scope; runtime permissions and
cluster allocation remain operator choices.
