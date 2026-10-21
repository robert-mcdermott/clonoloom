import { test } from 'node:test';
import assert from 'node:assert/strict';
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
