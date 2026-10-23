import { test } from 'node:test';
import assert from 'node:assert/strict';
import { databaseOptions, databaseWords, useDatabase } from './database.js';
import { choiceWords, trackingChoice, trackRequest, useTracking } from './tracking.js';

test('specificity choices and explicit Tracking selection preserve provenance', () => {
  const p = { datasets: [{ id: 'd', name: 'Test' }], history: [] };
  const next = useDatabase(p, 'd', { database: 'simulated-vaccine', genes: true }, 'now');
  assert.equal(p.datasets[0].specificity, undefined);
  assert.equal(next.history.length, 1);
  assert.match(databaseWords(next.datasets[0].specificity), /matching V\/J/);
  assert.equal(useDatabase(next, 'd', { genes: true }), next);
  assert.equal(databaseOptions({ specificity: { radius: 2000 } }).radius, 1000);
  const evidence = { artifact: 'spc-test', epitope: 'PEPTIDE', antigen: 'protein', species: 'Simulated' };
  const followed = useTracking(next, 'd', { by: 'list', list: ['a', 'b'], evidence });
  const choice = trackingChoice(followed.datasets[0]);
  assert.deepEqual(trackRequest(choice), { by: 'list', list: ['a', 'b'], evidence });
  assert.match(choiceWords(choice), /PEPTIDE.*spc-test/);
  const edited = useTracking(followed, 'd', { by: 'list', list: ['a'] });
  assert.equal(edited.datasets[0].tracking.evidence, undefined);
});
