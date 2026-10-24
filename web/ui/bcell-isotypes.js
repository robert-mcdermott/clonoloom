// Scientific counts and denominators are computed in Go; the browser displays saved results.
import { studySamples } from '../lib/datasets.js';
import { h, clear } from './dom.js';
import { addResult } from '../lib/results.js';
import { exportResult } from './exports.js';
import { toast } from './overlays.js';

async function json(url, body) {
  const response = await fetch(url, body ? { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) } : { cache: 'no-store' });
  const value = await response.json();
  if (!response.ok) throw new Error(value.error ?? `HTTP ${response.status}`);
  return value;
}
const td = (value) => h('td', String(value ?? 'Missing'));
const table = (headers, rows) => h('div.table-scroll', { tabIndex: 0 }, h('table.data', h('thead', h('tr', ...headers.map((s) => h('th', { scope: 'col' }, s)))), h('tbody', ...rows)));

export function mountIsotypes(app, container) {
  const { store } = app;
  const focused = () => store.ui.focus?.kind === 'result' ? store.project.artifacts.find((r) => r.id === store.ui.focus.id && r.kind === 'isotypes') : null;
  let chosen = focused()?.dataset ?? (store.ui.focus?.kind === 'dataset' ? store.ui.focus.id : store.project.datasets[0]?.id);
  let saved = focused()?.id ?? null, parent = '';
  let generation = 0, offset = 0, family = '', sample = '', level = 'class', left = '', right = '', kind = 'usage', page = null, detail = null;
  let status = 'idle', error = '', artifact = '', stopJob = null;
  const dataset = () => store.project.datasets.find((d) => d.id === chosen);
  const parents = () => store.project.artifacts.filter((r) => r.dataset === chosen && r.kind === 'families' && r.families?.threshold != null);
  const root = h('div.view-body'); container.append(root);
  const button = (label, fn, disabled = false) => h('button.btn.small', { type: 'button', disabled, onclick: () => Promise.resolve().then(fn).catch((e) => toast(e.message, { kind: 'error' })) }, label);
  function keep(answer) {
    if (store.project.artifacts.some((r) => r.id === answer.artifact.id)) return;
    const r = answer.result;
    store.commit(addResult(store.project, { id: answer.artifact.id, kind: 'isotypes', dataset: r.dataset, name: `Isotypes · ${r.datasetName}`, made: answer.artifact.created, method: 'Observed constant-region usage and unordered family co-occurrence', describe: r.description, isotypes: r.isotypeOptions, unit: r.unit, abundanceUnit: r.abundanceUnit, assigned: r.assigned, resolvedClass: r.resolvedClass, resolvedGene: r.resolvedGene, familyCount: answer.familyCount }), `Keep isotypes of ${r.datasetName}`, ['project']);
  }
  async function load(start = false) {
    const token = ++generation;
    stopJob?.(); stopJob = null; page = null; detail = null; app.inspector.render(); status = 'loading'; error = '';
    if (!parent) parent = parents().at(-1)?.id ?? '';
    render();
    if (!dataset() || (!saved && !parent)) { status = 'idle'; render(); return; }
    try {
      if (!saved) {
        const opened = await app.ensureDataset(dataset());
        if (token !== generation) return;
        if (opened.status !== 'open') throw new Error(opened.error ?? 'Open the dataset again.');
        const answer = await json('api/isotypes', { dataset: chosen, isotypes: { families: parent }, study: studySamples(store.project, chosen, opened.summary), start });
        if (token !== generation) return;
        artifact = answer.artifact;
        if (!answer.ready) {
          status = answer.job ? 'running' : 'absent'; render();
          if (answer.job) {
            const finished = (job) => {
              if (job.id !== answer.job || token !== generation) return;
              stopJob?.(); stopJob = null;
              if (job.state === 'complete') load().catch(() => {});
              else { status = 'failed'; error = job.error || `Job ${job.state}.`; render(); }
            };
            stopJob = app.jobs.onFinished(finished);
            const job = app.jobs.get(answer.job);
            if (job && ['complete', 'failed', 'cancelled', 'invalid', 'orphaned'].includes(job.state)) finished(job);
          }
          return;
        }
      } else artifact = saved;
      const query = new URLSearchParams({ kind, offset, limit: 50, family, sample, level, left, right });
      const answer = await json(`api/isotypes/${encodeURIComponent(artifact)}/page?${query}`);
      if (token !== generation) return;
      page = answer; status = 'ready'; keep(answer); render(); return answer;
    } catch (e) { if (token === generation) { status = 'failed'; error = e.message; render(); } throw e; }
  }
  async function showPage(nextKind = 'usage', options = {}) {
    kind = nextKind; family = options.family ?? ''; sample = options.sample ?? ''; level = options.level ?? 'class';
    left = options.left ?? ''; right = options.right ?? ''; offset = options.offset ?? 0; return load();
  }
  function inspect(row) { detail = row; app.focusItem({ kind: 'bcell-isotype-observation', id: row.id }); app.inspector.render(); }
  app.inspector.setSection('bcell-isotype-observation', () => !detail || store.ui.focus?.kind !== 'bcell-isotype-observation' || store.ui.focus.id !== detail.id ? null : h('section.inspector-section', h('h3', 'Isotype observation'), h('p', `${detail.sample} · ${detail.cell || detail.id}`), h('p', `${detail.class} · ${detail.gene}. ${detail.excluded || detail.issue || 'Resolved heavy constant call.'}`), h('p.small', `${detail.subject || 'No subject'} · ${detail.timepoint || 'No timepoint'} · ${detail.tissue || 'No tissue'}. Abundance: ${detail.abundance ?? 'Missing'}.`),
    ...detail.rows.map((r) => h('div', h('h4', `${r.locus} · ${r.sequenceId}`), h('p', `Raw C call: ${r.call || 'Missing'}`), h('p.small', { style: { overflowWrap: 'anywhere' } }, `${r.source.name}, line ${r.line} · SHA-256 ${r.source.sha256}`)))));
  const share = (value) => value == null ? 'Missing' : value.toLocaleString(undefined, { style: 'percent', maximumFractionDigits: 2 });

  function render() {
    clear(root);
    root.append(h('h2', 'Isotypes and family co-occurrence'), h('p.muted', 'Summarize observed heavy-chain constant calls in saved families. Co-occurrence describes labels in a family; it does not establish switching events, direction or rates. IgM/IgD alone is not switching evidence.'));
    const selector = h('select.input.small', { 'aria-label': 'Isotype dataset', onchange: () => { chosen = selector.value; saved = null; parent = ''; showPage().catch(() => {}); } }, ...store.project.datasets.map((d) => h('option', { value: d.id, selected: d.id === chosen }, d.name)));
    root.append(h('label.kv-inline', 'Dataset ', selector));
    if (!dataset()) { root.append(h('p', 'Open BCR data and save a thresholded family result.')); return; }
    if (saved) root.append(h('p', `Saved result ${saved}. Its family parent is recorded below.`), button('New summary', () => { saved = null; return showPage(); }));
    else {
      root.append(h('label.kv-inline', 'Saved families ', h('select.input.small', { 'aria-label': 'Saved families', onchange: (e) => { parent = e.target.value; showPage().catch(() => {}); } }, ...parents().map((p) => h('option', { value: p.id, selected: p.id === parent }, p.name)))));
      if (!parent) root.append(h('p', 'Save a Families result with a threshold first.'), button('Open Families', () => app.bcellsMode.show('families')));
    }
    if (status === 'loading' || status === 'running') { root.append(h('p', status === 'running' ? 'Counting isotypes… The job is available in Tasks.' : 'Reading the result…')); return; }
    if (error) root.append(h('p.callout.danger', error));
    if (!page) { if (parent) root.append(button('Summarize isotypes', () => load(true))); return; }
    const r = page.result, o = r.isotypeOptions;
    root.append(h('p', `${r.assigned} assigned ${r.unit} in ${page.familyCount} families. Class resolved for ${r.resolvedClass}; constant gene resolved for ${r.resolvedGene}.`), h('details', h('summary', 'Method and provenance'), h('p.small', r.description), h('p.small', { style: { overflowWrap: 'anywhere' } }, `Parent ${o.families} · SHA-256 ${o.familySha256}`), h('p.small', `Parent filter: ${r.familyFilter}. Resolution is constant gene after removing alleles; a class-only IGHG/IGHA call does not identify a subclass. No species-specific genomic order is assumed.`)));
    for (const [reason, count] of Object.entries(r.excluded)) root.append(h('p.small', `${count} observations excluded by parent: ${reason}.`));
    for (const [reason, count] of Object.entries(r.issues)) root.append(h('p.small', `${count} observations: ${reason}.`));
    root.append(h('div.btn-row', button('Sample usage', () => showPage()), button('Families', () => showPage('families')), button('Co-occurrence', () => showPage('pairs')), button('Observations', () => showPage('observations')), button('Export result', async () => { const id = page.artifact.id; const d = dataset(); const opened = await app.ensureDataset(d); if (opened.status !== 'open') throw new Error(opened.error ?? 'Open the dataset again.'); return exportResult(app, d, id); })));
    if (family) root.append(h('p.small', { style: { overflowWrap: 'anywhere' } }, `Family ${family}`), button(kind === 'usage' ? 'Family observations' : 'Family usage', () => showPage(kind === 'usage' ? 'observations' : 'usage', { family })));
    if (kind === 'usage' || kind === 'observations') {
      root.append(h('label.kv-inline', 'Sample ', h('select.input.small', { 'aria-label': 'Isotype sample', onchange: (e) => showPage(kind, { family, level, sample: e.target.value }).catch(() => {}) }, h('option', { value: '', selected: !sample }, 'All samples'), ...r.samples.map((name) => h('option', { value: name, selected: name === sample }, name)))));
    }
    if (kind === 'usage') {
      root.append(h('div.btn-row', button('Class', () => showPage(kind, { family, sample, level: 'class' }), level === 'class'), button('Constant gene / subclass', () => showPage(kind, { family, sample, level: 'gene' }), level === 'gene')),
        h('p.small.muted', 'Shares use all assigned observations in each sample or family/sample, including unresolved calls. Parent exclusions stay outside the denominator. Missing abundance is not zero. No row means no eligible observations; it is not a measured zero.'));
      root.append(table(['Sample', 'Timepoint', 'Tissue', 'State', 'Observations / assigned', 'Share', `Abundance (${r.abundanceUnit || "unspecified unit"}) / total`, 'Abundance share'], page.rows.map((u) => h('tr', td(u.sample), td(u.timepoint || '—'), td(u.tissue || '—'), td(u.state), td(`${u.observations} / ${u.total}`), td(share(u.share)), td(`${u.abundance ?? 'Missing'} / ${u.totalAbundance ?? 'Missing'}`), td(share(u.abundanceShare))))));
    } else if (kind === 'families') {
      if (left || right) root.append(h('p', `Families containing ${left} and ${right}.`));
      root.append(table(['Family', 'Scope', 'Observations', 'Resolved class / gene', 'Constant genes', 'Evidence'], page.rows.map((f) => h('tr', h('td', button(f.id, () => showPage('usage', { family: f.id }))), td(f.scope), td(f.observations), td(`${f.resolvedClass} / ${f.resolvedGene}`), td(f.genes.join(', ') || 'Unresolved'), td(f.evidence)))));
    } else if (kind === 'pairs') {
      root.append(h('p.small.muted', 'Each family contributes once to each unordered pair of resolved constant genes. Pairs overlap and must not be added. Families with any resolved gene are a coverage count, not fully observed families.'));
      root.append(table(['Unordered gene pair', 'Families with pair', 'Families with any resolved gene', 'All families'], page.rows.map((p) => h('tr', h('td', button(`${p.left} + ${p.right}`, () => showPage('families', { left: p.left, right: p.right }))), td(p.families), td(p.evaluableFamilies), td(p.totalFamilies)))));
    } else root.append(table(['Observation', 'Sample', 'Class', 'Constant gene', 'Evidence'], page.rows.map((o) => h('tr', h('td', button(o.cell || o.rows[0]?.sequenceId || o.id, () => inspect(o))), td(o.sample), td(o.class), td(o.gene), td(o.excluded || o.issue || 'Resolved')))));
    const move = (next) => showPage(kind, { family, sample, level, left, right, offset: next });
    root.append(h('div.btn-row', button('Previous', () => move(Math.max(0, offset - 50)), offset === 0), h('span', page.count ? `${offset + 1}–${Math.min(offset + page.rows.length, page.count)} of ${page.count}` : 'No rows'), button('Next', () => move(offset + 50), offset + page.rows.length >= page.count)), h('p.small.muted', `Kept ${page.artifact.created} as ${page.artifact.id}.`));
  }
  const view = { state: () => ({ status, error, page, artifact }), async show(id, options = {}) { chosen = id; saved = null; parent = options.families ?? ''; return showPage(); }, run: () => load(true), showPage, inspect };
  app.isotypesView = view; load().catch(() => {});
  return { update(topics) {
    const r = focused();
    if (topics.has('focus') && r && r.id !== saved) { chosen = r.dataset; saved = r.id; showPage().catch(() => {}); }
    else if (topics.has('focus') && store.ui.focus?.kind === 'dataset' && store.ui.focus.id !== chosen) { chosen = store.ui.focus.id; saved = null; parent = ''; showPage().catch(() => {}); }
    else if (topics.has('project-loaded') || topics.has('datasets')) { if (!dataset()) chosen = store.project.datasets[0]?.id; if (!saved) { parent = ''; showPage().catch(() => {}); } }
  }, destroy() { generation++; stopJob?.(); if (app.isotypesView === view) app.isotypesView = null; app.inspector.setSection('bcell-isotype-observation', null); root.remove(); } };
}
