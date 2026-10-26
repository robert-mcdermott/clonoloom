import test from 'node:test';
import assert from 'node:assert/strict';
import {pseudobulkPlot} from './pseudobulk-plot.js';
const input={bins:['1','>1','2'],options:{annotation:'type',minCells:10},samples:[{annotation:{name:'<sample>',subject:'subject',timepoint:'visit'}}]};
test('pseudobulk coverage has exact linear widths, separate missingness and threshold',()=>{
 const result={groups:[{sample:0,type:'A',bin:'1',cells:5,eligible:false,reason:'Below minimum cells'},{sample:0,type:'A',bin:'>1',cells:20,eligible:true},{sample:0,type:'A',bin:'2',cells:0,eligible:false,reason:'No eligible cells'}]};
 const p=pseudobulkPlot(input,result,'A');assert.match(p.svg,/width="24" height="18"/);assert.match(p.svg,/width="96" height="18"/);assert.equal((p.svg.match(/<rect/g)||[]).length,2);assert.match(p.svg,/Absent/);assert.match(p.svg,/&lt;sample&gt;/);assert.doesNotMatch(p.svg,/<sample>/);assert.match(p.svg,/M203 35V/);assert.match(p.description,/not expression differences/);
});
test('pseudobulk coverage never invents an absent type',()=>{assert.equal(pseudobulkPlot(input,{groups:[]},'missing').svg,'');});
