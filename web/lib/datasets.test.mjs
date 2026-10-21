import assert from 'node:assert/strict';
import test from 'node:test';
import { createProject } from './project.js';
import { addDataset, annotate, annotation, confirmUnit, datasetRecord, datasetUnit, fieldsFromMetadata, relocate, removeDataset, suggestFromNames } from './datasets.js';

const NOW = '2026-10-21T12:00:00.000Z';
const summary = {
  id: 'd-0123456789abcdef', name: 'CAR-T cohort', unit: '', formats: ['immunoSEQ export (v3)'], singleCell: false, rows: 1200, readerVersion: 1,
  sources: [{ path: '/data/P1_day0.tsv', name: 'P1_day0.tsv', size: 100, sha256: 'a'.repeat(64), modified: '2026-10-01T00:00:00Z', format: 'immunoseq-v3', rows: 600 }],
  samples: [{ name: 'P1_day0' }, { name: 'P1_day28' }],
  report: { fatal: 0, warnings: 2, notes: 1, findings: [] },
};

test('a dataset is recorded by its files\' fingerprints and its unit confirmed apart', () => {
  let project = createProject('Study', { now: NOW });
  const record = datasetRecord(summary, NOW);
  project = addDataset(project, record, NOW);
  assert.equal(project.datasets.length, 1);
  assert.equal(project.history.at(-1).action, 'Opened a dataset');
  assert.match(project.history.at(-1).detail, /2 samples, 1,200 rows/);
  assert.equal(datasetUnit(project.datasets[0]), '');
  project = confirmUnit(project, record.id, 'reads', NOW);
  assert.equal(datasetUnit(project.datasets[0]), 'reads');
  // Opened again: kept once, with the user's name and unit.
  const again = addDataset({ ...project, datasets: [{ ...project.datasets[0], name: 'Renamed' }] }, record, NOW);
  assert.equal(again.datasets.length, 1);
  assert.equal(again.datasets[0].name, 'Renamed');
  assert.equal(again.datasets[0].unitConfirmed, 'reads');
  const moved = relocate(project, record.id, ['/elsewhere/P1_day0.tsv'], NOW);
  assert.equal(moved.datasets[0].sources[0].path, '/elsewhere/P1_day0.tsv');
  assert.equal(project.datasets[0].sources[0].path, '/data/P1_day0.tsv');
});

test('annotation keeps who gave each value, and an empty value is missing', () => {
  let project = createProject('Study', { now: NOW });
  project = addDataset(project, datasetRecord(summary, NOW), NOW);
  project = annotate(project, [
    { dataset: summary.id, sample: 'P1_day0', field: 'subject', value: 'P1', origin: 'file name' },
    { dataset: summary.id, sample: 'P1_day0', field: 'timepoint', value: 'day0', origin: 'file name' },
  ], 'Suggested from file names', NOW);
  let a = annotation(project, summary.id, 'P1_day0');
  assert.deepEqual(a.fields, { subject: 'P1', timepoint: 'day0' });
  assert.equal(a.origin.subject, 'file name');
  project = annotate(project, [{ dataset: summary.id, sample: 'P1_day0', field: 'subject', value: 'Patient 1' }], undefined, NOW);
  a = annotation(project, summary.id, 'P1_day0');
  assert.equal(a.origin.subject, 'manual');
  project = annotate(project, [{ dataset: summary.id, sample: 'P1_day0', field: 'timepoint', value: '  ' }], undefined, NOW);
  assert.deepEqual(annotation(project, summary.id, 'P1_day0').fields, { subject: 'Patient 1' });
  assert.equal(annotate(project, [{ dataset: summary.id, sample: 'P1_day0', field: 'subject', value: 'Patient 1' }], undefined, NOW), project);
  assert.throws(() => annotate(project, [{ dataset: summary.id, sample: 'x', field: 'nonsense', value: '1' }]), /Unknown sample field/);
  const removed = removeDataset(project, summary.id, NOW);
  assert.equal(removed.datasets.length, 0);
  assert.equal(removed.samples.length, 0);
});

test('fields are suggested from file names by what their parts look like', () => {
  const names = ['P01_day0_PBMC.tsv', 'P01_day28_PBMC.tsv', 'P02_day0_tumor.tsv', 'P02_day28_tumor.tsv.gz'];
  const fields = Object.fromEntries(suggestFromNames(names).map((f) => [f.field, f.values]));
  assert.deepEqual(fields.subject, ['P01', 'P01', 'P02', 'P02']);
  assert.deepEqual(fields.timepoint, ['day0', 'day28', 'day0', 'day28']);
  assert.deepEqual(fields.tissue, ['PBMC', 'PBMC', 'tumor', 'tumor']);
  const tenx = suggestFromNames(['Donor1_CD8_filtered_contig_annotations.csv', 'Donor2_CD4_filtered_contig_annotations.csv']);
  assert.deepEqual(Object.fromEntries(tenx.map((f) => [f.field, f.values])), { subject: ['Donor1', 'Donor2'], cellSubset: ['CD8', 'CD4'] });
  const groups = suggestFromNames(['HIP001_responder', 'HIP002_responder', 'HIP003_nonresponder']);
  assert.deepEqual(groups.map((f) => f.field), ['subject', 'condition']);
  assert.deepEqual(suggestFromNames(['only.tsv']), []);
});

test('metadata gives fields: AIRR repertoires and immunoSEQ sample tags', () => {
  assert.deepEqual(fieldsFromMetadata({ repertoire: { subjectId: 'HIP15860', tissue: 'venous blood', timePoint: '14 day (vaccination)', studyGroup: 'Cohort 01', hla: ['HLA-A*01', 'HLA-B*14', 'Inferred HLA-A*33'] } }),
    { subject: 'HIP15860', tissue: 'venous blood', timepoint: '14 day (vaccination)', condition: 'Cohort 01', hla: 'HLA-A*01, HLA-B*14' });
  const tags = 'Age:41 Years,Biological Sex:Male,HLA MHC class I:HLA-A*24,HLA MHC class I:HLA-B*07,Inferred HLA type:Inferred HLA-A*03,Virus Diseases:Cytomegalovirus -';
  assert.deepEqual(fieldsFromMetadata({ metadata: { sample_tags: tags } }), { hla: 'HLA-A*24, HLA-B*07', condition: 'CMV-' });
  assert.deepEqual(fieldsFromMetadata({}), {});
});
