// The QC view (clonoloom-spec/requirements.md, Q1–Q3, Q6): whether each sample supports analysis.
// Per-sample metrics computed by the program under the dataset's filter, with plain words on what
// each measures; the shares of productive, out-of-frame and stop-codon rearrangements and the
// junction lengths as plots; single-cell chains per cell; the numbers the vendor reported,
// recomputed; and filters, saved as versions with their reasons, that never delete data.

import { h, icon, clear } from './dom.js';
import { showDialog, toast } from './overlays.js';
import { datasetQC } from './storage.js';
import { UNITS, datasetUnit } from '../lib/datasets.js';
import { LOCI, RULES, cleanRules, datasetFilter, describeRules, saveFilter, useFilter } from '../lib/filters.js';
import { columns, stackedBars } from '../lib/plot.js';

const count = (n) => (Number.isFinite(n) ? Number(n).toLocaleString('en-US', { maximumFractionDigits: 0 }) : '—');
const pct = (v, digits = 1) => (Number.isFinite(v) ? `${(100 * v).toFixed(digits)}%` : '—');
const fixed = (v, digits = 3) => (Number.isFinite(v) ? v.toFixed(digits) : '—');

// What each metric measures, said once beside the table.
const EXPLAIN = [
  ['Abundance', 'The sample\'s total in the dataset\'s unit (templates, reads, UMIs), or for single cells its cells with a productive chain. Samples sequenced to very different depths are compared at equal depth or coverage (Repertoire view).'],
  ['Productive', 'The share of the abundance in productive rearrangements, in frame without a stop codon. Out-of-frame rearrangements are common in genomic DNA (15–30% in immunoSEQ data) and rare in RNA; a high share can mean a failed library.'],
  ['Clonotypes, singletons', 'Distinct productive rearrangements by locus, amino-acid junction, V gene and J gene, and how many were seen once. Many singletons mean many clonotypes were not seen at all.'],
  ['Coverage', 'Sample coverage (Chao and Jost 2012): the estimated share of the repertoire\'s abundance belonging to clonotypes the sample saw. Low coverage means the sample is far from saturated; diversity comparisons should then be made at a common coverage. Counted in reads, coverage is optimistic: amplification gives every molecule many reads, so few clonotypes are seen once; templates and UMIs say more.'],
  ['Largest', 'The largest clonotype\'s share of the productive abundance: a hint of a dominant expansion (or of contamination).'],
  ['V gene, J gene', 'The share of the productive abundance whose V (J) call names one gene. Calls between several genes are kept as sets and never merged.'],
  ['Reads per unit', 'Reads per template or UMI where the data give both: how deeply each molecule was sequenced.'],
];

