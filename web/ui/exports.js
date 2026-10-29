// Exports (clonoloom-spec/requirements.md, O3 and O9): a dataset's rearrangements (AIRR TSV) or
// clonotypes (CSV) under its definition and filter, or a kept result (CSV), each a zip with its
// NOTICE, streamed by the program to the browser's downloads (api_export.go). Before exporting the
// rows of public data under a provider's terms (or under no stated license), their terms are shown,
// and the export is made only when the user says they will keep it to themselves.

import { invariantSpecies } from '../lib/invariant.js';
import { h } from './dom.js';
import { confirmDialog } from './overlays.js';
import { datasetDefinition } from '../lib/definitions.js';
import { datasetFilter } from '../lib/filters.js';

// The dataset's public data that may not be shared (their credits).
const restrictedOf = (d) => (d?.credits ?? []).filter((c) => c.licenseClass !== 'open');

// asked answers whether the export may go ahead: at once for open data; for the rest, once the user
// has read their terms. rows: the export holds the data themselves (not results computed from them).
async function asked(d, rows) {
  const restricted = restrictedOf(d);
  if (!restricted.length) return true;
  return confirmDialog({
    title: rows ? 'These data may not be shared' : 'Results of data that may not be shared',
    message: h('div',
      h('p', `${d.name} holds public data under a provider's terms of use, or under no stated license:`),
      h('ul', ...restricted.map((c) => h('li', h('b', c.title), ` — ${c.license}`))),
      h('p', rows
        ? 'Their terms forbid sharing the data. Export them only for yourself and your study; the export\'s NOTICE says so.'
        : 'These results were computed from them: read the terms before sharing the results. The export\'s NOTICE names the terms.')),
    confirm: 'Export',
  });
}

// download starts a download of the program's answer at url (streamed to the browser's downloads).
function download(url) {
  const a = h('a', { href: url, download: '' });
  document.body.append(a);
  a.click();
  a.remove();
}

// exportDataset exports a dataset's rearrangements or clonotypes under its definition and filter.
export async function exportDataset(app, d, what, format = 'text') {
  const state = app.datasetStates.get(d.id);
  const loci = Object.keys(state?.summary?.samples?.reduce((acc, s) => Object.assign(acc, s.loci), {}) ?? {});
  if (!(await asked(d, true))) return;
  if (format === 'parquet') {
    await app.setMode('reports');
    return app.parquet.export(d.id, {what, accept: restrictedOf(d).length > 0}).catch(() => {});
  }
  const params = new URLSearchParams({ species: invariantSpecies(d), what, dataset: d.id, definition: JSON.stringify(datasetDefinition(d, loci)), filter: JSON.stringify(datasetFilter(app.store.project, d)?.rules ?? {}) });
  if (restrictedOf(d).length) params.set('accept', '1');
  download(`api/export?${params}`);
  app.log(`Exported the ${what} of ${d.name}.`);
}

// exportResult exports a kept result as CSV.
export async function exportResult(app, d, artifact) {
  if (!(await asked(d, false))) return;
  const params = new URLSearchParams({ what: 'result', artifact });
  if (restrictedOf(d).length) params.set('accept', '1');
  download(`api/export?${params}`);
  app.log(`Exported ${artifact} of ${d.name}.`);
}
