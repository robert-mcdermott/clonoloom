import test from 'node:test';
import assert from 'node:assert/strict';
import { cellMap } from './cell-map.js';
test('imported map escapes supplied labels and only lays out source coordinates', () => {
  const rows = [{ barcode: '<bad>', values: ['A&B'], position: [0, 0] }, { barcode: 'b', values: [''], position: [0, 0] }];
  const before = JSON.stringify(rows), result = cellMap(rows, 0);
  assert.equal(JSON.stringify(rows), before);
  assert.ok(result.svg.includes('&lt;bad&gt;') && result.svg.includes('A&amp;B'));
  assert.ok(result.svg.includes('cx="310.00" cy="150.00"'));
  assert.deepEqual(result.legend.map((x) => x.name), ['A&B', 'Missing annotation']);
  assert.ok(!/NaN|Infinity/.test(cellMap([]).svg));
  const many = cellMap(Array.from({ length: 7 }, (_, i) => ({ barcode: String(i), values: [String(i)], position: [i, -i] })), 0);
  assert.equal(many.legend.length, 1); assert.match(many.legend[0].name, /six/);
});
