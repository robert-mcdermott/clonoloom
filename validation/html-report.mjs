// Offline-file checks against host-generated fixtures. No server or source files.
// CLONOLOOM_REPORT_ORACLE_DIR=/tmp/reports go test . -run TestHTMLReportsExactEvidenceAndReplay
// node validation/html-report.mjs /tmp/reports
import {readFileSync,readdirSync,mkdtempSync,writeFileSync,rmSync} from 'node:fs';
import {resolve,join} from 'node:path';
import {pathToFileURL} from 'node:url';
import {tmpdir} from 'node:os';
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {launch,sleep} from './cdp.mjs';

const folder=resolve(process.argv[2]),temp=mkdtempSync(join(tmpdir(),'clonoloom-html-check-'));
const browser=await launch({width:1280,height:1000});
const requests=[];await browser.send('Network.enable');browser.on('Network.requestWillBeSent',(e)=>{if(/^https?:/.test(e.request.url))requests.push(e.request.url);});
async function settled() {
  for(let i=0;i<100;i++){const s=await browser.eval(`document.getElementById('status')?.dataset.verdict`);if(s==='integrity-match'||s==='differs')return s;await sleep(100);}
  throw new Error('Offline checker did not finish');
}
try {
  for(const name of readdirSync(folder).filter((s)=>s.endsWith('.html'))) {
    await browser.goto(pathToFileURL(join(folder,name)).href,100);
    assert.equal(await settled(),'integrity-match',await browser.eval(`document.getElementById('status').textContent`));
    assert.ok(await browser.eval(`document.querySelectorAll('#report table').length>0`));
    assert.equal(await browser.eval(`document.querySelectorAll('#downloads button').length`),2);
    const plots=await browser.eval(`document.querySelectorAll('#report svg').length`);
    if(name==='batch-qc.html')assert.equal(plots,6);
    if(name==='cohort-compare.html')assert.ok(plots>=2);
    if(name==='cohort-counts.html')assert.equal(plots,1);
    assert.equal(await browser.eval(`Boolean(document.querySelector('#report img, #report script, #report iframe'))`),false);
    const fingerprint=await browser.eval(`document.getElementById('fingerprint').textContent`);
    await browser.eval(`document.getElementById('expected').value='${fingerprint}';document.getElementById('check').click();true`);assert.equal(await settled(),'integrity-match');
    await browser.eval(`document.getElementById('expected').value='${'0'.repeat(64)}';document.getElementById('check').click();true`);assert.equal(await settled(),'differs');
    assert.equal(await browser.eval(`document.getElementById('report').children.length`),0);
    // Corrupt a decoded result file; the unchanged inventory must refuse it.
    const original=readFileSync(join(folder,name),'utf8');
    const changed=original.replace(/(id="report-file-1">)([A-Za-z0-9+/=]+)/,(_,prefix,data)=>{const raw=Buffer.from(data,'base64');raw[0]^=1;return prefix+raw.toString('base64');});
    const bad=join(temp,name);writeFileSync(bad,changed);await browser.goto(pathToFileURL(bad).href,100);assert.equal(await settled(),'differs');
    console.log(`${name}: offline integrity, exact fingerprint, figures, changed bytes and hidden failed content pass`);
  }
  assert.deepEqual(requests,[],'Report made external network requests');
  // A matching payload must not conceal changed figure code or styles.
  const source=readFileSync(join(folder,'batch-qc.html'),'utf8');
  for(const variant of ['geometry','style']) {
    let changed=variant==='style'?source.replace('</style>','/* changed */</style>'):source.replace('r="4.5"','r="5.5"');
    if(variant==='geometry') {
      const code=changed.slice(changed.lastIndexOf('<script>')+8,changed.lastIndexOf('</script>'));
      changed=changed.replace(/script-src 'sha256-[^']+'/,`script-src 'sha256-${createHash('sha256').update(code).digest('base64')}'`);
    }
    const path=join(temp,variant+'.html');writeFileSync(path,changed);await browser.goto(pathToFileURL(path).href,100);assert.equal(await settled(),'differs');
  }
  console.log('No network requests from offline reports.');
} finally {await browser.close();rmSync(temp,{recursive:true,force:true});}
