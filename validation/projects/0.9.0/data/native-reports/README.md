# Simulated native repertoire reports

All sequences, counts, identifiers and annotations are constructed import controls, not observed
participants or validated receptors. Counts are deliberately uneven; fractional TRUST4 support,
separate IG/TR frequencies, incomplete junctions, stops, ambiguous codons, missing values and zero
are retained as distinct evidence. This is not a simulated biological treatment effect.

- `simulated-trust4.tsv`: modern report with exact frequency text and alternative source states.
- `simulated-trust4-legacy.tsv`: explicit read-count header and consensus-ID aliases.
- `simulated-vdjtools.tsv`: subset frequencies, frameshift/stop distinctions and preserved segment
  boundaries/annotations. The large external identifier tests textual fidelity, not clone abundance.

The literal source tables are the independent input; `validation/reference/native_reports.py`
uses standard CSV and Decimal parsing to generate/check `validation/reference/native_reports.json`.
It imports no ClonoLoom or vendor code. The fixture codons are illustrative. Native membership,
frequency and productivity interpretations are documented in `docs/NATIVE-REPORTS.md`.
