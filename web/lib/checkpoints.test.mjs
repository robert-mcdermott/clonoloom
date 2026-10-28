import test from 'node:test';
import assert from 'node:assert/strict';
import { createProject, isEmptyProject, parseProject, serializeProject } from './project.js';
import { addCheckpoint, checkpointAnalysis, recoverCheckpoint } from './checkpoints.js';
import { createStore } from '../ui/store.js';

const record = {format:'clonoloom-checkpoint',version:1,id:`cp-${'a'.repeat(64)}`,project:'p-test',name:'Before revision',created:'2026-10-27T00:00:00Z'};
test('checkpoint references and recovery preserve undo, redo and project history', () => {
  const base = {...createProject('Study',{id:'p-test'}), datasets:[{id:'d',definition:{junction:'aa'}}], artifacts:[{id:'result-original'}]};
  const store = createStore(base);
  store.commit(addCheckpoint(base,record),'Checkpoint');
  const checkpointed = store.project;
  assert.deepEqual(parseProject(serializeProject(checkpointed)),checkpointed);
  store.commit({...checkpointed,datasets:[{id:'d',definition:{junction:'nt'}}],artifacts:[],newSetting:true},'Revision');
  const edited = store.project, pending = store.sameProject();
  const response = {checkpoint:record,analysisJSON:JSON.stringify({datasets:base.datasets,artifacts:base.artifacts,decisions:[],views:[]}),recomputed:false};
  store.recover(recoverCheckpoint(edited,response),'Recovery');
  const recovered = store.project;
  assert.equal(pending(),false);
  assert.deepEqual(recovered.datasets,base.datasets); assert.deepEqual(recovered.artifacts,base.artifacts);
  assert.equal(recovered.newSetting,undefined); assert.deepEqual(recovered.checkpoints,edited.checkpoints);
  assert.equal(recovered.history.length,edited.history.length+1); assert.equal(recovered.name,edited.name);
  const duringRecovery = store.sameProject(); store.undo(); assert.equal(store.project,edited); assert.equal(duringRecovery(),false);
  store.redo(); assert.equal(store.project,recovered);
  assert.equal(edited.datasets[0].definition.junction,'nt');
});
test('checkpoint creation is undoable without mutating its prior state', () => {
  const base=createProject('Study',{id:'p-test'}),store=createStore(base);
  store.commit(addCheckpoint(base,record));store.undo();assert.equal(store.project,base);store.redo();assert.equal(store.project.checkpoints.length,1);
  assert.equal(isEmptyProject(store.project),false);
  assert.throws(()=>addCheckpoint(store.project,record),/already/);
  assert.throws(()=>addCheckpoint(base,{...record,project:'p-other'}),/different/);
  assert.throws(()=>parseProject(JSON.stringify({...base,checkpoints:{}})),/checkpoints/);
});
test('recovery refuses changed numeric tokens and unsupported envelopes', () => {
  assert.throws(()=>checkpointAnalysis('{"n":9007199254740993}'),/numeric tokens/);
  assert.throws(()=>checkpointAnalysis('{"n":1.0}'),/numeric tokens/);
  assert.throws(()=>checkpointAnalysis('{"version":2}'),/Unsupported/);
  assert.deepEqual(checkpointAnalysis('{"nameWithNumber":"123", "n":9007199254740992, "scientific":1e-8}'),{nameWithNumber:'123',n:9007199254740992,scientific:1e-8});
  assert.throws(()=>recoverCheckpoint(createProject('Other',{id:'p-other'}),{checkpoint:record,recomputed:false,analysisJSON:'{}'}),/different/);
});
