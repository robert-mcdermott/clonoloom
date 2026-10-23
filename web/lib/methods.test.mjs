import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { methodsParagraph, joinList } from './methods.js';

const program = { version: '0.1.0', commit: '96a92f4c0ffee' };

const pbmc = {
  id: 'd-1', name: 'PBMC 1k T cells', formats: ['Cell Ranger V(D)J contig annotations'], unit: 'cells', unitConfirmed: null, singleCell: true, rows: 968,
  samples: ['sc5p_v1p1_hs_PBMC_1k_t'], readerVersion: 1, opened: '2026-10-21T21:30:00.000Z',
  findings: { fatal: 0, warnings: 0, notes: 1 },
  sources: [{ path: '/x/a.csv', name: 'a.csv', size: 188126, sha256: 'ab'.repeat(32), format: 'cellranger-contigs' }],
  credits: [{ example: '10x-pbmc-1k', title: 'Human PBMC from a healthy donor, 1k cells', source: '10x Genomics, Human PBMC from a Healthy Donor, 1k cells (v1.1)', license: 'CC BY 4.0 (10x Genomics)', licenseClass: 'open', citation: '10x Genomics. Single Cell Immune Profiling datasets.' }],
};

test('a single-cell example: its source, reading, unit, validation and clonotypes, with numbered references', () => {
  const { text, references } = methodsParagraph({
    dataset: pbmc,
    definition: { name: 'Paired αβ: amino-acid junctions, V and J genes' },
    clonotypes: { id: 'def-0123456789ab', describe: 'paired TRA and TRB chains by amino-acid junction, V gene and J gene', count: 406, cells: 500 },
    program,
  });
  assert.match(text, /^The data are Human PBMC from a healthy donor, 1k cells, published under CC BY 4\.0 \(10x Genomics\) \[1\], downloaded and first read on 2026-10-21\./);
  assert.match(text, /The file \(10x Genomics Cell Ranger contig annotations\) was read with ClonoLoom 0\.1\.0 \(commit 96a92f4\) \[2\] by its own readers \(reader version 1\), which recorded its SHA-256 checksum as it was read\./);
  assert.match(text, /1 sample and 968 contigs of single cells, counted in cells\./);
  assert.match(text, /IMGT nomenclature \[3\], ambiguous calls kept as sets\./);
  assert.match(text, /found 0 fatal problems, 0 warnings and 1 note\./);
  assert.match(text, /\("Paired αβ: amino-acid junctions, V and J genes", ClonoLoom definition def-0123456789ab\)\. A junction is the IMGT junction, .* \[4\]\. The 500 cells formed 406 clonotypes\.$/);
  assert.equal(references.length, 4);
  assert.match(references[0], /^\[1\] 10x Genomics\./);
  assert.match(references[1], /^\[2\] McDermott R\. ClonoLoom, version 0\.1\.0\./);
  assert.match(references[3], /^\[4\] Lefranc MP, Pommié C/);
  // Nothing is said of a step not taken: no filter, no AIRR schema, no Adaptive names.
  assert.doesNotMatch(text, /filtered|AIRR Community|HGNC/);
});

