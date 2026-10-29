// Complete record-and-reach study through real reviewed actions. Adapted from create-project-0.7.mjs.
// node validation/create-project-0.8.mjs NEW_FIXTURE [--screenshots]
// Only generated inputs and isolated libraries; no network grant or user-library access.
import assert from 'node:assert/strict';
import {spawn} from 'node:child_process';
import {once} from 'node:events';
import {createServer} from 'node:net';
import {cpSync,existsSync,mkdirSync,mkdtempSync,readFileSync,readdirSync,renameSync,rmSync,writeFileSync} from 'node:fs';
import {join,resolve} from 'node:path';
import {tmpdir} from 'node:os';
import {gzipSync} from 'node:zlib';
import {launch,sleep} from './cdp.mjs';
import {acceptAutomation,waitForAutomationIdle} from './automation.mjs';
import {parseProject,serializeProject} from '../web/lib/project.js';
import {prepareBundle} from '../docs/examples/cluster-replay.mjs';
const root=resolve(import.meta.dirname,'..'),output=resolve(process.argv[2]??'validation/projects/0.8.0');
assert(!existsSync(output),'Never overwrite a release fixture');
const temp=mkdtempSync(join(tmpdir(),'clonoloom-record-study-')),binary=join(temp,'clonoloom'),study=join(temp,'study');
let browser,host,second;const pending=new Map();let next=0,buffer='';
async function command(cmd,args,env={}){const p=spawn(cmd,args,{cwd:root,env:{...process.env,...env},stdio:['ignore','pipe','inherit'],timeout:300000,killSignal:'SIGKILL'});let out='';p.stdout.on('data',c=>{out+=c});assert.equal((await once(p,'exit'))[0],0,cmd+' failed');return out;}
async function freePort(){const s=createServer();await new Promise(r=>s.listen(0,'127.0.0.1',r));const port=s.address().port;await new Promise(r=>s.close(r));return String(port);}
async function connection(library){for(let i=0;i<300&&!existsSync(join(library,'remote.json'));i++)await sleep(50);return JSON.parse(readFileSync(join(library,'remote.json')));}
async function opened(h){await browser.goto(h.url);for(let i=0;i<200&&!await browser.eval('Boolean(window.clonoloom?.remote)');i++)await sleep(50);}
async function act(h,action,args={}){await waitForAutomationIdle(browser);const r=await(await acceptAutomation(fetch(h.url+'/api/remote/action',{method:'POST',headers:{'Content-Type':'application/json','X-ClonoLoom-Token':h.token},body:JSON.stringify({action,args,client:'Record-and-reach fixture'})}),browser)).json();assert(r.ok,JSON.stringify(r));return r.data;}
function rpc(method,params={}){const id=++next;const result=new Promise((resolve,reject)=>{const timer=setTimeout(()=>{pending.delete(id);reject(new Error('MCP study query timed out'));},30000);pending.set(id,{resolve:value=>{clearTimeout(timer);resolve(value)},reject:error=>{clearTimeout(timer);reject(error)}})});host.stdin.write(JSON.stringify({jsonrpc:'2.0',id,method,params:{...params,_meta:{'io.modelcontextprotocol/protocolVersion':'2026-07-28','io.modelcontextprotocol/clientCapabilities':{},'io.modelcontextprotocol/clientInfo':{name:'Record-and-reach study',version:'1'}}}})+'\n');return result;}
async function download(h,action,args,path){await browser.eval(`(()=>{window.studyBlob=null;if(!window.studyCreateURL){window.studyCreateURL=URL.createObjectURL;URL.createObjectURL=b=>{window.studyBlob=b;return window.studyCreateURL(b)}}return true})()`);const result=await act(h,action,args);const data=await browser.eval(`(async()=>{const bytes=new Uint8Array(await window.studyBlob.arrayBuffer());let text='';for(const c of bytes)text+=String.fromCharCode(c);return btoa(text)})()`);writeFileSync(path,Buffer.from(data,'base64'));return result;}
try{
 await command('go',['build','-o',binary,'.']);await command(binary,['simulate','--preset','batch-qc-study','--out',study,'--quiet']);
 const library=join(temp,'library');
 host=spawn(binary,['mcp','--offline','--allow','edits,compute,files','--window','none','--port',await freePort(),'--data-dir',library,'--cache-dir',join(temp,'cache')],{stdio:['pipe','pipe','inherit']});
 host.stdout.on('data',c=>{buffer+=c;while(buffer.includes('\n')){const n=buffer.indexOf('\n'),line=buffer.slice(0,n);buffer=buffer.slice(n+1);try{const m=JSON.parse(line);if(m.id!==undefined){const p=pending.get(m.id);assert(p,'unexpected MCP response');pending.delete(m.id);p.resolve(m);}}catch(e){for(const p of pending.values())p.reject(e);}}});
 const h=await connection(library);browser=await launch({width:1600,height:1100});await opened(h);
 await act(h,'new_project',{name:'Record and reach · simulated batch QC'});
 const files=readdirSync(study).filter(n=>n.endsWith('.tsv')||n.endsWith('.airr.json')).sort();
 await act(h,'open_data',{paths:files.map(n=>join(study,n)),name:'Artificial batch-QC study · 26 observations'});
 const http=await act(h,'query_clonotypes',{limit:10});
 const mcp=await rpc('tools/call',{name:'query_clonotypes',arguments:{limit:10}});assert(!mcp.error,JSON.stringify(mcp));assert.equal(mcp.result.isError,false);assert.equal(mcp.result.structuredContent.data.resultJSON,http.resultJSON);
 const py=join(temp,'python.json'),r=join(temp,'r.json'),rLibrary=join(temp,'r-library');mkdirSync(rLibrary);
 writeFileSync(join(temp,'query.py'),'import sys, pathlib, clonoloom; c=clonoloom.connect(data_dir=sys.argv[1], client="Worked study Python"); pathlib.Path(sys.argv[2]).write_text(c.query_clonotypes(limit=10).data["resultJSON"])');
 await command('uv',['run','--no-project',join(temp,'query.py'),library,py],{PYTHONPATH:join(root,'clients/python/src')});
 await command('R',['CMD','INSTALL','--library='+rLibrary,join(root,'clients/r')]);
 await command('Rscript',['-e','library(clonoloom); a<-commandArgs(TRUE); cl<-cl_connect(data_dir=a[[1]],client="Worked study R"); x<-cl_query_clonotypes(limit=10L,cl=cl); writeBin(charToRaw(x$data$resultJSON),a[[2]])',library,r],{R_LIBS:rLibrary});
 assert.equal(readFileSync(py,'utf8'),http.resultJSON);assert.equal(readFileSync(r,'utf8'),http.resultJSON);
 const samples=JSON.parse(readFileSync(join(study,'truth/samples.json')));
 await act(h,'screen_batch_qc',{samples,options:{threshold:3.5,minSamples:5}});
 let project=await browser.eval('window.clonoloom.store.project');assert.equal(project.artifacts.length,1);const artifact=project.artifacts[0].id,originalResult=readFileSync(join(library,'artifacts',project.artifacts[0].id,'batch-qc.json'));
 const checkpoint=await act(h,'create_checkpoint',{name:'Original definition and complete QC evidence'});
 await act(h,'set_definition',{preset:'Amino-acid junction only'});
 const difference=await act(h,'compare_checkpoint',{checkpoint:checkpoint.id});assert(difference.changes.some(c=>c.category==='definition'||c.path.includes('definition')),'definition change absent');
 if(process.argv.includes('--screenshots'))for(const theme of ['light','dark']){
  await act(h,'set_theme',{theme});
  await browser.eval(`(()=>{const app=window.clonoloom;app.commit='';app.store.notify(['library']);const e=document.querySelector('.checkpoints-view');e.closest('.workbench-scroll').scrollTop=e.offsetTop-20;return true})()`);
  await sleep(250);
  await browser.eval(`(()=>{const w=document.createTreeWalker(document.body,NodeFilter.SHOW_TEXT);while(w.nextNode())w.currentNode.nodeValue=w.currentNode.nodeValue.split(${JSON.stringify(temp)}).join('/path/to/worked-study');return true})()`);
  await browser.capture(join(root,'docs/images/record-study-'+theme+'.webp'),{format:'webp',quality:86});
 }
 assert.deepEqual(readFileSync(join(library,'artifacts',artifact,'batch-qc.json')),originalResult);
 const recovered=await act(h,'recover_checkpoint',{checkpoint:checkpoint.id});assert.equal(recovered.recomputed,false);assert.equal((await act(h,'query_clonotypes',{limit:10})).resultJSON,http.resultJSON);
 await browser.eval('window.clonoloom.saveNow()');
 const evidence=join(temp,'evidence');mkdirSync(evidence);
 const certificate=await download(h,'export_certificate',{artifact},join(evidence,'result.certificate.clz'));
 await download(h,'export_html_report',{artifacts:[artifact],title:'Simulated batch QC · exact evidence and limits'},join(evidence,'report.html'));
 await command(binary,['verify','--integrity','--report',join(evidence,'integrity.json'),join(evidence,'result.certificate.clz')]);
 await command(binary,['verify','--replay','--report',join(evidence,'replay.json'),join(evidence,'result.certificate.clz')]);
 assert.equal(JSON.parse(readFileSync(join(evidence,'replay.json'))).verdict,'replay-match');
 prepareBundle(join(evidence,'bundle'),['batch-qc',join(evidence,'result.certificate.clz')]);
 await command(binary,['run','--recipe',join(evidence,'bundle/replay.json'),'--task','task-000','--input','evidence='+join(evidence,'bundle/task-000.certificate.clz'),'--out',join(temp,'headless')]);
 assert.equal(JSON.parse(readFileSync(join(temp,'headless/replay.json'))).verdict,'replay-match');
 const archive=await download(h,'export_project_archive',{includeSources:true,ownSources:true},join(evidence,'study.clz'));
 const inspected=await act(h,'inspect_project_archive',{path:join(evidence,'study.clz'),expected:archive.fingerprint});
 const original=inspected.projectJSON;project=JSON.parse(original);
 mkdirSync(output,{mode:0o700});cpSync(study,join(output,'data'),{recursive:true});cpSync(evidence,join(output,'evidence'),{recursive:true});
 cpSync(join(library,'checkpoints'),join(output,'checkpoints'),{recursive:true});
 writeFileSync(join(output,'original-project.json'),original);
 writeFileSync(join(output,'locations.json'),JSON.stringify({original:study,fixture:'data'}));
 writeFileSync(join(output,'query.json'),http.resultJSON);writeFileSync(join(output,'differences.json'),JSON.stringify(difference,null,2)+'\n');
 for(const ds of project.datasets)for(const f of [...ds.sources,...(ds.metadata??[])]){assert(f.path.startsWith(study+'/'));f.path='data/'+f.path.slice(study.length+1);}
 writeFileSync(join(output,'project.json'),serializeProject(parseProject(JSON.stringify(project))));
 const src=join(library,'artifacts',artifact),dst=join(output,'artifacts',artifact);mkdirSync(dst,{recursive:true});
 for(const file of readdirSync(src))writeFileSync(join(dst,file+'.gz'),gzipSync(readFileSync(join(src,file))));
 renameSync(study,join(temp,'unavailable-originals'));
 const importedLibrary=join(temp,'imported');second=spawn(binary,['--remote-control','--offline','--allow','edits,compute,files','--window','none','--port',await freePort(),'--data-dir',importedLibrary,'--cache-dir',join(temp,'imported-cache')],{stdio:['ignore','ignore','inherit']});
 const imported=await connection(importedLibrary);await opened(imported);
 await act(imported,'import_project_archive',{path:join(evidence,'study.clz'),expected:archive.fingerprint});
 assert.equal((await act(imported,'query_clonotypes',{limit:10})).resultJSON,http.resultJSON);
 assert.equal((await act(imported,'recover_checkpoint',{checkpoint:checkpoint.id})).recomputed,false);
 assert.equal((await act(imported,'query_clonotypes',{limit:10})).resultJSON,http.resultJSON);
 assert.deepEqual(readFileSync(join(importedLibrary,'artifacts',artifact,'batch-qc.json')),readFileSync(join(src,'batch-qc.json')));
 await act(imported,'compare_checkpoint',{checkpoint:checkpoint.id});
 for(const theme of ['light','dark']){
  await act(imported,'set_theme',{theme});await browser.eval(`(()=>{const e=document.querySelector('.checkpoints-view');e.closest('.workbench-scroll').scrollTop=e.offsetTop-20;return true})()`);await sleep(250);
  await browser.eval(`(0,eval)(${JSON.stringify(readFileSync(join(root,'validation/cache/axe-core/axe.min.js'),'utf8'))});true`);
  assert.deepEqual(await browser.eval(`(async()=>{const r=await axe.run(document,{runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21a','wcag21aa']}});return r.violations.map(v=>({id:v.id,nodes:v.nodes.map(n=>n.target)}))})()`),[]);

 }
 writeFileSync(join(output,'study.json'),JSON.stringify({format:'clonoloom-record-study',version:1,software:h.version,preset:'batch-qc-study',artifact,checkpoint:checkpoint.id,certificate:certificate.fingerprint,archive:archive.fingerprint,transports:['window/HTTP','MCP','Python','R'],queryBytesEqual:true,replay:'replay-match',recoveryRecomputed:false,limits:'Artificial descriptive QC; no batch correction, treatment inference, automatic exclusion or specificity claim. Recovery/integrity are separate from exact captured-evidence replay.'},null,2)+'\n');
 console.log('0.8.0 study: four exact query transports, checkpoint change/recovery, report, certificate, headless replay and source-free archive transfer passed.');
}finally{await browser?.close();for(const p of [host,second])if(p&&p.exitCode===null&&p.signalCode===null){const stopped=once(p,'exit');p.stdin?.end();p.kill();await Promise.race([stopped,sleep(5000)]);if(p.exitCode===null&&p.signalCode===null){p.kill('SIGKILL');await stopped;}}rmSync(temp,{recursive:true,force:true});}
