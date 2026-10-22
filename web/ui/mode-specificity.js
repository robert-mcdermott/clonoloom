// The Specificity view (clonoloom-spec/requirements.md, S1–S7 and R6; roadmap.md, wave 4), each
// part an analysis the program runs as a job and keeps as a result:
//   - Neighbors (slice 2: S1): each T-cell clonotype of a dataset compared with every other of the
//     same chains by TCRdist, as tcrdist3 computes it (api_tcrdist.go, internal/tcrdist): with the
//     V genes' CDR1, CDR2 and CDR2.5 (the reference data tcrdist3-loops, downloaded on request) or
//     the CDR3s alone; each clonotype's distance to its nearest, the clonotypes with the most
//     neighbors within the radius, and a chosen clonotype's nearest, their distance loop by loop.
//     Similar receptors may share a specificity; distance alone does not show that they do.
//   - Generation (slice 3: R6): each clonotype's generation probability, the probability that
//     recombination alone makes its CDR3s, as OLGA computes it (api_pgen.go, internal/pgen), under
//     OLGA's models (the reference data olga-models) or ClonoLoom's own model of human TRB; each
//     sample's distribution, and the clonotypes by abundance or probability.

import { h, icon, clear } from './dom.js';
import { toast } from './overlays.js';
import { columns, intervals } from '../lib/plot.js';
import { UNITS } from '../lib/datasets.js';
import { datasetDefinition } from '../lib/definitions.js';
import { datasetFilter } from '../lib/filters.js';
import { addResult } from '../lib/results.js';
import { MODELS, ORGANISMS, PGEN_ORDERS, PGEN_ORGANISMS, genesOf, junctionsOf, log10Histogram, nearestHistogram, optionWords, pgenOptions, pgenOrder, pgenRequest, pgenWords, probabilityWords, radiusOf, regionWords, tcrdistOptions, tcrdistRequest, tenTo, usePgen, useTCRdist } from '../lib/specificity.js';
import { exportable, madeBy } from './figures.js';
import { exportResult } from './exports.js';

const PARTS = { neighbors: 'Neighbors', generation: 'Generation' };
// The analysis each part shows, the part each opens in, its API, and what the head's button says.
const KIND_OF = { neighbors: 'tcrdist', generation: 'pgen' };
const PART_OF = { tcrdist: 'neighbors', pgen: 'generation' };
const KINDS = ['tcrdist', 'pgen'];
const START = { tcrdist: 'Compare the receptors', pgen: 'Compute the probabilities' };
const SHOWN = 100;

const count = (n) => Number(n ?? 0).toLocaleString('en-US');
const plural = (n, word, words = `${word}s`) => `${count(n)} ${n === 1 ? word : words}`;

async function getJSON(url, options = {}) {
  const response = await fetch(url, { cache: 'no-store', ...options });
  const answer = await response.json().catch(() => ({}));
  if (!response.ok) {
    const error = new Error(answer.error ?? `${url}: HTTP ${response.status}`);
    error.code = answer.code;
    throw error;
  }
  return answer;
}
const postJSON = (url, body) => getJSON(url, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });

// What each part says before its result is there.
export const NEIGHBORS_WORDS = 'The program compares each T-cell clonotype with every other of the same chains by TCRdist (Dash et al. 2017), as tcrdist3 computes it: the CDR3 (weighted 3) and the V gene\'s CDR1, CDR2 and CDR2.5 (each weighted 1), each pair of residues by how BLOSUM62 scores them, and a pair of chains as the sum of its chains. Each clonotype\'s neighbors within the radius are counted, and its nearest found. The result is kept with the project.';
export const GENERATION_WORDS = 'The program computes each clonotype\'s generation probability: the probability that V(D)J recombination alone makes its CDR3 (Sethna et al. 2019), summed over every way of making it (each gene, deletion and insertion), as OLGA computes it, under the recombination model of its chain; a pair\'s is the product of its chains\'. With the V and J genes, it is the probability of the CDR3 with those genes. The result is kept with the project.';

