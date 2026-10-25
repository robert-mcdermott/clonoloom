# Getting started with ClonoLoom

The guide is on ClonoLoom's website, a page a subject:
<https://robert-mcdermott.github.io/clonoloom/docs/>

- **Start:** [Getting started](https://robert-mcdermott.github.io/clonoloom/docs/getting-started.html)
  (install, the examples, the window);
  [Opening data](https://robert-mcdermott.github.io/clonoloom/docs/opening-data.html) (your own
  files, another tool's table, the checks).
- **Quality and clonotypes:** [quality control](https://robert-mcdermott.github.io/clonoloom/docs/qc.html);
  [clonotypes](https://robert-mcdermott.github.io/clonoloom/docs/clonotypes.html).
- **Repertoires:** [diversity](https://robert-mcdermott.github.io/clonoloom/docs/diversity.html);
  [rarefaction](https://robert-mcdermott.github.io/clonoloom/docs/rarefaction.html);
  [genes and junctions](https://robert-mcdermott.github.io/clonoloom/docs/genes.html);
  [overlap](https://robert-mcdermott.github.io/clonoloom/docs/overlap.html).
- **Tracking:** [subjects and time points](https://robert-mcdermott.github.io/clonoloom/docs/subjects.html),
  [following clonotypes](https://robert-mcdermott.github.io/clonoloom/docs/tracking.html),
  [expanded and contracted clonotypes](https://robert-mcdermott.github.io/clonoloom/docs/tests.html).
- **B cells:** [artificial immunization workflow](https://robert-mcdermott.github.io/clonoloom/docs/simulation.html#immunization);
  [sequence evidence](https://robert-mcdermott.github.io/clonoloom/docs/bcell-inputs.html),
  [families](https://robert-mcdermott.github.io/clonoloom/docs/bcell-families.html),
  [germlines](https://robert-mcdermott.github.io/clonoloom/docs/bcell-germlines.html),
  [SHM](https://robert-mcdermott.github.io/clonoloom/docs/bcell-shm.html),
  [lineages](https://robert-mcdermott.github.io/clonoloom/docs/bcell-lineages.html) and
  [isotypes](https://robert-mcdermott.github.io/clonoloom/docs/bcell-isotypes.html).
- **Results:** [figures and exports](https://robert-mcdermott.github.io/clonoloom/docs/figures.html);
  [the record](https://robert-mcdermott.github.io/clonoloom/docs/record.html).
- **Reference:** [simulated studies](https://robert-mcdermott.github.io/clonoloom/docs/simulation.html);
  [remote control](https://robert-mcdermott.github.io/clonoloom/docs/remote-control.html).

The pages are made from [docs/site/pages/docs/](site/pages/docs/) ([docs/site/README.md](site/README.md)).
The references stay in this folder: [FORMATS.md](FORMATS.md), [REMOTE-CONTROL.md](REMOTE-CONTROL.md),
[SIMULATION.md](SIMULATION.md) and [INSTALLING.md](INSTALLING.md).


For expression-derived maps, open **Cells → Use cell-state study**, then **Compute expression
map**. The UMAP is fitted from counts independently of the example's supplied coordinates.
Inspect retained/excluded cells, change preprocessing settings explicitly and reopen the saved
result from Results. [Computed expression maps](EXPRESSION.md) explains the method, defaults,
limits and provenance. Continue through clone sizes, phenotypes, graph tests, cited methods and AnnData export in the
[complete single-cell study](https://robert-mcdermott.github.io/clonoloom/docs/cell-study.html).
