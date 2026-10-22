// The Tracking view's Classes part (clonoloom-spec/requirements.md, T4; roadmap.md, wave 3, slice
// 8): in each subject, each clonotype tested between a base time point and every later one by the
// Tests part's test (api_classes.go, internal/track/classes.go), and classed by those calls:
// emerging, transient, contracting or persistent. Each subject's clonotypes by class; one class's
// clonotypes through the time points as threads, and in a table of their calls; and the class
// followed in the Clonotypes part on request.

import { h, icon } from './dom.js';
import { percent, stackedBars, threads } from '../lib/plot.js';
import { UNITS } from '../lib/datasets.js';
import { ALPHAS, CLASSES, CORRECTIONS, TESTS, fits } from '../lib/tracking.js';
import { exportable, madeBy } from './figures.js';
import { fitCard } from './tracking-tests.js';

const count = (n) => Number(n ?? 0).toLocaleString('en-US');
const plural = (n, word, words = `${word}s`) => `${count(n)} ${n === 1 ? word : words}`;
const pValue = (p) => (!Number.isFinite(p) ? '—' : p === 0 ? '< 10⁻³⁰⁰' : p >= 0.001 ? p.toFixed(3) : p.toExponential(1));
const junctionsOf = (c) => c.chains.map((ch) => ch.junction).join('; ') || c.id;
const genesOf = (c) => c.chains.map((ch) => [ch.v, ch.j].filter(Boolean).join(' ')).filter(Boolean).join('; ');
const ARROWS = { increase: '↑', decrease: '↓' };
// A frequency, or not found: never zero.
const frequency = (f) => (f > 0 ? percent(f) : 'not found');
// The most clonotypes of a class drawn and listed.
const MAX_SHOWN = 30;

// What the part says before its result is there.
export const CLASSES_WORDS = 'The program tests each clonotype of each subject between the base time point and every later one, by the Tests part\'s test (the beta-binomial test where its variance can be fitted), the false discovery rate controlled within each comparison, and classes it by its calls: emerging (called increased at the last time point), transient (called changed at a time point between, not at the last), contracting (called decreased at the last time point) or persistent (never called, and found at every time point). The result is kept with the project.';

// ctx: { app, body, d, points, comparison, state, spec, ui ({ subject, selected, class }),
// compareWith(changes), run(), followClass(class), showInClonotypes(id), select(key), render() }.
export function renderClasses(ctx) {
  const { app, body, points, comparison: c, state, ui } = ctx;
  const result = state.status === 'ready' ? state.result : null;
  const subjects = result ? result.subjects : [];
  const pick = (value, options, label, onPick) => h('select.input.small', { 'aria-label': label, onchange: (e) => onPick(e.target.value) },
    ...options.map(([v, text]) => h('option', { value: v, selected: String(v) === String(value) }, text)));
  const pointOptions = points.filter((p) => p.key !== 'reference').map((p) => [p.key, p.label]);
  body.append(h('div.repertoire-controls',
    h('label.kv-inline', h('span.muted', 'Against'), pick(c.base, pointOptions, 'Base time point', (v) => ctx.compareWith({ base: v }))),
    pick(c.test, Object.entries(TESTS).map(([k, t]) => [k, t.label]), 'Test', (v) => ctx.compareWith({ test: v })),
    h('label.kv-inline', h('span.muted', 'False discoveries'), pick(c.correction, Object.entries(CORRECTIONS).map(([k, x]) => [k, x.label]), 'Correction', (v) => ctx.compareWith({ correction: v })),
      pick(c.alpha, ALPHAS.map((a) => [a, `at ${a}`]), 'False discovery rate', (v) => ctx.compareWith({ alpha: Number(v) }))),
    subjects.length ? h('label.kv-inline', h('span.muted', 'Subject'), pick(ui.subject, subjects.map((s) => [s.subject, s.subject]), 'Subject', (v) => { ui.subject = v; ui.selected = null; ctx.render(); })) : null));
  if (fits(c)) body.append(h('p.muted.small', `The variance is fitted as the Tests part says: on ${c.stable ? `${c.stable.from} and ${c.stable.to}, named stable` : 'the replicate samples'}.`));
  if (!c.base || state.status === 'unavailable') {
    body.append(h('div.callout', h('p', 'The subjects need samples at two time points or more, besides a reference sample, to be classed: give the samples their time points in the Data view\'s samples table.')));
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
      h('h3', 'Clonotypes classed across time points'),
      h('p', CLASSES_WORDS),
      state.status === 'running'
        ? h('div.example-progress', h('div.progress', h('div', { style: { width: `${Math.round((job?.progress ?? 0) * 100)}%` } })), h('span.muted.small', job?.message ?? 'Waiting'))
        : h('div.btn-row', h('button.btn.primary', { type: 'button', onclick: () => ctx.run() }, icon('play'), 'Class the clonotypes'))));
    return;
  }
  body.append(h(`div.callout${result.test === 'betabinomial' ? '' : '.warn'}`, h('p', result.caveat)));
  if (result.without?.length) body.append(h('p.muted.small', `Not classed, without samples at ${result.classing.base} and a later time point: ${result.without.join(', ')}.`));
  if (!subjects.length) {
    body.append(h('div.callout', h('p', `No subject has samples at ${result.classing.base} and a later time point.`)));
    return;
  }
  body.append(countsCard(ctx, result));
  if (result.fit) body.append(fitCard(ctx, result.fit, { stable: result.classing.stable, minTotal: result.classing.minTotal, describe: result.describe }));
  const s = subjects.find((x) => x.subject === ui.subject) ?? subjects[0];
  renderSubject(ctx, result, s);
}

