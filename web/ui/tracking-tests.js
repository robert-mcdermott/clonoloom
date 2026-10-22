// The Tracking view's Tests part (clonoloom-spec/requirements.md, T2; roadmap.md, wave 3, slice 6):
// each clonotype tested between two time points of each subject by the program (api_expansion.go,
// internal/expansion), the false discovery rate controlled within each subject; the clonotypes at
// their two frequencies, those called increased or decreased marked; and the calls in a table,
// linked to the plot, to the Clonotypes view, and to the Clonotypes part, which follows them.

import { h, icon } from './dom.js';
import { changes, percent } from '../lib/plot.js';
import { UNITS } from '../lib/datasets.js';
import { ALPHAS, ALTERNATIVES, CORRECTIONS, TESTS, comparisonWords } from '../lib/tracking.js';
import { exportable, madeBy } from './figures.js';

const count = (n) => Number(n ?? 0).toLocaleString('en-US');
const plural = (n, word, words = `${word}s`) => `${count(n)} ${n === 1 ? word : words}`;
const pValue = (p) => (!Number.isFinite(p) ? '—' : p === 0 ? '< 10⁻³⁰⁰' : p >= 0.001 ? p.toFixed(3) : p.toExponential(1));
const junctionsOf = (c) => c.chains.map((ch) => ch.junction).join('; ') || c.id;
const genesOf = (c) => c.chains.map((ch) => [ch.v, ch.j].filter(Boolean).join(' ')).filter(Boolean).join('; ');

// The calls' colors, as the plot and the table show them.
const CALLS = {
  increase: { label: 'increased', className: 'plot-s1' },
  decrease: { label: 'decreased', className: 'plot-s4' },
};
// The most points the plot draws that were not called (the largest), and the most calls listed.
const MAX_DRAWN = 4000;
const MAX_LISTED = 200;

// What the part says before its result is there.
export const TESTS_WORDS = 'The program tests each clonotype between the two time points in each subject, the samples at one time pooled: by default the binomial test of its count at the first given its total in the two (Rytlewski et al. 2019; Fisher\'s exact test, as DeWitt et al. 2015 used it, is offered), two-sided, the false discovery rate controlled within each subject by Benjamini and Hochberg\'s procedure at 0.01. Clonotypes with fewer than 5 cells or molecules in the two together are not tested. The tests account for sampling alone: two blood samples taken at one time differ by more. The result is kept with the project.';

// ctx: { app, body, d, points (timePoints), comparison, state (the result: idle | looking | absent
// | running | ready | failed | unavailable), spec, ui ({ subject, selected }), compareWith(changes),
// run(), followCalls(), showInClonotypes(id), select(key), render() }.
export function renderTests(ctx) {
  const { app, body, d, points, comparison: c, state, ui } = ctx;
  const result = state.status === 'ready' ? state.result : null;
  const subjects = result ? result.subjects : [];
  const pick = (value, options, label, onPick) => h('select.input.small', { 'aria-label': label, onchange: (e) => onPick(e.target.value) },
    ...options.map(([v, text]) => h('option', { value: v, selected: String(v) === String(value) }, text)));
  const total = Math.max(0, ...points.map((p) => p.subjects));
  const pointOptions = points.map((p) => [p.key, `${p.label}${p.subjects < total ? ` (${plural(p.subjects, 'subject')})` : ''}`]);
  body.append(h('div.repertoire-controls',
    h('label.kv-inline', h('span.muted', 'From'), pick(c.from, pointOptions, 'From', (v) => ctx.compareWith({ from: v }))),
    h('label.kv-inline', h('span.muted', 'to'), pick(c.to, pointOptions, 'To', (v) => ctx.compareWith({ to: v }))),
    pick(c.test, Object.entries(TESTS).map(([k, t]) => [k, t.label]), 'Test', (v) => ctx.compareWith({ test: v })),
    pick(c.alternative, Object.entries(ALTERNATIVES).map(([k, a]) => [k, a.label]), 'Direction', (v) => ctx.compareWith({ alternative: v })),
    h('label.kv-inline', h('span.muted', 'False discoveries'), pick(c.correction, Object.entries(CORRECTIONS).map(([k, x]) => [k, x.label]), 'Correction', (v) => ctx.compareWith({ correction: v })),
      pick(c.alpha, ALPHAS.map((a) => [a, `at ${a}`]), 'False discovery rate', (v) => ctx.compareWith({ alpha: Number(v) }))),
    subjects.length ? h('label.kv-inline', h('span.muted', 'Subject'), pick(ui.subject, subjects.map((s) => [s.subject, s.subject]), 'Subject', (v) => { ui.subject = v; ui.selected = null; ctx.render(); })) : null));
  if (ctx.spec) {
    body.append(h('p.muted.small.repertoire-spec', 'Clonotypes: ', h('b', ctx.spec.definition.name ?? 'the definition'), ctx.spec.filterRecord ? ` · filter "${ctx.spec.filterRecord.name}" v${ctx.spec.filterRecord.version}` : ' · no filter'));
  }
  if (!c.from || !c.to || state.status === 'unavailable') {
    body.append(h('div.callout', h('p', 'The subjects need samples at two time points to be compared: give the samples their time points in the Data view\'s samples table.')));
    return;
  }
  if (state.status === 'looking' || state.status === 'idle') {
    body.append(h('p.muted', 'Reading…'));
    return;
  }
  if (state.status === 'failed') {
    body.append(h('div.callout.danger', h('p', state.error)));
    return;
  }
  if (state.status === 'absent' || state.status === 'running') {
    const job = state.job ? app.jobs.get(state.job) : null;
    body.append(h('div.card.repertoire-start',
      h('h3', 'Clonotypes that changed between two time points'),
      h('p', TESTS_WORDS),
      state.status === 'running'
        ? h('div.example-progress', h('div.progress', h('div', { style: { width: `${Math.round((job?.progress ?? 0) * 100)}%` } })), h('span.muted.small', job?.message ?? 'Waiting'))
        : h('div.btn-row', h('button.btn.primary', { type: 'button', onclick: () => ctx.run() }, icon('play'), 'Test the clonotypes'))));
    return;
  }
  body.append(h('div.callout.warn', h('p', result.caveat)));
  if (result.without?.length) body.append(h('p.muted.small', `Not tested, without samples at both time points: ${result.without.join(', ')}.`));
  const s = subjects.find((x) => x.subject === ui.subject) ?? subjects[0];
  if (!s) {
    body.append(h('div.callout', h('p', 'No subject has samples at both time points.')));
    return;
  }
  renderSubject(ctx, result, s);
}

