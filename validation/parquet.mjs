// Isolated real-host workflow; scientific values remain in native Go/Parquet bytes.
// node validation/parquet.mjs /absolute/path/to/clonoloom [--screenshots]
import assert from 'node:assert/strict';
import {spawn} from 'node:child_process';
import {once} from 'node:events';
import {existsSync,mkdtempSync,readFileSync,rmSync} from 'node:fs';
import {join,resolve} from 'node:path';
import {tmpdir} from 'node:os';
import {launch,sleep} from './cdp.mjs';
import {acceptAutomation,waitForAutomationIdle} from './automation.mjs';
const binary=resolve(process.argv[2]),temp=mkdtempSync(join(tmpdir(),'clonoloom-parquet-'));
let host,browser;
async function command(args){const p=spawn(binary,args,{stdio:['ignore','ignore','inherit']});assert.equal((await once(p,'exit'))[0],0);}
try{
 await command(['simulate','--preset','vaccine','--out',join(temp,'study'),'--quiet']);
 const library=join(temp,'library');host=spawn(binary,['--remote-control','--allow','edits,compute,files','--offline','--window','none','--port','9495','--data-dir',library,'--cache-dir',join(temp,'cache')],{stdio:['ignore','ignore','inherit']});
 for(let i=0;i<300&&!existsSync(join(library,'remote.json'));i++)await sleep(50);
 const h=JSON.parse(readFileSync(join(library,'remote.json'),'utf8'));browser=await launch({width:1600,height:1300});await browser.goto(h.url);
 for(let i=0;i<100&&!await browser.eval('Boolean(window.clonoloom)');i++)await sleep(50);
 async function act(action,args={}){await waitForAutomationIdle(browser);const response=await acceptAutomation(fetch(h.url+'/api/remote/action',{method:'POST',headers:{'Content-Type':'application/json','X-ClonoLoom-Token':h.token},body:JSON.stringify({action,args,client:'Parquet validation'})}),browser);const r=await response.json();assert(r.ok,JSON.stringify(r));return r.data;}
 await act('new_project',{name:'Artificial vaccination study · typed exports'});
 await act('open_data',{paths:['S01-pre-1.tsv','S01-d14.tsv','S01-d28.tsv','S02-pre-1.tsv','S02-d14.tsv','S02-d28.tsv'].map(n=>join(temp,'study',n)),name:'Simulated vaccination · two subjects'});
 const samples=await browser.eval('window.clonoloom.store.project.datasets[0].samples');
 await act('annotate_samples',{samples:Object.fromEntries(samples.map(name=>[name,{subject:name.startsWith('S01')?'Simulated subject 01':'Simulated subject 02',timepoint:name.includes('d14')?'day 14':name.includes('d28')?'day 28':'baseline',condition:'artificial vaccination',hla:'HLA-A*02:01'}]))});
 const original=await browser.eval('JSON.stringify(window.clonoloom.store.project)'),before=await act('query_clonotypes',{limit:1000});
 await browser.eval(`(()=>{window.parquetDownloads=[];const create=URL.createObjectURL;URL.createObjectURL=blob=>{window.parquetDownloads.push(blob);return create(blob)};return true})()`);
 const pending=fetch(h.url+'/api/remote/action',{method:'POST',headers:{'Content-Type':'application/json','X-ClonoLoom-Token':h.token},body:JSON.stringify({action:'export_parquet',args:{},client:'Rejected Parquet control'})});
 for(let i=0;i<100&&!await browser.eval('Boolean(document.querySelector("[data-automation-proposal]"))');i++)await sleep(50);
 await browser.eval(`(()=>{[...document.querySelectorAll('[data-automation-proposal] button')].find(b=>b.textContent==='Reject').click();return true})()`);
 assert.equal((await(await pending).json()).code,'rejected');assert.equal(await browser.eval('window.parquetDownloads.length'),0);
 const clones=await act('export_parquet',{what:'clonotypes'});assert.equal(clones.rows,JSON.parse(before.resultJSON).count);assert.equal(clones.unit,'umis');
 const rearrangements=await act('export_parquet',{what:'rearrangements'});assert.equal(rearrangements.rows,46447);
 assert.equal(await browser.eval('window.parquetDownloads.length'),2);
 assert.equal(await browser.eval('JSON.stringify(window.clonoloom.store.project)'),original);
 assert.equal((await act('query_clonotypes',{limit:1000})).resultJSON,before.resultJSON);
 const deid=await act('prepare_deidentified_export',{storageFormat:'parquet',dropText:true,dropHLA:true});assert.equal(deid.record.storageFormat,'parquet');assert.equal(deid.record.subjects,2);assert.equal(deid.record.clonotypes,clones.rows);
 await act('download_deidentified_export',{token:deid.token,expected:deid.fingerprint});assert.equal(await browser.eval('window.parquetDownloads.length'),3);
 assert(!deid.record.files.some(f=>f.path.includes('map')));assert(deid.record.files.some(f=>f.path==='rearrangements.parquet'));
 const axe=readFileSync(resolve('validation/cache/axe-core/axe.min.js'),'utf8');
 for(const theme of ['light','dark']){
  await act('set_theme',{theme});
  await browser.eval(`(0,eval)(${JSON.stringify(axe)});true`);
  const issues=await browser.eval(`(async()=>{const r=await axe.run(document,{runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21a','wcag21aa']}});return r.violations.map(v=>({id:v.id,nodes:v.nodes.map(n=>n.target)}))})()`);assert.deepEqual(issues,[]);
  await browser.eval(`(async()=>{const app=window.clonoloom;app.commit='';app.store.notify(['library']);await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));const walker=document.createTreeWalker(document.body,NodeFilter.SHOW_TEXT);for(let node=walker.nextNode();node;node=walker.nextNode())node.nodeValue=node.nodeValue.split(${JSON.stringify(library)}).join('~/Library/Application Support/ClonoLoom');return true})()`);
  for(const [selector,name] of [['.parquet-view','parquet'],['.deidentification-view','deidentification-parquet']]){
   await browser.eval(`(()=>{const root=document.querySelector(${JSON.stringify(selector)});root.closest('.workbench-scroll').scrollTop=root.offsetTop-20;return true})()`);await sleep(200);
   if(process.argv.includes('--screenshots'))await browser.capture(resolve(`docs/images/${name}-${theme}.webp`),{format:'webp',quality:86});
  }
 }
 await browser.eval(`(async()=>{
  const app=window.clonoloom,original=window.fetch,dataset=app.store.project.datasets[0].id;let entered,release;
  const waiting=new Promise(r=>entered=r),gate=new Promise(r=>release=r);
  window.fetch=async(...args)=>{const response=await original(...args);entered();await gate;return response};
  try{const work=app.parquet.export(dataset).then(()=>{throw new Error('Stale export accepted')},e=>{if(!e.message.includes('project changed'))throw e});await waiting;app.store.commit({...app.store.project,name:'Temporary edit'},'Stale control');release();await work;app.store.undo();}finally{window.fetch=original}
  window.fetch=(_url,opts)=>new Promise((_resolve,reject)=>{if(opts.signal.aborted)reject(new DOMException('Aborted','AbortError'));else opts.signal.addEventListener('abort',()=>reject(new DOMException('Aborted','AbortError')),{once:true})});
  try{const work=app.parquet.export(dataset).catch(e=>{if(e.name!=='AbortError')throw e});app.parquet.cancel();await work;if(app.parquet.state().busy||!app.parquet.state().error.includes('cancelled'))throw new Error('Cancellation stuck');}finally{window.fetch=original}return true;
 })()`);
 console.log('Parquet browser: native and pseudonymized exports, 46,447 rearrangements / 27,215 clonotypes, original project/query parity, reviewed rejection, stale state, cancellation and both-theme accessibility passed.');
}finally{if(browser)await browser.close();if(host){host.kill();if(host.exitCode===null)await once(host,'exit')}rmSync(temp,{recursive:true,force:true});}
