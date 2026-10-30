// Workbook content and certificates are prepared by Go. This view selects explicit
// results and preserves the existing provider acknowledgment and download flow.
import {h,clear,downloadBlob} from './dom.js';
import {confirmDialog} from './overlays.js';
import {certificateResults} from '../lib/certificates.js';

export function mountWorkbooks(app,container) {
  const {store}=app,root=h('section.card.workbooks-view');container.append(root);
  let selected=new Set(),title='Saved analysis workbook',busy=false,error='',fingerprint='',controller=null,alive=true,generation=store.state.generation;
  async function exportWorkbook(ids=[...selected],options={}) {
    if(busy) throw new Error('Wait for the workbook export.');
    if(!Array.isArray(ids)||ids.length<1||ids.length>8||new Set(ids).size!==ids.length||ids.some((id)=>!certificateResults(store.project).some((r)=>r.id===id))) throw new Error('Choose 1–8 distinct supported results in this project.');
    const project=store.project,same=store.sameProject(),valid=()=>alive&&same()&&store.project===project;
    selected=new Set(ids);title=options.title??title;
    let accept=options.accept===true;
    controller=new AbortController();busy=true;error='';fingerprint='';render();
    try {
      const request=()=>fetch('api/reports/workbook',{method:'POST',headers:{'Content-Type':'application/json'},signal:controller.signal,body:JSON.stringify({title:options.title??title,artifacts:ids,accept})});
      let response=await request();
      if(!valid())throw new Error('The analysis changed; the workbook was not downloaded.');
      if(!response.ok) {
        const failure=await response.json();
        if(failure.code==='restricted'&&options.prompt&&!accept) {
          accept=await confirmDialog({title:'Workbook evidence may not be shared',message:failure.error,confirm:'Prepare for my study'});
          if(!valid())throw new Error('The analysis changed; export again.');
          if(!accept)return null;
          response=await request();
        } else throw new Error(failure.error??'Workbook export failed.');
      }
      if(!response.ok)throw new Error((await response.json()).error??'Workbook export failed.');
      const hash=response.headers.get('X-ClonoLoom-Workbook');
      if(!/^[a-f0-9]{64}$/.test(hash??''))throw new Error('Workbook ZIP fingerprint is unavailable.');
      const blob=await response.blob();if(!valid())throw new Error('The analysis changed; the workbook was not downloaded.');
      downloadBlob(blob,'clonoloom-workbook.zip');fingerprint=hash;
      app.log('Exported an Excel evidence workbook; no scientific recomputation.');
      return {fingerprint,artifacts:ids,recomputed:false};
    } catch(e) {if(alive&&same())error=e.name==='AbortError'?'Workbook export cancelled.':e.message;throw e;}
    finally {if(alive&&same()){busy=false;controller=null;render();}}
  }
  function render() {
    if(!alive)return;
    const records=certificateResults(store.project);selected=new Set([...selected].filter((id)=>records.some((r)=>r.id===id)));
    clear(root);root.append(h('h2','Excel evidence workbook'),h('p','Choose 1–8 saved cohort, batch QC or motif results. Download results.xlsx with exact native JSON, provenance, provider notices and certificates in one ZIP. Each worksheet row is a typed JSON node, not an independent subject.'),h('p.muted','Excel numbers have 15 significant digits. Precision-sensitive numbers remain text; Exact JSON scalar preserves every original token. Null, empty strings and absent fields stay distinct. No formulas, date guessing or scientific recomputation. Keep the native companions; provider terms and identifiers remain.'),h('label.field',h('span','Workbook title'),h('input.input',{value:title,disabled:busy,maxLength:200,oninput:(e)=>{title=e.target.value;}})));
    const list=h('fieldset',h('legend','Explicitly selected results'));
    for(const r of records)list.append(h('label',{style:{display:'block'}},h('input',{type:'checkbox',checked:selected.has(r.id),disabled:busy,'aria-label':`${r.kind} ${r.id}`,onchange:(e)=>{e.target.checked?selected.add(r.id):selected.delete(r.id);render();}}),` ${r.name??r.kind} · ${r.id}`));
    root.append(list,h('p.muted',`${selected.size} selected. Other result kinds are outside this workbook contract; they are not silently included.`),h('button.btn.small',{type:'button',disabled:busy||!selected.size||selected.size>8,onclick:()=>exportWorkbook(undefined,{prompt:true}).catch(()=>{})},'Export Excel workbook'));
    if(controller)root.append(h('button.btn.small',{type:'button',onclick:()=>controller?.abort()},'Cancel workbook export'));
    if(error)root.append(h('p.callout.danger',{role:'alert'},error));
    if(fingerprint)root.append(h('p','Workbook ZIP fingerprint (keep separately):'),h('p.mono.certificate-fingerprint',fingerprint));
  }
  const view={exportWorkbook,state:()=>({busy,error,fingerprint})};app.workbooksView=view;render();
  return {update(topics){if(generation!==store.state.generation){controller?.abort();controller=null;generation=store.state.generation;selected=new Set();busy=false;error='';fingerprint='';}if(topics.has('project')||topics.has('mode'))render();},destroy(){alive=false;controller?.abort();if(app.workbooksView===view)app.workbooksView=null;root.remove();}};
}
