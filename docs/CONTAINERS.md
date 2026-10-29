# Containers and cluster execution

ClonoLoom 0.8.0 runs the **same native Go workers** in the container and application.
Packaging adds explicit job boundaries, not a second scientific implementation or a larger
replay support matrix. See the [headless guide](HEADLESS.md) for recipes, limits and statuses.

## Build a local image

The root `Dockerfile` uses a digest-pinned official Go 1.26.6 builder, disabled module/toolchain
network retrieval, a CGO-free binary and a `scratch` runtime. `.dockerignore` allowlists compilation
inputs and embedded assets. The runtime contains the program, mount-point directories and
ClonoLoom/Go license notices. The program retains its usual embedded assets and small fixtures;
no separate source tree, browser, shell or package manager is included.

From the repository root, with Docker available:

```sh
revision="$(git rev-parse HEAD)"
if test -n "$(git status --porcelain)"; then revision="${revision}-dirty"; fi
docker build --network=none --build-arg "REVISION=$revision" -t clonoloom:local .
image_id="$(docker image inspect clonoloom:local --format '{{.Id}}')"
docker image inspect "$image_id" > /tmp/clonoloom-image-inspect.json
```

Podman accepts the same build arguments. The builder image may need a one-time download;
`--network=none` disables build-step networking, not the registry's initial image retrieval.
The multi-platform builder digest is fixed in `Dockerfile`; update it deliberately and rerun
validation. `--platform linux/amd64` or `--platform linux/arm64` selects the target architecture,
with native compilation on the build platform. No registry image is published by this repository's
CI. Save the inspect record and source revision with your run; tags can move. A local image ID
pins content in that engine, while a registry digest is used for distributed image retrieval.

The `REVISION` argument is operator-supplied provenance, not a signature or proof of a clean
checkout. Its default is `unrecorded`; never silently label a modified source tree as a release.
An identical source revision alone does not guarantee identical OCI archives or every run record.

The build context uses explicit file exceptions, not whole-directory exceptions. When adding
compilation inputs or embedded assets, run `node validation/container-context-files.mjs --write`
and review the `.dockerignore` diff. The default invocation checks for drift; CI also inspects
the actual builder filesystem with injected exclusion controls.

## Permissions belong to the run command

The image defaults to UID/GID **65532:65532**, `/work`, `TMPDIR=/scratch`, and `run --help`.
No command starts a server or browser by default. The numeric user may be overridden to match
the owner of an explicitly writable results directory. Do not run as root to bypass ownership
problems or change ownership of original inputs.

| Resource | Explicit policy |
| --- | --- |
| Image root filesystem | `--read-only` |
| Input directory | Bind only the chosen directory to `/inputs`, read-only |
| Output parent | Bind a separate chosen directory to `/output`, writable |
| Scratch | Separate bounded tmpfs at `/scratch`, writable, noexec/nosuid/nodev |
| Network | `--network=none`; `--pull=never` uses the already selected local image |
| Privileges | Non-root numeric UID/GID, all capabilities dropped, no-new-privileges |
| Resources | Example uses 2 CPUs, 1 GiB memory, 128 processes and 1 GiB scratch; size for the task |
| Provider terms | No implicit `--accept-terms`; read the recorded terms and explicitly acknowledge only when appropriate |

Input and output mounts must be disjoint. Mounting a writable ancestor of an input directory
would provide a second path around the read-only bind. `docs/examples/container-run.mjs` rejects
that layout, mutable image tags, root execution, linked inputs, unsafe filenames and existing
outputs. It prints a complete quoted command for review; it never executes it. The native command
still validates the original recipe, named bindings, checksums, selected kind and permissions.
The helper reads binding metadata only; it does not reserialize scientific recipe parameters.

For rootless Podman, explicitly add `--userns=keep-id` where supported by your site so the numeric
user owns the writable mount. The validated local Podman VM uses a rootful engine with a non-root
container process. Podman also needs `--read-only-tmpfs=false` to avoid its additional automatic
writable tmpfs mounts; the helper includes that flag. Docker does not accept that Podman-only flag.
On SELinux systems, arrange site-approved volume labels on dedicated study directories; do not
relabel broad home/shared trees. Remote engines and desktop VMs need access to the specified
host paths. Runtime configuration and filesystem policy remain administrator/operator choices.

## Reproduce an artificial source computation

With a development `clonoloom` on PATH and a new working directory:

```sh
mkdir -m 700 /tmp/clonoloom-container-study
clonoloom simulate --preset batch-qc-study --out /tmp/clonoloom-container-study/inputs
mkdir -m 700 /tmp/clonoloom-container-study/results
node docs/examples/headless-recipe.mjs batch-qc \
  /tmp/clonoloom-container-study/inputs \
  /tmp/clonoloom-container-study/inputs/recipe.json \
  /tmp/clonoloom-container-study/results/native-comparison
node docs/examples/container-run.mjs docker "$image_id" \
  /tmp/clonoloom-container-study/inputs recipe.json study \
  /tmp/clonoloom-container-study/results container-run
```

The first helper prints an optional native comparison command; the second prints the explicit
container command. Review and execute the command you intend. For Podman substitute `podman`
and obtain its local image ID with `podman image inspect`. For the second supported source kind,
use simulator preset `motif-study`, helper kind `motifs`, and new directories. These examples
preserve their existing declared populations, counts, missing values, parameters and fixed recipe
clock. They do not represent measured human data or establish biological findings.

After computation, the new output directory contains scientific JSON, manifest, NOTICE,
certificate, effective recipe and `run.json`. Check exit status **and** the record. Never infer
success from a directory's existence. A read-only mount protects source files; the worker also
pins and snapshots their exact bytes before computation. Provider restrictions, identifiers and
sequences remain in the evidence. Containerization does not de-identify or authorize sharing.

