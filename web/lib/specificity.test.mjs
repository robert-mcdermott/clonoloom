import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createProject } from './project.js';
import { addDataset, datasetRecord } from './datasets.js';
import { nearestHistogram, optionWords, radiusOf, regionWords, tcrdistOptions, tcrdistRequest, useTCRdist } from './specificity.js';

const NOW = '2026-10-22T12:00:00.000Z';
const summary = { id: 'd-1', name: 'Dash', unit: 'cells', formats: [], singleCell: true, rows: 10, readerVersion: 1, sources: [], samples: [{ name: 's' }], report: { fatal: 0, warnings: 0, notes: 0, findings: [] } };

test('TCRdist\'s options are kept with the dataset, with their history', () => {
  let project = addDataset(createProject('P', { now: NOW }), datasetRecord(summary, NOW), NOW);
  assert.deepEqual(tcrdistOptions(project.datasets[0]), { radius: 0, cdr3Only: false, organism: '' });
  project = useTCRdist(project, 'd-1', { radius: 36, organism: 'mouse' }, NOW);
  assert.deepEqual(project.datasets[0].tcrdist, { radius: 36, organism: 'mouse' });
  assert.match(project.history.at(-1).detail, /Dash: neighbors within 36 TCRdist units, comparing the CDR3s and the V genes' loops \(mouse\)/);
  // The same options: the project unchanged.
  assert.equal(useTCRdist(project, 'd-1', { radius: 36 }, NOW), project);
  project = useTCRdist(project, 'd-1', { cdr3Only: true, organism: 'cat' }, NOW);
  assert.deepEqual(tcrdistOptions(project.datasets[0]), { radius: 36, cdr3Only: true, organism: '' });
  assert.deepEqual(tcrdistRequest(tcrdistOptions(project.datasets[0])), { radius: 36, cdr3Only: true, organism: '' });
});

test('the default radius is 24 a chain', () => {
  assert.equal(radiusOf({ radius: 0 }, { chains: ['TRA', 'TRB'] }), 48);
  assert.equal(radiusOf({ radius: 0 }, { chains: [] }), 24);
  assert.equal(radiusOf({ radius: 30 }, { chains: ['TRA', 'TRB'] }), 30);
  assert.equal(optionWords({ radius: 0, cdr3Only: true }, { chains: ['TRB'] }), 'neighbors within 24 TCRdist units, comparing the CDR3s alone');
});

test('the distances to the nearest, in bins', () => {
  const rows = [{ nearest: 'a', distance: 0 }, { nearest: 'b', distance: 5 }, { nearest: 'c', distance: 13 }, { nearest: '', distance: -1 }];
  assert.deepEqual(nearestHistogram(rows, 6), [{ x: '0', y: 2 }, { x: '6', y: 0 }, { x: '12', y: 1 }]);
  assert.deepEqual(nearestHistogram([]), []);
});

test('two clonotypes\' distance loop by loop, the loops that differ', () => {
  assert.equal(regionWords([[0, 0, 0, 0], [0, 0, 0, 9]], ['TRA', 'TRB']), 'TRA alike · TRB: CDR3 9');
  assert.equal(regionWords([[4, 0, 3, 12]], ['TRB']), 'TRB: CDR1 4, CDR2.5 3, CDR3 12');
});
