// The Repertoire view (clonoloom-spec/requirements.md, R1, R2, R3, R5 and Q3): each sample's
// clonotypes under its dataset's definition and filter, in four parts, each from an analysis the
// program runs as a job and keeps as a result (api_analysis.go). Measures: a measure for every
// sample with its interval, the Hill numbers' profile, clonal space homeostasis and a table.
// Rarefaction: the samples compared at equal coverage or equal size, their curves by size and by
// coverage, and how complete each sample is. Genes and Junctions (one analysis, gene usage;
// repertoire-usage.js): the genes the clonotypes use, their junctions' lengths and amino acids.
// Every measure explained in words; where a value cannot be trusted as far as its interval says,
// said so.

import { h, icon, clear } from './dom.js';
import { toast } from './overlays.js';
import { curves, intervals, lines, stackedBars } from '../lib/plot.js';
import { UNITS, annotation, datasetUnit, fieldsFromMetadata } from '../lib/datasets.js';
import { datasetDefinition } from '../lib/definitions.js';
import { datasetFilter } from '../lib/filters.js';
import { addResult } from '../lib/results.js';
import { renderGenes, renderJunctions, USAGE_WORDS } from './repertoire-usage.js';

// The measures, as the view offers them: what each is, in words.
export const MEASURES = [
  { id: 'hill1', label: 'Shannon diversity (Hill number 1), seen', words: 'The number of equally common clonotypes that would give the sample\'s Shannon entropy: exp(H). It counts common clonotypes fully and rare ones little.' },
  { id: 'hill1Estimated', label: 'Shannon diversity (Hill number 1), estimated', words: 'The Shannon diversity of the whole repertoire the sample was drawn from (Chao, Wang and Jost 2013). Where the sample coverage is low, a lower bound.' },
  { id: 'hill2', label: 'Simpson diversity (Hill number 2), seen', words: 'The number of equally common clonotypes that would give the sample\'s Simpson index: 1/Σp². It counts mostly the largest clonotypes.' },
  { id: 'hill2Estimated', label: 'Simpson diversity (Hill number 2), estimated', words: 'The Simpson diversity of the whole repertoire: n(n−1)/ΣX(X−1), nearly unbiased at any depth (Chao et al. 2014).' },
  { id: 'richness', label: 'Clonotypes seen (Hill number 0)', words: 'The clonotypes in the sample. It grows with depth: compare samples of different depths at equal coverage (Rarefaction) instead.' },
  { id: 'chao1', label: 'Richness, estimated (Chao1)', words: 'The clonotypes of the whole repertoire, seen and unseen, from those seen once and twice (Chao 1984; bias-corrected). A lower bound; where the coverage is low, far below the truth.' },
  { id: 'ace', label: 'Richness, estimated (ACE)', words: 'The abundance-based coverage estimator (Chao and Lee 1992), from the clonotypes seen at most 10 times. Unstable in deep repertoires; prefer Chao1 or a comparison at equal coverage.' },
  { id: 'clonality', label: 'Clonality (1 − evenness)', words: 'One minus Pielou\'s evenness H/ln S: 0 when every clonotype is equally common, near 1 when a few dominate. Adaptive\'s "clonality". It depends on depth.' },
  { id: 'simpsonClonality', label: 'Simpson clonality (√Σp²)', words: 'The square root of Simpson\'s index, as Adaptive defines it; less sensitive to depth than clonality.' },
  { id: 'evenness', label: 'Evenness (Pielou)', words: 'Shannon entropy over its largest possible value, ln S: 1 when every clonotype is equally common.' },
  { id: 'gini', label: 'Gini coefficient', words: 'How unequal the clonotypes\' abundances are: 0 when equal, near 1 when a few hold nearly everything.' },
  { id: 'top10', label: 'The 10 largest clonotypes\' share', words: 'The share of the sample\'s abundance in its 10 largest clonotypes.' },
  { id: 'top100', label: 'The 100 largest clonotypes\' share', words: 'The share of the sample\'s abundance in its 100 largest clonotypes.' },
  { id: 'maxFrequency', label: 'The largest clonotype\'s frequency', words: 'The largest clonotype\'s share of the sample.' },
  { id: 'shannon', label: 'Shannon entropy (natural log)', words: '−Σp ln p. Its exponential is the Shannon diversity.' },
  { id: 'simpson', label: 'Simpson\'s index (Σp²)', words: 'The chance that two of the sample\'s cells, drawn with replacement, are of the same clonotype.' },
  { id: 'coverage', label: 'Sample coverage', words: 'The share of the repertoire\'s cells whose clonotype the sample holds (Chao and Jost 2012). Below about 0.9, estimates of the whole repertoire are lower bounds.' },
];
const SHARES = new Set(['clonality', 'simpsonClonality', 'evenness', 'gini', 'top10', 'top100', 'maxFrequency', 'simpson', 'coverage']);
const HOMEOSTASIS = [
  { key: 'rare', label: 'rare (≤ 10⁻⁵)', className: 'plot-c5' },
  { key: 'small', label: 'small (≤ 10⁻⁴)', className: 'plot-c4' },
  { key: 'medium', label: 'medium (≤ 10⁻³)', className: 'plot-accent' },
  { key: 'large', label: 'large (≤ 10⁻²)', className: 'plot-warn' },
  { key: 'hyperexpanded', label: 'hyperexpanded', className: 'plot-danger' },
];

