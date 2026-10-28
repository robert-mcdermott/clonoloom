import {h,clear,downloadBlob,formatBytes} from './dom.js';
import {datasetDefinition} from '../lib/definitions.js';
import {datasetFilter} from '../lib/filters.js';
import {deidentificationAnnotations,validDeidentificationReceipt} from '../lib/deidentification.js';

export function createDeidentification(app) {
  const {store}=app;let busy=false,error='',prepared=null,source=null,sourceDataset='',controller=null;
  const notify=()=>store.notify(['deidentification']);
  store.subscribe(topics=>{if(topics.has('project')||topics.has('project-loaded')){prepared=null;source=null;sourceDataset='';notify();}});
  async function perform(work){
    if(busy||app.jobs.running().length||store.state.busy.size)throw new Error('Wait for current work before exporting.');
    busy=true;error='';controller=new AbortController();const project=store.project,same=store.sameProject(),valid=()=>same()&&project===store.project;notify();
    try{return await work(project,valid,controller.signal);}
    catch(e){error=e.name==='AbortError'?'Export cancelled. Prepare and review again.':e.message;throw e;}
    finally{busy=false;controller=null;notify();}
  }
  const api={
    state:()=>({busy,error,prepared,dataset:sourceDataset}),cancel:()=>controller?.abort(),
    forget(){prepared=null;source=null;sourceDataset='';error='';notify();},
    async prepare(dataset,{what='tables',storageFormat='text',dropText=true,dropHLA=true,accept=false}={}){
      return perform(async(project,valid,signal)=>{
        prepared=null;source=null;sourceDataset='';
        const d=project.datasets.find(d=>d.id===dataset||d.name===dataset)??(!dataset?project.datasets[0]:null);
        if(!d)throw new Error('Choose a dataset in this project.');
        const opened=await app.ensureDataset(d);if(opened.status!=='open')throw new Error(opened.error??'Dataset unavailable.');
        if(!valid())throw new Error('The project changed before preparation.');
        const summary=opened.summary,loci=[...new Set(summary.samples.flatMap(s=>Object.keys(s.loci??{})))];
        const response=await fetch('api/deidentification/prepare',{method:'POST',headers:{'Content-Type':'application/json'},signal,body:JSON.stringify({dataset:d.id,definition:datasetDefinition(d,loci),filter:datasetFilter(project,d)?.rules??{},what,storageFormat,dropText,dropHLA,accept,annotations:deidentificationAnnotations(project,d.id,summary.samples)})});
        const value=await response.json();if(!response.ok)throw new Error(value.error??'Preparation failed.');
        if(!valid())throw new Error('The project changed; prepare and review again.');
        if(!validDeidentificationReceipt(value))throw new Error('Unsupported preparation receipt.');
        prepared=value;source=project;sourceDataset=d.id;return value;
      });
    },
    async download({token=prepared?.token,expected=prepared?.fingerprint,identityMap=false}={}){
      return perform(async(project,valid,signal)=>{
        if(!prepared||source!==project||token!==prepared.token||expected!==prepared.fingerprint)throw new Error('Prepare and review this project before downloading.');
        const part=identityMap?'identity-map':'bundle';
        const response=await fetch(`api/deidentification/${encodeURIComponent(token)}?expected=${encodeURIComponent(expected)}&part=${part}`,{signal});
        if(!response.ok)throw new Error((await response.json()).error??'Download failed.');
        if(response.headers.get('X-ClonoLoom-Deidentification')!==expected)throw new Error('Prepared fingerprint changed.');
        const blob=await response.blob();if(!valid())throw new Error('The project changed; nothing downloaded.');
        downloadBlob(blob,identityMap?'clonoloom-private-identity-map.json':'clonoloom-deidentified.zip');
        return {fingerprint:expected,part,scientificValuesChanged:false,sourceProjectChanged:false};
      });
    },
  };return api;
}

