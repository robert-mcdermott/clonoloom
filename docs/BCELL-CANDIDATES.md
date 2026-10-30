# Descriptive B-cell candidate review

Unreleased wave 9, slice 15. Open a saved thresholded result in **B cells → Families** and choose
**Review B-cell candidates**. Keep its dataset open to inspect current provider terms. Choose a
sample, any matching SHM/isotype evidence, and one to four ordered criteria with their directions.
The sample and criteria start empty. No composite score or biological optimum is inferred.

This review prioritizes observations for a researcher's inspection. It does not estimate binding,
affinity, efficacy, selection or synthesis readiness. The table shows original counts, denominators,
missingness and tied ranks; there is no calibrated response axis or inferred biological trajectory.

## Criteria and experimental units

The ranked universe is **every family in the selected sample's saved scope**, including families
with no assigned observations in that sample. The optional baseline must be a different evaluable
sample from the same explicit subject, under subject-scoped families. Cross-subject comparison and
pooled baselines are refused. Naming a baseline is a user decision; the software does not infer
time ordering from sample names or metadata.

Let `n_f` be current assigned observations in family `f`, and `N` all current assigned observations
in this scope. With a baseline, let `b_f` and `B` be the corresponding baseline counts.

| Criterion | Exact quantity | Missingness and interpretation |
| --- | --- | --- |
| Assigned observations (`size`) | `n_f / 1` | Sample-scoped cells in single-cell data, heavy-chain rows in bulk data. No read, molecule or abundance weighting. Zero means no assigned observation, not biological absence. |
| Heavy nucleotide divergence (`mutation`) | Sum of observed heavy-row differences / sum of evaluable V/J bases | Every current heavy source row must have positive evaluable bases and no row/region exclusion. Otherwise unavailable; partial numerator/denominator and unavailable-row counts remain visible. |
| Preferred isotype share (`isotype`) | Observations with the explicitly preferred class / `n_f` | Every current observation must have a resolved class. Unresolved, excluded or missing class evidence makes the metric unavailable. No implicit Ig class hierarchy. |
| Observed share change (`change`) | `n_f/N − b_f/B`, retained as `(n_f*B − b_f*N)/(N*B)` | Needs the explicit baseline above. This is a difference of observed shares, not a fold change, expansion test or p-value. |

No-current-observation families have size zero and, with a baseline, a defined share change;
their mutation and isotype metrics are unavailable. Without optional parents the corresponding
metrics are unavailable, and choosing them as ranking criteria is refused. Requiring all heavy
rows to contribute **does not require complete V/J coverage**: eligible rows can cover different
positions. Pooled divergence weights rows and their evaluable coverage, not independent cells or
biological replicates. Identical source rows can contribute more than once if retained by the
family parent; these are not additional independent observations. No uncertainty interval is added.

Criteria are compared in the selected priority order using exact integer fractions. Directions are
explicit. Missing metrics sort after available values in either direction; a subsequent criterion
can order equal values or jointly missing values. If all selected criteria tie, competition ranks
are shared (for example 1, 1, 3). Stable family IDs order display ties without breaking their ranks.
Rounded decimal labels are only presentation; they do not decide order.

The denominator excludes observations rejected by the saved family definition. Their counts are
reported separately. Rows removed by the parent QC filter or ignored as non-B-cell inputs are
described in the original family evidence, not added to these denominators. Read depth, sampling,
QC, family threshold, reference choice and annotation coverage can all affect the ranking.
This is a descriptive comparison within supplied data, not inference about population abundance.

## Exact evidence

**Save exact candidate review** downloads native JSON with the complete ranked universe, all
current-sample source observations, options, counts, exact integer ratios, native numeric values,
missing reasons, ranks, limits and current provider notice. Original family and selected
SHM/germline/isotype parent JSON text, SHA-256 and manifests retain their source annotations,
scientific methods and citations. SHM/germline/isotype links must identify the exact family parent.

Each operation recomputes the native review and compares its fingerprint; changed parents or
provider notice require a fresh review. Downloads use the original native bytes, not browser
JSON serialization. Original parent text preserves exact numeric tokens. This is **not
de-identification**: identifiers, metadata and source annotations remain in the evidence.
Restricted sources require explicit export acknowledgment; that does not grant redistribution rights.

