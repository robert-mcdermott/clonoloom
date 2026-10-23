// Create the 0.4.0 compatibility project through the actual browser and remote actions.
// Uses an isolated temporary library; refuses to overwrite an existing fixture.
// node validation/create-project-0.4.mjs [new-output-directory]
import { gzipSync } from 'node:zlib';
import { spawn } from 'node:child_process';
import { cpSync, existsSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, writeFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { launch, sleep } from './cdp.mjs';

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const PORT = 8876;
const DATA = join(ROOT, 'validation/cache');
const output = resolve(process.argv[2] || 'validation/projects/0.4.0');
if (existsSync(output)) throw new Error('Refusing to overwrite a release fixture');
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


await new Promise((done, fail) => spawn('go', ['build', '-o', binary, '.'], { cwd: ROOT, stdio: 'inherit' }).on('exit', (code) => code ? fail(new Error('build failed')) : done()));
let server;
try {
  server = await startClonoLoom();
  b = await launch({ width: 1600, height: 1000 });
  await b.goto(URL, 800);
  await waitFor('Boolean(window.clonoloom?.remote)');
  await act('new_project', { name: 'The 0.4.0 project' });
  const data = join(temp, 'data'); mkdirSync(data);
  // Synthetic paired cells, constructed from the rule examples, not experimental samples.
  const alpha = [
    ['TRAV1-2','TRAJ33','CAVRDSNYQLIW'],
    ['TRAV1-2','TRAJ33','CAVMDSNYQLIW'],
    ['TRAV10','TRAJ18','CVVSDRGSTLGRLYF'],
    ['TRAV1-2','TRAJ9','CAVRNTGGFKTIF'],
  ];
  let tsv = 'sequence_id\trepertoire_id\tcell_id\tlocus\tv_call\tj_call\tjunction_aa\tproductive\tumi_count\n';
  for (let i = 0; i < 12; i++) {
    const [v,j,aa] = alpha[i % 4];
    tsv += `a${i}\tSynthetic\tc${i}\tTRA\t${v}\t${j}\t${aa}\tT\t3\n`;
    tsv += `b${i}\tSynthetic\tc${i}\tTRB\t${i % 4 === 2 ? 'TRBV25-1' : 'TRBV5-1'}\tTRBJ1-1\tCASSLAGGPEAFF\tT\t4\n`;
  }
  const paired = join(data, 'synthetic-cells.tsv'); writeFileSync(paired, tsv);
  await act('open_data', { paths: [paired], name: 'Synthetic invariant-rule examples' });
  await act('set_definition', { definition: { name: 'Paired alpha beta', chains: ['TRA','TRB'], junction: 'aa', v: 'gene', j: 'gene', cells: 'primary', productiveOnly: true } });
  const inv = await act('flag_invariant', { species: 'human' });
  if (inv.data.invariant.samples[0].matched.join(',') !== '6,3,3') throw new Error('Synthetic flags disagree');
  await act('find_neighbors', { cdr3Only: true, radius: 24, organism: 'human' });
  await act('find_networks', { metric: 'hamming', cutoff: 1, arms: 'all', sameV: true, sameJ: true });
  const bulk = join(data, 'simulated.vdjtools.txt');
  cpSync(join(ROOT, 'validation/projects/0.2.0/data/S02-1.vdjtools.txt'), bulk);
  await act('open_data', { paths: [bulk], name: 'Simulated beta chains', mappings: { [bulk]: { columns: { count: 'count', v_call: 'v', d_call: 'd', j_call: 'j', junction: 'cdr3nt', junction_aa: 'cdr3aa' }, unit: 'umis', sample: 'S02-1' } } });
  await act('measure_generation', { models: 'clonoloom', organism: 'human', genes: true });
  const project = await js('window.clonoloom.store.project');
  mkdirSync(output, { recursive: true }); cpSync(data, join(output, 'data'), { recursive: true });
  for (const d of project.datasets) for (const f of [...d.sources, ...(d.metadata ?? [])]) f.path = `data/${f.path.split('/').at(-1)}`;
  writeFileSync(join(output, 'project.json'), JSON.stringify(project));
  writeFileSync(join(output, 'invariant.json'), JSON.stringify(inv.data.invariant));
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
