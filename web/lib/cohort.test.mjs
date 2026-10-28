import assert from 'node:assert/strict';
import test from 'node:test';
import { cohortSamples, cohortSpec, saveCohortDesign, cohortDesignChanged } from './cohort.js';
import { createProject, parseProject, serializeProject } from './project.js';
import { annotate } from './datasets.js';
import { createStore } from '../ui/store.js';

const NOW = '2026-10-25T12:00:00Z';
const base = () => ({ ...createProject('Artificial cohort', { id: 'p-test', now: NOW }), datasets: [{ id: 'd-1', sources: [{ sha256: 'abc', path: '/source' }] }] });
const summary = { samples: [{ name: 'z', repertoire: { subjectId: 'S2', studyGroup: 'B' } }, { name: 'a', repertoire: { subjectId: 'S1', studyGroup: 'A' } }] };
const report = (p) => ({ ok: true, format: 'clonoloom-cohort-design', version: 1, id: 'cd-hash', words: ['Example design'], input: { dataset: 'd-1', samples: cohortSamples(p, 'd-1', summary), spec: cohortSpec({ levels: ['A', 'B'] }) } });

test('cohort annotations keep source names, missing values and manual overrides', () => {
  const p = annotate(base(), [{ dataset: 'd-1', sample: 'a', field: 'subject', value: ' Ω ' }]);
  const samples = cohortSamples(p, 'd-1', summary);
  assert.equal(samples[0].name, 'a'); assert.equal(samples[0].subject, 'Ω');
  assert.equal(samples[0].batch, ''); assert.equal(samples[1].condition, 'B');
});

test('saved cohort versions are immutable, round-trip and undo without rewriting old annotations', () => {
  const p = base(), r = report(p), store = createStore(p);
  store.commit(saveCohortDesign(p, r, 'Chosen subjects', NOW));
  r.input.samples[0].subject = 'changed';
  const first = store.project.cohortDesigns[0];
  assert.equal(first.result.input.samples[0].subject, 'S1');
  assert.deepEqual(parseProject(serializeProject(store.project)).cohortDesigns[0], first);
  assert.equal(cohortDesignChanged(first, cohortSamples(p, 'd-1', summary)), false);
  const edited = annotate(store.project, [{ dataset: 'd-1', sample: 'a', field: 'subject', value: 'S9' }]);
  assert.equal(cohortDesignChanged(first, cohortSamples(edited, 'd-1', summary)), true);
  store.commit(saveCohortDesign(store.project, report(p), 'Chosen subjects', NOW));
  assert.equal(store.project.cohortDesigns[1].revision, 2);
  assert.notEqual(store.project.cohortDesigns[1].id, first.id);
  store.undo(); assert.equal(store.project.cohortDesigns.length, 1);
  store.undo(); assert.equal(store.project.cohortDesigns, undefined);
  store.redo(); assert.equal(store.project.cohortDesigns[0].revision, 1);
});

test('unfinished cohort checks and malformed project fields are refused', () => {
  const p = base();
  assert.throws(() => saveCohortDesign(p, { ...report(p), ok: null }), /completed/);
  assert.throws(() => saveCohortDesign({ ...p, datasets: [] }, report(p)), /dataset/);
  assert.throws(() => parseProject(JSON.stringify({ ...p, cohortDesigns: {} })), /not a list/);
  assert.equal(parseProject(serializeProject(p)).cohortDesigns, undefined);
});


test('failed checks retain the findings and annotations without claiming a usable design', () => {
  const p = base(), r = { ...report(p), ok: false, findings: [{code:'cohort-batch-confounded', severity:'error'}] };
  const saved = saveCohortDesign(p, r, 'Confounded');
  const back = parseProject(serializeProject(saved)).cohortDesigns[0];
  assert.equal(back.result.ok, false);
  assert.deepEqual(back.result.findings, r.findings);
  r.findings.length = 0;
  assert.equal(back.result.findings.length, 1);
});
