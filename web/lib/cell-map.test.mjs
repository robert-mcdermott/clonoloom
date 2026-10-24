import test from 'node:test';
import assert from 'node:assert/strict';
import { cellMap } from './cell-map.js';
test('imported map escapes supplied labels and only lays out source coordinates', () => {
  const rows = [{ barcode: '<bad>', values: ['A&B'], position: [0, 0] }, { barcode: 'b', values: [''], position: [0, 0] }];
  const before = JSON.stringify(rows), result = cellMap(rows, 0);
  assert.equal(JSON.stringify(rows), before);
  assert.ok(result.svg.includes('&lt;bad&gt;') && result.svg.includes('A&amp;B'));
  assert.ok(result.svg.includes('cx="310.00" cy="150.00"'));
  assert.deepEqual(result.legend.map((x) => x.name), ['A&B', 'Missing annotation (no value)']);
  assert.ok(!/NaN|Infinity/.test(cellMap([]).svg));
  const many = cellMap(Array.from({ length: 7 }, (_, i) => ({ barcode: String(i), values: [String(i)], position: [i, -i] })), 0);
  assert.equal(many.legend.length, 1); assert.match(many.legend[0].name, /six/);
});

test('missing values remain gray above overlaps without changing source order or palette colors', () => {
  const rows = [{ barcode: 'missing', values: [''], position: [0, 0] }, { barcode: 'present', values: ['state A'], position: [0, 0] }];
  const result = cellMap(rows, 0);
  assert.ok(result.svg.indexOf('data-cell="1"') < result.svg.indexOf('data-cell="0"'));
  assert.match(result.svg, /class="cell-map-missing" data-cell="0"/);
  assert.match(result.svg, /class="plot-s1" data-cell="1"/);
  assert.equal(rows[0].barcode, 'missing');
  const named = cellMap([{ barcode: 'literal', values: ['Missing annotation'], position: [1, 1] }, ...rows], 0);
  assert.equal(named.legend.length, 3);
  assert.equal(named.legend[0].className, 'plot-s1');
  assert.equal(named.legend.at(-1).className, 'cell-map-missing');
  const many = cellMap([...rows, ...Array.from({ length: 7 }, (_, i) => ({ barcode: String(i), values: [String(i)], position: [i, i] }))], 0);
  assert.equal(many.legend.at(-1).className, 'cell-map-missing');
  assert.match(many.svg, /class="cell-map-missing" data-cell="0"/);
  assert.ok(!cellMap(rows).svg.includes('class="cell-map-missing"'));
});

test('embedding axes use selected provenance and escape source names', () => {
  const plot = cellMap([], -1, { axes: ['UMAP 1', 'UMAP 2'], path: '/obsm/X_umap', supplied: true });
  assert.match(plot.svg, /UMAP 1 \(supplied\)/); assert.match(plot.svg, /UMAP 2 \(supplied\)/);
  assert.match(cellMap([], -1, { axes: ['<script>', 'Y&Z'] }).svg, /&lt;script&gt;/);
  assert.match(cellMap([]).svg, /map_x/);
});


test('computed map identifies the method on both axes without claiming supplied coordinates', () => {
  const result = cellMap([{ barcode: 'A', values: [''], position: [1, 2] }], 0, { axes: ['UMAP 1', 'UMAP 2'], supplied: false });
  assert.match(result.svg, /UMAP 1 \(computed\)/);
  assert.match(result.svg, /UMAP 2 \(computed\)/);
  assert.match(result.svg, /Computed expression map/);
  assert.match(result.description, /Neighborhoods are defined in PCA space/);
  assert.doesNotMatch(result.svg, /Coordinates are not computed/);
  assert.equal(result.legend[0].className, 'cell-map-missing');
});

test('overlay styling uses saved sizes, dims unselected cells and preserves coordinates', () => {
  const rows = [{ barcode: 'a', values: [], position: [0, 0], size: 1 }, { barcode: 'b', values: [], position: [2, 4], size: 5 }, { barcode: 'c', values: [], position: [0, 0], size: null }];
  const before = JSON.stringify(rows);
  const style = { label: (c) => c.size == null ? 'QC excluded' : String(c.size), missing: (c) => c.size == null, continuous: true, min: 1, max: 5, value: (c) => c.size, selected: new Set(['a']) };
  const result = cellMap(rows, -1, null, style);
  assert.match(result.svg, /hsl\(205 65% 75%\)/); assert.match(result.svg, /hsl\(205 65% 30%\)/);
  assert.equal((result.svg.match(/opacity="0.14"/g) ?? []).length, 2);
  assert.match(result.svg, /class="cell-map-missing" data-cell="2"/);
  assert.equal(JSON.stringify(rows), before);
  assert.match(result.legend[0].name, /1 to 5/);
  const equal = cellMap([rows[0]], -1, null, { ...style, max: 1 });
  assert.doesNotMatch(equal.svg, /NaN|Infinity/);
});
