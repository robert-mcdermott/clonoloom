// Plots for the views, as SVG markup: horizontal bars (stacked or not), columns, estimates with
// their intervals, lines, rarefaction curves and heatmaps; and figures, a plot standing alone with
// its title, legend and caption, for a paper. Pure: each
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

// Ticks for an axis from 0 that must hold hi: ticks(hi), with one more step when the last falls
// short of it.
export function scale(hi, n = 4) {
  const t = ticks(hi, n);
  if (t.length > 1 && t.at(-1) < hi) t.push(Number((t.at(-1) + (t[1] - t[0])).toPrecision(12)));
  return t;
}

const defaultFormat = (v) => (Math.abs(v) >= 1000 ? v.toLocaleString('en-US', { maximumFractionDigits: 0 }) : String(Number(v.toPrecision(4))));

function wrap(width, height, label, description, body) {
  return `<svg class="plot" viewBox="0 0 ${width} ${height}" width="${width}" height="${height}" role="img" aria-label="${esc(label)}"><title>${esc(label)}</title><desc>${esc(description)}</desc>${body}</svg>`;
}

// Horizontal bars, stacked by parts: rows [{ label, parts: { key: value }, title }]; keys [{ key,
// label, className }]. With share, each row's parts are shares (0–1) and the axis is 0–100%.
export function stackedBars(rows, keys, { label = 'Bars', share = false, width = 520, labelWidth = 120, barHeight = 16, gap = 6, format = defaultFormat } = {}) {
  const plotLeft = labelWidth + 8;
  // Room on the right for half the last tick's label ("100%").
  const plotWidth = width - plotLeft - 20;
  const top = 6;
  const height = top + rows.length * (barHeight + gap) + 30;
  const totals = rows.map((r) => keys.reduce((s, k) => s + (r.parts[k.key] ?? 0), 0));
  const max = share ? 1 : Math.max(0, ...totals);
  const x = (v) => (max > 0 ? (v / max) * plotWidth : 0);
  let body = '';
  rows.forEach((row, i) => {
    const y = top + i * (barHeight + gap);
    body += `<text class="plot-label" x="${labelWidth}" y="${y + barHeight / 2 + 4}" text-anchor="end">${esc(truncate(row.label, Math.max(8, Math.floor(labelWidth / 7.5))))}</text>`;
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
  const left = 60;
  const bottom = 34;
  const plotWidth = width - left - 10;
  const plotHeight = height - bottom - 10;
  const max = Math.max(0, ...bins.map((b) => b.y));
  const ys = scale(max, 4);
  const top = ys.at(-1) || 1;
  const step = bins.length ? plotWidth / bins.length : plotWidth;
  const y = (v) => 10 + plotHeight - (v / top) * plotHeight;
  let body = '';
  for (const t of ys) body += `<line class="plot-gridline" x1="${left}" x2="${left + plotWidth}" y1="${y(t)}" y2="${y(t)}"/><text class="plot-tick" x="${left - 6}" y="${y(t) + 4}" text-anchor="end">${format(t)}</text>`;
  const every = Math.ceil(bins.length / 16);
  bins.forEach((b, i) => {
    const bx = left + i * step + step * 0.12;
    const bw = step * 0.76;
    body += `<rect class="${className}" x="${bx.toFixed(2)}" y="${y(b.y).toFixed(2)}" width="${bw.toFixed(2)}" height="${(10 + plotHeight - y(b.y)).toFixed(2)}"><title>${esc(`${xLabel} ${b.x}: ${format(b.y)}`)}</title></rect>`;
    if (i % every === 0) body += `<text class="plot-tick" x="${(bx + bw / 2).toFixed(2)}" y="${10 + plotHeight + 14}" text-anchor="middle">${esc(b.x)}</text>`;
  });
  body += `<line class="plot-axis" x1="${left}" x2="${left + plotWidth}" y1="${10 + plotHeight}" y2="${10 + plotHeight}"/>`;
  if (xLabel) body += `<text class="plot-label" x="${left + plotWidth / 2}" y="${height - 4}" text-anchor="middle">${esc(xLabel)}</text>`;
  if (yLabel) body += `<text class="plot-label" transform="translate(13 ${10 + plotHeight / 2}) rotate(-90)" text-anchor="middle">${esc(yLabel)}</text>`;
  const peak = bins.reduce((a, b) => (b.y > (a?.y ?? -1) ? b : a), null);
  const description = `${label}: ${bins.length} ${xLabel || 'bins'}${peak ? `, highest at ${peak.x} (${format(peak.y)})` : ''}.`;
  return { svg: wrap(width, height, label, description, body), description };
}

// Estimates with their intervals, one row each: rows [{ label, value, lower, upper, className }]
// (lower and upper may be missing). The axis starts at 0, or at from.
export function intervals(rows, { label = 'Estimates', width = 520, labelWidth = 120, rowHeight = 22, from = 0, format = defaultFormat, unit = '' } = {}) {
  const plotLeft = labelWidth + 8;
  const plotWidth = width - plotLeft - 20;
  const top = 6;
  const height = top + rows.length * rowHeight + 30;
  const values = rows.flatMap((r) => [r.value, r.upper]).filter(Number.isFinite);
  const hi = Math.max(from, ...values);
  const t = scale(hi - from, 4).map((v) => v + from);
  const span = (t.at(-1) - from) || 1;
  const x = (v) => plotLeft + ((Math.min(Math.max(v, from), t.at(-1)) - from) / span) * plotWidth;
  let body = '';
  rows.forEach((row, i) => {
    const y = top + i * rowHeight + rowHeight / 2;
    body += `<text class="plot-label" x="${labelWidth}" y="${y + 4}" text-anchor="end">${esc(truncate(row.label, Math.max(8, Math.floor(labelWidth / 7.5))))}</text>`;
    body += `<line class="plot-gridline" x1="${plotLeft}" x2="${plotLeft + plotWidth}" y1="${y}" y2="${y}"/>`;
    if (Number.isFinite(row.lower) && Number.isFinite(row.upper)) {
      body += `<line class="plot-whisker" x1="${x(row.lower).toFixed(2)}" x2="${x(row.upper).toFixed(2)}" y1="${y}" y2="${y}"/>`;
      for (const v of [row.lower, row.upper]) body += `<line class="plot-whisker" x1="${x(v).toFixed(2)}" x2="${x(v).toFixed(2)}" y1="${y - 4}" y2="${y + 4}"/>`;
    }
    if (Number.isFinite(row.value)) {
      const range = Number.isFinite(row.lower) ? ` (${format(row.lower)}–${format(row.upper)})` : '';
      body += `<circle class="${row.className ?? 'plot-accent'}" cx="${x(row.value).toFixed(2)}" cy="${y}" r="4"><title>${esc(`${row.label}: ${format(row.value)}${range}${unit}`)}</title></circle>`;
    }
  });
  const axisY = top + rows.length * rowHeight + 4;
  body += `<line class="plot-axis" x1="${plotLeft}" x2="${plotLeft + plotWidth}" y1="${axisY}" y2="${axisY}"/>`;
  for (const v of t) body += `<line class="plot-axis" x1="${x(v)}" x2="${x(v)}" y1="${axisY}" y2="${axisY + 4}"/><text class="plot-tick" x="${x(v)}" y="${axisY + 16}" text-anchor="middle">${format(v)}</text>`;
  const description = `${label}: ${rows.map((r) => `${r.label} ${Number.isFinite(r.value) ? format(r.value) : 'none'}${Number.isFinite(r.lower) ? ` (${format(r.lower)} to ${format(r.upper)})` : ''}`).join('; ')}.`;
  return { svg: wrap(width, height, label, description, body), description };
}

// Lines through points: series [{ label, points: [{ x, y }], className }], on a logarithmic y axis
// when log; format writes the y axis's values, xFormat the x axis's.
export function lines(series, { label = 'Lines', width = 520, height = 220, xLabel = '', yLabel = '', log = false, format = defaultFormat, xFormat = format } = {}) {
  const left = 60;
  const bottom = 34;
  const plotWidth = width - left - 26;
  const plotHeight = height - bottom - 10;
  const pts = series.flatMap((s) => s.points).filter((p) => Number.isFinite(p.y) && (!log || p.y > 0));
  const xs = pts.map((p) => p.x);
  const x0 = Math.min(...xs, 0);
  const x1 = Math.max(...xs, 1);
  const ty = (v) => (log ? Math.log10(v) : v);
  const ys = pts.map((p) => ty(p.y));
  let y0 = log ? Math.floor(Math.min(...ys, 0)) : 0;
  let y1 = log ? Math.ceil(Math.max(...ys, 1)) : Math.max(...ys, 0);
  const yt = log ? Array.from({ length: y1 - y0 + 1 }, (_, k) => y0 + k) : scale(y1, 4);
  if (!log) y1 = yt.at(-1) || 1;
  if (y1 === y0) y1 = y0 + 1;
  const px = (v) => left + ((v - x0) / (x1 - x0 || 1)) * plotWidth;
  const py = (v) => 10 + plotHeight - ((ty(v) - y0) / (y1 - y0)) * plotHeight;
  let body = '';
  for (const t of yt) {
    const y = 10 + plotHeight - ((t - y0) / (y1 - y0)) * plotHeight;
    body += `<line class="plot-gridline" x1="${left}" x2="${left + plotWidth}" y1="${y}" y2="${y}"/><text class="plot-tick" x="${left - 6}" y="${y + 4}" text-anchor="end">${format(log ? 10 ** t : t)}</text>`;
  }
  for (const v of ticks(x1 - x0, 4).map((v) => v + x0)) body += `<text class="plot-tick" x="${px(v)}" y="${10 + plotHeight + 14}" text-anchor="middle">${xFormat(v)}</text>`;
  body += `<line class="plot-axis" x1="${left}" x2="${left + plotWidth}" y1="${10 + plotHeight}" y2="${10 + plotHeight}"/>`;
  for (const s of series) {
    const ok = s.points.filter((p) => Number.isFinite(p.y) && (!log || p.y > 0));
    if (!ok.length) continue;
    body += `<polyline class="plot-line ${s.className ?? 'plot-line-accent'}" points="${ok.map((p) => `${px(p.x).toFixed(1)},${py(p.y).toFixed(1)}`).join(' ')}"><title>${esc(s.label)}</title></polyline>`;
  }
  if (xLabel) body += `<text class="plot-label" x="${left + plotWidth / 2}" y="${height - 4}" text-anchor="middle">${esc(xLabel)}</text>`;
  if (yLabel) body += `<text class="plot-label" transform="translate(13 ${10 + plotHeight / 2}) rotate(-90)" text-anchor="middle">${esc(yLabel)}</text>`;
  const description = `${label}: ${series.length} line${series.length === 1 ? '' : 's'} (${series.map((s) => s.label).join(', ')}).`;
  return { svg: wrap(width, height, label, description, body), description };
}

// Rarefaction and extrapolation curves: series [{ label, className, points: [{ x, y, lower,
// upper, extrapolated }], observed: { x, y } }], each drawn solid where interpolated and dashed
// where extrapolated, its interval as a band (with bands), a dot where the sample itself is.
// guides [{ x, label }] are vertical reference lines. x is linear from 0 (to xMax when given), or
// logarithmic with logX (x > 0); y is linear from 0.
export function curves(series, { label = 'Curves', width = 560, height = 260, xLabel = '', yLabel = '', logX = false, bands = true, guides = [], xMax = null, xFormat = defaultFormat, yFormat = defaultFormat } = {}) {
  const left = 62;
  const top = 12;
  const bottom = 36;
  // Room on the right for half the last tick's label (80,000), which is centered on the axis' end.
  const plotWidth = width - left - 26;
  const plotHeight = height - top - bottom;
  const usable = (p) => Number.isFinite(p.x) && Number.isFinite(p.y) && (!logX || p.x > 0);
  const pts = series.flatMap((s) => s.points.filter(usable));
  const xs = pts.map((p) => p.x);
  let x0 = 0;
  let x1;
  let xt;
  if (logX) {
    x0 = Math.floor(Math.log10(Math.min(...xs, 10)));
    x1 = Math.max(Math.ceil(Math.log10(Math.max(...xs, 10))), x0 + 1);
    xt = Array.from({ length: x1 - x0 + 1 }, (_, k) => x0 + k);
  } else {
    xt = xMax ? ticks(xMax, 4) : scale(Math.max(...xs, 1), 4);
    x1 = xMax ?? (xt.at(-1) || 1);
  }
  const yHi = Math.max(1, ...pts.map((p) => (bands && Number.isFinite(p.upper) ? Math.max(p.y, p.upper) : p.y)));
  const yt = scale(yHi, 4);
  const y1 = yt.at(-1) || 1;
  const px = (v) => left + (((logX ? Math.log10(v) : v) - x0) / (x1 - x0 || 1)) * plotWidth;
  const py = (v) => top + plotHeight - (Math.min(Math.max(v, 0), y1) / y1) * plotHeight;
  const path = (list) => list.map((p) => `${px(p.x).toFixed(1)},${py(p.y).toFixed(1)}`).join(' ');
  let body = '';
  for (const t of yt) body += `<line class="plot-gridline" x1="${left}" x2="${left + plotWidth}" y1="${py(t)}" y2="${py(t)}"/><text class="plot-tick" x="${left - 6}" y="${py(t) + 4}" text-anchor="end">${yFormat(t)}</text>`;
  for (const t of xt) {
    const v = logX ? 10 ** t : t;
    body += `<text class="plot-tick" x="${px(v)}" y="${top + plotHeight + 15}" text-anchor="middle">${xFormat(v)}</text>`;
  }
  body += `<line class="plot-axis" x1="${left}" x2="${left + plotWidth}" y1="${top + plotHeight}" y2="${top + plotHeight}"/>`;
  for (const s of series) {
    const ok = s.points.filter(usable);
    if (!ok.length) continue;
    const cls = s.className ?? 'plot-s1';
    const banded = ok.filter((p) => Number.isFinite(p.lower) && Number.isFinite(p.upper));
    if (bands && banded.length > 1) {
      const upper = banded.map((p) => ({ x: p.x, y: p.upper }));
      const lower = banded.map((p) => ({ x: p.x, y: p.lower })).reverse();
      body += `<polygon class="plot-band ${cls}" points="${path(upper)} ${path(lower)}"/>`;
    }
    const within = ok.filter((p) => !p.extrapolated);
    const beyond = ok.filter((p) => p.extrapolated);
    if (within.length > 1) body += `<polyline class="plot-line ${cls}" points="${path(within)}"><title>${esc(s.label)}</title></polyline>`;
    if (beyond.length) body += `<polyline class="plot-line plot-dashed ${cls}" points="${path([...within.slice(-1), ...beyond])}"><title>${esc(`${s.label}, extrapolated`)}</title></polyline>`;
    if (s.observed && usable(s.observed)) body += `<circle class="${cls}" cx="${px(s.observed.x).toFixed(1)}" cy="${py(s.observed.y).toFixed(1)}" r="3.5"><title>${esc(`${s.label}: ${yFormat(s.observed.y)} at ${xFormat(s.observed.x)}`)}</title></circle>`;
  }
  for (const g of guides) {
    if (!Number.isFinite(g.x) || (logX && g.x <= 0)) continue;
    const x = px(g.x).toFixed(1);
    body += `<line class="plot-guide" x1="${x}" x2="${x}" y1="${top}" y2="${top + plotHeight}"/>`;
    if (g.label) body += `<text class="plot-tick" x="${x}" y="${top + 9}" dx="4">${esc(g.label)}</text>`;
  }
  if (xLabel) body += `<text class="plot-label" x="${left + plotWidth / 2}" y="${height - 4}" text-anchor="middle">${esc(xLabel)}</text>`;
  if (yLabel) body += `<text class="plot-label" transform="translate(13 ${top + plotHeight / 2}) rotate(-90)" text-anchor="middle">${esc(yLabel)}</text>`;
  const ends = series.map((s) => {
    const last = s.points.filter(usable).at(-1);
    return last ? `${s.label} ${yFormat(last.y)} at ${xFormat(last.x)}` : `${s.label} none`;
  });
  const description = `${label}: ${series.length} curve${series.length === 1 ? '' : 's'}, solid where interpolated and dashed where extrapolated, ending at ${ends.join('; ')}.`;
  return { svg: wrap(width, height, label, description, body), description };
}

// A timeline of subjects' samples (a swimmer plot): rows [{ label, className, marks: [{ x, column,
// label, reference }] }], each mark a sample at x (days, or visits) on the axis, or in a column of
// its own where it has no day: 'reference' (a reference sample without a time point) and
// 'baseline' left of the axis, 'after' right of it. Reference samples are diamonds, others dots;
// marks at one place in a row are set one above the other.
export function timeline(rows, { label = 'Timeline', width = 640, labelWidth = 90, rowHeight = 26, xLabel = 'Days', format = defaultFormat } = {}) {
  const marks = rows.flatMap((r) => r.marks);
  const xs = marks.filter((m) => !m.column && Number.isFinite(m.x)).map((m) => m.x);
  const columns = ['reference', 'baseline', 'after'].filter((c) => marks.some((m) => m.column === c));
  const colWidth = 58;
  const left = columns.filter((c) => c !== 'after');
  const plotLeft = labelWidth + 12 + left.length * colWidth;
  const plotRight = width - 24 - (columns.includes('after') ? colWidth : 0);
  const top = 22;
  const height = top + rows.length * rowHeight + 40;
  // The axis: nice ticks over the days, holding day 0.
  let lo = Math.min(0, ...xs);
  let hi = Math.max(lo + 1, ...xs);
  const step = ticks(hi - lo, 5)[1] ?? 1;
  lo = Math.floor(lo / step) * step;
  hi = Math.ceil(hi / step) * step;
  const t = [];
  for (let v = lo; v <= hi + step * 1e-9; v += step) t.push(Number(v.toPrecision(12)));
  const x = (v) => plotLeft + ((v - lo) / (hi - lo)) * (plotRight - plotLeft);
  const colX = (c) => (c === 'after' ? plotRight + colWidth / 2 + 6 : labelWidth + 12 + left.indexOf(c) * colWidth + colWidth / 2);
  let body = '';
  for (const c of columns) body += `<text class="plot-tick" x="${colX(c)}" y="${top - 8}" text-anchor="middle">${c === 'reference' ? 'reference' : c}</text>`;
  if (lo < 0) body += `<line class="plot-guide" x1="${x(0)}" x2="${x(0)}" y1="${top - 4}" y2="${top + rows.length * rowHeight}"/>`;
  rows.forEach((row, i) => {
    const y = top + i * rowHeight + rowHeight / 2;
    body += `<text class="plot-label" x="${labelWidth}" y="${y + 4}" text-anchor="end">${esc(truncate(row.label, Math.max(8, Math.floor(labelWidth / 7.5))))}</text>`;
    body += `<line class="plot-gridline" x1="${plotLeft}" x2="${plotRight}" y1="${y}" y2="${y}"/>`;
    const at = new Map();
    for (const m of row.marks) {
      const key = m.column ?? String(m.x);
      at.set(key, [...(at.get(key) ?? []), m]);
    }
    for (const group of at.values()) {
      group.forEach((m, k) => {
        const cx = m.column ? colX(m.column) : x(m.x);
        const cy = y + (k - (group.length - 1) / 2) * 7;
        const title = `<title>${esc(`${row.label}, ${m.label}`)}</title>`;
        if (m.reference) body += `<path class="plot-reference" d="M${cx.toFixed(2)} ${(cy - 5).toFixed(2)}l5 5-5 5-5-5z">${title}</path>`;
        else body += `<circle class="${row.className ?? 'plot-accent'}" cx="${cx.toFixed(2)}" cy="${cy.toFixed(2)}" r="4">${title}</circle>`;
      });
    }
  });
  const axisY = top + rows.length * rowHeight + 4;
  body += `<line class="plot-axis" x1="${plotLeft}" x2="${plotRight}" y1="${axisY}" y2="${axisY}"/>`;
  for (const v of t) body += `<line class="plot-axis" x1="${x(v)}" x2="${x(v)}" y1="${axisY}" y2="${axisY + 4}"/><text class="plot-tick" x="${x(v)}" y="${axisY + 16}" text-anchor="middle">${format(v)}</text>`;
  body += `<text class="plot-label" x="${(plotLeft + plotRight) / 2}" y="${height - 4}" text-anchor="middle">${esc(xLabel)}</text>`;
  const description = `${label}: ${rows.length} subject${rows.length === 1 ? '' : 's'}; ${rows.map((r) => `${r.label}: ${r.marks.map((m) => m.label).join(', ')}`).join('; ')}.`;
  return { svg: wrap(width, height, label, description, body), description };
}

// A frequency as a percentage, to three significant digits: 0.1234 is 12.3%, 1e-5 is 0.001%.
export const percent = (v) => (Number.isFinite(v) ? `${Number((100 * v).toPrecision(3))}%` : '—');

// The x axis of a subject's time points, equally spaced in time order: their labels below, each
// truncated to its place.
function pointAxis(points, left, plotWidth, y) {
  const step = plotWidth / Math.max(points.length, 1);
  const px = (i) => left + (i + 0.5) * step;
  let body = '';
  points.forEach((p, i) => {
    body += `<text class="plot-tick" x="${px(i).toFixed(1)}" y="${y}" text-anchor="middle">${esc(truncate(p.label, Math.max(6, Math.floor(step / 6.4))))}<title>${esc(p.title ?? p.label)}</title></text>`;
  });
  return { px, step, body };
}

// Threads: clonotypes' frequencies through a subject's time points (points [{ label, title }],
// equally spaced in time order), on a logarithmic axis: series [{ key, label, className, values:
// [{ y, lower, upper, absent }] }], one value a point (y a frequency; not finite where the point
// has no abundance). A clonotype not found at a point is drawn on the lane below the axis as an
// open circle, never as zero, the lines to it dashed. With selected (a series' key), that series is
// drawn over the others with its intervals (at a point it was not found, up to what the point
// could have missed), and the others faded. Every series' marks carry data-key for the page.
export function threads(points, series, { label = 'Threads', width = 680, height = 300, selected = null, format = percent } = {}) {
  const left = 64;
  const top = 10;
  const lane = 30;
  const bottom = 30;
  const plotWidth = width - left - 16;
  const plotHeight = height - top - lane - bottom;
  const finite = (v) => Number.isFinite(v) && v > 0;
  const seen = series.flatMap((s) => s.values.filter((v) => !v.absent && finite(v.y)).flatMap((v) => [v.y, ...(finite(v.lower) ? [v.lower] : []), ...(finite(v.upper) ? [v.upper] : [])]));
  const limits = series.flatMap((s) => s.values.filter((v) => v.absent && finite(v.upper)).map((v) => v.upper));
  let y0 = Math.floor(Math.log10(seen.length || limits.length ? Math.min(...seen, ...limits) : 0.001));
  const y1 = Math.max(Math.ceil(Math.log10(seen.length ? Math.max(...seen) : 0.01)), y0 + 1);
  y0 = Math.max(y0, y1 - 7);
  const py = (v) => top + plotHeight - ((Math.min(Math.max(Math.log10(v), y0), y1) - y0) / (y1 - y0)) * plotHeight;
  const laneY = top + plotHeight + lane / 2 + 4;
  const axis = pointAxis(points, left, plotWidth, top + plotHeight + lane + 16);
  const at = (v) => (v.absent ? laneY : finite(v.y) ? py(v.y) : null);
  let body = '';
  for (let t = y0; t <= y1; t += 1) body += `<line class="plot-gridline" x1="${left}" x2="${left + plotWidth}" y1="${py(10 ** t).toFixed(1)}" y2="${py(10 ** t).toFixed(1)}"/><text class="plot-tick" x="${left - 6}" y="${(py(10 ** t) + 4).toFixed(1)}" text-anchor="end">${format(10 ** t)}</text>`;
  body += `<line class="plot-axis" x1="${left}" x2="${left + plotWidth}" y1="${top + plotHeight}" y2="${top + plotHeight}"/>`;
  body += `<text class="plot-tick" x="${left - 6}" y="${laneY + 4}" text-anchor="end">not found</text><line class="plot-gridline plot-dashed" x1="${left}" x2="${left + plotWidth}" y1="${laneY}" y2="${laneY}"/>`;
  body += axis.body;
  const draw = (s, chosen) => {
    const cls = `${s.className ?? 'plot-s1'}${selected !== null && !chosen ? ' plot-faded' : ''}${chosen ? ' plot-chosen' : ''}`;
    const key = esc(s.key ?? s.label);
    let out = `<g data-key="${key}"><title>${esc(s.label)}</title>`;
    for (let i = 0; i + 1 < s.values.length; i += 1) {
      const a = at(s.values[i]);
      const b = at(s.values[i + 1]);
      if (a === null || b === null || (s.values[i].absent && s.values[i + 1].absent)) continue;
      const dashed = s.values[i].absent || s.values[i + 1].absent ? ' plot-dashed' : '';
      out += `<path class="plot-line plot-thread ${cls}${dashed}" d="M${axis.px(i).toFixed(1)} ${a.toFixed(1)}L${axis.px(i + 1).toFixed(1)} ${b.toFixed(1)}"/>`;
    }
    s.values.forEach((v, i) => {
      const y = at(v);
      if (y === null) return;
      const x = axis.px(i).toFixed(1);
      if (chosen && finite(v.upper)) {
        const lo = v.absent ? laneY : py(finite(v.lower) ? v.lower : 10 ** y0);
        out += `<line class="plot-whisker${v.absent ? ' plot-dashed' : ''}" x1="${x}" x2="${x}" y1="${lo.toFixed(1)}" y2="${py(v.upper).toFixed(1)}"/>`;
      }
      const words = `${s.label} at ${points[i]?.title ?? points[i]?.label}: ${v.absent ? `not found${finite(v.upper) ? ` (up to ${format(v.upper)})` : ''}` : `${format(v.y)}${finite(v.lower) ? ` (${format(v.lower)}–${format(v.upper)})` : ''}`}`;
      out += v.absent
        ? `<circle class="plot-absent ${cls}" cx="${x}" cy="${y.toFixed(1)}" r="${chosen ? 4 : 3}"><title>${esc(words)}</title></circle>`
        : `<circle class="${cls}" cx="${x}" cy="${y.toFixed(1)}" r="${chosen ? 4 : 2.6}"><title>${esc(words)}</title></circle>`;
    });
    return `${out}</g>`;
  };
  const picked = series.find((s) => selected !== null && (s.key ?? s.label) === selected);
  for (const s of series) if (s !== picked) body += draw(s, false);
  if (picked) body += draw(picked, true);
  body += `<text class="plot-label" transform="translate(13 ${top + plotHeight / 2}) rotate(-90)" text-anchor="middle">Frequency (logarithmic)</text>`;
  const peaks = series.slice(0, 3).map((s) => {
    let best = -1;
    s.values.forEach((v, i) => { if (!v.absent && finite(v.y) && (best < 0 || v.y > s.values[best].y)) best = i; });
    return best < 0 ? `${s.label} not found` : `${s.label} highest at ${points[best].label} (${format(s.values[best].y)})`;
  });
  const description = `${label}: ${series.length} clonotype${series.length === 1 ? '' : 's'} through ${points.length} time points (${points.map((p) => p.label).join(', ')})${peaks.length ? `; ${peaks.join('; ')}` : ''}.`;
  return { svg: wrap(width, height, label, description, body), description };
}

// An alluvial plot: at each of a subject's time points (points [{ label, title }], equally spaced
// in time order), the clonotypes' frequencies stacked (series [{ key, label, className, values:
// [frequency] }], the first at the bottom, in one order at every point), each joined to the next
// point by a ribbon; the axis is the share of the sample, from 0 to the largest stack. With
// selected, the others are faded. Every series' marks carry data-key for the page.
export function alluvial(points, series, { label = 'Alluvial plot', width = 680, height = 300, selected = null, format = percent } = {}) {
  const left = 64;
  const top = 10;
  const bottom = 30;
  const plotWidth = width - left - 16;
  const plotHeight = height - top - bottom;
  const value = (v) => (Number.isFinite(v) && v > 0 ? v : 0);
  const totals = points.map((_, i) => series.reduce((sum, s) => sum + value(s.values[i]), 0));
  const yt = scale(Math.max(...totals, 0), 4);
  const hi = yt.at(-1) || 1;
  const py = (v) => top + plotHeight - (v / hi) * plotHeight;
  const axis = pointAxis(points, left, plotWidth, top + plotHeight + 16);
  const barW = Math.max(6, Math.min(26, axis.step * 0.28));
  // Each series' band at each point: [bottom, top] in shares, stacked in the series' order.
  const bands = series.map(() => []);
  points.forEach((_, i) => {
    let at = 0;
    series.forEach((s, k) => {
      const v = value(s.values[i]);
      bands[k][i] = [at, at + v];
      at += v;
    });
  });
  let body = '';
  for (const t of yt) body += `<line class="plot-gridline" x1="${left}" x2="${left + plotWidth}" y1="${py(t).toFixed(1)}" y2="${py(t).toFixed(1)}"/><text class="plot-tick" x="${left - 6}" y="${(py(t) + 4).toFixed(1)}" text-anchor="end">${format(t)}</text>`;
  body += `<line class="plot-axis" x1="${left}" x2="${left + plotWidth}" y1="${top + plotHeight}" y2="${top + plotHeight}"/>`;
  body += axis.body;
  series.forEach((s, k) => {
    const chosen = selected !== null && (s.key ?? s.label) === selected;
    const cls = `${s.className ?? 'plot-s1'}${selected !== null && !chosen ? ' plot-faded' : ''}`;
    let out = `<g data-key="${esc(s.key ?? s.label)}"><title>${esc(s.label)}</title>`;
    for (let i = 0; i + 1 < points.length; i += 1) {
      const [b1, t1] = bands[k][i];
      const [b2, t2] = bands[k][i + 1];
      if (t1 - b1 <= 0 && t2 - b2 <= 0) continue;
      const x1 = axis.px(i) + barW / 2;
      const x2 = axis.px(i + 1) - barW / 2;
      const mx = (x1 + x2) / 2;
      const f = (v) => v.toFixed(1);
      out += `<path class="plot-ribbon ${cls}" d="M${f(x1)} ${f(py(t1))}C${f(mx)} ${f(py(t1))} ${f(mx)} ${f(py(t2))} ${f(x2)} ${f(py(t2))}L${f(x2)} ${f(py(b2))}C${f(mx)} ${f(py(b2))} ${f(mx)} ${f(py(b1))} ${f(x1)} ${f(py(b1))}Z"/>`;
    }
    points.forEach((p, i) => {
      const [b, t] = bands[k][i];
      if (t - b <= 0) return;
      out += `<rect class="${cls}" x="${(axis.px(i) - barW / 2).toFixed(1)}" y="${py(t).toFixed(1)}" width="${barW.toFixed(1)}" height="${Math.max(0.6, py(b) - py(t)).toFixed(1)}"><title>${esc(`${s.label} at ${p.title ?? p.label}: ${format(t - b)}`)}</title></rect>`;
    });
    body += `${out}</g>`;
  });
  body += `<text class="plot-label" transform="translate(13 ${top + plotHeight / 2}) rotate(-90)" text-anchor="middle">Share of the sample</text>`;
  const description = `${label}: ${series.length} clonotype${series.length === 1 ? '' : 's'} stacked at ${points.length} time points, holding ${points.map((p, i) => `${format(totals[i])} at ${p.label}`).join(', ')}.`;
  return { svg: wrap(width, height, label, description, body), description };
}

// A heatmap: values[r][c] (not finite: left blank) for rowLabels × colLabels, each cell shaded from
// none to max (the largest value when not given), titled by title(r, c, v), with a scale. Column
// labels are turned to read upward.
export function heatmap(values, { label = 'Heatmap', rowLabels = [], colLabels = [], width = 640, labelWidth = 120, cellHeight = 16, colLabelHeight = 70, max = null, format = defaultFormat, title = null } = {}) {
  const rows = values.length;
  const cols = colLabels.length;
  const top = colLabelHeight;
  const cellWidth = Math.max(4, Math.min(28, (width - labelWidth - 10) / Math.max(cols, 1)));
  const plotWidth = cellWidth * cols;
  const height = top + rows * cellHeight + 34;
  const hi = max ?? Math.max(0, ...values.flat().filter(Number.isFinite));
  let body = '';
  colLabels.forEach((c, j) => {
    const x = labelWidth + (j + 0.5) * cellWidth;
    body += `<text class="plot-tick" transform="translate(${x.toFixed(1)} ${top - 4}) rotate(-60)" text-anchor="start">${esc(truncate(c, 14))}</text>`;
  });
  values.forEach((row, i) => {
    const y = top + i * cellHeight;
    body += `<text class="plot-label" x="${labelWidth - 6}" y="${y + cellHeight / 2 + 4}" text-anchor="end">${esc(truncate(rowLabels[i] ?? '', Math.max(8, Math.floor(labelWidth / 7))))}</text>`;
    row.forEach((v, j) => {
      if (!Number.isFinite(v)) return;
      const shade = hi > 0 ? Math.min(1, v / hi) : 0;
      const t = title ? title(i, j, v) : `${rowLabels[i]}, ${colLabels[j]}: ${format(v)}`;
      body += `<rect class="plot-heat" x="${(labelWidth + j * cellWidth).toFixed(1)}" y="${y}" width="${(cellWidth - 1).toFixed(1)}" height="${cellHeight - 1}" fill-opacity="${(0.06 + 0.94 * shade).toFixed(3)}"><title>${esc(t)}</title></rect>`;
    });
  });
  // The scale: from none to the largest.
  const sy = top + rows * cellHeight + 12;
  for (let k = 0; k < 10; k += 1) body += `<rect class="plot-heat" x="${labelWidth + k * 12}" y="${sy}" width="12" height="8" fill-opacity="${(0.06 + 0.94 * (k / 9)).toFixed(3)}"/>`;
  body += `<text class="plot-tick" x="${labelWidth - 4}" y="${sy + 8}" text-anchor="end">0</text><text class="plot-tick" x="${labelWidth + 124}" y="${sy + 8}">${format(hi)}</text>`;
  let best = { v: -Infinity };
  values.forEach((row, i) => row.forEach((v, j) => { if (Number.isFinite(v) && v > best.v) best = { v, i, j }; }));
  const description = `${label}: ${rows} row${rows === 1 ? '' : 's'} by ${cols} column${cols === 1 ? '' : 's'}${Number.isFinite(best.v) ? `; the largest ${format(best.v)}, ${rowLabels[best.i]} and ${colLabels[best.j]}` : ''}.`;
  return { svg: wrap(Math.max(width, labelWidth + plotWidth + 10), height, label, description, body), description };
}

function truncate(s, n) {
  const t = String(s);
  return t.length > n ? `${t.slice(0, n - 1)}…` : t;
}

// --- Figures --------------------------------------------------------------------------------------

// The colors a figure is printed in: the light theme's (web/styles.css), with colorVision the
// color-vision-friendly status colors. A figure stands alone, so its colors are written into it.
const PRINT = {
  text: '#171b26', text2: '#4b5468', text3: '#636c7e', line: '#e3e7ef', lineStrong: '#d0d6e2',
  accent: '#5b4ce6', accent2: '#0fa88f', brandA: '#c8406f', brandB: '#e9a23b', ok: '#138a52', warn: '#c27a00', danger: '#d63b4a',
};
const PRINT_CVD = { ok: '#0068a8', warn: '#9a6200', danger: '#b0306f' };

// The plot classes' colors for a figure (the rules of web/styles.css's plots, their tokens
// resolved).
export function figureStyle({ colorVision = false, font = 'Helvetica, Arial, sans-serif' } = {}) {
  const c = { ...PRINT, ...(colorVision ? PRINT_CVD : {}) };
  const series = [c.accent, c.brandB, c.accent2, c.brandA, c.ok, c.text3];
  let css = `text{font-family:${font}}.plot-label{font-size:12px;fill:${c.text2}}.plot-tick{font-size:11px;fill:${c.text3}}`
    + `.plot-axis{stroke:${c.lineStrong};stroke-width:1}.plot-gridline{stroke:${c.line};stroke-width:1}`
    + `.plot-ok{fill:${c.ok}}.plot-warn{fill:${c.warn}}.plot-danger{fill:${c.danger}}.plot-muted{fill:${c.text3}}.plot-accent{fill:${c.accent}}.plot-c4{fill:${c.accent2}}.plot-c5{fill:${c.brandA}}.plot-reference{fill:${c.brandA}}`
    + `.plot-whisker{stroke:${c.text2};stroke-width:1.4}.plot-line{fill:none;stroke-width:1.7;stroke-linejoin:round;stroke-linecap:round}.plot-line-accent{stroke:${c.accent}}`
    + `.plot-dashed{stroke-dasharray:5 4}.plot-heat{fill:${c.accent};stroke:none}.plot-guide{stroke:${c.text3};stroke-width:1;stroke-dasharray:3 3}.plot-band{stroke:none;fill-opacity:0.13}`
    + `.figure-title{font-size:14px;font-weight:bold;fill:${c.text}}.figure-caption{font-size:10.5px;fill:${c.text2}}.figure-legend{font-size:11px;fill:${c.text2}}`
    + `circle.plot-absent{fill:#ffffff;stroke-width:1.4}path.plot-ribbon{fill-opacity:0.3;stroke:none}.plot-thread{stroke-width:1.3}.plot-thread.plot-chosen{stroke-width:2.6}.plot-faded{opacity:0.16}`;
  series.forEach((color, i) => {
    css += `.plot-band.plot-s${i + 1}{fill:${color}}.plot-line.plot-s${i + 1}{stroke:${color}}circle.plot-s${i + 1},rect.plot-s${i + 1},path.plot-ribbon.plot-s${i + 1}{fill:${color}}circle.plot-absent.plot-s${i + 1}{fill:#ffffff;stroke:${color}}`;
  });
  return css;
}

// Words broken into lines of at most `width` characters.
function wrapWords(text, width) {
  const lines = [];
  let line = '';
  for (const word of String(text).split(/\s+/).filter(Boolean)) {
    if (line && line.length + 1 + word.length > width) {
      lines.push(line);
      line = word;
    } else line = line ? `${line} ${word}` : word;
  }
  if (line) lines.push(line);
  return lines;
}

// A figure: a plot's SVG standing alone, on white, with its title above it, its legend
// ([{ label, className }], a swatch of each series) and its caption (lines: what was measured, the
// clonotype definition, the abundance unit, how many samples, by what) below it, and its colors
// written in (figureStyle). Answers the SVG and its size in pixels.
export function figure(plotSvg, { title = '', legend = [], caption = [], colorVision = false } = {}) {
  const box = /viewBox="0 0 ([\d.]+) ([\d.]+)"/.exec(plotSvg);
  const pw = box ? Number(box[1]) : 520;
  const ph = box ? Number(box[2]) : 220;
  const inner = plotSvg.replace(/^<svg[^>]*>/, '').replace(/<\/svg>\s*$/, '');
  const pad = 16;
  const width = Math.max(pw + 2 * pad, 360);
  const chars = Math.floor((width - 2 * pad) / 5.6);
  let y = pad;
  let body = '';
  if (title) {
    for (const line of wrapWords(title, Math.floor((width - 2 * pad) / 7.6))) {
      y += 16;
      body += `<text class="figure-title" x="${pad}" y="${y}">${esc(line)}</text>`;
    }
    y += 8;
  }
  body += `<g transform="translate(${pad} ${y})">${inner}</g>`;
  y += ph + 6;
  // The legend: swatches and labels, as many to a row as fit.
  let x = pad;
  if (legend.length) {
    y += 12;
    for (const item of legend) {
      const w = 16 + Math.ceil(String(item.label).length * 6.2) + 14;
      if (x > pad && x + w > width - pad) {
        x = pad;
        y += 16;
      }
      body += `<rect class="${item.className ?? 'plot-s1'}" x="${x}" y="${y - 9}" width="10" height="10" rx="2"/><text class="figure-legend" x="${x + 15}" y="${y}">${esc(item.label)}</text>`;
      x += w;
    }
    y += 8;
  }
  for (const text of caption) {
    for (const line of wrapWords(text, chars)) {
      y += 14;
      body += `<text class="figure-caption" x="${pad}" y="${y}">${esc(line)}</text>`;
    }
  }
  const height = Math.ceil(y + pad);
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${height}" viewBox="0 0 ${width} ${height}"><style>${figureStyle({ colorVision })}</style><rect width="${width}" height="${height}" fill="#ffffff"/>${body}</svg>`;
  return { svg, width, height };
}
