// The Tracking view (clonoloom-spec/requirements.md, T1–T6; roadmap.md, wave 3). Subjects (slice 3:
// D10, G3): each subject's samples in time order, from the samples' annotation (their own fields,
// else their metadata's), read and checked by the program (api_study.go, internal/study): the time
// points as days from each subject's reference event, the reference samples (an infusion product,
// a donor), the samples taken at one time (replicates), and what is left out of tracking and why.
// Clonotypes (slice 5: T1, T6): the clonotypes chosen (a reference sample's largest, each sample's
// largest, or a list the user keeps) followed through each subject's time points by the program as
// a job, kept as a result (api_tracking.go, internal/track): threads of their frequencies with
// their intervals, an alluvial plot, and a table, linked to each other and to the Clonotypes view.
// Tests (slices 6 and 7: T2, T3; tracking-tests.js): each clonotype tested between two time points
// of each subject (api_expansion.go), the calls followed in the Clonotypes part on request. Classes
// (slice 8: T4; tracking-classes.js): each clonotype classed by its calls between a base time point
// and every later one. Attribution (slice 8: T5; tracking-attribution.js): each sample against its
// subject's reference sample. A dataset opened from an example with guided steps shows them, each
// performed by the action scripts use (remote.js).

import { h, icon, clear } from './dom.js';
import { showDialog, toast } from './overlays.js';
import { alluvial, percent, threads, timeline } from '../lib/plot.js';
import { UNITS, datasetUnit, fieldValue, studySamples } from '../lib/datasets.js';
import { datasetDefinition } from '../lib/definitions.js';
import { datasetFilter } from '../lib/filters.js';
import { addResult } from '../lib/results.js';
import { BY, CLASSES, CORRECTIONS, MAX_LIST, TESTS, choiceShort, choiceWords, classRequest, compareRequest, comparisonChoice, comparisonWords, parseList, timePoints, trackRequest, trackingChoice, useComparison, useTracking } from '../lib/tracking.js';
import { exportable, madeBy } from './figures.js';
import { exportResult } from './exports.js';
import { renderTests } from './tracking-tests.js';
import { renderClasses } from './tracking-classes.js';
import { renderAttribution } from './tracking-attribution.js';

const SEVERITY = {
  warning: { label: 'Warning', badge: 'warn', icon: 'warning' },
  note: { label: 'Note', badge: 'accent', icon: 'info' },
};

const PARTS = { subjects: 'Subjects', clonotypes: 'Clonotypes', tests: 'Tests', classes: 'Classes', attribution: 'Attribution' };
// The analysis each part shows, the part each opens in, and what the head's button says.
const KIND_OF = { clonotypes: 'tracking', tests: 'expansion', classes: 'classes', attribution: 'attribution' };
const PART_OF = { tracking: 'clonotypes', expansion: 'tests', classes: 'classes', attribution: 'attribution' };
const KINDS = ['tracking', 'expansion', 'classes', 'attribution'];
const START = { tracking: 'Follow clonotypes', expansion: 'Test the clonotypes', classes: 'Class the clonotypes', attribution: 'Attribute the samples' };
const SIZES = [5, 10, 20, 50];

const count = (n) => Number(n ?? 0).toLocaleString('en-US');
const plural = (n, word, words = `${word}s`) => `${count(n)} ${n === 1 ? word : words}`;

async function getJSON(url, options = {}) {
  const response = await fetch(url, { cache: 'no-store', ...options });
  const answer = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(answer.error ?? `${url}: HTTP ${response.status}`);
  return answer;
}
const postJSON = (url, body) => getJSON(url, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: typeof body === 'string' ? body : JSON.stringify(body) });

// The words a time point is shown in, with its event where the annotation names one.
const when = (t) => (t ? `${t.label}${t.event ? ` (${t.event})` : ''}` : 'no time point');

// A clonotype followed, in words: its junctions; and its genes.
const junctionsOf = (c) => c.chains.map((ch) => ch.junction).join('; ') || c.id;
const genesOf = (c) => c.chains.map((ch) => [ch.v, ch.j].filter(Boolean).join(' ')).filter(Boolean).join('; ');

// Why a clonotype is followed, in words.
function whyWords(why) {
  const parts = why.map((r) => {
    if (r.listed) return `listed as ${r.listed}`;
    if (r.called) return `${r.called === 'increase' ? 'increased' : 'decreased'}, the ${r.rank === 1 ? '' : `${nth(r.rank)} `}most significant call`;
    if (r.class) return `${r.class}, the ${r.rank === 1 ? '' : `${nth(r.rank)} `}${r.class === 'persistent' ? 'largest' : 'most significant'}`;
    return `${r.rank === 1 ? 'largest' : `${nth(r.rank)} largest`} in ${r.sample}`;
  });
  return parts.length > 2 ? `${parts.slice(0, 2).join(', ')} and ${parts.length - 2} more` : parts.join(', ');
}
const nth = (n) => `${n}${[, 'st', 'nd', 'rd'][n % 100 > 10 && n % 100 < 14 ? 0 : n % 10] ?? 'th'}`;

// A time point as the plots label it, and in full.
const pointLabel = (p) => ({ label: p.label, title: `${p.label}${p.event ? ` (${p.event})` : ''}${p.samples.length > 1 ? `, ${p.samples.length} samples pooled` : ''}` });

// The examples, as the program lists them (api_examples.go), read once.
let examplesList = null;
async function exampleList() {
  examplesList ??= getJSON('api/examples').then((r) => r.examples ?? r).catch(() => []);
  return examplesList;
}

// The example a dataset was opened from: its public data's credit, or its simulated study's ID.
function exampleOf(d, summary) {
  const credited = d.credits?.find((c) => c.example)?.example;
  if (credited) return credited;
  const study = (summary?.samples ?? []).map((x) => x.repertoire?.studyId).find((x) => x?.startsWith('clonoloom-simulated-'));
  return study ? study.replace(/^clonoloom-/, '') : null;
}

// Whether the guided steps are open (closed by the user, they stay closed).
let guideOpen = true;

