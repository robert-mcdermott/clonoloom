// The Specificity view's choices and their words (clonoloom-spec/requirements.md, S1 and R6;
// roadmap.md, wave 4): how TCRdist compares a dataset's clonotypes (the radius within which two are
// neighbors, the CDR3s alone or with the V genes' loops, the organism whose loops are used) and how
// their generation probabilities are computed (the models, the organism, with the V and J genes or
// not), kept with the dataset; the requests the program is sent (api_tcrdist.go, api_pgen.go);
// and the distributions shown. Every edit returns a new project.

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

// Generation probabilities' options, kept with the dataset: the models ('': OLGA's where
// downloaded, else ClonoLoom's; 'olga'; 'clonoloom'), the organism ('' guessed from the V genes)
// and whether each chain's V and J genes are part of the probability.
export const MODELS = { '': 'OLGA\'s where downloaded', olga: 'OLGA\'s default models', clonoloom: 'ClonoLoom\'s (human TRB)' };
export const PGEN_ORGANISMS = { '': 'From the V genes', human: 'Human', mouse: 'Mouse' };

export function pgenOptions(dataset) {
  const o = dataset?.pgen ?? {};
  return { models: MODELS[o.models] ? o.models ?? '' : '', organism: PGEN_ORGANISMS[o.organism] ? o.organism ?? '' : '', genes: Boolean(o.genes) };
}

export function usePgen(project, datasetId, changes, now = new Date().toISOString()) {
  const dataset = project.datasets.find((d) => d.id === datasetId);
  if (!dataset) return project;
  const before = pgenOptions(dataset);
  const next = pgenOptions({ pgen: { ...before, ...changes } });
  if (JSON.stringify(next) === JSON.stringify(before)) return project;
  const pgen = { ...(next.models ? { models: next.models } : {}), ...(next.organism ? { organism: next.organism } : {}), ...(next.genes ? { genes: true } : {}) };
  return touch({
    ...project,
    datasets: project.datasets.map((d) => (d.id === datasetId ? { ...d, pgen } : d)),
    history: [...project.history, { time: now, action: 'Chose how generation probabilities are computed', detail: `${dataset.name}: ${pgenWords(next)}` }],
  }, now);
}

// The request for a dataset's generation probabilities.
export function pgenRequest(options) {
  return { models: options.models, organism: options.organism, genes: options.genes };
}

export function pgenWords(o) {
  return `${o.genes ? 'each CDR3 with its V and J genes' : 'each CDR3'}, under ${o.models === 'clonoloom' ? 'ClonoLoom\'s model of human TRB' : o.models === 'olga' ? 'OLGA\'s default models' : 'OLGA\'s models where downloaded, else ClonoLoom\'s'}${o.organism && o.models !== 'clonoloom' ? ` (${o.organism})` : ''}`;
}

const SUPERSCRIPT = { '-': '⁻', 0: '⁰', 1: '¹', 2: '²', 3: '³', 4: '⁴', 5: '⁵', 6: '⁶', 7: '⁷', 8: '⁸', 9: '⁹' };

// A power of ten in words: 10⁻⁹ (an exponent that is not whole, as 10^-8.5).
export const tenTo = (e) => (Number.isInteger(e) ? `10${String(e).split('').map((c) => SUPERSCRIPT[c]).join('')}` : `10^${e}`);

// A probability in words: 2.3 × 10⁻⁹ (two significant digits), 0 as 0, none as —.
export function probabilityWords(p) {
  if (p == null || p < 0 || !Number.isFinite(p)) return '—';
  if (p === 0) return '0';
  const e = Math.floor(Math.log10(p));
  let m = p / 10 ** e;
  let exp = e;
  if (Number(m.toFixed(1)) >= 10) {
    m /= 10;
    exp += 1;
  }
  return `${m.toFixed(1)} × ${tenTo(exp)}`;
}

// The clonotypes' log₁₀ probabilities in bins of width: [{ x: the bin's start, y: clonotypes }],
// from the least to the largest seen (those without one, or of 0, left out).
export function log10Histogram(rows, width = 0.5) {
  const values = rows.filter((r) => r.pgen > 0).map((r) => Math.log10(r.pgen));
  if (!values.length) return [];
  const lo = Math.floor(Math.min(...values) / width) * width;
  const hi = Math.max(...values);
  const bins = Array.from({ length: Math.floor((hi - lo) / width) + 1 }, (_, i) => ({ x: (lo + i * width).toFixed(1), y: 0 }));
  for (const v of values) bins[Math.min(bins.length - 1, Math.floor((v - lo) / width))].y += 1;
  return bins;
}

