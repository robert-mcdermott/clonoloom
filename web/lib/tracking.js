// Which clonotypes a dataset's Tracking view follows (clonoloom-spec/requirements.md, T1), and the
// two time points its tests compare (T2), as the project keeps them with the dataset: a reference
// sample's largest, each sample's largest, a list of clonotypes (by ID or junction) the user made,
// which the Clonotypes view adds to, or those the comparison's test called. The program follows
// and tests them (internal/track, internal/expansion); every edit returns a new project.

import { touch } from './project.js';

// The tests, the alternatives and the corrections for testing many clonotypes, as the view offers
// them and the methods paragraph says them.
export const TESTS = {
  auto: { label: 'Beta-binomial where replicates allow', name: 'the beta-binomial test (the binomial test where its variance could not be fitted)', words: 'the beta-binomial test of its count at the first time point given its total, its variance fitted on samples that should not differ, or, without them, the binomial test' },
  betabinomial: { label: 'Beta-binomial test', name: 'the beta-binomial test', words: 'the beta-binomial test of its count at the first time point given its total, its variance fitted on samples that should not differ' },
  binomial: { label: 'Binomial test', name: 'the binomial test', words: 'the binomial test of its count at the first time point given its total, at the share of the depth' },
  fisher: { label: 'Fisher\'s exact test', name: 'Fisher\'s exact test', words: 'Fisher\'s exact test of its count against the others\' at the two time points' },
};
export const ALTERNATIVES = {
  'two-sided': { label: 'Either way', words: 'two-sided' },
  increase: { label: 'Increased', words: 'one-sided, for an increase' },
  decrease: { label: 'Decreased', words: 'one-sided, for a decrease' },
};
export const CORRECTIONS = {
  bh: { label: 'Benjamini–Hochberg', words: 'Benjamini and Hochberg\'s procedure' },
  storey: { label: 'Storey\'s q-values', words: 'Storey\'s q-values (π₀ estimated at λ = 0.5)' },
};
export const ALPHAS = [0.01, 0.05, 0.1];

export const BY = {
  reference: { label: 'The reference sample\'s largest', n: 10, words: (n) => `each reference sample's ${n} largest clonotypes (each sample's ${n} largest for a subject without one)`, short: (n) => `the reference sample's ${n} largest` },
  largest: { label: 'Each sample\'s largest', n: 5, words: (n) => `each sample's ${n} largest clonotypes`, short: (n) => `each sample's ${n} largest` },
  list: { label: 'A list', words: (n, list) => `${list.length} clonotype${list.length === 1 ? '' : 's'} listed by ID or junction`, short: (n, list) => `${list.length} listed` },
  test: { label: 'Those a test called', n: 20, words: (n, list, t) => `the ${n} clonotypes ${t ? `${TESTS[t.test]?.name ?? 'a test'} called changed most significantly between ${t.from} and ${t.to}` : 'a test called'}`, short: (n, list, t) => `the ${n} most significant calls${t ? ` between ${t.from} and ${t.to}` : ''}` },
};

// The most clonotypes a list holds (internal/track, MaxList).
export const MAX_LIST = 1000;

// A dataset's choice: its own, else a reference sample's 10 largest. Answers { by, n, list }, and,
// following a test's calls, the comparison given (comparisonChoice).
export function trackingChoice(dataset, comparison = null) {
  const c = dataset?.tracking ?? {};
  const by = BY[c.by] ? c.by : 'reference';
  const list = Array.isArray(c.list) ? c.list : [];
  const out = { by, n: by === 'list' ? undefined : Number.isInteger(c.n) && c.n > 0 ? c.n : BY[by].n, list };
  if (by === 'test') out.test = comparison;
  return out;
}

// The choice as the program is asked it (api_tracking.go): a list only when following one, the
// comparison only when following its calls.
export function trackRequest(choice) {
  if (choice.by === 'list') return { by: 'list', list: choice.list };
  if (choice.by === 'test') return { by: 'test', n: choice.n, test: choice.test ? compareRequest(choice.test) : undefined };
  return { by: choice.by, n: choice.n };
}

// The choice in words, as the methods paragraph says it; and in short, as the view does.
export function choiceWords(choice) {
  return BY[choice.by]?.words(choice.n, choice.list ?? [], choice.test) ?? choice.by;
}
export function choiceShort(choice) {
  return BY[choice.by]?.short(choice.n, choice.list ?? [], choice.test) ?? choice.by;
}

// The time points of a study's design (api_study.go) that subjects can be compared at: each key
// once, in time order (a reference sample first, then baseline, days, visits and after), labeled
// as its first sample's time point (or reference) is.
export function timePoints(design) {
  const seen = new Map();
  for (const s of design?.subjects ?? []) {
    for (const p of s.samples) {
      if (!p.key || seen.has(p.key)) continue;
      const t = p.time;
      const rank = p.key === 'reference' ? [0, 0] : t?.kind === 'before' ? [1, 0] : t?.kind === 'day' ? [2, t.days] : t?.kind === 'visit' ? [3, t.value] : [4, 0];
      seen.set(p.key, { key: p.key, label: p.key === 'reference' ? p.reference : t?.label ?? p.key, rank, subjects: 0 });
    }
    for (const key of new Set(s.samples.map((p) => p.key).filter(Boolean))) seen.get(key).subjects += 1;
  }
  return [...seen.values()].sort((a, b) => a.rank[0] - b.rank[0] || a.rank[1] - b.rank[1]);
}

