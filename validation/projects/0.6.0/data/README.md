# An artificial paired T-cell and expression study with supplied Gaussian clouds

Artificial study, model clonoloom-artificial-cell-states-clouds-v1, seed 20261028; ClonoLoom 0.6.0.

Open the root AIRR tables and repertoires.airr.json as one receptor dataset.
For each sample select expression/<sample> and its cells.tsv explicitly in Cells.
The supplied map is artificial layout, not UMAP, clustering or antigen specificity.
Repeated barcodes are scoped to samples. Truth is in truth/observations.tsv and truth/truth.json.

Two samples per subject, 360 cells per matrix, 10 clones per subject.
24 artificial genes in three blocks of eight; means 16 in the state's block, 2 elsewhere;
independent negative-binomial counts, size 4 (variance mean + mean squared / 4).
Clones are uniform; first-sample state is clone index modulo 3, second-sample state
advances modulo 3 with probability one half. These are chosen software test parameters.
Cell 1 lacks receptors; cell 2 lacks annotations; cell 3 lacks coordinates; cell 4 lacks a label;
the final matrix cell has all-zero expression. One extra paired receptor barcode and one
annotation-only barcode are included. No biological dropout or sequencing-error model is fitted.
Supplied Gaussian layout: x = 8*state + 1.5*N(0,1); y = 3*sample + sigma*N(0,1),
with sigma [1.2,2,0.8] by state (zero-based state/sample). Independent layout stream
50000 + 2*subject + sample; original integer jitter draws are retained and discarded
to preserve all biological observations at fixed parameters. Layout is illustrative,
not fitted or computed from expression. Ten uniform clones give first-sample state
probabilities 0.4,0.3,0.3 in the preset; these probabilities depend on clone count. Fictional SIM calls and random
stop-free 42-nt junctions; no real alleles. All data are artificial and unrestricted.
See docs/SIMULATION.md for the sampler references and reproducibility contract.
