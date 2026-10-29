// Local bounded benchmark; no paid model or external program is run by default.
import {existsSync,mkdirSync,mkdtempSync,readFileSync,readdirSync,rmSync,writeFileSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {basename,dirname,join,resolve} from 'node:path';
import {pathToFileURL} from 'node:url';
import {fixture,digest,FIXTURE_VERSION} from './fixture.mjs';
import {TASKS,BENCHMARK_VERSION,grade,reference} from './tasks.mjs';
import {commandAgent} from './protocol.mjs';
import {ROOT,GRANTS,processResult,withHarness} from './harness.mjs';
import {report} from './report.mjs';

export function options(argv) {
  const opts={agent:'reference',transport:'http',variant:1,repeat:1};
  const keys=new Set(['agent','transport','variant','repeat','tasks','out','binary','allow','review','command-file','label']);
  for(let i=0;i<argv.length;i++){
    const key=argv[i].replace(/^--/,'');
    if(!argv[i].startsWith('--')||!keys.has(key)||!argv[i+1]||argv[i+1].startsWith('--')||Object.hasOwn(opts,'seen-'+key))throw new Error('Unknown, missing or duplicate option: '+argv[i]);
    opts[key]=argv[++i];opts['seen-'+key]=true;
  }
  if(opts.allow!==GRANTS||opts.review!=='scripted')throw new Error('Explicit --allow edits,compute,files --review scripted are required for this isolated fixture');
  if(!['reference','none','command'].includes(opts.agent)||!['http','mcp','python','r'].includes(opts.transport))throw new Error('Invalid agent or transport');
  for(const [key,max] of [['variant',100],['repeat',10]]){if(!/^\d+$/.test(String(opts[key]))||+opts[key]<1||+opts[key]>max)throw new Error(`Invalid ${key}`);opts[key]=+opts[key];}
  opts.selected=opts.tasks?opts.tasks.split(','):TASKS.map(t=>t.id);
  if(new Set(opts.selected).size!==opts.selected.length||opts.selected.some(id=>!TASKS.some(t=>t.id===id)))throw new Error('Invalid or duplicate task selection');
  if(!opts.out||!opts.out.endsWith('.json'))throw new Error('--out must name a new .json file');
  opts.out=resolve(opts.out);opts.html=opts.out.slice(0,-5)+'.html';
  if(existsSync(opts.out)||existsSync(opts.html))throw new Error('Output exists; choose new paths');
  if((opts.agent==='command')!==Boolean(opts['command-file']))throw new Error('Only --agent command requires --command-file (trusted JSON argv)');
  return opts;
}
export async function main(argv) {
  const opts=options(argv),temp=mkdtempSync(join(tmpdir(),'clonoloom-benchmark-build-'));
  try{
    const binary=opts.binary?resolve(opts.binary):join(temp,process.platform==='win32'?'clonoloom.exe':'clonoloom');
    if(!opts.binary)await processResult('go',['build','-o',binary,'.'],{timeout:120000});
    let rLibrary;
    if(opts.transport==='r'){rLibrary=join(temp,'r-library');mkdirSync(rLibrary);await processResult('R',['CMD','INSTALL',`--library=${rLibrary}`,join(ROOT,'clients/r')],{timeout:120000});}
    const command=opts.agent==='command'?JSON.parse(readFileSync(opts['command-file'],'utf8')):null;
    const data=fixture(opts.variant);
    const clientFiles={};
    for(const dir of ['clients/python/src','clients/r/R'])for(const name of readdirSync(join(ROOT,dir),{recursive:true}).sort()){
      if(/\.(py|R)$/.test(name))clientFiles[dir+'/'+name]=digest(readFileSync(join(ROOT,dir,name)));
    }
    const transportRuntime=opts.transport==='python'?(await processResult('uv',['run','--no-project','python','--version'])).trim():opts.transport==='r'?(await processResult('Rscript',['-e','cat(R.version.string, as.character(packageVersion("curl")), as.character(packageVersion("jsonlite")), sep="; ")'],{env:{R_LIBS:rLibrary}})).trim():process.version;
    const code=Object.fromEntries(readdirSync(join(ROOT,'benchmark')).filter(n=>/\.(mjs|py|R)$/.test(n)).sort().map(n=>[n,digest(readFileSync(join(ROOT,'benchmark',n)))]));
    const result={format:1,benchmarkVersion:BENCHMARK_VERSION,fixtureVersion:FIXTURE_VERSION,created:new Date().toISOString(),agent:opts.agent,transport:opts.transport,label:opts.label??opts.agent,permissions:{allow:GRANTS,review:'scripted',offline:true},selectedTasks:opts.selected,omittedTasks:TASKS.filter(t=>!opts.selected.includes(t.id)).map(t=>t.id),provenance:{commit:(await processResult('git',['rev-parse','HEAD'])).trim(),dirty:Boolean((await processResult('git',['status','--porcelain'])).trim()),binarySHA256:digest(readFileSync(binary)),benchmarkFiles:code,clientFiles,catalogSHA256:digest(readFileSync(join(ROOT,'clients/tools.json'))),transportRuntime,fixtureSHA256:data.sha256,variant:opts.variant,node:process.version,platform:process.platform,architecture:process.arch,command:command?{program:basename(command[0]),argvSHA256:digest(JSON.stringify(command))}:null},runs:[]};
    for(let repetition=1;repetition<=opts.repeat;repetition++)for(const id of opts.selected){
      const task=TASKS.find(t=>t.id===id),started=Date.now();
      let run;
      try{run=await withHarness({binary,transport:opts.transport,fixture:data,task,rLibrary},async(call,input)=>opts.agent==='none'?null:opts.agent==='reference'?reference(task,call):commandAgent(command,input,call));}
      catch(error){run={answer:null,calls:[],reviews:[],error:error.message||String(error)};}
      const record={id,title:task.title,variant:opts.variant,repetition,durationMs:Date.now()-started,...run,grade:grade(task,data.truth,run)};
      result.runs.push(record);console.log(`${opts.transport} ${opts.agent} ${id} #${repetition}: ${record.grade.passed?'PASS':'FAIL'}${run.error?' · '+run.error:''}`);
    }
    mkdirSync(dirname(opts.out),{recursive:true});
    writeFileSync(opts.out,JSON.stringify(result,null,2)+'\n',{flag:'wx',mode:0o600});
    writeFileSync(opts.html,report(result),{flag:'wx',mode:0o600});
    console.log(`Evidence: ${opts.out}\nReport: ${opts.html}`);
    if(result.runs.some(r=>r.error)||opts.agent==='reference'&&result.runs.some(r=>!r.grade.passed)||opts.agent==='none'&&result.runs.some(r=>r.grade.passed))process.exitCode=1;
    return result;
  }finally{rmSync(temp,{recursive:true,force:true});}
}
if(process.argv[1]&&import.meta.url===pathToFileURL(resolve(process.argv[1])).href)main(process.argv.slice(2)).catch(e=>{console.error(e.message);process.exitCode=1;});