test('bulk AIRR data: the schema and Repertoire metadata cited, a filter with its counts, warnings named, an unknown unit said', () => {
  const airrFile = { ...pbmc.sources[0], format: 'airr' };
  const dataset = { ...pbmc, credits: [], formats: ['AIRR Rearrangement TSV'], singleCell: false, unit: '', rows: 120, samples: ['R1', 'R2'], findings: { fatal: 0, warnings: 1, notes: 0 }, sources: [airrFile, airrFile] };
  const summary = {
    samples: [{ name: 'R1', repertoire: { repertoire_id: 'R1' } }, { name: 'R2' }],
    sources: [{ geneNames: {} }],
    report: { findings: [{ severity: 'warning', title: 'Junctions out of frame while productive', count: 3 }, { severity: 'note', title: 'A note', count: 1 }] },
  };
  const filter = { name: 'Productive', version: 2, rules: { productiveOnly: true, minCount: 2 }, reason: 'Non-productive rearrangements are not expressed.' };
  const { text, references } = methodsParagraph({ dataset, summary, filter, filterResult: { rows: 120, kept: 90 }, program: { version: '0.1.0', commit: '' } });
  assert.match(text, /^The 2 files \(AIRR Rearrangement TSV\) were read with ClonoLoom 0\.1\.0 \[1\] by its own readers \(reader version 1\), which recorded each file's SHA-256 checksum as each was read\. AIRR files were read as the AIRR Community's Rearrangement schema defines them \[2\]\./);
  assert.match(text, /AIRR Repertoire metadata \[3\] were matched to 1 sample of 2 by repertoire_id\./);
  assert.match(text, /2 samples and 120 rearrangements, whose counts the files do not say are of reads or of molecules\./);
  assert.match(text, /1 warning and 0 notes: junctions out of frame while productive \(3 rows\)\./);
  assert.match(text, /Rearrangements were filtered with "Productive" \(version 2: productive rearrangements only; counts of at least 2; made because "Non-productive rearrangements are not expressed"\), which kept 90 of 120 rearrangements \(75\.0%\)\./);
  assert.equal(references.length, 4);
  assert.doesNotMatch(text, /Clonotypes were defined/);
});

test('mapped Adaptive names cite HGNC; a unit the user confirmed is said so; data under terms of use are "used under" them', () => {
  const dataset = { ...pbmc, formats: ['immunoSEQ export (v3)'], sources: [{ ...pbmc.sources[0], format: 'immunoseq-v3' }], singleCell: false, unit: '', unitConfirmed: 'reads', credits: [{ ...pbmc.credits[0], licenseClass: 'terms', license: 'immuneACCESS Terms of Use', example: 'e' }] };
  const { text } = methodsParagraph({ dataset, summary: { sources: [{ geneNames: { 'TCRBV05-01*01': 'TRBV5-1*01', 'TCRBV09-01': 'TRBV9' } }] }, program });
  assert.match(text, /used under immuneACCESS Terms of Use \[1\]/);
  assert.match(text, /\(Adaptive Biotechnologies immunoSEQ exports \(v3 columns\)\) was read/);
  assert.match(text, /counted in reads \(the unit confirmed by the user, which the files did not state\)/);
  assert.match(text, /2 Adaptive Biotechnologies gene names were mapped to IMGT's by ClonoLoom's rules and the HGNC list of human TR and IG genes \[\d\]/);
});

