// Two isolated hosts: exact portable evidence and reviewed import, never a user library.
// node validation/project-archives.mjs /absolute/path/to/clonoloom [--screenshots]
import assert from 'node:assert/strict';
import {spawn} from 'node:child_process';
import {once} from 'node:events';
import {existsSync,mkdtempSync,readFileSync,writeFileSync,rmSync,renameSync} from 'node:fs';
import {join,resolve} from 'node:path';
import {tmpdir} from 'node:os';
import {launch,sleep} from './cdp.mjs';
import {acceptAutomation,waitForAutomationIdle} from './automation.mjs';
const binary=resolve(process.argv[2]),temp=mkdtempSync(join(tmpdir(),'clonoloom-archives-'));
const hosts=[];let browser;
async function command(args){const p=spawn(binary,args,{stdio:['ignore','ignore','inherit']});assert.equal((await once(p,'exit'))[0],0);}
async function host(n){
 const library=join(temp,'library-'+n),process=spawn(binary,['--remote-control','--allow','edits,compute,files','--offline','--window','none','--port',String(9491+n),'--data-dir',library,'--cache-dir',join(temp,'cache-'+n)],{stdio:['ignore','ignore','inherit']});hosts.push(process);
 for(let i=0;i<300&&!existsSync(join(library,'remote.json'));i++)await sleep(50);
 return {library,...JSON.parse(readFileSync(join(library,'remote.json'),'utf8'))};
}
async function go(h){await browser.goto(h.url);for(let i=0;i<100&&!await browser.eval('Boolean(window.clonoloom)');i++)await sleep(50);}
async function act(h,action,args={}){
 await waitForAutomationIdle(browser);
 const response=await acceptAutomation(fetch(h.url+'/api/remote/action',{method:'POST',headers:{'Content-Type':'application/json','X-ClonoLoom-Token':h.token},body:JSON.stringify({action,args,client:'Archive validation'})}),browser);
 const r=await response.json();assert(r.ok,JSON.stringify(r));return r.data;
}
try {
 await command(['simulate','--preset','vaccine','--out',join(temp,'study'),'--quiet']);
 const first=await host(0);browser=await launch({width:1600,height:1100});await go(first);
 await act(first,'new_project',{name:'Artificial vaccination study · portable evidence'});
 await act(first,'open_data',{paths:['S01-pre-1.tsv','S01-d14.tsv','S01-d28.tsv'].map(n=>join(temp,'study',n)),name:'Simulated vaccination · subject S01'});
 const before=await act(first,'query_clonotypes',{limit:1000});assert(JSON.parse(before.resultJSON).count>100);
 await act(first,'measure_diversity');
 const cp=await act(first,'create_checkpoint',{name:'Before transfer'});
 const original=await browser.eval('JSON.stringify(window.clonoloom.store.project)');
 await browser.eval(`(()=>{const create=URL.createObjectURL;URL.createObjectURL=(blob)=>{window.archiveDownload=blob;return create(blob)};return true})()`);
 const inventory=await act(first,'export_project_archive',{includeSources:true,ownSources:true});
 assert.equal(inventory.sources.filter(s=>s.status==='included').length,6);
 const data=await browser.eval(`(async()=>{const b=new Uint8Array(await window.archiveDownload.arrayBuffer());let text='';for(const c of b)text+=String.fromCharCode(c);return btoa(text)})()`);
 const path=join(temp,'study.clz');writeFileSync(path,Buffer.from(data,'base64'));
 renameSync(join(temp,'study'),join(temp,'unavailable-originals'));
 const second=await host(1);await go(second);
 const inspected=await act(second,'inspect_project_archive',{path,expected:inventory.fingerprint});
 assert.equal(inspected.projectJSON,original);assert.equal(inspected.recomputed,false);
 // Rejection must leave the target library without the imported project.
 const pending=fetch(second.url+'/api/remote/action',{method:'POST',headers:{'Content-Type':'application/json','X-ClonoLoom-Token':second.token},body:JSON.stringify({action:'import_project_archive',args:{path,expected:inventory.fingerprint},client:'Rejected archive control'})});
 for(let i=0;i<100&&!await browser.eval('Boolean(document.querySelector("[data-automation-proposal]"))');i++)await sleep(50);
 await browser.eval(`(()=>{[...document.querySelectorAll('[data-automation-proposal] button')].find(b=>b.textContent==='Reject').click();return true})()`);
 assert.equal((await(await pending).json()).code,'rejected');
 assert(!existsSync(join(second.library,'projects',inventory.project+'.json')));
 await act(second,'import_project_archive',{path,expected:inventory.fingerprint});
 const after=await act(second,'query_clonotypes',{limit:1000});assert.equal(after.resultJSON,before.resultJSON);
 const recovered=await act(second,'recover_checkpoint',{checkpoint:cp.id});assert.equal(recovered.recomputed,false);
 assert.equal((await act(second,'query_clonotypes',{limit:1000})).resultJSON,before.resultJSON);
 const originalFile=readFileSync(join(second.library,'imports',inventory.fingerprint,'project.json'),'utf8');assert.equal(originalFile,original);
 for(const f of inventory.files.filter(f=>f.path.startsWith('artifacts/')||f.path.startsWith('checkpoints/')))assert.deepEqual(readFileSync(join(first.library,f.path)),readFileSync(join(second.library,f.path)));
 // Inspect and display the complete inventory in both themes; no scientific figure is changed.
 await act(second,'inspect_project_archive',{path,expected:inventory.fingerprint});
 for(const theme of ['light','dark']){
  await act(second,'set_theme',{theme});
  await browser.eval(`(()=>{const root=document.querySelector('.project-archives-view');root.closest('.workbench-scroll').scrollTop=root.offsetTop-20;return true})()`);
  await sleep(300);
  const axe=readFileSync(resolve('validation/cache/axe-core/axe.min.js'),'utf8');
  await browser.eval(`(0,eval)(${JSON.stringify(axe)});true`);
  const violations=await browser.eval(`(async()=>{const r=await axe.run(document,{runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21a','wcag21aa']}});return r.violations.map(v=>({id:v.id,nodes:v.nodes.map(n=>n.target)}))})()`);
  assert.deepEqual(violations,[]);
  await browser.eval(`(async()=>{const app=window.clonoloom;app.commit='';await app.saveNow();app.store.notify(['library']);await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));const walker=document.createTreeWalker(document.body,NodeFilter.SHOW_TEXT);for(let node=walker.nextNode();node;node=walker.nextNode())node.nodeValue=node.nodeValue.split(${JSON.stringify(second.library)}).join('~/Library/Application Support/ClonoLoom');return true})()`);
  if(process.argv.includes('--screenshots'))await browser.capture(resolve('docs/images/project-archives-'+theme+'.webp'),{format:'webp',quality:86});
 }
 // Delayed inspection cannot attach to a changed project; cancellation is visible.
 await browser.eval(`(async()=>{
  const app=window.clonoloom,original=window.fetch;let release,entered;
  const waiting=new Promise(r=>entered=r),gate=new Promise(r=>release=r);
  window.fetch=async(...args)=>{const response=await original(...args);entered();await gate;return response};
  try{
   const pending=app.archives.inspect(${JSON.stringify(path)}).then(()=>{throw new Error('Stale inspection succeeded')},e=>{if(!e.message.includes('project changed'))throw e});
   await waiting;app.store.commit({...app.store.project,name:'Temporary stale control'},'Stale control');release();await pending;app.store.undo();
  }finally{window.fetch=original}
  window.fetch=(_url,opts)=>new Promise((_resolve,reject)=>opts.signal.addEventListener('abort',()=>reject(new DOMException('Aborted','AbortError')),{once:true}));
  try{const pending=app.archives.inspect(${JSON.stringify(path)}).catch(e=>{if(e.name!=='AbortError')throw e});app.archives.cancel();await pending;if(app.archives.state().busy||!app.archives.state().error.includes('cancelled'))throw new Error('Cancellation state stuck');}
  finally{window.fetch=original}return true;
 })()`);
 console.log('Portable archives: reviewed export/import, rejection, exact native query/evidence, missing original paths, checkpoint recovery, both-theme accessibility passed.');
} finally {if(browser)await browser.close();for(const h of hosts)h.kill();await Promise.all(hosts.map(h=>h.exitCode!==null?Promise.resolve():once(h,'exit')));rmSync(temp,{recursive:true,force:true});}
