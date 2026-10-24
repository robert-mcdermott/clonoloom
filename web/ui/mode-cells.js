// Cells starts with raw expression evidence and exact sample-scoped barcode coverage. No
// clonotype count, embedding, normalization or phenotype is inferred by this inspection view.
import { h, icon, clear } from './dom.js';
import { expressionRecord, saveExpression, removeExpression } from '../lib/expression.js';
import { cellMap } from '../lib/cell-map.js';

async function request(endpoint, body, signal) {
  const response = await fetch(`api/expression/${endpoint}`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body), signal });
  const result = await response.json();
  if (!response.ok) throw new Error(result.error ?? 'The expression evidence could not be read.');
  return result;
}
const count = (n) => n === null ? 'Not measured' : n.toLocaleString('en-US');

export function mountCells(app, container) {
  const { store } = app;
  let generation = store.state.generation;
  let active = null, answer = null, status = 'idle', message = '', controller = null, serial = 0, view = 'cells';
  let colorColumn = -1;
  const path = h('input.input', { type: 'text', placeholder: '/path/to/filtered_feature_bc_matrix', 'aria-label': 'MEX directory', style: { width: '100%' } });
  const cellsPath = h('input.input', { type: 'text', placeholder: 'Optional /path/to/cells.tsv', 'aria-label': 'Cell annotations', style: { width: '100%' } });
  const datasets = h('select.input', { 'aria-label': 'Receptor dataset', onchange: drawSamples });
  const samples = h('select.input', { 'aria-label': 'Receptor sample' });
  const open = h('button.btn.primary', { type: 'submit' }, 'Open expression');
  const cancel = h('button.btn', { type: 'button', onclick: () => { controller?.abort(); serial++; status = 'idle'; message = 'Reading cancelled.'; draw(); } }, 'Cancel');
  const saved = h('div');
  const output = h('div');
  const form = h('form', { onsubmit: (event) => {
    event.preventDefault();
    if (datasets.value && samples.value === '') { message = 'Choose the receptor sample explicitly.'; draw(); return; }
    inspect({ path: path.value, cellsPath: cellsPath.value, dataset: datasets.value, sample: datasets.value ? Number(samples.value) : null }).catch(() => {});
  } }, h('label.field', h('span', 'MEX directory'), path),
  h('label.field', h('span', 'Cell annotations and imported coordinates (optional TSV)'), cellsPath),
  h('div.btn-row', h('label.field', h('span', 'Receptor dataset'), datasets), h('label.field', h('span', 'Receptor sample'), samples), open, cancel));
  const root = h('div.view', h('div.workbench-head', h('h1', icon('cell'), 'Cells'), h('span.spacer'),
    h('button.btn', { type: 'button', onclick: () => example(true).catch(() => {}) }, 'Use cell-state study'),
    h('button.btn', { type: 'button', onclick: () => example().catch(() => {}) }, 'Use artificial example')),
  h('div.view-body', h('p', 'Read a 10x MEX count matrix and check its barcode coverage in one receptor sample.'), form, saved, output));
  container.append(root);

  function drawSamples() {
    const d = store.project.datasets.find((x) => x.id === datasets.value);
    clear(samples).append(h('option', { value: '' }, d ? 'Choose a sample…' : 'No receptor binding'), ...(d?.samples ?? []).map((s, i) => h('option', { value: String(i) }, s)));
    samples.disabled = !d;
  }
  function drawDatasets() {
    const previous = datasets.value;
    clear(datasets).append(h('option', { value: '' }, 'Expression only'), ...store.project.datasets.filter((d) => d.singleCell).map((d) => h('option', { value: d.id, selected: d.id === previous }, d.name)));
    drawSamples();
  }
  function drawSaved() {
    clear(saved);
    const records = store.project.expression ?? [];
    if (!records.length) return;
    saved.append(h('h2', 'Saved inputs'), ...records.map((record, i) => {
      const d = store.project.datasets.find((x) => x.id === record.dataset);
      return h('div.btn-row', h('span', `${record.path.split(/[\\/]/).pop()} · ${record.dataset ? `${d?.name ?? 'Missing dataset'} / ${d?.samples[record.sample] ?? record.sample}` : 'expression only'}`),
        h('button.btn.small', { type: 'button', onclick: () => inspect({ ...record, expectedId: record.id }).catch(() => {}) }, 'Reopen'),
        h('button.btn.small', { type: 'button', onclick: () => { store.commit(removeExpression(store.project, i), 'Remove expression input', ['expression']); active = null; answer = null; status = 'idle'; controller?.abort(); serial++; draw(); } }, 'Remove'));
    }));
  }
  async function inspect(options) {
    controller?.abort(); controller = new AbortController(); const signal = controller.signal;
    const run = ++serial, sameProject = store.sameProject();
    status = 'loading'; message = ''; answer = null; active = null; draw();
    try {
      if (options.dataset) {
        const record = store.project.datasets.find((d) => d.id === options.dataset);
        if (!record) throw new Error('The saved receptor dataset is not in this project.');
        const state = await app.ensureDataset(record);
        if (state.status !== 'open') throw new Error(state.error ?? 'The receptor dataset cannot be reopened.');
      }
      if (run !== serial || !sameProject()) return;
      const summary = await request('open', { path: options.path, cellsPath: options.cellsPath, example: options.example, studyExample: options.studyExample, studyPreset: options.studyPreset, expectedId: options.expectedId }, signal);
      const record = expressionRecord(summary, options.dataset ?? '', options.sample ?? null);
      const page = await request('page', { id: summary.id, dataset: record.dataset, sample: record.sample, view: 'cells' }, signal);
      if (run !== serial || !sameProject()) return;
      active = record; answer = page; view = 'cells'; status = 'ready'; path.value = summary.path;
      cellsPath.value = record.cellsPath ?? ''; colorColumn = -1;
      store.commit(saveExpression(store.project, record), options.expectedId ? 'Reopen expression input' : 'Open expression input', ['expression']);
      datasets.value = record.dataset; drawSamples(); samples.value = record.sample === null ? '' : String(record.sample);
      draw(); return page;
    } catch (error) {
      if (run === serial && sameProject()) { status = 'error'; message = error.message; draw(); }
      throw error;
    }
  }
  async function page(nextView = view, offset = 0) {
    if (!active) return;
    const run = ++serial, sameProject = store.sameProject();
    controller?.abort(); controller = new AbortController();
    status = 'loading'; message = ''; draw();
    try {
      const result = await request('page', { id: active.id, dataset: active.dataset, sample: active.sample, view: nextView, offset }, controller.signal);
      if (run !== serial || !sameProject()) return;
      answer = result; view = nextView; status = 'ready'; draw();
    } catch (error) { if (run === serial && sameProject()) { status = 'error'; message = error.message; draw(); } throw error; }
  }
  async function example(study = false) {
    try {
      const sameProject = store.sameProject();
      const result = await app.actions.open_example({ example: study ? 'simulated-cell-states-clouds' : 'synthetic-expression', download: true });
      if (!sameProject()) return;
      await app.setMode('cells');
      return await app.cellsView.inspect({ example: !study, studyExample: study, studyPreset: study ? 'cell-states-clouds' : undefined, dataset: result.data.dataset, sample: 0 });
    } catch (error) { message = error.message; status = 'error'; draw(); throw error; }
  }
  function draw() {
    drawSaved(); clear(output);
    open.disabled = status === 'loading'; cancel.hidden = status !== 'loading';
    output.append(h('p', { role: 'status' }, status === 'loading' ? 'Reading expression evidence…' : message));
    if (!answer) return;
    const s = answer.summary, link = answer.links, annotations = s.annotations;
    output.append(h('h2', 'Source coverage'), h('p', `${count(s.cells)} expression barcodes · ${count(s.features)} features · ${count(s.entries)} stored coordinates, including explicit zeros.`),
      h('p', Object.entries(s.types).map(([type, n]) => `${type}: ${count(n)}`).join(' · ')));
    if (link) output.append(h('div.callout', h('div', h('p', `${count(link.matched)} matched · ${count(link.expressionOnly)} expression only · ${count(link.receptorOnly)} receptor only`),
      h('p', `${count(link.receptorCells)} distinct receptor barcodes in ${link.sampleName}; ${count(link.missingBarcodeRows)} receptor rows without a barcode.`))));
    output.append(h('p.muted', 'Exact barcode matches within the selected sample, before QC or clonotyping. Matching does not establish productivity, singlet status or cell type. Absent expression barcodes are missing, not zero counts.'),
      h('div.btn-row', ...['cells', 'features', ...(annotations ? ['annotations', 'map'] : [])].map((v) => h(`button.btn${view === v ? '.primary' : ''}`, { type: 'button', disabled: status === 'loading', onclick: () => page(v).catch(() => {}) }, { cells: 'Cells', features: 'Features', annotations: 'Annotations', map: 'Imported map' }[v]))));
    if (annotations) output.append(h('p', `${count(annotations.matched)} expression barcodes with annotation rows · ${count(annotations.withoutAnnotation)} without rows · ${count(annotations.annotationOnly)} annotation only. ${count(annotations.mapped)} expression barcodes have coordinates; ${count(annotations.withoutCoordinates)} do not.`));
    const selected = h('p', { role: 'status', 'aria-label': 'Selected cell' });
    const describe = (row) => { selected.textContent = `${row.barcode} · ${row.expression ? 'Expression present' : 'No expression barcode'} · ${annotations.columns.map((name, i) => `${name}: ${row.values[i] || 'Missing'}`).join(' · ')} · Coordinates: ${row.position?.join(', ') ?? 'Missing'}`; };
    if (view === 'map') {
      const points = answer.map ?? [], figure = cellMap(points, colorColumn);
      const choice = h('select.input', { 'aria-label': 'Map annotation', onchange: (e) => { colorColumn = Number(e.target.value); draw(); } }, h('option', { value: '-1', selected: colorColumn === -1 }, 'One color'), ...annotations.columns.map((name, i) => h('option', { value: String(i), selected: colorColumn === i }, name)));
      output.append(h('h2#imported-cell-map', 'Imported map'), h('p', `${count(annotations.mapped)} expression barcodes with supplied coordinates · ${count(annotations.withoutCoordinates)} without coordinates.`),
        h('label.field', h('span', 'Color by supplied annotation'), choice), h('div', { html: figure.svg, onclick: (event) => { const i = event.target.closest('[data-cell]')?.dataset.cell; if (i !== undefined) describe(points[Number(i)]); } }),
        h('div.plot-legend', ...figure.legend.map((entry) => h('span', h(`span.swatch.${entry.className}`), entry.name))), h('p.muted', figure.description));
      if (!points.length) output.append(h('p', 'No expression barcodes have supplied coordinates.'));
    }
    const annotationRows = view === 'annotations' || view === 'map';
    const headings = annotationRows ? ['Barcode', 'Expression', ...annotations.columns, 'map_x', 'map_y'] : view === 'cells' ? ['Barcode', 'Gene-expression UMIs', 'Detected genes', 'Receptor chain rows', ...(annotations?.columns ?? [])] : ['Feature ID', 'Name', 'Type'];
    const rows = annotationRows ? (answer.annotations ?? []).map((r) => [h('button.btn.small', { type: 'button', onclick: () => describe(r) }, r.barcode), r.expression ? 'Present' : 'Missing', ...r.values.map((v) => v || 'Missing'), ...(r.position ?? ['Missing', 'Missing'])]) : view === 'cells' ? answer.cells.map((c) => [c.barcode, count(c.geneUMIs), count(c.genes), c.chainRows === undefined ? 'Not linked' : count(c.chainRows), ...(annotations?.columns ?? []).map((_, i) => c.annotation?.values[i] || 'Missing')]) : answer.features.map((f) => [f.id, f.name, f.type]);
    output.append(selected);
    output.append(h('div', { style: { overflowX: 'auto' } }, h('table.data', h('thead', h('tr', ...headings.map((title) => h('th', { scope: 'col' }, title)))), h('tbody', ...rows.map((row) => h('tr', ...row.map((value) => h('td', value))))))));
    output.append(h('div.btn-row', h('button.btn.small', { type: 'button', disabled: status === 'loading' || answer.offset === 0, onclick: () => page(view, Math.max(0, answer.offset - 50)).catch(() => {}) }, 'Previous'),
      h('span', `${count(answer.offset)} offset · ${count(answer.count)} ${view}`), h('button.btn.small', { type: 'button', disabled: status === 'loading' || answer.offset + 50 >= answer.count, onclick: () => page(view, answer.offset + 50).catch(() => {}) }, 'Next')));
    output.append(h('details', h('summary', 'Sources and reading rules'), h('p', `Reader version ${s.readerVersion}; ${s.id}. Raw integer counts; feature IDs are unique, names may repeat. Duplicate coordinates are rejected. Only Gene Expression features contribute to gene UMIs and detected genes.`),
      ...s.sources.map((source) => h('p', h('strong', `${source.role}: `), source.path, h('br'), h('code', { style: { overflowWrap: 'anywhere' } }, `SHA-256 ${source.sha256}`))),
      h('p', h('a', { href: 'https://www.10xgenomics.com/support/software/cell-ranger/latest/analysis/cr-outputs-mex-matrices', target: '_blank', rel: 'noopener' }, '10x MEX specification'), ' · ', h('a', { href: 'https://math.nist.gov/MatrixMarket/formats.html', target: '_blank', rel: 'noopener' }, 'NIST Matrix Market format'))));
  }
  const api = { inspect, page, example, state: () => ({ status, page: answer, record: active }) };
  app.cellsView = api;
  drawDatasets(); draw();
  return {
    update(topics) {
      if (generation !== store.state.generation) {
        generation = store.state.generation; serial++; controller?.abort(); active = null; answer = null; status = 'idle'; message = ''; path.value = ''; cellsPath.value = ''; drawDatasets(); draw();
      }
      if (active && !(store.project.expression ?? []).some((r) => r.id === active.id && r.dataset === active.dataset && r.sample === active.sample)) {
        serial++; controller?.abort(); active = null; answer = null; status = 'idle'; draw();
      }
      if (topics.has('datasets')) { const sample = samples.value; drawDatasets(); samples.value = sample; }
      drawSaved();
    },
    destroy() { serial++; controller?.abort(); if (app.cellsView === api) delete app.cellsView; root.remove(); },
  };
}