export function mountDeidentification(app,container){
  const root=h('section.card.deidentification-view');container.append(root);
  let dataset=app.store.project.datasets[0]?.id??'',what='tables',storageFormat='text',dropText=true,dropHLA=true,accept=false;
  const run=work=>work().catch(()=>{});
  function render(){
    clear(root);const {busy,error,prepared,dataset:preparedDataset}=app.deidentification.state();
    if(prepared){dataset=preparedDataset;what=prepared.record.what;storageFormat=prepared.record.storageFormat??'text';dropText=prepared.record.dropText;dropHLA=prepared.record.dropHLA;}
    const fields=h('fieldset',h('legend','Explicit export choices'));
    for(const [label,checked,set] of [
      ['Remove free-text annotations (time point, tissue, reference, subset, condition, batch)',dropText,v=>{dropText=v;}],
      ['Remove HLA annotation',dropHLA,v=>{dropHLA=v;}],
      ['I acknowledge private-use restrictions on provider data',accept,v=>{accept=v;}],
    ])fields.append(h('label',{style:{display:'block'}},h('input',{type:'checkbox',checked,disabled:busy,onchange:e=>{set(e.target.checked);app.deidentification.forget();}}),' ',label));
    root.append(h('h2','De-identification export'),h('p','Prepare a separate copy of native dataset tables. Review fresh subject pseudonyms before downloading; original data, project and saved scientific evidence stay unchanged.'),
      h('p.callout','Pseudonymization is not anonymity. Receptor sequences, gene calls, stable clonotype IDs and rare measurements remain linkable. Retained annotations can identify participants. The private identity map is a separate download.'),
      h('label.field',h('span','Dataset'),h('select.input',{value:dataset,disabled:busy,onchange:e=>{dataset=e.target.value;app.deidentification.forget();}},...app.store.project.datasets.map(d=>h('option',{value:d.id,selected:d.id===dataset},d.name)))),
      h('label.field',h('span','Storage format'),h('select.input',{disabled:busy,onchange:e=>{storageFormat=e.target.value;app.deidentification.forget();}},h('option',{value:'text',selected:storageFormat==='text'},'CSV and AIRR TSV'),h('option',{value:'parquet',selected:storageFormat==='parquet'},'Typed Parquet'))),
      h('label.field',h('span','Supported tables'),h('select.input',{value:what,disabled:busy,onchange:e=>{what=e.target.value;app.deidentification.forget();}},h('option',{value:'tables',selected:what==='tables'},'AIRR rearrangements and clonotypes'),h('option',{value:'clonotypes',selected:what==='clonotypes'},'Clonotypes only'))),fields,
      h('p.muted','Other archives, saved results, figures, raw metadata and nested IMGT annotations are outside this export contract. Removing experimental annotations limits later study-design reconstruction. Missing subjects remain missing.'),
      h('button.btn.small',{disabled:busy||!app.store.project.datasets.length,onclick:()=>run(()=>app.deidentification.prepare(dataset,{what,storageFormat,dropText,dropHLA,accept}))},'Prepare and review pseudonyms'));
    if(busy)root.append(h('button.btn.small',{onclick:()=>app.deidentification.cancel()},'Cancel preparation/download'));
    if(error)root.append(h('p.callout.danger',{role:'alert'},error));
    if(prepared){
      const r=prepared.record;
      root.append(h('h3','Prepared export review'),h('p',`${r.subjects} subjects · ${r.missingSubjects} samples without a subject · ${r.samples} samples · ${r.retainedRows.toLocaleString('en-US')} retained rearrangements · ${r.clonotypes.toLocaleString('en-US')} clonotypes.`),
        h('p',`Prepared settings: ${r.storageFormat==='parquet'?'Parquet':'CSV/AIRR TSV'}; free text ${r.dropText?'removed':'retained'}; HLA ${r.dropHLA?'removed':'retained'}. ${r.what==='tables'?'AIRR and clonotype tables':'Clonotype table only'}. Changing choices requires preparing again.`),
        h('p.muted',`Expires ${new Date(prepared.expires).toLocaleTimeString()}; preparing again replaces this export and generates new pseudonyms.`));
      const rows=h('tbody');for(const s of prepared.subjects)rows.append(h('tr',h('td',s.original),h('td.mono',s.pseudonym)));
      root.append(h('div.table-wrap',{tabIndex:0},h('table.data',h('caption','Complete subject correspondence — local/private review only'),h('thead',h('tr',h('th',{scope:'col'},'Original subject'),h('th',{scope:'col'},'Export pseudonym'))),rows)),
        h('details',h('summary','Removed fields and excluded formats'),h('ul',...r.removed.map(s=>h('li',s)))),
        h('p',r.files.map(f=>`${f.path} (${formatBytes(f.size)})`).join(' · ')),h('p.mono.certificate-fingerprint',`Fingerprint: ${prepared.fingerprint}`),
        h('div.btn-row',h('button.btn.small.primary',{disabled:busy,onclick:()=>run(()=>app.deidentification.download())},'Download reviewed export'),h('button.btn.small',{disabled:busy,onclick:()=>run(()=>app.deidentification.download({identityMap:true}))},'Download PRIVATE identity map separately')));
    }
  }
  render();return{update(topics){if(topics.has('project')||topics.has('deidentification'))render();},destroy(){root.remove();}};
}
