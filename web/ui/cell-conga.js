// Immutable graph tests. JS chooses settings and displays Go's statistics without recalculation.
import { h, clear } from './dom.js';
import { addResult } from '../lib/results.js';
import { congaPlot } from '../lib/conga-plot.js';
async function json(url, body) {
  const response = await fetch(url, body === undefined ? { cache:'no-store' } : { method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify(body) });
  const value=await response.json(); if(!response.ok) throw new Error(value.error ?? `HTTP ${response.status}`); return value;
}
export function mountCellConga(app, container) {
  const { store }=app, root=h('section'); container.append(root);
  let serial=0, answer=null, loaded='', status='idle', error='', job='', parent='', clone='', offset=0, nullOffset=0, query='', generation=store.state.generation;
  const sizes=h('input.input',{type:'text',value:'5,15','aria-label':'Comparison neighborhood sizes'});
  const metric=h('select.input',{'aria-label':'Comparison receptor distance'},h('option',{value:'human'},'Full TCRdist · human'),h('option',{value:'mouse'},'Full TCRdist · mouse'),h('option',{value:'cdr3'},'CDR3-only TCRdist · explicit approximation'));
  const button=(label,action,disabled=false)=>h('button.btn.small',{type:'button',disabled,onclick:()=>Promise.resolve().then(action).catch((e)=>{error=e.message;draw();})},label);
  const number=(v)=>v==null?'—':Number(v).toPrecision(5);
  const probability=(p,log)=>log==null?'Not tested':p==null?`Below numeric range (ln p ${number(log)})`:number(p);
  function received(value){answer=value;loaded=value.artifact.id;status='ready';job='';offset=0;nullOffset=0;query='';clone=value.result.analysis.nodes[0]?.id??'';draw();}
  async function show(id){const token=++serial,same=store.sameProject();answer=null;loaded='';status='loading';error='';draw();try{const value=await json(`api/conga/${encodeURIComponent(id)}`);if(token!==serial||!same())return;received(value);return value;}catch(e){if(token===serial&&same()){error=e.message;status='failed';draw();}throw e;}}
  async function run(options={}){
    const selected=options.phenotypes??parent;
    if(!selected)throw new Error('Choose saved phenotypes made with a computed expression map.');
    const token=++serial,same=store.sameProject();status='loading';error='';answer=null;loaded='';job='';draw();
    try{
      const start=await json('api/conga',{phenotypes:selected,options:options.options??{neighbors:sizes.value.split(',').map((v)=>Number(v.trim())),cdr3Only:metric.value==='cdr3',organism:metric.value==='cdr3'?'':metric.value}});
      if(token!==serial||!same())return;
      if(!start.ready){job=start.job;status='running';draw();const done=await app.jobs.finished(job);if(token!==serial||!same())return;if(done?.state!=='complete')throw new Error(done?.error||`Conga ${done?.state??'unavailable'}.`);}
      const value=await json(`api/conga/${encodeURIComponent(start.artifact)}`);if(token!==serial||!same())return;
      received(value);
      if(!store.project.artifacts.some((r)=>r.id===loaded))store.commit(addResult(store.project,{id:loaded,kind:'conga',dataset:value.result.phenotypes.dataset,expression:value.result.phenotypes.input.id,name:value.artifact.name,made:value.artifact.created,method:'Receptor / expression neighborhoods',describe:value.result.description,citation:value.artifact.lineage.method.citation}),'Keep receptor / expression comparison',['project']);
      draw();return value;
    }catch(e){if(token===serial&&same()){error=e.message;status='failed';job='';draw();}throw e;}
  }
  function selectClone(id){clone=id;nullOffset=0;draw();}
  function table(heads,rows){return h('div.table-scroll',{tabIndex:0},h('table.data',h('thead',h('tr',...heads.map((v)=>h('th',{scope:'col'},v)))),h('tbody',...rows.map((row)=>h('tr',...row.map((v)=>h('td',v)))))));}
  function draw(){
    clear(root);root.append(h('h2#receptor-expression-neighborhoods','Receptor / expression neighborhoods'),h('p','Compare expression and receptor neighbors of paired αβ clonotypes in one sample. Each clone contributes one mean PC profile. UMAP coordinates are not distances for this test. Choose computed-map phenotypes and keep their receptor dataset open to prepare a new result.'));
    const parents=store.project.artifacts.filter((r)=>r.kind==='phenotypes');if(parent&&!parents.some((r)=>r.id===parent))parent='';
    root.append(h('details',{open:!answer},h('summary','Settings for the next result'),h('div.btn-row',h('label.field',h('span','Comparison phenotype parent'),h('select.input',{'aria-label':'Comparison phenotype parent',onchange:(e)=>{parent=e.target.value;}},h('option',{value:'',selected:!parent},'Choose saved phenotypes'),...parents.map((r)=>h('option',{value:r.id,selected:parent===r.id},`${r.name} · ${r.id.slice(-6)}`)))),h('label.field',h('span','Neighborhood sizes (comma separated)'),sizes),h('label.field',h('span','Receptor distance'),metric),button('Compare receptor and expression neighborhoods',()=>run(),!parents.length||['running','loading'].includes(status))),
      h('p.muted','CoNGA-style adaptation: mean PC profiles, exact distances, shared-CDR3 exclusions and inclusive cutoff ties. Full TCRdist requires the V-loop reference in Reference data. The CDR3-only option omits germline loops; it is not full TCRdist. The conditional null assumes independent, exchangeable neighbor labels within each candidate pool. Cell type and batch can violate this assumption. No specificity, causality or subject-level inference.')));
    const saved=store.project.artifacts.filter((r)=>r.kind==='conga');if(saved.length)root.append(h('div.btn-row',h('span','Saved comparisons'),...saved.map((r)=>button(`${r.name} · ${r.id.slice(-6)}`,()=>show(r.id),status==='running'))));
    if(job)root.append(button('Cancel comparison',()=>app.jobs.cancel(job)));
    root.append(h('p',{role:'status'},error||(status==='running'?app.jobs.get(job)?.message||'Comparing neighborhoods…':status==='loading'?'Reading comparison…':'')));
    if(!answer)return;
    const doc=answer.result,r=doc.analysis,p=doc.phenotypes;
    root.append(h('p',`${p.analysis.sample} · Subject: ${p.annotation.subject||'Missing'} · Timepoint: ${p.annotation.timepoint||'Missing'} · ${r.eligible} eligible / ${r.cells.length} retained expression cells · ${doc.expressionExcluded} expression cells outside the PCA population · ${r.nodes.length} clonotypes · ${r.tested} tests.`),
      h('p',`Saved distance: ${r.options.cdr3Only?'CDR3-only TCRdist (no germline loops)':`Full TCRdist · ${r.options.organism}`} · neighborhood sizes: ${r.options.neighbors.join(', ')}. Definition ${p.analysis.definitionId} · QC ${p.analysis.filterId}. Bonferroni covers every testable clone × k in this result. Filtering does not change that family.`),h('p.muted','Conditional independent-neighbor null; cell type and batch are not controlled. Saved PCs may include receptor genes, directly coupling the modalities. Exploratory association, not specificity or subject-level inference.'));
    const chosen=r.nodes.find((n)=>n.id===clone),index=r.nodes.indexOf(chosen),selected=r.tests.filter((t)=>t.node===index);
    if(chosen){
      root.append(h('h3','Observed and expected shared neighbors'),h('p',{style:{overflowWrap:'anywhere'}},`${chosen.id} · ${chosen.barcodes.length} retained cells · ${chosen.candidates} candidates · ${chosen.sharedChain} shared-chain exclusions`),h('div',{html:congaPlot(chosen,selected)}),h('p.muted','Solid bars: observed shared neighbors. Dashed markers: conditional expected overlap. Both use counts on a zero-based axis. This is not an effect uncertainty interval. Ties can enlarge either neighborhood beyond k; the test uses the actual degrees.'));
      root.append(table(['Requested k','Expression degree','Receptor degree','Observed overlap','Expected overlap','Upper-tail p','Bonferroni p','Status'],selected.map((t)=>[t.k,t.expression?.length??'—',t.receptor?.length??'—',t.logP==null?'—':t.overlap,number(t.expected),probability(t.p,t.logP),probability(t.adjusted,t.logAdjusted),t.reason||'Tested'])),
        h('details',h('summary','Exact neighbor identities and retained cells'),h('p',{style:{overflowWrap:'anywhere'}},chosen.barcodes.join(', ')),...selected.filter((t)=>t.logP!=null).map((t)=>h('div',h('h4',`k = ${t.k}`),h('p',{style:{overflowWrap:'anywhere'}},`Expression: ${t.expression.map((i)=>r.nodes[i].id).join(', ')}`),h('p',{style:{overflowWrap:'anywhere'}},`Receptor: ${t.receptor.map((i)=>r.nodes[i].id).join(', ')}`)))));
    }
    const entries=r.nodes.filter((n)=>n.id.toLowerCase().includes(query.toLowerCase()));
    root.append(h('label.field',h('span','Filter comparison clones'),h('input.input',{'aria-label':'Filter comparison clones',value:query,onchange:(e)=>{query=e.target.value;offset=0;draw();}})),
      table(['Clonotype','Retained cells','Candidates','Shared-chain exclusions'],entries.slice(offset,offset+50).map((n)=>[button(n.id,()=>selectClone(n.id)),n.barcodes.length,n.candidates,n.sharedChain])),
      h('div.btn-row',button('Previous clones',()=>{offset=Math.max(0,offset-50);draw();},offset===0),h('span',`${offset} offset · ${entries.length} matching clones`),button('Next clones',()=>{offset+=50;draw();},offset+50>=entries.length)),
      h('details',h('summary','Retained-cell eligibility'),table(['Barcode','Clone','Status'],r.cells.slice(nullOffset,nullOffset+50).map((c)=>[c.barcode,c.clone||'—',c.reason||'Eligible'])),h('div.btn-row',button('Previous cells',()=>{nullOffset=Math.max(0,nullOffset-50);draw();},nullOffset===0),h('span',`${nullOffset} offset · ${r.cells.length} cells`),button('Next cells',()=>{nullOffset+=50;draw();},nullOffset+50>=r.cells.length))),
      button('Open comparison phenotype overlay',async()=>{await app.cellPhenotypesView.show(answer.artifact.lineage.parents.find((p)=>p.kind==='phenotypes').id);if(clone)app.cellPhenotypesView.selectClone(clone);document.querySelector('#clonotype-phenotypes')?.scrollIntoView({block:'start'});}),
      h('details',h('summary','Comparison method and provenance'),h('p',doc.description),h('p',answer.artifact.lineage.method.citation),h('p',{style:{overflowWrap:'anywhere'}},`Result ${loaded} · Map ${doc.map}`),h('pre',{style:{whiteSpace:'pre-wrap',overflowWrap:'anywhere'}},JSON.stringify(answer.artifact.lineage,null,2))),
      h('a.btn.small',{href:`api/conga/${encodeURIComponent(loaded)}`,download:`${loaded}.json`},'Download comparison and provenance'));
  }
  const view={run,show,selectClone,state:()=>({status,error,answer,artifact:loaded,clone})};app.cellCongaView=view;
  const focused=()=>store.ui.focus?.kind==='result'&&store.project.artifacts.find((r)=>r.id===store.ui.focus.id&&r.kind==='conga');
  draw();if(focused())show(focused().id).catch(()=>{});
  return {update(topics){if(generation!==store.state.generation){generation=store.state.generation;serial++;answer=null;loaded='';status='idle';error='';job='';parent='';draw();}const target=focused();if(topics.has('focus')&&target&&target.id!==loaded)show(target.id).catch(()=>{});else if(topics.has('project')||topics.has('jobs')&&status==='running')draw();},destroy(){serial++;if(app.cellCongaView===view)delete app.cellCongaView;root.remove();}};
}
