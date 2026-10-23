# Simulated studies

`clonoloom simulate` writes studies whose truth is known. The bulk T-cell receptor β studies
record every clonotype of every subject, and the numbers an analysis estimates (diversity, the
clonotypes a sample of its depth shows, the clonotypes subjects share). It is for checking methods, ClonoLoom's or anyone's,
against the truth, and for learning ClonoLoom without data of your own. The Example data dialog
offers six studies, including an artificial paired B-cell immunization study, made on your
computer in a few seconds.

```sh
clonoloom simulate                         # the standard study, into clonoloom-simulated-standard
clonoloom simulate --preset null --seed 7  # another study, another seed
clonoloom simulate --print-params > study.json   # the parameters, to edit
clonoloom simulate --params study.json --out my-study
```

The same parameters, seed and program version give the same study, byte for byte, on every computer.

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
| `bcell-immunization` | 3 artificial subjects, 12 B-cell families each | days −7, 14 and 28, 240 paired cells per sample | known ancestors, substitutions, isotypes and sampled abundance; see [Artificial B-cell families](#artificial-b-cell-families-wave-5-slice-3) |

The four TRB series are described below; `vaccine` has receptors of known specificity too.
The files and population statistics in the next sections describe the TRB presets; the B-cell
model has its own truth schema, described at the end.

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

## Artificial B-cell families (wave 5, slice 3)

Wave 5 slice 5 versions the model as v2. It adds explicit V/J CIGARs and ungapped germline
coordinates, exports `reference/germlines.fasta`, and gives the two distinct J backbones
separate fictional allele names. The simulated nucleotide sequences, mutation events,
sampling and family recovery are unchanged. Existing v1 files/results remain openable;
regenerate an example to obtain the new reference evidence. Never use the artificial reference
for biological sequences. Germlines currently offers the default example reference (seed
20261027); other seeds need their own exported reference and are outside the current reference
selector. The reference is generated from V/J backbones, without reading
family ancestors or sampled observations.


```sh
clonoloom simulate --preset bcell-immunization --out bcell-study
clonoloom simulate --preset bcell-immunization --print-params > bcell-params.json
clonoloom simulate --params bcell-params.json --out another-study
```

The **artificial B-cell immunization** example is available offline. Its first guided steps show
subjects in time and inspect a supplied sequence and ancestor alignment. The default is three
subjects, twelve families per subject, days −7, 14 and 28, and 240 paired cells per sample:
2,160 cells and 4,320 rearrangement rows. Each cell has one IGH and one IGK or IGL chain.
Family identity is subject-specific and kept in truth only; ordinary junction clonotypes and
their IDs retain their existing meaning. The next guided steps inspect paired nearest distances
and infer families at an explicit threshold of 0.1. Both paired and heavy-only inference recover
the exact 36 known families at that threshold; at 0.05 they yield 43 and 38 families respectively.
The threshold is a demonstration for these artificial data, not a biological recommendation.
The development SHM and lineage views analyze these saved projections; see below.

This is a deliberately artificial benchmark, model `clonoloom-artificial-bcell-v2`. The sequences
are generated coding strings, not biological antibody references. Gene calls containing `SIM`
are fictional. These data test bookkeeping and recovery under a known model; they do not establish
accuracy on real repertoires or calibrate a biological mutation rate.

### Model and parameters

`bcells` selects this model instead of the TRB model. Edit its `subjects` (1–20), `families` per
subject (1–100), `cellsPerSample` (1–10,000), and `mutationsPerBranch` per chain (1–20). The whole
study is limited to 100,000 sampled cells. Keep the TRB parameters zero/empty and omit `series`;
nonzero TRB settings are rejected rather than silently ignored. `name`, `title` and `seed` remain
editable. The truth stores the effective parameters, model identifier and program version.

- **Ancestors:** 360-nt, stop-free sequences with uniformly sampled sense codons. Two shared
  backbones per locus have independent family junctions. Families alternate kappa and lambda.
  The junction is bases 283–321, with cysteine at 283–285 and tryptophan (IGH) or phenylalanine
  (light) at 319–321. Random streams for backbones, families and samples are separate.
- **Tree:** seven known states per family. N0 is the ancestor, N1/N2 its children, N3/N4 children
  of N1, and N5/N6 children of N2. Each edge adds the requested substitutions to each chain.
  Propose a uniform site and base; reject unchanged bases, anchor changes and stop codons.
  Accepted events can revisit a site. N3's first event deliberately reverses N1's last event.
  Each event's silent/replacement label compares its codon immediately before and after that
  event. Net differences to the ancestor can be smaller than accumulated events, including zero.
  Different nodes can have identical sequences; they remain distinct known states.
- **Isotypes:** N0–N6 have IGHM, IGHM, IGHG1, IGHM, IGHG1, IGHG1, IGHA1. These fixed transitions
  follow a subset of human constant-region order. There is no inferred switch rate or timing
  within an edge, and no modeled IgM/IgD co-expression.
- **Sampling:** within-family node weights at days −7, 14 and 28 are respectively
  `[1,0,0,0,0,0,0]`, `[1,4,4,0,0,0,0]` and `[1,1,1,4,4,4,4]`, normalized separately. The first
  `ceil(families/4)` families have abundance multipliers 1, 8 and 3; the others remain at 1.
  Normalize across families and draw cells independently from this categorical distribution.
  Every sample records all node probabilities and realized counts, including zeros. Families
  expand and partly contract by construction; this is not a statistical call or evidence of
  affinity. No extra biological variance, dropout, sequencing error, reads or UMIs are modeled.

All choices above belong to ClonoLoom. Real SHM depends on sequence context (Yaari et al. 2013,
*Front Immunol* 4:358, [doi:10.3389/fimmu.2013.00358](https://doi.org/10.3389/fimmu.2013.00358));
that paper's S5F model is **not** implemented here. Biological class-switch context is described
by Horns et al. 2016, *eLife* 5:e16578,
[doi:10.7554/eLife.16578](https://doi.org/10.7554/eLife.16578); its inference method is not used.
The benchmark does not model recombination, indels, motif targeting, selection or alternative
topologies. Broader biological simulations and real-data validation remain necessary for later
analyses.

### Observations and truth

| File | Meaning |
| --- | --- |
| `S01-pre.tsv`, `S01-d14.tsv`, `S01-d28.tsv`, … | AIRR observations. Each `cell_id` has two chains, each with its own `sequence_id`. Count cells once. Read/UMI counts and inferred family labels are absent. |
| `repertoires.airr.json` | Sample, subject, time relative to simulated immunization, single-cell status and locus metadata. The study's model marker survives a renamed title/preset. |
| `reference/germlines.fasta` | Artificial ungapped V/J segments; explicit fictional allele names. SHA-256 is recorded in reconstructed results. |
| `truth/truth.json` | Parameters, model and version; all nodes with parent, family, subject, isotype, heavy/light sequences and ordered edge events; sample probabilities and cell counts; SHA-256 manifest for every other output. |
| `truth/observations.tsv` | Exact sample/cell-to-node assignment, including repeated draws of a state. |
| `README.md` | Model, limits, file meanings and how to reproduce the run. |

`sequence_alignment` is the un-gapped observed coding string. `germline_alignment` is the **known
rearranged ancestor**, including its artificial junction, not a reconstructed or D-masked
reference. `germline_database` names the artificial model. The supplied V/J query spans are 1–285
and 319–360. Artificial FWR1/CDR1/FWR2/CDR2/FWR3/CDR3/FWR4 spans are 1–78, 79–114, 115–165,
166–195, 196–285, 286–318 and 319–360 (1-based closed); they are **not IMGT numbering**.
D calls are missing; v2 supplies V/J CIGARs and reference-segment coordinates. These limits matter when using
the truth to test germline reconstruction and mutation comparisons.

A complete study is built in a temporary sibling folder before being promoted. Cancellation or
failure leaves no partial output, and a nonempty destination is refused. The same parameters,
seed and program version reproduce the same files on arm64 and amd64. To repeat a saved study,
extract its `study` object from `truth/truth.json` and pass that JSON to `--params`.

Independent validation (`validation/reference/bcell_families.py`, run with `uv`) replays all
1,296 accepted events, including 72 deliberate reversals, and checks every paired observation,
coordinate, isotype, probability, count and file hash. Go tests compare its reduced CSV counts
with the app's readers, input checks and paired clonotypes. Tests also cover different seeds,
round-tripped parameters, cancellation, bounds and overwrite refusal. Fixed-seed sampling is
checked against the stated categorical probabilities. The old TRB simulations' pinned bytes
remain unchanged.

The wave 5 SHM comparison uses the v2 saved D-masked germlines. Each default-preset chain row
has 327 evaluable bases and 109 evaluable codons, with all 33 intervening bases masked. Across
4,320 source rows there are 9,941 net substitutions: 7,297 replacement and 2,360 silent in
single-hit codons, plus 284 unclassified substitutions in 142 multi-hit codons. Endpoint
codon counts are 7,439 replacement and 2,360 silent. These pooled source-row counts differ from
mutation events on the generating tree: sampled descendants repeat changes, and reversals
remove net differences. The independent `validation/reference/shm.py` compares observed
sequences directly with known root sequences by the fixed artificial regions, including all
43,200 row/region entries. It validates bookkeeping, not biological SHM accuracy.

### Lineage validation (wave 5, slice 7)

The guided workflow builds 72 separate family/locus trees on 327 shared V/J sites. Independent
rooted-bipartition enumeration and four-state scoring agree with every Go minimum score and
optimal-topology count: 1,093 changes in total, with ties in 71 searches. The known full generating
tree supplies an upper bound, not a uniquely recoverable topology. Unobserved states, reversals
and the masked junction prevent that claim. Observed isotype, timepoint and tissue labels are
retained without inferring switching or migration. The simulator and its v2 truth are unchanged.
