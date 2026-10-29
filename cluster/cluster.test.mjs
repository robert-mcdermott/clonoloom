import test from 'node:test';
import assert from 'node:assert/strict';
import {realpathSync,mkdtempSync,writeFileSync,readFileSync,rmSync,existsSync,symlinkSync,mkdirSync,openSync,ftruncateSync,closeSync,statSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join,resolve} from 'node:path';
import {spawnSync} from 'node:child_process';
import {createHash} from 'node:crypto';
import {prepareBundle} from '../docs/examples/cluster-replay.mjs';
const root=resolve(import.meta.dirname,'..');
function temporary(fn){const dir=mkdtempSync(join(tmpdir(),'clonoloom-cluster-test-'));try{fn(dir);}finally{rmSync(dir,{recursive:true,force:true});}}
test('bundle copies exact bytes, pins individual tasks and refuses overwrite',()=>temporary(dir=>{
  const file=join(dir,'source.clz'),raw=Buffer.from([0,128,255,10,13]);writeFileSync(file,raw);
  const out=join(dir,'bundle');assert.equal(prepareBundle(out,['batch-qc',file,'motifs',file]).array,'0-1');
  const recipe=JSON.parse(readFileSync(join(out,'replay.json')));
  assert.equal(recipe.tasks.length,2);assert.deepEqual(readFileSync(join(out,'task-000.certificate.clz')),raw);
  assert.equal(recipe.tasks[0].inputs[0].sha256,createHash('sha256').update(raw).digest('hex'));
  assert.equal(readFileSync(join(out,'tasks.txt'),'utf8'),'task-000\ntask-001\n');
  assert.throws(()=>prepareBundle(out,['batch-qc',file]),/EEXIST/);
  assert.deepEqual(readFileSync(file),raw);
}));
test('bundle rejects unsupported, missing, linked and excessive inputs without creating output',()=>temporary(dir=>{
  const file=join(dir,'source.clz');writeFileSync(file,'bytes');symlinkSync(file,join(dir,'link.clz'));
  for(const pairs of [[],['batch-qc'],['diversity',file],['batch-qc',join(dir,'missing')],['batch-qc',join(dir,'link.clz')],Array(65).fill(['batch-qc',file]).flat()]){
    assert.throws(()=>prepareBundle(join(dir,'out'),pairs));assert(!existsSync(join(dir,'out')));
  }
}));
test('bundle enforces per-file and aggregate byte bounds before copying',()=>temporary(dir=>{
  const file=join(dir,'large.clz'),fd=openSync(file,'w');
  try{ftruncateSync(fd,256*1024*1024+1);}finally{closeSync(fd);}
  const out=join(dir,'bundle');assert.throws(()=>prepareBundle(out,['batch-qc',file]),/256 MiB/);assert(!existsSync(out));
  const smaller=openSync(file,'r+');try{ftruncateSync(smaller,200*1024*1024);}finally{closeSync(smaller);}
  assert.throws(()=>prepareBundle(out,Array(3).fill(['batch-qc',file]).flat()),/512 MiB/);assert(!existsSync(out));
}));
test('bundle creates private certificate and recipe permissions',()=>temporary(dir=>{
  const source=join(dir,'source.clz'),bundle=join(dir,'bundle');writeFileSync(source,'bytes');prepareBundle(bundle,['batch-qc',source]);
  assert.equal(statSync(bundle).mode&0o077,0);
  for(const file of ['task-000.certificate.clz','replay.json','tasks.txt'])assert.equal(statSync(join(bundle,file)).mode&0o077,0);
}));
function invoke(dir,index='0',job='123',status=0){
  const binary=join(dir,'record arguments.sh'),bundle=join(dir,'input bundle'),out=join(dir,'output');
  mkdirSync(bundle,{recursive:true});mkdirSync(out,{recursive:true});
  writeFileSync(join(bundle,'tasks.txt'),'task-000\ntask-001\n');
  writeFileSync(binary,'#!/bin/sh\nprintf "%s\\n" "$@"\nexit '+status+'\n',{mode:0o700});
  return spawnSync('bash',[join(root,'cluster/replay-array.sh'),binary,bundle,out],{env:{...process.env,SLURM_ARRAY_TASK_ID:index,SLURM_ARRAY_JOB_ID:job},encoding:'utf8'});
}
test('array binds exactly one explicit task, quotes paths and propagates native exit status',()=>temporary(dir=>{
  const result=invoke(dir,'1','123',2);assert.equal(result.status,2);
  assert.deepEqual(result.stdout.trim().split('\n'),['run','--recipe',join(dir,'input bundle/replay.json'),'--task','task-001','--input','evidence='+join(dir,'input bundle/task-001.certificate.clz'),'--out',join(dir,'output/123-task-001')]);
  assert(!result.stdout.includes('--accept-terms'));
}));
for(const index of ['', '-1','01','64','2','1+1','$(touch x)','999999999999999999999'])test(`array refuses invalid or unlisted index ${JSON.stringify(index)}`,()=>temporary(dir=>{const r=invoke(dir,index);assert.equal(r.status,2);assert.equal(r.stdout,'');}));
for(const id of ['', '../outside','123;touch x'])test(`array refuses job ID ${JSON.stringify(id)}`,()=>temporary(dir=>{const r=invoke(dir,'0',id);assert.equal(r.status,2);assert.equal(r.stdout,'');}));

