// Exact saved measurements joined in Go; JS only selects and formats table columns.
import { h, clear, downloadBlob } from './dom.js';
import { confirmDialog } from './overlays.js';
import { addResult } from '../lib/results.js';

async function json(url, body) {
  const response = await fetch(url, body === undefined ? { cache: 'no-store' } : { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
  const value = await response.json();
  if (!response.ok) throw new Error(value.error ?? `HTTP ${response.status}`);
  return value;
}
export function mountCohortFeatures(app, container) {
  const { store } = app, root = h('section.card.cohort-features'); container.append(root);
  let serial = 0, generation = store.state.generation, answer = null, loaded = '', status = 'idle', error = '', job = '', design = '', parent = '', metric = 'shannon', query = '', offset = 0;
  const button = (label, action, disabled = false) => h('button.btn.small', { type: 'button', disabled, onclick: () => Promise.resolve().then(action).catch((e) => { error = e.message; draw(); }) }, label);
  const busy = () => ['loading', 'running'].includes(status);
  const table = (caption, heads, rows) => h('div.table-wrap', { tabIndex: 0 }, h('table.data', h('caption', caption), h('thead', h('tr', ...heads.map((v) => h('th', { scope: 'col' }, v)))), h('tbody', ...rows.map((row) => h('tr', ...row.map((v) => h('td', v)))))));
  function received(value) { answer = value; loaded = value.artifact.id; status = 'ready'; job = ''; offset = 0; query = ''; draw(); root.scrollIntoView({ block: 'start' }); }
  async function show(id) {
    const token = ++serial, same = store.sameProject(); answer = null; loaded = ''; status = 'loading'; error = ''; draw();
    try { const value = await json(`api/cohort-features/${encodeURIComponent(id)}`); if (token !== serial || !same()) return; received(value); return value; }
    catch (e) { if (token === serial && same()) { error = e.message; status = 'failed'; draw(); } throw e; }
  }
  async function run(options = {}) {
    if (busy()) throw new Error('Wait for the current feature table.');
    const record = (store.project.cohortDesigns ?? []).find((d) => d.id === (options.design ?? design));
    const diversity = options.diversity ?? parent;
    if (!record || !diversity) throw new Error('Choose a saved design version and a saved diversity result.');
    if (!store.project.artifacts.some((a) => a.id === diversity && a.kind === 'diversity' && a.dataset === record.dataset)) throw new Error('Choose a diversity result from the design’s dataset.');
    const token = ++serial, same = store.sameProject(); status = 'loading'; error = ''; answer = null; loaded = ''; job = ''; draw();
    try {
      const d = store.project.datasets.find((d) => d.id === record.dataset);
      if (!d) throw new Error('The design’s dataset is no longer in this project.');
      const opened = await app.ensureDataset(d);
      if (token !== serial || !same()) return;
      if (opened.status !== 'open') throw new Error(opened.error ?? 'Open the source dataset first.');
      const start = await json('api/cohort-features', { design: { record: record.id, name: record.name, revision: record.revision, id: record.result.id, input: record.result.input }, diversity });
      if (token !== serial || !same()) return;
      if (!start.ready) { job = start.job; status = 'running'; draw(); const done = await app.jobs.finished(job); if (token !== serial || !same()) return; if (done?.state !== 'complete') throw new Error(done?.error || `Feature table ${done?.state ?? 'unavailable'}.`); }
      const value = await json(`api/cohort-features/${encodeURIComponent(start.artifact)}`); if (token !== serial || !same()) return;
      received(value);
      if (!store.project.artifacts.some((r) => r.id === loaded)) store.commit(addResult(store.project, { id: loaded, kind: 'cohort-features', dataset: value.result.parent.dataset, name: value.artifact.name, made: value.artifact.created, method: 'Subject feature table', describe: value.result.description, citation: value.artifact.lineage.method.citation }), 'Keep subject feature table', ['project']);
      draw(); return value;
    } catch (e) { if (token === serial && same()) { error = e.message; status = 'failed'; job = ''; draw(); } throw e; }
  }
  async function download() {
    const value = answer, same = store.sameProject(); if (!value) return;
    const restricted = (value.result.notice.Credits ?? []).filter((c) => c.LicenseClass !== 'open');
    if (restricted.length && !await confirmDialog({ title: 'Provider terms apply', message: h('div', h('p', 'These derived results retain the following source terms. Read them before sharing.'), h('ul', ...restricted.map((c) => h('li', `${c.Title} — ${c.License}`)))), confirm: 'Export' })) return;
    if (!same()) return;
    const response = await fetch(`api/cohort-features/${encodeURIComponent(value.artifact.id)}/download${restricted.length ? '?accept=1' : ''}`);
    if (!response.ok) throw new Error((await response.json()).error ?? 'Export failed.');
    const blob = await response.blob(); if (same()) downloadBlob(blob, `${value.artifact.id}.zip`);
  }
  function draw() {
    clear(root);
    const designs = store.project.cohortDesigns ?? [], selected = designs.find((d) => d.id === design);
    const parents = store.project.artifacts.filter((r) => r.kind === 'diversity' && r.dataset === selected?.dataset);
    if (parent && !parents.some((r) => r.id === parent)) parent = '';
    root.append(h('h2#subject-features', 'Subject feature table'), h('p', 'Choose a saved design version and diversity result. Each selected subject/group observation keeps its sample’s measurements. Paired designs retain two rows per subject.'),
      h('h3', 'Settings for the next table'),
      h('div.cohort-controls', h('label.field', h('span', 'Feature design version'), h('select.input.small', { 'aria-label': 'Feature design version', disabled: busy(), onchange: (e) => { design = e.target.value; parent = ''; draw(); } }, h('option', { value: '', selected: !selected }, 'Choose saved design'), ...designs.map((d) => h('option', { value: d.id, selected: design === d.id }, `${d.name}, version ${d.revision} · ${store.project.datasets.find((x) => x.id === d.dataset)?.name ?? d.dataset}`)))),
        h('label.field', h('span', 'Diversity parent'), h('select.input.small', { 'aria-label': 'Diversity parent', disabled: busy(), onchange: (e) => { parent = e.target.value; draw(); } }, h('option', { value: '', selected: !parent }, 'Choose saved diversity'), ...parents.map((r) => h('option', { value: r.id, selected: parent === r.id }, `${r.name} · ${r.id.slice(-6)}`))))),
      h('p.muted.small', 'Selection uses the saved annotations and exclusions. Current annotation edits are not applied. Measure diversity in Repertoire first; its definition, QC and abundance unit travel with the feature table.'),
      button('Build subject feature table', () => run(), busy() || !selected || !parent));
    const saved = store.project.artifacts.filter((r) => r.kind === 'cohort-features');
    if (saved.length) root.append(h('div.btn-row', h('span', 'Saved tables'), ...saved.map((r) => button(`${r.name} · ${r.id.slice(-6)}`, () => show(r.id), busy()))));
    if (job) root.append(button('Cancel feature table', () => app.jobs.cancel(job)));
    if (error) root.append(h('p.callout.danger', { role: 'alert' }, error));
    if (busy()) root.append(h('p', { role: 'status' }, status === 'running' ? 'Building subject features…' : 'Reading saved evidence…'));
    if (!answer) return;
    const doc = answer.result, catalog = doc.table.catalog, index = Math.max(0, catalog.findIndex((f) => f.id === metric)), feature = catalog[index];
    const rows = doc.table.rows.filter((r) => `${r.name} ${r.subject} ${r.group}`.toLowerCase().includes(query.toLowerCase()));
    root.append(h('h3.feature-result-title', `${doc.selection.name}, version ${doc.selection.revision}`), h('p', `${doc.design.subjects} subjects · ${doc.design.included} selected observations · ${doc.design.excluded} excluded samples · ${doc.design.input.spec.layout}`),
      h('p', `Abundance: ${doc.parent.unit}. Definition: ${doc.parent.describe}. QC: ${doc.notice.Filter || 'No additional filter'}.`),
      ...doc.design.findings.map((f) => h('div.callout.warn', h('b', f.title), h('p', f.detail))),
      h('p.muted.small', 'Point estimates at each sample’s observed depth. No depth standardization or group testing. Missing is not zero. Parent bootstrap intervals are within-sample uncertainty, not between-subject uncertainty.'),
      h('label.field', h('span', 'Displayed feature'), h('select.input.small', { 'aria-label': 'Displayed feature', onchange: (e) => { metric = e.target.value; draw(); } }, ...catalog.map((f) => h('option', { value: f.id, selected: f.id === feature.id }, `${f.name} (${f.unit})`)))),
      h('p', feature.definition), h('label.field', h('span', 'Find feature observations'), h('input.input.small', { type: 'search', 'aria-label': 'Find feature observations', value: query, onchange: (e) => { query = e.target.value; offset = 0; draw(); } })),
      table(`${feature.name} by saved subject/group observation`, ['Subject', 'Group', 'Sample', 'Batch', `${feature.name} (${feature.unit})`, 'Missing-value reason'], rows.slice(offset, offset + 50).map((r) => { const x = r.values[index]; return [r.subject, r.group, r.name, r.batch || 'Missing', x.value == null ? 'Missing' : Number(x.value).toPrecision(6), x.missing || 'Available']; })),
      h('div.btn-row', button('Previous observations', () => { offset = Math.max(0, offset - 50); draw(); }, offset === 0), h('span', `${offset + (rows.length ? 1 : 0)}–${Math.min(rows.length, offset + 50)} of ${rows.length} matching observations`), button('Next observations', () => { offset += 50; draw(); }, offset + 50 >= rows.length), button('Export feature table ZIP', download)),
      h('details', h('summary', 'Feature catalog, method and provenance'), h('p', doc.description), h('p', answer.artifact.lineage.method.citation),
        table('All exported feature definitions', ['Feature', 'Unit', 'Definition'], catalog.map((f) => [f.name, f.unit, f.definition])),
        h('p', 'ZIP contains the complete CSV, feature catalog, JSON with design annotations and exclusions, exact parent measurements and manifest, result manifest and source NOTICE.'),
        h('pre', { style: { whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' } }, JSON.stringify(answer.artifact.lineage.parents, null, 2))));
  }
  const view = { run, show, download, state: () => ({ status, error, answer, artifact: loaded }) }; app.cohortFeaturesView = view;
  const focused = () => store.ui.focus?.kind === 'result' && store.project.artifacts.find((r) => r.id === store.ui.focus.id && r.kind === 'cohort-features');
  draw(); if (focused()) show(focused().id).catch(() => {});
  return { update(topics) { if (generation !== store.state.generation) { generation = store.state.generation; serial++; answer = null; loaded = ''; status = 'idle'; error = ''; job = ''; design = ''; parent = ''; }
    const target = focused(); if (topics.has('focus') && target && target.id !== loaded) show(target.id).catch(() => {}); else if (topics.has('project') || topics.has('jobs')) draw();
  }, destroy() { serial++; if (app.cohortFeaturesView === view) delete app.cohortFeaturesView; root.remove(); } };
}
