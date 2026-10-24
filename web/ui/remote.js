// Remote control (clonoloom-spec/requirements.md, A7): actions sent by programs on this computer
// (scripts, the documentation's screenshot capture, and later AI agents) are performed here, in
// the open window, where the user sees them; what they change goes into the project's history like
// any edit, and can be undone. Each action returns { ok, message, data }. The program lists the
// same actions with their arguments (actions.go, GET /api/remote/tools); a Go test checks the two
// lists agree. Adapted from SituCraft 0.9.0 web/ui/remote.js.

import { useInvariantSpecies } from '../lib/invariant.js';
import { networkOptions } from '../lib/networks.js';
import { closeDialogs, toast } from './overlays.js';
import { clonotypePresets } from './storage.js';
import { showMethods } from './methods.js';
import { datasetDefinition, sameMeaning, useDefinition } from '../lib/definitions.js';
import { datasetFilter } from '../lib/filters.js';
import { SAMPLE_FIELDS, annotate, datasetUnit } from '../lib/datasets.js';
import { ALPHAS, MAX_LIST, timePoints, trackingChoice, useComparison, useTracking } from '../lib/tracking.js';
import { BACKGROUNDS, CENTER_ORDERS, PGEN_ORDERS, centerOrder, pgenOrder } from '../lib/specificity.js';

export class ActionError extends Error {}

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

// A count with its noun: 1 sample, 2 samples, 1,561 rows.
const count = (n, noun) => `${n.toLocaleString('en-US')} ${noun}${n === 1 ? '' : 's'}`;

// The views by the names scripts use.
const VIEWS = { start: 'welcome', welcome: 'welcome', data: 'data', qc: 'qc', clonotypes: 'clonotypes', repertoire: 'repertoire', tracking: 'tracking', specificity: 'specificity', jobs: 'jobs' };

// One of `options` (strings) by a forgiving name: exact, then ignoring case, then the one that
// starts with or contains it. Errors list the choices.
function choose(value, options, what) {
  const text = String(value ?? '').trim();
  const lower = text.toLowerCase();
  const found = options.find((o) => o === text) ?? options.find((o) => o.toLowerCase() === lower);
  if (found) return found;
  const near = options.filter((o) => o.toLowerCase().startsWith(lower));
  if (lower && near.length === 1) return near[0];
  const inside = options.filter((o) => o.toLowerCase().includes(lower));
  if (lower && inside.length === 1) return inside[0];
  throw new ActionError(`${text ? `No ${what} "${text}"` : `Give the ${what}`}. ${what[0].toUpperCase()}${what.slice(1)}s: ${options.join(', ') || 'none'}.`);
}

// Waits for test() to answer something truthy, at most ms; answers it.
async function waitFor(test, ms, what) {
  const until = Date.now() + ms;
  for (;;) {
    const value = await test();
    if (value) return value;
    if (Date.now() > until) throw new ActionError(`Timed out waiting for ${what}.`);
    await sleep(80);
  }
}

async function getJSON(url, options = {}) {
  const response = await fetch(url, { cache: 'no-store', ...options });
  const body = await response.json().catch(() => ({}));
  if (!response.ok) throw new ActionError(body.error ?? `${url}: HTTP ${response.status}`);
  return body;
}

