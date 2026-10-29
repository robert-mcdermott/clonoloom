import test from 'node:test';
import assert from 'node:assert/strict';
import { createStore } from '../ui/store.js';

test('automation labels edits, invalidates old work, and preserves undo/redo',()=>{
 const initial={id:'study',name:'Before'}, store=createStore(initial),old=store.sameProject();
 const finish=store.beginAutomation('research notebook');assert.equal(old(),false);
 store.commit({...initial,name:'After'},'Rename');assert.match(store.undoLabel(),/by research notebook/);
 const during=store.sameProject();finish(true);assert.equal(during(),true);store.undo();assert.equal(store.project,initial);store.redo();assert.equal(store.project.name,'After');
 store.commit({...store.project,name:'Manual'},'Rename');assert.equal(store.undoLabel(),'Rename');
});
test('cancelled automation cannot attach changes or replace projects while settling',()=>{
 const initial={id:'study'},store=createStore(initial);const finish=store.beginAutomation('caller');const pending=store.sameProject();finish();assert.equal(pending(),false);
 assert.throws(()=>store.commit({...initial,name:'late'}),/Cancelled/);assert.throws(()=>store.reset({id:'other'}),/Cancelled/);assert.throws(()=>store.replace({...initial}),/Cancelled/);assert.throws(()=>store.beginAutomation('other'),/already/);
 finish(true);store.commit({...initial,name:'manual'});assert.equal(store.project.name,'manual');
});
test('starting a project by automation retains the previous project in undo',()=>{
 const initial={id:'old'},store=createStore(initial);const finish=store.beginAutomation('caller');store.reset({id:'new'});finish(true);store.undo();assert.equal(store.project,initial);store.redo();assert.equal(store.project.id,'new');
});

test('read-only computation preserves view generation and cannot edit a project',()=>{
 const initial={id:'study'},store=createStore(initial),before=store.sameProject();
 const finish=store.beginAutomation(null,{readOnly:true});
 assert.equal(before(),true);assert.throws(()=>store.commit({...initial,name:'no'}),/no permission/);assert.throws(()=>store.reset({id:'other'}),/no permission/);
 finish(true);assert.equal(before(),true);assert.equal(store.project,initial);
});

test('UI-only automation does not invalidate the current scientific view',()=>{
 const store=createStore({id:'study'}),current=store.sameProject();
 const finish=store.beginAutomation('caller',{invalidate:false});
 store.setUI({theme:'dark'});finish(true);assert.equal(current(),true);
});

test('reviewed computation invalidates old attachments without clearing loaded input views', async()=>{
 const store=createStore({id:'study'}),old=store.sameProject(),generation=store.state.generation;
 let loaded='expression-S01',viewGeneration=generation;
 store.subscribe(()=>{if(viewGeneration!==store.state.generation){loaded=null;viewGeneration=store.state.generation;}});
 const finish=store.beginAutomation('research notebook');await Promise.resolve();
 assert.equal(old(),false);assert.equal(loaded,'expression-S01');assert.equal(store.state.generation,generation);
 const current=store.sameProject();store.commit({...store.project,result:'captured evidence'});finish(true);await Promise.resolve();
 assert.equal(current(),true);assert.equal(loaded,'expression-S01');
 store.recover({...store.project,result:'checkpoint evidence'});await Promise.resolve();
 assert.equal(current(),false);assert.equal(loaded,null);
});

test('Undo and Redo invalidate pending work and reload a still-selected saved result', async()=>{
 const result={id:'saved',kind:'batch-qc'},initial={id:'study',artifacts:[result]},store=createStore(initial);
 store.setUI({focus:{kind:'result',id:result.id}});await Promise.resolve();
 let loaded=result.id,generation=store.state.generation;
 store.subscribe(topics=>{
  if(generation!==store.state.generation){generation=store.state.generation;loaded=null;}
  if(topics.has('focus'))loaded=store.project.artifacts.find(r=>r.id===store.ui.focus?.id)?.id??null;
 });
 store.commit({...initial,name:'Changed'});await Promise.resolve();
 const before=store.sameProject();store.undo();await Promise.resolve();
 assert.equal(before(),false);assert.equal(loaded,result.id);
 const undone=store.sameProject();store.redo();await Promise.resolve();
 assert.equal(undone(),false);assert.equal(loaded,result.id);
 store.commit({...store.project,artifacts:[]});store.undo();await Promise.resolve();
 assert.equal(loaded,result.id);store.redo();await Promise.resolve();assert.equal(loaded,null);
});
