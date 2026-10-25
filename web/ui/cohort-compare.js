// Saved subject-level comparisons. Go owns membership, tests, effects and intervals.
import { h, clear, downloadBlob } from './dom.js';
import { confirmDialog } from './overlays.js';
import { addResult } from '../lib/results.js';
import { cohortPoints, cohortEffect } from '../lib/cohort-plot.js';
import { figureButton, madeBy } from './figures.js';
async function json(url, body) {
  const r = await fetch(url, body === undefined ? {cache:'no-store'} : {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});
  const v = await r.json(); if (!r.ok) throw new Error(v.error ?? `HTTP ${r.status}`); return v;
}
export function mountCohortCompare(app, container) {
  const {store} = app, root = h('section.card.cohort-compare'); container.append(root);
  let serial=0,generation=store.state.generation,parent='',preview=null,selected=['shannon'],answer=null,loaded='',status='idle',error='',job='',metric='',offset=0;
  const draws=h('input.input.small',{type:'number',min:999,max:99999,step:1,value:9999,'aria-label':'Comparison permutations'});
  const seed=h('input.input.small',{type:'number',min:1,max:Number.MAX_SAFE_INTEGER,step:1,value:20261025,'aria-label':'Comparison seed'});
  const level=h('select.input.small',{'aria-label':'Comparison confidence'},...[.9,.95,.99].map((v)=>h('option',{value:v,selected:v===.95},`${100*v}%`)));
  const busy=()=>['loading','running'].includes(status);
  const button=(label,action,disabled=false)=>h('button.btn.small',{type:'button',disabled,onclick:()=>Promise.resolve().then(action).catch((e)=>{error=e.message;draw();})},label);
  const number=(v)=>v==null?'Unavailable':Number(v).toPrecision(5);
  const table=(caption,heads,rows)=>h('div.table-wrap',{tabIndex:0},h('table.data',h('caption',caption),h('thead',h('tr',...heads.map((v)=>h('th',{scope:'col'},v)))),h('tbody',...rows.map((r)=>h('tr',...r.map((v)=>h('td',v)))))));
  function received(value){answer=value;loaded=value.artifact.id;status='ready';job='';offset=0;metric=value.result.analysis.comparisons[0]?.feature.id??'';draw();root.scrollIntoView({block:'start'});}
  async function choose(id){const token=++serial,same=store.sameProject();parent=id;preview=null;status=id?'loading':'idle';error='';draw();if(!id)return;try{const v=await json(`api/cohort-features/${encodeURIComponent(id)}`);if(token!==serial||!same())return;preview=v.result;selected=['shannon'];status='idle';draw();}catch(e){if(token===serial&&same()){error=e.message;status='failed';draw();}throw e;}}
  async function show(id){const token=++serial,same=store.sameProject();answer=null;loaded='';status='loading';error='';draw();try{const v=await json(`api/cohort-compare/${encodeURIComponent(id)}`);if(token!==serial||!same())return;received(v);return v;}catch(e){if(token===serial&&same()){error=e.message;status='failed';draw();}throw e;}}
  async function run(options={}){
    if(busy())throw new Error('Wait for the current comparison.');
    const source=options.features??parent;if(!store.project.artifacts.some((r)=>r.id===source&&r.kind==='cohort-features'))throw new Error('Choose a saved subject feature table.');
    const settings=options.options??{features:selected,confidence:Number(level.value),permutations:Number(draws.value),seed:Number(seed.value)};
    const token=++serial,same=store.sameProject();status='loading';error='';answer=null;loaded='';job='';draw();
    try{
      const start=await json('api/cohort-compare',{features:source,options:settings});if(token!==serial||!same())return;
      if(!start.ready){job=start.job;status='running';draw();const done=await app.jobs.finished(job);if(token!==serial||!same())return;if(done?.state!=='complete')throw new Error(done?.error||`Comparison ${done?.state??'unavailable'}.`);}
      const value=await json(`api/cohort-compare/${encodeURIComponent(start.artifact)}`);if(token!==serial||!same())return;received(value);
      if(!store.project.artifacts.some((r)=>r.id===loaded))store.commit(addResult(store.project,{id:loaded,kind:'cohort-compare',dataset:value.result.features.parent.dataset,name:value.artifact.name,made:value.artifact.created,method:'Subject-level group comparison',describe:value.result.description,citation:value.artifact.lineage.method.citation}),'Keep group comparison',['project']);draw();return value;
    }catch(e){if(token===serial&&same()){error=e.message;status='failed';job='';draw();}throw e;}
  }
  async function download(){const v=answer,same=store.sameProject();if(!v)return;const restricted=(v.result.features.notice.Credits??[]).filter((c)=>c.LicenseClass!=='open');
    if(restricted.length&&!await confirmDialog({title:'Provider terms apply',message:h('div',h('p','Read these source terms before sharing derived results.'),h('ul',...restricted.map((c)=>h('li',`${c.Title} — ${c.License}`)))),confirm:'Export'}))return;
    if(!same())return;const r=await fetch(`api/cohort-compare/${encodeURIComponent(v.artifact.id)}/download${restricted.length?'?accept=1':''}`);if(!r.ok)throw new Error((await r.json()).error??'Export failed.');const blob=await r.blob();if(same())downloadBlob(blob,`${v.artifact.id}.zip`);
  }
  function plotSpec(which){
    if(!answer)return null;const doc=answer.result,r=doc.analysis,c=r.comparisons.find((c)=>c.feature.id===metric)??r.comparisons[0];
    const p=which==='effect'?cohortEffect(c,r):cohortPoints(c,r);
    return {name:`${loaded}-${c.feature.id}-${which}`,title:`${c.feature.name}: ${which==='effect'?'mean difference and interval':'subject observations'}`,plot:p.svg,
      legend:which==='effect'?[]:[{label:r.levels[0],className:'plot-s1'},{label:r.levels[1],className:'plot-s2'}],caption:[p.description,`Contrast: ${r.levels[1]} minus ${r.levels[0]}; ${r.layout}. Selected observations: ${c.nReference} reference, ${c.nComparison} comparison; ${c.pairs} pairs, ${c.missing} missing. Mean inference: ${c.parametric.name}.`,`Dataset: ${doc.features.parent.datasetName}. ${doc.features.parent.describe}; definition ${doc.features.parent.definitionId}. QC: ${doc.features.notice.Filter}. Abundance: ${doc.features.parent.unit}.`,`Design ${doc.features.selection.name}, version ${doc.features.selection.revision}. Family: ${r.family} test slots; BH does not adjust intervals. No depth standardization or covariate adjustment.`,...doc.features.design.findings.map((f)=>`${f.title}: ${f.detail}`),`Result ${loaded}; feature parent ${doc.parentManifest.id}.`,madeBy(app.version)]};
  }
  function draw(){
    clear(root);draws.disabled=seed.disabled=level.disabled=busy();
    const parents=store.project.artifacts.filter((r)=>r.kind==='cohort-features');
    root.append(h('h2#cohort-comparisons','Group comparisons'),h('p','Compare the saved subject observations under their independent or paired design. Choose the feature family before looking at test results.'),h('h3','Settings for the next comparison'),
      h('label.field',h('span','Comparison feature parent'),h('select.input.small',{'aria-label':'Comparison feature parent',disabled:busy(),onchange:(e)=>choose(e.target.value).catch(()=>{})},h('option',{value:'',selected:!parent},'Choose saved subject features'),...parents.map((r)=>h('option',{value:r.id,selected:r.id===parent},`${r.name} · ${r.id.slice(-6)}`)))));
    if(preview)root.append(h('fieldset',h('legend','Features in the correction family'),...preview.table.catalog.map((f)=>h('label.kv-inline',h('input',{type:'checkbox',checked:selected.includes(f.id),disabled:busy(),'aria-label':`Compare ${f.name}`,onchange:(e)=>{selected=e.target.checked?[...selected,f.id]:selected.filter((x)=>x!==f.id);draw();}}),`${f.name} (${f.unit})`))));
    root.append(h('div.cohort-controls',h('label.field',h('span','Confidence level'),level),h('label.field',h('span','Monte Carlo permutations'),draws),h('label.field',h('span','Seed'),seed)),
      h('p.muted.small','Both the mean test and its rank counterpart enter one BH family for every selected feature. A missing value leaves that feature untested; subjects and pairs are never silently dropped. Exact rank randomization is used for small designs. Confidence intervals are marginal, not adjusted for multiplicity.'),
      button('Compare selected features',()=>run(),busy()||!parent||!preview||!selected.length));
    const saved=store.project.artifacts.filter((r)=>r.kind==='cohort-compare');if(saved.length)root.append(h('div.btn-row',h('span','Saved comparisons'),...saved.map((r)=>button(`${r.name} · ${r.id.slice(-6)}`,()=>show(r.id),busy()))));
    if(job)root.append(button('Cancel comparison',()=>app.jobs.cancel(job)));
    if(error)root.append(h('p.callout.danger',{role:'alert'},error));if(busy())root.append(h('p',{role:'status'},status==='running'?'Testing saved subject observations…':'Reading saved evidence…'));
    if(!answer)return;
    const doc=answer.result,r=doc.analysis,c=r.comparisons.find((c)=>c.feature.id===metric)??r.comparisons[0];
    root.append(h('h3.comparison-result-title',`${doc.features.selection.name}, version ${doc.features.selection.revision}`),h('p',`${doc.features.parent.datasetName} · ${r.layout} · ${r.levels[1]} minus ${r.levels[0]}. ${r.tested} testable tests in ${r.family} declared family slots.`),
      h('p.small',`Definition: ${doc.features.parent.describe}. QC: ${doc.features.notice.Filter}. Abundance: ${doc.features.parent.unit}.`),
      ...doc.features.design.findings.map((f)=>h('div.callout.warn',h('b',f.title),h('p',f.detail))),
      h('p.muted.small','Tests assume independent biological subjects and their stated sampling models. Rank tests do not test merely equality of medians. BH has assumptions about dependence and covers only this selected family; changing features after seeing results creates additional multiplicity.'),
      table('All selected features and adjusted p-values',['Feature','Reference n','Comparison n','Pairs','Mean-test p','Mean-test BH p','Rank-test p','Rank-test BH p'],r.comparisons.map((x)=>[button(x.feature.name,()=>{metric=x.feature.id;offset=0;draw();}),x.nReference,x.nComparison,x.pairs,number(x.parametric.p),number(x.parametric.adjusted),number(x.rank.p),number(x.rank.adjusted)])),
      h('h3',`${c.feature.name} (${c.feature.unit})`),
      table('Effect estimates and distinct interval meanings',['Effect','Estimate','Lower','Upper','Unit','Interval / reason'],[c.mean,c.probability].map((e)=>[e.name,number(e.estimate),number(e.lower),number(e.upper),e.unit,`${100*r.options.confidence}% ${e.interval}${e.reason?`; ${e.reason}`:''}`])),
      h('p.small','The probability interval is conservative and may span 0–1 for a small study. Independent designs compare two independently drawn subjects; paired designs estimate within-subject increase. It is not a median-shift interval or an inversion of the rank test.'),
      table('Test details',['Test','Statistic','df','Raw p','BH p','Randomization / reason'],[c.parametric,c.rank].map((t)=>[t.name,number(t.statistic),number(t.df),number(t.p),number(t.adjusted),t.reason||t.nullMethod&&`${t.nullMethod}; tail ${t.tail} of ${t.draws}${r.layout==='paired'?`; ${t.nonzero} nonzero pairs`:''}`||'Two-sided t model'])));
    for(const which of ['observations','effect']){const spec=plotSpec(which),p=which==='effect'?cohortEffect(c,r):cohortPoints(c,r);if(p.svg){const box=h('div.plot-wrap');box.innerHTML=p.svg;root.append(h('figure.plot-card',h('figcaption',h('span',spec.title),figureButton(()=>plotSpec(which))),box));}else root.append(h('p.muted',p.description));}
    root.append(table('Saved observations (50 per page; all in export)',['Subject','Group','Sample',`Value (${c.feature.unit})`,'Missing reason'],c.points.slice(offset,offset+50).map((p)=>[p.subject,p.group,p.sample,number(p.value),p.missing||'Available'])),
      h('div.btn-row',button('Previous comparison observations',()=>{offset=Math.max(0,offset-50);draw();},offset===0),h('span',`${offset} offset · ${c.points.length} selected observations`),button('Next comparison observations',()=>{offset+=50;draw();},offset+50>=c.points.length),button('Export comparison ZIP',download)),
      h('details',h('summary','Comparison method, assumptions and provenance'),h('p',doc.description),h('p',answer.artifact.lineage.method.citation),h('pre',{style:{whiteSpace:'pre-wrap',overflowWrap:'anywhere'}},JSON.stringify(answer.artifact.lineage.parents,null,2))));
  }
  const view={choose,run,show,download,plotSpec,state:()=>({status,error,answer,artifact:loaded,metric})};app.cohortCompareView=view;
  const focused=()=>store.ui.focus?.kind==='result'&&store.project.artifacts.find((r)=>r.id===store.ui.focus.id&&r.kind==='cohort-compare');
  draw();if(focused())show(focused().id).catch(()=>{});
  return {update(topics){if(generation!==store.state.generation){generation=store.state.generation;serial++;parent='';preview=null;selected=['shannon'];answer=null;loaded='';status='idle';error='';job='';}const target=focused();if(topics.has('focus')&&target&&target.id!==loaded)show(target.id).catch(()=>{});else if(topics.has('project')||topics.has('jobs'))draw();},destroy(){serial++;if(app.cohortCompareView===view)delete app.cohortCompareView;root.remove();}};
}
