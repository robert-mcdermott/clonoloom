// Plots for the views, as SVG markup: horizontal bars (stacked or not) and columns. Pure: each
// takes its data and options and returns { svg, description }, the description saying in words
// what the plot shows (for screen readers, and as the plot's title). Colors are CSS classes the
// page styles (web/styles.css, "Plots"), so plots follow the theme and color-vision setting.

const esc = (s) => String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');

// Tick values for [0, hi]: about n "nice" numbers.
export function ticks(hi, n = 4) {
  if (!(hi > 0)) return [0];
  const raw = hi / n;
  const mag = 10 ** Math.floor(Math.log10(raw));
  const step = [1, 2, 2.5, 5, 10].map((m) => m * mag).find((s) => s >= raw * (1 - 1e-9)) ?? raw;
  const out = [];
  for (let k = 0; k * step <= hi * (1 + 1e-9); k += 1) out.push(Number((k * step).toPrecision(12)));
  return out;
}

const defaultFormat = (v) => (Math.abs(v) >= 1000 ? v.toLocaleString('en-US', { maximumFractionDigits: 0 }) : String(Number(v.toPrecision(4))));

function wrap(width, height, label, description, body) {
  return `<svg class="plot" viewBox="0 0 ${width} ${height}" width="${width}" height="${height}" role="img" aria-label="${esc(label)}"><title>${esc(label)}</title><desc>${esc(description)}</desc>${body}</svg>`;
}

// Horizontal bars, stacked by parts: rows [{ label, parts: { key: value }, title }]; keys [{ key,
// label, className }]. With share, each row's parts are shares (0–1) and the axis is 0–100%.
export function stackedBars(rows, keys, { label = 'Bars', share = false, width = 520, labelWidth = 120, barHeight = 16, gap = 6, format = defaultFormat } = {}) {
  const plotLeft = labelWidth + 8;
  const plotWidth = width - plotLeft - 12;
  const top = 6;
  const height = top + rows.length * (barHeight + gap) + 30;
  const totals = rows.map((r) => keys.reduce((s, k) => s + (r.parts[k.key] ?? 0), 0));
  const max = share ? 1 : Math.max(0, ...totals);
  const x = (v) => (max > 0 ? (v / max) * plotWidth : 0);
  let body = '';
  rows.forEach((row, i) => {
    const y = top + i * (barHeight + gap);
    body += `<text class="plot-label" x="${labelWidth}" y="${y + barHeight / 2 + 4}" text-anchor="end">${esc(truncate(row.label, Math.max(8, Math.floor(labelWidth / 6.5))))}</text>`;
    let at = plotLeft;
    for (const k of keys) {
      const v = row.parts[k.key] ?? 0;
      if (!(v > 0)) continue;
      const w = x(v);
      body += `<rect class="${k.className}" x="${at.toFixed(2)}" y="${y}" width="${Math.max(0.5, w).toFixed(2)}" height="${barHeight}"><title>${esc(`${row.label}: ${k.label} ${share ? `${(100 * v).toFixed(1)}%` : format(v)}`)}</title></rect>`;
      at += w;
    }
  });
  const axisY = top + rows.length * (barHeight + gap);
  body += `<line class="plot-axis" x1="${plotLeft}" x2="${plotLeft + plotWidth}" y1="${axisY}" y2="${axisY}"/>`;
  for (const t of ticks(max, 4)) {
    const tx = plotLeft + x(t);
    body += `<line class="plot-axis" x1="${tx}" x2="${tx}" y1="${axisY}" y2="${axisY + 4}"/><text class="plot-tick" x="${tx}" y="${axisY + 16}" text-anchor="middle">${share ? `${Math.round(100 * t)}%` : format(t)}</text>`;
  }
  const description = `${label}. ${rows.map((r) => `${r.label}: ${keys.filter((k) => (r.parts[k.key] ?? 0) > 0).map((k) => `${k.label} ${share ? `${(100 * r.parts[k.key]).toFixed(1)}%` : format(r.parts[k.key])}`).join(', ')}`).join('; ')}.`;
  return { svg: wrap(width, height, label, description, body), description };
}

// Columns: bins [{ x, y }] with x labels, y heights.
export function columns(bins, { label = 'Columns', xLabel = '', yLabel = '', width = 520, height = 200, className = 'plot-accent', format = defaultFormat } = {}) {
  const left = 48;
  const bottom = 34;
  const plotWidth = width - left - 10;
  const plotHeight = height - bottom - 10;
  const max = Math.max(0, ...bins.map((b) => b.y));
  const ys = ticks(max, 4);
  const top = ys.at(-1) || 1;
  const step = bins.length ? plotWidth / bins.length : plotWidth;
  const y = (v) => 10 + plotHeight - (v / top) * plotHeight;
  let body = '';
  for (const t of ys) body += `<line class="plot-grid" x1="${left}" x2="${left + plotWidth}" y1="${y(t)}" y2="${y(t)}"/><text class="plot-tick" x="${left - 6}" y="${y(t) + 4}" text-anchor="end">${format(t)}</text>`;
  const every = Math.ceil(bins.length / 16);
  bins.forEach((b, i) => {
    const bx = left + i * step + step * 0.12;
    const bw = step * 0.76;
    body += `<rect class="${className}" x="${bx.toFixed(2)}" y="${y(b.y).toFixed(2)}" width="${bw.toFixed(2)}" height="${(10 + plotHeight - y(b.y)).toFixed(2)}"><title>${esc(`${xLabel} ${b.x}: ${format(b.y)}`)}</title></rect>`;
    if (i % every === 0) body += `<text class="plot-tick" x="${(bx + bw / 2).toFixed(2)}" y="${10 + plotHeight + 14}" text-anchor="middle">${esc(b.x)}</text>`;
  });
  body += `<line class="plot-axis" x1="${left}" x2="${left + plotWidth}" y1="${10 + plotHeight}" y2="${10 + plotHeight}"/>`;
  if (xLabel) body += `<text class="plot-label" x="${left + plotWidth / 2}" y="${height - 4}" text-anchor="middle">${esc(xLabel)}</text>`;
  if (yLabel) body += `<text class="plot-label" transform="translate(11 ${10 + plotHeight / 2}) rotate(-90)" text-anchor="middle">${esc(yLabel)}</text>`;
  const peak = bins.reduce((a, b) => (b.y > (a?.y ?? -1) ? b : a), null);
  const description = `${label}: ${bins.length} ${xLabel || 'bins'}${peak ? `, highest at ${peak.x} (${format(peak.y)})` : ''}.`;
  return { svg: wrap(width, height, label, description, body), description };
}

function truncate(s, n) {
  const t = String(s);
  return t.length > n ? `${t.slice(0, n - 1)}…` : t;
}
