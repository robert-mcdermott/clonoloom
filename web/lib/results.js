// Results kept in the project (clonoloom-spec/design.md, "Results and their record"): each an
// artifact of the program (internal/artifacts) the project names with what it was made from, so
// the project tree lists it and the methods paragraph describes it. Every edit returns a new
// project.

import { touch } from './project.js';

// Records a result (or updates one kept before): { id, kind, name, dataset, definition (its
// name), filter (the filter version's id, or null), method, made }.
export function addResult(project, record, now = new Date().toISOString()) {
  const kept = (project.artifacts ?? []).filter((a) => a.id !== record.id);
  const known = (project.artifacts ?? []).some((a) => a.id === record.id);
  const next = { ...project, artifacts: [...kept, { ...record }] };
  if (known) return touch(next, now);
  return touch({ ...next, history: [...project.history, { time: now, action: 'Kept a result', detail: `${record.name}: ${record.method}` }] }, now);
}

// The results of a kind for a dataset, the latest last.
export function resultsOf(project, dataset, kind) {
  return (project.artifacts ?? []).filter((a) => a.dataset === dataset && a.kind === kind);
}
