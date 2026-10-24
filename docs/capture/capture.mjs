// Captures the documentation's screenshots, in the light and dark themes, as
// docs/images/<scene>-<theme>.webp: the README shows the one matching the reader's theme. Each
// scene starts a ClonoLoom built from source on a fresh, empty library, with the development data
// as its examples (node validation/fetch.mjs --ci downloads them), and drives it through its
// remote control (remote.go, web/ui/remote.js), as any script can, so every run gives the same
// pictures. Only openly licensed data are shown. Follows SituCraft 0.9.0's
// docs/capture/capture.mjs.
//
//   node docs/capture/capture.mjs [scene …] [--theme light|dark|both]
//
// Needs Go (to build ClonoLoom) and Chrome, Chromium, Edge or Brave (CHROME=path).

import { spawn } from 'node:child_process';
import { cpSync, existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { launch, sleep } from '../../validation/cdp.mjs';

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '../..');
const IMAGES = join(ROOT, 'docs/images');
// A port of ClonoLoom's range the development server (8878) does not use.
const PORT = 8877;
const DATA = process.env.CLONOLOOM_DATA || join(ROOT, 'validation/cache');
// What the pictures show as the places of the library and the examples (not this run's temporary
// ones): macOS's defaults.
const SHOWN = {
  library: '~/Library/Application Support/ClonoLoom',
  examples: '~/Library/Application Support/ClonoLoom/examples',
};

const args = process.argv.slice(2);
const option = (name) => {
  const i = args.indexOf(`--${name}`);
  return i >= 0 ? args[i + 1] : null;
};
const themes = { light: ['light'], dark: ['dark'], both: ['light', 'dark'] }[option('theme') ?? 'both'];
if (!themes) throw new Error('--theme is light, dark or both');
const wanted = args.filter((a, i) => !a.startsWith('--') && args[i - 1] !== '--theme');
for (const set of ['10x-pbmc-1k', '10x-pbmc-10k-v2']) {
  if (!existsSync(join(DATA, set))) throw new Error(`The scenes need the development data: run node validation/fetch.mjs --ci (${set} is missing)`);
}

// --- ClonoLoom -----------------------------------------------------------------------------------

const temp = mkdtempSync(join(tmpdir(), 'clonoloom-capture-'));
const binary = join(temp, process.platform === 'win32' ? 'clonoloom.exe' : 'clonoloom');
let URL = '';
let token = '';
let places = {};

// Started per scene on an empty library, so that no scene sees another's projects. examples: the
// examples folder (the development data by default; an empty one shows them to download).
async function startClonoLoom({ examples = DATA } = {}) {
  const library = mkdtempSync(join(temp, 'library-'));
  places = { library, examples };
  const server = spawn(binary, ['--remote-control', '--window', 'none', '--port', String(PORT), '--data-dir', library, '--examples-dir', examples, '--cache-dir', join(temp, 'cache')], { cwd: temp, stdio: ['ignore', 'pipe', 'inherit'] });
  let output = '';
  server.stdout.on('data', (chunk) => { output += chunk; });
  const stop = async () => {
    const exited = new Promise((done) => server.once('exit', done));
    server.kill();
    await Promise.race([exited, sleep(3000)]);
  };
  for (let i = 0; i < 150; i += 1) {
    const address = /running at (http:\/\/[\d.]+:\d+)/.exec(output)?.[1];
    if (address && existsSync(join(library, 'remote.json'))) {
      URL = `${address}/`;
      token = JSON.parse(readFileSync(join(library, 'remote.json'), 'utf8')).token;
      return { stop };
    }
    await sleep(100);
  }
  await stop();
  throw new Error(`ClonoLoom did not start:\n${output}`);
}

// An action through remote control, as any script sends it.
async function act(action, actionArgs = {}) {
  const response = await fetch(`${URL}api/remote/action`, { method: 'POST', headers: { 'Content-Type': 'application/json', 'X-ClonoLoom-Token': token }, body: JSON.stringify({ action, args: actionArgs, client: 'docs/capture' }) });
  const result = await response.json();
  if (!result.ok) throw new Error(`${action}: ${result.message ?? result.error}`);
  return result;
}

// --- Page helpers --------------------------------------------------------------------------------

let b;
const js = (expression) => b.eval(expression);

async function waitFor(expression, timeout = 60000) {
  const start = Date.now();
  while (Date.now() - start < timeout) {
    if (await js(expression)) return true;
    await sleep(200);
  }
  throw new Error(`Timed out waiting for ${expression}`);
}

const clearToasts = () => js(`document.querySelectorAll('.toast').forEach((t) => t.remove())`);

// The places shown in the window, as SHOWN names them: every text node and title that holds this
// run's temporary folders.
async function showPlaces() {
  const pairs = Object.entries(places).map(([k, v]) => [v, SHOWN[k]]).sort((a, c) => c[0].length - a[0].length);
  await js(`(() => {
    const pairs = ${JSON.stringify(pairs)};
    const fix = (text) => pairs.reduce((t, [from, to]) => t.split(from).join(to), text);
    const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
    for (let node = walker.nextNode(); node; node = walker.nextNode()) if (pairs.some(([from]) => node.nodeValue.includes(from))) node.nodeValue = fix(node.nodeValue);
    for (const el of document.querySelectorAll('[title]')) el.title = fix(el.title);
    return true;
  })()`);
}

// --- Scenes --------------------------------------------------------------------------------------

