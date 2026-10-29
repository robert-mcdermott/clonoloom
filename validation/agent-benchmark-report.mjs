// Re-render and check a saved control report without rerunning any scientific actions.
// node validation/agent-benchmark-report.mjs /path/to/http.json [--capture]
import assert from 'node:assert/strict';
import {readFileSync,writeFileSync} from 'node:fs';
import {join,resolve} from 'node:path';
import {pathToFileURL} from 'node:url';
import {launch} from './cdp.mjs';
import {report} from '../benchmark/report.mjs';
import {fixture,digest,FIXTURE_VERSION} from '../benchmark/fixture.mjs';
import {TASKS,BENCHMARK_VERSION,grade} from '../benchmark/tasks.mjs';
const root=resolve(import.meta.dirname,'..');
export async function checkReport(jsonFile,{capture=false}={}) {
  const result=JSON.parse(readFileSync(jsonFile,'utf8'));
  assert.equal(result.format,1);assert.equal(result.benchmarkVersion,BENCHMARK_VERSION);assert.equal(result.fixtureVersion,FIXTURE_VERSION);
  for(const run of result.runs){
    const data=fixture(run.variant);assert.equal(data.sha256,result.provenance.fixtureSHA256);
    assert.deepEqual(grade(TASKS.find(t=>t.id===run.id),data.truth,run),run.grade);
    for(const call of run.calls){
      if(call.raw)assert.equal(digest(call.raw),call.rawSHA256);
      if(call.nativeSHA256)assert.equal(digest(call.response.data.resultJSON),call.nativeSHA256);
    }
  }
  const html=resolve(jsonFile).replace(/\.json$/,'.html');assert.notEqual(html,resolve(jsonFile));
  writeFileSync(html,report(result),{mode:0o600});
  for(const width of [1440,390]){
    const browser=await launch({width,height:width===1440?1300:844});
    try{
      for(const theme of ['light','dark']){
        await browser.theme(theme==='dark');await browser.goto(pathToFileURL(html).href);
        await browser.eval(`(0,eval)(${JSON.stringify(readFileSync(join(root,'validation/cache/axe-core/axe.min.js'),'utf8'))});true`);
        const violations=await browser.eval(`(async()=> (await axe.run(document,{runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21a','wcag21aa']}})).violations.map(v=>({id:v.id,nodes:v.nodes.map(n=>n.target)})))()`);
        assert.deepEqual(violations,[],`${width} ${theme}`);
        assert(await browser.eval('document.documentElement.scrollWidth<=innerWidth+1'),'horizontal page overflow');
        await browser.eval(`(()=>{document.querySelector('details').open=true;return true})()`);
        assert(await browser.eval(`document.querySelector('details').innerText.includes('"answerCorrect"')`));
        await browser.eval(`(()=>{document.querySelector('details').open=false;return true})()`);
        if(width===1440&&capture)await browser.capture(join(root,`docs/images/agent-benchmark-${theme}.webp`),{format:'webp',quality:86});
      }
    }finally{await browser.close();}
  }
  console.log('Benchmark report: original JSON hashes and grades checked; desktop/phone light/dark layouts accessible.');
}
if(process.argv[1]&&import.meta.url===pathToFileURL(resolve(process.argv[1])).href)await checkReport(resolve(process.argv[2]),{capture:process.argv.includes('--capture')});
