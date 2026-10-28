// Checkpoints retain Go-owned evidence; browser recovery edits project values with
// the ordinary undo/redo store. No scientific result is parsed or recomputed here.
const ENVELOPE = new Set(['format', 'version', 'id', 'name', 'created', 'modified', 'history', 'checkpoints', 'portableArchive']);
const numericTokens = (text) => (text.match(/"(?:\\.|[^"\\])*"|-?(?:0|[1-9]\d*)(?:\.\d+)?(?:[eE][+-]?\d+)?/g) ?? []).filter((s) => !s.startsWith('"'));
export function checkpointAnalysis(text) {
  const value = JSON.parse(text);
  if (!value || Array.isArray(value) || typeof value !== 'object' || Object.keys(value).some((k) => ENVELOPE.has(k))) throw new Error('Unsupported checkpoint analysis.');
  // Refuse a browser round trip that changes even one numeric token. Exact result
  // files remain on the Go host and never pass through JavaScript numbers.
  if (JSON.stringify(numericTokens(text)) !== JSON.stringify(numericTokens(JSON.stringify(value)))) throw new Error('This checkpoint cannot be recovered exactly by this browser: numeric tokens would change.');
  return value;
}
export function addCheckpoint(project, record, now = new Date().toISOString()) {
  if (record.format !== 'clonoloom-checkpoint' || record.version !== 1 || record.project !== project.id || !/^cp-[0-9a-f]{64}$/.test(record.id)) throw new Error('Unsupported checkpoint or different project.');
  if ((project.checkpoints ?? []).some((c) => c.id === record.id)) throw new Error('Checkpoint is already attached.');
  return {...project, modified:now, checkpoints:[...(project.checkpoints ?? []), {id:record.id, name:record.name, created:record.created}], history:[...(project.history ?? []), {time:now, action:'Created checkpoint', detail:`${record.name} (${record.id}); saved evidence, not recomputation.`}]};
}
export function recoverCheckpoint(project, response, now = new Date().toISOString()) {
  const record = response.checkpoint;
  if (record?.format !== 'clonoloom-checkpoint' || record.version !== 1 || record.project !== project.id || response.recomputed !== false) throw new Error('Unsupported recovery or different project.');
  const analysis = checkpointAnalysis(response.analysisJSON);
  const next = Object.fromEntries(Object.entries(project).filter(([key]) => ENVELOPE.has(key)));
  return {...next, ...analysis, modified:now, history:[...(project.history ?? []), {time:now, action:'Recovered checkpoint', detail:`${record.name} (${record.id}); saved settings and result bytes restored, no scientific recomputation.`}]};
}
