// Replicated count inference is computed and saved in Go; this view only inspects it.
import { h, clear, downloadBlob } from './dom.js';
import { confirmDialog } from './overlays.js';
import { addResult } from '../lib/results.js';
import { countMA, countObservation, countEffect } from '../lib/count-plot.js';
import { figureButton, madeBy } from './figures.js';
async function json(url, body) {
  const response=await fetch(url,body===undefined?{cache:'no-store'}:{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});
  const value=await response.json();if(!response.ok)throw new Error(value.error??`HTTP ${response.status}`);return value;
}
export function mountCohortCounts(app,container) {
  const {store}=app,root=h('section.card.cohort-counts');container.append(root);
  let serial=0,generation=store.state.generation,parent='',answer=null,loaded='',status='idle',error='',job='',selected='',offset=0,query='',includeFiltered=false;
  const minCount=h('input.input.small',{type:'number',min:1,max:1000000,value:10,'aria-label':'Minimum clone count'});
  const minSamples=h('input.input.small',{type:'number',min:2,max:96,value:12,'aria-label':'Minimum count samples'});
  const batch=h('input',{type:'checkbox','aria-label':'Adjust count model for batch'});
  const confidence=h('select.input.small',{'aria-label':'Count model confidence'},...[.9,.95,.99].map((v)=>h('option',{value:v,selected:v===.95},`${100*v}%`)));
  const busy=()=>['loading','running'].includes(status);
  const number=(v)=>v==null?'Unavailable':Number(v).toPrecision(5);
  const button=(label,fn,disabled=false)=>h('button.btn.small',{type:'button',disabled,onclick:()=>Promise.resolve().then(fn).catch((e)=>{error=e.message;draw();})},label);
  const table=(caption,heads,rows)=>h('div.table-wrap',{tabIndex:0},h('table.data',h('caption',caption),h('thead',h('tr',...heads.map((v)=>h('th',{scope:'col'},v)))),h('tbody',...rows.map((r)=>h('tr',...r.map((v)=>h('td',v)))))));
  function ordered(){return [...(answer?.result.analysis.rows??[])].sort((a,b)=>Number(b.eligible)-Number(a.eligible)||b.meanCPM-a.meanCPM||a.id.localeCompare(b.id));}
  function received(v){answer=v;loaded=v.artifact.id;status='ready';job='';offset=0;query='';selected=ordered().find((r)=>r.log2Fold!=null)?.id??ordered()[0]?.id??'';draw();root.querySelector('.count-result-title')?.scrollIntoView({block:'start'});}
  async function show(id){const token=++serial,same=store.sameProject();answer=null;loaded='';status='loading';error='';draw();try{const v=await json(`api/cohort-counts/${encodeURIComponent(id)}`);if(token!==serial||!same())return;received(v);return v;}catch(e){if(token===serial&&same()){status='failed';error=e.message;draw();}throw e;}}
  async function run(options={}) {
    if(busy())throw new Error('Wait for the current count model.');
    const source=options.features??parent,record=store.project.artifacts.find((r)=>r.id===source&&r.kind==='cohort-features');
    if(!record)throw new Error('Choose saved subject features from the design to model.');
    const settings=options.options??{minCount:Number(minCount.value),minSamples:Number(minSamples.value),adjustBatch:batch.checked,confidence:Number(confidence.value)};
    const token=++serial,same=store.sameProject();status='loading';error='';answer=null;loaded='';job='';draw();
    try {
      const dataset=store.project.datasets.find((d)=>d.id===record.dataset);if(!dataset)throw new Error('The original receptor dataset is not in this project.');
      const opened=await app.ensureDataset(dataset);if(token!==serial||!same())return;if(opened.status!=='open')throw new Error(opened.error??'Open the original receptor dataset.');
      const start=await json('api/cohort-counts',{features:source,options:settings});if(token!==serial||!same())return;
      if(!start.ready){job=start.job;status='running';draw();const done=await app.jobs.finished(job);if(token!==serial||!same())return;if(done?.state!=='complete')throw new Error(done?.error||`Count model ${done?.state??'unavailable'}.`);}
      const v=await json(`api/cohort-counts/${encodeURIComponent(start.artifact)}`);if(token!==serial||!same())return;received(v);
      if(!store.project.artifacts.some((r)=>r.id===loaded))store.commit(addResult(store.project,{id:loaded,kind:'cohort-counts',dataset:v.result.features.parent.dataset,name:v.artifact.name,made:v.artifact.created,method:'Replicated negative-binomial counts',describe:v.result.description,citation:v.artifact.lineage.method.citation}),'Keep replicated count model',['project']);draw();return v;
    }catch(e){if(token===serial&&same()){status='failed';error=e.message;job='';draw();}throw e;}
  }
  async function download(){const v=answer,same=store.sameProject();if(!v)return;const terms=(v.result.features.notice.Credits??[]).filter((c)=>c.LicenseClass!=='open');
    if(terms.length&&!await confirmDialog({title:'Provider terms apply',message:h('div',h('p','These counts and derived results retain their source terms.'),h('ul',...terms.map((c)=>h('li',`${c.Title} — ${c.License}`)))),confirm:'Export'}))return;
    if(!same())return;const response=await fetch(`api/cohort-counts/${encodeURIComponent(v.artifact.id)}/download${terms.length?'?accept=1':''}`);if(!response.ok)throw new Error((await response.json()).error??'Export failed.');const blob=await response.blob();if(same())downloadBlob(blob,`${v.artifact.id}.zip`);
  }
  function plotSpec(which){if(!answer)return null;const doc=answer.result,r=doc.analysis,row=r.rows.find((x)=>x.id===selected);if(!row)return null;
    const p=which==='ma'?countMA(r,selected):which==='effect'?countEffect(row,r):countObservation(row,r,doc.features.design),levels=doc.features.design.input.spec.levels;
    return {name:`${loaded}-${which}-${which==='ma'?'all':row.id}`,title:which==='ma'?'Abundance and fitted effects':which==='effect'?'Selected clonotype: model effect and interval':'Selected clonotype: observed subject abundance',plot:p.svg,caption:[p.description,`Contrast: ${levels[1]} / ${levels[0]}; ${doc.features.design.input.spec.layout}; ${doc.features.design.subjects} subjects, ${r.design.rows.length} observations; ${r.design.residualDF} residual df.`,which==='ma'?`All ${r.tested} fitted of ${r.family} eligible clonotypes; highlighted ${row.id}.`:`Clonotype ${row.id}: ${row.chains.map((c)=>`${c.locus} ${c.junction} ${c.v??''} ${c.j??''}`).join('; ')}.`,`NB2, per-clonotype Cox–Reid dispersion; batch adjustment ${r.options.adjustBatch}. Offset: log assigned library total; no composition normalization. Intervals are marginal, conditional on fitted dispersion; BH does not adjust intervals.`,`Definition: ${doc.features.parent.describe} (${doc.features.parent.definitionId}); QC ${doc.features.notice.Filter}; count unit ${doc.features.parent.unit}.`,`Dataset ${doc.features.parent.datasetName}; saved design ${doc.features.selection.name}, version ${doc.features.selection.revision}; result ${loaded}; feature parent ${doc.parentManifest.id}.`,...doc.features.design.findings.map((f)=>`${f.title}: ${f.detail}`),madeBy(app.version)]};
  }
  function draw(){
    clear(root);for(const control of [minCount,minSamples,batch,confidence])control.disabled=busy();
    const parents=store.project.artifacts.filter((r)=>r.kind==='cohort-features');
    root.append(h('h2','Replicated clonotype counts'),h('p','Fit separate negative-binomial counts under the saved subject design. Select the count filter and batch policy before inspecting results. The original receptor dataset supplies counts under the saved definition and QC.'),
      h('label.field',h('span','Count-model feature parent'),h('select.input.small',{'aria-label':'Count-model feature parent',disabled:busy(),onchange:(e)=>{parent=e.target.value;draw();}},h('option',{value:'',selected:!parent},'Choose saved subject features'),...parents.map((r)=>h('option',{value:r.id,selected:r.id===parent},`${r.name} · ${r.id.slice(-6)}`)))),
      h('div.cohort-controls',h('label.field',h('span','At least this count'),minCount),h('label.field',h('span','In at least this many samples'),minSamples),h('label.field',h('span','Interval confidence'),confidence),h('label.kv-inline',batch,'Adjust for recorded batch')),
      h('p.muted.small','At least twenty-four residual degrees of freedom are required for unmoderated dispersion estimation. Paired designs include subject blocks. Offsets use all assigned library counts; no pooling, TMM normalization, empirical-Bayes shrinkage or Poisson fallback.'),button('Fit replicated counts',()=>run(),busy()||!parent));
    const saved=store.project.artifacts.filter((r)=>r.kind==='cohort-counts');if(saved.length)root.append(h('div.btn-row',h('span','Saved count models'),...saved.map((r)=>button(`${r.name} · ${r.id.slice(-6)}`,()=>show(r.id),busy()))));
    if(job)root.append(button('Cancel count model',()=>app.jobs.cancel(job)));if(error)root.append(h('p.callout.danger',{role:'alert'},error));if(busy())root.append(h('p',{role:'status'},status==='running'?'Estimating clonotype variability and group effects…':'Reading saved design and count evidence…'));if(!answer)return;
    const doc=answer.result,r=doc.analysis,d=doc.features.design,row=r.rows.find((x)=>x.id===selected),levels=d.input.spec.levels;
    root.append(h('h3.count-result-title',`${doc.features.selection.name}, version ${doc.features.selection.revision}`),h('p',`${d.subjects} subjects · ${r.design.rows.length} observations · ${d.input.spec.layout} · ${r.design.residualDF} residual df. ${levels[1]} / ${levels[0]}. ${r.tested} fitted of ${r.family} eligible clonotypes in one BH family.`),
      h('p.small',`Count unit ${doc.features.parent.unit}; definition ${doc.features.parent.describe}; QC ${doc.features.notice.Filter}. Batch adjustment: ${r.options.adjustBatch?'included':'not included'}.`),
      ...d.findings.map((f)=>h('div.callout.warn',h('b',f.title),h('p',f.detail))),
      h('p.callout','Effects are changes in relative repertoire abundance, conditional on the design and library-size offsets. Unmodeled composition changes, outliers and small replication can affect inference. Intervals are marginal and treat fitted dispersion as known. No automatic discovery labels are assigned.'));
    if(row){
      root.append(h('p',`Selected: ${row.chains.map((c)=>`${c.locus} ${c.junction}`).join(' + ')} · ${row.id}`));
      const plots=h('div.plot-grid');for(const which of ['ma','observations']){const spec=plotSpec(which),box=h('div.plot-wrap');if(spec.plot){box.innerHTML=spec.plot;plots.append(h('figure.plot-card',h('figcaption',h('span',spec.title),figureButton(()=>plotSpec(which))),box));}}
      root.append(plots);
      const effect=plotSpec('effect');if(effect.plot){const box=h('div.plot-wrap');box.innerHTML=effect.plot;root.append(h('figure.plot-card',h('figcaption',h('span',effect.title),figureButton(()=>plotSpec('effect'))),box));}
      root.append(table('Selected clonotype model',['Log2 ratio','Lower','Upper','Dispersion','LR (1 df)','Raw p','BH p','Status'],[[number(row.log2Fold),number(row.lower),number(row.upper),number(row.dispersion),number(row.lr),number(row.p),number(row.adjusted),row.reason||'Converged; asymptotic inference']]),h('p.small',`${100*r.options.confidence}% Wald interval, conditional on estimated dispersion, not adjusted by BH. Observed CPM points are not batch-adjusted fitted values.`));
    }
    const rows=ordered().filter((v)=>(includeFiltered||v.eligible)&&`${v.id} ${v.chains.map((c)=>c.junction).join(' ')}`.toLowerCase().includes(query.toLowerCase()));
    root.append(h('div.cohort-controls',h('label.field',h('span','Find clonotype'),h('input.input.small',{type:'search',value:query,'aria-label':'Find count-model clonotype',onchange:(e)=>{query=e.target.value;offset=0;draw();}})),h('label.kv-inline',h('input',{type:'checkbox',checked:includeFiltered,onchange:(e)=>{includeFiltered=e.target.checked;offset=0;draw();}}),'Include count-filtered clonotypes')),
      table('Clonotypes (display filters do not change the BH family)',['Clonotype','Mean CPM','Log2 ratio','Raw p','BH p','Status'],rows.slice(offset,offset+50).map((v)=>[button(v.chains.map((c)=>c.junction).join(' + ')||v.id,()=>{selected=v.id;draw();}),number(v.meanCPM),number(v.log2Fold),number(v.p),number(v.adjusted),v.reason||'Converged'])),
      h('div.btn-row',button('Previous count results',()=>{offset=Math.max(0,offset-50);draw();},!offset),h('span',`${offset} offset · ${rows.length} matching rows`),button('Next count results',()=>{offset+=50;draw();},offset+50>=rows.length),button('Export count-model ZIP',download)),
      h('details',h('summary','Design matrix, offsets and contrast'),h('p',`Contrast vector [${r.design.contrast.join(', ')}]. No columns were silently dropped.`),table('Exact model inputs',['Sample','Subject','Group','Batch','Library total','Log offset',...r.design.columns],r.design.rows.map((s,i)=>[s.name,s.subject,s.group,s.batch||'Missing',r.totals[i],number(r.offsets[i]),...r.design.x[i]]))),
      h('details',h('summary','Count-model methods and provenance'),h('p',doc.description),h('p',answer.artifact.lineage.method.citation)));
  }
  const view={run,show,download,plotSpec,choose:(id)=>{parent=id;draw();},select:(id)=>{selected=id;draw();},state:()=>({status,error,answer,artifact:loaded,selected})};app.cohortCountsView=view;
  const focused=()=>store.ui.focus?.kind==='result'&&store.project.artifacts.find((r)=>r.id===store.ui.focus.id&&r.kind==='cohort-counts');
  draw();if(focused())show(focused().id).catch(()=>{});
  return {update(topics){if(generation!==store.state.generation){generation=store.state.generation;serial++;parent='';answer=null;loaded='';status='idle';error='';job='';}const target=focused();if(topics.has('focus')&&target&&target.id!==loaded)show(target.id).catch(()=>{});else if(topics.has('project')||topics.has('jobs'))draw();},destroy(){serial++;if(app.cohortCountsView===view)delete app.cohortCountsView;root.remove();}};
}