// The actions, as the window performs them: for scripts through remote control (installRemote),
// and for the examples' guided steps (app.actions).
export function remoteActions(app) {
  const { store } = app;

  // A dataset of the project by id or name; the one in focus, else the first.
  function datasetOf(value) {
    const { datasets } = store.project;
    if (!datasets.length) throw new ActionError('The project has no dataset: open data first (open_data or open_example).');
    if (value === undefined || value === null || value === '') {
      const focus = store.ui.focus;
      return datasets.find((d) => focus?.kind === 'dataset' && d.id === focus.id) ?? datasets[0];
    }
    const byId = datasets.find((d) => d.id === value);
    if (byId) return byId;
    const name = choose(value, datasets.map((d) => d.name), 'dataset');
    return datasets.find((d) => d.name === name);
  }

  async function openDataset(d) {
    const state = await app.ensureDataset(d);
    if (state.status !== 'open') throw new ActionError(state.error ?? `${d.name} could not be opened.`);
    return state.summary;
  }

  const lociOf = (summary) => Object.keys(summary.samples.reduce((acc, s) => Object.assign(acc, s.loci), {}));

  const actions = {
    async get_state() {
      const { project, ui } = store;
      const list = await app.examples.refresh();
      return {
        message: `${project.name}: ${project.datasets.length} dataset${project.datasets.length === 1 ? '' : 's'}.`,
        data: {
          project: { id: project.id, name: project.name },
          view: ui.mode,
          focus: ui.focus ?? null,
          datasets: project.datasets.map((d) => ({
            id: d.id, name: d.name, formats: d.formats, unit: datasetUnit(d) || null, singleCell: d.singleCell, rows: d.rows, samples: d.samples,
            findings: d.findings, files: d.sources.map((s) => s.path), metadata: (d.metadata ?? []).map((m) => m.path),
            filter: datasetFilter(project, d)?.name ?? null, definition: datasetDefinition(d, []).name,
            credits: (d.credits ?? []).map((c) => ({ title: c.title, license: c.license, citation: c.citation })),
            state: app.datasetStates.get(d.id)?.status ?? 'not opened',
          })),
          examples: (list ?? []).map((e) => ({ name: e.name, title: e.title, license: e.license, licenseClass: e.licenseClass, size: e.size, state: e.state, opens: e.opens.map((o) => o.title) })),
        },
      };
    },

    async new_project({ name }) {
      await app.newProject();
      if (name) store.commit({ ...store.project, name: String(name) }, 'Name project');
      return { message: `Started ${store.project.name}.`, data: { project: { id: store.project.id, name: store.project.name } } };
    },

    async open_data({ paths, name, mappings }) {
      if (!Array.isArray(paths) || !paths.length) throw new ActionError('open_data needs {"paths": ["/path/to/airr_rearrangement.tsv", …]}.');
      const summary = await app.openData(paths.map(String), { name: name ? String(name) : '', mappings: mappings && typeof mappings === 'object' ? mappings : null, rethrow: true });
      if (!summary) throw new ActionError('The project changed while the files were read.');
      return { message: `Opened ${summary.name}: ${count(summary.samples.length, 'sample')}, ${count(summary.rows, 'row')}.`, data: { dataset: summary.id, name: summary.name, unit: summary.unit || null, findings: { fatal: summary.report.fatal, warnings: summary.report.warnings, notes: summary.report.notes } } };
    },

    async open_example({ example, opening, download }) {
      let list = await app.examples.refresh();
      const name = choose(example, (list ?? []).map((e) => e.name), 'example');
      let e = list.find((x) => x.name === name);
      if (e.state !== 'ready') {
        if (!download) throw new ActionError(`${e.title} is not downloaded: send download: true, or download it in the Example data dialog.`);
        if (e.licenseClass !== 'open') throw new ActionError(`${e.title} is under ${e.license}: only the person, accepting those terms in the Example data dialog, can download it.`);
        if (e.state !== 'downloading') await getJSON(`api/examples/${encodeURIComponent(name)}`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ accept: false }) });
        e = await waitFor(async () => {
          list = await app.examples.refresh();
          const now = list.find((x) => x.name === name);
          if (now.state !== 'downloading' && now.download?.error) throw new ActionError(`${now.title} was not downloaded: ${now.download.error}`);
          return now.state === 'ready' ? now : null;
        }, 30 * 60 * 1000, `${e.title} to download`);
      }
      const k = opening ? e.opens.indexOf(e.opens.find((o) => o.title === choose(opening, e.opens.map((x) => x.title), 'opening'))) : 0;
      const summary = await app.openData(e.paths[k], { name: e.opens[k].name, rethrow: true });
      return { message: `Opened ${summary.name} (${e.license}).`, data: { dataset: summary.id, name: summary.name } };
    },

    async show({ view, dataset }) {
      const mode = VIEWS[choose(view, Object.keys(VIEWS), 'view')];
      const d = dataset !== undefined || mode !== 'welcome' ? (store.project.datasets.length ? datasetOf(dataset) : null) : null;
      if (d) app.focusItem({ kind: 'dataset', id: d.id });
      await app.setMode(mode);
      if (d && mode === 'clonotypes') (await waitFor(() => app.clonotypesView, 5000, 'the Clonotypes view')).show(d.id);
      return { message: `Showing ${view}${d ? ` of ${d.name}` : ''}.` };
    },

    async set_definition({ dataset, preset, definition }) {
      const d = datasetOf(dataset);
      let chosen;
      let describe = '';
      if (preset !== undefined) {
        const presets = await clonotypePresets();
        const name = choose(preset, presets.map((p) => p.definition.name), 'preset');
        const p = presets.find((x) => x.definition.name === name);
        chosen = p.definition;
        describe = p.describe;
      } else if (definition && typeof definition === 'object') {
        chosen = { name: 'Definition by a program', ...definition };
      } else {
        throw new ActionError('set_definition needs a preset or a definition.');
      }
      const before = datasetDefinition(d, lociOf(await openDataset(d)));
      if (!(sameMeaning(before, chosen) && before.name === chosen.name)) store.commit(useDefinition(store.project, d.id, chosen, describe), `Define clonotypes of ${d.name}`);
      return { message: `${d.name}: clonotypes by ${chosen.name}${describe ? ` (${describe})` : ''}.`, data: { definition: chosen } };
    },

    async annotate_samples({ dataset, samples }) {
      const d = datasetOf(dataset);
      if (!samples || typeof samples !== 'object' || Array.isArray(samples)) throw new ActionError('annotate_samples needs samples: {name: {field: value}}.');
      const names = new Set((await openDataset(d)).samples.map((s) => s.name));
      const fields = SAMPLE_FIELDS.map((f) => f.id);
      const edits = [];
      for (const [name, values] of Object.entries(samples)) {
        if (!names.has(name)) throw new ActionError(`${d.name} has no sample ${name}. Its samples: ${[...names].slice(0, 10).join(', ')}${names.size > 10 ? ', …' : ''}.`);
        for (const [field, value] of Object.entries(values ?? {})) {
          if (!fields.includes(field)) throw new ActionError(`No field ${field}: the fields are ${fields.join(', ')}.`);
          edits.push({ dataset: d.id, sample: name, field, value: value ?? '' });
        }
      }
      const next = annotate(store.project, edits, 'Annotated by a program');
      if (next !== store.project) store.commit(next, `Annotate samples of ${d.name}`);
      return { message: `${d.name}: ${count(edits.length, 'value')} set on ${count(Object.keys(samples).length, 'sample')}.`, data: { dataset: d.id, values: edits.length } };
    },

    async describe_subjects({ dataset }) {
      const d = datasetOf(dataset);
      app.focusItem({ kind: 'dataset', id: d.id });
      await app.setMode('tracking');
      const view = await waitFor(() => app.trackingView, 5000, 'the Tracking view');
      await view.choose(d.id);
      const state = view.state();
      if (state.status !== 'ready') throw new ActionError(state.error ?? `The subjects of ${d.name} could not be read.`);
      const { design } = state;
      return { message: `${d.name}: ${count(design.subjects.length, 'subject')}, ${count(design.findings.length, 'finding')}.`, data: design };
    },

    async track_clonotypes({ dataset, by, n, list, subject, class: cls }) {
      const d = datasetOf(dataset);
      const choice = {};
      if (by !== undefined && by !== '') choice.by = choose(by, ['reference', 'largest', 'list', 'test', 'class'], 'way of choosing');
      if (cls !== undefined && cls !== '') {
        choice.class = choose(cls, ['emerging', 'transient', 'contracting', 'persistent'], 'class');
        choice.by ??= 'class';
      }
      if (n !== undefined && n !== null) {
        const k = Number(n);
        if (!Number.isInteger(k) || k < 1 || k > 100) throw new ActionError('n is a whole number from 1 to 100.');
        choice.n = k;
      }
      if (list !== undefined) {
        if (!Array.isArray(list) || list.some((e) => typeof e !== 'string')) throw new ActionError('list is an array of clonotype IDs or junctions.');
        if (list.length > MAX_LIST) throw new ActionError(`list holds at most ${MAX_LIST} clonotypes.`);
        choice.list = list;
        choice.by ??= 'list';
      }
      if (choice.by === 'list' && !(choice.list ?? trackingChoice(d).list).length) throw new ActionError('Following a list needs one: list the clonotypes by ID or junction.');
      app.focusItem({ kind: 'dataset', id: d.id });
      await app.setMode('tracking');
      const view = await waitFor(() => app.trackingView, 5000, 'the Tracking view');
      view.show('clonotypes');
      const next = useTracking(store.project, d.id, choice);
      if (next !== store.project) store.commit(next, `Choose the clonotypes followed in ${d.name}`);
      await view.choose(d.id);
      if (view.state().status === 'failed') throw new ActionError(view.state().error);
      if (view.followed().status === 'absent') await view.follow();
      const done = await waitFor(() => ['ready', 'failed', 'unavailable'].includes(view.followed().status) && view.followed(), 30 * 60 * 1000, 'the clonotypes to be followed');
      if (done.status !== 'ready') throw new ActionError(done.error ?? `${d.name}'s subjects have no two time points for a test to call clonotypes between.`);
      const r = done.result;
      if (subject !== undefined && subject !== '') view.showSubject(choose(subject, r.subjects.map((s) => s.id), 'subject'));
      const followed = r.subjects.reduce((k, s) => k + s.clonotypes.length, 0);
      const value = (v) => ({ count: v.count, total: v.total, frequency: v.frequency, lower: v.lower, upper: v.upper });
      return {
        message: `${d.name}: ${count(followed, 'clonotype')} followed in ${count(r.subjects.length, 'subject')} (${done.artifact}).`,
        data: {
          result: done.artifact, unit: r.unit, definition: r.describe, choice: r.choice, intervals: r.intervals, caveat: r.caveat ?? null, unmatched: r.unmatched ?? [], without: r.without ?? [],
          subjects: r.subjects.map((s) => ({
            id: s.id, by: s.by,
            points: s.points.map((p) => ({ label: p.label, kind: p.kind, days: p.days ?? null, samples: p.samples, followed: p.followed })),
            clonotypes: s.clonotypes.map((c) => ({ id: c.id, chains: c.chains, why: c.why, peak: s.points[c.peak]?.label ?? null, points: c.points.map(value) })),
          })),
        },
      };
    },

    async test_expansion({ dataset, from, to, test, alternative, correction, alpha, stable, subject, calls }) {
      const d = datasetOf(dataset);
      app.focusItem({ kind: 'dataset', id: d.id });
      await app.setMode('tracking');
      const view = await waitFor(() => app.trackingView, 5000, 'the Tracking view');
      view.show('tests');
      await view.choose(d.id);
      if (view.state().status !== 'ready') throw new ActionError(view.state().error ?? `The subjects of ${d.name} could not be read.`);
      const keys = timePoints(view.state().design).map((p) => p.key);
      const changes = { from: choose(from, keys, 'time point'), to: choose(to, keys, 'time point') };
      if (test !== undefined && test !== '') changes.test = choose(test, ['auto', 'betabinomial', 'binomial', 'fisher'], 'test');
      if (stable !== undefined && stable !== null) {
        if (!Array.isArray(stable) || (stable.length !== 0 && stable.length !== 2)) throw new ActionError('stable is two time points, or [] for the replicate samples.');
        changes.stable = stable.length ? { from: choose(stable[0], keys, 'time point'), to: choose(stable[1], keys, 'time point') } : null;
      }
      if (alternative !== undefined && alternative !== '') changes.alternative = choose(alternative, ['two-sided', 'increase', 'decrease'], 'alternative');
      if (correction !== undefined && correction !== '') changes.correction = choose(correction, ['bh', 'storey'], 'correction');
      if (alpha !== undefined && alpha !== null) {
        if (!ALPHAS.includes(Number(alpha))) throw new ActionError(`alpha is one of ${ALPHAS.join(', ')}.`);
        changes.alpha = Number(alpha);
      }
      const next = useComparison(store.project, d.id, changes);
      if (next !== store.project) store.commit(next, `Choose the time points compared in ${d.name}`);
      await view.choose(d.id);
      if (view.tested().status === 'absent') await view.test();
      const done = await waitFor(() => ['ready', 'failed', 'unavailable'].includes(view.tested().status) && view.tested(), 30 * 60 * 1000, 'the tests');
      if (done.status !== 'ready') throw new ActionError(done.error ?? `${d.name}'s subjects have no two time points to compare.`);
      const r = done.result;
      if (subject !== undefined && subject !== '') view.showTested(choose(subject, r.subjects.map((s) => s.subject), 'subject'));
      const most = Math.max(1, Math.min(Number(calls) || 10, 200));
      const called = r.subjects.reduce((k, s) => k + s.increased + s.decreased, 0);
      return {
        message: `${d.name}: ${count(called, 'clonotype')} called between ${r.comparison.from} and ${r.comparison.to} in ${count(r.subjects.length, 'subject')} (${done.artifact}).`,
        data: {
          result: done.artifact, unit: r.unit, definition: r.describe, comparison: r.comparison, test: r.test, caveat: r.caveat, without: r.without ?? [],
          fit: r.fit ? { on: r.fit.on, model: r.fit.model, logLik: r.fit.logLik, clonotypes: r.fit.clonotypes, pairs: r.fit.pairs, calledBySampling: r.fit.calledBySampling, calledByModel: r.fit.calledByModel } : null,
          subjects: r.subjects.map((s) => ({
            subject: s.subject, from: { label: s.from.label, samples: s.from.samples, total: s.totalA }, to: { label: s.to.label, samples: s.to.samples, total: s.totalB },
            tested: s.tested, leftOut: s.leftOut, increased: s.increased, decreased: s.decreased, pi0: s.pi0,
            calls: s.clonotypes.filter((c) => c.call).slice(0, most).map((c) => ({ id: c.id, chains: c.chains, call: c.call, a: c.a, b: c.b, freqA: c.freqA, freqB: c.freqB, log2Fold: c.log2Fold, p: c.p, q: c.q })),
          })),
        },
      };
    },

    async class_clonotypes({ dataset, base, subject, class: cls, count: most }) {
      const d = datasetOf(dataset);
      app.focusItem({ kind: 'dataset', id: d.id });
      await app.setMode('tracking');
      const view = await waitFor(() => app.trackingView, 5000, 'the Tracking view');
      view.show('classes');
      await view.choose(d.id);
      if (view.state().status !== 'ready') throw new ActionError(view.state().error ?? `The subjects of ${d.name} could not be read.`);
      if (base !== undefined && base !== '') {
        const keys = timePoints(view.state().design).map((p) => p.key).filter((k) => k !== 'reference');
        const next = useComparison(store.project, d.id, { base: choose(base, keys, 'time point') });
        if (next !== store.project) store.commit(next, `Choose the base time point in ${d.name}`);
        await view.choose(d.id);
      }
      if (view.classed().status === 'absent') await view.run('classes');
      const done = await waitFor(() => ['ready', 'failed', 'unavailable'].includes(view.classed().status) && view.classed(), 30 * 60 * 1000, 'the classes');
      if (done.status !== 'ready') throw new ActionError(done.error ?? `${d.name}'s subjects have no two time points to class clonotypes between.`);
      const r = done.result;
      const shown = cls !== undefined && cls !== '' ? choose(cls, ['emerging', 'transient', 'contracting', 'persistent'], 'class') : 'emerging';
      view.showClassed(subject !== undefined && subject !== '' ? choose(subject, r.subjects.map((s) => s.subject), 'subject') : null, shown);
      const k = Math.max(1, Math.min(Number(most) || 10, 200));
      return {
        message: `${d.name}: the clonotypes of ${count(r.subjects.length, 'subject')} classed against ${r.classing.base} (${done.artifact}).`,
        data: {
          result: done.artifact, unit: r.unit, definition: r.describe, classing: r.classing, test: r.test, caveat: r.caveat, without: r.without ?? [],
          subjects: r.subjects.map((s) => ({
            subject: s.subject, base: s.base.label, later: s.later.map((p) => p.label), counts: s.counts,
            [shown]: s.clonotypes.filter((c) => c.class === shown).slice(0, k).map((c) => ({ id: c.id, chains: c.chains, frequencies: c.frequencies, calls: c.calls, q: c.q })),
          })),
        },
      };
    },

    async attribute_samples({ dataset }) {
      const d = datasetOf(dataset);
      app.focusItem({ kind: 'dataset', id: d.id });
      await app.setMode('tracking');
      const view = await waitFor(() => app.trackingView, 5000, 'the Tracking view');
      view.show('attribution');
      await view.choose(d.id);
      if (view.attributed().status === 'absent') await view.run('attribution');
      const done = await waitFor(() => ['ready', 'failed'].includes(view.attributed().status) && view.attributed(), 30 * 60 * 1000, 'the attribution');
      if (done.status !== 'ready') throw new ActionError(done.error);
      const r = done.result;
      const value = (v) => (v ? { count: v.count, total: v.total, share: v.frequency, lower: v.lower, upper: v.upper } : null);
      return {
        message: `${d.name}: the samples of ${count(r.subjects.length, 'subject')} attributed to their reference samples (${done.artifact}).`,
        data: {
          result: done.artifact, unit: r.unit, definition: r.describe, caveat: r.caveat, without: r.without ?? [],
          subjects: r.subjects.map((s) => ({
            subject: s.subject, reference: s.reference, label: s.label, clonotypes: s.clonotypes, before: value(s.before), overlap: s.overlap ?? null, derived: s.derived, why: s.why ?? null,
            points: s.points.map((p) => ({ label: p.point.label, days: p.days ?? null, inReference: value(p.value), derived: value(p.from) })),
          })),
        },
      };
    },

    async find_neighbors({ dataset, radius, cdr3Only, organism, top = 10, select }) {
      const d = datasetOf(dataset);
      app.focusItem({ kind: 'dataset', id: d.id });
      await app.setMode('specificity');
      const view = await waitFor(() => app.specificityView, 5000, 'the Specificity view');
      view.show('neighbors');
      const changes = {};
      if (radius !== undefined) changes.radius = Math.round(Number(radius));
      if (cdr3Only !== undefined) changes.cdr3Only = Boolean(cdr3Only);
      if (organism !== undefined) changes.organism = String(organism);
      if (Object.keys(changes).length) await view.choose(changes);
      await view.load();
      await view.settled();
      let state = view.neighbors();
      if (state.status === 'no-loops') throw new ActionError('The V genes\' loops are not downloaded: download them in Reference data (tcrdist3-loops), or ask with cdr3Only.');
      if (state.status === 'unsupported') throw new ActionError('TCRdist compares the amino-acid junctions of T-cell receptors: this dataset or its definition has none.');
      if (state.status === 'absent') await view.run();
      const done = await waitFor(() => ['ready', 'failed'].includes(view.neighbors().status) && view.neighbors(), 30 * 60 * 1000, 'the comparison');
      if (done.status !== 'ready') throw new ActionError(done.error);
      const r = done.result;
      const ranked = [...r.clonotypes].sort((a, b) => b.neighbors - a.neighbors || (a.nearest ? a.distance : Infinity) - (b.nearest ? b.distance : Infinity) || b.total - a.total || (a.id < b.id ? -1 : 1));
      const row = (c) => ({ id: c.id, chains: c.chains, total: c.total, samples: c.samples, neighbors: c.neighbors, nearest: c.nearest ?? null, distance: c.nearest ? c.distance : null });
      const data = {
        result: done.artifact, radius: r.radius, cdr3Only: r.cdr3Only, organism: r.organism ?? null, loops: r.loops ?? null, definition: r.describe,
        compared: r.compared, withNeighbors: r.withNeighbors, excluded: r.excluded, groups: r.groups.map((g) => ({ loci: g.loci, clonotypes: g.receptors })),
        top: ranked.slice(0, Math.max(0, Math.round(top))).map(row),
      };
      if (select) {
        const chosen = ranked[Math.round(select) - 1];
        if (!chosen) throw new ActionError(`select: from 1 to ${ranked.length}.`);
        await view.select(chosen.id);
        data.selected = row(chosen);
      }
      return { message: `${d.name}: ${count(r.compared, 'clonotype')} compared by TCRdist; ${r.withNeighbors} have a neighbor within ${r.radius} (${done.artifact}).`, data };
    },

    async match_specificity({ dataset, database, metric, genes, radius, species, minScore, cdr3Only, follow } = {}) {
      const d = datasetOf(dataset);
      app.focusItem({ kind: 'dataset', id: d.id });
      await app.setMode('specificity');
      const view = await waitFor(() => app.specificityView, 5000, 'the Specificity view');
      view.show('database');
      const changes = {};
      if (database !== undefined) changes.database = choose(database, ['vdjdb', 'simulated-vaccine'], 'database');
      if (metric !== undefined) changes.metric = choose(metric, ['exact', 'tcrdist'], 'metric');
      if (species !== undefined) changes.species = choose(species, ['HomoSapiens', 'MusMusculus', 'MacacaMulatta'], 'species');
      for (const [key, value, max] of [['radius', radius, 1000], ['minScore', minScore, 3]]) if (value !== undefined) {
        if (!Number.isInteger(value) || value < 0 || value > max) throw new ActionError(`${key}: from 0 to ${max}.`);
        changes[key] = value;
      }
      for (const [key, value] of Object.entries({ genes, cdr3Only })) if (value !== undefined) changes[key] = Boolean(value);
      if (Object.keys(changes).length) await view.chooseDatabase(changes);
      await view.load(); await view.settled();
      const state = view.database();
      if (['unsupported', 'no-database', 'no-loops', 'failed'].includes(state.status)) throw new ActionError(state.error ?? 'Use an amino-acid definition and prepare the chosen database and V-gene loops, or explicitly compare CDR3s alone.');
      if (state.status === 'absent') await view.run('specificity');
      const done = await waitFor(() => ['ready', 'failed'].includes(view.database().status) && view.database(), 30 * 60 * 1000, 'specificity matching');
      if (done.status !== 'ready') throw new ActionError(done.error);
      const data = { artifact: done.artifact, matches: done.result.matches.length, epitopes: done.result.epitopes.map((e) => ({ epitope: e.epitope, antigen: e.antigen, species: e.species, clonotypes: e.clonotypes.length, shares: e.shares })), samples: done.result.samples };
      if (follow !== undefined) {
        if (!Number.isInteger(follow) || follow < 1 || follow > data.epitopes.length) throw new ActionError('Choose an epitope group by its number, from 1.');
        if (data.epitopes[follow - 1].clonotypes > 1000) throw new ActionError('Tracking accepts at most 1000 IDs; narrow the matching options.');
        await view.followEpitope(follow - 1);
      }
      return { message: `Found ${data.matches} chain–record matches in ${data.epitopes.length} epitope/antigen groups.`, data };
    },

    async find_bcell_families({ dataset, mode = 'auto', scope = 'subject', threshold = null } = {}) {
      const d = datasetOf(dataset);
      app.focusItem({ kind: 'dataset', id: d.id });
      await app.setMode('bcells'); app.bcellsMode.show('families');
      const view = app.familiesView;
      await view.show(d.id, { mode, scope, threshold });
      if (view.state().status === 'absent') await view.run();
      const done = await waitFor(() => ['ready', 'failed'].includes(view.state().status) && view.state(), 30 * 60 * 1000, 'B-cell families');
      if (done.status !== 'ready') throw new ActionError(done.error);
      return { message: `${done.page.familyCount} families; ${done.page.result.eligible} eligible observations.`, data: { artifact: done.artifact, families: done.page.familyCount, representatives: done.page.representativeCount, eligible: done.page.result.eligible, excluded: done.page.result.excluded, withoutNeighbor: done.page.result.withoutNeighbor, options: done.page.result.familiesOptions } };
    },

    async reconstruct_bcell_germlines({ dataset, families, reference } = {}) {
      const d = datasetOf(dataset);
      app.focusItem({ kind: 'dataset', id: d.id });
      await app.setMode('bcells'); app.bcellsMode.show('germlines');
      const view = app.germlinesView;
      await view.show(d.id, { families, reference });
      if (view.state().status === 'idle') throw new ActionError('Save a thresholded family result first.');
      if (view.state().status === 'absent') await view.run();
      const done = await waitFor(() => ['ready', 'failed'].includes(view.state().status) && view.state(), 30 * 60 * 1000, 'Germline reconstruction');
      if (done.status !== 'ready') throw new ActionError(done.error);
      return { message: `${done.page.result.reconstructed} reconstructed source rows.`, data: { artifact: done.artifact, reconstructed: done.page.result.reconstructed, rows: done.page.rowCount, familyLoci: done.page.familyCount, excluded: done.page.result.excluded, options: done.page.result.germlinesOptions } };
    },

    async measure_bcell_shm({ dataset, germlines } = {}) {
      const d = datasetOf(dataset);
      app.focusItem({ kind: 'dataset', id: d.id });
      await app.setMode('bcells'); app.bcellsMode.show('shm');
      const view = app.shmView;
      await view.show(d.id, { germlines });
      if (view.state().status === 'idle') throw new ActionError('Save a germline result first.');
      if (view.state().status === 'absent') await view.run();
      const done = await waitFor(() => ['ready', 'failed'].includes(view.state().status) && view.state(), 30 * 60 * 1000, 'Somatic hypermutation');
      if (done.status !== 'ready') throw new ActionError(done.error);
      return { message: `${done.page.result.evaluated} source rows with evaluable bases.`, data: { artifact: done.artifact, evaluated: done.page.result.evaluated, rows: done.page.rowCount, excluded: done.page.result.excluded, codonExclusions: done.page.result.codonExclusions, options: done.page.result.shmOptions } };
    },

    async build_bcell_lineages({ dataset, germlines } = {}) {
      const d = datasetOf(dataset);
      app.focusItem({ kind: 'dataset', id: d.id });
      await app.setMode('bcells'); app.bcellsMode.show('lineages');
      const view = app.lineagesView;
      await view.show(d.id, { germlines });
      if (view.state().status === 'idle') throw new ActionError('Save a germline result first.');
      if (view.state().status === 'absent') await view.run();
      const done = await waitFor(() => ['ready', 'failed'].includes(view.state().status) && view.state(), 30 * 60 * 1000, 'Lineage trees');
      if (done.status !== 'ready') throw new ActionError(done.error);
      return { message: `${done.page.result.built} family/locus trees built.`, data: { artifact: done.artifact, built: done.page.result.built, rows: done.page.rowCount, excluded: done.page.result.excluded, options: done.page.result.lineageOptions } };
    },

    async summarize_bcell_isotypes({ dataset, families } = {}) {
      const d = datasetOf(dataset);
      app.focusItem({ kind: 'dataset', id: d.id });
      await app.setMode('bcells'); app.bcellsMode.show('isotypes');
      const view = app.isotypesView;
      await view.show(d.id, { families });
      if (view.state().status === 'idle') throw new ActionError('Save a thresholded family result first.');
      if (view.state().status === 'absent') await view.run();
      const done = await waitFor(() => ['ready', 'failed'].includes(view.state().status) && view.state(), 30 * 60 * 1000, 'Isotypes');
      if (done.status !== 'ready') throw new ActionError(done.error);
      return { message: `${done.page.result.assigned} assigned observations summarized.`, data: { artifact: done.artifact, assigned: done.page.result.assigned, resolvedClass: done.page.result.resolvedClass, resolvedGene: done.page.result.resolvedGene, families: done.page.familyCount, excluded: done.page.result.excluded, issues: done.page.result.issues, options: done.page.result.isotypeOptions } };
    },

    async inspect_expression({ path, cellsPath, example = false, hdf5Example = false, studyExample = false, studyPreset, dataset, sample } = {}) {
      const d = dataset === '' ? null : datasetOf(dataset);
      if (d && (!Number.isInteger(sample) || sample < 0 || sample >= d.samples.length)) throw new ActionError('Give an explicit zero-based receptor sample index.');
      await app.setMode('cells');
      const view = await waitFor(() => app.cellsView, 5000, 'Cells');
      const page = await view.inspect({ path, cellsPath, example, hdf5Example, studyExample, studyPreset, dataset: d?.id ?? '', sample: d ? sample : null });
      if (!page) throw new ActionError('The project changed while reading expression.');
      return { message: 'Expression evidence opened.', data: { id: page.summary.id, links: page.links ?? null, annotations: page.summary.annotations ?? null } };
    },

    async inspect_bcell_inputs({ dataset, select } = {}) {
      const d = datasetOf(dataset);
      app.focusItem({ kind: 'dataset', id: d.id });
      await app.setMode('bcells');
      app.bcellsMode.show('inputs');
      const view = await waitFor(() => app.bcellsView, 5000, 'B cells');
      const page = await view.show(d.id);
      if (select !== undefined) {
        if (!Number.isInteger(select) || select < 1 || select > page.rows.length) throw new ActionError('Select a rearrangement on the first page, from 1.');
        await view.inspect(page.rows[select - 1].id);
      }
      return { message: `${d.name}: ${page.count} B-cell rearrangements under the active filter.`, data: page };
    },

    async flag_invariant({ dataset, species, population = '' } = {}) {
      const d = datasetOf(dataset);
      species = choose(species, ['human', 'mouse'], 'species');
      if (population) population = choose(population, ['MAIT', 'iNKT', 'GEM'], 'population');
      store.commit(useInvariantSpecies(store.project, d.id, species), 'Choose invariant-population species');
      app.focusItem({ kind: 'dataset', id: d.id });
      await app.setMode('clonotypes');
      const view = await waitFor(() => app.clonotypesView, 5000, 'Clonotypes');
      view.show(d.id);
      await view.population(population);
      const page = await view.page(0, 20);
      return { message: `${d.name}: ${species} invariant-population rules.`, data: page };
    },

    async find_networks({ dataset, metric, cutoff, arms, sameV, sameJ, cdr3Only, select } = {}) {
      const d = datasetOf(dataset);
      app.focusItem({ kind: 'dataset', id: d.id });
      await app.setMode('specificity');
      const view = await waitFor(() => app.specificityView, 5000, 'the Specificity view');
      view.show('networks');
      const changes = {};
      if (metric !== undefined) changes.metric = choose(metric, ['identity', 'hamming', 'levenshtein', 'tcrdist'], 'metric');
      if (arms !== undefined) changes.arms = choose(arms, ['all', 'any'], 'arms');
      if (cutoff !== undefined) {
        if (!Number.isInteger(cutoff) || cutoff < 0 || cutoff > ((metric ?? networkOptions(d).metric) === 'tcrdist' ? 1000 : (metric ?? networkOptions(d).metric) === 'identity' ? 0 : 2)) throw new ActionError('Invalid network cutoff.');
        changes.cutoff = cutoff;
      }
      for (const [key, value] of Object.entries({ sameV, sameJ, cdr3Only })) if (value !== undefined) changes[key] = Boolean(value);
      if (Object.keys(changes).length) await view.chooseNetworks(changes);
      await view.load(); await view.settled();
      const state = view.networks();
      if (['unsupported', 'no-loops', 'failed'].includes(state.status)) throw new ActionError(state.error ?? 'Choose a junction definition and download V-gene loops, or compare CDR3s alone.');
      if (state.status === 'absent') await view.run('networks');
      const done = await waitFor(() => ['ready', 'failed'].includes(view.networks().status) && view.networks(), 30 * 60 * 1000, 'the network');
      if (done.status !== 'ready') throw new ActionError(done.error);
      if (select !== undefined) view.selectCluster(Number(select));
      return { message: `Found ${done.result.graph.components.length} connected components.`, data: { artifact: done.artifact, nodes: done.result.clonotypes.length, edges: done.result.graph.edges.length, components: done.result.graph.components.length } };
    },

    async find_neighborhoods({ dataset, background, locus, control, minNeighbors, order = 'enriched', top = 10, select }) {
      const d = datasetOf(dataset);
      app.focusItem({ kind: 'dataset', id: d.id });
      await app.setMode('specificity');
      const view = await waitFor(() => app.specificityView, 5000, 'the Specificity view');
      view.show('neighborhoods');
      const by = choose(order, Object.keys(CENTER_ORDERS), 'order');
      view.centerOrder(by);
      const changes = {};
      if (background !== undefined) changes.background = choose(String(background), Object.keys(BACKGROUNDS), 'background');
      if (locus !== undefined) changes.locus = String(locus).toUpperCase();
      if (control !== undefined) changes.control = Number(control);
      if (minNeighbors !== undefined) changes.minNeighbors = Math.round(Number(minNeighbors));
      if (Object.keys(changes).length) await view.chooseNeighborhoods(changes);
      await view.load();
      await view.settled();
      const state = view.neighborhoods();
      if (state.status === 'no-loops') throw new ActionError('The V genes\' loops are not downloaded: download them in Reference data (tcrdist3-loops).');
      if (state.status === 'no-models') throw new ActionError('No recombination model of this locus is downloaded: download OLGA\'s models in Reference data (olga-models), or ask with background "shuffled".');
      if (state.status === 'unsupported') throw new ActionError('Neighborhoods need clonotypes defined by their amino-acid junctions: choose another definition.');
      if (state.status === 'absent') await view.run('neighborhoods');
      const done = await waitFor(() => ['ready', 'failed'].includes(view.neighborhoods().status) && view.neighborhoods(), 30 * 60 * 1000, 'the neighborhoods');
      if (done.status !== 'ready') throw new ActionError(done.error);
      const r = done.result;
      const ranked = centerOrder(r.centers, by);
      const row = (c) => ({
        id: c.id, chain: c.chain, total: c.total, samples: c.samples, radius: c.radius, neighbors: c.neighbors,
        abundance: c.abundance, background: c.background, backgroundWeighted: c.weighted,
        relativeRate: c.relativeRate, oddsRatio: c.oddsRatio, chiSquare: c.chiSquare, motif: c.motif,
      });
      const data = {
        result: done.artifact, locus: r.locus, loci: r.loci, definition: r.describe,
        source: r.source, organism: r.organism ?? null, loops: r.loops ?? null, cdr3Only: Boolean(r.neighborhoods.cdr3Only),
        control: r.neighborhoods.control, minNeighbors: r.neighborhoods.minNeighbors,
        receptors: r.result.receptors, centers: r.result.centers, shown: r.result.shown,
        largest: r.result.largest, largestRadius: r.result.largestRadius, excluded: r.result.excluded,
        order: by, top: ranked.slice(0, Math.max(0, Math.round(top))).map(row),
      };
      if (select) {
        const chosen = ranked[Math.round(select) - 1];
        if (!chosen) throw new ActionError(`select: from 1 to ${ranked.length}.`);
        view.selectCenter(chosen.id);
        data.selected = { ...row(chosen), members: chosen.members };
      }
      return { message: `${d.name}: ${count(r.result.centers, 'receptor')} of ${r.result.receptors.toLocaleString('en-US')} have a neighborhood of ${r.neighborhoods.minNeighbors} or more (${done.artifact}).`, data };
    },

    async measure_generation({ dataset, models, organism, genes, order = 'abundant', top = 10 }) {
      const d = datasetOf(dataset);
      app.focusItem({ kind: 'dataset', id: d.id });
      await app.setMode('specificity');
      const view = await waitFor(() => app.specificityView, 5000, 'the Specificity view');
      view.show('generation');
      const by = choose(order, Object.keys(PGEN_ORDERS), 'order');
      view.order(by);
      const changes = {};
      if (models !== undefined) changes.models = choose(String(models), ['', 'olga', 'clonoloom'], 'models');
      if (organism !== undefined) changes.organism = choose(String(organism), ['', 'human', 'mouse'], 'organism');
      if (genes !== undefined) changes.genes = Boolean(genes);
      if (Object.keys(changes).length) await view.choosePgen(changes);
      await view.load();
      await view.settled();
      const state = view.generation();
      if (state.status === 'no-models') throw new ActionError('OLGA\'s models are not downloaded: download them in Reference data (olga-models), or ask with models "clonoloom".');
      if (state.status === 'unsupported') throw new ActionError('Generation probabilities need clonotypes defined by their junctions: choose another definition.');
      if (state.status === 'absent') await view.run('pgen');
      const done = await waitFor(() => ['ready', 'failed'].includes(view.generation().status) && view.generation(), 30 * 60 * 1000, 'the probabilities');
      if (done.status !== 'ready') throw new ActionError(done.error);
      const r = done.result;
      const computed = r.clonotypes.filter((c) => c.pgen >= 0).length;
      const data = {
        result: done.artifact, models: r.models, set: { name: r.set.name, checksum: r.set.checksum }, organism: r.organism, genes: r.genes, loci: r.loci, definition: r.describe,
        clonotypes: r.clonotypes.length, computed, without: r.without,
        samples: r.samples.map((s) => ({ sample: s.name, clonotypes: s.with, zero: s.zero, log10Quartiles: s.with > s.zero ? s.quartiles : null })),
        order: by,
        top: pgenOrder(r.clonotypes, by).slice(0, Math.max(0, Math.round(top))).map((c) => ({ id: c.id, chains: c.chains, total: c.total, samples: c.samples, pgen: c.pgen >= 0 ? c.pgen : null, chain: c.chain.map((p) => (p >= 0 ? p : null)) })),
      };
      return { message: `${d.name}: ${count(computed, 'clonotype')} of ${r.clonotypes.length.toLocaleString('en-US')} with a generation probability, under ${r.set.name} (${done.artifact}).`, data };
    },

    async select_clonotype({ dataset, rank, id }) {
      const d = datasetOf(dataset);
      app.focusItem({ kind: 'dataset', id: d.id });
      await app.setMode('clonotypes');
      const view = await waitFor(() => app.clonotypesView, 5000, 'the Clonotypes view');
      view.show(d.id);
      let target = id;
      if (!target) {
        const r = Number(rank ?? 1);
        if (!Number.isInteger(r) || r < 1) throw new ActionError('rank is a whole number from 1.');
        const page = await view.page(r - 1, 1);
        if (!page.rows.length) throw new ActionError(`${d.name} has ${page.matched} clonotypes under its definition.`);
        target = page.rows[0].id;
      }
      view.select(target);
      return { message: `Chose clonotype ${target} of ${d.name}.`, data: { id: target } };
    },

    async measure_diversity({ dataset }) {
      const d = datasetOf(dataset);
      app.focusItem({ kind: 'dataset', id: d.id });
      await app.setMode('repertoire');
      const view = await waitFor(() => app.repertoireView, 5000, 'the Repertoire view');
      view.show('diversity');
      await waitFor(() => !['looking', 'idle'].includes(view.state().status), 60000, 'the view to look for a result');
      if (view.state().status === 'absent') await view.measure();
      const done = await waitFor(() => ['ready', 'failed'].includes(view.state().status) && view.state(), 30 * 60 * 1000, 'the measurement');
      if (done.status === 'failed') throw new ActionError(done.error);
      const keys = ['richness', 'chao1', 'hill1', 'hill1Estimated', 'hill2', 'hill2Estimated', 'clonality', 'simpsonClonality', 'gini', 'coverage'];
      return {
        message: `The diversity of ${d.name}: ${count(done.result.samples.length, 'sample')} (${done.artifact}).`,
        data: {
          result: done.artifact, unit: done.result.unit, definition: done.result.describe,
          samples: done.result.samples.map((s) => ({ name: s.name, abundance: s.result.abundance, measures: Object.fromEntries(keys.map((k) => [k, s.result.measures[k] ?? null])) })),
        },
      };
    },

    async rarefy({ dataset, level }) {
      const d = datasetOf(dataset);
      const key = level === undefined || level === '' ? 'coverage' : choose(level, ['coverage', 'size', 'extrapolatedCoverage', 'extrapolatedSize'], 'level');
      app.focusItem({ kind: 'dataset', id: d.id });
      await app.setMode('repertoire');
      const view = await waitFor(() => app.repertoireView, 5000, 'the Repertoire view');
      view.show('rarefaction');
      await waitFor(() => !['looking', 'idle'].includes(view.state('rarefaction').status), 60000, 'the view to look for a result');
      if (view.state('rarefaction').status === 'absent') await view.measure('rarefaction');
      const done = await waitFor(() => ['ready', 'failed'].includes(view.state('rarefaction').status) && view.state('rarefaction'), 30 * 60 * 1000, 'the curves');
      if (done.status === 'failed') throw new ActionError(done.error);
      view.setLevel(key);
      const r = done.result;
      const bySize = /size/i.test(key);
      const index = key.startsWith('extrapolated') ? 1 : 0;
      return {
        message: `${d.name}'s ${count(r.samples.length, 'sample')} compared at ${bySize ? 'equal size' : 'equal coverage'} (${done.artifact}).`,
        data: {
          result: done.artifact, unit: r.unit, definition: r.describe, levels: r.levels, comparedAt: key,
          samples: r.samples.map((s) => {
            const p = (bySize ? s.curves.atSize : s.curves.atCoverage)?.[index];
            return { name: s.name, abundance: s.curves.abundance, size: p?.size ?? null, coverage: p?.coverage ?? null, method: p?.method ?? null, clonotypes: p?.hill?.[0] ?? null, shannon: p?.hill?.[1] ?? null, simpson: p?.hill?.[2] ?? null };
          }),
        },
      };
    },

    async gene_usage({ dataset, part, segment, level, weight, rows }) {
      const d = datasetOf(dataset);
      const given = (v) => v !== undefined && v !== '';
      const shown = given(part) ? choose(part, ['genes', 'junctions'], 'part') : 'genes';
      const choices = {
        segment: given(segment) ? choose(segment, ['V', 'D', 'J', 'C'], 'segment') : 'V',
        level: given(level) ? choose(level, ['gene', 'family'], 'level') : 'gene',
        weight: given(weight) ? choose(weight, ['clonotypes', 'abundance'], 'weight') : 'clonotypes',
        rows: given(rows) ? choose(rows, ['samples', 'groups'], 'rows') : 'samples',
      };
      app.focusItem({ kind: 'dataset', id: d.id });
      await app.setMode('repertoire');
      const view = await waitFor(() => app.repertoireView, 5000, 'the Repertoire view');
      view.show(shown);
      await waitFor(() => !['looking', 'idle'].includes(view.state('usage').status), 60000, 'the view to look for a result');
      if (view.state('usage').status === 'absent') await view.measure('usage');
      const done = await waitFor(() => ['ready', 'failed'].includes(view.state('usage').status) && view.state('usage'), 30 * 60 * 1000, 'the counts');
      if (done.status === 'failed') throw new ActionError(done.error);
      view.setUsage(choices);
      const r = done.result;
      const units = choices.rows === 'groups' ? r.groups ?? [] : r.samples;
      const w = choices.weight;
      const top = (shares, n) => [...(shares ?? [])].filter((s) => s.name).sort((a, b) => (b[w] ?? 0) - (a[w] ?? 0)).slice(0, n)
        .map((s) => ({ name: s.name, share: s[w], interval: s[`${w}Interval`] ?? null }));
      return {
        message: `${d.name}: the ${choices.segment} ${choices.level === 'gene' ? 'genes' : 'families'} and junctions of ${units.length} ${choices.rows === 'groups' ? 'conditions' : 'samples'} by ${w} (${done.artifact}).`,
        data: {
          result: done.artifact, unit: r.unit, definition: r.describe, ...choices,
          groups: (r.groups ?? []).map((g) => ({ name: g.name, samples: g.samples })),
          rows: units.map((u) => ({
            name: u.name,
            loci: u.loci.map((l) => ({
              locus: l.locus, clonotypes: l.clonotypes,
              genes: top(l.genes.find((g) => g.segment === choices.segment && g.level === choices.level)?.shares, 10),
              lengths: top(l.lengths, 5),
              meanLength: { value: l.meanLength?.[w] ?? null, interval: l.meanLength?.[`${w}Interval`] ?? null },
            })),
          })),
        },
      };
    },

    async measure_overlap({ dataset, measure, pairs }) {
      const d = datasetOf(dataset);
      const ids = ['morisita', 'morisitaHorn', 'jaccard', 'jaccardRarefied', 'sorensen', 'sorensenRarefied', 'overlap', 'overlapRarefied', 'shared', 'sharedRarefied', 'abundance'];
      const id = measure === undefined || measure === '' ? 'morisita' : choose(measure, ids, 'measure');
      app.focusItem({ kind: 'dataset', id: d.id });
      await app.setMode('repertoire');
      const view = await waitFor(() => app.repertoireView, 5000, 'the Repertoire view');
      view.show('overlap');
      await waitFor(() => !['looking', 'idle'].includes(view.state('overlap').status), 60000, 'the view to look for a result');
      if (view.state('overlap').status === 'absent') await view.measure('overlap');
      const done = await waitFor(() => ['ready', 'failed'].includes(view.state('overlap').status) && view.state('overlap'), 30 * 60 * 1000, 'the overlap');
      if (done.status === 'failed') throw new ActionError(done.error);
      view.setOverlap({ measure: id });
      const r = done.result;
      const key = id === 'abundance' ? 'abundanceOfA' : id;
      const top = [...r.pairs].sort((x, y) => (y.measures[key]?.estimate ?? -1) - (x.measures[key]?.estimate ?? -1)).slice(0, Math.max(1, Math.min(Number(pairs) || 10, 200)));
      return {
        message: `${d.name}: the overlap of ${count(r.pairs.length, 'pair')} of ${count(r.samples.length, 'sample')} (${done.artifact}).`,
        data: {
          result: done.artifact, unit: r.unit, definition: r.describe, measure: id, depth: r.depth,
          summaries: (r.summaries ?? []).map((s) => ({ kind: s.kind, pairs: s.pairs, mean: s.measures[key]?.mean ?? null, min: s.measures[key]?.min ?? null, max: s.measures[key]?.max ?? null })),
          pairs: top.map((p) => ({ a: r.samples[p.a].name, b: r.samples[p.b].name, value: p.measures[key] ?? null, shared: p.measures.shared?.estimate ?? null })),
          public: { by: r.public.by, units: r.public.units, incidence: r.public.incidence, top: (r.public.top ?? []).slice(0, 10) },
        },
      };
    },

    async show_examples() {
      app.showExamples();
      return { message: 'Showing the example data.' };
    },

    async show_methods({ dataset }) {
      const d = datasetOf(dataset);
      const written = await showMethods(app, d);
      if (!written) throw new ActionError(`${d.name} could not be opened.`);
      return { message: `The methods paragraph of ${d.name}.`, data: written };
    },

    async set_theme({ theme }) {
      const t = choose(theme, ['light', 'dark', 'system'], 'theme');
      app.setTheme(t);
      return { message: `Theme: ${t}.` };
    },

    async close_dialogs() {
      closeDialogs();
      document.querySelectorAll('.toast').forEach((t) => t.remove());
      return { message: 'Closed.' };
    },
  };
  return actions;
}

