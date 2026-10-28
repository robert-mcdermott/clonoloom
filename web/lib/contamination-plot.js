// Geometry only; Go supplies all counts, fractions, review flags and Pgen context.
const esc=(x)=>String(x).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
export function sharingPlot(result,pair){
 const a=result.evidence.samples[pair.a].sample,b=result.evidence.samples[pair.b].sample;
 const rows=result.matches.filter((m)=>m.a===pair.a&&m.b===pair.b);
 const description=`Every exact nucleotide/V/J match for ${a.name} (${a.subject}) and ${b.name} (${b.subject}); ${rows.length} identities. Fractions use each sample's retained productive TRB abundance, including unevaluable sequence/gene rows. Equal logarithmic axes; only positive shared observations, no pseudocount. Dashed diagonal is equal frequency. Review thresholds are heuristic, not a contamination probability. High-Pgen outlines give model context, not clearance. No source or direction inferred.`;
 if(pair.reason)return {svg:'',description:description+' '+pair.reason};
 if(rows.length>2000)return {svg:'',description:description+' More than 2000 points: use complete exports; no subset drawn.'};
 const minimum=rows.length?Math.min(...rows.flatMap((x)=>[x.fractionA,x.fractionB])):1e-4;
 const low=Math.min(-2,Math.floor(Math.log10(minimum))),x=(f)=>95+320*(Math.log10(f)-low)/-low,y=(f)=>360-320*(Math.log10(f)-low)/-low;
 let body='<path d="M95 40V360H415" fill="none" class="plot-axis"/>';
 for(let i=low;i<=0;i++){const f=10**i;body+=`<path d="M${x(f)} 40V360M95 ${y(f)}H415" class="plot-gridline"/><text class="plot-tick" x="${x(f)}" y="380" text-anchor="middle">10^${i}</text><text class="plot-tick" x="85" y="${y(f)+4}" text-anchor="end">10^${i}</text>`;}
 body+='<path d="M95 360L415 40" stroke="currentColor" stroke-dasharray="4 4" opacity=".5"/>';
 for(const m of rows){const high=m.pgenClass.startsWith('High Pgen');body+=`<circle class="plot-s${m.review?2:1}${high?' plot-absent':''}" data-receptor="${esc(m.receptor)}" data-fraction-a="${m.fractionA}" data-fraction-b="${m.fractionB}" cx="${x(m.fractionA)}" cy="${y(m.fractionB)}" r="${high?6:4}" ${high?'style="fill:none;stroke-width:2"':''}><title>${esc(m.receptor)}: ${m.countA} / ${result.evidence.samples[pair.a].total} vs ${m.countB} / ${result.evidence.samples[pair.b].total}; ${esc(m.pattern)}; ${esc(m.pgenClass)}</title></circle>`;}
 body+=`<text class="plot-label" x="255" y="413" text-anchor="middle">${esc(a.name)} · fraction of retained TRB</text><text class="plot-label" transform="translate(22 200) rotate(-90)" text-anchor="middle">${esc(b.name)} · fraction of retained TRB</text>`;
 return {svg:`<svg class="plot" role="img" aria-label="Shared nucleotide receptor fractions" viewBox="0 0 510 435" width="510" height="435"><title>Shared nucleotide receptor fractions</title><desc>${esc(description)}</desc>${body}</svg>`,description};
}
