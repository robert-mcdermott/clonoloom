// B-cell evidence and families: all counts and checks come from Go.
import { mountFamilies } from './bcell-families.js';
import { h, icon, clear } from './dom.js';
import { bcellInputs } from './storage.js';
import { datasetFilter } from '../lib/filters.js';
import { datasetDefinition } from '../lib/definitions.js';
import { exportDataset } from './exports.js';
import { toast } from './overlays.js';

function mountInputs(app, container) {
  const { store } = app;
  let chosen = store.ui.focus?.kind === 'dataset' ? store.ui.focus.id : store.project.datasets[0]?.id;
  let offset = 0;
  let generation = 0;
  let selection = 0;
  let detail = null;
  let lastKey = '';
  let answer = null;
  const dataset = () => store.project.datasets.find((d) => d.id === chosen) ?? store.project.datasets[0];
  const request = () => {
    const d = dataset();
    return { dataset: d?.id, definition: datasetDefinition(d, Object.keys(app.datasetStates.get(d?.id)?.summary?.samples?.reduce((out, s) => Object.assign(out, s.loci), {}) ?? {})), filter: datasetFilter(store.project, d)?.rules ?? {}, offset, limit: 50 };
  };
  const key = () => JSON.stringify(request());
  const select = h('select.input.small', { 'aria-label': 'Dataset', onchange: () => { chosen = select.value; offset = 0; reload(); } });
  const body = h('div.view-body');
  const root = h('div.view', h('div.workbench-head', h('span.spacer'),
    h('label.unit-confirm', 'Dataset', select), h('button.btn.small', { type: 'button', onclick: () => { if (dataset()) exportDataset(app, dataset(), 'rearrangements'); } }, icon('download'), 'Export AIRR')), body);
  container.append(root);
  const cell = (v) => h('td', String(v));
  const table = (headers, rows) => h('div.table-scroll', { tabIndex: 0 }, h('table.data',
    h('thead', h('tr', ...headers.map((x) => h('th', { scope: 'col' }, x)))), h('tbody', ...rows)));
  const yes = (v) => v ? 'Present' : 'Missing';

  app.inspector.setSection('bcell-input', () => {
    if (!detail || store.ui.focus?.kind !== 'bcell-input' || store.ui.focus.id !== detail.id) return null;
    return h('section.inspector-section', h('h3', 'Sequence evidence'),
      h('p', `${detail.sample} · ${detail.locus} · ${detail.sequenceId || 'No sequence ID'}`),
      h('p.muted', `${detail.source}, line ${detail.line}`),
      ...detail.evidence.issues.map((i) => h('p', `${i.field}: ${i.message}`)),
      h('dl.kv', ...detail.fieldOrder.filter((name) => name !== 'imgt_annotations' && detail.fields[name] !== '').flatMap((name) => [h('dt', name), h('dd', { style: { overflowWrap: 'anywhere', whiteSpace: 'pre-wrap' } }, detail.fields[name])])),
      detail.fields.imgt_annotations ? h('details', h('summary', 'Native IMGT annotations'), h('pre', { style: { whiteSpace: 'pre-wrap', overflowWrap: 'anywhere', textAlign: 'left', fontSize: '12px' } }, nativeText(detail.fields.imgt_annotations))) : null,
      h('details', h('summary', 'Missing fields'), h('p.muted', { style: { overflowWrap: 'anywhere' } }, detail.fieldOrder.filter((name) => detail.fields[name] === '').join(', '))));
  });

  async function inspect(id) {
    const token = ++selection;
    const context = key();
    detail = null; app.inspector.render();
    try {
      const result = await bcellInputs({ ...request(), id });
      if (token !== selection || context !== key()) return;
      detail = result.rows[0];
      app.focusItem({ kind: 'bcell-input', id });
      app.inspector.render();
      return detail;
    } catch (error) { if (token === selection) toast(error.message, { kind: 'error' }); throw error; }
  }

  function paint() {
    clear(body);
    body.append(h('h2', 'Inputs'), h('p.muted', 'Supplied sequence evidence for IGH, IGK and IGL. These counts are rearrangements, not cells or clonal families. Structural checks do not establish a germline or mutation rate.'));
    if (!answer) return;
    const filter = datasetFilter(store.project, dataset());
    body.append(h('p', `Filter: ${filter?.name ?? 'None'}${filter ? ` (version ${filter.version})` : ''}. Definition: ${answer.describe}. The definition is context; these rows are not grouped into clonotypes.`));
    if (!answer.count) { body.append(h('p', 'No B-cell rearrangements under this filter. Open BCR AIRR or IMGT data or change the filter in QC.')); return; }
    body.append(table(['Sample', 'Locus', 'Rows', 'Sequence', 'Both alignments', 'Structurally valid', 'Region coordinates'], answer.samples.map((s) => h('tr', ...[s.sample, s.locus, s.rows, s.sequence, s.paired, s.valid, s.regions].map(cell)))));
    body.append(h('p.muted', 'Region coordinates counts rows with supplied FWR/CDR coordinates, including incomplete or invalid ones. Inspect the row before interpreting them. Missing evidence is not filled in.'));
    body.append(table(['Sequence ID', 'Sample', 'Locus', 'Sequence', 'Alignment evidence', 'Findings'], answer.rows.map((r) => h('tr',
      h('td', h('button.btn.small', { type: 'button', onclick: () => inspect(r.id).catch(() => {}) }, r.sequenceId || 'No sequence ID')),
      cell(r.sample), cell(r.locus), cell(yes(r.evidence.sequence)), cell(r.evidence.valid ? 'Structurally valid' : r.evidence.paired ? 'Needs attention' : 'Missing pair'),
      cell(r.evidence.issues.map((i) => i.field).join(', ') || 'None')))));
    body.append(h('div', { style: { display: 'flex', gap: '12px', alignItems: 'center', marginTop: '12px' } },
      h('button.btn.small', { disabled: offset === 0, onclick: () => { offset = Math.max(0, offset - 50); reload(); } }, 'Previous'),
      h('span', `${answer.offset + 1}–${Math.min(answer.offset + answer.rows.length, answer.count)} of ${answer.count} rearrangements`),
      h('button.btn.small', { disabled: offset + answer.rows.length >= answer.count, onclick: () => { offset += 50; reload(); } }, 'Next')));
  }

  async function load() {
    const token = ++generation; selection++; detail = null; answer = null;
    app.inspector.render();
    const d = dataset();
    clear(select).append(...store.project.datasets.map((x) => h('option', { value: x.id, selected: x.id === d?.id }, x.name)));
    select.disabled = store.project.datasets.length < 2;
    lastKey = key();
    clear(body).append(h('p.muted', d ? 'Reading sequence evidence…' : 'Open BCR data to inspect its sequence evidence.'));
    if (!d) return;
    try {
      const result = await bcellInputs(request());
      if (token !== generation) return;
      answer = result; paint(); return result;
    } catch (error) { if (token === generation) clear(body).append(h('p', error.message)); throw error; }
  }
  // UI events should display errors without unhandled rejections; remote callers receive them.
  const reload = () => load().catch(() => {});
  const view = { async show(id) { chosen = id; offset = 0; return load(); }, inspect, page: () => answer };
  app.bcellsView = view;
  reload();
  return {
    update(topics) {
      if (topics.has('focus') && store.ui.focus?.kind === 'dataset' && chosen !== store.ui.focus.id) { chosen = store.ui.focus.id; offset = 0; reload(); }
      else if (topics.has('project-loaded') || topics.has('datasets') || (topics.has('project') && lastKey !== key())) { offset = 0; reload(); }
    },
    destroy() { generation++; selection++; if (app.bcellsView === view) app.bcellsView = null; app.inspector.setSection('bcell-input', null); root.remove(); },
  };
}

