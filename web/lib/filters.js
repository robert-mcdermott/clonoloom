// QC filters as versions (clonoloom-spec/requirements.md, Q6): a filter is a named set of rules
// (internal/qc's Filter) with the reason it was made; editing one makes a new version, and no
// version is ever deleted, so a result made under an earlier one can always say what it kept. A
// dataset's filter is the version its analyses use. Every edit returns a new project.

import { touch } from './project.js';

// The rules a filter can have, as the QC view's editor shows them.
export const RULES = [
  { id: 'productiveOnly', kind: 'flag', label: 'Productive rearrangements only', hint: 'Leaves out rearrangements out of frame or with a stop codon.' },
  { id: 'minCount', kind: 'number', label: 'Minimum count', hint: 'Bulk data: leaves out rearrangements counted fewer times, in the dataset\'s unit.' },
  { id: 'loci', kind: 'loci', label: 'Loci', hint: 'Keeps only these loci (all when none is chosen).' },
  { id: 'cellsOnly', kind: 'flag', label: 'Cells only', hint: 'Single cell: keeps contigs of barcodes Cell Ranger called cells.' },
  { id: 'highConfidenceOnly', kind: 'flag', label: 'High-confidence contigs only', hint: 'Single cell: keeps Cell Ranger\'s high-confidence contigs.' },
  { id: 'maxChains', kind: 'number', label: 'Most productive chains of a locus in a cell', hint: 'Single cell: leaves out cells with more (possible doublets); two α chains or two light chains occur in single cells.' },
  { id: 'excludeSamples', kind: 'samples', label: 'Samples left out', hint: 'Leaves out whole samples, with the reason given below.' },
];

export const LOCI = ['TRA', 'TRB', 'TRG', 'TRD', 'IGH', 'IGK', 'IGL'];

// The rules without empty ones, in a fixed order (as the program reads them).
export function cleanRules(rules) {
  const out = {};
  if (rules.productiveOnly) out.productiveOnly = true;
  if (Number(rules.minCount) > 0) out.minCount = Number(rules.minCount);
  if (rules.loci?.length) out.loci = LOCI.filter((l) => rules.loci.includes(l));
  if (rules.cellsOnly) out.cellsOnly = true;
  if (rules.highConfidenceOnly) out.highConfidenceOnly = true;
  if (Number(rules.maxChains) > 0) out.maxChains = Math.floor(Number(rules.maxChains));
  if (rules.excludeSamples?.length) out.excludeSamples = [...new Set(rules.excludeSamples)].sort();
  return out;
}

// What a filter's rules do, in words.
export function describeRules(rules) {
  const r = cleanRules(rules);
  const parts = [];
  if (r.productiveOnly) parts.push('productive rearrangements only');
  if (r.minCount) parts.push(`counts of at least ${r.minCount}`);
  if (r.loci) parts.push(`${r.loci.join(', ')} only`);
  if (r.cellsOnly) parts.push('cells only');
  if (r.highConfidenceOnly) parts.push('high-confidence contigs only');
  if (r.maxChains) parts.push(`cells with at most ${r.maxChains} productive chain${r.maxChains === 1 ? '' : 's'} of a locus`);
  if (r.excludeSamples) parts.push(`${r.excludeSamples.length} sample${r.excludeSamples.length === 1 ? '' : 's'} left out (${r.excludeSamples.join(', ')})`);
  return parts.length ? parts.join('; ') : 'keeps every row';
}

function newId() {
  const random = globalThis.crypto?.randomUUID?.() ?? `${Date.now().toString(36)}${Math.random().toString(36).slice(2)}`;
  return `f-${random.replace(/-/g, '').slice(0, 16)}`;
}

// A filter's versions, oldest first.
export function versions(project, datasetId, name) {
  return (project.filters ?? []).filter((f) => f.dataset === datasetId && f.name === name).sort((a, b) => a.version - b.version);
}

// Saves a filter: a new name starts at version 1, an existing name gets its next version. The
// dataset's analyses use it from then on.
export function saveFilter(project, { dataset, name, rules, reason }, { id = newId(), now = new Date().toISOString() } = {}) {
  const trimmed = String(name).trim();
  if (!trimmed) throw new Error('A filter needs a name.');
  if (!String(reason ?? '').trim()) throw new Error('Say why the filter is made: the reason goes with every result that uses it.');
  const version = (versions(project, dataset, trimmed).at(-1)?.version ?? 0) + 1;
  const filter = { id, dataset, name: trimmed, version, rules: cleanRules(rules), reason: String(reason).trim(), created: now };
  const target = project.datasets.find((d) => d.id === dataset);
  return touch({
    ...project,
    filters: [...(project.filters ?? []), filter],
    datasets: project.datasets.map((d) => (d.id === dataset ? { ...d, filter: filter.id } : d)),
    history: [...project.history, { time: now, action: 'Saved a filter', detail: `${trimmed} version ${version} for ${target?.name ?? dataset}: ${describeRules(filter.rules)}. ${filter.reason}` }],
  }, now);
}

// Chooses the filter version a dataset's analyses use (null: none).
export function useFilter(project, datasetId, filterId, now = new Date().toISOString()) {
  const dataset = project.datasets.find((d) => d.id === datasetId);
  if (!dataset || (dataset.filter ?? null) === filterId) return project;
  const filter = (project.filters ?? []).find((f) => f.id === filterId);
  return touch({
    ...project,
    datasets: project.datasets.map((d) => (d.id === datasetId ? { ...d, filter: filterId } : d)),
    history: [...project.history, { time: now, action: 'Chose a filter', detail: `${dataset.name}: ${filter ? `${filter.name} version ${filter.version}` : 'no filter'}` }],
  }, now);
}

// The filter version a dataset uses, or null.
export function datasetFilter(project, dataset) {
  return dataset?.filter ? (project.filters ?? []).find((f) => f.id === dataset.filter) ?? null : null;
}
