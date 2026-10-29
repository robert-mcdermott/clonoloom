// Native reporting states only; no browser evidence parsing or completeness score.
import { h } from './dom.js';

const states = {
  absent: 'Absent key', null: 'Explicit null', empty: 'Explicit empty value',
  not_applicable: 'Explicit not_applicable', not_collected: 'Explicit not_collected',
  missing: 'Explicit missing (collected, unavailable)', 'unknown-marker': 'Nonstandard unknown marker',
  'value-present': 'Value supplied', 'structure-review': 'Structure supplied · review members',
  'shape-review': 'Unexpected value shape · review',
};
const levels = { essential: 'Essential · substantive information required', important: 'Important · explicit null-like permitted', defined: 'Defined · optional reporting', scope: 'Parent scope · children not assessed' };

export function miairrChecklist(report, recordLabel) {
  if (!report) return h('p.muted.small', 'MiAIRR checklist unavailable for this record.');
  const body = h('div.miairr-body', h('p', h('b', recordLabel)), h('p.small.mono', `${report.version} · AIRR schema ${report.schemaVersion} · declared ${report.declaredVersion || 'not supplied'}`),
    h('p.callout', report.notice));
  if (report.status === 'review') {
    body.append(h('p.small', 'Essential fields require substantive information; important fields allow explicit null-like reporting. Defined fields are optional, but prescribe where matching information belongs. These are reporting levels, not schema-required keys or a quality ranking.'),
      h('p.small', 'Conditional applicability needs source review. An absent field is never automatically not applicable. Explicit reasons remain assertions by the metadata author.'),
      h('p.small.miairr-counts', Object.entries(report.counts).map(([state, count]) => `${states[state] ?? state}: ${count}`).join(' · ')));
    const select = h('select', { 'aria-label': 'MiAIRR evidence group' }, h('option', { value: '' }, 'All reporting groups'));
    const groups = [...new Set(report.items.map((row) => row.group))];
    for (const group of groups) select.append(h('option', { value: group }, group));
    const rows = h('tbody');
    const render = () => {
      rows.replaceChildren(...report.items.filter((row) => !select.value || row.group === select.value).map((row) => h('tr',
        h('th', { scope: 'row', style: { overflowWrap: 'anywhere', minWidth: '150px' } }, h('code', row.path), h('div.small.muted', row.group)),
        h('td', levels[row.level] ?? row.level), h('td', states[row.state] ?? row.state),
        h('td', row.condition ? h('p.small', row.condition) : null, row.remedy))));
    };
    select.addEventListener('change', render); render();
    body.append(h('label', 'Reporting group ', select),
      h('div', { tabIndex: 0, role: 'region', 'aria-label': 'MiAIRR evidence paths and remedies', style: { overflow: 'auto', maxHeight: '420px', marginTop: '12px' } },
        h('table.data.miairr-table', h('caption.small', 'Paths are relative to this repertoire record. Counts cover all groups; filtering only changes displayed rows.'),
          h('thead', h('tr', ...['Evidence path', 'Reporting level', 'Observed state', 'Review / remedy'].map((label) => h('th', { scope: 'col' }, label)))), rows)));
  }
  body.append(h('p.small', h('a', { href: 'https://docs.airr-community.org/en/v2.0.0/miairr/data_elements.html', target: '_blank', rel: 'noopener noreferrer' }, 'MiAIRR 2.0 data elements'), ' · ',
    h('a', { href: 'https://docs.airr-community.org/en/v2.0.0/datarep/airr_schema_requirement_levels.html', target: '_blank', rel: 'noopener noreferrer' }, 'AIRR reporting levels')));
  return h('details.miairr-checklist', h('summary', 'Review MiAIRR reporting completeness'), body);
}
