// Choices only. Distances, bins, eligibility and memberships are computed in Go.
import { fieldValue } from './datasets.js';
import { touch } from './project.js';
export function familyOptions(dataset) {
  const o = dataset?.families ?? {};
  const mode = o.mode ?? 'auto';
  const scope = o.scope ?? 'subject';
  const threshold = o.threshold ?? null;
  if (!['auto', 'heavy', 'paired'].includes(mode)) throw new Error('Choose auto, heavy or paired chains.');
  if (!['subject', 'sample'].includes(scope)) throw new Error('Choose subject or sample boundaries.');
  if (threshold !== null && (typeof threshold !== 'number' || !Number.isFinite(threshold) || threshold < 0 || threshold > 1)) throw new Error('Choose a threshold from 0 to 1, or leave it empty for distances only.');
  return { mode, scope, threshold };
}
export function familyWords(o) {
  return `${o.mode} chains, within ${o.scope}, ${o.threshold === null ? 'nearest distances only' : `single linkage at distance ≤ ${o.threshold}`}`;
}
export function useFamilies(project, id, changes, now = new Date().toISOString()) {
  const d = project.datasets.find((x) => x.id === id);
  if (!d) return project;
  const before = familyOptions(d);
  const families = familyOptions({ families: { ...before, ...changes } });
  if (JSON.stringify(before) === JSON.stringify(families)) return project;
  return touch({ ...project, datasets: project.datasets.map((x) => x.id === id ? { ...x, families } : x), history: [...project.history, { time: now, action: 'Chose B-cell families', detail: `${d.name}: ${familyWords(families)}` }] }, now);
}

// Match the study views: explicit annotations override supplied metadata. No name inference.
export function familySubjects(project, id, summary) {
  return Object.fromEntries((summary?.samples ?? []).map((sample) => [sample.name, fieldValue(project, id, sample, 'subject') ?? '']));
}
