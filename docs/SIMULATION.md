# Simulated studies

`clonoloom simulate` writes a bulk T-cell receptor β study whose truth is known: every clonotype of
every subject, and the numbers an analysis estimates (diversity, the clonotypes a sample of its
depth shows, the clonotypes subjects share). It is for checking methods, ClonoLoom's or anyone's,
against the truth, and for learning ClonoLoom without data of your own. The Example data dialog
offers two of them, made on your computer in a few seconds.

```sh
clonoloom simulate                         # the standard study, into clonoloom-simulated-standard
clonoloom simulate --preset null --seed 7  # another study, another seed
clonoloom simulate --print-params > study.json   # the parameters, to edit
clonoloom simulate --params study.json --out my-study
```

The same parameters and seed give the same study, byte for byte, on every computer.

## Presets

| Preset | Subjects | Samples | What it is for |
| --- | --- | --- | --- |
| `standard` | 3 control, 3 with expanded clones (fewer clonotypes, a heavier tail, five clones holding 22% of the cells) | 2 of each subject, 12,000 to 45,000 templates | diversity, rarefaction and overlap, with a real difference between the groups |
| `null` | 6 of one kind, in two groups | 2 of each | a comparison of the groups should find nothing |
| `small` | 2 | 1 of each | tests |

## What it writes

| File | What it holds |
| --- | --- |
| `S01-1.tsv`, … | A sample's rearrangements as an AIRR rearrangement table: `umi_count` its templates, `duplicate_count` its reads. Calls are at gene level. A `sequence_id` names the sample and the population clone it came from. |
| `repertoires.airr.json` | Each sample's AIRR Repertoire metadata: subject, condition (as `study_group_description`), tissue. |
| `truth/truth.json` | The parameters and seed; each subject's population; each sample's depth and expectations; the subjects' shared clonotypes. |
| `truth/S01.tsv.gz`, … | Every clone of a subject's population: its genes, deletions, insertions, junction and cells. |
| `README.md` | What the study is. |

Open the folder in ClonoLoom (or the `.tsv` files with `repertoires.airr.json`) as one dataset: its
truth folder is left out.

### The truth

For each subject's population, by ClonoLoom's default definition for bulk data (productive
rearrangements grouped by amino-acid junction, V gene and J gene), as the analyses would measure it
were every cell sequenced:

- `clonotypes`; Hill numbers `hill0`, `hill1`, `hill2` (richness, the exponential of Shannon's
  entropy, the inverse of Simpson's index); `shannon` (natural log), `simpson` (Σp²), `evenness`
  (Pielou's H/ln S), `clonality` (1 − evenness), `gini` (of the clonotypes' cells), `top10Share`;
- `homeostasis`: the share of cells in rare (frequency ≤ 10⁻⁵), small (≤ 10⁻⁴), medium (≤ 10⁻³),
  large (≤ 10⁻²) and hyperexpanded (> 10⁻²) clonotypes.

For each sample: the templates drawn and seen, the productive templates, the reads, the
clonotypes seen, and the clonotypes and coverage a sample of its depth shows on average
(`expectedClonotypes` = Σ 1−(1−p)^m, `expectedCoverage` = 1 − Σ p(1−p)^m, over the population's
frequencies p and the sample's m productive templates). For each pair of subjects: the clonotypes
their populations share, and their Jaccard index.

## The model

- **Germline:** the human TRB V, D and J gene segments of the GRCh38 genome at Ensembl's
  annotation (no restrictions).
- **Recombination:** a V, a J, and a D the J allows; deletions at each end, or a few palindromic
  nucleotides; non-templated insertions between V and D and between D and J (Murugan et al.
  2012). Its parameters are ClonoLoom's own (no OLGA or IGoR model file is used).
- **Usage and selection:** V and J usage are those of 10x Genomics' 10k PBMC data (CC BY 4.0);
  productive rearrangements are selected on their junction's length so that their lengths are
  that data's too (mean 14.5 amino acids), as Elhanati et al. 2014 estimate selection.
- **Repertoires:** clone sizes from a power law; 15% of clones nonproductive; a few expanded clones
  where a preset asks.
- **Sequencing:** templates drawn in proportion to the clones' cells; reads per template negative
  binomial (mean 8); a template with no read is not seen.

It is a simple, documented model for testing methods against a known truth, not a model inferred
from data like IGoR's: its junctions look like real ones, but they are not a substitute for real
repertoires. The details, with their sources, are in
[clonoloom-spec/research.md](../clonoloom-spec/research.md#52-simulated-studies-wave-2).

Simulated data describe no person, and may be used and shared without restriction.
