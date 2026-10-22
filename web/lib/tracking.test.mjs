import assert from 'node:assert/strict';
import test from 'node:test';
import { createProject } from './project.js';
import { choiceShort, choiceWords, followAlso, parseList, trackingChoice, trackRequest, useTracking } from './tracking.js';

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