test('the readers of MiXCR\'s tables, OAS and OTS units and tcrdist3\'s tables say how they read', () => {
  const sources = [{ ...pbmc.sources[0], format: 'mixcr-clones' }, { ...pbmc.sources[0], format: 'oas-unit' }, { ...pbmc.sources[0], format: 'tcrdist3-clones' }];
  const { text, references } = methodsParagraph({ dataset: { ...pbmc, credits: [], sources }, program });
  assert.match(text, /\(MiXCR clone tables, Observed Antibody Space or Observed T cell receptor Space data units and tcrdist3 clone tables\) were read/);
  assert.match(text, /MiXCR clone tables were read with each segment's best-scoring hits as its call, MiXCR's allele \*00 \(no allele resolved\) dropped/);
  assert.match(text, /Data units of the Observed Antibody Space \[2\] and the Observed T cell receptor Space \[3\] were read with their metadata as their samples'; a paired unit's rows as single cells/);
  assert.match(text, /tcrdist3's clone tables \[4\] were read with each clone of n cells as n cells with its chains/);
  assert.match(references[1], /Olsen TH/);
  assert.match(references[3], /Mayer-Blackwell K/);
  // One format alone says only its own; a unit of T-cell receptors cites OTS alone.
  const alone = methodsParagraph({ dataset: { ...pbmc, credits: [], sources: [sources[0]] }, program }).text;
  assert.doesNotMatch(alone, /Observed|tcrdist3/);
  const ots = methodsParagraph({ dataset: { ...pbmc, credits: [], sources: [sources[1]] }, summary: { samples: [{ name: 'u', loci: { TRA: 2, TRB: 2 } }], sources: [{}] }, program }).text;
  assert.match(ots, /Data units of the Observed T cell receptor Space \[2\] were read/);
});

test('a TCRdist result says how the clonotypes were compared, with the loops\' database named by its checksum', () => {
  const result = { kind: 'tcrdist', definition: 'Paired αβ', filter: null, radius: 48, cdr3Only: false, organism: 'mouse', loops: { file: 'combo_xcr_2024-03-05.tsv', sha256: '1d7fcac3' }, compared: 1746, withNeighbors: 936 };
  const { text, references } = methodsParagraph({ dataset: { ...pbmc, credits: [] }, results: [result], program });
  assert.match(text, /Each T-cell clonotype \("Paired αβ"\) was compared with every other of the same chains by TCRdist \[\d\], as tcrdist3 computes it \[\d\]/);
  assert.match(text, /the loops of each mouse V allele taken from tcrdist3's database of genes \(combo_xcr_2024-03-05\.tsv, SHA-256 1d7fcac3\)/);
  assert.match(text, /Of 1,746 clonotypes compared, 936 had another within 48 TCRdist units\./);
  assert.ok(references.some((r) => /Dash P/.test(r)) && references.some((r) => /Mayer-Blackwell/.test(r)));
  const alone = methodsParagraph({ dataset: { ...pbmc, credits: [] }, results: [{ ...result, cdr3Only: true, loops: null }], program }).text;
  assert.match(alone, /each chain by their CDR3s alone/);
  assert.doesNotMatch(alone, /tcrdist3's database/);
});

test('a generation probabilities result names its models by their checksum', () => {
  const result = { kind: 'pgen', definition: 'AA', filter: null, models: 'olga', set: { name: 'OLGA\'s default models', checksum: 'ab12cd34' }, organism: 'human', genes: true, loci: { TRB: 'human_T_beta', TRA: 'human_T_alpha' }, clonotypes: 1200, computed: 1180 };
  const { text, references } = methodsParagraph({ dataset: { ...pbmc, credits: [] }, results: [result], program });
  assert.match(text, /Each clonotype's generation probability \("AA"\), the probability that V\(D\)J recombination alone makes its CDR3s with their V and J genes/);
  assert.match(text, /under OLGA's default models \[\d\] \(human_T_beta for TRB, human_T_alpha for TRA\), inferred by IGoR \[\d\] \(checksum ab12cd34\), human/);
  assert.match(text, /Of 1,200 clonotypes, 1,180 had one\./);
  assert.ok(references.some((r) => /Sethna Z/.test(r)) && references.some((r) => /Marcou Q/.test(r)));
  const own = methodsParagraph({ dataset: { ...pbmc, credits: [] }, results: [{ ...result, models: 'clonoloom', genes: false, set: { checksum: 'clonoloom_human_T_beta 1' }, loci: { TRB: 'clonoloom_human_T_beta' } }], program });
  assert.match(own.text, /under ClonoLoom's own model of human TRB recombination \(its simulator's: GRCh38's gene segments, version 1\)/);
  assert.ok(!own.references.some((r) => /Marcou Q/.test(r)));
});

test('a neighborhoods result names its background and how each radius was chosen', () => {
  const result = { kind: 'neighborhoods', definition: 'AA', filter: null, locus: 'TRB', cdr3Only: false, organism: 'human',
    loops: { file: 'combo_xcr_2024-03-05.tsv', sha256: '1d7fcac3' }, control: 1e-6, maxRadius: 48, minNeighbors: 2,
    background: { kind: 'model', name: 'human_T_beta', checksum: 'ab12cd34', receptors: 100000 },
    receptors: 7658, centers: 956 };
  const { text, references } = methodsParagraph({ dataset: { ...pbmc, credits: [] }, results: [result], program });
  assert.match(text, /Each TRB receptor \("AA"\) was taken as the centre of a neighborhood by ClonoLoom \[\d\] as Mayer-Blackwell et al\. define one \[\d\]/);
  assert.match(text, /at most a share 0\.000001 of the background's receptors, against a background of 100,000 receptors drawn from human_T_beta \(checksum ab12cd34\)/);
  assert.match(text, /Of 7,658 receptors, 956 had a neighborhood of 2 receptors or more/);
  assert.ok(references.some((r) => /Mayer-Blackwell/.test(r)) && references.some((r) => /Dash P/.test(r)));
  const shuffled = methodsParagraph({ dataset: { ...pbmc, credits: [] }, results: [{ ...result, cdr3Only: true, loops: null, background: { kind: 'shuffled', receptors: 7658 } }], program }).text;
  assert.match(shuffled, /the same receptors with their CDR3s moved among their V and J genes \(7,658 receptors\)/);
  assert.match(shuffled, /comparing their CDR3s alone/);
});

test('the same inputs give the same text', () => {
  const inputs = { dataset: pbmc, program };
  assert.deepEqual(methodsParagraph(inputs), methodsParagraph(inputs));
  assert.equal(joinList(['a', 'b', 'c']), 'a, b and c');
});

test('simulated data say how they were made, citing the sources of the model', () => {
  const dataset = { ...pbmc, credits: [], formats: ['AIRR Rearrangement TSV'], sources: [{ ...pbmc.sources[0], format: 'airr' }], singleCell: false, unit: 'umis' };
  const summary = { samples: [{ name: 'S01-1', repertoire: { repertoireId: 'S01-1', studyId: 'clonoloom-simulated-standard', studyTitle: 'A simulated TRB study' } }], sources: [{}] };
  const { text, references } = methodsParagraph({ dataset, summary, program });
  assert.match(text, /^The data are simulated \(A simulated TRB study\): T-cell receptor β rearrangements made by ClonoLoom's simulator \[1\] from GRCh38's gene segments at Ensembl's annotation \[2\]/);
  assert.match(text, /selection on junction length to that data's \[3\]/);
  assert.match(references[1], /Ensembl 2025/);
  assert.match(references[2], /Quantifying selection/);
});

test('a diversity result kept says how each measure was made, citing each method once', () => {
  const results = [{ kind: 'diversity', unit: 'templates', definition: 'Amino-acid junction, V and J genes', filter: null, options: { replicates: 200, seed: 20140101 } }];
  const { text, references } = methodsParagraph({ dataset: pbmc, results, program });
  assert.match(text, /Each sample's diversity was measured over its clonotypes' abundances in templates \("Amino-acid junction, V and J genes"\) by ClonoLoom \[2\]/);
  assert.match(text, /200 bootstrap replicates of each sample drawn from its estimated repertoire \[\d+\] \(seed 20140101\)/);
  const cited = references.map((r) => r.replace(/^\[\d+\] /, ''));
  assert.equal(new Set(cited).size, cited.length);
  for (const work of ['Hill MO', 'Chao A. Nonparametric', 'Chao A, Lee SM', 'Chao A, Wang YT', 'Chao A, Gotelli NJ', 'Chao A, Jost L']) {
    assert.ok(cited.some((r) => r.startsWith(work)), work);
  }
});

test('a rarefaction result kept says how the samples were compared, citing each method once', () => {
  const results = [
    { kind: 'diversity', unit: 'templates', definition: 'Amino-acid junction, V and J genes', filter: null, options: { replicates: 200, seed: 20140101 } },
    { kind: 'rarefaction', unit: 'templates', definition: 'Amino-acid junction, V and J genes', filter: 'Productive v2', options: { replicates: 200, seed: 20140101 }, levels: { coverage: 0.87402, size: 9438, extrapolatedCoverage: 0.93, extrapolatedSize: 18876 } },
  ];
  const { text, references } = methodsParagraph({ dataset: pbmc, results, program });
  assert.match(text, /were rarefied and extrapolated, over its clonotypes' abundances in templates \("Amino-acid junction, V and J genes", filter Productive v2\)/);
  assert.match(text, /compared at a sample coverage of 0\.874, the lowest among them, and at 9,438 templates, the smallest sample's size/);
  // Chao et al. 2014, Chao and Jost 2012, Hill and ClonoLoom each cited once.
  for (const key of ['Rarefaction and extrapolation with Hill numbers', 'Coverage-based rarefaction', 'Diversity and evenness', 'ClonoLoom, version']) {
    assert.equal(references.filter((r) => r.includes(key)).length, 1, key);
  }
});

test('a gene usage result kept says what was counted and how, citing IMGT once', () => {
  const results = [{ kind: 'usage', unit: 'umis', definition: 'Amino-acid junction, V and J genes', filter: null, options: { replicates: 200, seed: 20140101 }, groups: 2 }];
  const { text, references } = methodsParagraph({ dataset: pbmc, results, program });
  assert.match(text, /What each sample's clonotypes \("Amino-acid junction, V and J genes"\) are made of was counted by ClonoLoom/);
  assert.match(text, /of its abundance in UMIs, with a 95% interval from 200 bootstrap replicates of the sample's clonotypes \(seed 20140101\)/);
  assert.match(text, /Each condition was summarized by the mean of its samples' shares/);
  assert.equal(references.filter((r) => r.includes('IMGT unique numbering')).length, 1);
});

test('an overlap result kept says what was measured, at what depth, citing each index once', () => {
  const results = [{ kind: 'overlap', unit: 'templates', definition: 'Amino-acid junction, V and J genes', filter: null, options: { replicates: 200, seed: 20140101 }, depth: 9438, publicBy: 'subject' }];
  const { text, references } = methodsParagraph({ dataset: pbmc, results, program });
  assert.match(text, /both samples rarefied to 9,438 templates, the smallest sample's size/);
  assert.match(text, /Clonotypes found in more than one of the subjects were counted as public/);
  for (const key of ['Jaccard P.', 'Dice LR.', 'Morisita M.', 'Horn HS.', 'Horvitz DG']) {
    assert.equal(references.filter((r) => r.includes(key)).length, 1, key);
  }
});

test('a tracking result kept says what was followed and how, citing Wilson once; in reads, without intervals', () => {
  const results = [
    { kind: 'tracking', unit: 'templates', definition: 'Amino-acid junction, V and J genes', filter: null, options: { confidence: 0.95 }, choice: { by: 'reference', n: 10, list: [] }, subjects: 3, followed: 30, intervals: true },
    { kind: 'tracking', unit: 'templates', definition: 'Amino-acid junction, V and J genes', filter: null, options: { confidence: 0.95 }, choice: { by: 'list', list: ['c-1', 'CASSA'] }, subjects: 3, followed: 4, intervals: true },
  ];
  const { text, references } = methodsParagraph({ dataset: pbmc, results, program });
  assert.match(text, /Clonotypes \("Amino-acid junction, V and J genes"\) were followed through each subject's samples in time order by ClonoLoom \[\d\]: each reference sample's 10 largest clonotypes \(each sample's 10 largest for a subject without one\), 30 clonotypes in 3 subjects; each one's frequency at each time point is its count of the templates there, the samples taken at one time pooled, with Wilson's 95% score interval \[\d\]/);
  assert.match(text, /: 2 clonotypes listed by ID or junction, 4 clonotypes in 3 subjects;/);
  assert.equal(references.filter((r) => r.includes('Wilson EB.')).length, 1);
  const reads = methodsParagraph({ dataset: pbmc, results: [{ ...results[0], unit: 'reads', intervals: false }], program });
  assert.match(reads.text, /its share of the reads there, the samples taken at one time pooled, without an interval \(reads count each molecule many times over\)\./);
  assert.ok(!reads.references.some((r) => r.includes('Wilson EB.')));
});

test('a test result kept says what was tested between which time points, and how false discoveries were controlled', () => {
  const binomial = { kind: 'expansion', unit: 'templates', definition: 'Amino-acid junction, V and J genes', filter: null, comparison: { from: 'day -7', to: 'day 14', test: 'binomial', alternative: 'two-sided', correction: 'bh', alpha: 0.01, minTotal: 5 }, subjects: 3, tested: 9000, increased: 40, decreased: 12 };
  const { text, references } = methodsParagraph({ dataset: pbmc, results: [binomial], program });
  assert.match(text, /Each clonotype \("Amino-acid junction, V and J genes"\) was tested between day -7 and day 14 in each subject by ClonoLoom \[\d\], the samples at one time pooled: the binomial test of its count at the first time point given its total in the two, at the first's share of their templates \[\d\]/);
  assert.match(text, /Clonotypes with fewer than 5 templates in the two together were not tested \[\d\]\. The false discovery rate was controlled within each subject by Benjamini and Hochberg's procedure \[\d\], and clonotypes with an adjusted p-value of at most 0\.01 called increased or decreased: 40 clonotypes increased and 12 clonotypes decreased of 9,000 clonotypes tested in 3 subjects\. The test accounts for sampling alone/);
  for (const key of ['Rytlewski J', 'DeWitt WS', 'Benjamini Y']) assert.equal(references.filter((r) => r.includes(key)).length, 1, key);
  const fisher = methodsParagraph({ dataset: pbmc, results: [{ ...binomial, comparison: { ...binomial.comparison, test: 'fisher', correction: 'storey', alternative: 'increase' } }], program });
  assert.match(fisher.text, /Fisher's exact test of its count against the others' at the two time points \[\d\], one-sided, for an increase/);
  assert.match(fisher.text, /Storey's q-values, π₀ estimated at λ = 0\.5 \[\d\]/);
  assert.ok(fisher.references.some((r) => r.includes('Storey JD')));
  assert.match(text, /The test accounts for sampling alone, not for the variation between samples taken at one time\./);
  // The beta-binomial test, its variance fitted on replicates.
  const bb = methodsParagraph({ dataset: pbmc, results: [{ ...binomial, comparison: { ...binomial.comparison, test: 'auto' }, used: 'betabinomial', fit: { on: 'replicates', pairs: 3, clonotypes: 2345, a: -4.512, b: 0.04103, calledBySampling: 0.082, calledByModel: 0.027 } }], program });
  assert.match(bb.text, /the beta-binomial test of its count at the first time point given its total in the two, at the first's share of their templates, its scale v\(n\) = 2 \+ \(b ln n \+ a\)² of Rytlewski et al\. \[\d\] \(the 2, which keeps the beta unimodal, as in their reference code\) fitted by maximum likelihood on the clonotypes of 3 pairs of replicate samples, taken at one time \(2,345 clonotypes with at least 5 in the two; a = -4\.512, b = 0\.04103\), two-sided/);
  assert.match(bb.text, /a two-sided test at 0\.05 called 8\.2% of the clonotypes changed by sampling alone, and 2\.7% with the fitted variance/);
});

test('classes and attribution kept say how they were made', () => {
  const results = [
    { kind: 'classes', unit: 'templates', definition: 'Amino-acid junction, V and J genes', filter: null, classing: { base: 'day -7', test: 'auto', correction: 'bh', alpha: 0.01, minTotal: 5 }, used: 'betabinomial', fit: { on: 'replicates', pairs: 3, clonotypes: 2142, a: 3.631, b: 0.2349 }, subjects: 3, counts: { emerging: 0, transient: 20, contracting: 0, persistent: 1800 } },
    { kind: 'attribution', unit: 'templates', definition: 'Amino-acid junction, V and J genes', filter: null, options: { confidence: 0.95 }, subjects: 3, derived: 3, references: ['donor blood'], intervals: true },
  ];
  const { text, references } = methodsParagraph({ dataset: pbmc, results, program });
  assert.match(text, /tested between day -7 and every later time point of each subject by ClonoLoom \[\d\], by the beta-binomial test \[\d\], its variance fitted on the replicate samples \(a = 3\.631, b = 0\.2349\), two-sided/);
  assert.match(text, /In 3 subjects, 0 clonotypes were emerging, 20 transient, 0 contracting and 1,800 persistent\./);
  assert.match(text, /Each sample's share of templates in the clonotypes \("Amino-acid junction, V and J genes"\) its subject's reference sample \(donor blood\) holds was measured by ClonoLoom \[\d\], with Wilson's 95% score interval for sampling \[\d\]/);
  assert.match(text, /For the 3 subjects whose reference shared few clonotypes with the samples before the reference event/);
  assert.equal(references.filter((r) => r.includes('Wilson EB.')).length, 1);
});

test('a table read through a mapping says which of its columns gave what (the 0.2.0 project)', () => {
  const project = JSON.parse(readFileSync(new URL('../../validation/projects/0.2.0/project.json', import.meta.url), 'utf8'));
  const dataset = project.datasets.find((d) => d.sources.some((s) => s.mapping));
  const { text } = methodsParagraph({ dataset, results: project.artifacts.filter((a) => a.dataset === dataset.id), program });
  assert.match(text, /The file \(a table with mapped columns\) was read/);
  assert.match(text, /S02-1\.vdjtools\.txt's columns were mapped by the user: the junction's amino acids from "cdr3aa", the junction's nucleotides from "cdr3nt", the V gene from "v", the D gene from "d", the J gene from "j" and the count from "count", the count being of UMIs, every row of sample S02-1; a rearrangement was taken as productive when its junction is whole codons with no stop codon\./);
  assert.match(text, /Each sample's diversity was measured over its clonotypes' abundances in UMIs/);
});

test('a simulated series says it was sampled over time, and how its reference was mixed in', () => {
  const dataset = { ...pbmc, credits: [], formats: ['AIRR Rearrangement TSV'], sources: [{ ...pbmc.sources[0], format: 'airr' }], singleCell: false, unit: 'umis' };
  const summary = { samples: [
    { name: 'S01-d7', repertoire: { repertoireId: 'S01-d7', studyId: 'clonoloom-simulated-car-t', studyTitle: 'A simulated CAR-T study', timePoint: '7 day (infusion)', tissue: 'blood' } },
    { name: 'S01-product', repertoire: { repertoireId: 'S01-product', studyId: 'clonoloom-simulated-car-t', studyTitle: 'A simulated CAR-T study', tissue: 'infusion product' } },
  ], sources: [{}] };
  const { text, references } = methodsParagraph({ dataset, summary, program });
  assert.match(text, /Each subject was sampled at several time points, in days from the infusion, its clones' frequencies following known trajectories and varying between samples beyond sampling, as between blood samples \[4\]\. An infusion product, made from each subject's own T cells and expanded ex vivo, was mixed into its blood in known shares after the infusion, and sampled as its reference sample\./);
  assert.match(references[3], /^\[4\] Rytlewski J/);
  // A study at one time says nothing of it.
  const standard = { samples: [{ name: 'S01-1', repertoire: { repertoireId: 'S01-1', studyId: 'clonoloom-simulated-standard', studyTitle: 'A simulated TRB study' } }], sources: [{}] };
  assert.doesNotMatch(methodsParagraph({ dataset, summary: standard, program }).text, /time points/);
});

test('network methods carry chain rules, thresholds and source citations', () => {
  const { text, references } = methodsParagraph({ dataset: pbmc, program,
    results: [{ kind: 'networks', describe: 'paired amino-acid junctions', nodes: 10, edges: 6, components: 4,
      networks: { metric: 'levenshtein', cutoff: 2, arms: 'all', sameV: true } }],
  });
  assert.match(text, /Levenshtein distance/);
  assert.match(text, /cutoff 2/);
  assert.match(text, /bijection of all retained chains at each locus/);
  assert.match(text, /6 edges formed 4 connected components/);
  assert.match(text, /not proven antigen specificity/);
  assert.ok(references.some((r) => r.includes('Levenshtein VI')));
  assert.ok(references.some((r) => r.includes('Fruchterman')));
});
