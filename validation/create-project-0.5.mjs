// Create the 0.5.0 compatibility project through the actual browser and remote actions.
// Adapted from create-project-0.4.mjs.
// Uses an isolated temporary library; refuses to overwrite an existing fixture.
// node validation/create-project-0.5.mjs [new-output-directory]
import { acceptAutomation, waitForAutomationIdle } from './automation.mjs';
import { gzipSync } from 'node:zlib';
import { spawn } from 'node:child_process';
import { cpSync, existsSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, writeFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { launch, sleep } from './cdp.mjs';

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const PORT = 8876;
const output = resolve(process.argv[2] || 'validation/projects/0.5.0');
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
  const server = spawn(binary, ['--remote-control', '--allow', 'edits,compute,files,network', '--window', 'none', '--port', String(PORT), '--data-dir', library, '--examples-dir', examples, '--cache-dir', join(temp, 'cache')], { cwd: temp, stdio: ['ignore', 'pipe', 'inherit'] });
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
  await waitForAutomationIdle(b);
  const response = await acceptAutomation(fetch(`${URL}api/remote/action`, { method: 'POST', headers: { 'Content-Type': 'application/json', 'X-ClonoLoom-Token': token }, body: JSON.stringify({ action, args: actionArgs, client: 'validation/create-project-0.5' }) }), b);
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
  const study = join(examples, 'simulated-bcell-immunization');
  await new Promise((done, fail) => spawn(binary, ['simulate', '--preset', 'bcell-immunization', '--out', study, '--quiet'], { stdio: 'inherit' }).on('exit', (code) => code ? fail(new Error('simulation failed')) : done()));
  server = await startClonoLoom({ examples });
  b = await launch({ width: 1600, height: 1000 });
  await b.goto(URL, 800);
  await waitFor('Boolean(window.clonoloom?.remote)');
  await act('new_project', { name: 'The 0.5.0 project' });
  const data = join(temp, 'data'); mkdirSync(data);
  // Two cells per observed state/sample from three known families, across two subjects.
  // This preserves paired chains, both light loci, isotypes and duplicate observed states.
  const observations = readFileSync(join(study, 'truth/observations.tsv'), 'utf8').trimEnd().split('\n');
  const counts = new Map(), cells = new Set(), selected = [observations[0]];
  for (const line of observations.slice(1)) {
    const [sample, cell, node] = line.split('\t');
    if (!['S01-F001', 'S01-F002', 'S02-F001'].includes(node.slice(0, 8))) continue;
    const key = sample + '/' + node, count = counts.get(key) || 0;
    if (count >= 2) continue;
    counts.set(key, count + 1); cells.add(cell); selected.push(line);
  }
  if (!cells.size) throw new Error('No artificial observations selected');
  const paths = [], samples = new Set();
  for (const file of readdirSync(study).filter((f) => f.endsWith('.tsv'))) {
    const lines = readFileSync(join(study, file), 'utf8').trimEnd().split('\n');
    const cellColumn = lines[0].split('\t').indexOf('cell_id');
    const rows = lines.slice(1).filter((line) => cells.has(line.split('\t')[cellColumn]));
    if (!rows.length) continue;
    const dest = join(data, file + '.gz');
    writeFileSync(dest, gzipSync([lines[0], ...rows, ''].join('\n')));
    paths.push(dest); samples.add(file.slice(0, -4));
  }
  const metadata = JSON.parse(readFileSync(join(study, 'repertoires.airr.json'), 'utf8'));
  metadata.Repertoire = metadata.Repertoire.filter((r) => samples.has(r.repertoire_id));
  const meta = join(data, 'repertoires.airr.json');
  writeFileSync(meta, JSON.stringify(metadata)); paths.push(meta);
  await act('open_data', { paths, name: 'Artificial immunization: three families' });
  await act('find_bcell_families');
  await act('find_bcell_families', { threshold: 0.1 });
  await act('reconstruct_bcell_germlines', { reference: 'simulated-bcell-immunization' });
  await act('measure_bcell_shm');
  await act('build_bcell_lineages');
  await act('summarize_bcell_isotypes');
  const imgt = join(data, 'imgt-inputs');
  cpSync(join(ROOT, 'testdata/imgt-inputs'), imgt, { recursive: true });
  await act('open_data', { paths: [imgt], name: 'Synthetic IMGT annotations' });
  await act('find_bcell_families', { scope: 'sample', threshold: 0.1, mode: 'heavy' });
  const project = await js('window.clonoloom.store.project');
  mkdirSync(output, { recursive: true }); cpSync(data, join(output, 'data'), { recursive: true });
  for (const d of project.datasets) for (const f of [...d.sources, ...(d.metadata ?? [])]) f.path = `data/${f.path.slice(data.length + 1).replaceAll('\\', '/')}`;
  writeFileSync(join(output, 'project.json'), JSON.stringify(project));
  const catalog = await (await fetch(URL + 'api/examples')).json();
  const reference = catalog.examples.find((e) => e.name === 'simulated-bcell-immunization');
  // Pin the catalog provenance as well as the FASTA. A future software citation must not
  // silently replace the release's recorded reference source when remaking its results.
  writeFileSync(join(output, 'reference.json'), JSON.stringify({ name: reference.name, source: reference.source, license: reference.license, citation: reference.citation }));
  cpSync(join(study, 'reference'), join(output, 'references', reference.name, 'reference'), { recursive: true });
  writeFileSync(join(output, 'observations.tsv'), selected.join('\n') + '\n');
  for (const artifact of project.artifacts) {
    const src = join(places.library, 'artifacts', artifact.id);
    const dst = join(output, 'artifacts', artifact.id); mkdirSync(dst, { recursive: true });
    for (const file of readdirSync(src)) {
      const bytes = readFileSync(join(src, file));
      writeFileSync(join(dst, file === 'manifest.json' ? file : `${file}.gz`), file === 'manifest.json' ? bytes : gzipSync(bytes));
    }
  }
  console.log(`Saved ${output}: ${project.datasets.length} datasets, ${project.artifacts.length} results`);
} finally {
  await b?.close(); await server?.stop(); rmSync(temp, { recursive: true, force: true });
}
