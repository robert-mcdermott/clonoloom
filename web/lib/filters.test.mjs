import assert from 'node:assert/strict';
import test from 'node:test';
import { createProject } from './project.js';
import { cleanRules, datasetFilter, describeRules, saveFilter, useFilter, versions } from './filters.js';

const NOW = '2026-10-21T12:00:00.000Z';
const base = () => ({ ...createProject('Study', { now: NOW }), datasets: [{ id: 'd-1', name: 'Cohort' }] });

test('rules are cleaned and said in words', () => {
  assert.deepEqual(cleanRules({ productiveOnly: true, minCount: '2', loci: ['TRB', 'TRA'], maxChains: 0, excludeSamples: ['b', 'a', 'a'] }),
    { productiveOnly: true, minCount: 2, loci: ['TRA', 'TRB'], excludeSamples: ['a', 'b'] });
  assert.equal(describeRules({}), 'keeps every row');
  assert.equal(describeRules({ productiveOnly: true, maxChains: 2 }), 'productive rearrangements only; cells with at most 2 productive chains of a locus');
});

test('editing a filter makes a new version; versions are never deleted', () => {
  let p = saveFilter(base(), { dataset: 'd-1', name: 'Productive', rules: { productiveOnly: true }, reason: 'Clonotypes are defined from productive rearrangements.' }, { id: 'f-a', now: NOW });
  p = saveFilter(p, { dataset: 'd-1', name: 'Productive', rules: { productiveOnly: true, minCount: 2 }, reason: 'Singleton reads may be errors.' }, { id: 'f-b', now: NOW });
  assert.deepEqual(versions(p, 'd-1', 'Productive').map((f) => [f.id, f.version]), [['f-a', 1], ['f-b', 2]]);
  assert.equal(datasetFilter(p, p.datasets[0]).id, 'f-b');
  assert.match(p.history.at(-1).detail, /Productive version 2 for Cohort: productive rearrangements only; counts of at least 2\. Singleton reads may be errors\./);
  p = useFilter(p, 'd-1', 'f-a', NOW);
  assert.equal(datasetFilter(p, p.datasets[0]).version, 1);
  p = useFilter(p, 'd-1', null, NOW);
  assert.equal(datasetFilter(p, p.datasets[0]), null);
  assert.equal(p.filters.length, 2);
  assert.throws(() => saveFilter(p, { dataset: 'd-1', name: 'x', rules: {}, reason: ' ' }), /Say why/);
  assert.throws(() => saveFilter(p, { dataset: 'd-1', name: ' ', rules: {}, reason: 'r' }), /needs a name/);
});
