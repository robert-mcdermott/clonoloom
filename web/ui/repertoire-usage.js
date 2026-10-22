// The Repertoire view's Genes and Junctions parts (clonoloom-spec/requirements.md, R3), from one
// gene usage result (api_usage.go): the V, D, J and C genes each sample's clonotypes use, at the
// gene or family level, with one gene across the samples and the V–J pairs of one; the junctions'
// lengths (spectratypes) and mean length, their amino acids, and the amino acids at each IMGT
// position of the commonest lengths. Each by clonotypes or by abundance, for the samples or for
// their conditions (each sample counted once). The program computes every number; this draws.

import { h } from './dom.js';
import { heatmap, intervals, lines } from '../lib/plot.js';

export const USAGE_WORDS = 'Counted by the program over every sample\'s clonotypes under this definition and filter: the V, D, J and C genes they use and their V–J pairs, at the gene and the family level; the lengths of their junctions; and the amino acids in them, overall and at each IMGT position. Each is a share of the sample\'s clonotypes and of its abundance, with a 95% interval from 200 bootstrap replicates of the sample\'s clonotypes; each condition is summarized by the mean of its samples. The result is kept with the project.';

const pct = (v) => {
  if (!Number.isFinite(v)) return '—';
  if (v === 0) return '0%';
  if (v < 0.001) return '<0.1%';
  return `${(100 * v).toFixed(v < 0.1 ? 1 : 0)}%`;
};
const collator = new Intl.Collator('en', { numeric: true });
const WEIGHTS = { clonotypes: 'clonotypes', abundance: 'abundance' };

function segmented(label, options, value, set) {
  return h('div.segmented', { role: 'radiogroup', 'aria-label': label },
    ...options.map(([v, text]) => h(`button${v === value ? '.active' : ''}`, { type: 'button', role: 'radio', 'aria-checked': v === value ? 'true' : 'false', onclick: () => set(v) }, text)));
}

function picker(label, options, value, set) {
  return h('select.input.small', { 'aria-label': label, onchange: (e) => set(e.target.value) },
    ...options.map(([v, text]) => h('option', { value: v, selected: v === value }, text)));
}

// What both parts share: the rows (samples or conditions), the locus, the weighting, and the
// controls that choose them.
function setting(ctx) {
  const { state, ui, render } = ctx;
  const result = state.result;
  const loci = [...new Set(result.samples.flatMap((s) => s.loci.map((l) => l.locus)))].sort();
  ui.locus = loci.includes(ui.locus) ? ui.locus : loci[0];
  ui.weight = WEIGHTS[ui.weight] ? ui.weight : 'clonotypes';
  const groups = result.groups ?? [];
  if (ui.rows !== 'groups' || !groups.length) ui.rows = 'samples';
  const units = ui.rows === 'groups' ? groups : result.samples;
  const set = (key) => (v) => {
    ui[key] = v;
    render();
  };
  const controls = [
    loci.length > 1 ? segmented('Locus', loci.map((l) => [l, l]), ui.locus, set('locus')) : null,
    segmented('Weighting', [['clonotypes', 'Clonotypes'], ['abundance', 'Abundance']], ui.weight, set('weight')),
    groups.length ? segmented('Rows', [['samples', 'Samples'], ['groups', 'Conditions']], ui.rows, set('rows')) : null,
  ].filter(Boolean);
  const range = ui.rows === 'groups' ? 'the range of its samples' : 'its 95% interval';
  const by = ui.weight === 'clonotypes' ? 'of the clonotypes (each counted once)' : 'of the abundance';
  return {
    result, units, controls, range, by, set,
    locusOf: (u) => u.loci.find((l) => l.locus === ui.locus),
    interval: (s) => s?.[`${ui.weight}Interval`] ?? null,
    value: (s) => (s ? s[ui.weight] : NaN),
  };
}

// The view's figure saver and the conditions' legend, for the cards being drawn.
let current = { save: null, items: [] };

