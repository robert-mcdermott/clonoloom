// Go prepares the data-only interchange and original scientific evidence.
import {h,clear,downloadBlob} from './dom.js';
import {confirmDialog} from './overlays.js';
export function mountPrism(app,container){
 const {store}=app,root=h('section.card.prism-view');container.append(root);
 const records=()=> (store.project.artifacts??[]).filter(r=>['cohort-features','cohort-compare'].includes(r.kind));
 let selected='',title='Subject-level cohort evidence',busy=false,error='',fingerprint='',controller=null,alive=true,generation=store.state.generation;
 async function exportPrism(id=selected,options={}){
  if(busy)throw new Error('Wait for the Prism export.');
  if(!records().some(r=>r.id===id))throw new Error('Choose one saved subject-feature or cohort-comparison result.');
  const project=store.project,same=store.sameProject(),valid=()=>alive&&same()&&store.project===project;
  selected=id;title=options.title??title;let accept=options.accept===true;
  controller=new AbortController();busy=true;error='';fingerprint='';render();
  try{
   const request=()=>fetch('api/reports/prism',{method:'POST',headers:{'Content-Type':'application/json'},signal:controller.signal,body:JSON.stringify({artifact:id,title,accept})});
   let response=await request();if(!valid())throw new Error('The analysis changed; the Prism export was not downloaded.');
   if(!response.ok){const failure=await response.json();if(failure.code==='restricted'&&options.prompt&&!accept){accept=await confirmDialog({title:'Prism evidence may not be shared',message:failure.error,confirm:'Prepare for my study'});if(!valid())throw new Error('The analysis changed; export again.');if(!accept)return null;response=await request();}else throw new Error(failure.error??'Prism export failed.');}
   if(!response.ok)throw new Error((await response.json()).error??'Prism export failed.');
   const hash=response.headers.get('X-ClonoLoom-Prism');if(!/^[a-f0-9]{64}$/.test(hash??''))throw new Error('Prism ZIP fingerprint is unavailable.');
   const blob=await response.blob();if(!valid())throw new Error('The analysis changed; the Prism export was not downloaded.');
   downloadBlob(blob,'clonoloom-prism.zip');fingerprint=hash;app.log('Exported subject-level Prism tables; no statistical analysis or replay.');return {fingerprint,artifact:id,recomputed:false};
  }catch(e){if(alive&&same())error=e.name==='AbortError'?'Prism export cancelled.':e.message;throw e;}
  finally{if(alive&&same()){busy=false;controller=null;render();}}
 }
 function render(){
  if(!alive)return;if(!records().some(r=>r.id===selected))selected='';clear(root);
  root.append(h('h2','GraphPad Prism subject tables'),h('p','Choose one saved cohort feature or comparison result. Download data-only PZFX Column tables: one table per recorded feature, two groups in their declared order and one identified subject per row.'),h('p.muted','Paired measurements share their subject row. Independent subjects occupy only their own group column. Missing measurements remain blank, with reasons and exclusions in the companion evidence; zero stays zero. Cells, clones and technical replicates are not exported as independent subjects.'),h('p.callout','Prism may round imported values and omit incomplete pairs during analysis. Keep the original JSON and certificates. Review the recorded design and missingness before choosing a new analysis in Prism; this export contains no tests, graphs or analysis template.'),h('label.field',h('span','Saved subject-level result'),h('select.input',{disabled:busy,'aria-label':'Prism result',onchange:e=>{selected=e.target.value;fingerprint='';error='';render();}},h('option',{value:'',selected:!selected},'Choose a result…'),...records().map(r=>h('option',{value:r.id,selected:r.id===selected},`${r.name??r.kind} · ${r.id}`)))),h('label.field',h('span','Prism study title'),h('input.input',{value:title,maxLength:200,disabled:busy,oninput:e=>title=e.target.value})),h('button.btn.small',{type:'button',disabled:busy||!selected,onclick:()=>exportPrism(undefined,{prompt:true}).catch(()=>{})},'Export Prism tables'),h('p.muted','Read the Info sheet and mapping.json. Provider terms and identifiers remain. Other result kinds and general repeated-measures designs are outside this export contract.'));
  if(controller)root.append(h('button.btn.small',{type:'button',onclick:()=>controller?.abort()},'Cancel Prism export'));
  if(error)root.append(h('p.callout.danger',{role:'alert'},error));
  if(fingerprint)root.append(h('p','Prism ZIP fingerprint (keep separately):'),h('p.mono.certificate-fingerprint',fingerprint));
 }
 const view={exportPrism,state:()=>({busy,error,fingerprint})};app.prismView=view;render();
 return {update(topics){if(generation!==store.state.generation){controller?.abort();controller=null;generation=store.state.generation;selected='';busy=false;error='';fingerprint='';}if(topics.has('project')||topics.has('mode'))render();},destroy(){alive=false;controller?.abort();if(app.prismView===view)app.prismView=null;root.remove();}};
}
