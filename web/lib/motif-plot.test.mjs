import {test} from 'node:test';
import assert from 'node:assert/strict';
import {motifProfile} from './motif-plot.js';
test('motif profile displays exact Go counts on a fixed frequency scale',()=>{const p={n:4,alphabet:'AC',counts:[[3,1],[0,4]],frequencies:[[.75,.25],[0,1]]};const g={length:2};const out=motifProfile(p,g);assert.equal((out.svg.match(/<rect /g)||[]).length,4);assert.match(out.svg,/data-position="1" data-residue="A" data-count="3"/);assert.match(out.svg,/opacity="0.75"/);assert.match(out.description,/not information content/);assert.match(out.description,/not independent evidence/);assert.doesNotMatch(out.svg,/NaN|undefined/);assert.equal(motifProfile({...p,n:0},g).svg,'');});
