// Fixed local controls only. No model service, credentials or paid agent invocation.
// node validation/agent-benchmark.mjs /path/to/clonoloom /new/output/directory [--capture]
import assert from 'node:assert/strict';
import {mkdirSync,writeFileSync} from 'node:fs';
import {join,resolve} from 'node:path';
import {main} from '../benchmark/run.mjs';
import {digest,fixture} from '../benchmark/fixture.mjs';
import {TASKS,AA_ONLY,expectedAnswer,grade} from '../benchmark/tasks.mjs';
import {withHarness} from '../benchmark/harness.mjs';
import {checkReport} from './agent-benchmark-report.mjs';
const root=resolve(import.meta.dirname,'..');
const binary=resolve(process.argv[2]),out=resolve(process.argv[3]);
mkdirSync(out,{recursive:false});
const common=['--binary',binary,'--allow','edits,compute,files','--review','scripted'];
const results=[];
for(const transport of ['http','mcp','python','r']){
  const result=await main([...common,'--transport',transport,'--out',join(out,transport+'.json')]);
  assert.equal(result.runs.length,8);assert(result.runs.every(r=>r.grade.passed),transport+' reference failed');
  results.push(result);
}
// Independent arithmetic grading above, byte parity below: these are different claims.
for(let t=1;t<results.length;t++)for(let i=0;i<8;i++){
  const hashes=r=>r.calls.filter(c=>c.nativeSHA256).map(c=>c.nativeSHA256);
  assert.deepEqual(hashes(results[t].runs[i]),hashes(results[0].runs[i]),'Native transport bytes differ');
}
for(const result of results)for(const run of result.runs)for(const call of run.calls){
  if(call.raw)assert.equal(digest(call.raw),call.rawSHA256);
  if(call.nativeSHA256)assert.equal(digest(call.response.data.resultJSON),call.nativeSHA256);
}
const none=await main([...common,'--agent','none','--out',join(out,'none.json')]);
assert.equal(none.runs.length,8);assert(none.runs.every(r=>!r.grade.passed&&!r.grade.answerCorrect&&!r.grade.evidenceCorrect&&!r.error));
const argvFile=join(out,'scripted-control-argv.json');
writeFileSync(argvFile,JSON.stringify([process.execPath,join(root,'benchmark/reference-agent.mjs')]),{flag:'wx'});
const command=await main([...common,'--agent','command','--command-file',argvFile,'--label','Scripted JSONL reference control (not a model)','--transport','mcp','--tasks','population,definition,network','--out',join(out,'command.json')]);
assert(command.runs.every(r=>r.grade.passed));
const repeated=await main([...common,'--variant','2','--repeat','2','--tasks','population,denominator,definition','--out',join(out,'repeated.json')]);
assert.equal(repeated.runs.length,6);assert(repeated.runs.every(r=>r.grade.passed));
for(let i=0;i<3;i++)assert.deepEqual(repeated.runs[i].calls.map(c=>c.nativeSHA256),repeated.runs[i+3].calls.map(c=>c.nativeSHA256));
// A correct answer and native evidence cannot compensate for violating fixture policy.
const negativeTask=TASKS.find(t=>t.id==='population'),negativeFixture=fixture();
const negative=await withHarness({binary,transport:'http',fixture:negativeFixture,task:negativeTask},async call=>{
  assert.equal((await call('export_project_archive',{})).code,'benchmark-policy');
  assert.equal((await call('set_definition',AA_ONLY)).code,'rejected');
  await call('query_clonotypes',{limit:1000});
  return expectedAnswer('population',negativeFixture.truth);
});
assert(!negative.error,negative.error);assert(negative.projectUnchanged);assert(negative.inputsUnchanged);
const negativeGrade=grade(negativeTask,negativeFixture.truth,negative);
assert(negativeGrade.answerCorrect&&negativeGrade.evidenceCorrect&&!negativeGrade.permissionCorrect&&!negativeGrade.passed);
writeFileSync(join(out,'permission-negative.json'),JSON.stringify({...negative,grade:negativeGrade},null,2)+'\n',{flag:'wx',mode:0o600});
await checkReport(join(out,'http.json'),{capture:process.argv.includes('--capture')});
console.log('Agent benchmark: 32/32 transport controls, 0/8 no-action controls, 3/3 scripted JSONL controls, 6/6 repeated/scaled controls; exact native parity and four accessible report layouts; unexpected proposal and out-of-policy action refused. No model evaluated.');
