# TRUST4 reports and VDJtools tables

Wave 9 slice 5 development adds native readers for TRUST4 repertoire reports and VDJtools
clonotype tables. This is not part of the published 0.8.0 binary. Open the TSV files (optionally
gzip) in Data, then choose **Inspect native report**. Explicit saved column mappings still take
precedence, so older projects keep their interpretation and identities.

## Supported headers

Recognition uses columns, not filenames. Column names are compared without ASCII case; the
leading `#` on `count` is accepted. Required fields are count, frequency, CDR3 nucleotides and
amino acids, V, D and J. A missing D call is a value (`.`), not an omitted column.

- **VDJtools:** `count`, `frequency` (or `freq`), `CDR3nt`, `CDR3aa`, `V`, `D`, `J`.
  Optional segment boundaries and arbitrary annotation columns remain source evidence.
- **TRUST4 modern report:** the same core fields plus `C` and `cid`. `cid_full_length` and
  other extensions are retained without interpreting consensus completeness as productivity.
- **TRUST4 legacy report:** `read_count`, `frequency` (also
  `frequency(proportion of read_count)`), `CDR3_dna`, `CDR3_amino_acids`, `V`, `D`, `J`, `C`.
  Optional `consensus_id` and completeness columns are retained. The consensus ID, when present,
  becomes the sequence ID; repeated IDs are not pooled into cells or clones.

Duplicate names/aliases, both count columns, absent required columns and incorrect row widths
are refused. A generic table with `C` but no TRUST4 consensus ID or explicit `read_count` has
ambiguous marker conventions and requires a reviewed column mapping or AIRR export. TRUST4's
barcode report, intermediate contig/CDR output and nonstandard layouts are not this report format.
TRUST4 AIRR exports continue through the AIRR reader. Additional AIRR-like columns do not license
inferring a different biological object or pairing.

TSV fields may use the existing reader's single-line outer double quotes and doubled quotes;
embedded tabs/newlines within quoted fields are unsupported. A UTF-8 BOM and CRLF are accepted.
The header must be first. Blank data lines are ignored; comments/preambles are not skipped.

## Counts, missing values and source frequencies

Modern TRUST4 `count` can reflect assigned read support or barcode-count output; VDJtools counts
can be reads or molecules. Both remain **unknown unit** on native import. Use the existing Data
unit confirmation only when the upstream experiment establishes reads or UMIs. A barcode-count
report is not a list of identified cells and does not establish paired receptors. For such data,
retain the unknown unit or use a suitable explicit interchange workflow; do not label barcodes as
UMIs or manufacture single-cell identities. The legacy column `read_count` explicitly states reads.

Counts are nonnegative plain decimal tokens of at most 128 characters and at most `2^53 − 1`.
TRUST4 fractional support is retained for analysis; VDJtools counts must be whole numbers
(`2.0` is accepted). Count exponent notation, negative values, NaN/Inf and larger values are
refused, not rounded into plausible integer counts. Fractional TRUST4 counts use the existing
float64 analysis table; not every decimal has an exact binary representation. The source decimal
text remains exact in native evidence. Integer-only scientific estimators retain their own refusals.

Empty, `.` and `NA` are missing. Zero is a supplied zero. Missing counts stay missing, never one
or a count reconstructed from frequency. Missing genes stay missing; `*` also means a missing
gene call. Alternative gene calls remain together, with the existing gene/locus checks; no first
call is silently selected. Sequence IDs are retained without constructing cell or clone IDs.

Source frequency is preserved as text and checked against the exact interval [0,1]. Scientific
notation is supported, bounded to 128 characters and exponent −308…308. Frequencies are not
summed, renormalized or substituted for counts: TRUST4 can have separate IG/TR denominators,
and a VDJtools subset can retain frequencies relative to a larger sample. ClonoLoom analyses
calculate their own frequencies from the selected rows/counts and explicitly chosen unit.

## Junctions and flags

Both source conventions include the conserved junction anchors; ClonoLoom does not add or remove
residues. Supported nucleotide input is ASCII A/C/G/T/N (case-insensitive). Ordinary peptides use
the twenty standard amino acids, X and stop `*`; the format-specific markers below are handled
before normalization. Other alphabets/notations are refused; no missing peptide is translated.

