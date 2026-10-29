# Reviewed Proteoscope sequence handoff

Wave 9 slice 12 adds **Data → Proteoscope handoff**. This is unreleased source functionality,
not part of the 0.8.0 binary. It transfers one supplied AIRR amino-acid observation for sequence
similarity search in **Proteoscope 0.10.0**. It does not reconstruct a TCR/antibody, infer a paired
chain, predict a structure or establish binding specificity.

## Review and send

1. Start the separate Proteoscope application with remote control explicitly enabled, and keep
   its browser page open. For its default port:

   ```sh
   proteoscope --remote-control --port 8765
   ```

2. Open an AIRR Rearrangement TSV in ClonoLoom. In Data, choose **Proteoscope handoff**, select
   one source observation and enter Proteoscope's port on `127.0.0.1`. Selection starts empty.
   The observations are source rows, without the project's QC filter or clonotype grouping.
3. Choose **Review sequence and destination**. Check the exact sequence, identifier, sample,
   locus, completeness annotation, source row/hash and provider terms. Only the named local
   port's home/startup endpoints are probed; no sequence is sent by review.
4. Separately approve the transfer to local Proteoscope and disclosure of the query to **public
   RCSB sequence search**. Restricted provider evidence also requires acknowledgment of its
   terms. Acknowledgment does not grant disclosure or redistribution rights.
5. Choose **Send sequence and save receipt**. The exact reviewed evidence is downloaded before
   dispatch. A successful command acknowledgment produces a separate receipt. Inspect Proteoscope's
   **Find structures** results and errors. Its most recently connected browser page receives the
   command; this interface cannot target a particular window.

Proteoscope's supported interface is `POST /api/remote/command` with `{"command":"search SEQUENCE"}`.
It has no sequence-prefill URL. The command can trigger RCSB's sequence-search API and Proteoscope's
query cache; it is not a local-only visualization action. No accession, participant/sample label,
source filename, receptor ID or annotation is put into the query. Only the supplied amino acids
cross that boundary. This feature does not enable remote control, read its token files, choose
hits, open fetched structures or transmit arbitrary commands.

**Save reviewed evidence** works without sending a sequence, including when the receiver is
unavailable. ClonoLoom's `--offline` disables destination probing and sending while retaining
local evidence review/export. Proteoscope's own offline mode disables sending. No background
port scan or startup request occurs merely because the dialog opens.

## Exact sequence and scientific scope

The initial contract accepts native AIRR TSV (plain or gzip), with one exact `sequence_id`,
a declared receptor `locus`, and an unchanged `sequence_aa` of **25–5000 uppercase canonical
amino acids**. This conservative alphabet/length guarantees that the tested receiver classifies
the command as a protein sequence and preserves its bytes. Proteoscope accepts some broader
inputs interactively; ClonoLoom does not silently normalize those into this contract.

Missing amino acids, lowercase, ambiguity codes, gaps, stops and sequences outside the bounds
remain unavailable for sending. Nucleotides are not translated; junctions are not substituted;
residues are not uppercased, stripped, trimmed, padded or reconstructed. Other source formats,
mapped tables, paired complexes and synthesized candidate constructs are outside this slice.
Re-export supported evidence from the upstream annotation pipeline as AIRR when needed.

`sequence_aa` is supplied sequence evidence, not proof of a complete protein. AIRR's
`complete_vdj` concerns coverage of the annotated V(D)J region; it does not establish constant-region
coverage, an intact expressed product or biological function. A supplied `F`, empty field or
unknown annotation remains as supplied. Eligible fragments can be searched after review, but
partial similarity must not be presented as full-receptor identity. Productive/other flags and
gene calls are carried as annotations, not used to invent sequence or pairing.

Sequence similarity does not establish antigen specificity, affinity, causality or function.
Proteoscope controls the search method, parameters, cache and returned hits. In the tested
0.10.0 interface, its sequence endpoint uses default identity 0.3 and E-value 0.1, grouped by
entry. ClonoLoom does not translate these into a new analysis or declare that a hit establishes
the selected receptor's structure. A successful command acknowledgment can coexist with a
search error shown in Proteoscope; inspect that page. No live-service completeness or biological
performance was evaluated here.

