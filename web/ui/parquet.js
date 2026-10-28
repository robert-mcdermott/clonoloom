// Parquet is generated and validated by Go; the window never converts scientific values.
import {h,clear,downloadBlob} from './dom.js';
import {datasetDefinition} from '../lib/definitions.js';
import {datasetFilter} from '../lib/filters.js';
import {UNITS} from '../lib/datasets.js';
import {invariantSpecies} from '../lib/invariant.js';

export function createParquetExports(app){
 const {store}=app;let busy=false,error='',receipt=null,requested=null,controller=null;
 const notify=()=>store.notify(['parquet']);
 store.subscribe(topics=>{if(topics.has('project')||topics.has('project-loaded')){receipt=null;requested=null;notify();}});
 return {
  state:()=>({busy,error,receipt,requested}),clear(){receipt=null;requested=null;error='';notify();},cancel:()=>controller?.abort(),
  async export(dataset,{what='clonotypes',accept=false}={}){
   if(busy||app.jobs.running().length||store.state.busy.size)throw new Error('Wait for current work before exporting.');
   const project=store.project,same=store.sameProject(),valid=()=>same()&&store.project===project;
   busy=true;error='';receipt=null;requested={dataset,what};controller=new AbortController();notify();
   try{
    const d=project.datasets.find(d=>d.id===dataset);if(!d)throw new Error('Choose a dataset in this project.');
    const opened=await app.ensureDataset(d);if(opened.status!=='open')throw new Error(opened.error??'Dataset unavailable.');
    if(!valid())throw new Error('The project changed; export again.');
    const loci=[...new Set(opened.summary.samples.flatMap(s=>Object.keys(s.loci??{})))];
    const response=await fetch('api/parquet/export',{method:'POST',headers:{'Content-Type':'application/json'},signal:controller.signal,body:JSON.stringify({dataset:d.id,what,accept,species:invariantSpecies(d),definition:datasetDefinition(d,loci),filter:datasetFilter(project,d)?.rules??{}})});
    if(!response.ok)throw new Error((await response.json()).error??'Parquet export failed.');
    const record=JSON.parse(response.headers.get('X-ClonoLoom-Parquet-Receipt')??'null');
    if(record?.format!=='clonoloom-parquet-export'||!/^[a-f0-9]{64}$/.test(record.fingerprint))throw new Error('Unsupported Parquet receipt.');
    const blob=await response.blob();if(!valid())throw new Error('The project changed; nothing downloaded.');
    downloadBlob(blob,`clonoloom-${what}-parquet.zip`);receipt={...record,dataset:d.id};return receipt;
   }catch(e){error=e.name==='AbortError'?'Parquet export cancelled.':e.message;throw e;}
   finally{busy=false;controller=null;notify();}
  },
 };
}
export function mountParquetExports(app,container){
 const root=h('section.card.parquet-view');container.append(root);
 let dataset=app.store.project.datasets[0]?.id??'',what='clonotypes',accept=false;
 function render(){
  clear(root);const {busy,error,receipt,requested}=app.parquet.state();
  if(requested){dataset=requested.dataset;what=requested.what;}
  if(!app.store.project.datasets.some(d=>d.id===dataset))dataset=app.store.project.datasets[0]?.id??'';
  root.append(h('h2','Typed Parquet exports'),h('p','Export native clonotypes or rearrangements with nullable fields, integer counts, definition, QC, units and source provenance. The ZIP includes a schema, inventory and provider notice.'),
   h('p.callout','This export retains original identifiers. For a separate pseudonymized copy, use De-identification export below and choose Parquet. Neither representation is a scientific replay certificate.'),
   h('label.field',h('span','Parquet dataset'),h('select.input',{disabled:busy,onchange:e=>{dataset=e.target.value;app.parquet.clear();}},...app.store.project.datasets.map(d=>h('option',{value:d.id,selected:d.id===dataset},d.name)))),
   h('label.field',h('span','Native table'),h('select.input',{disabled:busy,onchange:e=>{what=e.target.value;app.parquet.clear();}},h('option',{value:'clonotypes',selected:what==='clonotypes'},'Clonotypes — sample abundance columns'),h('option',{value:'rearrangements',selected:what==='rearrangements'},'Rearrangements — retained AIRR fields'))),
   h('label',h('input',{type:'checkbox',checked:accept,disabled:busy,onchange:e=>{accept=e.target.checked;}}),' I acknowledge private-use restrictions on provider data'),
   h('p.muted','Missing values remain null; measured zero remains zero. Fractional counts and native counts above 9,007,199,254,740,991 are refused rather than rounded. Parquet cannot recover precision lost before export. Retained AIRR annotation text is not newly validated biology.'),
   h('div.btn-row',h('button.btn.small.primary',{disabled:busy||!dataset,onclick:()=>app.parquet.export(dataset,{what,accept}).catch(()=>{})},busy?'Preparing Parquet…':'Export Parquet ZIP'),busy?h('button.btn.small',{onclick:()=>app.parquet.cancel()},'Cancel export'):null));
  if(error)root.append(h('p.callout.danger',{role:'alert'},error));
  if(receipt)root.append(h('h3','Downloaded native table'),h('p',`${receipt.rows.toLocaleString('en-US')} rows · ${receipt.what} · clonotype abundance in ${UNITS[receipt.unit]??'unknown units'}. Original identifiers retained.`),h('p',receipt.files.map(f=>f.path).join(' · ')),h('p.mono.certificate-fingerprint',`Inventory fingerprint: ${receipt.fingerprint}`));
 }
 render();return {update(topics){if(topics.has('project'))accept=false;if(topics.has('project')||topics.has('parquet'))render();},destroy(){root.remove();}};
}
