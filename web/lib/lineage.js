// Presentation only: scores, topology, branch changes and ancestral states come from Go.
const esc = (s) => String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
export function annotationLabel(node, field) {
  const values = [...new Set((node.annotations ?? []).map((a) => a[field] || 'Missing'))].sort();
  return values.length === 0 ? 'Unobserved' : values.length === 1 ? values[0] : 'Mixed';
}
export function lineagePlot(tree, field = 'isotype') {
  const likelihood=tree.model==='JC69', edgeLabel=n=>likelihood?Number(n.branchLength).toPrecision(3):n.changes;
  const nodes = tree.nodes ?? [], by = new Map(nodes.map((n) => [n.id, n]));
  const children = new Map(nodes.map((n) => [n.id, []]));
  for (const n of nodes) if (n.parent) children.get(n.parent)?.push(n.id);
  const categories = [...new Set(nodes.filter((n) => n.rows).map((n) => annotationLabel(n, field)))].sort();
  const legend = categories.map((label, i) => ({ label, className: `plot-s${i % 6 + 1}` }));
  const maxDepth = Math.max(1, ...nodes.map((n) => n.depth)), y = new Map();
  let leaf = 0;
  function place(id) {
    const kids = children.get(id);
    const v = kids.length ? kids.map(place).reduce((a, b) => a + b, 0) / kids.length : 45 + 42 * leaf++;
    y.set(id, v); return v;
  }
  for (const n of nodes) if (!n.parent) place(n.id);
  const x = (n) => 45 + n.depth / maxDepth * 420;
  const root = nodes.find((n) => !n.parent);
  let body = root ? `<text class="plot-label" x="45" y="18">Reference · ${esc(annotationLabel(root, field))} · ${root.rows} source rows</text>` : '';
  for (const n of nodes) if (n.parent) {
    const p = by.get(n.parent);
    body += `<path class="plot-line plot-s1" d="M${x(p)} ${y.get(p.id)}V${y.get(n.id)}H${x(n)}"/><text class="plot-tick" x="${(x(p) + x(n)) / 2}" y="${y.get(n.id) - 5}">${edgeLabel(n)}</text>`;
  }
  for (const [i, n] of nodes.entries()) {
    const label = n.rows ? annotationLabel(n, field) : n.kind === 'reference' ? 'Reference' : 'Inferred';
    const cls = n.rows ? legend[categories.indexOf(label)].className : 'plot-s1 plot-absent';
    const text = n.kind === 'inferred' || n.kind === 'reference' ? String(i + 1) : `${i + 1} · ${label.length > 28 ? label.slice(0, 27) + '…' : label} · ${n.rows} rows`;
    body += `<g role="button" tabindex="0" data-node="${esc(n.id)}" aria-label="${esc(`Inspect node ${text}`)}"><title>${esc(`${n.id}: ${n.kind}; ${edgeLabel(n)} ${likelihood?'substitutions/site':'changes'}; ${n.rows} source rows`)}</title><circle class="${cls}" cx="${x(n)}" cy="${y.get(n.id)}" r="6"/><text class="plot-label" x="${x(n) + 10}" y="${y.get(n.id) + 15}">${esc(text)}</text></g>`;
  }
  const height = Math.max(150, 42 * leaf + 75);
  const description = likelihood ? 'Selected JC69 baseline topology. Edge labels are fitted expected substitutions/site, not observed change counts or dates. Horizontal position is topological depth, not genetic distance or elapsed time. Zero branches and near ties are unresolved evidence; annotations describe observed rows only.' : 'Selected parsimony topology. Edge labels are changes in one consistent assignment. Horizontal position is topological depth, not elapsed time or mutation distance. Metadata labels describe observed source rows only.';
  body += `<text class="plot-label" x="45" y="${height - 12}">Topological depth · edge labels: ${likelihood?'substitutions/site':'changes'} · select a node to inspect</text>`;
  return { svg: `<svg class="plot lineage-plot" viewBox="0 0 780 ${height}" width="780" height="${height}" role="group" aria-label="Lineage tree"><title>Lineage tree</title><desc>${description}</desc>${body}</svg>`, description, legend };
}
