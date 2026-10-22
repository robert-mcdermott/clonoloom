import assert from 'node:assert/strict';
import test from 'node:test';
import { alluvial, changes, curves, columns, dispersion, figure, figureStyle, heatmap, intervals, lines, percent, scale, stackedBars, threads, ticks, timeline } from './plot.js';

test('ticks are nice numbers covering the range', () => {
  assert.deepEqual(ticks(1), [0, 0.25, 0.5, 0.75, 1]);
  assert.deepEqual(ticks(870), [0, 250, 500, 750]);
  assert.deepEqual(ticks(0), [0]);
  const t = ticks(9131);
  assert.ok(t[0] === 0 && t.at(-1) <= 9131 && t.length >= 3);
});

test('an axis holds its largest value', () => {
  assert.deepEqual(scale(870), [0, 250, 500, 750, 1000]);
  assert.deepEqual(scale(0.22), [0, 0.1, 0.2, 0.3]);
  assert.deepEqual(scale(1), [0, 0.25, 0.5, 0.75, 1]);
  assert.deepEqual(scale(0), [0]);
  // A column, a line and an interval at the top stay inside the plot.
  const col = columns([{ x: 1, y: 870 }], {});
  assert.match(col.svg, />1,000</);
  const iv = intervals([{ label: 'a', value: 1.55, lower: 1.5, upper: 1.6 }], { from: 0 });
  assert.match(iv.svg, />2</);
});

test('stacked bars draw each part and say what they show', () => {
  const { svg, description } = stackedBars([
    { label: 'P00207', parts: { productive: 0.6, outOfFrame: 0.38, stop: 0.02 } },
    { label: 'P00441', parts: { productive: 0.86, outOfFrame: 0.13, stop: 0.01 } },
  ], [{ key: 'productive', label: 'productive', className: 'plot-ok' }, { key: 'outOfFrame', label: 'out of frame', className: 'plot-warn' }, { key: 'stop', label: 'stop codon', className: 'plot-danger' }], { label: 'Status', share: true });
  assert.equal((svg.match(/<rect /g) ?? []).length, 6);
  assert.match(svg, /role="img"/);
  assert.match(description, /P00207: productive 60\.0%, out of frame 38\.0%, stop codon 2\.0%/);
  assert.match(svg, /100%/);
});

test('columns label their bins and escape text', () => {
  const { svg, description } = columns([{ x: 12, y: 5 }, { x: 13, y: 40 }, { x: '<14>', y: 3 }], { label: 'Junction lengths', xLabel: 'Length' });
  assert.equal((svg.match(/<rect /g) ?? []).length, 3);
  assert.match(svg, /&lt;14&gt;/);
  assert.match(description, /highest at 13 \(40\)/);
});

test('intervals draw each estimate with its whiskers, and say them', () => {
  const { svg, description } = intervals([
    { label: 'S01-1', value: 9742, lower: 9500, upper: 10000 },
    { label: 'S04-1', value: 1310, lower: 1200, upper: 1450 },
    { label: 'none', value: NaN },
  ], { label: 'Shannon diversity' });
  assert.equal((svg.match(/<circle /g) ?? []).length, 2);
  assert.equal((svg.match(/class="plot-whisker"/g) ?? []).length, 6);
  assert.match(description, /S01-1 9,742 \(9,500 to 10,000\); S04-1 1,310 \(1,200 to 1,450\); none none/);
});

test('lines on a logarithmic axis leave out what is not positive', () => {
  const { svg, description } = lines([
    { label: 'S01-1', points: [{ x: 0, y: 59857 }, { x: 1, y: 9742 }, { x: 2, y: 306 }] },
    { label: 'S04-1', points: [{ x: 0, y: 29967 }, { x: 1, y: 0 }, { x: 2, y: 59 }] },
  ], { label: 'Hill profile', log: true });
  assert.equal((svg.match(/<polyline /g) ?? []).length, 2);
  assert.equal((svg.match(/,/g) ?? []).length >= 5, true);
  assert.match(description, /2 lines \(S01-1, S04-1\)/);
  assert.match(svg, />100,000</);
});