| Source notation | Parsed interpretation |
| --- | --- |
| TRUST4 `_` | Stop `*`; not a frameshift marker |
| TRUST4 `?` | Ambiguous residue X; stop status is unknown unless an explicit stop is present; productivity remains unknown unless a stop/out-of-frame state already rules it out |
| TRUST4 `out_of_frame` | No amino-acid junction; nonproductive/out of frame; supplied nucleotides retained |
| TRUST4 `partial` | Source sequence retained only in native evidence; no complete junction or productive flag invented |
| VDJtools `_` or legacy `~` | Incomplete codon: no continuous amino-acid junction; nonproductive/out of frame |
| Either format `*` within peptide | Stop; nonproductive |

For ordinary complete peptides, frame is derived only when nucleotides are supplied. A stop or
out-of-frame length implies nonproductive; a supplied nonambiguous peptide with whole nucleotide
codons implies productive under this junction-level convention. Otherwise productivity stays
unknown. Existing validation separately checks translation agreement, anchors and gene/locus
consistency. These are not full-transcript functionality, experimental receptor pairing,
ancestry, antigen-specificity or biological-quality certificates. Partial rows remain in input
QC and their source counts remain visible, but they cannot define complete-junction clonotypes.

## Evidence, bounds and compatibility

The original header and each accepted physical row are captured as UTF-8 text, excluding line
terminators (and the header's BOM). This retains unsupported boundary/annotation columns and
all numeric tokens without JavaScript number conversion. Original line numbers and compressed
file fingerprints identify the evidence. This is a row snapshot, not a byte-for-byte copy of
blank lines, line endings or gzip framing. The inspector reads that snapshot after source removal.

Evidence is retained in the private disposable table cache, explicitly separate from generic
source metadata. It is not silently inserted into scientific exports or private identity maps.
Source inclusion in portable archives keeps its existing explicit permission/restriction checks.
AIRR/result exports contain the mapped analysis fields, not a lossless native report export;
keep the original source when transferring unsupported annotations.

Limits: 16 MiB per physical line, one million data rows per native file, 64 MiB captured header/row
text per file and combined opening. Oversized/malformed inputs are refused without a partial
import. Inspector pages default to 20 rows (API maximum 50); a header/row preview over 4 MiB is
explicitly refused without truncating stored evidence. Empty reports can be opened but contain
no evaluable rows. Counts of files/rows do not represent independent biological replicates.

Reader-cache version 5 (shared with the Cell Ranger JSON reader) invalidates disposable old cache entries. Scientific identity version 1,
prior valid reader values, saved explicit mappings, release fixtures, automation permissions
and the five supported replay kinds remain unchanged.

## Validation and sources

`testdata/native-reports/` contains three declared artificial report files. The independent
standard-library CSV/Decimal reference checks every mapped field, exact source rows and SHA-256:

```sh
UV_CACHE_DIR=/tmp/clonoloom-uv uv run --no-project validation/reference/native_reports.py
GOCACHE=/tmp/clonoloom-go-cache go test ./validation -run TestNativeReportsIndependentValues
```

Native controls cover marker differences, partial/missing/zero cases, count/frequency bounds,
malformed headers/rows, cancellation, gzip/uploaded filenames, cache persistence, source removal,
explicit mapping precedence and fuzzing. Both-theme screenshots show simulated source evidence,
not inferred biology or research results.

Format references reviewed 2026-10-29:
[TRUST4 input/output documentation](https://github.com/liulab-dfci/TRUST4/blob/master/README.md),
[TRUST4 report helper options](https://github.com/liulab-dfci/TRUST4/blob/master/trust-simplerep.pl),
[VDJtools input specification](https://vdjtools-doc.readthedocs.io/en/master/input.html),
[VDJtools interchange notation](https://resources.qiagenbioinformatics.com/manuals/biomedicalgenomicsanalysis/current/index.php?manual=Import_Export_VDJtools_Clonotypes.html).
Methods context: [Song et al., 2021, TRUST4](https://doi.org/10.1038/s41592-021-01142-2) and
[Shugay et al., 2015, VDJtools](https://doi.org/10.1371/journal.pcbi.1004503).
These readers implement the documented table contracts, not either tool's reconstruction or
analysis algorithms. No vendor runtime dependency is added.
