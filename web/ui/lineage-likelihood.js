// Explicit selected-panel likelihood review; all fitting/probabilities come from Go.
import {h,clear,downloadBlob} from './dom.js';
import {showDialog} from './overlays.js';
import {lineagePlot} from '../lib/lineage.js';
import {figureButton,madeBy} from './figures.js';

export function showLineageLikelihood(app,artifact,tree){
  const project=app.store.project,same=app.store.sameProject(),content=h('div.likelihood-view');
  let alive=true,busy=false,review=null,error='',terms=false,node=null,site=0,field='isotype',controller=null;
  const active=()=>alive&&same()&&app.store.project===project;
  const dialog=showDialog({title:'JC69 lineage likelihood · baseline model',content,width:'wide',onClose:()=>{alive=false;controller?.abort();if(app.likelihoodView===view)app.likelihoodView=null;}});
  const action=fn=>()=>fn().catch(()=>{});
  async function work(fn){if(busy)throw Error('Wait for the current likelihood operation.');if(!active()){error='Project changed; reopen this review.';render();throw Error(error);}busy=true;error='';controller=new AbortController();render();try{return await fn();}catch(e){review=null;error=e.name==='AbortError'?'Likelihood operation cancelled; no partial tree retained.':e.message;throw e;}finally{busy=false;controller=null;if(alive)render();}}
  async function post(op){const r=await fetch(`api/lineage-likelihood/${op}`,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({artifact,tree:tree.id,review:review?.review??'',acceptTerms:terms}),signal:controller.signal});if(!r.ok){const e=await r.json();throw Error(e.error??'Likelihood review failed.');}return r;}
  async function run(){return work(async()=>{review=null;node=null;terms=false;const answer=await(await post('review')).json();if(!active()||controller.signal.aborted)throw Error('Project changed or review cancelled.');review=answer;});}
  async function save(){return work(async()=>{if(!review)throw Error('Fit a reviewed panel first.');const blob=await(await post('export')).blob();if(!active()||controller.signal.aborted)throw Error('Project changed or export cancelled.');downloadBlob(blob,'clonoloom-lineage-likelihood.json');});}
  const format=n=>Number(n).toPrecision(5);
  function render(){
    clear(content);content.append(h('p',h('b',`${tree.family} · ${tree.locus} · ${tree.sites} shared V/J sites`)),h('p.callout','JC69 is an equal-rate, equal-frequency baseline. It does not model somatic-hypermutation context or selection. Reference placement is assumed; observed isotypes, tissues and timepoints do not infer switching, migration or dates.'),h('p.small','One saved panel; at most seven distinct states including the reference. Constant sites are retained. Branches are bounded at 0–3 substitutions/site. Exhaustive topology enumeration does not prove global branch optimization.'),h('button.btn.small',{disabled:busy,onclick:action(run)},review?'Refit saved panel':'Fit JC69 likelihood'));
    if(review){const d=review.evidence,r=d.result;
      content.append(h('p.small',d.parent.name));
      content.append(h('p',`${r.taxa} states · ${r.topologies} binary topologies · log likelihood ${format(r.logLikelihood)} · ${r.nearBest} within 0.000001 of the best fit`),h('p.small','Numerical stationarity passed. Near ties and zero branches are not bootstrap support. Ancestors below are conditional on one selected topology and fitted lengths.'));
      const select=h('select.input.small',{'aria-label':'Likelihood observed annotation',disabled:busy,onchange:e=>{field=e.target.value;render();}},...['isotype','timepoint','tissue','sample'].map(v=>h('option',{value:v,selected:v===field},v)));
      const plot=lineagePlot(r,field),drawing=h('div.table-scroll',{tabIndex:0});drawing.innerHTML=plot.svg;
      const choose=e=>{const target=e.target.closest('[data-node]');if(target&&(e.type==='click'||e.key==='Enter'||e.key===' ')){e.preventDefault();node=r.nodes.find(n=>n.id===target.dataset.node);render();content.querySelector('[data-ml-detail]')?.focus();}};drawing.addEventListener('click',choose);drawing.addEventListener('keydown',choose);
      content.append(h('div.options-row',h('label','Observed annotation ',select),figureButton(()=>{if(!active()||(d.restricted&&!terms))throw Error("Review the current project and provider permissions before export.");return ({title:`JC69 baseline · ${tree.locus}`,name:'lineage-likelihood',plot:plot.svg,legend:plot.legend,caption:[d.parent.name,plot.description,`Log likelihood ${r.logLikelihood}; ${r.sites} complete-case V/J sites; ${r.topologies} enumerated topologies.`,d.citation,d.result.description,`Parent ${artifact}; tree ${tree.id}; SHA-256 ${d.parentSHA256}.`,madeBy(app.version)]});})),drawing,h('p.small.muted',plot.description));
      const nodeRows=r.nodes.map((n,i)=>h('tr',h('td',h('button.btn.small',{onclick:()=>{node=n;render();content.querySelector('[data-ml-detail]')?.focus();}},i+1)),h('td',n.kind),h('td',String(n.branchLength)),h('td',n.boundary),h('td',n.rows)));
      const nodeTable=h('table.data',h('thead',h('tr',...['Node','Kind','Substitutions/site','Boundary','Source rows'].map(x=>h('th',{scope:'col'},x)))),h('tbody',...nodeRows));
      content.append(h('details',h('summary','Every node and fitted edge'),h('div.table-scroll',{tabIndex:0},nodeTable)));
      if(node){const col=d.panel.columns[site],v=node.posterior[site];content.append(h('h3',{'data-ml-detail':'',tabIndex:-1},`Node ${r.nodes.indexOf(node)+1} · ${node.id} · ${node.kind}`),h('p.small',`Edge ${node.branchLength} substitutions/site (${node.boundary}); ${node.rows} source rows. ${node.sourceNode?`Source node ${node.sourceNode}.`:'No observed annotation assigned.'}`),h('label.field',h('span','Panel site (one-based)'),h('input.input.small',{type:'number',min:1,max:r.sites,value:site+1,'aria-label':'Likelihood panel site',onchange:e=>{const n=Number(e.target.value);if(Number.isInteger(n)&&n>=1&&n<=r.sites){site=n-1;render();}}})),h('p',`${col.segment} reference position ${col.position}. Conditional state probabilities:`),h('table.data',h('thead',h('tr',...['A','C','G','T'].map(x=>h('th',{scope:'col'},x)))),h('tbody',h('tr',...v.map(x=>h('td',format(x)))))),h('p.small','A/C/G/T probabilities condition on the selected tree, fitted lengths and fixed reference. They omit topology, branch-length, reference and alignment uncertainty. Marginal modes are not a joint ancestral reconstruction.'));}
      content.append(h('details',h('summary','Fit diagnostics, provenance and scientific limits'),h('pre',{style:{whiteSpace:'pre-wrap',overflowWrap:'anywhere',maxHeight:'240px',overflow:'auto'}},`${d.citation}\n\n${d.notice}\n\n${d.limits}\n\n${JSON.stringify({settings:r.settings,fits:r.fits},null,2)}`)));
      if(d.restricted)content.append(h('label.check',h('input',{type:'checkbox',checked:terms,disabled:busy,onchange:e=>{terms=e.target.checked;render();}}),'I reviewed provider terms and have permission to export these results and source annotations.'));
      content.append(h('button.btn.primary.small',{disabled:busy||(d.restricted&&!terms),onclick:action(save)},'Save exact likelihood evidence'),h('p.small','Save before closing. This separate record includes the original parent, diagnostics and full conditional probabilities. It does not replace parsimony results or add scientific replay. Reviews expire after 30 minutes or when evicted by later reviews.'));
    }
    if(busy)content.append(h('button.btn.small',{onclick:()=>controller?.abort()},'Cancel likelihood operation'));
    if(error)content.append(h('p.callout.danger',{role:'alert'},error));
  }
  const view={run,save,state:()=>({busy,review,error}),close:dialog.close};app.likelihoodView=view;render();return view;
}
