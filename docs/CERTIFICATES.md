# Saved-result evidence certificates

Wave 8 slice 1 adds **integrity certificates**, starting with saved cohort feature tables,
group comparisons and replicated count models. They keep a result's exact original manifest,
recorded recipe, output files and provider notice. They do not recompute a statistic or verify
original source files, separate parent artifacts, project history or scientific assumptions.

In **Reports**, choose a supported result and **Inspect certificate**. Keep its SHA-256
fingerprint separately if you need to recognize that exact record later. **Export integrity
certificate** downloads `<artifact>.certificate.clz`. This is a single-result evidence archive,
not a portable project: do not open it as a project. Unsupported result kinds are not silently
omitted or described as certified. Scientific replay and broader coverage follow in later slices.

Original source files are not bundled, but captured recipes and result evidence may include raw
counts, receptor sequences, subject identifiers, metadata and paths. This is not de-identified.
Provider restrictions and citations are preserved in `NOTICE.txt`; restricted-data exports need
explicit acknowledgment and remain for private study under those terms.

## Check without a running application

```sh
clonoloom verify --integrity result.certificate.clz
clonoloom verify --integrity --expect SHA256 --report verification.json result.certificate.clz
```

Replace `SHA256` with the 64 lowercase hexadecimal characters recorded separately. `--report`
writes a **new** JSON file and refuses to overwrite an existing file. `--timeout` defaults to
five minutes. Ctrl+C cancels. No browser, server, project library or network is needed.

Exit codes:

- **0:** `integrity-match`; the listed bytes agree. `recomputed` is always `false` in this slice.
- **1:** corrupt/unsupported archive, changed or missing bytes, inventory inconsistency, or
  mismatch against the separately supplied fingerprint.
- **2:** usage, input/output or cancellation error; verification could not finish.

`--integrity` is required. A bare `verify` never silently substitutes checksum checking for
scientific recomputation. The same Go verifier is available under **Reports → Check a
certificate**, with optional external fingerprint and a downloadable verification JSON report.
The file is uploaded only to the application's loopback host. No archive entries are extracted,
recorded source paths followed, or recipes executed.

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
- `POST /api/certificates/verify[?expect=SHA256]` with an archive body for the integrity report.

Restricted result inspection/download returns the usual 409 `restricted` response until
`?accept=1` explicitly acknowledges its terms. Inspection reads captured evidence and verifies
its output hashes; it creates no result and changes no saved project.
