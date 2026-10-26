import { h, clear, downloadBlob } from './dom.js';
import { confirmDialog } from './overlays.js';
import { fieldValue } from '../lib/datasets.js';
import { addResult } from '../lib/results.js';
import { figureButton, madeBy } from './figures.js';
import { hlaFrequencies, recurrencePlot } from '../lib/convergence-plot.js';

async function json(url, body) {
  const r = await fetch(url, body === undefined ? { cache: 'no-store' } : { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
  const v = await r.json(); if (!r.ok) throw new Error(v.error ?? `HTTP ${r.status}`); return v;
}
function parseTyping(text) {
  const out = {};
  for (const call of text.split(/[,;\n]/).map((s) => s.trim()).filter(Boolean)) {
    const match = /^(?:HLA-)?([A-Z0-9]+)\*/.exec(call);
    if (!match) throw new Error(`Use explicit HLA allele calls: ${call}`);
    (out[match[1]] ??= []).push(call);
  }
  return out;
}
export function mountConvergence(app, container) {
  const { store } = app, root = h('section.card.convergence-view'); container.append(root);
  let generation = store.state.generation, serial = 0, parent = '', preview = null, samples = [], subjects = [], groups = new Set(), alleles = '', human = false;
  let search = '', page = 0, answer = null, selected = 0, status = 'idle', error = '', job = '';
  const busy = () => ['loading', 'running'].includes(status);
  const num = (v) => v == null ? 'Unavailable' : Number(v).toPrecision(5);
  const prob = (v, log) => v === 0 ? `Underflow (ln ${num(log)})` : num(v);
  const button = (text, fn, disabled = false) => h('button.btn.small', { type: 'button', disabled, onclick: () => Promise.resolve().then(fn).catch((e) => { error = e.message; draw(); }) }, text);
  const table = (title, heads, rows) => h('div.table-wrap', { tabIndex: 0 }, h('table.data', h('caption', title), h('thead', h('tr', ...heads.map((s) => h('th', { scope: 'col' }, s)))), h('tbody', ...rows.map((r) => h('tr', ...r.map((v) => h('td', v)))))));
  function prepareSubjects() {
    const byID = new Map();
    for (const row of samples.filter((s) => s.include)) {
      const id = row.subject.trim(); if (!id) throw new Error(`Name the subject for ${row.name}.`);
      if (!byID.has(id)) byID.set(id, { id, samples: [], calls: new Set() });
      const v = byID.get(id); v.samples.push(row.name); if (row.hla.trim()) v.calls.add(row.hla.trim());
    }
    subjects = [...byID.values()].map((v) => ({ id: v.id, samples: v.samples, hla: v.calls.size === 1 ? [...v.calls][0] : '', conflict: v.calls.size > 1 }));
    draw();
  }
  async function loadParent(id) {
    const token = ++serial, same = store.sameProject(); parent = id; preview = null; samples = []; subjects = []; groups = new Set(); status = 'loading'; error = ''; draw();
    try {
      const v = await json(`api/motifs/${encodeURIComponent(id)}`); if (token !== serial || !same()) return;
      preview = v; const e = v.result.evidence, dataset = store.project.datasets.find((d) => d.id === e.dataset);
      let summary = null; try { const state = dataset ? await app.ensureDataset(dataset) : null; summary = state?.summary; } catch { /* Saved parent still supplies captured sample identities. */ }
      if (token !== serial || !same()) return;
      samples = [...e.query, ...e.background].map((name) => { const row = summary?.samples.find((s) => s.name === name) ?? { name }; return { name, include: true, subject: fieldValue(store.project, e.dataset, row, 'subject') || e.subjects?.[name] || '', hla: fieldValue(store.project, e.dataset, row, 'hla') || '' }; });
      status = 'idle'; if (samples.every((s) => s.subject)) prepareSubjects(); else draw();
    } catch (e) { if (token === serial && same()) { status = 'failed'; error = e.message; draw(); } throw e; }
  }
  function selection() {
    const current = samples.filter((s) => s.include);
    if (subjects.flatMap((s) => s.samples).sort().join('\n') !== current.map((s) => s.name).sort().join('\n') || current.some((s) => !subjects.some((sub) => sub.id === s.subject.trim() && sub.samples.includes(s.name)))) throw new Error('Prepare the subject table again after editing sample assignments.');
    if (subjects.some((s) => s.conflict)) throw new Error('Resolve conflicting typing annotations explicitly in the subject table.');
    return { species: human ? 'human' : '', samples: current.map((s) => s.name), subjects: subjects.map((s) => ({ id: s.id, samples: s.samples, typing: parseTyping(s.hla) })), groups: [...groups], alleles: alleles.split(/[,;\n]/).map((s) => s.trim()).filter(Boolean) };
  }
  function received(v) { answer = v; status = 'ready'; job = ''; error = ''; selected = 0; draw(); root.querySelector('.convergence-title')?.scrollIntoView({ block: 'start' }); }
  async function show(id) { const token = ++serial, same = store.sameProject(); status = 'loading'; error = ''; draw(); try { const v = await json(`api/convergence/${encodeURIComponent(id)}`); if (token === serial && same()) received(v); return v; } catch (e) { if (token === serial && same()) { status = 'failed'; error = e.message; draw(); } throw e; } }
  async function run(args = {}) {
    if (busy()) throw new Error('Wait for the current convergence analysis.');
    const request = args.motifs ? args : { motifs: parent, selection: selection() };
    const token = ++serial, same = store.sameProject(); status = 'loading'; error = ''; answer = null; draw();
    try {
      const start = await json('api/convergence', request); if (token !== serial || !same()) return;
      if (!start.ready) { status = 'running'; job = start.job; draw(); const done = await app.jobs.finished(job); if (token !== serial || !same()) return; if (done?.state !== 'complete') throw new Error(done?.error ?? `Analysis ${done?.state}`); }
      const v = await json(`api/convergence/${encodeURIComponent(start.artifact)}`); if (token !== serial || !same()) return;
      received(v);
      if (!store.project.artifacts.some((a) => a.id === v.artifact.id)) store.commit(addResult(store.project, { id: v.artifact.id, kind: 'convergence', dataset: v.artifact.dataset, name: v.artifact.name, made: v.artifact.created, method: 'Subject recurrence and conditional HLA association', describe: v.result.description, citation: v.artifact.lineage.method.citation }), 'Keep convergence and HLA analysis', ['project']);
      return v;
    } catch (e) { if (token === serial && same()) { status = 'failed'; error = e.message; job = ''; draw(); } throw e; }
  }
  async function download() {
    const v = answer, same = store.sameProject(); if (!v) return;
    const terms = (v.result.evidence.notice.Credits ?? []).filter((c) => c.LicenseClass !== 'open');
    if (terms.length && !await confirmDialog({ title: 'Provider terms apply', message: h('div', ...terms.map((c) => h('p', `${c.Title}: ${c.License}`))), confirm: 'Export' })) return;
    if (!same()) return; const r = await fetch(`api/convergence/${encodeURIComponent(v.artifact.id)}/download${terms.length ? '?accept=1' : ''}`);
    if (!r.ok) throw new Error((await r.json()).error ?? 'Export failed.'); const blob = await r.blob(); if (same()) downloadBlob(blob, `${v.artifact.id}.zip`);
  }
  function plotSpec(which = 'hla') {
    if (!answer) return null;
    const doc = answer.result, r = doc.analysis, row = r.associations[selected], g = r.recurrence.find((g) => g.id === row?.group);
    const p = which === 'recurrence' ? recurrencePlot(r) : row ? hlaFrequencies(row) : null; if (!p?.svg) return null;
    return { name: `${answer.artifact.id}-${which}-${selected}`, title: which === 'recurrence' ? 'Observed subject recurrence' : `${g.label} · ${row.allele}`, plot: p.svg, caption: [p.description, doc.description, `Declared groups: ${r.selection.groups.join(', ')}; declared two-field allele panel: ${r.selection.alleles.join(', ') || 'none (recurrence only)'}.`, `Result ${answer.artifact.id}; source motif method: ${doc.evidence.parentDescription}`, answer.artifact.lineage.method.citation, ...(doc.evidence.notice.Credits ?? []).map((c) => c.Citation).filter(Boolean), madeBy(app.version)] };
  }
  function draw() {
    clear(root); root.append(h('h2', 'Convergence and HLA association'), h('p', 'Measure observed recurrence of saved sequence groups across named subjects. Test a declared HLA allele panel against each complete typed-subject universe. Recurrence and association are hypotheses, not proof of convergent recombination, HLA restriction or shared antigen specificity.'));
    const parents = store.project.artifacts.filter((r) => r.kind === 'motifs');
    root.append(h('label.field', h('span', 'Saved motif screen'), h('select.input', { 'aria-label': 'Convergence motif parent', disabled: busy(), onchange: (e) => { if (e.target.value) loadParent(e.target.value).catch(() => {}); } }, h('option', { value: '' }, 'Choose a saved motif screen'), ...parents.map((p) => h('option', { value: p.id, selected: p.id === parent }, `${p.name} · ${p.id.slice(-6)}`)))));
    if (preview) {
      const settings = h('details', h('summary', 'Declare groups, samples, subjects and typing'));
      settings.append(h('p.small', 'Choose groups and alleles before inspecting HLA outcomes. Motif enrichment is a separate screen; it does not establish subject independence. All selected samples of one subject are unioned, including repeated observations. Select comparable sampling contexts.'), h('label.check', h('input', { type: 'checkbox', checked: human, disabled: busy(), onchange: (e) => { human = e.target.checked; } }), 'These are human subjects'), h('label.field', h('span', 'Allele panel (comma-separated; two-field comparisons)'), h('input.input', { value: alleles, placeholder: 'HLA-A*02:01, HLA-B*07:02', disabled: busy(), onchange: (e) => { alleles = e.target.value; } })), h('p.small', 'Leave the allele panel empty for descriptive recurrence only. Each tested locus requires two explicit allele copies; repeat a homozygous call. Missing, ambiguous, low-resolution or expression-suffix typing is unknown, never non-carriage. No typing imputation or HLA registry lookup is performed.'));
      settings.append(table('Selected sample universe', ['Include', 'Sample', 'Subject'], samples.map((s) => [h('input', { type: 'checkbox', checked: s.include, 'aria-label': `Include ${s.name}`, disabled: busy(), onchange: (e) => { s.include = e.target.checked; } }), s.name, h('input.input.small', { value: s.subject, 'aria-label': `Subject ${s.name}`, disabled: busy(), onchange: (e) => { s.subject = e.target.value; } })])), button('Prepare subject table from annotations', prepareSubjects, busy()));
      if (subjects.length) settings.append(table('Explicit subject typing', ['Subject', 'Samples', 'HLA calls (both copies per locus)'], subjects.map((s) => [s.id, s.samples.join(', '), h('div', h('textarea.input', { rows: 2, 'aria-label': `Typing ${s.id}`, disabled: busy(), onchange: (e) => { s.hla = e.target.value; s.conflict = false; } }, s.hla), s.conflict ? h('div',h('p.callout.danger', 'Annotations disagree. Enter the resolved calls, or explicitly leave unknown.'),button('Leave typing unknown',()=>{s.hla='';s.conflict=false;draw();})) : null)])));
      const rows = preview.result.analysis.groups.filter((g) => `${g.kind} ${g.motif} ${g.v} ${g.length} ${g.id}`.toLowerCase().includes(search.toLowerCase()));
      settings.append(h('label.field', h('span', 'Find saved signatures'), h('input.input', { value: search, 'aria-label': 'Find convergence signatures', onchange: (e) => { search = e.target.value; page = 0; draw(); } })), table(`Saved signatures; ${groups.size} selected; showing ${page*30+1}–${Math.min(rows.length,(page+1)*30)} of ${rows.length}`, ['Select', 'Signature', 'V / length', 'Start window', 'Motif-screen status'], rows.slice(page*30,(page+1)*30).map((g) => [h('input', { type: 'checkbox', checked: groups.has(g.id), 'aria-label': `Select ${g.id}`, disabled: busy(), onchange: (e) => { if (e.target.checked) groups.add(g.id); else groups.delete(g.id); } }), `${g.kind} ${g.motif} ${g.allowed}`, `${g.v} / ${g.length}`, `${g.start}–${g.end}`, g.reported ? 'Reported by motif screen' : 'Retained in full motif family'])), h('div.btn-row', button('Previous signatures', () => { page--; draw(); }, page === 0), button('Next signatures', () => { page++; draw(); }, (page+1)*30 >= rows.length)));
      // Keep settings open when the user is editing/filtering the declaration.
      settings.open = !answer; root.append(settings, button('Analyze recurrence and HLA', () => run(), busy()));
    }
    const saved = store.project.artifacts.filter((r) => r.kind === 'convergence'); if (saved.length) root.append(h('div.btn-row', ...saved.map((r) => button(`${r.name} · ${r.id.slice(-6)}`, () => show(r.id), busy()))));
    if (job) root.append(button('Cancel convergence analysis', () => app.jobs.cancel(job)));
    if (busy()) root.append(h('p', { role: 'status' }, status === 'running' ? 'Checking subject recurrence and HLA associations…' : 'Reading saved evidence…'));
    if (error) root.append(h('p.callout.danger', { role: 'alert' }, error)); if (!answer) return;
    const doc = answer.result, r = doc.analysis;
    root.append(h('h3.convergence-title', `Subject recurrence · ${doc.evidence.notice.Dataset}`), h('p', `${r.observedSubjects} subjects with receptor evidence of ${r.selection.subjects.length} selected · ${r.recurrence.length} declared groups · ${r.family} group × allele slots · ${r.tested} testable.`), h('p.small', 'The independent observation is one subject. HLA typing denominators vary by locus. BY covers every declared slot, including unavailable tests. Population structure, relatedness, linkage disequilibrium, unequal sampling and HLA-dependent ascertainment remain possible explanations.'), button('Export convergence ZIP', download), h('p.small', 'Exports include subject identifiers and raw HLA typing.'));
    const recurrence = plotSpec('recurrence'); if (recurrence) { const box = h('div.plot-wrap'); box.innerHTML = recurrence.plot; root.append(h('details', h('summary', 'Observed recurrence plot'), h('figure.plot-card', h('figcaption', recurrence.title, figureButton(() => plotSpec('recurrence'))), box))); }
    root.append(table('All selected sequence groups', ['Group', 'Signature', 'Observed subjects', 'Unique receptors'], r.recurrence.map((g, i) => [i+1, g.label, `${g.subjects.length} / ${g.observedSubjects}`, g.receptors.length])));
    if (r.associations.length) {
      root.append(h('label.field', h('span', 'Inspect association'), h('select.input', { 'aria-label': 'Displayed HLA association', onchange: (e) => { selected = Number(e.target.value); draw(); } }, ...r.associations.map((x, i) => h('option', { value: i, selected: i === selected }, `Group ${r.selection.groups.indexOf(x.group)+1} · ${x.allele}`)))));
      const x = r.associations[selected], spec = plotSpec(), box = h('div.plot-wrap'); box.innerHTML = spec.plot;
      root.append(h('figure.plot-card', h('figcaption', spec.title, figureButton(plotSpec)), box), h('p.small', 'Marginal 95% Wilson intervals describe each subject fraction. They are not simultaneous or adjusted by BY. The frequency difference is descriptive; no interval for that difference is implied.'), table('Selected conditional HLA association', ['Typed subjects', 'Group subjects', 'Carriers', 'Shared', 'Excluded', 'Difference', 'One-sided p', 'BY', 'Status'], [[x.n, x.groupSubjects, x.carriers, x.shared, x.excluded, num(x.difference), prob(x.p,x.logP), prob(x.q,x.logQ), x.reason || 'Conditional subject association']]), h('details', h('summary', 'Every declared association'), table('Complete correction family', ['Group', 'Allele', 'Typed', 'Shared', 'p', 'BY', 'Reason'], r.associations.map((a) => [a.group,a.allele,a.n,a.shared,prob(a.p,a.logP),prob(a.q,a.logQ),a.reason || 'Available']))));
    }
    root.append(h('details', h('summary', 'Subject membership, typing and exclusions'), table('Every selected subject', ['Subject', 'Samples', 'Receptors', 'Groups', 'Typing and missingness', 'Receptor evidence'], r.observations.map((s) => [s.subject,s.samples.join(', '),s.receptors,s.groups.join(', ') || 'None observed',s.typing.map((t) => `${t.locus}: ${t.raw.join(', ') || 'Missing'} → ${t.reason || t.alleles.join(', ')}`).join('; '),s.reason || 'Observed']))), h('details', h('summary', 'Exact recurrent receptors'), ...r.recurrence.map((g) => table(g.label, ['V gene', 'Full AA junction', 'Subjects'], g.receptors.map((rec) => [rec.v,rec.junction,rec.subjects.join(', ')])))), h('details', h('summary', 'Scientific methods and source provenance'), h('p',doc.description),h('p',doc.evidence.parentDescription),h('p',answer.artifact.lineage.method.citation)));
  }
  const focused = () => store.ui.focus?.kind === 'result' && store.project.artifacts.find((r) => r.id === store.ui.focus.id && r.kind === 'convergence');
  const view = { run, show, loadParent, download, plotSpec, select: (i) => { selected = i; draw(); }, state: () => ({ status, answer, error }), configure: (s) => { samples = s.samples.map((name) => ({ name, include: true, subject: s.subjects.find((sub) => sub.samples.includes(name))?.id ?? '', hla: '' })); subjects = s.subjects.map((sub) => ({ ...sub, hla: Object.values(sub.typing).flat().join(', '), conflict: false })); groups = new Set(s.groups); alleles = s.alleles.join(', '); human = s.species === 'human'; draw(); } }; app.convergenceView = view;
  draw(); if (focused()) show(focused().id).catch(() => {});
  return { update(topics) { if (generation !== store.state.generation) { generation = store.state.generation; serial++; parent = ''; preview = null; samples = []; subjects = []; answer = null; status = 'idle'; error = ''; } const f = focused(); if (topics.has('focus') && f && answer?.artifact.id !== f.id) show(f.id).catch(() => {}); else if (topics.has('project')) draw(); }, destroy() { serial++; if (app.convergenceView === view) delete app.convergenceView; root.remove(); } };
}
