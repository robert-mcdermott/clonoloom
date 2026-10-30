// Explicit, bounded native capture. No project mutation or automatic analysis.
import { h, clear, formatBytes } from './dom.js';
import { showDialog } from './overlays.js';
export function showADCDownload(repository, repertoire) {
  let controller; let generation = 0; let closed = false; let prepared;
  const processing = h('input.input', { 'aria-label': 'Processing ID', placeholder: 'Exact data_processing_id from the metadata' });
  const license = h('textarea.input', { 'aria-label': 'Permission evidence', placeholder: 'Provider license/terms, source URL, and permission for this download' });
  const citation = h('textarea.input', { 'aria-label': 'Source citation', placeholder: 'Study citation and source URL' });
  const pages = h('input.input', { 'aria-label': 'Maximum pages', type: 'number', min: 1, max: 10, value: '1' });
  const size = h('input.input', { 'aria-label': 'Records per page', type: 'number', min: 1, max: 1000, value: '100' });
  const accept = h('input', { type: 'checkbox', 'aria-label': 'Accept download terms' });
  const preview = h('div'); const status = h('p', { role: 'status' }); const result = h('div');
  const start = h('button.btn.primary', { disabled: true, onclick: download }, 'Download reviewed capture');
  const review = h('button.btn', { onclick: prepare }, 'Review download');
  const cancel = h('button.btn', { disabled: true, onclick: () => { generation++; controller?.abort(); status.textContent = 'Cancelled locally. An already completed capture may remain in the library; no existing files were overwritten.'; cancel.disabled = true; review.disabled = false; start.disabled = true; } }, 'Cancel download');
  const content = h('div.adc-download', h('p.callout', 'Capture one repertoire and processing run. Total source size is unknown. Review provider permission before downloading; ClonoLoom does not grant redistribution rights.'),
    h('p.mono', `${repository} · ${repertoire}`),
    ...[[processing, 'Processing ID'], [license, 'Provider permission evidence'], [citation, 'Source citation'], [pages, 'Maximum pages (1–10)'], [size, 'Records per page (1–1000)']].map(([input, title]) => h('label.field', h('span', title), input)),
    review, preview, h('label.field', h('span', accept, ' I reviewed the provider terms and have permission for this bounded download.')), h('div.btn-row', start, cancel), status, result);
  const dialog = showDialog({ title: 'Download ADC evidence', content, width: 'wide', onClose: () => { closed = true; controller?.abort(); } });
  const spec = () => ({ repository, repertoire, processing: processing.value, license: license.value, citation: citation.value, pages: Number(pages.value), size: Number(size.value), accept: accept.checked });
  for (const input of [processing, license, citation, pages, size]) input.addEventListener('input', () => { generation++; controller?.abort(); prepared = null; start.disabled = true; cancel.disabled = true; review.disabled = false; clear(preview); clear(result); status.textContent = 'Review the changed request before downloading.'; });
  accept.addEventListener('change', () => { start.disabled = !cancel.disabled || !prepared || !accept.checked || !license.value.trim() || !citation.value.trim(); });
  async function call(path, body) {
    controller = new AbortController();
    const response = await fetch(`api/adc/download${path}`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body), signal: controller.signal });
    const data = await response.json(); if (!response.ok) throw new Error(data.error?.message ?? data.error ?? 'Download failed.'); return data;
  }
  async function prepare() {
    const mine = ++generation; controller?.abort(); prepared = null; start.disabled = true; clear(preview); clear(result);
    try {
      const selected = spec(); const plan = await call('/preview', selected); if (closed || mine !== generation) return;
      prepared = selected;
      preview.append(h('p.small.mono', `Metadata: ${plan.metadata.endpoint}`), h('pre.adc-request', plan.metadata.requestJSON), h('p.small.mono', `Rearrangements: ${plan.endpoint}`), h('pre.adc-request', plan.requestJSON), h('p.callout', `At most ${plan.maxRecords} rearrangements and ${formatBytes(plan.maxBytes)} of decoded responses, including metadata. Five-minute total limit. Later offsets advance by returned records. No retries or redirects. Size is a budget, not a provider estimate.`), h('p.small', 'Original JSON pages and a provenance manifest are saved in a new private library folder. They are evidence files, not an analysis dataset. Pages can repeat or skip records as the repository changes; completeness remains unproven.'));
      start.disabled = !accept.checked || !license.value.trim() || !citation.value.trim(); status.textContent = 'Prepared locally; no repository contacted.';
    } catch (error) { if (!closed && mine === generation) status.textContent = error.message; }
  }
  async function download() {
    if (!prepared || !accept.checked || !cancel.disabled) return;
    const mine = ++generation; start.disabled = true; review.disabled = true; cancel.disabled = false; clear(result); status.textContent = 'Downloading bounded evidence…';
    try {
      const data = await call('', { ...prepared, accept: true }); if (closed || mine !== generation) return;
      result.append(h('p', `${data.manifest.records} rearrangements captured; stop: ${data.manifest.stop}; completeness not established.`), h('p.mono', { style: { overflowWrap: 'anywhere' } }, data.path), h('p.small', data.manifest.notice));
      status.textContent = 'Capture saved. Keep manifest.json together with the original JSON pages.'; prepared = null;
    } catch (error) { if (!closed && mine === generation) status.textContent = error.message; }
    finally { if (!closed && mine === generation) { cancel.disabled = true; review.disabled = false; } }
  }
  return dialog;
}