// card builds a plot card; its plot is saved as a figure (the view's save), captioned with the
// card's notes, its legend the conditions' when the card shows it.
function card(title, controls, plot, ...notes) {
  const el = h('figure.plot-card', h('figcaption', h('span', title), h('span.plot-pickers', ...controls)),
    plot ? h('div.plot-holder', { html: plot.svg }) : null, ...notes);
  if (!plot || !current.save) return el;
  const note = notes.filter((n) => n?.tagName === 'P').map((n) => n.textContent).join(' ');
  const legend = notes.some((n) => n?.classList?.contains('plot-legend')) ? current.items : [];
  return current.save(el, { title, plot, note, legend });
}

export function renderGenes(ctx) {
  const { body, ui, colors } = ctx;
  current = { save: ctx.save, items: colors.items ?? [] };
  const c = setting(ctx);
  ui.segment = ['V', 'D', 'J', 'C'].includes(ui.segment) ? ui.segment : 'V';
  ui.level = ui.level === 'family' ? 'family' : 'gene';
  const sharesOf = (u, segment = ui.segment) => c.locusOf(u)?.genes.find((g) => g.segment === segment && g.level === ui.level)?.shares ?? null;
  const names = [...new Set(c.units.flatMap((u) => (sharesOf(u) ?? []).map((s) => s.name)).filter(Boolean))].sort(collator.compare);
  const byName = (u) => new Map((sharesOf(u) ?? []).map((s) => [s.name, s]));
  const controls = [
    segmented('Segment', [['V', 'V'], ['D', 'D'], ['J', 'J'], ['C', 'C']], ui.segment, c.set('segment')),
    segmented('Level', [['gene', 'Genes'], ['family', 'Families']], ui.level, c.set('level')),
    ...c.controls,
  ];
  const what = `${ui.level === 'gene' ? `${ui.segment} genes` : `${ui.segment} gene families`}, ${ui.locus}`;
  if (!names.length) {
    body.append(h('div.repertoire-controls', ...controls), card(what, [], null, h('p.muted', `No clonotype here has a ${ui.segment} call: the data hold none.`)));
    return;
  }
  // Genes as rows, the samples (or conditions) as columns: every gene's name stays legible.
  const values = c.units.map((u) => {
    const m = byName(u);
    return sharesOf(u) ? names.map((n) => (m.has(n) ? c.value(m.get(n)) : 0)) : names.map(() => NaN);
  });
  const plot = heatmap(names.map((_, j) => values.map((row) => row[j])), {
    label: what, rowLabels: names, colLabels: c.units.map((u) => u.name), format: pct, width: 520, labelWidth: 110, cellHeight: 13,
    title: (j, i, v) => {
      const iv = c.interval(byName(c.units[i]).get(names[j]));
      return `${c.units[i].name}, ${names[j]}: ${pct(v)}${iv ? ` (${pct(iv[0])}–${pct(iv[1])})` : ''}`;
    },
  });
  // What was not called.
  const missing = c.units.map((u) => c.value(byName(u).get(''))).filter(Number.isFinite);
  const notCalled = missing.length && Math.max(...missing) > 0.005
    ? h('p.muted.small', `Without a ${ui.segment} call: ${pct(Math.min(...missing))}${missing.length > 1 ? ` to ${pct(Math.max(...missing))}` : ''} ${c.by}.`) : null;
  const genesCard = card(what, [], plot, notCalled,
    h('p.muted.small', `Each ${ui.rows === 'groups' ? 'condition' : 'sample'}'s share ${c.by} using each ${ui.level === 'gene' ? 'gene' : 'family'}; a call between several genes gives each an equal part. Hold the pointer over a cell for its share and ${c.range}.`));
  body.append(h('div.repertoire-controls', ...controls));

  // One gene across the rows.
  const mean = (n) => values.reduce((acc, row) => acc + (Number.isFinite(row[names.indexOf(n)]) ? row[names.indexOf(n)] : 0), 0);
  if (!names.includes(ui.gene)) ui.gene = names.reduce((best, n) => (mean(n) > mean(best) ? n : best), names[0]);
  const rows = c.units.map((u) => {
    const s = byName(u).get(ui.gene);
    const iv = c.interval(s);
    return { label: u.name, value: s ? c.value(s) : sharesOf(u) ? 0 : NaN, lower: iv?.[0] ?? NaN, upper: iv?.[1] ?? NaN, className: colors.classOf(u.name) };
  });
  body.append(h('div.plot-grid', genesCard,
    card(`One across the ${ui.rows === 'groups' ? 'conditions' : 'samples'}`, [picker(ui.level === 'gene' ? 'Gene' : 'Family', names.map((n) => [n, n]), ui.gene, c.set('gene'))],
      intervals(rows, { label: `${ui.gene}'s share`, width: 520, labelWidth: 120, format: pct }), colors.legend(),
      h('p.muted.small', `${ui.gene}'s share ${c.by}, with ${c.range}.`))), pairsCard(ctx, c));
}

