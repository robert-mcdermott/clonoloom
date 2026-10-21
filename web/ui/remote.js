// Remote control (clonoloom-spec/requirements.md, A7): actions sent by programs on this computer
// (scripts, the documentation's screenshot capture, and later AI agents) are performed here, in
// the open window, where the user sees them; what they change goes into the project's history like
// any edit, and can be undone. Each action returns { ok, message, data }. The program lists the
// same actions with their arguments (actions.go, GET /api/remote/tools); a Go test checks the two
// lists agree. Adapted from SituCraft 0.9.0 web/ui/remote.js.

import { closeDialogs, toast } from './overlays.js';
import { clonotypePresets } from './storage.js';
import { showMethods } from './methods.js';
import { datasetDefinition, sameMeaning, useDefinition } from '../lib/definitions.js';
import { datasetFilter } from '../lib/filters.js';
import { datasetUnit } from '../lib/datasets.js';

export class ActionError extends Error {}

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

// The views by the names scripts use.
const VIEWS = { start: 'welcome', welcome: 'welcome', data: 'data', qc: 'qc', clonotypes: 'clonotypes', jobs: 'jobs' };

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

export function installRemote(app) {
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

    async open_data({ paths, name }) {
      if (!Array.isArray(paths) || !paths.length) throw new ActionError('open_data needs {"paths": ["/path/to/airr_rearrangement.tsv", …]}.');
      const summary = await app.openData(paths.map(String), { name: name ? String(name) : '', rethrow: true });
      if (!summary) throw new ActionError('The project changed while the files were read.');
      return { message: `Opened ${summary.name}: ${summary.samples.length} samples, ${summary.rows} rows.`, data: { dataset: summary.id, name: summary.name, unit: summary.unit || null, findings: { fatal: summary.report.fatal, warnings: summary.report.warnings, notes: summary.report.notes } } };
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