export function installRemote(app) {
  const actions = app.actions ?? remoteActions(app);

  async function perform(event) {
    let outcome;
    try {
      const action = actions[event.action];
      if (!action) throw new ActionError(`This page does not know the action ${event.action}: reload it.`);
      const result = await action(event.args ?? {});
      outcome = { ok: true, message: result?.message ?? 'Done.', data: result?.data };
      app.log(`${event.client}: ${outcome.message}`);
    } catch (error) {
      outcome = { ok: false, message: error.message };
      app.log(`${event.client} asked for ${event.action}: ${error.message}`);
    }
    await fetch(`api/remote/result/${encodeURIComponent(event.id)}`, { method: 'POST', body: JSON.stringify(outcome), headers: { 'Content-Type': 'application/json' } }).catch(() => {});
  }

  // Only the run of ClonoLoom this page was opened from takes it as its window: when the
  // connection drops and another run answers, the page stops taking remote control and asks to be
  // reloaded.
  const session = app.info?.session ?? '';
  let source = null;
  let announced = false;
  const connect = () => {
    source = new EventSource(`api/remote/events?session=${encodeURIComponent(session)}`);
    source.addEventListener('open', () => {
      if (!announced) app.log('Remote control is on: programs on this computer can drive this window (clonoloom --remote-control).');
      announced = true;
    });
    source.addEventListener('action', (message) => {
      try {
        perform(JSON.parse(message.data));
      } catch {
        // not an action
      }
    });
    source.onerror = async () => {
      source.close();
      const now = await fetch('api/info', { cache: 'no-store' }).then((r) => (r.ok ? r.json() : null)).catch(() => null);
      if (now?.session && now.session !== session) {
        const words = `ClonoLoom was started again (${now.version}): this page no longer takes remote control. Reload it to work with that run.`;
        app.log(words);
        toast(words, { kind: 'error', timeout: 0 });
        return;
      }
      setTimeout(connect, 2000);
    };
  };
  connect();
  return { disconnect: () => source?.close(), actions };
}
