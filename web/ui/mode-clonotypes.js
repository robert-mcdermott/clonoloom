// The Clonotypes view (clonoloom-spec/requirements.md, C1–C3, C6): a dataset's clonotypes under
// the definition chosen for it and its filter, as a table the program sorts, searches and pages
// (api_clonotypes.go), so that a million rows scroll as smoothly as a hundred. The definition is
// chosen from the presets or edited, and said in words above the table; a clonotype selected is
// described in the inspector: its chains, its abundance in every sample, and the rearrangements or
// cells it is made of.

import { h, icon, clear, debounce } from './dom.js';
import { showDialog, toast } from './overlays.js';
import { clonotypeDetail, clonotypePage, clonotypePresets } from './storage.js';
import { UNITS, datasetUnit } from '../lib/datasets.js';
import { datasetFilter } from '../lib/filters.js';
import { JUNCTIONS, LEVELS, datasetDefinition, sameMeaning, saveDefinition, useDefinition } from '../lib/definitions.js';

const ROW = 30;
// The header row's height, above the rows inside the scrolling area.
const HEAD = 29;
const PAGE = 200;
const MAX_SAMPLE_COLUMNS = 8;
const count = (n) => (Number.isFinite(n) ? Number(n).toLocaleString('en-US', { maximumFractionDigits: 2 }) : '—');
const pct = (v) => (Number.isFinite(v) ? `${(100 * v).toFixed(v < 0.001 ? 3 : 2)}%` : '—');

