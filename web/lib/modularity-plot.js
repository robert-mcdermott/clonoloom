// Display only: Go supplies every observed edge, null frequency, expectation and test value.
const esc = (s) => String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
export function modularityPlot(clone, nullModel) {
  if (!clone || !nullModel) return '';
  if (nullModel.histogram.length > 2000) return '<p>More than 2000 distinct edge counts; use the null-frequency table or complete JSON.</p>';
  const bins = nullModel.histogram, maximum = Math.max(1, clone.observed, Math.ceil(nullModel.expected), ...bins.map((b) => b.edges));
  const ymax = Math.max(1, ...bins.map((b) => b.count));
  const x = (v) => 64 + (v + .5) * 500 / (maximum + 1), y = (v) => 235 - v * 195 / ymax;
  const width = 500 / (maximum + 1) * .9;
  const ticks = [...new Set([0, Math.round(maximum / 2), Math.ceil(maximum)])];
  const desc = `${nullModel.method} size-${clone.cells} null, ${nullModel.draws} subsets. Integer edge counts on the horizontal axis; subset frequencies on the vertical axis, starting at zero. Solid line: ${clone.observed} observed edges. Dashed line: ${nullModel.expected} analytic expected edges. This is a null distribution, not an uncertainty interval for a biological effect.`;
  return `<svg class="plot modularity-plot" viewBox="0 0 620 300" width="620" height="300" role="img" aria-label="Observed edges and size-matched null"><title>${esc(clone.id)}: observed edges and null</title><desc>${esc(desc)}</desc>
    ${bins.map((b) => `<rect class="plot-s1" x="${x(b.edges)-width/2}" y="${y(b.count)}" width="${width}" height="${235-y(b.count)}"><title>${b.edges} edges: ${b.count} subsets</title></rect>`).join('')}
    <path d="M64 40V235H564" fill="none" stroke="currentColor"/>
    <path data-expected="${nullModel.expected}" d="M${x(nullModel.expected)} 35V235" stroke="currentColor" stroke-width="2" stroke-dasharray="5 4"/>
    <path data-observed="${clone.observed}" d="M${x(clone.observed)} 35V235" stroke="currentColor" stroke-width="3"/>
    ${ticks.map((v) => `<text class="plot-label" x="${x(v)}" y="255" text-anchor="middle">${v}</text>`).join('')}
    <text class="plot-label" x="56" y="239" text-anchor="end">0</text><text class="plot-label" x="56" y="44" text-anchor="end">${ymax}</text>
    <text class="plot-label" x="314" y="282" text-anchor="middle">Internal undirected edges (integer counts)</text>
    <text class="plot-label" transform="translate(16 137) rotate(-90)" text-anchor="middle">Null subsets (frequency)</text>
    <text class="plot-label" x="64" y="20">Solid: observed ${clone.observed} · dashed: expected ${nullModel.expected.toPrecision(4)}</text></svg>`;
}
