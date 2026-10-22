// The example datasets (api_examples.go; clonoloom-spec/requirements.md, D13 and E2): public
// data listed with their source, size, license and citation; downloaded from their publishers
// only when asked (data under a provider's terms of use only once those are accepted), every file
// checked against its recorded SHA-256; opened as datasets that carry their license and citation.
// Adapted from SituCraft 0.9.0 web/ui/examples.js.

import { h, icon, clear, formatBytes } from './dom.js';
import { showDialog, toast } from './overlays.js';

async function getJSON(url, options = {}) {
  const response = await fetch(url, { cache: 'no-store', ...options });
  const body = await response.json().catch(() => ({}));
  if (!response.ok) {
    const error = new Error(body.error ?? `${url}: HTTP ${response.status}`);
    error.code = body.code;
    throw error;
  }
  return body;
}

const LICENSE_BADGE = { open: 'ok', terms: 'warn' };

// A link to a publisher's page, in the browser.
const external = (href, text) => h('a', { href, target: '_blank', rel: 'noopener noreferrer' }, text);

export function createExamples(app) {
  let list = null;
  let dir = '';
  let offline = false;
  let hosts = [];
  let error = null;
  let timer = 0;
  const listeners = new Set();
  const notify = () => listeners.forEach((fn) => fn());

  async function refresh() {
    clearTimeout(timer);
    const before = new Map((list ?? []).map((e) => [e.name, e.state]));
    try {
      const body = await getJSON('/api/examples');
      list = body.examples ?? [];
      dir = body.dir ?? '';
      offline = Boolean(body.offline);
      hosts = body.hosts ?? [];
      error = null;
    } catch (e) {
      error = e.message;
    }
    for (const e of list ?? []) {
      if (before.get(e.name) !== 'downloading' || e.state === 'downloading') continue;
      if (e.state === 'ready') {
        app.log(e.generated ? `Made ${e.title} in ${e.path}; its truth is in its truth folder.` : `Downloaded ${e.title} into ${e.path}; every file checked against its recorded SHA-256.`);
        toast(`${e.title} is ready.`, { kind: 'ok', timeout: 6000, action: e.opens?.length ? { label: `Open ${e.opens[0].title}`, onClick: () => open(e, 0) } : null });
      } else if (e.download?.error) {
        const verb = e.generated ? 'made' : 'downloaded';
        app.log(`${e.title} was not ${verb}: ${e.download.error}`);
        toast(`${e.title} was not ${verb}: ${e.download.error}`, { kind: 'error' });
      }
    }
    // A download's progress, while one runs.
    if ((list ?? []).some((e) => e.state === 'downloading')) timer = setTimeout(refresh, 500);
    notify();
    return list;
  }

  // Data under a provider's terms of use: their terms shown, and accepted, first.
  function acceptTerms(example) {
    return new Promise((resolve) => {
      const check = h('input', { type: 'checkbox', id: 'accept-terms' });
      let accepted = false;
      const { dialog } = showDialog({
        title: 'Terms of use',
        content: [
          h('p', h('b', example.title), ` is published by ${example.publisher} under its own terms, not an open license: `, h('b', example.license), '.'),
          h('p', 'Read the terms before you download. ClonoLoom fetches the files for you from the publisher, as your browser would; it keeps them in your examples folder and never passes them on.'),
          h('p', external(example.licenseUrl, 'Read the terms of use'), ' · ', external(example.page, 'The project\'s page')),
          h('label.check-row', check, h('span', 'I have read the terms and will use these data under them.')),
        ],
        buttons: [
          { label: 'Cancel', ghost: true },
          { label: `Accept and download ${formatBytes(example.size)}`, primary: true, onClick: () => { if (!check.checked) return false; accepted = true; return true; } },
        ],
        onClose: () => resolve(accepted),
      });
      const primary = dialog.querySelector('.dialog-foot .btn.primary');
      primary.disabled = true;
      check.addEventListener('change', () => { primary.disabled = !check.checked; });
    });
  }

  async function download(example) {
    let accept = false;
    if (example.licenseClass !== 'open') {
      accept = await acceptTerms(example);
      if (!accept) return;
      app.log(`Accepted ${example.license} (${example.licenseUrl}) for ${example.title}.`);
    }
    try {
      await getJSON(`/api/examples/${encodeURIComponent(example.name)}`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ accept }) });
      app.log(example.generated ? `Making ${example.title} in ${dir}.` : `Downloading ${example.title} (${formatBytes(example.size)}) from ${example.publisher} into ${dir}.`);
    } catch (e) {
      toast(e.message, { kind: 'error' });
    }
    await refresh();
  }

  async function cancel(example) {
    try {
      await getJSON(`/api/examples/${encodeURIComponent(example.name)}/cancel`, { method: 'POST' });
    } catch (e) {
      toast(e.message, { kind: 'error' });
    }
    await refresh();
  }

  async function remove(example) {
    try {
      await getJSON(`/api/examples/${encodeURIComponent(example.name)}`, { method: 'DELETE' });
      app.log(`Removed ${example.title} from ${example.path}.`);
    } catch (e) {
      toast(e.message, { kind: 'error' });
    }
    await refresh();
  }

  // One of the example's openings as a dataset of the project.
  async function open(example, k) {
    const opening = example.opens[k];
    return app.openData(example.paths[k], { name: opening.name });
  }

  function card(e, close) {
    const progress = e.state === 'downloading' && e.download
      ? h('div.example-progress',
        h('div.progress', h('div', { style: { width: `${Math.round(100 * e.download.fraction)}%` } })),
        h('span.muted.small', `${Math.round(100 * e.download.fraction)}% · ${e.download.message}`))
      : null;
    let actions;
    if (e.state === 'ready') {
      actions = [
        ...e.opens.map((o, k) => h('button.btn.small.primary', { type: 'button', title: o.summary, onclick: async () => { close(); await open(e, k); } }, icon('folder'), `Open ${o.title}`)),
        h('span.spacer'),
        h('button.btn.small.ghost', { type: 'button', title: `Delete it from ${e.path}`, onclick: () => remove(e) }, icon('trash'), 'Remove'),
      ];
    } else if (e.state === 'downloading') {
      actions = [h('button.btn.small', { type: 'button', onclick: () => cancel(e) }, 'Cancel')];
    } else if (e.generated) {
      actions = [h('button.btn.small.primary', { type: 'button', onclick: () => download(e) }, icon('sparkles'), `Make · about ${formatBytes(e.size)}`)];
    } else {
      actions = [h('button.btn.small.primary', { type: 'button', disabled: offline, onclick: () => download(e) }, icon('download'), `${e.state === 'incomplete' ? 'Download again' : 'Download'} · ${formatBytes(e.size)}`)];
    }
    return h('div.card.example-card',
      h('div.example-head',
        h('h4', e.title),
        e.generated
          ? h('span.badge.accent', { title: e.license }, 'Simulated')
          : h(`span.badge.${LICENSE_BADGE[e.licenseClass] ?? 'accent'}`, { title: e.license }, e.licenseClass === 'open' ? e.license.replace(/\s*\(.*\)$/, '') : 'Terms of use')),
      h('p', e.description),
      h('ul.example-opens', ...e.opens.map((o) => h('li', h('b', o.title), ': ', o.summary))),
      h('dl.kv.small',
        h('dt', 'Source'), h('dd', e.page ? external(e.page, e.source) : e.source),
        h('dt', 'License'), h('dd', e.licenseUrl ? external(e.licenseUrl, e.license) : e.license),
        h('dt', 'Cite'), h('dd', e.citation),
        h('dt', 'Size'), h('dd', e.generated ? `about ${formatBytes(e.size)}` : `${formatBytes(e.size)} in ${e.files.length} file${e.files.length === 1 ? '' : 's'}`)),
      e.download?.error && e.state !== 'downloading' ? h('div.callout.danger', h('p', e.download.error)) : null,
      progress,
      h('div.btn-row', ...actions));
  }

  function dialog() {
    const content = h('div.examples');
    let close = () => {};
    const render = () => {
      clear(content);
      if (error) content.append(h('div.callout.danger', h('p', error)));
      if (!list) {
        content.append(h('p.muted', 'Reading the list…'));
        return;
      }
      content.append(h('p', 'Public immune repertoires to learn ClonoLoom on and to check it against: single-cell TCR and BCR from 10x Genomics, the AIRR Community\'s test files, and bulk immunoSEQ exports, each opened as a dataset that carries its source, license and citation into your methods; and simulated studies, made on this computer, whose truth is known.'));
      if (offline) content.append(h('div.callout', h('p', 'ClonoLoom was started with --offline: it downloads nothing. Examples already downloaded open as usual, and simulated studies are made as usual.')));
      content.append(...list.map((e) => card(e, () => close())));
      content.append(h('p.muted.small', `Downloaded only when you ask, from ${hosts.join(', ')}, into ${dir}. Every file is checked against the size and SHA-256 recorded when ClonoLoom was released; a file the publisher has changed is refused.`));
    };
    listeners.add(render);
    const shown = showDialog({ title: 'Example data', width: 'wide', content, onClose: () => listeners.delete(render) });
    close = shown.close;
    render();
    refresh();
  }

  return { refresh, dialog, list: () => list };
}
