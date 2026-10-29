// Own temporary host, normal window and exact fixture proposal review; no approval API.
import {spawn} from 'node:child_process';
import {createServer} from 'node:net';
import {existsSync,mkdtempSync,mkdirSync,readFileSync,writeFileSync,rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join,resolve} from 'node:path';
import {isDeepStrictEqual as equal} from 'node:util';
import {launch,sleep} from '../validation/cdp.mjs';
import {digest} from './fixture.mjs';
import {AA_ONLY,NT} from './tasks.mjs';
import {MAX_CALLS} from './protocol.mjs';
export const ROOT=resolve(import.meta.dirname,'..');
export const ACTIONS=['get_state','query_clonotypes','set_definition','open_example'];
export const GRANTS='edits,compute,files';
async function freePort() {
  const server=createServer();
  await new Promise((resolve,reject)=>{server.once('error',reject);server.listen(0,'127.0.0.1',resolve);});
  const port=server.address().port;
  await new Promise((resolve,reject)=>server.close(e=>e?reject(e):resolve()));
  return String(port);
}
const CLIENT='Benchmark candidate';

export function processResult(command,args,{env={},input='',timeout=60000}={}) {
  return new Promise((resolve,reject)=>{
    const child=spawn(command,args,{cwd:ROOT,env:{...process.env,...env},stdio:['pipe','pipe','pipe'],shell:false});
    let output='',error='',size=0,finished=false;
    const done=(failure)=>{if(finished)return;finished=true;clearTimeout(timer);if(child.exitCode===null)child.kill('SIGKILL');failure?reject(failure):resolve(output);};
    const timer=setTimeout(()=>done(new Error(`${command} deadline exceeded`)),timeout);
    child.on('error',done);child.stdin.on('error',done);
    child.stdout.setEncoding('utf8');child.stderr.setEncoding('utf8');
    child.stdout.on('data',c=>{output+=c;size+=Buffer.byteLength(c);if(size>16*1024*1024)done(new Error('Transport output limit exceeded'));});
    child.stderr.on('data',c=>{error+=c;size+=Buffer.byteLength(c);if(size>16*1024*1024)done(new Error('Transport output limit exceeded'));});
    child.on('close',code=>done(code===0?null:new Error(`${command} exited ${code}: ${error.slice(0,2000)}`)));
    child.stdin.end(input);
  });
}
async function boundedText(response) {
  let size=0;const chunks=[];
  for await(const chunk of response.body){size+=chunk.length;if(size>16*1024*1024)throw new Error('Response exceeds 16 MiB');chunks.push(Buffer.from(chunk));}
  return Buffer.concat(chunks).toString('utf8');
}
export async function withHarness({binary,transport,fixture,task,rLibrary},candidate) {
  const port=await freePort();
  const temp=mkdtempSync(join(tmpdir(),'clonoloom-benchmark-'));
  const library=join(temp,'library');
  let browser,connection,catalog,stderr='',hostError;
  const run={calls:[],reviews:[],setupReviews:[],answer:null};
  const pending=new Map();let next=0,buffer='';
  const host=spawn(binary,[...(transport==='mcp'?['mcp']:['--remote-control']),'--allow',GRANTS,'--offline','--window','none','--port',port,'--data-dir',library,'--cache-dir',join(temp,'cache')],{stdio:['pipe','pipe','pipe']});
  const exited=new Promise(resolve=>host.once('close',resolve));
  host.on('error',e=>{hostError=e;});
  host.stdin.on('error',e=>{hostError=e;});
  host.stderr.on('data',c=>{stderr=(stderr+c).slice(-4000);});
  host.stdout.setEncoding('utf8');
  host.stdout.on('data',c=>{
    if(transport!=='mcp'){stderr=(stderr+c).slice(-4000);return;}
    buffer+=c;
    if(Buffer.byteLength(buffer)>16*1024*1024){hostError=new Error('MCP response exceeds limit');buffer='';for(const p of pending.values())p.reject(hostError);pending.clear();host.kill('SIGKILL');return;}
    while(buffer.includes('\n')){
      const i=buffer.indexOf('\n'),line=buffer.slice(0,i);buffer=buffer.slice(i+1);
      try{const msg=JSON.parse(line);if(msg.jsonrpc!=='2.0')throw new Error('Invalid MCP response');
        if(msg.id!==undefined){const p=pending.get(msg.id);if(!p)throw new Error('Unexpected MCP ID');pending.delete(msg.id);p.resolve({msg,raw:line+'\n'});}
        else if(msg.method!=='notifications/progress')throw new Error('Unexpected MCP notification');
      }catch(e){hostError=e;for(const p of pending.values())p.reject(e);pending.clear();}
    }
  });
  function mcp(method,params={}) {
    const id=++next;
    return new Promise((resolve,reject)=>{
      const timer=setTimeout(()=>{pending.delete(id);reject(new Error('MCP deadline exceeded'));},30000);
      pending.set(id,{resolve:x=>{clearTimeout(timer);resolve(x);},reject:e=>{clearTimeout(timer);reject(e);}});
      host.stdin.write(JSON.stringify({jsonrpc:'2.0',id,method,params:{...params,_meta:{'io.modelcontextprotocol/protocolVersion':'2026-07-28','io.modelcontextprotocol/clientCapabilities':{},'io.modelcontextprotocol/clientInfo':{name:CLIENT,version:'1'}}}})+'\n');
    });
  }
  async function idle() {
    const deadline=Date.now()+30000;
    while(Date.now()<deadline){
      if(hostError)throw hostError;
      if(await browser.eval('Boolean(window.clonoloom && !window.clonoloom.jobs.running().length && !window.clonoloom.store.state.busy.size)'))return;
      await sleep(50);
    }throw new Error('Window did not become idle');
  }
  const project=()=>browser.eval('JSON.stringify(window.clonoloom.store.project)');
  const tools=async()=>JSON.parse(await boundedText(await fetch(connection.url+'/api/remote/tools',{signal:AbortSignal.timeout(30000)})));
  async function send(action,args,mode) {
    if(mode==='mcp'){
      const {msg,raw}=await mcp('tools/call',{name:action,arguments:args});
      if(msg.error||!msg.result?.structuredContent)throw new Error('MCP transport error: '+raw);
      if(!equal(JSON.parse(msg.result.content[0].text),msg.result.structuredContent))throw new Error('MCP structured/text mismatch');
      return {response:msg.result.structuredContent,raw};
    }
    let raw,httpStatus;
    if(mode==='python')raw=await processResult('uv',['run','--no-project',join(ROOT,'benchmark/client.py'),library],{env:{PYTHONPATH:join(ROOT,'clients/python/src')},input:JSON.stringify({action,args})});
    else if(mode==='r')raw=await processResult('Rscript',[join(ROOT,'benchmark/client.R'),library],{env:{R_LIBS:rLibrary},input:JSON.stringify({action,args})});
    else {const response=await fetch(connection.url+'/api/remote/action',{method:'POST',headers:{'Content-Type':'application/json','X-ClonoLoom-Token':connection.token},body:JSON.stringify({action,args,client:CLIENT}),signal:AbortSignal.timeout(30000)});httpStatus=response.status;raw=await boundedText(response);}
    return {response:JSON.parse(raw),raw,...(httpStatus?{httpStatus}:{})};
  }
  async function reviewed(action,args,mode,setup=false) {
    await idle();let done=false;
    const result=send(action,args,mode).finally(()=>{done=true;});result.catch(()=>{});
    while(!done){
      const shown=await browser.eval(`(()=>{const d=document.querySelector('[data-automation-proposal]');return d?{text:d.querySelector('p').textContent,args:JSON.parse(d.querySelector('pre').textContent)}:null})()`);
      if(shown){
        const title=catalog.tools.find(t=>t.name===action)?.title;
        const approvedArgs=task.id==='definition'?AA_ONLY:NT;
        const matches=equal(shown.args,args)&&shown.text.startsWith(`${CLIENT} proposes “${title}” in `);
        const permitted=matches&&(setup||(action==='set_definition'&&task.review&&equal(args,approvedArgs)&&run.reviews.length===0));
        const decision=permitted?(setup?'accept':task.review):'reject';
        if(!permitted)run.violation='Unexpected proposal rejected by fixture reviewer';
        (setup?run.setupReviews:run.reviews).push({action,args,decision});
        await browser.eval(`(()=>{const d=document.querySelector('[data-automation-proposal]');[...d.querySelectorAll('button')].find(b=>b.textContent===${JSON.stringify(decision==='accept'?'Accept proposal':'Reject')}).click();return true})()`);
      }
      await sleep(30);
    }
    return result;
  }
  try {
    mkdirSync(join(temp,'inputs'));
    const paths=Object.entries(fixture.files).map(([name,data])=>{const path=join(temp,'inputs',name);writeFileSync(path,data);return path;});
    for(let i=0;i<300&&!existsSync(join(library,'remote.json'));i++){if(hostError||host.exitCode!==null)throw hostError??new Error(stderr||'Host exited before creating its owned connection file');await sleep(50);}
    connection=JSON.parse(readFileSync(join(library,'remote.json'),'utf8'));
    catalog=await tools();
    if(catalog.api!==2||catalog.catalogVersion!==4||catalog.policyVersion!==1||!equal(catalog.allowed,{read:true,edits:true,compute:true,files:true}))throw new Error('Unexpected catalog or grants');
    run.host={version:catalog.version,api:catalog.api,catalogVersion:catalog.catalogVersion,policyVersion:catalog.policyVersion,allowed:catalog.allowed};
    if(transport==='mcp')await mcp('server/discover');
    browser=await launch();await browser.goto(connection.url);await idle();
    for(const [action,args] of [['new_project',{name:'Artificial benchmark study'}],['open_data',{paths,name:'Benchmark study'}],['annotate_samples',{samples:fixture.annotations}]]){
      const {response}=await reviewed(action,args,'http',true);if(!response.ok)throw new Error('Fixture setup failed: '+JSON.stringify(response));
    }
    await idle();const before=await project();
    const call=async(action,args)=>{
      if(run.calls.length>=MAX_CALLS)throw new Error('Candidate call limit exceeded');
      if(!ACTIONS.includes(action)){run.violation='Action outside fixture policy';const response={ok:false,code:'benchmark-policy'};run.calls.push({action,args,response});return response;}
      const entry={action,args};run.calls.push(entry);
      const result=await reviewed(action,args,transport);Object.assign(entry,result);
      entry.rawSHA256=digest(result.raw);
      if(typeof result.response.data?.resultJSON==='string')entry.nativeSHA256=digest(result.response.data.resultJSON);
      return result.response;
    };
    try{run.answer=await candidate(call,{type:'task',id:task.id,prompt:task.prompt,answerShape:task.shape,tools:catalog.tools.filter(t=>ACTIONS.includes(t.name)),permissions:catalog.allowed,reviewPolicy:task.review?{action:'set_definition',args:task.id==='definition'?AA_ONLY:NT,decision:task.review,maximum:1}:{maximum:0}});}catch(e){run.error=e.message||String(e);}
    await idle();const after=await project();run.projectUnchanged=before===after;
    if(task.id==='definition'&&run.reviews.some(r=>r.decision==='accept')){
      await browser.eval('(()=>{window.clonoloom.store.undo();return true})()');
      run.undoRestores=(await project())===before;
      await browser.eval('(()=>{window.clonoloom.store.redo();return true})()');
      run.undoRestores&&=(await project())===after;
    }
    run.grantsUnchanged=equal((await tools()).allowed,catalog.allowed);
    run.inputsUnchanged=paths.every((p,i)=>readFileSync(p,'utf8')===Object.values(fixture.files)[i]);
    run.state={beforeSHA256:digest(before),afterSHA256:digest(after)};
  }catch(e){run.error=e.message||String(e);}
  finally{
    for(const p of pending.values())p.reject(new Error('Host closed'));pending.clear();
    if(browser)await browser.close();
    host.stdin.destroy();if(host.exitCode===null)host.kill('SIGKILL');
    await Promise.race([exited,sleep(2000)]);
    rmSync(temp,{recursive:true,force:true});
  }
  return run;
}
