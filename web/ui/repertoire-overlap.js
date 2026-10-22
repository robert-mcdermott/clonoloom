// The Repertoire view's Overlap part (clonoloom-spec/requirements.md, R4), from one overlap result
// (api_overlap.go): every pair of samples as a heatmap of one measure; the pairs of one subject and
// of two, of one condition and of two, summarized; every pair listed with its interval; and the
// public clonotypes, by how many subjects hold them. The program computes every number; this draws.

import { h } from './dom.js';
import { heatmap, intervals } from '../lib/plot.js';

export const OVERLAP_WORDS = 'Measured by the program for every pair of samples under this definition and filter: the clonotypes they share; Jaccard\'s, Sørensen–Dice\'s and the overlap coefficient over their clonotypes, as sampled and with both samples rarefied to the smallest sample\'s size; Morisita\'s index and the Morisita–Horn index over their abundances; each with a 95% interval. And the public clonotypes: those found in more than one subject. The result is kept with the project.';

// The measures, as the part offers them: what each is, in words.
export const OVERLAP_MEASURES = [
  { id: 'morisita', label: 'Morisita\'s index (abundance)', words: 'How much the two repertoires overlap, each clonotype weighted by its abundance: 1 for one repertoire sampled twice, 0 for repertoires sharing nothing. Morisita\'s own form (1959) is nearly unbiased at any depth, and can exceed 1 a little by chance.' },
  { id: 'morisitaHorn', label: 'Morisita–Horn index (abundance)', words: 'Horn\'s form (1966), as immunarch and vegan compute it. It is biased low where samples are shallow: in ClonoLoom\'s simulations, two samples of one repertoire read about 0.98 and their intervals rarely held 1. Prefer Morisita\'s.' },
  { id: 'jaccardRarefied', label: 'Jaccard index, rarefied', words: 'The share of the clonotypes either sample holds that both hold, as expected with both samples rarefied to the same size, so that depth does not decide it.' },
  { id: 'jaccard', label: 'Jaccard index, as sampled', words: 'The share of the clonotypes either sample holds that both hold. Deeper samples share more: compare samples of different depths rarefied.' },
  { id: 'sorensenRarefied', label: 'Sørensen–Dice index, rarefied', words: 'Twice the clonotypes shared over the clonotypes of both, as expected with both samples rarefied to the same size.' },
  { id: 'sorensen', label: 'Sørensen–Dice index, as sampled', words: 'Twice the clonotypes shared over the clonotypes of both.' },
  { id: 'overlapRarefied', label: 'Overlap coefficient, rarefied', words: 'The clonotypes shared over the clonotypes of the sample with fewer, as expected with both samples rarefied to the same size.' },
  { id: 'overlap', label: 'Overlap coefficient, as sampled', words: 'The clonotypes shared over the clonotypes of the sample with fewer.' },
  { id: 'sharedRarefied', label: 'Clonotypes shared, rarefied', words: 'The clonotypes both samples hold, as expected with both rarefied to the same size.' },
  { id: 'shared', label: 'Clonotypes shared, as sampled', words: 'The clonotypes both samples hold.' },
  { id: 'abundance', label: 'Abundance in clonotypes shared', words: 'The share of the row sample\'s abundance in clonotypes the column sample holds too (not symmetric): how much of one repertoire is found in the other.' },
];
const COUNTS = new Set(['shared', 'sharedRarefied']);

const fmtValue = (v, id) => {
  if (!Number.isFinite(v)) return '—';
  if (COUNTS.has(id)) return v >= 100 ? Math.round(v).toLocaleString('en-US') : v.toPrecision(3);
  if (v === 0) return '0';
  return v < 0.001 ? v.toExponential(1) : v.toFixed(3);
};

function picker(label, options, value, set) {
  return h('select.input.small', { 'aria-label': label, onchange: (e) => set(e.target.value) },
    ...options.map(([v, text]) => h('option', { value: v, selected: v === value }, text)));
}

function card(title, controls, plot, ...notes) {
  return h('figure.plot-card', h('figcaption', h('span', title), h('span.plot-pickers', ...controls)),
    plot ? h('div.plot-holder', { html: plot.svg }) : null, ...notes);
}

