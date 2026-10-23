// Network choices travel with the dataset and its immutable results. All graph
// computation and layout are performed by Go; this module only describes choices.
import { touch } from './project.js';

export const METRICS = { identity: 'Identity', hamming: 'Hamming', levenshtein: 'Levenshtein', tcrdist: 'TCRdist' };
export function networkOptions(dataset) {
  const o = dataset?.networks ?? {};
  const metric = METRICS[o.metric] ? o.metric : 'identity';
  const max = metric === 'identity' ? 0 : metric === 'tcrdist' ? 1000 : 2;
  return { metric, cutoff: Math.max(0, Math.min(max, Number.isInteger(o.cutoff) ? o.cutoff : 0)), arms: o.arms === 'any' ? 'any' : 'all', sameV: Boolean(o.sameV), sameJ: Boolean(o.sameJ), cdr3Only: Boolean(o.cdr3Only), organism: o.organism ?? '', drawNodes: 1000 };
}
export function networkWords(o) {
  return `${METRICS[o.metric]} at cutoff ${o.cutoff}, ${o.arms} retained chains${o.sameV ? ', same V gene' : ''}${o.sameJ ? ', same J gene' : ''}${o.metric === 'tcrdist' ? (o.cdr3Only ? ', CDR3s alone' : ', with V-gene loops') : ', whole junctions'}`;
}
export function useNetworks(project, id, changes, now = new Date().toISOString()) {
  const d = project.datasets.find((x) => x.id === id);
  if (!d) return project;
  const before = networkOptions(d);
  const networks = networkOptions({ networks: { ...before, ...changes } });
  if (JSON.stringify(before) === JSON.stringify(networks)) return project;
  return touch({ ...project, datasets: project.datasets.map((x) => x.id === id ? { ...x, networks } : x), history: [...project.history, { time: now, action: 'Chose a clonotype network', detail: `${d.name}: ${networkWords(networks)}` }] }, now);
}
