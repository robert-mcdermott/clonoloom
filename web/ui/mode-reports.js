// Certificates are prepared and checked in Go; this view displays those records.
import { h, icon, clear, downloadBlob, formatBytes } from './dom.js';
import { confirmDialog } from './overlays.js';
import { certificateResults, integrityLabel } from '../lib/certificates.js';

export function mountReports(app, container) {
  const { store } = app;
  const body = h('div.view-body');
  const root = h('div.view', h('div.workbench-head', h('h1', icon('report'), 'Reports')), h('div.workbench-scroll', body));
  container.append(root);
  let chosen = '', record = null, report = null, busy = false, error = '', generation = store.state.generation, alive = true;
  const current = () => certificateResults(store.project);
  const same = () => { const check = store.sameProject(); return () => alive && check(); };
  const button = (title, action, disabled = false) => h('button.btn.small', { type: 'button', disabled: busy || disabled, onclick: () => action().catch((e) => { error = e.message; render(); }) }, title);
  async function inspect(id = chosen, { accept = false, prompt = false } = {}) {
    if (busy) throw new Error('Wait for the certificate operation.');
    if (!current().some((r) => r.id === id)) throw new Error('Choose a supported saved cohort result in this project.');
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
  async function verify(file, expected = '') {
    if (busy) throw new Error('Wait for the certificate operation.');
    if (!file || file.size > 520 * 1024 * 1024) throw new Error('Choose a certificate no larger than 520 MiB.');
    if (expected && !/^[a-f0-9]{64}$/.test(expected)) throw new Error('Expected fingerprint must be 64 lowercase hexadecimal characters.');
    const valid = same(); busy = true; error = ''; report = null; render();
    try {
      const response = await fetch(`api/certificates/verify${expected ? `?expect=${expected}` : ''}`, { method: 'POST', headers: { 'Content-Type': 'application/zip' }, body: file });
      const value = await response.json(); if (!response.ok) throw new Error(value.error ?? 'Certificate could not be read.');
      if (!valid()) throw new Error('The project changed.'); report = value; return value;
    } finally { if (valid()) { busy = false; render(); } }
  }
  function render() {
    if (!alive) return;
    const records = current();
    if (!records.some((r) => r.id === chosen)) chosen = records.at(-1)?.id ?? '';
    clear(body);
    const select = h('select.input', { 'aria-label': 'Certificate result', disabled: busy, onchange: (e) => { chosen = e.target.value; record = null; error = ''; render(); } },
      ...records.map((r) => h('option', { value: r.id, selected: r.id === chosen }, `${r.name} · ${r.id}`)));
    body.append(h('section.card', h('h2', 'Saved-result evidence certificates'),
      h('p', 'Keep the exact saved recipe, output files, source fingerprints and provider notice in one checkable archive. Currently available for cohort feature tables, group comparisons and replicated counts.'),
      h('p.callout', 'Integrity checks compare recorded bytes. They do not recompute a statistic, verify original sources or separate parents, establish authenticity, or validate the study design. Captured evidence may contain raw counts, sequences, subject identifiers and paths; it is not de-identified.'),
      records.length ? h('label.field', h('span', 'Saved result'), select) : h('p.muted', 'Save a cohort feature table, comparison or count model to prepare its certificate. Existing results of other kinds remain unchanged.'),
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
    const upload = h('input', { type: 'file', accept: '.clz,.zip', 'aria-label': 'Certificate file', disabled: busy, onchange: (e) => verify(e.target.files[0], expected.value.trim()).catch((e) => { error = e.message; render(); }) });
    body.append(h('section.card', h('h2', 'Check a certificate'), h('p', 'Choose an archive to check in the local Go host. Its files are read without extraction; source paths and recipes are never followed or executed.'),
      h('label.field', h('span', 'Expected fingerprint (optional)'), expected), h('label.field', h('span', 'Certificate archive'), upload),
      h('p.muted', 'Terminal: clonoloom verify --integrity FILE.certificate.clz')));
    if (report) body.append(h('section.card.certificate-verdict', h('h2', integrityLabel(report)),
      h('p', `${report.filesChecked} files · ${formatBytes(report.bytesChecked)} checked. Scientific recomputation: not performed.`),
      h('p', report.limits), ...report.findings.map((f) => h('p.callout.danger', f)),
      button('Export verification JSON', async () => downloadBlob(new Blob([JSON.stringify(report, null, 2) + '\n'], { type: 'application/json' }), 'certificate-verification.json'))));
  }
  const view = { inspect, exportCertificate, verify, state: () => ({ record, report, busy, error }) };
  app.certificatesView = view; render();
  return { update(topics) {
    if (generation !== store.state.generation) { generation = store.state.generation; chosen = ''; record = null; report = null; busy = false; error = ''; }
    if (topics.has('project') || topics.has('mode')) render();
  }, destroy() { alive = false; if (app.certificatesView === view) app.certificatesView = null; root.remove(); } };
}