## Explicit replay tasks and cluster arrays

Prepare a new private bundle from already exported certificates. Supply each declared result
kind separately; the native verifier checks that it matches the certificate:

```sh
node docs/examples/cluster-replay.mjs /absolute/new-replay-bundle \
  batch-qc /absolute/qc/result.certificate.clz \
  motifs /absolute/motifs/result.certificate.clz
```

The helper copies exact certificate bytes into `task-000.certificate.clz`, `task-001.certificate.clz`,
and so on, and writes `replay.json` plus an explicit `tasks.txt` index. It accepts 1–64 regular
files, at most 256 MiB each and 512 MiB in total. Symlinks and existing output directories are
refused. Copies use private permissions, and a failed preparation removes only its newly created
bundle. It does not claim certificate integrity, inspect/rewrite scientific values, erase provider
terms or submit a job. Recipe input hashes pin each copied certificate's bytes; they are not
independent authenticity evidence. Retain separately obtained certificate fingerprints when
needed with the existing verifier's `--expect` contract.

Run one task in the container by printing its command:

```sh
node docs/examples/container-run.mjs docker "$image_id" \
  /absolute/new-replay-bundle replay.json task-000 \
  /absolute/existing-results replay-000
```

For a Slurm site with the native Linux binary installed, review the included batch script and
choose account, partition, resource allocation and array range according to site policy:

```sh
sbatch --array=0-1 cluster/replay-array.sh \
  /absolute/clonoloom-linux-amd64 /absolute/new-replay-bundle /absolute/existing-results
```

This is a command for the operator to execute, not something the helpers submit. The script
requires explicit numeric `SLURM_ARRAY_JOB_ID` and index 0–63. It selects exactly one listed task,
uses only that certificate binding, and writes a new `<array-job-id>-<task-id>` directory. It uses
argument quoting and `exec`, with no `eval`, dynamic shell instructions, implicit task pooling or
automatic provider acknowledgment. A rescheduled task with an existing destination fails instead
of overwriting or silently resuming it; choose a new destination deliberately after inspection.
Keep logs private: the script uses `umask 077`, and the scheduler's creation of the batch log also
needs an appropriate submission umask/site policy.

The native script opens inputs read-only but is not a filesystem sandbox. Use site ACLs or a
site-approved container launcher to enforce mount boundaries on cluster nodes. Do not grant a
Docker socket or privileged container solely to run these computations. The same explicit recipe
and bindings can be used with your site's launcher; validate signal and mount behavior there.

Apptainer is **not validated by this slice**. A shell-free `scratch` image is not a promise of
compatibility with every OCI-to-SIF conversion or site wrapper. Apptainer's defaults can bind home,
working and temporary directories, so a site integration must explicitly review those mounts,
environment inheritance and network policy. Use the validated native array path until that
integration is tested. No Apptainer or Slurm installation is required for ordinary desktop use.

## Cancellation, exactness and limits

Exec-form image startup places the Go command directly at PID 1; `STOPSIGNAL` is SIGTERM.
The existing `run` handler reports exit 2 and `cancelled` when it handles TERM/INT, and removes
its temporary snapshots. The array example requests a TERM warning before its allocation ends.
Give bounded capture/validation operations sufficient grace to finish before observing cancellation.
SIGKILL, OOM, power failure or a scheduler's hard timeout may leave partial outputs and scratch;
no final `run.json` is not success. Runtime startup failures (often 125–127) are distinct from
ClonoLoom's existing 0/1/2 result codes. Do not automatically restart a failed scientific run.

Source computation remains limited to **batch QC and motifs**. Exact certificate replay remains
limited to **batch QC, motifs, cohort features, cohort comparisons and cohort count models**.
The existing support matrix refuses other kinds. Container or scheduler execution does not
turn an unavailable result into a match or introduce a numerical tolerance. Fixed effective
recipes and exact named inputs preserve scientific files under the tested software contract;
verifier platform metadata and other execution provenance can differ. Replay uses the captured
evidence, not upstream source parsing, alignment or clonotyping. Biological interpretation,
independent replication, antigen specificity and provider permissions remain separate questions.

## Validation and sources

The validation scripts use generated studies and existing independent replay controls:

```sh
node --test cluster/*.test.mjs
node validation/container-context-files.mjs
node validation/container-context.mjs docker clonoloom:local /absolute/new-context-validation
node validation/containers.mjs docker clonoloom:local /absolute/new-runtime-validation
```

They check the actual build context/image inventory, non-root defaults, exact native/container
scientific exports, independent ZIP/inventory checks, five replay kinds, one-bit differences,
unsupported methods, explicit terms, denied mounts, existing outputs, array selection and SIGTERM.
The host user must be non-root so tests exercise meaningful mount ownership. Local validation
used Linux arm64 and emulated Linux amd64 through Podman; CI uses native Linux Docker. Array
selection was tested locally without a Slurm scheduler. No cluster job, image push or release
is performed by validation.

Packaging follows the official [Dockerfile reference](https://docs.docker.com/reference/dockerfile/),
[Docker runtime controls](https://docs.docker.com/engine/containers/run/),
[Podman runtime controls](https://docs.podman.io/en/latest/markdown/podman-run.1.html),
[Slurm job arrays](https://slurm.schedmd.com/job_array.html) and
[Slurm signal contract](https://slurm.schedmd.com/sbatch.html).
[Apptainer bind defaults](https://apptainer.org/docs/user/latest/bind_paths_and_mounts.html)
explain the unvalidated integration boundary. Scientific citations remain in each native result,
NOTICE and the [headless method boundaries](HEADLESS.md#scientific-and-source-boundaries).