const scenes = {
  async 'cell-states'() {
    await js(`window.clonoloom.setMode('cells')`);
    await waitFor('Boolean(window.clonoloom.cellsView)');
    await js(`(() => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'Use cell-state study').click(); return true; })()`);
    await waitFor(`window.clonoloom.cellsView?.state().status === 'ready'`);
    const state = await js('window.clonoloom.cellsView.state()');
    if (state.page.links.matched !== 359 || state.page.summary.annotations.mapped !== 358 || state.page.cells[1].annotation || state.page.cells[2].annotation.position !== null || state.page.cells[3].annotation.values[0] !== '') throw new Error('Known missing evidence differs');
    await js(`(() => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'Annotations').click(); return true; })()`);
    await waitFor(`window.clonoloom.cellsView.state().page.annotations?.length === 50`);
    for (let i = 0; i < 7; i++) {
      await js(`(() => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'Next').click(); return true; })()`);
      await waitFor(`window.clonoloom.cellsView.state().page.offset === ${(i + 1) * 50}`);
    }
    if (!(await js(`document.querySelector('.view-body').textContent.includes('ANNOTATION-ONLY')`))) throw new Error('Unmatched annotations hidden');
    await js(`(async () => {
      const app = window.clonoloom, record = app.cellsView.state().record;
      const before = JSON.stringify(app.store.project.expression);
      await app.cellsView.inspect({ ...record, cellsPath: record.path }).catch(() => {});
      if (app.cellsView.state().status !== 'error' || before !== JSON.stringify(app.store.project.expression)) throw new Error('Invalid annotation file changed project');
      await app.cellsView.inspect({ ...record, expectedId: record.id });
      await app.saveNow(); await app.openProject(app.store.project.id); await app.setMode('cells');
      const saved = app.store.project.expression[0];
      if (!saved.cellsPath || saved.sources.length !== 4) throw new Error('Annotation provenance lost');
      await app.cellsView.inspect({ ...saved, expectedId: saved.id });
      return true;
    })()`);
    // Exercise the visible optional-file form, then the map and keyboard-accessible table.
    await js(`(() => { document.querySelector('.view-body form').requestSubmit(); return true; })()`);
    await waitFor(`window.clonoloom.cellsView.state().status === 'ready'`);
    await act('inspect_expression', { studyExample: true, studyPreset: 'cell-states-clouds', sample: 0 });
    await js(`(() => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'Imported map').click(); return true; })()`);
    await waitFor(`window.clonoloom.cellsView.state().page.map?.length === 358`);
    await js(`(() => {
      const choice = document.querySelector('[aria-label="Map annotation"]'); choice.value = '0'; choice.dispatchEvent(new Event('change'));
      const missing = document.querySelector('circle.cell-map-missing');
      if (!missing || getComputedStyle(missing).fill !== 'rgb(163, 163, 163)' || missing !== [...document.querySelectorAll('circle[data-cell]')].at(-1)) throw new Error('Missing annotation must be gray and drawn last');
      missing.dispatchEvent(new MouseEvent('click', { bubbles: true }));
      if (!document.querySelector('[aria-label="Selected cell"]').textContent.includes('CELL00004-1')) throw new Error('Missing cell selection lost original index');
      document.querySelector('circle[data-cell]').dispatchEvent(new MouseEvent('click', { bubbles: true }));
      if (!document.querySelector('[aria-label="Selected cell"]').textContent.includes('Coordinates:')) throw new Error('Map cell inspection failed');
      document.querySelector('table.data tbody button').click();
      if (!document.querySelector('[aria-label="Selected cell"]').textContent.includes('CELL00001-1')) throw new Error('Table inspection failed');
      return true;
    })()`);
    await js(`document.querySelector('[aria-label="MEX directory"]').value = ${JSON.stringify(`${SHOWN.examples}/simulated-cell-states-clouds/expression/S01-1`)}`);
    await js(`document.querySelector('[aria-label="Cell annotations"]').value = ${JSON.stringify(`${SHOWN.examples}/simulated-cell-states-clouds/expression/S01-1/cells.tsv`)}`);
    await js(`document.querySelector('#imported-cell-map').scrollIntoView({ block: 'start' })`);
  },
  async cells() {
    await js(`window.clonoloom.setMode('cells')`);
    await waitFor('Boolean(window.clonoloom.cellsView)');
    await js(`(() => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'Use artificial example').click(); return true; })()`);
    await waitFor(`window.clonoloom.cellsView?.state().status === 'ready'`);
    const state = await js('window.clonoloom.cellsView.state()');
    if (state.page.links.matched !== 2 || state.page.links.receptorOnly !== 1 || state.page.cells[0].geneUMIs !== 5 || state.page.cells[3].geneUMIs !== 0) throw new Error('Expression evidence differs');
    await js(`(() => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'Features').click(); return true; })()`);
    await waitFor(`window.clonoloom.cellsView.state().page.features.length === 3`);
    if (!(await js(`document.querySelector('.view-body').textContent.includes('Antibody Capture')`))) throw new Error('Feature modality missing');
    await js(`window.clonoloom.setMode('data')`);
    await js(`window.clonoloom.setMode('cells')`);
    await waitFor('Boolean(window.clonoloom.cellsView)');
    await js(`(() => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'Reopen').click(); return true; })()`);
    await waitFor(`window.clonoloom.cellsView?.state().status === 'ready'`);
    if ((await js('window.clonoloom.cellsView.state().record.id')) !== state.record.id) throw new Error('Expression identity changed on reopen');
    await js(`(async () => {
      const app = window.clonoloom;
      const before = JSON.stringify(app.store.project.expression);
      await app.cellsView.inspect({ path: '/does-not-exist' }).catch(() => {});
      if (app.cellsView.state().status !== 'error' || JSON.stringify(app.store.project.expression) !== before) throw new Error('Invalid input changed saved evidence');
      const record = app.store.project.expression[0];
      await app.cellsView.inspect({ ...record, expectedId: record.id });
      return true;
    })()`);
    // Exercise the visible directory form and its explicit sample selector.
    await js(`(() => {
      const sample = document.querySelector('[aria-label="Receptor sample"]');
      sample.value = ''; document.querySelector('.view-body form').requestSubmit();
      if (!document.querySelector('[role="status"]').textContent.includes('explicitly')) throw new Error('Missing sample accepted');
      sample.value = '0'; document.querySelector('.view-body form').requestSubmit(); return true;
    })()`);
    await waitFor(`window.clonoloom.cellsView?.state().status === 'ready'`);
    await js(`(async () => {
      const app = window.clonoloom;
      const id = app.store.project.id;
      await app.saveNow();
      await app.openProject(id);
      await app.setMode('cells');
      const record = app.store.project.expression[0];
      await app.cellsView.inspect({ ...record, expectedId: record.id });
      if (app.cellsView.state().page.links.matched !== 2) throw new Error('Saved project expression did not reopen');
      return true;
    })()`);
    await js(`(() => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'Remove').click(); return true; })()`);
    if ((await js('window.clonoloom.store.project.expression.length')) !== 0) throw new Error('Expression removal failed');
    await js(`(() => { window.clonoloom.store.undo(); return true; })()`);
    await waitFor('window.clonoloom.store.project.expression.length === 1');
    await act('inspect_expression', { example: true, sample: 0 });
    await js(`document.querySelector('[aria-label="MEX directory"]').value = ${JSON.stringify(`${SHOWN.examples}/synthetic-expression`)}`);
  },

  async isotypes() {
    await act('open_example', { example: 'simulated-bcell-immunization', download: true });
    await act('find_bcell_families', { threshold: 0.1 });
    await js(`(() => { window.clonoloom.bcellsMode.show('isotypes'); return true; })()`);
    await waitFor(`window.clonoloom.isotypesView.state().status === 'absent'`);
    await js(`(() => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'Summarize isotypes').click(); return true; })()`);
    await waitFor(`window.clonoloom.isotypesView.state().status === 'ready'`);
    const result = await js('window.clonoloom.isotypesView.state()');
    if (result.page.result.assigned !== 2160 || result.page.result.resolvedGene !== 2160 || result.page.familyCount !== 36 || result.page.count !== 18) throw new Error('Wrong isotype totals');
    if (!result.page.rows.every((r) => r.total === 240 && r.timepoint && r.tissue)) throw new Error('Missing denominator or metadata');
    await js(`window.clonoloom.isotypesView.showPage('pairs')`);
    const pairs = await js('window.clonoloom.isotypesView.state().page.rows');
    if (pairs.length !== 3 || pairs.some((p) => p.families !== 36)) throw new Error('Wrong co-occurrence');
    await js(`(() => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'IGHA1 + IGHG1').click(); return true; })()`);
    await waitFor(`window.clonoloom.isotypesView.state().page?.kind === 'families'`);
    if ((await js('window.clonoloom.isotypesView.state().page.count')) !== 36) throw new Error('Pair filter lost families');
    const family = await js('window.clonoloom.isotypesView.state().page.rows[0].id');
    await js(`window.clonoloom.isotypesView.showPage('usage', {family:${JSON.stringify(family)},level:'gene'})`);
    if (!(await js('window.clonoloom.isotypesView.state().page.rows.every((r) => r.family && r.level === "gene")'))) throw new Error('Missing family usage');
    await js(`window.clonoloom.isotypesView.showPage('observations')`);
    const first = await js('window.clonoloom.isotypesView.state().page.rows[0].id');
    await js(`window.clonoloom.isotypesView.showPage('observations', {offset:50})`);
    if ((await js('window.clonoloom.isotypesView.state().page.rows[0].id')) === first) throw new Error('Observation page did not advance');
    await js(`(() => { const v=window.clonoloom.isotypesView; v.inspect(v.state().page.rows[0]); return true; })()`);
    await waitFor(`document.querySelector('.inspector').textContent.includes('Raw C call:')`);
    await act('show_methods');
    await waitFor(`Boolean(document.querySelector('.methods-text'))`);
    const methods = await js(`document.querySelector('.dialog').textContent`);
    if (!methods.includes('eLife.16578') || !methods.includes(result.page.result.isotypeOptions.familySha256) || !methods.includes('unordered')) throw new Error('Missing isotype methods');
    await act('close_dialogs');
    await js(`(() => { window.clonoloom.isotypesView.showPage('usage'); window.clonoloom.bcellsMode.show('inputs'); return true; })()`);
    await waitFor(`Boolean(window.clonoloom.bcellsView)`);
    await js(`(() => { window.clonoloom.focusItem({kind:'result',id:${JSON.stringify(result.artifact)}}); return true; })()`);
    await waitFor(`window.clonoloom.isotypesView?.state().status === 'ready'`);
    if ((await js('window.clonoloom.isotypesView.state().artifact')) !== result.artifact) throw new Error('Saved isotypes did not reopen');
  },
  async lineages() {
    await act('open_example', { example: 'simulated-bcell-immunization', download: true });
    await act('find_bcell_families', { threshold: 0.1 });
    await act('reconstruct_bcell_germlines', { reference: 'simulated-bcell-immunization' });
    await js(`(() => { window.clonoloom.bcellsMode.show('lineages'); return true; })()`);
    await waitFor(`window.clonoloom.lineagesView.state().status === 'absent'`);
    await js(`(() => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'Build lineage trees').click(); return true; })()`);
    await waitFor(`window.clonoloom.lineagesView.state().status === 'ready'`);
    const result = await js('window.clonoloom.lineagesView.state()');
    if (result.page.result.built !== 72 || result.page.rowCount !== 4320 || result.page.rows.length !== 50) throw new Error('Wrong lineage totals');
    await js(`window.clonoloom.lineagesView.openTree(${JSON.stringify(result.page.rows[0].id)})`);
    const tree = await js('window.clonoloom.lineagesView.state().tree');
    if ((await js(`getComputedStyle(document.querySelector('.lineage-plot path')).stroke`)) === 'none') throw new Error('Invisible tree edges');
    if (tree.sites !== 327 || !tree.optimalTrees || !tree.nodes.some((n) => n.kind === 'inferred')) throw new Error('Wrong lineage detail');
    const id = tree.nodes.find((n) => n.kind === 'inferred').id;
    await js(`(() => { document.querySelector('[data-node="${id}"]').dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true })); return true; })()`);
    await waitFor(`window.clonoloom.lineagesView.state().node?.id === ${JSON.stringify(id)}`);
    if ((await js('window.clonoloom.lineagesView.state().node.possible.length')) !== 327) throw new Error('Missing conditional states');
    if (!(await js(`document.activeElement.hasAttribute('data-lineage-detail')`))) throw new Error('Node details lost keyboard focus');
    await js(`(() => { const s=document.querySelector('[aria-label="Tree annotations"]'); s.value='timepoint'; s.dispatchEvent(new Event('change')); return true; })()`);
    await js(`window.clonoloom.lineagesView.showPage('rows')`);
    const row = await js('window.clonoloom.lineagesView.state().page.rows[0]');
    if (!row.isotype || !row.timepoint || !row.tissue || !row.node) throw new Error('Missing observed annotations');
    await js(`window.clonoloom.lineagesView.showPage('rows', '', '', 50)`);
    if ((await js('window.clonoloom.lineagesView.state().page.rows[0].id')) === row.id) throw new Error('Lineage page did not advance');
    await js(`(() => { const v=window.clonoloom.lineagesView; v.inspect(v.state().page.rows[0]); return true; })()`);
    await waitFor(`document.querySelector('.inspector').textContent.includes('Lineage source row')`);
    await act('show_methods');
    await waitFor(`Boolean(document.querySelector('.methods-text'))`);
    const methods = await js(`document.querySelector('.dialog').textContent`);
    if (!methods.includes('0128004') || !methods.includes('not bootstrap support') || !methods.includes(result.page.result.lineageOptions.germlineSha256)) throw new Error('Missing lineage methods');
    await act('close_dialogs');
    await js(`(() => { window.clonoloom.lineagesView.openTree(${JSON.stringify(tree.id)}); window.clonoloom.bcellsMode.show('inputs'); return true; })()`);
    await waitFor(`Boolean(window.clonoloom.bcellsView)`);
    await js(`(() => { window.clonoloom.focusItem({kind:'result',id:${JSON.stringify(result.artifact)}}); return true; })()`);
    await waitFor(`window.clonoloom.lineagesView?.state().status === 'ready'`);
    if ((await js('window.clonoloom.lineagesView.state().artifact')) !== result.artifact) throw new Error('Saved lineage did not reopen');
    await js(`window.clonoloom.lineagesView.openTree(${JSON.stringify(tree.id)})`);
    await js(`(() => { document.querySelector('.lineage-plot').scrollIntoView({block:'center'}); return true; })()`);
  },
  async shm() {
    await act('open_example', { example: 'simulated-bcell-immunization', download: true });
    await act('find_bcell_families', { threshold: 0.1 });
    await act('reconstruct_bcell_germlines', { reference: 'simulated-bcell-immunization' });
    await js(`(() => { window.clonoloom.bcellsMode.show('shm'); return true; })()`);
    await waitFor(`window.clonoloom.shmView.state().status === 'absent'`);
    await js(`(() => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'Measure SHM').click(); return true; })()`);
    await waitFor(`window.clonoloom.shmView.state().status === 'ready'`);
    const result = await js('window.clonoloom.shmView.state()');
    if (result.page.result.evaluated !== 4320 || result.page.summaryCount !== 270 || result.page.rows.length !== 50 || Object.keys(result.page.result.codonExclusions).length) throw new Error('Wrong SHM summary');
    await js(`window.clonoloom.shmView.showPage('rows')`);
    const row = await js('window.clonoloom.shmView.state().page.rows[0]');
    if (row.regions[0].evaluableBases !== 327 || row.regions[0].evaluableCodons !== 109 || row.regions[8].frequency !== null) throw new Error('Wrong SHM denominator');
    await js(`window.clonoloom.shmView.showPage('rows', '', '', 50)`);
    if ((await js('window.clonoloom.shmView.state().page.rows[0].id')) === row.id) throw new Error('SHM page did not advance');
    await js(`(() => { const v=window.clonoloom.shmView; v.inspect(v.state().page.rows[0]); return true; })()`);
    await waitFor(`document.querySelector('.inspector').textContent.includes('Endpoint codons')`);
    await act('show_methods');
    await waitFor(`Boolean(document.querySelector('.methods-text'))`);
    const methods = await js(`document.querySelector('.dialog').textContent`);
    if (!methods.includes('single-hit') || !methods.includes('No selection statistic') || !methods.includes(result.page.result.shmOptions.germlineSha256)) throw new Error('Missing SHM methods');
    await act('close_dialogs');
    await js(`(() => { window.clonoloom.shmView.showPage('rows'); window.clonoloom.bcellsMode.show('inputs'); return true; })()`);
    await waitFor(`Boolean(window.clonoloom.bcellsView)`);
    await js(`(() => { window.clonoloom.focusItem({kind:'result',id:${JSON.stringify(result.artifact)}}); return true; })()`);
    await waitFor(`window.clonoloom.shmView?.state().status === 'ready'`);
    if ((await js('window.clonoloom.shmView.state().artifact')) !== result.artifact) throw new Error('Saved SHM did not reopen');
    await js(`window.clonoloom.shmView.showPage('rows')`);
  },
  async germlines() {
    await act('open_example', { example: 'simulated-bcell-immunization', download: true });
    await act('find_bcell_families', { threshold: 0.1 });
    await js(`(() => { window.clonoloom.bcellsMode.show('germlines'); return true; })()`);
    await waitFor(`Boolean(document.querySelector('[aria-label="Reference"]'))`);
    await js(`(() => { const select = document.querySelector('[aria-label="Reference"]'); select.value = 'simulated-bcell-immunization'; select.dispatchEvent(new Event('change')); return true; })()`);
    await waitFor(`window.clonoloom.germlinesView.state().status === 'absent'`);
    await js(`(() => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'Reconstruct germlines').click(); return true; })()`);
    await waitFor(`window.clonoloom.germlinesView.state().status === 'ready'`);
    const result = await js('window.clonoloom.germlinesView.state()');
    if (result.page.result.reconstructed !== 4320 || result.page.familyCount !== 72 || result.page.rows.length !== 50) throw new Error('Wrong germline reconstruction');
    await js(`window.clonoloom.germlinesView.showPage('rows')`);
    const row = await js('window.clonoloom.germlinesView.state().page.rows[0]');
    if (row.germline.slice(285,318) !== 'N'.repeat(33) || row.queryPositions.length !== 360) throw new Error('Wrong masking or coordinates');
    await js(`(() => { const v=window.clonoloom.germlinesView; v.inspect(v.state().page.rows[0]); return true; })()`);
    await waitFor(`document.querySelector('.inspector').textContent.includes('not IMGT numbered')`);
    await act('show_methods');
    await waitFor(`Boolean(document.querySelector('.methods-text'))`);
    const methods = await js(`document.querySelector('.dialog').textContent`);
    if (!methods.includes('D-masked germline reconstruction') || !methods.includes('btv359') || !methods.includes(result.page.result.germlinesOptions.files[0].sha256)) throw new Error('Missing germline methods or provenance');
    await act('close_dialogs');
    // A delayed row load cannot repaint a destroyed view.
    await js(`(() => { window.clonoloom.germlinesView.showPage('rows'); window.clonoloom.bcellsMode.show('inputs'); return true; })()`);
    await waitFor(`Boolean(window.clonoloom.bcellsView)`);
    await js(`(() => { window.clonoloom.focusItem({kind:'result',id:${JSON.stringify(result.artifact)}}); return true; })()`);
    await waitFor(`window.clonoloom.germlinesView?.state().status === 'ready'`);
    if ((await js('window.clonoloom.germlinesView.state().artifact')) !== result.artifact) throw new Error('Saved germline result did not reopen');
  },
  async families() {
    await act('open_example', { example: 'simulated-bcell-immunization', download: true });
    await act('find_bcell_families');
    const diagnostic = await js('window.clonoloom.familiesView.state()');
    if (diagnostic.page.familyCount !== 0 || diagnostic.page.representativeCount !== 133 || diagnostic.page.result.eligible !== 2160) throw new Error('Wrong distance diagnostics: ' + JSON.stringify({ counts: [diagnostic.page.familyCount, diagnostic.page.representativeCount, diagnostic.page.result.eligible], result: diagnostic.page.result }));
    // Exercise the visible form, not just the remote handler.
    await js(`(() => {
      const input = document.querySelector('[aria-label="Family threshold"]'); input.value = '0.1';
      [...document.querySelectorAll('button')].find((b) => b.textContent === 'Apply threshold').click(); return true;
    })()`);
    await waitFor(`window.clonoloom.familiesView.state().status === 'absent'`);
    await js(`(() => { [...document.querySelectorAll('button')].find((b) => b.textContent === 'Find clonal families').click(); return true; })()`);
    await waitFor(`window.clonoloom.familiesView.state().status === 'ready'`);
    const result = await js('window.clonoloom.familiesView.state()');
    if (result.page.familyCount !== 36 || result.page.result.eligible !== 2160) throw new Error('Wrong family recovery');
    if (await js(`document.querySelector('.plot').outerHTML.includes('NaN')`)) throw new Error('Invalid histogram');
    await js(`window.clonoloom.familiesView.showPage('members', ${JSON.stringify(result.page.rows[0].id)})`);
    await js(`(() => { const v = window.clonoloom.familiesView; v.inspect(v.state().page.rows[0]); return true; })()`);
    await waitFor(`document.querySelector('.inspector').textContent.includes('SHA-256')`);
    await js(`window.clonoloom.familiesView.showPage('nearest')`);
    if ((await js('window.clonoloom.familiesView.state().page.rows.length')) !== 50) throw new Error('Unbounded nearest page');
    await act('show_methods');
    await waitFor(`Boolean(document.querySelector('.methods-text'))`);
    const methods = await js(`document.querySelector('.dialog').textContent`);
    if (!methods.includes('1601850') || !methods.includes('Paired maximum distance')) throw new Error('Family methods missing');
    await act('close_dialogs');
    // An older diagnostic is inspected by its identity, independent of current options.
    await js(`(() => { window.clonoloom.focusItem({kind:'result', id:${JSON.stringify(diagnostic.artifact)}}); return true; })()`);
    await waitFor(`window.clonoloom.familiesView.state().status === 'ready' && window.clonoloom.familiesView.state().page.familyCount === 0`);
    await js(`(() => { window.clonoloom.focusItem({kind:'result', id:${JSON.stringify(result.artifact)}}); return true; })()`);
    await waitFor(`window.clonoloom.familiesView.state().status === 'ready' && window.clonoloom.familiesView.state().page.familyCount === 36`);
  },
  async immunization() {
    await act('open_example', { example: 'simulated-bcell-immunization', download: true });
    const catalog = await (await fetch(`${URL}api/examples`)).json();
    const example = catalog.examples.find((e) => e.name === 'simulated-bcell-immunization');
    for (const step of example.opens[0].steps) await act(step.action, step.args);
    await act('close_dialogs');
    const saved = await js('window.clonoloom.store.project.artifacts');
    for (const kind of ['families', 'germlines', 'shm', 'lineages', 'isotypes']) {
      if (!saved.some((a) => a.kind === kind)) throw new Error(`Guided study did not save ${kind}`);
    }
    await act('inspect_bcell_inputs', { select: 1 });
    const page = await js('window.clonoloom.bcellsView.page()');
    if (page.count !== 4320 || page.samples.length !== 27 || page.samples.some((s) => s.rows !== s.valid || s.rows !== s.regions)) throw new Error('Artificial B-cell evidence differs');
    await act('show_methods');
    await waitFor(`Boolean(document.querySelector('.methods-text'))`);
    const methods = await js(`document.querySelector('.methods-text').textContent`);
    if (!methods.includes('artificial paired B-cell') || methods.includes('GRCh38') || methods.includes('varying between samples beyond sampling')) throw new Error('Wrong simulation methods');
    const references = await js(`document.querySelector('.methods-references').textContent`);
    for (const citation of ['10.4049/jimmunol.1601850', '10.1093/bioinformatics/btv359', '10.1093/sysbio/20.4.406', '10.1137/0128004', '10.1371/journal.pcbi.1009885']) {
      if (!references.includes(citation)) throw new Error(`Guided methods omitted ${citation}`);
    }
    await act('close_dialogs');
    await act('inspect_bcell_inputs', { select: 1 });
    await waitFor(`document.querySelector('.inspector').textContent.includes('clonoloom-artificial-bcell-v2')`);
  },
  async imgt() {
    await act('open_example', { example: 'synthetic-imgt-inputs', download: true });
    const catalog = await (await fetch(`${URL}api/examples`)).json();
    const example = catalog.examples.find((e) => e.name === 'synthetic-imgt-inputs');
    for (const step of example.opens[0].steps) await act(step.action, step.args);
    const page = await js('window.clonoloom.bcellsView.page()');
    if (page.count !== 3 || page.rows.some((r) => r.evidence.paired) || !page.rows[2].evidence.issues.length) throw new Error('IMGT evidence differs');
    await js(`(async () => {
      const app = window.clonoloom;
      const record = app.store.project.datasets[0];
      if (record.sources.length !== 5 || record.sources.some((s) => !s.imgtGroup)) throw new Error('IMGT source provenance missing');
      const response = await fetch('api/datasets/' + record.id, { method: 'DELETE' });
      if (!response.ok) throw new Error('Closing IMGT dataset failed');
      app.datasetStates.delete(record.id);
      const reopened = await app.ensureDataset(record);
      if (reopened.status !== 'open') throw new Error('IMGT project reopen failed');
      return true;
    })()`);
    await act('inspect_bcell_inputs', { select: 3 });
    await waitFor(`document.querySelector('.inspector').textContent.includes('withheld')`);
  },
  async bcells() {
    await act('open_example', { example: 'synthetic-bcell-inputs', download: true });
    const catalog = await (await fetch(`${URL}api/examples`)).json();
    const example = catalog.examples.find((e) => e.name === 'synthetic-bcell-inputs');
    for (const step of example.opens[0].steps) await act(step.action, step.args);
    const page = await js('window.clonoloom.bcellsView.page()');
    if (page.count !== 7 || page.rows[3].evidence.valid || !page.rows[0].evidence.valid) throw new Error('B-cell evidence summary differs');
    // A selection that finishes after reloading must not restore stale inspector evidence.
    await js(`(async () => {
      const app = window.clonoloom, view = app.bcellsView;
      const id = app.store.project.datasets[0].id;
      const oldFetch = window.fetch;
      let release;
      const gate = new Promise((resolve) => { release = resolve; });
      window.fetch = async (...args) => {
        const response = oldFetch(...args);
        if (String(args[0]).includes('bcells/inputs') && JSON.parse(args[1].body).id) await gate;
        return response;
      };
      try {
        const pending = view.inspect(view.page().rows[0].id);
        await view.show(id);
        release();
        if (await pending !== undefined) throw new Error('Stale inspector response applied');
      } finally { release(); window.fetch = oldFetch; }
      const { saveFilter } = await import('./lib/filters.js');
      app.store.commit(saveFilter(app.store.project, { dataset: id, name: 'Only light kappa', rules: { loci: ['IGK'] }, reason: 'Check evidence filtering' }), 'Filter evidence');
      return true;
    })()`);
    await waitFor(`window.clonoloom.bcellsView.page()?.count === 1`);
    await js(`(async () => {
      const app = window.clonoloom;
      const { saveFilter } = await import('./lib/filters.js');
      app.store.commit(saveFilter(app.store.project, { dataset: app.store.project.datasets[0].id, name: 'Only T cells', rules: { loci: ['TRB'] }, reason: 'Check empty B-cell inputs' }), 'Filter evidence');
      return true;
    })()`);
    await waitFor(`window.clonoloom.bcellsView.page()?.count === 0`);
    await js(`(async () => {
      const app = window.clonoloom;
      const { useFilter } = await import('./lib/filters.js');
      app.store.commit(useFilter(app.store.project, app.store.project.datasets[0].id, null), 'Restore inputs');
      return true;
    })()`);
    await waitFor(`window.clonoloom.bcellsView.page()?.count === 7`);
    await act('inspect_bcell_inputs', { select: 4 });
    await waitFor(`document.querySelector('.inspector').textContent.includes('different lengths')`);
  },
  // The start page of a first run.
  async start() {
    await waitFor(`Boolean(document.querySelector('.welcome'))`);
  },
  // The Example data dialog of a first run: nothing downloaded yet.
  async examples() {
    await act('show_examples');
    await waitFor(`document.querySelectorAll('.example-card').length >= 6`);
  },
  // The Data view of the PBMC 1k T cells: the dataset, its validation and samples; its files and
  // license in the inspector.
  async data() {
    await act('open_example', { example: '10x-pbmc-1k', opening: 'T cells' });
    await waitFor(`Boolean(document.querySelector('.dataset-card.focused'))`);
  },
  // QC of the PBMC 10k T cells: Cell Ranger's metrics recomputed, the chains of each cell.
  async qc() {
    await act('open_example', { example: '10x-pbmc-10k-v2', opening: 'T cells' });
    await act('show', { view: 'qc' });
    await waitFor(`document.querySelectorAll('.plot-card svg').length > 0`);
  },
  // The clonotypes of the PBMC 10k T cells under the paired αβ definition, the largest in the
  // inspector.
  async clonotypes() {
    await act('open_example', { example: '10x-pbmc-10k-v2', opening: 'T cells' });
    await act('select_clonotype', { rank: 1 });
    await waitFor(`document.querySelectorAll('.ctable-row:not(.loading)').length > 5 && Boolean(document.querySelector('.members'))`);
  },
  // The methods paragraph of the PBMC 1k T cells, with its references.
  async vaccine() {
    // Execute the catalog's actual guided actions, without downloaded reference dependencies.
    const catalog = await (await fetch(`${URL}api/examples`)).json();
    for (const name of ['dash-2017-tcrdist3', 'simulated-vaccine']) {
      const e = catalog.examples.find((e) => e.name === name);
      for (const opening of e.opens) {
        await act('new_project', { name: opening.name });
        await act('open_example', { example: name, opening: opening.title, download: true });
        for (const step of opening.steps ?? []) {
          await act(step.action, step.args ?? {});
          console.log(`Guided step: ${opening.title} / ${step.title}`);
        }
      }
    }
    await waitFor(`window.clonoloom.trackingView?.followed().status === 'ready'`);
    const tracked = await js('window.clonoloom.trackingView?.followed().result');
    if (!tracked?.choice.evidence?.artifact || !tracked.subjects.length) throw new Error('Guided specificity tracking failed');
    await act('match_specificity', {});
    await waitFor(`Boolean(document.querySelector('.specificity-view table tbody button'))`);
  },
  async invariant() {
    await act('open_example', { example: '10x-pbmc-10k-v2', opening: 'T cells' });
    const result = await act('flag_invariant', { species: 'human', population: 'MAIT' });
    if (result.data.invariant.samples[0].matched[0] !== 80) throw new Error('Unexpected MAIT cell count');
    await js('window.clonoloom.store.undo()');
    await waitFor(`window.clonoloom.clonotypeContext?.request.species === '' && window.clonoloom.clonotypeContext?.request.population === ''`);
    await act('flag_invariant', { species: 'human', population: 'MAIT' });
    await js(`document.querySelector('.clonotype-info details').open = true`);
    await waitFor(`document.querySelectorAll('.ctable-row:not(.loading)').length > 5`);
  },
  async methods() {
    await act('open_example', { example: '10x-pbmc-1k', opening: 'T cells' });
    await act('show_methods');
    await waitFor(`Boolean(document.querySelector('.methods-text'))`);
  },
  // The diversity of the simulated study's twelve samples, each with its 95% interval.
  async diversity() {
    await act('open_example', { example: 'simulated-standard' });
    await act('measure_diversity');
    await waitFor(`document.querySelectorAll('.repertoire-view .plot-card svg').length >= 3`);
  },
  // Its samples compared at equal coverage, with their rarefaction and extrapolation curves.
  async rarefaction() {
    await act('open_example', { example: 'simulated-standard' });
    await act('rarefy');
    await waitFor(`document.querySelectorAll('.repertoire-view .plot-card svg').length >= 3`);
  },
  // Its samples' V genes, and one gene across the samples.
  async genes() {
    await act('open_example', { example: 'simulated-standard' });
    await act('gene_usage');
    await waitFor(`document.querySelectorAll('.repertoire-view .plot-card svg').length >= 2`);
  },
  // Its samples' junction lengths and amino acids.
  async junctions() {
    await act('open_example', { example: 'simulated-standard' });
    await act('gene_usage', { part: 'junctions' });
    await waitFor(`document.querySelectorAll('.repertoire-view .plot-card svg').length >= 3`);
  },
  // Every pair of its samples by Morisita's index; the pairs of one subject and of two.
  async overlap() {
    await act('open_example', { example: 'simulated-standard' });
    await act('measure_overlap');
    await waitFor(`document.querySelectorAll('.repertoire-view .plot-card svg').length >= 2`);
  },
  // The simulated CAR-T study's subjects: each patient's samples in days from the infusion, its
  // infusion product as its reference sample, and the two samples taken before it (replicates).
  async subjects() {
    await act('open_example', { example: 'simulated-car-t', download: true });
    await act('describe_subjects');
    await waitFor(`document.querySelectorAll('.tracking-view .plot-card svg').length >= 1`);
  },
  // Its second patient's clonotypes followed through time: each sample's five largest, the one
  // that rises most after the infusion chosen and drawn with its intervals.
  async tracking() {
    await act('open_example', { example: 'simulated-car-t', download: true });
    await act('track_clonotypes', { by: 'largest', n: 5, subject: 'S02' });
    await js(`(() => {
      const view = window.clonoloom.trackingView;
      const s = view.followed().result.subjects.find((x) => x.id === 'S02');
      const rise = (c) => Math.max(...c.points.slice(2).map((v) => v.frequency ?? 0)) / Math.max(c.points[1].frequency ?? 0, 1e-6);
      view.select([...s.clonotypes].sort((a, b) => rise(b) - rise(a))[0].id);
      return true;
    })()`);
    await waitFor(`document.querySelectorAll('.tracking-view .plot-card svg').length >= 2 && Boolean(document.querySelector('.plot-chosen'))`);
  },
  // Its clonotypes tested between the week before the infusion and two weeks after by the
  // beta-binomial test: its variance, fitted on the samples taken twice before the infusion, at the
  // top; below, the second patient's calls, the most significant increase chosen.
  async tests() {
    await act('open_example', { example: 'simulated-car-t', download: true });
    await act('test_expansion', { from: 'day -7', to: 'day 14', subject: 'S02' });
    await js(`(() => {
      const view = window.clonoloom.trackingView;
      const s = view.tested().result.subjects.find((x) => x.subject === 'S02');
      view.showTested('S02', s.clonotypes.find((c) => c.call === 'increase').id);
      return true;
    })()`);
    await waitFor(`document.querySelectorAll('.tracking-view .plot-card svg').length >= 1 && Boolean(document.querySelector('.plot-ring'))`);
  },
  // Its clonotypes classed against the week before the infusion: the guided steps folded away, the
  // classes by subject, then the second patient's transient clonotypes rising to day 14.
  async classes() {
    await act('open_example', { example: 'simulated-car-t', download: true });
    await act('class_clonotypes', { base: 'day -7', class: 'transient', subject: 'S02' });
    await js(`(() => { const g = document.querySelector('.tracking-guide'); if (g) g.open = false; return true; })()`);
    await waitFor(`Boolean(document.querySelector('.tracking-class-head'))`);
    await js(`(() => { const el = document.querySelector('.tracking-class-head'); el.closest('.workbench-scroll').scrollTop = el.offsetTop - 160; return true; })()`);
  },
  // The transplant study's samples attributed to the donors: the share derived from each donor.
  async attribution() {
    await act('open_example', { example: 'simulated-transplant', download: true });
    await act('attribute_samples');
    await js(`(() => { const g = document.querySelector('.tracking-guide'); if (g) g.open = false; return true; })()`);
    await waitFor(`document.querySelectorAll('.tracking-view .plot-card svg').length >= 2`);
    await js(`(() => { const el = document.querySelectorAll('.tracking-view .plot-card')[1]; el.closest('.workbench-scroll').scrollTop = el.offsetTop - 60; return true; })()`);
  },
  // Dash et al.'s mouse TCRs compared by TCRdist with their V genes' loops (the reference data put
  // in the library, as a download leaves them): each clonotype's distance to its nearest, and the
  // clonotype with the most neighbors with its nearest, loop by loop.
  async tcrdist() {
    cpSync(join(DATA, 'tcrdist3-loops'), join(places.library, 'references', 'tcrdist3-loops'), { recursive: true });
    await act('open_example', { example: 'dash-2017-tcrdist3', download: true });
    await act('find_neighbors', { select: 1 });
    await waitFor(`Boolean(document.querySelector('.specificity-view .nearest table'))`);
    await js(`(() => { const el = document.querySelector('.specificity-view .plot-card'); el.closest('.workbench-scroll').scrollTop = el.offsetTop - 60; return true; })()`);
  },
  // A simulated sample's network, with the complete graph and bounded drawing.
  async database() {
    await act('open_example', { example: 'simulated-vaccine', download: true });
    cpSync(join(DATA, 'tcrdist3-loops'), join(places.library, 'references', 'tcrdist3-loops'), { recursive: true });
    const near = await act('match_specificity', { database: 'simulated-vaccine', metric: 'tcrdist', radius: 24, genes: true, cdr3Only: false });
    if (near.data?.matches <= 62) throw new Error('Near matching failed to find additional evidence');
    const result = await act('match_specificity', { metric: 'exact' });
    if (result.data?.epitopes.length !== 4 || result.data?.matches !== 62) throw new Error('Specificity remote action returned incorrect counts: ' + JSON.stringify(result));
    await act('match_specificity', { follow: 2 });
    await waitFor(`window.clonoloom.trackingView?.followed().status === 'ready'`);
    const tracked = await js(`window.clonoloom.trackingView.followed().result`);
    if (!tracked.choice.evidence?.artifact || !tracked.subjects.length) throw new Error('Specificity tracking lost its provenance or subjects');
    await act('match_specificity', {});
    await waitFor(`Boolean(document.querySelector('.specificity-view table tbody button'))`);
  },
  async networks() {
    await act('open_data', { paths: [join(DATA, 'simulated-vaccine/S01-d14.tsv')], name: 'Simulated vaccination · S01, day 14' });
    const result = await act('find_networks', { metric: 'levenshtein', cutoff: 2, sameV: true, sameJ: true, select: 1 });
    if (result.data?.nodes !== 7658 || result.data?.edges !== 1176) throw new Error('Network remote action returned incorrect counts');
    await waitFor(`Boolean(document.querySelector('.specificity-view canvas'))`);
    // Check bounded cluster navigation and a member's link before capturing.
    await js(`(() => { const input = document.querySelector('input[aria-label="Cluster number"]'); input.value = '201'; input.dispatchEvent(new Event('change', { bubbles: true })); })()`);
    await waitFor(`document.querySelector('select[aria-label="Cluster"]').value === '201'`);
    await js(`document.querySelector('.specificity-view table tbody button').click()`);
    await waitFor(`window.clonoloom.store.ui.mode === 'clonotypes'`);
    await act('find_networks', { select: 1 });
    await waitFor(`Boolean(document.querySelector('.specificity-view canvas'))`);
  },
  // Neighborhoods against a background drawn from OLGA's human TRB model.
  async neighborhoods() {
    cpSync(join(DATA, 'tcrdist3-loops'), join(places.library, 'references', 'tcrdist3-loops'), { recursive: true });
    cpSync(join(DATA, 'olga-models'), join(places.library, 'references', 'olga-models'), { recursive: true });
    await act('open_example', { example: 'simulated-vaccine', download: true });
    await act('find_neighborhoods', {});
    await waitFor(`document.querySelectorAll('.specificity-view table.selectable tbody tr').length > 10`);
    await js(`(() => { const el = document.querySelector('.specificity-view .summary-card'); el.closest('.workbench-scroll').scrollTop = el.offsetTop - 60; return true; })()`);
  },
  // Dash et al.'s mouse TCRs' generation probabilities under OLGA's mouse models (the reference
  // data put in the library, as a download leaves them): their distribution, and the clonotypes,
  // the most abundant first.
  async generation() {
    cpSync(join(DATA, 'olga-models'), join(places.library, 'references', 'olga-models'), { recursive: true });
    await act('open_example', { example: 'dash-2017-tcrdist3', download: true });
    await act('measure_generation', {});
    await waitFor(`document.querySelectorAll('.specificity-view .plot-card svg').length >= 2`);
    await js(`(() => { const el = document.querySelectorAll('.specificity-view .plot-card')[1]; el.closest('.workbench-scroll').scrollTop = el.offsetTop - 60; return true; })()`);
  },
};

