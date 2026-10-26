// Layout only: effect estimates, intervals, summaries and multiplicity are supplied by Go.
const esc = (s) => String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
export function robustnessEffects(result, feature) {
  const index = result.summaries.findIndex((s) => s.feature.id === feature);
  if (index < 0) return { svg: '', description: '' };
  const summary = result.summaries[index];
  const rows = result.variants.map((v, i) => ({ name: `${i === 0 ? 'Baseline: ' : ''}${v.definition.name || `${v.definition.junction.toUpperCase()} · V ${v.definition.v} · J ${v.definition.j}`}`, effect: v.analysis?.comparisons[index]?.mean, reason: v.failure }));
  const values = [0, ...rows.flatMap((r) => [r.effect?.estimate, r.effect?.lower, r.effect?.upper].filter((v) => v != null))];
  const low = Math.min(...values), high = Math.max(...values), pad = (high - low || 1) * .08;
  const min = low - pad, max = high + pad, x = (v) => 290 + (v - min) * 540 / (max - min);
  const height = 110 + rows.length * 48, axis = height - 65;
  const description = `${summary.feature.name}; comparison minus reference (${summary.feature.unit}). ${summary.direction}. ${summary.uncertainty}. Declared order, not sorted by p-value. Each row reuses the same biological subjects, not independent replication. ${100 * result.options.confidence}% marginal t intervals, not simultaneous or BY-adjusted. An unavailable interval is not a zero-width interval. The range of estimates is descriptive, not a confidence interval.`;
  let body = `<path d="M290 ${axis}H830" class="plot-axis" fill="none"/><path d="M${x(0)} 15V${axis}" class="plot-guide" fill="none"/>`;
  rows.forEach((r, i) => {
    const y = 35 + i * 48, e = r.effect;
    body += `<text class="plot-label" x="12" y="${y + 4}"><title>${esc(r.name)}</title>${esc(r.name.length > 35 ? r.name.slice(0, 32) + '…' : r.name)}</text>`;
    if (e?.lower != null && e?.upper != null) body += `<path data-definition="${i}" data-lower="${e.lower}" data-upper="${e.upper}" d="M${x(e.lower)} ${y}H${x(e.upper)}M${x(e.lower)} ${y-5}V${y+5}M${x(e.upper)} ${y-5}V${y+5}" fill="none" stroke="currentColor" stroke-width="2"/>`;
    if (e?.estimate != null) body += `<circle data-estimate="${e.estimate}" cx="${x(e.estimate)}" cy="${y}" r="5" class="${i === 0 ? 'plot-s2' : 'plot-s1'}"><title>${esc(`${r.name}: ${e.estimate} [${e.lower ?? 'unavailable'}, ${e.upper ?? 'unavailable'}]`)}</title></circle>`;
    if (e?.lower == null || e?.upper == null) body += `<text class="plot-tick" x="290" y="${y + 20}">${esc((r.reason || e?.reason || 'Interval unavailable').slice(0, 85))}</text>`;
  });
  for (let i = 0; i <= 4; i++) { const v = min + (max - min) * i / 4; body += `<text class="plot-tick" x="${x(v)}" y="${axis + 23}" text-anchor="middle">${v.toPrecision(4)}</text>`; }
  body += `<text class="plot-label" x="560" y="${height - 10}" text-anchor="middle">Mean difference (${esc(summary.feature.unit)})</text>`;
  return { svg: `<svg class="plot" role="img" aria-label="Definition sensitivity of mean differences" viewBox="0 0 880 ${height}" width="880" height="${height}"><title>Definition sensitivity</title><desc>${esc(description)}</desc>${body}</svg>`, description };
}