export function mountTracking(app, container) {
  const { store } = app;
  const datasetSelect = h('select.input.small', { 'aria-label': 'Dataset', onchange: (e) => { chosen = e.target.value; load(); } });
  const partSwitch = h('div.segmented', { role: 'tablist', 'aria-label': 'Part' });
  const annotateButton = h('button.btn.small', { type: 'button', title: 'Give the samples their subjects, time points and reference in the Data view\'s samples table', onclick: () => app.setMode('data') }, icon('edit'), 'Annotate samples');
  const startButton = h('button.btn.primary.small', { type: 'button', onclick: () => run(KIND_OF[part]) });
  const exportButton = h('button.btn.small', { type: 'button', title: 'Export this result as CSV, with a NOTICE of the data\'s source and license', onclick: () => {
    const r = results[KIND_OF[part]];
    if (r?.status === 'ready') exportResult(app, dataset(), r.artifact);
  } }, icon('download'), 'Export');
  const head = h('div.workbench-head', h('h1', icon('threads'), 'Tracking'), partSwitch,
    h('span.spacer'), h('label.kv-inline', h('span.muted', 'Dataset'), datasetSelect), annotateButton, startButton, exportButton);
  const body = h('div.view-body.tracking-view');
  const root = h('div.view', head, h('div.workbench-scroll', body));
  container.append(root);
  const focusedResult = store.ui.focus?.kind === 'result' ? store.project.artifacts.find((a) => a.id === store.ui.focus.id) : null;
  let chosen = store.ui.focus?.kind === 'dataset' ? store.ui.focus.id : focusedResult?.dataset ?? store.project.datasets[0]?.id ?? null;
  let part = PART_OF[focusedResult?.kind] ?? 'subjects';
  // The design: idle | loading | ready | failed, with the design and the samples it was read from.
  let state = { status: 'idle' };
  // Each analysis's result: idle | looking | absent | running | ready | failed (and, for the tests,
  // unavailable: no two time points to compare).
  const results = Object.fromEntries(KINDS.map((k) => [k, { status: 'idle' }]));
  let spec = null;
  let subjectShown = null;
  let selectedKey = null;
  // The Tests and Classes parts' subject, clonotype and class chosen.
  const testsUI = { subject: null, selected: null };
  const classesUI = { subject: null, selected: null, class: 'emerging' };
  // Analyses whose choice the user just made: run at once when no result is kept for it.
  const startWhenAbsent = new Set();
  let generation = 0;

  const dataset = () => store.project.datasets.find((d) => d.id === chosen) ?? store.project.datasets[0] ?? null;

  function specOf(d, open, samples, design) {
    const loci = Object.keys(open.summary.samples.reduce((acc, s) => Object.assign(acc, s.loci), {}));
    const filter = datasetFilter(store.project, d);
    const points = timePoints(design);
    const comparison = comparisonChoice(d, points);
    const choice = trackingChoice(d, comparison);
    const comparable = Boolean(comparison.from && comparison.to);
    const classable = Boolean(comparison.base);
    return { dataset: d.id, definition: datasetDefinition(d, loci), filter: filter?.rules ?? {}, filterRecord: filter, study: samples, points, comparison, choice,
      track: (choice.by === 'test' && !comparable) || (choice.by === 'class' && !classable) ? null : trackRequest(choice),
      compare: comparable ? compareRequest(comparison) : null, classing: classable ? classRequest(comparison) : null };
  }
  // What each analysis is asked: the clonotypes followed, the comparison, the classing, or the
  // study alone.
  const asked = { tracking: () => ({ track: spec.track }), expansion: () => ({ compare: spec.compare }), classes: () => ({ class: spec.classing }), attribution: () => ({}) };
  const request = (kind, extra = {}) => JSON.stringify({ dataset: spec.dataset, definition: spec.definition, filter: spec.filter, study: spec.study, ...asked[kind](), ...extra });

  // The latest load, and a promise that settles once no later one has begun (a load the store's
  // notice begins after a script's edit supersedes the one the script waits for).
  let latest = Promise.resolve();
  const settled = () => {
    const p = latest;
    return p.then(() => (p === latest ? undefined : settled()));
  };
  function load() {
    latest = reload();
    return latest;
  }

  async function reload() {
    const mine = ++generation;
    const d = dataset();
    renderHead();
    if (!d) {
      state = { status: 'idle' };
      for (const kind of KINDS) results[kind] = { status: 'idle' };
      render();
      return;
    }
    if (state.status !== 'ready' || state.dataset !== d.id) {
      state = { status: 'loading' };
      for (const kind of KINDS) results[kind] = { status: 'looking' };
      render();
    }
    try {
      const open = await app.ensureDataset(d);
      if (open.status !== 'open') throw new Error(open.error ?? `${d.name} could not be opened.`);
      const samples = studySamples(store.project, d.id, open.summary);
      const design = await postJSON('api/study', { samples });
      const name = exampleOf(d, open.summary);
      const catalog = await exampleList();
      const normalized = (path) => String(path).replaceAll('\\', '/');
      const example = catalog.find((e) => e.name === name) ?? catalog.find((e) => e.name.startsWith('simulated-counts-') && e.path && d.sources.length && d.sources.every((s) => normalized(s.path).startsWith(normalized(e.path) + '/'))) ?? null;
      if (mine !== generation) return;
      spec = specOf(d, open, samples, design);
      state = { status: 'ready', dataset: d.id, design, samples, summary: open.summary, example };
    } catch (error) {
      if (mine !== generation) return;
      state = { status: 'failed', error: error.message };
      for (const kind of KINDS) results[kind] = { status: 'failed', error: error.message };
      render();
      return;
    }
    render();
    await Promise.all(KINDS.map((kind) => look(mine, d, kind)));
    if (mine === generation) render();
  }

  // An analysis as chosen: the result kept, a job still running, or none yet (run at once when the
  // user just chose it).
  async function look(mine, d, kind) {
    if ((kind === 'tracking' && !spec.track) || (kind === 'expansion' && !spec.compare) || (kind === 'classes' && !spec.classing)) {
      results[kind] = { status: 'unavailable' };
      startWhenAbsent.delete(kind);
      return;
    }
    try {
      const answer = await postJSON(`api/${kind}`, request(kind, { start: false }));
      if (mine !== generation) return;
      if (answer.ready) {
        const read = await getJSON(`api/${kind}/${encodeURIComponent(answer.artifact)}`);
        if (mine !== generation) return;
        results[kind] = { status: 'ready', artifact: answer.artifact, result: read.result, manifest: read.artifact };
        keep(kind, d, read);
        if (kind === 'tracking' && !read.result.subjects.some((s) => s.id === subjectShown)) subjectShown = read.result.subjects[0]?.id ?? null;
        if (kind === 'expansion' && !read.result.subjects.some((s) => s.subject === testsUI.subject)) testsUI.subject = read.result.subjects[0]?.subject ?? null;
        if (kind === 'classes' && !read.result.subjects.some((s) => s.subject === classesUI.subject)) classesUI.subject = read.result.subjects[0]?.subject ?? null;
      } else {
        results[kind] = { status: answer.job ? 'running' : 'absent', artifact: answer.artifact, job: answer.job };
        if (answer.job) watch(kind, answer.job);
        else if (startWhenAbsent.has(kind)) {
          startWhenAbsent.delete(kind);
          await run(kind);
          return;
        }
      }
    } catch (error) {
      if (mine !== generation) return;
      results[kind] = { status: 'failed', error: error.message };
    }
    startWhenAbsent.delete(kind);
  }

  // The result in the project's record (the project tree lists it; the methods paragraph says it).
  function keep(kind, d, read) {
    if (store.project.artifacts.some((a) => a.id === read.artifact.id)) return;
    const r = read.result;
    const common = {
      id: read.artifact.id, kind, dataset: d.id, definition: spec.definition.name ?? '',
      filter: spec.filterRecord ? `${spec.filterRecord.name} v${spec.filterRecord.version}` : null,
      made: read.artifact.created, unit: r.unit, describe: r.describe,
    };
    const sum = (key) => r.subjects.reduce((n, s) => n + s[key], 0);
    const fitOf = (f) => (f ? { on: f.on, pairs: f.pairs.length, clonotypes: f.clonotypes, a: f.model.a, b: f.model.b, calledBySampling: f.calledBySampling, calledByModel: f.calledByModel } : null);
    const records = {
      tracking: () => ({ ...common, options: { confidence: r.confidence }, name: `Tracking · ${d.name}`, method: `Clonotypes followed over time, ${choiceShort({ ...r.choice, list: r.choice.list ?? [] })}; Wilson score intervals`,
        choice: r.choice, subjects: r.subjects.length, followed: r.subjects.reduce((n, s) => n + s.clonotypes.length, 0), intervals: r.intervals }),
      expansion: () => ({ ...common, name: `Tests · ${d.name}`, method: `${TESTS[r.test]?.label ?? r.test} between ${r.comparison.from} and ${r.comparison.to}; ${CORRECTIONS[r.comparison.correction]?.label ?? r.comparison.correction} at ${r.comparison.alpha}`,
        comparison: r.comparison, used: r.test, subjects: r.subjects.length, tested: sum('tested'), increased: sum('increased'), decreased: sum('decreased'), fit: fitOf(r.fit) }),
      classes: () => ({ ...common, name: `Classes · ${d.name}`, method: `Clonotypes classed against ${r.classing.base} by ${TESTS[r.test]?.name ?? r.test}; ${CORRECTIONS[r.classing.correction]?.label ?? r.classing.correction} at ${r.classing.alpha}`,
        classing: r.classing, used: r.test, subjects: r.subjects.length, later: Math.max(0, ...r.subjects.map((s) => s.later.length)),
        counts: Object.fromEntries(Object.keys(CLASSES).map((k) => [k, r.subjects.reduce((n, s) => n + s.counts[k], 0)])), fit: fitOf(r.fit) }),
      attribution: () => ({ ...common, options: { confidence: r.confidence }, name: `Attribution · ${d.name}`, method: 'Each sample\'s share in its reference sample\'s clonotypes, and the share derived from it where told; Wilson score intervals',
        subjects: r.subjects.length, derived: r.subjects.filter((s) => s.derived).length, references: [...new Set(r.subjects.map((s) => s.label))], intervals: r.intervals }),
    };
    const labels = { tracking: 'Follow clonotypes', expansion: 'Test the clonotypes', classes: 'Class the clonotypes', attribution: 'Attribute the samples' };
    store.commit(addResult(store.project, records[kind]()), `${labels[kind]} of ${d.name}`, ['project']);
  }

  // When a job ends: its result read, or what went wrong said. Each job watched once.
  const watching = new Set();
  function watch(kind, jobId) {
    if (watching.has(jobId)) return;
    watching.add(jobId);
    const stop = app.jobs.onFinished((job) => {
      if (job.id !== jobId) return;
      stop();
      watching.delete(jobId);
      if (job.state === 'complete') load();
      else {
        results[kind] = { status: 'failed', error: job.error || `The job ended ${job.state}.` };
        render();
      }
    });
  }

  async function run(kind) {
    const d = dataset();
    if (!d || !spec || !kind) return;
    try {
      const answer = await postJSON(`api/${kind}`, request(kind));
      if (answer.ready) {
        load();
        return;
      }
      results[kind] = { status: 'running', artifact: answer.artifact, job: answer.job };
      app.log({
        tracking: () => `Follow the clonotypes of ${d.name}: ${choiceWords(spec.choice)}.`,
        expansion: () => `Test the clonotypes of ${d.name}: ${spec.comparison.from} against ${spec.comparison.to}, ${comparisonWords(spec.comparison)}.`,
        classes: () => `Class the clonotypes of ${d.name} against ${spec.comparison.base}.`,
        attribution: () => `Attribute the samples of ${d.name} to their reference samples.`,
      }[kind]());
      render();
      watch(kind, answer.job);
    } catch (error) {
      toast(error.message, { kind: 'error' });
    }
  }

  // The user's choice of what is followed, kept with the dataset.
  function choose(choice) {
    const d = dataset();
    if (!d) return;
    const next = useTracking(store.project, d.id, choice);
    if (next === store.project) return;
    startWhenAbsent.add('tracking');
    store.commit(next, `Choose the clonotypes followed in ${d.name}`);
  }

  // The user's choice of the time points compared and how, kept with the dataset; the calls
  // followed again where the Clonotypes part follows them.
  function compareWith(changes) {
    const d = dataset();
    if (!d || !spec) return;
    const next = useComparison(store.project, d.id, { ...spec.comparison, ...changes });
    if (next === store.project) return;
    startWhenAbsent.add(part === 'classes' ? 'classes' : 'expansion');
    if (spec.choice.by === 'test' || spec.choice.by === 'class') startWhenAbsent.add('tracking');
    testsUI.selected = null;
    classesUI.selected = null;
    store.commit(next, `Choose the time points compared in ${d.name}`);
  }

  // A class followed in the Clonotypes part.
  function followClass(cls) {
    part = 'clonotypes';
    selectedKey = null;
    const d = dataset();
    if (spec?.choice.by === 'class' && d?.tracking?.class === cls) {
      render();
      return;
    }
    choose({ by: 'class', class: cls });
  }

  // The calls of the comparison followed in the Clonotypes part.
  function followCalls() {
    part = 'clonotypes';
    selectedKey = null;
    if (spec?.choice.by === 'test') {
      render();
      return;
    }
    choose({ by: 'test' });
  }

  function editList() {
    const d = dataset();
    if (!d) return;
    const area = h('textarea.input.tracking-list', { rows: 10, 'aria-label': 'Clonotypes, one a line', spellcheck: false }, trackingChoice(d).list.join('\n'));
    showDialog({
      title: `Clonotypes to follow · ${d.name}`,
      // The Follow menu back to what is followed, when the list was left as it was.
      onClose: () => render(),
      content: [
        h('p', 'One a line: a clonotype\'s ID (c-…), or its junction as the clonotype table writes it (CASSLAGGPEAFF; a paired clonotype\'s chains joined by "; "). A junction names every clonotype that has it.'),
        area,
        h('p.muted.small', `At most ${count(MAX_LIST)}. The Clonotypes view adds the clonotype chosen there with Follow over time.`),
      ],
      buttons: [
        { label: 'Cancel', ghost: true },
        { label: 'Follow these', primary: true, onClick: () => {
          const list = parseList(area.value);
          if (!list.length) {
            toast('Name at least one clonotype.', { kind: 'error' });
            return false;
          }
          choose({ by: 'list', list });
          return true;
        } },
      ],
    });
  }

  // A clonotype in the Clonotypes view, with the inspector describing it.
  async function showInClonotypes(id) {
    const d = dataset();
    if (!d) return;
    app.focusItem({ kind: 'dataset', id: d.id });
    await app.setMode('clonotypes');
    const view = app.clonotypesView;
    if (!view) return;
    view.show(d.id);
    view.select(id);
  }

  function renderHead() {
    clear(datasetSelect);
    for (const d of store.project.datasets) datasetSelect.append(h('option', { value: d.id, selected: d.id === dataset()?.id }, d.name));
    datasetSelect.disabled = !store.project.datasets.length;
  }

  function render() {
    clear(partSwitch);
    for (const [key, label] of Object.entries(PARTS)) {
      partSwitch.append(h(`button${key === part ? '.active' : ''}`, { type: 'button', role: 'tab', 'aria-selected': key === part ? 'true' : 'false', onclick: () => show(key) }, label));
    }
    clear(body);
    const d = dataset();
    const shown = results[KIND_OF[part]];
    annotateButton.hidden = !d || part !== 'subjects';
    clear(startButton).append(icon('play'), START[KIND_OF[part]] ?? '');
    startButton.hidden = !d || shown?.status !== 'absent';
    exportButton.hidden = !d || shown?.status !== 'ready';
    if (!d) {
      body.append(h('div.empty', icon('threads'), h('h3', 'No data yet'),
        h('p', 'Open a study\'s samples, give each its subject and time point (and mark each subject\'s reference sample, such as an infusion product), and follow its clonotypes through time here.'),
        h('div.btn-row', { style: { justifyContent: 'center', marginTop: '10px' } },
          h('button.btn.primary', { type: 'button', onclick: () => app.openPathDialog() }, icon('folder'), 'Open data'),
          h('button.btn', { type: 'button', onclick: () => app.showExamples() }, icon('flask'), 'Example data'))));
      return;
    }
    if (state.status === 'loading' || state.status === 'idle') {
      body.append(h('p.muted', 'Reading the samples…'));
      return;
    }
    if (state.status === 'failed') {
      body.append(h('div.callout.danger', h('p', state.error)));
      return;
    }
    if (!state.design.subjects.length) {
      body.append(h('div.callout', h('p', 'No sample has a subject yet. Tracking follows clonotypes within a subject: give each sample its subject and time point in the Data view\'s samples table, or fill them from the files\' metadata or names.')));
      for (const f of state.design.findings) body.append(finding(f));
      return;
    }
    const steps = state.example?.opens?.flatMap((o) => o.steps ?? []) ?? [];
    if (steps.length) body.append(guide(d, state.example, steps));
    if (part === 'clonotypes') renderClonotypes(d);
    else if (part === 'tests') {
      renderTests({ app, body, d, points: spec.points, comparison: spec.comparison, state: results.expansion, spec, ui: testsUI,
        compareWith, run: () => run('expansion'), followCalls, showInClonotypes, render,
        select: (key) => { testsUI.selected = testsUI.selected === key ? null : key; render(); } });
    } else if (part === 'classes') {
      renderClasses({ app, body, d, points: spec.points, comparison: spec.comparison, state: results.classes, spec, ui: classesUI,
        compareWith, run: () => run('classes'), followClass, showInClonotypes, render,
        select: (key) => { classesUI.selected = classesUI.selected === key ? null : key; render(); } });
    } else if (part === 'attribution') {
      renderAttribution({ app, body, d, state: results.attribution, render, run: () => run('attribution') });
    } else renderSubjects(d);
  }

  // The example's guided steps, each done by its action on this dataset.
  function guide(d, example, steps) {
    const doing = new Set();
    const list = h('ol.tracking-steps', ...steps.map((step, i) => {
      const button = h('button.btn.small', { type: 'button', onclick: async () => {
        if (doing.has(i)) return;
        doing.add(i);
        button.disabled = true;
        try {
          const result = await app.actions[step.action]({ dataset: d.id, ...(step.args ?? {}) });
          toast(result?.message ?? 'Done.', { kind: 'ok' });
        } catch (error) {
          toast(error.message, { kind: 'error' });
        } finally {
          doing.delete(i);
          button.disabled = false;
        }
      } }, icon('play'), 'Do it');
      return h('li', h('div.tracking-step', h('div', h('b', step.title), h('p.muted.small', step.words)), button));
    }));
    const details = h('details.card.tracking-guide', { open: guideOpen, ontoggle: () => { guideOpen = details.open; } },
      h('summary', h('b', 'Guided steps'), h('span.muted', ` · ${example.title}`)), list);
    return details;
  }

  function show(key) {
    part = PARTS[key] ? key : part;
    render();
  }

  // --- Subjects ----------------------------------------------------------------------------------

  function renderSubjects(d) {
    const { design } = state;
    const subjects = design.subjects;
    const placed = subjects.reduce((n, s) => n + s.samples.length, 0);
    const references = subjects.reduce((n, s) => n + (s.references?.length ?? 0), 0);
    const replicates = subjects.reduce((n, s) => n + (s.replicates?.length ?? 0), 0);
    body.append(h('p.tracking-summary',
      `${plural(subjects.length, 'subject')}, ${plural(placed, 'sample')} placed in time`,
      references ? `, ${plural(references, 'reference sample')}` : '',
      replicates ? `, ${plural(replicates, 'group')} of samples taken at one time` : '', '.'));
    for (const f of design.findings) body.append(finding(f));
    body.append(timelineCard(d, design));
    body.append(subjectTable(design));
  }

  function finding(f) {
    const s = SEVERITY[f.severity] ?? SEVERITY.note;
    const of = f.of ? ` of ${count(f.of)}` : '';
    return h(`div.finding.${f.severity}`,
      h('div.finding-head',
        h(`span.badge.${s.badge}`, icon(s.icon), s.label),
        h('b', f.title),
        h('span.muted', `${count(f.count)}${of}`),
        h('span.spacer'),
        h('code.finding-code', { title: 'The finding\'s code, the same in every version' }, f.code)),
      h('p', f.detail),
      f.examples?.length ? h('p.muted.small', `For example: ${f.examples.join('; ')}${f.count > f.examples.length ? '; …' : ''}.`) : null,
      f.remedy ? h('p.finding-remedy', h('b', 'What to do: '), f.remedy) : null);
  }

  // The subjects' samples on one axis of days (or visits, where no sample has a day), colored by
  // the subject's condition.
  function timelineCard(d, design) {
    const days = design.subjects.some((s) => s.samples.some((p) => p.time?.kind === 'day'));
    const summaryOf = new Map((state.summary?.samples ?? []).map((s) => [s.name, s]));
    const conditionOf = (subject) => {
      for (const p of subject.samples) {
        const s = summaryOf.get(p.name);
        const c = s && fieldValue(store.project, d.id, s, 'condition');
        if (c) return c;
      }
      return null;
    };
    const conditions = [...new Set(design.subjects.map(conditionOf).filter(Boolean))];
    const classOf = (subject) => {
      const c = conditionOf(subject);
      return c ? `plot-s${(conditions.indexOf(c) % 6) + 1}` : 'plot-accent';
    };
    const rows = design.subjects.map((s) => ({
      label: s.id, className: classOf(s),
      marks: s.samples.flatMap((p) => {
        const label = `${p.name}: ${p.reference ? `${p.reference}, ` : ''}${when(p.time)}`;
        if (!p.time) return [{ column: 'reference', label, reference: true }];
        switch (p.time.kind) {
          case 'before': return [{ column: 'baseline', label, reference: !!p.reference }];
          case 'after': return [{ column: 'after', label, reference: !!p.reference }];
          case 'visit': return days ? [] : [{ x: p.time.value, label, reference: !!p.reference }];
          default: return [{ x: p.time.days, label, reference: !!p.reference }];
        }
      }),
    }));
    // The axis says from what event, where every subject's time points name the same one.
    const events = [...new Set(design.subjects.flatMap((s) => s.samples.map((p) => p.time?.event).filter(Boolean)))];
    const xLabel = !days ? 'Visit' : events.length === 1 ? `Days from ${events[0]}` : 'Days from each subject\'s reference event (day 0)';
    const plot = timeline(rows, { label: `Subjects of ${d.name}`, xLabel, width: 680, labelWidth: 96 });
    const legend = [
      ...conditions.map((c, i) => ({ label: c, className: `plot-s${(i % 6) + 1}` })),
      ...(design.subjects.some((s) => s.references?.length) ? [{ label: 'reference sample', className: 'plot-reference' }] : []),
    ];
    const visitsLeftOut = days && design.subjects.some((s) => s.samples.some((p) => p.time?.kind === 'visit'));
    const note = `Each subject's samples by their time point${days ? ', in days' : ', by visit'}; reference samples as diamonds, samples taken at one time one above the other.${visitsLeftOut ? ' Samples given as visits are listed below, not placed among days.' : ''}`;
    const card = h('figure.plot-card',
      h('figcaption', 'Each subject\'s samples in time'),
      h('div.plot-holder', { html: plot.svg }),
      legend.length ? h('div.plot-legend', ...legend.map((l) => h('span', h(`span.swatch.${l.className}`), l.label))) : null,
      h('p.muted.small', note));
    return exportable(card, () => ({
      name: `${d.name} subjects`, title: `Subjects of ${d.name}`, plot: plot.svg, legend,
      caption: [note, `${d.name}: ${plural(design.subjects.length, 'subject')}. Time points as annotated, read by ClonoLoom.`, madeBy(app.version)],
    }));
  }

  // Each subject's samples in order: what each is, when, and what is left out.
  function subjectTable(design) {
    const rows = design.subjects.map((s) => {
      const replicateOf = new Map();
      (s.replicates ?? []).forEach((group, i) => group.forEach((n) => replicateOf.set(n, i + 1)));
      return h('tr',
        h('td', h('b', s.id)),
        h('td', h('div.subject-samples', ...s.samples.map((p) => h('span.subject-sample', { title: p.referenceFrom === 'tissue' ? 'A reference sample by its tissue' : null },
          p.reference ? h('span.badge.accent', p.reference) : null,
          h('span.mono', p.name), h('span.muted', { title: when(p.time) }, p.time ? p.time.label : 'no time point'),
          replicateOf.has(p.name) ? h('span.badge', { title: 'Taken at one time with other samples: replicates' }, `replicate ${replicateOf.get(p.name)}`) : null)))),
        h('td.muted', (s.untimed ?? []).join(', ') || '—'));
    });
    const table = h('table.data.subjects-table',
      h('thead', h('tr', h('th', 'Subject'), h('th', 'Samples, in time order'), h('th', { title: 'Samples without a time point, or with one not understood' }, 'Left out of tracking'))),
      h('tbody', ...rows));
    const section = h('section', { 'aria-label': 'Subjects' }, h('div.section-title', `Subjects · ${design.subjects.length}`), h('div.table-scroll', { tabIndex: 0 }, table));
    if (design.unassigned?.length) section.append(h('p.muted.small', `Without a subject: ${design.unassigned.join(', ')}.`));
    return section;
  }

  // --- Clonotypes followed -------------------------------------------------------------------------

  function renderClonotypes(d) {
    const followed = results.tracking;
    const choice = spec?.choice ?? trackingChoice(d);
    const result = followed.status === 'ready' ? followed.result : null;
    const subjects = result ? result.subjects : [];
    // What is followed, how many, and which subject is shown.
    const bySelect = h('select.input.small', { 'aria-label': 'Follow', onchange: (e) => (e.target.value === 'list' && !choice.list.length ? editList() : choose({ by: e.target.value })) },
      ...Object.entries(BY).map(([key, b]) => h('option', { value: key, selected: key === choice.by }, b.label)));
    const sizeSelect = h('select.input.small', { 'aria-label': 'How many', onchange: (e) => choose({ n: Number(e.target.value) }) },
      ...[...new Set([...SIZES, choice.n ?? 10])].sort((a, b) => a - b).map((n) => h('option', { value: n, selected: n === choice.n }, choice.by === 'test' || choice.by === 'class' ? `${n} most significant` : `${n} largest`)));
    const subjectSelect = h('select.input.small', { 'aria-label': 'Subject', onchange: (e) => { subjectShown = e.target.value; selectedKey = null; render(); } },
      ...subjects.map((s) => h('option', { value: s.id, selected: s.id === subjectShown }, s.id)));
    body.append(h('div.repertoire-controls',
      h('label.kv-inline', h('span.muted', 'Follow'), bySelect),
      choice.by === 'list'
        ? h('button.btn.small', { type: 'button', onclick: () => editList() }, icon('edit'), `Edit the list (${count(choice.list.length)})`)
        : sizeSelect,
      choice.by === 'test' && spec?.comparison.from
        ? h('button.btn.small', { type: 'button', title: 'Choose the time points compared and the test in the Tests part', onclick: () => show('tests') }, `${spec.comparison.from} against ${spec.comparison.to}`)
        : null,
      choice.by === 'class'
        ? h('select.input.small', { 'aria-label': 'Class', onchange: (e) => choose({ by: 'class', class: e.target.value }) },
          ...Object.entries(CLASSES).map(([k, x]) => h('option', { value: k, selected: k === choice.classing?.class }, `${x.label}, against ${spec?.comparison.base ?? 'the base'}`)))
        : null,
      subjects.length ? h('label.kv-inline', h('span.muted', 'Subject'), subjectSelect) : null));
    if (spec) {
      body.append(h('p.muted.small.repertoire-spec', 'Clonotypes: ', h('b', spec.definition.name ?? 'the definition'), spec.filterRecord ? ` · filter "${spec.filterRecord.name}" v${spec.filterRecord.version}` : ' · no filter',
        ` · abundance in ${UNITS[datasetUnit(d)] ?? 'counts'}`));
    }
    if (followed.status === 'looking' || followed.status === 'idle') {
      body.append(h('p.muted', 'Reading…'));
      return;
    }
    if (followed.status === 'unavailable') {
      body.append(h('div.callout', h('p', 'The subjects need samples at two time points for a test to call or class clonotypes: give the samples their time points in the Data view\'s samples table, or follow other clonotypes.')));
      return;
    }
    if (followed.status === 'failed') {
      body.append(h('div.callout.danger', h('p', followed.error)));
      return;
    }
    if (followed.status === 'absent' || followed.status === 'running') {
      const job = followed.job ? app.jobs.get(followed.job) : null;
      body.append(h('div.card.repertoire-start',
        h('h3', 'Clonotypes followed through time'),
        h('p', `The program follows ${choiceWords(choice)} through each subject's samples in time order, under this definition and filter: each one's frequency at every time point, the samples taken at one time pooled, with its 95% interval (Wilson's score interval) where the counts are of cells or molecules. A clonotype not found at a time point is shown as not found, with what that sample's depth could have missed, never as zero. The result is kept with the project.`),
        followed.status === 'running'
          ? h('div.example-progress', h('div.progress', h('div', { style: { width: `${Math.round((job?.progress ?? 0) * 100)}%` } })), h('span.muted.small', job?.message ?? 'Waiting'))
          : h('div.btn-row', h('button.btn.primary', { type: 'button', onclick: () => run('tracking') }, icon('play'), 'Follow clonotypes'))));
      return;
    }
    if (result.caveat) body.append(h(`div.callout${result.intervals ? '' : '.warn'}`, h('p', result.caveat)));
    if (result.unmatched?.length) body.append(h('div.callout.warn', h('p', `${plural(result.unmatched.length, 'entry', 'entries')} of the list name no clonotype under this definition: ${result.unmatched.slice(0, 8).join(', ')}${result.unmatched.length > 8 ? ', …' : ''}.`)));
    if (result.without?.length) body.append(h('p.muted.small', `Not followed, with fewer than two time points${result.choice.by === 'test' ? ' or without both time points compared' : ''}: ${result.without.join(', ')}.`));
    const s = subjects.find((x) => x.id === subjectShown) ?? subjects[0];
    if (!s) {
      body.append(h('div.callout', h('p', 'No subject has samples at two time points or more (or one and a reference sample): there is nothing to follow.')));
      return;
    }
    renderSubject(d, result, s);
  }

  function renderSubject(d, result, s) {
    const unit = UNITS[result.unit] ?? 'counts';
    const points = s.points.map(pointLabel);
    const classOf = (i) => `plot-s${(i % 6) + 1}`;
    const reference = s.points.find((p) => p.kind === 'reference');
    const last = s.points.at(-1);
    const how = s.by !== result.choice.by ? `each sample's ${result.choice.n} largest, as ${s.id} has no reference sample` : choiceShort({ ...result.choice, list: result.choice.list ?? [] });
    const followed = results.tracking;
    body.append(h('p.tracking-summary',
      `${s.id}: ${plural(s.points.length, 'time point')}, ${plural(s.clonotypes.length, 'clonotype')} followed (${how}). `,
      s.clonotypes.length ? `Together they hold ${reference ? `${percent(reference.followed)} of the ${reference.label} and ` : ''}${percent(last.followed)} of the sample at ${last.label}.` : '',
      s.more ? ` ${plural(s.more, 'more clonotype')} chosen are left out (at most 200 a subject).` : ''));
    if (!s.clonotypes.length) {
      body.append(h('div.callout', h('p', { list: 'None of the clonotypes listed is under this definition.', test: `The test called no clonotype in ${s.id} between ${result.choice.test?.from} and ${result.choice.test?.to}.`, class: `No clonotype of ${s.id} is ${result.choice.class} against ${result.choice.classing?.base}.` }[result.choice.by] ?? 'The samples hold no clonotype to follow.')));
      return;
    }
    if (!s.clonotypes.some((c) => c.points.some((v) => v.count > 0))) {
      body.append(h('div.callout', h('p', `None of the clonotypes followed is found in ${s.id}'s samples: the table says what each sample's depth could have missed.`)));
      body.append(followedTable(result, s, classOf));
      return;
    }
    // The plots: one series a clonotype, in the subject's order; the one chosen drawn over the rest.
    const series = s.clonotypes.map((c, i) => ({
      key: c.id, label: junctionsOf(c), className: classOf(i),
      values: c.points.map((v) => ({ y: v.frequency ?? NaN, lower: v.lower ?? NaN, upper: v.upper ?? NaN, absent: v.count === 0 && v.total > 0 })),
    }));
    const selected = s.clonotypes.some((c) => c.id === selectedKey) ? selectedKey : null;
    const pick = (event) => {
      const key = event.target.closest?.('[data-key]')?.getAttribute('data-key');
      if (key) select(key);
    };
    const chosenOne = s.clonotypes.find((c) => c.id === selected);
    const legend = s.clonotypes.slice(0, 12).map((c, i) => ({ label: junctionsOf(c), className: classOf(i) }));
    const caption = (note) => [note, `${d.name}, subject ${s.id}: ${plural(s.clonotypes.length, 'clonotype')} followed (${how}). Clonotypes: ${result.describe}. Abundance in ${unit}.`, madeBy(app.version)];
    const threadPlot = threads(points, series, { label: `Clonotypes of ${s.id} through time`, selected });
    const threadsNote = `Each clonotype's frequency at each time point (equally spaced, in time order; the samples taken at one time pooled), on a logarithmic axis; a clonotype not found at a time point on the lane below, as an open circle.${result.intervals ? ' The clonotype chosen is drawn with its 95% intervals (Wilson\'s); where it was not found, up to what that depth could have missed. The intervals are for sampling alone: two blood samples of one time differ by more.' : ''}`;
    body.append(exportable(h('figure.plot-card.tracking-plot',
      h('figcaption', h('span', 'Threads'), h('span.plot-pickers', chosenOne ? h('span.muted.small', junctionsOf(chosenOne)) : h('span.muted.small', 'Choose a clonotype on a plot or in the table'))),
      h('div.plot-holder', { html: threadPlot.svg, onclick: pick }),
      h('p.muted.small', threadsNote)), () => ({ name: `${d.name} ${s.id} threads`, title: `Clonotypes of ${s.id} through time, ${d.name}`, plot: threadPlot.svg, legend, caption: caption(threadsNote) })));
    const flowPlot = alluvial(points, s.clonotypes.map((c, i) => ({ key: c.id, label: junctionsOf(c), className: classOf(i), values: c.points.map((v) => v.frequency ?? NaN) })), { label: `Clonotypes of ${s.id}, stacked`, selected });
    const flowNote = 'Each clonotype\'s share of the sample at each time point, stacked in one order at every point (the highest at its peak at the bottom), each joined to the next time point by a ribbon: the stack\'s height is the share the clonotypes followed hold together.';
    body.append(exportable(h('figure.plot-card.tracking-plot',
      h('figcaption', 'Alluvial plot'),
      h('div.plot-holder', { html: flowPlot.svg, onclick: pick }),
      h('p.muted.small', flowNote)), () => ({ name: `${d.name} ${s.id} alluvial`, title: `Clonotypes of ${s.id}, stacked, ${d.name}`, plot: flowPlot.svg, legend, caption: caption(flowNote) })));
    body.append(followedTable(result, s, classOf));
    body.append(h('p.muted.small', `Followed ${new Date(followed.manifest.created).toLocaleString()} by ClonoLoom ${followed.manifest.lineage?.software?.[0]?.version ?? ''}; kept as ${followed.artifact}. Clonotypes: ${result.describe}.`));
  }

  // The clonotypes followed, each at every time point: choosing one shows it on the plots.
  function followedTable(result, s, classOf) {
    const cell = (v) => {
      if (!(v.total > 0)) return h('td.r.muted', '—');
      const interval = Number.isFinite(v.upper) ? (v.count === 0 ? `up to ${percent(v.upper)}` : `${percent(v.lower)}–${percent(v.upper)}`) : '';
      return h(`td.r${v.count === 0 ? '.muted' : ''}`, { title: `${count(v.count)} of ${count(v.total)}${interval ? `; 95% interval ${interval}` : ''}` },
        v.count === 0 ? 'not found' : percent(v.frequency), interval ? h('span.muted.small.tracking-interval', interval) : null);
    };
    const rows = s.clonotypes.map((c, i) => h(`tr${c.id === selectedKey ? '.selected' : ''}`, {
      tabIndex: 0, 'aria-selected': c.id === selectedKey ? 'true' : 'false',
      onclick: () => select(c.id),
      onkeydown: (e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); select(c.id); } },
    },
    h('td', h(`span.swatch.${classOf(i)}`, { 'aria-hidden': 'true' })),
    h('td', h('div.mono', junctionsOf(c)), h('div.muted.small', genesOf(c) || c.id), h('div.muted.small', whyWords(c.why))),
    ...c.points.map(cell),
    h('td', h('button.icon-button', { type: 'button', title: 'Show in the Clonotypes view', 'aria-label': `Show ${junctionsOf(c)} in the Clonotypes view`, onclick: (e) => { e.stopPropagation(); showInClonotypes(c.id); } }, icon('clonotypes')))));
    return h('section', { 'aria-label': 'Clonotypes followed' },
      h('div.section-title', `Clonotypes of ${s.id} · ${s.clonotypes.length}`),
      h('div.table-scroll', { tabIndex: 0 }, h('table.data.tracking-table',
        h('thead', h('tr', h('th', { 'aria-label': 'Color' }), h('th', 'Clonotype, and why it is followed'),
          ...s.points.map((p) => h('th.r', { title: pointLabel(p).title }, p.label)), h('th', { 'aria-label': 'Actions' }))),
        h('tbody', ...rows))),
      h('p.muted.small', `Frequencies of ${UNITS[result.unit] ?? 'counts'} at each time point${result.intervals ? ', with their 95% intervals (Wilson\'s, for sampling alone)' : ''}; the samples taken at one time pooled.`));
  }

  function select(key) {
    selectedKey = selectedKey === key ? null : key;
    render();
  }

  // The annotation edited (in the Data view, or by a script), the definition, the filter or what is
  // followed chosen again, or datasets opened: read again.
  let shownChoices = '';
  const choices = () => {
    const d = dataset();
    const summary = d ? app.datasetStates.get(d.id)?.summary : null;
    return JSON.stringify([d?.id ?? null, d && summary ? studySamples(store.project, d.id, summary) : null, d && summary ? summary.samples.map((s) => fieldValue(store.project, d.id, s, 'condition') ?? null) : null,
      d?.definition ?? null, d?.filter ?? null, d?.tracking ?? null, d?.comparison ?? null]);
  };
  function update(topics) {
    if (topics.has('jobs') && results[KIND_OF[part]]?.status === 'running') render();
    if (topics.has('focus')) {
      const r = store.ui.focus?.kind === 'result' ? store.project.artifacts.find((a) => a.id === store.ui.focus.id) : null;
      if (PART_OF[r?.kind]) {
        part = PART_OF[r.kind];
        // A result kept for another choice: that choice shown again.
        const d = store.project.datasets.find((x) => x.id === r.dataset);
        if (d && r.kind === 'tracking' && r.choice) {
          const kept = { by: r.choice.by, n: r.choice.n, list: r.choice.list, evidence: r.choice.evidence };
          if (JSON.stringify(trackRequest(trackingChoice({ tracking: kept }))) !== JSON.stringify(trackRequest(trackingChoice(d)))) store.commit(useTracking(store.project, d.id, kept), `Show the clonotypes followed in ${d.name}`);
          if (r.choice.test) store.commit(useComparison(store.project, d.id, compareRequest(r.choice.test)), `Show the clonotypes followed in ${d.name}`);
        }
        if (d && r.kind === 'expansion' && r.comparison) store.commit(useComparison(store.project, d.id, compareRequest(r.comparison)), `Show the clonotypes tested in ${d.name}`);
        if (d && r.kind === 'classes' && r.classing) store.commit(useComparison(store.project, d.id, { ...classRequest(r.classing), base: r.classing.base }), `Show the clonotypes classed in ${d.name}`);
        if (r.dataset !== chosen) {
          chosen = r.dataset;
          shownChoices = choices();
          load();
        } else render();
      }
    }
    if (topics.has('project') || topics.has('project-loaded') || topics.has('datasets')) {
      if (!store.project.datasets.some((d) => d.id === chosen)) chosen = store.project.datasets[0]?.id ?? null;
      const now = choices();
      if (topics.has('project-loaded') || now !== shownChoices) {
        shownChoices = now;
        load();
      } else renderHead();
    }
  }

  app.trackingView = {
    load: () => load(),
    state: () => state,
    followed: () => results.tracking,
    tested: () => results.expansion,
    classed: () => results.classes,
    attributed: () => results.attribution,
    run: (kind) => run(kind),
    show,
    follow: () => run('tracking'),
    test: () => run('expansion'),
    // The clonotypes as chosen now followed, at once when no result is kept for them.
    followNow: () => {
      startWhenAbsent.add('tracking');
      load();
      return settled();
    },
    showTested: (subject, key = null) => {
      if (subject) testsUI.subject = subject;
      testsUI.selected = key;
      render();
    },
    showClassed: (subject, cls, key = null) => {
      if (subject) classesUI.subject = subject;
      if (cls) classesUI.class = cls;
      classesUI.selected = key;
      render();
    },
    choose: (id) => {
      chosen = id;
      load();
      return settled();
    },
    showSubject: (id) => {
      subjectShown = id;
      selectedKey = null;
      render();
    },
    select: (key) => {
      selectedKey = key;
      render();
    },
  };
  shownChoices = choices();
  load();
  return {
    update,
    destroy() {
      app.trackingView = null;
      root.remove();
    },
  };
}
