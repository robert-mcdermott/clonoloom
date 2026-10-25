// Create the 0.6.0 compatibility project through the actual browser and remote actions.
// Adapted from create-project-0.5.mjs.
// Uses an isolated temporary library; refuses to overwrite an existing fixture.
// node validation/create-project-0.6.mjs [new-output-directory]
import { gzipSync } from 'node:zlib';
import { spawn } from 'node:child_process';
import { cpSync, existsSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, writeFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { launch, sleep } from './cdp.mjs';

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const PORT = 8876;
const output = resolve(process.argv[2] || 'validation/projects/0.6.0');
if (existsSync(output)) throw new Error('Refusing to overwrite a release fixture');
// --- ClonoLoom -----------------------------------------------------------------------------------

const temp = mkdtempSync(join(tmpdir(), 'clonoloom-capture-'));
const binary = join(temp, process.platform === 'win32' ? 'clonoloom.exe' : 'clonoloom');
let URL = '';
let token = '';
let places = {};

// Start on an empty library, with an artificial study made by this build.
async function startClonoLoom({ examples }) {
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
  const response = await fetch(`${URL}api/remote/action`, { method: 'POST', headers: { 'Content-Type': 'application/json', 'X-ClonoLoom-Token': token }, body: JSON.stringify({ action, args: actionArgs, client: 'validation/create-project-0.6' }) });
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


await new Promise((done, fail) => spawn('go', ['build', '-o', binary, '.'], { cwd: ROOT, stdio: 'inherit' }).on('exit', (code) => code ? fail(new Error('build failed')) : done()));
let server;
try {
  const examples = join(temp, 'examples');
  server = await startClonoLoom({ examples });
  b = await launch({ width: 1600, height: 1000 });
  await b.goto(URL, 800);
  await waitFor('Boolean(window.clonoloom?.remote)');
  await act('new_project', { name: 'The 0.6.0 project' });
  await act('open_example', { example: 'simulated-cell-states-clouds', download: true });
  const catalog = await (await fetch(URL + 'api/examples')).json();
  const example = catalog.examples.find((e) => e.name === 'simulated-cell-states-clouds');
  for (const step of example.opens[0].steps) {
    await act(step.action, step.args);
    console.log(`Guided step: ${step.title}`);
  }
  await act('close_dialogs');
  await js('window.clonoloom.saveNow()');
  const project = await js('window.clonoloom.store.project');
  const kinds = project.artifacts.map((a) => a.kind);
  if (JSON.stringify(kinds) !== JSON.stringify(['expression-map', 'phenotypes', 'modularity', 'conga', 'anndata-export'])) throw new Error(`Incomplete study: ${kinds}`);
  mkdirSync(output, { recursive: true });
  const study = join(examples, 'simulated-cell-states-clouds');
  cpSync(study, join(output, 'data'), { recursive: true });
  // The source location map is separate from scientific provenance. Artifact documents and
  // manifests retain their original bytes; only the project's reopening paths are portable.
  writeFileSync(join(output, 'locations.json'), JSON.stringify({ original: study, fixture: 'data' }));
  const portable = (path) => {
    if (!path.startsWith(study + '/')) throw new Error(`Unexpected source: ${path}`);
    return `data/${path.slice(study.length + 1)}`;
  };
  for (const d of project.datasets) for (const f of [...d.sources, ...(d.metadata ?? [])]) f.path = portable(f.path);
  for (const e of project.expression) {
    e.path = portable(e.path); if (e.cellsPath) e.cellsPath = portable(e.cellsPath);
    for (const source of e.sources) source.path = portable(source.path);
  }
  writeFileSync(join(output, 'project.json'), JSON.stringify(project));
  for (const artifact of project.artifacts) {
    const src = join(places.library, 'artifacts', artifact.id);
    const dst = join(output, 'artifacts', artifact.id); mkdirSync(dst, { recursive: true });
    for (const file of readdirSync(src)) {
      const bytes = readFileSync(join(src, file));
      writeFileSync(join(dst, file === 'manifest.json' ? file : `${file}.gz`), file === 'manifest.json' ? bytes : gzipSync(bytes));
    }
  }
  console.log(`Saved ${output}: ${project.datasets.length} dataset, ${project.expression.length} expression input, ${project.artifacts.length} results`);
} finally {
  await b?.close(); await server?.stop(); rmSync(temp, { recursive: true, force: true });
}
