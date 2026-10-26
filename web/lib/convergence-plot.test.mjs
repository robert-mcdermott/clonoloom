import { test } from 'node:test';
import assert from 'node:assert/strict';
import { hlaFrequencies, recurrencePlot } from './convergence-plot.js';
test('HLA plots use exact subject denominators and marginal intervals on a fixed scale', () => {
  const r = { n:28,excluded:4,locus:'A',carrier:{matches:17,total:20,estimate:.85,lower:.64,upper:.95},noncarrier:{matches:1,total:8,estimate:.125,lower:.02,upper:.47} };
  const out=hlaFrequencies(r); assert.match(out.svg,/17\/20 subjects/); assert.match(out.svg,/1\/8 subjects/); assert.match(out.svg,/data-lower="0.64"/);assert.match(out.svg,/100%/);assert.match(out.description,/not adjusted by BY/); assert.doesNotMatch(out.svg,/NaN|undefined/);
  const missing=hlaFrequencies({...r,carrier:{matches:0,total:0,estimate:null,lower:null,upper:null}});assert.match(missing.svg,/Unavailable: no subjects/);assert.equal((missing.svg.match(/<circle /g)||[]).length,1);
});
test('recurrence plot includes every declared group or draws no subset',()=>{
  const r={observedSubjects:8,recurrence:[{label:'group <one>',subjects:['a','b']},{label:'empty',subjects:[]}]};
  const out=recurrencePlot(r);assert.equal((out.svg.match(/<circle /g)||[]).length,2);assert.match(out.svg,/group &lt;one&gt;/);assert.match(out.description,/not a test/);
  assert.equal(recurrencePlot({...r,observedSubjects:0}).svg,'');assert.equal(recurrencePlot({...r,recurrence:Array(21).fill(r.recurrence[0])}).svg,'');
});
