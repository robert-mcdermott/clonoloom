import test from 'node:test';
import assert from 'node:assert/strict';
import { invariantSpecies, useInvariantSpecies } from './invariant.js';
test('invariant species is explicit and saved with history', () => {
  const p = { datasets: [{ id: 'd', name: 'Study' }], history: [] };
  assert.equal(invariantSpecies(p.datasets[0]), '');
  const q = useInvariantSpecies(p, 'd', 'mouse', '2026-10-23T00:00:00Z');
  assert.equal(invariantSpecies(q.datasets[0]), 'mouse');
  assert.equal(q.history.length, 1);
  assert.equal(p.history.length, 0);
  assert.equal(useInvariantSpecies(q, 'd', 'mouse'), q);
  assert.throws(() => useInvariantSpecies(p, 'd', 'guess'));
});
