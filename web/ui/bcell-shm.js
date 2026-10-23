// Scientific counts and denominators are computed in Go; the browser displays saved results.
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

export function mountSHM(app, container) {
  const { store } = app;
  const focused = () => store.ui.focus?.kind === 'result' ? store.project.artifacts.find((r) => r.id === store.ui.focus.id && r.kind === 'shm') : null;
  let chosen = focused()?.dataset ?? (store.ui.focus?.kind === 'dataset' ? store.ui.focus.id : store.project.datasets[0]?.id);
  let saved = focused()?.id ?? null, parent = '';
  let generation = 0, offset = 0, family = '', locus = '', kind = 'summaries', page = null, detail = null;
  let status = 'idle', error = '', artifact = '', stopJob = null;
  const dataset = () => store.project.datasets.find((d) => d.id === chosen);
  const parents = () => store.project.artifacts.filter((r) => r.dataset === chosen && r.kind === 'germlines');
  const root = h('div.view-body'); container.append(root);
  const button = (label, fn, disabled = false) => h('button.btn.small', { type: 'button', disabled, onclick: () => Promise.resolve().then(fn).catch((e) => toast(e.message, { kind: 'error' })) }, label);
  function keep(answer) {
    if (store.project.artifacts.some((r) => r.id === answer.artifact.id)) return;
    const r = answer.result;
    store.commit(addResult(store.project, { id: answer.artifact.id, kind: 'shm', dataset: r.dataset, name: `Somatic hypermutation · ${r.datasetName}`, made: answer.artifact.created, method: 'Observed nucleotide divergence; single-hit R/S attribution', describe: r.description, shm: r.shmOptions, germlineProvenance: r.germlineProvenance, evaluated: r.evaluated, rowCount: answer.rowCount }), `Keep SHM of ${r.datasetName}`, ['project']);
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
        const answer = await json('api/shm', { dataset: chosen, shm: { germlines: parent }, start });
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
      const query = new URLSearchParams({ kind, offset, limit: 50, family, locus });
      const answer = await json(`api/shm/${encodeURIComponent(artifact)}/page?${query}`);
      if (token !== generation) return;
      page = answer; status = 'ready'; keep(answer); render(); return answer;
    } catch (e) { if (token === generation) { status = 'failed'; error = e.message; render(); } throw e; }
  }
  async function showPage(nextKind = 'summaries', nextFamily = '', nextLocus = '', nextOffset = 0) { kind = nextKind; family = nextFamily; locus = nextLocus; offset = nextOffset; return load(); }
  function inspect(row) { detail = row; app.focusItem({ kind: 'bcell-shm-row', id: row.id }); app.inspector.render(); }
  const countCells = (r) => [td(r.evaluableBases), td(r.mutations), h('td', { title: String(r.frequency ?? 'Missing') }, r.frequency == null ? 'Missing' : r.frequency === 0 ? '0' : r.frequency.toPrecision(4)), ...[r.evaluableCodons, r.replacement, r.silent, r.unclassified].map(td)];
  const headers = ['Evaluable bases', 'Substitutions', 'Frequency', 'Evaluable codons', 'R · single-hit', 'S · single-hit', 'Unclassified'];
  app.inspector.setSection('bcell-shm-row', () => !detail || store.ui.focus?.kind !== 'bcell-shm-row' || store.ui.focus.id !== detail.id ? null : h('section.inspector-section', h('h3', 'Somatic hypermutation'), h('p', `${detail.sequenceId} · ${detail.sample} · ${detail.locus}`), h('p.small', { style: { overflowWrap: 'anywhere' } }, `${detail.family} · ${detail.source.name}, line ${detail.line} · SHA-256 ${detail.source.sha256}`), h('p', detail.excluded || detail.codonExclusion || `Codon phase checked at CDR3 start ${detail.frameAnchor}.`),
    ...detail.regions.map((r) => h('div', h('h4', r.region), r.unavailable ? h('p', r.unavailable) : h('p.small', `${r.mutations} substitutions / ${r.evaluableBases} evaluable bases; frequency ${r.frequency ?? 'Missing'}. ${r.masked} masked, ${r.ambiguous} ambiguous, ${r.gaps} gap columns. ${r.evaluableCodons} evaluable codons; ${r.replacement} replacement and ${r.silent} silent substitutions in single-hit codons; ${r.unclassified} unclassified substitutions. Endpoint codons: ${r.replacementCodons} replacement, ${r.silentCodons} silent, including ${r.multiHitCodons} multi-hit codons. ${r.stopCodons} stop-containing codons excluded.`)))));

  function render() {
    clear(root);
    root.append(h('h2', 'Somatic hypermutation'), h('p.muted', 'Compare source chain sequences with saved D-masked germlines. Frequency is substitutions divided by evaluable paired bases. Single-hit replacement/silent substitutions and endpoint codon changes are separate; no selection test or historical mutation rate is inferred.'));
    const selector = h('select.input.small', { 'aria-label': 'SHM dataset', onchange: () => { chosen = selector.value; saved = null; parent = ''; showPage().catch(() => {}); } }, ...store.project.datasets.map((d) => h('option', { value: d.id, selected: d.id === chosen }, d.name)));
    root.append(h('label.kv-inline', 'Dataset ', selector));
    if (!dataset()) { root.append(h('p', 'Open BCR data and save a germline result.')); return; }
    if (saved) root.append(h('p', `Saved result ${saved}. Its germline parent is recorded below.`), button('New comparison', () => { saved = null; return showPage(); }));
    else {
      const select = (label, value, choices, change) => h('label.kv-inline', label, h('select.input.small', { 'aria-label': label, onchange: (e) => { change(e.target.value); showPage().catch(() => {}); } }, ...choices.map(([id, name]) => h('option', { value: id, selected: value === id }, name))));
      root.append(h('div.options-row', select('Saved germlines', parent, parents().map((p) => [p.id, p.name]), (v) => { parent = v; })));
      if (!parent) root.append(h('p', 'Save a Germlines result first.'), button('Open Germlines', () => app.bcellsMode.show('germlines')));
      root.append(h('p.small.muted', 'Regional coordinates and junction evidence come from the original input. Missing or inconsistent evidence remains explicit. Source rows are not weighted by reads or treated as independent subjects.'));
    }
    if (status === 'loading' || status === 'running') { root.append(h('p', status === 'running' ? 'Counting mutations… The job is available in Tasks.' : 'Reading the result…')); return; }
    if (error) root.append(h('p.callout.danger', error));
    if (!page) { if (parent) root.append(button('Measure SHM', () => load(true))); return; }
    const r = page.result, o = r.shmOptions;
    root.append(h('p', `${r.evaluated} of ${page.rowCount} source rows have evaluable bases.`), h('details', h('summary', 'Method and provenance'), h('p.small', r.description), h('p.small', { style: { overflowWrap: 'anywhere' } }, `Parent ${o.germlines} · SHA-256 ${o.germlineSha256}`), ...r.germlineProvenance.files.map((f) => h('p.small', { style: { overflowWrap: 'anywhere' } }, `${f.reference} · ${f.file} · SHA-256 ${f.sha256} · ${f.license}. ${f.citation}. ${f.source}`))));
    for (const [reason, count] of Object.entries(r.excluded)) root.append(h('p.small', `${count} rows excluded: ${reason}.`));
    for (const [reason, count] of Object.entries(r.codonExclusions)) root.append(h('p.small', `${count} rows without codon attribution: ${reason}.`));
    root.append(h('div.btn-row', button('Sample / region totals', () => showPage()), button('Source rows', () => showPage('rows')), button('Export result', async () => { const id = page.artifact.id; const d = dataset(); const opened = await app.ensureDataset(d); if (opened.status !== 'open') throw new Error(opened.error ?? 'Open the dataset again.'); return exportResult(app, d, id); })));
    if (kind === 'summaries') {
      root.append(h('p.small.muted', 'Pooled source-row counts by sample and locus. Available rows have usable regional coordinates; rows with evaluable codons may be fewer. Overlapping all/V/J and FWR/CDR summaries must not be added together.'));
      root.append(table(['Sample', 'Locus', 'Region', 'Available / source rows', 'Rows with codons', ...headers], page.rows.map((r) => h('tr', td(r.sample), td(r.locus), td(r.region), td(`${r.availableRows} / ${r.rows}`), td(r.codonRows), ...(r.availableRows ? countCells(r) : headers.map(() => td(null)))))));
    } else root.append(table(['Sequence', 'Sample', 'Locus', ...headers, 'Evidence'], page.rows.map((r) => h('tr', h('td', button(r.sequenceId || r.id, () => inspect(r))), td(r.sample), td(r.locus), ...(r.excluded ? headers.map(() => td(null)) : countCells(r.regions[0])), td(r.excluded || r.codonExclusion || 'Evaluable')))));
    root.append(h('div.btn-row', button('Previous', () => showPage(kind, family, locus, Math.max(0, offset - 50)), offset === 0), h('span', page.count ? `${offset + 1}–${Math.min(offset + page.rows.length, page.count)} of ${page.count}` : 'No rows'), button('Next', () => showPage(kind, family, locus, offset + 50), offset + page.rows.length >= page.count)), h('p.small.muted', `Kept ${page.artifact.created} as ${page.artifact.id}.`));
  }
  const view = { state: () => ({ status, error, page, artifact }), async show(id, options = {}) { chosen = id; saved = null; parent = options.germlines ?? ''; return showPage(); }, run: () => load(true), showPage, inspect };
  app.shmView = view; load().catch(() => {});
  return { update(topics) {
    const r = focused();
    if (topics.has('focus') && r && r.id !== saved) { chosen = r.dataset; saved = r.id; showPage().catch(() => {}); }
    else if (topics.has('focus') && store.ui.focus?.kind === 'dataset' && store.ui.focus.id !== chosen) { chosen = store.ui.focus.id; saved = null; parent = ''; showPage().catch(() => {}); }
    else if (topics.has('project-loaded') || topics.has('datasets')) { if (!dataset()) chosen = store.project.datasets[0]?.id; if (!saved) { parent = ''; showPage().catch(() => {}); } }
  }, destroy() { generation++; stopJob?.(); if (app.shmView === view) app.shmView = null; app.inspector.setSection('bcell-shm-row', null); root.remove(); } };
}
