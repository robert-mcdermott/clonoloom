// The Tracking view's Attribution part (clonoloom-spec/requirements.md, T5; roadmap.md, wave 3,
// slice 8): each sample's share in its subject's reference sample's clonotypes, and, where the
// reference shares few clonotypes with the samples before the reference event (a donor), the share
// derived from the reference (api_classes.go, internal/track/attribution.go), each with its
// interval; and before the event, the share that owes nothing to it.

import { h, icon } from './dom.js';
import { intervals, percent } from '../lib/plot.js';
import { exportable, madeBy } from './figures.js';

const count = (n) => Number(n ?? 0).toLocaleString('en-US');
const plural = (n, word, words = `${word}s`) => `${count(n)} ${n === 1 ? word : words}`;

// What the part says before its result is there.
export const ATTRIBUTION_WORDS = 'The program measures each sample of a subject against its reference sample (an infusion product, a donor): its share in the clonotypes the reference sample holds, and, where the subject has samples before the reference event and the reference shares few clonotypes with them (a donor, not a product made from the subject\'s own blood), the share derived from the reference, from the clonotypes found on one side only. Each with its 95% interval (Wilson\'s, for sampling). The result is kept with the project.';

// ctx: { app, body, d, state, render, run() }.
export function renderAttribution(ctx) {
  const { app, body, d, state } = ctx;
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
      h('h3', 'Each sample against its reference sample'),
      h('p', ATTRIBUTION_WORDS),
      state.status === 'running'
        ? h('div.example-progress', h('div.progress', h('div', { style: { width: `${Math.round((job?.progress ?? 0) * 100)}%` } })), h('span.muted.small', job?.message ?? 'Waiting'))
        : h('div.btn-row', h('button.btn.primary', { type: 'button', onclick: () => ctx.run() }, icon('play'), 'Attribute the samples'))));
    return;
  }
  const r = state.result;
  body.append(h('div.callout', h('p', r.caveat)));
  if (r.without?.length) body.append(h('p.muted.small', `Without a reference sample: ${r.without.join(', ')}.`));
  if (!r.subjects.length) {
    body.append(h('div.callout', h('p', 'No subject has a reference sample: mark each subject\'s reference sample (an infusion product, a donor) in the Data view\'s Reference column, or give it such a tissue.')));
    return;
  }
  body.append(subjectsTable(r));
  const whys = [...new Set(r.subjects.filter((s) => !s.derived && s.why).map((s) => s.why))];
  for (const why of whys) body.append(h('p.muted.small', `${r.subjects.filter((s) => s.why === why).map((s) => s.subject).join(', ')}: ${why}`));
  const classOf = new Map(r.subjects.map((s, i) => [s.subject, `plot-s${(i % 6) + 1}`]));
  const legend = r.subjects.map((s) => ({ label: s.subject, className: classOf.get(s.subject) }));
  const rows = (pick) => r.subjects.flatMap((s) => s.points.filter((p) => pick(p)).map((p) => {
    const v = pick(p);
    return { label: `${s.subject} · ${p.point.label}`, value: v.frequency ?? NaN, lower: v.lower ?? NaN, upper: v.upper ?? NaN, className: classOf.get(s.subject) };
  }));
  const card = (title, list, note, name) => {
    // The axis from a tenth below the least, where the shares are all well above 0.
    const least = Math.min(...list.map((x) => (Number.isFinite(x.lower) ? x.lower : x.value)).filter(Number.isFinite));
    const plot = intervals(list, { label: title, width: 600, labelWidth: 150, format: percent, from: Number.isFinite(least) ? Math.max(0, Math.floor(10 * least) / 10 - 0.1) : 0 });
    return exportable(h('figure.plot-card',
      h('figcaption', title),
      h('div.plot-holder', { html: plot.svg }),
      h('div.plot-legend', ...legend.map((l) => h('span', h(`span.swatch.${l.className}`), l.label))),
      h('p.muted.small', note)), () => ({
      name: `${d.name} ${name}`, title: `${title}, ${d.name}`, plot: plot.svg, legend,
      caption: [note, `${d.name}: ${plural(r.subjects.length, 'subject')}. Clonotypes: ${r.describe}.`, madeBy(app.version)],
    }));
  };
  body.append(card('The share in the reference\'s clonotypes', rows((p) => p.value),
    'Each time point\'s share of cells or molecules in the clonotypes its subject\'s reference sample holds (the samples at one time pooled), with its 95% interval for sampling. Before the reference event, the share that owes nothing to it.', 'attribution'));
  const derived = rows((p) => p.from);
  if (derived.length) {
    body.append(card('The share derived from the reference', derived,
      'Of each time point\'s cells in clonotypes found in the reference or in the samples before the event, but not in both, the share in the reference\'s, with its 95% interval for sampling: the reference\'s share of the sample where clonotypes tell the two apart.', 'derived'));
  }
  body.append(table(r));
  body.append(h('p.muted.small', `Attributed ${new Date(state.manifest.created).toLocaleString()} by ClonoLoom ${state.manifest.lineage?.software?.[0]?.version ?? ''}; kept as ${state.artifact}. Clonotypes: ${r.describe}.`));
}

// Each subject's reference, the share before the event, and how much of the reference the samples
// before it hold.
function subjectsTable(r) {
  return h('div.table-scroll', { tabIndex: 0 }, h('table.data',
    h('thead', h('tr', h('th', 'Subject'), h('th', 'Reference'), h('th.r', 'Its clonotypes'),
      h('th.r', { title: 'The share of the samples before the reference event in the reference\'s clonotypes: the share that owes nothing to it' }, 'Before the event'),
      h('th.r', { title: 'The share of the reference\'s cells in clonotypes the samples before the event hold' }, 'Reference found before'),
      h('th', 'Derived share'))),
    h('tbody', ...r.subjects.map((s) => h('tr', h('td', h('b', s.subject)), h('td', `${s.label} (${s.reference.join(', ')})`), h('td.r', count(s.clonotypes)),
      h('td.r', s.before ? percent(s.before.frequency) : '—'), h('td.r', Number.isFinite(s.overlap) ? percent(s.overlap) : '—'),
      h('td', s.derived ? 'told' : h('span.muted', 'not told')))))));
}

// Every time point's and sample's measures.
function table(r) {
  const cell = (v) => (v ? h('td.r', { title: `${count(v.count)} of ${count(v.total)}` }, percent(v.frequency), Number.isFinite(v.lower) ? h('span.muted.small.tracking-interval', `${percent(v.lower)}–${percent(v.upper)}`) : null) : h('td.r.muted', '—'));
  const rows = r.subjects.flatMap((s) => s.samples.map((x) => h('tr', h('td', h('b', s.subject)), h('td.mono', x.name), h('td', x.point.label), cell(x.value), cell(x.from))));
  return h('section', { 'aria-label': 'Each sample' },
    h('div.section-title', `Samples · ${rows.length}`),
    h('div.table-scroll', { tabIndex: 0 }, h('table.data.tracking-table',
      h('thead', h('tr', h('th', 'Subject'), h('th', 'Sample'), h('th', 'Time point'), h('th.r', 'In the reference\'s clonotypes'), h('th.r', 'Derived from the reference'))),
      h('tbody', ...rows))));
}
