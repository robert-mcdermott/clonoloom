// Go supplies counts and expectations. JS only places bars/markers on a shared linear scale.
const esc=(s)=>String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
export function congaPlot(node, tests) {
  const rows=tests.filter((t)=>t.logP!=null);
  if(!node||!rows.length)return '<p>No testable neighborhood sizes; no overlap plot.</p>';
  const ymax=Math.max(1,...rows.flatMap((t)=>[t.overlap,t.expected]));
  const y=(v)=>235-v*195/ymax,step=500/rows.length,width=Math.min(46,step*.65),x=(i)=>64+step*(i+.5);
  return `<svg class="plot conga-plot" viewBox="0 0 620 300" width="620" height="300" role="img" aria-label="Observed and expected shared neighbors"><title>${esc(node.id)}: neighborhood overlap</title><desc>Each bar is the observed number of shared neighbors at its requested k. Dashed markers show conditional expected counts. The vertical count axis starts at zero. Ties can enlarge neighborhoods. This is not an uncertainty interval.</desc>
  ${rows.map((t,i)=>`<rect class="plot-s1" data-observed="${t.overlap}" x="${x(i)-width/2}" y="${y(t.overlap)}" width="${width}" height="${235-y(t.overlap)}"><title>k ${t.k}: ${t.overlap} observed, ${t.expected} expected</title></rect><path data-expected="${t.expected}" d="M${x(i)-width/2-5} ${y(t.expected)}h${width+10}" stroke="currentColor" stroke-width="3" stroke-dasharray="5 3"/><text class="plot-label" x="${x(i)}" y="256" text-anchor="middle">${t.k}</text>`).join('')}
  <path d="M64 40V235H564" fill="none" stroke="currentColor"/><text class="plot-label" x="56" y="239" text-anchor="end">0</text><text class="plot-label" x="56" y="44" text-anchor="end">${ymax.toPrecision(4)}</text>
  <text class="plot-label" x="314" y="282" text-anchor="middle">Requested neighborhood size k (ties included)</text><text class="plot-label" transform="translate(16 137) rotate(-90)" text-anchor="middle">Shared neighbors (count)</text><text class="plot-label" x="64" y="20">Solid: observed · dashed: conditional expectation</text></svg>`;
}
