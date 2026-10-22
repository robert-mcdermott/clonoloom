// The Jobs view: the program's jobs (internal/jobs), running and waiting first, then those a stop
// interrupted (to resume or discard), then the finished, newest first. Each shows its state,
// progress, times and runs; its whole log and its spec (every effective parameter) open below it.
// The list follows the program's events (ui/jobs.js); a job's progress is updated in place, so
// that its buttons are not lost to a redraw. Adapted from SituCraft 0.9.0 web/ui/mode-jobs.js.

import { h, icon, clear, formatBytes, relativeTime } from './dom.js';
import { confirmDialog, toast } from './overlays.js';
import { isFinished } from './jobs.js';

const KINDS = { example: 'Example data', diversity: 'Diversity', rarefaction: 'Rarefaction', overlap: 'Overlap' };
const STATES = {
  queued: ['', 'Waiting'], declared: ['', 'Waiting'], running: ['accent', 'Running'], validating: ['accent', 'Checking its outputs'],
  complete: ['ok', 'Complete'], cancelled: ['', 'Canceled'], failed: ['danger', 'Failed'], invalid: ['danger', 'Invalid outputs'], orphaned: ['warn', 'Interrupted'],
};

function duration(job) {
  if (!job.started) return '';
  const end = job.ended ? Date.parse(job.ended) : Date.now();
  const s = Math.max(0, (end - Date.parse(job.started)) / 1000);
  return s < 60 ? `${s.toFixed(1)} s` : `${Math.floor(s / 60)} min ${Math.round(s % 60)} s`;
}

