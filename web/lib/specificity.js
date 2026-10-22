// The Specificity view's choices and their words (clonoloom-spec/requirements.md, S1; roadmap.md,
// wave 4): how TCRdist compares a dataset's clonotypes (the radius within which two are
// neighbors, the CDR3s alone or with the V genes' loops, the organism whose loops are used), kept
// with the dataset; the request the program is sent (api_tcrdist.go); and the distribution of
// each clonotype's distance to its nearest. Every edit returns a new project.

import { touch } from './project.js';

// TCRdist's default radius: 24 a chain (two CDR3 residues 4 apart, weighted 3).
export const RADIUS_PER_CHAIN = 24;

// The organisms tcrdist3's loops are known for ('' guessed from the V genes).
export const ORGANISMS = { '': 'From the V genes', human: 'Human', mouse: 'Mouse', rhesus: 'Rhesus macaque' };

// A dataset's TCRdist options: { radius (0: the default), cdr3Only, organism }.
export function tcrdistOptions(dataset) {
  const o = dataset?.tcrdist ?? {};
  return { radius: Number.isInteger(o.radius) && o.radius > 0 ? o.radius : 0, cdr3Only: Boolean(o.cdr3Only), organism: ORGANISMS[o.organism] ? o.organism ?? '' : '' };
}

// The radius a definition's clonotypes are compared within: the option's, else 24 a chain.
export function radiusOf(options, definition) {
  return options.radius > 0 ? options.radius : RADIUS_PER_CHAIN * Math.max(1, definition?.chains?.length ?? 0);
}

// Keeps a dataset's TCRdist options, with the change in the project's history.
export function useTCRdist(project, datasetId, changes, now = new Date().toISOString()) {
  const dataset = project.datasets.find((d) => d.id === datasetId);
  if (!dataset) return project;
  const before = tcrdistOptions(dataset);
  const next = tcrdistOptions({ tcrdist: { ...before, ...changes } });
  if (JSON.stringify(next) === JSON.stringify(before)) return project;
  const tcrdist = { ...(next.radius ? { radius: next.radius } : {}), ...(next.cdr3Only ? { cdr3Only: true } : {}), ...(next.organism ? { organism: next.organism } : {}) };
  return touch({
    ...project,
    datasets: project.datasets.map((d) => (d.id === datasetId ? { ...d, tcrdist } : d)),
    history: [...project.history, { time: now, action: 'Chose how TCRdist compares the clonotypes', detail: `${dataset.name}: ${optionWords(next)}` }],
  }, now);
}

// The options in words.
export function optionWords(o, definition = null) {
  const radius = o.radius > 0 ? o.radius : definition ? radiusOf(o, definition) : `${RADIUS_PER_CHAIN} a chain`;
  const what = o.cdr3Only ? 'the CDR3s alone' : `the CDR3s and the V genes' loops${o.organism ? ` (${ORGANISMS[o.organism].toLowerCase()})` : ''}`;
  return `neighbors within ${radius} TCRdist units, comparing ${what}`;
}

// The request for a dataset's TCRdist analysis (with its definition and filter as the other
// analyses send them).
export function tcrdistRequest(options) {
  return { radius: options.radius, cdr3Only: options.cdr3Only, organism: options.organism };
}

// Each compared clonotype's distance to its nearest, in bins of width units: [{ x: the bin's
// first distance, y: clonotypes }], from 0 to the largest distance seen (clonotypes without
// another of their chains left out).
export function nearestHistogram(rows, width = 6) {
  const distances = rows.filter((r) => r.nearest).map((r) => r.distance);
  if (!distances.length) return [];
  const top = Math.max(...distances);
  const bins = Array.from({ length: Math.floor(top / width) + 1 }, (_, i) => ({ x: String(i * width), y: 0 }));
  for (const d of distances) bins[Math.floor(d / width)].y += 1;
  return bins;
}

// Two clonotypes' distance loop by loop, in words: for each chain, the loops that differ and by
// how much ("TRA alike · TRB: CDR3 9"). regions: each chain's [CDR1, CDR2, CDR2.5, CDR3].
export const LOOP_NAMES = ['CDR1', 'CDR2', 'CDR2.5', 'CDR3'];
export function regionWords(regions, loci) {
  return regions.map((r, k) => {
    const differ = r.map((v, i) => [LOOP_NAMES[i], v]).filter(([, v]) => v > 0);
    return `${loci[k] ?? ''}${differ.length ? `: ${differ.map(([name, v]) => `${name} ${v}`).join(', ')}` : ' alike'}`;
  }).join(' · ');
}

// A clonotype's junctions and genes, in words.
export const junctionsOf = (c) => c.chains.map((ch) => ch.junction).join('; ');
export const genesOf = (c) => c.chains.map((ch) => [ch.v, ch.j].filter(Boolean).join(' ')).filter(Boolean).join('; ');
