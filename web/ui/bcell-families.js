// Immutable family results, with bounded pages and stale-request protection.
import { h, clear } from './dom.js';
import { familyOptions, familyWords, familySubjects, useFamilies } from '../lib/families.js';
import { datasetFilter } from '../lib/filters.js';
import { addResult } from '../lib/results.js';
import { columns } from '../lib/plot.js';
import { exportResult } from './exports.js';
import { exportable, madeBy } from './figures.js';
import { toast } from './overlays.js';

async function json(url, body) {
  const response = await fetch(url, body ? { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) } : { cache: 'no-store' });
  const value = await response.json();
  if (!response.ok) throw new Error(value.error ?? `HTTP ${response.status}`);
  return value;
}
const table = (headers, rows) => h('div.table-scroll', { tabIndex: 0 }, h('table.data', h('thead', h('tr', ...headers.map((s) => h('th', { scope: 'col' }, s)))), h('tbody', ...rows)));
const td = (s) => h('td', String(s ?? 'Missing'));

export function mountFamilies(app, container) {
  const { store } = app;
  const focused = () => store.ui.focus?.kind === 'result' ? store.project.artifacts.find((r) => r.id === store.ui.focus.id && r.kind === 'families') : null;
  let chosen = focused()?.dataset ?? (store.ui.focus?.kind === 'dataset' ? store.ui.focus.id : store.project.datasets[0]?.id);
  let saved = focused()?.id ?? null;
  let generation = 0, offset = 0, kind = 'families', family = '', page = null, detail = null;
  let status = 'idle', error = '', artifact = '', stopJob = null, lastKey = '';
  const dataset = () => store.project.datasets.find((d) => d.id === chosen);
  const request = () => ({ dataset: chosen, families: familyOptions(dataset()), filter: datasetFilter(store.project, dataset())?.rules ?? {}, subjects: familySubjects(store.project, chosen, app.datasetStates.get(chosen)?.summary) });
  const key = () => JSON.stringify(request());
  const root = h('div.view-body.family-view'); container.append(root);
  const button = (label, fn, disabled = false) => h('button.btn.small', { type: 'button', disabled, onclick: () => Promise.resolve().then(fn).catch((e) => toast(e.message, { kind: 'error' })) }, label);
  function choose(changes) {
    const next = useFamilies(store.project, chosen, { ...(saved && page ? page.result.familiesOptions : {}), ...changes });
    saved = null; offset = 0; family = ''; kind = 'families';
    store.commit(next, 'Choose B-cell families', ['project']);
    return load(false);
  }
  function keep(answer) {
    if (store.project.artifacts.some((a) => a.id === answer.artifact.id)) return;
    const r = answer.result;
    store.commit(addResult(store.project, { id: answer.artifact.id, kind: 'families', dataset: r.dataset, name: `B-cell families · ${r.datasetName}`, made: answer.artifact.created, method: familyWords(r.familiesOptions), describe: r.description, unit: r.unit, families: r.familiesOptions, filter: r.filter, filterDescription: r.filterDescription, subjects: r.subjects, eligible: r.eligible, familyCount: answer.familyCount, representativeCount: answer.representativeCount }), `Keep B-cell families of ${r.datasetName}`, ['project']);
  }
  async function load(start = false) {
    const token = ++generation;
    stopJob?.(); stopJob = null; detail = null; app.inspector.render(); page = null; status = 'loading'; error = '';
    lastKey = key(); render();
    if (!dataset()) { status = 'idle'; render(); return; }
    try {
      if (!saved) {
        const open = await app.ensureDataset(dataset());
        if (token !== generation) return;
        if (open.status !== 'open') throw new Error(open.error ?? 'Open the dataset again from its files.');
        lastKey = key();
        const answer = await json('api/families', { ...request(), start });
        if (token !== generation) return;
        artifact = answer.artifact;
        if (!answer.ready) {
          status = answer.job ? 'running' : 'absent'; render();
          if (answer.job) {
            stopJob = app.jobs.onFinished((job) => {
              if (job.id !== answer.job || token !== generation) return;
              stopJob?.(); stopJob = null;
              if (job.state === 'complete') load().catch(() => {});
              else { status = 'failed'; error = job.error || `Job ${job.state}.`; render(); }
            });
            // The job can finish between its HTTP response and subscription.
            const completed = app.jobs.get(answer.job);
            if (completed && ['complete', 'failed', 'cancelled', 'invalid', 'orphaned'].includes(completed.state)) {
              stopJob?.(); stopJob = null;
              if (completed.state === 'complete') return load();
              throw new Error(completed.error || `Job ${completed.state}.`);
            }
          }
          return;
        }
      } else artifact = saved;
      const query = new URLSearchParams({ kind, offset, limit: 50, family });
      const answer = await json(`api/families/${encodeURIComponent(artifact)}/page?${query}`);
      if (token !== generation) return;
      page = answer; status = 'ready'; keep(answer); render(); return answer;
    } catch (e) { if (token === generation) { status = 'failed'; error = e.message; render(); } throw e; }
  }
  async function showPage(nextKind, nextFamily = '', nextOffset = 0) { kind = nextKind; family = nextFamily; offset = nextOffset; return load(); }
  function inspect(o) { detail = o; app.focusItem({ kind: 'bcell-family-member', id: o.id }); app.inspector.render(); }
  app.inspector.setSection('bcell-family-member', () => !detail || store.ui.focus?.kind !== 'bcell-family-member' || store.ui.focus.id !== detail.id ? null : h('section.inspector-section', h('h3', 'Family observation'), h('p', `${detail.sample} · ${detail.cell || 'bulk rearrangement'}`), h('p', detail.family || detail.excluded || 'Distances only'), h('p', `Subject: ${detail.subject || 'Missing'}. Abundance: ${detail.abundance ?? 'Missing'}.`), ...[detail.heavy, detail.light].filter((c) => c?.junction).map((c) => h('p', { style: { overflowWrap: 'anywhere' } }, `${c.locus} · ${c.v} · ${c.j} · ${c.junction}`)), ...detail.rows.map((r) => h('p.small', { style: { overflowWrap: 'anywhere' } }, `${r.sequenceId || 'No sequence ID'} · ${r.locus} · ${r.source.name}, line ${r.line} · SHA-256 ${r.source.sha256}`))));
  function render() {
    clear(root);
    const d = dataset();
    root.append(h('h2', 'Clonal families'), h('p.muted', 'Inspect nearest distances, then choose an explicit threshold. Families are separate from junction clonotypes and are not proven ancestry.'));
    const selector = h('select.input.small', { 'aria-label': 'Family dataset', onchange: () => { chosen = selector.value; saved = null; offset = 0; family = ''; load().catch(() => {}); } }, ...store.project.datasets.map((x) => h('option', { value: x.id, selected: x.id === chosen }, x.name)));
    root.append(h('label.kv-inline', 'Dataset ', selector));
    if (!d) { root.append(h('p', 'Open BCR data to compare its junctions.')); return; }
    const o = saved && page ? page.result.familiesOptions : familyOptions(d);
    const select = (label, name, options) => h('label.kv-inline', label, h('select.input.small', { 'aria-label': label, onchange: (e) => choose({ [name]: e.target.value }).catch(() => {}) }, ...options.map(([value, text]) => h('option', { value, selected: o[name] === value }, text))));
    const threshold = h('input.input.small', { type: 'number', min: 0, max: 1, step: 'any', value: o.threshold ?? '', placeholder: 'Distances only', 'aria-label': 'Family threshold' });
    root.append(h('div.options-row', select('Chains', 'mode', [['auto', 'Auto'], ['heavy', 'Heavy'], ['paired', 'Paired heavy/light']]), select('Boundaries', 'scope', [['subject', 'Within subject'], ['sample', 'Within sample']]), h('label.kv-inline', 'Threshold', threshold), button('Apply threshold', () => {
      if (!threshold.checkValidity()) throw new Error('Enter a threshold from 0 to 1.');
      return choose({ threshold: threshold.value.trim() === '' ? null : Number(threshold.value) });
    }), button('Distances only', () => choose({ threshold: null }))));
    root.append(h('p.muted.small', 'Auto uses paired chains when cell IDs are present. Paired distance is the larger heavy/light distance. Subject boundaries use Data annotations or supplied subject metadata; missing subjects are excluded. Within sample is an explicit alternative. The QC filter applies before pairing.'));
    if (saved) root.append(h('p', `Saved result ${saved}. Its recorded choices and filter are shown below; choosing options returns to a new analysis.`), button('Use current choices', () => { saved = null; return load(); }));
    if (status === 'loading' || status === 'running') { root.append(h('p', status === 'running' ? 'Comparing junctions… The job is available in Tasks.' : 'Reading the result…')); return; }
    if (status === 'failed') root.append(h('p.callout.danger', error));
    if (!page) { root.append(button(o.threshold === null ? 'Measure nearest distances' : 'Find clonal families', () => load(true))); return; }
    const r = page.result;
    const resultId = page.artifact.id;
    root.append(h('p', r.description), h('p', `${r.eligible} eligible of ${page.observationCount} observations · ${page.representativeCount} distinct receptors · ${page.familyCount} families · abundance unit: ${r.unit || 'unconfirmed'}.`), h('p.small', `QC: ${r.filterDescription || 'No filter'}. ${r.filteredRows} rows filtered; ${r.otherRows} rows outside the selected chains. ${r.withoutNeighbor} receptors have no comparable neighbor.`));
    for (const [reason, count] of Object.entries(r.excluded)) root.append(h('p.small', `${count} excluded: ${reason}.`));
    const plot = columns(r.histogram.map((b) => ({ x: b.lower.toFixed(2), y: b.count })), { label: 'Distance to nearest distinct receptor', xLabel: 'Normalized nucleotide Hamming distance (bin start)', yLabel: 'Distinct receptors', width: 640 });
    root.append(exportable(h('figure.plot-card', h('figcaption', 'Distance to nearest distinct receptor'), h('div.plot-holder', { html: plot.svg }), h('p.small.muted', 'Bins have width 0.05; the final bin includes 1. Missing neighbors are omitted. Abundance does not weight this distribution. No threshold is fitted automatically.')), () => ({ name: `${r.datasetName} nearest distances`, title: 'B-cell distance to nearest', plot: plot.svg, legend: [], caption: [r.description, `Result ${resultId}.`, madeBy(app.version)] })));
    root.append(h('div.btn-row', button('Families', () => showPage('families')), button('Nearest receptors', () => showPage('nearest')), button('All observations / exclusions', () => showPage('members')), button('Export result', async () => { const current = page.artifact.id; const opened = await app.ensureDataset(d); if (opened.status !== 'open') throw new Error(opened.error ?? 'Open the dataset again.'); return exportResult(app, d, current); })));
    if (kind === 'families') root.append(table(['Family', 'Scope', 'Receptors', 'Observations', 'Sample abundance'], page.rows.map((f) => h('tr', h('td', button(f.id, () => showPage('members', f.id))), td(f.scope), td(f.representatives), td(f.observations), td(f.samples.map((s) => `${s.sample}: ${s.abundance ?? 'Missing'}`).join('; '))))));
    if (kind === 'nearest') root.append(table(['Receptor', 'Scope', 'Heavy junction', 'Light junction', 'Nearest distance', 'Neighbor'], page.rows.map((r) => h('tr', td(r.id), td(r.scope), td(r.heavy.junction), td(r.light?.junction ?? '—'), td(r.nearest), td(r.neighbor || '—')))));
    if (kind === 'members') root.append(h('p', family ? `Members of ${family}` : 'All eligible and excluded observations'), table(['Observation', 'Sample', 'Subject', 'Cell', 'Abundance', 'Family / exclusion'], page.rows.map((o) => h('tr', h('td', button(o.id, () => inspect(o))), td(o.sample), td(o.subject || 'Missing'), td(o.cell || '—'), td(o.abundance), td(o.family || o.excluded || 'Distances only')))));
    root.append(h('div.btn-row', button('Previous', () => showPage(kind, family, Math.max(0, offset - 50)), offset === 0), h('span', page.count ? `${offset + 1}–${Math.min(offset + page.rows.length, page.count)} of ${page.count}` : 'No rows'), button('Next', () => showPage(kind, family, offset + 50), offset + page.rows.length >= page.count)), h('p.small.muted', `Kept ${page.artifact.created} as ${page.artifact.id}.`));
  }
  const view = { state: () => ({ status, error, page, artifact }), async show(id, changes = {}) { chosen = id; saved = null; offset = 0; family = ''; kind = 'families'; const next = useFamilies(store.project, id, changes); store.commit(next, 'Choose B-cell families', ['project']); return load(); }, run: () => load(true), showPage, inspect };
  app.familiesView = view;
  load().catch(() => {});
  return { update(topics) {
    const r = focused();
    if (topics.has('focus') && store.ui.focus?.kind === 'dataset' && store.ui.focus.id !== chosen) { chosen = store.ui.focus.id; saved = null; offset = 0; family = ''; kind = 'families'; load().catch(() => {}); }
    else if (topics.has('focus') && r && r.id !== saved) { chosen = r.dataset; saved = r.id; offset = 0; family = ''; kind = 'families'; load().catch(() => {}); }
    else if ((topics.has('project') || topics.has('project-loaded') || topics.has('datasets')) && !saved && key() !== lastKey) { offset = 0; load().catch(() => {}); }
  }, destroy() { generation++; stopJob?.(); if (app.familiesView === view) app.familiesView = null; app.inspector.setSection('bcell-family-member', null); root.remove(); } };
}
