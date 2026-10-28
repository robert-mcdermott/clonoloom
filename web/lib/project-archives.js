// Scientific evidence stays in Go. Refuse project loading that would change numeric tokens.
import { parseProject } from './project.js';
const tokens = (text) => (text.match(/"(?:\\.|[^"\\])*"|-?(?:0|[1-9]\d*)(?:\.\d+)?(?:[eE][+-]?\d+)?/g) ?? []).filter((s) => !s.startsWith('"'));
export function parseExactProject(text) {
  const value = JSON.parse(text);
  if (JSON.stringify(tokens(text)) !== JSON.stringify(tokens(JSON.stringify(value)))) throw new Error('This project cannot be opened exactly by this browser: numeric tokens would change. Original evidence remains unchanged.');
  return parseProject(text);
}
export function archiveCounts(record) {
  return (record?.sources ?? []).reduce((counts, s) => ({...counts, [s.status]:(counts[s.status] ?? 0)+1}), {});
}