export function mountJobs(app, container) {
  const list = h('div.jobs-list');
  const counts = h('span.crumbs');
  const cacheBox = h('div.jobs-cache');
  const root = h('div.view.jobs', h('div.workbench-head', h('h1', icon('jobs'), 'Jobs', counts)), h('div.workbench-scroll', h('div.jobs-body',
    h('p.muted.jobs-intro', 'Work that runs in the program: downloading or making example data, and the analyses that take a while (bootstrap intervals, rarefaction curves; distances and lineage trees later). Jobs keep running when the window reloads, and are kept when ClonoLoom stops: one it interrupted can be resumed, or discarded.'),
    list,
    cacheBox)));
  container.append(root);

  // The table cache (internal/cache): what it holds, and clearing it.
  async function showCache() {
    let u = null;
    try {
      u = await (await fetch('api/cache', { cache: 'no-store' })).json();
    } catch { /* shown as unknown */ }
    clear(cacheBox).append(h('div.section-title', 'The table cache'));
    if (!u?.enabled) {
      cacheBox.append(h('p.muted.jobs-empty', 'ClonoLoom has no cache folder: datasets are read each time they open.'));
      return;
    }
    const mb = formatBytes;
    cacheBox.append(
      h('p.muted.jobs-intro', `Tables read once are kept, so that datasets reopen without being read again (each file is checked by its SHA-256 first): ${u.tables} table${u.tables === 1 ? '' : 's'}, ${mb(u.size)}${u.limit ? ` of ${mb(u.limit)}` : ''}, in ${u.dir}. It can be cleared at any time; your files are never touched.`),
      h('div.btn-row', h('button.btn.small', { type: 'button', disabled: !u.tables, onclick: async () => {
        const response = await fetch('api/cache', { method: 'DELETE' });
        if (response.ok) {
          app.log('Cleared the table cache.');
          toast('The table cache is cleared.', { kind: 'ok' });
        } else toast('The cache could not be cleared.', { kind: 'error' });
        showCache();
      } }, icon('trash'), 'Clear the cache')));
  }
  showCache();
  const open = new Set(); // jobs whose log is shown
  const live = new Map(); // id → { bar, message, times }
  let shape = '';

  async function act(what, job) {
    try {
      if (what === 'cancel') await app.jobs.cancel(job.id);
      if (what === 'resume') {
        await app.jobs.resume(job.id);
        toast(`Resumed ${job.title}.`, { kind: 'ok' });
      }
      if (what === 'discard') {
        if (job.state === 'orphaned' && !(await confirmDialog({ title: 'Discard the job?', message: `${job.title} was interrupted; discarding it forgets it, and what it left unfinished. It can be started again from where it began.`, confirm: 'Discard', danger: true }))) return;
        await app.jobs.discard(job.id);
      }
    } catch (error) {
      toast(error.message, { kind: 'error' });
    }
  }

  // A job's whole log and spec, read when it is opened.
  async function details(job, host) {
    clear(host).append(h('p.muted', 'Reading its log…'));
    try {
      const full = await app.jobs.full(job.id);
      clear(host).append(
        h('div.job-log', ...(full.events ?? []).map((e) => h(`div.job-event.${e.level}`, h('span.mono.muted', new Date(e.time).toLocaleTimeString()), h('span', e.message)))),
        h('details.job-spec', h('summary', 'Its spec'), h('pre.mono', JSON.stringify(full.spec, null, 2))));
    } catch (error) {
      clear(host).append(h('p.muted', `Its log could not be read: ${error.message}`));
    }
  }

  function card(job) {
    const [badge, label] = STATES[job.state] ?? ['', job.state];
    const bar = h('div', { style: { width: `${Math.round(job.progress * 100)}%` } });
    const message = h('span.muted', job.message ?? '');
    const times = h('span.muted');
    live.set(job.id, { bar, message, times });
    const actions = h('div.btn-row');
    if (!isFinished(job.state)) actions.append(h('button.btn.small', { type: 'button', onclick: () => act('cancel', job) }, icon('close'), 'Stop'));
    if (job.state === 'orphaned' && job.resumable) actions.append(h('button.btn.small.primary', { type: 'button', onclick: () => act('resume', job) }, icon('play'), 'Resume'));
    if (isFinished(job.state)) actions.append(h('button.btn.small.ghost', { type: 'button', title: 'Forget this job', onclick: () => act('discard', job) }, icon('trash'), 'Discard'));
    const more = h('div.job-details');
    const toggle = h('button.link', { type: 'button', 'aria-expanded': String(open.has(job.id)), onclick: () => {
      if (open.has(job.id)) open.delete(job.id);
      else open.add(job.id);
      toggle.setAttribute('aria-expanded', String(open.has(job.id)));
      toggle.textContent = open.has(job.id) ? 'Hide its log' : 'Its log and spec';
      if (open.has(job.id)) details(job, more);
      else clear(more);
    } }, open.has(job.id) ? 'Hide its log' : 'Its log and spec');
    const el = h(`article.job-card${app.jobsFocus === job.id ? '.focused' : ''}`, { 'data-job': job.id },
      h('div.job-head', h('b', job.title), h('span.badge', KINDS[job.kind] ?? job.kind), h(`span.badge.${badge}`, label), h('span.spacer'), actions),
      isFinished(job.state) ? null : h('div.progress', bar),
      h('div.job-meta', message, times),
      job.error ? h('p.job-error', job.error) : null,
      job.failure?.remedy ? h('p.job-remedy', h('b', 'What to do: '), job.failure.remedy) : null,
      job.state === 'orphaned' ? h('p.muted.job-note', job.resumable ? 'ClonoLoom stopped before it finished. Resuming starts it again from its spec.' : 'This ClonoLoom cannot resume it; discard it, and start the work again.') : null,
      h('div.job-foot', toggle),
      more);
    if (open.has(job.id)) details(job, more);
    return el;
  }

  function updateLive() {
    for (const job of app.jobs.list()) {
      const parts = live.get(job.id);
      if (!parts) continue;
      parts.bar.style.width = `${Math.round(job.progress * 100)}%`;
      parts.message.textContent = job.message ?? '';
      const bits = [`queued ${relativeTime(job.created)}`];
      if (job.started) bits.push(`${isFinished(job.state) ? 'took' : 'running for'} ${duration(job)}`);
      if (job.runs > 1) bits.push(`run ${job.runs} times`);
      parts.times.textContent = ` · ${bits.join(' · ')}`;
    }
  }

  function render() {
    const jobs = app.jobs.list();
    const next = JSON.stringify(jobs.map((j) => [j.id, j.state, j.resumable, j.error ?? '']));
    if (next === shape) {
      updateLive();
      return;
    }
    shape = next;
    live.clear();
    clear(list);
    const running = jobs.filter((j) => !isFinished(j.state));
    const orphaned = jobs.filter((j) => j.state === 'orphaned').reverse();
    const finished = jobs.filter((j) => isFinished(j.state) && j.state !== 'orphaned').reverse();
    counts.textContent = jobs.length ? ` · ${running.length} running or waiting${orphaned.length ? `, ${orphaned.length} interrupted` : ''}, ${finished.length} finished` : '';
    const group = (title, items, empty) => {
      list.append(h('div.section-title', title));
      if (!items.length) list.append(h('p.muted.jobs-empty', empty));
      for (const job of items) list.append(card(job));
    };
    group('Running and waiting', running, 'Nothing is running.');
    if (orphaned.length) group('Interrupted', orphaned, '');
    group('Finished', finished, 'No job has finished yet.');
    updateLive();
    if (app.jobsFocus) {
      list.querySelector(`[data-job="${CSS.escape(app.jobsFocus)}"]`)?.scrollIntoView({ block: 'center' });
      app.jobsFocus = null;
    }
  }

  render();
  const timer = setInterval(updateLive, 1000);
  return {
    update(topics) {
      if (topics.has('jobs') || topics.has('mode')) render();
    },
    destroy() {
      clearInterval(timer);
      root.remove();
    },
  };
}
