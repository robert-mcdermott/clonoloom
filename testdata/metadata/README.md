# Simulated AIRR metadata inspection

These wholly artificial files illustrate a longitudinal bulk RNA T-cell workflow. The table
contains four small repertoires (four rearrangements each), counted in UMIs, not cells. The
records describe baseline and day-28 blood draws for S01 and baseline for S02. R004 deliberately
has no metadata; metadata-only R005 describes S03 without a corresponding table. Coverage is
therefore three matched records of four, one unmatched record, and one sample without metadata.
R005 is retained, not interpreted as a biological exclusion. S01's repeated draws are not
independent subjects. Metadata suggests this design; it does not validate it.

Sequence strings, gene calls, counts, subject identities, labels and processing identifiers are
illustrative. They are not empirical data, an annotation benchmark, antigen specificity evidence
or a statistically powered experiment. The metadata is intentionally incomplete for MiAIRR.
AIRR 2.0 field shapes follow the [Repertoire schema](https://docs.airr-community.org/en/v2.0.0/datarep/metadata.html).
No claim of full schema validation is made. `lab_extension` exercises preservation of extra
fields, false and null; day zero remains zero. Exact large-number stress cases live in the
parser/API tests rather than appearing as implausible biological counts in screenshots.

The documentation capture `repertoire-metadata` opens these files, checks coverage and original
values, runs accessibility checks, and captures both themes. No inference or new plot is made.
