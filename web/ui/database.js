import { h } from './dom.js';
import { databaseWords } from '../lib/database.js';

export function databaseControls(o, choose) {
  const select = (key, label, choices) => h('label.kv-inline', label, h('select.input.small', { 'aria-label': label, onchange: (e) => choose({ [key]: e.target.value }) }, ...Object.entries(choices).map(([value, text]) => h('option', { value, selected: o[key] === value }, text))));
  const check = (key, label) => h('label.kv-inline', h('input', { type: 'checkbox', checked: o[key], onchange: (e) => choose({ [key]: e.target.checked }) }), label);
  return h('div.options-row',
    select('database', 'Database', { vdjdb: 'VDJdb', 'simulated-vaccine': 'Simulated vaccine database' }),
    select('metric', 'Match', { exact: 'Exact CDR3', tcrdist: 'TCRdist radius' }),
    select('species', 'Receptor species', { HomoSapiens: 'Human', MusMusculus: 'Mouse', MacacaMulatta: 'Rhesus macaque' }),
    check('genes', 'Require matching V/J'),
    h('label.kv-inline', 'Minimum score', h('select.input.small', { 'aria-label': 'Minimum score', onchange: (e) => choose({ minScore: Number(e.target.value) }) }, ...[0, 1, 2, 3].map((n) => h('option', { value: n, selected: o.minScore === n }, String(n))))),
    o.metric === 'tcrdist' ? h('label.kv-inline', 'Radius', h('input.input.small', { type: 'number', min: 0, max: 1000, value: o.radius, 'aria-label': 'Specificity radius', onchange: (e) => choose({ radius: Number(e.target.value) }) })) : null,
    o.metric === 'tcrdist' ? check('cdr3Only', 'CDR3s alone') : null);
}

export function databaseResult(r, page, showPage, follow, showClonotype) {
  page = Math.max(0, Math.min(page, Math.ceil(r.matches.length / 100) - 1));
  const rows = r.matches.slice(page * 100, (page + 1) * 100);
  return h('div',
    h('div.callout', 'These are matches to reported chain evidence. Neither an exact chain match nor a nearby receptor proves specificity of the complete receptor. Epitope shares can overlap and need not sum to one.'),
    h('p', `${r.matches.length.toLocaleString()} chain–record matches; ${r.epitopes.length} epitope/antigen groups. ${databaseWords(r.specificity)}.`),
    h('p.muted.small', `${r.database.reference}: SHA-256 ${r.database.sha256}. ${r.database.license}. ${r.database.citation}`),
    ...Object.entries(r.excluded).map(([why, n]) => h('p.muted.small', `Not compared: ${n} ${why}.`)),
    h('div.section-title', 'Each sample’s share matched to an epitope'),
    h('div.table-scroll', { tabIndex: 0 }, h('table.data.specificity-epitopes',
      h('thead', h('tr', h('th', 'Epitope / antigen'), h('th', 'Tracking'), ...r.samples.map((s) => h('th.r', s)))),
      h('tbody', ...r.epitopes.map((e) => {
        const button = h('button.btn.small', {
          type: 'button', disabled: e.clonotypes.length > 1000,
          title: e.clonotypes.length > 1000 ? 'Tracking accepts at most 1000 IDs; narrow the matching options.' : 'Follow these matched clonotypes through annotated time points',
          onclick: () => follow(e),
        }, `Follow ${e.clonotypes.length} clonotype${e.clonotypes.length === 1 ? '' : 's'}`);
        return h('tr',
          h('td', h('span.mono', e.epitope), h('div.small.muted', `${e.antigen} · ${e.species}`)),
          h('td', button),
          ...e.shares.map((v) => h('td.r', v === null ? '—' : `${(100 * v).toFixed(3)}%`)));
      })))),
    h('p.muted.small', 'Each clonotype contributes once per epitope/antigen group, regardless of repeated records or matched chains. Shares use all retained abundance. Tracking displays at most 200 clonotypes per subject and reports those omitted.'),
    h('div.section-title', 'Chain evidence'),
    h('div.btn-row', h('button.btn.small', { type: 'button', disabled: page === 0, onclick: () => showPage(page - 1) }, 'Previous'), h('span', `Page ${page + 1} of ${Math.max(1, Math.ceil(r.matches.length / 100))}`), h('button.btn.small', { type: 'button', disabled: (page + 1) * 100 >= r.matches.length, onclick: () => showPage(page + 1) }, 'Next')),
    h('div.table-scroll', { tabIndex: 0 }, h('table.data', h('thead', h('tr', ...['Query chain', 'Database CDR3 / genes', 'Epitope / antigen', 'Species / MHC', 'Score / reference', 'Distance'].map((s) => h('th', s)))), h('tbody', ...rows.map((m) => {
      const c = r.clonotypes[m.clonotype], ch = c.chains[m.chain], x = r.records[m.record];
      return h('tr', h('td', h('button.btn.small.mono', { type: 'button', onclick: () => showClonotype(c.id) }, `${ch.locus} ${ch.junction}`)), h('td', h('div.mono', x.cdr3), h('div.small', `${x.v} ${x.j}`)), h('td', `${x.epitope} · ${x.antigen} (${x.antigenSpecies})`), h('td', `${x.species} · ${x.mhcA} / ${x.mhcB} · ${x.mhcClass}`), h('td', { title: x.method }, `${x.score} · ${x.reference} · row ${x.row}${x.complex !== '0' ? ` · complex ${x.complex}` : ''}`), h('td.r', String(m.distance)));
    })))));
}
