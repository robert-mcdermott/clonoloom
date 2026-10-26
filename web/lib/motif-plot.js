// Scientific counts and frequencies come from Go. This module only places marks.
const esc=(s)=>String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
export function motifProfile(profile,group){
 if(!profile?.n)return {svg:'',description:'No query members; residue frequencies are unavailable.'};
 const cell=18,x0=65,y0=28,width=Math.max(560,x0+group.length*cell+25),height=450;
 let body='';
 for(let a=0;a<profile.alphabet.length;a++){body+=`<text class="plot-label" x="42" y="${y0+a*cell+13}" text-anchor="end">${profile.alphabet[a]}</text>`;for(let p=0;p<group.length;p++){const f=profile.frequencies[p][a],n=profile.counts[p][a];body+=`<rect data-position="${p+1}" data-residue="${profile.alphabet[a]}" data-count="${n}" x="${x0+p*cell}" y="${y0+a*cell}" width="16" height="16" fill="var(--accent-text)" opacity="${f}"><title>${esc(`${profile.alphabet[a]} at junction position ${p+1}: ${n}/${profile.n} (${(100*f).toFixed(1)}%)`)}</title></rect>`;}}
 for(let p=0;p<group.length;p++)body+=`<text class="plot-tick" x="${x0+p*cell+8}" y="${y0+20*cell+20}" text-anchor="middle">${p+1}</text>`;
 const description=`Residue frequency among all ${profile.n} unique query V-gene/junction members. Full amino-acid junction positions, one-based from the conserved cysteine. Opacity is frequency on a fixed 0–100% scale, not information content. Conserved positions follow from selecting this signature and are not independent evidence of specificity. No alignment, abundance weighting or subsampling.`;
 body+=`<text class="plot-label" x="${width/2}" y="${height-12}" text-anchor="middle">Junction position · residue frequency 0% (empty) to 100% (solid)</text>`;
 return {svg:`<svg class="plot" role="img" aria-label="Query member residue frequencies" viewBox="0 0 ${width} ${height}" width="${width}" height="${height}"><title>Query member residue frequencies</title><desc>${esc(description)}</desc>${body}</svg>`,description};
}