// The clonotypes in the order chosen: 'abundant' (the most abundant first), 'probable' (the most
// probable first) or 'improbable' (the least first, 0 before any other); those without a
// probability last, then by abundance and ID.
export const PGEN_ORDERS = { abundant: 'Most abundant', probable: 'Most probable', improbable: 'Least probable' };
export function pgenOrder(rows, by = 'abundant') {
  const has = (r) => r.pgen >= 0;
  const tie = (a, b) => b.total - a.total || (a.id < b.id ? -1 : 1);
  return [...rows].sort((a, b) => {
    if (by === 'abundant') return tie(a, b);
    if (has(a) !== has(b)) return has(a) ? -1 : 1;
    if (!has(a)) return tie(a, b);
    return (by === 'probable' ? b.pgen - a.pgen : a.pgen - b.pgen) || tie(a, b);
  });
}

// Neighborhoods' options, kept with the dataset: the background ('model', receptors drawn from a
// recombination model with the dataset's V–J usage; 'shuffled', the dataset's own chains with
// their CDR3s moved among them), the locus whose chains are compared ('' the commonest), the
// share of the background a radius may hold, and the neighbors a centroid is reported with.
export const BACKGROUNDS = {
  model: 'Drawn from a recombination model',
  shuffled: 'The dataset\'s own CDR3s, shuffled',
};
export const CONTROL_RATES = { 1e-5: '10⁻⁵ (tcrdist3\'s)', 1e-6: '10⁻⁶ (Mayer-Blackwell et al.\'s)', 1e-7: '10⁻⁷' };

export function neighborhoodOptions(dataset) {
  const o = dataset?.neighborhoods ?? {};
  const control = Number(o.control);
  return {
    background: BACKGROUNDS[o.background] ? o.background : 'model',
    locus: typeof o.locus === 'string' ? o.locus : '',
    control: CONTROL_RATES[control] ? control : 1e-6,
    minNeighbors: Number.isInteger(o.minNeighbors) && o.minNeighbors > 0 ? o.minNeighbors : 2,
  };
}

export function useNeighborhoods(project, datasetId, changes, now = new Date().toISOString()) {
  const dataset = project.datasets.find((d) => d.id === datasetId);
  if (!dataset) return project;
  const before = neighborhoodOptions(dataset);
  const next = neighborhoodOptions({ neighborhoods: { ...before, ...changes } });
  if (JSON.stringify(next) === JSON.stringify(before)) return project;
  const neighborhoods = {
    ...(next.background !== 'model' ? { background: next.background } : {}),
    ...(next.locus ? { locus: next.locus } : {}),
    ...(next.control !== 1e-6 ? { control: next.control } : {}),
    ...(next.minNeighbors !== 2 ? { minNeighbors: next.minNeighbors } : {}),
  };
  return touch({
    ...project,
    datasets: project.datasets.map((d) => (d.id === datasetId ? { ...d, neighborhoods } : d)),
    history: [...project.history, { time: now, action: 'Chose how neighborhoods are found', detail: `${dataset.name}: ${neighborhoodWords(next)}` }],
  }, now);
}

// The request for a dataset's neighborhoods.
export function neighborhoodRequest(options) {
  return { background: options.background, locus: options.locus, control: options.control, minNeighbors: options.minNeighbors };
}

// The options in words.
export function neighborhoodWords(o) {
  const against = o.background === 'shuffled' ? 'the dataset\'s own CDR3s shuffled among its genes' : 'receptors drawn from a recombination model';
  return `${o.locus ? `${o.locus} chains` : 'the commonest locus\'s chains'} against ${against}, each radius holding at most ${tenTo(Math.round(Math.log10(o.control)))} of it`;
}

// A centroid's neighborhood in words: how much commoner it is in the dataset than in the
// background, and how much of the background its radius holds.
export function enrichmentWords(row) {
  const times = row.relativeRate >= 100 ? Math.round(row.relativeRate) : Number(row.relativeRate.toPrecision(2));
  return `${times.toLocaleString('en-US')}× commoner than in the background`;
}

// The centroids in the order chosen: 'enriched' (the most enriched first, as the program answers
// them), 'neighbors' (the largest neighborhoods first) or 'abundant' (the most cells or
// molecules first).
export const CENTER_ORDERS = { enriched: 'Most enriched', neighbors: 'Largest', abundant: 'Most abundant' };
export function centerOrder(rows, by = 'enriched') {
  const tie = (a, b) => b.chiSquare - a.chiSquare || (a.id < b.id ? -1 : 1);
  return [...rows].sort((a, b) => {
    if (by === 'neighbors') return b.neighbors - a.neighbors || tie(a, b);
    if (by === 'abundant') return b.abundance - a.abundance || tie(a, b);
    return tie(a, b);
  });
}
