import { test } from 'node:test';
import assert from 'node:assert/strict';
import { familyOptions, familyWords, familySubjects, useFamilies } from './families.js';

test('family choices preserve null, explicit zero and existing project fields', () => {
  const p = { datasets: [{ id: 'd', name: 'B cells', definition: { junction: 'aa' } }], history: [] };
  assert.equal(familyOptions(p.datasets[0]).threshold, null);
  const next = useFamilies(p, 'd', { mode: 'paired', threshold: 0 }, 'now');
  assert.equal(next.datasets[0].families.threshold, 0);
  assert.deepEqual(next.datasets[0].definition, p.datasets[0].definition);
  assert.equal(p.datasets[0].families, undefined);
  assert.equal(useFamilies(next, 'd', { threshold: 0 }), next);
  assert.equal(familyOptions(JSON.parse(JSON.stringify(next)).datasets[0]).mode, 'paired');
  assert.match(familyWords(next.datasets[0].families), /≤ 0/);
  for (const threshold of [NaN, Infinity, -1, 1.01, '0.1']) assert.throws(() => useFamilies(p, 'd', { threshold }));
  assert.throws(() => useFamilies(p, 'd', { mode: 'both' }));
});

test('family subjects share study metadata precedence without sample-name inference', () => {
  const summary = { samples: [{ name: 'a', repertoire: { subjectId: 'P1' } }, { name: 'S02-pre' }, { name: 'b', metadata: { Subject: 'P3' } }] };
  assert.deepEqual(familySubjects({ samples: [] }, 'd', summary), { a: 'P1', 'S02-pre': '', b: 'P3' });
  assert.equal(familySubjects({ samples: [{ dataset: 'd', sample: 'a', fields: { subject: 'P2' } }] }, 'd', summary).a, 'P2');
});
