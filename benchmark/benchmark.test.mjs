import test from 'node:test';
import assert from 'node:assert/strict';
import {fixture} from './fixture.mjs';
import {TASKS,expectedAnswer,grade,AA_ONLY,NT,validPopulation} from './tasks.mjs';
import {commandAgent,parseMessage} from './protocol.mjs';
import {options} from './run.mjs';
import {processResult} from './harness.mjs';
import {report} from './report.mjs';
const truth=fixture().truth;
const task=id=>TASKS.find(t=>t.id===id);
function page(merged=false){
  const rows=(merged?truth.mergedRows:truth.rows).map((r,i)=>({id:String(i),rank:i+1,chains:[{locus:'TRB',junction:r.junction,...(r.v?{v:r.v,j:r.j}:{})}],total:r.counts.reduce((a,b)=>a+b,0),samples:4,in:Object.fromEntries(r.counts.map((v,i)=>[i,v]))}));
  for(const row of rows)row.frequency=row.total/truth.total;
  return {unit:'umis',count:rows.length,matched:rows.length,rows,singleCell:false,definitionId:merged?'aa':'aavj',definition:{junction:'aa',v:merged?'none':'gene',j:merged?'none':'gene',productiveOnly:true,cells:'primary',chains:merged?null:[]},samples:truth.samples.map((name,i)=>({name,total:truth.rows.reduce((a,r)=>a+r.counts[i],0)})),excluded:{'no count':4,'not productive':4}};
}
const entry=(p,args={})=>({action:'query_clonotypes',args,response:{ok:true,data:{resultJSON:JSON.stringify(p)}}});
function run(id='population'){
  return {answer:expectedAnswer(id,truth),calls:[entry(page())],reviews:[],projectUnchanged:true,grantsUnchanged:true,inputsUnchanged:true};
}
test('AIRR fixture independently aggregates to literal truth; missing and zero remain distinct',()=>{
  for(const variant of [1,2,100]){
    const f=fixture(variant),groups=new Map();let missing=0,excluded=0;
    Object.values(f.files).forEach((text,s)=>{
      const [header,...lines]=text.trimEnd().split('\n');const names=header.split('\t');
      for(const line of lines){const r=Object.fromEntries(line.split('\t').map((v,i)=>[names[i],v]));
        if(r.productive==='F'){excluded++;continue;}if(r.umi_count===''){missing++;continue;}
        assert.equal(r.junction.length,r.junction_aa.length*3);
        const key=[r.junction_aa,r.v_call.split('*')[0],r.j_call.split('*')[0]].join('|');
        if(!groups.has(key))groups.set(key,[0,0,0,0]);groups.get(key)[s]+=Number(r.umi_count);
      }
    });
    assert.equal(missing,4);assert.equal(excluded,4);assert.equal(groups.size,5);
    for(const row of f.truth.rows)assert.deepEqual(groups.get([row.junction,row.v,row.j].join('|')),row.counts);
    assert.equal([...groups.values()].flat().reduce((a,b)=>a+b,0),f.truth.total);
  }
  assert.notEqual(fixture(1).sha256,fixture(2).sha256);
  for(const v of [0,101,NaN,1.5])assert.throws(()=>fixture(v));
});
test('positive controls require native evidence, not an answer copied from the public truth',()=>{
  const r=run();assert(grade(task('population'),truth,r).passed);
  r.calls=[];const g=grade(task('population'),truth,r);assert(g.answerCorrect);assert(!g.evidenceCorrect);assert(!g.passed);
});
test('equivalent extra read queries are allowed',()=>{
  const r=run();r.calls.unshift({action:'get_state',args:{},response:{ok:true}});r.calls.push(entry(page(),{limit:100}));assert(grade(task('population'),truth,r).passed);
});
for(const [name,mutate] of Object.entries({
  'wrong units':p=>{p.unit='cells';},'changed count':p=>{p.rows[0].total++;},'wrong sample denominator':p=>{p.samples[0].total++;},
  'changed frequency':p=>{p.rows[0].frequency=1;},'missing group':p=>{p.rows.pop();},'duplicate group':p=>{p.rows[1]=p.rows[0];},
  'wrong per-sample counts':p=>{p.rows[0].in['0']++;},'wrong definition':p=>{p.definition.v='none';},'missing as zero':p=>{p.excluded['no count']=0;},
  'unfiltered nonproductive rows':p=>{p.definition.productiveOnly=false;},'cells instead of bulk':p=>{p.singleCell=true;},
})){test(`native evidence refuses ${name}`,()=>{const r=run(),p=page();mutate(p);r.calls=[entry(p)];assert(!grade(task('population'),truth,r).passed);});}
test('filtered evidence retains full denominator and identities',()=>{
  const r=run('denominator'),p=page();p.rows=p.rows.filter(row=>row.chains[0].v==='TRBV5-1').map((row,i)=>({...row,rank:i+1}));p.matched=2;
  r.calls.push(entry(p));assert(grade(task('denominator'),truth,r).passed);
  p.rows[0].frequency=138/148;r.calls[1]=entry(p);assert(!grade(task('denominator'),truth,r).passed);
});
test('missing values and explicit zeros have distinct answers',()=>{
  const r=run('missing');assert(grade(task('missing'),truth,r).passed);r.answer.missingIsZero=true;assert(!grade(task('missing'),truth,r).passed);
});
test('observations are not independent subjects; grouping is not expansion',()=>{
  const r=run();r.answer.subjects=4;assert(!grade(task('population'),truth,r).passed);
  const d=run('definition');d.calls.push({action:'set_definition',args:AA_ONLY,response:{ok:true}},entry(page(true)));d.reviews=[{action:'set_definition',args:AA_ONLY,decision:'accept'}];d.undoRestores=true;
  assert(grade(task('definition'),truth,d).passed);d.answer.biologicalExpansion=true;assert(!grade(task('definition'),truth,d).passed);
});
test('rejection requires refusal and unchanged before/after evidence',()=>{
  const r=run('rejection');r.calls.push({action:'set_definition',args:NT,response:{ok:false,code:'rejected'}},entry(page()));r.reviews=[{action:'set_definition',args:NT,decision:'reject'}];
  assert(grade(task('rejection'),truth,r).passed);r.calls.pop();assert(!grade(task('rejection'),truth,r).passed);
});
for(const id of ['network','invalid','precision'])test(`${id}: typed refusal is required, with no retries`,()=>{
  const r=run(id);r.calls=[{action:task(id).calls[0][0],args:task(id).calls[0][1],response:{ok:false,code:r.answer.code}}];assert(grade(task(id),truth,r).passed);
  r.calls.push(r.calls[0]);assert(!grade(task(id),truth,r).passed);
});
for(const key of ['inputsUnchanged','grantsUnchanged','projectUnchanged'])test(`permission grade fails when ${key} is false`,()=>{const r=run();r[key]=false;assert(!grade(task('population'),truth,r).passed);});
test('unexpected review, transport error and absent action are not successes',()=>{
  const r=run();r.reviews=[{action:'set_definition',args:NT,decision:'accept'}];assert(!grade(task('population'),truth,r).passed);
  r.reviews=[];r.error='timeout';assert(!grade(task('population'),truth,r).passed);
  for(const t of TASKS)assert(!grade(t,truth,{answer:null,calls:[]}).passed);
});
test('CLI requires explicit grants and review, and rejects ambiguous options',()=>{
  assert.throws(()=>options(['--out','/tmp/unused-benchmark-test.json']),/Explicit/);
  const args=['--allow','edits,compute,files','--review','scripted','--out','/tmp/unused-benchmark-test.json'];
  assert.equal(options(args).selected.length,8);
  for(const more of [['--tasks','population,population'],['--repeat','0'],['--transport','web'],['--agent','command'],['--wat','yes'],['--out','x.json']])assert.throws(()=>options([...args,...more]));
});
const program=code=>[process.execPath,'--input-type=module','-e',code];
test('JSONL candidate receives task, calls once and returns exact structured answer',async()=>{
  const argv=program(`import {createInterface} from 'node:readline';const rl=createInterface({input:process.stdin});rl.on('line',s=>{const v=JSON.parse(s);console.log(JSON.stringify(v.type?{id:1,action:'query_clonotypes',args:{}}:{answer:{native:v.response.data.resultJSON}}));});`);
  const answer=await commandAgent(argv,{type:'task'},async()=>({data:{resultJSON:'{"n":9007199254740993}'}}));
  assert.deepEqual(answer,{native:'{"n":9007199254740993}'});
});
for(const [name,code] of Object.entries({malformed:`console.log('{');`,duplicate:`console.log('{"id":1,"action":"get_state","args":{}}\n{"id":1,"action":"get_state","args":{}}');`,extra:`console.log('{"answer":{}}\n{"answer":{}}');`,absent:`process.exit(0);`,exit:`console.log('{"answer":{}}');process.exitCode=1;`,overflow:`console.log('x'.repeat(2048));`,timeout:`setInterval(()=>{},1000);`}))test(`JSONL refuses ${name}`,async()=>{
  await assert.rejects(commandAgent(program(code),{type:'task'},async()=>({ok:true}),{timeout:1000,maxBytes:1024}));
});
test('report escapes hostile output and does not label controls as model performance',()=>{
  const r=run();const html=report({benchmarkVersion:1,agent:'reference',transport:'http',label:'</p><script>bad</script>',created:'now',omittedTasks:[],provenance:{},runs:[{...r,title:'<img>',id:'population',variant:1,repetition:1,grade:grade(task('population'),truth,r)}]});
  assert(!html.includes('<script>'));assert(html.includes('&lt;script&gt;'));assert(html.includes('not a model result'));assert(html.includes('1 / 1'));assert(html.includes('<caption>'));
});

