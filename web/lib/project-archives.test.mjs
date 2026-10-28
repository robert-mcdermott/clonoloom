import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, readdirSync } from 'node:fs';
import { parseExactProject, archiveCounts } from './project-archives.js';
import { createProject } from './project.js';
import { recoverCheckpoint } from './checkpoints.js';

test('archive loading refuses numeric changes, preserving original evidence outside the browser',()=>{
 const base=JSON.stringify(createProject('Artificial study',{id:'p-test'}));
 for(const token of ['9007199254740993','0.123456789012345678901','-0','1.0','1e+02']) {
  assert.throws(()=>parseExactProject(base.slice(0,-1)+`,"value":${token}}`),/numeric tokens/);
 }
 assert.equal(parseExactProject(base.slice(0,-1)+',"value":9007199254740992}').value,9007199254740992);
 assert.throws(()=>parseExactProject('{invalid'));
});
test('archive browser loading accepts every supported release fixture exactly',()=>{
 const folder=new URL('../../validation/projects/',import.meta.url);
 for(const version of readdirSync(folder).filter(v=>/^\d+\.\d+\.\d+$/.test(v))) {
  const raw=readFileSync(new URL(`${version}/project.json`,folder),'utf8');
  assert.equal(parseExactProject(raw).id,JSON.parse(raw).id);
 }
});
test('source inventory counts distinguish absence, restrictions and changed evidence',()=>{
 assert.deepEqual(archiveCounts({sources:['included','restricted','missing','omitted','changed','unsupported','included'].map(status=>({status}))}),{included:2,restricted:1,missing:1,omitted:1,changed:1,unsupported:1});
});
test('checkpoint recovery retains the local portable archive binding',()=>{
 const p={...createProject('Imported',{id:'p-test'}),portableArchive:'a'.repeat(64)};
 const recovered=recoverCheckpoint(p,{checkpoint:{format:'clonoloom-checkpoint',version:1,project:'p-test'},recomputed:false,analysisJSON:'{"datasets":[],"artifacts":[],"decisions":[],"views":[]}'});
 assert.equal(recovered.portableArchive,p.portableArchive);
});
