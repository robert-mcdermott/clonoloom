// Actual sibling receiver exercise. No sequence reaches a public search service:
// CDP intercepts every search request before Proteoscope's server can see it.
// node validation/proteoscope-handoff.mjs /path/clonoloom /path/proteoscope [output-dir]
import assert from 'node:assert/strict';
import {spawn} from 'node:child_process';
import {createServer} from 'node:net';
import {mkdtempSync,mkdirSync,rmSync,readFileSync,writeFileSync} from 'node:fs';
import {join,resolve} from 'node:path';
import {tmpdir} from 'node:os';
import {launch,sleep} from './cdp.mjs';

const [clono,proteo,output]=process.argv.slice(2);
assert(clono&&proteo,'Give ClonoLoom and Proteoscope 0.10.0 binaries');
const temp=mkdtempSync(join(tmpdir(),'clonoloom-proteoscope-')),children=[];
let browser;
async function port(){const s=createServer();await new Promise(r=>s.listen(0,'127.0.0.1',r));const p=s.address().port;await new Promise(r=>s.close(r));return p;}
async function start(binary,args,p){const c=spawn(resolve(binary),args,{cwd:temp,stdio:['ignore','pipe','pipe']});children.push(c);let log='';c.stdout.on('data',b=>log+=b);c.stderr.on('data',b=>log+=b);const url=`http://127.0.0.1:${p}/`;for(let i=0;i<150;i++){if(c.exitCode!==null)throw Error(log);try{if((await fetch(url,{signal:AbortSignal.timeout(500)})).ok)return url;}catch{}await sleep(100);}throw Error(log||'Startup timeout');}
async function post(url,operation,body){const r=await fetch(url+operation,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});const raw=await r.text();assert.equal(r.status,200,raw);return {raw,data:JSON.parse(raw)};}
try{
 const cp=await port(),pp=await port();
 const cu=await start(clono,['--window','none','--port',String(cp),'--data-dir',join(temp,'library'),'--cache-dir',join(temp,'cache')],cp);
 const pu=await start(proteo,['--no-open','--remote-control','--port',String(pp),'--cache-dir',join(temp,'proteoscope-cache')],pp);
 browser=await launch({width:1400,height:900});
 const queries=[],unexpected=[];
 browser.on('Fetch.requestPaused',async({requestId,request})=>{
  try{
   const u=new URL(request.url);
   if(u.origin!==new URL(pu).origin){unexpected.push(request.url);await browser.send('Fetch.failRequest',{requestId,errorReason:'BlockedByClient'});return;}
   if(u.pathname.startsWith('/api/search/')){
    if(u.pathname!=='/api/search/sequence'){unexpected.push(request.url);await browser.send('Fetch.failRequest',{requestId,errorReason:'BlockedByClient'});return;}
    queries.push({sequence:u.searchParams.get('seq'),keys:[...u.searchParams.keys()]});
    const answer={query:u.searchParams.get('seq'),total:0,entries:[]};
    await browser.send('Fetch.fulfillRequest',{requestId,responseCode:200,responseHeaders:[{name:'Content-Type',value:'application/json'}],body:Buffer.from(JSON.stringify(answer)).toString('base64')});return;
   }
   await browser.send('Fetch.continueRequest',{requestId});
  }catch(e){unexpected.push(e.message);}
 });
 await browser.send('Fetch.enable',{patterns:[{urlPattern:'*',requestStage:'Request'}]});
 await browser.goto(pu+'?renderer=canvas');
 for(let i=0;i<150&&!await browser.eval('Boolean(globalThis.proteoscope?.state?.startup?.remoteControl)');i++)await sleep(100);
 assert.equal(await browser.eval('proteoscope.state.startup.version'),'0.10.0');
 const source=resolve('testdata/proteoscope/simulated-airr.tsv');
 const ds=(await post(cu,'api/datasets/open',{paths:[source]})).data;
 const lines=readFileSync(source,'utf8').trimEnd().split('\n'),header=lines[0].split('\t');
 for(const row of [0,1]){
  const values=lines[row+1].split('\t'),expected=values[header.indexOf('sequence_aa')];
  const review=(await post(cu,'api/proteoscope/review',{dataset:ds.id,row,port:pp})).data;
  assert.equal(review.evidence.sequence,expected);assert.equal(review.evidence.destination.ready,true);assert.equal(queries.length,row);
  const request={dataset:ds.id,row,port:pp,review:review.review,fingerprint:review.fingerprint,acceptLocal:true,acceptPublic:true};
  const evidence=await post(cu,'api/proteoscope/export',request);
  const receipt=await post(cu,'api/proteoscope/send',request);
  assert.equal(receipt.data.response.ok,true);assert.equal(queries.length,row+1);
  assert.deepEqual(queries[row],{sequence:expected,keys:['seq']});
  assert.deepEqual(await browser.eval('({kind:proteoscope.state.discovery.kind,query:proteoscope.state.discovery.query,errors:proteoscope.state.discovery.errors})'),{kind:'sequence',query:expected,errors:[]});
  if(output){const dir=join(output,`receiver-${row}`);mkdirSync(dir,{recursive:true});writeFileSync(join(dir,'handoff.json'),evidence.raw);writeFileSync(join(dir,'receipt.json'),receipt.raw);writeFileSync(join(dir,'command.json'),JSON.stringify({command:'search '+queries[row].sequence}));}
 }
 assert.deepEqual(unexpected,[]);
 console.log('Proteoscope 0.10.0: two exact source sequences reached the real command parser, browser receiver and sequence-search boundary; public requests intercepted. No identifier disclosure, normalization or text-search fallback.');
}finally{
 if(browser)await browser.close();
 for(const child of children)child.kill();
 await sleep(300);rmSync(temp,{recursive:true,force:true});
}
