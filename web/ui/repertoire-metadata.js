// Supplied evidence stays text: never parse the original JSON into JS numbers.
import { h, clear } from './dom.js';
import { showDialog } from './overlays.js';

export function showRepertoireMetadata(app, dataset) {
  const sameProject = app.store.sameProject();
  const controller = new AbortController();
  const content = h('div.repertoire-metadata');
  let generation = 0;
  const active = () => !controller.signal.aborted && sameProject()
    && app.store.project.datasets.some((d) => d.id === dataset.id);
  const dialog = showDialog({ title: 'AIRR repertoire metadata', content, width: 'wide', onClose: () => controller.abort() });
  dialog.body.focus();
  async function get(query) {
    const response = await fetch(`api/datasets/${encodeURIComponent(dataset.id)}/metadata?${query}`, { signal: controller.signal });
    const answer = await response.json();
    if (!response.ok) throw new Error(answer.error?.message ?? answer.error ?? 'Could not inspect metadata.');
    return answer;
  }
  async function page(offset = 0) {
    const revision = ++generation;
    clear(content); content.append(h('p', { role: 'status' }, 'Reading supplied metadata…'));
    try {
      const answer = await get(`offset=${offset}&limit=20`);
      if (!active() || revision !== generation) return;
      clear(content);
      content.append(h('p', h('b', dataset.name)),
        h('p.muted.small', 'Supplied annotations, not a MiAIRR completeness or scientific validity assessment. Matching does not select a processing record or establish independent subjects.'),
        h('div.callout', `${answer.matchedRecords} matched / ${answer.total} repertoire records · ${answer.unmatchedRecords} unmatched records. ${answer.samplesWithMetadata} dataset samples have metadata; ${answer.samplesWithoutMetadata} do not.`),
        h('p.muted.small', 'Case-sensitive repertoire_id matching; legacy sample-name matching only for samples without a repertoire_id. Unmatched records are retained.'));
      const details = h('div.metadata-detail', { 'aria-live': 'polite' });
      let selection = 0;
      async function inspect(record) {
        const selected = ++selection;
        clear(details); details.append(h('p', 'Loading original record…'));
        try {
          const detail = await get(`record=${record.index}`);
          if (!active() || revision !== generation || selected !== selection) return;
          const source = answer.files[record.fileIndex];
          clear(details);
          details.append(h('h3', record.repertoireId),
            h('p.small', `${source.name} · declared AIRR schema: ${source.schemaVersion || 'not supplied'}`),
            h('p.mono.small', { style: { overflowWrap: 'anywhere' } }, `Source SHA-256: ${source.sha256}`),
            h('p.muted.small', 'Original record JSON, including unknown fields and exact numeric tokens. Absent fields, null, false and zero remain distinct. The source fingerprint covers the whole file, including its wrapper.'),
            h('pre.metadata-json', { tabIndex: 0, 'aria-label': 'Original repertoire JSON', style: { whiteSpace: 'pre-wrap', overflowWrap: 'anywhere', maxHeight: '300px', overflow: 'auto' } }, detail.rawJSON));
        } catch (error) {
          if (active() && revision === generation && selected === selection) { clear(details); details.append(h('p.callout.danger', error.message)); }
        }
      }
      const table = h('table.data',
        h('thead', h('tr', ...['Repertoire ID', 'Dataset samples', 'Match basis', 'Evidence'].map((label) => h('th', { scope: 'col' }, label)))),
        h('tbody', ...answer.records.map((record) => h('tr',
          h('td', { style: { overflowWrap: 'anywhere', maxWidth: '240px' } }, record.repertoireId),
          h('td', String(record.matchedSamples)),
          h('td', record.matchedSamples ? [record.idMatches ? `${record.idMatches} by repertoire_id` : '', record.nameMatches ? `${record.nameMatches} by sample name (legacy)` : ''].filter(Boolean).join('; ') : 'Unmatched'),
          h('td', h('button.btn.small', { type: 'button', 'aria-label': `Inspect ${record.repertoireId}`, onclick: () => inspect(record) }, 'Inspect'))))));
      content.append(h('div', { style: { overflowX: 'auto' } }, table),
        h('div.btn-row', { style: { marginTop: '12px' } },
          h('button.btn.small', { type: 'button', disabled: offset === 0, onclick: () => page(Math.max(0, offset - 20)) }, 'Previous records'),
          h('span.small', answer.total ? `${offset + 1}–${offset + answer.records.length} of ${answer.total}` : 'No repertoire records'),
          h('button.btn.small', { type: 'button', disabled: offset + answer.records.length >= answer.total, onclick: () => page(offset + 20) }, 'Next records')), details);
    } catch (error) {
      if (active() && revision === generation) { clear(content); content.append(h('p.callout.danger', error.message)); }
    }
  }
  page();
  return dialog;
}