// Each subject's clonotypes by class: those called, stacked; and every class in a table.
function countsCard(ctx, result) {
  const { app, d } = ctx;
  const keys = ['emerging', 'transient', 'contracting'].map((k) => ({ key: k, label: CLASSES[k].label.toLowerCase(), className: CLASSES[k].className }));
  const plot = stackedBars(result.subjects.map((s) => ({ label: s.subject, parts: s.counts })), keys, { label: 'Clonotypes classed by their calls, by subject', width: 560, labelWidth: 80 });
  const note = `Each subject's clonotypes called changed against ${result.classing.base}, by class. Persistent clonotypes (never called, and found at every time point) are counted in the table.`;
  const table = h('table.data',
    h('thead', h('tr', h('th', 'Subject'), h('th', 'Against'), h('th', 'Later time points'), ...['emerging', 'transient', 'contracting', 'persistent'].map((k) => h('th.r', { title: CLASSES[k].words }, CLASSES[k].label)), h('th.r', { title: 'Never called, and missing at a time point or more' }, 'Not classed'))),
    h('tbody', ...result.subjects.map((s) => h('tr', h('td', h('b', s.subject)), h('td', s.base.label), h('td.muted', s.later.map((p) => p.label).join(', ')),
      ...['emerging', 'transient', 'contracting', 'persistent', 'unclassed'].map((k) => h('td.r', count(s.counts[k])))))));
  return exportable(h('figure.plot-card',
    h('figcaption', 'Clonotypes by class'),
    h('div.plot-holder', { html: plot.svg }),
    h('div.plot-legend', ...keys.map((k) => h('span', h(`span.swatch.${k.className}`), k.label))),
    h('div.table-scroll', { tabIndex: 0 }, table),
    h('p.muted.small', note)), () => ({
    name: `${d.name} classes`, title: `Clonotypes by class, ${d.name}`, plot: plot.svg, legend: keys,
    caption: [note, `${d.name}: classed by ${TESTS[result.test]?.name ?? result.test} against ${result.classing.base}. Clonotypes: ${result.describe}.`, madeBy(app.version)],
  }));
}

