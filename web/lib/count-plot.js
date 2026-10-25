// Layout only. Go supplies model coefficients, observed CPM, intervals and fit status.
import { cohortPoints } from './cohort-plot.js';
const esc=(s)=>String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
export function countMA(result, selected='') {
  const rows=result.rows.filter((r)=>r.log2Fold!=null&&r.meanCPM>0), failed=result.family-result.tested;
  const description=`${rows.length} fitted clonotypes; ${failed} eligible but unavailable. Each point is one clonotype, not a biological replicate. X is mean observed counts per million assigned counts on a logarithmic scale; Y is the fitted comparison/reference log2 ratio. No p-value threshold or discovery colors. Unavailable fits remain in the table and BH family.`;
  if(!rows.length)return {svg:'',description};
  const logs=rows.map((r)=>Math.log10(r.meanCPM)),lo=Math.floor(Math.min(...logs)),hi=Math.max(lo+1,Math.ceil(Math.max(...logs)));
  const range=Math.max(1,...rows.map((r)=>Math.abs(r.log2Fold)))*1.1,x=(v)=>70+(Math.log10(v)-lo)*500/(hi-lo),y=(v)=>145-v*110/range;
  let body='<path d="M70 30V260H570" class="plot-axis" fill="none"/>';
  for(let i=-2;i<=2;i++){const v=range*i/2;body+=`<path d="M70 ${y(v)}H570" class="${i===0?'plot-guide':'plot-gridline'}"/><text class="plot-tick" x="63" y="${y(v)+4}" text-anchor="end">${v.toPrecision(3)}</text>`;}
  for(let i=0;i<=4;i++){const v=lo+(hi-lo)*i/4;body+=`<text class="plot-tick" x="${70+i*125}" y="280" text-anchor="middle">${(10**v).toPrecision(3)}</text>`;}
  for(const r of rows)body+=`<circle data-clone="${esc(r.id)}" cx="${x(r.meanCPM)}" cy="${y(r.log2Fold)}" r="${r.id===selected?6:3.5}" class="${r.id===selected?'plot-s2':'plot-s1'}"><title>${esc(`${r.id}; mean CPM ${r.meanCPM}; log2 ratio ${r.log2Fold}`)}</title></circle>`;
  body+='<text class="plot-label" x="70" y="18">Fitted log2 ratio (comparison / reference)</text><text class="plot-label" x="320" y="307" text-anchor="middle">Mean observed counts per million (log scale)</text>';
  return {svg:`<svg class="plot" role="img" aria-label="Abundance and fitted effects" viewBox="0 0 640 330" width="640" height="330"><title>Abundance and fitted effects</title><desc>${esc(description)}</desc>${body}</svg>`,description};
}
export function countObservation(row, result, design) {
  const c={feature:{name:'Observed clonotype abundance',unit:'counts per million assigned counts'},points:result.design.rows.map((s,i)=>({subject:s.subject,sample:s.name,group:s.group,value:row.cpm[i],missing:''})),nReference:design.groups[0].subjects,nComparison:design.groups[1].subjects,pairs:design.completePairs,missing:0};
  return cohortPoints(c,{layout:design.input.spec.layout,levels:design.input.spec.levels,zeroBaseline:true});
}
export function countEffect(row,result) {
  const description=`Fitted comparison/reference log2 abundance ratio ${row.log2Fold??'unavailable'}; ${100*result.options.confidence}% marginal Wald normal interval [${row.lower??'unavailable'}, ${row.upper??'unavailable'}], conditional on estimated dispersion, not BH-adjusted. ${row.reason||'Zero denotes equal modeled relative abundance; this is not an observation distribution.'}`;
  if(row.lower==null||row.upper==null)return {svg:'',description};
  const lo=Math.min(0,row.lower),hi=Math.max(0,row.upper),pad=(hi-lo||1)*.1,min=lo-pad,max=hi+pad,x=(v)=>80+(v-min)*510/(max-min);
  let body=`<path d="M80 115H590" class="plot-axis" fill="none"/><path d="M${x(0)} 30V115" stroke="currentColor" stroke-dasharray="5 4"/>
    <path data-lower="${row.lower}" data-upper="${row.upper}" d="M${x(row.lower)} 70H${x(row.upper)}M${x(row.lower)} 62V78M${x(row.upper)} 62V78" stroke="currentColor" fill="none" stroke-width="2"/>
    <circle data-estimate="${row.log2Fold}" cx="${x(row.log2Fold)}" cy="70" r="5" class="plot-s1"/>`;
  for(let i=0;i<=4;i++){const v=min+(max-min)*i/4;body+=`<text class="plot-tick" x="${x(v)}" y="137" text-anchor="middle">${v.toPrecision(4)}</text>`;}
  body+=`<text class="plot-label" x="80" y="20">Log2 ratio with ${100*result.options.confidence}% Wald interval</text><text class="plot-label" x="335" y="162" text-anchor="middle">Comparison / reference (log2 ratio)</text>`;
  return {svg:`<svg class="plot" role="img" aria-label="Model log2 ratio and interval" viewBox="0 0 640 180" width="640" height="180"><title>Model log2 ratio and interval</title><desc>${esc(description)}</desc>${body}</svg>`,description};
}