// The curves' orders.
export const ORDERS = [
  { q: 0, label: 'Clonotypes (Hill number 0)', short: 'Clonotypes', words: 'the clonotypes a sample of that size or coverage would hold' },
  { q: 1, label: 'Shannon diversity (Hill number 1)', short: 'Shannon diversity', words: 'the Shannon diversity it would show' },
  { q: 2, label: 'Simpson diversity (Hill number 2)', short: 'Simpson diversity', words: 'the Simpson diversity it would show' },
];

// The levels the samples are compared at (diversity.SuggestedLevels): two reached by rarefaction
// alone, two by extrapolating to twice a sample's size.
export const LEVELS = [
  { key: 'coverage', base: 'coverage', index: 0, extrapolated: false },
  { key: 'size', base: 'size', index: 0, extrapolated: false },
  { key: 'extrapolatedCoverage', base: 'coverage', index: 1, extrapolated: true },
  { key: 'extrapolatedSize', base: 'size', index: 1, extrapolated: true },
];

// The parts, the analysis each shows, and what each says before its result is there.
const PARTS = {
  diversity: {
    kind: 'diversity', label: 'Measures', start: 'Measure diversity', title: 'Each sample\'s diversity',
    words: 'Measured by the program over every sample\'s clonotypes under this definition and filter: the Hill numbers and their profile, Shannon\'s entropy, Simpson\'s index, evenness, clonality, Gini\'s coefficient, the largest clonotypes\' shares, clonal space homeostasis, and the whole repertoire\'s richness (Chao1, ACE) and Shannon and Simpson diversity, each with a 95% interval from 200 bootstrap replicates. The result is kept with the project, with how it was made.',
  },
  rarefaction: {
    kind: 'rarefaction', label: 'Rarefaction', start: 'Rarefy and extrapolate', title: 'Samples compared at equal coverage',
    words: 'A deeper sample holds more clonotypes because it is deeper. The program computes what each sample would show at any size up to twice its own, and at any sample coverage (Chao et al. 2014; Chao and Jost 2012): the clonotypes, the Shannon diversity and the Simpson diversity, each with a 95% interval from 200 bootstrap replicates (the same replicates as the measures). The samples are then compared at the same completeness, or the same size. The result is kept with the project.',
  },
  genes: { kind: 'usage', label: 'Genes', start: 'Count genes and junctions', title: 'The genes each sample\'s clonotypes use', words: USAGE_WORDS },
  junctions: { kind: 'usage', label: 'Junctions', start: 'Count genes and junctions', title: 'Each sample\'s junction lengths and amino acids', words: USAGE_WORDS },
};
// The analyses, and the part each result opens in.
const KINDS = [...new Set(Object.values(PARTS).map((p) => p.kind))];
const PART_OF = { diversity: 'diversity', rarefaction: 'rarefaction', usage: 'genes' };
const ACTIONS = { diversity: 'Measure the diversity', rarefaction: 'Rarefy the samples', usage: 'Count the genes and junctions' };

const fmt = (v, share) => {
  if (!Number.isFinite(v)) return '—';
  if (v === 0) return '0';
  if (share) return v < 0.001 && v > 0 ? v.toExponential(2) : v.toFixed(3);
  return v >= 100 ? Math.round(v).toLocaleString('en-US') : v.toPrecision(4);
};
const fmtCoverage = (v) => (!Number.isFinite(v) ? '—' : v > 0.99 && v < 1 ? v.toFixed(4) : v.toFixed(3));

async function getJSON(url, options = {}) {
  const response = await fetch(url, { cache: 'no-store', ...options });
  const body = await response.json().catch(() => ({}));
  if (!response.ok) {
    const error = new Error(body.error ?? `${url}: HTTP ${response.status}`);
    error.code = body.code;
    throw error;
  }
  return body;
}

