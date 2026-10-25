// A dataset's methods paragraph in a dialog (lib/methods.js): written from the project's record
// and what the program computes of it now (the rows its filter keeps, its clonotypes), with its
// numbered references, to copy into a manuscript.

import { h, icon } from './dom.js';
import { showDialog, toast } from './overlays.js';
import { clonotypePage, datasetQC } from './storage.js';
import { cellResultsForDataset, methodsParagraph } from '../lib/methods.js';
import { datasetFilter } from '../lib/filters.js';
import { datasetDefinition } from '../lib/definitions.js';
import { resultsOf } from '../lib/results.js';

// The paragraph's inputs the program computes: the filter's counts and the clonotypes, each left
// out when it cannot be computed (a paired definition on bulk data, say).
async function computed(dataset, summary, filter, definition) {
  let filterResult = null;
  let clonotypes = null;
  if (filter) {
    try {
      const qc = await datasetQC(dataset.id, filter.rules);
      filterResult = qc.report.samples.reduce((acc, s) => ({ rows: acc.rows + s.rows, kept: acc.kept + s.kept }), { rows: 0, kept: 0 });
    } catch { /* said without its counts */ }
  }
  try {
    const page = await clonotypePage({ dataset: dataset.id, definition, filter: filter?.rules ?? {}, offset: 0, limit: 1 });
    clonotypes = { id: page.definitionId, describe: page.describe, count: page.count, cells: page.cells };
  } catch { /* no clonotypes under this definition */ }
  return { filterResult, clonotypes };
}

export async function showMethods(app, dataset) {
  const state = await app.ensureDataset(dataset);
  if (state.status !== 'open') {
    toast(state.error ?? `${dataset.name} is not open.`, { kind: 'error' });
    return null;
  }
  const { store } = app;
  const summary = state.summary;
  const loci = Object.keys(summary.samples.reduce((acc, s) => Object.assign(acc, s.loci), {}));
  const definition = datasetDefinition(dataset, loci);
  const filter = datasetFilter(store.project, dataset);
  const { filterResult, clonotypes } = await computed(dataset, summary, filter, definition);
  // A dataset recorded before its files were known as public data takes their credits now.
  const credited = dataset.credits?.length ? dataset : { ...dataset, credits: summary.credits ?? [] };
  const results = ['diversity', 'rarefaction', 'usage', 'overlap', 'tracking', 'expansion', 'classes', 'attribution', 'tcrdist', 'pgen', 'neighborhoods', 'families', 'germlines', 'shm', 'lineages', 'isotypes'].flatMap((kind) => resultsOf(store.project, dataset.id, kind));
  results.push(...cellResultsForDataset(store.project, dataset.id));
  const { text, references } = methodsParagraph({ dataset: credited, summary, filter, filterResult, definition, clonotypes, results, program: { version: app.version, commit: app.commit } });
  const all = `${text}\n\nReferences\n${references.join('\n')}`;
  showDialog({
    title: `Methods · ${dataset.name}`,
    width: 'wide',
    content: [
      h('p.methods-text', text),
      h('div.section-title', 'References'),
      h('ol.methods-references', ...references.map((r) => h('li', r.replace(/^\[\d+\]\s*/, '')))),
      h('p.muted.small', 'Written from the project\'s record and what ClonoLoom computes of the dataset now: only what was done is said. Cite the data and the methods as numbered here.'),
    ],
    buttons: [
      { label: 'Close', ghost: true },
      { label: 'Copy', primary: true, onClick: async () => {
        try {
          await navigator.clipboard.writeText(all);
          toast('The methods paragraph and its references are copied.', { kind: 'ok' });
        } catch {
          toast('The clipboard is not available: select the text to copy it.', { kind: 'error' });
        }
        return false;
      } },
    ],
  });
  app.log(`Wrote the methods paragraph of ${dataset.name}.`);
  return { text, references };
}

// A button that shows it, for the inspector and the Data view.
export function methodsButton(app, dataset) {
  return h('button.btn.small', { type: 'button', onclick: (e) => { e.stopPropagation(); showMethods(app, dataset); } }, icon('file'), 'Methods paragraph');
}
