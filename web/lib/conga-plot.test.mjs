import test from 'node:test';
import assert from 'node:assert/strict';
import { congaPlot } from './conga-plot.js';
test('overlap plot preserves supplied counts and zero baseline',()=>{
  const s=congaPlot({id:'<clone>'},[{k:2,overlap:3,expected:.5,logP:-2},{k:4,overlap:0,expected:2,logP:0}]);
  assert.match(s,/&lt;clone&gt;/);assert.match(s,/data-observed="3"/);assert.match(s,/data-expected="0.5"/);assert.match(s,/height="0"/);assert.match(s,/starts at zero/);assert.doesNotMatch(s,/NaN|Infinity/);
});
test('untested sizes have no numerical marks; degenerate tested zero remains valid',()=>{
  assert.doesNotMatch(congaPlot({id:'a'},[{k:5,logP:null}]),/<svg/);
  assert.doesNotMatch(congaPlot({id:'a'},[{k:1,overlap:0,expected:0,logP:0}]),/NaN|Infinity/);
});