export function mountQC(app, container) {
  const { store } = app;
  const root = h('div.view');
  const datasetSelect = h('select.input.small', { 'aria-label': 'Dataset', onchange: (e) => { chosen = e.target.value; render(); } });
  const filterSelect = h('select.input.small', { 'aria-label': 'The dataset\'s filter', onchange: (e) => applyChoice(e.target.value) });
  const head = h('div.workbench-head',
    h('h1', icon('qc'), 'Quality control'),
    h('span.spacer'),
    h('label.unit-confirm', h('span.muted', 'Dataset'), datasetSelect),
    h('label.unit-confirm', h('span.muted', 'Filter'), filterSelect),
    h('button.btn.small', { type: 'button', onclick: () => editFilter() }, icon('filter'), 'New filter…'));
  const body = h('div.view-body.qc-view');
  root.append(head, body);
  container.append(root);

  let chosen = store.ui.focus?.kind === 'dataset' ? store.ui.focus.id : store.project.datasets[0]?.id ?? null;
  let answer = null;
  let answerKey = '';
  let shownSample = 0;
  let lengthBy = 'clonotypes';
  let loading = 0;

  const dataset = () => store.project.datasets.find((d) => d.id === chosen) ?? store.project.datasets[0] ?? null;

  function applyChoice(value) {
    const d = dataset();
    if (!d) return;
    store.commit(useFilter(store.project, d.id, value || null), value ? 'Choose a filter' : 'Use no filter');
  }

  function drawControls() {
    const { datasets } = store.project;
    const d = dataset();
    clear(datasetSelect).append(...datasets.map((x) => h('option', { value: x.id, selected: d && x.id === d.id }, x.name)));
    datasetSelect.disabled = datasets.length < 2;
    const filters = (store.project.filters ?? []).filter((f) => f.dataset === d?.id);
    const current = datasetFilter(store.project, d);
    clear(filterSelect).append(h('option', { value: '', selected: !current }, 'None (every row)'),
      ...filters.map((f) => h('option', { value: f.id, selected: current?.id === f.id }, `${f.name} · v${f.version}`)));
    filterSelect.disabled = !d;
  }

  async function load() {
    const d = dataset();
    if (!d) return;
    const filter = datasetFilter(store.project, d);
    const key = `${d.id}|${JSON.stringify(filter?.rules ?? {})}`;
    if (key === answerKey && answer) return;
    const token = ++loading;
    const state = await app.ensureDataset(d);
    if (token !== loading) return;
    if (state.status !== 'open') {
      answer = { error: state.error ?? 'The dataset is not open.' };
      answerKey = key;
      draw();
      return;
    }
    try {
      const result = await datasetQC(d.id, filter?.rules ?? {});
      if (token !== loading) return;
      answer = result;
      answerKey = key;
      shownSample = Math.min(shownSample, result.report.samples.length - 1);
    } catch (error) {
      if (token !== loading) return;
      answer = { error: error.message };
      answerKey = key;
    }
    draw();
  }

  function render() {
    drawControls();
    const d = dataset();
    if (!d) {
      clear(body).append(h('div.empty', icon('qc'), h('h3', 'No data to check'), h('p', 'Open data first: QC measures each sample of a dataset.'),
        h('button.btn.primary', { type: 'button', onclick: () => app.openPathDialog() }, icon('folder'), 'Open data')));
      return;
    }
    const filter = datasetFilter(store.project, d);
    const key = `${d.id}|${JSON.stringify(filter?.rules ?? {})}`;
    if (key !== answerKey) {
      clear(body).append(h('p.muted', 'Measuring the samples…'));
      load();
      return;
    }
    draw();
  }

  function draw() {
    clear(body);
    const d = dataset();
    if (!d || !answer) return;
    if (answer.error) {
      body.append(h('div.callout.danger', h('p', answer.error)));
      return;
    }
    const report = answer.report;
    const unit = datasetUnit(d);
    const filter = datasetFilter(store.project, d);
    body.append(h(`div.callout${filter ? '.accent' : ''}`,
      h('p', filter
        ? [h('b', `${filter.name} · version ${filter.version}: `), describeRules(filter.rules), '. ', h('span.muted', filter.reason)]
        : 'No filter: every row is measured. Save a filter to leave out rows by rules with a reason; the data are never changed.'),
      filter ? removedLine(report) : null));
    body.append(report.singleCell ? singleCellTable(report) : bulkTable(report, unit));
    body.append(h('details.qc-explain', h('summary', 'What these numbers mean'), h('dl', ...EXPLAIN.flatMap(([k, v]) => [h('dt', k), h('dd', v)]))));
    body.append(h('div.plot-grid', statusPlot(report), report.singleCell ? chainsPlot(report) : lengthsPlot(report)));
    if (report.singleCell) body.append(h('div.plot-grid', lengthsPlot(report)));
    body.append(vendorChecks());
  }

  function removedLine(report) {
    const total = {};
    for (const s of report.samples) for (const [reason, n] of Object.entries(s.removed)) total[reason] = (total[reason] ?? 0) + n;
    const parts = Object.entries(total).map(([reason, n]) => `${count(n)} ${reason}`);
    return h('p.muted.small', parts.length ? `Left out: ${parts.join(', ')}.` : 'The filter leaves nothing out of these samples.');
  }

  function bulkTable(report, unit) {
    const rows = report.samples.map((s, i) => h(`tr${i === shownSample ? '.selected' : ''}`, { onclick: () => { shownSample = i; draw(); } },
      h('td.mono.sample-name', { title: s.name }, s.name),
      h('td.r', `${count(s.abundance)}`),
      h('td.r', s.kept === s.rows ? count(s.rows) : `${count(s.kept)} of ${count(s.rows)}`),
      h(`td.r${s.productive < 0.6 ? '.attention' : ''}`, pct(s.productive)),
      h('td.r', pct(s.outOfFrame)),
      h('td.r', pct(s.stop)),
      h('td.r', count(s.clonotypes)),
      h('td.r', s.clonotypes ? pct(s.singletons / s.clonotypes) : '—'),
      h(`td.r${s.coverage < 0.5 ? '.attention' : ''}`, fixed(s.coverage)),
      h('td.r', pct(s.topShare)),
      h('td.r', pct(s.vGene)),
      h('td.r', pct(s.jGene)),
      h('td.r', Number.isFinite(s.readsPer) ? s.readsPer.toFixed(1) : '—')));
    return h('div.table-scroll', { tabIndex: 0 }, h('table.data.qc-table',
      h('caption.sr-only', 'QC of each sample'),
      h('thead', h('tr', h('th', 'Sample'), h('th.r', `Abundance (${UNITS[unit] ?? 'counts'})`), h('th.r', 'Rows'), h('th.r', 'Productive'), h('th.r', 'Out of frame'), h('th.r', 'Stop'),
        h('th.r', 'Clonotypes'), h('th.r', 'Singletons'), h('th.r', 'Coverage'), h('th.r', 'Largest'), h('th.r', 'V gene'), h('th.r', 'J gene'), h('th.r', 'Reads per unit'))),
      h('tbody', ...rows)),
    !report.integerCounts ? h('p.muted.small', 'Some counts are not whole numbers, so sample coverage is not estimated.') : null);
  }

  function singleCellTable(report) {
    const rows = report.samples.map((s, i) => {
      const c = s.chains ?? { cells: 0 };
      const q = c.umis;
      return h(`tr${i === shownSample ? '.selected' : ''}`, { onclick: () => { shownSample = i; draw(); } },
        h('td.mono.sample-name', { title: s.name }, s.name),
        h('td.r', count(c.cells)),
        h('td.r', `${count(c.paired)} (${pct(c.paired / c.cells)})`),
        h('td.r', count(c.single)),
        h('td.r', count(c.none)),
        h(`td.r${c.multi > 0 ? '.attention' : ''}`, count(c.multi)),
        h('td.r', pct(s.productive)),
        h('td', Object.entries(c.byLocus ?? {}).sort().map(([l, n]) => `${l} ${count(n)}`).join(' · ')),
        h('td.r', q ? `${fixed(q.median, 1)} (${fixed(q.q1, 1)}–${fixed(q.q3, 1)})` : '—'));
    });
    return h('div.table-scroll', { tabIndex: 0 }, h('table.data.qc-table',
      h('caption.sr-only', 'QC of each sample\'s cells'),
      h('thead', h('tr', h('th', 'Sample'), h('th.r', 'Cells'), h('th.r', 'Paired'), h('th.r', 'One arm'), h('th.r', 'No productive chain'), h('th.r', 'More than two of a locus'), h('th.r', 'Productive contigs'), h('th', 'Cells with each locus'), h('th.r', 'UMIs per contig (median, IQR)'))),
      h('tbody', ...rows)));
  }

  function plotCard(title, plot, note) {
    return h('figure.plot-card', h('figcaption', title), h('div.plot-holder', { html: plot.svg }), note ? h('p.muted.small', note) : null);
  }

  function statusPlot(report) {
    const keys = [
      { key: 'productive', label: 'productive', className: 'plot-ok' },
      { key: 'outOfFrame', label: 'out of frame', className: 'plot-warn' },
      { key: 'stop', label: 'stop codon', className: 'plot-danger' },
      { key: 'unknown', label: 'not productive, why not given', className: 'plot-c5' },
      { key: 'notGiven', label: 'status not given', className: 'plot-muted' },
    ];
    const plot = stackedBars(report.samples.map((s) => ({ label: s.name, parts: { productive: s.productive, outOfFrame: s.outOfFrame, stop: s.stop, unknown: s.unknown, notGiven: s.notGiven } })), keys, {
      label: report.singleCell ? 'Contigs by status' : 'Abundance by status', share: true, labelWidth: 150,
    });
    const shown = keys.filter((k) => report.samples.some((s) => s[k.key] > 0));
    return plotCard(report.singleCell ? 'Contigs: productive, out of frame, stop codon' : 'Abundance: productive, out of frame, stop codon', plot,
      h('span.plot-legend', ...shown.map((k) => h('span', h(`span.swatch.${k.className}`), k.label))));
  }

  function lengthsPlot(report) {
    const s = report.samples[shownSample];
    const bins = (s?.lengths ?? []).map((l) => ({ x: l.length, y: lengthBy === 'clonotypes' ? l.clonotypes : l.abundance }));
    const plot = columns(bins, { label: `Junction lengths of ${s?.name ?? ''}`, xLabel: 'Junction length (amino acids)', yLabel: lengthBy === 'clonotypes' ? 'Clonotypes' : 'Abundance' });
    const toggle = h('div.segmented', ...[['clonotypes', 'Clonotypes'], ['abundance', report.singleCell ? 'Contigs' : 'Abundance']].map(([id, label]) =>
      h(`button${lengthBy === id ? '.active' : ''}`, { type: 'button', onclick: () => { lengthBy = id; draw(); } }, label)));
    const card = plotCard(`Junction lengths · ${s?.name ?? ''}`, plot, 'Productive junctions, from the conserved cysteine to the phenylalanine or tryptophan. Choose a sample in the table.');
    card.querySelector('figcaption').append(toggle);
    return card;
  }

  function chainsPlot(report) {
    const configs = new Map();
    for (const s of report.samples) for (const [k, n] of Object.entries(s.chains?.configurations ?? {})) configs.set(k, (configs.get(k) ?? 0) + n);
    const top = [...configs.entries()].sort((a, b) => b[1] - a[1]).slice(0, 6).map(([k]) => k);
    const classes = ['plot-ok', 'plot-accent', 'plot-warn', 'plot-c4', 'plot-c5', 'plot-danger'];
    const keys = top.map((k, i) => ({ key: k, label: k, className: classes[i] }));
    const plot = stackedBars(report.samples.map((s) => {
      const parts = {};
      // (a share of the cells: each configuration's cells over the sample's)
      const total = s.chains?.cells || 1;
      for (const k of top) parts[k] = (s.chains?.configurations?.[k] ?? 0) / total;
      return { label: s.name, parts };
    }), keys, { label: 'Cells by their productive chains', share: true, labelWidth: 150 });
    return plotCard('Cells by their productive chains', plot, h('span.plot-legend', ...keys.map((k) => h('span', h(`span.swatch.${k.className}`), k.label))));
  }

  function vendorChecks() {
    if (!answer.vendor?.length) return null;
    const section = h('section.vendor-checks', h('div.section-title', 'The vendor\'s numbers, recomputed'),
      h('p.muted.small', 'What the files\' own summaries report about each sample, beside the same number ClonoLoom computes from the rows it read (before any filter). Agreement says the data were read as written.'));
    for (const v of answer.vendor) {
      section.append(h('h4', `${v.sample} · ${v.source}`), h('table.data',
        h('thead', h('tr', h('th', 'Metric'), h('th.r', 'Reported'), h('th.r', 'ClonoLoom'), h('th', 'Agree'), h('th', 'Note'))),
        h('tbody', ...v.checks.map((c) => h('tr',
          h('td', c.metric), h('td.r', c.reported), h('td.r', c.recomputed),
          h('td', c.agrees === null ? h('span.muted', '—') : c.agrees ? h('span.badge.ok', icon('check'), 'yes') : h('span.badge.warn', icon('warning'), 'no')),
          h('td.muted.small', c.note ?? ''))))));
    }
    return section;
  }

  // The filter editor: a new filter, or a new version of the dataset's current one.
  function editFilter() {
    const d = dataset();
    if (!d) return;
    const current = datasetFilter(store.project, d);
    const rules = { ...(current?.rules ?? { productiveOnly: true }) };
    const name = h('input.input', { type: 'text', value: current?.name ?? 'Productive', 'aria-label': 'Name' });
    const reason = h('textarea.input', { rows: 2, 'aria-label': 'Reason', placeholder: 'Why: the reason goes with every result that uses the filter' });
    const samples = d.samples;
    const fields = RULES.filter((r) => d.singleCell || !['cellsOnly', 'highConfidenceOnly', 'maxChains'].includes(r.id)).filter((r) => !d.singleCell || r.id !== 'minCount').map((rule) => {
      let control;
      if (rule.kind === 'flag') control = h('input', { type: 'checkbox', 'aria-label': rule.label, checked: Boolean(rules[rule.id]), onchange: (e) => { rules[rule.id] = e.target.checked; } });
      else if (rule.kind === 'number') control = h('input.input.small', { type: 'number', min: 0, 'aria-label': rule.label, step: rule.id === 'maxChains' ? 1 : 'any', value: rules[rule.id] ?? '', style: { width: '90px' }, oninput: (e) => { rules[rule.id] = e.target.value; } });
      else if (rule.kind === 'loci') control = h('div.btn-row', ...LOCI.map((l) => h('label.check', h('input', { type: 'checkbox', 'aria-label': l, checked: (rules.loci ?? []).includes(l), onchange: (e) => { rules.loci = e.target.checked ? [...(rules.loci ?? []), l] : (rules.loci ?? []).filter((x) => x !== l); } }), l)));
      else control = h('div.filter-samples', ...samples.map((s) => h('label.check', h('input', { type: 'checkbox', 'aria-label': `Leave out ${s}`, checked: (rules.excludeSamples ?? []).includes(s), onchange: (e) => { rules.excludeSamples = e.target.checked ? [...(rules.excludeSamples ?? []), s] : (rules.excludeSamples ?? []).filter((x) => x !== s); } }), s)));
      if (rule.kind === 'flag') return h('div.field', h('label.check', control, h('b', rule.label)), h('p.muted.small', { style: { margin: 0 } }, rule.hint));
      return h('div.field', h('span', rule.label), control, h('p.muted.small', { style: { margin: 0 } }, rule.hint));
    });
    showDialog({
      title: current ? `New version of ${current.name}` : 'New filter',
      width: 'wide',
      content: [
        h('p.muted', `For ${d.name}. A filter never changes the data: it leaves rows out of analyses, says how many each rule left out, and is kept as a version with its reason. ${current ? 'Saving under the same name makes a new version; the earlier ones stay.' : ''}`),
        h('label.field', h('span', 'Name'), name),
        ...fields,
        h('label.field', h('span', 'Reason'), reason),
      ],
      buttons: [
        { label: 'Cancel', ghost: true },
        { label: 'Save and use', primary: true, onClick: () => {
          try {
            store.commit(saveFilter(store.project, { dataset: d.id, name: name.value, rules: cleanRules(rules), reason: reason.value }), `Save filter ${name.value.trim()}`);
            return true;
          } catch (error) {
            toast(error.message, { kind: 'error' });
            return false;
          }
        } },
      ],
    });
  }

  render();
  return {
    update(topics) {
      if (topics.has('focus') && store.ui.focus?.kind === 'dataset' && store.ui.focus.id !== chosen) {
        chosen = store.ui.focus.id;
        render();
        return;
      }
      if (topics.has('project') || topics.has('datasets') || topics.has('project-loaded') || topics.has('theme')) render();
    },
    destroy() {
      root.remove();
    },
  };
}
