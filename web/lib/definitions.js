// Clonotype definitions in the project (clonoloom-spec/requirements.md, C1, C2): each dataset's
// definition, chosen from the presets or edited, and the user's own saved for reuse. The program
// normalizes a definition, groups clonotypes by it and says it in words (internal/clonotype);
// this module only keeps the choices. Every edit returns a new project.

import { touch } from './project.js';

export const JUNCTIONS = { aa: 'amino-acid junction', nt: 'nucleotide junction' };
export const LEVELS = { gene: 'gene', allele: 'allele', family: 'family', none: 'not used' };

// The definition a dataset starts with: paired chains for single cells (αβ for T cells, heavy and
// light for B cells), and for bulk data the amino-acid junction with V and J genes, each locus
// separately.
export function defaultDefinition(dataset, loci = []) {
  if (dataset?.singleCell) {
    if (loci.some((l) => l === 'IGH' || l === 'IGK' || l === 'IGL') && !loci.some((l) => l === 'TRA' || l === 'TRB')) {
      return { name: 'Paired heavy and light: amino-acid junctions, V and J genes', chains: ['IGH', 'light'], junction: 'aa', v: 'gene', j: 'gene', cells: 'primary' };
    }
    return { name: 'Paired αβ: amino-acid junctions, V and J genes', chains: ['TRA', 'TRB'], junction: 'aa', v: 'gene', j: 'gene', cells: 'primary' };
  }
  return { name: 'Amino-acid junction, V and J genes', chains: [], junction: 'aa', v: 'gene', j: 'gene' };
}

// The definition a dataset uses.
export function datasetDefinition(dataset, loci) {
  return dataset?.definition ?? defaultDefinition(dataset, loci);
}

// What makes two definitions group alike (their names aside).
export function sameMeaning(a, b) {
  const key = (d) => JSON.stringify([d.chains ?? [], d.junction ?? 'aa', d.v ?? 'gene', d.j ?? 'gene', Boolean(d.productiveOnly) || (d.junction ?? 'aa') === 'aa', d.cells ?? 'primary', Boolean(d.requireAll), Boolean(d.vendor)]);
  return key(a) === key(b);
}

// Chooses a dataset's definition.
export function useDefinition(project, datasetId, definition, describe = '', now = new Date().toISOString()) {
  const dataset = project.datasets.find((d) => d.id === datasetId);
  if (!dataset) return project;
  if (dataset.definition && sameMeaning(dataset.definition, definition) && dataset.definition.name === definition.name) return project;
  return touch({
    ...project,
    datasets: project.datasets.map((d) => (d.id === datasetId ? { ...d, definition: { ...definition } } : d)),
    history: [...project.history, { time: now, action: 'Chose a clonotype definition', detail: `${dataset.name}: ${definition.name}${describe ? ` (${describe})` : ''}` }],
  }, now);
}

// Saves a definition under its name for reuse in the project's other datasets (replacing one of
// the same name).
export function saveDefinition(project, definition, now = new Date().toISOString()) {
  const name = String(definition.name ?? '').trim();
  if (!name) throw new Error('A definition needs a name.');
  const kept = (project.definitions ?? []).filter((d) => d.name !== name);
  return touch({ ...project, definitions: [...kept, { ...definition, name }] }, now);
}
