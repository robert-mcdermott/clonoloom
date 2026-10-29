// Check the saved worked-study report without source files or a running ClonoLoom host.
// node validation/record-study.mjs [fixture-directory]
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {join,resolve} from 'node:path';
import {pathToFileURL} from 'node:url';
import {launch,sleep} from './cdp.mjs';
const folder=resolve(process.argv[2]??'validation/projects/0.8.0');
for(const width of [1280,390]){
 const browser=await launch({width,height:1000}),requests=[];
 try{
  await browser.send('Network.enable');browser.on('Network.requestWillBeSent',e=>{if(/^https?:/.test(e.request.url))requests.push(e.request.url)});
  for(const theme of ['light','dark']){
   await browser.theme(theme==='dark');await browser.goto(pathToFileURL(join(folder,'evidence/report.html')).href,300);
   for(let i=0;i<100&&await browser.eval("document.querySelector('#status')?.dataset.verdict")!=='integrity-match';i++)await sleep(50);
   assert.equal(await browser.eval("document.querySelector('#status').dataset.verdict"),'integrity-match');
   assert.equal(await browser.eval("document.querySelectorAll('#report svg').length"),6);
   assert(await browser.eval("document.querySelector('#report').textContent.includes('unscored')"));
   await browser.eval(`(0,eval)(${JSON.stringify(readFileSync(resolve('validation/cache/axe-core/axe.min.js'),'utf8'))});true`);
   assert.deepEqual(await browser.eval(`(async()=>{const r=await axe.run(document,{runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21a','wcag21aa']}});return r.violations.map(v=>({id:v.id,nodes:v.nodes.map(n=>n.target)}))})()`),[]);
   assert(await browser.eval('document.documentElement.scrollWidth<=innerWidth+1'));
   await browser.eval(`document.querySelector('#expected').value='${'0'.repeat(64)}';document.querySelector('#check').click();true`);
   for(let i=0;i<100&&await browser.eval("document.querySelector('#status').dataset.verdict")!=='differs';i++)await sleep(50);
   assert.equal(await browser.eval("document.querySelector('#status').dataset.verdict"),'differs');
   assert.equal(await browser.eval("document.querySelector('#report').children.length"),0);
   console.log(`Worked report: ${theme} ${width}px integrity, six native figures, unavailable evidence, fingerprint refusal and accessibility passed`);
  }
  assert.deepEqual(requests,[],'offline report made network requests');
 }finally{await browser.close();}
}
