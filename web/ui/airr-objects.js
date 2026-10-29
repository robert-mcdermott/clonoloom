// Imported source assertions are not computed clonotypes or inferred pairing.
import { h, clear } from './dom.js';
import { showDialog } from './overlays.js';

export function showAIRRObjects(app, dataset) {
  const sameProject = app.store.sameProject();
  const controller = new AbortController();
  const content = h('div.airr-objects');
  let generation = 0;
  const active = () => !controller.signal.aborted && sameProject() && app.store.project.datasets.some((d) => d.id === dataset.id);
  const dialog = showDialog({ title: 'Imported AIRR objects', content, width: 'wide', onClose: () => controller.abort() });
  dialog.body.focus();
  async function get(query) {
    const response = await fetch(`api/datasets/${encodeURIComponent(dataset.id)}/airr-objects?${query}`, { signal: controller.signal });
    const result = await response.json();
    if (!response.ok) throw new Error(result.error?.message ?? result.error ?? 'Could not inspect AIRR objects.');
    return result;
  }
  async function page(offset = 0) {
    const revision = ++generation;
    clear(content); content.append(h('p', 'Loading imported evidence…'));
    try {
      const result = await get(`offset=${offset}&limit=20`);
      if (!active() || generation !== revision) return;
      clear(content);
      content.append(h('p', h('b', dataset.name)), h('p.callout', 'Imported source assertions. Clone membership and counts do not replace your clonotype definition. Loaded links indicate identifier matches, not biological validity. No pairing or reactivity is inferred.'),
        h('p.small', 'AIRR 2.0 structural checks and receptor hash verification do not validate full variable domains, experimental pairing or schema/ontology conformance. Rearrangement links and repertoire-group membership are not assessed.'));
      const detail = h('div.airr-object-detail', { 'aria-live': 'polite' });
      let selection = 0;
      async function inspect(row) {
        const selected = ++selection;
        clear(detail); detail.append(h('p', 'Loading original object…'));
        try {
          const d = await get(`record=${row.index}`);
          if (!active() || generation !== revision || selected !== selection) return;
          clear(detail);
          detail.append(h('h3', `${d.object.kind} · ${d.object.id}`),
            h('p.small', `${d.source.name} · declared AIRR ${d.source.schemaVersion} · ${d.object.path}`),
            h('p.small.mono', { style: { overflowWrap: 'anywhere' } }, `Source SHA-256: ${d.source.sha256}`),
            h('p.small', 'Links use explicit identifiers only. Unresolved targets may be outside this opening; unscoped processing links lack a repertoire. They are never repaired by name or sequence.'),
            h('div', { style: { overflowX: 'auto' } }, h('table.data.object-links', h('thead', h('tr', ...['Evidence path', 'Target kind', 'Identifier', 'Resolution'].map((x) => h('th', { scope: 'col' }, x)))),
              h('tbody', ...d.object.links.map((link) => h('tr', ...[link.path, link.kind, link.id, link.status].map((x) => h('td', { style: { overflowWrap: 'anywhere', maxWidth: '250px' } }, x))))))),
            h('p.small', d.format === 'yaml' ? 'Decoded object JSON; exact values, not original YAML.' : 'Original object JSON; exact numeric tokens and unknown fields retained.'),
            h('pre.object-json', { tabIndex: 0, style: { whiteSpace: 'pre-wrap', overflowWrap: 'anywhere', maxHeight: '260px', overflow: 'auto' } }, d.rawJSON));
          if (d.sourceText != null) detail.append(h('details', h('summary', 'Full captured source document'), h('pre.object-source', { tabIndex: 0, style: { whiteSpace: 'pre-wrap', overflowWrap: 'anywhere', maxHeight: '260px', overflow: 'auto' } }, d.sourceText)));
          if (d.sourceNotice) detail.append(h('p.callout', d.sourceNotice));
        } catch (error) { if (active() && generation === revision && selected === selection) { clear(detail); detail.append(h('p.callout.danger', error.message)); } }
      }
      content.append(h('div', { style: { overflowX: 'auto' } }, h('table.data.object-list', h('thead', h('tr', ...['Kind', 'Identifier', 'Explicit links', 'Evidence'].map((x) => h('th', { scope: 'col' }, x)))),
        h('tbody', ...result.records.map((row) => h('tr', h('td', row.kind), h('td', { style: { overflowWrap: 'anywhere', maxWidth: '280px' } }, row.id), h('td', String(row.links)), h('td', h('button.btn.small', { type: 'button', 'aria-label': `Inspect ${row.kind} ${row.id}`, onclick: () => inspect(row) }, 'Inspect'))))))),
        h('div.btn-row', h('button.btn.small', { disabled: offset === 0, onclick: () => page(Math.max(0, offset - 20)) }, 'Previous objects'), h('span.small', `${result.total} imported objects`), h('button.btn.small', { disabled: offset + result.records.length >= result.total, onclick: () => page(offset + 20) }, 'Next objects')), detail);
    } catch (error) { if (active() && generation === revision) { clear(content); content.append(h('p.callout.danger', error.message)); } }
  }
  page();
  return dialog;
}
