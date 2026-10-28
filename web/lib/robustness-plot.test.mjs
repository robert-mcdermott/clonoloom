import {test} from 'node:test';
import assert from 'node:assert/strict';
import {robustnessEffects} from './robustness-plot.js';
const result={options:{confidence:.95},summaries:[{feature:{id:'shannon',name:'Shannon entropy',unit:'nats'},direction:'Effect direction changes across definitions',uncertainty:'Marginal intervals differ in their relation to zero'}],variants:[{definition:{name:'AA genes'},analysis:{comparisons:[{mean:{estimate:2,lower:1,upper:3}}]}},{definition:{name:'NT genes'},analysis:{comparisons:[{mean:{estimate:-1,lower:-3,upper:1}}]}},{definition:{name:'AA alone'},failure:'Unavailable source evidence'}]};
test('definition forest preserves declared estimates and marginal intervals, including failures',()=>{
 const {svg,description}=robustnessEffects(result,'shannon');assert.equal((svg.match(/<circle /g)||[]).length,2);assert.equal((svg.match(/data-lower=/g)||[]).length,2);assert.match(svg,/data-estimate="-1"/);assert.match(svg,/Unavailable source evidence/);assert.match(svg,/Baseline: AA genes/);assert.match(description,/not independent replication/);assert.match(description,/not simultaneous or BY-adjusted/);assert.match(description,/descriptive, not a confidence interval/);assert.ok(svg.indexOf('AA genes')<svg.indexOf('NT genes'));
});
test('definition forest escapes source labels and does not draw absent intervals',()=>{
 const r=structuredClone(result);r.variants[0].definition.name='<script>';r.variants[0].analysis.comparisons[0].mean.lower=null;r.variants[0].analysis.comparisons[0].mean.upper=null;const p=robustnessEffects(r,'shannon');assert.doesNotMatch(p.svg,/<script>|NaN|undefined/);assert.equal((p.svg.match(/<circle /g)||[]).length,2);assert.equal((p.svg.match(/data-lower=/g)||[]).length,1);assert.equal(robustnessEffects(r,'absent').svg,'');
});
