# Selected-panel lineage likelihood

Unreleased wave 9, slice 14. In **B cells → Lineages**, select a saved family/locus tree and
choose **Review JC69 likelihood**, then **Fit JC69 likelihood**. The dataset must be open so
its current provider terms can be reviewed. Fitting is explicit and cancellable.

This is a bounded JC69 baseline for inspecting the same observed/reference panel under a
likelihood objective. It is not a context-dependent somatic-hypermutation model. The saved
parsimony topology, scores and ancestral states do not constrain the fit. Existing parsimony
results, exports and scientific replay remain unchanged.

## Input and model

One checksum-verified saved lineage panel supplies 1–1024 shared V/J positions and at most
seven distinct states including the supplied reference. Larger panels are refused, not sampled.
The upstream complete-case panel retains only reference positions with canonical A/C/G/T
observations in every eligible row; D/N/P, gaps, ambiguity and uncovered positions are omitted.
Constant sites remain in the likelihood. Missingness can change which positions are retained;
this conditional panel is not an unbiased full-receptor alignment. Heavy/light loci remain separate.
Identical panel sequences collapse; source row counts and abundance do not weight the fit.

[Jukes & Cantor (1969)](https://doi.org/10.1016/B978-1-4832-3211-9.50009-7) defines the equal-frequency,
equal-rate nucleotide baseline. With branch length `t` in expected substitutions/site,
`P(same)=1/4+3/4 exp(-4t/3)` and `P(each other base)=1/4-1/4 exp(-4t/3)`.
Sites are independent with a common rate. [Felsenstein (1981)](https://doi.org/10.1007/BF01734359)
provides the pruning formulation for summing unknown ancestral states. ClonoLoom implements
these equations independently using its deterministic arithmetic; no external phylogeny
implementation is bundled or used as implementation source.

The reported **joint natural-log likelihood** includes every tip, including the reference,
with a uniform root-state prior. It is not divided by the reference's marginal probability.
Conditioning on that fixed reference sequence instead would add `sites × ln(4)` to every score,
without changing topology/length ranking. Compare candidate trees only for the same fixed
panel and model; raw log likelihoods do not rank different families or alignments. JC69 is reversible: the reference tip merely orients
the display and Newick; the model cannot identify a biological root or arrow of time.

## Search and numerical limits

These are ClonoLoom's bounded implementation choices, not defaults prescribed by those papers:

- Enumerate every unrooted binary topology: 945 at seven tips. Collapsed zero-length branches
  can make different binary resolutions equivalent. No bootstrap or clade support is computed.
- Fit every topology from uniform branch starts 0.001, 0.05 and 0.5, with each edge bounded
  to `[0,3]` substitutions/site. All starts and their diagnostics are retained.
- Holding other edges fixed gives site likelihood `a+b*x`, where `x=exp(-4t/3)`.
  The log likelihood is concave in that coordinate. Check both endpoints and otherwise bisect
  the derivative for 48 steps. Sweep edges in deterministic order, for at most 100 sweeps,
  stopping at a likelihood increment below `1e-9`.
- After fitting, independently revisit each coordinate. The largest possible single-edge gain
  must be at most `1e-7` for the highest-scoring start of **every** topology; otherwise the whole
  review is refused. Stationarity is not proof of a global continuous optimum.
- Select the lexical topology key among scores within `1e-8` of the overall maximum
  (not a chain of pairwise approximate ties). Report the number within
  `1e-6` of the selected score. These numerical near ties are not confidence probabilities.
- Refuse searches above two billion charged pattern/node/derivative work units. This is a
  computational budget, not elapsed time or a biological criterion. Cancellation or failure
  returns no usable partial tree. The seven-tip limit does not guarantee every panel fits the budget.

Each edge is marked root, interior, zero or upper boundary. An upper-bound fit indicates that
this bounded model has not established a finite unconstrained optimum. A zero edge provides no
positive length separating its endpoints. Neither establishes biological ancestry.

## Reading the viewer

The drawing uses **topological depth**, with edge labels in fitted substitutions/site. Horizontal
spacing is not genetic distance or elapsed time. The table exposes full native branch values;
the drawing rounds labels for readability. Reference/observed nodes show supplied isotype,
timepoint, tissue or sample annotations. Inferred nodes receive no observed traits. Observations
matching the reference panel state may be shown at the reference node; that does not date them.

Select a node with a pointer or keyboard, then a one-based panel site. The inspector shows its
original V/J reference coordinate and marginal A/C/G/T probabilities. They condition on **one
selected topology, its fitted lengths, the fixed reference and retained alignment panel**.
They do not integrate topology, parameter, germline, alignment or family-membership uncertainty.
The evidence includes marginal modal symbols (IUPAC ties within `1e-10`); combining them is not
necessarily a joint maximum-probability ancestral assignment. No ancestral FASTA is offered.

No SHM motif context, transition bias, site-rate variation, sequencing error, selection, affinity,
switching, migration or dates are inferred. See the [IgPhyML methodological context](https://igphyml.readthedocs.io/en/stable/)
for why ordinary independent-site substitution assumptions are limited for B-cell evolution.
Numerical correctness under JC69 does not establish that JC69 fits a biological repertoire.

## Evidence and permissions

**Save exact likelihood evidence** downloads the cached native JSON bytes, including the original
lineage parent text and SHA-256, manifest, selected panel, all topology/start diagnostics, settings,
Newick, branch values, conditional probabilities, methods and provider notice. The export rechecks
the parent and current notice. Restricted sources require explicit provider-term acknowledgment;
figure export also requires that acknowledgment. This is not de-identification: original identifiers
and source annotations are included. A checksum verifies bytes, not biological validity or permission.

The review store holds up to four records of at most 32 MiB each for 30 minutes; the parent is
bounded at 16 MiB and at most two fits run concurrently. Save before closing or later reviews
may evict the record. A project change invalidates the browser review. Downloaded evidence is
separate from project artifacts, certificates, exact replay and the automation action catalog.
There is no new remote/MCP/client action or automatic external disclosure.

## Validation and example

Use the existing **simulated B-cell immunization** study: build families, germlines and lineages,
then review a seven-state family/locus panel. The documentation screenshots use its complete
327-position IGH panel, preserving observed mixed isotypes and unresolved zero branches. These
are declared artificial controls, not observed immune histories or a promise of topology recovery.

Native tests check analytical one-/two-state cases (including saturation and upper bounds),
small-tree topology counts, missing/invalid panels, cancellation, budget and stationarity refusal,
parent integrity and provider permissions. Fixed result digests check ARM/Intel consistency.
The independent `validation/reference/lineage_likelihood.py` uses NumPy/SciPy to:

- verify transition matrices against a matrix exponential;
- generate the topology set independently by bipartitions;
- sum every internal ancestral assignment directly, without native pruning code;
- compare likelihood optimization using L-BFGS-B from three different starts, with a `1e-10`
  lower bound to avoid singular trial likelihoods; agreement tolerance is `2e-6` log units;
- check selected native lengths against direct likelihood (`1e-8`) and every node/site marginal
  probability (`2e-12`), including exact zero branches.

Optimizer comparisons cover all 3/15 topologies in the four-/five-tip controls. At seven tips,
the complete 945-key set is checked, while independent optimization covers the selected topology, its highest-scoring competitor
and first canonical topology (at most three), not every continuous fit. Actual native API and
both-theme browser downloads are checked. Numerical agreement and UI controls validate this
bounded implementation, not model adequacy, larger searches or a globally optimal branch fit.
