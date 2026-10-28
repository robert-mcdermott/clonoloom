// Certificates are prepared and checked in Go; this view displays those records.
import { h, icon, downloadBlob, formatBytes } from './dom.js';
import { confirmDialog } from './overlays.js';
import { certificateResults, verificationLabel } from '../lib/certificates.js';
import { mountCheckpoints } from './checkpoints.js';
import { mountProjectArchives } from './project-archives.js';
import { mountHTMLReports } from './html-reports.js';

export function mountReports(app, container) {
  const { store } = app;
  const body = h('div.view-body');
  const root = h('div.view', h('div.workbench-head', h('h1', icon('report'), 'Reports')), h('div.workbench-scroll', body));
  container.append(root);
  const archives = mountProjectArchives(app, body);
  const checkpoints = mountCheckpoints(app, body);
  const htmlReports = mountHTMLReports(app, body);
  let chosen = '', record = null, report = null, busy = false, error = '', generation = store.state.generation, alive = true;
  let verificationMode = 'integrity', controller = null;
  let reportText = '';
  const current = () => certificateResults(store.project);
  const same = () => { const check = store.sameProject(); return () => alive && check(); };
  const button = (title, action, disabled = false) => h('button.btn.small', { type: 'button', disabled: busy || disabled, onclick: () => action().catch((e) => { error = e.message; render(); }) }, title);
  async function inspect(id = chosen, { accept = false, prompt = false } = {}) {
    if (busy) throw new Error('Wait for the certificate operation.');
    if (!current().some((r) => r.id === id)) throw new Error('Choose a supported saved result in this project.');
    const valid = same(); chosen = id; record = null; report = null; error = ''; busy = true; render();
    try {
      const url = `api/certificates/${encodeURIComponent(id)}`;
      let response = await fetch(`${url}${accept ? '?accept=1' : ''}`, { cache: 'no-store' });
      let value = await response.json();
      if (!valid()) throw new Error('The project changed.');
      if (!response.ok && value.code === 'restricted' && prompt && !accept) {
        accept = await confirmDialog({ title: 'Evidence may not be shared', message: value.error, confirm: 'Prepare for my study' });
        if (!valid()) throw new Error('The project changed.');
        if (!accept) return null;
        response = await fetch(`${url}?accept=1`, { cache: 'no-store' }); value = await response.json();
      }
      if (!response.ok) throw new Error(value.error ?? 'Certificate preparation failed.');
      if (!valid()) throw new Error('The project changed.');
      record = value; return { ...value, accept };
    } finally { if (valid()) { busy = false; render(); } }
  }
  async function exportCertificate(id = chosen, options = {}) {
    const valid = same();
    const prepared = await inspect(id, options); if (!prepared || !valid()) return null;
    busy = true; render();
    try {
      const response = await fetch(`api/certificates/${encodeURIComponent(id)}/download${prepared.accept ? '?accept=1' : ''}`);
      if (!response.ok) throw new Error((await response.json()).error ?? 'Certificate export failed.');
      if (response.headers.get('X-ClonoLoom-Certificate') !== prepared.certificate.fingerprint) throw new Error('The certificate changed between inspection and download. Inspect it again.');
      const blob = await response.blob(); if (!valid()) throw new Error('The project changed.');
      downloadBlob(blob, `${id}.certificate.clz`);
      app.log(`Exported integrity certificate for ${id}; no scientific recomputation.`);
      return prepared.certificate;
    } finally { if (valid()) { busy = false; render(); } }
  }
  async function verify(file, expected = '', mode = 'integrity') {
    if (busy) throw new Error('Wait for the certificate operation.');
    if (!file || file.size > 520 * 1024 * 1024) throw new Error('Choose a certificate no larger than 520 MiB.');
    if (expected && !/^[a-f0-9]{64}$/.test(expected)) throw new Error('Expected fingerprint must be 64 lowercase hexadecimal characters.');
    if (!['integrity', 'replay'].includes(mode)) throw new Error('Choose integrity or replay.');
    const valid = same(); controller = new AbortController(); verificationMode = mode; busy = true; error = ''; report = null; render();
    try {
      const response = await fetch(`api/certificates/verify?mode=${mode}${expected ? `&expect=${expected}` : ''}`, { method: 'POST', headers: { 'Content-Type': 'application/zip' }, body: file, signal: controller.signal });
      const text = await response.text(), value = JSON.parse(text); if (!response.ok) throw new Error(value.error ?? 'Certificate could not be read.');
      if (!valid()) throw new Error('The project changed.'); report = value; reportText = text; return value;
    } finally { if (valid()) { controller = null; busy = false; render(); } }
  }
  function render() {
    if (!alive) return;
    const records = current();
    if (!records.some((r) => r.id === chosen)) chosen = records.at(-1)?.id ?? '';
    // Keep the checkpoint component mounted while certificate state rerenders.
    for (const child of [...body.children]) if (!child.classList.contains('checkpoints-view') && !child.classList.contains('html-reports-view') && !child.classList.contains('project-archives-view')) child.remove();
    const select = h('select.input', { 'aria-label': 'Certificate result', disabled: busy, onchange: (e) => { chosen = e.target.value; record = null; error = ''; render(); } },
      ...records.map((r) => h('option', { value: r.id, selected: r.id === chosen }, `${r.name} · ${r.id}`)));
    body.append(h('section.card', h('h2', 'Saved-result evidence certificates'),
      h('p', 'Keep the exact saved recipe, output files, source fingerprints and provider notice in one checkable archive. Available for cohort feature tables, comparisons, count models, batch QC and motif screens.'),
      h('p.callout', 'Integrity checks compare recorded bytes. They do not recompute a statistic, verify original sources or separate parents, establish authenticity, or validate the study design. Captured evidence may contain raw counts, sequences, subject identifiers and paths; it is not de-identified.'),
      records.length ? h('label.field', h('span', 'Saved result'), select) : h('p.muted', 'Save a supported cohort, batch QC or motif result to prepare its certificate. Existing results of other kinds remain unchanged.'),
      h('div.btn-row', button('Inspect certificate', () => inspect(chosen, { prompt: true }), !chosen), button('Export integrity certificate', () => exportCertificate(chosen, { prompt: true }), !chosen))));
    if (error) body.append(h('p.callout.danger', { role: 'alert' }, error));
    if (record) body.append(h('section.card.certificate-record', h('h2', 'Recorded evidence'),
      h('p', `Artifact: ${record.certificate.artifact} · ${record.certificate.kind}`),
      h('p', 'Certificate fingerprint (SHA-256):'), h('p.mono.certificate-fingerprint', record.certificate.fingerprint),
      h('p.muted', 'Keep this fingerprint separately when sharing. Anyone able to change the archive can also replace its internal checksums.'),
      h('div.table-wrap', { tabIndex: 0 }, h('table.data', h('caption', 'Complete archive inventory'),
        h('thead', h('tr', h('th', { scope: 'col' }, 'File'), h('th', { scope: 'col' }, 'Size'), h('th', { scope: 'col' }, 'SHA-256'))),
        h('tbody', ...record.certificate.files.map((f) => h('tr', h('td', f.path), h('td', formatBytes(f.size)), h('td.mono', f.sha256))))))));
    const expected = h('input.input', { 'aria-label': 'Expected certificate fingerprint', placeholder: 'Optional independently recorded SHA-256', disabled: busy });
    const terminal = h('p.muted', `Terminal: clonoloom verify --${verificationMode} FILE.certificate.clz`);
    const method = h('select.input', { 'aria-label': 'Certificate check', disabled: busy, onchange: (e) => { verificationMode = e.target.value; terminal.textContent = `Terminal: clonoloom verify --${verificationMode} FILE.certificate.clz`; } },
      h('option', { value: 'integrity', selected: verificationMode === 'integrity' }, 'Integrity only — saved bytes'),
      h('option', { value: 'replay', selected: verificationMode === 'replay' }, 'Exact replay — captured evidence'));
    const upload = h('input', { type: 'file', accept: '.clz,.zip', 'aria-label': 'Certificate file', disabled: busy, onchange: (e) => { const valid = same(); verify(e.target.files[0], expected.value.trim(), verificationMode).catch((e) => { if (valid()) { error = e.name === 'AbortError' ? 'Verification cancelled.' : e.message; render(); } }); } });
    body.append(h('section.card.certificate-check', h('h2', 'Check or replay a certificate'), h('p', 'Integrity checks saved bytes. Replay runs the supported Go workers in a disposable store, checking embedded parent hashes and exact outputs. Original receptor files and upstream data capture are not checked or rerun; the report names each replay boundary. No recorded source paths are opened.'),
      h('label.field', h('span', 'Check to perform'), method),
      h('label.field', h('span', 'Expected fingerprint (optional)'), expected), h('label.field', h('span', 'Certificate archive'), upload),
      controller ? h('button.btn.small', { type: 'button', onclick: () => controller?.abort() }, 'Cancel verification') : null,
      terminal));
    const integrity = report?.integrity ?? report;
    if (report) body.append(h('section.card.certificate-verdict', h('h2', verificationLabel(report)),
      h('p', `${integrity.filesChecked} files · ${formatBytes(integrity.bytesChecked)} checked. ${report.format === 'clonoloom-replay' ? 'Replay steps and limits are recorded below.' : 'Scientific recomputation: not performed.'}`),
      h('p', report.limits), ...report.findings.map((f) => h('p.callout.danger', f)),
      ...(report.steps ?? []).map((s) => h('div', h('h3', `${s.kind}: ${s.verdict}`), h('p', `Scientific metadata: ${s.metadataMatched ? 'exact match' : 'differs'}.`), h('p.mono.certificate-fingerprint', `Expected SHA-256: ${s.expectedSHA256}`), h('p.mono.certificate-fingerprint', `Recomputed SHA-256: ${s.actualSHA256 ?? 'unavailable'}`))),
      report.verifier ? h('p.muted', `Verifier: ${report.verifier.name} ${report.verifier.version} (${report.platform}). Original provenance is retained separately in the JSON report.`) : null,
      button('Export verification JSON', async () => downloadBlob(new Blob([reportText], { type: 'application/json' }), 'certificate-verification.json'))));
  }
  const view = { inspect, exportCertificate, verify, state: () => ({ record, report, reportText, busy, error }) };
  app.certificatesView = view; render();
  return { update(topics) {
    archives.update(topics);
    checkpoints.update(topics);
    htmlReports.update(topics);
    if (generation !== store.state.generation) { controller?.abort(); controller = null; generation = store.state.generation; chosen = ''; record = null; report = null; busy = false; error = ''; }
    if (topics.has('project') || topics.has('mode')) render();
  }, destroy() { alive = false; controller?.abort(); archives.destroy(); checkpoints.destroy(); htmlReports.destroy(); if (app.certificatesView === view) app.certificatesView = null; root.remove(); } };
}
