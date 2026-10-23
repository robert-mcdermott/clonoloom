// The species is a recorded user choice, never inferred from shared gene names.
import { touch } from './project.js';
export const POPULATIONS = ['MAIT', 'iNKT', 'GEM'];
export function invariantSpecies(d) { return ['human', 'mouse'].includes(d?.invariantSpecies) ? d.invariantSpecies : ''; }
export function useInvariantSpecies(project, id, species, now = new Date().toISOString()) {
  if (!['', 'human', 'mouse'].includes(species)) throw new Error('Choose human, mouse, or no evaluation.');
  const d = project.datasets.find((d) => d.id === id);
  if (!d || invariantSpecies(d) === species) return project;
  return touch({ ...project, datasets: project.datasets.map((d) => d.id === id ? { ...d, invariantSpecies: species } : d), history: [...project.history, { time: now, action: 'Chose invariant-population species', detail: `${d.name}: ${species || 'not evaluated'}` }] }, now);
}
