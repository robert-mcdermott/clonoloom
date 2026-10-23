// Scientific reconstruction and paging are performed in Go. Saved choices are immutable.
import { h, clear } from './dom.js';
import { addResult } from '../lib/results.js';
import { exportResult } from './exports.js';
import { toast } from './overlays.js';

const references = [['ogrdb-human-2026-05-27', 'Human OGRDB · 27 May 2026'], ['simulated-bcell-immunization', 'Artificial immunization · default v2 preset']];
async function json(url, body) {
  const response = await fetch(url, body ? { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) } : { cache: 'no-store' });
  const value = await response.json();
  if (!response.ok) throw new Error(value.error ?? `HTTP ${response.status}`);
  return value;
}
const td = (value) => h('td', String(value ?? 'Missing'));
const table = (headers, rows) => h('div.table-scroll', { tabIndex: 0 }, h('table.data', h('thead', h('tr', ...headers.map((s) => h('th', { scope: 'col' }, s)))), h('tbody', ...rows)));

export function mountGermlines(app, container) {
  const { store } = app;
  const focused = () => store.ui.focus?.kind === 'result' ? store.project.artifacts.find((r) => r.id === store.ui.focus.id && r.kind === 'germlines') : null;
  let chosen = focused()?.dataset ?? (store.ui.focus?.kind === 'dataset' ? store.ui.focus.id : store.project.datasets[0]?.id);
  let saved = focused()?.id ?? null, parent = '', reference = references[0][0];
  let generation = 0, offset = 0, family = '', locus = '', kind = 'families', page = null, detail = null;
  let status = 'idle', error = '', artifact = '', stopJob = null;
  const dataset = () => store.project.datasets.find((d) => d.id === chosen);
  const parents = () => store.project.artifacts.filter((r) => r.dataset === chosen && r.kind === 'families' && r.families?.threshold != null);
  const root = h('div.view-body'); container.append(root);
  const button = (label, fn, disabled = false) => h('button.btn.small', { type: 'button', disabled, onclick: () => Promise.resolve().then(fn).catch((e) => toast(e.message, { kind: 'error' })) }, label);
  function keep(answer) {
    if (store.project.artifacts.some((r) => r.id === answer.artifact.id)) return;
    const r = answer.result;
    store.commit(addResult(store.project, { id: answer.artifact.id, kind: 'germlines', dataset: r.dataset, name: `D-masked germlines · ${r.datasetName}`, made: answer.artifact.created, method: 'Family allele intersection and AIRR CIGAR projection', describe: r.description, germlines: r.germlinesOptions, reconstructed: r.reconstructed, rowCount: answer.rowCount }), `Keep germlines of ${r.datasetName}`, ['project']);
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
        const answer = await json('api/germlines', { dataset: chosen, germlines: { families: parent, reference }, start });
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
      const answer = await json(`api/germlines/${encodeURIComponent(artifact)}/page?${query}`);
      if (token !== generation) return;
      page = answer; status = 'ready'; keep(answer); render(); return answer;
    } catch (e) { if (token === generation) { status = 'failed'; error = e.message; render(); } throw e; }
  }
  async function showPage(nextKind = 'families', nextFamily = '', nextLocus = '', nextOffset = 0) { kind = nextKind; family = nextFamily; locus = nextLocus; offset = nextOffset; return load(); }
  function inspect(row) { detail = row; app.focusItem({ kind: 'bcell-germline-row', id: row.id }); app.inspector.render(); }
  app.inspector.setSection('bcell-germline-row', () => !detail || store.ui.focus?.kind !== 'bcell-germline-row' || store.ui.focus.id !== detail.id ? null : h('section.inspector-section', h('h3', 'D-masked germline'), h('p', `${detail.sequenceId} · ${detail.sample} · ${detail.locus}`), h('p.small', `${detail.family} · ${detail.v} · ${detail.j}`), h('p.small', { style: { overflowWrap: 'anywhere' } }, `${detail.source.name}, line ${detail.line} · SHA-256 ${detail.source.sha256}`), detail.excluded ? h('p', detail.excluded) : h('div', h('p.small', 'Query, reconstructed germline, then segment labels. N masks the intervening junction. Gaps are indels; this alignment is not IMGT numbered.'), h('pre', { tabIndex: 0, style: { overflow: 'auto', textAlign: 'left' } }, `${detail.query}\n${detail.germline}\n${detail.regions}`), h('p.small', 'Full coordinate mappings are retained in the JSON export.'))));
  function render() {
    clear(root);
    root.append(h('h2', 'Germlines'), h('p.muted', 'Reconstruct V/J germlines within saved clonal families, masking D/N/P junctions. Choose the reference used for your ungapped AIRR CIGAR coordinates. This does not infer a genotype or an unmutated junction.'));
    const selector = h('select.input.small', { 'aria-label': 'Germline dataset', onchange: () => { chosen = selector.value; saved = null; parent = ''; showPage().catch(() => {}); } }, ...store.project.datasets.map((d) => h('option', { value: d.id, selected: d.id === chosen }, d.name)));
    root.append(h('label.kv-inline', 'Dataset ', selector));
    if (!dataset()) { root.append(h('p', 'Open BCR data and save a thresholded family result.')); return; }
    if (saved) root.append(h('p', `Saved result ${saved}. Its family and reference are recorded below.`), button('New reconstruction', () => { saved = null; return showPage(); }));
    else {
      const select = (label, value, choices, change) => h('label.kv-inline', label, h('select.input.small', { 'aria-label': label, onchange: (e) => { change(e.target.value); showPage().catch(() => {}); } }, ...choices.map(([id, name]) => h('option', { value: id, selected: value === id }, name))));
      root.append(h('div.options-row', select('Saved families', parent, parents().map((p) => [p.id, `${p.id} · threshold ${p.families.threshold}`]), (v) => { parent = v; }), select('Reference', reference, references, (v) => { reference = v; })));
      if (!parent) root.append(h('p', 'Save a Families result with an explicit threshold first.'), button('Open Families', () => app.bcellsMode.show('families')));
      root.append(h('p.small.muted', 'Download human segments in Reference data. The default artificial example supplies its v2 reference (seed 20261027); regenerate an older example to use it. Missing CIGARs, including native IMGT input, are reported as exclusions.'));
    }
    if (status === 'loading' || status === 'running') { root.append(h('p', status === 'running' ? 'Reconstructing germlines… The job is available in Tasks.' : 'Reading the result…')); return; }
    if (error) root.append(h('p.callout.danger', error));
    if (!page) { if (parent) root.append(button('Reconstruct germlines', () => load(true))); return; }
    const r = page.result, o = r.germlinesOptions;
    root.append(h('p', `${r.reconstructed} reconstructed of ${page.rowCount} assigned source rows · ${page.familyCount} family/locus groups.`), h('p.small', r.description), h('p.small', { style: { overflowWrap: 'anywhere' } }, `Parent ${o.families} · SHA-256 ${o.familySha256}`));
    root.append(h('details', h('summary', 'Reference provenance'), ...o.files.map((f) => h('p.small', { style: { overflowWrap: 'anywhere' } }, `${f.reference} · ${f.file} · SHA-256 ${f.sha256} · ${f.license}. ${f.citation}. ${f.source}`))));
    for (const [reason, count] of Object.entries(r.excluded)) root.append(h('p.small', `${count} excluded: ${reason}.`));
    root.append(h('div.btn-row', button('Family alleles', () => showPage()), button('All source rows', () => showPage('rows')), button('Export result', async () => { const id = page.artifact.id; const d = dataset(); const opened = await app.ensureDataset(d); if (opened.status !== 'open') throw new Error(opened.error ?? 'Open the dataset again.'); return exportResult(app, d, id); })));
    if (kind === 'families') root.append(table(['Family', 'Locus', 'V allele', 'J allele', 'Reconstructed / rows', 'Exclusion'], page.rows.map((f) => h('tr', h('td', button(f.family, () => showPage('rows', f.family, f.locus))), td(f.locus), td(f.v), td(f.j), td(`${f.reconstructed} / ${f.rows}`), td(f.excluded || '—')))));
    else root.append(table(['Sequence', 'Sample', 'Locus', 'V allele', 'J allele', 'Status'], page.rows.map((r) => h('tr', h('td', button(r.sequenceId || r.id, () => inspect(r))), td(r.sample), td(r.locus), td(r.v), td(r.j), td(r.excluded || 'Reconstructed')))));
    root.append(h('div.btn-row', button('Previous', () => showPage(kind, family, locus, Math.max(0, offset - 50)), offset === 0), h('span', page.count ? `${offset + 1}–${Math.min(offset + page.rows.length, page.count)} of ${page.count}` : 'No rows'), button('Next', () => showPage(kind, family, locus, offset + 50), offset + page.rows.length >= page.count)), h('p.small.muted', `Kept ${page.artifact.created} as ${page.artifact.id}.`));
  }
  const view = { state: () => ({ status, error, page, artifact }), async show(id, options = {}) { chosen = id; saved = null; parent = options.families ?? ''; reference = options.reference ?? references[0][0]; return showPage(); }, run: () => load(true), showPage, inspect };
  app.germlinesView = view; load().catch(() => {});
  return { update(topics) {
    const r = focused();
    if (topics.has('focus') && r && r.id !== saved) { chosen = r.dataset; saved = r.id; showPage().catch(() => {}); }
    else if (topics.has('focus') && store.ui.focus?.kind === 'dataset' && store.ui.focus.id !== chosen) { chosen = store.ui.focus.id; saved = null; parent = ''; showPage().catch(() => {}); }
    else if (topics.has('project-loaded') || topics.has('datasets')) { if (!dataset()) chosen = store.project.datasets[0]?.id; if (!saved) { parent = ''; showPage().catch(() => {}); } }
  }, destroy() { generation++; stopJob?.(); if (app.germlinesView === view) app.germlinesView = null; app.inspector.setSection('bcell-germline-row', null); root.remove(); } };
}