## Evidence and permissions

`clonoloom-proteoscope-handoff.json` is a version 1 `clonoloom-proteoscope-handoff` record with:

- Dataset/source identity, complete recorded source metadata and source SHA-256.
- Original header/selected row text without line endings, and every decoded field as a string.
  Leading zeroes, empty fields and precision-sensitive numeric annotations remain exact text.
- Source observation index/line, sequence identifier, sample, locus, exact amino acids and SHA-256.
- Exact command, reviewed local endpoint/version/capabilities, public recipient, source notices,
  exporter identity and scientific limits.

The receipt identifies the review by SHA-256, the sequence hash, destination, explicit permission
flags and the receiver's JSON acknowledgment, with its original response text and SHA-256.
Numeric tokens in that acknowledgment are not converted through browser numbers; original
whitespace remains in `responseText`. The review fingerprint hashes the native compact JSON record
before its final newline; the sequence fingerprint hashes its UTF-8 bytes. Keep both downloaded
files. They retain identifiers and are **not de-identified**. This is an interchange record, not a
scientific certificate, archived remote database or replay recipe.

Reviews last five minutes and sending consumes the review once, including failed or uncertain
dispatch. The source bytes and receiver contract are checked again before sending. Changed
evidence, changed destination/version, missing permissions and concurrent/repeated submission
are refused. Port edits, selection changes and project changes invalidate the browser review.
The receiver probe identifies a local endpoint/contract, not cryptographic application identity
or a particular process/window.

Cancel or close the dialog to stop waiting. **Once dispatch starts, cancellation or a timeout
cannot retract a query that Proteoscope may already have received.** Check Proteoscope before
creating another review; no automatic retry occurs. An unavailable receiver never falls back to
a direct public query, file upload or arbitrary destination.

Bounds: one observation; 64 MiB original and decompressed source; 1 MiB per scanned line;
25–5000 amino acids; loopback ports 1–65535; 16 KiB requests; 1 MiB receiver responses;
two-second destination probe, 45-second HTTP client and 60-second request deadline; at most 128
pending five-minute reviews per process. Sources must be regular files; changed hashes, symlinks,
duplicate columns, mismatched row/header widths and invalid UTF-8 are refused. The selected
source must still be available; this path deliberately verifies it rather than trusting a cache.

Browser endpoints are `POST /api/proteoscope/page`, `/review`, `/export` and `/send`, accepting
an explicit dataset, row and port, then the returned review ID/fingerprint and permissions.
They require a local peer/Host and the existing same-origin protection. No new MCP, Python/R
client action, automation permission or scientific replay kind is added.

## Validation and sources

Native tests exercise exact text, identifiers, gzip, changed sources, unavailable sequences,
offline behavior, provider terms, redirects, incompatible receivers, cancellation and one-use
concurrent submission. An independent Python CSV/JSON/SHA-256 checker compares the source row,
every annotation, query bytes, permissions and receipt hash. CI runs those checks without a
public service. Both-theme browser tests use declared artificial controls and an isolated receiver.

`validation/proteoscope-handoff.mjs` also exercises the **actual Proteoscope 0.10.0** command
parser, remote receiver and browser classifier for two supplied sequences. Browser interception
captures the sequence-search boundary before any request reaches Proteoscope's server, preventing
public sequence disclosure. Both queries retain their exact residues, with no identifier leakage
or text-search fallback. This validates interchange, not live RCSB results or remote availability.
See [validation instructions](../validation/README.md) for reproduction.

- [AIRR Rearrangement schema 2.0](https://docs.airr-community.org/en/v2.0.0/datarep/rearrangements.html): supplied sequence, locus and V(D)J completeness definitions.
- [RCSB sequence similarity search](https://www.rcsb.org/docs/search-and-browse/advanced-search/sequence-similarity-search) and [Search API](https://search.rcsb.org/): destination/search interpretation.
- [Proteoscope](https://github.com/robert-mcdermott/proteoscope) 0.10.0, Apache-2.0: documented search/remote-control interface and tested receiver.
- [MaveScape](https://github.com/robert-mcdermott/mavescape), Apache-2.0: sibling loopback startup-probe pattern, adapted to explicit port review and disclosure permissions.
