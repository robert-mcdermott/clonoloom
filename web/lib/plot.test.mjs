import assert from 'node:assert/strict';
import test from 'node:test';
import { columns, intervals, lines, stackedBars, ticks } from './plot.js';

test('ticks are nice numbers covering the range', () => {
  assert.deepEqual(ticks(1), [0, 0.25, 0.5, 0.75, 1]);
  assert.deepEqual(ticks(870), [0, 250, 500, 750]);
  assert.deepEqual(ticks(0), [0]);
  const t = ticks(9131);
  assert.ok(t[0] === 0 && t.at(-1) <= 9131 && t.length >= 3);
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
