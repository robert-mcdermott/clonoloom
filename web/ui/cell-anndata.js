// AnnData exports preserve the verified Go result; JS selects, waits and downloads only.
import { h, clear } from './dom.js';
import { addResult } from '../lib/results.js';
import { confirmDialog } from './overlays.js';
async function json(url, body) {
  const response = await fetch(url, body === undefined ? { cache: 'no-store' } : { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
  const value = await response.json(); if (!response.ok) throw new Error(value.error ?? `HTTP ${response.status}`); return value;
}
export function mountCellAnnData(app, container) {
  const { store } = app, root = h('section'); container.append(root);
  let serial = 0, generation = store.state.generation, parent = '', answer = null, loaded = '', status = 'idle', error = '', job = '';
  const button = (label, action, disabled = false) => h('button.btn.small', { type: 'button', disabled, onclick: () => Promise.resolve().then(action).catch((e) => { error = e.message; draw(); }) }, label);
  function received(value) { answer = value; loaded = value.artifact.id; status = 'ready'; job = ''; draw(); }
  async function show(id) {
    const token = ++serial, same = store.sameProject(); status = 'loading'; answer = null; error = ''; draw();
    try { const value = await json(`api/anndata-export/${encodeURIComponent(id)}`); if (token !== serial || !same()) return; received(value); return value; }
    catch (e) { if (token === serial && same()) { error = e.message; status = 'failed'; draw(); } throw e; }
  }
  async function run(options = {}) {
    const selected = options.phenotypes ?? parent; if (!selected) throw new Error('Choose saved clonotype phenotypes first.');
    const token = ++serial, same = store.sameProject(); status = 'loading'; answer = null; error = ''; loaded = ''; job = ''; draw();
    try {
      const start = await json('api/anndata-export', { phenotypes: selected }); if (token !== serial || !same()) return;
      if (!start.ready) { job = start.job; status = 'running'; draw(); const done = await app.jobs.finished(job); if (token !== serial || !same()) return; if (done?.state !== 'complete') throw new Error(done?.error || `AnnData export ${done?.state ?? 'unavailable'}.`); }
      const value = await json(`api/anndata-export/${encodeURIComponent(start.artifact)}`); if (token !== serial || !same()) return;
      received(value);
      if (!store.project.artifacts.some((r) => r.id === loaded)) store.commit(addResult(store.project, { id: loaded, kind: 'anndata-export', dataset: value.result.dataset, expression: value.result.expression, name: value.artifact.name, made: value.artifact.created, method: 'AnnData export', describe: value.result.description, citation: value.artifact.lineage.method.citation }), 'Keep AnnData export', ['project']);
      draw(); return value;
    } catch (e) { if (token === serial && same()) { error = e.message; status = 'failed'; job = ''; draw(); } throw e; }
  }
  async function download() {
    const value = answer, same = store.sameProject(); if (!value) return;
    const restricted = (value.result.notice.Credits ?? []).filter((c) => c.LicenseClass !== 'open');
    if (restricted.length && !(await confirmDialog({ title: 'These data may not be shared', message: h('div', h('p', 'This export contains receptor evidence under these provider terms:'), h('ul', ...restricted.map((c) => h('li', `${c.Title} — ${c.License}`))), h('p', 'Export only for yourself and your study; the NOTICE travels with the data.')), confirm: 'Export' }))) return;
    if (!same()) return;
    const a = h('a', { href: `api/anndata-export/${encodeURIComponent(value.artifact.id)}/download${restricted.length ? '?accept=1' : ''}`, download: '' }); document.body.append(a); a.click(); a.remove();
  }
  function draw() {
    clear(root); root.append(h('h2#anndata-export', 'AnnData export'), h('p', 'Export raw counts with the saved phenotype result’s exact receptor sample binding. All expression cells and feature modalities are retained, including cells excluded from a computed map. Reopen the original expression sources and receptor dataset to prepare a new export.'));
    const parents = store.project.artifacts.filter((r) => r.kind === 'phenotypes'); if (parent && !parents.some((r) => r.id === parent)) parent = '';
    root.append(h('div.btn-row', h('label.field', h('span', 'Export phenotype parent'), h('select.input', { 'aria-label': 'Export phenotype parent', onchange: (e) => { parent = e.target.value; } }, h('option', { value: '', selected: !parent }, 'Choose saved phenotypes'), ...parents.map((r) => h('option', { value: r.id, selected: parent === r.id }, `${r.name} · ${r.id.slice(-6)}`)))), button('Prepare AnnData export', () => run(), !parents.length || ['loading', 'running'].includes(status))));
    const saved = store.project.artifacts.filter((r) => r.kind === 'anndata-export'); if (saved.length) root.append(h('div.btn-row', h('span', 'Saved AnnData exports'), ...saved.map((r) => button(`${r.name} · ${r.id.slice(-6)}`, () => show(r.id), status === 'running'))));
    if (job) root.append(button('Cancel AnnData export', () => app.jobs.cancel(job)));
    root.append(h('p', { role: 'status' }, error || (status === 'running' ? app.jobs.get(job)?.message || 'Preparing export…' : status === 'loading' ? 'Reading export…' : '')));
    if (!answer) return;
    const doc = answer.result;
    root.append(h('p', `${doc.cells} cells × ${doc.features} features · ${doc.entries} stored sparse entries · X: raw integer counts.`), h('p', `Embeddings: ${doc.embeddings.join(', ') || 'none supplied or computed'}. Missing coordinates are NaN. Supplied and computed coordinates remain distinct; plots and selections do not change the exported population.`),
      h('p', 'Receptor assignments, ambiguity and chain status are retained with their definitions. This is a ClonoLoom receptor-link schema, not scirpy’s raw AIRR-chain representation. Original annotation names map to collision-free columns; missing values remain missing.'),
      button('Download AnnData and NOTICE', download),
      h('details', h('summary', 'AnnData schema and provenance'), h('p', doc.description), h('p', answer.artifact.lineage.method.citation), h('p', { style: { overflowWrap: 'anywhere' } }, `Export ${loaded} · Phenotypes ${doc.phenotypes} · Expression ${doc.expression}`), h('pre', { style: { whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' } }, JSON.stringify(answer.artifact.lineage.parameters, null, 2))),
      h('a.btn.small', { href: `api/anndata-export/${encodeURIComponent(loaded)}`, download: `${loaded}.json` }, 'Download export record'));
  }
  const view = { run, show, state: () => ({ status, error, answer, artifact: loaded }) }; app.cellAnnDataView = view;
  const focused = () => store.ui.focus?.kind === 'result' && store.project.artifacts.find((r) => r.id === store.ui.focus.id && r.kind === 'anndata-export');
  draw(); if (focused()) show(focused().id).catch(() => {});
  return { update(topics) { if (generation !== store.state.generation) { generation = store.state.generation; serial++; answer = null; loaded = ''; status = 'idle'; error = ''; job = ''; parent = ''; draw(); } const target = focused(); if (topics.has('focus') && target && target.id !== loaded) show(target.id).catch(() => {}); else if (topics.has('project') || topics.has('jobs') && status === 'running') draw(); }, destroy() { serial++; if (app.cellAnnDataView === view) delete app.cellAnnDataView; root.remove(); } };
}
