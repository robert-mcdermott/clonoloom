import { test } from 'node:test';
import assert from 'node:assert/strict';
import { networkOptions, networkWords, useNetworks } from './networks.js';

test('network choices constrain metric cutoffs and preserve project history', () => {
  const p = { datasets: [{ id: 'd', name: 'Test' }], history: [] };
  assert.equal(networkOptions({ networks: { metric: 'hamming', cutoff: 99 } }).cutoff, 2);
  assert.equal(networkOptions({ networks: { metric: 'identity', cutoff: 2 } }).cutoff, 0);
  const next = useNetworks(p, 'd', { metric: 'levenshtein', cutoff: 2, arms: 'any', sameV: true }, 'now');
  assert.equal(p.datasets[0].networks, undefined);
  assert.equal(next.history.length, 1);
  assert.equal(networkWords(next.datasets[0].networks), 'Levenshtein at cutoff 2, any retained chains, same V gene, whole junctions');
  assert.equal(useNetworks(next, 'd', { cutoff: 2 }), next);
});