export function mountRepertoire(app, container) {
  const { store } = app;
  const datasetSelect = h('select.input.small', { 'aria-label': 'Dataset', onchange: (e) => { chosen = e.target.value; load(); } });
  const partSwitch = h('div.segmented', { role: 'tablist', 'aria-label': 'Part' });
  const measureButton = h('button.btn.primary.small', { type: 'button', onclick: () => measure(PARTS[part].kind) });
  const head = h('div.workbench-head', h('h1', icon('repertoire'), 'Repertoire'), partSwitch, h('span.spacer'),
    h('label.kv-inline', h('span.muted', 'Dataset'), datasetSelect), measureButton);
  const body = h('div.view-body.repertoire-view');
  const root = h('div.view', head, h('div.workbench-scroll', body));
  container.append(root);
  const focusedResult = store.ui.focus?.kind === 'result' ? store.project.artifacts.find((a) => a.id === store.ui.focus.id) : null;
  let chosen = store.ui.focus?.kind === 'dataset' ? store.ui.focus.id : focusedResult?.dataset ?? store.project.datasets[0]?.id ?? null;
  let part = PART_OF[focusedResult?.kind] ?? 'diversity';
  // Each analysis's state: idle | looking | absent | running | ready | failed.
  const states = Object.fromEntries(KINDS.map((k) => [k, { status: 'idle' }]));
  // The Genes and Junctions parts' choices (repertoire-usage.js).
  const usageUI = {};
  let spec = null;
  let shown = 'hill1';
  let order = 1;
  let level = 'coverage';
  let generation = 0;

  const dataset = () => store.project.datasets.find((d) => d.id === chosen) ?? store.project.datasets[0] ?? null;

  // Each sample's condition: its annotation's, else its metadata's.
  function conditionOf(d, name) {
    const summary = app.datasetStates.get(d.id)?.summary;
    return annotation(store.project, d.id, name).fields.condition
      ?? (summary ? fieldsFromMetadata(summary.samples.find((x) => x.name === name) ?? {}).condition : undefined);
  }

  // The samples' groups, by condition, for the analyses that summarize groups (none when no sample
  // has a condition).
  function groupsOf(d) {
    const summary = app.datasetStates.get(d.id)?.summary;
    const groups = {};
    for (const s of summary?.samples ?? []) {
      const c = conditionOf(d, s.name);
      if (c) groups[s.name] = c;
    }
    return Object.keys(groups).length ? groups : undefined;
  }

  async function specOf(d) {
    const open = await app.ensureDataset(d);
    if (open.status !== 'open') throw new Error(open.error ?? `${d.name} could not be opened.`);
    const loci = Object.keys(open.summary.samples.reduce((acc, s) => Object.assign(acc, s.loci), {}));
    const filter = datasetFilter(store.project, d);
    return { dataset: d.id, definition: datasetDefinition(d, loci), filter: filter?.rules ?? {}, filterRecord: filter, groups: groupsOf(d) };
  }

  const request = (s, kind, extra = {}) => JSON.stringify({ dataset: s.dataset, definition: s.definition, filter: s.filter, ...(kind === 'usage' ? { groups: s.groups } : {}), ...extra });

  // Looks for each part's result kept for the dataset as chosen; follows a job still running.
  async function load() {
    const mine = ++generation;
    const d = dataset();
    renderHead();
    spec = null;
    if (!d) {
      for (const kind of KINDS) states[kind] = { status: 'idle' };
      render();
      return;
    }
    for (const kind of KINDS) states[kind] = { status: 'looking' };
    render();
    try {
      spec = await specOf(d);
    } catch (error) {
      if (mine !== generation) return;
      for (const kind of KINDS) states[kind] = { status: 'failed', error: error.message };
      render();
      return;
    }
    await Promise.all(KINDS.map(async (kind) => {
      try {
        const answer = await getJSON(`api/${kind}`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: request(spec, kind, { start: false }) });
        if (mine !== generation) return;
        if (answer.ready) {
          const read = await getJSON(`api/${kind}/${encodeURIComponent(answer.artifact)}`);
          if (mine !== generation) return;
          states[kind] = { status: 'ready', artifact: answer.artifact, result: read.result, manifest: read.artifact };
          keep(kind, d, read);
        } else {
          states[kind] = { status: answer.job ? 'running' : 'absent', artifact: answer.artifact, job: answer.job };
          if (answer.job) follow(kind, answer.job);
        }
      } catch (error) {
        if (mine !== generation) return;
        states[kind] = { status: 'failed', error: error.message };
      }
    }));
    if (mine === generation) render();
  }

  // The result in the project's record (the project tree lists it; the methods paragraph says it).
  function keep(kind, d, read) {
    if (store.project.artifacts.some((a) => a.id === read.artifact.id)) return;
    const r = read.result;
    const common = {
      id: read.artifact.id, kind, dataset: d.id, definition: spec.definition.name ?? '',
      filter: spec.filterRecord ? `${spec.filterRecord.name} v${spec.filterRecord.version}` : null,
      made: read.artifact.created, options: r.options, unit: r.unit, describe: r.describe,
    };
    const record = {
      diversity: { ...common, name: `Diversity · ${d.name}`, method: `Hill numbers, Chao1, ACE; ${r.options.replicates} bootstrap replicates` },
      rarefaction: { ...common, name: `Rarefaction · ${d.name}`, method: `Rarefaction and extrapolation by size and coverage; ${r.options.replicates} bootstrap replicates`, levels: r.levels },
      usage: { ...common, name: `Gene usage · ${d.name}`, method: `V, D, J and C usage, junction lengths and amino-acid composition; ${r.replicates} bootstrap replicates of the clonotypes`, groups: (r.groups ?? []).length },
    }[kind];
    store.commit(addResult(store.project, record), `${ACTIONS[kind]} of ${d.name}`, ['project']);
  }

  // When a part's job ends: its result read, or what went wrong said. Each job followed once.
  const following = new Set();
  function follow(kind, jobId) {
    if (following.has(jobId)) return;
    following.add(jobId);
    const stop = app.jobs.onFinished((job) => {
      if (job.id !== jobId) return;
      stop();
      following.delete(jobId);
      if (job.state === 'complete') load();
      else {
        states[kind] = { status: 'failed', error: job.error || `The job ended ${job.state}.` };
        render();
      }
    });
  }

  async function measure(kind) {
    const d = dataset();
    if (!d) return;
    try {
      spec = spec ?? await specOf(d);
      const answer = await getJSON(`api/${kind}`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: request(spec, kind) });
      if (answer.ready) {
        load();
        return;
      }
      states[kind] = { status: 'running', artifact: answer.artifact, job: answer.job };
      app.log(`${ACTIONS[kind]} of ${d.name} (${spec.definition.name ?? 'its definition'}).`);
      render();
      follow(kind, answer.job);
    } catch (error) {
      toast(error.message, { kind: 'error' });
    }
  }

  function show(name) {
    part = PARTS[name] ? name : PART_OF[name] ?? part;
    render();
  }

  function renderHead() {
    clear(datasetSelect);
    for (const d of store.project.datasets) datasetSelect.append(h('option', { value: d.id, selected: d.id === dataset()?.id }, d.name));
    datasetSelect.disabled = !store.project.datasets.length;
  }

  function render() {
    clear(partSwitch);
    for (const [kind, p] of Object.entries(PARTS)) {
      partSwitch.append(h(`button${kind === part ? '.active' : ''}`, { type: 'button', role: 'tab', 'aria-selected': kind === part ? 'true' : 'false', onclick: () => show(kind) }, p.label));
    }
    clear(body);
    const d = dataset();
    const state = states[PARTS[part].kind];
    clear(measureButton);
    measureButton.append(icon('play'), PARTS[part].start);
    measureButton.hidden = !d || state.status === 'ready' || state.status === 'running';
    if (!d) {
      body.append(h('div.empty', icon('repertoire'), h('h3', 'No data yet'),
        h('p', 'Open data, define clonotypes, and measure each sample\'s diversity here, every measure with its interval, and compare samples at equal coverage.'),
        h('div.btn-row', { style: { justifyContent: 'center', marginTop: '10px' } },
          h('button.btn.primary', { type: 'button', onclick: () => app.openPathDialog() }, icon('folder'), 'Open data'),
          h('button.btn', { type: 'button', onclick: () => app.showExamples() }, icon('flask'), 'Example data'))));
      return;
    }
    if (spec) {
      body.append(h('p.muted.small.repertoire-spec', 'Clonotypes: ', h('b', spec.definition.name ?? 'the definition'), spec.filterRecord ? ` · filter "${spec.filterRecord.name}" v${spec.filterRecord.version}` : ' · no filter',
        ` · abundance in ${UNITS[datasetUnit(d)] ?? 'counts'}`));
    }
    // The estimators count each cell or molecule once; reads count each many times over.
    if (datasetUnit(d) === 'reads') {
      body.append(h('div.callout.warn', h('p', 'Abundance in reads: a clonotype\'s reads count each of its cells many times over, as often as it was amplified and sequenced, so few clonotypes are seen once or twice. Chao1, ACE, the sample coverage, the estimated diversities and the curves\' coverage and extrapolation all rest on those clonotypes: in reads, a sample looks complete when it is not. Rarefied to equal reads, samples are compared at equal sequencing depth, not equal numbers of cells. Use templates, UMIs or cells where the data have them.')));
    }
    if (state.status === 'looking') {
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
        h('h3', PARTS[part].title),
        h('p', PARTS[part].words),
        state.status === 'running'
          ? h('div.example-progress', h('div.progress', h('div', { style: { width: `${Math.round((job?.progress ?? 0) * 100)}%` } })), h('span.muted.small', job?.message ?? 'Waiting'))
          : h('div.btn-row', h('button.btn.primary', { type: 'button', onclick: () => measure(PARTS[part].kind) }, icon('play'), PARTS[part].start))));
      return;
    }
    if (state.status !== 'ready') return;
    if (part === 'diversity') renderMeasures(d, state);
    else if (part === 'rarefaction') renderRarefaction(d, state);
    else (part === 'genes' ? renderGenes : renderJunctions)({ body, state, ui: usageUI, render, colors: colors(d, state.result.samples.map((s) => s.name)) });
  }

  // Colors by condition: the samples' annotation, else their metadata's; a group (a condition) its
  // condition's color.
  function colors(d, names) {
    const conditions = [...new Set(names.map((n) => conditionOf(d, n)).filter(Boolean))];
    const classOf = (name) => {
      const c = conditionOf(d, name) ?? (conditions.includes(name) ? name : undefined);
      return c ? `plot-s${(conditions.indexOf(c) % 6) + 1}` : 'plot-s1';
    };
    const legend = () => (conditions.length ? h('div.plot-legend', ...conditions.map((c, i) => h('span', h(`span.swatch.plot-s${(i % 6) + 1}`), c))) : null);
    return { classOf, legend };
  }

  function renderMeasures(d, state) {
    const result = state.result;
    const samples = result.samples;
    const unit = UNITS[result.unit] ?? 'counts';
    const { classOf, legend } = colors(d, samples.map((s) => s.name));
    const low = samples.filter((s) => Number.isFinite(s.result.measures.coverage?.estimate) && s.result.measures.coverage.estimate < 0.9);
    const notes = [];
    if (low.length) notes.push(h('div.callout.warn', h('p', `${low.length === samples.length ? 'Every sample has' : `${low.length} of ${samples.length} samples have`} a sample coverage below 0.9 (${low.slice(0, 4).map((s) => `${s.name} ${s.result.measures.coverage.estimate.toFixed(2)}`).join(', ')}${low.length > 4 ? ', …' : ''}): there, Chao1 and the estimated Shannon diversity are lower bounds, and in ClonoLoom's simulations their intervals did not hold the truth. Compare such samples at equal coverage instead (Rarefaction).`)));
    const notWhole = samples.filter((s) => !s.result.whole);
    if (notWhole.length) notes.push(h('div.callout', h('p', `${notWhole.length} sample${notWhole.length === 1 ? '\'s' : 's\''} abundances are not whole numbers: only the measures that do not need counts are given, without intervals.`)));
    body.append(...notes);

    // One measure across the samples.
    const info = MEASURES.find((m) => m.id === shown) ?? MEASURES[0];
    const picker = h('select.input.small', { 'aria-label': 'Measure', onchange: (e) => { shown = e.target.value; render(); } },
      ...MEASURES.map((m) => h('option', { value: m.id, selected: m.id === shown }, m.label)));
    const rows = samples.map((s) => {
      const v = s.result.measures[info.id] ?? {};
      return { label: s.name, value: v.estimate ?? NaN, lower: v.lower ?? NaN, upper: v.upper ?? NaN, className: classOf(s.name) };
    });
    const plot = intervals(rows, { label: info.label, width: 560, labelWidth: 140, format: (v) => fmt(v, SHARES.has(info.id)) });
    body.append(h('figure.plot-card',
      h('figcaption', h('span', 'Each sample'), picker),
      h('div.plot-holder', { html: plot.svg }),
      legend(),
      h('p.muted.small', info.words, ' ', 'Bars are 95% intervals (Chao1\'s from its variance, the others from 200 bootstrap replicates of each sample).')));

    // The Hill profile and homeostasis.
    const profileQs = [0, 0.25, 0.5, 0.75, 1, 1.25, 1.5, 1.75, 2, 2.5, 3, 3.5, 4];
    const profile = lines(samples.map((s) => ({ label: s.name, className: classOf(s.name), points: profileQs.map((q) => ({ x: q, y: s.result.measures[`hill@${q}`]?.estimate ?? NaN })) })),
      { label: 'Hill numbers by order', xLabel: 'Order q', yLabel: 'Effective clonotypes', log: true });
    const homeo = stackedBars(samples.map((s) => ({ label: s.name, parts: Object.fromEntries(HOMEOSTASIS.map((b) => [b.key, s.result.measures[`homeostasis.${b.key}`]?.estimate ?? 0])) })), HOMEOSTASIS, { label: 'Clonal space homeostasis', share: true });
    body.append(h('div.plot-grid',
      h('figure.plot-card', h('figcaption', 'The Hill profile'), h('div.plot-holder', { html: profile.svg }), legend(),
        h('p.muted.small', 'Each sample\'s Hill numbers from order 0 (clonotypes seen) through 1 (Shannon) and 2 (Simpson) to 4: a profile that falls fast belongs to a repertoire dominated by a few clonotypes.')),
      h('figure.plot-card', h('figcaption', 'Clonal space homeostasis'), h('div.plot-holder', { html: homeo.svg }),
        h('div.plot-legend', ...HOMEOSTASIS.map((b) => h('span', h(`span.swatch.${b.className}`), b.label))),
        h('p.muted.small', 'The share of each sample in clonotypes of each frequency (immunarch\'s bins).'))));

    // The table.
    const cols = [['Clonotypes', 'richness'], ['Chao1', 'chao1'], ['Shannon diversity', 'hill1'], ['Simpson diversity', 'hill2'], ['Clonality', 'clonality'], ['Simpson clonality', 'simpsonClonality'], ['Gini', 'gini'], ['Coverage', 'coverage']];
    const cell = (v, id) => {
      if (!v) return h('td.r', '—');
      const share = SHARES.has(id);
      return h('td.r', { title: Number.isFinite(v.lower) ? `95% interval ${fmt(v.lower, share)}–${fmt(v.upper, share)}` : '' }, fmt(v.estimate, share),
        Number.isFinite(v.lower) ? h('span.muted.small', ` ${fmt(v.lower, share)}–${fmt(v.upper, share)}`) : null);
    };
    body.append(h('div.section-title', `Samples · ${samples.length}`),
      h('div.table-scroll', { tabIndex: 0 }, h('table.data.repertoire-table',
        h('thead', h('tr', h('th', 'Sample'), h('th.r', `Abundance (${unit})`), ...cols.map(([label]) => h('th.r', label)))),
        h('tbody', ...samples.map((s) => h('tr', h('td.sample-name', { title: s.name }, s.name), h('td.r', fmt(s.result.abundance)), ...cols.map(([, id]) => cell(s.result.measures[id], id))))))),
      h('p.muted.small', `Measured ${new Date(state.manifest.created).toLocaleString()} by ClonoLoom ${state.manifest.lineage?.software?.[0]?.version ?? ''}, ${result.options.replicates} bootstrap replicates (seed ${result.options.seed}); kept as ${state.artifact}. Clonotypes: ${result.describe}.`));
  }

  // Each level in words: what it is, and which sample sets it.
  function levelWords(l, result, unit) {
    const levels = result.levels;
    const nameOf = (i) => result.samples[i]?.name ?? 'one sample';
    switch (l.key) {
      case 'coverage': return { label: `Coverage ${fmtCoverage(levels.coverage)} · rarefied`, words: `a sample coverage of ${fmtCoverage(levels.coverage)}, the lowest among the samples (${nameOf(levels.coverageOf)}'s): every sample rarefied to it, none extrapolated` };
      case 'size': return { label: `${fmt(levels.size)} ${unit} · rarefied`, words: `${fmt(levels.size)} ${unit}, the smallest sample's size (${nameOf(levels.sizeOf)}'s): every sample rarefied to it` };
      case 'extrapolatedCoverage': return { label: `Coverage ${fmtCoverage(levels.extrapolatedCoverage)} · extrapolated`, words: `a sample coverage of ${fmtCoverage(levels.extrapolatedCoverage)}, the lowest any sample reaches at twice its size (iNEXT's default): samples whose own coverage is lower extrapolated to it` };
      default: return { label: `${fmt(levels.extrapolatedSize)} ${unit} · extrapolated`, words: `${fmt(levels.extrapolatedSize)} ${unit}, twice the smallest sample's size (iNEXT's default): samples smaller than it extrapolated` };
    }
  }

  function renderRarefaction(d, state) {
    const result = state.result;
    const samples = result.samples;
    const unit = UNITS[result.unit] ?? 'counts';
    const { classOf, legend } = colors(d, samples.map((s) => s.name));
    if (!result.levels) {
      body.append(h('div.callout', h('p', 'No sample can be rarefied: rarefaction needs abundances that are whole numbers (counts of cells, templates, UMIs or reads), and at least two of them.')));
      return;
    }
    const notWhole = samples.filter((s) => !s.curves.whole);
    if (notWhole.length) body.append(h('div.callout', h('p', `${notWhole.length} sample${notWhole.length === 1 ? '\'s' : 's\''} abundances are not whole numbers: ${notWhole.length === 1 ? 'it is' : 'they are'} not rarefied.`)));
    const lv = LEVELS.find((l) => l.key === level) ?? LEVELS[0];
    const ord = ORDERS[order];
    const words = levelWords(lv, result, unit);
    const at = (s) => (lv.base === 'size' ? s.curves.atSize : s.curves.atCoverage)?.[lv.index];

    // The comparison: every sample at the level chosen.
    const levelPicker = h('select.input.small', { 'aria-label': 'Compared at', onchange: (e) => { level = e.target.value; render(); } },
      ...LEVELS.map((l) => h('option', { value: l.key, selected: l.key === lv.key }, levelWords(l, result, unit).label)));
    const orderPicker = h('select.input.small', { 'aria-label': 'Order', onchange: (e) => { order = Number(e.target.value); render(); } },
      ...ORDERS.map((o) => h('option', { value: o.q, selected: o.q === ord.q }, o.label)));
    const rows = samples.filter((s) => s.curves.whole && at(s)).map((s) => {
      const p = at(s);
      const v = p.hill?.[ord.q] ?? {};
      return { label: `${s.name}${p.method === 'extrapolated' ? ' (extrapolated)' : ''}`, value: v.estimate ?? NaN, lower: v.lower ?? NaN, upper: v.upper ?? NaN, className: classOf(s.name) };
    });
    const plot = intervals(rows, { label: `${ord.label} at ${words.label}`, width: 560, labelWidth: 170, format: (v) => fmt(v) });
    const extrapolatedHere = samples.filter((s) => at(s)?.method === 'extrapolated');
    body.append(h('figure.plot-card',
      h('figcaption', h('span', 'Each sample at'), h('span.plot-pickers', levelPicker, orderPicker)),
      h('div.plot-holder', { html: plot.svg }),
      legend(),
      h('p.muted.small', `${ord.short} at ${words.words}. Bars are 95% intervals from ${result.options.replicates} bootstrap replicates of each sample. Comparing at equal coverage is fairer than at equal size: a sample twice as deep is not just more of the same repertoire (Chao and Jost 2012).`)));
    if (extrapolatedHere.length) {
      body.append(h('div.callout.warn', h('p', `${extrapolatedHere.length} sample${extrapolatedHere.length === 1 ? ' is' : 's are'} extrapolated at this level. In ClonoLoom's simulations of repertoires with many rare clonotypes, values extrapolated to twice a sample's size were within 1–3% of the truth on average, but their intervals were narrower than that and often missed it (the Simpson diversity's excepted). Prefer a rarefied level where it suffices.`)));
    }

    // The curves.
    const sizes = samples.map((s) => s.curves.abundance).filter((n) => n > 0);
    const logX = sizes.length > 1 && Math.max(...sizes) / Math.min(...sizes) > 10;
    const bands = samples.length <= 8;
    const point = (p, x, y, interval) => ({ x, y: y?.estimate ?? NaN, lower: interval ? y?.lower ?? NaN : NaN, upper: interval ? y?.upper ?? NaN : NaN, extrapolated: p.method === 'extrapolated' });
    const series = (pick) => samples.filter((s) => s.curves.whole && s.curves.bySize).map((s) => {
      const { points, observed } = pick(s);
      return { label: s.name, className: classOf(s.name), points, observed };
    });
    const bySize = curves(series((s) => {
      const points = s.curves.bySize.map((p) => point(p, p.size, p.hill[ord.q], true));
      const own = s.curves.bySize.find((p) => p.method === 'observed');
      return { points, observed: own ? { x: own.size, y: own.hill[ord.q].estimate } : null };
    }), { label: `${ord.short} by sample size`, xLabel: `Sample size (${unit}${logX ? ', logarithmic' : ''})`, yLabel: ord.short, logX, bands,
      guides: lv.base === 'size' ? [{ x: lv.index ? result.levels.extrapolatedSize : result.levels.size, label: 'compared' }] : [] });
    const byCoverage = curves(series((s) => {
      const points = s.curves.byCoverage.filter((p) => Number.isFinite(p.size)).map((p) => point(p, p.coverage.estimate, p.hill[ord.q], true));
      const own = s.curves.byCoverage.find((p) => p.method === 'observed');
      return { points, observed: own ? { x: own.coverage.estimate, y: own.hill[ord.q].estimate } : null };
    }), { label: `${ord.short} by sample coverage`, xLabel: 'Sample coverage', yLabel: ord.short, xMax: 1, bands, xFormat: (v) => String(Number(v.toFixed(2))),
      guides: lv.base === 'coverage' ? [{ x: lv.index ? result.levels.extrapolatedCoverage : result.levels.coverage, label: 'compared' }] : [] });
    const completeness = curves(series((s) => {
      const points = s.curves.bySize.map((p) => point(p, p.size, p.coverage, true));
      const own = s.curves.bySize.find((p) => p.method === 'observed');
      return { points, observed: own ? { x: own.size, y: own.coverage.estimate } : null };
    }), { label: 'Sample coverage by sample size', xLabel: `Sample size (${unit}${logX ? ', logarithmic' : ''})`, yLabel: 'Sample coverage', logX, bands, yFormat: (v) => String(Number(v.toFixed(2))) });
    const card = (title, plotted, text) => h('figure.plot-card', h('figcaption', title), h('div.plot-holder', { html: plotted.svg }), legend(), h('p.muted.small', text));
    body.append(h('div.plot-grid',
      card(`${ord.short} by size`, bySize, `What each sample would show at each size: solid where rarefied, dashed where extrapolated (to twice its size), the dot the sample itself${bands ? ', the band its 95% interval' : ''}.`),
      card(`${ord.short} by coverage`, byCoverage, `The same against the sample coverage each size reaches: where the curves cross a coverage, the samples are equally complete there.`),
      card('Sample completeness', completeness, `How the sample coverage rises with size (Chao and Jost 2012): a curve still rising at its dot is far from complete, and extrapolating it is less sure.`)));

    // The table: each sample at the level chosen, every order.
    const cell = (v) => (v && Number.isFinite(v.estimate)
      ? h('td.r', { title: Number.isFinite(v.lower) ? `95% interval ${fmt(v.lower)}–${fmt(v.upper)}` : '' }, fmt(v.estimate), Number.isFinite(v.lower) ? h('span.muted.small', ` ${fmt(v.lower)}–${fmt(v.upper)}`) : null)
      : h('td.r', '—'));
    body.append(h('div.section-title', `Samples at ${words.label} · ${samples.length}`),
      h('div.table-scroll', { tabIndex: 0 }, h('table.data.repertoire-table',
        h('thead', h('tr', h('th', 'Sample'), h('th.r', `Abundance (${unit})`), h('th.r', 'Coverage seen'), h('th.r', lv.base === 'size' ? 'Coverage there' : `Size there (${unit})`),
          ...ORDERS.map((o) => h('th.r', o.short)), h('th', 'How'))),
        h('tbody', ...samples.map((s) => {
          const p = at(s);
          const own = s.curves.bySize?.find((x) => x.method === 'observed');
          return h('tr', h('td.sample-name', { title: s.name }, s.name), h('td.r', fmt(s.curves.abundance)), h('td.r', fmtCoverage(own?.coverage?.estimate)),
            h('td.r', p ? (lv.base === 'size' ? fmtCoverage(p.coverage?.estimate) : fmt(p.size)) : '—'),
            ...ORDERS.map((o) => cell(p?.hill?.[o.q])), h('td', p ? { interpolated: 'rarefied', observed: 'as sampled', extrapolated: 'extrapolated', unreachable: 'out of reach' }[p.method] ?? p.method : 'not rarefied'));
        })))),
      h('p.muted.small', `Computed ${new Date(state.manifest.created).toLocaleString()} by ClonoLoom ${state.manifest.lineage?.software?.[0]?.version ?? ''}: ${result.knots} sizes along each curve, ${result.options.replicates} bootstrap replicates (seed ${result.options.seed}, the measures' replicates); kept as ${state.artifact}. Clonotypes: ${result.describe}.`));
  }

  // What the shown results depend on: the dataset, its definition and its filter as chosen, and its
  // samples' conditions.
  const choices = () => {
    const d = dataset();
    return JSON.stringify([d?.id ?? null, d?.definition ?? null, d?.filter ?? null, d ? groupsOf(d) ?? null : null]);
  };
  let shownChoices = choices();

  // A running job's progress, as the program reports it; a new definition or filter, measured again
  // or looked up; a result chosen elsewhere, shown in its part.
  function update(topics) {
    if (topics.has('jobs') && states[PARTS[part].kind].status === 'running') render();
    if (topics.has('focus')) {
      const r = store.ui.focus?.kind === 'result' ? store.project.artifacts.find((a) => a.id === store.ui.focus.id) : null;
      if (r && PART_OF[r.kind] && (PARTS[part].kind !== r.kind || r.dataset !== chosen)) {
        part = PART_OF[r.kind];
        if (r.dataset !== chosen) {
          chosen = r.dataset;
          shownChoices = choices();
          load();
        } else render();
      }
    }
    if (topics.has('project') || topics.has('project-loaded') || topics.has('datasets')) {
      if (!store.project.datasets.some((d) => d.id === chosen)) chosen = store.project.datasets[0]?.id ?? null;
      if (topics.has('project-loaded') || choices() !== shownChoices) {
        shownChoices = choices();
        load();
      } else renderHead();
    }
  }

  app.repertoireView = {
    measure: (kind = 'diversity') => measure(kind),
    show,
    setUsage: (choices) => {
      Object.assign(usageUI, choices);
      render();
    },
    setLevel: (key, q) => {
      if (key && LEVELS.some((l) => l.key === key)) level = key;
      if (q !== undefined && ORDERS[q]) order = q;
      render();
    },
    load: () => load(),
    state: (kind = 'diversity') => states[kind],
  };
  load();
  return {
    update,
    destroy() {
      app.repertoireView = null;
      root.remove();
    },
  };
}
