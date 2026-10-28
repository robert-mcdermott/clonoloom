// Display only: Go supplies observations, pairing, effects, intervals and sample counts.
// Horizontal jitter is deterministic and has no biological meaning. No bars, inferred
// densities, per-cell replication or parent bootstrap error bars are introduced.
const esc = (s) => String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
const fmt = (v) => Number(v).toPrecision(4);
const wrap = (label, desc, body, height = 330) => `<svg class="plot" viewBox="0 0 640 ${height}" width="640" height="${height}" role="img" aria-label="${esc(label)}"><title>${esc(label)}</title><desc>${esc(desc)}</desc>${body}</svg>`;
export function cohortPoints(c, result) {
  const points = c.points.filter((p) => p.value != null && !p.missing);
  if (!points.length || points.length > 500) return { svg: '', description: points.length ? 'More than 500 observations: use the full observation table or export; no subset is drawn.' : 'No available observations to plot.' };
  const values = points.map((p) => p.value), low = Math.min(...values), high = Math.max(...values), pad = (high - low || Math.max(1, Math.abs(low))) * .08;
  const min = result.zeroBaseline ? 0 : low - pad, max = high + pad, y = (v) => 260 - (v - min) * 220 / (max - min);
  const subjects = [...new Set(points.map((p) => p.subject))].sort();
  const x = (p) => (p.group === result.levels[0] ? 210 : 450) + ((subjects.indexOf(p.subject) * 7 % 17) - 8) * 3;
  const desc = `${c.feature.name} (${c.feature.unit}); ${result.layout}. Each point is one saved subject/group observation; ${c.nReference} selected reference observations and ${c.nComparison} comparison observations, ${c.pairs} pairs, ${c.missing} missing values. Lines join only the same subject in paired designs. Horizontal jitter separates overlaps and carries no quantitative information. No uncertainty bars on observations.`;
  let body = `<path d="M80 40V260H590" class="plot-axis" fill="none"/>`;
  for (let i = 0; i <= 4; i++) { const v = min + (max - min) * i / 4; body += `<path d="M80 ${y(v)}H590" class="plot-gridline"/><text class="plot-tick" x="73" y="${y(v)+4}" text-anchor="end">${fmt(v)}</text>`; }
  if (result.layout === 'paired') for (const subject of subjects) {
    const pair = points.filter((p) => p.subject === subject);
    if (pair.length === 2) body += `<path data-subject="${esc(subject)}" d="M${x(pair[0])} ${y(pair[0].value)}L${x(pair[1])} ${y(pair[1].value)}" class="plot-line plot-s6"><title>${esc(subject)} paired observations</title></path>`;
  }
  for (const p of points) body += `<circle data-sample="${esc(p.sample)}" cx="${x(p)}" cy="${y(p.value)}" r="4.5" class="${p.group === result.levels[0] ? 'plot-s1' : 'plot-s2'}"><title>${esc(`${p.subject}; ${p.sample}; ${p.group}: ${p.value} ${c.feature.unit}`)}</title></circle>`;
  for (let i = 0; i < 2; i++) body += `<text class="plot-label" x="${i ? 450 : 210}" y="285" text-anchor="middle">${esc(result.levels[i].slice(0, 28))}</text><text class="plot-tick" x="${i ? 450 : 210}" y="306" text-anchor="middle">n=${i ? c.nComparison : c.nReference} selected observations</text>`;
  body += `<text class="plot-label" x="80" y="22">${esc(c.feature.name)} (${esc(c.feature.unit)})</text>`;
  return { svg: wrap('Subject observations', desc, body), description: desc };
}
export function cohortEffect(c, result) {
  const effect = c.mean, desc = `${effect.name}: ${effect.estimate ?? 'missing'} ${effect.unit}. ${result.options.confidence*100}% ${effect.interval}. ${effect.reason || 'Mean effect across independent subjects or paired subject differences.'} Zero reference line; this is an effect interval, not a distribution of observations.`;
  if (effect.lower == null || effect.upper == null) return { svg: '', description: desc };
  const low = Math.min(0, effect.lower), high = Math.max(0, effect.upper), pad = (high-low || 1)*.1;
  const min = low-pad, max=high+pad, x=(v)=>80+(v-min)*510/(max-min);
  let body = `<path d="M80 115H590" class="plot-axis"/><path d="M${x(0)} 30V115" stroke="currentColor" stroke-width="1" stroke-dasharray="5 4"/>
    <path data-lower="${effect.lower}" data-upper="${effect.upper}" d="M${x(effect.lower)} 70H${x(effect.upper)}M${x(effect.lower)} 62V78M${x(effect.upper)} 62V78" stroke="currentColor" fill="none" stroke-width="2"/>
    <circle data-estimate="${effect.estimate}" cx="${x(effect.estimate)}" cy="70" r="5" class="plot-s1"/>`;
  for (let i=0;i<=4;i++){const v=min+(max-min)*i/4;body+=`<text class="plot-tick" x="${x(v)}" y="137" text-anchor="middle">${fmt(v)}</text>`;}
  body+=`<text class="plot-label" x="80" y="20">Mean difference with ${result.options.confidence*100}% t interval</text><text class="plot-label" x="335" y="162" text-anchor="middle">Comparison minus reference (${esc(effect.unit)})</text>`;
  return { svg: wrap('Mean difference and interval',desc,body,180), description:desc };
}
