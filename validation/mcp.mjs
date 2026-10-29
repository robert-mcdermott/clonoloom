// Real stdio subprocess + normal window. Test-only clicks review visible proposals;
// no client-side approval capability or scientific implementation is added.
// Usage: node validation/mcp.mjs /path/to/clonoloom [--capture]
import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { once } from 'node:events';
import { existsSync, mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { launch, sleep } from './cdp.mjs';
import { acceptAutomation, waitForAutomationIdle } from './automation.mjs';
const binary = resolve(process.argv[2] ?? '/tmp/clonoloom-slice7');
const capture = process.argv.includes('--capture');
const root = resolve(import.meta.dirname, '..');
const temp = mkdtempSync(join(tmpdir(), 'clonoloom-mcp-'));
let browser;
const host = spawn(binary, ['mcp', '--window', 'none', '--offline', '--port', '9470', '--data-dir', join(temp, 'library'), '--cache-dir', join(temp, 'cache'), '--allow', 'edits,compute,files'], { stdio: ['pipe', 'pipe', 'pipe'] });
const exited = once(host, 'exit');
let stderr = '', buffer = '', next = 0, protocolError;
const waiting = new Map();
const replies = new Set();
host.stdout.setEncoding('utf8');
host.stderr.setEncoding('utf8');
host.stderr.on('data', (chunk) => { stderr += chunk; });
host.stdout.on('data', (chunk) => {
  buffer += chunk.toString();
  while (buffer.includes('\n')) {
    const end = buffer.indexOf('\n'), line = buffer.slice(0, end); buffer = buffer.slice(end + 1);
    try {
      const msg = JSON.parse(line);
      assert.equal(msg.jsonrpc, '2.0');
      if (msg.id !== undefined) {
        assert(!replies.has(msg.id), 'duplicate response'); replies.add(msg.id);
        const pending = waiting.get(msg.id); assert(pending, 'unsolicited response'); waiting.delete(msg.id);
        pending.resolve(msg);
      } else assert.equal(msg.method, 'notifications/progress');
    } catch (error) { protocolError = error; for (const p of waiting.values()) p.reject(error); }
  }
});
const meta = () => ({ 'io.modelcontextprotocol/protocolVersion': '2026-07-28', 'io.modelcontextprotocol/clientCapabilities': {}, 'io.modelcontextprotocol/clientInfo': { name: 'Research notebook · MCP', version: '1' } });
function request(method, params = {}, modern = true) {
  const id = ++next;
  const promise = new Promise((resolve, reject) => waiting.set(id, { resolve, reject }));
  host.stdin.write(`${JSON.stringify({ jsonrpc: '2.0', id, method, params: modern ? { ...params, _meta: { ...meta(), ...params._meta } } : params })}\n`);
  return { id, promise };
}
const notify = (method, params = {}) => host.stdin.write(`${JSON.stringify({ jsonrpc: '2.0', method, params })}\n`);
async function waitFor(expression) {
  const deadline = Date.now() + 30000;
  while (Date.now() < deadline) { if (await browser.eval(expression)) return; await sleep(50); }
  throw new Error(`Timed out: ${expression}`);
}
async function call(name, args = {}) {
  await waitForAutomationIdle(browser);
  const response = await acceptAutomation(request('tools/call', { name, arguments: args }).promise, browser);
  assert(!response.error, JSON.stringify(response));
  assert.equal(response.result.isError, false, JSON.stringify(response));
  assert.deepEqual(JSON.parse(response.result.content[0].text), response.result.structuredContent);
  return response.result.structuredContent;
}
async function screenshot(name) {
  if (!capture) return;
  // Match the documentation capture convention: show a stable illustrative library path.
  // Only presentation text changes; no project, result or scientific value is modified.
  await browser.eval(`(()=>{
    document.querySelectorAll('.toast').forEach(t=>t.remove());
    const from=${JSON.stringify(join(temp, 'library'))},to='/path/to/mcp-library';
    const walker=document.createTreeWalker(document.body,NodeFilter.SHOW_TEXT);
    while(walker.nextNode())walker.currentNode.nodeValue=walker.currentNode.nodeValue.split(from).join(to);
    for(const el of document.querySelectorAll('[title]'))el.title=el.title.split(from).join(to);
    return true;
  })()`);
  await browser.capture(join(root, `docs/images/${name}.webp`), { format: 'webp', quality: 86 });
}
async function a11y() {
  await sleep(300);
  await browser.eval(`(0, eval)(${JSON.stringify(readFileSync(join(root, 'validation/cache/axe-core/axe.min.js'), 'utf8'))}); true`);
  const violations = await browser.eval(`(async()=> (await axe.run(document, {runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21a','wcag21aa']},resultTypes:['violations']})).violations.map(v=>({id:v.id,nodes:v.nodes.map(n=>n.target)})))()`);
  assert.deepEqual(violations, []);
}
try {
  const discover = await request('server/discover').promise;
  assert.equal(discover.result.resultType, 'complete');
  const catalog = (await request('tools/list').promise).result.tools;
  assert(catalog.some((t) => t.name === 'query_clonotypes'));
  assert(!catalog.some((t) => t.name === 'open_example'), 'network grant unexpectedly available');
  const refused = await request('tools/call', { name: 'open_example', arguments: { example: 'simulated-vaccine' } }).promise;
  assert.equal(refused.result.structuredContent.code, 'permission-denied');
  for (let i = 0; i < 200 && !existsSync(join(temp, 'library/remote.json')); i++) await sleep(50);
  const { url } = JSON.parse(readFileSync(join(temp, 'library/remote.json')));
  browser = await launch(); await browser.goto(url);
  await waitFor('Boolean(window.clonoloom)');
  await call('new_project', { name: 'Artificial vaccination repertoire · MCP review' });
  await call('open_data', { paths: ['S01-pre-1.tsv', 'S01-d14.tsv', 'S01-d28.tsv'].map((p) => join(root, 'validation/cache/simulated-vaccine', p)), name: 'Simulated vaccination · subject S01' });
  const fullRaw = (await call('query_clonotypes', { limit: 1000 })).data.resultJSON;
  const full = JSON.parse(fullRaw); assert(full.count > 100);
  const selected = full.rows[3];
  const selectedRaw = (await call('query_clonotypes', { where: { field: 'id', op: 'eq', value: selected.id }, limit: 1 })).data.resultJSON;
  const filtered = JSON.parse(selectedRaw);
  assert.equal(filtered.count, full.count); assert.equal(filtered.matched, 1);
  assert.equal(filtered.definitionId, full.definitionId); assert.equal(filtered.rows[0].rank, 1);
  assert.deepEqual({ ...filtered.rows[0], rank: selected.rank }, selected);
  // Compare the exact native query string returned via HTTP to MCP under the same state.
  const connection = JSON.parse(readFileSync(join(temp, 'library/remote.json')));
  const http = await fetch(`${url}/api/remote/action`, { method: 'POST', headers: { 'Content-Type': 'application/json', 'X-ClonoLoom-Token': connection.token }, body: JSON.stringify({ action: 'query_clonotypes', args: { limit: 1000 }, client: 'Transport parity control' }) }).then((r) => r.json());
  assert.equal(http.data.resultJSON, fullRaw, 'HTTP/MCP numeric tokens or scientific selection differ');
  await call('show', { view: 'Clonotypes' });
  await call('set_theme', { theme: 'system' });
  for (const theme of ['light', 'dark']) {
    await browser.theme(theme === 'dark');
    await waitForAutomationIdle(browser);
    const pending = request('tools/call', { name: 'set_definition', arguments: { preset: 'Nucleotide junction' } });
    await waitFor(`Boolean(document.querySelector('[data-automation-proposal]'))`);
    await a11y();
    await screenshot(`mcp-${theme}`);
    await browser.eval(`(()=>{[...document.querySelectorAll('[data-automation-proposal] button')].find(b=>b.textContent==='Reject').click();return true})()`);
    assert.equal((await pending.promise).result.structuredContent.code, 'rejected');
    await browser.eval(`(()=>{window.clonoloom.store.setUI({drawer:{open:true,tab:'automation'}},['drawer']);return true})()`);
    await sleep(100); await a11y();
    await screenshot(`mcp-audit-${theme}`);
    await browser.eval(`(()=>{window.clonoloom.store.setUI({drawer:{open:false}},['drawer']);return true})()`);
  }
  const cancelled = request('tools/call', { name: 'set_definition', arguments: { preset: 'Nucleotide junction' } });
  await waitFor(`Boolean(document.querySelector('[data-automation-proposal]'))`);
  notify('notifications/cancelled', { requestId: cancelled.id });
  await waitFor(`!document.querySelector('[data-automation-proposal]')`);
  await request('ping').promise; assert(!replies.has(cancelled.id)); waiting.delete(cancelled.id);
  await call('set_definition', { preset: 'Nucleotide junction' });
  assert(await browser.eval(`window.clonoloom.store.undoLabel().includes('Research notebook · MCP')`));
  await browser.eval(`(()=>{const s=window.clonoloom.store;s.undo();s.redo();return true})()`);
  const init = await request('initialize', { protocolVersion: '2025-11-25', capabilities: {}, clientInfo: { name: 'Legacy notebook', version: '1' } }, false).promise;
  assert.equal(init.result.protocolVersion, '2025-11-25'); notify('notifications/initialized');
  const legacy = await request('tools/call', { name: 'get_state' }, false).promise;
  assert.equal(legacy.result.isError, false); assert(!('resultType' in legacy.result));
  const state = await call('get_state'); assert(state.ok);
  // EOF with a pending proposal cancels review and removes the owned connection file.
  request('tools/call', { name: 'set_definition', arguments: { preset: 'Amino-acid junction, V and J genes' } });
  await waitFor(`Boolean(document.querySelector('[data-automation-proposal]'))`);
  host.stdin.end();
  const [code] = await Promise.race([exited, sleep(15000).then(() => { throw new Error('EOF did not stop host'); })]);
  assert.equal(code, 0, stderr); assert(!existsSync(join(temp, 'library/remote.json')));
  assert.equal(buffer, ''); assert(!protocolError, String(protocolError));
  console.log('MCP browser checks passed: both protocols, grants, exact HTTP parity, fixed denominator, review/rejection/cancellation, history/Undo/Redo, both themes, accessibility and EOF cleanup.');
} finally {
  if (browser) await browser.close();
  if (host.exitCode === null) host.kill();
  rmSync(temp, { recursive: true, force: true });
}
