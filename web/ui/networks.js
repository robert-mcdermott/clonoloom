// The drawing is a bounded view of the full graph. Component selection never
// recomputes scientific membership in the browser.
import { ORGANISMS } from '../lib/specificity.js';
import { UNITS } from '../lib/datasets.js';
import { h } from './dom.js';
import { METRICS, networkWords } from '../lib/networks.js';

export function networkControls(o, choose) {
  const select = (name, values, value, change) => h('label.kv-inline', h('span.muted', name), h('select.input.small', { 'aria-label': name, onchange: (e) => change(e.target.value) }, ...Object.entries(values).map(([k, v]) => h('option', { value: k, selected: value === k }, v))));
  return h('div.options-row',
    select('Metric', METRICS, o.metric, (metric) => choose({ metric, cutoff: metric === 'tcrdist' ? 24 : metric === 'identity' ? 0 : 1 })),
    h('label.kv-inline', h('span.muted', 'Cutoff'), h('input.input.small', { type: 'number', min: 0, max: o.metric === 'tcrdist' ? 1000 : o.metric === 'identity' ? 0 : 2, step: 1, value: o.cutoff, 'aria-label': 'Network cutoff', onchange: (e) => choose({ cutoff: Number(e.target.value) }) })),
    ...(o.metric === 'tcrdist' && !o.cdr3Only ? [select('Organism', ORGANISMS, o.organism, (organism) => choose({ organism }))] : []),
    select('Chains', { all: 'All retained chains', any: 'Any shared chain' }, o.arms, (arms) => choose({ arms })),
    ...[['sameV', 'Same V gene'], ['sameJ', 'Same J gene'], ...(o.metric === 'tcrdist' ? [['cdr3Only', 'CDR3s alone']] : [])].map(([key, label]) => h('label.check', h('input', { type: 'checkbox', checked: o[key], onchange: (e) => choose({ [key]: e.target.checked }) }), label)));
}

export function networkResult(r, selected, select, showClonotype) {
  const graph = r.graph;
  const component = graph.components.find((c) => c.id === selected) ?? graph.components[0];
  const canvas = h('canvas', { width: 1200, height: 650, role: 'img', 'aria-label': 'Clonotype network; select a cluster in the table below', style: { width: '100%', maxHeight: '650px', objectFit: 'contain' } });
  const ctx = canvas.getContext('2d');
  const style = getComputedStyle(document.documentElement);
  const normal = style.getPropertyValue('--text-3').trim();
  const chosenColor = style.getPropertyValue('--accent-text').trim();
  const scale = Math.min(1160 / Math.max(1, r.drawing.width), 610 / Math.max(1, r.drawing.height));
  const points = new Map(r.drawing.points.map((p) => [p.node, { x: 20 + p.x * scale, y: 20 + p.y * scale }]));
  ctx.strokeStyle = normal; ctx.globalAlpha = 0.25;
  ctx.beginPath();
  for (const e of graph.edges) {
    const a = points.get(e.a); const b = points.get(e.b);
    if (a && b) { ctx.moveTo(a.x, a.y); ctx.lineTo(b.x, b.y); }
  }
  ctx.stroke(); ctx.globalAlpha = 1;
  for (const [n, p] of points) {
    ctx.fillStyle = graph.membership[n] === component?.id ? chosenColor : normal;
    ctx.beginPath(); ctx.arc(p.x, p.y, Math.min(10, 2 + Math.log1p(r.clonotypes[n].total)), 0, 2 * Math.PI); ctx.fill();
  }
  canvas.addEventListener('click', (event) => {
    const rect = canvas.getBoundingClientRect();
    const x = (event.clientX - rect.left) * canvas.width / rect.width;
    const y = (event.clientY - rect.top) * canvas.height / rect.height;
    let best = 144; let chosen = null;
    for (const [n, p] of points) { const d = (x - p.x) ** 2 + (y - p.y) ** 2; if (d < best) { best = d; chosen = n; } }
    if (chosen !== null) select(graph.membership[chosen]);
  });
  const pageStart = Math.floor(((component?.id ?? 1) - 1) / 200) * 200;
  const visible = graph.components.slice(pageStart, pageStart + 200);
  const excluded = Object.entries(graph.excluded).map(([why, n]) => `${n} ${why}`).join('; ');
  const picker = h('div.options-row',
    h('label.kv-inline', h('span.muted', 'Cluster'), h('select.input', { 'aria-label': 'Cluster', onchange: (e) => select(Number(e.target.value)) }, ...visible.map((c) => h('option', { value: c.id, selected: c.id === component?.id }, `${c.id}: ${c.nodes.length} clonotypes · ${c.abundance} ${UNITS[r.unit] ?? r.unit} · ${c.samples.length} samples · ${c.subjects.length} subjects`)))),
    graph.components.length > 200 ? h('label.kv-inline', h('span.muted', 'Cluster number'), h('input.input.small', { type: 'number', min: 1, max: graph.components.length, value: component?.id ?? 1, 'aria-label': 'Cluster number', onchange: (e) => select(Math.max(1, Math.min(graph.components.length, Math.round(Number(e.target.value) || 1)))) })) : null,
  );
  return h('div',
    h('p', `${r.clonotypes.length.toLocaleString()} clonotypes, ${graph.edges.length.toLocaleString()} edges, ${graph.components.length.toLocaleString()} connected components. ${networkWords(r.networks)}. Clonotypes: ${r.describe}.`),
    h('p.muted.small', 'Connected components describe sequence similarity, not proven shared specificity. Drawing positions have no distance interpretation. Nodes are sized by abundance.'),
    excluded ? h('p.muted.small', `Not compared; retained as isolated nodes: ${excluded}.`) : null,
    canvas,
    r.drawing.omitted ? h('p.muted.small', `${r.drawing.omitted.toLocaleString()} nodes omitted from the drawing; every node and edge remains in the tables and export.`) : null,
    picker,
    component ? h('div', h('p.muted.small', `Samples: ${component.samples.map((s) => r.samples[s]).join(', ')}. Subjects: ${component.subjects.join(', ') || 'not annotated'}. Showing up to 100 members; export includes every member.`),
      h('div.table-scroll', h('table.data', h('thead', h('tr', h('th', 'Clonotype junctions'), h('th.r', 'Abundance'), h('th.r', 'Samples'))), h('tbody', ...component.nodes.slice(0, 100).map((n) => {
        const c = r.clonotypes[n];
        return h('tr', h('td', h('button.btn.small', { type: 'button', onclick: () => showClonotype(c.id) }, c.chains.map((ch) => ch.junction).join('; '))), h('td.r', String(c.total)), h('td.r', String(c.samples)));
      }))))) : null);
}
