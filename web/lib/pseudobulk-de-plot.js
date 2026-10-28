// Geometry only: saved Go results supply all expression values and inference.
import { cohortPoints } from './cohort-plot.js';
import { countEffect } from './count-plot.js';
const esc=(s)=>String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
export function geneMA(result, selected='') {
  const rows=result.rows.filter((r)=>r.log2Fold!=null&&r.mean>0), failed=result.family-result.tested;
  const description=`${rows.length} fitted genes; ${failed} eligible but unavailable. Each point is one gene, not a biological replicate. X is mean observed size-factor-normalized raw counts on a logarithmic scale; Y is the fitted comparison/reference log2 ratio. No p-value threshold or discovery colors. Unavailable fits remain in the table and BH family.`;
  if(!rows.length)return {svg:'',description};
  const logs=rows.map((r)=>Math.log10(r.mean)),lo=Math.floor(Math.min(...logs)),hi=Math.max(lo+1,Math.ceil(Math.max(...logs)));
  const range=Math.max(1,...rows.map((r)=>Math.abs(r.log2Fold)))*1.1,x=(v)=>70+(Math.log10(v)-lo)*500/(hi-lo),y=(v)=>145-v*110/range;
  let body='<path d="M70 30V260H570" class="plot-axis" fill="none"/>';
  for(let i=-2;i<=2;i++){const v=range*i/2;body+=`<path d="M70 ${y(v)}H570" class="${i===0?'plot-guide':'plot-gridline'}"/><text class="plot-tick" x="63" y="${y(v)+4}" text-anchor="end">${v.toPrecision(3)}</text>`;}
  for(let i=0;i<=4;i++){const v=lo+(hi-lo)*i/4;body+=`<text class="plot-tick" x="${70+i*125}" y="280" text-anchor="middle">${(10**v).toPrecision(3)}</text>`;}
  for(const r of rows)body+=`<circle data-gene="${esc(r.feature.id)}" cx="${x(r.mean)}" cy="${y(r.log2Fold)}" r="${r.feature.id===selected?6:3.5}" class="${r.feature.id===selected?'plot-s2':'plot-s1'}"><title>${esc(`${r.feature.id}; mean normalized count ${r.mean}; log2 ratio ${r.log2Fold}`)}</title></circle>`;
  body+='<text class="plot-label" x="70" y="18">Fitted log2 ratio (comparison / reference)</text><text class="plot-label" x="320" y="307" text-anchor="middle">Mean observed normalized counts (log scale)</text>';
  return {svg:`<svg class="plot" role="img" aria-label="Abundance and fitted effects" viewBox="0 0 640 330" width="640" height="330"><title>Abundance and fitted effects</title><desc>${esc(description)}</desc>${body}</svg>`,description};
}

export function geneObservation(row,result){
 const c={feature:{name:row.feature.name,unit:'size-factor-normalized raw counts'},points:result.observations.map((s,i)=>({subject:s.subject,sample:s.sample,group:s.bin,value:row.normalized[i],missing:''})),nReference:result.observations.length/2,nComparison:result.observations.length/2,pairs:result.observations.length/2,missing:0};
 const p=cohortPoints(c,{layout:'paired',levels:[result.options.reference,result.options.comparison],zeroBaseline:true});
 p.description+=' Each dot is one subject/category pseudobulk, not a cell. Lines join the same subject at the selected timepoint. Counts are divided by the saved median-of-ratios size factor; no log transform, pseudocount, per-cell averaging or significance color.';return p;
}
export function geneEffect(row,result){return countEffect(row,result);}
