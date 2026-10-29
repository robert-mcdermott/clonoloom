// The Data view (clonoloom-spec/requirements.md, D1–D11): the project's datasets with what each
// holds and whether to trust it (its files and their fingerprints, its format and unit, and the
// validator's findings with their examples and remedies), and its samples' annotation, typed or
// suggested from file names and metadata.

import { showNativeReport } from './native-reports.js';
import { showProteoscope } from './proteoscope.js';
import { showADCOverlap } from './adc-overlap.js';
import { showADC } from './adc.js';
import { showAIRRObjects } from './airr-objects.js';
import { showRepertoireMetadata } from './repertoire-metadata.js';
import { h, icon, clear, formatBytes } from './dom.js';
import { showDialog, showMenu, promptDialog, confirmDialog, toast } from './overlays.js';
import { methodsButton, showMethods } from './methods.js';
import { SAMPLE_FIELDS, UNITS, annotate, annotation, confirmUnit, datasetUnit, fieldsFromMetadata, removeDataset, renameDataset, suggestFromNames } from '../lib/datasets.js';

const SEVERITY = {
  fatal: { label: 'Fatal', badge: 'danger', icon: 'stop', words: 'stops analysis' },
  warning: { label: 'Warning', badge: 'warn', icon: 'warning', words: 'shown with every result' },
  note: { label: 'Note', badge: 'accent', icon: 'info', words: 'for information' },
};

const count = (n) => Number(n ?? 0).toLocaleString('en-US');
const plural = (n, word, words = `${word}s`) => `${count(n)} ${n === 1 ? word : words}`;
const percent = (part, whole) => (whole > 0 ? `${((100 * part) / whole).toFixed(part / whole < 0.001 && part > 0 ? 2 : 1)}%` : '');

// A number in a dataset's unit: whole numbers of cells, templates, reads or UMIs as integers.
function abundance(value, unit) {
  if (!Number.isFinite(value)) return '—';
  const text = Number.isInteger(value) ? count(value) : value.toLocaleString('en-US', { maximumFractionDigits: 2 });
  return `${text} ${UNITS[unit] ?? 'counts'}`;
}

