// Display geometry only. Go supplies the complete bounded population and original coordinates;
// this renderer neither computes an embedding nor infers biological groups.
const esc = (s) => String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
export function cellMap(rows, column = -1) {
  const label = (row) => column < 0 ? 'Supplied coordinates' : (row.values[column] || 'Missing annotation');
  const labels = [...new Set(rows.map(label))].sort();
  const grouped = labels.length <= 6;
  const legend = grouped ? labels.map((name, i) => ({ name, className: `plot-s${i + 1}` })) : [{ name: 'More than six labels; inspect values in the table', className: 'plot-s1' }];
  const description = 'Imported map of expression barcodes with supplied coordinates. Missing coordinates and annotation-only barcodes are excluded. Coordinates are not computed by ClonoLoom. The table provides source values.';
  let xmin = Infinity, xmax = -Infinity, ymin = Infinity, ymax = -Infinity;
  for (const row of rows) { const [x, y] = row.position; xmin = Math.min(xmin, x); xmax = Math.max(xmax, x); ymin = Math.min(ymin, y); ymax = Math.max(ymax, y); }
  const scale = Math.min(540 / (xmax - xmin || 1), 250 / (ymax - ymin || 1));
  const cx = (x) => 310 + (x - (xmin + xmax) / 2) * scale;
  const cy = (y) => 150 - (y - (ymin + ymax) / 2) * scale;
  const points = rows.map((row, i) => `<circle class="plot-s${grouped ? labels.indexOf(label(row)) + 1 : 1}" data-cell="${i}" cx="${cx(row.position[0]).toFixed(2)}" cy="${cy(row.position[1]).toFixed(2)}" r="4"><title>${esc(`${row.barcode}: ${label(row)}; ${row.position.join(', ')}`)}</title></circle>`).join('');
  const svg = `<svg class="plot" viewBox="0 0 620 310" width="620" height="310" role="img" aria-label="Imported cell map"><title>Imported cell map</title><desc>${description}</desc>${points}<text class="plot-label" x="310" y="300" text-anchor="middle">Supplied map_x / map_y · equal axis scale</text></svg>`;
  return { svg, description, legend };
}