// The V–J pairs of one sample or condition, as a heatmap of V by J.
function pairsCard(ctx, c) {
  const { ui } = ctx;
  if (!c.units.some((u) => u.name === ui.pairsOf)) ui.pairsOf = c.units[0]?.name;
  const u = c.units.find((x) => x.name === ui.pairsOf);
  const shares = c.locusOf(u)?.genes.find((g) => g.segment === 'VJ' && g.level === ui.level)?.shares ?? [];
  const pairs = shares.map((s) => ({ v: s.name.split('+')[0], j: s.name.split('+')[1], value: c.value(s) })).filter((p) => p.v && p.j);
  const vs = [...new Set(pairs.map((p) => p.v))].sort(collator.compare);
  const js = [...new Set(pairs.map((p) => p.j))].sort(collator.compare);
  const values = vs.map((v) => js.map((j) => pairs.find((p) => p.v === v && p.j === j)?.value ?? 0));
  const plot = vs.length ? heatmap(values, { label: `V–J pairs of ${u.name}`, rowLabels: vs, colLabels: js, format: pct, width: 520, labelWidth: 110, cellHeight: 12 }) : null;
  return card('V–J pairs', [picker('Sample', c.units.map((x) => [x.name, x.name]), ui.pairsOf, c.set('pairsOf'))], plot,
    h('p.muted.small', `${u?.name ?? ''}'s share ${c.by} using each pair of a V and a J ${ui.level === 'gene' ? 'gene' : 'family'} (pairs with a segment not called left out).`));
}

