import { h, clear } from './dom.js';
import { showDialog } from './overlays.js';

// Native cells are strings throughout: frequencies/counts/extensions are evidence,
// not browser arithmetic. Parsed numeric analysis remains in the native worker.
export function showNativeReport(app, dataset) {
  const sameProject = app.store.sameProject();
  const controller = new AbortController();
  const sources = app.datasetStates.get(dataset.id)?.summary?.sources ?? [];
  const content = h('div.native-report');
  let generation = 0;
  const active = () => !controller.signal.aborted && sameProject() && app.store.project.datasets.some((d) => d.id === dataset.id);
  const dialog = showDialog({ title: 'Native report evidence', content, width: 'wide', onClose: () => controller.abort() });
  dialog.body.focus();
  async function get(query) {
    const response = await fetch(`api/datasets/${encodeURIComponent(dataset.id)}/native-report?${query}`, { signal: controller.signal });
    const answer = await response.json();
    if (!response.ok) throw new Error(answer.error?.message ?? answer.error ?? 'Could not inspect native report.');
    return answer;
  }
  async function page(source, offset = 0) {
    const revision = ++generation;
    clear(content); content.append(h('p', 'Loading captured evidence…'));
    try {
      const answer = await get(`source=${source}&offset=${offset}&limit=20`);
      if (!active() || revision !== generation) return;
      clear(content);
      content.append(h('p', h('b', dataset.name)),
        h('p.callout', 'Original report values. Counts do not establish cell numbers or pairing. Confirm an unknown abundance unit from the upstream experiment, not from the size of a count.'),
        h('p.small', answer.format === 'trust4-report'
          ? 'TRUST4: _ marks a stop; ? is ambiguous; partial junctions stay out of complete-junction clonotyping. Source frequencies can use separate IG/TR denominators and are not recomputed here.'
          : 'VDJtools: _ or ~ marks an incomplete codon, not a stop. Counts may be reads or molecules; source frequencies can describe a larger, pre-subset sample.'),
        h('label', 'Source ', h('select.input.native-source', { onchange: (e) => page(Number(e.target.value)) }, ...sources.flatMap((s, i) => s.nativeEvidence ? [h('option', { value: String(i), selected: i === source }, s.name)] : []))),
        h('p.small.mono', { style: { overflowWrap: 'anywhere' } }, `Source SHA-256: ${answer.sha256}`));
      const detail = h('div.native-detail', { 'aria-live': 'polite' });
      let selection = 0;
      async function inspect(index) {
        const selected = ++selection;
        clear(detail); detail.append(h('p', 'Loading source row…'));
        try {
          const d = await get(`source=${source}&record=${index}`);
          if (!active() || revision !== generation || selected !== selection) return;
          clear(detail);
          detail.append(h('h3', `Source line ${d.line}`),
            h('p.small', 'Text is retained exactly, including unsupported annotations. Empty, zero and missing-value markers are distinct. Complete source CDR3s use junction anchors; no residues or cell identities are invented.'),
            h('table.data.native-fields', h('thead', h('tr', h('th', { scope: 'col' }, 'Source column'), h('th', { scope: 'col' }, 'Supplied value'))),
              h('tbody', ...d.columns.map((name, i) => h('tr', h('th', { scope: 'row' }, name), h('td', h('code', { style: { whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' } }, d.values[i] === '' ? '(empty)' : d.values[i])))))),
            h('details', h('summary', 'Exact header and row text'), h('pre.native-raw', { tabIndex: 0, style: { whiteSpace: 'pre-wrap', overflowWrap: 'anywhere', maxHeight: '200px', overflow: 'auto' } }, `${d.headerText}\n${d.rowText}`)));
        } catch (error) { if (active() && revision === generation && selected === selection) { clear(detail); detail.append(h('p.callout.danger', error.message)); } }
      }
      content.append(h('div.btn-row',
        h('button.btn.small', { disabled: offset === 0, onclick: () => page(source, Math.max(0, offset - 20)) }, 'Previous rows'),
        h('label', 'Row ', h('select.input.native-row', { onchange: (e) => inspect(Number(e.target.value)) }, ...answer.rows.map((r) => h('option', { value: String(r.index) }, `Source line ${r.line}`)))),
        h('span.small', `${answer.total} source rows`),
        h('button.btn.small', { disabled: offset + answer.rows.length >= answer.total, onclick: () => page(source, offset + 20) }, 'Next rows')), detail);
      if (answer.rows.length) inspect(answer.rows[0].index);
      else detail.append(h('p', 'No data rows in this report.'));
    } catch (error) { if (active() && revision === generation) { clear(content); content.append(h('p.callout.danger', error.message)); } }
  }
  page(sources.findIndex((s) => s.nativeEvidence));
  return dialog;
}
