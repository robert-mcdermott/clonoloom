// Local HTTP fixtures and installed R package check. No host grants or user data.
import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { spawn } from 'node:child_process';
import { once } from 'node:events';
import { mkdtempSync, readFileSync, readdirSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
const root = resolve(import.meta.dirname, '..');
const catalog = readFileSync(join(root, 'clients/tools.json'), 'utf8');
const native = '{"rows":[{"total":9007199254740993,"frequency":0.12345678901234567890123456789,"missing":null}]}';
let config = {}, requests = [];
const server = createServer(async (req, res) => {
  let body = ''; for await (const part of req) body += part;
  if (req.url === '/control') { config = JSON.parse(body || '{}'); requests = []; res.end('{}'); return; }
  if (req.url === '/requests') { res.end(JSON.stringify(requests)); return; }
  requests.push({ method: req.method, token: req.headers['x-clonoloom-token'], body });
  if (req.method === 'GET') {
    let payload = catalog.slice(0, -2) + ', "version":"test", "allowed":{"read":true,"compute":true}}';
    for (const key of ['api', 'catalogVersion', 'policyVersion']) if (config[key]) payload = payload.replace(new RegExp(`"${key}": \\d+`), `"${key}":${config[key]}`);
    res.setHeader('Content-Type', 'application/json'); res.end(payload); return;
  }
  const current = config;
  if (current.delay) await new Promise(resolve => setTimeout(resolve, current.delay));
  res.writeHead(current.status || 200, { 'Content-Type': current.type || 'application/json', Location: '/redirect-target' });
  res.end(current.oversize ? ' '.repeat(16 * 1024 * 1024 + 1) : current.body ?? JSON.stringify({ok:true,message:'ok',data:{resultJSON:native}}));
});
server.listen(0, '127.0.0.1'); await once(server, 'listening');
const temp = mkdtempSync(join(tmpdir(), 'clonoloom-r-check-'));
async function run(command, args, cwd = root) {
  const child = spawn(command, args, { cwd, stdio: 'inherit', env: { ...process.env, CLONOLOOM_FAKE_URL: `http://127.0.0.1:${server.address().port}`, _R_CHECK_FORCE_SUGGESTS_: 'false', _R_CHECK_CRAN_INCOMING_REMOTE_: 'false' } });
  const [code] = await once(child, 'exit'); assert.equal(code, 0, `${command} ${args.join(' ')} failed`);
}
try {
  await run('Rscript', ['-e', 'testthat::test_local("clients/r", stop_on_failure=TRUE)']);
  if (!process.argv.includes('--tests-only')) {
    await run('R', ['CMD', 'build', join(root, 'clients/r')], temp);
    const archive = readdirSync(temp).find(f => f.endsWith('.tar.gz'));
    await run('R', ['CMD', 'check', '--no-manual', '--no-vignettes', archive], temp);
    const log = readFileSync(join(temp, 'clonoloom.Rcheck/00check.log'), 'utf8');
    assert(!/Status: .*?(WARNING|ERROR|NOTE)/.test(log), log);
  }
  console.log('R client fake HTTP, exact values, installed package and check passed.');
} finally { server.closeAllConnections(); server.close(); rmSync(temp, { recursive: true, force: true }); }
