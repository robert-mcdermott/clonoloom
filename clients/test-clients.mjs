// Real-host client validation, adapted from Apache-2.0 CytoWeave/MaveScape clients/test-clients.mjs.
// Generates its own seeded vaccine fixture. Review clicks belong only to this test driver.
// Usage: node clients/test-clients.mjs [--binary /absolute/path/to/clonoloom]
import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { once } from 'node:events';
import { existsSync, mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { launch, sleep } from '../validation/cdp.mjs';
const root = resolve(import.meta.dirname, '..');
const temp = mkdtempSync(join(tmpdir(), 'clonoloom-python-'));
const option = process.argv.indexOf('--binary');
const binary = option >= 0 ? resolve(process.argv[option + 1]) : join(temp, process.platform === 'win32' ? 'clonoloom.exe' : 'clonoloom');
async function exec(command, args, env = {}) {
  const child = spawn(command, args, { cwd: root, stdio: 'inherit', env: { ...process.env, ...env } });
  const [code] = await once(child, 'exit');
  assert.equal(code, 0, `${command} failed`);
}
let host, browser, child;
try {
  if (option < 0) await exec('go', ['build', '-o', binary, '.']);
  await exec(binary, ['simulate', '--preset', 'vaccine', '--out', join(temp, 'study'), '--quiet']);
  host = spawn(binary, ['--remote-control', '--allow', 'edits,compute,files', '--offline', '--window', 'none', '--port', '9490', '--data-dir', join(temp, 'library'), '--cache-dir', join(temp, 'cache')], { stdio: ['ignore', 'ignore', 'inherit'] });
  const connectionFile = join(temp, 'library/remote.json');
  for (let i = 0; i < 200 && !existsSync(connectionFile); i++) await sleep(50);
  const { url } = JSON.parse(readFileSync(connectionFile, 'utf8'));
  browser = await launch(); await browser.goto(url);
  for (let i = 0; i < 200 && !await browser.eval('Boolean(window.clonoloom)'); i++) await sleep(50);
  child = spawn('uv', ['run', '--no-project', '--with', 'pandas', join(root, 'clients/python/tests/live_client.py')], { cwd: root, stdio: ['ignore', 'inherit', 'inherit'], env: { ...process.env, PYTHONPATH: join(root, 'clients/python/src'), CLONOLOOM_TEST_LIBRARY: join(temp, 'library'), CLONOLOOM_TEST_STUDY: join(temp, 'study') } });
  let finished = false;
  const exit = once(child, 'exit').then(([code]) => { finished = true; return code; });
  const deadline = Date.now() + 180000;
  while (!finished && Date.now() < deadline) {
    await browser.eval(`(()=>{
      const proposal=document.querySelector('[data-automation-proposal]');
      if(!proposal || proposal.textContent.includes('Python cancellation control'))return false;
      if(proposal.textContent.includes('Python rejection control')) [...proposal.querySelectorAll('button')].find(b=>b.textContent==='Reject').click();
      else proposal.querySelector('button.primary').click();
      return true;
    })()`);
    await sleep(40);
  }
  assert(finished, 'Python workflow timed out');
  assert.equal(await exit, 0, 'Python live checks failed');
  const history = await browser.eval(`(()=>{const s=window.clonoloom.store;const label=s.undoLabel();s.undo();s.redo();return label})()`);
  assert(history.includes('Python research notebook'));
  assert(!await browser.eval(`Boolean(document.querySelector('[data-automation-proposal]'))`));
  await exec('uv', ['run', '--no-project', join(root, 'clients/examples/query_clonotypes.py'), '--data-dir', join(temp, 'library')], { PYTHONPATH: join(root, 'clients/python/src') });
  console.log('Client browser checks passed: named History and normal Undo/Redo; no proposal left open.');
} finally {
  if (child && child.exitCode === null) child.kill();
  if (browser) await browser.close();
  if (host && host.exitCode === null) { const stopped = once(host, 'exit'); host.kill(); await Promise.race([stopped, sleep(5000)]); }
  rmSync(temp, { recursive: true, force: true });
}
