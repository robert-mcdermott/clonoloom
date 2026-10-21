// The Repertoire view (clonoloom-spec/requirements.md, R1 and R5): each sample's diversity under
// its dataset's clonotype definition and filter, measured by the program as a job and kept as a
// result (api_diversity.go): a measure for every sample with its interval, the Hill numbers'
// profile, clonal space homeostasis, and a table of the measures; each explained in words, and
// where a sample is too little sampled for an estimate to hold, said so.

import { h, icon, clear } from './dom.js';
import { toast } from './overlays.js';
import { intervals, lines, stackedBars } from '../lib/plot.js';
import { UNITS, annotation, datasetUnit, fieldsFromMetadata } from '../lib/datasets.js';
import { datasetDefinition } from '../lib/definitions.js';
import { datasetFilter } from '../lib/filters.js';
import { addResult } from '../lib/results.js';

// The measures, as the view offers them: what each is, in words.
export const MEASURES = [
  { id: 'hill1', label: 'Shannon diversity (Hill number 1), seen', words: 'The number of equally common clonotypes that would give the sample\'s Shannon entropy: exp(H). It counts common clonotypes fully and rare ones little.' },
  { id: 'hill1Estimated', label: 'Shannon diversity (Hill number 1), estimated', words: 'The Shannon diversity of the whole repertoire the sample was drawn from (Chao, Wang and Jost 2013). Where the sample coverage is low, a lower bound.' },
  { id: 'hill2', label: 'Simpson diversity (Hill number 2), seen', words: 'The number of equally common clonotypes that would give the sample\'s Simpson index: 1/Σp². It counts mostly the largest clonotypes.' },
  { id: 'hill2Estimated', label: 'Simpson diversity (Hill number 2), estimated', words: 'The Simpson diversity of the whole repertoire: n(n−1)/ΣX(X−1), nearly unbiased at any depth (Chao et al. 2014).' },
  { id: 'richness', label: 'Clonotypes seen (Hill number 0)', words: 'The clonotypes in the sample. It grows with depth: compare samples of different depths at equal coverage (rarefaction) instead.' },
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

const fmt = (v, share) => {
  if (!Number.isFinite(v)) return '—';
  if (share) return v < 0.001 && v > 0 ? v.toExponential(2) : v.toFixed(3);
  return v >= 100 ? Math.round(v).toLocaleString('en-US') : v.toPrecision(4);
};

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
  const measureButton = h('button.btn.primary.small', { type: 'button', onclick: () => measure() }, icon('play'), 'Measure diversity');
  const head = h('div.workbench-head', h('h1', icon('repertoire'), 'Repertoire'), h('span.spacer'),
    h('label.kv-inline', h('span.muted', 'Dataset'), datasetSelect), measureButton);
  const body = h('div.view-body.repertoire-view');
  const root = h('div.view', head, h('div.workbench-scroll', body));
  container.append(root);
  let chosen = store.ui.focus?.kind === 'dataset' ? store.ui.focus.id : store.project.datasets.find((d) => store.ui.focus?.kind === 'result' && store.project.artifacts.find((a) => a.id === store.ui.focus.id)?.dataset === d.id)?.id ?? store.project.datasets[0]?.id ?? null;
  let state = { status: 'idle' }; // idle | looking | absent | running | ready | failed
  let shown = 'hill1';
  let generation = 0;

  const dataset = () => store.project.datasets.find((d) => d.id === chosen) ?? store.project.datasets[0] ?? null;

  async function specOf(d) {
    const open = await app.ensureDataset(d);
    if (open.status !== 'open') throw new Error(open.error ?? `${d.name} could not be opened.`);
    const loci = Object.keys(open.summary.samples.reduce((acc, s) => Object.assign(acc, s.loci), {}));
    const filter = datasetFilter(store.project, d);
    return { dataset: d.id, definition: datasetDefinition(d, loci), filter: filter?.rules ?? {}, filterRecord: filter };
  }

  async function load() {
    const mine = ++generation;
    const d = dataset();
    renderHead();
    if (!d) {
      state = { status: 'idle' };
      render();
      return;
    }
    state = { status: 'looking' };
    render();
    try {
      const spec = await specOf(d);
      const answer = await getJSON('api/diversity', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ dataset: spec.dataset, definition: spec.definition, filter: spec.filter, start: false }) });
      if (mine !== generation) return;
      if (answer.ready) {
        const read = await getJSON(`api/diversity/${encodeURIComponent(answer.artifact)}`);
        if (mine !== generation) return;
        state = { status: 'ready', artifact: answer.artifact, result: read.result, manifest: read.artifact, spec };
        keep(d, spec, read);
      } else {
        state = { status: answer.job ? 'running' : 'absent', artifact: answer.artifact, job: answer.job, spec };
      }
    } catch (error) {
      if (mine !== generation) return;
      state = { status: 'failed', error: error.message };
    }
    render();
  }

  // The result in the project's record (the project tree lists it; the methods paragraph says it).
  function keep(d, spec, read) {
    if (store.project.artifacts.some((a) => a.id === read.artifact.id)) return;
    store.commit(addResult(store.project, {
      id: read.artifact.id, kind: 'diversity', name: `Diversity · ${d.name}`, dataset: d.id, definition: spec.definition.name ?? '',
      filter: spec.filterRecord ? `${spec.filterRecord.name} v${spec.filterRecord.version}` : null,
      method: `Hill numbers, Chao1, ACE; ${read.result.options.replicates} bootstrap replicates`, made: read.artifact.created,
      options: read.result.options, unit: read.result.unit, describe: read.result.describe,
    }), `Measure the diversity of ${d.name}`, ['project']);
  }

  async function measure() {
    const d = dataset();
    if (!d) return;
    try {
      const spec = state.spec ?? await specOf(d);
      const answer = await getJSON('api/diversity', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ dataset: spec.dataset, definition: spec.definition, filter: spec.filter }) });
      if (answer.ready) {
        load();
        return;
      }
      state = { status: 'running', artifact: answer.artifact, job: answer.job, spec };
      app.log(`Measuring the diversity of ${d.name} (${spec.definition.name ?? 'its definition'}).`);
      render();
      const stop = app.jobs.onFinished((job) => {
        if (job.id !== answer.job) return;
        stop();
        if (job.state === 'complete') load();
        else {
          state = { status: 'failed', error: job.error || `The job ended ${job.state}.` };
          render();
        }
      });
    } catch (error) {
      toast(error.message, { kind: 'error' });
    }
  }

  function renderHead() {
    clear(datasetSelect);
    for (const d of store.project.datasets) datasetSelect.append(h('option', { value: d.id, selected: d.id === dataset()?.id }, d.name));
    datasetSelect.disabled = !store.project.datasets.length;
  }

  function render() {
    clear(body);
    const d = dataset();
    measureButton.hidden = !d || state.status === 'ready' || state.status === 'running';
    if (!d) {
      body.append(h('div.empty', icon('repertoire'), h('h3', 'No data yet'),
        h('p', 'Open data, define clonotypes, and measure each sample\'s diversity here, every measure with its interval.'),
        h('div.btn-row', { style: { justifyContent: 'center', marginTop: '10px' } },
          h('button.btn.primary', { type: 'button', onclick: () => app.openPathDialog() }, icon('folder'), 'Open data'),
          h('button.btn', { type: 'button', onclick: () => app.showExamples() }, icon('flask'), 'Example data'))));
      return;
    }
    const spec = state.spec;
    if (spec) {
      body.append(h('p.muted.small.repertoire-spec', 'Clonotypes: ', h('b', spec.definition.name ?? 'the definition'), spec.filterRecord ? ` · filter "${spec.filterRecord.name}" v${spec.filterRecord.version}` : ' · no filter',
        ` · abundance in ${UNITS[datasetUnit(d)] ?? 'counts'}`));
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
        h('h3', 'Each sample\'s diversity'),
        h('p', 'Measured by the program over every sample\'s clonotypes under this definition and filter: the Hill numbers and their profile, Shannon\'s entropy, Simpson\'s index, evenness, clonality, Gini\'s coefficient, the largest clonotypes\' shares, clonal space homeostasis, and the whole repertoire\'s richness (Chao1, ACE) and Shannon and Simpson diversity, each with a 95% interval from 200 bootstrap replicates. The result is kept with the project, with how it was made.'),
        state.status === 'running'
          ? h('div.example-progress', h('div.progress', h('div', { style: { width: `${Math.round((job?.progress ?? 0) * 100)}%` } })), h('span.muted.small', job?.message ?? 'Waiting'))
          : h('div.btn-row', h('button.btn.primary', { type: 'button', onclick: () => measure() }, icon('play'), 'Measure diversity'))));
      return;
    }
    if (state.status !== 'ready') return;
    const result = state.result;
    const samples = result.samples;
    const unit = UNITS[result.unit] ?? 'counts';
    // Groups by condition: the samples' annotation, else their metadata's.
    const summary = app.datasetStates.get(d.id)?.summary;
    const conditionOf = (name) => annotation(store.project, d.id, name).fields.condition
      ?? (summary ? fieldsFromMetadata(summary.samples.find((x) => x.name === name) ?? {}).condition : undefined);
    const conditions = [...new Set(samples.map((s) => conditionOf(s.name)).filter(Boolean))];
    const classOf = (name) => {
      const c = conditionOf(name);
      return c ? `plot-s${(conditions.indexOf(c) % 6) + 1}` : 'plot-s1';
    };
    const low = samples.filter((s) => Number.isFinite(s.result.measures.coverage?.estimate) && s.result.measures.coverage.estimate < 0.9);
    const notes = [];
    if (low.length) notes.push(h('div.callout.warn', h('p', `${low.length === samples.length ? 'Every sample has' : `${low.length} of ${samples.length} samples have`} a sample coverage below 0.9 (${low.slice(0, 4).map((s) => `${s.name} ${s.result.measures.coverage.estimate.toFixed(2)}`).join(', ')}${low.length > 4 ? ', …' : ''}): there, Chao1 and the estimated Shannon diversity are lower bounds, and in ClonoLoom's simulations their intervals did not hold the truth. Compare such samples at equal coverage instead.`)));
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
    const legend = conditions.length ? h('div.plot-legend', ...conditions.map((c, i) => h('span', h(`span.swatch.plot-s${(i % 6) + 1}`), c))) : null;
    body.append(h('figure.plot-card',
      h('figcaption', h('span', 'Each sample'), picker),
      h('div.plot-holder', { html: plot.svg }),
      legend,
      h('p.muted.small', info.words, ' ', 'Bars are 95% intervals (Chao1\'s from its variance, the others from 200 bootstrap replicates of each sample).')));

    // The Hill profile and homeostasis.
    const profileQs = [0, 0.25, 0.5, 0.75, 1, 1.25, 1.5, 1.75, 2, 2.5, 3, 3.5, 4];
    const profile = lines(samples.map((s) => ({ label: s.name, className: classOf(s.name), points: profileQs.map((q) => ({ x: q, y: s.result.measures[`hill@${q}`]?.estimate ?? NaN })) })),
      { label: 'Hill numbers by order', xLabel: 'Order q', yLabel: 'Effective clonotypes', log: true });
    const homeo = stackedBars(samples.map((s) => ({ label: s.name, parts: Object.fromEntries(HOMEOSTASIS.map((b) => [b.key, s.result.measures[`homeostasis.${b.key}`]?.estimate ?? 0])) })), HOMEOSTASIS, { label: 'Clonal space homeostasis', share: true });
    body.append(h('div.plot-grid',
      h('figure.plot-card', h('figcaption', 'The Hill profile'), h('div.plot-holder', { html: profile.svg }), legend?.cloneNode(true),
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

  // What the shown result depends on: the dataset, its definition and its filter as chosen.
  const choices = () => {
    const d = dataset();
    return JSON.stringify([d?.id ?? null, d?.definition ?? null, d?.filter ?? null]);
  };
  let shownChoices = choices();

  // A running job's progress, as the program reports it; a new definition or filter, measured again
  // or looked up.
  function update(topics) {
    if (topics.has('jobs') && state.status === 'running') render();
    if (topics.has('project') || topics.has('project-loaded') || topics.has('datasets')) {
      if (!store.project.datasets.some((d) => d.id === chosen)) chosen = store.project.datasets[0]?.id ?? null;
      if (topics.has('project-loaded') || choices() !== shownChoices) {
        shownChoices = choices();
        load();
      } else renderHead();
    }
  }

  app.repertoireView = { measure: () => measure(), load: () => load(), state: () => state };
  load();
  return {
    update,
    destroy() {
      app.repertoireView = null;
      root.remove();
    },
  };
}
