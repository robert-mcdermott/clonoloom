// Report content and certificates are prepared by Go. This view selects explicit
// results and preserves the existing provider acknowledgment and download flow.
import {h,clear,downloadBlob} from './dom.js';
import {confirmDialog} from './overlays.js';
import {certificateResults} from '../lib/certificates.js';

export function mountHTMLReports(app,container) {
  const {store}=app,root=h('section.card.html-reports-view');container.append(root);
  let selected=new Set(),title='Saved analysis report',busy=false,error='',fingerprint='',controller=null,alive=true,generation=store.state.generation;
  async function exportReport(ids=[...selected],options={}) {
    if(busy) throw new Error('Wait for the report export.');
    if(!Array.isArray(ids)||ids.length<1||ids.length>8||new Set(ids).size!==ids.length||ids.some((id)=>!certificateResults(store.project).some((r)=>r.id===id))) throw new Error('Choose 1–8 distinct supported results in this project.');
    const project=store.project,same=store.sameProject(),valid=()=>alive&&same()&&store.project===project;
    selected=new Set(ids);title=options.title??title;
    let accept=options.accept===true;
    controller=new AbortController();busy=true;error='';fingerprint='';render();
    try {
      const request=()=>fetch('api/reports/html',{method:'POST',headers:{'Content-Type':'application/json'},signal:controller.signal,body:JSON.stringify({title:options.title??title,artifacts:ids,accept})});
      let response=await request();
      if(!valid())throw new Error('The analysis changed; the report was not downloaded.');
      if(!response.ok) {
        const failure=await response.json();
        if(failure.code==='restricted'&&options.prompt&&!accept) {
          accept=await confirmDialog({title:'Report evidence may not be shared',message:failure.error,confirm:'Prepare for my study'});
          if(!valid())throw new Error('The analysis changed; export again.');
          if(!accept)return null;
          response=await request();
        } else throw new Error(failure.error??'Report export failed.');
      }
      if(!response.ok)throw new Error((await response.json()).error??'Report export failed.');
      const hash=response.headers.get('X-ClonoLoom-Report');
      if(!/^[a-f0-9]{64}$/.test(hash??''))throw new Error('Report fingerprint is unavailable.');
      const blob=await response.blob();if(!valid())throw new Error('The analysis changed; the report was not downloaded.');
      downloadBlob(blob,'clonoloom-report.html');fingerprint=hash;
      app.log('Exported a self-checking HTML report; no scientific recomputation.');
      return {fingerprint,artifacts:ids,recomputed:false};
    } catch(e) {if(alive&&same())error=e.name==='AbortError'?'Report export cancelled.':e.message;throw e;}
    finally {if(alive&&same()){busy=false;controller=null;render();}}
  }
  function render() {
    if(!alive)return;
    const records=certificateResults(store.project);selected=new Set([...selected].filter((id)=>records.some((r)=>r.id===id)));
    clear(root);root.append(h('h2','Self-checking HTML report'),h('p','Choose 1–8 saved results. Export complete recorded tables, existing figures, methods, references, provider notices and native certificates in one offline file.'),h('p.muted','The browser checks embedded bytes; it does not recompute science or validate the study. Original source files are not included. Captured identifiers, sequences and provider restrictions remain.'),h('label.field',h('span','Report title'),h('input.input',{value:title,disabled:busy,maxLength:200,oninput:(e)=>{title=e.target.value;}})));
    const list=h('fieldset',h('legend','Explicitly selected results'));
    for(const r of records)list.append(h('label',{style:{display:'block'}},h('input',{type:'checkbox',checked:selected.has(r.id),disabled:busy,'aria-label':`${r.kind} ${r.id}`,onchange:(e)=>{e.target.checked?selected.add(r.id):selected.delete(r.id);render();}}),` ${r.name??r.kind} · ${r.id}`));
    root.append(list,h('p.muted',`${selected.size} selected. Other result kinds are outside this report contract; they are not silently included.`),h('button.btn.small',{type:'button',disabled:busy||!selected.size||selected.size>8,onclick:()=>exportReport(undefined,{prompt:true}).catch(()=>{})},'Export self-checking HTML'));
    if(controller)root.append(h('button.btn.small',{type:'button',onclick:()=>controller?.abort()},'Cancel report export'));
    if(error)root.append(h('p.callout.danger',{role:'alert'},error));
    if(fingerprint)root.append(h('p','Report fingerprint (keep separately):'),h('p.mono.certificate-fingerprint',fingerprint));
  }
  const view={exportReport,state:()=>({busy,error,fingerprint})};app.htmlReportsView=view;render();
  return {update(topics){if(generation!==store.state.generation){controller?.abort();controller=null;generation=store.state.generation;selected=new Set();busy=false;error='';fingerprint='';}if(topics.has('project')||topics.has('mode'))render();},destroy(){alive=false;controller?.abort();if(app.htmlReportsView===view)app.htmlReportsView=null;root.remove();}};
}
