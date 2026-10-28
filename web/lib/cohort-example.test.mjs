import test from 'node:test';
import assert from 'node:assert/strict';
import { cohortExampleStep } from '../ui/cohort-example.js';

function setup(preset = 'counts-independent') {
  const dataset = {id:'d1', sources:[{path:`/examples/simulated-${preset}/S01-1.tsv`}]};
  const project = {cohortDesigns:[], artifacts:[]};
  const app = {store:{project, sameProject:()=>()=>true},
    ensureDataset:async()=>({status:'open',summary:{samples:Array.from({length:32},(_,i)=>({name:`S${String(i+1).padStart(2,'0')}-1`}))}}),
    examples:{refresh:async()=>[{name:`simulated-${preset}`,path:`/examples/simulated-${preset}`}]}};
  return {app,dataset,project};
}

test('prescribed example annotations never apply to an unrelated dataset or partial study', async () => {
  const {app,dataset}=setup(); let called=false;
  const actions={annotate_samples:async()=>{called=true;}};
  dataset.sources[0].path='/research/S01-1.tsv';
  await assert.rejects(cohortExampleStep(app,actions,dataset,{preset:'counts-independent',step:'design'}), /matching artificial example/);
  assert.equal(called,false);
  dataset.sources[0].path='/examples/simulated-counts-independent/S01-1.tsv';
  app.ensureDataset=async()=>({status:'open',summary:{samples:[{name:'S01-1'}]}});
  await assert.rejects(cohortExampleStep(app,actions,dataset,{preset:'counts-independent',step:'design'}), /membership/);
  assert.equal(called,false);
});

test('confounded and failed saved designs cannot run downstream guided analyses', async () => {
  const s=setup('counts-confounded');
  await assert.rejects(cohortExampleStep(s.app,{},s.dataset,{preset:'counts-confounded',step:'counts'}), /no identifiable group contrast/);
  const q=setup();q.project.cohortDesigns=[{dataset:'d1',name:'Guided counts-independent',result:{ok:false}}];
  await assert.rejects(cohortExampleStep(q.app,{},q.dataset,{preset:'counts-independent',step:'compare'}), /design step first/);
});

test('guided analysis binds the selected dataset and exact latest design version', async () => {
  const {app,dataset,project}=setup();
  project.cohortDesigns=[{id:'design-1',dataset:'d1',name:'Guided counts-independent',result:{ok:true}}, {id:'design-2',dataset:'d1',name:'Guided counts-independent',result:{ok:true}}];
  project.artifacts=[{id:'old-feature',dataset:'d1',kind:'cohort-features'}, {id:'other-dataset-feature',dataset:'d2',kind:'cohort-features'}];
  const original=globalThis.fetch; const read=[];
  globalThis.fetch=async(url)=>{read.push(url);return {ok:true,json:async()=>({result:{selection:{record:url.endsWith('new-feature')?'design-2':'design-1'}}})};};
  try {
    await assert.rejects(cohortExampleStep(app,{},dataset,{preset:'counts-independent',step:'compare'}), /this design version first/);
    assert.deepEqual(read,['api/cohort-features/old-feature']);
    project.artifacts.push({id:'new-feature',dataset:'d1',kind:'cohort-features'});
    let used;
    await cohortExampleStep(app,{compare_cohort:async(args)=>{used=args.features;}},dataset,{preset:'counts-independent',step:'compare'});
    assert.equal(used,'new-feature');
  } finally {globalThis.fetch=original;}
});
