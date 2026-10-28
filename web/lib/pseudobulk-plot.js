// Geometry only; cell counts, group membership and eligibility are supplied by Go.
const esc=(s)=>String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
export function pseudobulkPlot(input,result,type){
  const groups=result.groups.filter((g)=>g.type===type);
  const description=`Cells contributing to each sample × size category for supplied ${input.options.annotation}: ${type}. Each column is one declared size category; every panel uses the same zero-based linear cell-count scale. Purple: meets the evidence rule; gray: below minimum or zero library. Absent groups have no bar and unavailable expression counts. Dashed lines mark the minimum of ${input.options.minCells} cells, not statistical significance. Samples and literal timepoints are separate; repeated subjects are not independent. These are coverage counts, not expression differences or biological replicates.`;
  if(!groups.length)return {svg:'',description};
  const max=Math.ceil(Math.max(input.options.minCells,...groups.map((g)=>g.cells))/20)*20,height=105+input.samples.length*36,width=180+input.bins.length*135;
  let body='';
  input.samples.forEach((s,i)=>{const a=s.annotation,label=a.name;body+=`<text class="plot-label" x="140" y="${58+i*36}" text-anchor="end"><title>${esc(a.subject)} · ${esc(a.timepoint)}</title>${esc(label.length>20?label.slice(0,19)+'…':label)}</text>`;});
  input.bins.forEach((bin,bi)=>{const origin=155+bi*135,x=(v)=>origin+96*v/max;
    body+=`<text class="plot-label" x="${origin+48}" y="20" text-anchor="middle">${esc(bin)} ${bin==='1'?'cell':'cells'}/clone</text>`;
    for(let k=0;k<=2;k++){const value=max*k/2;body+=`<path class="plot-gridline" d="M${x(value)} 35V${height-48}"/><text class="plot-tick" x="${x(value)}" y="${height-29}" text-anchor="middle">${Number(value.toFixed(1))}</text>`;}
    body+=`<path d="M${x(input.options.minCells)} 35V${height-48}" stroke="currentColor" stroke-dasharray="4 3"/>`;
    for(const g of groups.filter((g)=>g.bin===bin)){const a=input.samples[g.sample].annotation,y=44+g.sample*36;if(g.cells)body+=`<rect class="plot-s${g.eligible?1:6}" data-sample="${esc(a.name)}" data-cells="${g.cells}" x="${origin}" y="${y}" width="${x(g.cells)-origin}" height="18"><title>${esc(a.name)} · ${esc(bin)}: ${g.cells} cells; ${esc(g.reason||'Eligible')}</title></rect>`;body+=`<text class="plot-tick" x="${x(Math.max(g.cells,input.options.minCells))+3}" y="${y+13}">${g.cells||'Absent'}</text>`;}
  });
  body+=`<text class="plot-label" x="${width/2}" y="${height-4}" text-anchor="middle">Contributing cells per sample and category (not replicates)</text>`;
  return {svg:`<svg class="plot" role="img" aria-label="Pseudobulk cell coverage" width="${width}" height="${height}" viewBox="0 0 ${width} ${height}"><title>Pseudobulk cell coverage</title><desc>${esc(description)}</desc>${body}</svg>`,description};
}
