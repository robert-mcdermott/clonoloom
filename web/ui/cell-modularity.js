// Immutable graph tests. JS chooses settings and displays Go's statistics without recalculation.
import { h, clear } from './dom.js';
import { addResult } from '../lib/results.js';
import { modularityPlot } from '../lib/modularity-plot.js';
async function json(url, body) {
  const response = await fetch(url, body === undefined ? { cache:'no-store' } : { method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify(body) });
  const value=await response.json(); if(!response.ok) throw new Error(value.error ?? `HTTP ${response.status}`); return value;
}
export function mountCellModularity(app, container) {
  const { store }=app, root=h('section'); container.append(root);
  let serial=0, answer=null, loaded='', status='idle', error='', job='', parent='', clone='', offset=0, nullOffset=0, query='', generation=store.state.generation;
  const draws=h('input.input',{type:'number',value:9999,min:99,max:99999,step:1,'aria-label':'Modularity Monte Carlo draws'});
  const seed=h('input.input',{type:'number',value:20261024,min:1,max:Number.MAX_SAFE_INTEGER,step:1,'aria-label':'Modularity seed'});
  const button=(label,action,disabled=false)=>h('button.btn.small',{type:'button',disabled,onclick:()=>Promise.resolve().then(action).catch((e)=>{error=e.message;draw();})},label);
  const number=(v)=>v==null?'Not tested':Number(v).toPrecision(5);
  function received(value){answer=value;loaded=value.artifact.id;status='ready';job='';offset=0;nullOffset=0;query='';clone=value.result.analysis.clones.find((c)=>c.p!=null)?.id??'';draw();}
  async function show(id){const token=++serial,same=store.sameProject();answer=null;loaded='';status='loading';error='';draw();try{const value=await json(`api/modularity/${encodeURIComponent(id)}`);if(token!==serial||!same())return;received(value);return value;}catch(e){if(token===serial&&same()){error=e.message;status='failed';draw();}throw e;}}
  async function run(options={}){
    const selected=options.phenotypes??parent;
    if(!selected)throw new Error('Choose saved phenotypes made with a computed expression map.');
    const token=++serial,same=store.sameProject();status='loading';error='';answer=null;loaded='';job='';draw();
    try{
      const start=await json('api/modularity',{phenotypes:selected,options:options.options??{permutations:Number(draws.value),seed:Number(seed.value)}});
      if(token!==serial||!same())return;
      if(!start.ready){job=start.job;status='running';draw();const done=await app.jobs.finished(job);if(token!==serial||!same())return;if(done?.state!=='complete')throw new Error(done?.error||`Modularity ${done?.state??'unavailable'}.`);}
      const value=await json(`api/modularity/${encodeURIComponent(start.artifact)}`);if(token!==serial||!same())return;
      received(value);
      if(!store.project.artifacts.some((r)=>r.id===loaded))store.commit(addResult(store.project,{id:loaded,kind:'modularity',dataset:value.result.phenotypes.dataset,expression:value.result.phenotypes.input.id,name:value.artifact.name,made:value.artifact.created,method:'Clonotype modularity',describe:value.result.description,citation:value.artifact.lineage.method.citation}),'Keep clonotype modularity',['project']);
      draw();return value;
    }catch(e){if(token===serial&&same()){error=e.message;status='failed';job='';draw();}throw e;}
  }
  function selectClone(id){clone=id;nullOffset=0;draw();}
  function table(heads,rows){return h('div.table-scroll',{tabIndex:0},h('table.data',h('thead',h('tr',...heads.map((v)=>h('th',{scope:'col'},v)))),h('tbody',...rows.map((row)=>h('tr',...row.map((v)=>h('td',v)))))));}
  function draw(){
    clear(root);root.append(h('h2#clonotype-modularity','Clonotype modularity'),h('p','Test whether a clone has more internal expression-graph edges than uniform groups of the same size. Choose a saved phenotype result made with a computed map. The graph is fixed in PCA space; UMAP coordinates are not used.'));
    const parents=store.project.artifacts.filter((r)=>r.kind==='phenotypes');if(parent&&!parents.some((r)=>r.id===parent))parent='';
    root.append(h('h3','Settings for the next result'),h('div.btn-row',h('label.field',h('span','Modularity phenotype parent'),h('select.input',{'aria-label':'Modularity phenotype parent',onchange:(e)=>{parent=e.target.value;}},h('option',{value:'',selected:!parent},'Choose saved phenotypes'),...parents.map((r)=>h('option',{value:r.id,selected:parent===r.id},`${r.name} · ${r.id.slice(-6)}`)))),h('label.field',h('span','Monte Carlo draws'),draws),h('label.field',h('span','Seed'),seed),button('Compute clonotype modularity',()=>run(),!parents.length||['running','loading'].includes(status))),
      h('p.muted','Null: clone membership is exchangeable among uniquely assigned graph cells in one sample. Cell type, batch, receptor family and vertex degree are not conditioned on. This is exploratory within-sample connectivity, not antigen specificity or a subject-level effect. Small null spaces are enumerated exhaustively.'));
    const saved=store.project.artifacts.filter((r)=>r.kind==='modularity');if(saved.length)root.append(h('div.btn-row',h('span','Saved modularity'),...saved.map((r)=>button(`${r.name} · ${r.id.slice(-6)}`,()=>show(r.id),status==='running'))));
    if(job)root.append(button('Cancel modularity',()=>app.jobs.cancel(job)));
    root.append(h('p',{role:'status'},error||(status==='running'?app.jobs.get(job)?.message||'Testing graph connectivity…':status==='loading'?'Reading modularity…':'')));
    if(!answer)return;
    const doc=answer.result,r=doc.analysis,p=doc.phenotypes;
    root.append(h('p',`${p.analysis.sample} · Subject: ${p.annotation.subject||'Missing'} · Timepoint: ${p.annotation.timepoint||'Missing'} · ${r.eligible} eligible / ${r.graphCells} graph cells · ${doc.expressionExcluded} expression cells outside the graph · ${r.edges} induced edges · ${r.tested} tested clones.`),h('p',`Definition ${p.analysis.definitionId} · QC ${p.analysis.filterId}. Sizes below count only the graph population. BY-adjusted p-values cover all ${r.tested} tested clones in this result, including zero-edge clones. Filtering and plot selection do not change that family.`));
    const chosen=r.clones.find((c)=>c.id===clone),nullModel=chosen?.p==null?null:r.nulls.find((n)=>n.size===chosen.cells);
    if(chosen&&nullModel){
      root.append(h('h3','Observed connectivity and its null'),h('p',{style:{overflowWrap:'anywhere'}},`${chosen.id} · ${chosen.cells} cells · score ${number(chosen.score)} · upper-tail p ${number(chosen.p)} · BY-adjusted p ${number(chosen.adjusted)}`),h('div',{html:modularityPlot(chosen,nullModel)}),h('p.muted',`${nullModel.method}: ${nullModel.draws} size-matched subsets. Solid marker: observed edges; dashed marker: analytic expected edges. Frequencies count subsets, not cells; the null is not an uncertainty interval for a biological effect. ${chosen.tail} draws had at least as many edges as observed. ${nullModel.method==='Monte Carlo'?`p = (tail + 1) / (draws + 1); theoretical lower bound for raw p is 1/${nullModel.draws+1}. A floor-limited p-value is not zero; finite resolution can limit multiple-testing power.`:'p = tail / exhaustive subsets, including the observed subset.'}`),
        h('details',h('summary','Null-frequency table'),table(['Internal edges','Subsets'],nullModel.histogram.slice(nullOffset,nullOffset+50).map((b)=>[b.edges,b.count])),h('div.btn-row',button('Previous null rows',()=>{nullOffset=Math.max(0,nullOffset-50);draw();},nullOffset===0),h('span',`${nullOffset} offset · ${nullModel.histogram.length} occupied edge counts`),button('Next null rows',()=>{nullOffset+=50;draw();},nullOffset+50>=nullModel.histogram.length))));
    }else if(chosen)root.append(h('p',`${chosen.id}: ${chosen.reason}. No test or null plot.`));
    const entries=r.clones.filter((c)=>`${c.id} ${c.reason??''}`.toLowerCase().includes(query.toLowerCase()));
    root.append(h('label.field',h('span','Filter modularity clones'),h('input.input',{'aria-label':'Filter modularity clones',value:query,onchange:(e)=>{query=e.target.value;offset=0;draw();}})),
      table(['Clonotype','Graph cells','Observed edges','Expected edges','log2 score','Upper-tail p','BY-adjusted p','Status'],entries.slice(offset,offset+50).map((c)=>[button(c.id,()=>selectClone(c.id)),c.cells,c.observed,number(c.expected),number(c.score),number(c.p),number(c.adjusted),c.reason||'Tested'])),
      h('div.btn-row',button('Previous clones',()=>{offset=Math.max(0,offset-50);draw();},offset===0),h('span',`${offset} offset · ${entries.length} matching clones`),button('Next clones',()=>{offset+=50;draw();},offset+50>=entries.length)),
      button('Open parent phenotype overlay',async()=>{await app.cellPhenotypesView.show(answer.artifact.lineage.parents.find((p)=>p.kind==='phenotypes').id);if(clone)app.cellPhenotypesView.selectClone(clone);document.querySelector('#clonotype-phenotypes')?.scrollIntoView({block:'start'});}),
      h('details',h('summary','Modularity method and provenance'),h('p',doc.description),h('p',answer.artifact.lineage.method.citation),h('p',{style:{overflowWrap:'anywhere'}},`Result ${loaded} · Map ${doc.map}`),h('pre',{style:{whiteSpace:'pre-wrap',overflowWrap:'anywhere'}},JSON.stringify(answer.artifact.lineage,null,2))),
      h('a.btn.small',{href:`api/modularity/${encodeURIComponent(loaded)}`,download:`${loaded}.json`},'Download modularity and provenance'));
  }
  const view={run,show,selectClone,state:()=>({status,error,answer,artifact:loaded,clone})};app.cellModularityView=view;
  const focused=()=>store.ui.focus?.kind==='result'&&store.project.artifacts.find((r)=>r.id===store.ui.focus.id&&r.kind==='modularity');
  draw();if(focused())show(focused().id).catch(()=>{});
  return {update(topics){if(generation!==store.state.generation){generation=store.state.generation;serial++;answer=null;loaded='';status='idle';error='';job='';parent='';draw();}const target=focused();if(topics.has('focus')&&target&&target.id!==loaded)show(target.id).catch(()=>{});else if(topics.has('project')||topics.has('jobs')&&status==='running')draw();},destroy(){serial++;if(app.cellModularityView===view)delete app.cellModularityView;root.remove();}};
}