Bounds are 16 MiB per parent, 32 MiB combined parent text, 64 MiB native review, 50,000 observations,
5,000 families and 250,000 source rows. At most two candidate operations run concurrently. Work can
be cancelled; a project change invalidates the browser review. Save before closing: these downloads
are separate from project artifacts, certificates, scientific replay and the automation catalog.

## Review one original source chain

Select a family, one **current-sample observation**, then one source chain. None is chosen as a
representative automatically. **Review supplied amino acids** reopens the original AIRR TSV or
gzip source, verifies its bytes and row identity, and shows its exact `sequence_aa`, hash, locus,
`complete_vdj` and source annotations. The existing source reader bounds compressed and decoded
files to 64 MiB each, with lines below 1 MiB. Other input formats can contribute family evidence but are not supported by
this source-sequence export path.

FASTA requires a declared IGH/IGK/IGL locus and 25–5,000 unchanged canonical uppercase amino acids.
Missing, ambiguous, stopped or gapped sequences are refused. **No product translation, repair,
consensus, chain assembly, codon optimization or inferred ancestor is offered.** AIRR
[`complete_vdj`](https://docs.airr-community.org/en/v2.0.0/datarep/rearrangements.html) concerns the
annotated V(D)J region; it does not establish full protein completeness. A saved cell-associated
heavy/light assignment is an operational grouping, not independent proof of biological pairing.

After the explicit sequence-limit acknowledgment and any provider acknowledgment, **Save FASTA
and exact evidence** downloads the full review, a native sequence companion and the one-chain
FASTA. Keep all three together. The companion preserves original header/row text and every source
field as strings, review fingerprint and sequence hash. FASTA has an opaque safe identifier;
original identities remain in the companion. These files do not constitute a validated paired
receptor or synthesis-ready construct.

**Open Proteoscope review (sends nothing)** preselects this source row in the existing handoff
dialog. It does not probe a destination, approve terms or send a query. Review the local recipient
and grant fresh local-transfer, public RCSB-search and provider permissions as described in
[PROTEOSCOPE.md](PROTEOSCOPE.md). A sequence-similarity result does not establish binding.

## Scientific basis and example

Family, reference, mutation and isotype methods remain those of the selected saved parents;
see [the research record, sections 5.16–5.20 and 5.71](../clonoloom-spec/research.md).
The ordered criteria, strict missingness policy and tied ranks are ClonoLoom review choices,
not a published predictor. In their studied plasma-cell repertoires,
[Neumeier et al. (2022)](https://doi.org/10.1073/pnas.2113766119) reported that clonal expansion
and mutation burden did not predict measured affinity. That context supports caution; it does
not establish the absence of associations in every repertoire or validate this ranking policy.

The built-in **simulated B-cell immunization** example supports ranking after families, SHM and
isotypes are built. Its original inputs do not contain `sequence_aa`, so this export remains
unavailable. The smaller [candidate fixture](../testdata/candidates/README.md) adds declared
artificial upstream amino-acid annotations to copies of historical controls for the full workflow.
Open its six TSVs and repertoire metadata, use subject-scoped paired families at the demonstration
threshold 0.1, then the historical simulated germline reference, SHM and isotypes. Review S01-d28
against S01-pre, prefer IgG, and order share change descending then divergence descending.
The two families contain 13 and 11 current cells, with two baseline cells each. The small counts
are transparent software controls, not an adequately powered biological study. Both-theme
screenshots use these same native results and explicitly identify the simulated study.

Validation uses independent Python `Fraction` arithmetic to reconstruct membership, denominators,
missingness, priority ordering and ties. Exact native JSON/source/sequence/FASTA hashes are checked
for API and both-theme browser downloads. Biopython 1.86 independently checks the artificial
fixture translations under NCBI table 1; it is a validation dependency only. Native tests pin
ARM/Intel result digests and exercise missing-source, changed-evidence and permission refusals.
These checks validate implementation and byte preservation, not biological prediction.
