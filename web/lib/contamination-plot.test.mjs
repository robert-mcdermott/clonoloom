import {test} from 'node:test';
import assert from 'node:assert/strict';
import {sharingPlot} from './contamination-plot.js';
test('sharing geometry uses positive fractions, equal log axes and every match',()=>{
 const r={evidence:{samples:[{sample:{name:'A <x>',subject:'one'},total:100},{sample:{name:'B',subject:'two'},total:1000}]},matches:[{a:0,b:1,receptor:'nt-one',countA:50,countB:2,fractionA:.5,fractionB:.002,review:true,pattern:'Asymmetric',pgenClass:'Unknown Pgen'},{a:0,b:1,receptor:'nt-two',countA:1,countB:10,fractionA:.01,fractionB:.01,review:false,pattern:'Below',pgenClass:'High Pgen: plausible publicness'}]};
 const p={a:0,b:1,reason:''};const out=sharingPlot(r,p);assert.equal((out.svg.match(/<circle /g)||[]).length,2);assert.match(out.svg,/data-fraction-a="0.5"/);assert.match(out.svg,/A &lt;x&gt;/);assert.match(out.svg,/style="fill:none;stroke-width:2"/);assert.match(out.description,/no pseudocount/);assert.equal(sharingPlot(r,{...p,reason:'No depth'}).svg,'');assert.equal(sharingPlot({...r,matches:Array(2001).fill(r.matches[0])},p).svg,'');
});
