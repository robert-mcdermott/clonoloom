# Suite and public-data study

ClonoLoom 0.9.0 connects imported evidence, explicit public-data comparisons and reviewed
interchange. This guide uses **separate artificial studies** to demonstrate the supported
boundaries. Do not combine their subjects, samples or statistical denominators. The compatibility
project in `validation/projects/0.9.0` records the original data, project, saved parents and review
bytes; `project.json` is a separate portable serialization for reopening. No patient data, live
repository response or external sequence-search result is presented as a simulated observation.

## 1. Establish the local evidence

Open rearrangements with their original AIRR Repertoire JSON or supported YAML. In **Data →
Inspect AIRR metadata**, inspect the repertoire ID, match basis, original record and source hash.
An explicit unmatched ID stays unmatched; a sample-name fallback cannot repair it. YAML's decoded
JSON projection is distinct from the retained original YAML. Preserve large integers, precise
numbers, explicit null, false and zero. Do not copy browser-rounded values into evidence.

Use **Review MiAIRR reporting completeness** to inspect the pinned AIRR 2.0 checklist and each
processing entry separately. Absent or unsupported schema declarations remain **not assessed**.
Required-field presence is a reporting assessment, not scientific validity, ontology validation,
consent or provider permission. Do not change a schema version merely to obtain a checklist.
The distinction follows [MiAIRR](https://doi.org/10.1038/ni.3873) and the
[AIRR metadata specification](https://docs.airr-community.org/en/v2.0.0/datarep/metadata.html).

Imported Clone, Cell and Receptor objects retain source assertions and unresolved links. They do
not replace the active clonotype definition or prove biological pairing. Inspect detailed Cell
Ranger JSON, TRUST4 and VDJtools inputs through **Inspect native report**, including unsupported
annotations and declared units. Their layout recognition is bounded; it does not certify a
pipeline's scientific correctness. See [AIRR objects](AIRR-OBJECTS.md), [YAML](AIRR-YAML.md),
[native tables](NATIVE-REPORTS.md) and [Cell Ranger JSON](CELLRANGER-JSON.md).

The release controls retain exact-number stress values, missing annotations and out-of-scope
object links. These are labeled software controls, not plausible biological measurements.

## 2. Review a public comparison universe

In **Data → Public repositories**, choose the approved endpoint and filters, inspect the exact
request, then explicitly send it. Opening the dialog does not contact a repository. Only the
selected metadata filters are sent, not local receptor sequences. A returned page is not a complete
cohort; a service error, empty response and unavailable service are different outcomes.

For download, select one exact repertoire and processing ID, review provider permission evidence,
citation and size limits, then accept. Retain original pages, retrieval time, endpoint, request,
SHA-256 inventory and completion/stop reason. An exhausted page sequence does not establish that
an underlying biological cohort was completely sampled. A checksum establishes byte identity,
not provider authenticity or redistribution rights. Captures are separate from analysis datasets.

In **Data → Public overlap**, explicitly select the **local dataset**, one sample, one locus and
the captured folders. In a multi-dataset project, inspect the dataset selector rather than assuming
it follows the currently focused panel. Accept the named productive/exact-junction presence
contract. This comparison is offline and does not upload the local query. Missing subjects stay
outside the evaluable subject denominator; scoped subjects and ambiguous/unassigned captures stay
visible. Empty-set comparisons remain unavailable. No abundance weighting, inferred pairing,
antigen specificity or automatic visit/processing pooling is added.

The artificial TRB control has 83 local rows, 80 eligible observations and 40 distinct query keys.
Four separately captured controls retain missing/unassigned subject evidence and an empty eligible
set. Native counts and Jaccard fractions are checked against independent sets and rational
arithmetic. These selected subsets are not population prevalence estimates or independent
biological replicates. See [ADC](ADC.md) and [observed overlap](ADC-OVERLAP.md).

## 3. Carry a B-cell result through review

Use the separate [artificial candidate inputs](../testdata/candidates/README.md). They preserve
120 historical source rows across six samples from two artificial subjects and add explicitly
supplied upstream amino-acid annotations, with one deliberately missing value. Original source
fields and older release fixtures stay unchanged. The small example is a software control, not
an adequately powered biological study.

1. Open the six TSVs with their repertoire metadata. Build **subject-scoped paired families** at
   the demonstration threshold 0.1. This threshold is not a recommendation for real data.
2. Reconstruct D-masked germlines with the supplied artificial reference; build SHM, isotypes and
   parsimony lineages. Keep the threshold, reference SHA-256, evaluable positions, source rows and
   citations with every result. Family inference is operational similarity, not proven ancestry.
3. Select a bounded family/locus panel in Lineages and explicitly fit **JC69 likelihood**. Inspect
   branch units, zero/upper boundaries, near ties and conditional ancestral probabilities. The
   reference orients the display; it does not establish the root, dates, switching or migration.
   JC69 is an equal-rate baseline, not an SHM-context model. Numerical stationarity and exhaustive
   topology enumeration do not prove a globally optimal continuous fit or biological adequacy.
4. In Families, open **Review B-cell candidates**. Choose S01-d28 with S01-pre as the baseline,
   matching SHM/isotype parents, IgG as the explicit preferred class, and descending observed share
   change followed by descending divergence. The two S01 families have 13 and 11 current cells,
   with two baseline cells each. Shares therefore change by +4/96 and −4/96. This is descriptive
   sampling evidence, not a statistical expansion test or binding/efficacy prediction.
5. Inspect exact ratios, missing evidence and tied ranks. Select a current observation and one
   original chain; no representative is selected automatically. Review unchanged `sequence_aa`,
   completeness and pairing limits. The deliberately missing amino-acid field cannot be filled
   from nucleotide sequence. Save the full review, source companion and FASTA together after
   the explicit acknowledgments. A one-chain sequence is not a synthesis-ready paired receptor.

The [family methods](../clonoloom-spec/research.md#516-b-cell-clonal-families-wave-5-slice-4),
[likelihood contract](LINEAGE-LIKELIHOOD.md) and [candidate contract](BCELL-CANDIDATES.md) give
published sources and ClonoLoom-specific choices. All numbers come from native Go workers;
figures and tables display those saved values. Source recomputation and conditional numerical
validation do not establish biological ground truth.

## 4. Interchange with explicit permissions

| Destination | Review and preserve | Limits |
| --- | --- | --- |
| Excel | Select supported saved results; keep the complete workbook ZIP with original JSON, manifests, notices and certificates. | JSON-node rows are not subjects. Precision-sensitive numbers are text. Re-saving in a spreadsheet can alter evidence. B-cell/public-overlap reviews are not workbook input kinds. |
| Prism | Export one supported cohort feature/comparison result; retain the subject mapping, original evidence and missingness. | Data-only Column tables, with paired subjects aligned. No statistical analysis is selected. Import rounding and complete-case behavior remain separate from native evidence; no native Prism GUI validation is claimed. |
| Proteoscope | Select a supplied chain, review the local receiver, then separately approve local transfer, public RCSB search and applicable provider terms. | Opening from candidate review sends nothing. Search similarity is not receptor completeness, predicted structure, antigen specificity or binding. |
| CytoWeave | Keep source FCS/compensation/gating records, export wells, and explicitly map one acquisition to one selected receptor sample. | Exact well/barcode linkage does not infer cell identity or phenotype validity. Missing and conflicting mappings remain visible; subjects are not pooled. |

Use the existing [cohort study](site/pages/docs/cohort-study.html) for subject-level Excel/Prism examples; do not turn
B-cell source rows into independent subject replicates to fit an export format. The wave-9 browser
controls exercise these interfaces independently, with public searches intercepted. The release
compatibility fixture covers local evidence and scientific parents; it does not contain a real
public search or claim compatibility with every external application version.

## 5. Keep the right record

| Record or operation | What it establishes | What it does not establish |
| --- | --- | --- |
| Original source and native review hashes | Exact retained bytes and explicit choices | Biological validity, consent, authorship or authenticity |
| Portable project reopening | Stable dataset identities, source hashes and saved result files | Recalculation of every review or automatic restoration of external libraries |
| Release source-regression tests | Same scientific output from the recorded inputs/settings/reference | New public certificate replay support |
| Candidate/likelihood numerical checks | Correct descriptive ranking or conditional JC69 calculation | Calibrated affinity, uncertainty over all model choices or synthesis readiness |
| Existing certificates/replay | Exact supported captured cohort, batch-QC and motif evidence | Replay of ADC, candidate, likelihood or sibling-transfer reviews |

Keep standalone ADC captures and candidate/likelihood/sibling review files explicitly alongside
projects. They are not automatically project artifacts or certificate contents. Retained originals
can contain identifiers and annotations; exports are not de-identification unless that separate
workflow is explicitly used within its documented scope. Broader source workflows, replay kinds,
models and synthesis constructs remain outside 0.9.0.

## Reproduce the release controls

From a source checkout with Go, Node 22 and Chrome/Chromium:

```sh
node validation/create-project-0.9.mjs /tmp/clonoloom-suite-study
GOCACHE=/tmp/clonoloom-go-cache go test -run 'TestSuiteRelease|TestEveryReleasesProjectStillOpens/0.9.0' .
uv run --no-project --with biopython==1.86 validation/reference/candidates.py /tmp/clonoloom-suite-study/evidence
uv run --no-project --with numpy==2.3.5 --with scipy==1.18.1 validation/reference/lineage_likelihood.py /tmp/clonoloom-suite-study/evidence
```

Use a new output directory. The driver refuses overwrite, starts an isolated offline library,
uses explicit automation grants and accepts visible proposals only in that test window. The
existing reference action requires the network capability even for the already generated local
reference; `--offline` still prevents downloads. No user library, live repository or public sequence
service is used. The native fixture tests separately check the committed compatibility project.
The [validation record](../validation/README.md) states completed checks and environment limits.
