import test from 'node:test';
import assert from 'node:assert/strict';
import { annotationLabel, lineagePlot } from './lineage.js';

test('lineage drawing retains zero edges, observed mixtures and escaped interactive labels', () => {
 const tree = { nodes: [
  {id:'root',kind:'reference',changes:0,depth:0,rows:0,annotations:[]},
  {id:'inner',parent:'root',kind:'inferred',depth:1,changes:0,rows:0,annotations:[]},
  {id:'a"<',parent:'inner',kind:'observed',depth:2,changes:1,rows:2,annotations:[{isotype:'IGHM'},{isotype:'IGHG1'}]},
  {id:'b',parent:'inner',kind:'observed',depth:2,changes:0,rows:1,annotations:[{isotype:'<script>'}]},
 ]};
 assert.equal(annotationLabel(tree.nodes[2],'isotype'),'Mixed');
 assert.equal(annotationLabel(tree.nodes[3],'timepoint'),'Missing');
 const plot=lineagePlot(tree);
 assert.match(plot.svg,/&lt;script&gt;/);assert.doesNotMatch(plot.svg,/<script>/);
 assert.match(plot.svg,/data-node="a&quot;&lt;"/);
 assert.equal((plot.svg.match(/role="button"/g)||[]).length,4);
 assert.equal((plot.svg.match(/class="plot-line plot-s1"/g)||[]).length,3);
 assert.match(plot.svg,/>0<\/text>/);assert.match(plot.description,/not elapsed time/);
 assert.doesNotMatch(plot.svg,/NaN|undefined/);
});
test('single reference state has a bounded standalone drawing',()=>{
 const p=lineagePlot({nodes:[{id:'r',kind:'reference',changes:0,depth:0,rows:1,annotations:[{tissue:'blood'}]}]},'tissue');
 assert.match(p.svg,/blood/);assert.doesNotMatch(p.svg,/NaN|undefined/);
});
test('likelihood drawing labels fitted units and preserves zero edges without a time axis',()=>{
 const plot=lineagePlot({model:'JC69',nodes:[{id:'r',kind:'reference',depth:0,branchLength:0,rows:0,annotations:[]},{id:'i',parent:'r',kind:'inferred',depth:1,branchLength:0,rows:0,annotations:[]},{id:'a',parent:'i',kind:'observed',depth:2,branchLength:.0123456789,rows:3,annotations:[{isotype:'IGHG1'}]}]});
 assert.match(plot.description,/substitutions\/site/);assert.match(plot.description,/not genetic distance or elapsed time/);
 assert.doesNotMatch(plot.svg,/parsimony|edge labels: changes|NaN|undefined/);assert.match(plot.svg,/>0\.00<\/text>/);assert.match(plot.svg,/>0\.0123<\/text>/);
 assert.equal((plot.svg.match(/role="button"/g)||[]).length,3);
});
