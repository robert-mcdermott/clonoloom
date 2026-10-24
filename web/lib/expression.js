// Saved expression inputs pin raw sources and an explicit receptor dataset/sample. The program
// computes all coverage and counts; the project retains only provenance and the binding.
import { touch } from './project.js';

export function expressionRecord(summary, dataset = '', sample = null) {
  if ((dataset === '') !== (sample === null) || (sample !== null && (!Number.isInteger(sample) || sample < 0))) {
    throw new Error('Choose both a receptor dataset and sample, or neither.');
  }
  const cellsPath = summary.sources.find((s) => s.role === 'cells')?.path;
  return { id: summary.id, path: summary.path, sources: summary.sources.map((s) => ({ ...s })), readerVersion: summary.readerVersion, dataset, sample, ...(cellsPath ? { cellsPath } : {}) };
}

export function saveExpression(project, record, now = new Date().toISOString()) {
  const key = (x) => `${x.id}|${x.dataset}|${x.sample}`;
  const inputs = project.expression ?? [];
  const existing = inputs.findIndex((x) => key(x) === key(record));
  if (existing >= 0 && JSON.stringify(inputs[existing]) === JSON.stringify(record)) return project;
  const expression = inputs.slice();
  if (existing < 0) expression.push(record); else expression[existing] = record;
  const history = [...project.history, { time: now, action: existing < 0 ? 'Opened expression input' : 'Updated expression input', detail: `${record.path} (${record.id}); ${record.dataset ? `receptor dataset ${record.dataset}, sample ${record.sample}` : 'expression only'}` }];
  return touch({ ...project, expression, history }, now);
}

export function removeExpression(project, index, now = new Date().toISOString()) {
  const record = (project.expression ?? [])[index];
  if (!record) return project;
  return touch({ ...project, expression: project.expression.filter((_, i) => i !== index), history: [...project.history, { time: now, action: 'Removed expression input', detail: `${record.path} (${record.id})` }] }, now);
}
