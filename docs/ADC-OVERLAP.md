# Observed overlap with captured public evidence

Development: wave 9, slice 9. Open a local dataset, then choose **Data → Public overlap**.
Select one query sample, one locus and 1–20 ADC captures already in this library. Use the
capture folder recorded by the [reviewed download](ADC.md#reviewed-downloads-wave-9-slice-8)
and inspect its manifest to identify the repository, repertoire and processing run before
selection. Review the separate presence contract, check its acknowledgment and compare.
No repository is contacted; this works offline. The local query is never uploaded.

The selected captures are the entire declared comparison universe. They are **observations
of selected records**, not complete public cohorts or a census of public receptors. Review
species, study design, sample source, assay and sequencing depth yourself before interpreting
sharing. ClonoLoom does not establish their equivalence or identify the same person across
studies or repositories. Provider permission and citation statements remain those recorded by
the person at download; hashes do not establish permission or provider authenticity.

## Matching contract

The versioned definition is `productive-single-chain-locus-exact-junction-aa-v1`:

- One of TRA, TRB, TRG, TRD, IGH, IGK or IGL, matched exactly.
- Supplied productive status must be true. False, null, missing and malformed public flags
  are ineligible; they are not reannotated. Local flags are the loaded reader's interpretation.
- `junction_aa` must contain 3–256 canonical uppercase amino acids, begin with C and end
  with F or W. This is the full anchored junction convention, not anchor-free CDR3. No
  trimming, case folding, ambiguity expansion, approximate matching or translation occurs.
  Unusual/noncanonical junctions are deliberately excluded; eligibility is not a biological
  functionality certificate. Missing/wrong-type public junction or locus values are excluded.
- The key is locus plus exact junction amino-acid sequence. Repeated eligible rows collapse
  to presence. Sequence IDs validate capture identity but do not define shared receptors.

This is separate from the project's active clonotype definition and QC filter. It does not
match V/J genes, nucleotides, paired chains or complete receptors. Different V/J assignments
with the same locus and junction match under this contract; distinct nucleotide rearrangements
can encode the same amino-acid junction. Presence does not demonstrate shared ancestry,
biological identity or antigen specificity. No counts are used as weights, and no reads,
UMIs, templates or cells are inferred from public `duplicate_count`. Count values, including
zero or missing, do not determine eligibility: this measures presence of supplied sequence
records, not verified positive biological abundance. The query's source unit
is displayed as recorded by its reader, including unknown. Abundance indices and rarefaction
remain the separate [within-project overlap](https://robert-mcdermott.github.io/clonoloom/docs/overlap.html) workflow.

## Counts and unavailable results

For query set A and captured set B, **shared = |A ∩ B|**, **union = |A ∪ B|** and
**Jaccard = shared / union**. Both sets must be nonempty. Otherwise Jaccard is `null` and the
UI says **Unavailable**, even when the mathematical union is nonzero. This avoids interpreting
an empty eligible observation as evidence of biological absence. Two nonempty disjoint sets
have Jaccard zero. Raw, eligible, excluded and unique counts accompany each observation.

Jaccard is shown to four significant digits; exact integer intersection/union counts and native
binary64 ratio values remain in the saved JSON. No uncertainty interval, prevalence estimate,
p-value, normalization for assay/depth or complete-cohort extrapolation is computed.

A subject ID is scoped by **repository + study ID + subject ID**. Missing either study or
subject leaves the capture unassigned. Assigned subjects with both eligible sets nonempty
are evaluable; the others are counted as missing/unevaluable for this comparison. Unassigned
captures remain visible in pairwise overlap and aggregate key sharing, but do not enter the
subject denominator. Missingness is not a negative biological observation. An empty query
makes every assigned subject comparison unevaluable.

The shared-key inspector reports capture support and scoped subject support among the
selected evidence. Duplicate rows contribute only once per capture. Subject support uses the
evaluable denominator above; it is not population frequency. The first 100 keys are displayed,
with every observed shared key retained in the native result. Query keys seen in no capture
are not listed, and nondetection cannot establish their absence from the underlying cohort.

Repeated captures of the same repository/repertoire/processing scope are refused. Multiple
captures for the same scoped subject are also refused: select one observation deliberately.
The application will not silently pool visits, tissues, assays or alternative processing runs.
Different scoped IDs are not proof of independent people; mirrored repositories can contain
the same study or participant, and independence must be checked externally.

## Evidence and limits

Every selected manifest, original metadata/page checksum and byte count, exact request and
request checksum, endpoint, timestamp, selected processing entry, record scope, unique sequence
ID and stop reason is checked before comparison. Fixed member names and regular files are
required; symlink members/directories and inconsistent captures are refused. Supported native
capture format is version 1 under the pinned ADC contract/registry. All captures must retain
`completeness: not-established`. An empty terminal page does not certify a stable snapshot;
page-budget capture can omit arbitrarily many records. There is no automatic retrieval.

Budgets: one loaded dataset of at most one million rows, one sample, 1–20 captures, at most
10,000 records per capture under its existing download budget, 8 MiB per source page, 256 KiB
per manifest and 64 MiB total selected capture bytes (including manifests), with a one-minute
comparison deadline. A valid larger download may exceed this comparison budget and be refused.
Failed validation produces no comparison result. The loaded query snapshot is used; its original
files are not reread. Reopen the dataset to intentionally use changed source files.

Each success writes a new private `adc-overlap-*.json` file in the library, without replacing
prior results. It records the query dataset ID, sample name/index, full source/metadata hashes,
reader identity, source units, matching definition, selected capture manifest hashes, scoped
IDs, citations/permission statements, stop reasons, counts and all shared-key support. Keep
this result with the original captures and local source files. Result JSON is an observational
report, not a new project artifact, portable archive or exact remote replay certificate. It is
not automatically added to project history, reports or MCP/Python/R action catalogs. Existing
scientific replay kinds and automation permissions are unchanged.

Changing inputs invalidates acceptance and cancels pending comparison locally. Close or Cancel
discards pending UI results. A result saved just before cancellation may remain in the library.
The result may contain sensitive query junctions and study/subject identifiers; its private file
mode does not de-identify them. No automatic sharing or redistribution occurs.

## Scientific basis and validation

The [AIRR Rearrangement schema](https://docs.airr-community.org/en/v1.5.1/datarep/rearrangements.html)
distinguishes junctions, flags, loci and count annotations. The
[immunarch overlap documentation](https://immunarch.com/articles/web_only/v4_overlap.html)
defines the finite-set Jaccard statistic used here. Exact counts, rather than abundance
weights, are appropriate for this deliberately presence-only contract. The
[AIRR Community Guide, repertoire analysis §§3.6–3.7](https://www.ncbi.nlm.nih.gov/books/NBK586971/)
discusses public/shared sequences, convergent recombination and interpretation constraints.
These methods sources do not certify the scientific validity of a selected cohort.

`testdata/public-overlap` is an explicitly artificial fixture: 83 query rows, varied plausible
TRB junction lengths, repeated observations, unequal sequence support, selected overlap,
nonproductive evidence, a missing subject and both stop reasons. It is not a human study,
repertoire-generation model, power calculation or estimate of expected sharing. Source count
units remain unknown. A large-integer metadata extension checks exact transport, not cell counts.
The screenshot uses the actual native comparison of these captured fixture files.

```bash
UV_CACHE_DIR=/tmp/clonoloom-uv uv run --offline --no-project \
  validation/reference/adc_overlap.py
GOCACHE=/tmp/clonoloom-go-cache go test -race \
  -run 'TestPublic|TestCapture|TestADCPublic' . ./internal/adc
```

The independent standard-library reference uses CSV, exact JSON, sets and rational arithmetic.
It checks all row/set counts, shared-key support, subject denominators and ratios. Native/API
controls cover altered evidence, request/scope changes, empty sets, duplicates, missing review,
local-only access, cancellation and byte budgets. Both-theme browser checks compare native
results with that reference and exercise acceptance, edited inputs, late responses, cancellation,
errors and concurrent submissions. Validation uses no live repository or provider source code.
