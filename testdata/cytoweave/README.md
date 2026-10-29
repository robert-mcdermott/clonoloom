# Artificial index-sort identity controls

No patient, study or acquired FCS data. Created for wave 9 slice 13; repository license applies.

- `wells.csv`: 96 unique wells in row-major order, one event per well. Deterministic seed 913,
  Python standard-library Gaussian/log-normal sampling; unequal CD4/CD8-like artificial gate
  labels and linear compensated-like channel values. No measured compensation or fitted gates.
  A4 has an empty population and CD4 value; A7 has a literal NaN CD8 value.
- `mapping.csv`: A1 through H6 map to BC00001-1 through BC00090-1. H7–H12 are unmapped.
- `receptors.tsv`: sample A has 80 mapped cell IDs and four additional receptor-only IDs, two
  chain rows per cell, plus one row without a barcode. Ten mapped targets are absent. Sample B
  reuses BC00001-1, BC00081-1 and BC00082-1: these must never fill sample A's absent targets.
  Repeated artificial junctions are format controls, not a representative clonotype distribution
  or confirmation of physical pairing.
- `provenance.json`: explicit artificial context; no upstream FCS hash/matrix is invented.

Independent expected coverage in sample A: 80 linked, 6 unmapped, 10 absent targets,
4 receptor-only, 84 distinct receptor cell IDs, one missing-barcode row. In sample B:
3 linked, 87 absent targets, 6 unmapped, no receptor-only cells.

The separate `validation/cytoweave-links.mjs` uses the actual Apache-2.0 CytoWeave artificial
index-sort FCS generator, reader, compensation and browser exporter; it does not substitute
these hand-authored format controls for real sibling interoperability.
