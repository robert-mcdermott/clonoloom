// Inspect the actual filtered build context and exported image, not a simulated ignore matcher.
// node validation/container-context.mjs docker|podman LOCAL_RUNTIME_IMAGE NEW_OUTPUT_DIRECTORY
import assert from 'node:assert/strict';
import {spawnSync} from 'node:child_process';
import {mkdirSync,writeFileSync,rmSync} from 'node:fs';
import {join,resolve} from 'node:path';
const [engine,image,target,...extra]=process.argv.slice(2);
assert(['docker','podman'].includes(engine)&&image&&target&&!extra.length);
const root=resolve(import.meta.dirname,'..'),out=resolve(target);mkdirSync(out,{mode:0o700});
const canary=join(root,`.clonoloom-context-control-${process.pid}`),plain=join(root,`context-control-${process.pid}.txt`);
const stage=`localhost/clonoloom-context-control:${process.pid}`,name=`clonoloom-inventory-${process.pid}`;
function exec(command,args){const r=spawnSync(command,args,{cwd:root,encoding:'utf8',timeout:180000,maxBuffer:4*1024*1024});assert.equal(r.status,0,`${command}: ${r.stderr}`);return r.stdout;}
let created=false,built=false;
try{
  mkdirSync(canary);writeFileSync(join(canary,'unexpected.go'),'// build-context exclusion control\n');writeFileSync(plain,'must not enter build context\n',{flag:'wx'});
  exec(engine,['build','--network=none','--target','build','--build-arg','REVISION=context-validation','-t',stage,'.']);built=true;
  const files=exec(engine,['run','--rm','--network=none','--read-only',stage,'find','/src','-type','f']).trim().split('\n').map(p=>p.replace(/^\/src\//,''));
  const allowed=p=>/^[^/]+\.go$/.test(p)||/^internal\/.*\.go$/.test(p)||['Dockerfile','go.mod','LICENSE','NOTICE','validation/sources.json','internal/adc/registry.tsv','web/index.html','web/styles.css','web/app.js','web/favicon.svg','web/ui/offline-report.css','testdata/bcell-inputs.tsv','testdata/expression-hdf5/checksum.h5','testdata/anndata/gzip-csr.h5ad'].includes(p)||/^web\/(lib|ui)\/[^/]+\.js$/.test(p)||/^testdata\/imgt-inputs\/[^/]+\.txt$/.test(p)||/^testdata\/expression-mex\/[^/]+\.(tsv|mtx)$/.test(p);
  assert(files.length>100);const unexpected=files.filter(p=>!allowed(p)||p.endsWith('_test.go'));assert.equal(unexpected.length,0,'Unapproved context classes: '+JSON.stringify({injected:unexpected.filter(p=>p.includes('context-control')).length,ignoreFile:unexpected.includes('.dockerignore'),testGo:unexpected.filter(p=>p.endsWith('_test.go')).length,other:unexpected.filter(p=>!p.includes('context-control')&&p!=='.dockerignore'&&!p.endsWith('_test.go')).length}));
  exec(engine,['create','--name',name,image]);created=true;
  const archive=join(out,'runtime.tar');exec(engine,['export','--output',archive,name]);
  const members=exec('tar',['-tf',archive]).trim().split('\n').map(p=>p.replace(/^\.\//,''));
  // Docker also creates a console mount placeholder and the conventional /proc/mounts link.
  const permitted=new Set(['clonoloom','licenses/LICENSE','licenses/NOTICE','licenses/GO-LICENSE','licenses/GO-PATENTS','etc/hosts','etc/hostname','etc/resolv.conf','etc/mtab','dev/console','run/.containerenv','.dockerenv']);
  const unexpectedMembers=members.filter(p=>!p.endsWith('/')&&p!==''&&!permitted.has(p));
  assert.equal(unexpectedMembers.length,0,'Unexpected runtime payload: '+JSON.stringify(unexpectedMembers));
  for(const p of ['clonoloom','licenses/LICENSE','licenses/NOTICE','licenses/GO-LICENSE','licenses/GO-PATENTS'])assert(members.includes(p),p);
  writeFileSync(join(out,'inventory.json'),JSON.stringify({contextFiles:files.length,runtimeMembers:members},null,2)+'\n');
  console.log(`Build context: ${files.length} allowlisted source/assets; injected excluded files absent. Runtime contains only the native program, licenses, mount points and engine-created files.`);
}finally{
  if(created)exec(engine,['rm','-f',name]);if(built)exec(engine,['image','rm',stage]);
  rmSync(canary,{recursive:true,force:true});rmSync(plain,{force:true});
}
