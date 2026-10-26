// Display only. Go owns subject denominators, frequencies and Wilson intervals.
const esc = (s) => String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
export function hlaFrequencies(row) {
  const description = `Group membership among allele carriers and non-carriers in the same ${row.n}-subject complete-typing universe for ${row.locus}. ${row.excluded} selected subjects excluded for missing typing or receptor evidence. Points are observed subject fractions; bars are marginal 95% Wilson intervals, not adjusted by BY. Fixed 0–100% scale. No causal, HLA-restriction or antigen-specificity inference.`;
  const x = (v) => 180 + v * 400;
  let body = '<path d="M180 25V180H580" class="plot-axis" fill="none"/>';
  for (let i = 0; i <= 4; i++) {
    body += `<path d="M${x(i/4)} 25V180" class="plot-gridline"/><text class="plot-tick" x="${x(i/4)}" y="202" text-anchor="middle">${25*i}%</text>`;
  }
  for (const [i, f] of [row.carrier, row.noncarrier].entries()) {
    const y = 65 + i * 75, label = i ? 'Non-carriers' : 'Carriers';
    body += `<text class="plot-label" x="165" y="${y-4}" text-anchor="end">${label}</text><text class="plot-tick" x="165" y="${y+15}" text-anchor="end">${f.matches}/${f.total} subjects</text>`;
    if (f.estimate == null) { body += `<text class="plot-label" x="200" y="${y+4}">Unavailable: no subjects</text>`; continue; }
    body += `<path data-lower="${f.lower}" data-upper="${f.upper}" d="M${x(f.lower)} ${y}H${x(f.upper)}M${x(f.lower)} ${y-6}V${y+6}M${x(f.upper)} ${y-6}V${y+6}" fill="none" stroke="currentColor" stroke-width="2"/><circle data-total="${f.total}" data-matches="${f.matches}" data-estimate="${f.estimate}" cx="${x(f.estimate)}" cy="${y}" r="5" class="plot-s${i+1}"><title>${label}: ${f.matches}/${f.total}; marginal Wilson 95% interval ${f.lower}–${f.upper}</title></circle>`;
  }
  body += '<text class="plot-label" x="380" y="235" text-anchor="middle">Subjects with the selected sequence group</text>';
  return { svg: `<svg class="plot" role="img" aria-label="Subject group frequency by HLA carriage" viewBox="0 0 640 250" width="640" height="250"><title>Subject group frequency by HLA carriage</title><desc>${esc(description)}</desc>${body}</svg>`, description };
}
export function recurrencePlot(result) {
  const description = `Observed recurrence across ${result.observedSubjects} subjects with eligible receptor evidence. Each subject is counted once across declared samples. All ${result.recurrence.length} selected signatures, in declared order. Counts are descriptive, not a test of convergent recombination or specificity.`;
  if (!result.observedSubjects) return {svg:'',description:description+' No eligible subjects; frequencies are unavailable.'};
  if (result.recurrence.length > 20) return { svg: '', description: description + ' More than 20 groups: use the complete table; no subset is drawn.' };
  const n = result.observedSubjects, height = 70 + result.recurrence.length * 38, x = (v) => 100 + v * 480 / Math.max(1, n);
  let body = `<path d="M100 22V${height-34}H580" class="plot-axis" fill="none"/>`;
  for (let i = 0; i <= 4; i++) body += `<text class="plot-tick" x="${x(n*i/4)}" y="${height-13}" text-anchor="middle">${n*i/4}</text>`;
  result.recurrence.forEach((g, i) => { const y = 42 + 38*i; body += `<text class="plot-label" x="85" y="${y+4}" text-anchor="end">Group ${i+1}</text><circle cx="${x(g.subjects.length)}" cy="${y}" r="5" class="plot-s1"><title>${esc(g.label)}: ${g.subjects.length}/${n} subjects</title></circle><text class="plot-tick" x="${x(g.subjects.length)+10}" y="${y+4}">${g.subjects.length}</text>`; });
  return { svg: `<svg class="plot" role="img" aria-label="Observed cross-subject recurrence" viewBox="0 0 640 ${height}" width="640" height="${height}"><title>Observed cross-subject recurrence</title><desc>${esc(description)}</desc>${body}</svg>`, description };
}
