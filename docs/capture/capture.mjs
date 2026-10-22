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
  // The simulated vaccination study's neighborhoods against a background drawn from OLGA's human
  // TRB model (the reference data put in the library, as a download leaves them), the most
  // enriched first.
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