// Native annotations are displayed as text; invalid JSON from an AIRR input remains inspectable.
function nativeText(value) {
  try { return JSON.stringify(JSON.parse(value), null, 2); } catch { return value; }
}

export function mountBCells(app, container) {
  const root = h('div.view');
  const tabs = h('div.segmented', { role: 'group', 'aria-label': 'B-cell analyses' });
  const content = h('div', { style: { display: 'contents' } });
  root.append(h('div.workbench-head', h('h1', icon('lineage'), 'B cells'), tabs), content); container.append(root);
  let part = '', child = null;
  function show(next) {
    if (part === next) return;
    part = next; child?.destroy(); clear(content); clear(tabs);
    for (const [id, label] of [['inputs', 'Inputs'], ['families', 'Families']]) tabs.append(h(`button${part === id ? '.active' : ''}`, { type: 'button', 'aria-pressed': part === id, onclick: () => show(id) }, label));
    child = (part === 'families' ? mountFamilies : mountInputs)(app, content);
  }
  const focused = () => app.store.ui.focus?.kind === 'result' && app.store.project.artifacts.some((r) => r.id === app.store.ui.focus.id && r.kind === 'families');
  const view = { show }; app.bcellsMode = view;
  show(focused() ? 'families' : 'inputs');
  return { update(topics) { if (topics.has('focus') && focused()) show('families'); child?.update(topics); }, destroy() { child?.destroy(); if (app.bcellsMode === view) app.bcellsMode = null; root.remove(); } };
}