export function renderOverlap({ body, state, ui, render, colors }) {
  const result = state.result;
  const samples = result.samples;
  const n = samples.length;
  if (n < 2) {
    body.append(h('div.callout', h('p', 'Overlap needs two samples or more.')));
    return;
  }
  const info = OVERLAP_MEASURES.find((m) => m.id === ui.measure) ?? OVERLAP_MEASURES[0];
  ui.measure = info.id;
  const set = (key) => (v) => {
    ui[key] = v;
    render();
  };
  const unit = result.unit === 'umis' ? 'UMIs' : result.unit || 'counts';
  const depth = result.depth > 0 ? `${Math.round(result.depth).toLocaleString('en-US')} ${unit}` : null;
  // A pair's value of the measure, from row sample a's side (the abundance shared is not symmetric).
  const pairOf = new Map(result.pairs.map((p) => [`${p.a} ${p.b}`, p]));
  const valueOf = (a, b) => {
    if (a === b) return null;
    const p = pairOf.get(`${Math.min(a, b)} ${Math.max(a, b)}`);
    if (!p) return null;
    if (info.id === 'abundance') return p.measures[a < b ? 'abundanceOfA' : 'abundanceOfB'];
    return p.measures[info.id];
  };
  const values = samples.map((_, a) => samples.map((__, b) => valueOf(a, b)?.estimate ?? NaN));
  const finite = values.flat().filter(Number.isFinite);
  const plot = heatmap(values, {
    label: info.label, rowLabels: samples.map((s) => s.name), colLabels: samples.map((s) => s.name), width: 520, labelWidth: 110, cellHeight: n > 30 ? 10 : 16,
    max: finite.length ? Math.max(...finite) : 1, format: (v) => fmtValue(v, info.id),
    title: (a, b) => {
      const v = valueOf(a, b);
      return `${samples[a].name} and ${samples[b].name}: ${fmtValue(v?.estimate, info.id)}${Number.isFinite(v?.lower) ? ` (${fmtValue(v.lower, info.id)}–${fmtValue(v.upper, info.id)})` : ''}`;
    },
  });
  const rarefied = info.id.endsWith('Rarefied') && depth ? ` Rarefied to ${depth}, the smallest sample's size.` : '';
  body.append(h('div.repertoire-controls', picker('Measure', OVERLAP_MEASURES.map((m) => [m.id, m.label]), info.id, set('measure'))));

  // The kinds of pairs, summarized.
  const kinds = (result.summaries ?? []).map((s) => ({ label: `${s.kind} (${s.pairs})`, value: s.measures[info.id === 'abundance' ? 'abundanceOfA' : info.id]?.mean ?? NaN, lower: s.measures[info.id === 'abundance' ? 'abundanceOfA' : info.id]?.min ?? NaN, upper: s.measures[info.id === 'abundance' ? 'abundanceOfA' : info.id]?.max ?? NaN, className: s.kind.startsWith('same') ? 'plot-s1' : 'plot-s2' }));
  body.append(h('div.plot-grid',
    card(info.label, [], plot, h('p.muted.small', `${info.words}${rarefied} Hold the pointer over a cell for its value and 95% interval.`)),
    kinds.length
      ? card('Pairs of one subject and of two', [], intervals(kinds, { label: `${info.label} by kind of pair`, width: 520, labelWidth: 160, format: (v) => fmtValue(v, info.id) }),
        h('p.muted.small', 'The mean over the pairs of each kind, with their lowest and highest. Two samples of one subject should overlap far more than samples of two; pairs are not independent of one another, so this describes them and tests nothing.'))
      : card('Pairs of one subject and of two', [], null, h('p.muted', 'Annotate the samples\' subjects (or conditions) in the Data view to compare the pairs of one subject with those of two.'))));

  // Every pair, the most overlapping first.
  const rows = result.pairs.map((p) => ({ p, v: info.id === 'abundance' ? p.measures.abundanceOfA : p.measures[info.id] }))
    .sort((x, y) => (y.v?.estimate ?? -1) - (x.v?.estimate ?? -1)).slice(0, 200);
  const cell = (v, id) => h('td.r', fmtValue(v?.estimate, id), Number.isFinite(v?.lower) ? h('span.muted.small', ` ${fmtValue(v.lower, id)}–${fmtValue(v.upper, id)}`) : null);
  body.append(h('div.section-title', `Pairs · ${result.pairs.length}${result.pairs.length > 200 ? ' (the 200 most overlapping)' : ''}`),
    h('div.table-scroll', { tabIndex: 0 }, h('table.data.repertoire-table',
      h('thead', h('tr', h('th', 'Samples'), h('th', 'Subjects'), h('th.r', info.label), h('th.r', 'Shared'), h('th.r', 'Jaccard, rarefied'), h('th.r', 'Morisita'))),
      h('tbody', ...rows.map(({ p, v }) => h('tr',
        h('td.sample-name', `${samples[p.a].name} · ${samples[p.b].name}`),
        h('td', samples[p.a].subject && samples[p.b].subject ? (samples[p.a].subject === samples[p.b].subject ? `one (${samples[p.a].subject})` : 'two') : '—'),
        cell(v, info.id), cell(p.measures.shared, 'shared'), cell(p.measures.jaccardRarefied, 'jaccardRarefied'), cell(p.measures.morisita, 'morisita')))))),
    h('p.muted.small', `Intervals are 95%: from ${result.replicates} bootstrap replicates of the samples' cells, widened for the chance that decides which clonotypes two samples share, on the logarithmic scale.`));

  // The public clonotypes.
  const pub = result.public;
  const by = pub.by === 'subject' ? 'subjects' : 'samples';
  const incidence = (pub.incidence ?? []).filter((x) => x.clonotypes > 0);
  body.append(h('div.section-title', `Public clonotypes · found in more than one of ${pub.units} ${by}`),
    pub.by !== 'subject' ? h('div.callout', h('p', 'No sample has a subject: each sample is counted as its own subject, so that two samples of one person make their clonotypes look public. Annotate the samples\' subjects in the Data view.')) : null,
    h('p.small', incidence.map((x) => `${x.clonotypes.toLocaleString('en-US')} in ${x.units} ${x.units === 1 ? by.replace(/s$/, '') : by}`).join(' · ')),
    pub.top?.length
      ? h('div.table-scroll', { tabIndex: 0 }, h('table.data.repertoire-table',
        h('thead', h('tr', h('th', 'Clonotype'), h('th.r', by[0].toUpperCase() + by.slice(1)), h('th', 'Samples'), h('th.r', `Abundance (${unit})`))),
        h('tbody', ...pub.top.map((c) => h('tr',
          h('td', { title: c.id }, c.chains.map((ch) => [ch.locus, ch.junction, ch.v, ch.j].filter(Boolean).join(' ')).join(' / ')),
          h('td.r', String(c.units)), h('td.sample-name', { title: c.samples.join(', ') }, c.samples.join(', ')), h('td.r', Math.round(c.abundance).toLocaleString('en-US')))))))
      : h('p.muted', `No clonotype is found in more than one ${by.replace(/s$/, '')}.`));
  void colors;
}
