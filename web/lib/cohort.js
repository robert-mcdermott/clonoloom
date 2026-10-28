// Versioned cohort designs, following SituCraft 0.9.0's named design snapshots.
// Scientific checks, populations and descriptions come from Go (internal/study).
import { fieldValue } from './datasets.js';
import { touch } from './project.js';

export const GROUP_FIELDS = { condition: 'Condition', timepoint: 'Time point', tissue: 'Tissue', cellSubset: 'Cell subset' };
const FIELDS = ['subject', 'condition', 'timepoint', 'tissue', 'cellSubset', 'batch'];

export function cohortSamples(project, dataset, summary) {
  return (summary?.samples ?? []).map((s) => ({ name: s.name, ...Object.fromEntries(FIELDS.map((f) => [f, String(fieldValue(project, dataset, s, f) ?? '').trim()])) }))
    .sort((a, b) => (a.name < b.name ? -1 : a.name > b.name ? 1 : 0));
}

export function cohortSpec(spec = {}) {
  return { unit: spec.unit ?? 'subject', layout: spec.layout ?? 'independent', groupField: spec.groupField ?? 'condition', levels: [...(spec.levels ?? ['', ''])], exclude: { ...(spec.exclude ?? {}) } };
}

export function cohortDesigns(project, dataset) {
  return (project.cohortDesigns ?? []).filter((d) => d.dataset === dataset);
}

export function saveCohortDesign(project, result, name = 'Design', now = new Date().toISOString()) {
  if (typeof result?.ok !== 'boolean' || result.format !== 'clonoloom-cohort-design' || result.version !== 1) throw new Error('Only a completed cohort design check can be saved.');
  const dataset = project.datasets.find((d) => d.id === result.input.dataset);
  if (!dataset) throw new Error('The design’s dataset is no longer in this project.');
  const title = String(name).trim() || 'Design';
  const prior = cohortDesigns(project, dataset.id).filter((d) => d.name === title);
  const revision = Math.max(0, ...prior.map((d) => d.revision)) + 1;
  const record = { id: `${result.id}-${(project.cohortDesigns ?? []).length + 1}`, dataset: dataset.id, name: title, revision, saved: now,
    sources: structuredClone(dataset.sources ?? []), metadata: structuredClone(dataset.metadata ?? []), result: structuredClone(result) };
  return touch({ ...project, cohortDesigns: [...(project.cohortDesigns ?? []), record], history: [...project.history, { time: now, action: 'Saved a cohort design', detail: `${title}, version ${revision}: ${result.words[0]}` }] }, now);
}

export function cohortDesignChanged(record, samples) {
  // The complete sample annotation is pinned, including explicitly excluded samples.
  const canonical = (rows) => JSON.stringify(rows.map((s) => [s.name, ...FIELDS.map((f) => String(s[f] ?? '').trim())]).sort((a, b) => (a[0] < b[0] ? -1 : a[0] > b[0] ? 1 : 0)));
  return canonical(record.result.input.samples) !== canonical(samples);
}
