// Seeded artificial longitudinal repertoire: private review, separate downloads and unchanged science.
// node validation/deidentification.mjs /absolute/path/to/clonoloom [--screenshots]
import assert from 'node:assert/strict';
import {spawn} from 'node:child_process';
import {once} from 'node:events';
import {existsSync,mkdtempSync,readFileSync,rmSync} from 'node:fs';
import {join,resolve} from 'node:path';
import {tmpdir} from 'node:os';
import {launch,sleep} from './cdp.mjs';
import {acceptAutomation,waitForAutomationIdle} from './automation.mjs';
const binary=resolve(process.argv[2]),temp=mkdtempSync(join(tmpdir(),'clonoloom-deidentification-'));
let host,browser;
async function command(args){const p=spawn(binary,args,{stdio:['ignore','ignore','inherit']});assert.equal((await once(p,'exit'))[0],0);}
try {
 await command(['simulate','--preset','vaccine','--out',join(temp,'study'),'--quiet']);
 const library=join(temp,'library');
 host=spawn(binary,['--remote-control','--allow','edits,compute,files','--offline','--window','none','--port','9494','--data-dir',library,'--cache-dir',join(temp,'cache')],{stdio:['ignore','ignore','inherit']});
 for(let i=0;i<300&&!existsSync(join(library,'remote.json'));i++)await sleep(50);
 const h=JSON.parse(readFileSync(join(library,'remote.json'),'utf8'));
 browser=await launch({width:1600,height:1300});await browser.goto(h.url);
 for(let i=0;i<100&&!await browser.eval('Boolean(window.clonoloom)');i++)await sleep(50);
 async function act(action,args={}){
  await waitForAutomationIdle(browser);
  const response=await acceptAutomation(fetch(h.url+'/api/remote/action',{method:'POST',headers:{'Content-Type':'application/json','X-ClonoLoom-Token':h.token},body:JSON.stringify({action,args,client:'De-identification validation'})}),browser);
  const value=await response.json();assert(value.ok,JSON.stringify(value));return value.data;
 }
 await act('new_project',{name:'Artificial vaccination study · export review'});
 await act('open_data',{paths:['S01-pre-1.tsv','S01-d14.tsv','S01-d28.tsv','S02-pre-1.tsv','S02-d14.tsv','S02-d28.tsv'].map(n=>join(temp,'study',n)),name:'Simulated vaccination · two subjects'});
 const sampleNames=await browser.eval('window.clonoloom.store.project.datasets[0].samples');
 await act('annotate_samples',{samples:Object.fromEntries(sampleNames.map(name=>[name,{subject:name.startsWith('S01')?'Simulated subject 01':'Simulated subject 02',timepoint:name.includes('d14')?'day 14':name.includes('d28')?'day 28':'baseline',condition:'artificial vaccination',hla:'HLA-A*02:01'}]))});
 const before=await act('query_clonotypes',{limit:1000});assert(JSON.parse(before.resultJSON).count>100);
 const original=await browser.eval('JSON.stringify(window.clonoloom.store.project)');
 const p=await act('prepare_deidentified_export',{dropText:true,dropHLA:true});
 assert.equal(p.record.subjects,2);assert.equal(p.record.samples,6);assert(p.record.retainedRows>10000);
 assert.equal(p.record.unit,'umis');
 assert.equal(await browser.eval('JSON.stringify(window.clonoloom.store.project)'),original);
 assert.equal((await act('query_clonotypes',{limit:1000})).resultJSON,before.resultJSON);
 // Rejected downloads do not create files; map download is separately requested and reviewed.
 await browser.eval(`(()=>{window.deidDownloads=[];const create=URL.createObjectURL;URL.createObjectURL=blob=>{window.deidDownloads.push(blob);return create(blob)};return true})()`);
 const pending=fetch(h.url+'/api/remote/action',{method:'POST',headers:{'Content-Type':'application/json','X-ClonoLoom-Token':h.token},body:JSON.stringify({action:'download_deidentified_export',args:{token:p.token,expected:p.fingerprint},client:'Rejected download control'})});
 for(let i=0;i<100&&!await browser.eval('Boolean(document.querySelector("[data-automation-proposal]"))');i++)await sleep(50);
 await browser.eval(`(()=>{[...document.querySelectorAll('[data-automation-proposal] button')].find(b=>b.textContent==='Reject').click();return true})()`);
 assert.equal((await(await pending).json()).code,'rejected');assert.equal(await browser.eval('window.deidDownloads.length'),0);
 const downloaded=await act('download_deidentified_export',{token:p.token,expected:p.fingerprint});assert.equal(downloaded.part,'bundle');
 assert.equal(await browser.eval('window.deidDownloads.length'),1);
 await act('download_deidentified_export',{token:p.token,expected:p.fingerprint,identityMap:true});
 assert.equal(await browser.eval('window.deidDownloads.length'),2);
 const privateMap=await browser.eval('(async()=>JSON.parse(await window.deidDownloads[1].text()))()');
 assert.equal(privateMap.bundleFingerprint,p.fingerprint);assert.equal(privateMap.bindings.filter(b=>b.kind==='subject').length,2);
 // Changing a removal choice invalidates the browser receipt before any download.
 await browser.eval(`(()=>{const input=document.querySelector('.deidentification-view input[type=checkbox]');input.click();if(window.clonoloom.deidentification.state().prepared)throw new Error('Choice change retained stale review');return true})()`);
 await act('prepare_deidentified_export',{dropText:true,dropHLA:true});
 const axe=readFileSync(resolve('validation/cache/axe-core/axe.min.js'),'utf8');
 for(const theme of ['light','dark']) {
  await act('set_theme',{theme});
  await browser.eval(`(()=>{const root=document.querySelector('.deidentification-view');root.closest('.workbench-scroll').scrollTop=root.offsetTop-20;return true})()`);await sleep(200);
  await browser.eval(`(0,eval)(${JSON.stringify(axe)});true`);
  const violations=await browser.eval(`(async()=>{const r=await axe.run(document,{runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21a','wcag21aa']}});return r.violations.map(v=>({id:v.id,nodes:v.nodes.map(n=>n.target)}))})()`);assert.deepEqual(violations,[]);
  await browser.eval(`(async()=>{const app=window.clonoloom;app.commit='';app.store.notify(['library']);await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));const walker=document.createTreeWalker(document.body,NodeFilter.SHOW_TEXT);for(let node=walker.nextNode();node;node=walker.nextNode())node.nodeValue=node.nodeValue.split(${JSON.stringify(library)}).join('~/Library/Application Support/ClonoLoom');return true})()`);
  if(process.argv.includes('--screenshots'))await browser.capture(resolve('docs/images/deidentification-'+theme+'.webp'),{format:'webp',quality:86});
 }
 // A response arriving after an edit is refused; abort clears the busy state.
 await browser.eval(`(async()=>{
  const app=window.clonoloom,original=window.fetch;let release,entered;
  const waiting=new Promise(r=>entered=r),gate=new Promise(r=>release=r);
  window.fetch=async(...args)=>{const response=await original(...args);entered();await gate;return response};
  try{const pending=app.deidentification.prepare().then(()=>{throw new Error('Stale export accepted')},e=>{if(!e.message.includes('project changed'))throw e});await waiting;app.store.commit({...app.store.project,name:'Temporary stale control'},'Stale control');release();await pending;app.store.undo();if(app.deidentification.state().prepared)throw new Error('Stale receipt retained');}finally{window.fetch=original}
  window.fetch=(_url,opts)=>new Promise((_resolve,reject)=>{if(opts.signal.aborted)reject(new DOMException('Aborted','AbortError'));else opts.signal.addEventListener('abort',()=>reject(new DOMException('Aborted','AbortError')),{once:true})});
  try{const pending=app.deidentification.prepare().catch(e=>{if(e.name!=='AbortError')throw e});app.deidentification.cancel();await pending;if(app.deidentification.state().busy||!app.deidentification.state().error.includes('cancelled'))throw new Error('Cancellation stuck');}finally{window.fetch=original}
  return true;
 })()`);
 console.log('De-identification: two simulated subjects/six observations, exact native query, separate pinned downloads, review rejection, stale project, cancellation and both-theme accessibility passed.');
} finally {if(browser)await browser.close();if(host){host.kill();if(host.exitCode===null)await once(host,'exit')}rmSync(temp,{recursive:true,force:true});}
