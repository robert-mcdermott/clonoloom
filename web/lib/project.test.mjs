import assert from 'node:assert/strict';
import { readdirSync, readFileSync } from 'node:fs';
import test from 'node:test';
import { PROJECT_FORMAT, PROJECT_VERSION, createProject, isEmptyProject, parseProject, rename, serializeProject } from './project.js';

test('a new project is an empty, versioned document with a valid id', () => {
  const project = createProject('CAR-T kinetics', { now: '2026-10-21T12:00:00.000Z' });
  assert.equal(project.format, PROJECT_FORMAT);
  assert.equal(project.version, PROJECT_VERSION);
  assert.match(project.id, /^[A-Za-z0-9_-]{1,80}$/);
  assert.equal(project.created, '2026-10-21T12:00:00.000Z');
  assert.ok(isEmptyProject(project));
  assert.notEqual(createProject().id, createProject().id);
});

test('a project survives saving and reopening byte for byte', () => {
  const project = createProject('Round trip', { now: '2026-10-14T12:00:00.000Z' });
  const text = serializeProject(project);
  const back = parseProject(text);
  assert.deepEqual(back, project);
  assert.equal(serializeProject(back), text);
});

test('fields a later version of the envelope added are filled in when an older document opens', () => {
  const old = JSON.stringify({ format: PROJECT_FORMAT, version: 1, id: 'p-old', name: 'Old', created: '2026-10-01T00:00:00Z' });
  const project = parseProject(old);
  assert.deepEqual(project.datasets, []);
  assert.deepEqual(project.artifacts, []);
  assert.deepEqual(project.decisions, []);
  assert.deepEqual(project.views, []);
  assert.equal(project.name, 'Old');
});

test('other documents are refused with a plain message', () => {
  assert.throws(() => parseProject('{not json'), /not valid JSON/);
  assert.throws(() => parseProject(JSON.stringify({ format: 'mavescape-workspace', version: 1, id: 'x' })), /not a ClonoLoom project/);
  assert.throws(() => parseProject(JSON.stringify({ format: PROJECT_FORMAT, version: 99, id: 'x' })), /newer ClonoLoom/);
  assert.throws(() => parseProject(JSON.stringify({ format: PROJECT_FORMAT, version: 1, id: '../etc' })), /no valid id/);
  assert.throws(() => parseProject(JSON.stringify({ format: PROJECT_FORMAT, version: 1, id: 'p-1', datasets: {} })), /datasets is not a list/);
});

test('rename returns a new value and leaves the original alone', () => {
  const project = createProject('A');
  const renamed = rename(project, '  B  ', '2026-10-14T13:00:00.000Z');
  assert.equal(renamed.name, 'B');
  assert.equal(renamed.modified, '2026-10-14T13:00:00.000Z');
  assert.equal(project.name, 'A');
  assert.equal(rename(project, '   '), project);
  assert.equal(rename(project, 'A'), project);
});

test('a project with a dataset, decision or saved view is worth saving', () => {
  const empty = createProject();
  assert.ok(isEmptyProject(empty));
  for (const field of ['datasets', 'artifacts', 'decisions', 'views']) {
    assert.equal(isEmptyProject({ ...empty, [field]: [{ id: 'x' }] }), false, field);
  }
});

test('every release\'s saved project still opens, byte for byte (validation/projects)', () => {
  const root = new URL('../../validation/projects/', import.meta.url);
  const versions = readdirSync(root).filter((v) => /^\d+\.\d+\.\d+$/.test(v));
  assert.ok(versions.includes('0.2.0'));
  for (const v of versions) {
    const text = readFileSync(new URL(`${v}/project.json`, root), 'utf8');
    const project = parseProject(text);
    assert.equal(serializeProject(project), text, v);
    assert.ok(project.datasets.length && project.artifacts.length, v);
    for (const a of project.artifacts) {
      assert.ok(a.kind === 'expression-map'
        ? project.expression.some((e) => e.id === a.expression)
        : project.datasets.some((d) => d.id === a.dataset), `${v}: ${a.id}`);
    }
  }
});

test('the single-cell release retains all five results and every pinned parent', () => {
  const root = new URL('../../validation/projects/0.6.0/', import.meta.url);
  const project = parseProject(readFileSync(new URL('project.json', root), 'utf8'));
  assert.equal(project.datasets.length, 1);
  assert.equal(project.datasets[0].samples.length, 4);
  assert.equal(project.expression.length, 1);
  assert.equal(project.expression[0].sample, 0);
  assert.equal(project.expression[0].sources.length, 4);
  assert.deepEqual(project.artifacts.map((a) => a.kind), ['expression-map', 'phenotypes', 'modularity', 'conga', 'anndata-export']);
  const seen = new Set([project.datasets[0].id, project.expression[0].id]);
  for (const a of project.artifacts) {
    const manifest = JSON.parse(readFileSync(new URL(`artifacts/${a.id}/manifest.json`, root), 'utf8'));
    for (const parent of manifest.lineage.parents) {
      assert.ok(seen.has(parent.id), `${a.kind}: missing or later parent ${parent.id}`);
      if (!['dataset', 'expression'].includes(parent.kind)) assert.match(parent.sha256, /^[a-f0-9]{64}$/);
    }
    seen.add(a.id);
  }
});

