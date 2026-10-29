// Offline presence comparison of explicitly selected source evidence. All set arithmetic is native.
import { h, clear } from './dom.js';
import { showDialog } from './overlays.js';

export function showADCOverlap(app) {
  const sameProject = app.store.sameProject();
  let controller; let generation = 0; let closed = false; let busy = false; let loading = false;
  const dataset = h('select.input', { 'aria-label': 'Local query dataset' },
    ...app.store.project.datasets.map((d) => h('option', { value: d.id }, d.name)));
  const sample = h('select.input', { 'aria-label': 'Local query sample' });
  const locus = h('select.input', { 'aria-label': 'Matching locus' },
    ...['TRB', 'TRA', 'TRG', 'TRD', 'IGH', 'IGK', 'IGL'].map((l) => h('option', { value: l }, l)));
  const captures = h('div.adc-captures');
  const accept = h('input', { type: 'checkbox', 'aria-label': 'Accept presence contract' });
  const status = h('p', { role: 'status' });
  const result = h('div.adc-overlap-result');
  const run = h('button.btn.primary', { onclick: compare, disabled: true }, 'Compare selected evidence');
  const cancel = h('button.btn', { disabled: true, onclick: () => { invalidate(); status.textContent = 'Cancelled locally. A result already saved may remain in the library.'; } }, 'Cancel');
  const content = h('div.adc-overlap',
    h('p.callout', 'Separate presence contract: productive records, one locus, exact amino-acid junction with C and F/W anchors. No V/J matching, pairing, abundance weighting or project QC filter. Captured subsets are not complete cohorts.'),
    h('div.adc-fields', h('label.field', h('span', 'Local dataset'), dataset),
      h('label.field', h('span', 'One query sample'), sample), h('label.field', h('span', 'Locus'), locus)),
    h('h3', 'Captured public evidence (1–20)'), captures,
    h('label.field', h('span', accept, ' Use this explicit contract, independently of the active project definition.')),
    h('div.btn-row', run, cancel), status, result);
  const dialog = showDialog({ title: 'Observed public-cohort overlap', content, width: 'wide',
    onClose: () => { closed = true; controller?.abort(); unsubscribe(); } });
  const usable = () => sameProject() && app.store.project.datasets.some((d) => d.id === dataset.value);
  const current = (mine) => !closed && mine === generation && usable();
  function buttons() {
    const count = captures.querySelectorAll('input:checked').length;
    run.disabled = busy || loading || !usable() || !sample.options.length || !accept.checked || count < 1 || count > 20;
    cancel.disabled = !busy;
    sample.disabled = loading; locus.disabled = loading; accept.disabled = loading;
  }
  function invalidate(resetAcceptance = true) {
    generation++; controller?.abort(); busy = false;
    if (resetAcceptance) accept.checked = false;
    clear(result); buttons(); status.textContent = 'Inputs changed; review and compare again.';
  }
  const unsubscribe = app.store.subscribe(() => {
    if (!usable()) { invalidate(); status.textContent = 'The project or query dataset changed. Close and reopen this comparison.'; }
  });
  for (const input of [sample, locus]) input.addEventListener('change', () => invalidate());
  accept.addEventListener('change', () => invalidate(false));
  dataset.addEventListener('change', load);

  async function load() {
    invalidate(); const mine = generation;
    clear(captures); clear(sample); loading = true; buttons(); controller = new AbortController();
    try {
      const response = await fetch(`api/adc/overlap/catalog?dataset=${encodeURIComponent(dataset.value)}`, { signal: controller.signal });
      const data = await response.json();
      if (!response.ok) throw new Error(data.error?.message ?? data.error);
      if (!current(mine)) return;
      sample.append(...data.samples.map((s, i) => h('option', { value: String(i) }, s.name)));
      captures.append(...data.captures.map((name) => h('label.field', h('span',
        h('input', { type: 'checkbox', value: name, onchange: () => invalidate() }), ` ${name}`))));
      status.textContent = data.captures.length
        ? 'Select captures deliberately. Checksums and scope are verified on comparison.'
        : 'No ADC captures in this library. Download reviewed evidence first.';
    } catch (error) { if (current(mine)) status.textContent = error.message; }
    finally { if (!closed && mine === generation) { loading = false; buttons(); } }
  }

  function render(d, path) {
    const r = d.result;
    const headers = (labels) => h('thead', h('tr', ...labels.map((x) => h('th', { scope: 'col' }, x))));
    result.append(h('h3', 'Observed set overlap'),
      h('p.small', `Query: ${d.querySample} · source count unit: ${d.querySourceUnit}. Public count units: ${d.publicSourceUnit}. Counts are not weights.`),
      h('p', `Query: ${r.query.unique} unique eligible junction keys from ${r.query.eligible}/${r.query.rows} records; ${r.query.excluded} excluded. ${r.queryKeysShared} query keys observed in at least one selected capture.`),
      h('p', `${r.evaluableSubjects}/${r.assignedSubjects} scoped subject comparisons are evaluable; empty-set comparisons: ${r.missingSubjects}; captures missing a study or subject ID: ${r.unassignedCaptures}. These are selected evidence denominators, not population prevalence.`),
      h('div.adc-table', { tabIndex: 0 }, h('table.data',
        headers(['Capture / subject', 'Eligible / rows', 'Unique keys', 'Shared / union', 'Jaccard', 'Stop']),
        h('tbody', ...r.pairs.map((p) => h('tr',
          h('td', p.capture, h('div.small', `${p.study || 'Study missing'} · ${p.subject || 'Subject missing'}`)),
          h('td', `${p.eligibility.eligible}/${p.eligibility.rows}`), h('td', String(p.eligibility.unique)),
          h('td', `${p.shared}/${p.union}`), h('td', p.jaccard == null ? 'Unavailable' : p.jaccard.toPrecision(4)), h('td', p.stop)))))),
      h('p.small', 'Jaccard is displayed to four significant digits; shared and union counts are exact. Full native values are retained below.'),
      h('details', h('summary', `Shared junction keys (${r.sharedKeys.length}; first 100 shown)`),
        h('p.small', 'Subject support uses only assigned scoped IDs. Capture support also includes unassigned evidence. Counts describe this selected, incomplete universe.'),
        h('div.adc-table', { tabIndex: 0 }, h('table.data', headers(['Junction key', 'Captures', 'Scoped subjects']),
          h('tbody', ...r.sharedKeys.slice(0, 100).map((p) => h('tr', h('td', p.key), h('td', String(p.captures)), h('td', `${p.subjects}/${r.evaluableSubjects}`))))))),
      h('p.callout', r.notice),
      h('p.small', 'Species, assays, sequencing depth, QC and sampling can change observed sharing. Shared junctions do not establish antigen specificity or biological identity.'),
      h('details', h('summary', 'Native result and source fingerprints'), h('pre.adc-json', { tabIndex: 0 }, JSON.stringify(d, null, 2))),
      h('p.small', { style: { overflowWrap: 'anywhere' } }, `Saved result: ${path}`));
  }
  async function compare() {
    if (run.disabled) return;
    const mine = ++generation;
    controller?.abort(); controller = new AbortController(); busy = true; buttons(); clear(result);
    status.textContent = 'Verifying captured evidence and comparing observed sets…';
    try {
      const response = await fetch('api/adc/overlap', {
        method: 'POST', headers: { 'Content-Type': 'application/json' }, signal: controller.signal,
        body: JSON.stringify({ dataset: dataset.value, sample: Number(sample.value), locus: locus.value,
          captures: [...captures.querySelectorAll('input:checked')].map((x) => x.value), accept: accept.checked }),
      });
      const data = await response.json();
      if (!response.ok) throw new Error(data.error?.message ?? data.error);
      if (!current(mine)) return;
      render(data.document, data.path); status.textContent = 'Comparison saved. No repository was contacted.';
    } catch (error) { if (current(mine)) status.textContent = error.message; }
    finally { if (!closed && mine === generation) { busy = false; buttons(); } }
  }
  load(); return dialog;
}