// A dataset's comparison: its own where its time points are among those given (timePoints), else
// the first two time points other than the reference sample's; the beta-binomial test where its
// variance can be fitted (on replicate samples, or on two time points named as stable: stable),
// two-sided, Benjamini–Hochberg at 0.01.
export function comparisonChoice(dataset, points) {
  const c = dataset?.comparison ?? {};
  const keys = points.map((p) => p.key);
  const others = points.filter((p) => p.key !== 'reference');
  const from = keys.includes(c.from) ? c.from : others[0]?.key ?? keys[0] ?? null;
  const to = keys.includes(c.to) && c.to !== from ? c.to : (others.find((p) => p.key !== from) ?? points.find((p) => p.key !== from))?.key ?? null;
  return {
    from, to,
    test: TESTS[c.test] ? c.test : 'auto',
    alternative: ALTERNATIVES[c.alternative] ? c.alternative : 'two-sided',
    correction: CORRECTIONS[c.correction] ? c.correction : 'bh',
    alpha: ALPHAS.includes(c.alpha) ? c.alpha : 0.01,
    stable: c.stable && keys.includes(c.stable.from) && keys.includes(c.stable.to) && c.stable.from !== c.stable.to ? { from: c.stable.from, to: c.stable.to } : null,
  };
}

// Whether a comparison's test fits a variance (and so takes a stable pair).
export const fits = (c) => c.test === 'auto' || c.test === 'betabinomial';

// The comparison as the program is asked it (api_expansion.go).
export function compareRequest(c) {
  return { from: c.from, to: c.to, test: c.test, alternative: c.alternative, correction: c.correction, alpha: c.alpha, ...(c.stable && fits(c) ? { stable: { from: c.stable.from, to: c.stable.to } } : {}) };
}

// The comparison in words, as the view and the methods paragraph say it.
export function comparisonWords(c) {
  return `${TESTS[c.test]?.words ?? c.test}, ${ALTERNATIVES[c.alternative]?.words ?? c.alternative}, the false discovery rate controlled within each subject by ${CORRECTIONS[c.correction]?.words ?? c.correction} at ${c.alpha}`;
}

// Chooses a dataset's comparison (any of its fields; the others kept).
export function useComparison(project, datasetId, changes, now = new Date().toISOString()) {
  const dataset = project.datasets.find((d) => d.id === datasetId);
  if (!dataset) return project;
  const before = dataset.comparison ?? {};
  const comparison = { ...before, ...changes };
  if (JSON.stringify(comparison) === JSON.stringify(before)) return project;
  return touch({
    ...project,
    datasets: project.datasets.map((d) => (d.id === datasetId ? { ...d, comparison } : d)),
    history: [...project.history, { time: now, action: 'Chose the time points compared', detail: `${dataset.name}: ${comparison.from ?? '—'} against ${comparison.to ?? '—'}` }],
  }, now);
}

// Chooses a dataset's clonotypes followed. A list keeps what it held, whatever is followed.
export function useTracking(project, datasetId, choice, now = new Date().toISOString()) {
  const dataset = project.datasets.find((d) => d.id === datasetId);
  if (!dataset) return project;
  const before = trackingChoice(dataset);
  const next = { by: choice.by ?? before.by, list: cleanList(choice.list ?? before.list) };
  next.n = next.by === 'list' ? undefined : Number.isInteger(choice.n) && choice.n > 0 ? choice.n : before.by === next.by ? before.n : BY[next.by].n;
  if (next.by === before.by && next.n === before.n && JSON.stringify(next.list) === JSON.stringify(before.list)) return project;
  const tracking = { by: next.by, ...(next.n ? { n: next.n } : {}), ...(next.list.length ? { list: next.list } : {}) };
  return touch({
    ...project,
    datasets: project.datasets.map((d) => (d.id === datasetId ? { ...d, tracking } : d)),
    history: [...project.history, { time: now, action: 'Chose the clonotypes followed', detail: `${dataset.name}: ${choiceWords(next)}` }],
  }, now);
}

// Adds clonotypes (IDs or junctions) to a dataset's list, and follows the list.
export function followAlso(project, datasetId, entries, now = new Date().toISOString()) {
  const dataset = project.datasets.find((d) => d.id === datasetId);
  if (!dataset) return project;
  const list = trackingChoice(dataset).list;
  return useTracking(project, datasetId, { by: 'list', list: [...list, ...entries] }, now);
}

// A list's entries from text: one a line, or separated by commas or spaces (a paired clonotype's
// chains are joined by "; " and kept together).
export function parseList(text) {
  return cleanList(String(text ?? '').split(/[\n,]+|\s{2,}|\t/).flatMap((part) => (part.includes(';') ? [part] : part.trim().split(/\s+/))));
}

function cleanList(list) {
  const out = [];
  for (const e of list ?? []) {
    const v = String(e).trim();
    if (v && !out.includes(v)) out.push(v);
  }
  return out.slice(0, MAX_LIST);
}