export function mountClonotypes(app, container) {
  const { store } = app;
  const root = h('div.view');
  const datasetSelect = h('select.input.small', { 'aria-label': 'Dataset', onchange: (e) => { chosen = e.target.value; reset(); } });
  const definitionSelect = h('select.input.small', { 'aria-label': 'Clonotype definition', onchange: (e) => chooseDefinition(e.target.value) });
  const search = h('input.input.small', { type: 'search', placeholder: 'Junction, gene or ID', 'aria-label': 'Search the clonotypes', style: { width: '200px' }, oninput: debounce(() => { query.text = search.value; reload(); }, 250) });
  const head = h('div.workbench-head',
    h('h1', icon('clonotypes'), 'Clonotypes'),
    h('span.spacer'),
    h('label.unit-confirm', h('span.muted', 'Dataset'), datasetSelect),
    h('label.unit-confirm', h('span.muted', 'Definition'), definitionSelect),
    h('button.btn.small', { type: 'button', onclick: () => editDefinition() }, icon('edit'), 'Edit…'),
    search);
  const info = h('div.clonotype-info');
  const header = h('div.ctable-head', { role: 'row' });
  const spacer = h('div.ctable-spacer');
  const windowEl = h('div.ctable-window', { role: 'rowgroup' });
  const scroller = h('div.ctable-scroll', { role: 'grid', tabIndex: 0, 'aria-label': 'Clonotypes', onscroll: () => paint() }, header, spacer, windowEl);
  const body = h('div.view-body.clonotypes-view', info, h('div.ctable', scroller));
  root.append(head, body);
  container.append(root);

  let chosen = store.ui.focus?.kind === 'dataset' ? store.ui.focus.id : store.project.datasets[0]?.id ?? null;
  let presets = [];
  let sort = { by: 'total', desc: true };
  const query = { text: '' };
  let meta = null; // the first page's answer: counts, samples, definition in words
  let pages = new Map();
  let generation = 0;
  let selected = null;
  // What the table was last made from: its dataset, definition and filter.
  let lastKey = '';
  const loci = () => {
    const state = app.datasetStates.get(dataset()?.id);
    return Object.keys(state?.summary?.samples?.reduce((acc, s) => Object.assign(acc, s.loci), {}) ?? {});
  };

  const dataset = () => store.project.datasets.find((d) => d.id === chosen) ?? store.project.datasets[0] ?? null;
  const definition = () => datasetDefinition(dataset(), loci());

  function request(offset) {
    const d = dataset();
    return { dataset: d.id, definition: definition(), filter: datasetFilter(store.project, d)?.rules ?? {}, sort: sort.by, desc: sort.desc, query: { text: query.text }, offset, limit: PAGE };
  }

  function chooseDefinition(value) {
    const d = dataset();
    if (!d) return;
    const options = definitionOptions();
    const option = options.find((o) => o.key === value);
    if (!option) return;
    store.commit(useDefinition(store.project, d.id, option.definition), `Define clonotypes of ${d.name}`);
  }

  function definitionOptions() {
    const out = presets.map((p, i) => ({ key: `preset:${i}`, label: p.definition.name, definition: p.definition }));
    (store.project.definitions ?? []).forEach((def, i) => out.push({ key: `saved:${i}`, label: `${def.name} (saved)`, definition: def }));
    const current = definition();
    if (!out.some((o) => sameMeaning(o.definition, current) && o.definition.name === current.name)) out.unshift({ key: 'current', label: current.name ?? 'This dataset\'s definition', definition: current });
    return out;
  }

  function drawControls() {
    const { datasets } = store.project;
    const d = dataset();
    clear(datasetSelect).append(...datasets.map((x) => h('option', { value: x.id, selected: d && x.id === d.id }, x.name)));
    datasetSelect.disabled = datasets.length < 2;
    const current = definition();
    clear(definitionSelect).append(...definitionOptions().map((o) => h('option', { value: o.key, selected: sameMeaning(o.definition, current) && o.definition.name === current.name }, o.label)));
  }

  function reset() {
    sort = { by: 'total', desc: true };
    reload();
  }

  async function reload() {
    drawControls();
    const d = dataset();
    lastKey = d ? JSON.stringify([d.id, d.definition, d.filter]) : '';
    const token = ++generation;
    pages = new Map();
    meta = null;
    if (!d) {
      clear(info).append(h('div.empty', icon('clonotypes'), h('h3', 'No data'), h('p', 'Open data first: clonotypes are defined on a dataset\'s rearrangements.'),
        h('button.btn.primary', { type: 'button', onclick: () => app.openPathDialog() }, icon('folder'), 'Open data')));
      header.hidden = true;
      scroller.hidden = true;
      return;
    }
    header.hidden = false;
    scroller.hidden = false;
    clear(info).append(h('p.muted', 'Grouping the clonotypes…'));
    const state = await app.ensureDataset(d);
    if (token !== generation) return;
    if (state.status !== 'open') {
      clear(info).append(h('div.callout.danger', h('p', state.error ?? 'The dataset is not open.')));
      return;
    }
    drawControls();
    try {
      const first = await clonotypePage(request(0));
      if (token !== generation) return;
      meta = first;
      pages.set(0, first.rows);
      app.clonotypeContext = { dataset: d.id, request: request(0), describe: first.describe, unit: first.unit, name: definition().name };
      store.notify(['clonotype-context']);
      drawInfo();
      drawHeader();
      spacer.style.height = `${first.matched * ROW}px`;
      scroller.scrollTop = 0;
      paint();
    } catch (error) {
      if (token !== generation) return;
      clear(info).append(h('div.callout.danger', h('p', error.message)));
    }
  }

  function drawInfo() {
    const d = dataset();
    const filter = datasetFilter(store.project, d);
    const unit = meta.singleCell ? 'cells' : datasetUnit(d) || meta.unit;
    const excluded = Object.entries(meta.excluded ?? {}).filter(([, n]) => n > 0).map(([why, n]) => `${count(n)} ${why}`);
    clear(info).append(
      h('p.clonotype-definition', h('b', definition().name ?? 'Clonotypes'), ': ', meta.describe, '.'),
      h('p.muted.small',
        `${count(meta.matched)}${meta.matched !== meta.count ? ` of ${count(meta.count)}` : ''} clonotype${meta.count === 1 ? '' : 's'}`,
        meta.singleCell ? ` of ${count(meta.cells)} cells` : '',
        ` · abundance in ${UNITS[unit] ?? 'counts'}`,
        filter ? ` · filter ${filter.name} v${filter.version}` : ' · no filter',
        excluded.length ? ` · in no clonotype: ${excluded.join(', ')}` : ''));
  }

  function sampleColumns() {
    return meta && meta.samples.length > 1 && meta.samples.length <= MAX_SAMPLE_COLUMNS ? meta.samples : [];
  }

  function template() {
    const n = sampleColumns().length;
    return `52px minmax(260px, 2.4fr) minmax(130px, 1fr) minmax(110px, 0.9fr) 100px 84px 70px${n ? ` repeat(${n}, 96px)` : ''}`;
  }

  // The narrowest the columns go: wider than the view, the table scrolls sideways, header and all.
  function minWidth() {
    return 52 + 260 + 130 + 110 + 100 + 84 + 70 + 96 * sampleColumns().length;
  }

  function drawHeader() {
    clear(header);
    header.style.gridTemplateColumns = template();
    header.style.minWidth = `${minWidth()}px`;
    windowEl.style.minWidth = `${minWidth()}px`;
    const col = (label, by, numeric = false, title = null) => h(`button.ctable-th${numeric ? '.r' : ''}${sort.by === by ? '.sorted' : ''}`, {
      type: 'button', role: 'columnheader', title: title ?? `Sort by ${label.toLowerCase()}`,
      'aria-sort': sort.by === by ? (sort.desc ? 'descending' : 'ascending') : 'none',
      onclick: () => { sort = sort.by === by ? { by, desc: !sort.desc } : { by, desc: numeric }; reload(); },
    }, label, sort.by === by ? icon(sort.desc ? 'chevronDown' : 'chevronUp') : null);
    header.append(h('span.ctable-th.r', '#'), col('Junction', 'junction'), col('V', 'v'), col('J', 'j'), col('Total', 'total', true), h('span.ctable-th.r', 'Frequency'), col('Samples', 'samples', true),
      ...sampleColumns().map((s, i) => col(s.name, `sample:${i}`, true, `Sort by the abundance in ${s.name}`)));
  }

  const fetching = new Set();
  async function ensurePage(n) {
    if (pages.has(n) || fetching.has(n)) return;
    fetching.add(n);
    const token = generation;
    try {
      const page = await clonotypePage(request(n * PAGE));
      if (token !== generation) return;
      pages.set(n, page.rows);
      paint();
    } catch (error) {
      toast(error.message, { kind: 'error' });
    } finally {
      fetching.delete(n);
    }
  }

  function rowAt(k) {
    const page = pages.get(Math.floor(k / PAGE));
    return page?.[k % PAGE] ?? null;
  }

  // Draws the rows in view (and a few around them); fetches the pages they are on.
  function paint() {
    if (!meta) return;
    const top = scroller.scrollTop;
    const height = scroller.clientHeight || 600;
    const first = Math.max(0, Math.floor((top - HEAD) / ROW) - 10);
    const last = Math.min(meta.matched, Math.ceil((top + height - HEAD) / ROW) + 10);
    for (let p = Math.floor(first / PAGE); p <= Math.floor(Math.max(first, last - 1) / PAGE); p += 1) ensurePage(p);
    clear(windowEl);
    windowEl.style.transform = `translateY(${HEAD + first * ROW}px)`;
    const tpl = template();
    const samples = sampleColumns();
    for (let k = first; k < last; k += 1) {
      const r = rowAt(k);
      if (!r) {
        windowEl.append(h('div.ctable-row.loading', { style: { gridTemplateColumns: tpl } }, h('span.r.muted', String(k + 1)), h('span.muted', '…')));
        continue;
      }
      const chains = r.chains.length ? r.chains : [{ locus: '', junction: r.vendor ?? '' }];
      windowEl.append(h(`div.ctable-row${selected === r.id ? '.selected' : ''}`, {
        role: 'row', style: { gridTemplateColumns: tpl }, title: r.id,
        onclick: () => select(r.id),
      },
      h('span.r.muted', String(r.rank)),
      h('span.junctions', ...chains.map((c) => h('span.chain', c.locus ? h('span.locus', c.locus) : null, h('span.mono', c.junction)))),
      h('span.genes', chains.map((c) => c.v).filter(Boolean).join(' / ') || h('span.muted', '—')),
      h('span.genes', chains.map((c) => c.j).filter(Boolean).join(' / ') || h('span.muted', '—')),
      h('span.r', count(r.total)),
      h('span.r', pct(r.frequency)),
      h('span.r', String(r.samples)),
      ...samples.map((s, i) => h('span.r', r.in[String(i)] ? count(r.in[String(i)]) : h('span.muted', '·')))));
    }
  }

  function select(id) {
    selected = id;
    app.focusItem({ kind: 'clonotype', id });
    paint();
  }

  // The definition editor: a definition of the user's own, from the one in use.
  function editDefinition() {
    const d = dataset();
    if (!d) return;
    const def = { chains: [], junction: 'aa', v: 'gene', j: 'gene', cells: 'primary', ...structuredClone(definition()) };
    delete def.vendor;
    const name = h('input.input', { type: 'text', value: def.name ?? '', 'aria-label': 'Name' });
    const chainsSelect = h('select.input', { 'aria-label': 'Chains' },
      ...[['', 'Each locus separately'], ['TRB', 'TRB chains'], ['TRA', 'TRA chains'], ['IGH', 'IGH chains'], ['TRA,TRB', 'Paired TRA and TRB (single cell)'], ['IGH,light', 'Paired heavy and light (single cell)'], ['TRG,TRD', 'Paired TRG and TRD (single cell)']]
        .map(([v, label]) => h('option', { value: v, selected: (def.chains ?? []).join(',') === v }, label)));
    const radio = (group, options, value) => h('div.segmented', { role: 'radiogroup', 'aria-label': group }, ...Object.entries(options).map(([v, label]) => {
      const button = h(`button${value() === v ? '.active' : ''}`, { type: 'button', role: 'radio', 'aria-checked': value() === v ? 'true' : 'false', onclick: () => { def[group] = v; for (const b of button.parentElement.children) b.classList.toggle('active', b === button); } }, label);
      return button;
    }));
    const productive = h('input', { type: 'checkbox', checked: def.productiveOnly !== false, 'aria-label': 'Productive rearrangements only' });
    const requireAll = h('input', { type: 'checkbox', checked: Boolean(def.requireAll), 'aria-label': 'Cells must have both chains' });
    const save = h('input', { type: 'checkbox', checked: false, 'aria-label': 'Save it for the project\'s other datasets' });
    showDialog({
      title: 'Clonotype definition',
      width: 'wide',
      content: [
        h('p.muted', 'How rearrangements (or cells) are grouped into clonotypes. The definition is stated with every table, figure and methods sentence that counts clonotypes; changing it changes the clonotypes\' IDs, never the data.'),
        h('label.field', h('span', 'Name'), name),
        h('label.field', h('span', 'Chains'), chainsSelect),
        h('div.field', h('span', 'Junction'), radio('junction', JUNCTIONS, () => def.junction)),
        h('div.field', h('span', 'V gene'), radio('v', LEVELS, () => def.v)),
        h('div.field', h('span', 'J gene'), radio('j', LEVELS, () => def.j)),
        h('label.check', productive, 'Productive rearrangements only (always, for amino-acid junctions)'),
        h('div.field', h('span', 'Single cells'), radio('cells', { primary: 'primary chain of each locus (most UMIs)', all: 'every productive chain' }, () => def.cells)),
        h('label.check', requireAll, 'Paired chains: leave out cells without both (else a cell with one chain is grouped with cells of that chain alone)'),
        h('label.check', { style: { marginTop: '8px' } }, save, 'Save it for the project\'s other datasets'),
      ],
      buttons: [
        { label: 'Cancel', ghost: true },
        { label: 'Use', primary: true, onClick: () => {
          const next = {
            name: name.value.trim() || 'Custom definition',
            chains: chainsSelect.value ? chainsSelect.value.split(',') : [],
            junction: def.junction, v: def.v, j: def.j,
            productiveOnly: def.junction === 'aa' || productive.checked,
            cells: def.cells, requireAll: requireAll.checked,
          };
          if (next.chains.length === 2 && !d.singleCell) {
            toast('Paired chains need single-cell data: bulk sequencing does not say which chains share a cell.', { kind: 'error', timeout: 5000 });
            return false;
          }
          let project = useDefinition(store.project, d.id, next);
          if (save.checked) project = saveDefinition(project, next);
          store.commit(project, `Define clonotypes of ${d.name}`);
          return true;
        } },
      ],
    });
  }

  // The inspector's section: the clonotype selected, in full.
  let detail = null;
  app.inspector.setSection('clonotype', () => {
    const focus = store.ui.focus;
    if (focus?.kind !== 'clonotype') return null;
    if (!detail || detail.clonotype.id !== focus.id) {
      const ctx = app.clonotypeContext;
      if (ctx) {
        clonotypeDetail({ ...ctx.request, id: focus.id }).then((answer) => {
          detail = answer;
          app.inspector.render();
        }).catch((error) => toast(error.message, { kind: 'error' }));
      }
      return h('section.inspector-section', h('h3', icon('clonotypes'), 'Clonotype'), h('p.muted', 'Reading…'));
    }
    const c = detail.clonotype;
    const unit = UNITS[detail.unit] ?? 'counts';
    return h('section.inspector-section',
      h('h3', icon('clonotypes'), 'Clonotype'),
      h('p.mono.small', { title: 'The ID is made from the definition and the clonotype\'s chains: the same clonotype has the same ID in every run.' }, c.id),
      ...c.chains.map((ch) => h('dl.kv',
        h('dt', 'Locus'), h('dd', ch.locus),
        h('dt', 'Junction'), h('dd.mono', ch.junction),
        ch.v ? [h('dt', 'V'), h('dd', ch.v)] : null,
        ch.j ? [h('dt', 'J'), h('dd', ch.j)] : null)),
      c.vendor ? h('dl.kv', h('dt', 'Source\'s clonotype'), h('dd', c.vendor)) : null,
      h('div.section-title', { style: { marginTop: '10px' } }, `In ${detail.in.length} sample${detail.in.length === 1 ? '' : 's'}`),
      h('table.data', h('thead', h('tr', h('th', 'Sample'), h('th.r', unit), h('th.r', 'Frequency'))),
        h('tbody', ...detail.in.map((s) => h('tr', h('td', s.sample), h('td.r', count(s.abundance)), h('td.r', pct(s.frequency)))))),
      h('div.section-title', { style: { marginTop: '10px' } }, `${count(detail.memberCount)} ${detail.unit === 'cells' ? 'contigs' : 'rearrangements'}`),
      h('div', { style: { maxHeight: '280px', overflow: 'auto' } }, h('table.data.members',
        h('thead', h('tr', h('th', detail.unit === 'cells' ? 'Cell' : 'Sequence'), h('th', 'V / J as given'), h('th.r', 'Count'))),
        h('tbody', ...detail.members.slice(0, 100).map((m) => h('tr', { title: `${m.sample}, line ${m.line}${m.junction ? `\n${m.junction}` : ''}` },
          h('td.mono', m.cell || m.sequenceId || `line ${m.line}`),
          h('td', [m.v, m.j].filter(Boolean).join(' / ')),
          h('td.r', count(m.count))))))),
      detail.memberCount > 100 ? h('p.muted.small', `The first 100 of ${count(detail.memberCount)}.`) : null,
      h('p.muted.small', { style: { marginTop: '8px' } }, `Defined as: ${detail.describe}.`));
  });

  clonotypePresets().then((list) => { presets = list; drawControls(); }).catch(() => {});
  reload();
  const onResize = () => paint();
  window.addEventListener('resize', onResize);
  return {
    update(topics) {
      if (topics.has('focus') && store.ui.focus?.kind === 'dataset' && store.ui.focus.id !== chosen) {
        chosen = store.ui.focus.id;
        reset();
        return;
      }
      if (topics.has('project-loaded') || topics.has('datasets')) {
        reload();
        return;
      }
      if (topics.has('project')) {
        // A new definition or filter: the table again; otherwise nothing that changes it.
        const d = dataset();
        if ((d ? JSON.stringify([d.id, d.definition, d.filter]) : '') !== lastKey) reload();
      }
    },
    destroy() {
      window.removeEventListener('resize', onResize);
      app.inspector.setSection('clonotype', null);
      root.remove();
    },
  };
}
