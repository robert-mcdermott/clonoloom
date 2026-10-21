import assert from 'node:assert/strict';
import test from 'node:test';
import { createProject } from './project.js';
import { addResult, resultsOf } from './results.js';

const NOW = '2026-10-21T12:00:00.000Z';

test('a result is kept once, with a line of history, and found by its dataset and kind', () => {
  let project = createProject('Study', { now: NOW });
  const record = { id: 'div-0123456789abcdef', kind: 'diversity', name: 'Diversity · Study', dataset: 'd-1', definition: 'Amino-acid junction, V and J genes', filter: null, method: 'Hill numbers…', made: NOW };
  project = addResult(project, record, NOW);
  project = addResult(project, { ...record, name: 'Renamed' }, NOW);
  assert.equal(project.artifacts.length, 1);
  assert.equal(project.artifacts[0].name, 'Renamed');
  assert.equal(project.history.filter((h) => h.action === 'Kept a result').length, 1);
  assert.equal(resultsOf(project, 'd-1', 'diversity').length, 1);
  assert.equal(resultsOf(project, 'd-2', 'diversity').length, 0);
});
