// The Cohort design view. Named, versioned snapshots follow SituCraft's cohort-design
// workflow; all scientific checks and counts are computed by ClonoLoom's Go host.
import { h, icon, clear, downloadBlob } from './dom.js';
import { toast } from './overlays.js';
import { GROUP_FIELDS, cohortSamples, cohortSpec, cohortDesigns, saveCohortDesign, cohortDesignChanged } from '../lib/cohort.js';

export function mountCohort(app, container) {
  const { store } = app;
  let chosen = store.ui.focus?.kind === 'dataset' ? store.ui.focus.id : store.project.datasets[0]?.id;
  let samples = [], spec = cohortSpec(), result = null, error = '', busy = false, search = '', name = 'Design';
  let generation = 0, sourceProject = null, loaded = false;
  const body = h('div.view-body.cohort-view');
  const select = h('select.input.small', { 'aria-label': 'Dataset', onchange: (e) => choose(e.target.value) });
  container.append(h('div.view', h('div.workbench-head', h('h1', icon('samples'), 'Cohort'), h('span.spacer'), h('label.kv-inline', h('span.muted', 'Dataset'), select),
    h('button.btn.small', { type: 'button', onclick: () => app.setMode('data') }, 'Annotate samples')), h('div.workbench-scroll', body)));
  const dataset = () => store.project.datasets.find((d) => d.id === chosen);
  const reportError = (e) => { error = e.message; render(); };
  const change = (patch) => { if (busy) return; spec = { ...spec, ...patch }; result = null; error = ''; render(); };
  const selection = (label, value, options, onchange) => h('label.field', h('span', label), h('select.input.small', { 'aria-label': label, disabled: busy, onchange: (e) => onchange(e.target.value) },
    h('option', { value: '', selected: !value }, 'Choose…'), ...options.map(([v, text]) => h('option', { value: v, selected: value === v }, text))));

  async function choose(id) {
    chosen = id;
    spec = cohortSpec();
    result = null;
    return load();
  }
  async function load() {
    const mine = ++generation;
    sourceProject = store.project;
    const d = dataset();
    samples = []; loaded = false; result = null; error = ''; busy = true; render();
    if (!d) { busy = false; render(); return; }
    try {
      const opened = await app.ensureDataset(d);
      if (mine !== generation) return;
      if (store.project !== sourceProject) return load();
      if (opened.status !== 'open') throw new Error(opened.error ?? 'The dataset could not be opened.');
      samples = cohortSamples(store.project, d.id, opened.summary);
      loaded = true;
    } catch (e) { if (mine === generation) error = e.message; }
    if (mine === generation) { busy = false; render(); }
  }
  async function check({ save = false } = {}) {
    const project = store.project, d = dataset();
    if (!d || busy || !samples.length) throw new Error('Open a dataset and wait for its annotations.');
    const mine = ++generation;
    const input = { dataset: d.id, samples, spec };
    busy = true; result = null; error = ''; render();
    try {
      const response = await fetch('api/cohort/design', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(input) });
      const answer = await response.json();
      if (mine !== generation || store.project !== project) throw new Error('The project or annotations changed. Check the design again.');
      if (!response.ok) throw new Error(answer.error ?? 'The design could not be checked.');
      result = answer;
      if (save) {
        const next = saveCohortDesign(project, answer, name);
        sourceProject = next;
        store.commit(next, `Save cohort design ${name}`);
      }
      return answer;
    } catch (e) { if (mine === generation) error = e.message; throw e; }
    finally { if (mine === generation) { busy = false; render(); } }
  }

  function render() {
    clear(select);
    for (const d of store.project.datasets) select.append(h('option', { value: d.id, selected: d.id === chosen }, d.name));
    clear(body);
    if (!dataset()) { body.append(h('div.empty', icon('samples'), h('h2', 'Describe the study before comparing groups'), h('p', 'Open a dataset, then annotate its subjects and groups in Data.'))); return; }
    const levels = [...new Set(samples.map((s) => s[spec.groupField]).filter(Boolean))].sort();
    const saved = cohortDesigns(store.project, chosen);
    const card = h('section.card.cohort-design', h('h2', 'Study design'),
      h('p', 'Choose two groups and their pairing. The independent unit is the subject; cells, sequences and repeated samples do not increase the number of subjects.'),
      h('div.cohort-controls',
        selection('Layout', spec.layout, [['independent', 'Independent subjects'], ['paired', 'Paired within subjects']], (v) => change({ layout: v })),
        selection('Group field', spec.groupField, Object.entries(GROUP_FIELDS), (v) => change({ groupField: v, levels: ['', ''] })),
        selection('Reference group', spec.levels[0], levels.map((v) => [v, v]), (v) => change({ levels: [v, spec.levels[1]] })),
        selection('Comparison group', spec.levels[1], levels.map((v) => [v, v]), (v) => change({ levels: [spec.levels[0], v] }))),
      h('p.muted.small', 'Labels match exactly. No automatic pooling, covariate adjustment or deletion of incomplete pairs. This view checks and saves the design; it performs no statistical test.'),
      h('div.btn-row', h('button.btn.primary.small', { type: 'button', disabled: busy, onclick: () => check().catch(reportError) }, busy ? 'Reading…' : 'Check design'),
        h('input.input.small', { value: name, 'aria-label': 'Design name', onchange: (e) => { name = e.target.value.trim() || 'Design'; } }),
        h('button.btn.small', { type: 'button', disabled: busy || !result?.ok, onclick: () => check({ save: true }).catch(reportError) }, 'Save design')));
    body.append(card);
    if (error) card.append(h('p.callout.danger', { role: 'alert' }, error));
    if (result) {
      card.append(h('p', h(`span.badge.${result.ok ? 'ok' : 'warn'}`, result.ok ? 'Structural checks passed' : 'Design needs attention')),
        ...result.words.map((w) => h('p', w)),
        h('div.table-wrap', { tabIndex: 0 }, h('table.data', h('caption', 'Selected groups: samples and distinct subjects'),
          h('thead', h('tr', h('th', 'Group'), h('th', 'Samples'), h('th', 'Subjects'))),
          h('tbody', ...result.groups.map((g) => h('tr', h('td', g.level), h('td', g.samples), h('td', g.subjects)))))),
        ...result.findings.map((f) => h(`div.callout.${f.severity === 'error' ? 'danger' : 'warn'}`, h('b', f.title), h('p', f.detail), h('p.small', `${f.code}: ${(f.examples ?? []).join('; ')}`))),
        h('details', h('summary', 'Scientific sources'), h('ul', ...result.citations.map((c) => h('li', c)))));
    }
    const tableBox = h('section.card', h('h2', 'Samples and exclusions'), h('p.muted.small', 'Enter a reason to exclude a sample; clear it to include the sample again. Every excluded sample remains in the saved design.'));
    const rowsBox = h('div');
    const drawRows = () => {
      clear(rowsBox);
      const matches = samples.filter((s) => `${s.name} ${s.subject} ${s[spec.groupField]}`.toLowerCase().includes(search.toLowerCase()));
      const rows = matches.slice(0, 100), disposition = new Map((result?.rows ?? []).map((r) => [r.name, r]));
      rowsBox.append(h('p.muted.small', `Showing ${rows.length} of ${matches.length} matching sample annotations. Search to find others.`),
        h('div.table-wrap', { tabIndex: 0 }, h('table.data', h('caption', 'Sample annotations and explicit exclusions'),
          h('thead', h('tr', ...['Sample', 'Subject', 'Group', 'Time point', 'Tissue', 'Cell subset', 'Batch', 'Exclusion reason', 'Checked status'].map((v) => h('th', v)))),
          h('tbody', ...rows.map((s) => h('tr', ...[s.name, s.subject, s[spec.groupField], s.timepoint, s.tissue, s.cellSubset, s.batch].map((v) => h('td', v || 'Missing')),
            h('td', h('input.input.small', { value: Object.hasOwn(spec.exclude, s.name) ? spec.exclude[s.name] : '', 'aria-label': `Exclusion reason for ${s.name}`, disabled: busy, onchange: (e) => {
              const exclude = { ...spec.exclude, [s.name]: e.target.value.trim() }; if (!e.target.value.trim()) delete exclude[s.name]; change({ exclude });
            } })), h('td', disposition.get(s.name)?.included ? 'Included' : disposition.get(s.name)?.reason ?? 'Not checked')))))));
    };
    tableBox.append(h('input.input.small', { type: 'search', value: search, placeholder: 'Find a sample or subject', 'aria-label': 'Find a sample or subject', oninput: (e) => { search = e.target.value; drawRows(); } }), rowsBox);
    drawRows(); body.append(tableBox);
    if (saved.length) body.append(h('section.card', h('h2', 'Saved designs'), ...saved.map((record) => h('details.cohort-saved', { 'data-design': record.id },
      h('summary', `${record.name}, version ${record.revision} · ${!loaded ? 'current annotations unavailable' : cohortDesignChanged(record, samples) ? 'annotations changed' : 'annotations as saved'}`),
      ...record.result.words.map((w) => h('p', w)),
      h('div.table-wrap', { tabIndex: 0 }, h('table.data', h('caption', 'Saved sample membership (first 100; complete record in JSON)'),
        h('thead', h('tr', ...['Sample', 'Subject', 'Group', 'Disposition'].map((v) => h('th', v)))),
        h('tbody', ...record.result.rows.slice(0, 100).map((r) => h('tr', h('td', r.name), h('td', r.subject || 'Missing'), h('td', r.group || 'Missing'), h('td', r.included ? 'Included' : r.reason)))))),
      h('div.btn-row', h('button.btn.small', { type: 'button', disabled: busy, onclick: () => { spec = cohortSpec(record.result.input.spec); name = record.name; result = null; render(); } }, 'Use settings'),
        h('button.btn.small', { type: 'button', onclick: () => downloadBlob(new Blob([JSON.stringify(record, null, 2)], { type: 'application/json' }), `${record.id}.json`) }, 'Export saved JSON')),
      h('p.small', record.result.citations.join('; '))))));
  }

  const view = {
    choose,
    async configure(options = {}) { if (busy) throw new Error('Wait for the current design check.'); spec = cohortSpec(options.spec ?? spec); name = String(options.name ?? name); result = null; render(); return check({ save: Boolean(options.save) }); },
    state: () => ({ samples, spec, result, error, busy }),
  };
  app.cohortView = view;
  load().catch((e) => toast(e.message, { kind: 'error' }));
  return {
    update(topics) { if (topics.has('project') && sourceProject !== store.project) load().catch(reportError); },
    destroy() { generation++; if (app.cohortView === view) delete app.cohortView; },
  };
}