// One subject's clonotypes of the class chosen: threads, and a table of their calls.
function renderSubject(ctx, result, s) {
  const { app, body, d, ui } = ctx;
  const cls = CLASSES[ui.class] ? ui.class : 'emerging';
  const of = s.clonotypes.filter((x) => x.class === cls);
  const shown = of.slice(0, MAX_SHOWN);
  const points = [s.base, ...s.later].map((p) => ({ label: p.label, title: `${p.label}${p.samples.length > 1 ? `, ${p.samples.length} samples pooled` : ''}` }));
  const classPicker = h('div.segmented', { role: 'tablist', 'aria-label': 'Class' }, ...Object.entries(CLASSES).map(([k, x]) => h(`button${k === cls ? '.active' : ''}`, {
    type: 'button', role: 'tab', 'aria-selected': k === cls ? 'true' : 'false', title: x.words, onclick: () => { ui.class = k; ui.selected = null; ctx.render(); },
  }, `${x.label} · ${count(s.counts[k])}`)));
  body.append(h('div.tracking-class-head', h('b', s.subject), classPicker,
    shown.length ? h('button.btn.small', { type: 'button', title: `Follow ${s.subject}'s ${CLASSES[cls].label.toLowerCase()} clonotypes, and every subject's, through every time point in the Clonotypes part`, onclick: () => ctx.followClass(cls) }, icon('threads'), 'Follow this class') : null));
  if (!shown.length) {
    body.append(h('p.muted', `No clonotype of ${s.subject} is ${cls}.`));
    return;
  }
  const selected = shown.some((x) => x.id === ui.selected) ? ui.selected : null;
  const series = shown.map((x, i) => ({ key: x.id, label: junctionsOf(x), className: `plot-s${(i % 6) + 1}`, values: x.frequencies.map((f) => ({ y: f, absent: f === 0 })) }));
  const plot = threads(points, series, { label: `${CLASSES[cls].label} clonotypes of ${s.subject}`, selected });
  const pickKey = (event) => {
    const key = event.target.closest?.('[data-key]')?.getAttribute('data-key');
    if (key) ctx.select(key);
  };
  const note = `${of.length > shown.length ? `The ${shown.length} ${cls === 'persistent' ? 'largest' : 'most significant'} of ${count(of.length)} ${cls} clonotypes` : `The ${count(of.length)} ${cls} clonotype${of.length === 1 ? '' : 's'}`} of ${s.subject} (${CLASSES[cls].words}), at ${s.base.label} and every later time point (equally spaced, in time order; the samples at one time pooled), on a logarithmic axis.`;
  body.append(exportable(h('figure.plot-card.tracking-plot',
    h('figcaption', h('span', `${CLASSES[cls].label} clonotypes`), h('span.plot-pickers', h('span.muted.small', selected ? junctionsOf(shown.find((x) => x.id === selected)) : 'Choose a clonotype on the plot or in the table'))),
    h('div.plot-holder', { html: plot.svg, onclick: pickKey }),
    h('p.muted.small', note)), () => ({
    name: `${d.name} ${s.subject} ${cls}`, title: `${CLASSES[cls].label} clonotypes of ${s.subject}, ${d.name}`, plot: plot.svg,
    legend: shown.slice(0, 12).map((x, i) => ({ label: junctionsOf(x), className: `plot-s${(i % 6) + 1}` })),
    caption: [note, `${d.name}: classed by ${TESTS[result.test]?.name ?? result.test} against ${result.classing.base}. Clonotypes: ${result.describe}. Abundance in ${UNITS[result.unit] ?? 'counts'}.`, madeBy(app.version)],
  })));
  const rows = shown.map((x, i) => h(`tr${x.id === ui.selected ? '.selected' : ''}`, {
    tabIndex: 0, 'aria-selected': x.id === ui.selected ? 'true' : 'false',
    onclick: () => ctx.select(x.id),
    onkeydown: (e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); ctx.select(x.id); } },
  },
  h('td', h(`span.swatch.plot-s${(i % 6) + 1}`, { 'aria-hidden': 'true' })),
  h('td', h('div.mono', junctionsOf(x)), h('div.muted.small', genesOf(x) || x.id)),
  h(`td.r${x.frequencies[0] === 0 ? '.muted' : ''}`, frequency(x.frequencies[0])),
  ...s.later.map((p, k) => h(`td.r${x.frequencies[k + 1] === 0 ? '.muted' : ''}`, { title: x.calls[k] ? `Called ${x.calls[k]}d against ${s.base.label}` : 'Not called' }, frequency(x.frequencies[k + 1]), x.calls[k] ? h('span.tracking-call', ` ${ARROWS[x.calls[k]]}`) : null)),
  h('td.r', cls === 'persistent' ? '—' : pValue(x.q)),
  h('td', h('button.icon-button', { type: 'button', title: 'Show in the Clonotypes view', 'aria-label': `Show ${junctionsOf(x)} in the Clonotypes view`, onclick: (e) => { e.stopPropagation(); ctx.showInClonotypes(x.id); } }, icon('clonotypes')))));
  body.append(h('section', { 'aria-label': `${CLASSES[cls].label} clonotypes` },
    h('div.table-scroll', { tabIndex: 0 }, h('table.data.tracking-table',
      h('thead', h('tr', h('th', { 'aria-label': 'Color' }), h('th', 'Clonotype'), h('th.r', s.base.label), ...s.later.map((p) => h('th.r', p.label)), h('th.r', { title: 'The least adjusted p-value of its calls' }, 'adjusted p'), h('th', { 'aria-label': 'Actions' }))),
      h('tbody', ...rows))),
    h('p.muted.small', `Frequencies at each time point; ↑ and ↓ mark the calls against ${s.base.label}.`)));
  body.append(h('p.muted.small', `Classed ${new Date(ctx.state.manifest.created).toLocaleString()} by ClonoLoom ${ctx.state.manifest.lineage?.software?.[0]?.version ?? ''}; kept as ${ctx.state.artifact}. Clonotypes: ${result.describe}.`));
}
