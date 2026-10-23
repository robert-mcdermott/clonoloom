# Simulated studies

`clonoloom simulate` writes a bulk T-cell receptor β study whose truth is known: every clonotype of
every subject, and the numbers an analysis estimates (diversity, the clonotypes a sample of its
depth shows, the clonotypes subjects share). It is for checking methods, ClonoLoom's or anyone's,
against the truth, and for learning ClonoLoom without data of your own. The Example data dialog
offers five of them, made on your computer in a few seconds.

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
| `car-t` | 2 responders, 1 non-responder | a week before infusion (twice), days 7, 14, 28 and 90, and the infusion product | tracking, expansion tests and attribution after a CAR-T infusion |
| `transplant` | 2 recipients without graft-versus-host disease, 1 with | before the transplant, days 30, 100 and 365 (twice), and the donor's blood | tracking and attribution to a donor |
| `null-series` | 4 of one kind | days 0, 14 and 28, twice each | expansion tests should find nothing |
| `vaccine` | 3 vaccinated, 2 given a placebo | a week before the vaccination (twice), days 7, 14, 28 and 90 | receptors of known specificity: similar receptors, their expansion, and matching to a specificity database |

The last four are studies over time, below; the last has receptors of known specificity too.

## What it writes

| File | What it holds |
| --- | --- |
| `S01-1.tsv`, … | A sample's rearrangements as an AIRR rearrangement table: `umi_count` its templates, `duplicate_count` its reads. Calls are at gene level. A `sequence_id` names the sample and the population clone it came from. |
| `repertoires.airr.json` | Each sample's AIRR Repertoire metadata: subject, condition (as `study_group_description`), tissue. |
| `truth/truth.json` | The parameters and seed; each subject's population; each sample's depth and expectations; the subjects' shared clonotypes. |
| `truth/S01.tsv.gz`, … | Every clone of a subject's population: its genes, deletions, insertions, junction and cells. |
| `README.md` | What the study is. |

Open the folder in ClonoLoom (or the `.tsv` files with `repertoires.airr.json`) as one dataset: its
truth folder is left out, and so is the vaccination study's database folder.

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

## Studies over time

A preset with a `series` samples each subject at several time points, in days from its reference
event (the infusion, the transplant), instead of `samplesPerSubject` samples at one time:

- **Trajectories:** classes of clones (a share of the productive clones, chosen at random, of the
  subjects of some conditions only if given) whose cells are multiplied by a fold at each time
  point. CAR-T: 2% of the product's clones expand (5×, 20×, 10× and 5× at days 7, 14, 28 and 90)
  and 20% contract (to 0.3×, 0.1× and 0.02× from day 14); transplant: in the recipient with
  graft-versus-host disease, 0.2% of the donor's clones expand (10×, 40×, 5× at days 30, 100, 365).
- **A reference population** mixed into the blood in a known share at each time point:
  - an **infusion product**, made from 30,000 cells collected from the subject's own blood (so it
    holds the subject's clonotypes, its largest among them), each clone then expanded ex vivo by a
    log-normal fold (σ = 1); its share of the blood is 10%, 25%, 10% and 3% at days 7, 14, 28 and
    90 in responders, a tenth of that or less in the non-responder;
  - a **donor's repertoire**, a population of its own, which is 60%, 85% and 97% of the recipient's
    blood at days 30, 100 and 365.
  It is sampled once, as the subject's reference sample, its tissue in the metadata (`infusion
  product`, `donor blood`), so ClonoLoom marks it as the subject's reference.
- **Biological variance:** in every sample, replicates at one time included, each clone's cells are
  its expectation times a gamma of mean 1 and variance 0.1, so two samples of one subject differ by
  more than sampling, as blood samples do (Rytlewski et al. 2019); replicates measure it.
- **Time points in the metadata:** each sample's `collection_time_point_relative` (in days), its
  unit, and `collection_time_point_reference` (the event), which ClonoLoom reads as the sample's
  time point (`7 day (infusion)`).

Samples are named by subject and day: `S01-pre-1` and `S01-pre-2` (before the event, twice),
`S01-d7`, `S01-d14`, `S01-product` (or `S01-donor`).

What their truth adds:

| File | What it holds |
| --- | --- |
| `truth/truth.json` | Each sample's `day` and `replicate`, or `reference: true`; the reference population's expected share of its cells (`referenceShare`) and the expected share of its productive cells in clonotypes the reference population holds (`attribution`, what attribution measures); each subject's reference population (its clones, cells and clonotypes) and how many clones each trajectory took. |
| `truth/S01.series.tsv.gz`, … | Each of the subject's productive clonotypes: the trajectories its clones follow, whether the reference holds it and its share of the reference's productive cells, and its expected share of the subject's productive cells at each time point (`frequency_pre`, `frequency_d7`, …). |

An infusion product made from the subject's own cells holds the subject's own clonotypes: before
the infusion, about three quarters of the blood is already in clonotypes the product holds. The
truth gives both the infused share and that share in the product's clonotypes, which are not the
same.

## Receptors of known specificity

A series' `response` adds receptors that recognize epitopes (the `vaccine` preset: three epitopes
of a vaccine, peptides made up for it):

- **Founders:** each epitope's (20, 15 and 10), rearrangements the generator makes, the same in every
  subject, as public receptors are.
- **Families:** in each subject, a Poisson number of receptors of each founder, whose mean is drawn
  once for the founder (exponential, mean 4), so that some families are large in every subject and
  many small. A member is a rearrangement of the founder's genes and deletions whose non-templated
  nucleotides differ: its CDR3 differs from the founder's at a Poisson number of residues (mean 1;
  none, and it is the founder itself), each changed to a residue TCRdist puts d units away with a
  chance in proportion to e^(−0.5d), mostly ones alike.
- **Private receptors:** each subject's own, unlike any other, twice as many as the epitope's
  founders on average: Dash et al. 2017 found a third or more of an epitope's receptors without a
  near neighbor.
- **Shares:** the receptors are a share of each subject's blood at each time point, by its
  condition (vaccinated: 0.1% a week before, 1%, 3%, 1% and 0.3% at days 7, 14, 28 and 90; placebo:
  0.1% throughout), divided among the epitopes (half, three tenths and a fifth), each receptor's
  cells from a power law; with the series' variance between samples.

Pooled across the five subjects, 58% of the receptors have another of the same epitope within 12
TCRdist units (with the V genes' loops), 63% within 24 and 68% within 48; Dash et al.'s β chains, 54%,
59% and 69% (human) and 61%, 71% and 85% (mouse).

What its truth adds:

| File | What it holds |
| --- | --- |
| `truth/specificity.tsv` | Every subject's receptors that recognize an epitope: genes, junction, epitope, antigen, founder (or private) and the residues by which it differs from the founder. Any other clonotype recognizes none. |
| `truth/founders.tsv` | Each founder: its epitope, rearrangement and family's mean size. |
| `truth/S01.series.tsv.gz`, … | An `epitope` column, and the trajectory `response` for its receptors. |
| `truth/truth.json` | Each subject's receptors by epitope (family members, founders with a family, members that are the founder, private receptors) and the files above. |
| `database/vdjdb.txt` | A specificity database in VDJdb's format (`vdjdb.txt`'s columns to `vdjdb.score`): each epitope's founders (score 3) and three receptors of other donors around each (score 1), with two epitopes no subject responds to. The MHC is `simulated`; the antigen's species, `Simulated`. |

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
