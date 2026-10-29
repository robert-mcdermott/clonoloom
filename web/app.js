// ClonoLoom: application bootstrap, views, shortcuts, opening data and autosave. Adapted from
// SituCraft 0.9.0 web/app.js (clonoloom-spec/design.md, "The host, from the siblings").

import { h, icon, clear, debounce, isTyping, downloadBlob, modKey } from './ui/dom.js';
import { showMenu, showDialog, promptDialog, confirmDialog, toast, closeMenu } from './ui/overlays.js';
import { createStore } from './ui/store.js';
import { createLibrary, detectBackend, getDataset, locateFile, openDataset, prefs } from './ui/storage.js';
import { pickPaths } from './ui/file-picker.js';
import { REFERENCES, createExamples } from './ui/examples.js';
import { createJobs } from './ui/jobs.js';
import { showMethods } from './ui/methods.js';
import { exportDataset } from './ui/exports.js';
import { mapColumns } from './ui/mapping.js';
import { installRemote, remoteActions } from './ui/remote.js';
import { mountSidebar } from './ui/sidebar.js';
import { mountInspector } from './ui/inspector.js';
import { mountDrawer } from './ui/drawer.js';
import { openPalette } from './ui/palette.js';
import { colorVisionFriendly, setColorVisionFriendly } from './lib/colormaps.js';
import { createProject, isEmptyProject, rename, serializeProject } from './lib/project.js';
import { UNITS, addDataset, datasetFiles, datasetRecord, datasetUnit, relocate } from './lib/datasets.js';
import { datasetDefinition } from './lib/definitions.js';

import { parseExactProject as parseProject } from './lib/project-archives.js';
import { createParquetExports } from './ui/parquet.js';
import { createDeidentification } from './ui/deidentification.js';
import { createProjectArchives } from './ui/project-archives.js';

const VERSION = '0.8.0';

const toOpen = { label: 'Open data', icon: 'folder', run: (app) => app.openPathDialog() };

// The views, in the order of the work: data in, check it, define clonotypes, then the questions.
// Each loads its module when first shown; views still to be built say what they are for
// (mode-planned.js) until the slice that builds them (clonoloom-spec/roadmap.md).
const MODES = [
  { id: 'welcome', label: 'Start', icon: 'grid', hidden: true, load: () => import('./ui/mode-welcome.js').then((m) => m.mountWelcome) },
  { id: 'data', label: 'Data', icon: 'database', load: () => import('./ui/mode-data.js').then((m) => m.mountData) },
  { id: 'qc', label: 'QC', icon: 'qc', load: () => import('./ui/mode-qc.js').then((m) => m.mountQC) },
  { id: 'clonotypes', label: 'Clonotypes', icon: 'clonotypes', load: () => import('./ui/mode-clonotypes.js').then((m) => m.mountClonotypes) },
  { id: 'repertoire', label: 'Repertoire', icon: 'repertoire', load: () => import('./ui/mode-repertoire.js').then((m) => m.mountRepertoire) },
  { id: 'tracking', label: 'Tracking', icon: 'threads', load: () => import('./ui/mode-tracking.js').then((m) => m.mountTracking) },
  { id: 'specificity', label: 'Specificity', icon: 'receptor', load: () => import('./ui/mode-specificity.js').then((m) => m.mountSpecificity) },
  { id: 'bcells', label: 'B cells', icon: 'lineage', load: () => import('./ui/mode-bcells.js').then((m) => m.mountBCells) },
  { id: 'cells', label: 'Cells', icon: 'cell', load: () => import('./ui/mode-cells.js').then((m) => m.mountCells) },
  'sep',
  { id: 'cohort', label: 'Cohort', icon: 'samples', load: () => import('./ui/mode-cohort.js').then((m) => m.mountCohort) },
  { id: 'reports', label: 'Reports', icon: 'report', load: () => import('./ui/mode-reports.js').then((m) => m.mountReports) },
  { id: 'jobs', label: 'Jobs', icon: 'jobs', load: () => import('./ui/mode-jobs.js').then((m) => m.mountJobs) },
];

// --- Theme ---------------------------------------------------------------------------------------

function applyTheme(preference) {
  const dark = preference === 'dark' || (preference === 'system' && matchMedia('(prefers-color-scheme: dark)').matches);
  document.documentElement.dataset.theme = dark ? 'dark' : 'light';
}

// Color-vision-friendly colors (a setting): the palettes (colormaps.js) and status colors.
function applyColorVision(on) {
  setColorVisionFriendly(on);
  if (on) document.documentElement.dataset.cvd = '';
  else delete document.documentElement.dataset.cvd;
}

// The kind of a file by its name: the same table as the program's (local.go, fileKind).
export function fileKind(name) {
  const lower = name.toLowerCase();
  const has = (...suffixes) => suffixes.some((s) => lower.endsWith(s));
  if (has('.clz')) return 'project';
  if (has('.tsv', '.tsv.gz', '.csv', '.csv.gz', '.txt', '.txt.gz', '.tab', '.tab.gz')) return 'table';
  if (has('.json', '.json.gz', '.yaml', '.yml')) return 'metadata';
  if (has('.h5ad', '.h5mu', '.h5')) return 'expression';
  if (has('.mtx', '.mtx.gz')) return 'matrix';
  if (has('.fasta', '.fa', '.fna', '.fasta.gz', '.fa.gz')) return 'fasta';
  if (has('.zip')) return 'archive';
  return '';
}

