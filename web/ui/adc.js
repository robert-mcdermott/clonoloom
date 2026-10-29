// Public metadata browsing sends only the fields entered here. No dataset/project access.
import { showADCDownload } from './adc-download.js';
import { h, clear } from './dom.js';
import { showDialog } from './overlays.js';

export function showADC() {
  const content = h('div.adc-browser');
  let closed = false;
  let revision = 0;
  let pending;
  let prepared;
  let offline = true;
  const dialog = showDialog({ title: 'AIRR Data Commons', content, width: 'wide', onClose: () => { closed = true; pending?.abort(); } });
  const repository = h('select.input.adc-repository', { 'aria-label': 'Repository' });
  const species = h('input.input', { value: 'NCBITAXON:9606', 'aria-label': 'Species ontology ID' });
  const locus = h('select.input', { 'aria-label': 'Locus' }, ...['', 'TRA', 'TRB', 'TRG', 'TRD', 'IGH', 'IGK', 'IGL'].map((value) => h('option', { value, selected: value === 'TRB' }, value || 'Any locus')));
  const study = h('input.input', { 'aria-label': 'Study ID', placeholder: 'Any study' });
  const repertoire = h('input.input', { 'aria-label': 'Repertoire ID', placeholder: 'Any repertoire' });
  const from = h('input.input', { type: 'number', min: 0, max: 10000, value: '0', step: 1, 'aria-label': 'Offset' });
  const size = h('input.input', { type: 'number', min: 1, max: 100, value: '20', step: 1, 'aria-label': 'Page limit' });
  const fields = [repository, species, locus, study, repertoire, from, size];
  const preview = h('div.adc-preview');
  const status = h('p.small', { role: 'status' });
  const result = h('div.adc-result');
  const queryButton = h('button.btn.primary', { disabled: true, onclick: () => send('query') }, 'Send metadata query');
  const infoButton = h('button.btn', { disabled: true, onclick: () => send('info') }, 'Request service info');
  const cancelButton = h('button.btn', { disabled: true, onclick: () => { revision += 1; pending?.abort(); status.textContent = 'Cancelled locally. The repository may already have received the request.'; busy(false); } }, 'Cancel request');

  function busy(value) {
    queryButton.disabled = infoButton.disabled = value || !prepared || offline;
    cancelButton.disabled = !value;
  }
  async function call(operation, body, signal) {
    const response = await fetch(`api/adc/${operation}`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body), cache: 'no-store', signal });
    const data = await response.json();
    if (!response.ok) throw new Error(data.error?.message ?? data.error ?? 'ADC request failed.');
    return data;
  }
  const current = () => ({ repository: repository.value, species: species.value, locus: locus.value, study: study.value, repertoire: repertoire.value, from: Number(from.value), size: Number(size.value) });
  async function prepare() {
    const mine = ++revision;
    pending?.abort(); pending = new AbortController();
    prepared = null; busy(false); clear(preview); clear(result);
    status.textContent = 'Preparing locally; no repository contacted.';
    try {
      if (!from.value || !size.value || !from.checkValidity() || !size.checkValidity()) throw new Error('Enter a whole offset from 0 to 10000 and page limit from 1 to 100.');
      const query = current();
      const p = await call('prepare', { query }, pending.signal);
      if (closed || mine !== revision) return;
      prepared = { query, preview: p };
      preview.append(h('p.small.mono', `POST ${p.endpoint}`), h('pre.adc-request', { tabIndex: 0 }, p.requestJSON), h('p.small.mono', `Service info: GET ${p.repository.url}/airr/v1/info`));
      status.textContent = offline ? 'Offline mode: requests are disabled. The bundled registry remains available.' : 'Review the destination and exact query above. Each button sends one request; pages are never fetched automatically.';
      busy(false);
    } catch (error) { if (!closed && mine === revision) status.textContent = error.message; }
  }
  for (const field of fields) field.addEventListener('input', () => { if (field !== from && field !== size) from.value = '0'; prepare(); });
  function evidence(e) {
    return h('details.adc-evidence', h('summary', 'Exact response and retrieval evidence'),
      h('p.small.mono', `Retrieved ${e.retrievedAt} · ${e.responseBytes} bytes`),
      h('p.small.mono', `Response SHA-256: ${e.responseSHA256}`),
      h('p.small', 'Exact response text. Unknown fields and numeric tokens are retained. This session view is not a saved download or replayable cohort.'),
      h('pre.adc-json', { tabIndex: 0 }, e.responseJSON));
  }
  function showPage(page) {
    result.append(h('h3', `${page.records.length} repertoire metadata records returned`),
      h('p.small.adc-page', `Offset ${page.from} · requested at most ${page.size} · completeness not established`));
    if (!page.records.length) result.append(h('p.callout', 'No records returned for this request. This does not establish biological absence or a complete search across repositories.'));
    const detail = h('div.adc-detail');
    const label = (text, missing = 'Not supplied') => text ? (text.length > 240 ? `${text.slice(0, 240)}…` : text) : missing;
    function inspect(row) {
      clear(detail); detail.append(h('details', { open: true }, h('summary', `Original repertoire · ${row.id}`), h('pre.adc-json', { tabIndex: 0 }, row.rawJSON)), h('button.btn', { disabled: offline, onclick: () => showADCDownload(page.repository.id, row.id) }, 'Download this repertoire…'));
    }
    if (page.records.length) result.append(h('div.adc-table', { tabIndex: 0 }, h('table.data',
      h('thead', h('tr', ...['Repertoire', 'Study / title', 'Subject ID', 'Evidence'].map((x) => h('th', { scope: 'col' }, x)))),
      h('tbody', ...page.records.map((row) => h('tr', h('td', label(row.id)), h('td', label(row.study), h('div.small', label(row.title, 'Title not supplied'))), h('td', label(row.subject)),
        h('td', h('button.btn.small', { 'aria-label': `Inspect ${row.id}`, onclick: () => inspect(row) }, 'Inspect'))))))));
    if (page.records.some((row) => [row.id, row.study, row.title, row.subject].some((value) => value.length > 240))) result.append(h('p.small', 'Long labels are shortened in the table. Inspect the original JSON for complete values.'));
    result.append(h('p.small', 'Repertoires are not independent subjects. IDs are repository/study scoped. Short pages may reflect server caps; offsets may repeat or skip records if the repository changes. There is no stable ordering or snapshot guarantee.'),
      h('div.btn-row', h('button.btn.small', { disabled: page.nextOffset == null, onclick: () => { from.value = String(page.nextOffset); prepare(); } }, 'Prepare next page'), h('span.small', 'Review, then send again.')), detail,
      h('details', h('summary', 'Exact request fingerprint'), h('p.small.mono', page.requestSHA256)), evidence(page.evidence));
  }
  async function send(operation) {
    if (!prepared || offline || cancelButton.disabled === false) return;
    const mine = ++revision;
    const selected = prepared;
    pending?.abort(); pending = new AbortController();
    clear(result); busy(true); status.textContent = 'Request in progress (45 second limit)…';
    try {
      const data = await call(operation, { query: selected.query, confirm: true }, pending.signal);
      if (closed || mine !== revision) return;
      if (operation === 'query') showPage(data);
      else result.append(h('h3', 'Repository service information'), h('p.small', 'Provider declarations; server, ADC API and AIRR schema versions are separate. Availability and compatibility are not certified.'), evidence(data));
      status.textContent = 'One request completed. No data installed or local receptor data sent.';
    } catch (error) {
      if (!closed && mine === revision) { status.textContent = 'Request failed; no results installed.'; result.append(h('p.callout.danger', error.message)); }
    } finally { if (!closed && mine === revision) busy(false); }
  }
  async function load() {
    try {
      pending = new AbortController();
      const response = await fetch('api/adc/repositories', { cache: 'no-store', signal: pending.signal });
      const catalog = await response.json();
      if (!response.ok) throw new Error(catalog.error?.message ?? catalog.error ?? 'Could not list repositories.');
      if (closed) return;
      offline = catalog.offline;
      repository.append(...catalog.repositories.map((r) => h('option', { value: r.id }, r.id)));
      content.append(h('p.callout', 'Browse public repertoire metadata. Only the filters below are sent. Blank fields impose no restriction.'),
        h('p.small', `${catalog.contract}. Bundled approved registry; no automatic repository discovery or contact.`),
        h('details', h('summary', 'Pinned registry'), h('p.small.mono', catalog.registryRevision), h('p.small.mono', `SHA-256: ${catalog.registrySHA256}`)),
        h('div.adc-fields', ...fields.map((field, i) => h('label.field', h('span', ['Repository', 'Species ontology ID (exact)', 'Locus (exact)', 'Study ID (exact)', 'Repertoire ID (exact)', 'Offset (records)', 'Page limit'][i]), field))),
        preview, h('div.btn-row', queryButton, infoButton, cancelButton), status,
        h('p.small', 'Provider metadata do not certify scientific validity or reuse permission. No local data are sent or results installed.'), result);
      await prepare();
    } catch (error) { if (!closed) content.append(h('p.callout.danger', error.message)); }
  }
  load();
  return dialog;
}
