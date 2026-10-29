// Execute the actual Apache-2.0 sibling plate CSV exporter on its artificial FCS,
// then import that output through ClonoLoom's native sample-scoped join.
// node validation/cytoweave-links.mjs CLONOLOOM_BINARY CYTOWEAVE_REPO OUTPUT_DIR
import assert from 'node:assert/strict';
import {spawn} from 'node:child_process';
import {createServer} from 'node:http';
import {mkdtempSync,mkdirSync,readFileSync,writeFileSync,rmSync} from 'node:fs';
import {join,resolve,extname} from 'node:path';
import {tmpdir} from 'node:os';
import {createHash} from 'node:crypto';
import {launch,sleep} from './cdp.mjs';
const [binary,sibling,output]=process.argv.slice(2);assert(binary&&sibling&&output,'Give binary, sibling repository and output directory');
assert.match(readFileSync(join(sibling,'LICENSE'),'utf8'),/Apache License/);
const temp=mkdtempSync(join(tmpdir(),'clonoloom-cytoweave-'));let browser,child;
const sha=b=>createHash('sha256').update(b).digest('hex');
const server=createServer((req,res)=>{try{const pathname=new URL(req.url,'http://local').pathname;if(pathname==='/'){res.setHeader('Content-Type','text/html');res.end('<!doctype html><title>CytoWeave interchange control</title><body></body>');return;}const file=resolve(sibling,'web','.'+pathname);assert(file.startsWith(resolve(sibling,'web')+'/'));res.setHeader('Content-Type',extname(file)==='.js'?'text/javascript':'application/octet-stream');res.end(readFileSync(file));}catch{res.writeHead(404).end();}});
const post=async(url,op,body,status=200)=>{const r=await fetch(url+op,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});const text=await r.text();assert.equal(r.status,status,text);return {text,data:JSON.parse(text)};};
try{
 await new Promise(r=>server.listen(0,'127.0.0.1',r));browser=await launch({width:1300,height:900});await browser.goto(`http://127.0.0.1:${server.address().port}/`);
 const exported=await browser.eval(`(async()=>{
  const {generateExample}=await import('/lib/examples.js'),{parseFCS,readSpillover}=await import('/lib/fcs.js'),{SampleView}=await import('/lib/engine.js'),{createWorkspace,sampleFromDataset}=await import('/lib/workspace.js'),{createPlateView}=await import('/ui/plate-view.js');
  const {files}=generateExample('index-sort',{}),file=files.find(f=>f.name==='Plate1_IndexSort.fcs'),dataset=parseFCS(file.bytes).datasets[0];
  const record=sampleFromDataset(dataset,{name:file.name,size:file.bytes.length}),ws=createWorkspace('Artificial index sort'),view=new SampleView(record,dataset);ws.samples=[record];
  const spill=readSpillover(dataset.keywords,dataset.parameters);view.setCompensation(spill?{...spill,id:'file'}:null);
  const app={store:{ws,ui:{sampleId:record.id},setUI(){}},data:{view:()=>view},selectGate(){}};
  const plate=createPlateView(app);document.body.append(plate.el);plate.render();
  let blob;const original=URL.createObjectURL;URL.createObjectURL=b=>{blob=b;return original(b)};
  try{[...plate.el.querySelectorAll('button')].find(b=>b.textContent.includes('Export wells')).click();}finally{URL.createObjectURL=original}
  const channels=view.parameters.filter(p=>p.type==='fluorescence'||p.type==='scatter');
  return {wells:await blob.text(),fcs:Array.from(file.bytes),sample:record,spill:spill?{...spill,matrix:Array.from(spill.matrix)}:null,keywords:dataset.keywords,truth:file.meta.truth,values:Array.from({length:view.eventCount},(_,i)=>channels.map(p=>String(view.column(p.name)[i])))};
 })()`);
 const dir=resolve(output,'sibling');mkdirSync(dir,{recursive:true});const fcs=Buffer.from(exported.fcs);writeFileSync(join(dir,'artificial.fcs'),fcs);writeFileSync(join(dir,'wells.csv'),exported.wells);
 // Independent synthetic barcode assignment from original FCS event indices. Never
 // infer real receptor identities from the CSV order; this is a known-truth control.
 const truth=exported.truth;const csvRows=exported.wells.trimEnd().split('\n').slice(1).map(l=>l.split(','));
 assert.equal(csvRows.length,95);const mappings=['well,cell_id'];const airr=['sequence_id\tcell_id\trepertoire_id\tlocus\tproductive\tv_call\tj_call\tjunction_aa'];
 for(const row of csvRows){const event=Number(row[3])-1;assert.equal(row[0],truth.wellNames?.[truth.wells[event]]??truth.wells[event]);assert.deepEqual(row.slice(5),exported.values[event]);const barcode='BC'+String(event+1).padStart(5,'0')+'-1';mappings.push(row[0]+','+barcode);airr.push(`seq${event}\t${barcode}\tArtificial plate A\tIGH\tT\tIGHV3-23*01\tIGHJ4*02\tCARDRGYYFDYW`);}
 const source=join(dir,'receptors.tsv');writeFileSync(source,airr.join('\n')+'\n');const mapping=mappings.join('\n')+'\n';writeFileSync(join(dir,'mapping.csv'),mapping);
 const provenance=JSON.stringify({exporterVersion:'CytoWeave 0.8.1 (19d47bb) actual plate-view exporter',sample:exported.sample.name,acquisition:'Artificial FACSAria-like 96-well plate from sibling generator',sourceSHA256:sha(fcs),compensation:exported.spill?JSON.stringify(exported.spill):'none',valueScale:'linear',gating:'No gates loaded; exported population is empty, never a negative phenotype call.',evidenceText:JSON.stringify({keywords:exported.keywords,sample:exported.sample,truth:exported.truth})},null,2);
 writeFileSync(join(dir,'provenance.json'),provenance);
 const free=createServer();await new Promise(r=>free.listen(0,'127.0.0.1',r));const port=free.address().port;await new Promise(r=>free.close(r));const url=`http://127.0.0.1:${port}/`;
 child=spawn(resolve(binary),['--window','none','--port',String(port),'--data-dir',join(temp,'library'),'--cache-dir',join(temp,'cache')],{stdio:'ignore'});
 for(let i=0;i<150;i++){try{if((await fetch(url)).ok)break;}catch{}await sleep(100);}
 const ds=(await post(url,'api/datasets/open',{paths:[source]})).data;
 const request={dataset:ds.id,sample:0,wells:exported.wells,mapping,provenance};const review=(await post(url,'api/cytoweave/review',request)).data;
 assert.equal(review.evidence.result.matched,95);assert.equal(review.evidence.result.absentTargets,0);
 const saved=await post(url,'api/cytoweave/export',{...request,fingerprint:review.fingerprint});writeFileSync(join(dir,'links.json'),saved.text);
 // Actual upstream export contains all events. A second event at the same well must be refused.
 const repeated=exported.wells+csvRows[0].map((s,i)=>i===3?'96':s).join(',')+'\n';await post(url,'api/cytoweave/review',{...request,wells:repeated},400);
 console.log('Actual CytoWeave artificial FCS → parser/compensation → plate CSV exporter → 95 exact sample-scoped ClonoLoom links; duplicate-well refusal verified.');
}finally{if(browser)await browser.close();if(child)child.kill();await new Promise(r=>server.close(r));await sleep(200);rmSync(temp,{recursive:true,force:true});}
