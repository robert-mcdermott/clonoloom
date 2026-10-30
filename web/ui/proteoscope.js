// One reviewed source observation. Only Go constructs the query or contacts Proteoscope.
import {h,clear,downloadBlob} from './dom.js';
import {showDialog} from './overlays.js';

export function showProteoscope(app,dataset){
 const project=app.store.project,same=app.store.sameProject(),content=h('div.proteoscope-view');
 let alive=true,busy=false,offset=0,rows=[],total=0,row=null,port=8765,review=null,error='',receipt='',receiptText='',local=false,publicSearch=false,terms=false,controller=null,generation=0;
 const active=()=>alive&&same()&&app.store.project===project&&app.store.project.datasets.some(d=>d.id===dataset.id);
 const dialog=showDialog({title:'Proteoscope sequence handoff',content,width:'wide',onClose:()=>{alive=false;controller?.abort();if(app.proteoscopeView===view)app.proteoscopeView=null;}});
 const request=()=>({dataset:dataset.id,row,port,review:review?.review,fingerprint:review?.fingerprint,acceptLocal:local,acceptPublic:publicSearch,acceptTerms:terms});
 async function post(operation,body){
  const r=await fetch(`api/proteoscope/${operation}`,{method:'POST',headers:{'Content-Type':'application/json'},signal:controller.signal,body:JSON.stringify(body)});
  if(!active())throw new Error('The project changed; close this review and select the observation again.');
  if(!r.ok){const x=await r.json();throw new Error(x.error??'Handoff failed.');}return r;
 }
 function invalidate(){generation++;review=null;local=publicSearch=terms=false;error='';receipt='';receiptText='';}
 async function work(task){
  if(busy)throw new Error('Wait for this handoff operation.');
  if(!active()){error='The project changed; close this review.';render();throw new Error(error);}
  busy=true;error='';controller=new AbortController();render();
  try{return await task();}catch(e){if(alive)error=e.name==='AbortError'?'Cancelled. If dispatch began, check Proteoscope: cancellation cannot retract a query.':e.message;throw e;}
  finally{busy=false;controller=null;if(alive)render();}
 }
 async function page(next=offset){return work(async()=>{invalidate();row=null;offset=next;const result=await(await post('page',{dataset:dataset.id,offset})).json();if(!active())throw new Error('Project changed.');rows=result.rows;total=result.total;});}
 async function inspect(){return work(async()=>{invalidate();const revision=generation;const result=await(await post('review',request())).json();if(!active()||generation!==revision)throw new Error('Selection changed; review again.');review=result;});}
 async function exportEvidence(){return work(async()=>{if(!review)throw new Error('Review one observation first.');const response=await post('export',request());const blob=await response.blob();if(!active())throw new Error('Project changed.');downloadBlob(blob,'clonoloom-proteoscope-handoff.json');});}
 async function send(){return work(async()=>{
  if(!review||!local||!publicSearch||(review.evidence.restricted&&!terms))throw new Error('Approve the named destinations and applicable provider terms first.');
  const body=request(),evidence=await(await post('export',body)).text();if(!active())throw new Error('Project changed.');
  // Save the exact reviewed record before dispatch, including on an uncertain result.
  downloadBlob(new Blob([evidence],{type:'application/json'}),'clonoloom-proteoscope-handoff.json');
  let response;
  try{response=await(await post('send',body)).text();}finally{review=null;local=publicSearch=terms=false;}
  if(!active())throw new Error('Project changed; check Proteoscope for the dispatched query.');
  receiptText=response;
  downloadBlob(new Blob([response],{type:'application/json'}),'clonoloom-proteoscope-receipt.json');
  receipt='Proteoscope acknowledged the command. Inspect its Find structures results and errors; acknowledgment does not establish a match or scientific result. Review again before another transfer.';
 });}
 const action=(fn)=>()=>fn().catch(()=>{});
 function render(){
  clear(content);
  content.append(h('p',h('b',dataset.name)),h('p','Select one original AIRR observation. The supplied sequence_aa is handed to a running Proteoscope 0.10.0 for sequence search. No paired chain or full receptor is reconstructed.'),
   h('p.callout','Sending invokes public RCSB sequence search. The amino-acid query can leave this computer and be cached by Proteoscope. Identifiers and annotations stay in your downloaded evidence. No sequence is sent during review.'),
   h('label.field',h('span','Source observation'),h('select.input.proteoscope-row',{'aria-label':'Proteoscope observation',disabled:busy,onchange:e=>{row=e.target.value===''?null:Number(e.target.value);invalidate();render();}},h('option',{value:'',selected:row===null},'Choose an observation…'),...rows.map(r=>h('option',{value:r.row,selected:r.row===row},`${r.sequenceId||'(missing ID)'} · ${r.sample} · ${r.locus||'unknown locus'} · ${r.source}:${r.line}`)))),
   h('div.btn-row',h('button.btn.small',{disabled:busy||offset===0,onclick:action(()=>page(Math.max(0,offset-25)))},'Previous observations'),h('span.small',`${total?offset+1:0}–${Math.min(offset+rows.length,total)} of ${total} source observations · no QC selection`),h('button.btn.small',{disabled:busy||offset+rows.length>=total,onclick:action(()=>page(offset+25))},'Next observations')),
   h('label.field',h('span','Proteoscope port on 127.0.0.1'),h('input.input.proteoscope-port',{type:'number',min:1,max:65535,value:port,disabled:busy,onchange:e=>{port=Number(e.target.value);invalidate();render();}})),
   h('button.btn.small',{disabled:busy||row===null,onclick:action(inspect)},'Review sequence and destination'));
  if(review){const d=review.evidence;
   content.append(h('h3',`${d.sequenceId} · ${d.locus} · ${d.sample}`),h('p',`Source: ${d.source.name}, line ${d.line}. sequence_aa: ${d.sequence.length} residues. complete_vdj: ${d.fields.complete_vdj||'not supplied'} (V(D)J region only; full protein completeness is not established).`),
    h('pre.proteoscope-sequence',{tabIndex:0,style:{whiteSpace:'pre-wrap',overflowWrap:'anywhere',maxHeight:'160px',overflow:'auto'}},d.sequence||'(sequence_aa missing)'),
    h('p.small.mono',{style:{overflowWrap:'anywhere'}},`Sequence SHA-256: ${d.sequenceSHA256}`),
    h('p',`Destination: ${d.destination.url} · Proteoscope ${d.destination.version||'unavailable'}. ${d.destination.reason||'Remote control is enabled. Keep its browser page open; the most recently connected page receives the query.'}`),
    d.reason?h('p.callout',d.reason):null,
    h('details',h('summary','Annotations, provenance and provider terms'),h('pre',{style:{whiteSpace:'pre-wrap',overflowWrap:'anywhere',maxHeight:'220px',overflow:'auto'}},d.notice+'\n\n'+d.headerText+'\n'+d.rowText)),
    h('p.small','Sequence similarity does not establish antigen specificity, affinity or function. No structure prediction is run. Cancellation after dispatch cannot retract the query; check Proteoscope before retrying an uncertain outcome. Keep the downloaded evidence: this handoff is not scientific replay.'));
   const check=(label,value,set,cls)=>h('label.check',h('input',{type:'checkbox',className:cls,checked:value,disabled:busy,onchange:e=>{set(e.target.checked);render();}}),label);
   content.append(check(`I approve sending this exact sequence to ${d.destination.url}.`,local,v=>local=v,'proteoscope-local'),check('I authorize Proteoscope to disclose this sequence to public RCSB sequence search.',publicSearch,v=>publicSearch=v,'proteoscope-public'));
   if(d.restricted)content.append(check('I have reviewed the provider terms and have permission for this use. Acknowledgment does not grant disclosure rights.',terms,v=>terms=v,'proteoscope-terms'));
   content.append(h('div.btn-row',h('button.btn.small',{disabled:busy||(d.restricted&&!terms),onclick:action(exportEvidence)},'Save reviewed evidence'),h('button.btn.primary.small',{disabled:busy||!d.eligible||!d.destination.ready||!local||!publicSearch||(d.restricted&&!terms),onclick:action(send)},'Send sequence and save receipt')));
  }
  if(busy)content.append(h('button.btn.small',{onclick:()=>controller?.abort()},'Cancel handoff operation'));
  if(error)content.append(h('p.callout.danger',{role:'alert'},error));
  if(receipt)content.append(h('p.callout',{role:'status'},receipt),h('button.btn.small',{onclick:()=>downloadBlob(new Blob([receiptText],{type:'application/json'}),'clonoloom-proteoscope-receipt.json')},'Save acknowledgment again'));
 }
 const view={page,inspect,send,exportEvidence,select(i,p=port){if(busy)throw new Error('Operation pending');row=i;port=p;invalidate();render();},approve({local:l=false,publicSearch:p=false,terms:t=false}){local=l;publicSearch=p;terms=t;render();},state:()=>({busy,review,error,receipt}),close:dialog.close};
 app.proteoscopeView=view;render();view.ready=page();view.ready.catch(()=>{});return view;
}
