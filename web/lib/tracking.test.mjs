import assert from 'node:assert/strict';
import test from 'node:test';
import { createProject } from './project.js';
import { choiceShort, choiceWords, comparisonChoice, comparisonWords, compareRequest, followAlso, parseList, timePoints, trackingChoice, trackRequest, useComparison, useTracking } from './tracking.js';

const NOW = '2026-10-21T12:00:00.000Z';
const project = () => ({ ...createProject('S', { now: NOW }), datasets: [{ id: 'd-1', name: 'CAR-T' }] });

test('a dataset follows its reference sample\'s 10 largest until told otherwise', () => {
  const c = trackingChoice(project().datasets[0]);
  assert.deepEqual(c, { by: 'reference', n: 10, list: [] });
  assert.deepEqual(trackRequest(c), { by: 'reference', n: 10 });
  assert.match(choiceWords(c), /each reference sample's 10 largest/);
  assert.equal(choiceShort(c), 'the reference sample\'s 10 largest');
  assert.equal(choiceShort({ by: 'list', list: ['a', 'b'] }), '2 listed');
});

test('choosing what is followed, and a list kept whatever is followed', () => {
  let p = useTracking(project(), 'd-1', { by: 'largest' }, NOW);
  assert.deepEqual(trackingChoice(p.datasets[0]), { by: 'largest', n: 5, list: [] });
  assert.match(p.history.at(-1).detail, /CAR-T: each sample's 5 largest/);
  assert.equal(useTracking(p, 'd-1', { by: 'largest', n: 5 }, NOW), p, 'the same choice changes nothing');
  p = useTracking(p, 'd-1', { n: 20 }, NOW);
  assert.equal(trackingChoice(p.datasets[0]).n, 20);
  p = followAlso(p, 'd-1', ['c-0123', 'CASSLAGGPEAFF', 'c-0123'], NOW);
  assert.deepEqual(p.datasets[0].tracking, { by: 'list', list: ['c-0123', 'CASSLAGGPEAFF'] });
  assert.deepEqual(trackRequest(trackingChoice(p.datasets[0])), { by: 'list', list: ['c-0123', 'CASSLAGGPEAFF'] });
  p = useTracking(p, 'd-1', { by: 'reference' }, NOW);
  assert.deepEqual(trackingChoice(p.datasets[0]), { by: 'reference', n: 10, list: ['c-0123', 'CASSLAGGPEAFF'] });
  assert.equal(followAlso(p, 'd-9', ['x'], NOW), p);
});

test('lists read from text', () => {
  assert.deepEqual(parseList('CASSLAGGPEAFF\nc-0123, CASSPGQGAYEQYF CASSA\n\nCAVSN; CASSL\nCASSA'), ['CASSLAGGPEAFF', 'c-0123', 'CASSPGQGAYEQYF', 'CASSA', 'CAVSN; CASSL']);
  assert.deepEqual(parseList(''), []);
});

// A design as api_study.go answers it: two subjects, one without the day-7 sample.
const design = {
  subjects: [
    { id: 'P1', samples: [
      { name: 'P1-product', reference: 'infusion product', key: 'reference' },
      { name: 'P1-pre', time: { kind: 'before', label: 'baseline' }, key: 'baseline' },
      { name: 'P1-d7', time: { kind: 'day', days: 7, label: 'day 7' }, key: 'day 7' },
      { name: 'P1-w4', time: { kind: 'day', days: 28, label: 'week 4' }, key: 'day 28' },
    ] },
    { id: 'P2', samples: [
      { name: 'P2-pre', time: { kind: 'before', label: 'pre' }, key: 'baseline' },
      { name: 'P2-d28', time: { kind: 'day', days: 28, label: 'day 28' }, key: 'day 28' },
    ] },
  ],
};

test('the time points subjects can be compared at, and the comparison chosen', () => {
  const points = timePoints(design);
  assert.deepEqual(points.map((p) => [p.key, p.label, p.subjects]), [['reference', 'infusion product', 1], ['baseline', 'baseline', 2], ['day 7', 'day 7', 1], ['day 28', 'week 4', 2]]);
  let p = project();
  const c = comparisonChoice(p.datasets[0], points);
  assert.deepEqual(c, { from: 'baseline', to: 'day 7', test: 'auto', alternative: 'two-sided', correction: 'bh', alpha: 0.01, stable: null });
  assert.match(comparisonWords(c), /^the beta-binomial test of its count at the first time point given its total, its variance fitted on samples that should not differ, or, without them, the binomial test, two-sided, the false discovery rate controlled within each subject by Benjamini and Hochberg's procedure at 0\.01$/);
  // Two time points named as stable go with the beta-binomial test only.
  assert.deepEqual(compareRequest({ ...c, stable: { from: 'baseline', to: 'day 7' } }).stable, { from: 'baseline', to: 'day 7' });
  assert.equal(compareRequest({ ...c, test: 'fisher', stable: { from: 'baseline', to: 'day 7' } }).stable, undefined);
  assert.equal(comparisonChoice({ comparison: { stable: { from: 'day 99', to: 'baseline' } } }, points).stable, null);
  p = useComparison(p, 'd-1', { to: 'day 28', correction: 'storey' }, NOW);
  assert.match(p.history.at(-1).detail, /CAR-T: — against day 28/);
  const d = comparisonChoice(p.datasets[0], points);
  assert.deepEqual([d.from, d.to, d.correction], ['baseline', 'day 28', 'storey']);
  assert.equal(useComparison(p, 'd-1', { to: 'day 28' }, NOW), p);
  // A kept comparison of time points the design no longer has: the defaults.
  assert.deepEqual(comparisonChoice({ comparison: { from: 'day 99', to: 'baseline' } }, points).from, 'baseline');
  // Following its calls.
  p = useTracking(p, 'd-1', { by: 'test' }, NOW);
  const choice = trackingChoice(p.datasets[0], d);
  assert.deepEqual(trackRequest(choice), { by: 'test', n: 20, test: compareRequest(d) });
  assert.equal(choiceShort(choice), 'the 20 most significant calls between baseline and day 28');
  assert.match(choiceWords(choice), /the 20 clonotypes the beta-binomial test \(the binomial test where its variance could not be fitted\) called changed most significantly between baseline and day 28/);
});
