// ClonoLoom: application bootstrap, views, shortcuts, opening data and autosave. Adapted from
// SituCraft 0.9.0 web/app.js (clonoloom-spec/design.md, "The host, from the siblings").

import { h, icon, clear, debounce, isTyping, downloadBlob, modKey } from './ui/dom.js';
import { showMenu, showDialog, promptDialog, confirmDialog, toast, closeMenu } from './ui/overlays.js';
import { createStore } from './ui/store.js';
import { createLibrary, detectBackend, openPaths, prefs } from './ui/storage.js';
import { mountSidebar } from './ui/sidebar.js';
import { mountInspector } from './ui/inspector.js';
import { mountDrawer } from './ui/drawer.js';
import { plannedMode } from './ui/mode-planned.js';
import { openPalette } from './ui/palette.js';
import { colorVisionFriendly, setColorVisionFriendly } from './lib/colormaps.js';
import { createProject, isEmptyProject, parseProject, rename, serializeProject } from './lib/project.js';

const VERSION = '0.1.0';

const toOpen = { label: 'Open data', icon: 'folder', run: (app) => app.openPathDialog() };

// The views, in the order of the work: data in, check it, define clonotypes, then the questions.
// Each loads its module when first shown; views still to be built say what they are for
// (mode-planned.js) until the slice that builds them (clonoloom-spec/roadmap.md).
const MODES = [
  { id: 'welcome', label: 'Start', icon: 'grid', hidden: true, load: () => import('./ui/mode-welcome.js').then((m) => m.mountWelcome) },
  {
    id: 'data', label: 'Data', icon: 'database',
    load: async () => plannedMode({
      title: 'Data', icon: 'database',
      purpose: 'What each file holds and whether to trust it: the format and its version, the samples in it, the abundance unit (reads, UMIs, templates or cells), and a validation report that keeps fatal errors, warnings and notes apart, each with its count, example rows and a remedy. Samples are annotated here: subject, timepoint, tissue, cell subset, condition.',
      steps: ['Open AIRR rearrangement tables, Cell Ranger V(D)J outputs or immunoSEQ exports.', 'Read the report; ClonoLoom never changes your files.', 'Annotate the samples, then go on to QC.'],
      actions: [toOpen],
    }),
  },
  {
    id: 'qc', label: 'QC', icon: 'qc',
    load: async () => plannedMode({
      title: 'Quality control', icon: 'qc',
      purpose: 'Whether each sample supports analysis: depth in its unit, productive and out-of-frame shares, junction lengths, gene resolution, singletons and coverage; for single cells, chains per cell and pairing. Filters are kept as named versions with their reasons and never delete data.',
      steps: ['Open data.', 'Compare the samples\' metrics and distributions.', 'Save a filter, and see what it keeps.'],
      actions: [toOpen],
    }),
  },
  {
    id: 'clonotypes', label: 'Clonotypes', icon: 'clonotypes',
    load: async () => plannedMode({
      title: 'Clonotypes', icon: 'clonotypes',
      purpose: 'Clonotypes under a definition you choose and can see: nucleotide or amino-acid junction, V and J genes or not, one chain or paired chains, and rules for cells with extra chains. The table lists each clonotype\'s abundance in every sample; the inspector shows its sequences, genes, cells and samples.',
      steps: ['Open and check data.', 'Choose or edit a clonotype definition.', 'Sort, filter and inspect the clonotypes.'],
    }),
  },
  {
    id: 'repertoire', label: 'Repertoire', icon: 'repertoire',
    load: async () => plannedMode({
      title: 'Repertoire', icon: 'repertoire',
      purpose: 'How diverse each repertoire is and how repertoires overlap: Hill numbers and their profile, richness estimators, clonality and evenness with intervals; rarefaction and extrapolation to compare samples at equal depth or coverage; gene usage, junction lengths and shared clonotypes.',
      steps: ['Define clonotypes.', 'Choose the samples and the depth or coverage to compare at.', 'Read each statistic with its interval and its plain-language explanation.'],
    }),
  },
  {
    id: 'tracking', label: 'Tracking', icon: 'threads',
    load: async () => plannedMode({
      title: 'Tracking', icon: 'threads',
      purpose: 'Clonotypes followed across a subject\'s samples, each drawn as a thread through time: which persist, expand or contract, tested with methods that account for sampling and biological variance, and how much of each sample comes from a reference such as an infusion product or a donor.',
      steps: ['Annotate subjects and timepoints.', 'Choose the clonotypes to follow.', 'Test expansion with the false discovery rate stated.'],
    }),
  },
  {
    id: 'specificity', label: 'Specificity', icon: 'receptor',
    load: async () => plannedMode({
      title: 'Specificity', icon: 'receptor',
      purpose: 'Receptors grouped by likely shared specificity: TCRdist neighborhoods against a background, specificity groups, networks of similar receptors, and matches to receptors of known specificity in public databases, with their evidence. Similarity suggests specificity; it does not prove it.',
      steps: ['Define clonotypes.', 'Compute distances and neighborhoods.', 'Inspect groups and their database matches.'],
    }),
  },
  {
    id: 'bcells', label: 'B cells', icon: 'lineage',
    load: async () => plannedMode({
      title: 'B cells', icon: 'lineage',
      purpose: 'Clonal families with a threshold chosen from the data, germlines reconstructed, somatic hypermutation by region, isotypes and class switching, and lineage trees with their ancestors.',
      steps: ['Open BCR data.', 'Cluster clonal families and check the threshold.', 'Explore mutations and trees.'],
    }),
  },
  {
    id: 'cells', label: 'Cells', icon: 'cell',
    load: async () => plannedMode({
      title: 'Cells', icon: 'cell',
      purpose: 'Single-cell receptors linked to gene expression: clonotypes on the cells\' map, phenotypes of expanded clonotypes, and whether receptor similarity goes with expression similarity.',
      steps: ['Open V(D)J with its gene expression (AnnData or Cell Ranger).', 'Color the map by clonotype or size.', 'Compare expanded and other cells by type, subject by subject.'],
    }),
  },
  'sep',
  {
    id: 'cohort', label: 'Cohort', icon: 'samples',
    load: async () => plannedMode({
      title: 'Cohort', icon: 'samples',
      purpose: 'Groups of subjects compared with the subject, not the sequence or the cell, as the unit: diversity, clonality, gene usage and specificity groups, with the test chosen from the design, effect sizes and intervals, and how the conclusion depends on the clonotype definition.',
      steps: ['Describe the design: subjects, groups, timepoints, pairing.', 'Choose the metrics to compare.', 'Read the results with their corrections recorded.'],
    }),
  },
  {
    id: 'reports', label: 'Reports', icon: 'report',
    load: async () => plannedMode({
      title: 'Reports', icon: 'report',
      purpose: 'Results others can check: figures with the clonotype definition and abundance unit in their legends, a methods paragraph written from what was actually done with numbered references, AIRR-compliant exports, and certificates that recompute every number.',
      steps: ['Analyze.', 'Choose the figures and results to report.', 'Export.'],
    }),
  },
  {
    id: 'jobs', label: 'Jobs', icon: 'jobs',
    load: async () => plannedMode({
      title: 'Jobs', icon: 'jobs',
      purpose: 'Work that runs in the background (distances, generation probabilities, lineage trees, downloads): progress, logs and cancellation. Jobs keep running when the window reloads, and their results are checked before they can be used.',
      steps: ['Start work that takes a while, in any view.', 'Follow it here, or keep working elsewhere.'],
    }),
  },
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
        rail.append(h('span.rail-sep', { role: 'separator' }));
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

  app.focusItem = (focus) => store.setUI({ focus }, ['focus']);

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
    for (const item of added) app.log(`Opened ${item.name} (${KIND_NAMES[item.kind] ?? item.kind}).`);
    if (added.length) store.notify(['local']);
    return added;
  }

  // A browser does not tell a page where a file or folder is, so data are named by their path (or
  // on the clonoloom command line); the program reads them where they are.
  app.openPathDialog = async () => {
    const path = await promptDialog({
      title: 'Open data',
      label: 'Path on this computer',
      placeholder: '/data/run42/outs',
      hint: 'An AIRR rearrangement table, a Cell Ranger V(D)J output folder, immunoSEQ exports, or a folder of them. ClonoLoom reads them where they are and never changes them.',
      confirm: 'Open',
    });
    if (!path) return;
    try {
      const answer = await openPaths([path]);
      const added = setLocalItems(answer.files);
      for (const problem of answer.problems ?? []) {
        toast(problem, { kind: 'error' });
        app.log(problem);
      }
      if (added.length) {
        toast(`Opened ${added.length === 1 ? added[0].name : `${added.length} items`}. Reading repertoires arrives in a coming release; the item is listed under On this computer.`, { timeout: 5000 });
        app.focusItem({ kind: 'local', id: added[0].url });
      } else if (!(answer.problems ?? []).length) {
        toast('Already open.', { timeout: 1800 });
      }
    } catch (error) {
      toast(error.message, { kind: 'error' });
    }
  };

  // --- Files opened in the browser ---------------------------------------------------------------

  const fileInput = document.getElementById('file-input');
  app.pickFiles = (accept) => {
    fileInput.accept = accept ?? '';
    fileInput.value = '';
    fileInput.click();
  };
  fileInput.addEventListener('change', () => app.importFiles([...fileInput.files]));

  // Opens dropped or picked files by kind, with the importers the slices register.
  app.importFiles = async (files) => {
    const unread = new Map();
    const unknown = [];
    for (const file of files) {
      const kind = fileKind(file.name);
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
    if (unknown.length) messages.push(`${unknown.length === 1 ? unknown[0] : `${unknown.length} files`}: ClonoLoom opens rearrangement tables (.tsv, .csv, gzipped or not), AIRR metadata (.json, .yaml), Cell Ranger output folders and projects (.clz).`);
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

  // The project document as JSON (the .clz archive arrives with the record, wave 1, slice 7).
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
      '-',
      { section: 'Export' },
      { label: 'Project document (JSON)', icon: 'download', onSelect: exportProjectJSON },
      '-',
      { label: 'Start page', icon: 'grid', onSelect: () => app.setMode('welcome') },
    ]);
  });

  // --- Commands (palette) ------------------------------------------------------------------------

  app.commands = () => [
    ...MODES.filter((m) => m !== 'sep').map((m) => ({ label: `Go to ${m.label}`, icon: m.icon, run: () => app.setMode(m.id), keywords: m.id })),
    { label: 'Open data', icon: 'folder', hint: `${modKey}O`, run: () => app.openPathDialog(), keywords: 'airr cell ranger immunoseq folder path import tsv csv' },
    { label: 'Open files', icon: 'file', run: () => app.pickFiles() },
    { label: 'New project', icon: 'plus', run: () => app.newProject() },
    { label: 'Open a saved project', icon: 'library', hint: `${modKey}⇧O`, run: openLibraryDialog },
    { label: 'Save project now', icon: 'save', hint: `${modKey}S`, run: saveNow },
    { label: 'Export the project document', icon: 'download', run: exportProjectJSON },
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
      h('button.item.statusbar-link', { type: 'button', title: `History and log (${modKey}J)`, onclick: () => app.toggleDrawer() }, icon('drawer'), store.ui.drawer.open ? 'Hide the drawer' : 'History and log'),
      h('span.spacer'),
      h('span.item', version),
      h('span.item.muted', 'Research use only'));
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
  if (app.localItems.length) app.log(`${app.localItems.length} item${app.localItems.length === 1 ? '' : 's'} named on the command line.`);

  // A second "clonoloom <paths>" launch hands its paths to the program and brings this window
  // forward, so checking on focus (and now and then) is enough.
  let checking = false;
  const check = async () => {
    if (checking || document.visibilityState !== 'visible') return;
    checking = true;
    try {
      const fresh = await detectBackend();
      if (fresh) setLocalItems(fresh.files);
    } finally {
      checking = false;
    }
  };
  window.addEventListener('focus', check);
  setInterval(check, 30000);
  window.clonoloom = app;
}

start().catch((error) => {
  console.error(error);
  document.getElementById('workbench').append(h('div.empty', h('h3', 'ClonoLoom could not start'), h('p', error.message)));
});