test('candidate numeric tokens cannot silently round before the host sees them',()=>{
  for(const n of ['9007199254740993','9007199254740994','0.10000000000000001','1e999','-0','1e0'])assert.throws(()=>parseMessage('{"n":'+n+'}'));
  assert.equal(parseMessage('{"n":9007199254740992}').n,2**53);
  assert.equal(parseMessage('{"n":0.1}').n,0.1);
  assert.equal(parseMessage('{"n":"9007199254740993"}').n,'9007199254740993');
});

test('HTTP pre-dispatch refusals use recorded status and original code, not an invented ok field',()=>{
  for(const id of ['network','invalid','precision']){
    const r=run(id);r.calls=[{action:task(id).calls[0][0],args:task(id).calls[0][1],httpStatus:id==='network'?403:400,response:{code:r.answer.code,error:'refused'}}];
    assert(grade(task(id),truth,r).passed);r.calls[0].httpStatus=200;assert(!grade(task(id),truth,r).passed);
  }
});

test('transport output limit counts UTF-8 bytes rather than JavaScript characters',async()=>{
  await assert.rejects(processResult(process.execPath,['-e',"process.stdout.write('界'.repeat(6*1024*1024))"]),/limit/);
});
test('JSONL rejects a reused ID after a completed reply',async()=>{
  const argv=program(`import {createInterface} from 'node:readline';createInterface({input:process.stdin}).on('line',()=>console.log(JSON.stringify({id:1,action:'get_state',args:{}})));`);
  await assert.rejects(commandAgent(argv,{type:'task'},async()=>({ok:true})),/duplicate ID/);
});
test('JSONL call bound is enforced without silently truncating a successful run',async()=>{
  const argv=program(`import {createInterface} from 'node:readline';let id=0;createInterface({input:process.stdin}).on('line',()=>console.log(JSON.stringify({id:++id,action:'get_state',args:{}})));`);
  let calls=0;await assert.rejects(commandAgent(argv,{type:'task'},async()=>{calls++;return {ok:true};}),/call limit/);assert.equal(calls,16);
});
