// Which clonotypes a dataset's Tracking view follows (clonoloom-spec/requirements.md, T1), as the
// project keeps it with the dataset: a reference sample's largest, each sample's largest, or a list
// of clonotypes (by ID or junction) the user made, which the Clonotypes view adds to. The program
// follows them (internal/track); every edit returns a new project.

import { touch } from './project.js';

export const BY = {
  reference: { label: 'The reference sample\'s largest', n: 10, words: (n) => `each reference sample's ${n} largest clonotypes (each sample's ${n} largest for a subject without one)`, short: (n) => `the reference sample's ${n} largest` },
  largest: { label: 'Each sample\'s largest', n: 5, words: (n) => `each sample's ${n} largest clonotypes`, short: (n) => `each sample's ${n} largest` },
  list: { label: 'A list', words: (n, list) => `${list.length} clonotype${list.length === 1 ? '' : 's'} listed by ID or junction`, short: (n, list) => `${list.length} listed` },
};

// The most clonotypes a list holds (internal/track, MaxList).
export const MAX_LIST = 1000;

// A dataset's choice: its own, else a reference sample's 10 largest. Answers { by, n, list }.
export function trackingChoice(dataset) {
  const c = dataset?.tracking ?? {};
  const by = BY[c.by] ? c.by : 'reference';
  const list = Array.isArray(c.list) ? c.list : [];
  return { by, n: by === 'list' ? undefined : Number.isInteger(c.n) && c.n > 0 ? c.n : BY[by].n, list };
}

// The choice as the program is asked it (api_tracking.go): a list only when following one.
export function trackRequest(choice) {
  return choice.by === 'list' ? { by: 'list', list: choice.list } : { by: choice.by, n: choice.n };
}

// The choice in words, as the methods paragraph says it; and in short, as the view does.
export function choiceWords(choice) {
  return BY[choice.by]?.words(choice.n, choice.list ?? []) ?? choice.by;
}
export function choiceShort(choice) {
  return BY[choice.by]?.short(choice.n, choice.list ?? []) ?? choice.by;
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
