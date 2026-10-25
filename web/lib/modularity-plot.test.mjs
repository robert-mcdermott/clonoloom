import { test } from 'node:test';
import assert from 'node:assert/strict';
import { modularityPlot } from './modularity-plot.js';
test('Modularity figure preserves discrete frequencies, zero baseline, observed and expected markers', () => {
  const svg = modularityPlot({ id:'<clone>', cells:3, observed:7 }, { method:'Exhaustive', draws:20, expected:1.5, histogram:[{edges:0,count:15},{edges:2,count:5}] });
  assert.match(svg,/&lt;clone&gt;/); assert.match(svg,/0 edges: 15 subsets/); assert.match(svg,/2 edges: 5 subsets/);
  assert.match(svg,/data-observed="7"/); assert.match(svg,/data-expected="1.5"/); assert.match(svg,/not an uncertainty interval/);
  assert.match(svg,/frequency/); assert.match(svg,/starting at zero/); assert.doesNotMatch(svg,/NaN|Infinity/);
});
test('Degenerate null retains a finite plot and unavailable test produces no plot', () => {
  assert.equal(modularityPlot(null,null),'');
  const svg=modularityPlot({id:'A',cells:2,observed:0},{method:'Exhaustive',draws:6,expected:0,histogram:[{edges:0,count:6}]});
  assert.doesNotMatch(svg,/NaN|Infinity/); assert.match(svg,/0 edges: 6 subsets/);
});
