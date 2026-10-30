# Artificial candidate-review inputs

These six AIRR TSVs are copies of the small, explicitly simulated B-cell immunization controls in
`validation/projects/0.5.0/data`. All 120 original source rows and fields are preserved, together
with the original repertoire metadata. Historical files and checksums are unchanged.

For the sequence-export example only, `validation/reference/make_candidates.py` adds three fields:

- `sequence_aa`: 119 upstream translations of the artificial in-frame `sequence`, using the
  [NCBI standard genetic code (table 1)](https://www.ncbi.nlm.nih.gov/Taxonomy/Utils/wprintgc.cgi#SG1).
  The first S01-d28 row deliberately has no amino-acid annotation, even though nucleotide sequence
  is present. Product review must retain that missing value and refuse FASTA for it.
- `complete_vdj`: empty. Neither V(D)J completeness nor full protein completeness is asserted.
- `candidate_fixture`: explicitly labels the artificial upstream annotation and its limits.

These fictional SIM genes and sequences demonstrate software behavior. They are not patient
data, experimentally characterized antibodies, validated paired constructs or synthesis-ready
sequences. Translation occurs only in this fixture generator; ClonoLoom's export reads the supplied
field unchanged. The independent validator uses Biopython 1.86 to verify all supplied translations
and every original field. No runtime biology library is added.

From the repository root, regenerate with:

```sh
uv run --no-project validation/reference/make_candidates.py
```

The generator prints each historical source SHA-256. The candidate API and browser controls build
families and all optional parents from these inputs, using the existing artificial reference at
`validation/projects/0.5.0/references/simulated-bcell-immunization/reference/germlines.fasta`.
S01-d28 has 24 assigned cells, S01-pre has four; there are two S01 families at paired threshold 0.1.
The full example and scientific limits are in [BCELL-CANDIDATES.md](../../docs/BCELL-CANDIDATES.md).
