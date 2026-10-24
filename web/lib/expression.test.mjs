import test from 'node:test';
import assert from 'node:assert/strict';
import { createProject, isEmptyProject, parseProject, serializeProject } from './project.js';
import { expressionRecord, saveExpression, removeExpression } from './expression.js';

const summary = { id: 'x-abc', path: '/data/mex', sources: [{ role: 'matrix', path: '/data/mex/matrix.mtx', size: 40, sha256: 'abcdef' }], readerVersion: 1 };
test('expression records retain fingerprints and explicit sample zero through saved reopening', () => {
  const before = createProject();
  const record = expressionRecord(summary, 'd-one', 0);
  const project = saveExpression(before, record);
  assert.ok(isEmptyProject(before)); assert.equal(isEmptyProject(project), false);
  assert.equal(project.history.at(-1).action, 'Opened expression input');
  assert.equal(saveExpression(project, record), project);
  assert.equal(removeExpression(project, 0).history.at(-1).action, 'Removed expression input');
  const moved = saveExpression(project, { ...record, path: '/moved/mex' });
  assert.equal(moved.expression.length, 1);
  assert.equal(moved.history.at(-1).action, 'Updated expression input');
  assert.equal(record.sample, 0); assert.notEqual(record.sources[0], summary.sources[0]);
  assert.deepEqual(parseProject(serializeProject(project)), project);
  assert.equal(saveExpression(project, record).expression.length, 1);
  const other = saveExpression(project, expressionRecord(summary, 'd-two', 0));
  assert.equal(other.expression.length, 2);
  assert.equal(removeExpression(other, 0).expression[0].dataset, 'd-two');
  assert.equal(other.expression.length, 2);
});
test('unbound expression is explicit and partial bindings are refused', () => {
  assert.equal(expressionRecord(summary).sample, null);
  for (const args of [['d-one', null], ['', 0], ['d-one', -1], ['d-one', 0.5]]) assert.throws(() => expressionRecord(summary, ...args), /both/);
  const project = createProject(); project.expression = {};
  assert.throws(() => parseProject(serializeProject(project)), /expression is not a list/);
});

test('optional annotation sources survive saved reopening without changing old record shapes', () => {
  assert.equal('cellsPath' in expressionRecord(summary), false);
  const record = expressionRecord({ ...summary, sources: [...summary.sources, { role: 'cells', path: '/data/cells.tsv', sha256: 'xyz', size: 30 }] });
  assert.equal(record.cellsPath, '/data/cells.tsv');
  const project = saveExpression(createProject(), record);
  assert.deepEqual(parseProject(serializeProject(project)).expression[0], record);
});

test('AnnData selections survive project serialization without aliasing the response', () => {
  const selection = { modality: 'gex', layer: 'layers/counts', embedding: 'X_umap', airrModality: 'receptors', airrKey: 'airr', featureName: 'gene_name', featureType: 'feature_type' };
  const record = expressionRecord({ ...summary, anndata: { selection } });
  assert.deepEqual(record.anndata, selection); assert.notEqual(record.anndata, selection);
  const restored = parseProject(serializeProject(saveExpression(createProject(), record)));
  assert.deepEqual(restored.expression[0].anndata, selection);
  assert.equal('anndata' in expressionRecord(summary), false);
});
