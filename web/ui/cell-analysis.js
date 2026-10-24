// Saved expression analyses. Scientific transforms, graph construction and coordinates come
// from Go; this component only chooses settings, follows jobs and displays the result.
import { h, clear } from './dom.js';
import { addResult } from '../lib/results.js';
import { cellMap } from '../lib/cell-map.js';

async function json(url, body) {
  const response = await fetch(url, body === undefined ? { cache: 'no-store' } : { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
  const result = await response.json();
  if (!response.ok) throw new Error(result.error ?? `HTTP ${response.status}`);
  return result;
}
export function mountCellAnalysis(app, container, source) {
  const { store } = app;
  const root = h('section'); container.append(root);
  let serial = 0, answer = null, job = '', status = 'idle', error = '', color = -1, offset = 0, tableKind = 'cells';
  let projectGeneration = store.state.generation, inputID = '', loaded = '';
  const fields = [
    ['minGenes', 'Minimum detected genes per cell', 1, 1, 200000], ['minCells', 'Minimum cells per gene', 1, 1, 10000],
    ['features', 'Selected variable features', 500, 2, 1000], ['components', 'Principal components', 30, 2, 100],
    ['neighbors', 'Other neighbors per cell', 15, 2, 100], ['target', 'Normalized count total', 10000, 1, 1000000],
    ['seed', 'Map seed', 20261024, 1, Number.MAX_SAFE_INTEGER], ['epochs', 'UMAP epochs', 500, 10, 1000], ['minDist', 'UMAP minimum distance', 0.5, 0, 1],
  ].map(([key, label, value, min, max]) => ({ key, label, input: h('input.input', { type: 'number', value, min, max, step: key === 'minDist' ? '0.01' : '1', 'aria-label': label }) }));
  const controls = h('details', h('summary', 'Preprocessing and map settings'), h('div.btn-row', ...fields.map(({ label, input }) => h('label.field', h('span', label), input))),
    h('p.muted', 'All eligible expression cells are analyzed independently of receptor coverage. Gene Expression counts only; total normalization and ln(1+x), variance-ranked features, centered PCA without variance scaling, exact Euclidean neighbors and UMAP with PCA initialization. This does not apply receptor QC or correct batches. Limits: 100000 source barcodes, 10000 retained cells, 1000 selected features; large dense PCA work is refused.'));
  const button = (label, action, disabled = false) => h('button.btn.small', { type: 'button', disabled, onclick: () => Promise.resolve().then(action).catch(() => {}) }, label);
  function keep(value) {
    if (store.project.artifacts.some((r) => r.id === value.artifact.id)) return;
    store.commit(addResult(store.project, { id: value.artifact.id, kind: 'expression-map', dataset: '', expression: value.result.input.id, name: value.artifact.name, made: value.artifact.created, method: 'PCA / UMAP', describe: value.result.description, citation: value.artifact.lineage.method.citation }), 'Keep expression map', ['project']);
  }
  async function show(id) {
    const token = ++serial, sameProject = store.sameProject(); status = 'loading'; error = ''; draw();
    try {
      const value = await json(`api/expression-map/${encodeURIComponent(id)}`);
      if (token !== serial || !sameProject()) return;
      answer = value; loaded = id; color = -1; offset = 0; status = 'ready'; draw(); return value;
    } catch (e) { if (token === serial && sameProject()) { status = 'failed'; error = e.message; draw(); } throw e; }
  }
  async function run(options) {
    const input = source(); if (!input) throw new Error('Open an expression input first.');
    const token = ++serial, sameProject = store.sameProject(); status = 'loading'; error = ''; answer = null; loaded = ''; job = ''; draw();
    try {
      const selected = options ?? Object.fromEntries(fields.map(({ key, input }) => [key, Number(input.value)]));
      const start = await json('api/expression-map', { id: input.id, options: selected });
      if (token !== serial || !sameProject()) return;
      if (!start.ready) {
        job = start.job; status = 'running'; draw();
        const finished = await app.jobs.finished(job);
        if (token !== serial || !sameProject()) return;
        if (finished?.state !== 'complete') throw new Error(finished?.error || `Expression map ${finished?.state ?? 'unavailable'}.`);
      }
      const value = await json(`api/expression-map/${encodeURIComponent(start.artifact)}`);
      if (token !== serial || !sameProject()) return;
      answer = value; loaded = start.artifact; job = ''; status = 'ready'; offset = 0; color = -1; keep(value); draw(); return value;
    } catch (e) { if (token === serial && sameProject()) { job = ''; status = 'failed'; error = e.message; draw(); } throw e; }
  }
  function draw() {
    clear(root); root.append(h('h2#computed-cell-map', 'Computed expression map'));
    root.append(h('p', 'Compute an exploratory UMAP and retain its PCA and expression-neighbor graph. Supplied coordinates remain separate. Neighborhoods use all retained principal components, not distances on the two-dimensional map.'), controls,
      button('Compute expression map', () => run(), !source() || status === 'running' || status === 'loading'));
    const maps = store.project.artifacts.filter((a) => a.kind === 'expression-map');
    if (maps.length) root.append(h('div.btn-row', h('span', 'Saved maps'), ...maps.map((m) => button(m.name, () => show(m.id), status === 'running'))));
    if (job) root.append(button('Cancel map', () => app.jobs.cancel(job)));
    root.append(h('p', { role: 'status' }, error || (status === 'running' ? (app.jobs.get(job)?.message || 'Computing expression map… See Tasks for progress.') : status === 'loading' ? 'Reading expression analysis…' : '')));
    if (!answer) return;
    const r = answer.result.analysis;
    root.append(h('p', `${r.cells.length} retained cells · ${r.excluded.length} excluded · ${r.features.length} selected features · ${r.components} principal components · ${r.neighbors.k} other neighbors · ${r.graphComponents} connected graph components.`));
    const points = r.cells.map((c) => ({ ...c, expression: true }));
    const figure = cellMap(points, color, { axes: r.axes, supplied: false });
    const selected = h('p', { role: 'status', 'aria-label': 'Selected computed cell' });
    const describe = (c) => { selected.textContent = `${c.barcode} · ${c.total} Gene Expression counts · ${c.genes} detected genes · ${c.reason || c.position.join(', ')}${r.columns.map((name, i) => ` · ${name}: ${c.values[i] || 'Missing'}`).join('')}`; };
    root.append(h('label.field', h('span', 'Color by supplied annotation'), h('select.input', { 'aria-label': 'Computed map annotation', onchange: (event) => { color = Number(event.target.value); draw(); } }, h('option', { value: '-1', selected: color === -1 }, 'One color'), ...r.columns.map((name, i) => h('option', { value: String(i), selected: color === i }, name)))),
      h('div', { html: figure.svg, onclick: (event) => { const index = event.target.closest('[data-cell]')?.dataset.cell; if (index !== undefined) describe(r.cells[Number(index)]); } }),
      h('div.plot-legend', ...figure.legend.map((entry) => h('span', h(`span.swatch.${entry.className}`), entry.name))), h('p.muted', figure.description), selected,
      h('details', h('summary', 'Method, settings and provenance'), h('p', answer.result.description), h('p', answer.artifact.lineage.method.citation), h('p', { style: { overflowWrap: 'anywhere' } }, `Expression ${answer.result.input.id} · Result ${answer.artifact.id}`), h('pre', { style: { whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' } }, JSON.stringify(answer.artifact.lineage.parameters, null, 2))),
      h('a.btn.small', { href: `api/expression-map/${encodeURIComponent(answer.artifact.id)}`, download: `${answer.artifact.id}.json` }, 'Download result and provenance'));
    root.append(h('div.btn-row', ...['cells', 'excluded', 'features'].map((kind) => button({ cells: 'Mapped cells', excluded: 'Excluded cells', features: 'Selected features' }[kind], () => { tableKind = kind; offset = 0; draw(); }))));
    const entries = r[tableKind], rows = entries.slice(offset, offset + 50);
    const heads = tableKind === 'features' ? ['Feature ID', 'Name', 'Detected cells', 'Log mean', 'Log variance'] : ['Barcode', 'Gene Expression counts', 'Detected genes', ...(tableKind === 'excluded' ? ['Exclusion'] : r.axes), ...r.columns];
    const values = (c) => tableKind === 'features' ? [c.id, c.name, c.detected, c.mean, c.variance] : [button(c.barcode, () => describe(c)), c.total, c.genes, ...(c.reason ? [c.reason] : c.position), ...c.values.map((v) => v || 'Missing')];
    root.append(h('div.table-scroll', { tabIndex: 0 }, h('table.data', h('thead', h('tr', ...heads.map((v) => h('th', { scope: 'col' }, v)))), h('tbody', ...rows.map((c) => h('tr', ...values(c).map((v) => h('td', v))))))),
      h('div.btn-row', button('Previous rows', () => { offset = Math.max(0, offset - 50); draw(); }, offset === 0), h('span', `${offset} offset · ${entries.length} ${tableKind}`), button('Next rows', () => { offset += 50; draw(); }, offset + 50 >= entries.length)));
  }
  const view = { run, show, state: () => ({ status, error, answer, artifact: loaded }) }; app.cellAnalysisView = view;
  const focused = () => store.ui.focus?.kind === 'result' && store.project.artifacts.find((a) => a.id === store.ui.focus.id && a.kind === 'expression-map');
  draw(); if (focused()) show(focused().id).catch(() => {});
  return {
    update(topics) {
      if (projectGeneration !== store.state.generation || inputID !== (source()?.id ?? '')) {
        projectGeneration = store.state.generation; inputID = source()?.id ?? ''; serial++; answer = null; loaded = ''; job = ''; status = 'idle'; error = ''; draw();
      }
      const target = focused();
      if (topics.has('focus') && target && target.id !== loaded) show(target.id).catch(() => {});
      else if ((topics.has('jobs') && status === 'running') || topics.has('project') || topics.has('expression')) draw();
    },
    destroy() { serial++; if (app.cellAnalysisView === view) delete app.cellAnalysisView; root.remove(); },
  };
}
