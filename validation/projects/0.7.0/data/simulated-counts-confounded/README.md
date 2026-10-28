# A fully confounded repertoire study

Artificial replicated-count benchmark v1, seed 20261106. 32 subjects, independent layout.
48 shared fictional receptors and 600 private receptors per subject, with a long-tailed abundance distribution.
Counts represent simulated UMIs (AIRR umi_count). Gamma-Poisson counts have heterogeneous dispersions; sampling exposures vary lognormally.
Shared receptors have subject effects and a 1.2-fold B2 batch effect. Four shared receptors increase fourfold,
four decrease by 2^1.5, and forty are unchanged (all effects zero in the null preset).
Batch is B1 for every control and B2 for every stimulated subject: batch and condition are fully confounded. Truth records every mean, dispersion, exposure, effect and count.
The exposure is a generator scale, not the realized library total: total-count-offset effects are relative repertoire effects.
Fictional stop-free junctions and SIM genes do not imply specificity or a biological recombination model.
Open root TSVs and AIRR metadata together; annotate the recorded batches before batch-adjusted modeling.

This design cannot distinguish a batch effect from a group effect. Retain the failed design check; do not fit or interpret a group contrast.