const {containerCommand}=await import('../docs/examples/container-run.mjs');
function commandFixture(dir){const inputs=join(dir,'inputs'),outputs=join(dir,'outputs');mkdirSync(inputs);mkdirSync(outputs);writeFileSync(join(inputs,'evidence.clz'),'exact bytes');writeFileSync(join(inputs,'recipe.json'),JSON.stringify({format:'clonoloom-run-recipe',version:1,tasks:[{id:'replay',inputs:[{name:'evidence',file:'evidence.clz'}]}]}));return ['docker','a'.repeat(64),inputs,'recipe.json','replay',outputs,'new-run'];}
test('container command pins an image and explicitly scopes all writable mounts and grants',()=>temporary(dir=>{
  const args=commandFixture(dir),command=containerCommand(...args,{uid:501,gid:20});
  for(const flag of ['--pull=never','--network=none','--read-only','--cap-drop=ALL','--security-opt=no-new-privileges'])assert(command.includes(flag));
  assert(command.includes('type=bind,src='+realpathSync(args[2])+',dst=/inputs,ro'));assert(command.includes('type=bind,src='+realpathSync(args[5])+',dst=/output,readonly=false'));
  assert(command.includes('evidence=/inputs/evidence.clz'));assert(!command.includes('--accept-terms'));assert(!command.includes('--privileged'));
  assert(!existsSync(join(args[5],'new-run')),'printing created an output');
  args[0]='podman';assert(containerCommand(...args,{uid:501,gid:20}).includes('--read-only-tmpfs=false'));
}));
test('container helper rejects root, mutable tags, traversal, overlapping mounts and existing output',()=>temporary(dir=>{
  const args=commandFixture(dir),user={uid:501,gid:20};
  assert.throws(()=>containerCommand(...args,{uid:0,gid:0}));
  for(const [index,value] of [[0,'other'],[1,'clonoloom:latest'],[3,'../recipe.json'],[4,'missing'],[6,'../escape'],[5,args[2]]]){const changed=[...args];changed[index]=value;assert.throws(()=>containerCommand(...changed,user));}
  mkdirSync(join(args[5],'new-run'));assert.throws(()=>containerCommand(...args,user),/must not exist/);
}));
test('container helper refuses linked inputs and unsupported recipes',()=>temporary(dir=>{
  const args=commandFixture(dir),user={uid:501,gid:20};
  rmSync(join(args[2],'evidence.clz'));symlinkSync(join(args[2],'recipe.json'),join(args[2],'evidence.clz'));assert.throws(()=>containerCommand(...args,user),/regular/);
  writeFileSync(join(args[2],'recipe.json'),JSON.stringify({format:'wrong',version:1,tasks:[]}));assert.throws(()=>containerCommand(...args,user),/Unsupported/);
}));