const names = wanted.length ? wanted : Object.keys(scenes);
for (const name of names) if (!scenes[name]) throw new Error(`Unknown scene ${name}. Scenes: ${Object.keys(scenes).join(', ')}`);
mkdirSync(IMAGES, { recursive: true });
await new Promise((done, fail) => spawn('go', ['build', '-o', binary, '.'], { cwd: ROOT, stdio: 'inherit' }).on('exit', (code) => (code ? fail(new Error('go build failed (is Go installed?)')) : done())));
try {
  for (const theme of themes) {
    for (const name of names) {
      let clonoloom = null;
      b = null;
      try {
        clonoloom = await startClonoLoom(name === 'examples' ? { examples: mkdtempSync(join(temp, 'examples-')) } : {});
        // A Chrome that does not start is started again once; a scene that cannot start fails
        // alone, not the scenes after it.
        b = await launch({ width: 1600, height: 1000, scale: 1.25 }).catch(async () => {
          await sleep(2000);
          return launch({ width: 1600, height: 1000, scale: 1.25 });
        });
        await b.theme(theme === 'dark');
        await b.goto(URL, 800);
        await waitFor('Boolean(window.clonoloom?.remote)', 30000);
        // No commit in the status bar: the pictures are of the release, not of a build.
        await js(`(() => { const app = window.clonoloom; app.commit = ''; app.store.notify(['library']); return true; })()`);
        await scenes[name]();
        await clearToasts();
        await sleep(800);
        await showPlaces();
        await b.capture(join(IMAGES, `${name}-${theme}.webp`), { format: 'webp', quality: 86 });
        console.log(`${name} (${theme})`);
      } catch (error) {
        console.error(`${name} (${theme}) failed: ${error.message}`);
        process.exitCode = 1;
      } finally {
        await b?.close();
        await clonoloom?.stop();
      }
    }
  }
} finally {
  rmSync(temp, { recursive: true, force: true });
}
