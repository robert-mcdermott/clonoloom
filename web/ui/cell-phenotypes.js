// Go supplies immutable cell sizes, coverage and phenotype denominators. JS only lays out,
// filters and selects saved observations; it never recomputes a scientific count or fraction.
import { h, clear } from './dom.js';
import { cellMap } from '../lib/cell-map.js';
import { addResult } from '../lib/results.js';
import { datasetDefinition, sameMeaning } from '../lib/definitions.js';
import { datasetFilter } from '../lib/filters.js';
import { studySamples } from '../lib/datasets.js';

async function json(url, body) {
  const response = await fetch(url, body === undefined ? { cache: 'no-store' } : { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
  const result = await response.json(); if (!response.ok) throw new Error(result.error ?? `HTTP ${response.status}`); return result;
}
export function mountCellPhenotypes(app, container, source) {
  const { store } = app, root = h('section'); container.append(root);
  let serial = 0, answer = null, status = 'idle', error = '', job = '', loaded = '', generation = store.state.generation, inputKey = '';
  let mapID = '', population = 'receptor', annotation = '', color = 'size', table = 'cells', offset = 0, query = '', clone = '';
  let selected = new Set(), undo = [];
  const breaks = h('input.input', { value: '1, 2, 5', 'aria-label': 'Clonotype size breakpoints', style: { width: '120px' } });
  const button = (label, action, disabled = false) => h('button.btn.small', { type: 'button', disabled, onclick: () => Promise.resolve().then(action).catch((e) => { error = e.message; draw(); }) }, label);
  const select = (label, value, choices, change) => h('label.field', h('span', label), h('select.input', { 'aria-label': label, onchange: (e) => change(e.target.value) }, ...choices.map(([v, name]) => h('option', { value: v, selected: v === value }, name))));
  const percent = (n) => n == null ? 'Not evaluable' : `${(100 * n).toFixed(1)}%`;
  const sourceKey = () => JSON.stringify([source()?.id, source()?.dataset, source()?.sample]);
  function resetSelection() { selected = new Set(); undo = []; clone = ''; query = ''; offset = 0; }
  function selection(barcodes) { undo.push({ selected: [...selected], clone }); if (undo.length > 30) undo.shift(); selected = new Set(barcodes); draw(); app.inspector.render(); }
  function chooseClone(id) { const cells = answer?.result.analysis.cells ?? []; selection(cells.filter((c) => c.clone === id).map((c) => c.barcode)); clone = id; offset = 0; draw(); }
  function consumePending() {
    const p = app.pendingCellClone, doc = answer?.result;
    if (!p || !doc) return;
    if (p.dataset === doc.dataset && sameMeaning(p.definition, doc.analysis.definition) && JSON.stringify(p.filter ?? {}) === JSON.stringify(doc.analysis.filter)) { chooseClone(p.id); delete app.pendingCellClone; }
  }
  async function show(id) {
    const token = ++serial, sameProject = store.sameProject(); status = 'loading'; error = ''; answer = null; loaded = ''; resetSelection(); draw();
    try { const value = await json(`api/phenotypes/${encodeURIComponent(id)}`); if (serial !== token || !sameProject()) return; answer = value; loaded = id; resetSelection(); status = 'ready'; consumePending(); draw(); app.inspector.render(); return value; }
    catch (e) { if (serial === token && sameProject()) { error = e.message; status = 'failed'; draw(); } throw e; }
  }
  async function run(options = {}) {
    const input = source(), d = store.project.datasets.find((d) => d.id === input?.dataset);
    if (!input || !d || input.sample == null) throw new Error('Open expression evidence with an explicit receptor dataset and sample first.');
    const token = ++serial, sameProject = store.sameProject(); status = 'loading'; error = ''; answer = null; loaded = ''; job = ''; resetSelection(); draw();
    try {
      const state = await app.ensureDataset(d); if (serial !== token || !sameProject()) return;
      if (state.status !== 'open') throw new Error(state.error ?? 'Reopen the receptor dataset.');
      const metadata = studySamples(store.project, d.id, state.summary)[input.sample] ?? { name: d.samples[input.sample] };
      const filterRecord = datasetFilter(store.project, d);
      const request = { ...(filterRecord ? { filterRecord: { id: filterRecord.id, name: filterRecord.name, version: filterRecord.version, reason: filterRecord.reason } } : {}), id: input.id, dataset: d.id, sample: input.sample, map: options.map ?? mapID, definition: datasetDefinition(d), filter: datasetFilter(store.project, d)?.rules ?? {}, annotation: metadata,
        options: { population: options.population ?? population, annotation: options.annotation ?? annotation, breakpoints: options.breakpoints ?? breaks.value.split(',').map((v) => Number(v.trim())) } };
      const start = await json('api/phenotypes', request); if (serial !== token || !sameProject()) return;
      if (!start.ready) { job = start.job; status = 'running'; draw(); const done = await app.jobs.finished(job); if (serial !== token || !sameProject()) return; if (done?.state !== 'complete') throw new Error(done?.error || `Phenotype analysis ${done?.state ?? 'unavailable'}.`); }
      const value = await json(`api/phenotypes/${encodeURIComponent(start.artifact)}`); if (serial !== token || !sameProject()) return;
      answer = value; loaded = start.artifact; job = ''; status = 'ready';
      if (!store.project.artifacts.some((r) => r.id === loaded)) store.commit(addResult(store.project, { id: loaded, kind: 'phenotypes', dataset: d.id, expression: input.id, name: value.artifact.name, made: value.artifact.created, method: 'Clone sizes', describe: value.result.description, citation: value.artifact.lineage.method.citation }), 'Keep clonotype phenotypes', ['project']);
      consumePending(); draw(); app.inspector.render(); return value;
    } catch (e) { if (serial === token && sameProject()) { job = ''; status = 'failed'; error = e.message; draw(); } throw e; }
  }
  function draw() {
    clear(root); root.append(h('h2#clonotype-phenotypes', 'Clonotypes and phenotypes'));
    const input = source(), columns = app.cellsView?.state().page?.summary.annotations?.columns ?? [];
    const maps = store.project.artifacts.filter((a) => a.kind === 'expression-map' && a.expression === input?.id);
    if (mapID && !maps.some((m) => m.id === mapID)) mapID = '';
    if (annotation && !columns.includes(annotation)) annotation = '';
    root.append(h('p', 'Link one sample’s clonotypes to imported or computed coordinates. Sizes count distinct cells after the saved definition and QC rules. Phenotypes are supplied annotations; samples and subjects remain separate.'),
      h('h3', 'Settings for the next result'), h('div.btn-row', select('Overlay map', mapID, [['', 'Imported coordinates'], ...maps.map((m) => [m.id, m.name])], (v) => { mapID = v; }),
        select('Size population', population, [['receptor', 'All eligible receptor cells in sample'], ['mapped', 'Mapped eligible cells only']], (v) => { population = v; }),
        select('Phenotype annotation', annotation, [['', 'Whole expression sample'], ...columns.map((v) => [v, v])], (v) => { annotation = v; }),
        h('label.field', h('span', 'Inclusive size breakpoints'), breaks), button('Compute clonotype phenotypes', () => run(), !input?.dataset || status === 'running' || status === 'loading')));
    if (!input?.dataset) root.append(h('p.muted', 'Unlinked expression input: choose a receptor dataset and sample when opening evidence. Clonotype size is not available.'));
    if (job) root.append(button('Cancel phenotypes', () => app.jobs.cancel(job)));
    const saved = store.project.artifacts.filter((a) => a.kind === 'phenotypes');
    if (saved.length) root.append(h('div.btn-row', h('span', 'Saved phenotypes'), ...saved.map((a) => button(`${a.name} · ${a.id.slice(-6)}`, () => show(a.id), status === 'running'))));
    root.append(h('p', { role: 'status' }, error || (status === 'running' ? app.jobs.get(job)?.message || 'Counting cells…' : status === 'loading' ? 'Reading phenotype result…' : '')));
    if (app.pendingCellClone) root.append(h('p', 'A clonotype is selected in Clonotypes. Open or compute an overlay with the same dataset, definition and QC filter to highlight its cells.'));
    if (!answer) return;
    const doc = answer.result, r = doc.analysis, coverage = r.coverage;
    root.append(h('p', `${r.sample} · Subject: ${doc.annotation.subject || 'Missing'} · Timepoint: ${doc.annotation.timepoint || 'Missing'} · ${r.cells.length} expression cells · ${coverage.mapped} mapped · ${coverage.matched} raw receptor matches · ${coverage.expressionOnly} expression only · ${coverage.receptorOnly} receptor only.`),
      h('p', `Size population: ${r.options.population === 'receptor' ? 'all eligible receptor cells in this sample' : 'mapped eligible cells only'} · ${coverage.eligible} uniquely assigned receptor cells · ${coverage.assigned} assigned expression cells · ${coverage.sized} expression cells with a size. Definition: ${r.definitionId} · QC: ${r.filterId} · Phenotype: ${r.options.annotation || 'whole expression sample'}.`));
    const choices = [['size', 'Clonotype size (continuous)'], ['bin', 'Clonotype size category'], ['status', 'Receptor / assignment status'], ['chains', 'Productive chain evidence'], ['sample', 'Sample'], ['subject', 'Subject'], ['timepoint', 'Timepoint'], ...r.columns.map((v, i) => [`annotation:${i}`, v])];
    if (!choices.some(([v]) => v === color)) color = 'size';
    root.append(select('Overlay coloring', color, choices, (v) => { color = v; draw(); }));
    const points = r.cells.filter((c) => c.position), isSize = color === 'size' || color === 'bin';
    const label = (c) => color === 'size' ? (c.size == null ? c.status : String(c.size)) : color === 'bin' ? c.bin || c.status : color === 'sample' ? r.sample : color === 'subject' ? doc.annotation.subject || '' : color === 'timepoint' ? doc.annotation.timepoint || '' : color.startsWith('annotation:') ? c.values[Number(color.split(':')[1])] : c[color];
    const missing = (c) => isSize ? c.size == null : color === 'status' ? c.status !== 'Assigned' : color === 'chains' ? ['No receptor evidence', 'QC excluded', 'No productive chains', 'Unresolved locus'].includes(c.chains) : !label(c);
    if (!points.length) root.append(h('p', 'No cells have coordinates for this map. Phenotype tables still include all expression cells.'));
    const values = points.filter((c) => c.size != null).map((c) => c.size);
    const figure = cellMap(points, -1, { axes: doc.axes, supplied: doc.supplied }, { label, missing, missingLabel: 'Missing / unresolved evidence (see table)', continuous: color === 'size', min: values.length ? Math.min(...values) : 0, max: values.length ? Math.max(...values) : 0, value: (c) => c.size, selected });
    const plot = h('div.cell-overlay-plot', { html: figure.svg }); root.append(plot,
      h('div.plot-legend', ...figure.legend.map((entry) => h('span', h(`span.swatch.${entry.className}`), entry.name))),
      h('p.muted', `${figure.description} Drag a rectangle to select cells, or use the table and barcode filter. Gray means missing or unresolved evidence; the table distinguishes each reason. Selection dims other cells and never changes sizes or denominators.`),
      h('div.btn-row', button('Reset cell selection', () => { selection([]); clone = ''; draw(); }, !selected.size), button('Undo cell selection', () => { const previous = undo.pop(); selected = new Set(previous.selected); clone = previous.clone; draw(); app.inspector.render(); }, !undo.length)));
    let down = null, rectangle = null, dragged = false;
    const svg = plot.querySelector('svg');
    const point = (event) => { const p = svg.createSVGPoint(); p.x = event.clientX; p.y = event.clientY; return p.matrixTransform(svg.getScreenCTM().inverse()); };
    svg.style.touchAction = 'none';
    svg.onpointerdown = (event) => { if (event.button !== 0) return; down = point(event); dragged = false; svg.setPointerCapture(event.pointerId); };
    svg.onpointermove = (event) => { if (!down) return; const p = point(event); if (Math.abs(p.x - down.x) + Math.abs(p.y - down.y) < 4) return; dragged = true; if (!rectangle) { rectangle = document.createElementNS('http://www.w3.org/2000/svg', 'rect'); rectangle.setAttribute('class', 'cell-map-brush'); svg.append(rectangle); } for (const [key, value] of Object.entries({ x: Math.min(p.x, down.x), y: Math.min(p.y, down.y), width: Math.abs(p.x - down.x), height: Math.abs(p.y - down.y) })) rectangle.setAttribute(key, value); };
    svg.onpointerup = (event) => { if (!down) return; const p = point(event), start = down; down = null; if (dragged) { const chosen = [...svg.querySelectorAll('[data-cell]')].filter((c) => Number(c.getAttribute('cx')) >= Math.min(p.x, start.x) && Number(c.getAttribute('cx')) <= Math.max(p.x, start.x) && Number(c.getAttribute('cy')) >= Math.min(p.y, start.y) && Number(c.getAttribute('cy')) <= Math.max(p.y, start.y)).map((c) => points[Number(c.dataset.cell)].barcode); clone = ''; selection(chosen); } };
    svg.onpointercancel = () => { down = null; rectangle?.remove(); rectangle = null; };
    svg.onclick = (event) => { if (dragged) { dragged = false; return; } const index = event.target.closest('[data-cell]')?.dataset.cell; if (index !== undefined) { clone = ''; selection([points[Number(index)].barcode]); } };
    root.append(h('details', h('summary', 'Phenotype method and provenance'), h('p', doc.description), h('p', answer.artifact.lineage.method.citation), h('p', { style: { overflowWrap: 'anywhere' } }, `Result ${loaded} · Expression ${doc.input.id} · Map ${doc.map || 'imported source coordinates'}`), h('pre', { style: { whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' } }, JSON.stringify(answer.artifact.lineage.parameters, null, 2))),
      h('a.btn.small', { href: `api/phenotypes/${encodeURIComponent(loaded)}`, download: `${loaded}.json` }, 'Download phenotypes and provenance'));
    root.append(h('div.btn-row', ...[['cells', 'Overlay cells'], ['clones', 'Clone coverage'], ['groups', 'Phenotype groups'], ['associations', 'Phenotypes per clone']].map(([key, label]) => button(label, () => { table = key; offset = 0; draw(); }))));
    const filter = h('input.input', { type: 'search', value: query, 'aria-label': 'Filter phenotype table', placeholder: 'Barcode, clone ID or label', onchange: (e) => { query = e.target.value; offset = 0; draw(); } });
    root.append(filter, button('Select matching cells', () => { clone = ''; selection(r.cells.filter((c) => matches(c)).map((c) => c.barcode)); }));
    const matches = (row) => !query || JSON.stringify(row).toLowerCase().includes(query.toLowerCase());
    const rows = r[table].filter(matches);
    const heads = { cells: ['Cell', 'Clonotype', 'Size', 'Category', 'Assignment', 'Chains', ...doc.axes, ...r.columns], clones: ['Clonotype', 'Receptor cells', 'Expression cells', 'Mapped cells', 'Chosen size'], groups: ['Annotation', 'Expression cells', 'Mapped', 'Assigned', 'Sized denominator', 'Expanded', 'Expanded share', 'Clonotypes'], associations: ['Clonotype', 'Annotation', 'Cells', 'Mapped', 'Clone denominator', 'Within clone', 'Group denominator', 'Within group'] }[table];
    const groupLabel = (value) => r.options.annotation ? value || 'Missing annotation (no value)' : 'Whole expression sample';
    const cloneButton = (id) => button(id || 'No unique assignment', () => chooseClone(id), !id);
    const data = (c) => table === 'cells' ? [button(c.barcode, () => { clone = ''; selection([c.barcode]); }), cloneButton(c.clone), c.size ?? 'Not available', c.bin || '—', c.status, c.chains, ...(c.position ?? ['Missing', 'Missing']), ...c.values.map((v) => v || 'Missing')] : table === 'clones' ? [cloneButton(c.id), c.receptor, c.expression, c.mapped, c.size || 'No cells in population'] : table === 'groups' ? [groupLabel(c.value), c.cells, c.mapped, c.assigned, c.sized, c.expanded, percent(c.expandedShare), c.clonotypes] : [cloneButton(c.clone), groupLabel(c.value), c.cells, c.mapped, c.cloneExpression, percent(c.cloneShare), c.groupExpression, percent(c.groupShare)];
    root.append(h('div.table-scroll', { tabIndex: 0 }, h('table.data', h('thead', h('tr', ...heads.map((v) => h('th', { scope: 'col' }, v)))), h('tbody', ...rows.slice(offset, offset + 50).map((c) => h('tr', ...data(c).map((v) => h('td', v))))))),
      h('div.btn-row', button('Previous phenotype rows', () => { offset = Math.max(0, offset - 50); draw(); }, !offset), h('span', `${offset} offset · ${rows.length} rows`), button('Next phenotype rows', () => { offset += 50; draw(); }, offset + 50 >= rows.length)));
  }
  app.inspector.setSection('cell-phenotypes', () => {
    if (!answer || !selected.size) return null;
    const rows = answer.result.analysis.cells.filter((c) => selected.has(c.barcode));
    return h('section.inspector-section', h('h3', 'Selected expression cells'), h('p', `${answer.result.analysis.sample} · ${rows.length} selected cells`), h('p.small', 'Selection does not change the saved size population.'),
      h('div', { style: { maxHeight: '420px', overflow: 'auto' } }, ...rows.slice(0, 100).map((c) => h('p.small', h('b', c.barcode), h('br'), c.status, ' · ', c.chains, h('br'), c.clone ? button(c.clone, () => chooseClone(c.clone)) : `Candidate IDs: ${c.candidates.join(', ') || 'none'}`, c.size == null ? ' · Size not available' : ` · Size ${c.size}`))), rows.length > 100 ? h('p.small', 'First 100 selected cells; use the table filter for all observations.') : null);
  });
  const view = { run, show, selectClone: chooseClone, selectCells: selection, state: () => ({ status, error, artifact: loaded, answer, selected: [...selected], clone }) }; app.cellPhenotypesView = view;
  const focused = () => store.ui.focus?.kind === 'result' && store.project.artifacts.find((a) => a.id === store.ui.focus.id && a.kind === 'phenotypes');
  inputKey = sourceKey(); draw();
  const initial = focused() || (app.pendingCellClone && store.project.artifacts.filter((a) => a.kind === 'phenotypes' && a.dataset === app.pendingCellClone.dataset).at(-1));
  if (initial) show(initial.id).catch(() => {});
  return {
    update(topics) {
      if (generation !== store.state.generation || inputKey !== sourceKey()) { if (generation !== store.state.generation) delete app.pendingCellClone; generation = store.state.generation; inputKey = sourceKey(); serial++; answer = null; loaded = ''; job = ''; status = 'idle'; error = ''; resetSelection(); draw(); app.inspector.render(); }
      const target = focused(); if (topics.has('focus') && target && target.id !== loaded) show(target.id).catch(() => {});
      else if (topics.has('project') || topics.has('expression') || topics.has('jobs')) draw();
    },
    destroy() { serial++; app.inspector.setSection('cell-phenotypes', null); if (app.cellPhenotypesView === view) delete app.cellPhenotypesView; root.remove(); },
  };
}
