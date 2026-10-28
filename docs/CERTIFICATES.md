# Saved-result evidence certificates

Wave 8 adds **evidence certificates**, starting with saved cohort feature tables,
group comparisons and replicated count models. They keep a result's exact original manifest,
recorded recipe, output files and provider notice. Choose an integrity check or exact replay
from captured evidence. Neither mode verifies original source files, project history, authorship
or scientific assumptions.

In **Reports**, choose a supported result and **Inspect certificate**. Keep its SHA-256
fingerprint separately if you need to recognize that exact record later. **Export integrity
certificate** downloads `<artifact>.certificate.clz`. This is a single-result evidence archive,
not a portable project: do not open it as a project. Unsupported result kinds are not silently
omitted or described as certified. Broader result and source replay coverage follows in later slices.

Original source files are not bundled, but captured recipes and result evidence may include raw
counts, receptor sequences, subject identifiers, metadata and paths. This is not de-identified.
Provider restrictions and citations are preserved in `NOTICE.txt`; restricted-data exports need
explicit acknowledgment and remain for private study under those terms.

## Check without a running application

```sh
clonoloom verify --integrity result.certificate.clz
clonoloom verify --integrity --expect SHA256 --report verification.json result.certificate.clz
clonoloom verify --replay --expect SHA256 --report replay.json result.certificate.clz
```

Replace `SHA256` with the 64 lowercase hexadecimal characters recorded separately. `--report`
writes a **new** JSON file and refuses to overwrite an existing file. `--timeout` defaults to
five minutes. Ctrl+C cancels. No browser, server, project library or network is needed.

Exit codes:

- **0:** the requested check succeeded: `integrity-match` or `replay-match`.
  Integrity reports always have `recomputed: false`.
- **1:** corrupt/unsupported archive, changed or missing bytes, inventory inconsistency, or
  mismatch against the separately supplied fingerprint.
- **2:** `unavailable` replay, or usage, input/output or cancellation error. No success is implied.

Choose exactly one of `--integrity` and `--replay`. Bare `verify` is refused. The same Go
implementations are available under **Reports → Check or replay a certificate**. Select the
check, optionally enter a separately retained fingerprint, then choose the archive. The report
can be downloaded as the exact host JSON, preserving large integers and original numeric tokens. **Cancel verification** stops the request and running worker.
The file is uploaded only to the application's loopback host. No recorded source paths are
followed; only the three supported cohort workers may execute during replay.

## Exact replay boundary

| Saved kind | Recomputed | Retained input, not independently recomputed |
| --- | --- | --- |
| `cohort-features` | Design checks and subject feature table | Captured diversity measurements and source provenance |
| `cohort-compare` | Feature parent, comparison tests, effects, intervals and correction family | Captured diversity measurements |
| `cohort-counts` | Feature parent, sample-total checks and the negative-binomial model | Captured repertoire count matrix; no recount of source sequences |

Replay first verifies archive integrity, then the embedded parent bytes against their manifest
and the recipe's pinned hash. A disposable artifact store runs the existing Go job handlers.
The expected target result is never installed before computing it. Every recomputed output byte
must match, including the serialized numbers, missing values, descriptions and embedded evidence.
Scientific manifest metadata is compared too. No tolerance or rounding fallback is used.

Original parent provenance is preserved when supplying a successfully replayed feature parent
to the next worker. New job IDs, execution timestamps and software do not replace the original
record or masquerade as a scientific difference. Temporary evidence and outputs are removed
on completion or cancellation, without changing the open library or project.

The separate `clonoloom-replay` version-1 report has scope `captured-cohort-evidence`. It includes
the integrity report, original manifest, current verifier software/platform, findings and
ordered steps with artifact/kind, expected and actual SHA-256, verdict and `metadataMatched`.
`recomputed` means the requested target worker produced output, even if that output differs;
completed parent steps alone do not set it. Verdicts are:

- **`replay-match`:** every required step and scientific metadata matched exactly.
- **`differs`:** integrity, bindings, output bytes or scientific metadata differ. Review findings
  and completed steps; a partial match is not success.