function renderSubject(ctx, result, s) {
  const { app, body, d, ui } = ctx;
  const c = result.comparison;
  const unit = UNITS[result.unit] ?? 'counts';
  const from = s.from.label;
  const to = s.to.label;
  const pooled = (p) => (p.samples.length > 1 ? `${p.samples.length} samples, ` : '');
  body.append(h('p.tracking-summary',
    `${s.subject}: ${from} (${pooled(s.from)}${count(s.totalA)} ${unit}) against ${to} (${pooled(s.to)}${count(s.totalB)} ${unit}). ${plural(s.tested, 'clonotype')} tested, ${count(s.leftOut)} with fewer than ${c.minTotal} in the two left out. `,
    h('b', `${plural(s.increased, 'clonotype')} increased and ${count(s.decreased)} decreased`),
    ` at a false discovery rate of ${c.alpha} (${CORRECTIONS[c.correction]?.label ?? c.correction}${Number.isFinite(s.pi0) ? `, π₀ ${s.pi0.toFixed(3)}` : ''}).`));
  // The plot: the calls drawn over the rest; of those not called, the largest.
  const called = s.clonotypes.filter((x) => x.call);
  const rest = s.clonotypes.filter((x) => !x.call).sort((a, b) => b.a + b.b - (a.a + a.b)).slice(0, MAX_DRAWN);
  const point = (x) => ({ key: x.id, label: junctionsOf(x), x: x.freqA, y: x.freqB, className: x.call ? CALLS[x.call].className : 'plot-dim', kind: x.call ? CALLS[x.call].label : 'not called', r: x.call ? 3 : 2.2 });
  const selected = s.clonotypes.some((x) => x.id === ui.selected) ? ui.selected : null;
  const plot = changes([...rest.map(point), ...called.map(point)], { label: `Clonotypes of ${s.subject}, ${from} against ${to}`, xLabel: `Frequency at ${from} (logarithmic)`, yLabel: `Frequency at ${to} (logarithmic)`, selected });
  const legend = [{ label: 'increased', className: 'plot-s1' }, { label: 'decreased', className: 'plot-s4' }, { label: 'not called', className: 'plot-dim' }];
  const drawnNote = rest.length < s.tested - called.length ? ` Of the ${count(s.tested - called.length)} not called, the ${count(rest.length)} largest are drawn.` : '';
  const note = `Each clonotype tested at its frequency at ${from} and at ${to}, on logarithmic axes, the dashed line where they are equal; a clonotype not found at one of them on that axis's lane. Those called at a false discovery rate of ${c.alpha} are colored.${drawnNote}`;
  const pickKey = (event) => {
    const key = event.target.closest?.('[data-key]')?.getAttribute('data-key');
    if (key) ctx.select(key);
  };
  const chosen = s.clonotypes.find((x) => x.id === selected);
  body.append(exportable(h('figure.plot-card.tracking-plot',
    h('figcaption', h('span', 'Each clonotype at the two time points'), h('span.plot-pickers', h('span.muted.small', chosen ? `${junctionsOf(chosen)}: p ${pValue(chosen.p)}, adjusted ${pValue(chosen.q)}` : 'Choose a clonotype on the plot or in the table'))),
    h('div.plot-holder', { html: plot.svg, onclick: pickKey }),
    h('div.plot-legend', ...legend.map((l) => h('span', h(`span.swatch.${l.className}`), l.label))),
    h('p.muted.small', note)), () => ({
    name: `${d.name} ${s.subject} ${from} to ${to}`, title: `Clonotypes of ${s.subject}, ${from} against ${to}, ${d.name}`, plot: plot.svg, legend,
    caption: [note, `${d.name}, subject ${s.subject}: ${comparisonWords(c)}; ${plural(s.tested, 'clonotype')} tested, ${count(s.increased)} increased, ${count(s.decreased)} decreased. Clonotypes: ${result.describe}. Abundance in ${unit}. ${result.caveat}`, madeBy(app.version)],
  })));
  body.append(callsTable(ctx, result, s, called));
  body.append(h('p.muted.small', `Tested ${new Date(ctx.state.manifest.created).toLocaleString()} by ClonoLoom ${ctx.state.manifest.lineage?.software?.[0]?.version ?? ''}: ${comparisonWords(c)}; kept as ${ctx.state.artifact}. Clonotypes: ${result.describe}.`));
}

