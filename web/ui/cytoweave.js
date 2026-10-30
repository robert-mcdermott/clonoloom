// Native sample-scoped joins. Channel and provenance text never pass through Number().
import {h,clear,downloadBlob} from './dom.js';
import {showDialog} from './overlays.js';

export function showCytoWeave(app,dataset){
 const project=app.store.project,same=app.store.sameProject(),content=h('div.cytoweave-view');
 const samples=app.datasetStates.get(dataset.id)?.summary?.samples??[];
 let alive=true,busy=false,sample=null,wells='',mapping='',provenance='',review=null,terms=false,error='',offset=0,controller=null;
 const active=()=>alive&&same()&&app.store.project===project&&project.datasets.some(d=>d.id===dataset.id);
 const dialog=showDialog({title:'CytoWeave phenotype links',content,width:'wide',onClose:()=>{alive=false;controller?.abort();if(app.cytoweaveView===view)app.cytoweaveView=null;}});
 const invalidate=()=>{review=null;terms=false;error='';offset=0;};
 const body=()=>({dataset:dataset.id,sample,wells,mapping,provenance,fingerprint:review?.fingerprint??'',acceptTerms:terms});
 async function work(fn){
  if(busy)throw Error('Wait for the pending phenotype operation.');
  if(!active()){error='Project changed; reopen this review.';render();throw Error(error);}
  busy=true;error='';controller=new AbortController();render();
  try{return await fn();}catch(e){review=null;if(alive)error=e.name==='AbortError'?'Operation cancelled; no links were saved.':e.message;throw e;}
  finally{busy=false;controller=null;if(alive)render();}
 }
 async function post(op){const r=await fetch(`api/cytoweave/${op}`,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body()),signal:controller.signal});if(!r.ok){const e=await r.json();throw Error(e.error??'Phenotype link failed.');}return r;}
 async function inspect(){return work(async()=>{invalidate();const next=await(await post('review')).json();if(!active()||controller.signal.aborted)throw Error('Review changed or cancelled.');review=next;});}
 async function save(){return work(async()=>{if(!review)throw Error('Review first.');const blob=await(await post('export')).blob();if(!active()||controller.signal.aborted)throw Error('Project changed or operation cancelled.');downloadBlob(blob,'clonoloom-cytoweave-links.json');});}
 const action=fn=>()=>fn().catch(()=>{});
 async function file(input,kind){const f=input.files?.[0];if(!f)return;return work(async()=>{invalidate();if(f.size>(kind==='provenance'?65536:4*1024*1024))throw Error('File exceeds the supported bound.');const text=new TextDecoder('utf-8',{fatal:true,ignoreBOM:true}).decode(await f.arrayBuffer());if(!active()||controller.signal.aborted)throw Error('Project changed or operation cancelled.');if(kind==='wells')wells=text;else if(kind==='mapping')mapping=text;else provenance=text;});}
 function render(){
  clear(content);
  content.append(h('p',h('b',dataset.name)),h('p','Link one CytoWeave index-sort acquisition to one native AIRR receptor sample using an explicit well-to-cell barcode map. Inputs stay on this computer.'),
   h('p.callout','A link is supplied identity evidence, not proof of a singlet, paired receptor or biological phenotype. No subjects are pooled; no QC, gates or compensation are computed.'),
   h('label.field',h('span','Receptor sample'),h('select.input.cyto-sample',{'aria-label':'Receptor sample',disabled:busy,onchange:e=>{sample=e.target.value===''?null:Number(e.target.value);invalidate();render();}},h('option',{value:'',selected:sample===null},'Choose one sample…'),...samples.map((s,i)=>h('option',{value:i,selected:sample===i},`${s.name} · sample ${i+1}`)))));
  const upload=(kind,label,value)=>h('label.field',h('span',label),h('input.input',{type:'file',accept:kind==='provenance'?'.json':'.csv',disabled:busy,onchange:e=>file(e.target,kind).catch(()=>{})}),h('span.small',value?`${new TextEncoder().encode(value).length.toLocaleString()} UTF-8 bytes loaded`:'No file selected'));
  if(!review)content.append(upload('wells','CytoWeave wells CSV',wells),upload('mapping','Mapping CSV · well,cell_id',mapping),upload('provenance','Acquisition / compensation declaration JSON',provenance),h('p.small','Use the declaration template in the documentation. Record unknown context as unknown; do not guess. Wells and mapping: at most 4 MiB and 10,000 rows each. Original text is retained.'));
  content.append(h('div.btn-row',h('button.btn.small',{disabled:busy||sample===null||!wells||!mapping||!provenance,onclick:action(inspect)},'Review exact links'),review?h('button.btn.small',{disabled:busy,onclick:()=>{invalidate();render();}},'Change inputs'):null));
  if(review){const d=review.evidence,r=d.result,p=d.declaration;
   content.append(h('h3',`${p.sample} → ${d.sample.name}`),h('p',`${r.matched} linked · ${r.unmapped} unmapped wells · ${r.absentTargets} absent receptor targets · ${r.receptorOnly.length} receptor-only cells`),h('p.small',`${r.observations.length} exported events; ${r.receptorCells} distinct receptor cell IDs; ${r.missingBarcodeRows} receptor rows without a barcode. Repeated chain rows are not independent cells.`),
    h('p.small',`Acquisition: ${p.acquisition}. Value scale: ${p.valueScale}. Compensation: ${p.compensation}. This is a supplied declaration, not upstream verification.`));
   const shown=r.observations.slice(offset,offset+8);
   content.append(h('div',{style:{overflow:'auto',maxHeight:'280px'}},h('table.data',h('thead',h('tr',...['Well','Event','Cell ID','Link status','Receptor rows','Population',...r.columns.slice(5)].map(c=>h('th',{scope:'col'},c)))),h('tbody',...shown.map(o=>h('tr',...[o.values[0],o.values[3],o.cell||'(not mapped)',o.status,o.receptorRows.length,o.values[4]||'(empty)',...o.values.slice(5).map(v=>v===''?'(empty)':v)].map(v=>h('td',{style:{whiteSpace:'nowrap'}},v))))))),
    h('div.btn-row',h('button.btn.small',{disabled:busy||offset===0,onclick:()=>{offset=Math.max(0,offset-8);render();}},'Previous wells'),h('span.small',`${offset+1}–${Math.min(offset+8,r.observations.length)} of ${r.observations.length}`),h('button.btn.small',{disabled:busy||offset+8>=r.observations.length,onclick:()=>{offset+=8;render();}},'Next wells')),
    h('p.small','Channel values are exact source text, including empty and NaN. Population is one exported deepest gate label; it does not represent all memberships. No phenotype colors or threshold calls are inferred.'),
    h('details',h('summary','Provenance, unmatched cells and limits'),h('pre',{style:{whiteSpace:'pre-wrap',overflowWrap:'anywhere',maxHeight:'220px',overflow:'auto'}},`${d.provenance.text}\n\nReceptor-only IDs: ${r.receptorOnly.join(', ')}\n\n${d.notice}\n\n${d.limits}`)));
   if(d.restricted)content.append(h('label.check',h('input',{type:'checkbox',checked:terms,disabled:busy,onchange:e=>{terms=e.target.checked;render();}}),'I reviewed provider terms and have permission to export these links.'));
   content.append(h('button.btn.primary.small',{disabled:busy||(d.restricted&&!terms),onclick:action(save)},'Save exact link evidence'),h('p.small','Save the JSON before closing. Links are a separate local evidence record; they do not change project annotations or scientific replay.'));
  }
  if(busy)content.append(h('button.btn.small',{onclick:()=>controller?.abort()},'Cancel phenotype operation'));
  if(error)content.append(h('p.callout.danger',{role:'alert'},error));
 }
 const view={inspect,save,state:()=>({busy,review,error}),configure(values){if(busy)throw Error('Operation pending');({sample,wells,mapping,provenance}=values);invalidate();render();},close:dialog.close};
 app.cytoweaveView=view;render();return view;
}
