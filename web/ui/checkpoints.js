// Named checkpoints share the Reports view and the ordinary project undo history.
import { h, clear, downloadBlob } from './dom.js';
import { promptDialog, confirmDialog } from './overlays.js';
import { addCheckpoint, recoverCheckpoint } from '../lib/checkpoints.js';

export function mountCheckpoints(app, container) {
  const {store} = app;
  const root = h('section.card.checkpoints-view'); container.append(root);
  let chosen = '', against = '', difference = null, differenceText = '', error = '', busy = false, controller = null, alive = true, generation = store.state.generation;
  const entries = () => store.project.checkpoints ?? [];
  async function request(path, data) {
    const response = await fetch(`api/checkpoints${path}`, {method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify(data), signal:controller.signal});
    const text = await response.text(), value = JSON.parse(text);
    if (!response.ok) throw new Error(value.error ?? 'Checkpoint operation failed.');
    return {value, text};
  }
  async function perform(work) {
    if (busy) throw new Error('Wait for the checkpoint operation.');
    const project = store.project, same = store.sameProject();
    const valid = () => alive && same() && store.project === project;
    controller = new AbortController(); busy = true; error = ''; difference = null; render();
    try { return await work(project, valid); }
    catch (e) { if (alive && same()) error = e.name === 'AbortError' ? 'Checkpoint operation cancelled; no analysis recovered.' : e.message; throw e; }
    finally { if (alive && same()) { busy = false; controller = null; render(); } }
  }
  async function create(name) {
    if (name === undefined) name = await promptDialog({title:'Create checkpoint', label:'Name', value:`Checkpoint ${entries().length + 1}`, hint:'Keep the current analysis and attached completed result files in this local library. Original source files are not copied.'});
    if (!name) return null;
    return perform(async (project, valid) => {
      const {value} = await request('', {name, project});
      if (!valid()) throw new Error('The analysis changed while checkpointing; the checkpoint was not attached.');
      store.commit(addCheckpoint(project, value), `Checkpoint ${value.name}`); chosen = value.id;
      return value;
    });
  }
  async function compare(id = chosen, to = against) {
    return perform(async (project, valid) => {
      const {value, text} = await request(`/${encodeURIComponent(id)}/compare`, {project, ...(to ? {against:to} : {})});
      if (!valid()) throw new Error('The analysis changed while comparing; compare again.');
      chosen = id; against = to; difference = value; differenceText = text; return value;
    });
  }
  async function recover(id = chosen, prompt = false) {
    if (prompt && !await confirmDialog({title:'Recover checkpoint?', message:'Restore its analysis settings and attached result bytes? The current analysis remains available through Undo. Original source files and running jobs are not restored or rerun.', confirm:'Recover'})) return null;
    return perform(async (project, valid) => {
      const {value} = await request(`/${encodeURIComponent(id)}/recover`, {project});
      if (!valid()) throw new Error('The analysis changed during recovery; it was not replaced. Verified result files may remain in the library.');
      store.recover(recoverCheckpoint(project, value), `Recover ${value.checkpoint.name}`);
      store.setUI({focus:null, selection:null}, ['focus','selection']); chosen = id;
      return {checkpoint:value.checkpoint.id, recomputed:false, limits:value.limits};
    });
  }
  const button = (name, action, disabled = false) => h('button.btn.small', {type:'button', disabled:busy || disabled, onclick:() => action().catch(() => {})}, name);
  function render() {
    if (!alive) return;
    const records = entries(); if (!records.some((c) => c.id === chosen)) chosen = records.at(-1)?.id ?? '';
    if (against && !records.some((c) => c.id === against)) against = '';
    clear(root);
    root.append(h('h2', 'Analysis checkpoints'), h('p', 'Keep immutable local copies of analysis settings and attached completed results. Compare recorded sources, definitions, QC, populations, parameters and result identities, then recover with Undo available.'), h('p.muted', 'Recovery restores saved evidence; it does not recompute a statistic. Original source files, unattached parents, reference installations and running jobs are not backed up. Conflicting result bytes are refused, never overwritten.'), button('Create checkpoint', () => create()));
    if (records.length) {
      root.append(h('label.field', h('span', 'Checkpoint'), h('select.input', {'aria-label':'Analysis checkpoint', disabled:busy, onchange:(e) => {chosen = e.target.value; difference = null; render();}}, ...records.map((c) => h('option', {value:c.id, selected:c.id === chosen}, c.name)))),
        h('label.field', h('span', 'Compare with'), h('select.input', {'aria-label':'Checkpoint comparison', disabled:busy, onchange:(e) => {against = e.target.value; difference = null; render();}}, h('option', {value:''}, 'Current analysis'), ...records.map((c) => h('option', {value:c.id, selected:c.id === against}, c.name)))),
        h('div.btn-row', button('Compare analysis', () => compare()), button('Recover checkpoint', () => recover(chosen, true))));
    } else root.append(h('p.muted', 'Create a checkpoint before revising a study. Checkpoint names and references remain part of project undo/redo.'));
    if (controller) root.append(h('button.btn.small', {type:'button', onclick:() => controller.abort()}, 'Cancel checkpoint operation'));
    if (error) root.append(h('p.callout.danger', {role:'alert'}, error));
    if (difference) {
      root.append(h('h3', 'Recorded analysis differences'), h('p', `${difference.changes.length} recorded difference${difference.changes.length === 1 ? '' : 's'}. No effect sizes or significance inferred.`), h('p.muted', 'Paths identify exact fields; array indices start at zero and ordering is significant. “(absent)” differs from null. Source paths and hashes are compared as records, not reopened or independently verified.'));
      if (!difference.changes.length) root.append(h('p', 'No recorded analysis or result-evidence differences.'));
      for (const c of difference.changes) root.append(h('details.checkpoint-change', h('summary', c.summary), h('p', 'Before (exact JSON):'), h('pre', {style:{whiteSpace:'pre-wrap', overflowWrap:'anywhere'}}, c.beforeJSON), h('p', 'After (exact JSON):'), h('pre', {style:{whiteSpace:'pre-wrap', overflowWrap:'anywhere'}}, c.afterJSON)));
      root.append(button('Export checkpoint differences JSON', async () => downloadBlob(new Blob([differenceText], {type:'application/json'}), 'checkpoint-differences.json')), h('p.muted', difference.limits));
    }
  }
  const view = {create, compare, recover, state:() => ({chosen, difference, busy, error})}; app.checkpointsView = view; render();
  return {update(topics) {
    if (generation !== store.state.generation) {controller?.abort();controller=null;generation=store.state.generation;chosen='';against='';difference=null;busy=false;error='';}
    if (topics.has('project') || topics.has('mode')) { difference = null; render(); }
  }, destroy() {alive=false;controller?.abort();if(app.checkpointsView===view)app.checkpointsView=null;root.remove();}};
}
