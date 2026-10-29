// Real OCI runtime + native workers. No registry push, scheduler submission or browser.
// node validation/containers.mjs docker|podman LOCAL_IMAGE NEW_OUTPUT_DIRECTORY
import assert from 'node:assert/strict';
import {spawn} from 'node:child_process';
import {createHash} from 'node:crypto';
import {copyFileSync,existsSync,mkdirSync,readFileSync,readdirSync,statSync,writeFileSync} from 'node:fs';
import {join,resolve,relative} from 'node:path';
import {containerCommand} from '../docs/examples/container-run.mjs';
import {prepareBundle} from '../docs/examples/cluster-replay.mjs';
const [engine,image,target,...extra]=process.argv.slice(2);
assert(['docker','podman'].includes(engine)&&image&&target&&!extra.length,'Usage: containers.mjs docker|podman LOCAL_IMAGE NEW_OUTPUT_DIRECTORY');
const root=resolve(import.meta.dirname,'..'),dir=resolve(target),inputs=join(dir,'inputs'),outputs=join(dir,'outputs');
mkdirSync(dir,{mode:0o700});mkdirSync(inputs);mkdirSync(outputs);
const hash=b=>createHash('sha256').update(b).digest('hex');
async function exec(command,args,{env={},expected=0,timeout=180000}={}){
  return new Promise((resolve,reject)=>{
    const child=spawn(command,args,{cwd:root,env:{...process.env,...env},stdio:['ignore','pipe','pipe']});
    let stdout='',stderr='',done=false;const timer=setTimeout(()=>{child.kill('SIGKILL');finish(new Error(`${command} timed out`));},timeout);
    function finish(error){if(done)return;done=true;clearTimeout(timer);error?reject(error):resolve({stdout,stderr});}
    child.stdout.on('data',c=>{stdout+=c;});child.stderr.on('data',c=>{stderr+=c;});child.on('error',finish);
    child.on('close',code=>finish(code===expected?null:new Error(`${command} exit ${code}, expected ${expected}: ${stdout}\n${stderr}`)));
  });
}
const info=JSON.parse((await exec(engine,['image','inspect',image])).stdout)[0];
const imageID=info.Id??info.ID,config=info.Config;
assert.equal(config.User,'65532:65532');assert.deepEqual(config.Entrypoint,['/clonoloom']);assert.deepEqual(config.Cmd,['run','--help']);assert.equal(config.StopSignal,'SIGTERM');
assert(!config.ExposedPorts||!Object.keys(config.ExposedPorts).length);assert(!config.Volumes||!Object.keys(config.Volumes).length);
assert((info.Size??info.VirtualSize)<40*1024*1024,'headless image exceeds 40 MiB budget');
const revision=config.Labels['org.opencontainers.image.revision'];assert(/^[a-zA-Z0-9._-]+$/.test(revision));
const binary=join(dir,'clonoloom-native');
await exec('go',['build','-trimpath','-buildvcs=false','-ldflags',`-s -w -X main.commit=${revision}`,'-o',binary,'.'],{env:{CGO_ENABLED:'0'}});
const user=`${process.getuid()}:${process.getgid()}`;
// Refuse root rather than weakening private fixture ownership to make a test pass.
assert(process.getuid()!==0,'Run this validation as a non-root host user so mount ownership is meaningful');
const restrictions=['--network=none','--read-only','--cap-drop=ALL','--security-opt=no-new-privileges','--user',user,'--pids-limit=128','--memory=1g','--cpus=2',...(engine==='podman'?['--read-only-tmpfs=false']:[])];
const mounts=['--mount',`type=bind,src=${inputs},dst=/inputs,ro`,'--mount',`type=bind,src=${outputs},dst=/output,rw`];
const scratch=['--tmpfs','/scratch:rw,noexec,nosuid,nodev,size=1g,mode=1777'];
const run=(args,options={})=>exec(engine,['run','--rm',...restrictions,...mounts,...(options.noScratch?[]:scratch),imageID,...args],options);
const before=new Map();
function snapshot(folder){for(const entry of readdirSync(folder,{withFileTypes:true})){const path=join(folder,entry.name);if(entry.isDirectory())snapshot(path);else if(!before.has(path))before.set(path,hash(readFileSync(path)));}}
await run(['run','--help']);
await exec(engine,['run','--rm','--network=none','--read-only','--cap-drop=ALL','--security-opt=no-new-privileges',imageID,'--version']);
const certs=[];
for(const [kind,preset] of [['batch-qc','batch-qc-study'],['motifs','motif-study']]){
  const study=join(inputs,kind),recipe=join(inputs,kind+'.json'),native=join(dir,'native-'+kind);
  await exec(binary,['simulate','--preset',preset,'--out',study,'--quiet']);
  await exec(process.execPath,['docs/examples/headless-recipe.mjs',kind,study,recipe,native]);
  snapshot(inputs);
  const declared=JSON.parse(readFileSync(recipe)),task=declared.tasks[0];
  const args=['run','--recipe',recipe,'--out',native,...task.inputs.flatMap(i=>['--input',`${i.name}=${join(study,i.file)}`])];
  await exec(binary,args);
  const inside=['run','--recipe',`/inputs/${kind}.json`,'--out',`/output/${kind}`,...task.inputs.flatMap(i=>['--input',`${i.name}=/inputs/${kind}/${i.file}`])];
  await run(inside);
  copyFileSync(recipe,join(study,'recipe.json'));
  snapshot(inputs);
  const printed=containerCommand(engine,imageID,study,'recipe.json','study',outputs,'printed-'+kind);
  await exec(printed[0],printed.slice(1));
  const record=JSON.parse(readFileSync(join(native,'run.json')));
  for(const f of record.outputs)for(const folder of [kind,'printed-'+kind])assert.deepEqual(readFileSync(join(outputs,folder,f.path)),readFileSync(join(native,f.path)),`${kind} native/container bytes: ${f.path}`);
  const cert=join(inputs,kind+'.certificate.clz');copyFileSync(join(native,'result.certificate.clz'),cert);certs.push(kind,cert);
  snapshot(inputs);
  await run(['verify','--replay','--report',`/output/${kind}-replay.json`,`/inputs/${kind}.certificate.clz`]);
  await exec('uv',['run','--no-project','validation/reference/headless_run.py',join(outputs,kind),join(outputs,kind+'-replay.json')]);
  await run(inside,{expected:2});
  assert.deepEqual(readFileSync(join(outputs,kind,'run.json')),readFileSync(join(native,'run.json')));
  if(kind==='batch-qc'){
    await run(inside.map(a=>a==='/output/batch-qc'?'/inputs/forbidden':a),{expected:2});assert(!existsSync(join(inputs,'forbidden')));
    await run(inside.map(a=>a==='/output/batch-qc'?'/work/forbidden':a),{expected:2});
    await run(inside.map(a=>a==='/output/batch-qc'?'/output/no-scratch':a),{expected:2,noScratch:true});
    const changed=structuredClone(declared);changed.tasks[0].inputs[0].sha256='0'.repeat(64);
    writeFileSync(join(inputs,'changed.json'),JSON.stringify(changed));
    await run(inside.map(a=>a==='/inputs/batch-qc.json'?'/inputs/changed.json':a==='/output/batch-qc'?'/output/changed':a),{expected:2});
    const unsupported=structuredClone(declared);unsupported.tasks[0].kind='diversity';writeFileSync(join(inputs,'unsupported.json'),JSON.stringify(unsupported));
    await run(inside.map(a=>a==='/inputs/batch-qc.json'?'/inputs/unsupported.json':a==='/output/batch-qc'?'/output/unsupported':a),{expected:2});assert(!existsSync(join(outputs,'unsupported')));
    const restricted=structuredClone(declared);restricted.tasks[0].credits=[{Title:'Artificial restriction control',License:'test-only private-use restriction',LicenseClass:'restricted'}];writeFileSync(join(inputs,'restricted.json'),JSON.stringify(restricted));
    const restrictedArgs=inside.map(a=>a==='/inputs/batch-qc.json'?'/inputs/restricted.json':a==='/output/batch-qc'?'/output/restricted':a);
    await run(restrictedArgs,{expected:2});assert(!existsSync(join(outputs,'restricted/batch-qc.json')));
    await run([...restrictedArgs.map(a=>a==='/output/restricted'?'/output/accepted':a),'--accept-terms']);
  }
  console.log(`${kind}: exact native/container scientific files, inventories and replay passed`);
}
const fixtures=join(inputs,'cohort');
await exec('go',['test','-run','^TestContainerReplayFixtures$','-count=1','.'],{env:{CLONOLOOM_CONTAINER_FIXTURES:fixtures}});
snapshot(inputs);
for(const kind of ['cohort-features','cohort-compare','cohort-counts','differs','unavailable']){
  const cert=join(fixtures,kind+'.certificate.clz'),expected=kind==='differs'?1:kind==='unavailable'?2:0;
  await run(['verify','--replay','--report',`/output/${kind}-replay.json`,`/inputs/cohort/${kind}.certificate.clz`],{expected});
  const actual=JSON.parse(readFileSync(join(outputs,kind+'-replay.json')));
  assert.equal(actual.verdict,kind==='differs'?'differs':kind==='unavailable'?'unavailable':'replay-match');
  assert.equal(actual.integrity.verdict,'integrity-match');
  if(!expected){assert(actual.recomputed);assert(actual.steps.every(s=>s.expectedSHA256===s.actualSHA256&&s.metadataMatched));certs.push(kind,cert);}
}
const bundle=join(inputs,'bundle');prepareBundle(bundle,certs);snapshot(inputs);
for(let index=0;index<5;index++){
  const id='task-'+String(index).padStart(3,'0');
  await exec('bash',['cluster/replay-array.sh',binary,bundle,outputs],{env:{SLURM_ARRAY_TASK_ID:String(index),SLURM_ARRAY_JOB_ID:'12345'}});
  await run(['run','--recipe','/inputs/bundle/replay.json','--task',id,'--input',`evidence=/inputs/bundle/${id}.certificate.clz`,'--out',`/output/container-${id}`]);
  const report=JSON.parse(readFileSync(join(outputs,`container-${id}/replay.json`)));
  assert.equal(report.verdict,'replay-match');assert(report.steps.every(s=>s.metadataMatched&&s.expectedSHA256===s.actualSHA256));
}
await exec('bash',['cluster/replay-array.sh',binary,bundle,outputs],{env:{SLURM_ARRAY_TASK_ID:'0',SLURM_ARRAY_JOB_ID:'12345'},expected:2});
await run(['verify','--replay','--expect','0'.repeat(64),'/inputs/batch-qc.certificate.clz'],{expected:1});
// SIGTERM must reach the native process after its cancellation handler and scratch exist.
const large=Buffer.from('sequence_id\tlocus\tproductive\tv_call\tj_call\tjunction_aa\tumi_count\n'+'row\tTRB\tT\tTRBV5-1*01\tTRBJ2-5*01\tCASSLGQETQYF\t1\n'.repeat(1800000));
writeFileSync(join(inputs,'large.tsv'),large);
writeFileSync(join(inputs,'cancel.json'),JSON.stringify({format:'clonoloom-run-recipe',version:1,tasks:[{id:'cancel',operation:'compute',kind:'batch-qc',study:'Cancellation control',inputs:[{name:'source',file:'large.tsv',size:large.length,sha256:hash(large)}],parameters:{samples:[{name:'large',subject:'S1',batch:'A',exclude:''}],options:{}}}]}));
const scratchDir=join(dir,'scratch');mkdirSync(scratchDir);snapshot(inputs);
const name='clonoloom-cancel-'+process.pid;
try{
  await exec(engine,['run','-d','--name',name,...restrictions,...mounts,'--mount',`type=bind,src=${scratchDir},dst=/scratch,rw`,imageID,'run','--recipe','/inputs/cancel.json','--input','source=/inputs/large.tsv','--out','/output/cancel']);
  const deadline=Date.now()+15000;
  while(!readdirSync(scratchDir).length&&Date.now()<deadline)await new Promise(r=>setTimeout(r,20));
  assert(readdirSync(scratchDir).length,'native scratch/cancellation handler did not become ready');
  await exec(engine,['kill','--signal','TERM',name]);
  const waited=await exec(engine,['wait',name],{timeout:120000});assert.equal(waited.stdout.trim(),'2');
  const cancelled=JSON.parse(readFileSync(join(outputs,'cancel/run.json')));assert.equal(cancelled.status,'cancelled');
  assert.deepEqual(readdirSync(scratchDir),[],'handled SIGTERM left scratch files');
}finally{await exec(engine,['rm','-f',name]);}
for(const [file,pin] of before)assert.equal(hash(readFileSync(file)),pin,'source changed: '+relative(inputs,file));
const summary={image:imageID,architecture:info.Architecture,size:info.Size,revision,user,sourceKinds:2,replayKinds:5,arrayTasks:5,byteParity:true,signal:'SIGTERM -> exit 2, cancelled, scratch cleaned',limitations:'Local runtime and array script exercised; no Slurm scheduler or Apptainer runtime was used.'};
writeFileSync(join(dir,'validation.json'),JSON.stringify(summary,null,2)+'\n');
console.log(JSON.stringify(summary,null,2));
