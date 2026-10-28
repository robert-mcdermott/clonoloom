import test from 'node:test';
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {checkReport} from './report-check.js';
const bytes=(s)=>new TextEncoder().encode(s),digest=(b)=>createHash('sha256').update(b).digest('hex');
function fixture() {
  const files=[bytes('exact archived bytes'),bytes('{"n":9007199254740993,"missing":null}'),bytes('geometry'),bytes('style'),bytes('<h1>Study</h1>')];
  const record={format:'clonoloom-html-report',version:1,scope:'embedded-report-bytes',results:[{artifact:'a-test',kind:'batch-qc',certificate:'a-test.certificate.clz',result:'a-test.result.json',fingerprint:'a'.repeat(64)}],files:['a-test.certificate.clz','a-test.result.json','geometry.js','report.css','report.html'].map((path,i)=>({path,size:files[i].length,sha256:digest(files[i])}))};
  return {record,files};
}
test('offline checker retains exact bytes, including unsafe scientific integers',async()=>{
  const {record,files}=fixture(),raw=bytes(JSON.stringify(record)),sha=digest(raw);
  const checked=await checkReport(raw,files,sha,sha,digest);
  assert.equal(checked.verdict,'integrity-match');assert.equal(checked.recomputed,false);assert.equal(checked.filesChecked,5);
  assert.equal(new TextDecoder().decode(files[1]),'{"n":9007199254740993,"missing":null}');
});
test('offline checker refuses changed content, evidence, inventory and external fingerprints',async()=>{
  const {record,files}=fixture(),raw=bytes(JSON.stringify(record)),sha=digest(raw);
  for(let i=0;i<files.length;i++){const changed=files.map((f)=>f.slice());changed[i][0]^=1;await assert.rejects(checkReport(raw,changed,sha,'',digest),/differ/);}
  await assert.rejects(checkReport(raw,files,sha,'b'.repeat(64),digest),/fingerprint differs/);
  await assert.rejects(checkReport(raw,files,sha,'bad',digest),/64 lowercase/);
  await assert.rejects(checkReport(raw,files.slice(1),sha,'',digest),/member set/);
  const changed=raw.slice();changed[0]^=1;await assert.rejects(checkReport(changed,files,sha,'',digest),/fingerprint differs/);
});
test('offline checker rejects internally rehashed unsupported contracts and missing/duplicate members',async()=>{
  for(const edit of [(r)=>r.version=2,(r)=>r.scope='replay',(r)=>r.results[0].kind='unknown',(r)=>r.files[0].size=-1,(r)=>r.files[0].path='../escape',(r)=>r.files[1].path=r.files[0].path,(r)=>r.results.push(r.results[0])]) {
    const {record,files}=fixture();edit(record);const raw=bytes(JSON.stringify(record));await assert.rejects(checkReport(raw,files,digest(raw),'',digest));
  }
});
