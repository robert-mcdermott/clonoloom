import assert from 'node:assert/strict';
import test from 'node:test';
import { createProject } from './project.js';
import { datasetDefinition, defaultDefinition, sameMeaning, saveDefinition, useDefinition } from './definitions.js';

const NOW = '2026-10-21T12:00:00.000Z';

test('datasets start with the definition their data suit', () => {
  assert.deepEqual(defaultDefinition({ singleCell: false }).chains, []);
  assert.deepEqual(defaultDefinition({ singleCell: true }, ['TRA', 'TRB']).chains, ['TRA', 'TRB']);
  assert.deepEqual(defaultDefinition({ singleCell: true }, ['IGH', 'IGK', 'IGL']).chains, ['IGH', 'light']);
  assert.equal(datasetDefinition({ singleCell: false, definition: { name: 'x', junction: 'nt' } }).junction, 'nt');
});

test('choosing and saving definitions', () => {
  let p = { ...createProject('S', { now: NOW }), datasets: [{ id: 'd-1', name: 'Cohort', singleCell: false }] };
  const nt = { name: 'Nucleotides', chains: [], junction: 'nt', v: 'gene', j: 'gene', productiveOnly: true };
  p = useDefinition(p, 'd-1', nt, 'each locus separately by nucleotide junction, V gene and J gene', NOW);
  assert.equal(p.datasets[0].definition.junction, 'nt');
  assert.match(p.history.at(-1).detail, /Cohort: Nucleotides \(each locus/);
  assert.equal(useDefinition(p, 'd-1', { ...nt }, '', NOW), p);
  p = saveDefinition(p, nt, NOW);
  p = saveDefinition(p, { ...nt, v: 'family' }, NOW);
  assert.equal(p.definitions.length, 1);
  assert.equal(p.definitions[0].v, 'family');
  assert.ok(sameMeaning({ name: 'a', junction: 'aa' }, { name: 'b', junction: 'aa', productiveOnly: true, v: 'gene', j: 'gene', chains: [] }));
  assert.ok(!sameMeaning({ junction: 'aa' }, { junction: 'aa', v: 'none' }));
  assert.throws(() => saveDefinition(p, { name: ' ' }), /needs a name/);
});
