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
import { existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync } from 'node:fs';
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
  const server = spawn(binary, ['--remote-control', '--window', 'none', '--port', String(PORT), '--data-dir', library, '--examples-dir', examples], { cwd: temp, stdio: ['ignore', 'pipe', 'inherit'] });
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
