# Simulated public-overlap evidence

All junctions, subject IDs, counts, repertoire IDs and metadata here are artificial. No human
records or provider data were copied. These fixtures are dedicated to the public domain under
CC0-1.0. The repository hostname models the pinned ADC request contract, not a live request.

Regenerate with `UV_CACHE_DIR=/tmp/clonoloom-uv uv run --offline --no-project
validation/reference/adc_overlap.py --write` from the repository root. Validate without
`--write` to independently recompute the expected results using sets and rational arithmetic.
The native tests and browser capture consume the same declared inputs, not a mocked result.

The query has 83 rows, including 40 unique eligible TRB junctions, duplicate observations and
three exclusions. Captures A and B overlap by design; C has only nonproductive rows; D lacks
a subject ID. B ends on an empty page while the others exhaust their one-page budget. Every
capture remains incomplete. Count units are deliberately unknown and weights are unused.
A metadata extension larger than JavaScript's safe integer range is an exact-token control,
not a cell count. Sparse metadata do not form a complete MiAIRR example.

These varied-length synthetic junctions are structurally plausible test strings, not outputs
of a biological recombination model. Sharing was designed to check arithmetic and missingness,
not to estimate realistic cohort prevalence, assay effects or antigen specificity.