- **`unavailable`:** missing captured evidence, unsupported kind/method/recipe, failed worker
  checks or an operational failure prevented completion. Never treat this as a match.

An integrity match can therefore coexist with a replay difference. A one-bit output alteration
with newly calculated archive checksums passes integrity but fails scientific replay. Conversely,
a coordinated change to inputs, recipes and results can reproduce: neither mode proves authorship.
Original receptor files, external source fingerprints, separate parent files and upstream
diversity computations are outside this boundary. A replay match does not establish model
calibration, sound study design, replication with new data, causality or biological truth.
See [NASEM (2019)](https://doi.org/10.17226/25303) and the project's
[scientific rationale](../clonoloom-spec/research.md#543-exact-replay-from-captured-cohort-evidence-wave-8-slice-2).

## Record and limits

The ZIP contains `certificate.json`, `manifest.json`, `NOTICE.txt`, `README.txt` and every saved
artifact file under `artifact/`. No directory entries, links or extra files are accepted.
The record's format is `clonoloom-certificate`, version 1, scope `recorded-artifact-bytes`.
Its ordered fields are `format`, `version`, `scope`, `artifact`, `kind`, `software`, `files`,
`fingerprint`; the software fields follow `artifacts.Software` (`name`, `version`, optional
`commit`), and each path-sorted file has `path`, `size`, `sha256` in that order.

The fingerprint is SHA-256 of Go `encoding/json`'s compact UTF-8 record encoding **without**
`fingerprint`. Strings use that encoder's escaping (including HTML-sensitive characters);
there are no floating-point values in this record. This is ClonoLoom's versioned encoding,
not a claim of RFC 8785/JCS compatibility. The final `certificate.json` uses the same compact
encoding, with no newline. Noncanonical bytes, duplicate keys, unknown fields and unsupported
versions are refused. Saved manifests/results are hashed as original bytes without reformatting
or numeric conversion. SHA-256 uses Go's standard library; see
[NIST FIPS 180-4](https://doi.org/10.6028/NIST.FIPS.180-4).

Version 1 bounds are 256 inventoried files plus `certificate.json`, 256 MiB per member,
64 MiB for the saved manifest, 1 MiB for certificate metadata and provider notice,
512 MiB of inventoried bytes and 520 MiB of compressed input. Paths are relative UTF-8 names
of at most 1,024 bytes, with no traversal, backslashes, colons or control characters. Verification
checks actual decompressed bytes, CRCs, sizes, SHA-256, the exact member set and correspondence
to the original artifact manifest. It stops at cancellation or declared bounds.

A digest is **not a signature**. Someone who can replace both contents and checksums can make
a new internally consistent archive. An independently retained fingerprint identifies a record;
it does not establish authorship, correct methods, sufficient replication or biological truth.

## Automation

The existing action catalog now exposes `inspect_certificate` and `export_certificate`, each
with an `artifact` ID and optional explicit provider-term `accept`. The visible Reports view
performs both actions; they return the record and fingerprint. Export downloads through the
browser; server-side output paths belong to the later automation slice. File verification is
available through the browser upload or native CLI; arbitrary filesystem paths are not added to
remote control here.

HTTP clients on the existing permitted local origin can use:

- `GET /api/certificates/{id}` for an inventory (unsupported/changed results return 409).
- `GET /api/certificates/{id}/download` for the ZIP, with `X-ClonoLoom-Certificate` fingerprint.
- `POST /api/certificates/verify[?mode=integrity|replay&expect=SHA256]` with an archive body.
  Omitted mode retains the integrity-only contract; unknown modes return 400. Replay has a
  five-minute deadline and follows request cancellation. A completed verification request returns
  its report with HTTP 200; inspect `verdict`, not HTTP status, to decide whether it matched.

Restricted result inspection/download returns the usual 409 `restricted` response until
`?accept=1` explicitly acknowledges its terms. Inspection reads captured evidence and verifies
its output hashes; it creates no result and changes no saved project.