const KIND_NAMES = {
  cellranger: 'Cell Ranger outputs', project: 'project archives (.clz)', table: 'tables', metadata: 'metadata files',
  expression: 'expression files', matrix: 'matrices', fasta: 'FASTA files', archive: 'archives',
};

// Without its program (the page opened from a plain web server or a file), ClonoLoom cannot reach
// the data on this computer: say so and how to start it.
function showNeedsProgram() {
  document.getElementById('workbench').append(h('div.workbench-scroll', h('div.empty',
    icon('terminal'),
    h('h3', 'Start ClonoLoom with its program'),
    h('p', 'ClonoLoom reads repertoires where they are on this computer and computes every number with its program, which a web page alone cannot do. Run the clonoloom program (it opens this window itself), for example:'),
    h('pre.mono', 'clonoloom /path/to/airr_rearrangement.tsv'))));
}

async function start() {
  const themePref = prefs.get('theme', 'system');
  applyTheme(themePref);
  applyColorVision(prefs.get('colorVision', false));
  const info = await detectBackend();
  if (!info) {
    showNeedsProgram();
    return;
  }
  const library = createLibrary(info);

  // ClonoLoom starts on the start page with a new, empty project: the last one is a click away in
  // its recent projects.
  const store = createStore(createProject());
  store.state.ui.theme = themePref;
  store.state.ui.drawer = { open: prefs.get('drawerOpen', false), tab: prefs.get('drawerTab', 'history') };
  const app = { store, library, info, version: VERSION, commit: info.commit ?? '' };
  // What was handed to the program (command line, a later launch, Open data): local.go's list.
  app.localItems = info.files ?? [];
  // Readers of files opened in the browser by kind, registered by the slices that build them:
  // kind → async (item).
  app.importers = new Map();
  app.archives = createProjectArchives(app);
  app.deidentification = createDeidentification(app);
  app.parquet = createParquetExports(app);
  app.importers.set('project', async ({file}) => { await app.setMode('reports'); await app.archives.inspect(file); });

  app.sidebar = mountSidebar(app);
  app.inspector = mountInspector(app);
  app.drawer = mountDrawer(app);
  app.log = (message) => app.drawer.log(message);

  // --- Views -------------------------------------------------------------------------------------

  const workbench = document.getElementById('workbench');
  const rail = document.getElementById('view-rail');
  let current = null;
  let currentId = null;
  let mounting = 0;

  function renderRail() {
    clear(rail);
    for (const mode of MODES) {
      if (mode === 'sep') {
        rail.append(h('span.rail-sep', { role: 'presentation', 'aria-hidden': 'true' }));
        continue;
      }
      if (mode.hidden) continue;
      const active = store.ui.mode === mode.id;
      rail.append(h(`button.rail-tab${active ? '.active' : ''}`, { type: 'button', role: 'tab', 'aria-selected': active ? 'true' : 'false', title: mode.label, onclick: () => app.setMode(mode.id) }, icon(mode.icon), h('span', mode.label)));
    }
  }

  app.setMode = async (id) => {
    if (id === currentId && current) return;
    const mode = MODES.find((m) => m !== 'sep' && m.id === id) ?? MODES[0];
    const token = ++mounting;
    store.setUI({ mode: mode.id }, ['mode']);
    document.getElementById('app').dataset.mode = mode.id;
    renderRail();
    current?.destroy();
    current = null;
    currentId = mode.id;
    clear(workbench);
    let mount;
    try {
      mount = await mode.load();
    } catch (error) {
      if (token !== mounting) return;
      workbench.append(h('div.workbench-scroll', h('div.empty', icon(mode.icon), h('h3', `${mode.label} is not available in this build`), h('p', error.message))));
      return;
    }
    if (token !== mounting) return;
    current = mount(app, workbench);
    current.update?.(new Set(['project', 'focus', 'selection', 'mode']));
  };

  app.focusItem = (focus) => {
    store.setUI({ focus }, ['focus']);
    // A result opens in its view.
    const result = focus?.kind === 'result' ? store.project.artifacts.find((a) => a.id === focus.id) : null;
    if (['cohort-features', 'cohort-compare', 'definition-robustness', 'cohort-counts'].includes(result?.kind) && store.ui.mode !== 'cohort') app.setMode('cohort');
    if (['pseudobulk-de', 'pseudobulk', 'expression-map', 'phenotypes', 'modularity', 'conga', 'anndata-export'].includes(result?.kind) && store.ui.mode !== 'cells') app.setMode('cells');
    if (['families', 'germlines', 'shm', 'lineages', 'isotypes'].includes(result?.kind) && store.ui.mode !== 'bcells') app.setMode('bcells');
    if ((result?.kind === 'diversity' || result?.kind === 'rarefaction') && store.ui.mode !== 'repertoire') app.setMode('repertoire');
    if (['tracking', 'expansion', 'classes', 'attribution'].includes(result?.kind) && store.ui.mode !== 'tracking') app.setMode('tracking');
    if (['contamination', 'batch-qc'].includes(result?.kind) && store.ui.mode !== 'qc') app.setMode('qc');
    if (['convergence', 'motifs', 'tcrdist', 'pgen', 'neighborhoods', 'networks', 'specificity'].includes(result?.kind) && store.ui.mode !== 'specificity') app.setMode('specificity');
  };

  app.toggleDrawer = (open = !store.ui.drawer.open, tab = store.ui.drawer.tab) => {
    store.setUI({ drawer: { open, tab } }, ['drawer']);
    prefs.set('drawerOpen', open);
    prefs.set('drawerTab', tab);
  };

  // --- Data on this computer ---------------------------------------------------------------------

  // The program's list of what it was handed, after a later launch or Open data added to it.
  function setLocalItems(files) {
    const before = new Set(app.localItems.map((f) => f.url));
    app.localItems = files ?? [];
    const added = app.localItems.filter((f) => !before.has(f.url));
    for (const item of added) app.log(`Handed ${item.name} (${KIND_NAMES[item.kind] ?? item.kind}).`);
    if (added.length) store.notify(['local']);
    return added;
  }

  // Items named on the command line (or by a later launch) open as datasets: a Cell Ranger output
  // each, and the tables of one folder (or the files named together) as one.
  app.openLocalItems = async (items) => {
    const groups = new Map();
    for (const item of items) {
      if (item.kind === 'cellranger') groups.set(item.path, [item]);
      else if (item.kind === 'table' || item.kind === 'metadata') {
        const key = item.folder ? `folder:${item.folder}` : 'named';
        groups.set(key, [...(groups.get(key) ?? []), item]);
      }
    }
    for (const item of items.filter((item) => item.kind === 'project')) { await app.setMode('reports'); await app.archives.inspect(item.path); }
    for (const group of groups.values()) await app.openData(group.map((item) => item.path));
  };

  // A browser does not tell a page where a file or folder is, so data are chosen in ClonoLoom's own
  // dialog of this computer's folders (file-picker.js) or named on the clonoloom command line; the
  // program reads them where they are.
  app.openPathDialog = async () => {
    const paths = await pickPaths({
      title: 'Open data',
      confirm: 'Open',
      choose: 'open',
      multiple: true,
      hint: 'AIRR rearrangement tables, a Cell Ranger V(D)J output folder, immunoSEQ exports, or a folder of a cohort\'s files: what you choose together opens as one dataset. ClonoLoom reads it where it is and never changes it.',
    });
    if (paths?.length) await app.openData(paths);
  };

  // The program's background jobs (ui/jobs.js), and public example data and the reference data
  // the analyses use, downloaded when asked (ui/examples.js).
  app.jobs = createJobs(app);
  app.examples = createExamples(app);
  app.showExamples = () => app.examples.dialog();
  app.references = createExamples(app, REFERENCES);
  app.showReferences = () => app.references.dialog();

  // --- Datasets ----------------------------------------------------------------------------------

  // What the program says of each dataset of the project this session (id → { status, summary,
  // error }): open, opening, missing (a file is not where it was), changed (a file's contents
  // differ from the project's record) or failed.
  app.datasetStates = new Map();
  const setState = (id, state) => {
    app.datasetStates.set(id, state);
    store.notify(['datasets']);
  };

  // Opens files as one dataset and records it in the project.
  // rethrow: an error is thrown to the caller (remote control) as well as shown.
  app.openData = async (paths, { name = '', library = null, mappings = null, rethrow = false } = {}) => {
    const sameProject = store.sameProject();
    const busy = `open-${Date.now()}`;
    store.setBusy(busy, `Reading ${library ? `${library.length} file${library.length === 1 ? '' : 's'}` : paths.length === 1 ? paths[0].split(/[\\/]/).pop() : `${paths.length} files`}…`);
    try {
      let summary;
      try {
        summary = await openDataset(library ? { library, name } : { paths, name, ...(mappings ? { mappings } : {}) });
      } catch (error) {
        // One table ClonoLoom does not recognize: its columns mapped, if the user maps them.
        const one = library ? library.length === 1 : paths.length === 1;
        if (error.code !== 'format' || !one || rethrow) throw error;
        store.setBusy(busy, null);
        const mapping = await mapColumns(library ? { sha256: library[0].sha256, name: library[0].name } : { path: paths[0] });
        if (!mapping) return null;
        store.setBusy(busy, 'Reading through the mapping…');
        summary = await openDataset(library ? { library: [{ ...library[0], mapping }], name } : { paths, name, mappings: { [paths[0]]: mapping } });
      }
      if (!sameProject()) return null;
      app.datasetStates.set(summary.id, { status: 'open', summary });
      store.commit(addDataset(store.project, datasetRecord(summary)), `Open ${summary.name}`, ['datasets']);
      const r = summary.report;
      app.log(`Opened ${summary.name}: ${summary.samples.length} sample${summary.samples.length === 1 ? '' : 's'}, ${summary.rows.toLocaleString('en-US')} rows; ${r.fatal} fatal, ${r.warnings} warning${r.warnings === 1 ? '' : 's'}, ${r.notes} note${r.notes === 1 ? '' : 's'}.`);
      toast(r.fatal ? `${summary.name} has ${r.fatal} fatal finding${r.fatal === 1 ? '' : 's'}: see the Data view.` : `Opened ${summary.name}: ${summary.samples.length} sample${summary.samples.length === 1 ? '' : 's'}.`, { kind: r.fatal ? 'error' : 'ok', timeout: 3500 });
      app.focusItem({ kind: 'dataset', id: summary.id });
      if (store.ui.mode !== 'data') app.setMode('data');
      return summary;
    } catch (error) {
      toast(error.message, { kind: 'error', timeout: 6000 });
      app.log(error.message);
      if (rethrow) throw error;
      return null;
    } finally {
      store.setBusy(busy, null);
    }
  };

  // Makes sure the program has a dataset of the project open, opening it again from its files
  // when it does not (after a restart). Answers its state.
  app.ensureDataset = async (record) => {
    const known = app.datasetStates.get(record.id);
    if (known && known.status !== 'failed') return known.promise ?? known;
    const promise = (async () => {
      try {
        let summary = await getDataset(record.id);
        if (!summary) {
          if (datasetFiles(record).some((s) => s.archiveStatus && !['included', 'rebound'].includes(s.archiveStatus))) throw Object.assign(new Error('This archive omitted some source files. Locate the original files by their recorded fingerprints before reopening.'), {code:'missing'});
          summary = await openDataset({ inputs: datasetFiles(record), name: record.name });
        }
        if (summary.id !== record.id) {
          const state = { status: 'changed', summary, error: 'The files are not those the project recorded: their contents changed.' };
          setState(record.id, state);
          return state;
        }
        const state = { status: 'open', summary };
        setState(record.id, state);
        return state;
      } catch (error) {
        const state = { status: error.code === 'missing' ? 'missing' : 'failed', error: error.message };
        setState(record.id, state);
        return state;
      }
    })();
    app.datasetStates.set(record.id, { status: 'opening', promise });
    store.notify(['datasets']);
    return promise;
  };

  app.ensureDatasets = () => Promise.all(store.project.datasets.map((d) => app.ensureDataset(d)));

  // A dataset whose files moved: looked for near where they were (and near the other datasets'
  // files), each by its fingerprint; else chosen in the Open dialog and checked.
  app.locateDataset = async (record) => {
    const dirOf = (p) => p.replace(/[\\/][^\\/]*$/, '');
    const near = [...new Set([...datasetFiles(record), ...store.project.datasets.flatMap((d) => datasetFiles(d))].flatMap((s) => {
      const dir = dirOf(s.path);
      return [dir, dirOf(dir), dirOf(dirOf(dir))];
    }))].filter(Boolean);
    const found = [];
    for (const source of datasetFiles(record)) {
      let path = await locateFile({ name: source.name, size: source.size, sha256: source.sha256, near });
      if (!path) {
        const picked = await pickPaths({ title: `Where is ${source.name}?`, confirm: 'Use this file', choose: 'file', start: dirOf(source.path), hint: `The project recorded ${source.name} at ${source.path}. Choose where it is now; ClonoLoom checks it is the same file.` });
        if (!picked?.length) return false;
        path = picked[0];
      }
      found.push(path);
    }
    store.commit(relocate(store.project, record.id, found), `Locate ${record.name}`, ['datasets']);
    app.datasetStates.delete(record.id);
    const moved = store.project.datasets.find((d) => d.id === record.id);
    const state = await app.ensureDataset(moved);
    if (state.status !== 'open') toast(state.error ?? 'The files found are not the dataset\'s.', { kind: 'error' });
    return state.status === 'open';
  };

  // --- Files opened in the browser ---------------------------------------------------------------

  const fileInput = document.getElementById('file-input');
  app.pickFiles = (accept) => {
    fileInput.accept = accept ?? '';
    fileInput.value = '';
    fileInput.click();
  };
  fileInput.addEventListener('change', () => app.importFiles([...fileInput.files]));

  // Opens dropped or picked files by kind: tables and metadata are added to the library (a page
  // never learns a file's place) and opened together as one dataset; other kinds by the importers
  // the slices register.
  app.importFiles = async (files) => {
    const unread = new Map();
    const unknown = [];
    const tables = files.filter((f) => ['table', 'metadata'].includes(fileKind(f.name)));
    if (tables.length) {
      const library = [];
      for (const file of tables) {
        try {
          const added = await app.library.addFile(new Uint8Array(await file.arrayBuffer()));
          library.push({ sha256: added.sha256, name: file.name });
        } catch (error) {
          toast(`${file.name}: ${error.message}`, { kind: 'error' });
        }
      }
      if (library.length) await app.openData([], { library, name: tables.length > 1 ? (tables[0].folder ?? '') : '' });
    }
    for (const file of files) {
      const kind = fileKind(file.name);
      if (kind === 'table' || kind === 'metadata') continue;
      const importer = app.importers.get(kind);
      if (!importer) {
        if (kind) unread.set(kind, (unread.get(kind) ?? 0) + 1);
        else unknown.push(file.name);
        continue;
      }
      try {
        await importer({ file, name: file.name, folder: file.folder ?? null });
      } catch (error) {
        toast(`${file.name}: ${error.message}`, { kind: 'error' });
        app.log(`${file.name}: ${error.message}`);
      }
    }
    const messages = [];
    if (unread.size) messages.push(`This build of ClonoLoom cannot read ${[...unread.keys()].map((k) => KIND_NAMES[k] ?? k).join(', ')} yet.`);
    if (unknown.length) messages.push(`${unknown.length === 1 ? unknown[0] : `${unknown.length} files`}: ClonoLoom opens rearrangement tables (.tsv, .csv, gzipped or not), AIRR Repertoire metadata (.json), Cell Ranger output folders and projects (.clz).`);
    for (const message of messages) {
      toast(message, { kind: 'error' });
      app.log(message);
    }
  };

  // --- Projects ----------------------------------------------------------------------------------

  async function loadProject(doc) {
    store.reset(doc);
    prefs.set('lastProject', doc.id);
    store.setUI({ focus: null, selection: null }, ['focus', 'selection']);
    app.setMode(isEmptyProject(doc) ? 'welcome' : 'data');
    updateTitle();
    // The project's datasets, opened again from their files in the background.
    app.ensureDatasets();
  }
  app.loadProject = loadProject;

  app.openProject = async (id) => {
    try {
      await saveNow();
      await loadProject(parseProject(await library.loadProject(id)));
    } catch (error) {
      toast(error.message, { kind: 'error' });
    }
  };

  const saveState = document.getElementById('save-state');
  // The save under way, if any. A save asked for meanwhile (opening another project) waits for
  // it, then saves what changed since: returning at once would lose those edits.
  let saving = null;
  async function writeProject(project) {
    saveState.className = 'save-state saving';
    saveState.setAttribute('aria-label', 'Saving');
    try {
      await library.saveProject(project.id, serializeProject(project));
      prefs.set('lastProject', project.id);
      // Another project may have been opened while this one was written.
      if (store.project.id === project.id) store.markSaved(project);
      saveState.className = 'save-state';
      saveState.title = `Saved to ${library.location}`;
      saveState.setAttribute('aria-label', 'Saved');
    } catch (error) {
      saveState.className = 'save-state error';
      saveState.title = `Not saved: ${error.message}`;
      saveState.setAttribute('aria-label', 'Not saved');
    }
  }
  async function saveNow() {
    while (saving) await saving;
    const { project } = store;
    if (!store.isDirty() || isEmptyProject(project)) return;
    saving = writeProject(project);
    try {
      await saving;
    } finally {
      saving = null;
    }
    if (store.isDirty()) autosave();
  }
  app.saveNow = saveNow;
  const autosave = debounce(saveNow, 1200);

  app.newProject = async () => {
    await saveNow();
    await loadProject(createProject());
  };

  async function openLibraryDialog() {
    let list = [];
    try {
      list = await library.listProjects();
    } catch (error) {
      toast(error.message, { kind: 'error' });
      return;
    }
    const body = h('tbody');
    const dialog = showDialog({
      title: 'Open a project',
      width: 'wide',
      content: [h('p', `Projects in ${library.location}.`), h('div', { style: { maxHeight: '60vh', overflow: 'auto' } }, h('table.data', h('thead', h('tr', h('th', 'Name'), h('th.r', 'Datasets'), h('th', 'Modified'), h('th', h('span.sr-only', 'Delete')))), body))],
    });
    for (const item of list) {
      body.append(h(`tr${item.id === store.project.id ? '.selected' : ''}`, { style: { cursor: 'pointer' }, onclick: () => { dialog.close(); app.openProject(item.id); } },
        h('td', item.name || 'Untitled'), h('td.r', String(item.datasets ?? '')), h('td', new Date(item.modified).toLocaleString()),
        h('td.r', h('button.icon-button.small', {
          type: 'button',
          title: 'Delete',
          'aria-label': `Delete ${item.name || 'Untitled'}`,
          onclick: async (event) => {
            event.stopPropagation();
            if (!(await confirmDialog({ title: 'Delete project?', message: `Delete "${item.name}"? It is moved to the library's trash folder. Its data files are not touched.`, confirm: 'Delete', danger: true }))) return;
            try {
              await library.deleteProject(item.id);
              event.target.closest('tr').remove();
              store.notify(['library']);
            } catch (error) {
              toast(error.message, { kind: 'error' });
            }
          },
        }, icon('trash')))));
    }
    if (!list.length) body.append(h('tr', h('td', { colSpan: 4 }, h('span.muted', 'No saved projects yet.'))));
  }
  app.openLibrary = openLibraryDialog;

  // The plain project document remains available beside portable .clz archives.
  function exportProjectJSON() {
    downloadBlob(new Blob([serializeProject(store.project)], { type: 'application/json' }), `${store.project.name.replace(/[^\w.-]+/g, '_') || 'project'}.clonoloom.json`);
  }

  document.getElementById('project-menu').addEventListener('click', (event) => {
    showMenu(event.currentTarget, [
      { label: 'New project', icon: 'plus', onSelect: () => app.newProject() },
      { label: 'Open…', icon: 'library', hint: `${modKey}⇧O`, onSelect: openLibraryDialog },
      { label: 'Rename…', icon: 'edit', onSelect: async () => {
        const name = await promptDialog({ title: 'Rename project', label: 'Name', value: store.project.name });
        if (name) store.commit(rename(store.project, name), 'Rename project');
      } },
      { label: 'Save now', icon: 'save', hint: `${modKey}S`, onSelect: async () => { await saveNow(); toast('Saved.', { kind: 'ok' }); } },
      '-',
      { section: 'Open' },
      { label: 'Data…', icon: 'folder', hint: `${modKey}O`, onSelect: () => app.openPathDialog() },
      { label: 'Files…', icon: 'file', onSelect: () => app.pickFiles() },
      { label: 'Example data…', icon: 'flask', onSelect: () => app.showExamples() },
      '-',
      { section: 'Export' },
      { label: 'Portable project archive (.clz)', icon: 'download', onSelect: () => app.setMode('reports') },
      { label: 'Project document (JSON)', icon: 'download', onSelect: exportProjectJSON },
      { label: 'Clonotypes of the dataset in focus (CSV)', icon: 'download', onSelect: () => exportFocused('clonotypes') },
      { label: 'Its rearrangements (AIRR TSV)', icon: 'download', onSelect: () => exportFocused('rearrangements') },
      '-',
      { label: 'Start page', icon: 'grid', onSelect: () => app.setMode('welcome') },
    ]);
  });

  // The dataset in focus's clonotypes or rearrangements, exported (ui/exports.js).
  function exportFocused(what) {
    const d = store.project.datasets.find((x) => store.ui.focus?.kind === 'dataset' && x.id === store.ui.focus.id) ?? store.project.datasets[0];
    if (d) exportDataset(app, d, what);
    else toast('Open data first.', { kind: 'error' });
  }

  // --- Commands (palette) ------------------------------------------------------------------------

  app.commands = () => [
    ...MODES.filter((m) => m !== 'sep').map((m) => ({ label: `Go to ${m.label}`, icon: m.icon, run: () => app.setMode(m.id), keywords: m.id })),
    { label: 'Open data', icon: 'folder', hint: `${modKey}O`, run: () => app.openPathDialog(), keywords: 'airr cell ranger immunoseq folder path import tsv csv' },
    { label: 'Open files', icon: 'file', run: () => app.pickFiles() },
    { label: 'Example data', icon: 'flask', run: () => app.showExamples(), keywords: 'download public 10x pbmc immunoseq emerson airr try sample demo' },
    { label: 'Methods paragraph of the dataset in focus', icon: 'file', run: () => {
      const d = store.project.datasets.find((x) => store.ui.focus?.kind === 'dataset' && x.id === store.ui.focus.id) ?? store.project.datasets[0];
      if (d) showMethods(app, d);
      else toast('Open data first: the methods paragraph is written from a dataset\'s record.', { kind: 'error' });
    }, keywords: 'methods text manuscript references cite citation' },
    { label: 'New project', icon: 'plus', run: () => app.newProject() },
    { label: 'Open a saved project', icon: 'library', hint: `${modKey}⇧O`, run: openLibraryDialog },
    { label: 'Save project now', icon: 'save', hint: `${modKey}S`, run: saveNow },
    { label: 'Export the project document', icon: 'download', run: exportProjectJSON },
    { label: 'Export the clonotypes of the dataset in focus (CSV)', icon: 'download', run: () => exportFocused('clonotypes'), keywords: 'csv table download' },
    { label: 'Export the rearrangements of the dataset in focus (AIRR TSV)', icon: 'download', run: () => exportFocused('rearrangements'), keywords: 'airr tsv download' },
    { label: 'Show or hide the drawer (history, log)', icon: 'drawer', hint: `${modKey}J`, run: () => app.toggleDrawer(), keywords: 'history log undo' },
    { label: 'Toggle dark theme', icon: 'moon', run: () => toggleTheme() },
    { label: 'Color-vision-friendly colors (on or off)', icon: 'eye', run: () => toggleColorVision(), keywords: 'color blind accessibility deuteranopia protanopia palette' },
    { label: 'Keyboard shortcuts', icon: 'keyboard', hint: '?', run: showHelp },
  ];

  // --- Top bar -----------------------------------------------------------------------------------

  const undoButton = document.getElementById('undo-button');
  const redoButton = document.getElementById('redo-button');
  const themeButton = document.getElementById('theme-button');
  undoButton.append(icon('undo'));
  redoButton.append(icon('redo'));
  document.getElementById('help-button').append(icon('help'));
  document.getElementById('help-button').addEventListener('click', showHelp);
  document.getElementById('command-button').addEventListener('click', () => openPalette(app));
  undoButton.addEventListener('click', () => doUndo());
  redoButton.addEventListener('click', () => doRedo());
  themeButton.addEventListener('click', () => showMenu(themeButton, [
    { section: 'Appearance' },
    ...[['light', 'Light', 'sun'], ['dark', 'Dark', 'moon'], ['system', 'Match the system', 'settings']].map(([id, label, glyph]) => ({ label, icon: glyph, checked: prefs.get('theme', 'system') === id, onSelect: () => setTheme(id) })),
    '-',
    { label: 'Color-vision-friendly colors', icon: 'eye', checked: colorVisionFriendly(), onSelect: () => toggleColorVision() },
  ]));

  function doUndo() {
    const label = store.undo();
    if (label) toast(`Undid: ${label}`, { timeout: 1800 });
  }
  function doRedo() {
    const label = store.redo();
    if (label) toast(`Redid: ${label}`, { timeout: 1800 });
  }

  function toggleTheme() {
    setTheme(document.documentElement.dataset.theme === 'dark' ? 'light' : 'dark');
  }
  function toggleColorVision() {
    const next = !colorVisionFriendly();
    prefs.set('colorVision', next);
    applyColorVision(next);
    store.notify(['theme', 'colors']);
    toast(next ? 'Color-vision-friendly colors: palettes and status colors stay distinguishable with red–green and blue–yellow color blindness.' : 'Default colors.', { timeout: 3500 });
  }
  app.setTheme = (next) => setTheme(next);
  function setTheme(next) {
    prefs.set('theme', next);
    store.state.ui.theme = next;
    applyTheme(next);
    renderTheme();
    store.notify(['theme']);
  }
  function renderTheme() {
    clear(themeButton).append(icon(document.documentElement.dataset.theme === 'dark' ? 'sun' : 'moon'));
  }
  renderTheme();
  matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
    if (prefs.get('theme', 'system') === 'system') {
      applyTheme('system');
      renderTheme();
      store.notify(['theme']);
    }
  });

  function updateTitle() {
    const { project } = store;
    document.getElementById('project-name').textContent = project.name;
    document.title = `${project.name} · ClonoLoom`;
    undoButton.disabled = !store.canUndo();
    redoButton.disabled = !store.canRedo();
    undoButton.title = store.canUndo() ? `Undo ${store.undoLabel()} (${modKey}Z)` : 'Nothing to undo';
    redoButton.title = store.canRedo() ? `Redo ${store.redoLabel()} (⇧${modKey}Z)` : 'Nothing to redo';
    if (!saveState.classList.contains('error') && !saveState.classList.contains('saving')) {
      const dirty = store.isDirty() && !isEmptyProject(project);
      saveState.className = `save-state${dirty ? ' dirty' : ''}`;
      saveState.setAttribute('aria-label', dirty ? 'Unsaved changes' : 'Saved');
    }
  }

  // --- Status bar --------------------------------------------------------------------------------

  const statusbar = document.getElementById('statusbar');
  function renderStatus() {
    clear(statusbar);
    const busy = [...store.state.busy.values()];
    const { project } = store;
    const version = `ClonoLoom ${VERSION}${app.commit ? ` (${app.commit.slice(0, 7)})` : ''}`;
    statusbar.append(
      h('span.item', h(`span.dot${busy.length ? '.busy' : ''}`), busy.length ? busy[0] : 'Ready'),
      h('span.item', icon('library'), `Library: ${library.location}`),
      h('span.item', `${project.datasets.length} dataset${project.datasets.length === 1 ? '' : 's'}`),
      definitionStatus(),
      h('button.item.statusbar-link', { type: 'button', title: `History and log (${modKey}J)`, onclick: () => app.toggleDrawer() }, icon('drawer'), store.ui.drawer.open ? 'Hide the drawer' : 'History and log'),
      h('span.spacer'),
      h('span.item', version),
      h('span.item.muted', 'Research use only'));
  }

  // The clonotype definition and abundance unit of the dataset in focus (or the first): stated
  // wherever clonotypes are counted (requirement C2).
  function definitionStatus() {
    const { project, ui } = store;
    const d = project.datasets.find((x) => ui.focus?.kind === 'dataset' && x.id === ui.focus.id)
      ?? project.datasets.find((x) => x.id === app.clonotypeContext?.dataset) ?? project.datasets[0];
    if (!d) return null;
    const loci = Object.keys(app.datasetStates.get(d.id)?.summary?.samples?.reduce((acc, s) => Object.assign(acc, s.loci), {}) ?? {});
    const def = datasetDefinition(d, loci);
    const unit = d.singleCell ? 'cells' : datasetUnit(d);
    return h('span.item', { title: `${d.name}: clonotypes by ${def.name}; abundance in ${UNITS[unit] ?? 'counts'}` }, icon('clonotypes'), `${def.name} · ${UNITS[unit] ?? 'counts'}`);
  }

  function showHelp() {
    const rows = [
      ['Search and commands', `${modKey}K`],
      ['Undo / redo', `${modKey}Z / ⇧${modKey}Z`],
      ['Open data', `${modKey}O`],
      ['Open a saved project', `⇧${modKey}O`],
      ['Save now', `${modKey}S`],
      ['Go to a view', `${modKey}1 … ${modKey}9`],
      ['Toggle project tree / inspector', `${modKey}\\ / ${modKey}⇧\\`],
      ['Drawer (history, log)', `${modKey}J`],
      ['Close a menu or dialog', 'Esc'],
    ];
    showDialog({
      title: 'Keyboard shortcuts',
      content: h('table.data', h('tbody', ...rows.map(([action, keys]) => h('tr', h('td', action), h('td.r', h('kbd', keys)))))),
    });
  }

  // --- Keyboard ----------------------------------------------------------------------------------

  document.addEventListener('keydown', (event) => {
    const mod = event.metaKey || event.ctrlKey;
    const key = event.key.toLowerCase();
    if (mod && key === 'k') {
      event.preventDefault();
      openPalette(app);
      return;
    }
    if (isTyping(event)) return;
    if (mod && key === 'z') {
      event.preventDefault();
      if (event.shiftKey) doRedo();
      else doUndo();
      return;
    }
    if (mod && key === 'y') {
      event.preventDefault();
      doRedo();
      return;
    }
    if (mod && key === 's') {
      event.preventDefault();
      saveNow().then(() => toast('Saved.', { kind: 'ok', timeout: 1500 }));
      return;
    }
    if (mod && key === 'o') {
      event.preventDefault();
      if (event.shiftKey) openLibraryDialog();
      else app.openPathDialog();
      return;
    }
    if (mod && key === 'j') {
      event.preventDefault();
      app.toggleDrawer();
      return;
    }
    if (mod && event.key === '\\') {
      event.preventDefault();
      document.getElementById('app').classList.toggle(event.shiftKey ? 'inspector-closed' : 'sidebar-closed');
      store.notify(['layout']);
      return;
    }
    if (mod && /^[1-9]$/.test(event.key)) {
      const modes = MODES.filter((m) => m !== 'sep' && !m.hidden);
      const mode = modes[Number(event.key) - 1];
      if (mode) {
        event.preventDefault();
        app.setMode(mode.id);
      }
      return;
    }
    if (mod || event.altKey) return;
    if (event.key === '?') {
      showHelp();
      return;
    }
    if (event.key === 'Escape') closeMenu();
  });

  // --- Drag and drop -----------------------------------------------------------------------------

  const dropOverlay = document.getElementById('drop-overlay');
  let dragDepth = 0;
  const hasFiles = (event) => [...(event.dataTransfer?.types ?? [])].includes('Files');
  window.addEventListener('dragenter', (event) => {
    if (!hasFiles(event)) return;
    dragDepth += 1;
    dropOverlay.hidden = false;
  });
  window.addEventListener('dragleave', (event) => {
    if (!hasFiles(event)) return;
    dragDepth = Math.max(0, dragDepth - 1);
    if (!dragDepth) dropOverlay.hidden = true;
  });
  window.addEventListener('dragover', (event) => {
    if (hasFiles(event)) event.preventDefault();
  });
  window.addEventListener('drop', async (event) => {
    if (!hasFiles(event)) return;
    event.preventDefault();
    dragDepth = 0;
    dropOverlay.hidden = true;
    app.importFiles(await collectDropped(event.dataTransfer));
  });

  // Walks dropped folders one level deep (Chromium and Safari support webkitGetAsEntry), without
  // reading a large folder.
  async function collectDropped(transfer) {
    const entries = [...transfer.items].map((item) => item.webkitGetAsEntry?.()).filter(Boolean);
    if (!entries.length) return [...transfer.files];
    const out = [];
    for (const entry of entries) {
      if (entry.isFile) {
        out.push(await new Promise((resolve, reject) => entry.file(resolve, reject)));
      } else if (entry.isDirectory) {
        const batch = await new Promise((resolve, reject) => entry.createReader().readEntries(resolve, reject));
        for (const child of batch) {
          if (!child.isFile || child.name.startsWith('.')) continue;
          const file = await new Promise((resolve, reject) => child.file(resolve, reject));
          Object.defineProperty(file, 'folder', { value: entry.name });
          out.push(file);
        }
      }
    }
    return out;
  }

  // --- Store subscription ------------------------------------------------------------------------

  store.subscribe((topics) => {
    if (topics.has('project') || topics.has('history') || topics.has('saved')) updateTitle();
    // (The status bar is drawn again on every change, the definition's included.)
    if (topics.has('project')) autosave();
    app.sidebar.update(topics);
    app.inspector.update(topics);
    app.drawer.update(topics);
    current?.update?.(topics);
    renderStatus();
  });

  window.addEventListener('beforeunload', (event) => {
    if (store.isDirty() && !isEmptyProject(store.project)) {
      saveNow();
      event.preventDefault();
    }
  });
  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'hidden') saveNow();
  });

  // --- Start -------------------------------------------------------------------------------------

  renderRail();
  updateTitle();
  renderStatus();
  app.sidebar.render();
  app.inspector.render();
  app.drawer.render();
  await app.setMode('welcome');
  if (app.localItems.length) {
    app.log(`${app.localItems.length} item${app.localItems.length === 1 ? '' : 's'} named on the command line.`);
    app.openLocalItems(app.localItems);
  }

  // A second "clonoloom <paths>" launch hands its paths to the program and brings this window
  // forward, so checking on focus (and now and then) is enough.
  let checking = false;
  const check = async () => {
    if (checking || document.visibilityState !== 'visible') return;
    checking = true;
    try {
      const fresh = await detectBackend();
      if (fresh) {
        const added = setLocalItems(fresh.files);
        if (added.length) app.openLocalItems(added);
      }
    } finally {
      checking = false;
    }
  };
  window.addEventListener('focus', check);
  setInterval(check, 30000);
  window.clonoloom = app;
  // Programs on this computer may drive the window (clonoloom --remote-control).
  // The actions scripts take, which the examples' guided steps take too.
  app.actions = remoteActions(app);
  if (info.remoteControl) app.remote = installRemote(app);
}

start().catch((error) => {
  console.error(error);
  document.getElementById('workbench').append(h('div.empty', h('h3', 'ClonoLoom could not start'), h('p', error.message)));
});
