import {test} from 'node:test';
import assert from 'node:assert/strict';
import {countMA,countObservation,countEffect} from './count-plot.js';
const row={id:'clone',meanCPM:10,log2Fold:1,lower:-.5,upper:2.5,cpm:[10,30,15,20]};
const result={rows:[row,{id:'failed',log2Fold:null,meanCPM:2}],family:2,tested:1,options:{confidence:.95},design:{rows:[{subject:'one',name:'a',group:'pre'},{subject:'one',name:'b',group:'post'},{subject:'two',name:'c',group:'pre'},{subject:'two',name:'d',group:'post'}]}};
const design={input:{spec:{layout:'paired',levels:['pre','post']}},groups:[{subjects:2},{subjects:2}],completePairs:2};
test('count MA preserves fitted effects and reports omitted failed fits',()=>{const p=countMA(result,'clone');assert.equal((p.svg.match(/data-clone=/g)||[]).length,1);assert.match(p.description,/1 eligible but unavailable/);assert.match(p.svg,/log scale/);assert.match(p.svg,/M70 30V260H570" class="plot-axis" fill="none"/);assert.doesNotMatch(countMA({...result,rows:[{...row,id:'<script>'}]}).svg,/<script>/);});
test('count observations preserve subject pairing and measured zeros',()=>{const p=countObservation({...row,cpm:[0,30,15,20]},result,design);assert.equal((p.svg.match(/data-subject=/g)||[]).length,2);assert.equal((p.svg.match(/data-sample=/g)||[]).length,4);assert.match(p.description,/counts per million/);});
test('count intervals are labeled Wald log ratios, not t or mean differences',()=>{const p=countEffect(row,result);assert.match(p.svg,/Wald interval/);assert.doesNotMatch(p.svg,/t interval|Mean difference|minus reference/);assert.match(p.description,/conditional on estimated dispersion/);assert.match(p.svg,/data-lower="-0.5" data-upper="2.5"/);});