// The calls, the most significant first: choosing one shows it on the plot.
function callsTable(ctx, result, s, called) {
  const { ui } = ctx;
  const unit = UNITS[result.unit] ?? 'counts';
  const listed = called.slice(0, MAX_LISTED);
  const rows = listed.map((x) => h(`tr${x.id === ui.selected ? '.selected' : ''}`, {
    tabIndex: 0, 'aria-selected': x.id === ui.selected ? 'true' : 'false',
    onclick: () => ctx.select(x.id),
    onkeydown: (e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); ctx.select(x.id); } },
  },
  h('td', h(`span.swatch.${CALLS[x.call].className}`, { 'aria-hidden': 'true' })),
  h('td', h('div.mono', junctionsOf(x)), h('div.muted.small', genesOf(x) || x.id)),
  h('td.r', count(x.a), h('span.muted.small.tracking-interval', percent(x.freqA))),
  h('td.r', count(x.b), h('span.muted.small.tracking-interval', percent(x.freqB))),
  h('td.r', Number.isFinite(x.log2Fold) ? x.log2Fold.toFixed(2) : x.a === 0 ? 'new' : 'gone'),
  h('td.r', pValue(x.p)),
  h('td.r', pValue(x.q)),
  h('td', h(`span.badge${x.call === 'increase' ? '.accent' : ''}`, CALLS[x.call].label)),
  h('td', h('button.icon-button', { type: 'button', title: 'Show in the Clonotypes view', 'aria-label': `Show ${junctionsOf(x)} in the Clonotypes view`, onclick: (e) => { e.stopPropagation(); ctx.showInClonotypes(x.id); } }, icon('clonotypes')))));
  const followButton = h('button.btn.small', { type: 'button', title: 'Follow the clonotypes called through every time point, in the Clonotypes part', onclick: () => ctx.followCalls() }, icon('threads'), 'Follow the calls');
  return h('section', { 'aria-label': 'Clonotypes called' },
    h('div.section-title.tracking-calls-title', h('span', `Called in ${s.subject} · ${count(called.length)}`), called.length ? followButton : null),
    called.length
      ? h('div.table-scroll', { tabIndex: 0 }, h('table.data.tracking-table',
        h('thead', h('tr', h('th', { 'aria-label': 'Call' }), h('th', 'Clonotype'), h('th.r', `At ${s.from.label} (${unit})`), h('th.r', `At ${s.to.label}`), h('th.r', { title: 'The log₂ of the ratio of the frequencies' }, 'log₂ fold'), h('th.r', 'p'),
          h('th.r', { title: result.comparison.correction === 'storey' ? 'Storey\'s q-value' : 'Benjamini and Hochberg\'s adjusted p-value' }, result.comparison.correction === 'storey' ? 'q' : 'adjusted p'), h('th', 'Call'), h('th', { 'aria-label': 'Actions' }))),
        h('tbody', ...rows)))
      : h('p.muted', 'No clonotype is called at this false discovery rate.'),
    called.length > MAX_LISTED ? h('p.muted.small', `The ${MAX_LISTED} most significant of ${count(called.length)}; Export writes them all.`) : null);
}
