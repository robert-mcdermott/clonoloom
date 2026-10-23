// Database choices and provenance remain with the dataset and immutable results.
import { touch } from './project.js';
export function databaseOptions(dataset) {
  const o = dataset?.specificity ?? {};
  return { database: o.database === 'simulated-vaccine' ? o.database : 'vdjdb', metric: o.metric === 'tcrdist' ? 'tcrdist' : 'exact', genes: Boolean(o.genes), radius: Number.isInteger(o.radius) ? Math.max(0, Math.min(1000, o.radius)) : 24, species: ['HomoSapiens', 'MusMusculus', 'MacacaMulatta'].includes(o.species) ? o.species : 'HomoSapiens', minScore: Number.isInteger(o.minScore) ? Math.max(0, Math.min(3, o.minScore)) : 0, cdr3Only: Boolean(o.cdr3Only) };
}
export function databaseWords(o) {
  return `${o.database}, ${o.metric === 'exact' ? 'exact CDR3' : `TCRdist ≤ ${o.radius}${o.cdr3Only ? ', CDR3s alone' : ', with V-gene loops'}`}${o.genes ? ', matching V/J calls' : ''}, ${o.species}, score ≥ ${o.minScore}`;
}
export function useDatabase(project, id, changes, now = new Date().toISOString()) {
  const d = project.datasets.find((d) => d.id === id);
  if (!d) return project;
  const before = databaseOptions(d), specificity = databaseOptions({ specificity: { ...before, ...changes } });
  if (JSON.stringify(before) === JSON.stringify(specificity)) return project;
  return touch({ ...project, datasets: project.datasets.map((d) => d.id === id ? { ...d, specificity } : d), history: [...project.history, { time: now, action: 'Chose specificity matching', detail: `${d.name}: ${databaseWords(specificity)}` }] }, now);
}
