// The columns of a table ClonoLoom does not recognize, mapped (clonoloom-spec/requirements.md, D7):
// its header and first rows shown, each field ClonoLoom uses matched to a column (guessed from the
// columns' names), the count's unit said, and the whole file checked column by column by the
// program before it is opened through the mapping, which the dataset keeps (reads.Mapping).

import { h, clear } from './dom.js';
import { showDialog, toast } from './overlays.js';

const UNITS = [['', 'Choose…'], ['reads', 'Reads'], ['umis', 'UMIs'], ['templates', 'Templates'], ['cells', 'Cells']];

async function post(url, body) {
  const response = await fetch(url, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
  const answer = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(answer.error ?? `${url}: HTTP ${response.status}`);
  return answer;
}

// mapColumns asks how to read a table (file: { path } on this computer, or { sha256, name } in the
// library), and answers the mapping the user opened it with, or null.
export async function mapColumns(file) {
  const where = file.sha256 ? { sha256: file.sha256 } : { path: file.path };
  const name = file.name ?? file.path.split(/[\\/]/).pop();
  let inspected;
  try {
    inspected = await post('api/datasets/inspect', where);
  } catch (error) {
    toast(error.message, { kind: 'error' });
    return null;
  }
  const { inspection, fields } = inspected;
  const columns = { ...inspection.guess.columns };
  const choose = (field) => h('select.input.small', { 'aria-label': field.label, onchange: (e) => { columns[field.id] = e.target.value; checked.textContent = ''; } },
    h('option', { value: '' }, '—'), ...inspection.header.map((c) => h('option', { value: c, selected: columns[field.id] === c }, c)));
  const unit = h('select.input.small', { 'aria-label': 'What the count counts' }, ...UNITS.map(([v, label]) => h('option', { value: v, selected: v === (inspection.guess.unit ?? '') }, label)));
  const sample = h('input.input.small', { type: 'text', 'aria-label': 'Sample name', placeholder: name.replace(/\.(tsv|csv|txt|tab)(\.gz)?$/i, '') });
  const checked = h('div.mapping-check');
  const mapping = () => {
    const cols = Object.fromEntries(Object.entries(columns).filter(([, c]) => c));
    return { columns: cols, unit: cols.count ? unit.value : '', sample: cols.sample ? '' : sample.value.trim() };
  };
  const preview = h('div.table-scroll', { tabIndex: 0 }, h('table.data.mapping-preview',
    h('thead', h('tr', ...inspection.header.map((c) => h('th', c)))),
    h('tbody', ...inspection.rows.slice(0, 6).map((r) => h('tr', ...inspection.header.map((_, i) => h('td', r[i] ?? '')))))));
  const content = h('div.mapping',
    h('p', `ClonoLoom does not recognize the columns of ${name} (a ${inspection.separator}-separated table). Say which column holds what; ClonoLoom guessed from their names. A junction (amino acids or nucleotides) is needed; the rest is as the table has it.`),
    preview,
    h('div.mapping-fields', ...fields.map((f) => h('label.mapping-field', h('span', f.label), choose(f))),
      h('label.mapping-field', h('span', 'The count counts'), unit),
      h('label.mapping-field', h('span', 'Sample (when no column)'), sample)),
    checked);
  // Checks the whole file through the mapping: answers whether it can be opened.
  async function check() {
    clear(checked).append(h('p.muted', 'Checking every row…'));
    try {
      const result = await post('api/datasets/check', { ...where, mapping: mapping() });
      clear(checked).append(
        h('p', `${result.rows.toLocaleString('en-US')} rows; ${result.samples.length} sample${result.samples.length === 1 ? '' : 's'}: ${result.samples.slice(0, 6).join(', ')}${result.samples.length > 6 ? ', …' : ''}.`),
        h('table.data.mapping-checks', h('thead', h('tr', h('th', 'Field'), h('th', 'Column'), h('th.r', 'Valid'), h('th', 'Not valid'), h('th', 'Note'))),
          h('tbody', ...result.columns.map((c) => h('tr',
            h('td', fields.find((f) => f.id === c.field)?.label ?? c.field), h('td', c.column || '—'),
            h(`td.r${c.values && c.valid < c.values ? '.attention' : ''}`, c.values ? `${c.valid.toLocaleString('en-US')} of ${c.values.toLocaleString('en-US')}` : '—'),
            h('td.mono', (c.invalid ?? []).join(', ')), h('td.muted.small', c.note ?? ''))))));
      return true;
    } catch (error) {
      clear(checked).append(h('div.callout.danger', h('p', error.message)));
      return false;
    }
  }
  return new Promise((resolve) => {
    showDialog({
      title: `Map the columns of ${name}`,
      width: 'wide',
      content,
      buttons: [
        { label: 'Cancel', ghost: true, value: null },
        { label: 'Check', onClick: async () => { await check(); return false; } },
        { label: 'Open', primary: true, onClick: async () => ((await check()) ? mapping() : false) },
      ],
      onClose: (result) => resolve(result ?? null),
    });
  });
}