export function renderJunctions(ctx) {
  const { body, ui, colors } = ctx;
  current = { save: ctx.save, items: colors.items ?? [] };
  const c = setting(ctx);
  const loci = c.units.map((u) => c.locusOf(u));
  // Spectratypes.
  const series = c.units.map((u, i) => ({
    label: u.name, className: colors.classOf(u.name),
    points: (loci[i]?.lengths ?? []).map((s) => ({ x: Number(s.name), y: c.value(s) })).sort((a, b) => a.x - b.x),
  }));
  const spectra = lines(series, { label: `Junction lengths, ${ui.locus}`, xLabel: 'Junction length (amino acids)', yLabel: ui.weight === 'clonotypes' ? 'Share of clonotypes' : 'Share of abundance', format: pct, xFormat: (v) => String(v) });
  const meanRows = c.units.map((u, i) => {
    const m = loci[i]?.meanLength;
    const iv = c.interval(m);
    return { label: u.name, value: m ? c.value(m) : NaN, lower: iv?.[0] ?? NaN, upper: iv?.[1] ?? NaN, className: colors.classOf(u.name) };
  });
  const finite = meanRows.flatMap((r) => [r.value, r.lower]).filter(Number.isFinite);
  const from = finite.length ? Math.max(0, Math.floor(Math.min(...finite)) - 1) : 0;
  body.append(h('div.repertoire-controls', ...c.controls));
  body.append(h('div.plot-grid',
    card(`Junction lengths, ${ui.locus}`, [], spectra, colors.legend(),
      h('p.muted.small', `The spectratype: each ${ui.rows === 'groups' ? 'condition' : 'sample'}'s share ${c.by} by junction length, from the conserved cysteine to the phenylalanine or tryptophan (IMGT positions 104 to 118).`)),
    card('Mean junction length', [], intervals(meanRows, { label: 'Mean junction length (amino acids)', width: 520, labelWidth: 120, from, format: (v) => v.toFixed(2) }), colors.legend(),
      h('p.muted.small', `Mean length in amino acids ${ui.weight === 'clonotypes' ? 'over the clonotypes' : 'weighted by abundance'}, with ${c.range}.`))));

  // Composition.
  const residues = [...new Set(loci.flatMap((l) => (l?.composition ?? []).map((s) => s.name)))];
  const composition = c.units.map((u, i) => {
    const m = new Map((loci[i]?.composition ?? []).map((s) => [s.name, s]));
    return residues.map((a) => (loci[i] ? c.value(m.get(a)) || 0 : NaN));
  });
  body.append(card(`Amino acids in the junctions, ${ui.locus}`, [], heatmap(residues.map((_, j) => composition.map((row) => row[j])), {
    label: `Amino acids in the junctions, ${ui.locus}`, rowLabels: residues, colLabels: c.units.map((u) => u.name), format: pct, width: 520, labelWidth: 40, cellHeight: 13,
    title: (j, i, v) => {
      const iv = c.interval((loci[i]?.composition ?? []).find((s) => s.name === residues[j]));
      return `${c.units[i].name}, ${residues[j]}: ${pct(v)}${iv ? ` (${pct(iv[0])}–${pct(iv[1])})` : ''}`;
    },
  }), h('p.muted.small', `Each amino acid's share of the junctions' residues ${ui.weight === 'clonotypes' ? 'over the clonotypes' : 'weighted by abundance'}, the conserved ends included.`)));

  // Amino acids by IMGT position, for one of the commonest lengths.
  const lengths = c.result.profiled?.[ui.locus] ?? [];
  if (!lengths.length) return;
  if (!lengths.includes(Number(ui.profileLength))) ui.profileLength = lengths[Math.floor(lengths.length / 2)];
  if (!c.units.some((u) => u.name === ui.profileOf)) ui.profileOf = c.units[0]?.name;
  const unit = c.units.find((u) => u.name === ui.profileOf);
  const profile = c.locusOf(unit)?.profiles?.find((p) => p.length === Number(ui.profileLength));
  const aa = 'ACDEFGHIKLMNPQRSTVWY'.split('');
  const plot = profile ? heatmap(aa.map((a) => profile.positions.map((p) => {
    const s = p.shares.find((x) => x.name === a);
    return p.shares.length ? (s ? c.value(s) : 0) : NaN;
  })), { label: `Amino acids by IMGT position, ${ui.profileLength}-residue junctions of ${unit.name}`, rowLabels: aa, colLabels: profile.positions.map((p) => p.imgt), format: pct, width: 520, labelWidth: 40, cellHeight: 12, max: 1 }) : null;
  body.append(card('Amino acids by position', [
    picker('Length', lengths.map((n) => [String(n), `${n} residues`]), String(ui.profileLength), c.set('profileLength')),
    picker('Sample', c.units.map((u) => [u.name, u.name]), ui.profileOf, c.set('profileOf')),
  ], plot, plot ? null : h('p.muted', `${unit?.name ?? ''} has no junction of ${ui.profileLength} residues.`),
  h('p.muted.small', `At each IMGT position (Lefranc et al. 2003) of the junctions of one length, each amino acid's share ${c.by}: the conserved cysteine at 104 and phenylalanine or tryptophan at 118, the germline-encoded ends, and the varied middle. The lengths shown are those of at least 5% of the clonotypes.`)));
}
