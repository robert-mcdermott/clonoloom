# A complete record-and-reach study

ClonoLoom 0.8.0 connects one simulated study across the normal window, HTTP, MCP, Python,
R, checkpoints, reports, portable archives and native replay. Each boundary preserves the
original evidence while stating what was checked. The worked fixture is in
`validation/projects/0.8.0`; older release fixtures are unchanged.

## Study and scientific question

Use the **Batch quality-control study** example, or generate a fresh copy:

```sh
clonoloom simulate --preset batch-qc-study --out /absolute/new-study
```

These are artificial AIRR records: 26 observations and 2,427 generating rows. The declared
included batches have 8, 8, 3 and 5 subjects; two observations have explicit exclusions.
Read `truth/samples.json` and the complete generating truth before running QC. The example
contains varied sample sizes, a shifted batch, low productivity, missing counts, a small batch
and a constant-count group. It is a controlled demonstration, not measured research data.

The question is descriptive: which sample measurements merit review within their declared
batch, and which cannot be scored? It is not a treatment comparison, correction model or
proof that an observation is erroneous. A shifted batch can have no within-batch flags.

In **QC → Batch QC**, load all assignments and exclusions, use threshold 3.5 and minimum five
evaluable subjects, and select **Screen batch QC**. Inspect raw observations, units, valid/scored
counts and unavailability reasons. Fraction axes span 0–1; positive counts use labeled log10
scales without pseudocounts. Median lines are not confidence intervals. Small groups and zero
MAD remain unscored. No flag automatically removes a sample. The active clonotype definition
is a separate setting and is not used by this sample-QC calculation.

The modified score is `0.6745 × (value − median) / MAD`; the flag requires absolute score
strictly greater than 3.5. This labeling convention follows Iglewicz and Hoaglin (1993), as
summarized by the [NIST handbook](https://www.itl.nist.gov/div898/handbook/eda/section3/eda35h.htm).
ClonoLoom's minimum-size rule, metric denominators and unavailable-value policies are its
explicit choices; see [Batch QC](site/pages/docs/batch-qc.html). AIRR nulls remain missing,
not zero ([AIRR rearrangement schema](https://docs.airr-community.org/en/stable/datarep/rearrangements.html)).

## Connect without changing the science

Start an isolated host with only the grants needed for this workflow:

```sh
clonoloom mcp --offline --allow edits,compute,files --data-dir /absolute/new-library
```

An MCP client launches that command and owns stdin/stdout. The same host serves the normal
window and local HTTP API; Python/R can discover that library's `remote.json`. For an HTTP-only
session use `clonoloom --remote-control` with the same explicit grants. Keep the window open to
review proposals. No client can approve its own edit, and no network capability is needed.

Open the generated TSV and AIRR metadata files as one dataset. Call `query_clonotypes` with
`limit: 10` through HTTP, MCP or a generated client. The limit selects a page, not a new
frequency denominator. In Python, keep `result.data['resultJSON']`; in R, keep
`result$data$resultJSON`. The fixture generator compares these original native strings byte for
byte across the four transports. Parsed Python decimals and R number tokens preserve precision;
conversion to ordinary floating-point frames is a separate caller choice. Query parity checks
transport fidelity, not independent scientific correctness.

See the [Python](PYTHON.md), [R](R.md), [MCP](MCP.md) and [permission](AUTOMATION.md) guides for
installation, connection selection, typed failures and exact argument handling. The automated
fixture driver clicks only its own isolated study's declared proposals; production clients
still require the researcher's visible review.

## Keep, compare and recover

1. In **Reports**, create a checkpoint named **Original definition and complete QC evidence**.
2. Propose **Amino-acid junction only** in the clonotype definition. Review and accept it.
3. Compare the checkpoint with the current analysis. The definition change must be recorded;
   it is not biological expansion or a change in the saved batch-QC result.
4. Recover the checkpoint and query again. The original definition and exact query evidence
   return. Recovery restores recorded settings and completed files; it recomputes no statistic.
   Undo remains available. Original sources and running jobs are not checkpoint backups.

Export the selected batch-QC result as a certificate and as a self-checking HTML report.
Keep the certificate fingerprint separately. The HTML report retains complete tables, existing
figures, methods, provider notices and original evidence. Its browser check is integrity only.

```sh
clonoloom verify --integrity --report /absolute/new-integrity.json result.certificate.clz
clonoloom verify --replay --report /absolute/new-replay.json result.certificate.clz
```

The first checks the bounded inventory. The second reruns the captured sample-QC worker and
requires exact result bytes and scientific metadata. Neither verifies authorship or the original
experiment. Captured-evidence replay does not rerun upstream source parsing or clonotyping.
Use `--expect` with a separately retained fingerprint when that is part of your evidence policy.

## Transfer and run outside the window

Export a portable archive, explicitly choosing whether you have authority to include sources.
For this generated fixture the source files are artificial and may be included. Inspect the
archive and its fingerprint in a fresh library, then review import. Query evidence and saved
result files must remain identical when the original source directory is unavailable.
Checkpoint recovery also works from the imported evidence. Import can rebuild dataset tables;
it does not recompute saved analyses. Missing/restricted sources are never silently represented
as present, and conflicting existing evidence is refused.

For native replay prepare a private bundle and execute one explicit task:

```sh
node docs/examples/cluster-replay.mjs /absolute/new-bundle batch-qc result.certificate.clz
clonoloom run --recipe /absolute/new-bundle/replay.json --task task-000 \
  --input evidence=/absolute/new-bundle/task-000.certificate.clz \
  --out /absolute/new-replay-output
```

The [container guide](CONTAINERS.md) adds explicit read-only inputs, separate writable output
and scratch, a non-root identity and no network. Container packaging does not enlarge replay
scope. Slurm submission and site integration remain operator actions; Apptainer is unvalidated.

## What 0.8.0 promises

| Operation | Exact contract | Boundary |
| --- | --- | --- |
| Query through HTTP/MCP/Python/R | Original native scientific JSON | No extra analysis or renormalization |
| Certificate integrity | Saved inventory and bytes | No recomputation, authenticity or source verification |
| Certificate replay | Batch QC, motifs, cohort features/comparisons/counts | Captured inputs only; unsupported kinds unavailable |
| Headless source computation | Batch QC and motifs | Explicit recipe, pinned inputs, new output |
| Checkpoint recovery | Saved settings and attached completed files | No source backup or running-job resume |
| HTML report | Explicitly selected supported results and byte checks | Browser does not recompute science |
| Portable archive | Project/history/results/checkpoints and permitted optional sources | No implicit sharing permission or de-identification |
| Parquet / pseudonymized tables | Mapped native dataset fields, units and missing values | Not transformed certificates or proof of anonymity |
| Agent benchmark | Known-truth local contract tasks and explicit review | Script controls are not model or biological performance |

All exports can retain identifiers, sequences and paths unless the explicitly selected mapped
pseudonymization workflow removes them. Private-use acknowledgment is not redistribution
permission. A digest, successful command or accepted proposal does not establish biological validity.

The maintainer can regenerate the complete integration study into a **new** directory:

```sh
node validation/create-project-0.8.mjs /absolute/new-fixture
```

It uses the local Go toolchain, Node, uv, R and Chrome, generated inputs and isolated libraries.
It executes real workers and explicitly scripted fixture review, but never publishes, submits a
cluster job or invokes a model service. The committed fixture is immutable after release.