test('curves draw interpolation solid, extrapolation dashed, the sample as a dot, bands and guides', () => {
  const points = [
    { x: 1, y: 1, lower: 1, upper: 1 },
    { x: 50, y: 30, lower: 25, upper: 35 },
    { x: 100, y: 45, lower: 40, upper: 50 },
    { x: 150, y: 52, lower: 44, upper: 60, extrapolated: true },
    { x: 200, y: 56, lower: 45, upper: 67, extrapolated: true },
    { x: NaN, y: 3 },
  ];
  const { svg, description } = curves([{ label: 'S<1>', className: 'plot-s2', points, observed: { x: 100, y: 45 } }],
    { label: 'Clonotypes by size', xLabel: 'Templates', guides: [{ x: 120, label: 'compared' }] });
  assert.equal((svg.match(/<polyline/g) ?? []).length, 2);
  assert.match(svg, /class="plot-line plot-dashed plot-s2"/);
  assert.match(svg, /<polygon class="plot-band plot-s2"/);
  assert.match(svg, /<circle class="plot-s2"/);
  assert.match(svg, /class="plot-guide"/);
  assert.match(svg, /S&lt;1&gt;/);
  assert.match(description, /1 curve, solid where interpolated and dashed where extrapolated, ending at S<1> 56 at 200/);
  // The dashed line starts where the solid one ends.
  const dashed = svg.match(/plot-dashed plot-s2" points="([^"]+)"/)[1].split(' ');
  const solid = svg.match(/class="plot-line plot-s2" points="([^"]+)"/)[1].split(' ');
  assert.equal(dashed[0], solid.at(-1));
  // Without bands, none; on a logarithmic axis, decades.
  const log = curves([{ label: 'a', points }], { logX: true, bands: false });
  assert.doesNotMatch(log.svg, /plot-band/);
  assert.match(log.svg, />1,000</);
});

test('a heatmap shades each cell by its value, leaves the missing blank and says the largest', () => {
  const { svg, description } = heatmap([[0.1, 0.4, NaN], [0, 0.2, 0.05]], { label: 'V genes', rowLabels: ['S1', 'S<2>'], colLabels: ['TRBV2', 'TRBV5-1', 'TRBV10-3'] });
  // Five cells and the ten steps of the scale.
  assert.equal((svg.match(/<rect class="plot-heat"/g) ?? []).length, 15);
  assert.match(svg, /fill-opacity="1.000"><title>S1, TRBV5-1: 0.4<\/title>/);
  assert.match(svg, /S&lt;2&gt;/);
  assert.match(svg, /rotate\(-60\)/);
  assert.equal(description, 'V genes: 2 rows by 3 columns; the largest 0.4, S1 and TRBV5-1.');
});

test('a figure stands alone: the plot on white with its title, legend, caption and colors written in', () => {
  const plot = intervals([{ label: 'S1', value: 2, lower: 1, upper: 3, className: 'plot-s2' }], { label: 'Shannon diversity' });
  const { svg, width, height } = figure(plot.svg, {
    title: 'Shannon diversity of each sample', legend: [{ label: 'control', className: 'plot-s1' }, { label: 'expanded <x>', className: 'plot-s2' }],
    caption: ['Clonotypes: amino-acid junction, V and J genes; abundance in templates; 12 samples.', 'ClonoLoom 0.2.0.'],
  });
  assert.match(svg, /^<svg xmlns="http:\/\/www.w3.org\/2000\/svg" width="\d+" height="\d+"/);
  assert.match(svg, /<style>[^<]*\.plot-whisker\{stroke:#4b5468/);
  assert.match(svg, /circle\.plot-s2,rect\.plot-s2,path\.plot-ribbon\.plot-s2\{fill:#e9a23b\}/);
  assert.match(svg, /<text class="figure-title"[^>]*>Shannon diversity of each sample<\/text>/);
  assert.match(svg, /expanded &lt;x&gt;/);
  assert.match(svg, /<text class="figure-caption"[^>]*>ClonoLoom 0.2.0.<\/text>/);
  assert.ok(!/var\(--/.test(svg), 'no theme variables');
  assert.ok(width >= 360 && height > 100);
  // With color vision, the status colors change.
  assert.match(figureStyle({ colorVision: true }), /\.plot-warn\{fill:#9a6200\}/);
});

test('a timeline places each subject\'s samples by day, references and baselines in columns, replicates stacked', () => {
  const { svg, description } = timeline([
    { label: 'P1', marks: [{ column: 'reference', label: 'infusion product', reference: true }, { column: 'baseline', label: 'baseline' }, { x: 7, label: 'day 7' }, { x: 28, label: 'day 28' }, { x: 28, label: 'week 4' }] },
    { label: 'P2 <x>', className: 'plot-s2', marks: [{ x: -7, label: 'day -7' }, { x: 90, label: 'month 3' }] },
  ], { label: 'Subjects', xLabel: 'Days from infusion' });
  assert.match(svg, /^<svg class="plot"/);
  assert.match(svg, />reference<\/text>/);
  assert.match(svg, />baseline<\/text>/);
  assert.equal((svg.match(/<path class="plot-reference"/g) ?? []).length, 1);
  assert.equal((svg.match(/<circle /g) ?? []).length, 6);
  // Day 0 marked when the axis reaches before it; the two day-28 samples one above the other.
  assert.match(svg, /class="plot-guide"/);
  const ys = [...svg.matchAll(/<circle class="plot-accent" cx="([\d.]+)" cy="([\d.]+)"/g)].map((m) => [Number(m[1]), Number(m[2])]);
  const at28 = ys.filter(([cx]) => Math.abs(cx - ys.at(-1)[0]) < 0.01);
  assert.equal(at28.length, 2);
  assert.ok(Math.abs(at28[0][1] - at28[1][1]) >= 6);
  assert.match(svg, /P2 &lt;x&gt;/);
  assert.match(svg, />Days from infusion</);
  assert.match(description, /2 subjects; P1: infusion product, baseline, day 7, day 28, week 4; P2 <x>: day -7, month 3\./);
});

const POINTS = [{ label: 'infusion product' }, { label: 'day -7' }, { label: 'day 14', title: 'day 14 (2 samples)' }];

test('percentages to three digits', () => {
  assert.equal(percent(0.1234), '12.3%');
  assert.equal(percent(1e-5), '0.001%');
  assert.equal(percent(NaN), '—');
});

test('threads draw each clonotype through the time points, one not found on its own lane, the chosen with its intervals', () => {
  const series = [
    { key: 'c-1', label: 'CASSA', className: 'plot-s1', values: [{ y: 0.05, lower: 0.04, upper: 0.06 }, { y: 0, absent: true, upper: 0.0004 }, { y: 0.01, lower: 0.008, upper: 0.012 }] },
    { key: 'c-<2>', label: 'CASSB <b>', className: 'plot-s2', values: [{ y: 0.002, lower: 0.001, upper: 0.003 }, { y: 0.001 }, { y: NaN }] },
  ];
  const { svg, description } = threads(POINTS, series, { label: 'Threads', selected: 'c-1' });
  assert.match(svg, /^<svg class="plot"/);
  assert.match(svg, />not found<\/text>/);
  // One open circle, its segments dashed; the missing value draws nothing.
  assert.equal((svg.match(/class="plot-absent /g) ?? []).length, 1);
  assert.equal((svg.match(/plot-thread plot-s1 plot-chosen plot-dashed/g) ?? []).length, 2);
  assert.equal((svg.match(/plot-thread plot-s2 plot-faded"/g) ?? []).length, 1);
  // The chosen drawn last, with a whisker at each point (dashed where not found); the others none.
  assert.ok(svg.indexOf('data-key="c-1"') > svg.indexOf('data-key="c-&lt;2&gt;"'));
  assert.equal((svg.match(/class="plot-whisker/g) ?? []).length, 3);
  assert.match(svg, /CASSA at infusion product: 5%/);
  assert.match(svg, /CASSA at day -7: not found \(up to 0\.04%\)/);
  assert.match(svg, />10%<\/text>/);
  assert.match(description, /2 clonotypes through 3 time points \(infusion product, day -7, day 14\); CASSA highest at infusion product \(5%\); CASSB <b> highest at infusion product \(0\.2%\)\./);
});

test('an alluvial plot stacks the clonotypes in one order at every point and joins them by ribbons', () => {
  const series = [
    { key: 'c-1', label: 'CASSA', className: 'plot-s1', values: [0.5, 0, 0.2] },
    { key: 'c-2', label: 'CASSB', className: 'plot-s2', values: [0.3, 0.1, 0.1] },
  ];
  const { svg, description } = alluvial(POINTS, series, { label: 'Alluvial', selected: 'c-2' });
  // Bars where present (CASSA not at day -7), ribbons wherever either end is.
  assert.equal((svg.match(/<rect /g) ?? []).length, 5);
  assert.equal((svg.match(/class="plot-ribbon /g) ?? []).length, 4);
  assert.equal((svg.match(/plot-faded/g) ?? []).length, 2 + 2);
  // At the product, CASSB sits on CASSA: its bar's bottom is CASSA's top.
  const bars = [...svg.matchAll(/<rect class="(plot-s\d)[^"]*" x="([\d.]+)" y="([\d.]+)" width="[\d.]+" height="([\d.]+)"/g)].map((m) => ({ cls: m[1], x: Number(m[2]), y: Number(m[3]), h: Number(m[4]) }));
  const [a, b] = bars.filter((r) => r.x === bars[0].x);
  assert.equal(a.cls, 'plot-s1');
  assert.ok(Math.abs(b.y + b.h - a.y) < 0.11);
  assert.match(svg, />80%<\/text>/);
  assert.match(description, /2 clonotypes stacked at 3 time points, holding 80% at infusion product, 10% at day -7, 30% at day 14\./);
});

test('figures style threads, ribbons and clonotypes not found', () => {
  const css = figureStyle();
  assert.match(css, /path\.plot-ribbon\.plot-s2\{fill:#e9a23b\}/);
  assert.match(css, /circle\.plot-absent\.plot-s1\{fill:#ffffff;stroke:#5b4ce6\}/);
  assert.match(css, /\.plot-faded\{opacity:0\.16\}/);
});

test('changes place each clonotype at its two frequencies, one not found on its lane, the chosen ringed', () => {
  const points = [
    { key: 'c-1', label: 'CASSA', x: 0.001, y: 0.01, className: 'plot-s1', kind: 'increased' },
    { key: 'c-2', label: 'CASSB', x: 0.002, y: 0, className: 'plot-s4', kind: 'decreased' },
    { key: 'c-3', label: 'CASSC', x: 0.0005, y: 0.0004, className: 'plot-dim' },
  ];
  const { svg, description } = changes(points, { label: 'Changes', xLabel: 'day -7', yLabel: 'day 14', selected: 'c-1' });
  assert.equal((svg.match(/<circle class="plot-(s1|s4|dim)"/g) ?? []).length, 3);
  assert.equal((svg.match(/>not found</g) ?? []).length, 2);
  assert.match(svg, /class="plot-guide"/);
  assert.match(svg, /<circle class="plot-ring"/);
  assert.match(svg, /CASSB: 0\.2% at the first, not found at the second/);
  // The one not found at the second sits on the lane below the axis.
  const cy = (key) => Number(new RegExp(`cy="([\\d.]+)" r="[\\d.]+" data-key="${key}"`).exec(svg)[1]);
  assert.ok(cy('c-2') > cy('c-3') && cy('c-3') > cy('c-1'));
  assert.match(description, /3 clonotypes, 1 increased, 1 decreased; day -7 against day 14/);
});

test('the variance between replicates: each clonotype as a z-score of sampling, sampling\'s band and the model\'s', () => {
  const points = [{ n: 5, z: 0.4 }, { n: 50, z: -2.5 }, { n: 800, z: 3.1 }, { n: 800, z: 9 }];
  // A scale of 20 at every n: the band at n = 800 is 1.96 √(820/21), about 12.2.
  const { svg, description } = dispersion(points, () => 20, { label: 'Replicates' });
  assert.equal((svg.match(/<circle class="plot-dim"/g) ?? []).length, 4);
  assert.equal((svg.match(/<line class="plot-guide"/g) ?? []).length, 2);
  assert.match(svg, /<polygon class="plot-band plot-s1"/);
  assert.match(description, /4 clonotypes; 3 outside sampling's 95% band, 0 outside the model's\./);
});