test('the B-cell release retains analysis choices, IMGT grouping and its result chain', () => {
  const project = parseProject(readFileSync(new URL('../../validation/projects/0.5.0/project.json', import.meta.url), 'utf8'));
  const [study, imgt] = project.datasets;
  assert.equal(study.rows, 120);
  assert.equal(study.samples.length, 6);
  assert.deepEqual(study.families, { mode: 'auto', scope: 'subject', threshold: 0.1 });
  assert.equal(imgt.sources.length, 5);
  assert.ok(imgt.sources.every((s) => s.imgtGroup === 'imgt-inputs'));
  assert.deepEqual(project.artifacts.filter((a) => a.dataset === study.id).map((a) => a.kind), ['families', 'families', 'germlines', 'shm', 'lineages', 'isotypes']);
  for (const a of project.artifacts) {
    const manifest = JSON.parse(readFileSync(new URL(`../../validation/projects/0.5.0/artifacts/${a.id}/manifest.json`, import.meta.url), 'utf8'));
    const parameters = manifest.lineage.parameters;
    const options = parameters[a.kind];
    for (const parent of [options?.families, options?.germlines].filter(Boolean)) {
      assert.ok(project.artifacts.some((p) => p.id === parent && p.dataset === a.dataset), `${a.kind}: missing parent ${parent}`);
    }
  }
});


test('the cohort release retains every study, usable and failed checks, and analysis parents', () => {
  const p = parseProject(readFileSync(new URL('../../validation/projects/0.7.0/project.json', import.meta.url), 'utf8'));
  assert.equal(p.datasets.length, 5);
  assert.equal(p.cohortDesigns.length, 5);
  assert.equal(p.artifacts.length, 16);
  for (const d of p.cohortDesigns) {
    const kinds = p.artifacts.filter((a) => a.dataset === d.dataset).map((a) => a.kind);
    assert.deepEqual(kinds, d.result.ok ? ['diversity','cohort-features','cohort-compare','cohort-counts'] : []);
  }
  const failed=p.cohortDesigns.find((d)=>!d.result.ok);
  assert.ok(failed.result.findings.some((f)=>f.code==='cohort-batch-confounded'));
});

test('the record release retains its checkpoint and exact saved QC evidence', () => {
  const root = new URL('../../validation/projects/0.8.0/', import.meta.url);
  const p = parseProject(readFileSync(new URL('project.json', root), 'utf8'));
  const study = JSON.parse(readFileSync(new URL('study.json', root), 'utf8'));
  assert.equal(p.datasets.length, 1);
  assert.equal(p.datasets[0].samples.length, 26);
  assert.deepEqual(p.artifacts.map(a => a.kind), ['batch-qc']);
  assert.equal(p.artifacts[0].id, study.artifact);
  assert(p.checkpoints.some(c => c.id === study.checkpoint));
  const original = parseProject(readFileSync(new URL('original-project.json', root), 'utf8'));
  assert.deepEqual(original.artifacts, p.artifacts);
  assert.deepEqual(original.checkpoints, p.checkpoints);
});

test('the suite release retains separate dataset scopes and only portable path changes', () => {
  const root = new URL('../../validation/projects/0.9.0/', import.meta.url);
  const project = parseProject(readFileSync(new URL('project.json', root), 'utf8'));
  const original = parseProject(readFileSync(new URL('original-project.json', root), 'utf8'));
  assert.equal(project.datasets.length, 8);
  assert.deepEqual(project.artifacts.map(a => a.kind), ['families', 'germlines', 'shm', 'isotypes', 'lineages']);
  assert(project.artifacts.every(a => a.dataset === project.datasets[0].id));
  for (let i = 0; i < project.datasets.length; i++) {
    for (const key of ['sources', 'metadata']) {
      const portable = project.datasets[i][key] ?? [];
      const saved = original.datasets[i][key] ?? [];
      assert.equal(portable.length, saved.length);
      for (let j = 0; j < portable.length; j++) {
        assert(portable[j].path.startsWith('data/'));
        assert(saved[j].path.endsWith('/' + portable[j].path));
        saved[j].path = portable[j].path;
      }
    }
  }
  assert.deepEqual(project, original);
});