export function mountSpecificity(app, container) {
  const { store } = app;
  const datasetSelect = h('select.input.small', { 'aria-label': 'Dataset', onchange: (e) => { chosen = e.target.value; load(); } });
  const partSwitch = h('div.segmented', { role: 'tablist', 'aria-label': 'Part' });
  const startButton = h('button.btn.primary.small', { type: 'button', onclick: () => run() });
  const exportButton = h('button.btn.small', { type: 'button', title: 'Export this result as CSV, with a NOTICE of the data\'s source and license', onclick: () => {
    const r = results[KIND_OF[part]];
    if (r.status === 'ready') exportResult(app, dataset(), r.artifact);
  } }, icon('download'), 'Export');
  const head = h('div.workbench-head', h('h1', icon('receptor'), 'Specificity'), partSwitch,
    h('span.spacer'), h('label.kv-inline', h('span.muted', 'Dataset'), datasetSelect), startButton, exportButton);
  const body = h('div.view-body.specificity-view');
  const root = h('div.view', head, h('div.workbench-scroll', body));
  container.append(root);
  const focusedResult = store.ui.focus?.kind === 'result' ? store.project.artifacts.find((a) => a.id === store.ui.focus.id) : null;
  let chosen = store.ui.focus?.kind === 'dataset' ? store.ui.focus.id : focusedResult?.dataset ?? store.project.datasets[0]?.id ?? null;
  let part = PART_OF[focusedResult?.kind] ?? 'neighbors';
  // The dataset: idle | loading | ready | failed; each analysis's result: idle | looking | absent |
  // running | ready | failed | unsupported (and no-loops, no-models: the reference data needed).
  let state = { status: 'idle' };
  const results = Object.fromEntries(KINDS.map((k) => [k, { status: 'idle' }]));
  let spec = null;
  let selected = null;
  let order = 'abundant';
  let nearest = { status: 'idle' };
  let generation = 0;
  let latest = Promise.resolve();
  const settled = () => {
    const p = latest;
    return p.then(() => (p === latest ? undefined : settled()));
  };

  const dataset = () => store.project.datasets.find((d) => d.id === chosen) ?? store.project.datasets[0] ?? null;
  const request = (extra = {}) => ({ dataset: spec.dataset, definition: spec.definition, filter: spec.filter, tcrdist: tcrdistRequest(spec.options), ...extra });
  const pgenBody = (extra = {}) => ({ dataset: spec.dataset, definition: spec.definition, filter: spec.filter, pgen: pgenRequest(spec.pgen), ...extra });
  const bodyOf = (kind, extra) => (kind === 'pgen' ? pgenBody(extra) : request(extra));

  function load() {
    latest = reload();
    return latest;
  }

  async function reload() {
    const mine = ++generation;
    const d = dataset();
    renderHead();
    if (!d) {
      state = { status: 'idle' };
      for (const k of KINDS) results[k] = { status: 'idle' };
      render();
      return;
    }
    if (state.status !== 'ready' || state.dataset !== d.id) {
      state = { status: 'loading' };
      for (const k of KINDS) results[k] = { status: 'looking' };
      selected = null;
      render();
    }
    try {
      const open = await app.ensureDataset(d);
      if (open.status !== 'open') throw new Error(open.error ?? `${d.name} could not be opened.`);
      if (mine !== generation) return;
      const loci = Object.keys(open.summary.samples.reduce((acc, s) => Object.assign(acc, s.loci), {}));
      const filter = datasetFilter(store.project, d);
      spec = { dataset: d.id, definition: datasetDefinition(d, loci), filter: filter?.rules ?? {}, filterRecord: filter, options: tcrdistOptions(d), pgen: pgenOptions(d), loci };
      state = { status: 'ready', dataset: d.id, summary: open.summary };
    } catch (error) {
      if (mine !== generation) return;
      state = { status: 'failed', error: error.message };
      for (const k of KINDS) results[k] = { status: 'failed', error: error.message };
      render();
      return;
    }
    await Promise.all(KINDS.map((k) => look(mine, d, k)));
    if (mine === generation) render();
  }

  // Why an analysis cannot run on the dataset as defined, or null.
  function unsupported(kind) {
    if (spec.definition.vendor) return 'vendor';
    if (kind === 'pgen') return null;
    if ((spec.definition.junction ?? 'aa') !== 'aa') return 'aa';
    if (!spec.loci.some((l) => l.startsWith('TR'))) return 'tcells';
    return null;
  }

  async function look(mine, d, kind) {
    const why = unsupported(kind);
    if (why) {
      results[kind] = { status: 'unsupported', why };
      return;
    }
    try {
      const answer = await postJSON(`api/${kind}`, bodyOf(kind, { start: false }));
      if (mine !== generation) return;
      if (answer.ready) {
        const read = await getJSON(`api/${kind}/${encodeURIComponent(answer.artifact)}`);
        if (mine !== generation) return;
        results[kind] = { status: 'ready', artifact: answer.artifact, result: read.result, manifest: read.artifact };
        keep(d, kind, read);
        if (kind === 'tcrdist' && selected && !read.result.clonotypes.some((c) => c.id === selected)) selected = null;
      } else {
        results[kind] = { status: answer.job ? 'running' : 'absent', artifact: answer.artifact, job: answer.job };
        if (answer.job) watch(kind, answer.job);
      }
    } catch (error) {
      if (mine !== generation) return;
      results[kind] = ['no-loops', 'no-models'].includes(error.code) ? { status: error.code } : { status: 'failed', error: error.message };
    }
  }

  // The result in the project's record (the project tree lists it; the methods paragraph says it).
  function keep(d, kind, read) {
    if (store.project.artifacts.some((a) => a.id === read.artifact.id)) return;
    const r = read.result;
    if (kind === 'pgen') {
      store.commit(addResult(store.project, {
        id: read.artifact.id, kind: 'pgen', dataset: d.id, definition: spec.definition.name ?? '',
        filter: spec.filterRecord ? `${spec.filterRecord.name} v${spec.filterRecord.version}` : null,
        made: read.artifact.created, unit: r.unit, describe: r.describe, name: `Generation probabilities · ${d.name}`,
        method: `Generation probabilities, ${pgenWords({ models: r.models, organism: r.organism, genes: r.genes })}`,
        models: r.models, set: { name: r.set.name, checksum: r.set.checksum, citation: r.set.citation }, organism: r.organism, genes: r.genes, loci: r.loci,
        clonotypes: r.clonotypes.length, computed: r.clonotypes.filter((c) => c.pgen >= 0).length,
      }), `Compute the generation probabilities of ${d.name}`, ['project']);
      return;
    }
    store.commit(addResult(store.project, {
      id: read.artifact.id, kind: 'tcrdist', dataset: d.id, definition: spec.definition.name ?? '',
      filter: spec.filterRecord ? `${spec.filterRecord.name} v${spec.filterRecord.version}` : null,
      made: read.artifact.created, unit: r.unit, describe: r.describe, name: `TCRdist · ${d.name}`,
      method: `TCRdist between clonotypes, ${optionWords({ radius: r.radius, cdr3Only: r.cdr3Only, organism: r.organism ?? '' })}`,
      radius: r.radius, cdr3Only: r.cdr3Only, organism: r.organism ?? '', loops: r.loops ? { file: r.loops.file, sha256: r.loops.sha256 } : null,
      compared: r.compared, withNeighbors: r.withNeighbors, groups: r.groups.map((g) => ({ loci: g.loci, receptors: g.receptors })),
    }), `Compare the receptors of ${d.name}`, ['project']);
  }

  const watching = new Set();
  function watch(kind, jobId) {
    if (watching.has(jobId)) return;
    watching.add(jobId);
    const stop = app.jobs.onFinished((job) => {
      if (job.id !== jobId) return;
      stop();
      watching.delete(jobId);
      if (job.state === 'complete') load();
      else {
        results[kind] = { status: 'failed', error: job.error || `The job ended ${job.state}.` };
        render();
      }
    });
  }

  async function run(kind = KIND_OF[part]) {
    const d = dataset();
    if (!d || !spec || results[kind].status === 'unsupported') return;
    try {
      const answer = await postJSON(`api/${kind}`, bodyOf(kind));
      if (answer.ready) {
        load();
        return;
      }
      results[kind] = { status: 'running', artifact: answer.artifact, job: answer.job };
      app.log(kind === 'pgen'
        ? `Compute the generation probabilities of ${d.name}: ${pgenWords(spec.pgen)}.`
        : `Compare the receptors of ${d.name} by TCRdist: ${optionWords(spec.options, spec.definition)}.`);
      render();
      watch(kind, answer.job);
    } catch (error) {
      if (['no-loops', 'no-models'].includes(error.code)) {
        results[kind] = { status: error.code };
        render();
        return;
      }
      toast(error.message, { kind: 'error' });
    }
  }

  // Options kept with the dataset; the result for them shown, or asked for.
  function choose(changes) {
    const d = dataset();
    if (!d) return;
    const next = useTCRdist(store.project, d.id, changes);
    if (next === store.project) return;
    store.commit(next, `Choose how TCRdist compares the clonotypes of ${d.name}`);
  }
  function choosePgen(changes) {
    const d = dataset();
    if (!d) return;
    const next = usePgen(store.project, d.id, changes);
    if (next === store.project) return;
    store.commit(next, `Choose how the generation probabilities of ${d.name} are computed`);
  }

  function show(key) {
    part = PARTS[key] ? key : part;
    render();
  }

  async function showNearest(id) {
    selected = selected === id ? null : id;
    nearest = { status: selected ? 'loading' : 'idle' };
    render();
    if (!selected) return;
    try {
      const answer = await postJSON('api/tcrdist/nearest', request({ clonotype: id, n: 10 }));
      if (selected !== id) return;
      nearest = { status: 'ready', ...answer };
    } catch (error) {
      if (selected !== id) return;
      nearest = { status: 'failed', error: error.message };
    }
    render();
  }

  function renderHead() {
    clear(datasetSelect);
    for (const d of store.project.datasets) datasetSelect.append(h('option', { value: d.id, selected: d.id === dataset()?.id }, d.name));
    datasetSelect.disabled = !store.project.datasets.length;
  }

  function pgenOptionsRow() {
    const o = spec.pgen;
    const genesPossible = !['family', 'none'].includes(spec.definition.v) && !['family', 'none'].includes(spec.definition.j);
    const models = h('select.input.small', { 'aria-label': 'Models', onchange: (e) => choosePgen({ models: e.target.value }) },
      ...Object.entries(MODELS).map(([k, label]) => h('option', { value: k, selected: k === o.models }, label)));
    const organism = h('select.input.small', { 'aria-label': 'Organism', disabled: o.models === 'clonoloom', onchange: (e) => choosePgen({ organism: e.target.value }) },
      ...Object.entries(PGEN_ORGANISMS).map(([k, label]) => h('option', { value: k, selected: k === o.organism }, label)));
    const genes = h('input', { type: 'checkbox', checked: o.genes && genesPossible, disabled: !genesPossible, onchange: (e) => choosePgen({ genes: e.target.checked }) });
    return h('div.options-row',
      h('label.kv-inline', h('span.muted', 'Models'), models),
      h('label.kv-inline', h('span.muted', 'Organism'), organism),
      h('label.check', { title: genesPossible ? 'The probability of the CDR3 with its V and J genes, not of the CDR3 alone' : 'The clonotypes are defined without their genes' }, genes, 'With the V and J genes'),
      h('span.muted.small', pgenWords({ ...o, genes: o.genes && genesPossible })));
  }

  function optionsRow() {
    const o = spec.options;
    const radius = h('input.input.small', { type: 'number', min: 1, max: 1000, step: 1, value: o.radius || '', placeholder: String(radiusOf(o, spec.definition)), 'aria-label': 'Radius',
      onchange: (e) => choose({ radius: Math.max(0, Math.round(Number(e.target.value) || 0)) }) });
    const cdr3 = h('input', { type: 'checkbox', checked: o.cdr3Only, onchange: (e) => choose({ cdr3Only: e.target.checked }) });
    const organism = h('select.input.small', { 'aria-label': 'Organism', disabled: o.cdr3Only, onchange: (e) => choose({ organism: e.target.value }) },
      ...Object.entries(ORGANISMS).map(([k, label]) => h('option', { value: k, selected: k === o.organism }, label)));
    return h('div.options-row',
      h('label.kv-inline', h('span.muted', 'Radius'), radius),
      h('label.check', cdr3, 'Compare the CDR3s alone'),
      h('label.kv-inline', h('span.muted', 'Loops of'), organism),
      h('span.muted.small', optionWords(o, spec.definition)));
  }

  // The body drawn again where it was scrolled to.
  const scroller = root.querySelector('.workbench-scroll');
  function render() {
    const top = scroller.scrollTop;
    draw();
    scroller.scrollTop = top;
  }

  function draw() {
    clear(partSwitch);
    for (const [key, label] of Object.entries(PARTS)) {
      partSwitch.append(h(`button${key === part ? '.active' : ''}`, { type: 'button', role: 'tab', 'aria-selected': key === part ? 'true' : 'false', onclick: () => show(key) }, label));
    }
    clear(body);
    const d = dataset();
    const kind = KIND_OF[part];
    const result = results[kind];
    clear(startButton).append(icon('play'), START[kind]);
    startButton.hidden = !d || result.status !== 'absent';
    exportButton.hidden = !d || result.status !== 'ready';
    if (!d) {
      body.append(h('div.empty', icon('receptor'), h('h3', 'No data yet'),
        h('p', 'Open T-cell receptors (single-cell pairs, or bulk β chains) and find the receptors like each other here: those that may share a specificity.'),
        h('div.btn-row', { style: { justifyContent: 'center', marginTop: '10px' } },
          h('button.btn.primary', { type: 'button', onclick: () => app.openPathDialog() }, icon('folder'), 'Open data'),
          h('button.btn', { type: 'button', onclick: () => app.showExamples() }, icon('flask'), 'Example data'))));
      return;
    }
    if (state.status === 'loading' || state.status === 'idle') {
      body.append(h('p.muted', 'Reading the dataset…'));
      return;
    }
    if (state.status === 'failed') {
      body.append(h('div.callout.danger', h('p', state.error)));
      return;
    }
    if (result.status === 'unsupported') {
      const words = {
        tcells: 'TCRdist compares T-cell receptors, and this dataset has none: open TRA, TRB, TRG or TRD chains.',
        aa: 'TCRdist compares the junctions\' amino acids: choose a clonotype definition by amino-acid junction in the Clonotypes view.',
        vendor: `${kind === 'pgen' ? 'Generation probabilities need' : 'TCRdist needs'} clonotypes defined by their junctions, not by the source's own clonotypes: choose another definition in the Clonotypes view.`,
      };
      body.append(h('div.callout', h('p', words[result.why])));
      return;
    }
    if (kind === 'pgen') {
      drawGeneration(d, result);
      return;
    }
    body.append(optionsRow());
    if (result.status === 'looking' || result.status === 'idle') {
      body.append(h('p.muted', 'Reading…'));
      return;
    }
    if (result.status === 'failed') {
      body.append(h('div.callout.danger', h('p', result.error)));
      return;
    }
    if (result.status === 'no-loops') {
      body.append(h('div.card.repertoire-start',
        h('h3', 'The V genes\' loops'),
        h('p', 'TCRdist compares each chain\'s CDR1, CDR2 and CDR2.5 as well as its CDR3: the loops each V gene gives, as tcrdist3\'s database of genes aligns them (1.2 MB, MIT; made from IMGT\'s germline genes). ClonoLoom downloads it once, when you ask, and names its checksum in every result.'),
        h('div.btn-row',
          h('button.btn.primary', { type: 'button', onclick: () => app.showReferences() }, icon('download'), 'Reference data…'),
          h('button.btn', { type: 'button', onclick: () => choose({ cdr3Only: true }) }, 'Compare the CDR3s alone'))));
      return;
    }
    if (result.status === 'absent' || result.status === 'running') {
      const job = result.job ? app.jobs.get(result.job) : null;
      body.append(h('div.card.repertoire-start',
        h('h3', 'Each clonotype against every other'),
        h('p', NEIGHBORS_WORDS),
        result.status === 'running'
          ? h('div.example-progress', h('div.progress', h('div', { style: { width: `${Math.round((job?.progress ?? 0) * 100)}%` } })), h('span.muted.small', job?.message ?? 'Waiting'))
          : h('div.btn-row', h('button.btn.primary', { type: 'button', onclick: () => run() }, icon('play'), 'Compare the receptors'))));
      return;
    }
    renderResult(d, result);
  }

  function renderResult(d, result) {
    const r = result.result;
    body.append(h('div.callout', h('p', 'Receptors this alike may recognize the same antigen: Dash et al. found an epitope\'s receptors far nearer each other than unrelated ones. Distance suggests a shared specificity; it does not show one.')));
    const groups = r.groups.map((g) => `${plural(g.receptors, 'clonotype')} of ${g.loci.join(' and ')}`).join(', ');
    const excluded = Object.entries(r.excluded ?? {}).map(([why, n]) => `${count(n)} with ${why}`);
    const loops = r.cdr3Only
      ? `the CDR3s alone${r.why ? `, as ${r.why}` : ''}`
      : `the CDR3s and the V genes' loops (${r.organism}, from ${r.loops.file}, SHA-256 ${r.loops.sha256.slice(0, 12)}…)`;
    body.append(h('div.card.summary-card',
      h('p', `${plural(r.compared, 'clonotype')} compared (${groups || 'none'}), each with every other of the same chains, comparing ${loops}. `,
        h('b', `${count(r.withNeighbors)} (${r.compared ? Math.round((100 * r.withNeighbors) / r.compared) : 0}%)`), ` have a neighbor within ${r.radius} TCRdist units.`),
      excluded.length ? h('p.muted.small', `Not compared: ${excluded.join('; ')}.`) : null,
      Object.keys(r.alleles ?? {}).length ? h('p.muted.small', `V genes read as an allele they do not name: ${Object.entries(r.alleles).slice(0, 6).map(([v, a]) => `${v} as ${a}`).join(', ')}${Object.keys(r.alleles).length > 6 ? ` and ${count(Object.keys(r.alleles).length - 6)} more` : ''}.`) : null));
    const bins = nearestHistogram(r.clonotypes);
    if (bins.length) {
      const plot = columns(bins, { label: 'Distance to the nearest clonotype', xLabel: 'TCRdist to the nearest', yLabel: 'Clonotypes', width: 640 });
      const note = `Each clonotype's distance to its nearest other clonotype of the same chains, in bins of 6 TCRdist units, each labeled by its first; the radius is ${r.radius}.`;
      body.append(exportable(h('figure.plot-card',
        h('figcaption', 'Distance to the nearest clonotype'),
        h('div.plot-holder', { html: plot.svg }),
        h('p.muted.small', note)), () => ({
        name: `${d.name} tcrdist`, title: `Distance to the nearest clonotype, ${d.name}`, plot: plot.svg, legend: [],
        caption: [note, `${d.name}: ${plural(r.compared, 'clonotype')}, comparing ${loops}. Clonotypes: ${r.describe}.`, madeBy(app.version)],
      })));
    }
    const rows = [...r.clonotypes].sort((a, b) => b.neighbors - a.neighbors || (a.nearest ? a.distance : Infinity) - (b.nearest ? b.distance : Infinity) || b.total - a.total || (a.id < b.id ? -1 : 1));
    const unit = UNITS[r.unit] ?? 'count';
    body.append(h('div.section-title', `The clonotypes with the most neighbors · ${count(Math.min(SHOWN, rows.length))} of ${count(rows.length)}`),
      h('div.table-scroll', { tabIndex: 0 }, h('table.data.selectable',
        h('thead', h('tr', h('th', 'Junctions'), h('th', 'Genes'), h('th.r', unit.charAt(0).toUpperCase() + unit.slice(1)), h('th.r', 'Samples'), h('th.r', `Within ${r.radius}`), h('th.r', 'Nearest'))),
        h('tbody', ...rows.slice(0, SHOWN).flatMap((c) => {
          const row = h(`tr${c.id === selected ? '.selected' : ''}`, { tabIndex: 0, onclick: () => showNearest(c.id), onkeydown: (e) => { if (e.key === 'Enter') showNearest(c.id); } },
            h('td.mono', junctionsOf(c)), h('td.small', genesOf(c)), h('td.r', count(c.total)), h('td.r', count(c.samples)), h('td.r', count(c.neighbors)), h('td.r', c.nearest ? count(c.distance) : '—'));
          return c.id === selected ? [row, h('tr.detail', h('td', { colSpan: 6 }, nearestPanel(c)))] : [row];
        })))),
      h('p.muted.small', 'Choose a clonotype to see its nearest, and how far apart each loop is.'),
      h('p.muted.small', `Compared ${new Date(result.manifest.created).toLocaleString()} by ClonoLoom ${result.manifest.lineage?.software?.[0]?.version ?? ''}; kept as ${result.artifact}. Clonotypes: ${r.describe}.`));
  }

  // --- Generation ------------------------------------------------------------------------------

  function drawGeneration(d, result) {
    body.append(pgenOptionsRow());
    if (result.status === 'looking' || result.status === 'idle') {
      body.append(h('p.muted', 'Reading…'));
      return;
    }
    if (result.status === 'failed') {
      body.append(h('div.callout.danger', h('p', result.error)));
      return;
    }
    if (result.status === 'no-models') {
      body.append(h('div.card.repertoire-start',
        h('h3', 'OLGA\'s recombination models'),
        h('p', 'OLGA\'s default models (Sethna et al. 2019) describe recombination of human TRA, TRB, IGH, IGK and IGL and mouse TRA and TRB, each inferred from unproductive rearrangements by IGoR (Marcou et al. 2018): 28 files, 2.2 MB, from OLGA\'s repository (GPL-3.0). ClonoLoom downloads them once, when you ask, and names their checksum in every result. Its own model of human TRB, the simulator\'s, is built in.'),
        h('div.btn-row',
          h('button.btn.primary', { type: 'button', onclick: () => app.showReferences() }, icon('download'), 'Reference data…'),
          h('button.btn', { type: 'button', onclick: () => choosePgen({ models: 'clonoloom' }) }, 'Use ClonoLoom\'s model'))));
      return;
    }
    if (result.status === 'absent' || result.status === 'running') {
      const job = result.job ? app.jobs.get(result.job) : null;
      body.append(h('div.card.repertoire-start',
        h('h3', 'How likely recombination is to make each receptor'),
        h('p', GENERATION_WORDS),
        result.status === 'running'
          ? h('div.example-progress', h('div.progress', h('div', { style: { width: `${Math.round((job?.progress ?? 0) * 100)}%` } })), h('span.muted.small', job?.message ?? 'Waiting'))
          : h('div.btn-row', h('button.btn.primary', { type: 'button', onclick: () => run('pgen') }, icon('play'), START.pgen))));
      return;
    }
    renderGeneration(d, result);
  }

  function renderGeneration(d, result) {
    const r = result.result;
    const rows = r.clonotypes;
    const computed = rows.filter((c) => c.pgen >= 0);
    body.append(h('div.callout', h('p', 'A receptor recombination makes often is likely to be found in many people by chance alone (Elhanati et al. 2018); one it rarely makes, found in several samples or expanded, is more likely to have been selected. The probability says how a receptor is made, not what it recognizes.')));
    if (r.models === 'clonoloom' && !spec.pgen.models) {
      body.append(h('div.callout', h('p', 'Computed with ClonoLoom\'s own model of human TRB. OLGA\'s models, of human TRA, TRB, IGH, IGK and IGL and mouse TRA and TRB, can be downloaded in ',
        h('a', { href: '#', onclick: (e) => { e.preventDefault(); app.showReferences(); } }, 'Reference data'), '.')));
    }
    const loci = Object.entries(r.loci ?? {}).map(([locus, model]) => `${locus} by ${model}`).join(', ');
    const without = Object.entries(r.without ?? {}).map(([why, n]) => `${count(n)} chains with ${why}`);
    const zero = computed.filter((c) => c.pgen === 0).length;
    body.append(h('div.card.summary-card',
      h('p', h('b', `${count(computed.length)} of ${plural(rows.length, 'clonotype')}`), ` have a generation probability: ${pgenWords({ models: r.models, organism: r.organism, genes: r.genes })}${loci ? `; ${loci}` : ''}.`,
        zero ? ` ${plural(zero, 'clonotype')} the model cannot make (a probability of 0).` : ''),
      without.length ? h('p.muted.small', `Without one: ${without.join('; ')}.`) : null,
      h('p.muted.small', `${r.set.name}; checksum ${r.set.checksum.length > 32 ? `${r.set.checksum.slice(0, 16)}…` : r.set.checksum}. ${r.set.citation}.`)));
    const sampled = r.samples.filter((s) => s.with > s.zero);
    if (sampled.length) {
      const lo = Math.floor(Math.min(...sampled.map((s) => s.quartiles[0]))) - 1;
      const plot = intervals(sampled.map((s) => ({ label: s.name, value: s.quartiles[1], lower: s.quartiles[0], upper: s.quartiles[2] })),
        { label: 'Generation probabilities by sample', from: lo, format: tenTo, width: 640, labelWidth: 160 });
      const note = 'Each sample\'s clonotypes\' generation probabilities (a pair\'s, the product of its chains\'): the median (the dot) and the middle half (the line), on a logarithmic scale.';
      body.append(exportable(h('figure.plot-card',
        h('figcaption', 'Generation probabilities by sample'),
        h('div.plot-holder', { html: plot.svg }),
        h('p.muted.small', note)), () => ({
        name: `${d.name} pgen samples`, title: `Generation probabilities by sample, ${d.name}`, plot: plot.svg, legend: [],
        caption: [note, `${d.name}: ${pgenWords({ models: r.models, organism: r.organism, genes: r.genes })}. Clonotypes: ${r.describe}.`, madeBy(app.version)],
      })));
    }
    const bins = log10Histogram(rows);
    if (bins.length) {
      const plot = columns(bins, { label: 'Generation probabilities', xLabel: 'log₁₀ generation probability', yLabel: 'Clonotypes', width: 640 });
      const note = 'The clonotypes\' generation probabilities, in bins of half a power of ten, each labeled by its first (those of 0 left out).';
      body.append(exportable(h('figure.plot-card',
        h('figcaption', 'The clonotypes\' generation probabilities'),
        h('div.plot-holder', { html: plot.svg }),
        h('p.muted.small', note)), () => ({
        name: `${d.name} pgen`, title: `Generation probabilities, ${d.name}`, plot: plot.svg, legend: [],
        caption: [note, `${d.name}: ${plural(computed.length, 'clonotype')}, ${pgenWords({ models: r.models, organism: r.organism, genes: r.genes })}. Clonotypes: ${r.describe}.`, madeBy(app.version)],
      })));
    }
    const ordered = pgenOrder(rows, order);
    const unit = UNITS[r.unit] ?? 'count';
    const orderSelect = h('select.input.small', { 'aria-label': 'Order', onchange: (e) => { order = e.target.value; render(); } },
      ...Object.entries(PGEN_ORDERS).map(([k, label]) => h('option', { value: k, selected: k === order }, label)));
    const chainWords = (c) => c.chains.map((ch, k) => `${ch.locus} ${probabilityWords(c.chain[k])}`).join(' · ');
    // A pair without a product (a chain without a model) shows the chains that have one.
    const pgenCell = (c) => (c.pgen < 0 && c.chain.some((p) => p >= 0)
      ? h('td.r', '— ', h('span.muted.small', `(${c.chains.map((ch, k) => (c.chain[k] >= 0 ? `${ch.locus} ${probabilityWords(c.chain[k])}` : '')).filter(Boolean).join(' · ')})`))
      : h('td.r', probabilityWords(c.pgen)));
    body.append(h('div.section-title', `The clonotypes · ${count(Math.min(SHOWN, ordered.length))} of ${count(ordered.length)}`),
      h('div.options-row', h('label.kv-inline', h('span.muted', 'First'), orderSelect)),
      h('div.table-scroll', { tabIndex: 0 }, h('table.data.selectable',
        h('thead', h('tr', h('th', 'Junctions'), h('th', 'Genes'), h('th.r', unit.charAt(0).toUpperCase() + unit.slice(1)), h('th.r', 'Samples'), h('th.r', 'Generation probability'))),
        h('tbody', ...ordered.slice(0, SHOWN).map((c) => h('tr', { tabIndex: 0, title: c.chains.length > 1 ? chainWords(c) : '', onclick: () => showInClonotypes(c.id), onkeydown: (e) => { if (e.key === 'Enter') showInClonotypes(c.id); } },
          h('td.mono', junctionsOf(c)), h('td.small', genesOf(c)), h('td.r', count(c.total)), h('td.r', count(c.samples)), pgenCell(c)))))),
      h('p.muted.small', 'Choose a clonotype to see it in the Clonotypes view. A pair\'s probability is the product of its chains\'; each chain\'s shows when you point at its row.'),
      h('p.muted.small', `Computed ${new Date(result.manifest.created).toLocaleString()} by ClonoLoom ${result.manifest.lineage?.software?.[0]?.version ?? ''}; kept as ${result.artifact}. Clonotypes: ${r.describe}.`));
  }

  function nearestPanel(c) {
    if (nearest.status === 'loading') return h('p.muted', 'Finding its nearest…');
    if (nearest.status === 'failed') return h('div.callout.danger', h('p', nearest.error));
    if (nearest.status !== 'ready') return h('span');
    const loci = c.chains.map((ch) => ch.locus);
    return h('div.nearest',
      h('table.data.compact',
        h('thead', h('tr', h('th', 'Nearest clonotypes'), h('th', 'Genes'), h('th.r', 'TCRdist'), h('th', 'Loop by loop'))),
        h('tbody', ...(nearest.nearest ?? []).map((n) => h('tr', h('td.mono', junctionsOf(n)), h('td.small', genesOf(n)), h('td.r', count(n.distance)), h('td.small', regionWords(n.regions, loci)))))),
      h('div.btn-row', h('button.btn.small', { type: 'button', onclick: () => showInClonotypes(c.id) }, icon('clonotypes'), 'Show in the Clonotypes view')));
  }

  // A clonotype in the Clonotypes view, with the inspector describing it.
  async function showInClonotypes(id) {
    const d = dataset();
    if (!d) return;
    app.focusItem({ kind: 'dataset', id: d.id });
    await app.setMode('clonotypes');
    const view = app.clonotypesView;
    if (!view) return;
    view.show(d.id);
    view.select(id);
  }

  let shownChoices = '';
  const choices = () => {
    const d = dataset();
    return JSON.stringify([d?.id ?? null, d?.definition ?? null, d?.filter ?? null, d?.tcrdist ?? null, d?.pgen ?? null]);
  };
  function update(topics) {
    if (topics.has('jobs') && results[KIND_OF[part]].status === 'running') render();
    if (topics.has('focus')) {
      const r = store.ui.focus?.kind === 'result' ? store.project.artifacts.find((a) => a.id === store.ui.focus.id) : null;
      if (PART_OF[r?.kind]) {
        const before = part;
        part = PART_OF[r.kind];
        if (r.dataset !== chosen) {
          chosen = r.dataset;
          shownChoices = choices();
          load();
        } else if (part !== before) render();
      }
    }
    if (topics.has('project') || topics.has('project-loaded') || topics.has('datasets')) {
      if (!store.project.datasets.some((d) => d.id === chosen)) chosen = store.project.datasets[0]?.id ?? null;
      const now = choices();
      if (topics.has('project-loaded') || now !== shownChoices) {
        shownChoices = now;
        load();
      } else renderHead();
    }
  }

  // Reference data downloaded (or removed): asked again.
  let olgaReady = app.references.list()?.some((e) => e.name === 'olga-models' && e.state === 'ready') ?? false;
  const unsubscribe = app.references.subscribe(() => {
    const list = app.references.list() ?? [];
    const ready = list.some((e) => e.name === 'olga-models' && e.state === 'ready');
    const changed = ready !== olgaReady;
    olgaReady = ready;
    if (results.tcrdist.status === 'no-loops' && list.some((e) => e.name === 'tcrdist3-loops' && e.state === 'ready')) load();
    else if (changed && spec && !spec.pgen.models) load();
    else if (results.pgen.status === 'no-models' && ready) load();
  });

  app.specificityView = {
    load: () => load(),
    settled,
    state: () => state,
    neighbors: () => results.tcrdist,
    generation: () => results.pgen,
    show: (key) => show(key),
    run: (kind) => run(kind),
    choose: (changes) => {
      choose(changes);
      return settled();
    },
    choosePgen: (changes) => {
      choosePgen(changes);
      return settled();
    },
    order: (by) => {
      order = PGEN_ORDERS[by] ? by : order;
      render();
    },
    // A clonotype's nearest shown (not hidden when it already is).
    select: (id) => {
      if (selected === id && nearest.status === 'ready') return Promise.resolve();
      selected = null;
      return showNearest(id);
    },
  };
  shownChoices = choices();
  load();
  return {
    update,
    destroy() {
      unsubscribe();
      app.specificityView = null;
      root.remove();
    },
  };
}
