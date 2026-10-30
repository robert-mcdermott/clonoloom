// Suite/public-data release controls through the actual window and native endpoints.
// Adapted from create-project-0.5.mjs and create-project-0.8.mjs. Offline isolated library.
// node validation/create-project-0.9.mjs NEW_FIXTURE [--screenshots]
import assert from 'node:assert/strict';
import {spawn} from 'node:child_process';
import {once} from 'node:events';
import {createServer} from 'node:net';
import {createHash} from 'node:crypto';
import {cpSync,existsSync,mkdirSync,mkdtempSync,readFileSync,readdirSync,rmSync,writeFileSync} from 'node:fs';
import {join,relative,resolve} from 'node:path';
import {tmpdir} from 'node:os';
import {gzipSync} from 'node:zlib';
import {launch,sleep} from './cdp.mjs';
import {acceptAutomation,waitForAutomationIdle} from './automation.mjs';
import {parseProject,serializeProject} from '../web/lib/project.js';
const root=resolve(import.meta.dirname,'..'),output=resolve(process.argv[2]??'validation/projects/0.9.0');
assert(!existsSync(output),'Never overwrite a release fixture');
const temp=mkdtempSync(join(tmpdir(),'clonoloom-suite-study-')),binary=join(temp,'clonoloom'),library=join(temp,'library'),examples=join(temp,'examples'),data=join(temp,'data'),evidence=join(temp,'evidence');
const hash=b=>createHash('sha256').update(b).digest('hex');
let browser,host,h;
async function command(cmd,args){const p=spawn(cmd,args,{cwd:root,stdio:['ignore','pipe','inherit']});p.stdout.resume();assert.equal((await once(p,'exit'))[0],0,cmd+' failed');}
async function freePort(){const s=createServer();await new Promise(r=>s.listen(0,'127.0.0.1',r));const p=s.address().port;await new Promise(r=>s.close(r));return String(p);}
async function waitFor(expression){for(let i=0;i<600;i++){if(await browser.eval(expression))return;await sleep(100);}throw Error('Timed out: '+expression);}
async function act(action,args={}){await waitForAutomationIdle(browser);const r=await(await acceptAutomation(fetch(h.url+'/api/remote/action',{method:'POST',headers:{'Content-Type':'application/json','X-ClonoLoom-Token':h.token},body:JSON.stringify({action,args,client:'Suite study release validation'})}),browser)).json();assert(r.ok,JSON.stringify(r));return r.data;}
async function captureDownloads(expression,names){await browser.eval(`(()=>{window.suiteDownloads=[];window.suiteURL=URL.createObjectURL;URL.createObjectURL=b=>{window.suiteDownloads.push(b);return window.suiteURL(b)};return true})()`);try{await browser.eval(expression);const texts=await browser.eval('Promise.all(window.suiteDownloads.map(b=>b.text()))');assert.equal(texts.length,names.length);names.forEach((n,i)=>writeFileSync(join(evidence,n),texts[i]));}finally{await browser.eval('URL.createObjectURL=window.suiteURL;true');}}
async function native(path,body){const r=await fetch(h.url+path,body?{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)}:{});assert(r.ok,await r.clone().text());return r.text();}
async function open(name,folder,files){await act('open_data',{name,paths:files.map(f=>join(data,folder,f))});return (await browser.eval('window.clonoloom.store.project.datasets')).find(d=>d.name===name);}
const inspectors=[];
try{
 await command('go',['build','-o',binary,'.']);
 await command(binary,['simulate','--preset','bcell-immunization','--out',join(examples,'simulated-bcell-immunization'),'--quiet']);
 mkdirSync(data);mkdirSync(evidence);mkdirSync(library);
 for(const name of ['candidates','metadata','airr-objects','native-reports','cellranger-json','public-overlap'])cpSync(join(root,'testdata',name),join(data,name),{recursive:true});
 for(const suffix of ['a','b','c','d'])cpSync(join(data,'public-overlap','adc-download-sim-'+suffix),join(library,'adc-download-sim-'+suffix),{recursive:true});
 host=spawn(binary,['--remote-control','--offline','--allow','edits,compute,files,network','--window','none','--port',await freePort(),'--data-dir',library,'--examples-dir',examples,'--cache-dir',join(temp,'cache')],{stdio:['ignore','ignore','inherit']});
 for(let i=0;i<300&&!existsSync(join(library,'remote.json'));i++)await sleep(100);
 h=JSON.parse(readFileSync(join(library,'remote.json')));assert.match(h.version,/^\d+\.\d+\.\d+$/);
 browser=await launch({width:1600,height:1000});await browser.goto(h.url);await waitFor('Boolean(window.clonoloom?.remote)');
 await act('new_project',{name:'Suite and public data · separate simulated controls'});
 const ds=await open('Simulated B-cell study · supplied chain evidence','candidates',['S01-d14.tsv','S01-d28.tsv','S01-pre.tsv','S02-d14.tsv','S02-d28.tsv','S02-pre.tsv','repertoires.airr.json']);
 for(const [action,args] of [['find_bcell_families',{threshold:0.1}],['reconstruct_bcell_germlines',{reference:'simulated-bcell-immunization'}],['measure_bcell_shm',{}],['summarize_bcell_isotypes',{}],['build_bcell_lineages',{}]])await act(action,args);
 const project=await browser.eval('window.clonoloom.store.project'),artifact=kind=>project.artifacts.find(a=>a.dataset===ds.id&&a.kind===kind).id;
 await waitFor("window.clonoloom.lineagesView?.state().status==='ready'");
 const tree=(await browser.eval('window.clonoloom.lineagesView.state().page.rows')).find(t=>!t.excluded&&t.taxa>2&&t.taxa<=7);assert(tree);
 await browser.eval(`window.clonoloom.lineagesView.openTree(${JSON.stringify(tree.id)})`);
 await browser.eval("[...document.querySelectorAll('button')].find(b=>b.textContent==='Review JC69 likelihood').click();true");
 await browser.eval('(async()=>{await window.clonoloom.likelihoodView.run();return true})()');
 await captureDownloads('(async()=>{await window.clonoloom.likelihoodView.save();return true})()',['likelihood.json']);
 await act('close_dialogs');
 await browser.eval(`window.clonoloom.focusItem({kind:'result',id:${JSON.stringify(artifact('families'))}})`);await waitFor("window.clonoloom.familiesView?.state().status==='ready'");
 await browser.eval("[...document.querySelectorAll('button')].find(b=>b.textContent==='Review B-cell candidates').click();true");
 const config={shm:artifact('shm'),isotypes:artifact('isotypes'),options:{sample:'S01-d28',baseline:'S01-pre',preferredClass:'IgG',criteria:[{metric:'change',direction:'descending'},{metric:'mutation',direction:'descending'}]}};
 await browser.eval(`(async()=>{const v=window.clonoloom.candidatesView;v.configure(${JSON.stringify(config)});await v.run();return true})()`);
 const review=await browser.eval('window.clonoloom.candidatesView.state().review.evidence');assert.equal(review.result.currentTotal,24);assert.equal(review.result.baselineTotal,4);
 const family=review.result.rows.find(f=>f.observations.some(o=>o.rows.some(r=>r.sequenceId==='S01-d28-C000016-IGH'))),obs=family.observations.find(o=>o.rows.some(r=>r.sequenceId==='S01-d28-C000016-IGH')),row=obs.rows.find(r=>r.locus==='IGH');
 await browser.eval(`(async()=>{const v=window.clonoloom.candidatesView;v.select(${JSON.stringify(family.family)},${JSON.stringify(obs.id)},${JSON.stringify(row.id)});await v.inspect();v.approve({sequence:true});return true})()`);
 await captureDownloads('(async()=>{await window.clonoloom.candidatesView.exportFASTA();return true})()',['candidates.json','candidate-sequence.json','candidate.fasta']);
 if(process.argv.includes('--screenshots'))for(const theme of ['light','dark']){await act('set_theme',{theme});await browser.eval("window.clonoloom.commit='';window.clonoloom.store.notify(['library']);document.querySelector('.candidates-view').closest('.dialog-body').scrollTop=0;true");await sleep(250);await browser.capture(join(root,'docs/images/suite-study-'+theme+'.webp'),{format:'webp',quality:86});}
 await act('close_dialogs');
 for(const [name,folder,files,endpoint] of [
  ['Simulated exact YAML metadata','metadata',['study.tsv','yaml/study.yaml'],'metadata?record=0'],
  ['Simulated MiAIRR reporting controls','metadata',['study.tsv','miairr.airr.json'],'metadata?record=0'],
  ['Simulated imported object assertions','airr-objects',['study.tsv','study.yaml'],'airr-objects?record=2'],
  ['Simulated TRUST4 native report','native-reports',['simulated-trust4.tsv'],'native-report?source=0&record=0'],
  ['Simulated VDJtools native table','native-reports',['simulated-vdjtools.tsv'],'native-report?source=0&record=0'],
  ['Simulated Cell Ranger detailed contigs','cellranger-json',['simulated_all_contig_annotations.json'],'native-report?source=0&record=0'],
 ]){const d=await open(name,folder,files);const file=`inspection-${inspectors.length}.json`;writeFileSync(join(evidence,file),await native(`/api/datasets/${d.id}/${endpoint}`));inspectors.push({dataset:d.id,endpoint,file});}
 const query=await open('Simulated public-overlap query · independent TRB control','public-overlap',['query.tsv']);
 await act('show',{view:'data'});await browser.eval("[...document.querySelectorAll('button')].find(b=>b.textContent==='Public overlap').click();true");await waitFor("document.querySelectorAll('.adc-captures input').length===4");
 await browser.eval(`(()=>{const d=document.querySelector('[aria-label="Local query dataset"]');d.value=${JSON.stringify(query.id)};d.dispatchEvent(new Event('change'));return true})()`);await waitFor("document.querySelectorAll('.adc-captures input').length===4");
 await browser.eval("document.querySelectorAll('.adc-captures input').forEach(x=>x.click());document.querySelector('[aria-label=\"Accept presence contract\"]').click();document.querySelector('.adc-overlap .primary').click();true");
 await waitFor("document.querySelector('.adc-overlap [role=\"status\"]').textContent.includes('Comparison saved')");
 const overlaps=readdirSync(library).filter(f=>/^adc-overlap-.*\.json$/.test(f));assert.equal(overlaps.length,1);cpSync(join(library,overlaps[0]),join(evidence,'public-overlap.json'));
 const overlap=JSON.parse(readFileSync(join(evidence,'public-overlap.json'))),expected=JSON.parse(readFileSync(join(data,'public-overlap/expected.json')));for(const k of ['query','queryKeysShared','assignedSubjects','evaluableSubjects','missingSubjects'])assert.deepEqual(overlap.result[k],expected[k]);
 await act('close_dialogs');await browser.eval('window.clonoloom.saveNow()');
 const saved=await browser.eval("(async()=>{const {serializeProject}=await import('./lib/project.js');return serializeProject(window.clonoloom.store.project)})()");
 mkdirSync(output);cpSync(data,join(output,'data'),{recursive:true});cpSync(evidence,join(output,'evidence'),{recursive:true});writeFileSync(join(output,'original-project.json'),saved);
 const portable=parseProject(saved);for(const d of portable.datasets)for(const f of [...d.sources,...(d.metadata??[])]){assert(f.path.startsWith(data+'/'));f.path='data/'+relative(data,f.path).split('\\').join('/');}
 writeFileSync(join(output,'project.json'),serializeProject(portable));
 const catalog=JSON.parse(await native('/api/examples')),reference=catalog.examples.find(e=>e.name==='simulated-bcell-immunization');
 writeFileSync(join(output,'reference.json'),JSON.stringify({name:reference.name,source:reference.source,license:reference.license,citation:reference.citation}));cpSync(join(examples,reference.name,'reference'),join(output,'references',reference.name,'reference'),{recursive:true});
 for(const a of portable.artifacts){const src=join(library,'artifacts',a.id),dst=join(output,'artifacts',a.id);mkdirSync(dst,{recursive:true});for(const file of readdirSync(src)){const bytes=readFileSync(join(src,file));writeFileSync(join(dst,file==='manifest.json'?file:file+'.gz'),file==='manifest.json'?bytes:gzipSync(bytes));}}
 const inventory=[];function walk(dir){for(const e of readdirSync(dir,{withFileTypes:true})){const p=join(dir,e.name);if(e.isDirectory())walk(p);else inventory.push({path:relative(output,p).split('\\').join('/'),sha256:hash(readFileSync(p))});}}walk(output);
 writeFileSync(join(output,'study.json'),JSON.stringify({format:'clonoloom-suite-study',version:1,software:h.version,bcellDataset:ds.id,queryDataset:query.id,inspectors,files:inventory.sort((a,b)=>a.path.localeCompare(b.path)),limits:'Separate artificial studies, not pooled subjects. No live repository or public sequence search. Saved-byte integrity, source recomputation and conditional review are distinct; no new certificate replay support.'},null,2)+'\n');
 console.log(`0.9.0 suite study: ${portable.datasets.length} separate datasets, ${portable.artifacts.length} saved parents, exact metadata/native reviews, bounded likelihood, candidate FASTA and offline captured public overlap.`);
}finally{await browser?.close();if(host&&host.exitCode===null&&host.signalCode===null){const stopped=once(host,'exit');host.kill();await Promise.race([stopped,sleep(5000)]);if(host.exitCode===null&&host.signalCode===null){host.kill('SIGKILL');await stopped;}}rmSync(temp,{recursive:true,force:true});}
