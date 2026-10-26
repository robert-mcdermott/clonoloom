// Definition audits retain immutable Go evidence; display choices never refit the family.
import { h, clear, downloadBlob } from './dom.js';
import { confirmDialog } from './overlays.js';
import { addResult } from '../lib/results.js';
import { sameMeaning } from '../lib/definitions.js';
import { robustnessEffects } from '../lib/robustness-plot.js';
import { cohortPoints } from '../lib/cohort-plot.js';
import { figureButton, madeBy } from './figures.js';
async function json(url, body) {
  const response = await fetch(url, body === undefined ? { cache: 'no-store' } : { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
  const value = await response.json(); if (!response.ok) throw new Error(value.error ?? `HTTP ${response.status}`); return value;
}
export function mountRobustness(app, container) {
  const { store } = app, root = h('section.card.definition-robustness'); container.append(root);
  let generation = store.state.generation, serial = 0, parent = '', preview = null, alternatives = [], chosen = new Set(), answer = null, loaded = '', status = 'idle', error = '', job = '', metric = '', variant = 0;
  const busy = () => ['loading', 'running'].includes(status);
  const number = (v) => v == null ? 'Unavailable' : Number(v).toPrecision(5);
  const button = (label, fn, disabled = false) => h('button.btn.small', { type: 'button', disabled, onclick: () => Promise.resolve().then(fn).catch((e) => { error = e.message; draw(); }) }, label);
  const table = (caption, heads, rows) => h('div.table-wrap', { tabIndex: 0 }, h('table.data', h('caption', caption), h('thead', h('tr', ...heads.map((v) => h('th', { scope: 'col' }, v)))), h('tbody', ...rows.map((r) => h('tr', ...r.map((v) => h('td', v)))))));
  const label = (v, i) => `${i === 0 ? 'Baseline: ' : ''}${v.definition.name || `${v.definition.junction.toUpperCase()} · V ${v.definition.v} · J ${v.definition.j}`}`;
  async function choose(id) {
    const token = ++serial, same = store.sameProject(); parent = id; preview = null; alternatives = []; chosen = new Set(); status = id ? 'loading' : 'idle'; error = ''; draw(); if (!id) return;
    try {
      const v = await json(`api/cohort-compare/${encodeURIComponent(id)}`); if (token !== serial || !same()) return;
      preview = v.result; const base = preview.features.parent.definition;
      alternatives = [['nt','gene','NT · V/J genes'],['aa','allele','AA · V/J alleles'],['aa','none','AA · junction only'],['aa','gene','AA · V/J genes'],['nt','allele','NT · V/J alleles'],['aa','family','AA · V/J families']].map(([junction,resolution,name]) => ({ ...base, name, junction, v: resolution, j: resolution })).filter((d) => !sameMeaning(d, base) && (d.junction !== 'aa' || base.productiveOnly));
      chosen = new Set(alternatives.slice(0, 3).map((_, i) => i)); status = 'idle'; draw();
    } catch (e) { if (token === serial && same()) { status = 'failed'; error = e.message; draw(); } throw e; }
  }
  function received(v) { answer = v; loaded = v.artifact.id; status = 'ready'; job = ''; variant = 0; metric = v.result.analysis.summaries[0]?.feature.id ?? ''; draw(); root.querySelector('.robustness-title')?.scrollIntoView({ block: 'start' }); }
  async function show(id) { const token = ++serial, same = store.sameProject(); answer = null; loaded = ''; status = 'loading'; error = ''; draw(); try { const v = await json(`api/definition-robustness/${encodeURIComponent(id)}`); if (token !== serial || !same()) return; received(v); return v; } catch (e) { if (token === serial && same()) { status = 'failed'; error = e.message; draw(); } throw e; } }
  async function run(options = {}) {
    if (busy()) throw new Error('Wait for the current definition audit.');
    const source = options.comparison ?? parent, record = store.project.artifacts.find((r) => r.id === source && r.kind === 'cohort-compare');
    if (!record) throw new Error('Choose a saved subject group comparison.');
    const definitions = options.definitions ?? alternatives.filter((_, i) => chosen.has(i));
    const token = ++serial, same = store.sameProject(); status = 'loading'; error = ''; answer = null; loaded = ''; job = ''; draw();
    try {
      const dataset = store.project.datasets.find((d) => d.id === record.dataset); if (!dataset) throw new Error('The original dataset is not in this project.');
      const opened = await app.ensureDataset(dataset); if (token !== serial || !same()) return; if (opened.status !== 'open') throw new Error(opened.error ?? 'Open the original receptor dataset.');
      const start = await json('api/definition-robustness', { comparison: source, definitions }); if (token !== serial || !same()) return;
      if (!start.ready) { status = 'running'; job = start.job; draw(); const done = await app.jobs.finished(job); if (token !== serial || !same()) return; if (done?.state !== 'complete') throw new Error(done?.error || `Definition audit ${done?.state ?? 'unavailable'}.`); }
      const v = await json(`api/definition-robustness/${encodeURIComponent(start.artifact)}`); if (token !== serial || !same()) return; received(v);
      if (!store.project.artifacts.some((r) => r.id === loaded)) store.commit(addResult(store.project, { id: loaded, kind: 'definition-robustness', dataset: v.artifact.dataset, name: v.artifact.name, made: v.artifact.created, method: 'Descriptive definition sensitivity', describe: v.result.description, citation: v.artifact.lineage.method.citation }), 'Keep definition audit', ['project']); draw(); return v;
    } catch (e) { if (token === serial && same()) { status = 'failed'; error = e.message; job = ''; draw(); } throw e; }
  }
  async function download() {
    const v = answer, same = store.sameProject(); if (!v) return;
    const terms = (v.result.parent.features.notice.Credits ?? []).filter((c) => c.LicenseClass !== 'open');
    if (terms.length && !await confirmDialog({ title: 'Provider terms apply', message: h('div', h('p', 'The audit retains its source terms.'), h('ul', ...terms.map((c) => h('li', `${c.Title} — ${c.License}`)))), confirm: 'Export' })) return;
    if (!same()) return; const response = await fetch(`api/definition-robustness/${encodeURIComponent(v.artifact.id)}/download${terms.length ? '?accept=1' : ''}`); if (!response.ok) throw new Error((await response.json()).error ?? 'Export failed.'); const blob = await response.blob(); if (same()) downloadBlob(blob, `${v.artifact.id}.zip`);
  }
  function plotSpec(which = 'effects') {
    if (!answer) return null; const doc = answer.result, r = doc.analysis, f = doc.parent.features, j = r.summaries.findIndex((s) => s.feature.id === metric), v = r.variants[variant], c = v?.analysis?.comparisons[j];
    const p = which === 'effects' ? robustnessEffects(r, metric) : c ? cohortPoints(c, v.analysis) : { svg: '', description: v.failure || 'Comparison unavailable.' };
    return { name: `${loaded}-${metric}-${which}-${variant}`, title: which === 'effects' ? `${r.summaries[j].feature.name}: definition sensitivity` : `${label(v, variant)}: subject observations`, plot: p.svg,
      caption: [p.description, `Contrast: ${doc.parent.analysis.levels[1]} minus ${doc.parent.analysis.levels[0]}; ${f.design.input.spec.layout}, ${f.design.subjects} subjects, ${f.design.included} observations.`, `Definitions in declared order: ${r.variants.map((v, i) => `${i+1}. ${label(v,i)}; ${v.definition.junction} junction, V ${v.definition.v}, J ${v.definition.j}; ${v.definition.productiveOnly ? 'productive only' : 'all productivity states'}; chains ${v.definition.chains?.join('+') || 'each locus separately'}; cells ${v.definition.cells}; require all ${Boolean(v.definition.requireAll)}`).join(' | ')}.`, `QC ${f.notice.Filter}; source abundance unit ${f.parent.unit}; ${r.family} across-audit BY slots. Definitions reuse subjects; marginal intervals are not simultaneous or adjusted by BY. Estimate ranges are descriptive, not confidence intervals.`, `Dataset ${f.parent.datasetName}; design ${f.selection.name}, version ${f.selection.revision}; comparison parent ${doc.parentManifest.id}; result ${loaded}.`, ...f.design.findings.map((finding) => `${finding.title}: ${finding.detail}`), madeBy(app.version)] };
  }
  function draw() {
    clear(root); const parents = store.project.artifacts.filter((r) => r.kind === 'cohort-compare');
    root.append(h('h2', 'Definition robustness'), h('p', 'Recompute a saved subject-feature comparison under declared alternative junction and V/J definitions. Subjects, QC, features, tests and seeds remain fixed. Select defensible alternatives before inspecting outcomes.'),
      h('label.field', h('span', 'Comparison to audit'), h('select.input.small', { 'aria-label': 'Comparison to audit', disabled: busy(), onchange: (e) => choose(e.target.value).catch(() => {}) }, h('option', { value: '', selected: !parent }, 'Choose saved group comparison'), ...parents.map((r) => h('option', { value: r.id, selected: r.id === parent }, `${r.name} · ${r.id.slice(-6)}`)))));
    if (preview) root.append(h('p.small', `Baseline: ${preview.features.parent.describe}. ${preview.features.design.subjects} subjects; features: ${preview.analysis.options.features.join(', ')}.`), h('fieldset', h('legend', 'Alternative definitions (one to five)'), ...alternatives.map((d, i) => h('label.kv-inline', h('input', { type: 'checkbox', checked: chosen.has(i), disabled: busy(), onchange: (e) => { if (e.target.checked) chosen.add(i); else chosen.delete(i); draw(); } }), d.name))));
    root.append(button('Audit definitions', () => run(), busy() || !preview || chosen.size < 1 || chosen.size > 5));
    const saved = store.project.artifacts.filter((r) => r.kind === 'definition-robustness'); if (saved.length) root.append(h('div.btn-row', h('span', 'Saved definition audits'), ...saved.map((r) => button(`${r.name} · ${r.id.slice(-6)}`, () => show(r.id), busy()))));
    if (job) root.append(button('Cancel definition audit', () => app.jobs.cancel(job))); if (error) root.append(h('p.callout.danger', { role: 'alert' }, error)); if (busy()) root.append(h('p', { role: 'status' }, status === 'running' ? 'Recomputing the declared definitions…' : 'Reading saved comparison and source evidence…')); if (!answer) return;
    const doc = answer.result, r = doc.analysis, f = doc.parent.features, j = r.summaries.findIndex((s) => s.feature.id === metric), s = r.summaries[j], v = r.variants[variant];
    root.append(h('h3.robustness-title', `${f.selection.name}, version ${f.selection.revision}: definition audit`), h('p', `${f.design.subjects} subjects · ${f.design.included} observations · ${f.design.input.spec.layout} · ${doc.parent.analysis.levels[1]} minus ${doc.parent.analysis.levels[0]}. ${r.variants.length} declared definitions; ${r.family} across-audit BY slots.`), h('p.small', `QC ${f.notice.Filter}; abundance ${f.parent.unit}. Definitions reuse the same subjects and are not independent replications.`),
      h('label.field', h('span', 'Feature to inspect'), h('select.input.small', { 'aria-label': 'Robustness feature', onchange: (e) => { metric = e.target.value; draw(); } }, ...r.summaries.map((x) => h('option', { value: x.feature.id, selected: metric === x.feature.id }, x.feature.name)))),
      h('div.callout', h('b', s.direction), h('p', `${s.uncertainty}. ${s.available}/${s.definitions} estimates and ${s.intervals}/${s.definitions} intervals available. Estimate range ${number(s.minimum)} to ${number(s.maximum)} ${s.feature.unit}; span ${number(s.span)}. This descriptive range is not a confidence interval or equivalence test.`)));
    const spec = plotSpec(), box = h('div.plot-wrap'); box.innerHTML = spec.plot; root.append(h('figure.plot-card', h('figcaption', h('span', spec.title), figureButton(() => plotSpec())), box));
    root.append(table('Effects and tests in declared definition order', ['Definition', 'Mean difference', 'Lower', 'Upper', 'Raw p', 'Within BH', 'Across BY', 'Status'], r.variants.map((v, i) => { const c = v.analysis?.comparisons[j]; return [button(label(v, i), () => { variant = i; draw(); }), number(c?.mean.estimate), number(c?.mean.lower), number(c?.mean.upper), number(c?.parametric.p), number(c?.parametric.adjusted), number(v.across[j].mean), v.failure || c?.mean.reason || 'Available']; })),
      h('p.small', `${100*r.options.confidence}% marginal t intervals, not simultaneous or BY-adjusted. The descriptive conclusion does not use p-value thresholds. Changed definitions can change assigned observations and the meaning of diversity. No pooled effect or joint specification-curve test is calculated.`));
    const c = v.analysis?.comparisons[j];
    if (c) {
      root.append(h('p.small', `${v.analysis.layout === 'paired' ? 'Within-subject probability of an increase' : 'Probability that a comparison-group subject exceeds a reference-group subject'}, with half credit for ties. ${100*r.options.confidence}% conservative bounded-difference intervals; these are not rank-test inversions or median-shift intervals.`));
      root.append(table(`${label(v, variant)}: probability effect and rank test`, ['Probability', 'Lower', 'Upper', 'Rank p', 'Within BH', 'Across BY', 'Status'], [[number(c.probability.estimate), number(c.probability.lower), number(c.probability.upper), number(c.rank.p), number(c.rank.adjusted), number(v.across[j].rank), c.rank.reason || c.probability.interval]]));
      const observations = plotSpec('observations'), ob = h('div.plot-wrap'); ob.innerHTML = observations.plot; root.append(h('details', h('summary', 'Inspect subject observations'), h('figure.plot-card', h('figcaption', observations.title, figureButton(() => plotSpec('observations'))), ob)));
    }
    root.append(h('details', h('summary', 'Definitions, assigned evidence and exclusions'), ...r.variants.map((v, i) => h('div', h('h4', label(v, i)), h('p.small', JSON.stringify(v.definition)), h('p.small', v.failure || `Excluded source rows/cells: ${JSON.stringify(v.excluded)}`))),
      table(`${label(v, variant)}: evidence compared with baseline`, ['Sample', 'Subject', 'Group', 'Baseline assigned', 'This definition assigned', 'Baseline richness', 'This definition richness'], f.table.rows.map((row, i) => { const current = v.table?.rows[i]; return [row.name, row.subject, row.group, number(r.variants[0].table.rows[i].values[0].value), number(current?.values[0].value), number(r.variants[0].table.rows[i].values[1].value), number(current?.values[1].value)]; }))),
      button('Export definition audit ZIP', download), h('details', h('summary', 'Definition-audit methods and provenance'), h('p', doc.description), h('p', answer.artifact.lineage.method.citation)));
  }
  const view = { run, choose, show, download, plotSpec, select: (id) => { metric = id; draw(); }, state: () => ({ status, answer, artifact: loaded, error }) }; app.robustnessView = view;
  const focused = () => store.ui.focus?.kind === 'result' && store.project.artifacts.find((r) => r.id === store.ui.focus.id && r.kind === 'definition-robustness');
  draw(); if (focused()) show(focused().id).catch(() => {});
  return { update(topics) { if (generation !== store.state.generation) { generation = store.state.generation; serial++; parent = ''; preview = null; answer = null; loaded = ''; status = 'idle'; error = ''; job = ''; } const target = focused(); if (topics.has('focus') && target && target.id !== loaded) show(target.id).catch(() => {}); else if (topics.has('project') || topics.has('jobs')) draw(); }, destroy() { serial++; if (app.robustnessView === view) delete app.robustnessView; root.remove(); } };
}