export function mountData(app, container) {
  const { store } = app;
  const root = h('div.view');
  const openButton = h('button.btn.primary.small', { type: 'button', onclick: () => app.openPathDialog() }, icon('folder'), 'Open data');
  const examplesButton = h('button.btn.small', { type: 'button', onclick: () => app.showExamples() }, icon('flask'), 'Example data');
  const adcButton = h('button.btn.small', { type: 'button', onclick: () => showADC() }, 'Public repositories');
  const head = h('div.workbench-head', h('h1', icon('database'), 'Data'), h('span.spacer'), h('button.btn.small', { onclick: () => showADCOverlap(app) }, 'Public overlap'), adcButton, examplesButton, openButton);
  const body = h('div.view-body.data-view');
  root.append(head, body);
  container.append(root);

  app.ensureDatasets();

  // The dataset whose report is shown: the one in focus, else the first.
  function shownDataset() {
    const { datasets } = store.project;
    const focus = store.ui.focus;
    return datasets.find((d) => focus?.kind === 'dataset' && d.id === focus.id) ?? datasets[0] ?? null;
  }

  function render() {
    clear(body);
    const { datasets } = store.project;
    if (!datasets.length) {
      body.append(h('div.empty',
        icon('database'),
        h('h3', 'No data yet'),
        h('p', 'Open AIRR rearrangement tables, a Cell Ranger V(D)J or IMGT/HighV-QUEST output folder, or immunoSEQ exports. What you open together becomes one dataset: a cohort\'s exports, say, or a run\'s T and B cells. ClonoLoom reads each file where it is, checks it, and records its fingerprint in the project.'),
        h('div.btn-row', { style: { marginTop: '10px', justifyContent: 'center' } },
          h('button.btn.primary', { type: 'button', onclick: () => app.openPathDialog() }, icon('folder'), 'Open data'),
          h('button.btn', { type: 'button', onclick: () => app.showExamples() }, icon('flask'), 'Example data'))));
      return;
    }
    body.append(h('div.section-title', `Datasets · ${datasets.length}`), h('div.dataset-cards', ...datasets.map(datasetCard)));
    const shown = shownDataset();
    if (shown) {
      body.append(report(shown));
      if (shown.metadata?.length) body.append(h('div.btn-row', { style: { margin: '12px 0' } },
        h('button.btn.small', { type: 'button', disabled: app.datasetStates.get(shown.id)?.status !== 'open', onclick: () => showRepertoireMetadata(app, shown) }, icon('info'), 'Inspect AIRR metadata')));
    }
    if (shown && app.datasetStates.get(shown.id)?.summary?.metadata?.some((file) => file.objects)) body.append(h('button.btn.small', { type: 'button', disabled: app.datasetStates.get(shown.id)?.status !== 'open', onclick: () => showAIRRObjects(app, shown) }, 'Inspect AIRR objects'));
    if (shown && app.datasetStates.get(shown.id)?.summary?.sources?.some((file) => file.nativeEvidence)) body.append(h('button.btn.small', { type: 'button', disabled: app.datasetStates.get(shown.id)?.status !== 'open', onclick: () => showNativeReport(app, shown) }, 'Inspect native report'));
    if (shown) body.append(h('button.btn.small', { type: 'button', disabled: app.datasetStates.get(shown.id)?.status !== 'open', onclick: () => showProteoscope(app, shown) }, 'Proteoscope handoff'));
    body.append(samplesSection());
  }

  // --- Datasets ----------------------------------------------------------------------------------

  function datasetCard(d) {
    const state = app.datasetStates.get(d.id) ?? { status: 'opening' };
    const summary = state.summary;
    const focused = store.ui.focus?.kind === 'dataset' && store.ui.focus.id === d.id;
    const unit = datasetUnit(d);
    const status = {
      opening: h('span.badge', 'Opening…'),
      open: null,
      missing: h('span.badge.danger', 'Files missing'),
      changed: h('span.badge.danger', 'Files changed'),
      failed: h('span.badge.danger', 'Could not open'),
    }[state.status];
    const findings = h('div.btn-row',
      d.findings.fatal ? h('span.badge.danger', plural(d.findings.fatal, 'fatal finding')) : null,
      d.findings.warnings ? h('span.badge.warn', plural(d.findings.warnings, 'warning')) : null,
      d.findings.notes ? h('span.badge.accent', plural(d.findings.notes, 'note')) : null,
      !d.findings.fatal && !d.findings.warnings && !d.findings.notes ? h('span.badge.ok', 'No findings') : null);
    const unitControl = !d.unit
      ? h('label.unit-confirm', { onclick: (e) => e.stopPropagation() },
        h('span', 'Unit'),
        h('select.input.small', {
          'aria-label': `The unit of ${d.name}'s counts`,
          onchange: (e) => store.commit(confirmUnit(store.project, d.id, e.target.value), `Confirm the unit of ${d.name}`),
        },
        h('option', { value: '', selected: !d.unitConfirmed }, 'not stated: choose'),
        ...['reads', 'umis'].map((u) => h('option', { value: u, selected: d.unitConfirmed === u }, UNITS[u]))))
      : h('span.kv-inline', h('span.muted', 'Unit'), h('b', UNITS[unit] ?? unit));
    return h(`div.card.dataset-card${focused ? '.focused' : ''}`, {
      role: 'group', 'aria-label': d.name,
      onclick: () => app.focusItem({ kind: 'dataset', id: d.id }),
    },
    h('div.dataset-card-head',
      icon(d.singleCell ? 'chip' : 'table'),
      h('h4', h('button.btn.ghost.small', { type: 'button', 'aria-pressed': focused ? 'true' : 'false', onclick: (e) => { e.stopPropagation(); app.focusItem({ kind: 'dataset', id: d.id }); } }, d.name)),
      status,
      h('span.spacer'),
      h('button.icon-button.small', { type: 'button', title: 'More', 'aria-label': `More for ${d.name}`, onclick: (e) => { e.stopPropagation(); datasetMenu(e.currentTarget, d, state); } }, icon('more'))),
    h('p', d.formats.join(', ')),
    d.credits?.length ? h('p.muted.small', icon('flask'), ` Public data: ${d.credits.map((c) => c.license).join('; ')}`) : null,
    h('div.dataset-stats',
      h('span', plural(d.samples.length, 'sample')),
      h('span', plural(d.rows, d.singleCell ? 'contig' : 'rearrangement')),
      h('span', plural(d.sources.length, 'file')),
      d.singleCell ? h('span', 'single cell') : h('span', 'bulk'),
      unitControl),
    findings,
    state.status === 'missing' || state.status === 'failed' || state.status === 'changed'
      ? h('div.callout.danger', { onclick: (e) => e.stopPropagation() },
        h('p', state.error ?? 'The dataset could not be opened.'),
        state.status !== 'failed' ? h('button.btn.small', { type: 'button', onclick: () => app.locateDataset(d) }, icon('search'), 'Locate the files') : null)
      : null,
    summary && d.unit === '' && !d.unitConfirmed
      ? h('p.muted.small', 'The files\' counts do not say whether reads or molecules (UMIs) were counted. Until a unit is confirmed, results say "counts".')
      : null);
  }

  function datasetMenu(anchor, d, state) {
    showMenu(anchor, [
      { label: 'Rename…', icon: 'edit', onSelect: async () => {
        const name = await promptDialog({ title: 'Rename dataset', label: 'Name', value: d.name });
        if (name) store.commit(renameDataset(store.project, d.id, name), `Rename ${d.name}`);
      } },
      { label: 'Methods paragraph…', icon: 'file', disabled: state.status !== 'open', onSelect: () => showMethods(app, d) },
      { label: 'Locate its files…', icon: 'search', onSelect: () => app.locateDataset(d) },
      { label: 'Read its files again', icon: 'history', disabled: state.status === 'opening', onSelect: async () => {
        app.datasetStates.delete(d.id);
        const s = await app.ensureDataset(d);
        toast(s.status === 'open' ? `${d.name} read again: its files are those the project recorded.` : s.error, { kind: s.status === 'open' ? 'ok' : 'error' });
      } },
      '-',
      { label: 'Remove from the project', icon: 'trash', danger: true, onSelect: async () => {
        if (!(await confirmDialog({ title: 'Remove the dataset?', message: `Remove ${d.name} and its samples' annotation from the project? Its files are not touched.`, confirm: 'Remove', danger: true }))) return;
        store.commit(removeDataset(store.project, d.id), `Remove ${d.name}`);
      } },
    ]);
  }

  // --- The validation report ---------------------------------------------------------------------

  function report(d) {
    const state = app.datasetStates.get(d.id);
    const section = h('section.report', { 'aria-label': `Validation of ${d.name}` },
      h('div.section-title', `Validation · ${d.name}`));
    if (!state?.summary) {
      section.append(h('p.muted', state?.status === 'opening' || !state ? 'Reading the files…' : 'The report is shown once the files are read.'));
      return section;
    }
    const summary = state.summary;
    const findings = summary.report.findings;
    const sources = summary.sources;
    if (!findings.length) {
      section.append(h('div.callout.ok', h('p', `No findings: every check passed on ${plural(summary.rows, 'row')} of ${plural(sources.length, 'file')}.`)));
      return section;
    }
    section.append(h('p.muted.small', summary.report.fatal
      ? 'A fatal finding stops analysis of the dataset until it is fixed. Warnings are shown with every result that uses the data; notes inform.'
      : 'No fatal findings: the dataset can be analyzed. Warnings are shown with every result that uses it; notes inform.'));
    for (const f of findings) {
      const s = SEVERITY[f.severity] ?? SEVERITY.note;
      const of = f.of ? ` of ${count(f.of)} (${percent(f.count, f.of)})` : '';
      const examples = f.examples?.length
        ? h('details.finding-examples',
          h('summary', `${f.examples.length === 1 ? 'An example' : `${f.examples.length} examples`}`),
          h('table.data',
            h('thead', h('tr', h('th', 'File'), h('th.r', 'Line'), h('th', 'Sequence'), h('th', 'Sample'), h('th', 'Column'), h('th', 'Value'))),
            h('tbody', ...f.examples.map((e) => h('tr',
              h('td', sources[e.source]?.name ?? ''),
              h('td.r', e.line ? count(e.line) : ''),
              h('td.mono', e.sequenceId ?? ''),
              h('td', e.sample ?? ''),
              h('td.mono', e.column ?? ''),
              h('td.mono.value', e.value ?? ''))))))
        : null;
      section.append(h(`div.finding.${f.severity}`,
        h('div.finding-head',
          h(`span.badge.${s.badge}`, icon(s.icon), s.label),
          h('b', f.title),
          h('span.muted', `${count(f.count)}${of}`),
          h('span.spacer'),
          h('code.finding-code', { title: 'The finding\'s code, the same in every version' }, f.code)),
        f.detail ? h('p', f.detail) : null,
        f.remedy ? h('p.finding-remedy', h('b', 'What to do: '), f.remedy) : null,
        examples));
    }
    return section;
  }

  // --- Samples -----------------------------------------------------------------------------------

  // Every sample of the project's datasets, with what the program said of it.
  function sampleRows() {
    const rows = [];
    for (const d of store.project.datasets) {
      const summary = app.datasetStates.get(d.id)?.summary;
      d.samples.forEach((name, i) => rows.push({ dataset: d, name, summary: summary?.samples?.[i] }));
    }
    return rows;
  }

  function samplesSection() {
    const rows = sampleRows();
    const section = h('section', { 'aria-label': 'Samples' });
    section.append(h('div.section-title', `Samples · ${rows.length}`),
      h('div.btn-row', { style: { marginBottom: '8px' } },
        h('button.btn.small', { type: 'button', onclick: () => suggest(rows, 'names') }, icon('sparkles'), 'Suggest from file names'),
        h('button.btn.small', { type: 'button', onclick: () => suggest(rows, 'metadata') }, icon('braces'), 'Fill from metadata'),
        h('span.muted.small', 'The subject is the unit of every comparison between groups: name it for each sample.')));
    const table = h('table.data.samples-table',
      h('thead', h('tr',
        h('th', 'Sample'), h('th', 'Dataset'), h('th.r', 'Rows'), h('th.r', 'Productive'), h('th.r', 'Abundance'), h('th', 'Loci'),
        ...SAMPLE_FIELDS.map((f) => h('th', { title: f.hint }, f.label)))),
      h('tbody', ...rows.map(sampleRow)));
    section.append(h('div.table-scroll', { tabIndex: 0 }, table));
    return section;
  }

  function sampleRow({ dataset, name, summary }) {
    const a = annotation(store.project, dataset.id, name);
    const unit = datasetUnit(dataset);
    const loci = summary ? Object.entries(summary.loci).sort((x, y) => y[1] - x[1]).map(([l]) => l).join(' ') : '';
    return h('tr',
      h('td.mono', { title: summary?.repertoireId ? `repertoire_id ${summary.repertoireId}` : null }, name),
      h('td.dataset-name', { title: dataset.name }, dataset.name),
      h('td.r', summary ? count(summary.rows) : ''),
      h('td.r', summary ? percent(summary.productive, summary.rows) : ''),
      h('td.r', summary ? abundance(summary.abundance, unit) : ''),
      h('td', loci),
      ...SAMPLE_FIELDS.map((f) => h('td.annotation', h('input.input.small', {
        type: 'text', value: a.fields[f.id] ?? '', 'aria-label': `${f.label} of ${name}`,
        title: a.origin[f.id] && a.origin[f.id] !== 'manual' ? `From the ${a.origin[f.id]}` : f.hint,
        class: a.origin[f.id] && a.origin[f.id] !== 'manual' ? 'suggested' : '',
        onchange: (e) => store.commit(annotate(store.project, [{ dataset: dataset.id, sample: name, field: f.id, value: e.target.value }]), `Annotate ${name}`),
      }))));
  }

  // Suggests annotation from the samples' names (per dataset, whose names share a pattern) or
  // their metadata, shows what would change, and fills only empty fields unless asked.
  function suggest(rows, from) {
    const proposals = [];
    if (from === 'names') {
      for (const d of store.project.datasets) {
        const these = rows.filter((r) => r.dataset.id === d.id);
        for (const { field, values } of suggestFromNames(these.map((r) => r.name))) {
          these.forEach((r, i) => proposals.push({ dataset: d.id, sample: r.name, field, value: values[i], origin: 'file name' }));
        }
      }
    } else {
      for (const r of rows) {
        if (!r.summary) continue;
        for (const [field, value] of Object.entries(fieldsFromMetadata(r.summary))) {
          proposals.push({ dataset: r.dataset.id, sample: r.name, field, value, origin: 'metadata' });
        }
      }
    }
    if (!proposals.length) {
      toast(from === 'names' ? 'The samples\' names have no parts that differ in a way ClonoLoom recognizes.' : 'The samples have no metadata to fill fields from (AIRR Repertoire files, immunoSEQ sample tags, OAS and OTS data units, tcrdist3 clone tables).', { timeout: 4000 });
      return;
    }
    const fields = [...new Set(proposals.map((p) => p.field))];
    const chosen = new Set(fields);
    const replace = h('input', { type: 'checkbox' });
    const preview = h('table.data',
      h('thead', h('tr', h('th', 'Sample'), ...fields.map((f) => h('th', SAMPLE_FIELDS.find((x) => x.id === f).label)))),
      h('tbody', ...[...new Set(proposals.map((p) => `${p.dataset}\u0000${p.sample}`))].slice(0, 12).map((key) => {
        const [dataset, sample] = key.split('\u0000');
        return h('tr', h('td.mono', sample), ...fields.map((f) => h('td', proposals.find((p) => p.dataset === dataset && p.sample === sample && p.field === f)?.value ?? '')));
      })));
    showDialog({
      title: from === 'names' ? 'Suggest from file names' : 'Fill from metadata',
      width: 'wide',
      content: [
        h('p', from === 'names'
          ? 'The parts of the samples\' names that differ, read by what they look like: time points (d7, day28, baseline), tissues, cell subsets, batches, subjects (P01, HIP00110) and the condition. Check the fields to fill.'
          : 'Fields from the samples\' metadata: AIRR Repertoire files (subject, tissue, cell subset, time point, diagnosis or study group, HLA), immunoSEQ sample tags (HLA, CMV status), OAS and OTS data units (subject, tissue, cells sorted, disease) and tcrdist3 clone tables (subject, and the epitope as the condition).'),
        h('div.btn-row', ...fields.map((f) => {
          const box = h('input', { type: 'checkbox', checked: true, onchange: (e) => (e.target.checked ? chosen.add(f) : chosen.delete(f)) });
          return h('label.check', box, SAMPLE_FIELDS.find((x) => x.id === f).label);
        })),
        h('div', { style: { maxHeight: '40vh', overflow: 'auto', marginTop: '8px' } }, preview),
        proposals.length > 12 * fields.length ? h('p.muted.small', 'The first 12 samples shown.') : null,
        h('label.check', { style: { marginTop: '8px' } }, replace, 'Replace values already entered'),
      ],
      buttons: [
        { label: 'Cancel', ghost: true },
        { label: 'Fill', primary: true, onClick: () => {
          const edits = proposals.filter((p) => chosen.has(p.field) && (replace.checked || !annotation(store.project, p.dataset, p.sample).fields[p.field]));
          const next = annotate(store.project, edits, from === 'names' ? 'Suggested from file names' : 'Filled from metadata');
          if (next === store.project) toast('Nothing to fill: every field chosen already has a value.', { timeout: 3000 });
          else store.commit(next, from === 'names' ? 'Annotate from file names' : 'Annotate from metadata');
          return true;
        } },
      ],
    });
  }

  // --- The inspector's section: a dataset's files ------------------------------------------------

  app.inspector.setSection('dataset', () => {
    const focus = store.ui.focus;
    if (focus?.kind !== 'dataset') return null;
    const d = store.project.datasets.find((x) => x.id === focus.id);
    if (!d) return null;
    const summary = app.datasetStates.get(d.id)?.summary;
    const names = summary ? summary.sources.reduce((n, s) => n + Object.keys(s.geneNames ?? {}).length, 0) : 0;
    return h('section.inspector-section',
      h('h3', icon('database'), d.name),
      h('dl.kv',
        h('dt', 'Formats'), h('dd', d.formats.join(', ')),
        h('dt', 'Unit'), h('dd', UNITS[datasetUnit(d)] ?? datasetUnit(d)),
        h('dt', 'Samples'), h('dd', count(d.samples.length)),
        h('dt', d.singleCell ? 'Contigs' : 'Rearrangements'), h('dd', count(d.rows)),
        summary ? [h('dt', 'In memory'), h('dd', formatBytes(summary.bytes))] : null,
        names ? [h('dt', 'Gene names mapped'), h('dd', count(names))] : null),
      h('div.section-title', { style: { marginTop: '10px' } }, 'Files'),
      ...d.sources.map((s) => h('div.source-row',
        h('b', s.name),
        h('span.muted.small', `${formatBytes(s.size)} · ${s.format}`),
        h('span.mono.small', { title: `SHA-256 ${s.sha256}` }, `SHA-256 ${s.sha256.slice(0, 16)}…`),
        h('span.muted.small.path', { title: s.path }, s.path))),
      ...(summary?.sources ?? []).filter((s) => s.annotationParameters).map((s) => h('details', h('summary', `${s.imgtGroup}: annotation parameters`), h('pre', { style: { whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' } }, s.annotationParameters))),
      d.metadata?.length
        ? [h('div.section-title', { style: { marginTop: '10px' } }, 'Repertoire metadata'), ...d.metadata.map((m) => h('div.source-row',
          h('b', m.name),
          h('span.muted.small', formatBytes(m.size)),
          h('span.mono.small', { title: `SHA-256 ${m.sha256}` }, `SHA-256 ${m.sha256.slice(0, 16)}…`),
          h('span.muted.small.path', { title: m.path }, m.path)))]
        : null,
      d.credits?.length
        ? [h('div.section-title', { style: { marginTop: '10px' } }, 'Source and license'), ...d.credits.map((c) => h('div.source-row',
          h('b', c.title),
          h('span.small', h('a', { href: c.page, target: '_blank', rel: 'noopener noreferrer' }, c.source)),
          h('span.small', c.licenseUrl ? h('a', { href: c.licenseUrl, target: '_blank', rel: 'noopener noreferrer' }, c.license) : c.license),
          h('span.muted.small', `Cite: ${c.citation}`)))]
        : null,
      summary?.companions?.length
        ? [h('div.section-title', { style: { marginTop: '10px' } }, 'Beside them'), ...summary.companions.map((c) => h('div.source-row', h('span', c.path.split(/[\\/]/).pop()), h('span.muted.small', c.kind.replace('cellranger-', 'Cell Ranger '))))]
        : null,
      h('p.muted.small', { style: { marginTop: '8px' } }, 'Read where they are: ClonoLoom never changes, copies or moves them. The fingerprints tell the project when a file changed or moved.'),
      summary ? h('div.btn-row', { style: { marginTop: '8px' } }, methodsButton(app, d)) : null);
  });

  render();
  return {
    update(topics) {
      if (topics.has('project') || topics.has('datasets') || topics.has('focus') || topics.has('project-loaded')) render();
    },
    destroy() {
      app.inspector.setSection('dataset', null);
      root.remove();
    },
  };
}
