import { test } from 'node:test';
import assert from 'node:assert/strict';
import { cohortPoints, cohortEffect } from './cohort-plot.js';
const c = {feature:{name:'Shannon entropy',unit:'nats'},nReference:2,nComparison:2,pairs:2,missing:0,points:[
  {subject:'one',sample:'a',group:'pre',value:1},{subject:'one',sample:'b',group:'post',value:3},
  {subject:'two',sample:'c',group:'pre',value:2},{subject:'two',sample:'d',group:'post',value:4},
],mean:{name:'Mean difference',unit:'nats',estimate:2,lower:-1,upper:5,interval:'Two-sided t interval; marginal, not multiplicity-adjusted'}};
const result={layout:'paired',levels:['pre','post'],options:{confidence:.95}};
test('subject plot preserves points and pairs, with explicit units and n',()=>{
  const p=cohortPoints(c,result);assert.equal((p.svg.match(/<circle /g)||[]).length,4);assert.equal((p.svg.match(/data-subject=/g)||[]).length,2);assert.match(p.svg,/n=2 selected observations/);assert.match(p.description,/nats/);
  assert.doesNotMatch(cohortPoints(c,{...result,layout:'independent'}).svg,/data-subject=/);
  const missing={...c,missing:1,points:c.points.map((p,i)=>i===1?{...p,value:null,missing:'absent'}:p)};
  const q=cohortPoints(missing,result);assert.equal((q.svg.match(/<circle /g)||[]).length,3);assert.equal((q.svg.match(/data-subject=/g)||[]).length,1);
});
test('effect plot carries the Go interval and zero reference, no subject error bars',()=>{
  const p=cohortEffect(c,result);assert.match(p.svg,/data-lower="-1" data-upper="5"/);assert.match(p.svg,/data-estimate="2"/);assert.match(p.svg,/95% t interval/);assert.match(p.description,/not multiplicity-adjusted/);
  assert.equal(cohortEffect({...c,mean:{...c.mean,lower:null,upper:null,reason:'Zero standard error'}},result).svg,'');
});
test('plots escape labels and refuse silent subsampling',()=>{
  assert.doesNotMatch(cohortPoints({...c,feature:{name:'<script>',unit:'<bad>'}},result).svg,/<script>|<bad>/);
  const p=cohortPoints({...c,points:Array.from({length:501},()=>c.points[0])},result);assert.equal(p.svg,'');assert.match(p.description,/no subset/);
});
