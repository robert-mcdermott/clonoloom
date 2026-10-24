// Topology, scores and ancestral states come from Go; the browser displays saved results.
import { h, clear } from './dom.js';
import { addResult } from '../lib/results.js';
import { exportResult } from './exports.js';
import { studySamples } from '../lib/datasets.js';
import { lineagePlot } from '../lib/lineage.js';
import { figureButton, madeBy } from './figures.js';
import { toast } from './overlays.js';

async function json(url, body) {
  const response = await fetch(url, body ? { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) } : { cache: 'no-store' });
  const value = await response.json();
  if (!response.ok) throw new Error(value.error ?? `HTTP ${response.status}`);
  return value;
}
const td = (value) => h('td', String(value ?? 'Missing'));
const table = (headers, rows) => h('div.table-scroll', { tabIndex: 0 }, h('table.data', h('thead', h('tr', ...headers.map((s) => h('th', { scope: 'col' }, s)))), h('tbody', ...rows)));

export function mountLineages(app, container) {
  const { store } = app;
  const focused = () => store.ui.focus?.kind === 'result' ? store.project.artifacts.find((r) => r.id === store.ui.focus.id && r.kind === 'lineages') : null;
  let chosen = focused()?.dataset ?? (store.ui.focus?.kind === 'dataset' ? store.ui.focus.id : store.project.datasets[0]?.id);
  let saved = focused()?.id ?? null, parent = '';
  let generation = 0, offset = 0, family = '', locus = '', kind = 'trees', page = null, detail = null;
  let tree = null, node = null, field = 'isotype', selection = 0;
  let status = 'idle', error = '', artifact = '', stopJob = null;
  const dataset = () => store.project.datasets.find((d) => d.id === chosen);
  const parents = () => store.project.artifacts.filter((r) => r.dataset === chosen && r.kind === 'germlines');
  const root = h('div.view-body'); container.append(root);
  const button = (label, fn, disabled = false) => h('button.btn.small', { type: 'button', disabled, onclick: () => Promise.resolve().then(fn).catch((e) => toast(e.message, { kind: 'error' })) }, label);
  function keep(answer) {
    if (store.project.artifacts.some((r) => r.id === answer.artifact.id)) return;
    const r = answer.result;
    store.commit(addResult(store.project, { id: answer.artifact.id, kind: 'lineages', dataset: r.dataset, name: `Lineage trees · ${r.datasetName}`, made: answer.artifact.created, method: 'Equal-cost nucleotide parsimony', describe: r.description, lineages: r.lineageOptions, germlineProvenance: r.germlineProvenance, built: r.built, rowCount: answer.rowCount }), `Keep lineages of ${r.datasetName}`, ['project']);
  }
  async function load(start = false) {
    const token = ++generation; selection++; tree = null; node = null;
    stopJob?.(); stopJob = null; page = null; detail = null; app.inspector.render(); status = 'loading'; error = '';
    if (!parent) parent = parents().at(-1)?.id ?? '';
    render();
    if (!dataset() || (!saved && !parent)) { status = 'idle'; render(); return; }
    try {
      if (!saved) {
        const opened = await app.ensureDataset(dataset());
        if (token !== generation) return;
        if (opened.status !== 'open') throw new Error(opened.error ?? 'Open the dataset again.');
        const answer = await json('api/lineages', { dataset: chosen, lineages: { germlines: parent }, study: studySamples(store.project, chosen, opened.summary), start });
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
      const answer = await json(`api/lineages/${encodeURIComponent(artifact)}/page?${query}`);
      if (token !== generation) return;
      page = answer; status = 'ready'; keep(answer); render(); return answer;
    } catch (e) { if (token === generation) { status = 'failed'; error = e.message; render(); } throw e; }
  }
  async function showPage(nextKind = 'trees', nextFamily = '', nextLocus = '', nextOffset = 0) { kind = nextKind; family = nextFamily; locus = nextLocus; offset = nextOffset; return load(); }
  function inspect(row) { detail = row; app.focusItem({ kind: 'bcell-lineage-row', id: row.id }); app.inspector.render(); }
  async function openTree(id) {
    const token = ++selection, current = generation;
    const answer = await json(`api/lineages/${encodeURIComponent(artifact)}/page?kind=detail&tree=${encodeURIComponent(id)}`);
    if (token !== selection || current !== generation) return;
    tree = answer.tree; node = null; render(); return tree;
  }
  async function inspectNode(id) {
    const selected = tree?.nodes?.find((n) => n.id === id);
    if (!selected) return;
    const token = ++selection, current = generation;
    const answer = await json(`api/lineages/${encodeURIComponent(artifact)}/page?kind=rows&tree=${encodeURIComponent(tree.id)}&node=${encodeURIComponent(id)}&limit=50`);
    if (token !== selection || current !== generation) return;
    node = { ...selected, members: answer.rows, memberCount: answer.count }; render();
    root.querySelector('[data-lineage-detail]')?.focus();
  }
  app.inspector.setSection('bcell-lineage-row', () => !detail || store.ui.focus?.kind !== 'bcell-lineage-row' || store.ui.focus.id !== detail.id ? null : h('section.inspector-section', h('h3', 'Lineage source row'), h('p', `${detail.sequenceId} · ${detail.sample} · ${detail.locus}`), h('p.small', { style: { overflowWrap: 'anywhere' } }, `${detail.family} · ${detail.source.name}, line ${detail.line} · SHA-256 ${detail.source.sha256}`), h('p', detail.excluded || `Node ${detail.node}`), h('p', `Isotype: ${detail.isotype || 'Missing'}; timepoint: ${detail.timepoint || 'Missing'}; tissue: ${detail.tissue || 'Missing'}.`)));
  function drawTree() {
    if (!tree) return;
    root.append(h('h3', `${tree.family} · ${tree.locus}`));
    if (tree.excluded) { root.append(h('p.callout', tree.excluded)); return; }
    root.append(h('p', `${tree.included} source rows · ${tree.taxa} distinct states including reference · ${tree.sites} shared sites · ${tree.score} changes.`), h('p.small', `${tree.search}; ${tree.examined} candidate scores. ${tree.optimalTrees == null ? 'Global optimum and topology tie count unknown.' : `${tree.optimalTrees} equally optimal binary topologies.`} ${tree.budgetLimited ? 'Search budget reached.' : ''}`), h('p.small.muted', 'One selected topology. Zero-change edges and binary resolutions do not establish distinct ancestors. Metadata are observed labels; no switching, migration or event dates are inferred.'));
    const select = h('select.input.small', { 'aria-label': 'Tree annotations', onchange: (e) => { field = e.target.value; render(); } }, ...['isotype', 'timepoint', 'tissue', 'sample'].map((v) => h('option', { value: v, selected: field === v }, v)));
    const plot = lineagePlot(tree, field), drawing = h('div.table-scroll', { tabIndex: 0 }); drawing.innerHTML = plot.svg;
    const choose = (e) => { const target = e.target.closest('[data-node]'); if (target && (e.type === 'click' || e.key === 'Enter' || e.key === ' ')) { e.preventDefault(); inspectNode(target.dataset.node).catch((e) => toast(e.message, { kind: 'error' })); } };
    drawing.addEventListener('click', choose); drawing.addEventListener('keydown', choose);
    root.append(h('div.options-row', h('label.kv-inline', 'Observed annotation ', select), figureButton(() => ({ title: `Lineage · ${tree.locus}`, name: tree.id, plot: plot.svg, legend: plot.legend, caption: [plot.description, `${tree.sites} shared V/J sites; ${tree.score} changes; ${tree.search}; ${tree.optimalTrees ?? 'unknown'} optimal binary trees.`, 'Conditional ancestral states; no switching or migration inference.', `Artifact ${artifact}; tree ${tree.id}.`, madeBy(app.version)] }))), drawing, h('p.small.muted', plot.description));
    if (node) {
      const wrap = { style: { overflowWrap: 'anywhere', whiteSpace: 'pre-wrap', textAlign: 'left' } };
      root.append(h('section', h('h3', { tabIndex: -1, 'data-lineage-detail': '' }, `Node ${tree.nodes.findIndex((n) => n.id === node.id) + 1} · ${node.kind}`), h('p.small', wrap, node.id), h('p', `${node.changes} changes from parent; ${node.distance} along the path from reference. ${node.splitTrees == null ? 'Split tie count unavailable.' : `This split occurs in ${node.splitTrees} of ${tree.optimalTrees} optimal binary trees; this is not bootstrap support or a probability.`}`), h('h4', 'Chosen panel sequence'), h('pre', wrap, node.sequence), h('h4', 'Possible states · conditional IUPAC'), h('pre', wrap, node.possible), h('p.small', 'IUPAC symbols give marginal optimal states on this topology. Arbitrary combinations need not form a joint optimal assignment. These concatenated panel sequences omit masked and unavailable sites; they are not full rearrangements.'), table(['Sample', 'Isotype', 'Timepoint', 'Tissue', 'Source rows'], node.annotations.map((a) => h('tr', ...[a.sample, a.isotype || 'Missing', a.timepoint || 'Missing', a.tissue || 'Missing', a.rows].map(td)))), h('p.small', `Showing ${node.members.length} of ${node.memberCount} source rows. Use Source rows for the complete paged membership or export it.`), table(['Sequence', 'Sample', 'Source line'], node.members.map((r) => h('tr', h('td', button(r.sequenceId || r.id, () => inspect(r))), td(r.sample), td(`${r.source.name}:${r.line}`))))));
    }
    root.append(h('details', h('summary', 'All nodes · accessible table'), table(['Node', 'Kind', 'Changes from parent', 'Source rows', 'Split occurrences'], tree.nodes.map((n, i) => h('tr', h('td', button(String(i + 1), () => inspectNode(n.id))), td(n.kind), td(n.changes), td(n.rows), td(n.splitTrees))))), h('details', h('summary', 'Panel coordinates'), h('p.small', { style: { overflowWrap: 'anywhere' } }, tree.columns.map((c) => `${c.segment}:${c.position}`).join(', '))));
  }

  function render() {
    clear(root);
    root.append(h('h2', 'Lineage trees'), h('p.muted', 'Build family/locus trees from saved germlines on shared V/J reference positions. Inspect ancestral ambiguity and observed isotype, timepoint and tissue annotations.'));
    const selector = h('select.input.small', { 'aria-label': 'Lineage dataset', onchange: () => { chosen = selector.value; saved = null; parent = ''; showPage().catch(() => {}); } }, ...store.project.datasets.map((d) => h('option', { value: d.id, selected: d.id === chosen }, d.name)));
    root.append(h('label.kv-inline', 'Dataset ', selector));
    if (!dataset()) { root.append(h('p', 'Open BCR data and save a germline result.')); return; }
    if (saved) root.append(h('p', `Saved result ${saved}. Its germline parent is recorded below.`), button('New analysis', () => { saved = null; return showPage(); }));
    else {
      const select = (label, value, choices, change) => h('label.kv-inline', label, h('select.input.small', { 'aria-label': label, onchange: (e) => { change(e.target.value); showPage().catch(() => {}); } }, ...choices.map(([id, name]) => h('option', { value: id, selected: value === id }, name))));
      root.append(h('div.options-row', select('Saved germlines', parent, parents().map((p) => [p.id, p.name]), (v) => { parent = v; })));
      if (!parent) root.append(h('p', 'Save a Germlines result first.'), button('Open Germlines', () => app.bcellsMode.show('germlines')));
      root.append(h('p.small.muted', 'Current sample timepoint and tissue annotations are pinned when this analysis runs. Identical panel states collapse without abundance weighting. At most 8 states are searched exactly; 9–64 use a bounded heuristic.'));
    }
    if (status === 'loading' || status === 'running') { root.append(h('p', status === 'running' ? 'Searching trees… The job is available in Tasks.' : 'Reading the result…')); return; }
    if (error) root.append(h('p.callout.danger', error));
    if (!page) { if (parent) root.append(button('Build lineage trees', () => load(true))); return; }
    const r = page.result, o = r.lineageOptions;
    root.append(h('p', `${r.built} of ${page.treeCount} family/locus trees built from ${page.rowCount} source rows.`), h('details', h('summary', 'Method and provenance'), h('p.small', r.description), h('p.small', { style: { overflowWrap: 'anywhere' } }, `Parent ${o.germlines} · SHA-256 ${o.germlineSha256}`), ...r.germlineProvenance.files.map((f) => h('p.small', { style: { overflowWrap: 'anywhere' } }, `${f.reference} · ${f.file} · SHA-256 ${f.sha256} · ${f.license}. ${f.citation}. ${f.source}`))));
    for (const [reason, count] of Object.entries(r.excluded)) root.append(h('p.small', `${count} rows excluded: ${reason}.`));
    root.append(h('div.btn-row', button('Trees', () => showPage()), button('Source rows', () => showPage('rows')), button('Export result', async () => { const id = page.artifact.id; const d = dataset(); const opened = await app.ensureDataset(d); if (opened.status !== 'open') throw new Error(opened.error ?? 'Open the dataset again.'); return exportResult(app, d, id); })));
    if (kind === 'trees') {
      drawTree();
      root.append(table(['Family', 'Locus', 'Included / source rows', 'Panel sites', 'States', 'Changes', 'Search / evidence'], page.rows.map((r) => h('tr', h('td', button(r.family, () => openTree(r.id))), td(r.locus), td(`${r.included} / ${r.rows}`), td(r.sites), td(r.taxa), td(r.excluded ? null : r.score), td(r.excluded || `${r.search}${r.budgetLimited ? ' · budget reached' : ''}`)))));
    } else root.append(table(['Sequence', 'Sample', 'Locus', 'Isotype', 'Timepoint', 'Tissue', 'Evidence'], page.rows.map((r) => h('tr', h('td', button(r.sequenceId || r.id, () => inspect(r))), ...[r.sample, r.locus, r.isotype || 'Missing', r.timepoint || 'Missing', r.tissue || 'Missing', r.excluded || r.node].map(td)))));
    root.append(h('div.btn-row', button('Previous', () => showPage(kind, family, locus, Math.max(0, offset - 50)), offset === 0), h('span', page.count ? `${offset + 1}–${Math.min(offset + page.rows.length, page.count)} of ${page.count}` : 'No rows'), button('Next', () => showPage(kind, family, locus, offset + 50), offset + page.rows.length >= page.count)), h('p.small.muted', `Kept ${page.artifact.created} as ${page.artifact.id}.`));
  }
  const view = { state: () => ({ status, error, page, artifact, tree, node }), async show(id, options = {}) { chosen = id; saved = null; parent = options.germlines ?? ''; return showPage(); }, run: () => load(true), showPage, inspect, openTree, inspectNode };
  app.lineagesView = view; load().catch(() => {});
  return { update(topics) {
    const r = focused();
    if (topics.has('focus') && r && r.id !== saved) { chosen = r.dataset; saved = r.id; showPage().catch(() => {}); }
    else if (topics.has('focus') && store.ui.focus?.kind === 'dataset' && store.ui.focus.id !== chosen) { chosen = store.ui.focus.id; saved = null; parent = ''; showPage().catch(() => {}); }
    else if (topics.has('project-loaded') || topics.has('datasets')) { if (!dataset()) chosen = store.project.datasets[0]?.id; if (!saved) { parent = ''; showPage().catch(() => {}); } }
  }, destroy() { generation++; stopJob?.(); if (app.lineagesView === view) app.lineagesView = null; app.inspector.setSection('bcell-lineage-row', null); root.remove(); } };
}
