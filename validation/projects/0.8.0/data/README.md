# Artificial batch-QC study

Seed 20261101; 26 samples, 25 fictional subjects. 24 included subjects across four batches (8,8,3,5); one explicitly excluded repeat and one missing-batch exclusion.
Human TRB junctions use ClonoLoom's length-selected model. Nonproductive controls insert an internal stop codon; flags and missing genes/counts are deliberate construction. Counts use shifted gamma-Poisson variation with lognormal sample exposure. Run A has a planted low-productivity sample and a missing-count control; run B has a productivity shift; run C is too small for scoring; run D has constant row counts. These demonstrate descriptive QC, not calibrated sensitivity or real biological batch effects. No correction or automatic removal.
Use truth/samples.json for explicit assignments/exclusions. All row-level generating observations and source hashes are retained.
